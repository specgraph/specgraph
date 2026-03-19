// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSpec_AtomicWithChangeLog(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "tx-atomic-create", "intent", "p2", "medium")
	require.NoError(t, err)

	// Verify spec and ChangeLog exist atomically.
	spec, err := store.GetSpec(ctx, "tx-atomic-create")
	require.NoError(t, err)
	assert.Equal(t, "tx-atomic-create", spec.Slug)

	entries, err := store.ListChanges(ctx, "tx-atomic-create", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1, "ChangeLog must exist if spec exists (atomic)")
	assert.True(t, entries[0].Checkpoint)
}

func TestLifecycleAmendSpec_AtomicWithChangeLog(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "tx-atomic-amend", "intent", "p2", "medium")
	require.NoError(t, err)

	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "tx-atomic-amend", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	amended, err := store.LifecycleAmendSpec(ctx, "tx-atomic-amend", "rework", "shape")
	require.NoError(t, err)
	assert.Equal(t, storage.SpecStageShape, amended.Stage)

	entries, err := store.ListChanges(ctx, "tx-atomic-amend", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	require.NotEmpty(t, entries, "expected at least one checkpoint ChangeLog entry")
	last := entries[len(entries)-1]
	assert.True(t, last.Checkpoint)
	assert.Contains(t, last.Summary, "Amended")
}
