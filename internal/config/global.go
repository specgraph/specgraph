// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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

// AuthConfig configures authentication and authorization.
type AuthConfig struct {
	Mode          string                `yaml:"mode"`
	DefaultRole   string                `yaml:"default_role"`
	APIKeys       []APIKeyConfig        `yaml:"api_keys"`
	OIDCProviders []OIDCProviderConfig  `yaml:"oidc_providers"`
	Roles         map[string]RoleConfig `yaml:"roles"`
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
// writes defaults and returns them.
func LoadGlobal(path string) (*GlobalConfig, error) {
	cfg := globalDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if writeErr := writeGlobal(path, cfg); writeErr != nil {
			return nil, fmt.Errorf("write default config: %w", writeErr)
		}
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
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
			Listen:  "127.0.0.1:7890",
			Mode:    "service",
			Backend: "postgres",
			Postgres: PostgresConfig{ //nolint:gosec // dev default, not a real credential
				URL: "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable",
			},
			Docker: true,
		},
		Client: ClientConfig{
			DefaultServer: "http://127.0.0.1:7890",
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
