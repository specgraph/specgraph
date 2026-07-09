// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

// selfKeyPHC is a valid argon2id-shaped PHC string >= 32 chars (schema CHECK).
const selfKeyPHC = "phc-self-padded-to-meet-min-length-ok-00"

// TestAuthStore_GetAPIKeyForUser_ForeignKeyNotFound proves owner-scoping on the
// read path: fetching another user's key returns ErrAPIKeyNotFound (T-02-04).
func TestAuthStore_GetAPIKeyForUser_ForeignKeyNotFound(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	bob, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "writer"}, nil)
	key, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC, Label: "alice-key"})
	require.NoError(t, err)

	// Owner can read.
	got, err := auth.GetAPIKeyForUser(ctx, alice.ID, key.ID)
	require.NoError(t, err)
	require.Equal(t, key.ID, got.ID)
	require.Equal(t, alice.ID, got.UserID)

	// Foreign caller gets a uniform NotFound (not a permission error).
	_, err = auth.GetAPIKeyForUser(ctx, bob.ID, key.ID)
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)

	// Missing key ID also NotFound.
	_, err = auth.GetAPIKeyForUser(ctx, alice.ID, "00000000-0000-0000-0000-aaaaaaaaaaaa")
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
}

// TestAuthStore_RevokeAPIKeyForUser_ForeignKeyNotFound proves owner-scoping on
// revoke: a foreign key is NotFound; re-revoking your own key is idempotent.
func TestAuthStore_RevokeAPIKeyForUser_ForeignKeyNotFound(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	bob, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "writer"}, nil)
	key, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC})
	require.NoError(t, err)

	// Bob cannot revoke Alice's key — uniform NotFound, and the key stays live.
	err = auth.RevokeAPIKeyForUser(ctx, bob.ID, key.ID)
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
	stillLive, err := auth.LookupAPIKeyByPrefix(ctx, key.Prefix)
	require.NoError(t, err)
	require.Nil(t, stillLive.RevokedAt, "foreign revoke must not touch the key")

	// Owner revoke succeeds.
	require.NoError(t, auth.RevokeAPIKeyForUser(ctx, alice.ID, key.ID))
	revoked, err := auth.LookupAPIKeyByPrefix(ctx, key.Prefix)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt)

	// Re-revoking your OWN already-revoked key is an idempotent no-op success
	// (Finding F4 — no revoked_at IS NULL guard on the owner-scoped path).
	require.NoError(t, auth.RevokeAPIKeyForUser(ctx, alice.ID, key.ID))

	// Missing key ID is NotFound.
	err = auth.RevokeAPIKeyForUser(ctx, alice.ID, "00000000-0000-0000-0000-aaaaaaaaaaaa")
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
}

// TestAuthStore_RotateAPIKeyForUser_ForeignKeyNotFound proves owner-scoping on
// rotate: a foreign or already-revoked key is uniformly NotFound.
func TestAuthStore_RotateAPIKeyForUser_ForeignKeyNotFound(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	bob, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "writer"}, nil)
	key, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC, RoleDowngrade: "reader"})
	require.NoError(t, err)

	// Bob cannot rotate Alice's key.
	_, err = auth.RotateAPIKeyForUser(ctx, bob.ID, key.ID, &storage.APIKey{PHCHash: "phc-new-padded-to-meet-min-length-ok-01"})
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
	// Alice's key remains live (the foreign rotate rolled back).
	stillLive, err := auth.LookupAPIKeyByPrefix(ctx, key.Prefix)
	require.NoError(t, err)
	require.Nil(t, stillLive.RevokedAt)

	// Rotating an already-revoked key (owner) is NotFound.
	require.NoError(t, auth.RevokeAPIKeyForUser(ctx, alice.ID, key.ID))
	_, err = auth.RotateAPIKeyForUser(ctx, alice.ID, key.ID, &storage.APIKey{PHCHash: "phc-new-padded-to-meet-min-length-ok-02"})
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
}

// TestAuthStore_RotateAPIKeyForUser_ExplicitArgs proves the rotated key carries
// exactly the explicit newKey values (floored downgrade, capped expiry), never
// the old key's stale higher ceiling (T-02-06).
func TestAuthStore_RotateAPIKeyForUser_ExplicitArgs(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "admin"}, nil)

	oldExpiry := time.Now().Add(720 * time.Hour).UTC() // long-lived old key
	old, err := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID:        alice.ID,
		PHCHash:       selfKeyPHC,
		Label:         "ci-bot",
		RoleDowngrade: "admin", // high ceiling on the OLD key
		ExpiresAt:     &oldExpiry,
	})
	require.NoError(t, err)

	// Rotate with an EXPLICIT lower downgrade and a shorter expiry.
	newExpiry := time.Now().Add(24 * time.Hour).UTC()
	rotated, err := auth.RotateAPIKeyForUser(ctx, alice.ID, old.ID, &storage.APIKey{
		PHCHash:       "phc-new-padded-to-meet-min-length-ok-03",
		Label:         "ci-bot-rotated",
		RoleDowngrade: "reader", // floored — must be persisted, not "admin"
		ExpiresAt:     &newExpiry,
	})
	require.NoError(t, err)
	require.NotEqual(t, old.Prefix, rotated.Prefix)

	// Returned value carries the explicit args.
	require.Equal(t, alice.ID, rotated.UserID)
	require.Equal(t, "reader", rotated.RoleDowngrade)
	require.Equal(t, "ci-bot-rotated", rotated.Label)
	require.NotNil(t, rotated.ExpiresAt)
	require.WithinDuration(t, newExpiry, *rotated.ExpiresAt, time.Second)

	// Persisted row also carries exactly the explicit args — NOT the old
	// key's "admin"/long expiry.
	reload, err := auth.LookupAPIKeyByPrefix(ctx, rotated.Prefix)
	require.NoError(t, err)
	require.Equal(t, "reader", reload.RoleDowngrade, "rotate must persist the explicit floored downgrade, not the old ceiling")
	require.NotNil(t, reload.ExpiresAt)
	require.WithinDuration(t, newExpiry, *reload.ExpiresAt, time.Second, "rotate must persist the explicit capped expiry")
	require.Nil(t, reload.RevokedAt)

	// Old key is revoked.
	oldReload, err := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NoError(t, err)
	require.NotNil(t, oldReload.RevokedAt)
}

// TestAuthStore_SelfMintQuota proves the quota cap: minting up to quota
// succeeds, the next mint returns ErrQuotaExceeded, and revoking a key frees a
// slot. Also exercises CountActiveAPIKeys.
func TestAuthStore_SelfMintQuota(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)

	const quota = 3
	var lastKeyID string
	for i := 0; i < quota; i++ {
		k, err := auth.CreateAPIKeyForUser(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC}, quota)
		require.NoError(t, err, "mint %d within quota should succeed", i)
		lastKeyID = k.ID
	}

	n, err := auth.CountActiveAPIKeys(ctx, alice.ID)
	require.NoError(t, err)
	require.Equal(t, quota, n)

	// The next mint is over quota.
	_, err = auth.CreateAPIKeyForUser(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC}, quota)
	require.ErrorIs(t, err, storage.ErrQuotaExceeded)

	// Revoking one frees a slot.
	require.NoError(t, auth.RevokeAPIKeyForUser(ctx, alice.ID, lastKeyID))
	n, err = auth.CountActiveAPIKeys(ctx, alice.ID)
	require.NoError(t, err)
	require.Equal(t, quota-1, n)
	_, err = auth.CreateAPIKeyForUser(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC}, quota)
	require.NoError(t, err, "a freed slot allows one more mint")

	// Missing/soft-deleted user is rejected.
	_, err = auth.CreateAPIKeyForUser(ctx, &storage.APIKey{UserID: "00000000-0000-0000-0000-aaaaaaaaaaaa", PHCHash: selfKeyPHC}, quota)
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}

// TestAuthStore_SelfMintQuota_Concurrency fires N > quota parallel mints for one
// user and asserts the final active count never exceeds quota — proving the
// parent-row FOR UPDATE lock serializes the count+insert (T-02-05, T-02-07).
func TestAuthStore_SelfMintQuota_Concurrency(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	alice, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)

	const quota = 5
	const concurrent = 20

	errs := make([]error, concurrent)
	var wg sync.WaitGroup
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := auth.CreateAPIKeyForUser(ctx, &storage.APIKey{UserID: alice.ID, PHCHash: selfKeyPHC}, quota)
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	var successes, overQuota int
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, storage.ErrQuotaExceeded):
			overQuota++
		default:
			t.Fatalf("unexpected error from concurrent mint: %v", err)
		}
	}

	require.Equal(t, quota, successes, "exactly quota mints should succeed")
	require.Equal(t, concurrent-quota, overQuota, "all remaining mints should be over-quota")

	// Ground truth: the DB never holds more than quota active keys.
	n, err := auth.CountActiveAPIKeys(ctx, alice.ID)
	require.NoError(t, err)
	require.LessOrEqual(t, n, quota, "active key count must never exceed quota (TOCTOU serialized)")
	require.Equal(t, quota, n)
}
