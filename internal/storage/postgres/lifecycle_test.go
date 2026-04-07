// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestLifecycle(t *testing.T) {
	t.Run("AmendSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-me", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStage("shape"), amended.Stage)
		require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3

		// Verify re-entry stage was persisted, not just returned in-memory.
		fetched, err := store.GetSpec(ctx, "amend-me")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStage("shape"), fetched.Stage)
		require.Equal(t, int32(3), fetched.Version)
	})

	t.Run("AmendSpec_DefaultToAmended", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-default", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-default", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, "amend-default", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)
	})

	t.Run("AmendSpec_NotDone", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "not-done", "Test spec", "p1", "medium")
		require.NoError(t, err)
		// Spec is at "spark" -- not done, so amend should fail.
		_, err = store.LifecycleAmendSpec(ctx, "not-done", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotDone)
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

		_, err := store.CreateSpec(ctx, "amend-terminal", "Test spec", "p1", "medium")
		require.NoError(t, err)

		// Abandon the spec to make it terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "amend-terminal", "abandoned")
		require.NoError(t, err)

		// Amend should fail -- spec is in a terminal state.
		_, err = store.LifecycleAmendSpec(ctx, "amend-terminal", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("AmendSpec_VersionGuard", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "toctou-amend", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "toctou-amend", nil, &doneStage, nil, nil, nil)
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
					errors.Is(e, storage.ErrConcurrentModification) || errors.Is(e, storage.ErrSpecNotDone),
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

		for _, stage := range []string{"done", "amended", "superseded", "abandoned"} {
			slug := "amend-reentry-" + stage
			_, err := store.CreateSpec(ctx, slug, "Test spec", "p1", "medium")
			require.NoError(t, err)
			doneStage := "done"
			_, err = store.UpdateSpec(ctx, slug, nil, &doneStage, nil, nil, nil)
			require.NoError(t, err)

			_, err = store.LifecycleAmendSpec(ctx, slug, "reason", stage)
			require.ErrorIs(t, err, storage.ErrInvalidReEntryStage, "stage %q should be rejected", stage)
		}
	})

	t.Run("SupersedeSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium")
		require.NoError(t, err)

		old, newSpec, err := store.LifecycleSupersedeSpec(ctx, "old-lifecycle", "new-lifecycle")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, old.Stage)
		require.Equal(t, "new-lifecycle", old.SupersededBy)
		require.Equal(t, "old-lifecycle", newSpec.Supersedes)
	})

	t.Run("SupersedeSpec_EdgePersists", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "edge-old", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "edge-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "edge-old", "edge-new")
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

		_, err := store.CreateSpec(ctx, "exists-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "nonexistent-old", "exists-new")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("SupersedeSpec_NewNotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "exists-old", "Old spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "exists-old", "nonexistent-new")
		require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
	})

	t.Run("SupersedeSpec_TerminalState", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "terminal-old", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "replacement", "New spec", "p1", "medium")
		require.NoError(t, err)

		// Abandon the old spec first to make it terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "terminal-old", "abandoned")
		require.NoError(t, err)

		// Supersede should fail because old spec is terminal.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "terminal-old", "replacement")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("SupersedeSpec_NewTerminal", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "old-sup-aband", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-sup-aband", "New spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.LifecycleAbandonSpec(ctx, "new-sup-aband", "no longer needed")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "old-sup-aband", "new-sup-aband")
		require.ErrorIs(t, err, storage.ErrNewSpecTerminal)
	})

	t.Run("SupersedeSpec_SameSlug", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "same-slug", "Test spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "same-slug", "same-slug")
		require.ErrorIs(t, err, storage.ErrSameSlugs)
	})

	t.Run("AbandonSpec_HappyPath", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "abandon-me", "Test spec", "p1", "medium")
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

		_, err := store.CreateSpec(ctx, "abandon-twice", "Test spec", "p1", "medium")
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

		_, err := store.CreateSpec(ctx, "toctou-abandon", "Test spec", "p1", "medium")
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

	t.Run("AmendedSpec_CanBeAbandoned", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-abandon", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-abandon", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Amend with empty reEntryStage -> "amended" stage.
		amended, err := store.LifecycleAmendSpec(ctx, "amend-abandon", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Amended is not fully terminal -- abandon should succeed.
		abandoned, err := store.LifecycleAbandonSpec(ctx, "amend-abandon", "no longer needed")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
	})

	t.Run("AmendedSpec_CanBeSuperseded", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "amend-supersede-old", "Old spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-supersede-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, "amend-supersede-old", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		_, err = store.CreateSpec(ctx, "amend-supersede-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		oldSpec, newSpec, err := store.LifecycleSupersedeSpec(ctx, "amend-supersede-old", "amend-supersede-new")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, oldSpec.Stage)
		require.Equal(t, "amend-supersede-new", oldSpec.SupersededBy)
		require.Equal(t, "amend-supersede-old", newSpec.Supersedes)
	})

	t.Run("AcknowledgeDrift_Basic", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		// Create upstream and downstream, link with DEPENDS_ON.
		_, err := store.CreateSpec(ctx, "ack-upstream", "Upstream", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "ack-drift", "Test spec", "p1", "medium")
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

		_, err := store.CreateSpec(ctx, "all-up1", "Upstream 1", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-up2", "Upstream 2", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-down", "Downstream", "p1", "medium")
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
		_, err := store.CreateSpec(ctx, "ack-ineligible", "Test spec", "p1", "medium")
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

	t.Run("AcknowledgeDrift_AmendedStage", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "ack-amended", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "ack-amended", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		amended, err := store.LifecycleAmendSpec(ctx, "ack-amended", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Amended specs should be eligible for drift acknowledgment.
		err = store.LifecycleAcknowledgeDrift(ctx, "ack-amended", "", "divergence accepted")
		require.NoError(t, err)
	})
}

func TestLifecycle_AmendRefreshesEdgeHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create upstream and downstream specs.
	_, err := store.CreateSpec(ctx, "upstream-hash", "Upstream spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-hash", "Downstream spec", "p1", "medium")
	require.NoError(t, err)

	// Link downstream -> upstream with a DEPENDS_ON edge.
	_, err = store.AddEdge(ctx, "downstream-hash", "upstream-hash", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Record the initial edge hash.
	initialDeps, err := store.GetDependenciesWithEdgeData(ctx, "downstream-hash")
	require.NoError(t, err)
	require.Len(t, initialDeps, 1)
	initialHashAtLink := initialDeps[0].ContentHashAtLink

	// Advance upstream to done so it can be amended.
	_, err = store.UpdateSpec(ctx, "upstream-hash", nil, strPtr("done"), nil, nil, nil)
	require.NoError(t, err)

	// Amend the upstream spec — this changes its content hash.
	_, err = store.LifecycleAmendSpec(ctx, "upstream-hash", "needs revision", "shape")
	require.NoError(t, err)

	// Confirm upstream's content hash changed.
	upstream, err := store.GetSpec(ctx, "upstream-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHashAtLink, upstream.ContentHash, "amend should change the upstream content hash")

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
