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
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

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
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.True(t, cfg.IsRemote())
	assert.Equal(t, "https://specgraph.example.com", cfg.Server.Remote)
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "specgraph.yaml")

	yaml := `
storage:
  backend: memgraph
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "docker", cfg.Server.Mode)
	assert.Equal(t, "bolt://localhost:7687", cfg.Storage.Memgraph.BoltURI)
	assert.Equal(t, ".specgraph/docker-compose.yaml", cfg.Storage.Docker.ComposeFile)
}
