// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore creates a fresh Memgraph-backed store for a single test,
// registering cleanup of the container and store connection.
func newTestStore(t *testing.T, opts ...memgraph.Option) (*memgraph.Store, context.Context) {
	t.Helper()
	boltURI, cleanup := setupMemgraph(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	store, err := newStore(ctx, boltURI, opts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := store.Close(ctx); err != nil {
			t.Errorf("store.Close: %v", err)
		}
	})
	return store, ctx
}

func TestAuthoring(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	t.Cleanup(cleanup)

	t.Run("TransitionStage", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low")
		require.NoError(t, err)

		// CreateSpec sets stage to "spark", so transition spark → shape.
		err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape))
		require.NoError(t, err)

		spec, err := store.GetSpec(ctx, "funnel-test")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageShape, spec.Stage)

		err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
		require.NoError(t, err)

		// Invalid: skipping from specify straight to approved (must go through decompose).
		err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageApproved))
		require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
	})

	t.Run("TransitionStage_WrongStage", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "wrong-stage", "Wrong stage test", "p1", "low")
		require.NoError(t, err)

		// Spec is at "spark", but we claim it's at "shape" → should fail.
		err = store.TransitionStage(ctx, "wrong-stage", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
		require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
	})

	t.Run("StoreSparkOutput", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "spark-out", "Spark output test", "p1", "low")
		require.NoError(t, err)

		err = store.StoreSparkOutput(ctx, "spark-out", &storage.SparkOutput{
			Seed:       "Build a login system",
			Signal:     "User request",
			Questions:  []string{"OAuth or password?", "MFA required?"},
			ScopeSniff: "medium",
			KillTest:   "If no users need auth",
		})
		require.NoError(t, err)
	})

	t.Run("StoreDecomposeOutput", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "decomp-parent", "Parent spec", "p1", "medium")
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
	})

	t.Run("AmendSpec", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "amend-test", "Amend test", "p1", "low")
		require.NoError(t, err)
		// CreateSpec sets stage to "spark". Advance through stages.
		require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
		require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))

		// Amend back to shape (valid backward transition).
		spec, err := store.AmendSpec(ctx, "amend-test", "need to reconsider scope", storage.AuthoringStage(authoring.StageShape))
		require.NoError(t, err)
		require.Equal(t, storage.AuthoringStage(authoring.StageShape), spec.Stage)
		require.Equal(t, int32(2), spec.Version, "version should increment after amendment")
	})

	t.Run("AmendSpec_AlreadyApproved", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "approved-spec", "Will be approved", "p1", "low")
		require.NoError(t, err)
		require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
		require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))
		require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageDecompose)))
		require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)))

		_, err = store.AmendSpec(ctx, "approved-spec", "too late", storage.AuthoringStage(authoring.StageShape))
		require.ErrorIs(t, err, storage.ErrSpecAlreadyApproved)
	})

	t.Run("AmendSpec_InvalidTransition", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "amend-invalid", "Invalid amend", "p1", "low")
		require.NoError(t, err)
		require.NoError(t, store.TransitionStage(ctx, "amend-invalid", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))

		// Amend forward (shape → specify) should fail — amend only allows backward.
		_, err = store.AmendSpec(ctx, "amend-invalid", "forward not allowed", storage.AuthoringStage(authoring.StageSpecify))
		require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
	})

	t.Run("AmendSpec_NotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		// AmendSpec on a non-existent slug should return ErrSpecNotFound.
		_, err = store.AmendSpec(ctx, "nonexistent-spec", "reason", storage.AuthoringStage(authoring.StageShape))
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("AmendSpec_EmptyReason", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "empty-reason", "Empty reason test", "p1", "low")
		require.NoError(t, err)
		require.NoError(t, store.TransitionStage(ctx, "empty-reason", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
		require.NoError(t, store.TransitionStage(ctx, "empty-reason", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))

		// Amend with empty reason should still succeed at the storage layer
		// (validation is the handler's responsibility), but verify the operation works.
		result, err := store.AmendSpec(ctx, "empty-reason", "", storage.AuthoringStage(authoring.StageShape))
		require.NoError(t, err)
		require.Equal(t, storage.AuthoringStage(authoring.StageShape), result.Stage)
	})

	t.Run("SupersedeSpec", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "old-spec", "Original spec", "p1", "low")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-spec", "Replacement spec", "p1", "low")
		require.NoError(t, err)

		err = store.SupersedeSpec(ctx, "old-spec", "new-spec", "better approach found")
		require.NoError(t, err)

		// Verify the old spec is now at stage "superseded".
		old, err := store.GetSpec(ctx, "old-spec")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, old.Stage)
	})

	t.Run("SupersedeSpec_NotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "existing-spec", "Exists", "p1", "low")
		require.NoError(t, err)

		// Non-existent old spec.
		err = store.SupersedeSpec(ctx, "nonexistent", "existing-spec", "reason")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)

		// Non-existent new spec.
		err = store.SupersedeSpec(ctx, "existing-spec", "nonexistent", "reason")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("StoreDecomposeOutput_Idempotent", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "idem-parent", "Idempotency parent", "p1", "medium")
		require.NoError(t, err)

		output := &storage.DecomposeOutput{
			Strategy: storage.StrategyVerticalSlice,
			Slices: []storage.DecomposeSlice{
				{ID: "slice-a", Intent: "First slice", Verify: []string{"a works"}, Touches: []string{"a.go"}},
				{ID: "slice-b", Intent: "Second slice", Verify: []string{"b works"}, Touches: []string{"b.go"}, DependsOn: []string{"slice-a"}},
			},
		}

		// First call.
		children1, err := store.StoreDecomposeOutput(ctx, "idem-parent", output)
		require.NoError(t, err)
		require.Len(t, children1, 2)
		require.Equal(t, "idem-parent/slice-a", children1[0])
		require.Equal(t, "idem-parent/slice-b", children1[1])

		// Second call with identical data — must succeed and return the same slugs.
		children2, err := store.StoreDecomposeOutput(ctx, "idem-parent", output)
		require.NoError(t, err)
		require.Len(t, children2, 2)
		require.Equal(t, children1[0], children2[0])
		require.Equal(t, children1[1], children2[1])
	})

	t.Run("StoreDecomposeOutput_MissingParent", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		// Do not create the parent spec — slug does not exist in the database.
		_, err = store.StoreDecomposeOutput(ctx, "ghost-parent", &storage.DecomposeOutput{
			Strategy: storage.StrategyVerticalSlice,
			Slices: []storage.DecomposeSlice{
				{ID: "slice-x", Intent: "Orphan slice", Verify: []string{"x works"}, Touches: []string{"x.go"}},
			},
		})
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("StoreDecomposeOutput_DuplicateSliceID", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "dup-parent", "Duplicate slice test", "p1", "low")
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
	})

	t.Run("StoreDecomposeOutput_UnknownDependency", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "dep-parent", "Unknown dep test", "p1", "low")
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
	})

	t.Run("TransitionStage_BackwardViaAmend", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "backward-test", "Backward transition", "p1", "low")
		require.NoError(t, err)

		// Advance spark → shape → specify.
		require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
		require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))

		// AmendSpec back to spark (two stages back).
		result, err := store.AmendSpec(ctx, "backward-test", "starting over", storage.AuthoringStage(authoring.StageSpark))
		require.NoError(t, err)
		require.Equal(t, storage.AuthoringStage(authoring.StageSpark), result.Stage)
		require.Equal(t, int32(2), result.Version)

		// After amend, can transition forward again from spark.
		require.NoError(t, store.TransitionStage(ctx, "backward-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
	})

	t.Run("TransitionStage_ApprovedGuard", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "approved-guard", "Approved guard test", "p1", "low")
		require.NoError(t, err)

		// Full forward path to approved.
		require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
		require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))
		require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageDecompose)))
		require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)))

		// Once approved, further forward transitions should fail with ErrSpecAlreadyApproved.
		err = store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageApproved), storage.AuthoringStage(authoring.StageSpark))
		require.ErrorIs(t, err, storage.ErrSpecAlreadyApproved)
	})

	t.Run("TransitionStage_SupersededGuard", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "superseded-old", "Will be superseded", "p1", "low")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "superseded-new", "Replacement", "p1", "low")
		require.NoError(t, err)

		// Mark the old spec as superseded.
		err = store.SupersedeSpec(ctx, "superseded-old", "superseded-new", "better approach")
		require.NoError(t, err)

		// Attempting TransitionStage on a superseded spec should fail because
		// "superseded" is not a valid funnel stage and ValidateTransition rejects it.
		err = store.TransitionStage(ctx, "superseded-old", storage.AuthoringStage("superseded"), storage.AuthoringStage(authoring.StageShape))
		require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
	})

	t.Run("StoreShapeOutput_CreatesDecisionNodes", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "shape-decisions-test", "test spec", "p1", "medium")
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

		// Verify DECIDED_IN edge exists with correct direction: spec→decision (ADR-003).
		edges, err := store.ListEdges(ctx, "shape-decisions-test", storage.EdgeTypeDecidedIn)
		require.NoError(t, err)
		require.Len(t, edges, 1)
		assert.Equal(t, storage.EdgeTypeDecidedIn, edges[0].EdgeType)
		assert.Equal(t, "shape-decisions-test", edges[0].FromID)
		assert.Equal(t, "use-memgraph", edges[0].ToID)
	})

	t.Run("StoreShapeOutput_IdempotentDecisions", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "idempotent-test", "test spec", "p1", "medium")
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
	})

	t.Run("StoreSafetyFlags", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "safety-flags-spec", "Safety flags test", "p1", "low")
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

		// Verify the safety_flags property was persisted on the spec node by
		// querying Memgraph directly with a raw Cypher query.
		driver, err := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
		require.NoError(t, err)
		defer driver.Close(ctx) //nolint:errcheck

		result, err := neo4j.ExecuteQuery(ctx, driver,
			`MATCH (s:Spec {slug: $slug}) RETURN s.safety_flags`,
			map[string]any{"slug": "safety-flags-spec"},
			neo4j.EagerResultTransformer,
		)
		require.NoError(t, err)
		require.Len(t, result.Records, 1, "spec node should exist")

		rawJSON, ok := result.Records[0].Values[0].(string)
		require.True(t, ok, "safety_flags should be a JSON string on the spec node")
		require.NotEmpty(t, rawJSON)

		var persisted []storage.SafetyFlag
		require.NoError(t, json.Unmarshal([]byte(rawJSON), &persisted))
		require.Len(t, persisted, 2)
		require.Equal(t, storage.SafetyCategory("security"), persisted[0].Category)
		require.Equal(t, storage.SeverityCritical, persisted[0].Severity)
		require.Equal(t, "Spec requests unrestricted filesystem access", persisted[0].Description)
		require.Equal(t, storage.SafetyCategory("privacy"), persisted[1].Category)
		require.Equal(t, storage.SeverityWarning, persisted[1].Severity)
	})

	t.Run("StoreSafetyFlags_SpecNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		err = store.StoreSafetyFlags(ctx, "nonexistent-spec", []storage.SafetyFlag{
			{Category: storage.SafetyCategory("security"), Severity: storage.SeverityCritical, Description: "test"},
		})
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("StoreSparkOutput_UpdatesContentHash", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

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
	})

	t.Run("TransitionStage_UpdatesContentHash", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		spec, err := store.CreateSpec(ctx, "stage-hash", "Test", "p1", "medium")
		require.NoError(t, err)
		initialHash := spec.ContentHash

		err = store.TransitionStage(ctx, "stage-hash",
			storage.AuthoringStage(authoring.StageSpark),
			storage.AuthoringStage(authoring.StageShape))
		require.NoError(t, err)

		updated, err := store.GetSpec(ctx, "stage-hash")
		require.NoError(t, err)
		require.NotEqual(t, initialHash, updated.ContentHash)
		require.NotEmpty(t, updated.ContentHash)
	})

	t.Run("AmendSpec_UpdatesContentHash", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		_, err = store.CreateSpec(ctx, "amend-hash", "Test", "p1", "medium")
		require.NoError(t, err)

		// Advance to shape so we can amend back to spark.
		err = store.TransitionStage(ctx, "amend-hash",
			storage.AuthoringStage(authoring.StageSpark),
			storage.AuthoringStage(authoring.StageShape))
		require.NoError(t, err)

		preAmend, err := store.GetSpec(ctx, "amend-hash")
		require.NoError(t, err)

		_, err = store.AmendSpec(ctx, "amend-hash", "rework needed",
			storage.AuthoringStage(authoring.StageSpark))
		require.NoError(t, err)

		updated, err := store.GetSpec(ctx, "amend-hash")
		require.NoError(t, err)
		require.NotEqual(t, preAmend.ContentHash, updated.ContentHash)
		require.NotEmpty(t, updated.ContentHash)
	})

	t.Run("StoreRedTeamFindings_DoesNotUpdateContentHash", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx) //nolint:errcheck

		spec, err := store.CreateSpec(ctx, "analytical-hash", "Test", "p1", "medium")
		require.NoError(t, err)
		initialHash := spec.ContentHash

		err = store.StoreRedTeamFindings(ctx, "analytical-hash", []storage.RedTeamFinding{
			{Severity: storage.SeverityCritical, Finding: "test finding"},
		})
		require.NoError(t, err)

		updated, err := store.GetSpec(ctx, "analytical-hash")
		require.NoError(t, err)
		assert.Equal(t, initialHash, updated.ContentHash)
	})
}
