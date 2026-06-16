// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
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

func TestAuthStore_LookupAPIKeyByPrefix(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Seed: a Human + one APIKey via direct SQL (CRUD methods come later).
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, bootstrap)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'reader', false)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, prefix, phc_hash, label)
		VALUES ('00000000-0000-0000-0000-000000000002'::uuid,
		        '00000000-0000-0000-0000-000000000001'::uuid,
		        'abc12345', '$argon2id$v=19$m=65536,t=1,p=4$stub-salt-padded-to-32chars', 'test-key')`)
	require.NoError(t, err)

	key, err := auth.LookupAPIKeyByPrefix(ctx, "abc12345")
	require.NoError(t, err)
	require.Equal(t, "abc12345", key.Prefix)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", key.UserID)
	require.Equal(t, "test-key", key.Label)

	// Miss returns ErrAPIKeyNotFound.
	_, err = auth.LookupAPIKeyByPrefix(ctx, "no-such-prefix")
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
}

func TestAuthStore_LookupOIDCBinding(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'reader')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO oidc_bindings (id, user_id, issuer, subject)
		VALUES ('00000000-0000-0000-0000-000000000003'::uuid,
		        '00000000-0000-0000-0000-000000000001'::uuid,
		        'https://login.microsoftonline.com/tenant/v2.0',
		        'sub-12345')`)
	require.NoError(t, err)

	b, err := auth.LookupOIDCBinding(ctx, "https://login.microsoftonline.com/tenant/v2.0", "sub-12345")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", b.UserID)
	require.Equal(t, "https://login.microsoftonline.com/tenant/v2.0", b.Issuer)
	require.Equal(t, "sub-12345", b.Subject)

	_, err = auth.LookupOIDCBinding(ctx, "https://github.com", "sub-12345")
	require.ErrorIs(t, err, storage.ErrOIDCBindingNotFound)
}

func TestAuthStore_GetUserByID(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, email, role, bootstrap, created_at, deleted_at)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'alice@example.com', 'reader',
		        false, $1, NULL),
		       ('00000000-0000-0000-0000-000000000002'::uuid,
		        'human', 'bob', '', 'writer', false, $1, $2)`,
		now, now.Add(time.Hour))
	require.NoError(t, err)

	// Active user.
	u, err := auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	require.Equal(t, "alice", u.DisplayName)
	require.True(t, u.IsActive())

	// Soft-deleted user — still returned (caller gates).
	u, err = auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)
	require.False(t, u.IsActive())
	require.NotNil(t, u.DeletedAt)

	// Miss.
	_, err = auth.GetUserByID(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// ServiceAccount round-trip: owner_user_id is populated, scan handles
	// the nullable-FK column via coalesce. Seed via direct SQL (Task 14
	// implements CreateServiceAccount; this test verifies the read path
	// independently).
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, owner_user_id)
		VALUES ('00000000-0000-0000-0000-000000000003'::uuid,
		        'service_account', 'ci-bot', 'writer',
		        '00000000-0000-0000-0000-000000000001'::uuid)`)
	require.NoError(t, err)
	sa, err := auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000003")
	require.NoError(t, err)
	require.True(t, sa.IsServiceAccount())
	require.Equal(t, "00000000-0000-0000-0000-000000000001", sa.OwnerUserID)
}

func TestAuthStore_GetBootstrap(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// No bootstrap yet.
	_, err := auth.GetBootstrap(ctx)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Insert active bootstrap.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, bootstrap)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'admin', 'admin', true)`)
	require.NoError(t, err)

	u, err := auth.GetBootstrap(ctx)
	require.NoError(t, err)
	require.True(t, u.Bootstrap)
	require.Equal(t, "admin", u.DisplayName)
}

func TestAuthStore_CreateHuman(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u := &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "alice",
		Email:       "alice@example.com",
		Role:        "reader",
	}
	created, err := auth.CreateHuman(ctx, u, nil)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "alice", created.DisplayName)
	require.True(t, created.IsHuman())
	require.True(t, created.IsActive())

	// With binding atomically.
	u2 := &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "reader"}
	b := &storage.OIDCBinding{
		Issuer: "https://login.microsoftonline.com/t/v2.0", Subject: "sub-bob",
		EmailAtBind: "bob@example.com",
	}
	created2, err := auth.CreateHuman(ctx, u2, b)
	require.NoError(t, err)

	// Verify the binding via direct SQL — ListOIDCBindings is implemented
	// later in Task 25, so we don't depend on it here.
	var bindingCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oidc_bindings
		WHERE user_id = $1::uuid AND subject = 'sub-bob'`, created2.ID).
		Scan(&bindingCount))
	require.Equal(t, 1, bindingCount)

	// Bootstrap dedup: first bootstrap insert succeeds.
	boot := &storage.User{Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true}
	_, err = auth.CreateHuman(ctx, boot, nil)
	require.NoError(t, err)
	// Second bootstrap insert fails.
	boot2 := &storage.User{Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true}
	_, err = auth.CreateHuman(ctx, boot2, nil)
	require.ErrorIs(t, err, storage.ErrBootstrapExists)
}

func TestAuthStore_CreateServiceAccount(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Owner first.
	owner, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "owner", Role: "admin",
	}, nil)
	require.NoError(t, err)

	sa, err := auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "ci-bot",
		Role: "writer", OwnerUserID: owner.ID,
	})
	require.NoError(t, err)
	require.True(t, sa.IsServiceAccount())
	require.Equal(t, owner.ID, sa.OwnerUserID)

	// Missing owner rejected.
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "no-owner", Role: "writer",
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Soft-deleted human owner must be rejected with ErrUserNotFound.
	deletedOwner, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "deleted-owner", Role: "admin",
	}, nil)
	require.NoError(t, err)
	require.NoError(t, auth.SoftDeleteUser(ctx, deletedOwner.ID))
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "sa-for-deleted", Role: "writer",
		OwnerUserID: deletedOwner.ID,
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound, "soft-deleted human owner must be rejected")

	// Service-account owner must be rejected with ErrUserNotFound (only humans may own).
	saOwner, err := auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "sa-owner-attempt", Role: "writer",
		OwnerUserID: owner.ID,
	})
	require.NoError(t, err)
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "sa-of-sa", Role: "writer",
		OwnerUserID: saOwner.ID,
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound, "service-account owner must be rejected — only active humans may own")
}

func TestAuthStore_UpdateUserRole(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, nil)
	require.NoError(t, err)

	err = auth.UpdateUserRole(ctx, u.ID, "writer")
	require.NoError(t, err)

	reloaded, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "writer", reloaded.Role)

	// Soft-deleted user is not updateable. Set deleted_at via direct SQL
	// rather than calling SoftDeleteUser (Task 16) to avoid a forward
	// reference; soft-delete cascade semantics are exercised in Task 16's
	// own test.
	_, err = pool.Exec(ctx, `UPDATE users SET deleted_at = now() WHERE id = $1::uuid`, u.ID)
	require.NoError(t, err)
	err = auth.UpdateUserRole(ctx, u.ID, "admin")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Nonexistent.
	err = auth.UpdateUserRole(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa", "reader")
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}

func TestAuthStore_UpdateUserOnLogin(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "old-sub", Email: "old@x.io", Role: "reader",
	}, nil)
	require.NoError(t, err)

	// Happy path: all three columns update.
	require.NoError(t, auth.UpdateUserOnLogin(ctx, u.ID, "Ada", "ada@x.io", "admin"))
	got, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "Ada", got.DisplayName)
	require.Equal(t, "ada@x.io", got.Email)
	require.Equal(t, "admin", got.Role)

	// Unknown user -> ErrUserNotFound.
	err = auth.UpdateUserOnLogin(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa", "x", "x@x.io", "reader")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Soft-deleted user -> ErrUserNotFound (active-row guard).
	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))
	err = auth.UpdateUserOnLogin(ctx, u.ID, "y", "y@x.io", "writer")
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}

func TestAuthStore_SoftDeleteUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, &storage.OIDCBinding{Issuer: "iss1", Subject: "sub1"})
	require.NoError(t, err)

	// Seed two API keys directly (phc_hash >= 32 chars per schema CHECK).
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash, label)
		VALUES ($1::uuid, 'pre00001', '$argon2id$v=19$m=65536,t=1,p=4$stub-salt-padded-to-32chars', 'k1'),
		       ($1::uuid, 'pre00002', '$argon2id$v=19$m=65536,t=1,p=4$stub-salt-padded-to-32chars', 'k2')`, u.ID)
	require.NoError(t, err)

	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))

	// User deleted_at set.
	reloaded, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, reloaded.IsActive())

	// Both keys revoked in the same tx (same revoked_at timestamp).
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM api_keys
		WHERE user_id = $1::uuid AND revoked_at IS NOT NULL`, u.ID).Scan(&count))
	require.Equal(t, 2, count)

	// Bindings rows remain (they're history).
	var bindingCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oidc_bindings WHERE user_id = $1::uuid`, u.ID).Scan(&bindingCount))
	require.Equal(t, 1, bindingCount)

	// Idempotent: re-deleting is a no-op.
	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))

	// Unknown/nonexistent userID is also a no-op (returns nil, NOT
	// ErrUserNotFound) — intentional idempotent-delete semantics, distinct
	// from UpdateUserRole.
	require.NoError(t, auth.SoftDeleteUser(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa"))
}

func TestAuthStore_PurgeUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, &storage.OIDCBinding{Issuer: "iss1", Subject: "sub1"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash, label)
		VALUES ($1::uuid, 'pre00003', '$argon2id$v=19$m=65536,t=1,p=4$stub-salt-padded-to-32chars', 'k')`, u.ID)
	require.NoError(t, err)

	require.NoError(t, auth.PurgeUser(ctx, u.ID))

	// User gone.
	_, err = auth.GetUserByID(ctx, u.ID)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Cascaded keys + bindings gone.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id = $1::uuid`, u.ID).Scan(&n))
	require.Equal(t, 0, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM oidc_bindings WHERE user_id = $1::uuid`, u.ID).Scan(&n))
	require.Equal(t, 0, n)

	// Idempotent.
	require.NoError(t, auth.PurgeUser(ctx, u.ID))
}

func TestAuthStore_ListUsers(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Three humans, one service account.
	owner, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h1", Role: "admin"}, nil)
	_, _ = auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h2", Role: "reader"}, nil)
	deleted, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h3", Role: "writer"}, nil)
	require.NoError(t, auth.SoftDeleteUser(ctx, deleted.ID))
	_, _ = auth.CreateServiceAccount(ctx, &storage.User{Kind: storage.KindServiceAccount, DisplayName: "sa1", Role: "writer", OwnerUserID: owner.ID})

	all, err := auth.ListUsers(ctx, storage.ListUsersFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3) // excludes deleted by default

	withDeleted, err := auth.ListUsers(ctx, storage.ListUsersFilter{IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, withDeleted, 4)

	humansOnly, err := auth.ListUsers(ctx, storage.ListUsersFilter{Kind: storage.KindHuman})
	require.NoError(t, err)
	require.Len(t, humansOnly, 2) // h3 deleted, sa1 excluded

	readers, err := auth.ListUsers(ctx, storage.ListUsersFilter{Role: "reader"})
	require.NoError(t, err)
	require.Len(t, readers, 1)
	require.Equal(t, "h2", readers[0].DisplayName)
}

func TestAuthStore_CreateAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)

	k, err := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-stub-padded-to-meet-min-length-ok", Label: "first key",
	})
	require.NoError(t, err)
	require.Len(t, k.Prefix, 8)
	require.NotEmpty(t, k.ID)
	require.Equal(t, "first key", k.Label)

	// Soft-deleted user must be rejected with ErrUserNotFound.
	deleted, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "gone", Role: "reader",
	}, nil)
	require.NoError(t, err)
	require.NoError(t, auth.SoftDeleteUser(ctx, deleted.ID))
	_, err = auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  deleted.ID,
		PHCHash: "phc-stub-padded-to-meet-min-length-ok",
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound, "CreateAPIKey for soft-deleted user must return ErrUserNotFound")
}

func TestAuthStore_CreateAPIKey_CollisionRetry(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Build a dedicated AuthStore with a stubbed prefix generator.
	calls := 0
	gen := func() (string, error) {
		calls++
		if calls <= 2 {
			return "collide1", nil
		}
		return "newpre23", nil
	}
	auth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(gen))
	require.NoError(t, err)
	t.Cleanup(func() { _ = auth.Close(ctx) })

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)

	// Seed a key with the prefix the generator will collide against.
	_, err = pool.Exec(ctx, `INSERT INTO api_keys (user_id, prefix, phc_hash)
	                         VALUES ($1::uuid, 'collide1', 'phc-seed-string-padding-to-meet-length-check')`, u.ID)
	require.NoError(t, err)

	k, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-string-padding-to-meet-length-check"})
	require.NoError(t, err)
	require.Equal(t, "newpre23", k.Prefix)
	require.GreaterOrEqual(t, calls, 3)

	// Exhaustion: a generator that always returns the same colliding prefix.
	authExhaust, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(
		func() (string, error) { return "collide1", nil }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = authExhaust.Close(ctx) })
	_, err = authExhaust.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-string-padding-to-meet-length-check"})
	require.ErrorIs(t, err, storage.ErrAPIKeyPrefixExists)
}

func TestAuthStore_RevokeAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	k, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-padded-to-meet-min-length-ok"})

	require.NoError(t, auth.RevokeAPIKey(ctx, k.ID))

	reloaded, err := auth.LookupAPIKeyByPrefix(ctx, k.Prefix)
	require.NoError(t, err)
	require.NotNil(t, reloaded.RevokedAt)
	require.False(t, reloaded.IsActive(time.Now()))

	// Idempotent on already-revoked.
	require.NoError(t, auth.RevokeAPIKey(ctx, k.ID))

	// Nonexistent ID is also a no-op (no error).
	require.NoError(t, auth.RevokeAPIKey(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa"))
}

func TestAuthStore_RotateAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	// Create a second user to supply as a "bogus" caller-side owner — the
	// rotate must NOT use this; it must inherit the old key's owner.
	other, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "impostor", Role: "reader",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID:        u.ID,
		PHCHash:       "phc-old-padded-to-meet-min-length-ok-00",
		Label:         "ci-bot",
		RoleDowngrade: "reader",
	})

	// Pass a newKey with completely different metadata to prove the impl
	// ignores everything except PHCHash.
	newKey, err := auth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID:        other.ID, // bogus — must be overridden
		PHCHash:       "phc-new-padded-to-meet-min-length-ok-00",
		Label:         "WRONG-LABEL", // bogus — must be overridden
		RoleDowngrade: "admin",       // bogus — must be overridden
	})
	require.NoError(t, err)
	require.NotEqual(t, old.Prefix, newKey.Prefix)

	// Owner, label, and role_downgrade must be inherited from the OLD key.
	require.Equal(t, u.ID, newKey.UserID, "UserID must be inherited from old key, not caller's newKey.UserID")
	require.Equal(t, "ci-bot", newKey.Label, "Label must be inherited from old key")
	require.Equal(t, "reader", newKey.RoleDowngrade, "RoleDowngrade must be inherited from old key")

	// Verify the persisted row also has the correct owner and metadata.
	newReload, err := auth.LookupAPIKeyByPrefix(ctx, newKey.Prefix)
	require.NoError(t, err)
	require.Equal(t, u.ID, newReload.UserID)
	require.Equal(t, "ci-bot", newReload.Label)
	require.Equal(t, "reader", newReload.RoleDowngrade)
	require.Nil(t, newReload.RevokedAt)

	// Old key is revoked.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NotNil(t, oldReload.RevokedAt)
}

// TestAuthStore_RotateAPIKey_ExpiresAt verifies the fail-safe expiry semantics
// on rotation: a non-nil newKey.ExpiresAt overrides; a nil one inherits the old
// key's expiry (never silently clears it). Owner/label/role_downgrade remain
// inherited regardless.
func TestAuthStore_RotateAPIKey_ExpiresAt(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)

	oldExpiry := time.Now().Add(24 * time.Hour).UTC()
	newExpiry := time.Now().Add(720 * time.Hour).UTC()

	t.Run("non-nil ExpiresAt overrides the old expiry", func(t *testing.T) {
		old, err := auth.CreateAPIKey(ctx, &storage.APIKey{
			UserID: u.ID, PHCHash: "phc-old-padded-to-meet-min-length-ok-00",
			Label: "ci-bot", ExpiresAt: &oldExpiry,
		})
		require.NoError(t, err)

		rotated, err := auth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
			PHCHash: "phc-new-padded-to-meet-min-length-ok-01", ExpiresAt: &newExpiry,
		})
		require.NoError(t, err)
		require.NotNil(t, rotated.ExpiresAt)
		require.WithinDuration(t, newExpiry, *rotated.ExpiresAt, time.Second,
			"a provided ExpiresAt must override the old key's expiry")
		require.Equal(t, "ci-bot", rotated.Label, "label still inherited")
	})

	t.Run("nil ExpiresAt inherits the old expiry", func(t *testing.T) {
		old, err := auth.CreateAPIKey(ctx, &storage.APIKey{
			UserID: u.ID, PHCHash: "phc-old-padded-to-meet-min-length-ok-02",
			ExpiresAt: &oldExpiry,
		})
		require.NoError(t, err)

		rotated, err := auth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
			PHCHash: "phc-new-padded-to-meet-min-length-ok-03", // ExpiresAt nil
		})
		require.NoError(t, err)
		require.NotNil(t, rotated.ExpiresAt, "a nil ExpiresAt must inherit, never clear, the old expiry")
		require.WithinDuration(t, oldExpiry, *rotated.ExpiresAt, time.Second)
	})
}

func TestAuthStore_RotateAPIKey_RollbackOnInsertFailure(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	auth := authTestSetup(t)
	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old-padding-to-meet-min-length-check",
	})

	// Build a SECOND store with a bad generator — returns empty string
	// which violates the prefix length CHECK constraint (23514), forcing
	// the INSERT to fail on the FIRST attempt (not via the 23505 retry
	// loop) and rolling back the entire tx including the revoke of `old`.
	badAuth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(
		func() (string, error) { return "", nil }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = badAuth.Close(ctx) })

	_, err = badAuth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-new-padding-to-meet-min-length-check",
	})
	require.Error(t, err)

	// Old key MUST still be live (rollback worked) and no new row exists.
	// `auth` and `badAuth` deliberately share the same pool/tables, so this
	// read-back via `auth` verifies committed/rolled-back DB state — not any
	// in-memory store state — of the rotate performed through `badAuth`.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.Nil(t, oldReload.RevokedAt, "old key must still be live after rollback")
	var keyCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id = $1::uuid`, u.ID).Scan(&keyCount))
	require.Equal(t, 1, keyCount, "only old key exists; no orphan from failed insert")
}

func TestAuthStore_RotateAPIKey_PrefixCollisionRetry(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	auth := authTestSetup(t)
	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old-padding-to-meet-min-length-check",
	})

	// Seed a separate user with a key whose prefix the rotate generator
	// will collide against. Using a different user keeps the test focused
	// on the rotate path (not on the user's own key inventory).
	other, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "bob", Role: "writer",
	}, nil)
	_, err := pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash)
		VALUES ($1::uuid, 'collidex', 'phc-bob-padding-to-meet-min-length-check')`, other.ID)
	require.NoError(t, err)

	// Generator returns the colliding prefix twice (triggers 23505 +
	// retry both times), then a fresh prefix on attempt 3.
	calls := 0
	gen := func() (string, error) {
		calls++
		if calls <= 2 {
			return "collidex", nil
		}
		return "rotated2", nil
	}
	rotateAuth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(gen))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rotateAuth.Close(ctx) })

	newKey, err := rotateAuth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-rot-padding-to-meet-min-length-check",
	})
	require.NoError(t, err)
	require.Equal(t, "rotated2", newKey.Prefix)
	require.GreaterOrEqual(t, calls, 3, "retry loop should have invoked the generator at least 3 times")

	// Old key is revoked, new key is active — same invariant as the happy
	// path, but the path through the retry loop is now exercised.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NotNil(t, oldReload.RevokedAt)
	newReload, _ := auth.LookupAPIKeyByPrefix(ctx, newKey.Prefix)
	require.Nil(t, newReload.RevokedAt)
}

func TestAuthStore_ListAPIKeys(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u1, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	u2, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "writer"}, nil)

	k1, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u1.ID, PHCHash: "phc1-padded-to-meet-min-length-ok-xxx", Label: "k1"})
	_, _ = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u1.ID, PHCHash: "phc2-padded-to-meet-min-length-ok-xxx", Label: "k2"})
	_, _ = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u2.ID, PHCHash: "phc3-padded-to-meet-min-length-ok-xxx", Label: "k3"})
	require.NoError(t, auth.RevokeAPIKey(ctx, k1.ID))

	// Per-user, excluding revoked by default.
	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u1.ID})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "k2", keys[0].Label)

	// IncludeRevoked.
	keys, err = auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u1.ID, IncludeRevoked: true})
	require.NoError(t, err)
	require.Len(t, keys, 2)

	// All users (admin).
	all, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{})
	require.NoError(t, err)
	require.Len(t, all, 2) // k1 revoked excluded, k2 and k3 remain
}

func TestAuthStore_TouchLastUsed(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	k, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-padded-to-meet-min-length-ok"})

	require.Nil(t, k.LastUsedAt)
	require.NoError(t, auth.TouchLastUsed(ctx, k.ID))

	reloaded, _ := auth.LookupAPIKeyByPrefix(ctx, k.Prefix)
	require.NotNil(t, reloaded.LastUsedAt)
}

func TestAuthStore_JITCreateHuman(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u := &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Email: "alice@x.com", Role: "reader"}
	b := &storage.OIDCBinding{Issuer: "iss1", Subject: "alice-sub", EmailAtBind: "alice@x.com"}

	user, binding, err := auth.JITCreateHuman(ctx, u, b)
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)
	require.NotEmpty(t, binding.ID)
	require.Equal(t, user.ID, binding.UserID)

	// Second JIT for same (issuer, subject) returns the existing user.
	user2, binding2, err := auth.JITCreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice-dup", Role: "reader"}, b)
	require.NoError(t, err)
	require.Equal(t, user.ID, user2.ID)
	require.Equal(t, binding.ID, binding2.ID)
}

func TestAuthStore_ListOIDCBindings(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-entra"})
	// Add a second binding (different provider).
	_, _ = pool.Exec(ctx, `INSERT INTO oidc_bindings (user_id, issuer, subject)
	                      VALUES ($1::uuid, 'github', 'alice-gh')`, u.ID)

	bindings, err := auth.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 2)

	// Empty list for unbound user.
	other, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "reader"}, nil)
	bindings, err = auth.ListOIDCBindings(ctx, other.ID)
	require.NoError(t, err)
	require.NotNil(t, bindings)
	require.Empty(t, bindings)
}

func TestAuthStore_UnbindOIDC(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-entra"})
	bindings, _ := auth.ListOIDCBindings(ctx, u.ID)
	require.Len(t, bindings, 1)

	require.NoError(t, auth.UnbindOIDC(ctx, bindings[0].ID))

	after, _ := auth.ListOIDCBindings(ctx, u.ID)
	require.Empty(t, after)

	// Idempotent.
	require.NoError(t, auth.UnbindOIDC(ctx, bindings[0].ID))
}

// TestAuthStore_BootstrapRace verifies that N concurrent CreateHuman calls with
// Bootstrap=true result in exactly one success and N-1 ErrBootstrapExists
// errors, proving the partial unique index users_one_bootstrap is correctly
// mapped at runtime.
func TestAuthStore_BootstrapRace(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	const concurrent = 5
	// Each goroutine writes its own result into a dedicated slot, so no
	// synchronization beyond the WaitGroup is needed (race-free).
	errs := make([]error, concurrent)
	var wg sync.WaitGroup
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := auth.CreateHuman(ctx, &storage.User{
				Kind: storage.KindHuman, DisplayName: "admin",
				Role: "admin", Bootstrap: true,
			}, nil)
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	var successes, expectsBootstrapExists int
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, storage.ErrBootstrapExists):
			expectsBootstrapExists++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 1, successes, "exactly one bootstrap insert should succeed")
	require.Equal(t, concurrent-1, expectsBootstrapExists)
}

// TestAuthStore_JITRace verifies that N concurrent JITCreateHuman calls for
// the same (issuer, subject) all resolve to the same user and binding, proving
// the race-recovery relookup path maintains the (issuer,subject) uniqueness
// invariant.
func TestAuthStore_JITRace(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	const concurrent = 5
	type result struct {
		userID    string
		bindingID string
		err       error
	}
	// Each goroutine writes its own result into a dedicated slot, so no
	// synchronization beyond the WaitGroup is needed (race-free).
	results := make([]result, concurrent)
	var wg sync.WaitGroup
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go func(idx int) {
			defer wg.Done()
			u, b, err := auth.JITCreateHuman(ctx,
				&storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
				&storage.OIDCBinding{Issuer: "iss1", Subject: "alice-sub"})
			if err != nil {
				results[idx] = result{err: err}
				return
			}
			results[idx] = result{userID: u.ID, bindingID: b.ID}
		}(i)
	}
	wg.Wait()

	userIDs := map[string]struct{}{}
	bindingIDs := map[string]struct{}{}
	for _, r := range results {
		require.NoError(t, r.err)
		userIDs[r.userID] = struct{}{}
		bindingIDs[r.bindingID] = struct{}{}
	}
	require.Len(t, userIDs, 1, "all callers should resolve to the same user")
	require.Len(t, bindingIDs, 1, "all callers should resolve to the same binding")
}

// TestAuthStore_FullLifecycle is an end-to-end smoke test walking the complete
// identity lifecycle: bootstrap → JIT-create real user → mint key → rotate key
// → promote role → soft-delete bootstrap, asserting final state is coherent.
//
// Plan note: the PHCHash values in the plan ("phc", "phc-rot") are shorter than
// the schema CHECK constraint (length(phc_hash) >= 32). They are replaced here
// with valid 32+ character strings.
func TestAuthStore_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Bootstrap admin.
	admin, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true,
	}, nil)
	require.NoError(t, err)

	got, err := auth.GetBootstrap(ctx)
	require.NoError(t, err)
	require.Equal(t, admin.ID, got.ID)

	// JIT a real person via OIDC.
	person, binding, err := auth.JITCreateHuman(ctx,
		&storage.User{Kind: storage.KindHuman, DisplayName: "alice", Email: "alice@x.com", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-sub", EmailAtBind: "alice@x.com"})
	require.NoError(t, err)

	// Mint an API key. PHCHash must be >= 32 chars (schema CHECK constraint).
	key, err := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: person.ID, PHCHash: "phc-stub-padded-to-meet-min-length-ok", Label: "personal",
	})
	require.NoError(t, err)

	// Rotate it. PHCHash must be >= 32 chars (schema CHECK constraint).
	newKey, err := auth.RotateAPIKey(ctx, key.ID, &storage.APIKey{
		UserID: person.ID, PHCHash: "phc-rot-stub-padded-to-meet-min-length", Label: "personal",
	})
	require.NoError(t, err)
	require.NotEqual(t, key.Prefix, newKey.Prefix)

	// Promote alice via UpdateUserRole.
	require.NoError(t, auth.UpdateUserRole(ctx, person.ID, "admin"))

	// Soft-delete bootstrap admin (force-flag policy enforced at handler layer, not here).
	require.NoError(t, auth.SoftDeleteUser(ctx, admin.ID))

	// GetBootstrap now empty.
	_, err = auth.GetBootstrap(ctx)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Final state: alice is admin with one active key, one binding.
	reloaded, err := auth.GetUserByID(ctx, person.ID)
	require.NoError(t, err)
	require.Equal(t, "admin", reloaded.Role)

	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: person.ID})
	require.NoError(t, err)
	require.Len(t, keys, 1)

	bindings, err := auth.ListOIDCBindings(ctx, person.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, binding.ID, bindings[0].ID)
}

// ---- Task 30: Invariant sweep — ServiceAccount has no OIDC bindings ----

// TestAuthStore_ServiceAccountNoBindingInvariant verifies that neither
// CreateHuman nor JITCreateHuman will accept Kind=ServiceAccount, so a
// ServiceAccount can never acquire an OIDC binding via these write surfaces.
func TestAuthStore_ServiceAccountNoBindingInvariant(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// CreateHuman already rejects Kind=ServiceAccount (per Task 13's
	// existing check). Verify that property survived.
	_, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "wrong", Role: "writer",
	}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "KindHuman")

	// JITCreateHuman with Kind=ServiceAccount must refuse explicitly.
	_, _, err = auth.JITCreateHuman(ctx,
		&storage.User{Kind: storage.KindServiceAccount, DisplayName: "wrong", Role: "reader"},
		&storage.OIDCBinding{Issuer: "iss", Subject: "sub"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "KindHuman")

	// Defense in depth: no rows created by either failed call.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE kind='service_account'`).Scan(&n))
	require.Equal(t, 0, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM oidc_bindings`).Scan(&n))
	require.Equal(t, 0, n)
}

// ---- Task 31: Boundary sweep — pagination, empty/null, missing-required ----

// TestAuthStore_Pagination_DefaultLimit verifies that Limit=0 is treated as the
// default (100), and that an offset beyond the result set returns a non-nil
// empty slice.
func TestAuthStore_Pagination_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Insert 5 users; default limit is 100 so all are returned.
	for i := 0; i < 5; i++ {
		_, err := auth.CreateHuman(ctx, &storage.User{
			Kind: storage.KindHuman, DisplayName: fmt.Sprintf("u%d", i), Role: "reader",
		}, nil)
		require.NoError(t, err)
	}

	// Limit=0 means "use default" (100), not "return zero".
	users, err := auth.ListUsers(ctx, storage.ListUsersFilter{Limit: 0})
	require.NoError(t, err)
	require.Len(t, users, 5)

	// Offset beyond rows returns empty slice (not nil, not error).
	users, err = auth.ListUsers(ctx, storage.ListUsersFilter{Offset: 100})
	require.NoError(t, err)
	require.Empty(t, users)
	require.True(t, users != nil, "empty result must be []*User{}, never nil")
}

// TestAuthStore_Pagination_MaxLimit verifies that a caller-supplied Limit
// above the storage maximum (maxListLimit = 1000) is clamped, so an admin
// passing Limit: 1_000_000 cannot trigger an unbounded fetch. Applies to both
// ListUsers and ListAPIKeys.
func TestAuthStore_Pagination_MaxLimit(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Seed 1001 users (one over the clamp) via a single batch insert.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (kind, display_name, role)
		SELECT 'human', 'u' || g, 'reader'
		FROM generate_series(1, 1001) AS g`)
	require.NoError(t, err)

	// A Limit far above the max is clamped to maxListLimit (1000), not honored
	// verbatim and not reset to the default 100.
	users, err := auth.ListUsers(ctx, storage.ListUsersFilter{Limit: 1_000_000})
	require.NoError(t, err)
	require.Len(t, users, 1000, "ListUsers must clamp an over-max Limit to maxListLimit")

	// Seed 1001 API keys for one user via a single batch insert. phc_hash must
	// satisfy CHECK(length(phc_hash) >= 32); prefixes must be unique.
	owner := users[0]
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash)
		SELECT $1::uuid, 'pfx' || lpad(g::text, 12, '0'),
		       '$argon2id$v=19$m=65536,t=1,p=4$stub-salt-padded-to-32chars'
		FROM generate_series(1, 1001) AS g`, owner.ID)
	require.NoError(t, err)

	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{Limit: 1_000_000})
	require.NoError(t, err)
	require.Len(t, keys, 1000, "ListAPIKeys must clamp an over-max Limit to maxListLimit")
}

// TestAuthStore_EmptyResults_NotNil verifies that ListOIDCBindings and
// ListAPIKeys return non-nil empty slices (not nil) when no rows match,
// and that ListUsers does the same.
func TestAuthStore_EmptyResults_NotNil(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"}, nil)
	require.NoError(t, err)

	// User with no bindings returns empty slice, not nil.
	bindingSlice, err := auth.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Empty(t, bindingSlice)
	require.NotNil(t, bindingSlice, "ListOIDCBindings empty result must be non-nil slice")

	// User with no keys returns empty slice, not nil.
	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u.ID})
	require.NoError(t, err)
	require.Empty(t, keys)
	require.NotNil(t, keys, "ListAPIKeys empty result must be non-nil slice")

	// ListUsers on an empty filter (after truncate) returns non-nil empty.
	truncateAuthTables(t, pool)
	allUsers, err := auth.ListUsers(ctx, storage.ListUsersFilter{})
	require.NoError(t, err)
	require.Empty(t, allUsers)
	require.True(t, allUsers != nil, "ListUsers empty result must be non-nil slice")
}

// TestAuthStore_RequiredFields verifies that CreateAPIKey returns clear errors
// for missing UserID and missing PHCHash, and that CreateServiceAccount
// rejects a missing OwnerUserID.
func TestAuthStore_RequiredFields(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// CreateAPIKey requires UserID.
	_, err := auth.CreateAPIKey(ctx, &storage.APIKey{PHCHash: "phc-stub-padded-to-meet-min-length-ok"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "UserID required")

	// CreateAPIKey requires PHCHash.
	u, err := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	require.NoError(t, err)
	_, err = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PHCHash required")

	// CreateServiceAccount requires OwnerUserID (also covered in Task 14).
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "no-owner", Role: "writer",
	})
	require.Error(t, err)
}
