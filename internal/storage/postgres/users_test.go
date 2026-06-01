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
