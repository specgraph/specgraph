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
	Server ServerSection `yaml:"server" koanf:"server"`
	Client ClientConfig  `yaml:"client" koanf:"client"`
	Auth   AuthConfig    `yaml:"auth" koanf:"auth"`
	Export ExportConfig  `yaml:"export" koanf:"export"`
}

// ExportConfig holds settings for project export and import operations.
type ExportConfig struct {
	SigningKey string `yaml:"signing_key" koanf:"signing_key"`
}

// ServerSection configures the specgraph server daemon.
type ServerSection struct {
	Listen   string         `yaml:"listen" koanf:"listen"`
	Mode     string         `yaml:"mode" koanf:"mode"`
	Backend  string         `yaml:"backend" koanf:"backend"`
	Postgres PostgresConfig `yaml:"postgres" koanf:"postgres"`
	Docker   bool           `yaml:"docker" koanf:"docker"`
	Probes   ProbesConfig   `yaml:"probes,omitempty" koanf:"probes"`
}

// ProbesConfig configures the plain-HTTP Kubernetes/Knative probe listener.
// When Listen is empty, the probe endpoints are disabled. Callers should
// treat raw Interval/Timeout fields as untrusted and consume the result of
// Resolved() — that's the single path that fuses defaulting and validation.
type ProbesConfig struct {
	Listen   string        `yaml:"listen,omitempty" koanf:"listen"`
	Interval time.Duration `yaml:"interval,omitempty" koanf:"interval"`
	Timeout  time.Duration `yaml:"timeout,omitempty" koanf:"timeout"`
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
	DefaultServer string  `yaml:"default_server" koanf:"default_server"`
	Routes        []Route `yaml:"routes,omitempty" koanf:"routes"`
}

// Route maps a project slug glob to a server URL.
type Route struct {
	Project string `yaml:"project" koanf:"project"`
	Server  string `yaml:"server" koanf:"server"`
}

// OIDCConfig wraps the OIDC provider list and JIT settings under a
// nested auth.oidc key. Replaces the flat AuthConfig.OIDCProviders.
type OIDCConfig struct {
	Providers []OIDCProviderConfig `yaml:"providers" koanf:"providers"`
	JITCreate JITCreateConfig      `yaml:"jit_create" koanf:"jit_create"`
}

// JITCreateConfig parametrizes just-in-time Human creation on first
// OIDC sign-in. Consumed by the identity resolver (Authn plan).
type JITCreateConfig struct {
	Enabled              bool     `yaml:"enabled" koanf:"enabled"`
	DefaultRole          string   `yaml:"default_role" koanf:"default_role"`
	RateLimitPerHour     int      `yaml:"rate_limit_per_hour" koanf:"rate_limit_per_hour"`
	EmailDomainAllowlist []string `yaml:"email_domain_allowlist" koanf:"email_domain_allowlist"`
}

// AuthConfig configures authentication and authorization.
type AuthConfig struct {
	Mode          string               `yaml:"mode" koanf:"mode"`                   // deprecated; ignored after Authn plan
	DefaultRole   string               `yaml:"default_role" koanf:"default_role"`   // deprecated; ignored after Authn plan
	APIKeys       []APIKeyConfig       `yaml:"api_keys" koanf:"api_keys"`           // ignored after Authn plan (storage owns)
	OIDCProviders []OIDCProviderConfig `yaml:"oidc_providers" koanf:"oidc_providers"` // deprecated; superseded by OIDC.Providers
	Roles         []string             `yaml:"roles" koanf:"roles"`
	Policies      PolicyConfig         `yaml:"policies" koanf:"policies"`
	OIDC          OIDCConfig           `yaml:"oidc" koanf:"oidc"`
}

// PolicyConfig configures the Cedar authorization engine's policy
// sources. Built-in policies are always loaded; ExtraDirs adds operator
// policy directories (each *.cedar file becomes a DirectoryPolicySource).
type PolicyConfig struct {
	ExtraDirs []string `yaml:"extra_dirs" koanf:"extra_dirs"`
}

// APIKeyConfig defines a single API key and its associated role.
type APIKeyConfig struct {
	ID   string `yaml:"id" koanf:"id"`
	Key  string `yaml:"key" koanf:"key"`
	Name string `yaml:"name" koanf:"name"`
	Role string `yaml:"role" koanf:"role"`
}

// OIDCProviderConfig defines a single OIDC identity provider.
type OIDCProviderConfig struct {
	ID            string         `yaml:"id" koanf:"id"`
	Issuer        string         `yaml:"issuer" koanf:"issuer"`
	ClientID      string         `yaml:"client_id" koanf:"client_id"`
	Audience      string         `yaml:"audience" koanf:"audience"`
	ClaimsMapping []ClaimMapping `yaml:"claims_mapping" koanf:"claims_mapping"`
}

// ClaimMapping maps a JWT claim value to a SpecGraph role.
type ClaimMapping struct {
	Claim string `yaml:"claim" koanf:"claim"`
	Value string `yaml:"value" koanf:"value"`
	Role  string `yaml:"role" koanf:"role"`
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
