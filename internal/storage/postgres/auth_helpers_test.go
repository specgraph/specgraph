// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// sharedTestPool returns a fresh pgxpool against the testcontainers Postgres
// managed by postgrestest.SharedPool. The pool is closed via t.Cleanup;
// callers do not need to manage lifecycle.
//
// Delegates to postgrestest.SharedPool so there is ONE container-start
// implementation, importable cross-package (Task 32).
func sharedTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return postgrestest.SharedPool(t, ctx)
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
