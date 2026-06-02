// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

func TestIntegration_APIKeyResolve(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	// Truncate identity tables, then seed a user + key via direct SQL.
	// Using TRUNCATE users CASCADE (not RESTART IDENTITY — id is UUID, no sequence).
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	user := activeUser("aaaa0000-0000-0000-0000-000000000001", "writer", storage.KindHuman)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, kind, display_name, role)
	                         VALUES ($1::uuid, $2, $3, $4)`,
		user.ID, string(storage.KindHuman), user.DisplayName, user.Role)
	require.NoError(t, err)

	// Insert the key directly with stubPHCHash so the resolver can verify
	// a token built by stubAPIKeyToken("abc12345").
	_, err = pool.Exec(ctx, `INSERT INTO api_keys (id, user_id, prefix, phc_hash)
	                         VALUES ('bbbb0000-0000-0000-0000-000000000002'::uuid,
	                                 $1::uuid, 'abc12345', $2)`,
		user.ID, stubPHCHash)
	require.NoError(t, err)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   authStore,
		Tracker: tracker,
	})
	require.NoError(t, err)

	id, err := resolver.Resolve(ctx, stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, user.ID, id.UserID)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, auth.Role("writer"), id.EffectiveRole)
	require.Equal(t, "apikey", id.Source)
}

func TestIntegration_Lifecycle(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	// Seed a bootstrap admin via the AuthStore's own method.
	admin, err := authStore.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true,
	}, nil)
	require.NoError(t, err)

	// Mint a key for admin. stubPHCHash is the verifiable hash for stubPHCSecret.
	adminKey, err := authStore.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  admin.ID,
		PHCHash: stubPHCHash,
	})
	require.NoError(t, err)

	// Build resolver.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   authStore,
		Tracker: tracker,
	})
	require.NoError(t, err)

	// Bootstrap admin authenticates successfully.
	id, err := resolver.Resolve(ctx, stubAPIKeyToken(adminKey.Prefix))
	require.NoError(t, err)
	require.Equal(t, admin.ID, id.UserID)

	// Revoke the key.
	require.NoError(t, authStore.RevokeAPIKey(ctx, adminKey.ID))

	// Bootstrap can no longer authenticate after revocation.
	_, err = resolver.Resolve(ctx, stubAPIKeyToken(adminKey.Prefix))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
