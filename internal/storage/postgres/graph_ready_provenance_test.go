// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestGetReady_ProvenanceAndStageFilters asserts that GetReady returns only
// AUTHORED specs at stage=approved with no active claim. Specs in terminal
// stages (done, superseded, abandoned), non-approved stages (spark), or
// non-AUTHORED provenance (declared) must all be excluded.
func TestGetReady_ProvenanceAndStageFilters(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// 1. AUTHORED + approved — the one that must appear in GetReady.
	_, err := store.CreateSpec(ctx, "a-authored-approved", "authored approved", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	approvedStage := string(storage.SpecStageApproved)
	_, err = store.UpdateSpec(ctx, "a-authored-approved", nil, &approvedStage, nil, nil, nil)
	require.NoError(t, err)

	// 2. AUTHORED + spark — not ready (wrong stage).
	_, err = store.CreateSpec(ctx, "b-authored-spark", "authored spark", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// 3. AUTHORED + done — not ready (terminal, wrong stage).
	_, err = store.CreateSpec(ctx, "c-authored-done", "authored done", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	doneStage := string(storage.SpecStageDone)
	_, err = store.UpdateSpec(ctx, "c-authored-done", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	// 4. DECLARED + done — not ready (wrong provenance type).
	_, err = store.CreateSpec(ctx, "d-declared-done", "declared done", "p1", "medium",
		storage.SpecProvenanceDeclared,
		storage.SpecProvenanceDetail{
			Declared: &storage.DeclaredProvenance{DeclaredBy: "platform-team"},
		},
		nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "d-declared-done", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	// 4b. DECLARED + approved — not ready (wrong provenance type even at the
	// correct stage). This fixture isolates the provenance filter: if the
	// stage=approved predicate were the only gate, this would incorrectly
	// appear in ready.
	_, err = store.CreateSpec(ctx, "d2-declared-approved", "declared at approved", "p1", "medium",
		storage.SpecProvenanceDeclared,
		storage.SpecProvenanceDetail{
			Declared: &storage.DeclaredProvenance{DeclaredBy: "platform-team"},
		},
		nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "d2-declared-approved", nil, &approvedStage, nil, nil, nil)
	require.NoError(t, err)

	// 5. AUTHORED + superseded — not ready (fully terminal).
	_, err = store.CreateSpec(ctx, "e-authored-superseded-old", "to be superseded", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "e-authored-superseded-old", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "e-authored-superseded-new", "superseding spec", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, _, err = store.LifecycleSupersedeSpec(ctx, "e-authored-superseded-old", "e-authored-superseded-new", "")
	require.NoError(t, err)

	// 6. AUTHORED + abandoned — not ready (fully terminal).
	_, err = store.CreateSpec(ctx, "f-authored-abandoned", "authored abandoned", "p1", "medium",
		storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.LifecycleAbandonSpec(ctx, "f-authored-abandoned", "no longer needed")
	require.NoError(t, err)

	// GetReady must return exactly one slug: a-authored-approved.
	refs, err := store.GetReady(ctx)
	require.NoError(t, err)

	slugs := make([]string, 0, len(refs))
	for _, r := range refs {
		slugs = append(slugs, r.Slug)
	}

	require.Len(t, slugs, 1, "expected exactly one ready spec, got: %v", slugs)
	require.Equal(t, "a-authored-approved", slugs[0])
}
