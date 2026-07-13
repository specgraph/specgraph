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

// maxPrefixRetries bounds prefix-collision regeneration before returning
// ErrAPIKeyPrefixExists. With ~40 bits of prefix entropy a collision is
// astronomically unlikely, so 3 attempts is ample.
const maxPrefixRetries = 3

// defaultListLimit is applied when a list filter's Limit is <= 0.
// maxListLimit caps an over-large caller-supplied Limit so an admin passing
// a huge value cannot trigger an unbounded fetch. Both ListUsers and
// ListAPIKeys clamp to [1, maxListLimit] via clampListLimit.
const (
	defaultListLimit = 100
	maxListLimit     = 1000
)

// clampListLimit normalizes a caller-supplied list limit: a non-positive
// limit becomes defaultListLimit; a limit above maxListLimit is capped.
func clampListLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

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
		return nil, fmt.Errorf("scan api key: %w", err)
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
		return nil, fmt.Errorf("scan oidc binding: %w", err)
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
		return nil, fmt.Errorf("scan user: %w", err)
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
		return nil, fmt.Errorf("scan user: %w", err)
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
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

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
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	u.ID = id
	u.Kind = storage.KindHuman
	u.CreatedAt = createdAt
	return u, nil
}

// CreateServiceAccount inserts a ServiceAccount row. OwnerUserID must
// reference an existing, active (not soft-deleted) Human user; the check
// is enforced atomically by the INSERT … SELECT so there is no TOCTOU window.
func (s *AuthStore) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	if u.OwnerUserID == "" {
		return nil, fmt.Errorf("CreateServiceAccount: OwnerUserID required: %w", storage.ErrUserNotFound)
	}
	// The INSERT … SELECT enforces that the owner exists, is a human, and is
	// not soft-deleted. If the WHERE EXISTS sub-select matches no row (owner
	// missing, deleted, or not a human), QueryRow returns pgx.ErrNoRows.
	const q = `
		INSERT INTO users (kind, display_name, email, role, owner_user_id)
		SELECT 'service_account', $1, $2, $3, $4::uuid
		WHERE EXISTS (
			SELECT 1 FROM users
			WHERE id = $4::uuid AND kind = 'human' AND deleted_at IS NULL
		)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, u.DisplayName, u.Email, u.Role, u.OwnerUserID).
		Scan(&id, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("insert service account: %w", storage.ErrUserNotFound)
	}
	if err != nil {
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

// UpdateUserOnLogin sets display_name, email, and role on an active user in a
// single statement. Returns ErrUserNotFound if no active user has the given ID.
func (s *AuthStore) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	const q = `
		UPDATE users SET display_name = $1, email = $2, role = $3
		WHERE id = $4::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, displayName, email, role, userID)
	if err != nil {
		return fmt.Errorf("update user on login: %w", err)
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
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
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

// ListUsers returns users matching the filter. The limit is clamped to
// [1, maxListLimit]: a non-positive Limit defaults to defaultListLimit (100),
// and a Limit above maxListLimit (1000) is capped.
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
	limit := clampListLimit(f.Limit)
	args = append(args, limit)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))
	args = append(args, f.Offset)
	q += fmt.Sprintf(` OFFSET $%d`, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	out := make([]*storage.User, 0)
	for rows.Next() {
		var u storage.User
		var kindStr string
		err := rows.Scan(&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
			&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Kind = storage.Kind(kindStr)
		out = append(out, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// CreateAPIKey inserts a new API key with a generated prefix. Retries up to
// 3 times on prefix-uniqueness violation; returns ErrAPIKeyPrefixExists if
// all retries collide (essentially impossible at 40 bits of entropy).
//
// The plaintext prefix and secret are NOT taken from the caller; the
// caller passes only metadata (UserID, PHCHash, RoleDowngrade, Label,
// ExpiresAt). Prefix is generated via s.genPrefix (overridable per-
// instance via WithAuthKeyPrefixGenerator for tests; never package-global).
func (s *AuthStore) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	if k.UserID == "" {
		return nil, errors.New("CreateAPIKey: UserID required")
	}
	if k.PHCHash == "" {
		return nil, errors.New("CreateAPIKey: PHCHash required")
	}
	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, err := s.genPrefix()
		if err != nil {
			return nil, fmt.Errorf("generate prefix: %w", err)
		}
		// INSERT … SELECT guards that the user exists and is not soft-deleted.
		// Both humans and service accounts may hold keys; only deleted_at IS NULL
		// matters. If the WHERE EXISTS sub-select matches no row, QueryRow
		// returns pgx.ErrNoRows — break out immediately (do NOT retry).
		const q = `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			SELECT $1::uuid, $2, $3, $4, $5, $6
			WHERE EXISTS (
				SELECT 1 FROM users WHERE id = $1::uuid AND deleted_at IS NULL
			)
			RETURNING id, created_at`
		var id string
		var createdAt time.Time
		err = s.pool.QueryRow(ctx, q, k.UserID, prefix, k.PHCHash, k.RoleDowngrade, k.Label, k.ExpiresAt).
			Scan(&id, &createdAt)
		if err == nil {
			k.ID = id
			k.Prefix = prefix
			k.CreatedAt = createdAt
			return k, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("insert api key: %w", storage.ErrUserNotFound)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue // retry with new prefix
		}
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}

// RevokeAPIKey marks the key revoked. Idempotent on already-revoked or
// nonexistent IDs (does not error).
func (s *AuthStore) RevokeAPIKey(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), keyID)
	if err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}
	return nil
}

// RotateAPIKey revokes the old key and inserts a new one in one transaction.
// Rotation preserves the key's identity and authority: owner (user_id),
// role_downgrade, and label are always inherited from the old key (never the
// caller's newKey values). The caller supplies newKey.PHCHash (the new secret)
// and MAY supply newKey.ExpiresAt to set the new secret's validity window; a
// nil ExpiresAt inherits the old key's expiry (fail-safe — never silently
// clears it). Returns the new key with generated prefix and ID populated.
func (s *AuthStore) RotateAPIKey(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	// Read the old key's metadata first so the new key inherits owner +
	// metadata, not the potentially bogus values from the caller.
	var oldUserID, oldRoleDowngrade, oldLabel string
	var oldExpiresAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT user_id, coalesce(role_downgrade, ''), coalesce(label, ''), expires_at
		FROM api_keys
		WHERE id = $1::uuid AND revoked_at IS NULL`, oldKeyID).
		Scan(&oldUserID, &oldRoleDowngrade, &oldLabel, &oldExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read old key: %w", err)
	}

	// Revoke the old key now that we know it exists and is active.
	_, err = tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), oldKeyID)
	if err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}

	// Expiry is the one attribute the caller may set on rotation: it describes
	// the freshness of the NEW secret. Fail-safe semantics: a non-nil
	// newKey.ExpiresAt overrides; a nil one inherits the old expiry and never
	// silently clears it (clearing would relax a security constraint).
	expiresAt := oldExpiresAt
	if newKey.ExpiresAt != nil {
		expiresAt = newKey.ExpiresAt
	}

	// Insert new key inside the same tx with collision retry.
	// Use savepoints to roll back only the failed INSERT on collision,
	// keeping the revoke above intact. Postgres aborts the whole tx on
	// any error without a savepoint, so we need one per attempt.
	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, err := s.genPrefix()
		if err != nil {
			return nil, fmt.Errorf("generate prefix: %w", err)
		}
		if _, spErr := tx.Exec(ctx, `SAVEPOINT rotate_insert`); spErr != nil {
			return nil, fmt.Errorf("savepoint: %w", spErr)
		}
		var id string
		var createdAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			oldUserID, prefix, newKey.PHCHash, oldRoleDowngrade,
			oldLabel, expiresAt).Scan(&id, &createdAt)
		if err == nil {
			// Explicitly release the savepoint so its lifecycle is balanced
			// (Commit would auto-release, but the explicit RELEASE keeps a
			// future in-tx edit from leaving an orphaned savepoint).
			if _, relErr := tx.Exec(ctx, `RELEASE SAVEPOINT rotate_insert`); relErr != nil {
				return nil, fmt.Errorf("release savepoint: %w", relErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit tx: %w", commitErr)
			}
			out := &storage.APIKey{
				ID:            id,
				UserID:        oldUserID,
				Prefix:        prefix,
				PHCHash:       newKey.PHCHash,
				RoleDowngrade: oldRoleDowngrade,
				Label:         oldLabel,
				ExpiresAt:     expiresAt,
				CreatedAt:     createdAt,
			}
			return out, nil
		}
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT rotate_insert`); rbErr != nil {
			return nil, fmt.Errorf("rollback savepoint: %w", rbErr)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert new key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}

// GetAPIKeyForUser returns the caller's own key, scoped by user_id so a
// foreign or missing key is uniformly ErrAPIKeyNotFound (T-02-04). Includes
// revoked keys — the caller (rotate at the handler) gates on IsActive.
func (s *AuthStore) GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*storage.APIKey, error) {
	const q = `
		SELECT id, user_id, prefix, phc_hash, role_downgrade, label,
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE id = $1::uuid AND user_id = $2::uuid`
	row := s.pool.QueryRow(ctx, q, keyID, userID)

	var k storage.APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Prefix, &k.PHCHash, &k.RoleDowngrade, &k.Label,
		&k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan api key: %w", err)
	}
	return &k, nil
}

// RevokeAPIKeyForUser marks the caller's own key revoked. Unlike the admin
// RevokeAPIKey (which uses a `revoked_at IS NULL` guard and is silently
// idempotent with no RowsAffected check), the owner-scoped variant scopes on
// user_id and DROPS the `revoked_at IS NULL` guard so re-revoking your OWN key
// is an idempotent no-op success (Finding F4). A RowsAffected()==0 result then
// means the key is foreign or missing — returned as ErrAPIKeyNotFound so
// "not yours"/"missing" are indistinguishable (T-02-04).
func (s *AuthStore) RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = coalesce(revoked_at, $1)
		WHERE id = $2::uuid AND user_id = $3::uuid`, s.now(), keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke key for user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrAPIKeyNotFound
	}
	return nil
}

// RotateAPIKeyForUser revokes the caller's own active key and inserts a new one
// in one transaction, scoped by user_id. The new key is built ENTIRELY from the
// explicit newKey argument — newKey.PHCHash, newKey.RoleDowngrade (floored by
// the handler), and newKey.ExpiresAt (capped by the handler). It NEVER inherits
// the old key's role_downgrade or expires_at, so a rotate can never re-pin a
// stale higher ceiling (T-02-06). Owner is taken from the scoped userID. A
// foreign/missing/already-revoked old key returns ErrAPIKeyNotFound.
func (s *AuthStore) RotateAPIKeyForUser(ctx context.Context, userID, keyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	// Scope the old-key read/revoke with AND user_id AND revoked_at IS NULL so
	// a foreign, missing, or already-revoked key is uniformly NotFound. We do
	// not read role_downgrade/expires_at — the new key uses ONLY the explicit
	// newKey values (T-02-06).
	tag, err := tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND user_id = $3::uuid AND revoked_at IS NULL`,
		s.now(), keyID, userID)
	if err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, storage.ErrAPIKeyNotFound
	}

	// Insert the new key inside the same tx with savepoint-guarded prefix
	// collision retry, mirroring RotateAPIKey. Every attribute comes from the
	// explicit newKey; owner is the scoped userID.
	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, genErr := s.genPrefix()
		if genErr != nil {
			return nil, fmt.Errorf("generate prefix: %w", genErr)
		}
		if _, spErr := tx.Exec(ctx, `SAVEPOINT rotate_self_insert`); spErr != nil {
			return nil, fmt.Errorf("savepoint: %w", spErr)
		}
		var id string
		var createdAt time.Time
		insErr := tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			userID, prefix, newKey.PHCHash, newKey.RoleDowngrade,
			newKey.Label, newKey.ExpiresAt).Scan(&id, &createdAt)
		if insErr == nil {
			if _, relErr := tx.Exec(ctx, `RELEASE SAVEPOINT rotate_self_insert`); relErr != nil {
				return nil, fmt.Errorf("release savepoint: %w", relErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit tx: %w", commitErr)
			}
			return &storage.APIKey{
				ID:            id,
				UserID:        userID,
				Prefix:        prefix,
				PHCHash:       newKey.PHCHash,
				RoleDowngrade: newKey.RoleDowngrade,
				Label:         newKey.Label,
				ExpiresAt:     newKey.ExpiresAt,
				CreatedAt:     createdAt,
			}, nil
		}
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT rotate_self_insert`); rbErr != nil {
			return nil, fmt.Errorf("rollback savepoint: %w", rbErr)
		}
		var pgErr *pgconn.PgError
		if errors.As(insErr, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert new key: %w", insErr)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}

// activeKeyCountQuery counts a user's keys that are neither revoked nor
// expired (as of now()). Shared by CreateAPIKeyForUser (inside the quota tx)
// and CountActiveAPIKeys (standalone) so the "active" definition is identical.
const activeKeyCountQuery = `
	SELECT count(*) FROM api_keys
	WHERE user_id = $1::uuid AND revoked_at IS NULL
	  AND (expires_at IS NULL OR expires_at > now())`

// CreateAPIKeyForUser mints a key under a quota-safe transaction. It locks the
// parent users row FOR UPDATE to serialize a single user's concurrent mints
// (closing the quota TOCTOU race, T-02-05), counts the user's active keys, and
// rejects with ErrQuotaExceeded when the active count is already >= quota. The
// caller supplies key.PHCHash; storage assigns the prefix and inserts. Postgres
// forbids a row-locking clause on an aggregate query, so the lock is taken on
// the parent users row instead — the count then runs inside the serialized
// window.
func (s *AuthStore) CreateAPIKeyForUser(ctx context.Context, key *storage.APIKey, quota int) (*storage.APIKey, error) {
	if key.UserID == "" {
		return nil, errors.New("CreateAPIKeyForUser: UserID required")
	}
	if key.PHCHash == "" {
		return nil, errors.New("CreateAPIKeyForUser: PHCHash required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	// Lock the parent users row (active only) to serialize this user's mints.
	// A missing/soft-deleted user matches zero rows → ErrUserNotFound.
	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM users
		WHERE id = $1::uuid AND deleted_at IS NULL
		FOR UPDATE`, key.UserID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("lock user: %w", storage.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lock user row: %w", err)
	}

	// Count active keys inside the serialized window. No concurrent mint for
	// this user can commit between this count and our INSERT (they block on
	// the users-row lock above), so the quota check is race-free.
	var n int
	if err := tx.QueryRow(ctx, activeKeyCountQuery, key.UserID).Scan(&n); err != nil {
		return nil, fmt.Errorf("count active keys: %w", err)
	}
	if n >= quota {
		return nil, storage.ErrQuotaExceeded
	}

	// Insert with a generated prefix + collision retry, reusing the
	// CreateAPIKey INSERT…SELECT…WHERE EXISTS body inside the tx.
	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, genErr := s.genPrefix()
		if genErr != nil {
			return nil, fmt.Errorf("generate prefix: %w", genErr)
		}
		if _, spErr := tx.Exec(ctx, `SAVEPOINT create_self_insert`); spErr != nil {
			return nil, fmt.Errorf("savepoint: %w", spErr)
		}
		var id string
		var createdAt time.Time
		insErr := tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			SELECT $1::uuid, $2, $3, $4, $5, $6
			WHERE EXISTS (
				SELECT 1 FROM users WHERE id = $1::uuid AND deleted_at IS NULL
			)
			RETURNING id, created_at`,
			key.UserID, prefix, key.PHCHash, key.RoleDowngrade, key.Label, key.ExpiresAt).
			Scan(&id, &createdAt)
		if insErr == nil {
			if _, relErr := tx.Exec(ctx, `RELEASE SAVEPOINT create_self_insert`); relErr != nil {
				return nil, fmt.Errorf("release savepoint: %w", relErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit tx: %w", commitErr)
			}
			key.ID = id
			key.Prefix = prefix
			key.CreatedAt = createdAt
			return key, nil
		}
		if errors.Is(insErr, pgx.ErrNoRows) {
			// The users row existed under the lock above, so a no-rows insert
			// here would be a surprise; treat as user-not-found defensively.
			return nil, fmt.Errorf("insert api key: %w", storage.ErrUserNotFound)
		}
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT create_self_insert`); rbErr != nil {
			return nil, fmt.Errorf("rollback savepoint: %w", rbErr)
		}
		var pgErr *pgconn.PgError
		if errors.As(insErr, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert api key: %w", insErr)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}

// CountActiveAPIKeys returns the number of the user's keys that are neither
// revoked nor expired (as of now). Standalone read, outside any quota tx.
func (s *AuthStore) CountActiveAPIKeys(ctx context.Context, userID string) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, activeKeyCountQuery, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active keys: %w", err)
	}
	return n, nil
}

// ListAPIKeys returns keys matching the filter. The limit is clamped to
// [1, maxListLimit] with the same semantics as ListUsers.
func (s *AuthStore) ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	q := `
		SELECT id, user_id, prefix, phc_hash, role_downgrade, label,
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys WHERE 1=1`
	args := []any{}
	if f.UserID != "" {
		args = append(args, f.UserID)
		q += fmt.Sprintf(` AND user_id = $%d::uuid`, len(args))
	}
	if !f.IncludeRevoked {
		q += ` AND revoked_at IS NULL`
	}
	limit := clampListLimit(f.Limit)
	args = append(args, limit, f.Offset)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	out := make([]*storage.APIKey, 0)
	for rows.Next() {
		var k storage.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Prefix, &k.PHCHash, &k.RoleDowngrade,
			&k.Label, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		out = append(out, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// TouchLastUsed sets last_used_at = now() for the key. Nonexistent or
// already-revoked IDs are silent no-ops (fire-and-forget semantics).
// Excluding revoked keys prevents audit-log confusion when a key is
// revoked between a successful verify and the async last-used update.
func (s *AuthStore) TouchLastUsed(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), keyID)
	if err != nil {
		return fmt.Errorf("touch last_used_at: %w", err)
	}
	return nil
}

// JITCreateHuman creates a Human + OIDC binding atomically. If the (issuer,
// subject) already exists (race with another JIT), re-reads and returns the
// winning binding's user.
//
// Race recovery semantics: when a concurrent JIT wins the (issuer, subject)
// uniqueness contest, this caller's INSERT receives a 23505. Postgres
// guarantees the winner has already COMMITTED before the loser's INSERT can
// observe the constraint violation (the winner's tx held a row lock until
// commit, after which the loser's insert attempt resolves to "duplicate
// key"). At READ COMMITTED isolation, the loser's subsequent SELECT for
// the binding will see the committed row. No retry loop is needed. The user
// row inserted before the failed binding INSERT is discarded by the
// transaction rollback, so no orphan user accrues.
func (s *AuthStore) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	if u.Kind != "" && u.Kind != storage.KindHuman {
		return nil, nil, fmt.Errorf("JITCreateHuman: u.Kind must be KindHuman or empty, got %q", u.Kind)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	var userID string
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO users (kind, display_name, email, role)
		VALUES ('human', $1, $2, $3)
		RETURNING id, created_at`,
		u.DisplayName, u.Email, u.Role).Scan(&userID, &createdAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert user (jit): %w", err)
	}

	var bindingID string
	var bindingCreatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO oidc_bindings (user_id, issuer, subject, email_at_bind)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id, created_at`,
		userID, b.Issuer, b.Subject, b.EmailAtBind).Scan(&bindingID, &bindingCreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "oidc_bindings_issuer_subject_key" {
			// Race: another JIT won. Rollback, re-read.
			// Explicit rollback discards the just-inserted (uncommitted) user
			// row and frees the connection for the pool re-read below; the
			// deferred Rollback then no-ops on the already-closed tx.
			_ = tx.Rollback(ctx) //nolint:errcheck // best-effort rollback before read-back; tx is already aborted by the constraint violation
			existing, lookupErr := s.LookupOIDCBinding(ctx, b.Issuer, b.Subject)
			if lookupErr != nil {
				return nil, nil, fmt.Errorf("race recovery: %w", lookupErr)
			}
			existingUser, lookupErr := s.GetUserByID(ctx, existing.UserID)
			if lookupErr != nil {
				return nil, nil, fmt.Errorf("race recovery user: %w", lookupErr)
			}
			return existingUser, existing, nil
		}
		return nil, nil, fmt.Errorf("insert binding (jit): %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit tx: %w", err)
	}

	u.ID = userID
	u.Kind = storage.KindHuman
	u.CreatedAt = createdAt
	b.ID = bindingID
	b.UserID = userID
	b.CreatedAt = bindingCreatedAt
	return u, b, nil
}

// ListOIDCBindings returns bindings for the given user.
func (s *AuthStore) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, issuer, subject, email_at_bind, created_at
		FROM oidc_bindings
		WHERE user_id = $1::uuid
		ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list bindings: %w", err)
	}
	defer rows.Close()

	out := make([]*storage.OIDCBinding, 0)
	for rows.Next() {
		var b storage.OIDCBinding
		if err := rows.Scan(&b.ID, &b.UserID, &b.Issuer, &b.Subject, &b.EmailAtBind, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan oidc binding: %w", err)
		}
		out = append(out, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// UnbindOIDC deletes the binding. Idempotent on already-deleted.
func (s *AuthStore) UnbindOIDC(ctx context.Context, bindingID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM oidc_bindings WHERE id = $1::uuid`, bindingID)
	if err != nil {
		return fmt.Errorf("unbind oidc: %w", err)
	}
	return nil
}
