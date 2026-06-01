//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
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
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old-padded-to-meet-min-length-ok-00", Label: "ci-bot",
	})

	newKey, err := auth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-new-padded-to-meet-min-length-ok-00", Label: "ci-bot",
	})
	require.NoError(t, err)
	require.NotEqual(t, old.Prefix, newKey.Prefix)
	require.Equal(t, "ci-bot", newKey.Label)

	// Old key is revoked, new key is active.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NotNil(t, oldReload.RevokedAt)
	newReload, _ := auth.LookupAPIKeyByPrefix(ctx, newKey.Prefix)
	require.Nil(t, newReload.RevokedAt)
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
