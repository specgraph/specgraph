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

// TestIntegration_E2ESeedCredential verifies the exact user + api_key INSERT that
// e2e/ui/seed.sql performs. If this test passes, the seed SQL will produce a
// credential that the pgIdentityStore can resolve to role admin.
//
// Credential chosen:
//
//	token:  spgr_sk_e2eadmin_e2esecret32charsfixedpaddingaaa0
//	prefix: e2eadmin  (8 chars)
//	secret: e2esecret32charsfixedpaddingaaa0  (32 chars)
//	phc:    $argon2id$v=19$m=19456,t=2,p=1$ZTJlc2FsdGUyZXNhbHQxNg$Zc9Glm0pc9ozY/IU2gdEFm+7T9DLuvBVgsvMeBbVOVw
func TestIntegration_E2ESeedCredential(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	// Truncate and re-seed exactly as seed.sql does (idempotent ON CONFLICT skipped
	// here since TRUNCATE gives us a clean slate for isolation).
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	const (
		e2eUserID  = "e2e00000-0000-0000-0000-000000000001"
		e2eKeyID   = "e2e00000-0000-0000-0000-000000000002"
		e2ePrefix  = "e2eadmin"
		e2ePHCHash = "$argon2id$v=19$m=19456,t=2,p=1$ZTJlc2FsdGUyZXNhbHQxNg$Zc9Glm0pc9ozY/IU2gdEFm+7T9DLuvBVgsvMeBbVOVw"
		e2eToken   = "spgr_sk_e2eadmin_e2esecret32charsfixedpaddingaaa0"
	)

	// Mirror the exact SQL from e2e/ui/seed.sql.
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, kind, display_name, role)
		 VALUES ($1::uuid, 'human', 'E2E Admin', 'admin')`,
		e2eUserID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, prefix, phc_hash, role_downgrade, label)
		 VALUES ($1::uuid, $2::uuid, $3, $4, '', 'e2e')`,
		e2eKeyID, e2eUserID, e2ePrefix, e2ePHCHash)
	require.NoError(t, err)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   authStore,
		Tracker: tracker,
	})
	require.NoError(t, err)

	id, err := resolver.Resolve(ctx, e2eToken)
	require.NoError(t, err)
	require.Equal(t, e2eUserID, id.UserID)
	require.Equal(t, auth.RoleAdmin, id.Role)
	require.Equal(t, auth.RoleAdmin, id.EffectiveRole)
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
