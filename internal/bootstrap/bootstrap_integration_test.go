// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package bootstrap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/bootstrap"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

func TestIntegration_EnsureIdempotentAndResolvable(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx) // EXPORTED cross-package harness
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	res, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created)
	require.NotEmpty(t, res.Token)

	// The minted token resolves to the bootstrap admin.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)
	id, err := resolver.Resolve(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, res.UserID, id.UserID)
	require.Equal(t, auth.Role("admin"), id.Role)

	// Second call is a no-op.
	res2, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res2.Created)
	require.Empty(t, res2.Token)
	require.Equal(t, res.UserID, res2.UserID)
}
