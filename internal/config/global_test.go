// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobal_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:7890", cfg.Server.Listen)
	assert.Equal(t, "service", cfg.Server.Mode)
	assert.Equal(t, "memgraph", cfg.Server.Backend)
	assert.Equal(t, "bolt://localhost:7687", cfg.Server.Memgraph.BoltURI)
	assert.True(t, cfg.Server.Docker)
	assert.Equal(t, "http://127.0.0.1:7890", cfg.Client.DefaultServer)
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
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

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

func TestLoadGlobal_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server: [\ninvalid yaml"), 0o600))

	_, err := config.LoadGlobal(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoadGlobal_ReadOnlyParentDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	parent := t.TempDir()
	// Make parent read-only so creating config.yaml inside it fails.
	require.NoError(t, os.Chmod(parent, 0o555))       //nolint:gosec // intentional for test
	t.Cleanup(func() { _ = os.Chmod(parent, 0o750) }) //nolint:gosec // restore perms for cleanup

	path := filepath.Join(parent, "subdir", "config.yaml")
	_, err := config.LoadGlobal(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write default config")
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

func TestLoadGlobal_AuthConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
auth:
  mode: oidc
  default_role: writer
  api_keys:
    - id: k1
      key: spgr_sk_test
      name: Test
      role: admin
  oidc_providers:
    - id: entra
      issuer: https://login.microsoftonline.com/tenant/v2.0
      client_id: app-id
      audience: api-audience
      claims_mapping:
        - claim: groups
          value: specgraph-admins
          role: admin
  roles:
    deployer:
      permissions: ["spec:read", "execution:*"]
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "oidc", cfg.Auth.Mode)
	assert.Equal(t, "writer", cfg.Auth.DefaultRole)
	require.Len(t, cfg.Auth.OIDCProviders, 1)
	assert.Equal(t, "entra", cfg.Auth.OIDCProviders[0].ID)
	assert.Equal(t, "https://login.microsoftonline.com/tenant/v2.0", cfg.Auth.OIDCProviders[0].Issuer)
	assert.Equal(t, "app-id", cfg.Auth.OIDCProviders[0].ClientID)
	assert.Equal(t, "api-audience", cfg.Auth.OIDCProviders[0].Audience)
	require.Len(t, cfg.Auth.OIDCProviders[0].ClaimsMapping, 1)
	assert.Equal(t, "groups", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Claim)
	assert.Equal(t, "specgraph-admins", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Value)
	assert.Equal(t, "admin", cfg.Auth.OIDCProviders[0].ClaimsMapping[0].Role)
}
