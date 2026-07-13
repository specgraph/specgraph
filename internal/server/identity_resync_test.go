// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- AUTH-02: ResyncUserRole operator seam (unit / fake backend) ---

// TestResyncUserRole_WritesRole proves the operator-driven re-sync writes the
// target user's role through the existing UpdateUserRole path (the live floor
// every standing key clamps to on its next request) and returns the updated
// user. The role-derivation INPUT stays explicit (operator-supplied) so a
// future automation driver reuses the same entrypoint (D-01 seam).
func TestResyncUserRole_WritesRole(t *testing.T) {
	var gotID, gotRole string
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, userID, role string) error {
			gotID, gotRole = userID, role
			return nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, DisplayName: "u", Role: "reader", CreatedAt: time.Now()}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)

	resp, err := h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{
		Id: "u1", Role: "reader",
	}))
	require.NoError(t, err)
	require.Equal(t, "u1", gotID, "id forwarded to UpdateUserRole")
	require.Equal(t, "reader", gotRole, "operator-supplied role forwarded verbatim")
	require.Equal(t, "reader", resp.Msg.GetUser().GetRole(), "response mirrors UpdateUserRole's updated-user shape")
}

// TestResyncUserRole_RevokeKeysTrue proves the hard off-board path: with
// revoke_keys set, every active standing key for the user is revoked (D-02).
func TestResyncUserRole_RevokeKeysTrue(t *testing.T) {
	revoked := map[string]bool{}
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, _, _ string) error { return nil },
		listAPIKeys: func(_ context.Context, filter storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			require.Equal(t, "u1", filter.UserID, "keys listed scoped to the target user")
			return []*storage.APIKey{
				{ID: "k1", UserID: "u1"},
				{ID: "k2", UserID: "u1"},
			}, nil
		},
		revokeAPIKey: func(_ context.Context, keyID string) error {
			revoked[keyID] = true
			return nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, Role: "reader", CreatedAt: time.Now()}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)

	_, err := h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{
		Id: "u1", Role: "reader", RevokeKeys: true,
	}))
	require.NoError(t, err)
	require.True(t, revoked["k1"], "active key k1 revoked on hard off-board")
	require.True(t, revoked["k2"], "active key k2 revoked on hard off-board")
	require.Len(t, revoked, 2, "all active keys revoked")
}

// TestResyncUserRole_RevokeKeysFalse proves the convergence (soft) path: without
// revoke_keys, no key is revoked — standing keys keep working at the reduced
// role floor (D-03). The revokeAPIKey/listAPIKeys stubs are left unset;
// revokeAPIKey fails loud if called, so a spurious revoke surfaces immediately.
func TestResyncUserRole_RevokeKeysFalse(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, _, _ string) error { return nil },
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, Role: "reader", CreatedAt: time.Now()}, nil
		},
		// revokeAPIKey intentionally unset → errUnexpected if the handler
		// touches it when revoke_keys is false.
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)

	_, err := h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{
		Id: "u1", Role: "reader", RevokeKeys: false,
	}))
	require.NoError(t, err, "soft re-sync must not revoke keys")
}

// TestResyncUserRole_UnknownUser proves a missing user maps to CodeNotFound
// (asserted on the connect code, not the sanitized message string).
func TestResyncUserRole_UnknownUser(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, _, _ string) error { return storage.ErrUserNotFound },
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)

	_, err := h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{
		Id: "ghost", Role: "reader",
	}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestResyncUserRole_RequiresIDAndRole asserts the input guards reject empty
// id/role with CodeInvalidArgument before any storage write.
func TestResyncUserRole_RequiresIDAndRole(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)

	_, err := h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{Role: "reader"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "empty id rejected")

	_, err = h.ResyncUserRole(context.Background(), connect.NewRequest(&specv1.ResyncUserRoleRequest{Id: "u1"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "empty role rejected")
}
