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

func TestCreateSpec_CreatesChangeLogEntry(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-changelog", "test intent", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-changelog", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, int32(1), entry.Version)
	assert.Equal(t, storage.SpecStageSpark, entry.Stage)
	assert.True(t, entry.Checkpoint, "initial creation should be a checkpoint")
	assert.NotEmpty(t, entry.ContentHash)
	assert.NotEmpty(t, entry.ID)
}
