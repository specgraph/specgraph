// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/storage"
)

// CreateSession inserts a web session row, returning the generated id and
// created_at.
func (s *AuthStore) CreateSession(ctx context.Context, sess *storage.Session) (*storage.Session, error) {
	if len(sess.TokenHash) == 0 {
		return nil, errors.New("CreateSession: TokenHash required")
	}
	if sess.UserID == "" {
		return nil, errors.New("CreateSession: UserID required")
	}
	if sess.ExpiresAt.IsZero() {
		return nil, errors.New("CreateSession: ExpiresAt required")
	}
	const q = `
		INSERT INTO web_sessions (token_hash, user_id, issuer, oidc_subject, expires_at)
		SELECT $1, $2::uuid, $3, $4, $5
		WHERE EXISTS (SELECT 1 FROM users WHERE id = $2::uuid AND deleted_at IS NULL)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, sess.TokenHash, sess.UserID, sess.Issuer, sess.OIDCSubject, sess.ExpiresAt).
		Scan(&id, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("create session: %w", storage.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	sess.ID = id
	sess.CreatedAt = createdAt
	return sess, nil
}

// LookupSessionByHash returns the session for the given token hash.
func (s *AuthStore) LookupSessionByHash(ctx context.Context, tokenHash []byte) (*storage.Session, error) {
	const q = `
		SELECT id, token_hash, user_id, issuer, oidc_subject, created_at, expires_at, revoked_at
		FROM web_sessions
		WHERE token_hash = $1`
	row := s.pool.QueryRow(ctx, q, tokenHash)
	var sess storage.Session
	err := row.Scan(&sess.ID, &sess.TokenHash, &sess.UserID, &sess.Issuer,
		&sess.OIDCSubject, &sess.CreatedAt, &sess.ExpiresAt, &sess.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return &sess, nil
}

// RevokeSession marks the session revoked by token hash. Idempotent.
func (s *AuthStore) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE web_sessions SET revoked_at = $1
		WHERE token_hash = $2 AND revoked_at IS NULL`, s.now(), tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired rows. Revoked rows are left to expire
// naturally so a revoked id never silently reappears as "not found vs revoked".
func (s *AuthStore) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM web_sessions WHERE expires_at <= $1`, s.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CreateLoginFlow inserts a flow row and returns its opaque id.
func (s *AuthStore) CreateLoginFlow(ctx context.Context, f *storage.LoginFlow) (string, error) {
	if f.State == "" || f.Nonce == "" || f.CodeVerifier == "" || f.ProviderID == "" {
		return "", errors.New("CreateLoginFlow: state, nonce, code_verifier, provider_id required")
	}
	if f.ExpiresAt.IsZero() {
		return "", errors.New("CreateLoginFlow: ExpiresAt required")
	}
	const q = `
		INSERT INTO oidc_login_flows (state, nonce, code_verifier, provider_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	var id string
	if err := s.pool.QueryRow(ctx, q, f.State, f.Nonce, f.CodeVerifier, f.ProviderID, f.ExpiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("create login flow: %w", err)
	}
	return id, nil
}

// ConsumeLoginFlow atomically deletes and returns the (unexpired) flow.
func (s *AuthStore) ConsumeLoginFlow(ctx context.Context, flowID string) (*storage.LoginFlow, error) {
	const q = `
		DELETE FROM oidc_login_flows
		WHERE id = $1::uuid AND expires_at > $2
		RETURNING id, state, nonce, code_verifier, provider_id, created_at, expires_at`
	row := s.pool.QueryRow(ctx, q, flowID, s.now())
	var f storage.LoginFlow
	err := row.Scan(&f.ID, &f.State, &f.Nonce, &f.CodeVerifier, &f.ProviderID, &f.CreatedAt, &f.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrLoginFlowNotFound
	}
	// A malformed flowID (not a uuid) yields a 22P02 invalid_text_representation
	// cast error; map only that to not-found so the handler renders
	// auth_error=expired. Genuine DB errors (outage, pool exhaustion) propagate
	// so the caller can treat them as transient — mirroring LookupSessionByHash.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return nil, storage.ErrLoginFlowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume login flow: %w", err)
	}
	return &f, nil
}

// DeleteExpiredLoginFlows removes expired flow rows.
func (s *AuthStore) DeleteExpiredLoginFlows(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM oidc_login_flows WHERE expires_at <= $1`, s.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired login flows: %w", err)
	}
	return tag.RowsAffected(), nil
}
