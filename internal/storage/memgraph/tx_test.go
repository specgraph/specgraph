// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunInTransaction_Commit(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Both CreateSpec calls inside a single transaction should commit together.
	err = store.RunInTransaction(ctx, func(txCtx context.Context) error {
		if _, createErr := store.CreateSpec(txCtx, "tx-spec-a", "First transactional spec", "p1", "low"); createErr != nil {
			return createErr
		}
		_, createErr := store.CreateSpec(txCtx, "tx-spec-b", "Second transactional spec", "p2", "medium")
		return createErr
	})
	require.NoError(t, err)

	// Both specs should be visible after commit.
	specA, err := store.GetSpec(ctx, "tx-spec-a")
	require.NoError(t, err)
	require.Equal(t, "tx-spec-a", specA.Slug)

	specB, err := store.GetSpec(ctx, "tx-spec-b")
	require.NoError(t, err)
	require.Equal(t, "tx-spec-b", specB.Slug)
}

func TestRunInTransaction_Rollback(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// First op succeeds, second returns an error -- entire tx should roll back.
	deliberateErr := errors.New("deliberate failure")
	err = store.RunInTransaction(ctx, func(txCtx context.Context) error {
		if _, createErr := store.CreateSpec(txCtx, "tx-rollback-spec", "Should be rolled back", "p1", "low"); createErr != nil {
			return createErr
		}
		return deliberateErr
	})
	require.ErrorIs(t, err, deliberateErr)

	// The spec created in the failed transaction must not be visible.
	_, err = store.GetSpec(ctx, "tx-rollback-spec")
	require.Error(t, err, "spec should not exist after rollback")
}
