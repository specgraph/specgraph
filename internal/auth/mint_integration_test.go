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

// TestAPIKeyMint_RoundTrip is the cryptographic round-trip contract:
// a key minted via GenerateAPIKeySecret must be resolvable by the resolver.
// A failure here means mint's argon2 params or token format diverged from the
// resolver's expectations — fix mint, never hack the test.
func TestAPIKeyMint_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	// Clean slate.
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	// Create a human user.
	user, err := authStore.CreateHuman(ctx, &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "mint-test-user",
		Role:        "writer",
	}, nil)
	require.NoError(t, err)

	// Generate a secret + PHC hash via the mint helper.
	secret, phc, err := auth.GenerateAPIKeySecret()
	require.NoError(t, err)
	require.Len(t, secret, 32)

	// Persist the key WITHOUT a prefix — storage assigns one.
	created, err := authStore.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  user.ID,
		PHCHash: phc,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.Prefix, "storage must assign a prefix")

	// Assemble the bearer token from storage-assigned prefix + mint secret.
	token := auth.FormatAPIKeyToken(created.Prefix, secret)

	// Build resolver.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   authStore,
		Tracker: tracker,
	})
	require.NoError(t, err)

	// Round-trip: the resolver must accept the minted token and return the correct UserID.
	id, err := resolver.Resolve(ctx, token)
	require.NoError(t, err, "minted token must resolve — argon2 params or token format mismatch if this fails")
	require.Equal(t, user.ID, id.UserID)
}
