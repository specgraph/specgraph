// SPDX-License-Identifier: Apache-2.0
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

func TestTransitionStage_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// CreateSpec sets stage to "spark", so transition spark → shape.
	err = store.TransitionStage(ctx, "funnel-test", storage.SpecStageSpark, storage.SpecStageShape)
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "funnel-test")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageShape, spec.Stage)

	err = store.TransitionStage(ctx, "funnel-test", storage.SpecStageShape, storage.SpecStageSpecify)
	require.NoError(t, err)

	// Invalid: skipping from specify straight to approved (must go through decompose).
	err = store.TransitionStage(ctx, "funnel-test", storage.SpecStageSpecify, storage.SpecStageApproved)
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestTransitionStage_InvalidTransition(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "wrong-stage", "Wrong stage test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// Spec is at "spark", but we claim it's at "shape" → should fail.
	err = store.TransitionStage(ctx, "wrong-stage", storage.SpecStageShape, storage.SpecStageSpecify)
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestTransitionStage_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	err := store.TransitionStage(ctx, "nonexistent", storage.SpecStageSpark, storage.SpecStageShape)
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestTransitionStage_ApprovedGuard(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "approved-guard", "Approved guard test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.SpecStageSpark, storage.SpecStageShape))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.SpecStageShape, storage.SpecStageSpecify))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.SpecStageSpecify, storage.SpecStageDecompose))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.SpecStageDecompose, storage.SpecStageApproved))

	// Once approved, further transitions should fail with ErrSpecAlreadyApproved.
	err = store.TransitionStage(ctx, "approved-guard", storage.SpecStageApproved, storage.SpecStageSpark)
	require.ErrorIs(t, err, storage.ErrSpecAlreadyApproved)
}

func TestTransitionStage_UpdatesContentHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	spec, err := store.CreateSpec(ctx, "stage-hash", "Test", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	initialHash := spec.ContentHash

	err = store.TransitionStage(ctx, "stage-hash", storage.SpecStageSpark, storage.SpecStageShape)
	require.NoError(t, err)

	updated, err := store.GetSpec(ctx, "stage-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHash, updated.ContentHash)
	require.NotEmpty(t, updated.ContentHash)
}

func TestStoreSparkOutput(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "spark-out", "Spark output test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "spark-out", &storage.SparkOutput{
		Seed:       "Build a login system",
		Signal:     "User request",
		Questions:  []string{"OAuth or password?", "MFA required?"},
		ScopeSniff: "medium",
		KillTest:   "If no users need auth",
	})
	require.NoError(t, err)

	// Verify output is persisted and retrievable.
	spec, err := store.GetSpec(ctx, "spark-out")
	require.NoError(t, err)
	require.NotNil(t, spec.SparkOutput)
	assert.Equal(t, "Build a login system", spec.SparkOutput.Seed)
	assert.Equal(t, "User request", spec.SparkOutput.Signal)
	assert.Len(t, spec.SparkOutput.Questions, 2)
}

func TestStoreSparkOutput_UpdatesContentHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	spec, err := store.CreateSpec(ctx, "authoring-hash", "Test", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	initialHash := spec.ContentHash
	require.NotEmpty(t, initialHash)

	err = store.StoreSparkOutput(ctx, "authoring-hash", &storage.SparkOutput{Seed: "test seed"})
	require.NoError(t, err)

	updated, err := store.GetSpec(ctx, "authoring-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHash, updated.ContentHash)
	require.NotEmpty(t, updated.ContentHash)
}

func TestStoreSparkOutput_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	err := store.StoreSparkOutput(ctx, "nonexistent", &storage.SparkOutput{Seed: "x"})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestStoreShapeOutput_PromotesDecisions(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "shape-decisions-test", "test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	shapeOut := &storage.ShapeOutput{
		ScopeIn: []string{"feature A"},
		Decisions: []storage.DecisionInput{
			{
				Slug:      "use-memgraph",
				Title:     "Use Memgraph",
				Body:      "We chose Memgraph for graph storage",
				Rationale: "Native graph, Bolt protocol, good Go driver",
			},
		},
	}
	err = store.StoreShapeOutput(ctx, "shape-decisions-test", shapeOut)
	require.NoError(t, err)

	// Verify decision node was created.
	decision, err := store.GetDecision(ctx, "use-memgraph")
	require.NoError(t, err)
	assert.Equal(t, "Use Memgraph", decision.Title)
	assert.Equal(t, "We chose Memgraph for graph storage", decision.Body)
	assert.Equal(t, storage.DecisionStatusProposed, decision.Status)

	// Verify DECIDED_IN edge exists with correct direction: spec→decision.
	edges, err := store.ListEdges(ctx, "shape-decisions-test", storage.EdgeTypeDecidedIn)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, storage.EdgeTypeDecidedIn, edges[0].EdgeType)
	assert.Equal(t, "shape-decisions-test", edges[0].FromID)
	assert.Equal(t, "use-memgraph", edges[0].ToID)
}

func TestStoreShapeOutput_IdempotentDecisions(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "idempotent-test", "test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	shapeOut := &storage.ShapeOutput{
		ScopeIn: []string{"feature"},
		Decisions: []storage.DecisionInput{
			{Slug: "reuse-decision", Title: "Reuse", Body: "Reuse it", Rationale: "Why not"},
		},
	}

	// Store twice — should not fail or create duplicate.
	require.NoError(t, store.StoreShapeOutput(ctx, "idempotent-test", shapeOut))
	require.NoError(t, store.StoreShapeOutput(ctx, "idempotent-test", shapeOut))

	// DECIDED_IN edge should be deduplicated.
	edges, err := store.ListEdges(ctx, "idempotent-test", storage.EdgeTypeDecidedIn)
	require.NoError(t, err)
	require.Len(t, edges, 1, "expected exactly one DECIDED_IN edge after idempotent store")

	// Still just one decision node.
	decision, err := store.GetDecision(ctx, "reuse-decision")
	require.NoError(t, err)
	assert.Equal(t, "Reuse", decision.Title)
}

func TestStoreSpecifyOutput(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "specify-out", "Specify test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	err = store.StoreSpecifyOutput(ctx, "specify-out", &storage.SpecifyOutput{
		Interfaces: []storage.InterfaceSection{
			{Name: "API", Body: "REST endpoint"},
		},
		VerifyCriteria: []storage.VerifyCriterion{
			{Category: "functional", Description: "Returns 200"},
		},
		Invariants: []string{"No data loss"},
	})
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "specify-out")
	require.NoError(t, err)
	require.NotNil(t, spec.SpecifyOutput)
	assert.Len(t, spec.SpecifyOutput.Interfaces, 1)
	assert.Len(t, spec.SpecifyOutput.VerifyCriteria, 1)
}

func TestStoreDecomposeOutput_CreatesSlices(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "decomp-parent", "Parent spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	children, err := store.StoreDecomposeOutput(ctx, "decomp-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-1", Intent: "Auth endpoint", Verify: []string{"login works"}, Touches: []string{"auth.go"}},
			{ID: "slice-2", Intent: "Token refresh", Verify: []string{"refresh works"}, Touches: []string{"token.go"}, DependsOn: []string{"slice-1"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, children, 2)
	require.Equal(t, "decomp-parent/slice-1", children[0])
	require.Equal(t, "decomp-parent/slice-2", children[1])

	// Verify Slice nodes were created.
	sl1, err := store.GetSlice(ctx, "decomp-parent/slice-1")
	require.NoError(t, err)
	require.Equal(t, "Auth endpoint", sl1.Intent)
	require.Equal(t, []string{"login works"}, sl1.Verify)
	require.Equal(t, []string{"auth.go"}, sl1.Touches)
	require.Equal(t, storage.SliceStatusOpen, sl1.Status)
	require.Empty(t, sl1.DependsOn, "slice-1 has no dependencies")

	sl2, err := store.GetSlice(ctx, "decomp-parent/slice-2")
	require.NoError(t, err)
	require.Equal(t, "Token refresh", sl2.Intent)
	require.Equal(t, []string{"decomp-parent/slice-1"}, sl2.DependsOn)

	// Verify no child Spec nodes were created.
	_, err = store.GetSpec(ctx, "decomp-parent/slice-1")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)

	// Verify Slices appear in GetFullGraph.
	graph, err := store.GetFullGraph(ctx)
	require.NoError(t, err)
	slugLabels := make(map[string]storage.NodeLabel)
	for _, n := range graph.Nodes {
		slugLabels[n.Slug] = n.Label
	}
	require.Equal(t, storage.NodeLabelSpec, slugLabels["decomp-parent"])
	require.Equal(t, storage.NodeLabelSlice, slugLabels["decomp-parent/slice-1"])
	require.Equal(t, storage.NodeLabelSlice, slugLabels["decomp-parent/slice-2"])

	// Verify COMPOSES and DEPENDS_ON edges in the graph.
	edgeSet := make(map[string]storage.EdgeType)
	for _, e := range graph.Edges {
		key := e.FromID + "->" + e.ToID
		edgeSet[key] = e.EdgeType
	}
	require.Equal(t, storage.EdgeTypeComposes, edgeSet["decomp-parent/slice-1->decomp-parent"])
	require.Equal(t, storage.EdgeTypeComposes, edgeSet["decomp-parent/slice-2->decomp-parent"])
	require.Equal(t, storage.EdgeTypeDependsOn, edgeSet["decomp-parent/slice-2->decomp-parent/slice-1"])
}

func TestStoreDecomposeOutput_Idempotent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "idem-parent", "Idempotency parent", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	output := &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "First slice", Verify: []string{"a works"}, Touches: []string{"a.go"}},
			{ID: "slice-b", Intent: "Second slice", Verify: []string{"b works"}, Touches: []string{"b.go"}, DependsOn: []string{"slice-a"}},
		},
	}

	children1, err := store.StoreDecomposeOutput(ctx, "idem-parent", output)
	require.NoError(t, err)
	require.Len(t, children1, 2)

	// Second call with identical data — must succeed.
	children2, err := store.StoreDecomposeOutput(ctx, "idem-parent", output)
	require.NoError(t, err)
	require.Len(t, children2, 2)
	require.Equal(t, children1[0], children2[0])
	require.Equal(t, children1[1], children2[1])
}

// TestStoreDecomposeOutput_ReconcilesOnReauthor pins CR-01: re-running
// StoreDecomposeOutput (the amend → re-decompose flow) must reconcile the
// existing child Slice nodes rather than treating them as immutable — updating
// changed slice bodies, creating new slices, and pruning removed slices along
// with their edges. Also exercises the amend → re-author round trip (WR-02).
func TestStoreDecomposeOutput_ReconcilesOnReauthor(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "reauth-parent", "Parent spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// Original decomposition: slice-a, slice-b, slice-c (c depends on a).
	_, err = store.StoreDecomposeOutput(ctx, "reauth-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "Original A", Verify: []string{"a v1"}, Touches: []string{"a.go"}},
			{ID: "slice-b", Intent: "Original B", Verify: []string{"b v1"}, Touches: []string{"b.go"}},
			{ID: "slice-c", Intent: "Original C", Verify: []string{"c v1"}, Touches: []string{"c.go"}, DependsOn: []string{"slice-a"}},
		},
	})
	require.NoError(t, err)

	// Move the spec to an amend-eligible stage and amend it back to re-enter at
	// decompose (lands one stage earlier, at specify), retaining the slices.
	approved := "approved"
	_, err = store.UpdateSpec(ctx, "reauth-parent", nil, &approved, nil, nil, nil)
	require.NoError(t, err)
	amended, err := store.LifecycleAmendSpec(ctx, "reauth-parent", "revisit slices", "decompose")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("decompose").PrecedingAuthStage(), amended.Stage)

	// Re-author the decomposition:
	//   - slice-a: changed body (intent/verify/touches)
	//   - slice-b: REMOVED
	//   - slice-c: changed dependency (now depends on nothing) + new body
	//   - slice-d: NEW slice depending on slice-c
	children, err := store.StoreDecomposeOutput(ctx, "reauth-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "Updated A", Verify: []string{"a v2"}, Touches: []string{"a2.go"}},
			{ID: "slice-c", Intent: "Updated C", Verify: []string{"c v2"}, Touches: []string{"c.go"}},
			{ID: "slice-d", Intent: "New D", Verify: []string{"d v1"}, Touches: []string{"d.go"}, DependsOn: []string{"slice-c"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"reauth-parent/slice-a", "reauth-parent/slice-c", "reauth-parent/slice-d"}, children)

	// slice-a body was updated (not dropped).
	slA, err := store.GetSlice(ctx, "reauth-parent/slice-a")
	require.NoError(t, err)
	require.Equal(t, "Updated A", slA.Intent)
	require.Equal(t, []string{"a v2"}, slA.Verify)
	require.Equal(t, []string{"a2.go"}, slA.Touches)

	// slice-c body updated and its stale DEPENDS_ON edge removed.
	slC, err := store.GetSlice(ctx, "reauth-parent/slice-c")
	require.NoError(t, err)
	require.Equal(t, "Updated C", slC.Intent)
	require.Empty(t, slC.DependsOn, "slice-c no longer depends on slice-a")

	// slice-d created.
	slD, err := store.GetSlice(ctx, "reauth-parent/slice-d")
	require.NoError(t, err)
	require.Equal(t, "New D", slD.Intent)
	require.Equal(t, []string{"reauth-parent/slice-c"}, slD.DependsOn)

	// slice-b was pruned (node gone).
	_, err = store.GetSlice(ctx, "reauth-parent/slice-b")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)

	// Verify the graph reflects the new decomposition with no orphans.
	graph, err := store.GetFullGraph(ctx)
	require.NoError(t, err)
	nodeSlugs := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeSlugs[n.Slug] = true
	}
	require.False(t, nodeSlugs["reauth-parent/slice-b"], "pruned slice-b must not remain as a node")
	require.True(t, nodeSlugs["reauth-parent/slice-a"])
	require.True(t, nodeSlugs["reauth-parent/slice-c"])
	require.True(t, nodeSlugs["reauth-parent/slice-d"])

	edgeSet := make(map[string]storage.EdgeType)
	for _, e := range graph.Edges {
		if e.FromID == "reauth-parent/slice-b" || e.ToID == "reauth-parent/slice-b" {
			t.Fatalf("orphaned edge referencing pruned slice-b: %s->%s", e.FromID, e.ToID)
		}
		edgeSet[e.FromID+"->"+e.ToID] = e.EdgeType
	}
	// Stale c->a DEPENDS_ON edge is gone; new d->c edge exists.
	_, hasStale := edgeSet["reauth-parent/slice-c->reauth-parent/slice-a"]
	require.False(t, hasStale, "stale DEPENDS_ON edge slice-c->slice-a must be removed")
	require.Equal(t, storage.EdgeTypeDependsOn, edgeSet["reauth-parent/slice-d->reauth-parent/slice-c"])
	// COMPOSES edges for surviving slices intact.
	require.Equal(t, storage.EdgeTypeComposes, edgeSet["reauth-parent/slice-a->reauth-parent"])
	require.Equal(t, storage.EdgeTypeComposes, edgeSet["reauth-parent/slice-c->reauth-parent"])
	require.Equal(t, storage.EdgeTypeComposes, edgeSet["reauth-parent/slice-d->reauth-parent"])
}

func TestStoreDecomposeOutput_MissingParent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.StoreDecomposeOutput(ctx, "ghost-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-x", Intent: "Orphan slice", Verify: []string{"x works"}, Touches: []string{"x.go"}},
		},
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestStoreDecomposeOutput_DuplicateSliceID(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "dup-parent", "Duplicate slice test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.StoreDecomposeOutput(ctx, "dup-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "First"},
			{ID: "slice-a", Intent: "Duplicate ID"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestStoreDecomposeOutput_UnknownDependency(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "dep-parent", "Unknown dep test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.StoreDecomposeOutput(ctx, "dep-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "First"},
			{ID: "slice-b", Intent: "Second", DependsOn: []string{"nonexistent"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown sibling")
}

func TestStoreSafetyFlags(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "safety-flags-spec", "Safety flags test", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	flags := []storage.SafetyFlag{
		{
			Category:    storage.SafetyCategory("security"),
			Severity:    storage.SeverityCritical,
			Description: "Spec requests unrestricted filesystem access",
		},
		{
			Category:    storage.SafetyCategory("privacy"),
			Severity:    storage.SeverityWarning,
			Description: "May expose PII without consent mechanism",
		},
	}

	err = store.StoreSafetyFlags(ctx, "safety-flags-spec", flags)
	require.NoError(t, err)
}

func TestStoreSafetyFlags_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	err := store.StoreSafetyFlags(ctx, "nonexistent-spec", []storage.SafetyFlag{
		{Category: storage.SafetyCategory("security"), Severity: storage.SeverityCritical, Description: "test"},
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

