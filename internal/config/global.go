// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultProbeInterval/Timeout are chosen so the 5s cache refresh stays
// fresh against kubelet's default periodSeconds=10, with 2s headroom for
// pgxpool.Ping.
const (
	DefaultProbeInterval = 5 * time.Second
	DefaultProbeTimeout  = 2 * time.Second
)

// GlobalConfig is the new top-level config at ~/.config/specgraph/config.yaml.
type GlobalConfig struct {
	Server ServerSection `yaml:"server"`
	Client ClientConfig  `yaml:"client"`
	Auth   AuthConfig    `yaml:"auth"`
	Export ExportConfig  `yaml:"export"`
}

// ExportConfig holds settings for project export and import operations.
type ExportConfig struct {
	SigningKey string `yaml:"signing_key"`
}

// ServerSection configures the specgraph server daemon.
type ServerSection struct {
	Listen   string         `yaml:"listen"`
	Mode     string         `yaml:"mode"`
	Backend  string         `yaml:"backend"`
	Postgres PostgresConfig `yaml:"postgres"`
	Docker   bool           `yaml:"docker"`
	Probes   ProbesConfig   `yaml:"probes,omitempty"`
}

// ProbesConfig configures the plain-HTTP Kubernetes/Knative probe listener.
// When Listen is empty, the probe endpoints are disabled. Callers should
// treat raw Interval/Timeout fields as untrusted and consume the result of
// Resolved() — that's the single path that fuses defaulting and validation.
type ProbesConfig struct {
	Listen   string        `yaml:"listen,omitempty"`
	Interval time.Duration `yaml:"interval,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`
}

// Resolved returns a copy with zero-valued Interval/Timeout filled from
// DefaultProbeInterval/DefaultProbeTimeout, after rejecting negative
// durations and a per-probe timeout that exceeds the interval (probes
// would overlap and stack up behind a slow Postgres). Fusing defaulting
// and validation prevents callers from reading raw fields and skipping
// either step.
func (p ProbesConfig) Resolved() (ProbesConfig, error) {
	if p.Interval < 0 {
		return ProbesConfig{}, fmt.Errorf("probes.interval must be non-negative, got %s", p.Interval)
	}
	if p.Timeout < 0 {
		return ProbesConfig{}, fmt.Errorf("probes.timeout must be non-negative, got %s", p.Timeout)
	}
	out := p
	if out.Interval == 0 {
		out.Interval = DefaultProbeInterval
	}
	if out.Timeout == 0 {
		out.Timeout = DefaultProbeTimeout
	}
	if out.Timeout > out.Interval {
		return ProbesConfig{}, fmt.Errorf("probes.timeout (%s) must not exceed probes.interval (%s)",
			out.Timeout, out.Interval)
	}
	return out, nil
}

// ClientConfig configures how CLI commands connect to the server.
type ClientConfig struct {
	DefaultServer string  `yaml:"default_server"`
	Routes        []Route `yaml:"routes,omitempty"`
}

// Route maps a project slug glob to a server URL.
type Route struct {
	Project string `yaml:"project"`
	Server  string `yaml:"server"`
}

// OIDCConfig wraps the OIDC provider list and JIT settings under a
// nested auth.oidc key. Replaces the flat AuthConfig.OIDCProviders.
type OIDCConfig struct {
	Providers []OIDCProviderConfig `yaml:"providers"`
	JITCreate JITCreateConfig      `yaml:"jit_create"`
}

// JITCreateConfig parametrizes just-in-time Human creation on first
// OIDC sign-in. Consumed by the identity resolver (Authn plan).
type JITCreateConfig struct {
	Enabled              bool     `yaml:"enabled"`
	DefaultRole          string   `yaml:"default_role"`
	RateLimitPerHour     int      `yaml:"rate_limit_per_hour"`
	EmailDomainAllowlist []string `yaml:"email_domain_allowlist"`
}

// AuthConfig configures authentication and authorization.
type AuthConfig struct {
	Mode          string                `yaml:"mode"`           // deprecated; ignored after Authn plan
	DefaultRole   string                `yaml:"default_role"`   // deprecated; ignored after Authn plan
	APIKeys       []APIKeyConfig        `yaml:"api_keys"`       // ignored after Authn plan (storage owns)
	OIDCProviders []OIDCProviderConfig  `yaml:"oidc_providers"` // deprecated; superseded by OIDC.Providers
	Roles         map[string]RoleConfig `yaml:"roles"`
	OIDC          OIDCConfig            `yaml:"oidc"`
}

// APIKeyConfig defines a single API key and its associated role.
type APIKeyConfig struct {
	ID   string `yaml:"id"`
	Key  string `yaml:"key"`
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

// RoleConfig defines a custom role with explicit permissions.
type RoleConfig struct {
	Permissions []string `yaml:"permissions"`
}

// OIDCProviderConfig defines a single OIDC identity provider.
type OIDCProviderConfig struct {
	ID            string         `yaml:"id"`
	Issuer        string         `yaml:"issuer"`
	ClientID      string         `yaml:"client_id"`
	Audience      string         `yaml:"audience"`
	ClaimsMapping []ClaimMapping `yaml:"claims_mapping"`
}

// ClaimMapping maps a JWT claim value to a SpecGraph role.
type ClaimMapping struct {
	Claim string `yaml:"claim"`
	Value string `yaml:"value"`
	Role  string `yaml:"role"`
}

// LoadGlobal loads the global config from path. If the file doesn't exist,
// writes defaults and returns them. Use LoadGlobalExplicit when path was
// operator-supplied (via --config), where materializing defaults at a
// typo'd path would silently mask the error.
func LoadGlobal(path string) (*GlobalConfig, error) {
	return loadGlobalAt(path, true)
}

// LoadGlobalExplicit loads the global config from an operator-supplied path,
// returning an error if the file does not exist. Server commands call this
// when --config is set so a missing or mistyped path fails loudly instead of
// being materialized as a default config at an unexpected location.
func LoadGlobalExplicit(path string) (*GlobalConfig, error) {
	return loadGlobalAt(path, false)
}

func loadGlobalAt(path string, materializeDefaults bool) (*GlobalConfig, error) {
	cfg := globalDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if !materializeDefaults {
			return nil, fmt.Errorf("config file not found at %s", path)
		}
		if writeErr := writeGlobal(path, cfg); writeErr != nil {
			return nil, fmt.Errorf("write default config: %w", writeErr)
		}
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Migrate the deprecated flat auth.oidc_providers into auth.oidc.providers.
	// New path wins if both are set (no migration in that case).
	if len(cfg.Auth.OIDCProviders) > 0 && len(cfg.Auth.OIDC.Providers) == 0 {
		cfg.Auth.OIDC.Providers = cfg.Auth.OIDCProviders
		slog.Warn("auth.oidc_providers is deprecated; move providers under auth.oidc.providers")
	}

	return cfg, nil
}

// ResolveServer determines the server URL for a given project slug.
func (c *GlobalConfig) ResolveServer(projectSlug, repoOverride string) string {
	if repoOverride != "" {
		return repoOverride
	}
	for _, r := range c.Client.Routes {
		matched, matchErr := filepath.Match(r.Project, projectSlug)
		if matchErr != nil {
			continue
		}
		if matched {
			return r.Server
		}
	}
	return c.Client.DefaultServer
}

func globalDefaults() *GlobalConfig {
	return &GlobalConfig{
		Server: ServerSection{
			Listen:  "0.0.0.0:9090",
			Mode:    "service",
			Backend: "postgres",
			Postgres: PostgresConfig{ //nolint:gosec // dev default, not a real credential
				URL: "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable",
			},
			Docker: true,
		},
		Client: ClientConfig{
			DefaultServer: "http://127.0.0.1:9090",
		},
	}
}

func writeGlobal(path string, cfg *GlobalConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}
