// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package postgres implements storage backends using PostgreSQL.
package postgres

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertions.
var (
	_ storage.ScopedBackend = (*Store)(nil)
	_ storage.Scoper        = (*Store)(nil)
	_ storage.Subscribable  = (*Store)(nil)
)

// sharedState holds state shared between root and scoped Store instances.
type sharedState struct {
	subscribers []storage.ChangeSubscriber
}

// Store implements storage backends using PostgreSQL via pgx.
type Store struct {
	pool     *pgxpool.Pool
	nowFunc  func() time.Time
	sliceOps storage.SliceBackend
	project  string
	ownsPool bool
	shared   *sharedState
}

// Option configures a Store.
type Option func(*Store)

// WithClock overrides the default wall clock used for timestamps.
// Intended for testing — production callers should omit this option.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.nowFunc = fn }
}

// WithProject sets the project slug for namespacing.
// Required — New() returns an error if no project is set.
func WithProject(slug string) Option {
	return func(s *Store) { s.project = slug }
}

// WithSliceOps overrides the default SliceBackend used by StoreDecomposeOutput.
// Intended for testing — production callers should omit this option.
func WithSliceOps(ops storage.SliceBackend) Option {
	return func(s *Store) { s.sliceOps = ops }
}

// New creates a new PostgreSQL-backed Store, runs migrations, and ensures the
// project row exists.
func New(ctx context.Context, connString string, opts ...Option) (*Store, error) {
	s := &Store{nowFunc: time.Now, ownsPool: true, shared: &sharedState{}}
	for _, o := range opts {
		o(s)
	}

	if s.project == "" {
		return nil, fmt.Errorf("postgres: project slug required: use postgres.WithProject(slug)")
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: verify connectivity: %w", err)
	}

	if err := runMigrations(connString); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: run migrations: %w", err)
	}

	s.pool = pool

	if err := s.ensureProjectRow(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	// sliceOps defaults to self once all interface methods are implemented.
	// Set externally via WithSliceOps until then.

	return s, nil
}

// Ping verifies Postgres connectivity through the connection pool. The
// wrapping is load-bearing: callers surface the message verbatim, and
// unwrapped pgxpool errors are too opaque to diagnose in isolation.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}

// ensureProjectRow inserts the project row idempotently.
func (s *Store) ensureProjectRow(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO projects (slug) VALUES ($1) ON CONFLICT (slug) DO NOTHING`,
		s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: ensure project row: %w", err)
	}
	return nil
}

// ClearAll truncates all data tables and re-ensures the project row.
// Intended for test isolation — not for production use.
func (s *Store) ClearAll(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `TRUNCATE
		sync_mappings, constitutions, execution_events, claims,
		conversation_logs, findings, changelog_entries, edges,
		slices, decisions, specs, projects
		CASCADE`)
	if err != nil {
		return fmt.Errorf("postgres: clear all: %w", err)
	}
	return s.ensureProjectRow(ctx)
}

// Close releases the connection pool if this Store owns it.
func (s *Store) Close(_ context.Context) error {
	if s.ownsPool && s.pool != nil {
		s.pool.Close()
	}
	return nil
}

// Scoped returns a new Store sharing this Store's pool but targeting a
// different project. The project row is ensured before returning.
func (s *Store) Scoped(ctx context.Context, project string) (storage.ScopedBackend, error) {
	if project == "" {
		return nil, fmt.Errorf("postgres: project slug required")
	}
	scoped := &Store{
		pool:     s.pool,
		nowFunc:  s.nowFunc,
		project:  project,
		ownsPool: false,
		shared:   s.shared,
	}
	// sliceOps defaults to self once all interface methods are implemented.
	if err := scoped.ensureProjectRow(ctx); err != nil {
		return nil, err
	}
	return scoped, nil
}

// Subscribe registers a subscriber for change notifications.
// Must be called before any writes (at startup). Not goroutine-safe.
func (s *Store) Subscribe(sub storage.ChangeSubscriber) {
	s.shared.subscribers = append(s.shared.subscribers, sub)
}

// now returns the current UTC time from the Store's clock.
func (s *Store) now() time.Time {
	return s.nowFunc().UTC()
}

// newID produces a prefixed ULID: prefix + "-" + ULID.
// ULIDs are 128-bit and lexicographically sortable by timestamp.
func newID(prefix string) string {
	return prefix + "-" + ulid.MustNew(ulid.Now(), rand.Reader).String()
}
