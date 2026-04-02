// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListChanges_ReturnsEntries(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "cl-entries", "initial intent", "p1", "medium")
	require.NoError(t, err)

	newIntent := "updated intent"
	_, err = store.UpdateSpec(ctx, "cl-entries", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "cl-entries", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, int32(1), entries[0].Version)
	assert.True(t, entries[0].Checkpoint, "initial creation should be a checkpoint")
	assert.NotEmpty(t, entries[0].ContentHash)
	assert.NotEmpty(t, entries[0].ID)

	assert.Equal(t, int32(2), entries[1].Version)
	assert.False(t, entries[1].Checkpoint)
	require.Len(t, entries[1].Changes, 1)
	assert.Equal(t, "intent", entries[1].Changes[0].Field)
	assert.Equal(t, "initial intent", entries[1].Changes[0].OldValue)
	assert.Equal(t, "updated intent", entries[1].Changes[0].NewValue)
}

func TestListChanges_CheckpointsOnly(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "cl-checkpoints", "intent", "p1", "medium")
	require.NoError(t, err)

	newIntent := "updated"
	_, err = store.UpdateSpec(ctx, "cl-checkpoints", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	all, err := store.ListChanges(ctx, "cl-checkpoints", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	cps, err := store.ListChanges(ctx, "cl-checkpoints", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	require.Len(t, cps, 1)
	assert.True(t, cps[0].Checkpoint)
}

func TestListChanges_SinceVersion(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "cl-since", "intent", "p1", "medium")
	require.NoError(t, err)

	newIntent := "v2"
	_, err = store.UpdateSpec(ctx, "cl-since", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "cl-since", storage.ChangeLogFilter{SinceVersion: 1})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(2), entries[0].Version)
}

func TestListChanges_Limit(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "cl-limit", "intent", "p1", "medium")
	require.NoError(t, err)

	for i := range 5 {
		v := "intent-updated-" + string(rune('a'+i))
		_, err = store.UpdateSpec(ctx, "cl-limit", &v, nil, nil, nil, nil)
		require.NoError(t, err)
	}

	entries, err := store.ListChanges(ctx, "cl-limit", storage.ChangeLogFilter{Limit: 3})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, int32(1), entries[0].Version)
}

func TestListChanges_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.ListChanges(ctx, "does-not-exist", storage.ChangeLogFilter{})
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListAllChanges(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "all-cl-alpha", "alpha intent", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "all-cl-beta", "beta intent", "p2", "high")
	require.NoError(t, err)

	newIntent := "alpha updated"
	_, err = store.UpdateSpec(ctx, "all-cl-alpha", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListAllChanges(ctx)
	require.NoError(t, err)

	// At least 3 entries: 1 create for alpha, 1 update for alpha, 1 create for beta.
	require.GreaterOrEqual(t, len(entries), 3)

	// All entries must have SpecSlug set.
	for _, e := range entries {
		assert.NotEmpty(t, e.SpecSlug, "SpecSlug must be populated in ListAllChanges")
	}

	// Verify ordering: grouped by spec_slug, then version.
	alphaEntries := filterBySlug(entries, "all-cl-alpha")
	require.Len(t, alphaEntries, 2)
	assert.Equal(t, int32(1), alphaEntries[0].Version)
	assert.Equal(t, int32(2), alphaEntries[1].Version)

	betaEntries := filterBySlug(entries, "all-cl-beta")
	require.Len(t, betaEntries, 1)
}

func filterBySlug(entries []*storage.ChangeLogEntry, slug string) []*storage.ChangeLogEntry {
	var out []*storage.ChangeLogEntry
	for _, e := range entries {
		if e.SpecSlug == slug {
			out = append(out, e)
		}
	}
	return out
}
