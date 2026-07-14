// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specgraph/specgraph/internal/drift"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/stretchr/testify/require"
)

func TestLifecycle(t *testing.T) {
	t.Run("AmendSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		inProgressStage := "in_progress"
		_, err = store.UpdateSpec(ctx, "amend-me", nil, &inProgressStage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
		require.NoError(t, err)
		// re-entry "shape" lands at "spark" so that `specgraph shape` (spark→shape) succeeds.
		require.Equal(t, storage.SpecStageSpark, amended.Stage)
		require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3

		// Verify landing stage was persisted, not just returned in-memory.
		fetched, err := store.GetSpec(ctx, "amend-me")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSpark, fetched.Stage)
		require.Equal(t, int32(3), fetched.Version)
	})

	t.Run("AmendSpec_RequiresReEntryStage", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-noreentry", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		inProgressStage := "in_progress"
		_, err = store.UpdateSpec(ctx, "amend-noreentry", nil, &inProgressStage, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.LifecycleAmendSpec(ctx, "amend-noreentry", "needs rework", "")
		require.ErrorIs(t, err, storage.ErrReEntryStageRequired)
	})

	t.Run("AmendSpec_NotAmendable_Spark", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "not-amendable-spark", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		// Spec is at "spark" -- not amend-eligible, so amend should fail.
		_, err = store.LifecycleAmendSpec(ctx, "not-amendable-spark", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotAmendable)
	})

	t.Run("AmendSpec_NotAmendable_Done", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "not-amendable-done", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "not-amendable-done", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		// Spec is at "done" -- not amend-eligible, so amend should fail.
		_, err = store.LifecycleAmendSpec(ctx, "not-amendable-done", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotAmendable)
	})

	t.Run("AmendSpec_NotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.LifecycleAmendSpec(ctx, "nonexistent", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("AmendSpec_Terminal", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-terminal", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		// Abandon the spec to make it terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "amend-terminal", "abandoned")
		require.NoError(t, err)

		// Amend should fail -- spec is in a terminal state.
		_, err = store.LifecycleAmendSpec(ctx, "amend-terminal", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("AmendSpec_AllEligibleStages", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		for _, stage := range []string{"approved", "in_progress", "review"} {
			slug := "amend-eligible-" + stage
			_, err := store.CreateSpec(ctx, slug, "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
			require.NoError(t, err)
			_, err = store.UpdateSpec(ctx, slug, nil, &stage, nil, nil, nil)
			require.NoError(t, err)

			amended, err := store.LifecycleAmendSpec(ctx, slug, "needs rework", "shape")
			require.NoError(t, err, "amend from %q should succeed", stage)
			// re-entry "shape" lands at "spark" so that `specgraph shape` (spark→shape) succeeds.
			require.Equal(t, storage.SpecStageSpark, amended.Stage, "stage %q", stage)
		}
	})

	t.Run("AmendSpec_VersionGuard", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "toctou-amend", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		inProgressStage := "in_progress"
		_, err = store.UpdateSpec(ctx, "toctou-amend", nil, &inProgressStage, nil, nil, nil)
		require.NoError(t, err)

		// Two goroutines race to amend the same spec.
		const n = 2
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			go func() {
				_, aErr := store.LifecycleAmendSpec(ctx, "toctou-amend", "concurrent amend", "shape")
				errs <- aErr
			}()
		}

		var succeeded, failed int
		for i := 0; i < n; i++ {
			if e := <-errs; e != nil {
				failed++
				require.True(t,
					errors.Is(e, storage.ErrConcurrentModification) || errors.Is(e, storage.ErrSpecNotAmendable),
					"unexpected error: %v", e)
			} else {
				succeeded++
			}
		}
		require.Equal(t, 1, succeeded, "exactly one amend should succeed")
		require.Equal(t, 1, failed, "exactly one amend should fail")
	})

	t.Run("AmendSpec_InvalidReEntryStage", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		// Beyond the terminal stages (done/superseded/abandoned), the D-03
		// allowlist also rejects the non-terminal approved/in_progress/review
		// stages — defense-in-depth against a storage-only revert to the weak
		// ExcludesReEntry guard.
		for _, stage := range []string{"done", "superseded", "abandoned", "approved", "in_progress", "review"} {
			slug := "amend-reentry-" + stage
			_, err := store.CreateSpec(ctx, slug, "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
			require.NoError(t, err)
			inProgressStage := "in_progress"
			_, err = store.UpdateSpec(ctx, slug, nil, &inProgressStage, nil, nil, nil)
			require.NoError(t, err)

			_, err = store.LifecycleAmendSpec(ctx, slug, "reason", stage)
			require.ErrorIs(t, err, storage.ErrInvalidReEntryStage, "stage %q should be rejected", stage)
		}
	})

	t.Run("SupersedeSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "old-lifecycle", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		old, newSpec, err := store.LifecycleSupersedeSpec(ctx, "old-lifecycle", "new-lifecycle", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, old.Stage)
		require.Equal(t, "new-lifecycle", old.SupersededBy)
		require.Equal(t, "old-lifecycle", newSpec.Supersedes)
	})

	t.Run("SupersedeSpec_EdgePersists", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "edge-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "edge-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "edge-new", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "edge-old", "edge-new", "")
		require.NoError(t, err)

		// Verify SUPERSEDES edge was written.
		edges, err := store.ListEdges(ctx, "edge-new", storage.EdgeTypeSupersedes)
		require.NoError(t, err)
		require.Len(t, edges, 1, "expected one SUPERSEDES edge from new to old")
		require.Equal(t, "edge-new", edges[0].FromID)
		require.Equal(t, "edge-old", edges[0].ToID)
	})

	t.Run("SupersedeSpec_OldNotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "exists-new", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "nonexistent-old", "exists-new", "")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("SupersedeSpec_NewNotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "exists-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "exists-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "exists-old", "nonexistent-new", "")
		require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
	})

	t.Run("SupersedeSpec_NotDone", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "not-done-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "not-done-new", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		// Old spec is at spark -- not done, supersede should fail.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "not-done-old", "not-done-new", "")
		require.ErrorIs(t, err, storage.ErrSpecNotDone)
	})

	t.Run("SupersedeSpec_TerminalState", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "terminal-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "replacement", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		// Abandon the old spec first to make it terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "terminal-old", "abandoned")
		require.NoError(t, err)

		// Supersede should fail because old spec is not done.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "terminal-old", "replacement", "")
		require.ErrorIs(t, err, storage.ErrSpecNotDone)
	})

	t.Run("SupersedeSpec_NewTerminal", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "old-sup-aband", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "old-sup-aband", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-sup-aband", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.LifecycleAbandonSpec(ctx, "new-sup-aband", "no longer needed")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "old-sup-aband", "new-sup-aband", "")
		require.ErrorIs(t, err, storage.ErrNewSpecTerminal)
	})

	t.Run("SupersedeSpec_SameSlug", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "same-slug", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "same-slug", "same-slug", "")
		require.ErrorIs(t, err, storage.ErrSameSlugs)
	})

	t.Run("SupersedeSpec_ReasonThreaded", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		doneStage := "done"

		// Supplied reason lands verbatim on the old spec's changelog entry.
		_, err := store.CreateSpec(ctx, "reason-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "reason-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "reason-new", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		const suppliedReason = "replaced by clearer design"
		_, _, err = store.LifecycleSupersedeSpec(ctx, "reason-old", "reason-new", suppliedReason)
		require.NoError(t, err)
		require.Equal(t, suppliedReason, latestSupersededReason(t, store, ctx, "reason-old"))

		// Empty reason falls back to the default "Superseded by <new>" note.
		_, err = store.CreateSpec(ctx, "default-old", "Old spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "default-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "default-new", "New spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "default-old", "default-new", "")
		require.NoError(t, err)
		require.Equal(t, "Superseded by default-new", latestSupersededReason(t, store, ctx, "default-old"))
	})

	t.Run("AbandonSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "abandon-me", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		abandoned, err := store.LifecycleAbandonSpec(ctx, "abandon-me", "no longer needed")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
		require.Equal(t, int32(2), abandoned.Version) // create=1, abandon=2
	})

	t.Run("AbandonSpec_Terminal", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "abandon-twice", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "first abandon")
		require.NoError(t, err)

		// Second abandon should fail -- already terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "second abandon")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("AbandonSpec_NotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.LifecycleAbandonSpec(ctx, "nonexistent-spec", "reason")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("AbandonSpec_VersionGuard", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "toctou-abandon", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		// Two goroutines race to abandon the same spec.
		const n = 2
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			go func() {
				_, aErr := store.LifecycleAbandonSpec(ctx, "toctou-abandon", "concurrent abandon")
				errs <- aErr
			}()
		}

		var succeeded, failed int
		for i := 0; i < n; i++ {
			if e := <-errs; e != nil {
				failed++
				require.True(t,
					errors.Is(e, storage.ErrConcurrentModification) || errors.Is(e, storage.ErrSpecTerminal),
					"unexpected error: %v", e)
			} else {
				succeeded++
			}
		}
		require.Equal(t, 1, succeeded, "exactly one abandon should succeed")
		require.Equal(t, 1, failed, "exactly one abandon should fail")
	})

	t.Run("AcknowledgeDrift_Basic", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		// Create upstream and downstream, link with DEPENDS_ON.
		_, err := store.CreateSpec(ctx, "ack-upstream", "Upstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "ack-drift", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "ack-drift", "ack-upstream", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "ack-drift", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Modify upstream to create drift.
		newIntent := "Changed upstream"
		_, err = store.UpdateSpec(ctx, "ack-upstream", &newIntent, nil, nil, nil, nil)
		require.NoError(t, err)
		upstream, err := store.GetSpec(ctx, "ack-upstream")
		require.NoError(t, err)

		// Acknowledge drift for specific upstream.
		err = store.LifecycleAcknowledgeDrift(ctx, "ack-drift", "ack-upstream", "drift is intentional")
		require.NoError(t, err)

		// Verify edge hash updated to upstream's current hash.
		deps, err := store.GetDependenciesWithEdgeData(ctx, "ack-drift")
		require.NoError(t, err)
		require.Len(t, deps, 1)
		require.Equal(t, upstream.ContentHash, deps[0].ContentHashAtLink)
	})

	t.Run("AcknowledgeDrift_AllUpstreams", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "all-up1", "Upstream 1", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-up2", "Upstream 2", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-down", "Downstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.AddEdge(ctx, "all-down", "all-up1", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "all-down", "all-up2", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "all-down", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Modify both upstreams.
		intent1 := "Changed upstream 1"
		_, err = store.UpdateSpec(ctx, "all-up1", &intent1, nil, nil, nil, nil)
		require.NoError(t, err)
		intent2 := "Changed upstream 2"
		_, err = store.UpdateSpec(ctx, "all-up2", &intent2, nil, nil, nil, nil)
		require.NoError(t, err)

		up1, err := store.GetSpec(ctx, "all-up1")
		require.NoError(t, err)
		up2, err := store.GetSpec(ctx, "all-up2")
		require.NoError(t, err)

		// Blanket ack.
		err = store.LifecycleAcknowledgeDrift(ctx, "all-down", "", "blanket ack")
		require.NoError(t, err)

		deps, err := store.GetDependenciesWithEdgeData(ctx, "all-down")
		require.NoError(t, err)
		require.Len(t, deps, 2)
		hashes := map[string]string{}
		for _, d := range deps {
			hashes[d.Slug] = d.ContentHashAtLink
		}
		require.Equal(t, up1.ContentHash, hashes["all-up1"])
		require.Equal(t, up2.ContentHash, hashes["all-up2"])
	})

	t.Run("AcknowledgeDrift_IneligibleStage", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		// Spec at spark stage is not eligible.
		_, err := store.CreateSpec(ctx, "ack-ineligible", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		err = store.LifecycleAcknowledgeDrift(ctx, "ack-ineligible", "", "should fail")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecIneligibleStage)
	})

	t.Run("AcknowledgeDrift_NotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		err := store.LifecycleAcknowledgeDrift(ctx, "nonexistent-spec", "", "should fail")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("CheckAllSpecs_MixedState_SkippedCount", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		doneStage := "done"

		// (a) Drifted-done pair: downstream done, then upstream mutated so its
		// content hash diverges from the edge's baselined content_hash_at_link.
		_, err := store.CreateSpec(ctx, "mix-drift-up", "Upstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "mix-drift-down", "Downstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "mix-drift-down", "mix-drift-up", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "mix-drift-down", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		driftIntent := "Changed upstream intent to diverge the content hash"
		_, err = store.UpdateSpec(ctx, "mix-drift-up", &driftIntent, nil, nil, nil, nil)
		require.NoError(t, err)

		// (b) Clean-done pair: downstream done, upstream left untouched (no drift).
		_, err = store.CreateSpec(ctx, "mix-clean-up", "Upstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "mix-clean-down", "Downstream", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "mix-clean-down", "mix-clean-up", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "mix-clean-down", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// (c) At least one NON-done spec left at spark → counted as skipped in
		// all-specs mode.
		_, err = store.CreateSpec(ctx, "mix-skipped", "Skipped", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		// Full-graph check (empty slug): non-done specs are skipped; zero-item
		// reports are filtered out, so only the drifted downstream should surface.
		engine := drift.NewEngine(store, nil)
		result, err := engine.Check(ctx, "", "deps")
		require.NoError(t, err)
		require.GreaterOrEqual(t, result.SkippedCount, int32(1),
			"non-done specs (e.g. mix-skipped) must be counted as skipped")

		reportsBySlug := map[string]storage.DriftReport{}
		for _, r := range result.Reports {
			reportsBySlug[r.SpecSlug] = r
		}

		driftReport, ok := reportsBySlug["mix-drift-down"]
		require.True(t, ok, "the drifted downstream must appear in the reports")
		require.NotEmpty(t, driftReport.Items, "mix-drift-down should have drift items")

		// The clean-done spec has zero drift items, so it is filtered out of the
		// response entirely.
		cleanReport, ok := reportsBySlug["mix-clean-down"]
		if ok {
			require.Empty(t, cleanReport.Items, "mix-clean-down should have no drift items")
		}
	})
}

func TestLifecycle_AmendRefreshesEdgeHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create upstream and downstream specs.
	_, err := store.CreateSpec(ctx, "upstream-hash", "Upstream spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-hash", "Downstream spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// Link downstream -> upstream with a DEPENDS_ON edge.
	_, err = store.AddEdge(ctx, "downstream-hash", "upstream-hash", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Advance upstream to in_progress so it can be amended.
	_, err = store.UpdateSpec(ctx, "upstream-hash", nil, strPtr("in_progress"), nil, nil, nil)
	require.NoError(t, err)

	// Record the hash after advancing to in_progress — this is our baseline for detecting change.
	// We cannot use the initial edge hash because amend now lands at "spark" (the stage before
	// the re-entry target), which produces the same hash as the spec had at creation time.
	upstreamBeforeAmend, err := store.GetSpec(ctx, "upstream-hash")
	require.NoError(t, err)
	hashBeforeAmend := upstreamBeforeAmend.ContentHash

	// Also record the edge hash for the final refresh assertion.
	initialDeps, err := store.GetDependenciesWithEdgeData(ctx, "downstream-hash")
	require.NoError(t, err)
	require.Len(t, initialDeps, 1)
	initialHashAtLink := initialDeps[0].ContentHashAtLink

	// Amend the upstream spec — re-entry "shape" lands at "spark", changing its content hash.
	_, err = store.LifecycleAmendSpec(ctx, "upstream-hash", "needs revision", "shape")
	require.NoError(t, err)

	// Confirm upstream's content hash changed from its in_progress value.
	upstream, err := store.GetSpec(ctx, "upstream-hash")
	require.NoError(t, err)
	require.NotEqual(t, hashBeforeAmend, upstream.ContentHash, "amend should change the upstream content hash")

	// Advance upstream back to done — this should refresh the edge hash.
	_, err = store.UpdateSpec(ctx, "upstream-hash", nil, strPtr("done"), nil, nil, nil)
	require.NoError(t, err)

	// Verify the edge hash was refreshed.
	refreshedDeps, err := store.GetDependenciesWithEdgeData(ctx, "downstream-hash")
	require.NoError(t, err)
	require.Len(t, refreshedDeps, 1)
	require.NotEqual(t, initialHashAtLink, refreshedDeps[0].ContentHashAtLink,
		"ContentHashAtLink should be refreshed after upstream is re-completed")
}

// latestSupersededReason returns the Reason on the most recent "superseded"
// changelog entry for slug. Fails the test if no such entry exists.
func latestSupersededReason(t *testing.T, store *postgres.Store, ctx context.Context, slug string) string {
	t.Helper()
	entries, err := store.ListChanges(ctx, slug, storage.ChangeLogFilter{})
	require.NoError(t, err)
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Stage == string(storage.SpecStageSuperseded) {
			return entries[i].Reason
		}
	}
	require.Fail(t, "no superseded changelog entry found", "slug=%s", slug)
	return ""
}

// countClaimedByEdges returns the number of CLAIMED_BY edges from spec `slug`
// in the default test project, queried directly against the database.
func countClaimedByEdges(t *testing.T, ctx context.Context, slug string) int {
	t.Helper()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM edges
		 WHERE project_slug = 'test' AND from_slug = $1 AND edge_type = 'CLAIMED_BY'`,
		slug,
	).Scan(&count))
	return count
}

// TestLifecycleAmend_ReleasesClaim pins D-08: amending a claimed spec back to
// authoring deletes both the claims row and the CLAIMED_BY edge inside the amend
// transaction, and lands the spec one stage before the re-entry target. Amending
// an unclaimed spec is a harmless no-op with respect to claims.
func TestLifecycleAmend_ReleasesClaim(t *testing.T) {
	t.Run("ClaimedSpec_ReleasesClaimAndEdge", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "claimed-amend", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		inProgress := "in_progress"
		_, err = store.UpdateSpec(ctx, "claimed-amend", nil, &inProgress, nil, nil, nil)
		require.NoError(t, err)

		// Seed an active claim.
		_, err = store.ClaimSpec(ctx, "claimed-amend", "agent-1", 0)
		require.NoError(t, err)
		claim, err := store.GetActiveClaim(ctx, "claimed-amend")
		require.NoError(t, err)
		require.NotNil(t, claim, "precondition: spec must hold an active claim before amend")
		require.Equal(t, 1, countClaimedByEdges(t, ctx, "claimed-amend"), "precondition: CLAIMED_BY edge present")

		// Amend back to authoring at re-entry "shape" (lands at "spark").
		amended, err := store.LifecycleAmendSpec(ctx, "claimed-amend", "needs rework", "shape")
		require.NoError(t, err)

		// (a) Claim row gone.
		after, err := store.GetActiveClaim(ctx, "claimed-amend")
		require.NoError(t, err)
		require.Nil(t, after, "amend must release the active claim")

		// (b) CLAIMED_BY edge gone.
		require.Equal(t, 0, countClaimedByEdges(t, ctx, "claimed-amend"), "amend must delete the CLAIMED_BY edge")

		// (c) Spec landed one stage before the re-entry target.
		expected := storage.SpecStage("shape").PrecedingAuthStage()
		require.Equal(t, expected, amended.Stage)
		require.Equal(t, storage.SpecStageSpark, amended.Stage)
	})

	t.Run("UnclaimedSpec_NoErrorNoClaim", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "unclaimed-amend", "Test spec", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)
		approved := "approved"
		_, err = store.UpdateSpec(ctx, "unclaimed-amend", nil, &approved, nil, nil, nil)
		require.NoError(t, err)

		// No claim seeded — amend must be a harmless no-op w.r.t. claims.
		require.Nil(t, mustGetActiveClaim(t, store, ctx, "unclaimed-amend"))

		amended, err := store.LifecycleAmendSpec(ctx, "unclaimed-amend", "revisit approach", "specify")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStage("specify").PrecedingAuthStage(), amended.Stage)

		// Still no claim and no CLAIMED_BY edge.
		require.Nil(t, mustGetActiveClaim(t, store, ctx, "unclaimed-amend"))
		require.Equal(t, 0, countClaimedByEdges(t, ctx, "unclaimed-amend"))
	})
}

// mustGetActiveClaim fetches the active claim for slug, failing on error.
func mustGetActiveClaim(t *testing.T, store *postgres.Store, ctx context.Context, slug string) *storage.Claim {
	t.Helper()
	claim, err := store.GetActiveClaim(ctx, slug)
	require.NoError(t, err)
	return claim
}
