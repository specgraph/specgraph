package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
}

type ServerConfig struct {
	Mode   string `yaml:"mode"`   // docker | external
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Remote string `yaml:"remote"` // if set, CLI-only mode
}

type StorageConfig struct {
	Backend  string         `yaml:"backend"` // memgraph | postgres
	Memgraph MemgraphConfig `yaml:"memgraph"`
	Postgres PostgresConfig `yaml:"postgres"`
	Docker   DockerConfig   `yaml:"docker"`
}

type MemgraphConfig struct {
	BoltURI string `yaml:"bolt_uri"`
}

type PostgresConfig struct {
	URL string `yaml:"url"`
}

type DockerConfig struct {
	ComposeFile string `yaml:"compose_file"`
}

func (c *Config) IsRemote() bool {
	return c.Server.Remote != ""
}

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
