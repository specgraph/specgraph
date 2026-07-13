// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// IdentityHandler implements the ConnectRPC IdentityService.
type IdentityHandler struct {
	users  storage.UsersBackend
	logger *slog.Logger
	// selfKeys is the self-service key policy (expiry caps, quota, rate limit)
	// consumed by the four self-mint handlers (AUTH-03).
	selfKeys config.SelfServiceKeysConfig
	// selfMintLimiters holds one token-bucket limiter per identity (userID ->
	// *rate.Limiter), bounding argon2id compute spent on self-mint/rotate
	// (T-02-17). Lazily populated by selfMintLimiter.
	selfMintLimiters sync.Map
}

var _ specgraphv1connect.IdentityServiceHandler = (*IdentityHandler)(nil)

// RegisterIdentityService mounts the IdentityService on the given mux.
// selfKeys carries the self-service key policy (expiry caps, quota, rate limit)
// consumed by the self-mint handlers (AUTH-03).
func RegisterIdentityService(mux *http.ServeMux, users storage.UsersBackend, selfKeys config.SelfServiceKeysConfig, opts ...connect.HandlerOption) {
	handler := &IdentityHandler{
		users:    users,
		logger:   slog.Default(),
		selfKeys: selfKeys,
	}
	path, h := specgraphv1connect.NewIdentityServiceHandler(handler, opts...)
	// Mount the Plan 03 double-submit CSRF validator IN FRONT of the Connect
	// IdentityService handler (T-02-31b). It self-scopes to cookie-authed POSTs
	// on the four self-key procedures and exempts Bearer/CLI callers, so admin
	// RPCs and CLI usage are unaffected.
	mux.Handle(path, csrfValidate(h))
}

// selfMintLimiter returns (or lazily creates) the per-identity token-bucket
// limiter that bounds self-mint/rotate attempts for a single user, reusing the
// sync.Map pattern from the OIDC JIT limiter (identitystore.go). Thresholds come
// from selfKeys; non-positive values fall back to safe defaults (30/hr, burst 5).
func (h *IdentityHandler) selfMintLimiter(userID string) *rate.Limiter {
	if l, ok := h.selfMintLimiters.Load(userID); ok {
		return l.(*rate.Limiter) //nolint:errcheck // sync.Map always stores *rate.Limiter
	}
	perHr := h.selfKeys.RateLimitPerHour
	if perHr <= 0 {
		perHr = 30
	}
	burst := h.selfKeys.RateLimitBurst
	if burst <= 0 {
		burst = 5
	}
	refill := rate.Every(time.Hour / time.Duration(perHr))
	l := rate.NewLimiter(refill, burst)
	actual, _ := h.selfMintLimiters.LoadOrStore(userID, l)
	return actual.(*rate.Limiter) //nolint:errcheck // sync.Map always stores *rate.Limiter
}

// clampExpiry resolves a self-mint request's expiry against the policy caps
// (D-08). An absent request timestamp defaults to now+defaultDays; a timestamp
// beyond now+maxDays is rejected with CodeInvalidArgument (T-02-18). The
// returned time is always non-nil on success so self-minted keys never live
// forever.
func clampExpiry(reqTs *timestamppb.Timestamp, defaultDays, maxDays int) (*time.Time, error) {
	now := time.Now().UTC()
	if reqTs == nil {
		exp := now.Add(time.Duration(defaultDays) * 24 * time.Hour)
		return &exp, nil
	}
	req := reqTs.AsTime()
	maxAllowed := now.Add(time.Duration(maxDays) * 24 * time.Hour)
	if req.After(maxAllowed) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("expires_at exceeds the maximum allowed key lifetime"))
	}
	return &req, nil
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
	case errors.Is(err, storage.ErrQuotaExceeded):
		return connect.NewError(connect.CodeResourceExhausted, errors.New("api key quota exceeded"))
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
	// RoleDowngrade is a privilege cap and is only meaningful among the ranked
	// built-in roles; a custom target would fail closed to reader at resolve
	// time, so reject it here with a clear error (spgr-rjrt.9).
	if d := msg.GetRoleDowngrade(); d != "" && !auth.IsBuiltinRole(auth.Role(d)) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("role_downgrade must be one of: reader, writer, admin"))
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

// RotateAPIKey atomically revokes the old key and issues a new one. Rotation
// preserves identity/authority — storage inherits owner/label/role_downgrade
// from the old key — so the request carries only key_id and an optional
// expires_at. The plaintext bearer token is returned exactly once.
func (h *IdentityHandler) RotateAPIKey(ctx context.Context, req *connect.Request[specv1.RotateAPIKeyRequest]) (*connect.Response[specv1.RotateAPIKeyResponse], error) {
	msg := req.Msg
	if msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "RotateAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	// Only PHCHash (new secret) and an optional ExpiresAt override are set;
	// owner/label/role_downgrade are inherited by storage from the old key.
	newKey := &storage.APIKey{PHCHash: phc}
	if ts := msg.GetExpiresAt(); ts != nil {
		t := ts.AsTime()
		newKey.ExpiresAt = &t
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

// The methods below are compile stubs added in plan 02-01 (proto/gen only) so
// the tree builds against the expanded IdentityServiceHandler interface. Real
// logic is implemented downstream in this phase: the four self-service handlers
// and ResyncUserRole (owner derived from context, quota/rate-limit, hard
// off-board). Until then they return CodeUnimplemented.

// CreateMyAPIKey mints an API key owned by the authenticated caller. The owner
// is derived from context (never a request field); an api-key caller is
// rejected to prevent key-chaining (T-02-15); the minted role is floored at the
// caller's live effective role (T-02-14); expiry is clamped to the policy caps
// (T-02-18); and minting is per-identity rate limited (T-02-17). The plaintext
// bearer token is returned exactly once and never stored.
func (h *IdentityHandler) CreateMyAPIKey(ctx context.Context, req *connect.Request[specv1.CreateMyAPIKeyRequest]) (*connect.Response[specv1.CreateMyAPIKeyResponse], error) {
	msg := req.Msg
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	if id.Source == "apikey" {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("api keys may not mint keys; sign in to provision a key"))
	}
	if !h.selfMintLimiter(id.UserID).Allow() {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("self-mint rate limit exceeded"))
	}

	// RoleDowngrade is a privilege cap meaningful only among the ranked
	// built-in roles; reject a custom target early.
	if d := msg.GetRoleDowngrade(); d != "" && !auth.IsBuiltinRole(auth.Role(d)) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("role_downgrade must be one of: reader, writer, admin"))
	}
	downgrade := msg.GetRoleDowngrade()
	if downgrade == "" {
		downgrade = string(id.EffectiveRole)
	}
	floored := auth.RoleMin(auth.Role(downgrade), id.EffectiveRole)

	expiry, err := clampExpiry(msg.GetExpiresAt(), h.selfKeys.DefaultTTLDays, h.selfKeys.MaxTTLDays)
	if err != nil {
		return nil, err
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateMyAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	created, err := h.users.CreateAPIKeyForUser(ctx, &storage.APIKey{
		UserID:        id.UserID,
		PHCHash:       phc,
		Label:         msg.GetLabel(),
		RoleDowngrade: string(floored),
		ExpiresAt:     expiry,
	}, h.selfKeys.Quota)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}

	// Structured audit line: server-derived fields ONLY. The plaintext secret
	// and the raw user-supplied label are never logged (log-injection / PII
	// guard, RESEARCH §6 / T-02-33).
	h.logger.InfoContext(ctx, "apikey.self.create",
		slog.String("actor", id.UserID),
		slog.String("key_id", created.ID),
		slog.String("action", "create"))

	return connect.NewResponse(&specv1.CreateMyAPIKeyResponse{
		Key:       apiKeyToProto(created),
		Plaintext: auth.FormatAPIKeyToken(created.Prefix, secret),
	}), nil
}

// ListMyAPIKeys returns only the authenticated caller's API keys. The storage
// filter's UserID is HARD-SET from context — never from the request — so an
// empty filter can never leak every user's keys (T-02-16).
func (h *IdentityHandler) ListMyAPIKeys(ctx context.Context, _ *connect.Request[specv1.ListMyAPIKeysRequest]) (*connect.Response[specv1.ListMyAPIKeysResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: id.UserID})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListMyAPIKeysResponse{Keys: apiKeysToProto(keys)}), nil
}

// RotateMyAPIKey mints a fresh secret for one of the caller's own keys and
// revokes the old one. The role ceiling and expiry are RE-DERIVED here — never
// inherited from the old key: the new role is floored at the caller's live
// effective role (T-02-14) and expiry is re-clamped to the policy caps
// (defaulting to DefaultTTLDays, not the old window). Owner-from-context,
// api-key rejection, and rate limiting mirror CreateMyAPIKey. The plaintext
// bearer token is returned exactly once.
func (h *IdentityHandler) RotateMyAPIKey(ctx context.Context, req *connect.Request[specv1.RotateMyAPIKeyRequest]) (*connect.Response[specv1.RotateMyAPIKeyResponse], error) {
	msg := req.Msg
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	if id.Source == "apikey" {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("api keys may not rotate keys; sign in to rotate a key"))
	}
	if msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if !h.selfMintLimiter(id.UserID).Allow() {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("self-mint rate limit exceeded"))
	}

	old, err := h.users.GetAPIKeyForUser(ctx, id.UserID, msg.GetKeyId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	// Re-floor at the caller's live effective role; never re-pin the old
	// key's stale ceiling.
	floored := auth.RoleMin(auth.Role(old.RoleDowngrade), id.EffectiveRole)

	// A ttl-less rotate defaults to DefaultTTLDays; it does NOT inherit the
	// old key's window.
	expiry, err := clampExpiry(msg.GetExpiresAt(), h.selfKeys.DefaultTTLDays, h.selfKeys.MaxTTLDays)
	if err != nil {
		return nil, err
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "RotateMyAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	newKey := &storage.APIKey{PHCHash: phc, RoleDowngrade: string(floored), ExpiresAt: expiry}
	rotated, err := h.users.RotateAPIKeyForUser(ctx, id.UserID, msg.GetKeyId(), newKey)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}

	h.logger.InfoContext(ctx, "apikey.self.rotate",
		slog.String("actor", id.UserID),
		slog.String("key_id", rotated.ID),
		slog.String("action", "rotate"))

	return connect.NewResponse(&specv1.RotateMyAPIKeyResponse{
		Key:       apiKeyToProto(rotated),
		Plaintext: auth.FormatAPIKeyToken(rotated.Prefix, secret),
	}), nil
}

// RevokeMyAPIKey revokes one of the authenticated caller's own keys. The
// owner is derived from context and passed to the owner-scoped storage call, so
// a foreign or missing key surfaces as CodeNotFound and can never touch another
// user's key (T-02-16). Re-revoking the caller's own key is an idempotent
// success (Finding F4).
func (h *IdentityHandler) RevokeMyAPIKey(ctx context.Context, req *connect.Request[specv1.RevokeMyAPIKeyRequest]) (*connect.Response[specv1.RevokeMyAPIKeyResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	if req.Msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if err := h.users.RevokeAPIKeyForUser(ctx, id.UserID, req.Msg.GetKeyId()); err != nil {
		return nil, h.identityError(ctx, err)
	}

	// Structured audit line: server-derived fields ONLY. The plaintext secret
	// and the raw user-supplied label are never logged (log-injection / PII
	// guard, RESEARCH §6 / T-02-33).
	h.logger.InfoContext(ctx, "apikey.self.revoke",
		slog.String("actor", id.UserID),
		slog.String("key_id", req.Msg.GetKeyId()),
		slog.String("action", "revoke"))

	return connect.NewResponse(&specv1.RevokeMyAPIKeyResponse{}), nil
}

// ResyncUserRole forces a re-application of a user's authoritative role
// (AUTH-02). It is admin-gated (mapped to user.manage in the Plan 04 action
// map). The role write goes through the EXISTING UpdateUserRole path — the
// single reusable server-side seam (D-01/D-04) — which sets users.role, the
// live floor that every standing key clamps to on its next request via
// resolveAPIKey's per-request live-role read (no re-mint, no re-login). When
// revoke_keys is set it additionally hard-revokes the user's active standing
// keys for a full off-board (D-02). The input role stays explicit
// (operator-supplied) so a future automation driver can reuse this same
// entrypoint with a derived role (D-01 seam); no IdP derivation this phase
// (D-05). No schema change (D-03).
func (h *IdentityHandler) ResyncUserRole(ctx context.Context, req *connect.Request[specv1.ResyncUserRoleRequest]) (*connect.Response[specv1.ResyncUserRoleResponse], error) {
	msg := req.Msg
	if msg.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if msg.GetRole() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}

	// Write users.role via the reusable seam → live floor for all standing keys.
	if err := h.users.UpdateUserRole(ctx, msg.GetId(), msg.GetRole()); err != nil {
		return nil, h.identityError(ctx, err)
	}

	// D-02 hard off-board: revoke every active standing key for the user.
	// RevokeAPIKey is idempotent (already-revoked keys are no-ops), and we skip
	// inactive keys so a spurious revoke never fires on the convergence path.
	revoked := 0
	if msg.GetRevokeKeys() {
		keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: msg.GetId()})
		if err != nil {
			return nil, h.identityError(ctx, err)
		}
		now := time.Now()
		for _, k := range keys {
			if !k.IsActive(now) {
				continue
			}
			if err := h.users.RevokeAPIKey(ctx, k.ID); err != nil {
				return nil, h.identityError(ctx, err)
			}
			revoked++
		}
	}

	u, err := h.users.GetUserByID(ctx, msg.GetId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	pb, err := userToProto(u)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}

	// Structured audit line for the forced demotion / off-board (T-02-22):
	// server-derived fields ONLY — target user, applied role, revoke intent, and
	// the count of keys revoked. No token material is ever logged.
	h.logger.InfoContext(ctx, "user.resync",
		slog.String("target", msg.GetId()),
		slog.String("role", msg.GetRole()),
		slog.Bool("revoke_keys", msg.GetRevokeKeys()),
		slog.Int("keys_revoked", revoked),
		slog.String("action", "resync"))

	return connect.NewResponse(&specv1.ResyncUserRoleResponse{User: pb}), nil
}
