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

// CreateHuman inserts a Human row and (optionally) an OIDCBinding in one tx.
// Returns ErrBootstrapExists if u.Bootstrap is true and another active
// bootstrap user already exists (caught via the partial unique index).
func (s *AuthStore) CreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, error) {
	if u.Kind != "" && u.Kind != storage.KindHuman {
		return nil, errors.New("CreateHuman: u.Kind must be KindHuman or empty")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const insertUser = `
		INSERT INTO users (kind, display_name, email, role, bootstrap)
		VALUES ('human', $1, $2, $3, $4)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err = tx.QueryRow(ctx, insertUser, u.DisplayName, u.Email, u.Role, u.Bootstrap).
		Scan(&id, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" /* unique_violation */ &&
			pgErr.ConstraintName == "users_one_bootstrap" {
			return nil, storage.ErrBootstrapExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if b != nil {
		const insertBinding = `
			INSERT INTO oidc_bindings (user_id, issuer, subject, email_at_bind)
			VALUES ($1::uuid, $2, $3, $4)`
		_, err = tx.Exec(ctx, insertBinding, id, b.Issuer, b.Subject, b.EmailAtBind)
		if err != nil {
			return nil, fmt.Errorf("insert binding: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	u.ID = id
	u.Kind = storage.KindHuman
	u.CreatedAt = createdAt
	return u, nil
}

// CreateServiceAccount inserts a ServiceAccount row. OwnerUserID must
// reference an existing user (FK enforced).
func (s *AuthStore) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	if u.OwnerUserID == "" {
		return nil, fmt.Errorf("CreateServiceAccount: OwnerUserID required: %w", storage.ErrUserNotFound)
	}
	const q = `
		INSERT INTO users (kind, display_name, email, role, owner_user_id)
		VALUES ('service_account', $1, $2, $3, $4::uuid)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, u.DisplayName, u.Email, u.Role, u.OwnerUserID).
		Scan(&id, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("insert service account: %w", storage.ErrUserNotFound)
		}
		return nil, fmt.Errorf("insert service account: %w", err)
	}
	u.ID = id
	u.Kind = storage.KindServiceAccount
	u.CreatedAt = createdAt
	return u, nil
}

// UpdateUserRole sets the role on an active user. Returns ErrUserNotFound
// if no active user has the given ID.
func (s *AuthStore) UpdateUserRole(ctx context.Context, userID, role string) error {
	const q = `
		UPDATE users SET role = $1
		WHERE id = $2::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, role, userID)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}

// SoftDeleteUser sets deleted_at on the user and revokes all their active
// keys in the same transaction. Idempotent on already-deleted users (the
// user UPDATE matches zero rows, the keys UPDATE matches zero rows; both
// silently succeed).
func (s *AuthStore) SoftDeleteUser(ctx context.Context, userID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := s.now()

	_, err = tx.Exec(ctx, `
		UPDATE users SET deleted_at = $1
		WHERE id = $2::uuid AND deleted_at IS NULL`, now, userID)
	if err != nil {
		return fmt.Errorf("soft-delete user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE user_id = $2::uuid AND revoked_at IS NULL`, now, userID)
	if err != nil {
		return fmt.Errorf("revoke keys: %w", err)
	}

	return tx.Commit(ctx)
}

// PurgeUser hard-deletes the user; CASCADE constraints handle bindings
// and keys. Idempotent on already-purged users.
func (s *AuthStore) PurgeUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	if err != nil {
		return fmt.Errorf("purge user: %w", err)
	}
	return nil
}

// ListUsers returns users matching the filter. Default limit is 100.
func (s *AuthStore) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	q := `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users WHERE 1=1`
	args := []any{}
	if !f.IncludeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	if f.Kind != "" {
		args = append(args, string(f.Kind))
		q += fmt.Sprintf(` AND kind = $%d`, len(args))
	}
	if f.Role != "" {
		args = append(args, f.Role)
		q += fmt.Sprintf(` AND role = $%d`, len(args))
	}
	if f.CreatedAfter != nil {
		args = append(args, *f.CreatedAfter)
		q += fmt.Sprintf(` AND created_at > $%d`, len(args))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))
	args = append(args, f.Offset)
	q += fmt.Sprintf(` OFFSET $%d`, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []*storage.User
	for rows.Next() {
		var u storage.User
		var kindStr string
		err := rows.Scan(&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
			&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt)
		if err != nil {
			return nil, err
		}
		u.Kind = storage.Kind(kindStr)
		out = append(out, &u)
	}
	return out, rows.Err()
}
