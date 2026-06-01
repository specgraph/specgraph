//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres"
)

// TestAuthStore_NewAuth_RunsMigrations asserts the constructor brings the
// schema up.
func TestAuthStore_NewAuth_RunsMigrations(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	auth, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	defer auth.Close(ctx)

	// Tables exist?
	var exists bool
	row := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_name = 'users')`)
	require.NoError(t, row.Scan(&exists))
	require.True(t, exists, "users table should exist after migrations")
}
