// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGetDecision(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	d, err := store.CreateDecision(ctx, "use-memgraph", "Use Memgraph", "Use Memgraph as primary DB", "Native Cypher support")
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Contains(t, d.ID, "dec-")
	require.Equal(t, "use-memgraph", d.Slug)
	require.Equal(t, "Use Memgraph", d.Title)
	require.Equal(t, storage.DecisionStatusProposed, d.Status)
	require.Equal(t, "Use Memgraph as primary DB", d.Decision)
	require.Equal(t, "Native Cypher support", d.Rationale)
	require.NotNil(t, d.CreatedAt)

	got, err := store.GetDecision(ctx, "use-memgraph")
	require.NoError(t, err)
	require.Equal(t, d.ID, got.ID)
	require.Equal(t, d.Slug, got.Slug)
}

func TestListDecisions(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateDecision(ctx, "dec-a", "First", "Decision A", "Reason A")
	require.NoError(t, err)
	_, err = store.CreateDecision(ctx, "dec-b", "Second", "Decision B", "Reason B")
	require.NoError(t, err)

	all, err := store.ListDecisions(ctx, "", 0)
	require.NoError(t, err)
	require.Len(t, all, 2)

	filtered, err := store.ListDecisions(ctx, storage.DecisionStatusProposed, 0)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
}

func TestUpdateDecision(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateDecision(ctx, "update-dec", "Original Title", "Original decision", "Original rationale")
	require.NoError(t, err)

	newStatus := storage.DecisionStatusAccepted
	updated, err := store.UpdateDecision(ctx, "update-dec", nil, &newStatus, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, storage.DecisionStatusAccepted, updated.Status)
	require.Equal(t, "Original Title", updated.Title)

	_, err = store.UpdateDecision(ctx, "nonexistent", nil, &newStatus, nil, nil, nil)
	require.Error(t, err)
}

func TestGetDecision_NotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetDecision(ctx, "nonexistent")
	require.Error(t, err)
}
