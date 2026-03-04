// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

// newTestStore creates a fresh Memgraph-backed store for a single test,
// registering cleanup of the container and store connection.
func newTestStore(t *testing.T) (*memgraph.Store, context.Context) {
	t.Helper()
	boltURI, cleanup := setupMemgraph(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close(ctx) })
	return store, ctx
}

func TestTransitionStage(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low")
	require.NoError(t, err)

	// CreateSpec sets stage to "spark", so transition spark → shape.
	err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape))
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "funnel-test")
	require.NoError(t, err)
	require.Equal(t, authoring.StageShape, spec.Stage)

	err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
	require.NoError(t, err)

	// Invalid: skipping from specify straight to approved (must go through decompose).
	err = store.TransitionStage(ctx, "funnel-test", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageApproved))
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestTransitionStage_WrongStage(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "wrong-stage", "Wrong stage test", "p1", "low")
	require.NoError(t, err)

	// Spec is at "spark", but we claim it's at "shape" → should fail.
	err = store.TransitionStage(ctx, "wrong-stage", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestStoreSparkOutput(t *testing.T) {
	store, ctx := newTestStore(t)

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
}

func TestStoreDecomposeOutput(t *testing.T) {
	store, ctx := newTestStore(t)

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
}

func TestAmendSpec(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-test", "Amend test", "p1", "low")
	require.NoError(t, err)
	// CreateSpec sets stage to "spark". Advance through stages.
	require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
	require.NoError(t, store.TransitionStage(ctx, "amend-test", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))

	// Amend back to shape (valid backward transition).
	spec, err := store.AmendSpec(ctx, "amend-test", "need to reconsider scope", storage.AuthoringStage(authoring.StageShape))
	require.NoError(t, err)
	require.Equal(t, storage.AuthoringStage(authoring.StageShape), spec.Stage)
	require.Equal(t, int32(2), spec.Version, "version should increment after amendment")
}

func TestAmendSpec_AlreadyApproved(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "approved-spec", "Will be approved", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageDecompose)))
	require.NoError(t, store.TransitionStage(ctx, "approved-spec", storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)))

	_, err = store.AmendSpec(ctx, "approved-spec", "too late", storage.AuthoringStage(authoring.StageShape))
	require.ErrorIs(t, err, storage.ErrSpecAlreadyApproved)
}

func TestAmendSpec_InvalidTransition(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-invalid", "Invalid amend", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "amend-invalid", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))

	// Amend forward (shape → specify) should fail — amend only allows backward.
	_, err = store.AmendSpec(ctx, "amend-invalid", "forward not allowed", storage.AuthoringStage(authoring.StageSpecify))
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestSupersedeSpec(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "old-spec", "Original spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "new-spec", "Replacement spec", "p1", "low")
	require.NoError(t, err)

	err = store.SupersedeSpec(ctx, "old-spec", "new-spec", "better approach found")
	require.NoError(t, err)

	// Verify the old spec is now at stage "superseded".
	old, err := store.GetSpec(ctx, "old-spec")
	require.NoError(t, err)
	require.Equal(t, "superseded", old.Stage)
}

func TestSupersedeSpec_NotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "existing-spec", "Exists", "p1", "low")
	require.NoError(t, err)

	// Non-existent old spec.
	err = store.SupersedeSpec(ctx, "nonexistent", "existing-spec", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)

	// Non-existent new spec.
	err = store.SupersedeSpec(ctx, "existing-spec", "nonexistent", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestStoreDecomposeOutput_Idempotent(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "idem-parent", "Idempotency parent", "p1", "medium")
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
}

func TestStoreDecomposeOutput_MissingParent(t *testing.T) {
	store, ctx := newTestStore(t)

	// Do not create the parent spec — slug does not exist in the database.
	_, err := store.StoreDecomposeOutput(ctx, "ghost-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-x", Intent: "Orphan slice", Verify: []string{"x works"}, Touches: []string{"x.go"}},
		},
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestStoreDecomposeOutput_DuplicateSliceID(t *testing.T) {
	store, ctx := newTestStore(t)

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
	store, ctx := newTestStore(t)

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

func TestTransitionStage_BackwardViaAmend(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "backward-test", "Backward transition", "p1", "low")
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
}

func TestTransitionStage_ApprovedGuard(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "approved-guard", "Approved guard test", "p1", "low")
	require.NoError(t, err)

	// Full forward path to approved.
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape)))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify)))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageDecompose)))
	require.NoError(t, store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)))

	// Once approved, further forward transitions should fail.
	err = store.TransitionStage(ctx, "approved-guard", storage.AuthoringStage(authoring.StageApproved), storage.AuthoringStage(authoring.StageSpark))
	require.Error(t, err)
}
