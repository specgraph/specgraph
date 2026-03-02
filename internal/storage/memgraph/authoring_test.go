// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestTransitionStage(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low")
	require.NoError(t, err)

	// CreateSpec sets stage to "spark", so transition spark → shape.
	err = store.TransitionStage(ctx, "funnel-test", "spark", "shape")
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "funnel-test")
	require.NoError(t, err)
	require.Equal(t, "shape", spec.Stage)

	err = store.TransitionStage(ctx, "funnel-test", "shape", "specify")
	require.NoError(t, err)

	// Invalid: skipping from specify straight to approved (must go through decompose).
	err = store.TransitionStage(ctx, "funnel-test", "specify", "approved")
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestStoreSparkOutput(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "spark-out", "Spark output test", "p1", "low")
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "spark-out", &specv1.SparkOutput{
		Seed:       "Build a login system",
		Signal:     "User request",
		Questions:  []string{"OAuth or password?", "MFA required?"},
		ScopeSniff: "medium",
		KillTest:   "If no users need auth",
	})
	require.NoError(t, err)
}

func TestStoreDecomposeOutput(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "decomp-parent", "Parent spec", "p1", "medium")
	require.NoError(t, err)

	children, err := store.StoreDecomposeOutput(ctx, "decomp-parent", &specv1.DecomposeOutput{
		Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
		Slices: []*specv1.DecompositionSlice{
			{Id: "slice-1", Intent: "Auth endpoint", Verify: []string{"login works"}, Touches: []string{"auth.go"}},
			{Id: "slice-2", Intent: "Token refresh", Verify: []string{"refresh works"}, Touches: []string{"token.go"}, DependsOn: []string{"slice-1"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, children, 2)
	require.Equal(t, "slice-1", children[0].Slug)
	require.Equal(t, "slice-2", children[1].Slug)
}

func TestAmendSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "amend-test", "Amend test", "p1", "low")
	require.NoError(t, err)
	// CreateSpec sets stage to "spark". Advance through stages.
	require.NoError(t, store.TransitionStage(ctx, "amend-test", "spark", "shape"))
	require.NoError(t, store.TransitionStage(ctx, "amend-test", "shape", "specify"))

	// Amend back to shape (valid backward transition).
	spec, err := store.AmendSpec(ctx, "amend-test", "need to reconsider scope", "shape")
	require.NoError(t, err)
	require.Equal(t, "shape", spec.Stage)
	require.Equal(t, int32(2), spec.Version, "version should increment after amendment")
}

func TestSupersedeSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "old-spec", "Original spec", "p1", "low")
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
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "existing-spec", "Exists", "p1", "low")
	require.NoError(t, err)

	// Non-existent old spec.
	err = store.SupersedeSpec(ctx, "nonexistent", "existing-spec", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)

	// Non-existent new spec.
	err = store.SupersedeSpec(ctx, "existing-spec", "nonexistent", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}
