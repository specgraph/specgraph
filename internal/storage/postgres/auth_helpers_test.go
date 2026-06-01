//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres"
)

// sharedTestPool returns a fresh pgxpool against the testcontainers Postgres
// started in postgres_test.go's TestMain. The pool is closed via t.Cleanup;
// callers do not need to manage lifecycle.
//
// Each call opens a new pool. This is the same pattern clearDatabase uses
// (see postgres_test.go:108). The container is shared; the pools are not.
func sharedTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// authTestSetup constructs an AuthStore against the test container's
// Postgres. It uses sharedTestPool internally and registers AuthStore.Close
// via t.Cleanup. The returned store has the auth migrations applied.
func authTestSetup(t *testing.T) *postgres.AuthStore {
	t.Helper()
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)

	auth, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = auth.Close(ctx) })

	return auth
}

// truncateAuthTables wipes identity tables between tests. FK CASCADE on
// oidc_bindings.user_id and api_keys.user_id handles cleanup of child rows.
func truncateAuthTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)
}
