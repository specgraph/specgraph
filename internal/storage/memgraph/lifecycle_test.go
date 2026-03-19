// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

// newTestClock returns a controllable clock and an advance function.
// The clock starts at the current time; advance moves it forward.
func newTestClock() (now func() time.Time, advance func(time.Duration)) {
	mu := &sync.Mutex{}
	t := time.Now()
	now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return t
	}
	advance = func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		t = t.Add(d)
	}
	return
}

func TestLifecycle(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	t.Cleanup(cleanup)

	t.Run("AmendSpec_HappyPath", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-me", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStage("shape"), amended.Stage)
		require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3
		// History field removed — changelog is now tracked via ChangeLog graph nodes.

		// Verify re-entry stage was persisted to Memgraph, not just returned in-memory.
		fetched, err := store.GetSpec(ctx, "amend-me")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStage("shape"), fetched.Stage)
		require.Equal(t, int32(3), fetched.Version)
	})

	t.Run("AmendSpec_NotDone", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "not-done", "Test spec", "p1", "medium")
		require.NoError(t, err)
		// Spec is at "spark" — not done, so amend should fail.
		_, err = store.LifecycleAmendSpec(ctx, "not-done", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotDone)
	})

	t.Run("AmendSpec_LifecycleNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.LifecycleAmendSpec(ctx, "nonexistent", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("SupersedeSpec_HappyPath", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium")
		require.NoError(t, err)

		old, newSpec, err := store.LifecycleSupersedeSpec(ctx, "old-lifecycle", "new-lifecycle")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, old.Stage)
		require.Equal(t, "new-lifecycle", old.SupersededBy)
		require.Equal(t, "old-lifecycle", newSpec.Supersedes)
	})

	t.Run("SupersedeSpec_SupersedesEdgePersists", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "edge-old", "Old spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "edge-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "edge-old", "edge-new")
		require.NoError(t, err)

		// Verify SUPERSEDES edge was written to Memgraph.
		edges, err := store.ListEdges(ctx, "edge-new", storage.EdgeTypeSupersedes)
		require.NoError(t, err)
		require.Len(t, edges, 1, "expected one SUPERSEDES edge from new to old")
		require.Equal(t, "edge-new", edges[0].FromID)
		require.Equal(t, "edge-old", edges[0].ToID)
	})

	t.Run("SupersedeSpec_OldNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "exists-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "nonexistent-old", "exists-new")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("SupersedeSpec_NewNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "exists-old", "Old spec", "p1", "medium")
		require.NoError(t, err)

		_, _, err = store.LifecycleSupersedeSpec(ctx, "exists-old", "nonexistent-new")
		require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
	})

	t.Run("SupersedeSpec_TerminalState", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "terminal-old", "Old spec", "p1", "medium")
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

	t.Run("SupersedeSpec_SameSlug", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "same-slug", "Test spec", "p1", "medium")
		require.NoError(t, err)

		// Superseding a spec with itself must fail at the storage layer.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "same-slug", "same-slug")
		require.ErrorIs(t, err, storage.ErrSameSlugs)
	})

	t.Run("AbandonSpec_HappyPath", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "abandon-me", "Test spec", "p1", "medium")
		require.NoError(t, err)

		abandoned, err := store.LifecycleAbandonSpec(ctx, "abandon-me", "no longer needed")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
		require.Equal(t, int32(2), abandoned.Version) // create=1, abandon=2
		// History field removed — changelog is now tracked via ChangeLog graph nodes.
	})

	t.Run("AbandonSpec_ConcurrentModification", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "toctou-abandon", "Test spec", "p1", "medium")
		require.NoError(t, err)

		// Two goroutines race to abandon the same spec. Exactly one should
		// succeed; the other hits the WHERE guard (version/stage mismatch)
		// or the Go-level terminal pre-check, validating atomicity.
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

	t.Run("AbandonSpec_Terminal", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "abandon-twice", "Test spec", "p1", "medium")
		require.NoError(t, err)

		_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "first abandon")
		require.NoError(t, err)

		// Second abandon should fail — already terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "second abandon")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("CheckDrift_DependencyDrift", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		clock, advance := newTestClock()
		store, err := newStore(ctx, boltURI, memgraph.WithClock(clock))
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create upstream and downstream specs.
		_, err = store.CreateSpec(ctx, "upstream-spec", "Upstream", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "downstream-spec", "Downstream", "p1", "medium")
		require.NoError(t, err)

		// Create DEPENDS_ON edge: downstream → upstream.
		_, err = store.AddEdge(ctx, "downstream-spec", "upstream-spec", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		// Advance downstream-spec to "done" so it is eligible for drift checking.
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "downstream-spec", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Advance the clock so the upstream update gets a strictly newer timestamp
		// than the downstream spec's created_at / updated_at.
		advance(2 * time.Second)

		// Update upstream to bump its updated_at.
		newIntent := "Upstream updated"
		_, err = store.UpdateSpec(ctx, "upstream-spec", &newIntent, nil, nil, nil, nil)
		require.NoError(t, err)

		// Use the drift engine to check drift.
		engine := drift.NewEngine(store, nil)
		reports, err := engine.Check(ctx, "downstream-spec", "")
		require.NoError(t, err)
		require.Len(t, reports, 1)
		require.Equal(t, "downstream-spec", reports[0].SpecSlug)
		require.NotEmpty(t, reports[0].Items)
		require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
		require.Equal(t, "upstream-spec", reports[0].Items[0].UpstreamSlug)
	})

	t.Run("AbandonSpec_NotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.LifecycleAbandonSpec(ctx, "nonexistent-spec", "reason")
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("AmendSpec_Terminal", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "amend-terminal", "Test spec", "p1", "medium")
		require.NoError(t, err)

		// Abandon the spec to make it terminal.
		_, err = store.LifecycleAbandonSpec(ctx, "amend-terminal", "abandoned")
		require.NoError(t, err)

		// Amend should fail — spec is in a terminal state.
		_, err = store.LifecycleAmendSpec(ctx, "amend-terminal", "reason", "shape")
		require.ErrorIs(t, err, storage.ErrSpecTerminal)
	})

	t.Run("AcknowledgeDrift", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create upstream and downstream, link with DEPENDS_ON.
		_, err = store.CreateSpec(ctx, "ack-upstream", "Upstream", "p1", "medium")
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

	t.Run("AcknowledgeDrift_IneligibleStage", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create a spec at spark stage (not eligible for drift acknowledgment).
		_, err = store.CreateSpec(ctx, "ack-ineligible", "Test spec", "p1", "medium")
		require.NoError(t, err)

		err = store.LifecycleAcknowledgeDrift(ctx, "ack-ineligible", "", "should fail")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecIneligibleStage)
	})

	t.Run("AmendSpec_ConcurrentModification", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "toctou-amend", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "toctou-amend", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Two goroutines race to amend the same spec. Exactly one should
		// succeed; the other hits the WHERE guard (version/stage mismatch),
		// validating atomicity. Thanks to the .11 fix, the loser correctly
		// receives ErrConcurrentModification instead of ErrSpecNotDone.
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

	t.Run("CheckDrift_AllSpecs_Integration", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		clock, advance := newTestClock()
		store, err := newStore(ctx, boltURI, memgraph.WithClock(clock))
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create two done specs with upstream dependency.
		_, err = store.CreateSpec(ctx, "up-integ", "Upstream", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "down1-integ", "Down1", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "down2-integ", "Down2", "p1", "medium")
		require.NoError(t, err)

		// Add dependency edges.
		_, err = store.AddEdge(ctx, "down1-integ", "up-integ", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "down2-integ", "up-integ", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		// Move downstream specs to "done".
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "down1-integ", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "down2-integ", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Advance the clock so upstream update gets a strictly newer timestamp.
		advance(2 * time.Second)
		newIntent := "Updated upstream"
		_, err = store.UpdateSpec(ctx, "up-integ", &newIntent, nil, nil, nil, nil)
		require.NoError(t, err)

		// Check all specs (empty slug) — should find drift on both.
		engine := drift.NewEngine(store, nil)
		reports, err := engine.Check(ctx, "", "")
		require.NoError(t, err)
		require.Len(t, reports, 2)

		// Check with scope filter — deps should find drift, interfaces should not.
		depsReports, err := engine.Check(ctx, "", "deps")
		require.NoError(t, err)
		require.Len(t, depsReports, 2)

		ifaceReports, err := engine.Check(ctx, "", "interfaces")
		require.NoError(t, err)
		require.Len(t, ifaceReports, 2)
		for _, r := range ifaceReports {
			require.Empty(t, r.Items)
			require.Contains(t, r.ErrorMessage, "not yet implemented")
		}
	})

	t.Run("AcknowledgeDrift_PerUpstream_UpdatesEdgeHash", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create upstream and downstream specs.
		_, err = store.CreateSpec(ctx, "ack-upstream", "Upstream spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "ack-downstream", "Downstream spec", "p1", "medium")
		require.NoError(t, err)

		// Add DEPENDS_ON edge (baselines upstream's content hash).
		_, err = store.AddEdge(ctx, "ack-downstream", "ack-upstream", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		// Move downstream to done stage so ack is eligible.
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "ack-downstream", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Modify upstream (changes its content hash).
		newIntent := "Updated upstream intent"
		_, err = store.UpdateSpec(ctx, "ack-upstream", &newIntent, nil, nil, nil, nil)
		require.NoError(t, err)
		upstream, err := store.GetSpec(ctx, "ack-upstream")
		require.NoError(t, err)

		// Acknowledge drift for this specific upstream.
		err = store.LifecycleAcknowledgeDrift(ctx, "ack-downstream", "ack-upstream", "intentional drift")
		require.NoError(t, err)

		// Verify edge hash updated to upstream's current hash.
		deps, err := store.GetDependenciesWithEdgeData(ctx, "ack-downstream")
		require.NoError(t, err)
		require.Len(t, deps, 1)
		require.Equal(t, upstream.ContentHash, deps[0].ContentHashAtLink)
	})

	t.Run("AcknowledgeDrift_AllUpstreams_UpdatesAllEdges", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create two upstreams and one downstream.
		_, err = store.CreateSpec(ctx, "all-up1", "Upstream 1", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-up2", "Upstream 2", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "all-down", "Downstream", "p1", "medium")
		require.NoError(t, err)

		// Add DEPENDS_ON edges.
		_, err = store.AddEdge(ctx, "all-down", "all-up1", storage.EdgeTypeDependsOn)
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "all-down", "all-up2", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		// Move downstream to done.
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

		// Blanket ack (empty upstreamSlug = all).
		err = store.LifecycleAcknowledgeDrift(ctx, "all-down", "", "blanket ack")
		require.NoError(t, err)

		// Verify both edges updated.
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

	t.Run("AmendedSpec_CanBeAbandoned", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "amend-abandon", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-abandon", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Amend with empty reEntryStage → "amended" stage.
		amended, err := store.LifecycleAmendSpec(ctx, "amend-abandon", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Amended is not fully terminal — abandon should succeed.
		abandoned, err := store.LifecycleAbandonSpec(ctx, "amend-abandon", "no longer needed")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
	})

	t.Run("AmendedSpec_CanBeSuperseded", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "amend-supersede-old", "Old spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amend-supersede-old", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Amend with empty reEntryStage → "amended" stage.
		amended, err := store.LifecycleAmendSpec(ctx, "amend-supersede-old", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Create a new spec to supersede the old one.
		_, err = store.CreateSpec(ctx, "amend-supersede-new", "New spec", "p1", "medium")
		require.NoError(t, err)

		// Amended is not fully terminal — supersede should succeed.
		oldSpec, newSpec, err := store.LifecycleSupersedeSpec(ctx, "amend-supersede-old", "amend-supersede-new")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageSuperseded, oldSpec.Stage)
		require.Equal(t, "amend-supersede-new", oldSpec.SupersededBy)
		require.Equal(t, "amend-supersede-old", newSpec.Supersedes)
	})

	t.Run("AmendSpec_ReEntryExcludedStages", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		for _, stage := range []string{"done", "amended", "superseded", "abandoned"} {
			slug := "amend-reentry-" + stage
			_, err = store.CreateSpec(ctx, slug, "Test spec", "p1", "medium")
			require.NoError(t, err)
			doneStage := "done"
			_, err = store.UpdateSpec(ctx, slug, nil, &doneStage, nil, nil, nil)
			require.NoError(t, err)

			_, err = store.LifecycleAmendSpec(ctx, slug, "reason", stage)
			require.ErrorIs(t, err, storage.ErrInvalidReEntryStage, "stage %q should be rejected", stage)
		}
	})

	t.Run("SupersedeSpec_ConcurrentModificationOnNewSpec", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create old spec at done stage.
		_, err = store.CreateSpec(ctx, "old-supersede", "Old spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "old-supersede", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Create new spec (not terminal, so supersede WHERE guard allows it).
		_, err = store.CreateSpec(ctx, "new-supersede", "New spec", "p1", "medium")
		require.NoError(t, err)

		// Race a supersede against a concurrent update on the new spec.
		// One should succeed, the other should fail or detect the modification.
		const n = 2
		type result struct {
			err error
		}
		results := make(chan result, n)

		go func() {
			_, _, sErr := store.LifecycleSupersedeSpec(ctx, "old-supersede", "new-supersede")
			results <- result{err: sErr}
		}()
		go func() {
			// Concurrently modify the new spec's version.
			newIntent := "Modified concurrently"
			_, uErr := store.UpdateSpec(ctx, "new-supersede", &newIntent, nil, nil, nil, nil)
			results <- result{err: uErr}
		}()

		// At least one should succeed. If supersede loses the race, it should
		// return ErrConcurrentModification (not an opaque error).
		var errs []error
		for i := 0; i < n; i++ {
			r := <-results
			if r.err != nil {
				errs = append(errs, r.err)
			}
		}
		for _, e := range errs {
			require.True(t,
				errors.Is(e, storage.ErrConcurrentModification) ||
					errors.Is(e, storage.ErrSpecNotFound) ||
					errors.Is(e, storage.ErrSpecTerminal) ||
					errors.Is(e, storage.ErrInternalGuardFailure),
				"unexpected error: %v", e)
		}
	})

	t.Run("SupersedeSpec_NewSpecConcurrentlyAbandoned", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create old spec at done stage.
		_, err = store.CreateSpec(ctx, "old-sup-aband", "Old spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "old-sup-aband", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Create new spec at done stage, then abandon it to put it in a terminal state.
		_, err = store.CreateSpec(ctx, "new-sup-aband", "New spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "new-sup-aband", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.LifecycleAbandonSpec(ctx, "new-sup-aband", "no longer needed")
		require.NoError(t, err)

		// Supersede should detect the new spec is in a terminal state.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "old-sup-aband", "new-sup-aband")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNewSpecTerminal)
	})

	t.Run("SupersedeSpec_NewSpecNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create old spec at done stage.
		_, err = store.CreateSpec(ctx, "old-sup-nf", "Old spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "old-sup-nf", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Supersede with a non-existent new spec.
		_, _, err = store.LifecycleSupersedeSpec(ctx, "old-sup-nf", "no-such-spec")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
	})

	t.Run("CheckDrift_AmendedSpecDrift", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		clock, advance := newTestClock()
		store, err := newStore(ctx, boltURI, memgraph.WithClock(clock))
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create upstream and downstream specs, move to done.
		_, err = store.CreateSpec(ctx, "amended-drift-up", "Upstream", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "amended-drift-down", "Downstream", "p1", "medium")
		require.NoError(t, err)

		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "amended-drift-up", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "amended-drift-down", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Amend the downstream spec (done → amended).
		amended, err := store.LifecycleAmendSpec(ctx, "amended-drift-down", "rework needed", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Create DEPENDS_ON edge: downstream → upstream.
		_, err = store.AddEdge(ctx, "amended-drift-down", "amended-drift-up", storage.EdgeTypeDependsOn)
		require.NoError(t, err)

		// Advance the clock so upstream update gets a strictly newer timestamp.
		advance(2 * time.Second)

		// Update upstream to trigger drift.
		newIntent := "Updated upstream"
		_, err = store.UpdateSpec(ctx, "amended-drift-up", &newIntent, nil, nil, nil, nil)
		require.NoError(t, err)

		// Drift engine should detect drift on the amended downstream spec.
		engine := drift.NewEngine(store, nil)
		reports, err := engine.Check(ctx, "amended-drift-down", "")
		require.NoError(t, err)
		require.Len(t, reports, 1)
		require.Equal(t, "amended-drift-down", reports[0].SpecSlug)
		require.NotEmpty(t, reports[0].Items)
		require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
	})

	t.Run("BatchGetSpecs", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create three specs.
		_, err = store.CreateSpec(ctx, "batch-a", "Spec A", "p1", "medium")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "batch-b", "Spec B", "p2", "low")
		require.NoError(t, err)
		_, err = store.CreateSpec(ctx, "batch-c", "Spec C", "p0", "high")
		require.NoError(t, err)

		// All slugs present.
		result, err := store.BatchGetSpecs(ctx, []string{"batch-a", "batch-b", "batch-c"})
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, "batch-a", result["batch-a"].Slug)
		require.Equal(t, "batch-b", result["batch-b"].Slug)
		require.Equal(t, "batch-c", result["batch-c"].Slug)

		// Partial — some slugs missing.
		result, err = store.BatchGetSpecs(ctx, []string{"batch-a", "nonexistent", "batch-c"})
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Contains(t, result, "batch-a")
		require.Contains(t, result, "batch-c")
		require.NotContains(t, result, "nonexistent")

		// Empty slug list returns empty map.
		result, err = store.BatchGetSpecs(ctx, []string{})
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("AcknowledgeDrift_AmendedStage", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create spec, move to done, then amend (empty re-entry → "amended" stage).
		_, err = store.CreateSpec(ctx, "ack-amended", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "ack-amended", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)
		amended, err := store.LifecycleAmendSpec(ctx, "ack-amended", "needs rework", "")
		require.NoError(t, err)
		require.Equal(t, storage.SpecStageAmended, amended.Stage)

		// Amended specs should be eligible for drift acknowledgment (blanket ack).
		err = store.LifecycleAcknowledgeDrift(ctx, "ack-amended", "", "divergence accepted")
		require.NoError(t, err)
	})

	t.Run("AcknowledgeDrift_NotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		err = store.LifecycleAcknowledgeDrift(ctx, "nonexistent-spec", "", "should fail")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("AcknowledgeDrift_ConcurrentAckAndCheck", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "ack-race", "Test spec", "p1", "medium")
		require.NoError(t, err)
		doneStage := "done"
		_, err = store.UpdateSpec(ctx, "ack-race", nil, &doneStage, nil, nil, nil)
		require.NoError(t, err)

		// Race an acknowledge against a concurrent acknowledge.
		const n = 2
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				aErr := store.LifecycleAcknowledgeDrift(ctx, "ack-race", "", fmt.Sprintf("note-%d", i))
				errs <- aErr
			}()
		}

		// Both should succeed (last-writer-wins for SET operations), but
		// Memgraph may raise a transaction conflict for one of them. That's
		// acceptable — the caller would retry.
		for i := 0; i < n; i++ {
			if e := <-errs; e != nil {
				require.ErrorIs(t, e, storage.ErrConcurrentModification,
					"concurrent acknowledge should either succeed or return ErrConcurrentModification, got: %v", e)
			}
		}
	})
}
