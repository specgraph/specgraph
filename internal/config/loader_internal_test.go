// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvKeyMapper_MapsUnderscoreKeys(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(structs.Provider(globalDefaults(), "koanf"), nil))
	m := envKeyMapper(k)

	// underscore-bearing key must NOT be mangled to client.default.server
	assert.Equal(t, "client.default_server", m("SPECGRAPH_CLIENT_DEFAULT_SERVER"))
	assert.Equal(t, "server.postgres.url", m("SPECGRAPH_SERVER_POSTGRES_URL"))
	assert.Equal(t, "server.listen", m("SPECGRAPH_SERVER_LISTEN"))
	assert.Equal(t, "", m("SPECGRAPH_UNKNOWN_KEY")) // unknown -> ignored

	// Slice keys are not env-settable; this is the behavior isEnvSettable
	// enforces. SPECGRAPH_AUTH_OIDC_PROVIDERS is also the env form that the
	// deprecated auth.oidc_providers and auth.oidc.providers both collide on.
	assert.Equal(t, "", m("SPECGRAPH_AUTH_ROLES"))
	assert.Equal(t, "", m("SPECGRAPH_AUTH_OIDC_PROVIDERS"))
}

func TestEnvKeyMapper_NoEnvFormCollisions(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(structs.Provider(globalDefaults(), "koanf"), nil))
	m := envKeyMapper(k)
	// Every env-settable (scalar) key must round-trip through its env form.
	// A collision would make one key fail to map back to itself.
	for _, key := range k.Keys() {
		if !isEnvSettable(k.Get(key)) {
			continue
		}
		form := "SPECGRAPH_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		assert.Equal(t, key, m(form), "scalar key %q must round-trip via its env form", key)
	}
}

func TestGlobalDefaults_LogRequestsDefaultsTrue(t *testing.T) {
	assert.True(t, globalDefaults().Log.Requests,
		"access logging must be on by default")
	assert.False(t, globalDefaults().Server.Probes.LogRequests,
		"probe-request logging must be off by default")
}

func TestLoadGlobal_LogRequestsExplicitFalseOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("log:\n  requests: false\n"), 0o600))

	cfg, err := LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.False(t, cfg.Log.Requests, "explicit log.requests:false must override the true default")
}
