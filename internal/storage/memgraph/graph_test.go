// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestAddAndListEdges(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create two specs
	_, err = store.CreateSpec(ctx, "spec-x", "Spec X", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-y", "Spec Y", "p2", "medium")
	require.NoError(t, err)

	// Add edge: spec-x depends on spec-y
	edge, err := store.AddEdge(ctx, "spec-x", "spec-y", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	require.NotEmpty(t, edge.FromID)
	require.NotEmpty(t, edge.ToID)
	require.Equal(t, storage.EdgeTypeDependsOn, edge.EdgeType)

	// List edges for spec-x
	edges, err := store.ListEdges(ctx, "spec-x", "")
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	// Remove edge
	err = store.RemoveEdge(ctx, "spec-x", "spec-y", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
}

func TestGetDependencies(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "parent", "Parent spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "child", "Child spec", "p2", "low")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "parent", "child", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	deps, err := store.GetDependencies(ctx, "parent")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, "child", deps[0].Slug)
}

func TestDiamondDependencies(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create diamond: A -> B, A -> C, B -> D, C -> D
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err = store.CreateSpec(ctx, slug, "Spec "+slug, "p1", "low")
		require.NoError(t, err)
	}

	_, err = store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "a", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Transitive deps of A should include B, C, D (no duplicates)
	trans, err := store.GetTransitiveDeps(ctx, "a")
	require.NoError(t, err)
	require.Len(t, trans, 3) // b, c, d

	// Impact of D should include A, B, C
	impact, err := store.GetImpact(ctx, "d")
	require.NoError(t, err)
	require.Len(t, impact, 3) // a, b, c
}

func TestBlocksEdgeDirection(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create two specs: A blocks B
	_, err = store.CreateSpec(ctx, "spec-alpha", "Alpha spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-beta", "Beta spec", "p2", "low")
	require.NoError(t, err)

	// Add BLOCKS edge: spec-alpha blocks spec-beta
	edge, err := store.AddEdge(ctx, "spec-alpha", "spec-beta", storage.EdgeTypeBlocks)
	require.NoError(t, err)
	require.Equal(t, storage.EdgeTypeBlocks, edge.EdgeType)

	// ListEdges for spec-alpha should return the BLOCKS edge
	edges, err := store.ListEdges(ctx, "spec-alpha", "")
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	var found *storage.Edge
	for _, e := range edges {
		if e.EdgeType == storage.EdgeTypeBlocks {
			found = e
			break
		}
	}
	require.NotNil(t, found, "expected a BLOCKS edge in ListEdges result")

	// Direction must be preserved: from=spec-alpha, to=spec-beta
	require.Equal(t, "spec-alpha", found.FromID)
	require.Equal(t, "spec-beta", found.ToID)
}

func TestGetReady(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create specs: A depends on B, C has no deps
	_, err = store.CreateSpec(ctx, "blocked", "Blocked spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "blocker", "Blocker spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "free", "Free spec", "p2", "low")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "blocked", "blocker", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// "free" has no deps so it's ready; "blocked" depends on "blocker" which is not done
	ready, err := store.GetReady(ctx)
	require.NoError(t, err)

	slugs := make([]string, len(ready))
	for i, r := range ready {
		slugs[i] = r.Slug
	}
	require.Contains(t, slugs, "free")
	require.Contains(t, slugs, "blocker") // blocker has no deps itself
	require.NotContains(t, slugs, "blocked")
}
