// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobal_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9090", cfg.Server.Listen)
	assert.Equal(t, "service", cfg.Server.Mode)
	assert.Equal(t, "postgres", cfg.Server.Backend)
	assert.Equal(t, "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable", cfg.Server.Postgres.URL)
	assert.True(t, cfg.Server.Docker)
	assert.Equal(t, "http://127.0.0.1:9090", cfg.Client.DefaultServer)
	assert.Empty(t, cfg.Client.Routes)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestLoadGlobal_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	//nolint:gosec // test fixture with dev credentials
	yaml := `
server:
  listen: "0.0.0.0:9999"
  mode: manual
  backend: postgres
  postgres:
    url: "postgres://specgraph:specgraph@db:5432/specgraph?sslmode=disable"
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

func TestLoadGlobalExplicit_ErrorsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	_, err := config.LoadGlobalExplicit(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
	assert.Contains(t, err.Error(), path)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "must not materialize defaults at operator-supplied path")
}

func TestLoadGlobalExplicit_LoadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  mode: manual\n"), 0o600))

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.Equal(t, "manual", cfg.Server.Mode)
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

func TestProbesConfig_Resolved_FillsDefaults(t *testing.T) {
	resolved, err := config.ProbesConfig{}.Resolved()
	require.NoError(t, err)
	assert.Equal(t, config.DefaultProbeInterval, resolved.Interval)
	assert.Equal(t, config.DefaultProbeTimeout, resolved.Timeout)
}

func TestProbesConfig_Resolved_PreservesOverrides(t *testing.T) {
	p := config.ProbesConfig{Interval: 30 * time.Second, Timeout: 4 * time.Second}
	resolved, err := p.Resolved()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, resolved.Interval)
	assert.Equal(t, 4*time.Second, resolved.Timeout)
}

func TestProbesConfig_Resolved(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.ProbesConfig
		wantErr string
	}{
		{name: "zero values resolve to defaults", cfg: config.ProbesConfig{}},
		{name: "positive values pass", cfg: config.ProbesConfig{Interval: 10 * time.Second, Timeout: 3 * time.Second}},
		{name: "timeout equals interval passes", cfg: config.ProbesConfig{Interval: 2 * time.Second, Timeout: 2 * time.Second}},
		{name: "negative interval", cfg: config.ProbesConfig{Interval: -1 * time.Second}, wantErr: "probes.interval must be non-negative"},
		{name: "negative timeout", cfg: config.ProbesConfig{Timeout: -1 * time.Second}, wantErr: "probes.timeout must be non-negative"},
		{name: "timeout exceeds interval", cfg: config.ProbesConfig{Interval: 2 * time.Second, Timeout: 5 * time.Second}, wantErr: "must not exceed probes.interval"},
		{name: "timeout exceeds default interval", cfg: config.ProbesConfig{Timeout: 30 * time.Second}, wantErr: "must not exceed probes.interval"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cfg.Resolved()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLoadGlobal_ProbesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
server:
  probes:
    listen: "0.0.0.0:9091"
    interval: 10s
    timeout: 3s
`
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9091", cfg.Server.Probes.Listen)
	assert.Equal(t, 10*time.Second, cfg.Server.Probes.Interval)
	assert.Equal(t, 3*time.Second, cfg.Server.Probes.Timeout)
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

func TestLoadGlobal_OIDCJITConfig(t *testing.T) {
	yamlBody := []byte(`
auth:
  oidc:
    providers:
      - id: entra
        issuer: https://login.microsoftonline.com/tenant/v2.0
        client_id: app-id
    jit_create:
      enabled: true
      default_role: reader
      rate_limit_per_hour: 200
      email_domain_allowlist: [example.com, other.com]
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, yamlBody, 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.True(t, cfg.Auth.OIDC.JITCreate.Enabled)
	require.Equal(t, "reader", cfg.Auth.OIDC.JITCreate.DefaultRole)
	require.Equal(t, 200, cfg.Auth.OIDC.JITCreate.RateLimitPerHour)
	require.Len(t, cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist, 2)
	require.Len(t, cfg.Auth.OIDC.Providers, 1)
}

func TestLoadGlobal_LegacyOIDCProvidersStillWorks(t *testing.T) {
	yamlBody := []byte(`
auth:
  oidc_providers:
    - id: legacy-entra
      issuer: https://login.microsoftonline.com/old/v2.0
      client_id: app-id
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, yamlBody, 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.Len(t, cfg.Auth.OIDC.Providers, 1, "legacy path should migrate transparently")
	require.Equal(t, "legacy-entra", cfg.Auth.OIDC.Providers[0].ID)
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
  roles: [deployer]
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

func TestLoadGlobal_RolesAndPolicies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
auth:
  roles: [auditor, releaser]
  policies:
    extra_dirs: ["/etc/specgraph/policies"]
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"auditor", "releaser"}, cfg.Auth.Roles)
	assert.Equal(t, []string{"/etc/specgraph/policies"}, cfg.Auth.Policies.ExtraDirs)
}

// TestLoadGlobal_LegacyMapRolesRejected verifies that the OLD map-with-
// permissions roles shape fails to parse under the Cedar list shape. This
// is deliberate: silently dropping a mapping value would strip authorization
// intent. The YAML type mismatch (mapping vs sequence) must surface as an
// error rather than be quietly ignored.
func TestLoadGlobal_LegacyMapRolesRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
auth:
  roles:
    deployer:
      permissions: ["spec:read", "execution:*"]
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	_, err := config.LoadGlobal(path)
	require.Error(t, err, "legacy map-shaped roles must fail to parse, not be silently dropped")
}

func TestLoadGlobal_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_LISTEN", "0.0.0.0:2222")

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:2222", cfg.Server.Listen) // env beats file
}

func TestLoadGlobal_SetFlagBeatsEnvAndFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_LISTEN", "0.0.0.0:2222")

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("listen", "", "")
	require.NoError(t, fs.Parse([]string{"--listen", "0.0.0.0:3333"}))

	cfg, err := config.LoadGlobalExplicit(path, config.WithFlags(fs))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3333", cfg.Server.Listen) // set flag wins
}

func TestLoadGlobal_UnsetFlagDoesNotClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  listen: \"0.0.0.0:1111\"\n"), 0o600))

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("listen", "0.0.0.0:9999", "") // non-empty DEFAULT, not set on cmdline
	require.NoError(t, fs.Parse([]string{}))

	cfg, err := config.LoadGlobalExplicit(path, config.WithFlags(fs))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:1111", cfg.Server.Listen) // file wins; flag default ignored
}

func TestLoadGlobal_PgURLCoercesBackend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  backend: \"\"\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_POSTGRES_URL", "postgres://x/y")

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.Equal(t, "postgres://x/y", cfg.Server.Postgres.URL)
	assert.Equal(t, "postgres", cfg.Server.Backend) // coerced
}

func TestLoadGlobal_SliceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	doc := "auth:\n  oidc:\n    providers:\n      - id: p1\n        client_id: cid\n        claims_mapping:\n          - claim: groups\n            value: admins\n            role: admin\n"
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	require.Len(t, cfg.Auth.OIDC.Providers, 1)
	assert.Equal(t, "p1", cfg.Auth.OIDC.Providers[0].ID)
	assert.Equal(t, "cid", cfg.Auth.OIDC.Providers[0].ClientID)
	require.Len(t, cfg.Auth.OIDC.Providers[0].ClaimsMapping, 1)
	assert.Equal(t, "admin", cfg.Auth.OIDC.Providers[0].ClaimsMapping[0].Role)
}

func TestLoadGlobal_MaterializedFileMatchesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := config.LoadGlobal(path) // missing -> materialize
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:9090", cfg.Server.Listen) // matches globalDefaults()

	reread, err := config.LoadGlobalExplicit(path) // now exists
	require.NoError(t, err)
	assert.Equal(t, "postgres://specgraph:specgraph@localhost:5432/specgraph?sslmode=disable",
		reread.Server.Postgres.URL)
}

// Env values are always strings; this guards the WeaklyTypedInput + duration
// decode-hook coercion that lets non-string scalars be set from the environment.
func TestLoadGlobal_EnvCoercesNonStringScalars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  docker: true\n"), 0o600))
	t.Setenv("SPECGRAPH_SERVER_DOCKER", "false")       // string -> bool
	t.Setenv("SPECGRAPH_SERVER_PROBES_INTERVAL", "7s") // string -> time.Duration

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.False(t, cfg.Server.Docker)
	assert.Equal(t, 7*time.Second, cfg.Server.Probes.Interval)
}
