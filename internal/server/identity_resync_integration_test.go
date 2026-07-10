// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package server

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// TestResync_RPCSeam_LiveRoleClamp proves SC#3's live floor is driven by the
// operator ResyncUserRole RPC seam itself — not only the storage/resolver
// primitive (cursor #3). It builds a real AuthStore + resolver against Postgres,
// mints a standing key while the user is `writer`, then demotes the user THROUGH
// the ResyncUserRole RPC handler (in-package construction gives access to the
// unexported fields) and asserts the SAME standing token clamps writer → reader
// on its next Resolve. This closes the unit-stub-to-operator-seam gap.
func TestResync_RPCSeam_LiveRoleClamp(t *testing.T) {
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
		DisplayName: "resync-rpc-seam-user",
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

	// Drive the demotion through the ResyncUserRole RPC handler itself (not
	// authStore.UpdateUserRole directly) — the operator seam under test.
	h := &IdentityHandler{users: authStore, logger: slog.Default()}
	resp, err := h.ResyncUserRole(ctx, connect.NewRequest(&specv1.ResyncUserRoleRequest{
		Id: user.ID, Role: "reader",
	}))
	require.NoError(t, err)
	require.Equal(t, "reader", resp.Msg.GetUser().GetRole(), "RPC returns the updated user at reader")

	// After the RPC-driven demotion: the SAME token clamps to reader on its
	// next call — proving the operator RPC seam drives the standing-key clamp.
	id2, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, auth.RoleReader, id2.EffectiveRole, "standing key clamps to reader after the ResyncUserRole RPC")
	require.Equal(t, auth.RoleReader, id2.Role, "resolved base role reflects the RPC-driven live DB write")
}
