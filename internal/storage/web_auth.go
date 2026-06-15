// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// WebAuthStore persists interactive-login web sessions and short-lived OAuth2
// login-flow handshake state. Kept separate from UsersBackend so existing
// UsersBackend implementations/fakes are unaffected. Implemented by
// *postgres.AuthStore.
type WebAuthStore interface {
	// --- sessions ---

	// CreateSession inserts a new session row. s.ExpiresAt must be set.
	CreateSession(ctx context.Context, s *Session) (*Session, error)

	// LookupSessionByHash returns the session with the given token hash.
	// Returns ErrSessionNotFound on miss. Does NOT filter expired/revoked —
	// the caller (resolver) gates on Session.IsActive.
	LookupSessionByHash(ctx context.Context, tokenHash []byte) (*Session, error)

	// RevokeSession marks the session revoked by token hash. Idempotent.
	RevokeSession(ctx context.Context, tokenHash []byte) error

	// DeleteExpiredSessions removes expired/long-revoked rows; returns count.
	DeleteExpiredSessions(ctx context.Context) (int64, error)

	// --- login-flow state ---

	// CreateLoginFlow inserts a flow row and returns its opaque id (the PK).
	CreateLoginFlow(ctx context.Context, f *LoginFlow) (flowID string, err error)

	// ConsumeLoginFlow atomically deletes and returns the flow for flowID.
	// Returns ErrLoginFlowNotFound if the id is unknown or already expired.
	ConsumeLoginFlow(ctx context.Context, flowID string) (*LoginFlow, error)

	// DeleteExpiredLoginFlows removes expired flow rows; returns count.
	DeleteExpiredLoginFlows(ctx context.Context) (int64, error)

	// --- CLI one-time login codes ---

	// CreateCLICode inserts a one-time CLI login code. codeHash is the SHA-256
	// of the opaque code; the raw code never reaches storage.
	CreateCLICode(ctx context.Context, codeHash []byte, userID, subject, challenge string, expiresAt time.Time) error

	// ExchangeCLICode atomically consumes an unexpired code and mints a session
	// in one transaction. gotChallenge is S256(verifier) precomputed by the
	// caller; it is constant-time compared against the stored challenge.
	// sess must carry TokenHash and ExpiresAt; UserID/OIDCSubject/ID/CreatedAt
	// are filled from the consumed code and the inserted row.
	// Returns ErrCLICodeNotFound (unknown/expired), ErrCLIChallengeMismatch
	// (PKCE mismatch), or ErrUserNotFound (user soft-deleted mid-flow).
	ExchangeCLICode(ctx context.Context, codeHash []byte, sess *Session, gotChallenge string) (*Session, error)

	// DeleteExpiredCLICodes removes expired code rows; returns count.
	DeleteExpiredCLICodes(ctx context.Context) (int64, error)
}
