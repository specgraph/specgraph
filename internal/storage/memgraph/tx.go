// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Transaction Usage Guide
//
// All multi-query write paths MUST use RunInTransaction for atomicity.
// SpecGraph uses optimistic concurrency control: version guards in
// WHERE clauses detect concurrent modifications, and transactions
// ensure partial failures roll back cleanly.
//
// Pattern:
//
//	func (s *Store) SomeWriteOp(ctx context.Context, ...) error {
//	    // Validation (no DB) stays outside — reduces lock time.
//	    return s.RunInTransaction(ctx, func(txCtx context.Context) error {
//	        // All DB operations use txCtx, not ctx.
//	        records, err := s.executeQuery(txCtx, query, params)
//	        return s.createChangeLog(txCtx, slug, entry, deltas)
//	    })
//	}
//
// For functions returning values, capture via closure variable:
//
//	var result *storage.Spec
//	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
//	    result = spec
//	    return nil
//	})
//	return result, err
//
// Nested RunInTransaction calls reuse the outer transaction — safe for
// operations like StoreDecomposeOutput that call CreateSpec internally.

package memgraph

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/specgraph/specgraph/internal/storage"
)

// txKey is the context key used to thread a managed transaction through storage calls.
type txKey struct{}

// txToContext stores a ManagedTransaction in the context.
func txToContext(ctx context.Context, tx neo4j.ManagedTransaction) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// txFromContext retrieves a ManagedTransaction from the context, if present.
func txFromContext(ctx context.Context) (neo4j.ManagedTransaction, bool) {
	tx, ok := ctx.Value(txKey{}).(neo4j.ManagedTransaction)
	return tx, ok
}

// RunInTransaction executes fn within a single Memgraph write transaction.
// The transaction from the session is threaded through the context so that
// storage methods called inside fn use it instead of auto-commit queries.
// If fn returns an error the transaction is rolled back automatically by the
// neo4j driver; otherwise it is committed.
//
// If the context already carries a transaction (from a parent RunInTransaction
// call), fn is executed directly within that existing transaction — no nested
// session or transaction is created. This makes it safe for storage methods
// like StoreShapeOutput to call RunInTransaction internally while also being
// called from a higher-level transaction via runInTxOrSequential.
func (s *Store) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) (retErr error) {
	// Reuse existing transaction if already in one (prevents nested tx conflicts).
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	ctx = storage.InitChangeEvents(ctx)

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer func() {
		if closeErr := session.Close(ctx); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("memgraph: close session: %w", closeErr)
		}
	}()

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		txCtx := txToContext(ctx, tx)
		if err := fn(txCtx); err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("memgraph: transaction: %w", err)
	}
	s.dispatchChangeEvents(ctx)
	return nil
}

// mapTxConflict wraps Memgraph's "Cannot resolve conflicting transactions"
// error with ErrConcurrentModification so callers can handle it uniformly.
func mapTxConflict(err error) error {
	if err != nil && strings.Contains(err.Error(), "Cannot resolve conflicting transactions") {
		return fmt.Errorf("%w: %w", storage.ErrConcurrentModification, err)
	}
	return err
}

// executeQuery runs a Cypher query using the transaction from context if present,
// otherwise falls back to an auto-commit query via the driver.
func (s *Store) executeQuery(ctx context.Context, query string, params map[string]any) ([]*neo4j.Record, error) {
	if tx, ok := txFromContext(ctx); ok {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, mapTxConflict(fmt.Errorf("memgraph: tx run: %w", err))
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return nil, mapTxConflict(fmt.Errorf("memgraph: tx collect: %w", err))
		}
		return records, nil
	}
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, mapTxConflict(fmt.Errorf("memgraph: execute query: %w", err))
	}
	return result.Records, nil
}

// dispatchChangeEvents fires stashed change events to all registered subscribers.
// Called after successful commit. Each subscriber is isolated with panic recovery.
func (s *Store) dispatchChangeEvents(ctx context.Context) {
	if s.shared == nil {
		return
	}
	events := storage.DrainChangeEvents(ctx)
	if len(events) == 0 {
		return
	}
	notifyCtx := storage.WithGraphBackend(ctx, s)
	for _, sub := range s.shared.subscribers {
		for _, event := range events {
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("change subscriber panicked",
							"subscriber", fmt.Sprintf("%T", sub),
							"slug", event.Slug,
							"panic", r,
						)
					}
				}()
				sub.OnSpecChanged(notifyCtx, event)
			}()
		}
	}
}
