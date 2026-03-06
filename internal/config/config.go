// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package config handles loading and persisting SpecGraph configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"gopkg.in/yaml.v3"
)

// Config holds the full SpecGraph configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
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
	Backend          string         `yaml:"backend"` // memgraph | postgres
	Memgraph         MemgraphConfig `yaml:"memgraph"`
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

// ConstitutionConfig represents a constitution YAML document.
type ConstitutionConfig struct {
	Name         string                    `yaml:"name"`
	Layer        string                    `yaml:"layer"`
	Tech         ConstitutionTech          `yaml:"tech,omitempty"`
	Principles   []ConstitutionPrinciple   `yaml:"principles,omitempty"`
	Constraints  []string                  `yaml:"constraints,omitempty"`
	Antipatterns []ConstitutionAntipattern `yaml:"antipatterns,omitempty"`
	References   []ConstitutionReference   `yaml:"references,omitempty"`
}

// ConstitutionTech holds technology stack configuration.
type ConstitutionTech struct {
	Languages      ConstitutionLangs `yaml:"languages,omitempty"`
	Frameworks     map[string]string `yaml:"frameworks,omitempty"`
	Infrastructure map[string]string `yaml:"infrastructure,omitempty"`
}

// ConstitutionLangs holds language configuration.
type ConstitutionLangs struct {
	Primary          string            `yaml:"primary,omitempty"`
	Allowed          []string          `yaml:"allowed,omitempty"`
	Forbidden        []string          `yaml:"forbidden,omitempty"`
	ForbiddenReasons map[string]string `yaml:"forbidden_reasons,omitempty"`
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

// referenceTypeMap maps YAML reference type strings to the proto enum.
var referenceTypeMap = map[string]specv1.ReferenceType{
	"adr":  specv1.ReferenceType_REFERENCE_TYPE_ADR,
	"spec": specv1.ReferenceType_REFERENCE_TYPE_SPEC,
	"doc":  specv1.ReferenceType_REFERENCE_TYPE_DOC,
	"url":  specv1.ReferenceType_REFERENCE_TYPE_URL,
}

// referenceTypeToString maps proto reference type enum values to YAML strings.
var referenceTypeToString = map[specv1.ReferenceType]string{
	specv1.ReferenceType_REFERENCE_TYPE_ADR:  "adr",
	specv1.ReferenceType_REFERENCE_TYPE_SPEC: "spec",
	specv1.ReferenceType_REFERENCE_TYPE_DOC:  "doc",
	specv1.ReferenceType_REFERENCE_TYPE_URL:  "url",
}

// ReferenceTypeFromString maps a YAML reference type string to the proto enum.
func ReferenceTypeFromString(s string) specv1.ReferenceType {
	if v, ok := referenceTypeMap[strings.ToLower(s)]; ok {
		return v
	}
	return specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED
}

// ToProto converts a ConstitutionConfig (YAML) to a specv1.Constitution proto message.
func (c *ConstitutionConfig) ToProto() *specv1.Constitution {
	layerKey := "CONSTITUTION_LAYER_" + strings.ToUpper(c.Layer)
	layerVal, ok := specv1.ConstitutionLayer_value[layerKey]
	if !ok {
		layerVal = int32(specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED)
	}

	principles := make([]*specv1.Principle, 0, len(c.Principles))
	for _, p := range c.Principles {
		principles = append(principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}

	antipatterns := make([]*specv1.Antipattern, 0, len(c.Antipatterns))
	for _, a := range c.Antipatterns {
		antipatterns = append(antipatterns, &specv1.Antipattern{
			Pattern: a.Pattern,
			Why:     a.Why,
			Instead: a.Instead,
		})
	}

	references := make([]*specv1.Reference, 0, len(c.References))
	for _, r := range c.References {
		references = append(references, &specv1.Reference{
			ReferenceType: ReferenceTypeFromString(r.Type),
			Path:          r.Path,
		})
	}

	return &specv1.Constitution{
		Name:         c.Name,
		Layer:        specv1.ConstitutionLayer(layerVal),
		Principles:   principles,
		Constraints:  c.Constraints,
		Antipatterns: antipatterns,
		References:   references,
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary:          c.Tech.Languages.Primary,
				Allowed:          c.Tech.Languages.Allowed,
				Forbidden:        c.Tech.Languages.Forbidden,
				ForbiddenReasons: c.Tech.Languages.ForbiddenReasons,
			},
			Frameworks:     c.Tech.Frameworks,
			Infrastructure: c.Tech.Infrastructure,
		},
	}
}

// ConstitutionConfigFromProto converts a specv1.Constitution proto message to a ConstitutionConfig (YAML).
func ConstitutionConfigFromProto(pb *specv1.Constitution) *ConstitutionConfig {
	c := &ConstitutionConfig{
		Name:        pb.GetName(),
		Constraints: pb.GetConstraints(),
	}

	layer := pb.GetLayer()
	if layer != specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		c.Layer = strings.ToLower(strings.TrimPrefix(layer.String(), "CONSTITUTION_LAYER_"))
	}

	if tech := pb.GetTech(); tech != nil {
		if langs := tech.GetLanguages(); langs != nil {
			c.Tech.Languages.Primary = langs.GetPrimary()
			c.Tech.Languages.Allowed = langs.GetAllowed()
			c.Tech.Languages.Forbidden = langs.GetForbidden()
			c.Tech.Languages.ForbiddenReasons = langs.GetForbiddenReasons()
		}
		c.Tech.Frameworks = tech.GetFrameworks()
		c.Tech.Infrastructure = tech.GetInfrastructure()
	}

	for _, p := range pb.GetPrinciples() {
		c.Principles = append(c.Principles, ConstitutionPrinciple{
			ID:         p.GetId(),
			Statement:  p.GetStatement(),
			Rationale:  p.GetRationale(),
			Exceptions: p.GetExceptions(),
		})
	}

	for _, a := range pb.GetAntipatterns() {
		c.Antipatterns = append(c.Antipatterns, ConstitutionAntipattern{
			Pattern: a.GetPattern(),
			Why:     a.GetWhy(),
			Instead: a.GetInstead(),
		})
	}

	for _, r := range pb.GetReferences() {
		refType := "unspecified"
		if s, ok := referenceTypeToString[r.GetReferenceType()]; ok {
			refType = s
		}
		c.References = append(c.References, ConstitutionReference{
			Type: refType,
			Path: r.GetPath(),
		})
	}

	return c
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
	if cfg.Storage.ConstitutionPath == "" {
		cfg.Storage.ConstitutionPath = ".specgraph/constitution.yaml"
	}
}
