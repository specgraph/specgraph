// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"time"
)

// Kind discriminates a User row as a Human (OIDC-backed person) or a
// ServiceAccount (machine identity owned by a Human). The kind constrains
// which credentials and ownership relationships are valid (see UsersBackend
// invariants); it does NOT change lifecycle semantics — soft-delete behaves
// identically for both kinds.
type Kind string

const (
	KindHuman          Kind = "human"
	KindServiceAccount Kind = "service_account"
)

// User is the identity row for a Human or ServiceAccount. Single table with
// a Kind discriminator; lifecycle is uniform across kinds.
//
// Invariants enforced at the schema or store layer (see UsersBackend):
//   - ServiceAccounts MUST have OwnerUserID set; Humans MUST NOT.
//   - At most one User has Bootstrap=true AND DeletedAt=nil.
//   - ServiceAccounts MUST NOT have rows in oidc_bindings (store-layer).
type User struct {
	ID          string // uuid
	Kind        Kind
	DisplayName string
	Email       string // nullable; empty when not set
	Role        string // role assignment; built-in or custom (validated against config)
	OwnerUserID string // ServiceAccount owner; empty for Humans
	Bootstrap   bool   // true for the system bootstrap admin
	CreatedAt   time.Time
	DeletedAt   *time.Time // nil = active
}

// IsHuman reports whether the user is a Human kind.
func (u *User) IsHuman() bool { return u.Kind == KindHuman }

// IsServiceAccount reports whether the user is a ServiceAccount kind.
func (u *User) IsServiceAccount() bool { return u.Kind == KindServiceAccount }

// IsActive reports whether the user has not been soft-deleted.
func (u *User) IsActive() bool { return u.DeletedAt == nil }

// OIDCBinding links a User to an external OIDC subject. The (Issuer, Subject)
// pair is globally unique across providers to prevent same-sub collisions
// between IdPs.
type OIDCBinding struct {
	ID          string
	UserID      string
	Issuer      string
	Subject     string
	EmailAtBind string // captured at JIT bind; nullable
	CreatedAt   time.Time
}

// APIKey is an issued credential owned by a single User (Human or
// ServiceAccount). Plaintext is never persisted; only the argon2id PHC
// hash. The prefix is queryable for O(log N) lookup; the secret is
// constant-time-verified against PHCHash.
//
// RoleDowngrade, when set, caps the EffectiveRole at request time to at most
// the named role (subject to the partial-ordering rules in the design).
type APIKey struct {
	ID            string
	UserID        string
	Prefix        string
	PHCHash       string // argon2id PHC-format string
	RoleDowngrade string // empty = no downgrade
	Label         string
	ExpiresAt     *time.Time // nil = no expiry; expired when now >= ExpiresAt
	LastUsedAt    *time.Time // nil = never used
	RevokedAt     *time.Time // nil = active
	CreatedAt     time.Time
}

// IsActive reports whether the key is neither revoked nor expired (as of now).
// The expiry boundary is exclusive: a key is expired once now reaches or
// passes ExpiresAt, so an active key requires now < ExpiresAt.
func (k *APIKey) IsActive(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return false
	}
	return true
}
