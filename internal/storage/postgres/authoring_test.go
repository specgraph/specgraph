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

	_, err := store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low")
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

	_, err := store.CreateSpec(ctx, "wrong-stage", "Wrong stage test", "p1", "low")
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

	_, err := store.CreateSpec(ctx, "approved-guard", "Approved guard test", "p1", "low")
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

	spec, err := store.CreateSpec(ctx, "stage-hash", "Test", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "spark-out", "Spark output test", "p1", "low")
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

	spec, err := store.CreateSpec(ctx, "authoring-hash", "Test", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "shape-decisions-test", "test spec", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "idempotent-test", "test spec", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "specify-out", "Specify test", "p1", "low")
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

	_, err := store.CreateSpec(ctx, "decomp-parent", "Parent spec", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "idem-parent", "Idempotency parent", "p1", "medium")
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

	_, err := store.CreateSpec(ctx, "dup-parent", "Duplicate slice test", "p1", "low")
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

	_, err := store.CreateSpec(ctx, "dep-parent", "Unknown dep test", "p1", "low")
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

	_, err := store.CreateSpec(ctx, "safety-flags-spec", "Safety flags test", "p1", "low")
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

func TestSupersedeSpec_Authoring(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "old-spec", "Original spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "new-spec", "Replacement spec", "p1", "low")
	require.NoError(t, err)

	err = store.SupersedeSpec(ctx, "old-spec", "new-spec", "better approach found")
	require.NoError(t, err)

	// Verify the old spec is now at stage "superseded".
	old, err := store.GetSpec(ctx, "old-spec")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageSuperseded, old.Stage)
}

func TestSupersedeSpec_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "existing-spec", "Exists", "p1", "low")
	require.NoError(t, err)

	// Non-existent old spec.
	err = store.SupersedeSpec(ctx, "nonexistent", "existing-spec", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)

	// Non-existent new spec.
	err = store.SupersedeSpec(ctx, "existing-spec", "nonexistent", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestAmendSpec(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-test", "Amend test", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.SpecStageSpark, storage.SpecStageShape))
	require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.SpecStageShape, storage.SpecStageSpecify))

	// Amend back to shape (valid backward transition).
	result, err := store.AmendSpec(ctx, "amend-test", "need to reconsider scope", storage.SpecStageShape)
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageShape, result.Stage)
	require.Equal(t, int32(4), result.Version, "version should increment after amendment (1 create + 2 transitions + 1 amend = 4)")
}

func TestAmendSpec_AlreadyApproved(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "approved-spec", "Will be approved", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.SpecStageSpark, storage.SpecStageShape))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.SpecStageShape, storage.SpecStageSpecify))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.SpecStageSpecify, storage.SpecStageDecompose))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.SpecStageDecompose, storage.SpecStageApproved))

	_, err = store.AmendSpec(ctx, "approved-spec", "too late", storage.SpecStageShape)
	require.ErrorIs(t, err, storage.ErrSpecAlreadyApproved)
}

func TestAmendSpec_InvalidTransition(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-invalid", "Invalid amend", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "amend-invalid", storage.SpecStageSpark, storage.SpecStageShape))

	// Amend forward (shape → specify) should fail — amend only allows backward.
	_, err = store.AmendSpec(ctx, "amend-invalid", "forward not allowed", storage.SpecStageSpecify)
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestAmendSpec_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.AmendSpec(ctx, "nonexistent-spec", "reason", storage.SpecStageShape)
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestAmendSpec_UpdatesContentHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-hash", "Test", "p1", "medium")
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "amend-hash", storage.SpecStageSpark, storage.SpecStageShape)
	require.NoError(t, err)

	preAmend, err := store.GetSpec(ctx, "amend-hash")
	require.NoError(t, err)

	_, err = store.AmendSpec(ctx, "amend-hash", "rework needed", storage.SpecStageSpark)
	require.NoError(t, err)

	updated, err := store.GetSpec(ctx, "amend-hash")
	require.NoError(t, err)
	require.NotEqual(t, preAmend.ContentHash, updated.ContentHash)
	require.NotEmpty(t, updated.ContentHash)
}

func TestTransitionStage_BackwardViaAmend(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "backward-test", "Backward transition", "p1", "low")
	require.NoError(t, err)

	// Advance spark → shape → specify.
	require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.SpecStageSpark, storage.SpecStageShape))
	require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.SpecStageShape, storage.SpecStageSpecify))

	// AmendSpec back to spark (two stages back).
	result, err := store.AmendSpec(ctx, "backward-test", "starting over", storage.SpecStageSpark)
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageSpark, result.Stage)

	// After amend, can transition forward again from spark.
	require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.SpecStageSpark, storage.SpecStageShape))
}

func TestTransitionStage_SupersededGuard(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "superseded-old", "Will be superseded", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "superseded-new", "Replacement", "p1", "low")
	require.NoError(t, err)

	err = store.SupersedeSpec(ctx, "superseded-old", "superseded-new", "better approach")
	require.NoError(t, err)

	// Attempting TransitionStage on a superseded spec should fail.
	err = store.TransitionStage(ctx, "superseded-old", storage.SpecStageSuperseded, storage.SpecStageShape)
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}
