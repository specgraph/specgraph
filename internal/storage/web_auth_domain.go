// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import "time"

// Session is a SpecGraph-issued opaque web session minted by the interactive
// OIDC login flow. The raw token is never stored — only its SHA-256 hash.
type Session struct {
	ID          string
	TokenHash   []byte // SHA-256 of the opaque session token
	UserID      string
	Issuer      string // OIDC issuer (audit)
	OIDCSubject string // audit
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time // nil = active
}

// IsActive reports whether the session is neither revoked nor expired as of now.
func (s *Session) IsActive(now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	return s.ExpiresAt.After(now)
}

// LoginFlow is server-side OAuth2 handshake state for one interactive login
// attempt. The opaque flow id (ID) is carried in the short-lived tx cookie;
// state/nonce/code_verifier never leave the server.
type LoginFlow struct {
	ID           string // opaque flow id (tx cookie value)
	State        string // CSRF token, constant-time compared at callback
	Nonce        string
	CodeVerifier string // PKCE
	ProviderID   string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
