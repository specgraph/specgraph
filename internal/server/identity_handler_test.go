// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// newTestIdentityHandler builds an httptest.Server with the IdentityService
// registered, using stub as the UsersBackend. Returns a live client.
func newTestIdentityHandler(t *testing.T, stub *usersBackendStub) specgraphv1connect.IdentityServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	RegisterIdentityService(mux, stub)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewIdentityServiceClient(http.DefaultClient, srv.URL)
}

func TestWhoami_ReturnsContextIdentity(t *testing.T) {
	stub := &usersBackendStub{}

	id := &auth.Identity{
		UserID:        "u-abc",
		Subject:       "oidc:sub123",
		DisplayName:   "Alice",
		Role:          auth.RoleAdmin,
		EffectiveRole: auth.RoleAdmin,
		Email:         "alice@example.com",
		Source:        "oidc",
	}

	// Wrap the handler with identity-injecting middleware so the server
	// receives the identity in the request context.
	mux := http.NewServeMux()
	RegisterIdentityService(mux, stub)
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(auth.WithIdentity(r.Context(), id))
		mux.ServeHTTP(w, r)
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	directClient := specgraphv1connect.NewIdentityServiceClient(http.DefaultClient, srv.URL)

	resp, err := directClient.Whoami(context.Background(), connect.NewRequest(&specv1.WhoamiRequest{}))
	require.NoError(t, err)
	require.Equal(t, "u-abc", resp.Msg.GetUserId())
	require.Equal(t, string(auth.RoleAdmin), resp.Msg.GetRole())
	require.Equal(t, string(auth.RoleAdmin), resp.Msg.GetEffectiveRole())
	require.Equal(t, "oidc", resp.Msg.GetSource())
	require.Equal(t, "alice@example.com", resp.Msg.GetEmail())
	require.Equal(t, "Alice", resp.Msg.GetDisplayName())
	require.Equal(t, "oidc:sub123", resp.Msg.GetSubject())
}

func TestWhoami_NoIdentityUnauthenticated(t *testing.T) {
	stub := &usersBackendStub{}
	client := newTestIdentityHandler(t, stub)

	_, err := client.Whoami(context.Background(), connect.NewRequest(&specv1.WhoamiRequest{}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnauthenticated, connErr.Code())
}

// --- Task 4: ListUsers + GetUser ---

func TestListUsers_MapsRows(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
			require.Equal(t, storage.KindHuman, f.Kind)
			require.True(t, f.IncludeDeleted)
			return []*storage.User{
				{ID: "u1", Kind: storage.KindHuman, Role: "admin", CreatedAt: time.Now()},
			}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.ListUsers(context.Background(), connect.NewRequest(&specv1.ListUsersRequest{
		Kind: specv1.UserKind_USER_KIND_HUMAN, IncludeDeleted: true,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetUsers(), 1)
	require.Equal(t, "u1", resp.Msg.GetUsers()[0].GetId())
}

func TestGetUser_NotFound(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(context.Context, string) (*storage.User, error) {
			return nil, storage.ErrUserNotFound
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.GetUser(context.Background(), connect.NewRequest(&specv1.GetUserRequest{Id: "missing"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetUser_Found(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			require.Equal(t, "u1", id)
			return &storage.User{ID: "u1", Kind: storage.KindHuman, Role: "reader", CreatedAt: time.Now()}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.GetUser(context.Background(), connect.NewRequest(&specv1.GetUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.Equal(t, "reader", resp.Msg.GetUser().GetRole())
}

// --- Task 5: CreateServiceAccount + UpdateUserRole ---

func TestCreateServiceAccount_Happy(t *testing.T) {
	stub := &usersBackendStub{
		createServiceAccount: func(_ context.Context, u *storage.User) (*storage.User, error) {
			require.Equal(t, storage.KindServiceAccount, u.Kind)
			require.Equal(t, "My SA", u.DisplayName)
			require.Equal(t, "owner-1", u.OwnerUserID)
			out := *u
			out.ID = "sa-new"
			out.CreatedAt = time.Now()
			return &out, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.CreateServiceAccount(context.Background(), connect.NewRequest(&specv1.CreateServiceAccountRequest{
		DisplayName: "My SA",
		Role:        "reader",
		OwnerUserId: "owner-1",
	}))
	require.NoError(t, err)
	require.Equal(t, "sa-new", resp.Msg.GetUser().GetId())
	require.Equal(t, specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, resp.Msg.GetUser().GetKind())
}

func TestCreateServiceAccount_RequiresDisplayName(t *testing.T) {
	stub := &usersBackendStub{}
	client := newTestIdentityHandler(t, stub)
	_, err := client.CreateServiceAccount(context.Background(), connect.NewRequest(&specv1.CreateServiceAccountRequest{
		Role:        "reader",
		OwnerUserId: "owner-1",
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUserRole_Happy(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(_ context.Context, id, role string) error {
			require.Equal(t, "u1", id)
			require.Equal(t, "admin", role)
			return nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, Role: "admin", CreatedAt: time.Now()}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.UpdateUserRole(context.Background(), connect.NewRequest(&specv1.UpdateUserRoleRequest{
		Id:   "u1",
		Role: "admin",
	}))
	require.NoError(t, err)
	require.Equal(t, "admin", resp.Msg.GetUser().GetRole())
}

func TestUpdateUserRole_NotFound(t *testing.T) {
	stub := &usersBackendStub{
		updateUserRole: func(context.Context, string, string) error {
			return storage.ErrUserNotFound
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UpdateUserRole(context.Background(), connect.NewRequest(&specv1.UpdateUserRoleRequest{
		Id:   "missing",
		Role: "reader",
	}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Task 6: SoftDeleteUser + PurgeUser ---

func TestSoftDeleteUser_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: false}, nil
		},
		softDeleteUser: func(_ context.Context, id string) error {
			require.Equal(t, "u1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestSoftDeleteUser_RefusesBootstrapWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: true}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "boot"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSoftDeleteUser_AllowsBootstrapWithForce(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: true}, nil
		},
		softDeleteUser: func(_ context.Context, id string) error {
			require.Equal(t, "boot", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "boot", Force: true}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestSoftDeleteUser_NonBootstrapNoForceNeeded(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: false}, nil
		},
		softDeleteUser: func(_ context.Context, id string) error {
			require.Equal(t, "u1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestPurgeUser_RefusesBootstrapWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: true}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "boot"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestPurgeUser_AllowsBootstrapWithForce(t *testing.T) {
	purged := false
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: true}, nil
		},
		purgeUser: func(context.Context, string) error { purged = true; return nil },
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "boot", Force: true}))
	require.NoError(t, err)
	require.True(t, purged)
}

func TestSoftDeleteUser_RequiresID(t *testing.T) {
	stub := &usersBackendStub{}
	client := newTestIdentityHandler(t, stub)
	_, err := client.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPurgeUser_NotFound(t *testing.T) {
	// NotFound now comes from the guard's GetUserByID pre-read (default stub →
	// ErrUserNotFound), not from the purgeUser mutation (which is never reached).
	stub := &usersBackendStub{
		purgeUser: func(context.Context, string) error {
			return storage.ErrUserNotFound
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "missing"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Code-review additions: Boundary/Happy coverage ---

func TestGetUser_RequiresID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.GetUser(context.Background(), connect.NewRequest(&specv1.GetUserRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateUserRole_RequiresRole(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.UpdateUserRole(context.Background(), connect.NewRequest(&specv1.UpdateUserRoleRequest{Id: "u1"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPurgeUser_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: false}, nil
		},
		purgeUser: func(_ context.Context, id string) error {
			require.Equal(t, "u1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestListUsers_DefaultLimit(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
			require.Equal(t, defaultIdentityListLimit, f.Limit)
			return nil, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.ListUsers(context.Background(), connect.NewRequest(&specv1.ListUsersRequest{}))
	require.NoError(t, err)
}

// --- Task 7+8: CreateAPIKey + RotateAPIKey ---

func TestCreateAPIKey_ReturnsPlaintextOnce(t *testing.T) {
	const userID = "u-create-key"
	stub := &usersBackendStub{
		createAPIKey: func(_ context.Context, k *storage.APIKey) (*storage.APIKey, error) {
			// Handler must NOT pass a caller-supplied prefix — storage generates it.
			require.Empty(t, k.Prefix, "handler must not supply a prefix; storage assigns it")
			// PHC hash must be argon2id.
			require.True(t, strings.HasPrefix(k.PHCHash, "$argon2id$"), "PHCHash must be PHC-encoded argon2id")
			require.Equal(t, userID, k.UserID)
			out := *k
			out.ID = "key-id-1"
			out.Prefix = "abcd1234"
			out.CreatedAt = time.Now()
			return &out, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.CreateAPIKey(context.Background(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
		UserId: userID,
		Label:  "my-key",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetPlaintext(), "plaintext token must be returned")
	require.True(t, strings.HasPrefix(resp.Msg.GetPlaintext(), auth.APIKeyTokenPrefix()), "plaintext must begin with the key prefix")
	require.Equal(t, "key-id-1", resp.Msg.GetKey().GetId())
	require.Equal(t, "abcd1234", resp.Msg.GetKey().GetPrefix())
	require.True(t, strings.Contains(resp.Msg.GetPlaintext(), "abcd1234"), "plaintext must contain the storage-assigned prefix")
}

func TestCreateAPIKey_RequiresUserID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.CreateAPIKey(context.Background(), connect.NewRequest(&specv1.CreateAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRotateAPIKey_ReturnsNewPlaintext(t *testing.T) {
	const (
		oldKeyID = "old-key-id"
		userID   = "u-rotate"
	)
	stub := &usersBackendStub{
		rotateAPIKey: func(_ context.Context, oldID string, newKey *storage.APIKey) (*storage.APIKey, error) {
			require.Equal(t, oldKeyID, oldID)
			require.Equal(t, userID, newKey.UserID)
			// Handler must NOT pass a prefix — storage assigns it.
			require.Empty(t, newKey.Prefix, "handler must not supply a prefix; storage assigns it")
			out := *newKey
			out.ID = "new-key-id"
			out.Prefix = "efgh5678"
			out.CreatedAt = time.Now()
			return &out, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.RotateAPIKey(context.Background(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
		KeyId:  oldKeyID,
		UserId: userID,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetPlaintext(), "new plaintext token must be returned")
	require.True(t, strings.HasPrefix(resp.Msg.GetPlaintext(), auth.APIKeyTokenPrefix()))
}

func TestRotateAPIKey_RequiresKeyAndUser(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})

	// Missing key_id.
	_, err := client.RotateAPIKey(context.Background(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
		UserId: "u1",
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// Missing user_id.
	_, err = client.RotateAPIKey(context.Background(), connect.NewRequest(&specv1.RotateAPIKeyRequest{
		KeyId: "k1",
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- Task 9: RevokeAPIKey + ListAPIKeys ---

func TestRevokeAPIKey_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		revokeAPIKey: func(_ context.Context, id string) error {
			require.Equal(t, "k1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.RevokeAPIKey(context.Background(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{KeyId: "k1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestRevokeAPIKey_RequiresID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.RevokeAPIKey(context.Background(), connect.NewRequest(&specv1.RevokeAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListAPIKeys_PassesFilter(t *testing.T) {
	stub := &usersBackendStub{
		listAPIKeys: func(_ context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			require.Equal(t, "u1", f.UserID)
			require.True(t, f.IncludeRevoked)
			return []*storage.APIKey{
				{ID: "key-1", UserID: "u1", Prefix: "abc12345", CreatedAt: time.Now()},
			}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.ListAPIKeys(context.Background(), connect.NewRequest(&specv1.ListAPIKeysRequest{
		UserId:         "u1",
		IncludeRevoked: true,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetKeys(), 1)
	require.Equal(t, "abc12345", resp.Msg.GetKeys()[0].GetPrefix())
}

// --- Task 10: ListOIDCBindings + UnbindOIDC ---

func TestListOIDCBindings_Happy(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, userID string) ([]*storage.OIDCBinding, error) {
			require.Equal(t, "u1", userID)
			return []*storage.OIDCBinding{
				{ID: "b1", UserID: "u1", Issuer: "https://accounts.example.com", Subject: "sub1", CreatedAt: time.Now()},
			}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	resp, err := client.ListOIDCBindings(context.Background(), connect.NewRequest(&specv1.ListOIDCBindingsRequest{UserId: "u1"}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetBindings(), 1)
	require.Equal(t, "b1", resp.Msg.GetBindings()[0].GetId())
}

func TestUnbindOIDC_Happy(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}, nil
		},
		unbindOIDC: func(_ context.Context, id string) error {
			require.Equal(t, "b1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1", Force: true}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestUnbindOIDC_NotFound(t *testing.T) {
	// NotFound now comes from the ownership check (binding "x" not in user's
	// bindings); unbindOIDC is never reached.
	stub := &usersBackendStub{
		unbindOIDC: func(context.Context, string) error {
			return storage.ErrOIDCBindingNotFound
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "x", UserId: "u1"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUnbindOIDC_RefusesLastCredentialWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}, nil
		},
		listAPIKeys: func(_ context.Context, _ storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			return nil, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUnbindOIDC_AllowsLastWithForce(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}, nil
		},
		unbindOIDC: func(_ context.Context, id string) error {
			require.Equal(t, "b1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1", Force: true}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestUnbindOIDC_AllowsWhenOtherCredentialsExist(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{
				{ID: "b1", UserID: "u1"},
				{ID: "b2", UserID: "u1"},
			}, nil
		},
		listAPIKeys: func(_ context.Context, _ storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			return nil, nil
		},
		unbindOIDC: func(_ context.Context, id string) error {
			require.Equal(t, "b1", id)
			called = true
			return nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1"}))
	require.NoError(t, err)
	require.True(t, called)
}

func TestUnbindOIDC_RequiresUserID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUnbindOIDC_RejectsBindingNotOwnedByUser(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{
				{ID: "b1", UserID: "u1"},
				{ID: "b2", UserID: "u1"},
			}, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "other", UserId: "u1", Force: true}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Code-review additions: boundary coverage (Tasks 9-10) ---

func TestListOIDCBindings_RequiresUserID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.ListOIDCBindings(context.Background(), connect.NewRequest(&specv1.ListOIDCBindingsRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUnbindOIDC_RequiresID(t *testing.T) {
	client := newTestIdentityHandler(t, &usersBackendStub{})
	_, err := client.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListAPIKeys_DefaultLimit(t *testing.T) {
	stub := &usersBackendStub{
		listAPIKeys: func(_ context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			require.Equal(t, defaultIdentityListLimit, f.Limit)
			return nil, nil
		},
	}
	client := newTestIdentityHandler(t, stub)
	_, err := client.ListAPIKeys(context.Background(), connect.NewRequest(&specv1.ListAPIKeysRequest{}))
	require.NoError(t, err)
}
