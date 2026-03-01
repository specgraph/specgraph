// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package config handles loading and persisting SpecGraph configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the full SpecGraph configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
}

// ServerConfig describes how the SpecGraph server runs.
type ServerConfig struct {
	Mode   string `yaml:"mode"`   // docker | external
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Remote string `yaml:"remote"` // if set, CLI-only mode
}

// StorageConfig describes the storage backend and its options.
type StorageConfig struct {
	Backend  string         `yaml:"backend"` // memgraph | postgres
	Memgraph MemgraphConfig `yaml:"memgraph"`
	Postgres PostgresConfig `yaml:"postgres"`
	Docker   DockerConfig   `yaml:"docker"`
}

// MemgraphConfig holds Memgraph-specific connection settings.
type MemgraphConfig struct {
	BoltURI string `yaml:"bolt_uri"`
}

// PostgresConfig holds Postgres-specific connection settings.
type PostgresConfig struct {
	URL string `yaml:"url"`
}

// DockerConfig holds Docker Compose settings.
type DockerConfig struct {
	ComposeFile string `yaml:"compose_file"`
}

// IsRemote reports whether the config targets a remote server.
func (c *Config) IsRemote() bool {
	return c.Server.Remote != ""
}

// Load reads and parses a YAML config file, applying sensible defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(cfg)
	return cfg, nil
}

// Write persists the config to the given path as YAML.
func (c *Config) Write(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9090
	}
	if cfg.Server.Mode == "" && !cfg.IsRemote() {
		cfg.Server.Mode = "docker"
	}
	if cfg.Storage.Memgraph.BoltURI == "" {
		cfg.Storage.Memgraph.BoltURI = "bolt://localhost:7687"
	}
	if cfg.Storage.Docker.ComposeFile == "" {
		cfg.Storage.Docker.ComposeFile = ".specgraph/docker-compose.yaml"
	}
}
