// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPoolConfig_DefaultsConnectTimeout(t *testing.T) {
	cfg, err := buildPoolConfig("postgres://u:p@example.invalid:5432/db?sslmode=disable")
	require.NoError(t, err)
	assert.Equal(t, defaultConnectTimeout, cfg.ConnConfig.ConnectTimeout,
		"a blackholed dial must fail within a bounded timeout, not block on the OS SYN-retransmit budget (~127s) and starve the probe listeners")
}

func TestBuildPoolConfig_PreservesExplicitConnectTimeout(t *testing.T) {
	cfg, err := buildPoolConfig("postgres://u:p@example.invalid:5432/db?sslmode=disable&connect_timeout=3")
	require.NoError(t, err)
	assert.Equal(t, 3*time.Second, cfg.ConnConfig.ConnectTimeout,
		"an operator-specified connect_timeout must win over the default")
}

func TestBuildPoolConfig_ClampsExplicitZeroToDefault(t *testing.T) {
	// libpq's connect_timeout=0 means "wait indefinitely", which pgconn parses
	// to a zero ConnectTimeout — indistinguishable from unset. Both are clamped
	// to the default: an unbounded dial is the startup hang this guards against.
	cfg, err := buildPoolConfig("postgres://u:p@example.invalid:5432/db?sslmode=disable&connect_timeout=0")
	require.NoError(t, err)
	assert.Equal(t, defaultConnectTimeout, cfg.ConnConfig.ConnectTimeout,
		"an explicit connect_timeout=0 (infinite) is intentionally clamped to avoid reintroducing the unbounded-dial hang")
}

func TestBuildPoolConfig_InvalidConnString(t *testing.T) {
	_, err := buildPoolConfig("://not-a-valid-dsn")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres: parse config")
}
