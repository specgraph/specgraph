// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobal_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:7890", cfg.Server.Listen)
	assert.Equal(t, "service", cfg.Server.Mode)
	assert.Equal(t, "memgraph", cfg.Server.Backend)
	assert.Equal(t, "bolt://localhost:7687", cfg.Server.Memgraph.BoltURI)
	assert.True(t, cfg.Server.Docker)
	assert.Equal(t, "http://localhost:7890", cfg.Client.DefaultServer)
	assert.Empty(t, cfg.Client.Routes)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestLoadGlobal_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
server:
  listen: "0.0.0.0:9999"
  mode: manual
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://db:7687"
  docker: false
client:
  default_server: "http://remote:9999"
  routes:
    - project: "org-*"
      server: "https://shared:7890"
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600)) //nolint:gosec // test file in temp dir

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9999", cfg.Server.Listen)
	assert.Equal(t, "manual", cfg.Server.Mode)
	assert.False(t, cfg.Server.Docker)
	assert.Equal(t, "http://remote:9999", cfg.Client.DefaultServer)
	require.Len(t, cfg.Client.Routes, 1)
	assert.Equal(t, "org-*", cfg.Client.Routes[0].Project)
	assert.Equal(t, "https://shared:7890", cfg.Client.Routes[0].Server)
}

func TestResolveServer_RepoOverride(t *testing.T) {
	cfg := &config.GlobalConfig{
		Client: config.ClientConfig{
			DefaultServer: "http://localhost:7890",
		},
	}
	url := cfg.ResolveServer("myproject", "https://team-server:7890")
	assert.Equal(t, "https://team-server:7890", url)
}

func TestResolveServer_RouteMatch(t *testing.T) {
	cfg := &config.GlobalConfig{
		Client: config.ClientConfig{
			DefaultServer: "http://localhost:7890",
			Routes: []config.Route{
				{Project: "org-*", Server: "https://shared:7890"},
			},
		},
	}
	assert.Equal(t, "https://shared:7890", cfg.ResolveServer("org-frontend", ""))
	assert.Equal(t, "http://localhost:7890", cfg.ResolveServer("my-project", ""))
}
