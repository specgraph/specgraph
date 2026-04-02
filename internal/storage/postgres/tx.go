// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/storage"
)

// txKey is the context key used to thread a pgx.Tx through storage calls.
type txKey struct{}

// txToContext stores a pgx.Tx in the context.
func txToContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// txFromContext retrieves a pgx.Tx from the context, if present.
func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// RunInTransaction executes fn within a single PostgreSQL transaction.
// The transaction is threaded through the context so that storage methods
// called inside fn use it instead of the pool directly.
// If fn returns an error the transaction is rolled back; otherwise committed.
//
// If the context already carries a transaction (nested call), fn is executed
// directly within that existing transaction — no new transaction is started.
func (s *Store) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	ctx = storage.InitChangeEvents(ctx)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin transaction: %w", err)
	}

	txCtx := txToContext(ctx, tx)
	if fnErr := fn(txCtx); fnErr != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			slog.Error("postgres: rollback failed", "error", rbErr)
		}
		return fnErr
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit transaction: %w", err)
	}

	s.dispatchChangeEvents(ctx)
	return nil
}

// query routes a SELECT to the in-context transaction or the pool.
func (s *Store) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	var rows pgx.Rows
	var err error
	if tx, ok := txFromContext(ctx); ok {
		rows, err = tx.Query(ctx, sql, args...)
	} else {
		rows, err = s.pool.Query(ctx, sql, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	return rows, nil
}

// exec routes a DML statement to the in-context transaction or the pool.
func (s *Store) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	var err error
	if tx, ok := txFromContext(ctx); ok {
		tag, err = tx.Exec(ctx, sql, args...)
	} else {
		tag, err = s.pool.Exec(ctx, sql, args...)
	}
	if err != nil {
		return tag, fmt.Errorf("postgres: %w", err)
	}
	return tag, nil
}

// queryRow routes a single-row query to the in-context transaction or the pool.
func (s *Store) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := txFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return s.pool.QueryRow(ctx, sql, args...)
}

// dispatchChangeEvents fires stashed change events to all registered subscribers.
// Called after a successful commit. Each subscriber is isolated with panic recovery.
func (s *Store) dispatchChangeEvents(ctx context.Context) {
	if s.shared == nil {
		return
	}
	events := storage.DrainChangeEvents(ctx)
	if len(events) == 0 {
		return
	}
	notifyCtx := context.Background()
	for _, sub := range s.shared.subscribers {
		for i := range events {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("change subscriber panicked",
							"subscriber", fmt.Sprintf("%T", sub),
							"slug", events[i].Slug,
							"panic", r,
						)
					}
				}()
				sub.OnSpecChanged(notifyCtx, &events[i])
			}()
		}
	}
}
