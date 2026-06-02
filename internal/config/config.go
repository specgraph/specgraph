// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package config handles loading and persisting SpecGraph configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/specgraph/specgraph/internal/storage"
	"gopkg.in/yaml.v3"
)

// Config holds the full SpecGraph configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Sync    SyncConfig    `yaml:"sync"`
}

// SyncConfig describes sync adapter settings.
type SyncConfig struct {
	GitHubRepo string `yaml:"github_repo"` // owner/repo for GitHub adapter; if empty, inferred from git remote
}

// ServerConfig describes how the SpecGraph server runs.
type ServerConfig struct {
	Mode   string `yaml:"mode"` // docker | external
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Remote string `yaml:"remote"` // if set, CLI-only mode
	// TLS controls the URL scheme used when constructing the base URL from Host
	// and Port (i.e. when Remote is not set). When true, "https" is used;
	// otherwise "http" is used. TLS has no effect when Remote is set directly —
	// the Remote value is used verbatim and must include the scheme.
	TLS bool `yaml:"tls"`
}

// StorageConfig describes the storage backend and its options.
type StorageConfig struct {
	Backend          string         `yaml:"backend"` // postgres
	Postgres         PostgresConfig `yaml:"postgres"`
	Docker           DockerConfig   `yaml:"docker"`
	ConstitutionPath string         `yaml:"constitution_path"`
}

// ConstitutionPrinciple represents a principle in the constitution YAML.
// It supports both string form ("Keep it simple") and struct form
// (statement/rationale/exceptions) for ergonomic flexibility.
type ConstitutionPrinciple struct {
	ID         string `yaml:"id,omitempty"`
	Statement  string `yaml:"statement"`
	Rationale  string `yaml:"rationale,omitempty"`
	Exceptions string `yaml:"exceptions,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler so that ConstitutionPrinciple
// can be decoded from either a plain scalar string or a full struct node.
func (p *ConstitutionPrinciple) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		p.Statement = value.Value
		return nil
	}
	// Struct form — use an alias to avoid infinite recursion.
	type raw ConstitutionPrinciple
	if err := value.Decode((*raw)(p)); err != nil {
		return fmt.Errorf("decode principle: %w", err)
	}
	return nil
}

// ConstitutionAntipattern represents an antipattern in the constitution YAML.
type ConstitutionAntipattern struct {
	Pattern string `yaml:"pattern"`
	Why     string `yaml:"why,omitempty"`
	Instead string `yaml:"instead,omitempty"`
}

// ConstitutionReference represents a reference in the constitution YAML.
type ConstitutionReference struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
}

// ConstitutionProcess holds process configuration in the constitution YAML.
type ConstitutionProcess struct {
	SpecReview     string                          `yaml:"spec_review,omitempty"`
	SecurityReview *ConstitutionSecurityReview     `yaml:"security_review,omitempty"`
	Deployment     *ConstitutionDeployment         `yaml:"deployment,omitempty"`
	Documentation  *ConstitutionDocumentation      `yaml:"documentation,omitempty"`
}

// ConstitutionSecurityReview holds security review configuration.
type ConstitutionSecurityReview struct {
	When string `yaml:"when,omitempty"`
}

// ConstitutionDeployment holds deployment configuration.
type ConstitutionDeployment struct {
	Strategy string `yaml:"strategy,omitempty"`
	Rollback string `yaml:"rollback,omitempty"`
}

// ConstitutionDocumentation holds documentation requirements configuration.
type ConstitutionDocumentation struct {
	APIDocs string `yaml:"api_docs,omitempty"`
	Runbook string `yaml:"runbook,omitempty"`
}

// ConstitutionConfig represents a constitution YAML document.
type ConstitutionConfig struct {
	Name         string                    `yaml:"name"`
	Layer        string                    `yaml:"layer"`
	Tech         ConstitutionTech          `yaml:"tech,omitempty"`
	Principles   []ConstitutionPrinciple   `yaml:"principles,omitempty"`
	Constraints  []string                  `yaml:"constraints,omitempty"`
	Antipatterns []ConstitutionAntipattern `yaml:"antipatterns,omitempty"`
	References   []ConstitutionReference   `yaml:"references,omitempty"`
	Process      *ConstitutionProcess      `yaml:"process,omitempty"`
}

// ConstitutionTech holds technology stack configuration.
type ConstitutionTech struct {
	Languages      ConstitutionLangs `yaml:"languages,omitempty"`
	Frameworks     map[string]string `yaml:"frameworks,omitempty"`
	Infrastructure map[string]string `yaml:"infrastructure,omitempty"`
	APIStandards   map[string]string `yaml:"api_standards,omitempty"`
	Data           map[string]string `yaml:"data,omitempty"`
}

// ConstitutionLangs holds language configuration.
type ConstitutionLangs struct {
	Primary          string            `yaml:"primary,omitempty"`
	Allowed          []string          `yaml:"allowed,omitempty"`
	Forbidden        []string          `yaml:"forbidden,omitempty"`
	ForbiddenReasons map[string]string `yaml:"forbidden_reasons,omitempty"`
}

// PostgresConfig holds Postgres-specific connection settings.
type PostgresConfig struct {
	URL string `yaml:"url" koanf:"url"`
}

// DockerConfig holds Docker Compose settings.
type DockerConfig struct {
	ComposeFile string `yaml:"compose_file"`
}

// validLayers are the accepted constitution layer strings in YAML files.
var validLayers = map[string]bool{
	"":        true, // maps to UNSPECIFIED
	"user":    true,
	"org":     true,
	"project": true,
	"domain":  true,
}

// ValidateLayer checks if a layer string is a valid ConstitutionLayer value.
func ValidateLayer(layer string) error {
	if !validLayers[strings.ToLower(layer)] {
		return fmt.Errorf("unknown constitution layer %q; valid values: user, org, project, domain", layer)
	}
	return nil
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

// LoadConstitutionYAML reads and parses a constitution YAML file.
func LoadConstitutionYAML(path string) (*ConstitutionConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read constitution: %w", err)
	}
	return ParseConstitutionYAML(data)
}

// ParseConstitutionYAML parses constitution YAML from raw bytes.
func ParseConstitutionYAML(data []byte) (*ConstitutionConfig, error) {
	c := &ConstitutionConfig{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse constitution: %w", err)
	}
	if err := ValidateLayer(c.Layer); err != nil {
		return nil, err
	}
	return c, nil
}

// WriteConstitutionYAML persists a constitution to the given path as YAML.
func WriteConstitutionYAML(path string, c *ConstitutionConfig) error {
	if err := ValidateLayer(c.Layer); err != nil {
		return fmt.Errorf("write constitution: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal constitution: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create constitution dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write constitution: %w", err)
	}
	return nil
}

// ToDomain converts a ConstitutionConfig (YAML) to a storage.Constitution domain type.
func (c *ConstitutionConfig) ToDomain() *storage.Constitution {
	return &storage.Constitution{
		Name:         c.Name,
		Layer:        storage.ConstitutionLayer(strings.ToLower(c.Layer)),
		Tech:         techToDomain(&c.Tech),
		Principles:   principlesToDomain(c.Principles),
		Constraints:  c.Constraints,
		Antipatterns: antipatternsToDomain(c.Antipatterns),
		References:   referencesToDomain(c.References),
		Process:      processToDomain(c.Process),
	}
}

func techToDomain(t *ConstitutionTech) *storage.TechStack {
	if t == nil {
		return nil
	}
	ts := &storage.TechStack{
		Frameworks:     t.Frameworks,
		Infrastructure: t.Infrastructure,
		APIStandards:   t.APIStandards,
		Data:           t.Data,
	}
	if t.Languages.Primary != "" || len(t.Languages.Allowed) > 0 || len(t.Languages.Forbidden) > 0 || len(t.Languages.ForbiddenReasons) > 0 {
		ts.Languages = &storage.Languages{
			Primary:          t.Languages.Primary,
			Allowed:          t.Languages.Allowed,
			Forbidden:        t.Languages.Forbidden,
			ForbiddenReasons: t.Languages.ForbiddenReasons,
		}
	}
	return ts
}

func principlesToDomain(ps []ConstitutionPrinciple) []storage.Principle {
	result := make([]storage.Principle, len(ps))
	for i, p := range ps {
		result[i] = storage.Principle{
			ID:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		}
	}
	return result
}

func antipatternsToDomain(aps []ConstitutionAntipattern) []storage.Antipattern {
	result := make([]storage.Antipattern, len(aps))
	for i, a := range aps {
		result[i] = storage.Antipattern{
			Pattern: a.Pattern,
			Why:     a.Why,
			Instead: a.Instead,
		}
	}
	return result
}

func referencesToDomain(refs []ConstitutionReference) []storage.Reference {
	result := make([]storage.Reference, len(refs))
	for i, r := range refs {
		result[i] = storage.Reference{
			Type: r.Type,
			Path: r.Path,
		}
	}
	return result
}

func processToDomain(p *ConstitutionProcess) *storage.ProcessConfig {
	if p == nil {
		return nil
	}
	pc := &storage.ProcessConfig{
		SpecReview: p.SpecReview,
	}
	if p.SecurityReview != nil {
		pc.SecurityReview = &storage.SecurityReviewConfig{When: p.SecurityReview.When}
	}
	if p.Deployment != nil {
		pc.Deployment = &storage.DeploymentConfig{
			Strategy: p.Deployment.Strategy,
			Rollback: p.Deployment.Rollback,
		}
	}
	if p.Documentation != nil {
		pc.Documentation = &storage.DocumentationConfig{
			APIDocs: p.Documentation.APIDocs,
			Runbook: p.Documentation.Runbook,
		}
	}
	return pc
}

// ParseConstitutionConfig parses raw YAML/JSON bytes into a ConstitutionConfig.
// Used by LoadConstitutionYAML (which adds file I/O) and by
// internal/constitution/load (which works on bytes from remote fetches).
// This is an alias for ParseConstitutionYAML for callers that prefer the
// Config-centric naming convention.
func ParseConstitutionConfig(data []byte) (*ConstitutionConfig, error) {
	return ParseConstitutionYAML(data)
}

// ConstitutionConfigFromDomain converts a storage.Constitution domain type to a ConstitutionConfig (YAML).
func ConstitutionConfigFromDomain(c *storage.Constitution) *ConstitutionConfig {
	cfg := &ConstitutionConfig{
		Name:        c.Name,
		Layer:       string(c.Layer),
		Constraints: c.Constraints,
	}

	if c.Tech != nil {
		if c.Tech.Languages != nil {
			cfg.Tech.Languages.Primary = c.Tech.Languages.Primary
			cfg.Tech.Languages.Allowed = c.Tech.Languages.Allowed
			cfg.Tech.Languages.Forbidden = c.Tech.Languages.Forbidden
			cfg.Tech.Languages.ForbiddenReasons = c.Tech.Languages.ForbiddenReasons
		}
		cfg.Tech.Frameworks = c.Tech.Frameworks
		cfg.Tech.Infrastructure = c.Tech.Infrastructure
	}

	for _, p := range c.Principles {
		cfg.Principles = append(cfg.Principles, ConstitutionPrinciple{
			ID:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}

	for _, a := range c.Antipatterns {
		cfg.Antipatterns = append(cfg.Antipatterns, ConstitutionAntipattern{
			Pattern: a.Pattern,
			Why:     a.Why,
			Instead: a.Instead,
		})
	}

	for _, r := range c.References {
		cfg.References = append(cfg.References, ConstitutionReference{
			Type: r.Type,
			Path: r.Path,
		})
	}

	if c.Process != nil {
		cp := &ConstitutionProcess{SpecReview: c.Process.SpecReview}
		if c.Process.SecurityReview != nil {
			cp.SecurityReview = &ConstitutionSecurityReview{When: c.Process.SecurityReview.When}
		}
		if c.Process.Deployment != nil {
			cp.Deployment = &ConstitutionDeployment{
				Strategy: c.Process.Deployment.Strategy,
				Rollback: c.Process.Deployment.Rollback,
			}
		}
		if c.Process.Documentation != nil {
			cp.Documentation = &ConstitutionDocumentation{
				APIDocs: c.Process.Documentation.APIDocs,
				Runbook: c.Process.Documentation.Runbook,
			}
		}
		cfg.Process = cp
	}

	return cfg
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
	if cfg.Storage.Postgres.URL == "" {
		cfg.Storage.Postgres.URL = "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable"
	}
	if cfg.Storage.Docker.ComposeFile == "" {
		cfg.Storage.Docker.ComposeFile = ".specgraph/docker-compose.yaml"
	}
	if cfg.Storage.ConstitutionPath == "" {
		cfg.Storage.ConstitutionPath = ".specgraph/constitution.yaml"
	}
}
