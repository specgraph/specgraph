// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// IdentityHandler implements the ConnectRPC IdentityService.
type IdentityHandler struct {
	users  storage.UsersBackend
	logger *slog.Logger
}

var _ specgraphv1connect.IdentityServiceHandler = (*IdentityHandler)(nil)

// RegisterIdentityService mounts the IdentityService on the given mux.
func RegisterIdentityService(mux *http.ServeMux, users storage.UsersBackend, opts ...connect.HandlerOption) {
	handler := &IdentityHandler{
		users:  users,
		logger: slog.Default(),
	}
	path, h := specgraphv1connect.NewIdentityServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// Whoami returns the identity of the authenticated caller.
func (h *IdentityHandler) Whoami(ctx context.Context, _ *connect.Request[specv1.WhoamiRequest]) (*connect.Response[specv1.WhoamiResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	return connect.NewResponse(&specv1.WhoamiResponse{
		Subject:       id.Subject,
		UserId:        id.UserID,
		DisplayName:   id.DisplayName,
		Role:          string(id.Role),
		EffectiveRole: string(id.EffectiveRole),
		Email:         id.Email,
		Source:        id.Source,
	}), nil
}

// identityError maps storage sentinel errors to appropriate connect error codes.
func (h *IdentityHandler) identityError(ctx context.Context, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	case errors.Is(err, storage.ErrAPIKeyNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("api key not found"))
	case errors.Is(err, storage.ErrOIDCBindingNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("oidc binding not found"))
	case errors.Is(err, storage.ErrBootstrapExists):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("bootstrap user already exists"))
	case errors.Is(err, storage.ErrAPIKeyPrefixExists):
		return connect.NewError(connect.CodeAborted, errors.New("api key prefix collision — retry"))
	default:
		h.logger.ErrorContext(ctx, "identityError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// --- Unimplemented stubs (replaced in Tasks 4–10) ---

const defaultIdentityListLimit = 100

// ListUsers returns a filtered, paginated list of users.
func (h *IdentityHandler) ListUsers(ctx context.Context, req *connect.Request[specv1.ListUsersRequest]) (*connect.Response[specv1.ListUsersResponse], error) {
	msg := req.Msg
	limit := int(msg.GetLimit())
	if limit == 0 {
		limit = defaultIdentityListLimit
	}
	users, err := h.users.ListUsers(ctx, storage.ListUsersFilter{
		Kind:           userKindFromProto(msg.GetKind()),
		Role:           msg.GetRole(),
		IncludeDeleted: msg.GetIncludeDeleted(),
		Limit:          limit,
		Offset:         int(msg.GetOffset()),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pbs, err := usersToProto(users)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListUsersResponse{Users: pbs}), nil
}

// GetUser returns a single user by ID.
func (h *IdentityHandler) GetUser(ctx context.Context, req *connect.Request[specv1.GetUserRequest]) (*connect.Response[specv1.GetUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	u, err := h.users.GetUserByID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.GetUserResponse{User: pb}), nil
}

// CreateServiceAccount creates a new service account owned by the given human user.
// Role validation against the project config is intentionally deferred to Cedar (4b+);
// an unknown role is default-deny-safe under Cedar.
func (h *IdentityHandler) CreateServiceAccount(ctx context.Context, req *connect.Request[specv1.CreateServiceAccountRequest]) (*connect.Response[specv1.CreateServiceAccountResponse], error) {
	msg := req.Msg
	if msg.GetDisplayName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("display_name is required"))
	}
	if msg.GetRole() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	if msg.GetOwnerUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("owner_user_id is required"))
	}
	u, err := h.users.CreateServiceAccount(ctx, &storage.User{
		Kind:        storage.KindServiceAccount,
		DisplayName: msg.GetDisplayName(),
		Role:        msg.GetRole(),
		OwnerUserID: msg.GetOwnerUserId(),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.CreateServiceAccountResponse{User: pb}), nil
}

// UpdateUserRole sets the role on an existing user, then returns the updated user.
// Role membership validation is intentionally deferred to Cedar (4b+);
// an unknown role is default-deny-safe under Cedar.
func (h *IdentityHandler) UpdateUserRole(ctx context.Context, req *connect.Request[specv1.UpdateUserRoleRequest]) (*connect.Response[specv1.UpdateUserRoleResponse], error) {
	msg := req.Msg
	if msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if msg.GetRole() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	if err := h.users.UpdateUserRole(ctx, msg.GetId(), msg.GetRole()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	u, err := h.users.GetUserByID(ctx, msg.GetId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.UpdateUserRoleResponse{User: pb}), nil
}

// requireForceForBootstrap refuses to delete the bootstrap admin unless force
// is set. It is a no-op for non-bootstrap users.
func (h *IdentityHandler) requireForceForBootstrap(ctx context.Context, id string, force bool) error {
	u, err := h.users.GetUserByID(ctx, id)
	if err != nil {
		return h.identityError(ctx, err)
	}
	if u.Bootstrap && !force {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("refusing to delete the bootstrap admin without force; pass --force to confirm"))
	}
	return nil
}

// SoftDeleteUser marks the user as deleted and revokes all their active keys.
// Deleting the bootstrap admin requires force (see requireForceForBootstrap).
func (h *IdentityHandler) SoftDeleteUser(ctx context.Context, req *connect.Request[specv1.SoftDeleteUserRequest]) (*connect.Response[specv1.SoftDeleteUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.requireForceForBootstrap(ctx, req.Msg.GetId(), req.Msg.GetForce()); err != nil {
		return nil, err
	}
	if err := h.users.SoftDeleteUser(ctx, req.Msg.GetId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.SoftDeleteUserResponse{}), nil
}

// PurgeUser hard-deletes the user, cascading through bindings and keys.
// Purging the bootstrap admin requires force (see requireForceForBootstrap).
func (h *IdentityHandler) PurgeUser(ctx context.Context, req *connect.Request[specv1.PurgeUserRequest]) (*connect.Response[specv1.PurgeUserResponse], error) {
	if req.Msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if err := h.requireForceForBootstrap(ctx, req.Msg.GetId(), req.Msg.GetForce()); err != nil {
		return nil, err
	}
	if err := h.users.PurgeUser(ctx, req.Msg.GetId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.PurgeUserResponse{}), nil
}

// CreateAPIKey mints a new API key for the given user. The plaintext bearer
// token is returned exactly once in the response and never stored.
func (h *IdentityHandler) CreateAPIKey(ctx context.Context, req *connect.Request[specv1.CreateAPIKeyRequest]) (*connect.Response[specv1.CreateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	newKey := &storage.APIKey{
		UserID:        msg.GetUserId(),
		PHCHash:       phc,
		Label:         msg.GetLabel(),
		RoleDowngrade: msg.GetRoleDowngrade(),
	}
	if ts := msg.GetExpiresAt(); ts != nil {
		t := ts.AsTime()
		newKey.ExpiresAt = &t
	}

	created, err := h.users.CreateAPIKey(ctx, newKey)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}

	plaintext := auth.FormatAPIKeyToken(created.Prefix, secret)
	return connect.NewResponse(&specv1.CreateAPIKeyResponse{
		Key:       apiKeyToProto(created),
		Plaintext: plaintext,
	}), nil
}

// RevokeAPIKey marks the given API key as revoked so it can no longer authenticate.
func (h *IdentityHandler) RevokeAPIKey(ctx context.Context, req *connect.Request[specv1.RevokeAPIKeyRequest]) (*connect.Response[specv1.RevokeAPIKeyResponse], error) {
	if req.Msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if err := h.users.RevokeAPIKey(ctx, req.Msg.GetKeyId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.RevokeAPIKeyResponse{}), nil
}

// RotateAPIKey atomically revokes the old key and issues a new one. The
// plaintext bearer token is returned exactly once in the response.
func (h *IdentityHandler) RotateAPIKey(ctx context.Context, req *connect.Request[specv1.RotateAPIKeyRequest]) (*connect.Response[specv1.RotateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "RotateAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	newKey := &storage.APIKey{
		UserID:        msg.GetUserId(),
		PHCHash:       phc,
		Label:         msg.GetLabel(),
		RoleDowngrade: msg.GetRoleDowngrade(),
	}

	created, err := h.users.RotateAPIKey(ctx, msg.GetKeyId(), newKey)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}

	plaintext := auth.FormatAPIKeyToken(created.Prefix, secret)
	return connect.NewResponse(&specv1.RotateAPIKeyResponse{
		Key:       apiKeyToProto(created),
		Plaintext: plaintext,
	}), nil
}

// ListAPIKeys returns a filtered, paginated list of API keys.
func (h *IdentityHandler) ListAPIKeys(ctx context.Context, req *connect.Request[specv1.ListAPIKeysRequest]) (*connect.Response[specv1.ListAPIKeysResponse], error) {
	msg := req.Msg
	limit := int(msg.GetLimit())
	if limit == 0 {
		limit = defaultIdentityListLimit
	}
	keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{
		UserID:         msg.GetUserId(),
		IncludeRevoked: msg.GetIncludeRevoked(),
		Limit:          limit,
		Offset:         int(msg.GetOffset()),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListAPIKeysResponse{Keys: apiKeysToProto(keys)}), nil
}

// ListOIDCBindings returns all OIDC bindings for the given user.
func (h *IdentityHandler) ListOIDCBindings(ctx context.Context, req *connect.Request[specv1.ListOIDCBindingsRequest]) (*connect.Response[specv1.ListOIDCBindingsResponse], error) {
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	bindings, err := h.users.ListOIDCBindings(ctx, req.Msg.GetUserId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListOIDCBindingsResponse{Bindings: oidcBindingsToProto(bindings)}), nil
}

// containsBindingID reports whether bindings contains a binding with the given ID.
func containsBindingID(bindings []*storage.OIDCBinding, id string) bool {
	for _, b := range bindings {
		if b.ID == id {
			return true
		}
	}
	return false
}

// UnbindOIDC removes an OIDC binding by ID. It verifies the binding belongs to
// user_id (ownership check) and refuses to remove a user's only remaining
// credential unless force is set.
func (h *IdentityHandler) UnbindOIDC(ctx context.Context, req *connect.Request[specv1.UnbindOIDCRequest]) (*connect.Response[specv1.UnbindOIDCResponse], error) {
	bindingID := req.Msg.GetBindingId()
	userID := req.Msg.GetUserId()
	if bindingID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("binding_id is required"))
	}
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}

	bindings, err := h.users.ListOIDCBindings(ctx, userID)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	if !containsBindingID(bindings, bindingID) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("oidc binding not found"))
	}

	if !req.Msg.GetForce() {
		keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{
			UserID:         userID,
			IncludeRevoked: false,
		})
		if err != nil {
			return nil, h.identityError(ctx, err)
		}
		if len(bindings)+len(keys) <= 1 {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("refusing to remove the user's only credential without force; pass --force to confirm"))
		}
	}

	if err := h.users.UnbindOIDC(ctx, bindingID); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.UnbindOIDCResponse{}), nil
}
