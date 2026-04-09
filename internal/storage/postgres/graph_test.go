// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestAddEdge_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "spec-x", "Spec X", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-y", "Spec Y", "p2", "medium")
	require.NoError(t, err)

	edge, err := store.AddEdge(ctx, "spec-x", "spec-y", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	require.Equal(t, "spec-x", edge.FromID)
	require.Equal(t, "spec-y", edge.ToID)
	require.Equal(t, storage.EdgeTypeDependsOn, edge.EdgeType)

	// Verify via ListEdges.
	edges, err := store.ListEdges(ctx, "spec-x", "")
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	var found bool
	for _, e := range edges {
		if e.FromID == "spec-x" && e.ToID == "spec-y" && e.EdgeType == storage.EdgeTypeDependsOn {
			found = true
			break
		}
	}
	require.True(t, found, "expected DEPENDS_ON edge in ListEdges")
}

func TestAddEdge_DependsOn_CapturesContentHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create upstream with a known content hash.
	upstream, err := store.CreateSpec(ctx, "upstream", "Upstream intent", "p1", "low")
	require.NoError(t, err)
	require.NotEmpty(t, upstream.ContentHash)

	_, err = store.CreateSpec(ctx, "downstream", "Downstream intent", "p2", "low")
	require.NoError(t, err)

	edge, err := store.AddEdge(ctx, "downstream", "upstream", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	require.Equal(t, upstream.ContentHash, edge.ContentHashAtLink)
}

func TestAddEdge_Idempotent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "a", "A", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "b", "B", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "a", "b", storage.EdgeTypeRelatesTo)
	require.NoError(t, err)
	// Second add should not error (ON CONFLICT DO NOTHING).
	_, err = store.AddEdge(ctx, "a", "b", storage.EdgeTypeRelatesTo)
	require.NoError(t, err)
}

func TestAddEdge_NodeNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "exists", "Exists", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "exists", "missing", storage.EdgeTypeDependsOn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestRemoveEdge(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "rm-from", "From", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "rm-to", "To", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "rm-from", "rm-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	err = store.RemoveEdge(ctx, "rm-from", "rm-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Verify removal.
	edges, err := store.ListEdges(ctx, "rm-from", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	require.Empty(t, edges)
}

func TestRemoveEdge_InTransaction_RollsBack(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "tx-from", "From", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "tx-to", "To", "", "")
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "tx-from", "tx-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	deliberateErr := errors.New("deliberate rollback")
	err = store.RunInTransaction(ctx, func(txCtx context.Context) error {
		if removeErr := store.RemoveEdge(txCtx, "tx-from", "tx-to", storage.EdgeTypeDependsOn); removeErr != nil {
			return removeErr
		}
		return deliberateErr
	})
	require.ErrorIs(t, err, deliberateErr)

	// Edge should survive the rolled-back transaction.
	edges, err := store.ListEdges(ctx, "tx-from", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	require.Len(t, edges, 1, "edge should survive transaction rollback")
}

func TestListEdges_ExcludesInternalTypes(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "internal-test", "Test", "", "")
	require.NoError(t, err)

	// CreateSpec already creates a BELONGS_TO edge internally.
	// Unfiltered listing should NOT include it.
	edges, err := store.ListEdges(ctx, "internal-test", "")
	require.NoError(t, err)
	for _, e := range edges {
		require.NotEqual(t, storage.EdgeType("BELONGS_TO"), e.EdgeType,
			"internal edge type BELONGS_TO should be excluded from unfiltered ListEdges")
	}
}

func TestListEdges_Bidirectional(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "bidir-a", "A", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "bidir-b", "B", "", "")
	require.NoError(t, err)

	// A -> B
	_, err = store.AddEdge(ctx, "bidir-a", "bidir-b", storage.EdgeTypeBlocks)
	require.NoError(t, err)

	// Query from B's perspective should show the incoming edge.
	edges, err := store.ListEdges(ctx, "bidir-b", "")
	require.NoError(t, err)
	require.NotEmpty(t, edges)

	var found bool
	for _, e := range edges {
		if e.FromID == "bidir-a" && e.ToID == "bidir-b" && e.EdgeType == storage.EdgeTypeBlocks {
			found = true
			break
		}
	}
	require.True(t, found, "incoming BLOCKS edge should appear in ListEdges for target node")
}

func TestGetDependencies(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "parent", "Parent", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "child", "Child", "p2", "low")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "parent", "child", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	deps, err := store.GetDependencies(ctx, "parent")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, "child", deps[0].Slug)
	require.Equal(t, storage.NodeLabelSpec, deps[0].Label)
}

func TestGetDependencies_IncludesBlocksSources(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "blocker", "Blocker", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "blocked", "Blocked", "", "")
	require.NoError(t, err)

	// blocker BLOCKS blocked
	_, err = store.AddEdge(ctx, "blocker", "blocked", storage.EdgeTypeBlocks)
	require.NoError(t, err)

	deps, err := store.GetDependencies(ctx, "blocked")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	require.Equal(t, "blocker", deps[0].Slug)
}

func TestGetDependenciesWithEdgeData(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	upstream, err := store.CreateSpec(ctx, "up", "Upstream", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "down", "Downstream", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "down", "up", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	refs, err := store.GetDependenciesWithEdgeData(ctx, "down")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, "up", refs[0].Slug)
	require.Equal(t, upstream.ContentHash, refs[0].ContentHashAtLink)
	require.Equal(t, upstream.ContentHash, refs[0].UpstreamContentHash)
}

func TestRefreshDependencyHashes(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "refresh-up", "Upstream", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "refresh-down", "Downstream", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "refresh-down", "refresh-up", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Mutate upstream's content hash by updating it.
	newIntent := "Updated upstream"
	_, err = store.UpdateSpec(ctx, "refresh-up", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	// Before refresh, edge hash should be stale.
	refs, err := store.GetDependenciesWithEdgeData(ctx, "refresh-down")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.NotEqual(t, refs[0].ContentHashAtLink, refs[0].UpstreamContentHash,
		"hash should be stale before refresh")

	// Refresh.
	err = store.RefreshDependencyHashes(ctx, "refresh-down")
	require.NoError(t, err)

	// After refresh, hashes should match.
	refs, err = store.GetDependenciesWithEdgeData(ctx, "refresh-down")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, refs[0].ContentHashAtLink, refs[0].UpstreamContentHash,
		"hash should be fresh after refresh")
}

func TestGetTransitiveDeps(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Chain: A -> B -> C -> D
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	trans, err := store.GetTransitiveDeps(ctx, "a")
	require.NoError(t, err)
	require.Len(t, trans, 3)

	slugs := make([]string, len(trans))
	for i, r := range trans {
		slugs[i] = r.Slug
	}
	require.Contains(t, slugs, "b")
	require.Contains(t, slugs, "c")
	require.Contains(t, slugs, "d")
}

func TestGetTransitiveDeps_Diamond(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Diamond: A -> B, A -> C, B -> D, C -> D
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "a", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	trans, err := store.GetTransitiveDeps(ctx, "a")
	require.NoError(t, err)
	require.Len(t, trans, 3, "diamond should yield 3 unique transitive deps: b, c, d")
}

func TestGetTransitiveDeps_BoundedChain(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create chain longer than the 50-hop bound.
	const chainLen = 55
	slugs := make([]string, 0, chainLen)
	for i := 0; i < chainLen; i++ {
		slugs = append(slugs, fmt.Sprintf("n%02d", i))
	}
	for _, slug := range slugs {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	// Chain: n54 -> n53 -> ... -> n00
	for i := len(slugs) - 1; i > 0; i-- {
		_, err := store.AddEdge(ctx, slugs[i], slugs[i-1], storage.EdgeTypeDependsOn)
		require.NoError(t, err)
	}

	trans, err := store.GetTransitiveDeps(ctx, slugs[len(slugs)-1])
	require.NoError(t, err)
	require.Len(t, trans, 50, "traversal should be bounded at depth 50")
}

func TestGetImpact(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Chain: A -> B -> C -> D
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Impact of D should include A, B, C.
	impact, err := store.GetImpact(ctx, "d")
	require.NoError(t, err)
	require.Len(t, impact, 3)

	slugs := make([]string, len(impact))
	for i, r := range impact {
		slugs[i] = r.Slug
	}
	require.Contains(t, slugs, "a")
	require.Contains(t, slugs, "b")
	require.Contains(t, slugs, "c")
}

func TestGetImpact_Diamond(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Diamond: A -> B, A -> C, B -> D, C -> D
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "a", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	impact, err := store.GetImpact(ctx, "d")
	require.NoError(t, err)
	require.Len(t, impact, 3, "diamond impact of d should yield a, b, c")
}

func TestGetReady(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create: blocked depends on blocker, free has no deps.
	_, err := store.CreateSpec(ctx, "blocked", "Blocked", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "blocker", "Blocker", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "free", "Free", "", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "blocked", "blocker", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	ready, err := store.GetReady(ctx)
	require.NoError(t, err)

	slugs := make([]string, len(ready))
	for i, r := range ready {
		slugs[i] = r.Slug
	}
	require.Contains(t, slugs, "free")
	require.Contains(t, slugs, "blocker")
	require.NotContains(t, slugs, "blocked")
}

func TestGetReady_BlocksEdgePreventsReady(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "block-target", "Target", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "block-source", "Source", "", "")
	require.NoError(t, err)

	// block-source BLOCKS block-target
	_, err = store.AddEdge(ctx, "block-source", "block-target", storage.EdgeTypeBlocks)
	require.NoError(t, err)

	ready, err := store.GetReady(ctx)
	require.NoError(t, err)

	slugs := make([]string, len(ready))
	for i, r := range ready {
		slugs[i] = r.Slug
	}
	require.Contains(t, slugs, "block-source")
	require.NotContains(t, slugs, "block-target", "spec blocked by active BLOCKS should not be ready")
}

func TestGetReady_DoneSpecsDontAppear(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "done-spec", "Done", "", "")
	require.NoError(t, err)

	done := "done"
	_, err = store.UpdateSpec(ctx, "done-spec", nil, &done, nil, nil, nil)
	require.NoError(t, err)

	ready, err := store.GetReady(ctx)
	require.NoError(t, err)

	for _, r := range ready {
		require.NotEqual(t, "done-spec", r.Slug, "done specs should not appear in ready list")
	}
}

func TestGetCriticalPath(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Diamond: A -> B -> D, A -> C -> D
	// Both paths have length 3, so critical path from A should be one of them.
	for _, slug := range []string{"a", "b", "c", "d"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "a", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "c", "d", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	path, err := store.GetCriticalPath(ctx, "a")
	require.NoError(t, err)
	require.Len(t, path, 3, "critical path through diamond should have 3 nodes (a -> X -> d)")

	// Path must start with "a" and end with "d".
	require.Equal(t, "a", path[0].Slug)
	require.Equal(t, "d", path[len(path)-1].Slug)
}

func TestGetCriticalPath_LinearChain(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// A -> B -> C
	for _, slug := range []string{"a", "b", "c"} {
		_, err := store.CreateSpec(ctx, slug, "Spec "+slug, "", "")
		require.NoError(t, err)
	}
	_, err := store.AddEdge(ctx, "a", "b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "b", "c", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	path, err := store.GetCriticalPath(ctx, "a")
	require.NoError(t, err)
	require.Len(t, path, 3)
	require.Equal(t, "a", path[0].Slug)
	require.Equal(t, "b", path[1].Slug)
	require.Equal(t, "c", path[2].Slug)
}

func TestGetCriticalPath_NoDeps(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "lonely", "No deps", "", "")
	require.NoError(t, err)

	path, err := store.GetCriticalPath(ctx, "lonely")
	require.NoError(t, err)
	// Node with no deps: path is just itself.
	require.Len(t, path, 1)
	require.Equal(t, "lonely", path[0].Slug)
}

func TestGetFullGraph(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "fg-a", "A", "p1", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "fg-b", "B", "p2", "")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "fg-a", "fg-b", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	graph, err := store.GetFullGraph(ctx)
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Should have at least 2 nodes.
	require.GreaterOrEqual(t, len(graph.Nodes), 2)

	nodeSlugs := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeSlugs[n.Slug] = true
	}
	require.True(t, nodeSlugs["fg-a"])
	require.True(t, nodeSlugs["fg-b"])

	// Should have the DEPENDS_ON edge.
	var foundEdge bool
	for _, e := range graph.Edges {
		if e.FromID == "fg-a" && e.ToID == "fg-b" && e.EdgeType == storage.EdgeTypeDependsOn {
			foundEdge = true
			break
		}
	}
	require.True(t, foundEdge, "full graph should contain the DEPENDS_ON edge")
}

func TestGetFullGraph_ExcludesInternalEdges(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "fg-internal", "Internal test", "", "")
	require.NoError(t, err)

	graph, err := store.GetFullGraph(ctx)
	require.NoError(t, err)

	for _, e := range graph.Edges {
		require.NotEqual(t, storage.EdgeType("BELONGS_TO"), e.EdgeType,
			"full graph should exclude BELONGS_TO edges")
		require.NotEqual(t, storage.EdgeType("HAS_CHANGE"), e.EdgeType,
			"full graph should exclude HAS_CHANGE edges")
	}
}

func TestGetFullGraph_Empty(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	graph, err := store.GetFullGraph(ctx)
	require.NoError(t, err)
	require.NotNil(t, graph)
	require.Empty(t, graph.Nodes)
	require.Empty(t, graph.Edges)
}

func TestBlocksEdgeDirection(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "spec-alpha", "Alpha", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-beta", "Beta", "", "")
	require.NoError(t, err)

	edge, err := store.AddEdge(ctx, "spec-alpha", "spec-beta", storage.EdgeTypeBlocks)
	require.NoError(t, err)
	require.Equal(t, storage.EdgeTypeBlocks, edge.EdgeType)

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
	require.Equal(t, "spec-alpha", found.FromID)
	require.Equal(t, "spec-beta", found.ToID)
}

func TestRemoveEdge_Idempotent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "idem-from", "From", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "idem-to", "To", "", "")
	require.NoError(t, err)

	// Remove an edge that does not exist — should not error.
	err = store.RemoveEdge(ctx, "idem-from", "idem-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Add and then remove twice — second remove should also not error.
	_, err = store.AddEdge(ctx, "idem-from", "idem-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	err = store.RemoveEdge(ctx, "idem-from", "idem-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	err = store.RemoveEdge(ctx, "idem-from", "idem-to", storage.EdgeTypeDependsOn)
	require.NoError(t, err)
}

func TestGetDependenciesWithEdgeData_NonexistentSlug(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Nonexistent slug — resolveNodeRef returns minimal NodeRef (no error).
	refs, err := store.GetDependenciesWithEdgeData(ctx, "nonexistent-slug")
	require.NoError(t, err)
	require.Empty(t, refs)
}

func TestGetDependencies_NonexistentSpec(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// GetDependencies on a slug that doesn't exist — no edges, empty result.
	deps, err := store.GetDependencies(ctx, "ghost-slug")
	require.NoError(t, err)
	require.Empty(t, deps)
}
