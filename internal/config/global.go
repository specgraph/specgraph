// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig is the new top-level config at ~/.config/specgraph/config.yaml.
type GlobalConfig struct {
	Server ServerSection `yaml:"server"`
	Client ClientConfig  `yaml:"client"`
}

// ServerSection configures the specgraph server daemon.
type ServerSection struct {
	Listen   string         `yaml:"listen"`
	Mode     string         `yaml:"mode"`
	Backend  string         `yaml:"backend"`
	Memgraph MemgraphConfig `yaml:"memgraph"`
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

// LoadGlobal loads the global config from path. If the file doesn't exist,
// writes defaults and returns them.
func LoadGlobal(path string) (*GlobalConfig, error) {
	cfg := globalDefaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
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
			Backend: "memgraph",
			Memgraph: MemgraphConfig{
				BoltURI: "bolt://localhost:7687",
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
