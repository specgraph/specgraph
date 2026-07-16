// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// UsersBackend defines storage operations for the identity domain (users,
// API keys, OIDC bindings). It is the canonical cross-domain seam for any
// other domain that needs to ask identity questions (story #3, #4).
//
// Implementations MUST honor the invariants stated on User and the design
// document (docs/plans/2026-05-22-identity-storage-design.md).
type UsersBackend interface {
	// --- resolve-path queries (hot) ---

	// LookupAPIKeyByPrefix returns the key with the given prefix. Returns
	// ErrAPIKeyNotFound on miss. Does not verify the secret; callers must.
	LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error)

	// LookupOIDCBinding returns the binding for the (issuer, subject) pair.
	// Returns ErrOIDCBindingNotFound on miss.
	LookupOIDCBinding(ctx context.Context, issuer, subject string) (*OIDCBinding, error)

	// GetUserByID returns the user with the given ID, including soft-deleted.
	// Callers gate on User.IsActive() per their semantics. Returns
	// ErrUserNotFound on miss.
	GetUserByID(ctx context.Context, id string) (*User, error)

	// GetBootstrap returns the active bootstrap user, or ErrUserNotFound if
	// none exists. Used by bootstrap path to detect idempotency.
	GetBootstrap(ctx context.Context) (*User, error)

	// --- user CRUD ---

	// CreateHuman inserts a Human row. The OIDCBinding is created in the
	// same transaction; pass binding=nil for admin-created Humans (rare).
	// Returns ErrBootstrapExists if u.Bootstrap is true AND a bootstrap
	// already exists.
	CreateHuman(ctx context.Context, u *User, binding *OIDCBinding) (*User, error)

	// CreateServiceAccount inserts a ServiceAccount row. u.OwnerUserID
	// must reference an existing active Human.
	CreateServiceAccount(ctx context.Context, u *User) (*User, error)

	// UpdateUserRole sets the role on an active user. Role validation
	// against the YAML config is the caller's responsibility.
	UpdateUserRole(ctx context.Context, userID, role string) error

	// UpdateUserOnLogin sets display_name, email, AND role on an active user in
	// a single UPDATE (deleted_at IS NULL guard, like UpdateUserRole). Used ONLY
	// by applyLoginSync, which legitimately needs to write all three fields
	// together after re-deriving role from the current claims mapping. Returns
	// ErrUserNotFound if no active row matched. Role validation is the
	// caller's responsibility.
	//
	// Do NOT call this from any path that only intends to touch display_name —
	// use UpdateDisplayNameOnLogin instead. A second caller of this method that
	// derives role/email differently than applyLoginSync would reintroduce the
	// lost-update race fixed by CR-01 (AUTH-06 deep review, iteration 3).
	UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error

	// UpdateDisplayNameOnLogin sets ONLY display_name on an active user
	// (deleted_at IS NULL guard). Used by materializeIdentity's standalone
	// display-name reconciliation branch (the one that fires when the
	// login-sync gate does NOT — i.e. login-sync disabled, or a non-interactive
	// resolve) so that path structurally cannot write role or email. Returns
	// ErrUserNotFound if no active row matched.
	UpdateDisplayNameOnLogin(ctx context.Context, userID, displayName string) error

	// SoftDeleteUser sets deleted_at and revokes all active keys in one tx.
	// Idempotent (re-deleting already-deleted user is a no-op). An
	// unknown/nonexistent userID is also treated as a no-op: it returns nil,
	// NOT ErrUserNotFound. This intentional idempotent-delete behavior is
	// distinct from UpdateUserRole, which returns ErrUserNotFound for a
	// missing target. (The deleted_at IS NULL guard means an already-deleted
	// user matches zero rows too, so erroring on zero rows would break the
	// documented re-delete idempotency.)
	SoftDeleteUser(ctx context.Context, userID string) error

	// PurgeUser hard-deletes the user; cascades through bindings and keys.
	PurgeUser(ctx context.Context, userID string) error

	// ListUsers returns users matching the filter, optionally including
	// soft-deleted rows. Pagination is offset/limit; impl may add cursor
	// later.
	ListUsers(ctx context.Context, filter ListUsersFilter) ([]*User, error)

	// --- API key CRUD ---

	// CreateAPIKey inserts a new key. Retries on prefix-uniqueness violation
	// up to 3 times before returning ErrAPIKeyPrefixExists.
	CreateAPIKey(ctx context.Context, k *APIKey) (*APIKey, error)

	// RevokeAPIKey marks the key revoked. Idempotent on already-revoked keys.
	RevokeAPIKey(ctx context.Context, keyID string) error

	// RotateAPIKey revokes the old key and creates a new one in one transaction.
	// Owner (user_id), role_downgrade, and label are always inherited from the
	// old key. The caller supplies newKey.PHCHash (the new secret) and MAY set
	// newKey.ExpiresAt to override the new key's expiry; a nil ExpiresAt inherits
	// the old key's expiry (fail-safe — never silently cleared).
	// Returns the new key with a freshly generated prefix and new ID.
	RotateAPIKey(ctx context.Context, oldKeyID string, newKey *APIKey) (*APIKey, error)

	// ListAPIKeys returns keys for the given user; pass userID="" to list
	// across all users (admin operation). Excludes revoked keys unless
	// IncludeRevoked is set.
	ListAPIKeys(ctx context.Context, filter ListAPIKeysFilter) ([]*APIKey, error)

	// TouchLastUsed sets last_used_at = now() for the key. Fire-and-forget
	// from caller's perspective; impl is fast and ignores errors silently
	// (but returns them for tests).
	TouchLastUsed(ctx context.Context, keyID string) error

	// --- owner-scoped (self-service) API key CRUD ---
	//
	// These methods enforce ownership in the SQL WHERE clause so a caller can
	// only ever act on keys they own. A key that exists but is owned by
	// someone else is indistinguishable from a missing key: every owner-scoped
	// method returns ErrAPIKeyNotFound uniformly (enumeration-hardening, T-02-04).
	// Storage NEVER generates or returns the plaintext secret — the handler
	// owns secret generation (mirrors the admin CreateAPIKey/RotateAPIKey). The
	// caller supplies key.PHCHash; storage returns *APIKey only.

	// GetAPIKeyForUser returns the caller's key. Returns ErrAPIKeyNotFound if
	// the key does not exist OR belongs to another user (uniform NotFound).
	// Used by rotate to re-floor the old key's downgrade at the handler.
	GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*APIKey, error)

	// RevokeAPIKeyForUser marks the caller's own key revoked. Re-revoking an
	// already-revoked key you own is an idempotent no-op success. A foreign or
	// missing key returns ErrAPIKeyNotFound (uniform NotFound, T-02-04).
	RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error

	// RotateAPIKeyForUser revokes the caller's own active key and mints a new
	// one in one transaction, scoped to userID. The new key is built ENTIRELY
	// from the explicit newKey argument: newKey.PHCHash (the new secret),
	// newKey.RoleDowngrade (already floored by the handler), and
	// newKey.ExpiresAt (already capped by the handler). It NEVER inherits the
	// old key's role_downgrade or expires_at (prevents re-pinning a stale
	// higher ceiling, T-02-06). Returns the populated new key (no plaintext),
	// or ErrAPIKeyNotFound if the old key is foreign/missing/already-revoked.
	RotateAPIKeyForUser(ctx context.Context, userID, keyID string, newKey *APIKey) (*APIKey, error)

	// CreateAPIKeyForUser mints a key for key.UserID under a quota-safe
	// transaction: it locks the parent users row FOR UPDATE to serialize a
	// single user's concurrent mints, counts the user's active keys, and
	// rejects with ErrQuotaExceeded when the active count is already >= quota
	// (closes the quota TOCTOU race, T-02-05). The caller sets key.PHCHash;
	// storage assigns the prefix and inserts, returning the populated *APIKey.
	CreateAPIKeyForUser(ctx context.Context, key *APIKey, quota int) (*APIKey, error)

	// CountActiveAPIKeys returns the number of the user's keys that are
	// neither revoked nor expired (as of now). Standalone count, outside any
	// quota transaction.
	CountActiveAPIKeys(ctx context.Context, userID string) (int, error)

	// --- OIDC binding CRUD ---

	// JITCreateHuman creates a Human + its OIDCBinding atomically. Used by
	// the OIDC resolver when a binding lookup misses. On (issuer, subject)
	// uniqueness violation, returns the existing user via a re-lookup
	// (race-safe).
	JITCreateHuman(ctx context.Context, u *User, binding *OIDCBinding) (*User, *OIDCBinding, error)

	// ListOIDCBindings returns bindings for the given user.
	ListOIDCBindings(ctx context.Context, userID string) ([]*OIDCBinding, error)

	// UnbindOIDC removes the given binding. Last-credential protection is
	// the caller's responsibility (handler-level policy, not storage).
	UnbindOIDC(ctx context.Context, bindingID string) error
}

// ListUsersFilter narrows ListUsers results.
type ListUsersFilter struct {
	Kind           Kind   // empty = all kinds
	Role           string // empty = all roles
	IncludeDeleted bool
	CreatedAfter   *time.Time
	Limit          int // <= 0 = default (100); values above the store max are capped
	Offset         int
}

// ListAPIKeysFilter narrows ListAPIKeys results.
type ListAPIKeysFilter struct {
	UserID         string // empty = all users (admin)
	IncludeRevoked bool
	Limit          int // <= 0 = default (100); values above the store max are capped
	Offset         int
}
