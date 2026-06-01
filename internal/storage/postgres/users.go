// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// LookupAPIKeyByPrefix returns the api_keys row whose prefix matches.
func (s *AuthStore) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*storage.APIKey, error) {
	const q = `
		SELECT id, user_id, prefix, phc_hash, role_downgrade, label,
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE prefix = $1`
	row := s.pool.QueryRow(ctx, q, prefix)

	var k storage.APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Prefix, &k.PHCHash, &k.RoleDowngrade, &k.Label,
		&k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// LookupOIDCBinding returns the binding for (issuer, subject).
func (s *AuthStore) LookupOIDCBinding(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
	const q = `
		SELECT id, user_id, issuer, subject, email_at_bind, created_at
		FROM oidc_bindings
		WHERE issuer = $1 AND subject = $2`
	row := s.pool.QueryRow(ctx, q, issuer, subject)

	var b storage.OIDCBinding
	err := row.Scan(&b.ID, &b.UserID, &b.Issuer, &b.Subject, &b.EmailAtBind, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrOIDCBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// GetUserByID returns the user row (including soft-deleted).
func (s *AuthStore) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	const q = `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users
		WHERE id = $1::uuid`
	row := s.pool.QueryRow(ctx, q, id)

	var u storage.User
	var kindStr string
	err := row.Scan(
		&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
		&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Kind = storage.Kind(kindStr)
	return &u, nil
}

// GetBootstrap returns the active bootstrap admin, or ErrUserNotFound.
func (s *AuthStore) GetBootstrap(ctx context.Context) (*storage.User, error) {
	const q = `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users
		WHERE bootstrap = true AND deleted_at IS NULL
		LIMIT 1`
	row := s.pool.QueryRow(ctx, q)

	var u storage.User
	var kindStr string
	err := row.Scan(
		&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
		&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Kind = storage.Kind(kindStr)
	return &u, nil
}
