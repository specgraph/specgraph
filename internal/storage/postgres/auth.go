// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	cryptorand "crypto/rand"
	"embed"
	"encoding/base32"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/specgraph/specgraph/internal/storage"
)

//go:embed auth_migrations/*.sql
var authMigrations embed.FS

// Compile-time assertion that *AuthStore implements UsersBackend.
// Mirrors the convention used by *Store for ConstitutionBackend etc.
// Note: this assertion forces every UsersBackend method to exist on
// *AuthStore. Stubs for every method are declared further down in this
// file to satisfy the assertion; real implementations land in tasks 9–26
// which replace the stubs one at a time.
var _ storage.UsersBackend = (*AuthStore)(nil)

// AuthStore is the Postgres implementation of UsersBackend. It is a sibling
// to *Store: shares the database pool, holds no project scope, owns the
// identity tables exclusively.
//
// Pool ownership: *Store owns the pool; *AuthStore borrows it. Callers
// MUST close *AuthStore before *Store at shutdown. Today AuthStore.Close
// is a no-op for the pool, but this ordering rule lets future flush-on-
// shutdown work (usagetracker drain etc.) slot in cleanly.
//
// Migration safety: NewAuth runs auth migrations using a separate goose
// version table (goose_db_version_auth) to avoid colliding with project
// migrations. Goose mutates package-global state (BaseFS, TableName,
// Dialect) so NewAuth and the existing postgres.New MUST NOT be called
// concurrently. The expected pattern is sequential startup: New first,
// then NewAuth, both single-threaded.
type AuthStore struct {
	pool      *pgxpool.Pool
	nowFunc   func() time.Time
	genPrefix func() (string, error)
}

// AuthOption configures an AuthStore.
type AuthOption func(*AuthStore)

// WithAuthClock overrides the wall clock used for explicit mutation
// timestamps (deleted_at, revoked_at). Test-only. Does NOT affect insert
// timestamps that come from SQL DEFAULT now().
func WithAuthClock(fn func() time.Time) AuthOption {
	return func(s *AuthStore) { s.nowFunc = fn }
}

// WithAuthKeyPrefixGenerator overrides the API-key prefix generator.
// Test-only — used to force collision scenarios in CreateAPIKey tests.
// Production code does not call WithAuthKeyPrefixGenerator.
func WithAuthKeyPrefixGenerator(fn func() (string, error)) AuthOption {
	return func(s *AuthStore) { s.genPrefix = fn }
}

// NewAuth constructs an AuthStore wrapping the given pool. The caller
// retains ownership of the pool; AuthStore.Close is a no-op for the pool.
// Auth migrations run inline using a dedicated goose version table.
//
// MUST be called after postgres.New and never concurrently with it
// (goose uses package-global state). See type docstring.
func NewAuth(ctx context.Context, pool *pgxpool.Pool, opts ...AuthOption) (*AuthStore, error) {
	if pool == nil {
		return nil, errors.New("postgres: NewAuth: pool must not be nil")
	}
	s := &AuthStore{
		pool:      pool,
		nowFunc:   time.Now,
		genPrefix: defaultGenerateKeyPrefix,
	}
	for _, o := range opts {
		o(s)
	}

	if err := s.runAuthMigrations(ctx); err != nil {
		return nil, fmt.Errorf("postgres: auth migrations: %w", err)
	}
	return s, nil
}

// runAuthMigrations runs the embedded auth migrations using a dedicated
// goose version table. Goose state is package-global; do not call
// concurrently with the existing runMigrations from migrate.go.
func (s *AuthStore) runAuthMigrations(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(s.pool)
	// stdlib.OpenDBFromPool wraps the pool in a *sql.DB facade; closing
	// the *sql.DB does NOT close the underlying pgxpool. Verified via pgx
	// docs (jackc/pgx#1023) — pool ownership stays with the original
	// caller.
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(authMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetTableName("goose_db_version_auth")
	defer goose.SetTableName("goose_db_version") // restore default for any subsequent caller
	defer goose.SetBaseFS(nil)                   // restore goose's default (OS) filesystem; see concurrency note on the type

	if err := goose.UpContext(ctx, db, "auth_migrations"); err != nil {
		return fmt.Errorf("run auth migrations: %w", err)
	}
	return nil
}

// Close releases AuthStore resources. Today this is a no-op (the pool is
// borrowed; no other resources held). Future flush-on-shutdown work
// (usagetracker drain etc.) will hook here. Callers MUST call Close
// before closing the underlying *Store.
func (s *AuthStore) Close(_ context.Context) error {
	return nil
}

// now returns the wall clock time used for explicit mutation timestamps.
func (s *AuthStore) now() time.Time { return s.nowFunc() }

// --- UsersBackend method stubs ---
// These satisfy the compile-time assertion above; real implementations land
// in tasks 9–26, which replace each stub with a SQL-backed method one at a
// time. Stubs return errors.New("not implemented") so that accidental use
// in tests fails loudly with a recognizable message rather than returning
// a zero value.

func (s *AuthStore) CreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, error) {
	return nil, errors.New("CreateHuman not implemented")
}

func (s *AuthStore) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	return nil, errors.New("CreateServiceAccount not implemented")
}

func (s *AuthStore) UpdateUserRole(ctx context.Context, userID, role string) error {
	return errors.New("UpdateUserRole not implemented")
}

func (s *AuthStore) SoftDeleteUser(ctx context.Context, userID string) error {
	return errors.New("SoftDeleteUser not implemented")
}

func (s *AuthStore) PurgeUser(ctx context.Context, userID string) error {
	return errors.New("PurgeUser not implemented")
}

func (s *AuthStore) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	return nil, errors.New("ListUsers not implemented")
}

func (s *AuthStore) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	return nil, errors.New("CreateAPIKey not implemented")
}

func (s *AuthStore) RevokeAPIKey(ctx context.Context, keyID string) error {
	return errors.New("RevokeAPIKey not implemented")
}

func (s *AuthStore) RotateAPIKey(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	return nil, errors.New("RotateAPIKey not implemented")
}

func (s *AuthStore) ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return nil, errors.New("ListAPIKeys not implemented")
}

func (s *AuthStore) TouchLastUsed(ctx context.Context, keyID string) error {
	return errors.New("TouchLastUsed not implemented")
}

func (s *AuthStore) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	return nil, nil, errors.New("JITCreateHuman not implemented")
}

func (s *AuthStore) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	return nil, errors.New("ListOIDCBindings not implemented")
}

func (s *AuthStore) UnbindOIDC(ctx context.Context, bindingID string) error {
	return errors.New("UnbindOIDC not implemented")
}

// defaultGenerateKeyPrefix produces 8 random URL-safe base32 characters.
// Overridable via WithAuthKeyPrefixGenerator. Per-instance (not package-
// global) so parallel tests do not race.
func defaultGenerateKeyPrefix() (string, error) {
	const prefixLen = 8
	buf := make([]byte, 5) // 5 bytes -> 8 base32 chars
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)[:prefixLen], nil
}
