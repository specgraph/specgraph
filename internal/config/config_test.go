// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
server:
  mode: external
  host: 127.0.0.1
  port: 8080
storage:
  backend: memgraph
  memgraph:
    bolt_uri: bolt://db:7687
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "external", cfg.Server.Mode)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)
	assert.Equal(t, "bolt://db:7687", cfg.Storage.Memgraph.BoltURI)
	assert.False(t, cfg.IsRemote())
}

func TestLoadConfig_Remote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
server:
  remote: https://specgraph.example.com
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.True(t, cfg.IsRemote())
	assert.Equal(t, "https://specgraph.example.com", cfg.Server.Remote)
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg := &Config{
		Server: ServerConfig{
			Mode: "docker",
			Host: "127.0.0.1",
			Port: 8080,
		},
		Storage: StorageConfig{
			Backend: "memgraph",
		},
	}

	require.NoError(t, cfg.Write(path))

	// Read it back
	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "docker", loaded.Server.Mode)
	assert.Equal(t, "127.0.0.1", loaded.Server.Host)
	assert.Equal(t, 8080, loaded.Server.Port)
	assert.Equal(t, "memgraph", loaded.Storage.Backend)
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
storage:
  backend: memgraph
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "docker", cfg.Server.Mode)
	assert.Equal(t, "bolt://localhost:7687", cfg.Storage.Memgraph.BoltURI)
	assert.Equal(t, ".specgraph/docker-compose.yaml", cfg.Storage.Docker.ComposeFile)
}

func TestLoadConfig_MemgraphAuthAndTLS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
storage:
  backend: memgraph
  memgraph:
    bolt_uri: bolt://db:7687
    username: admin
    password: secret
    use_tls: true
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "bolt://db:7687", cfg.Storage.Memgraph.BoltURI)
	assert.Equal(t, "admin", cfg.Storage.Memgraph.Username)
	assert.Equal(t, "secret", cfg.Storage.Memgraph.Password)
	assert.True(t, cfg.Storage.Memgraph.UseTLS)
}

func TestLoadConfig_MemgraphDefaultsNoAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
storage:
  backend: memgraph
  memgraph:
    bolt_uri: bolt://localhost:7687
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "", cfg.Storage.Memgraph.Username)
	assert.Equal(t, "", cfg.Storage.Memgraph.Password)
	assert.False(t, cfg.Storage.Memgraph.UseTLS)
}

// --- Constitution YAML tests ---

func TestConstitutionYAML_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	original := &ConstitutionConfig{
		Name:  "my-project",
		Layer: "project",
		Tech: ConstitutionTech{
			Languages: ConstitutionLangs{
				Primary: "go",
				Allowed: []string{"go", "typescript"},
			},
			Frameworks:     map[string]string{"web": "echo"},
			Infrastructure: map[string]string{"db": "memgraph"},
		},
		Principles: []ConstitutionPrinciple{
			{ID: "p1", Statement: "Keep it simple", Rationale: "Reduces bugs"},
			{Statement: "Test everything", Exceptions: "prototypes"},
		},
		Constraints: []string{"no global state"},
		Antipatterns: []ConstitutionAntipattern{
			{Pattern: "god object", Why: "hard to test", Instead: "split responsibilities"},
		},
		References: []ConstitutionReference{
			{Type: "adr", Path: "docs/adr/001.md"},
		},
	}

	require.NoError(t, WriteConstitutionYAML(path, original))

	loaded, err := LoadConstitutionYAML(path)
	require.NoError(t, err)

	assert.Equal(t, original.Name, loaded.Name)
	assert.Equal(t, original.Layer, loaded.Layer)
	assert.Equal(t, original.Tech.Languages.Primary, loaded.Tech.Languages.Primary)
	assert.Equal(t, original.Tech.Languages.Allowed, loaded.Tech.Languages.Allowed)
	assert.Equal(t, original.Tech.Frameworks, loaded.Tech.Frameworks)
	assert.Equal(t, original.Tech.Infrastructure, loaded.Tech.Infrastructure)
	assert.Equal(t, original.Principles, loaded.Principles)
	assert.Equal(t, original.Constraints, loaded.Constraints)
	assert.Equal(t, original.Antipatterns, loaded.Antipatterns)
	assert.Equal(t, original.References, loaded.References)
}

func TestConstitutionYAML_WriteThenLoad_WithStructuredPrinciples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	original := &ConstitutionConfig{
		Name:  "structured",
		Layer: "org",
		Principles: []ConstitutionPrinciple{
			{ID: "p1", Statement: "Prefer composition", Rationale: "More flexible", Exceptions: "value types"},
		},
	}

	require.NoError(t, WriteConstitutionYAML(path, original))

	loaded, err := LoadConstitutionYAML(path)
	require.NoError(t, err)

	require.Len(t, loaded.Principles, 1)
	assert.Equal(t, "p1", loaded.Principles[0].ID)
	assert.Equal(t, "Prefer composition", loaded.Principles[0].Statement)
	assert.Equal(t, "More flexible", loaded.Principles[0].Rationale)
	assert.Equal(t, "value types", loaded.Principles[0].Exceptions)
}

func TestConstitutionYAML_LoadStringPrinciples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	rawYAML := `name: legacy
layer: project
principles:
  - "Keep it simple"
  - "Test everything"
`
	require.NoError(t, os.WriteFile(path, []byte(rawYAML), 0o600))

	loaded, err := LoadConstitutionYAML(path)
	require.NoError(t, err)

	require.Len(t, loaded.Principles, 2)
	assert.Equal(t, "Keep it simple", loaded.Principles[0].Statement)
	assert.Equal(t, "Test everything", loaded.Principles[1].Statement)
}

func TestConstitutionYAML_InvalidLayer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	rawYAML := `name: bad
layer: invalid_layer
`
	require.NoError(t, os.WriteFile(path, []byte(rawYAML), 0o600))

	_, err := LoadConstitutionYAML(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown constitution layer")
}

func TestConstitutionYAML_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	require.NoError(t, os.WriteFile(path, []byte(":\tinvalid:\t[yaml"), 0o600))

	_, err := LoadConstitutionYAML(path)
	require.Error(t, err)
}

func TestConstitutionYAML_WriteCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "constitution.yaml")

	c := &ConstitutionConfig{Name: "test", Layer: "user"}
	require.NoError(t, WriteConstitutionYAML(path, c))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}

func TestConstitutionConfig_ToDomainRoundTrip(t *testing.T) {
	original := &ConstitutionConfig{
		Name:  "round-trip-test",
		Layer: "project",
		Tech: ConstitutionTech{
			Languages: ConstitutionLangs{
				Primary:          "go",
				Allowed:          []string{"go", "typescript"},
				Forbidden:        []string{"php"},
				ForbiddenReasons: map[string]string{"php": "not in stack"},
			},
			Frameworks:     map[string]string{"api": "connectrpc"},
			Infrastructure: map[string]string{"db": "memgraph"},
		},
		Principles: []ConstitutionPrinciple{
			{ID: "p1", Statement: "Keep it simple", Rationale: "Reduces bugs", Exceptions: "protos"},
			{ID: "p2", Statement: "Test everything"},
		},
		Constraints: []string{"no global state", "no panics in library code"},
		Antipatterns: []ConstitutionAntipattern{
			{Pattern: "god object", Why: "hard to test", Instead: "split responsibilities"},
		},
		References: []ConstitutionReference{
			{Type: "adr", Path: "docs/adr/001.md"},
			{Type: "url", Path: "https://example.com"},
		},
	}

	dom := original.ToDomain()
	require.NotNil(t, dom)

	roundTripped := ConstitutionConfigFromDomain(dom)
	require.NotNil(t, roundTripped)

	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.Layer, roundTripped.Layer)
	assert.Equal(t, original.Constraints, roundTripped.Constraints)

	assert.Equal(t, original.Tech.Languages.Primary, roundTripped.Tech.Languages.Primary)
	assert.Equal(t, original.Tech.Languages.Allowed, roundTripped.Tech.Languages.Allowed)
	assert.Equal(t, original.Tech.Languages.Forbidden, roundTripped.Tech.Languages.Forbidden)
	assert.Equal(t, original.Tech.Languages.ForbiddenReasons, roundTripped.Tech.Languages.ForbiddenReasons)
	assert.Equal(t, original.Tech.Frameworks, roundTripped.Tech.Frameworks)
	assert.Equal(t, original.Tech.Infrastructure, roundTripped.Tech.Infrastructure)

	require.Len(t, roundTripped.Principles, len(original.Principles))
	assert.Equal(t, original.Principles[0].ID, roundTripped.Principles[0].ID)
	assert.Equal(t, original.Principles[0].Statement, roundTripped.Principles[0].Statement)
	assert.Equal(t, original.Principles[0].Rationale, roundTripped.Principles[0].Rationale)
	assert.Equal(t, original.Principles[0].Exceptions, roundTripped.Principles[0].Exceptions)
	assert.Equal(t, original.Principles[1].Statement, roundTripped.Principles[1].Statement)

	require.Len(t, roundTripped.Antipatterns, len(original.Antipatterns))
	assert.Equal(t, original.Antipatterns[0].Pattern, roundTripped.Antipatterns[0].Pattern)
	assert.Equal(t, original.Antipatterns[0].Why, roundTripped.Antipatterns[0].Why)
	assert.Equal(t, original.Antipatterns[0].Instead, roundTripped.Antipatterns[0].Instead)

	require.Len(t, roundTripped.References, len(original.References))
	assert.Equal(t, original.References[0].Type, roundTripped.References[0].Type)
	assert.Equal(t, original.References[0].Path, roundTripped.References[0].Path)
	assert.Equal(t, original.References[1].Type, roundTripped.References[1].Type)
	assert.Equal(t, original.References[1].Path, roundTripped.References[1].Path)
}
