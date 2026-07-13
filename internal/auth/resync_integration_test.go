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

// TestResync_LiveRoleClamp proves SC#3's standing-key live floor at the
// storage+resolver level (cursor #5): a key minted while the user is `writer`
// resolves at EffectiveRole=writer; after UpdateUserRole lowers the user to
// `reader`, the SAME standing token resolves at EffectiveRole=reader on its next
// call — no re-mint, no re-login. This isolates the propagation mechanism
// (resolveAPIKey's per-request live DB role read) that ResyncUserRole relies on.
// Modeled on mint_integration_test.go.
func TestResync_LiveRoleClamp(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	// Clean slate.
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	// Create a human user at role writer.
	user, err := authStore.CreateHuman(ctx, &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "resync-live-floor-user",
		Role:        "writer",
	}, nil)
	require.NoError(t, err)

	// Mint a standing key (no per-key downgrade → EffectiveRole tracks user.role).
	secret, phc, err := auth.GenerateAPIKeySecret()
	require.NoError(t, err)
	created, err := authStore.CreateAPIKey(ctx, &storage.APIKey{UserID: user.ID, PHCHash: phc})
	require.NoError(t, err)
	token := auth.FormatAPIKeyToken(created.Prefix, secret)

	// Build the resolver.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)

	// Before the demotion: the standing key resolves at writer.
	id, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, auth.RoleWriter, id.EffectiveRole, "standing key resolves at the user's writer role")

	// Lower the user's role via the SAME write path ResyncUserRole reuses.
	require.NoError(t, authStore.UpdateUserRole(ctx, user.ID, "reader"))

	// After the demotion: the SAME token clamps to reader on its next call —
	// the live per-request DB read, not a re-mint or re-login.
	id2, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, auth.RoleReader, id2.EffectiveRole, "standing key clamps to the new reader floor on next resolve")
	require.Equal(t, auth.RoleReader, id2.Role, "resolved base role reflects the live DB write")
}
