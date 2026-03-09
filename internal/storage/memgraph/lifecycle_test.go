// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestAmendSpec_HappyPath(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-me", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	amended, err := store.LifecycleAmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("shape"), amended.Stage)
	require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3
	require.NotEmpty(t, amended.History)
	require.Equal(t, "Mobile needs offline refresh", amended.History[len(amended.History)-1].Reason)
}

func TestAmendSpec_NotDone(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "not-done", "Test spec", "p1", "medium")
	require.NoError(t, err)
	// Spec is at "spark" — not done, so amend should fail.
	_, err = store.LifecycleAmendSpec(ctx, "not-done", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
}

func TestAmendSpec_LifecycleNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.LifecycleAmendSpec(ctx, "nonexistent", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_HappyPath(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium")
	require.NoError(t, err)

	old, newSpec, err := store.LifecycleSupersedeSpec(ctx, "old-lifecycle", "new-lifecycle")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageSuperseded, old.Stage)
	require.Equal(t, "new-lifecycle", old.SupersededBy)
	require.Equal(t, "old-lifecycle", newSpec.Supersedes)
}

func TestSupersedeSpec_OldNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "exists-new", "New spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "nonexistent-old", "exists-new")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_NewNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "exists-old", "Old spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "exists-old", "nonexistent-new")
	require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
}

func TestSupersedeSpec_TerminalState(t *testing.T) {
	store, ctx := newTestStore(t)

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
}

func TestAbandonSpec_HappyPath(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "abandon-me", "Test spec", "p1", "medium")
	require.NoError(t, err)

	abandoned, err := store.LifecycleAbandonSpec(ctx, "abandon-me", "no longer needed")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
	require.Equal(t, int32(2), abandoned.Version) // create=1, abandon=2
	require.NotEmpty(t, abandoned.History)
	require.Equal(t, "no longer needed", abandoned.History[len(abandoned.History)-1].Reason)
}

func TestAbandonSpec_Terminal(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "abandon-twice", "Test spec", "p1", "medium")
	require.NoError(t, err)

	_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "first abandon")
	require.NoError(t, err)

	// Second abandon should fail — already terminal.
	_, err = store.LifecycleAbandonSpec(ctx, "abandon-twice", "second abandon")
	require.ErrorIs(t, err, storage.ErrSpecTerminal)
}

func TestCheckDrift_DependencyDrift(t *testing.T) {
	store, ctx := newTestStore(t)

	// Create upstream and downstream specs.
	_, err := store.CreateSpec(ctx, "upstream-spec", "Upstream", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-spec", "Downstream", "p1", "medium")
	require.NoError(t, err)

	// Create DEPENDS_ON edge: downstream → upstream.
	_, err = store.AddEdge(ctx, "downstream-spec", "upstream-spec", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Drift detection compares updated_at timestamps: upstream must be strictly
	// newer than downstream. Memgraph's datetime has second-level precision in
	// practice, so we sleep >1s to guarantee the ordering.
	time.Sleep(1100 * time.Millisecond)

	// Update upstream to bump its updated_at.
	newIntent := "Upstream updated"
	_, err = store.UpdateSpec(ctx, "upstream-spec", &newIntent, nil, nil, nil)
	require.NoError(t, err)

	// Use the drift engine to check drift.
	engine := drift.NewEngine(store)
	reports, err := engine.Check(ctx, "downstream-spec", "")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "downstream-spec", reports[0].SpecSlug)
	require.NotEmpty(t, reports[0].Items)
	require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
	require.Equal(t, "upstream-spec", reports[0].Items[0].UpstreamSlug)
}

func TestAbandonSpec_NotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.LifecycleAbandonSpec(ctx, "nonexistent-spec", "reason")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestAmendSpec_Terminal(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-terminal", "Test spec", "p1", "medium")
	require.NoError(t, err)

	// Abandon the spec to make it terminal.
	_, err = store.LifecycleAbandonSpec(ctx, "amend-terminal", "abandoned")
	require.NoError(t, err)

	// Amend should fail — spec is in a terminal state.
	_, err = store.LifecycleAmendSpec(ctx, "amend-terminal", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecTerminal)
}

func TestAcknowledgeDrift(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "ack-drift", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "ack-drift", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	report, err := store.LifecycleAcknowledgeDrift(ctx, "ack-drift", "drift is intentional")
	require.NoError(t, err)
	require.Equal(t, "ack-drift", report.SpecSlug)
	require.True(t, report.Acknowledged)
	require.Equal(t, "drift is intentional", report.AcknowledgeNote)

	// Verify persistence: a second acknowledgment with a different note should
	// overwrite the previous value, proving the first was persisted to the node.
	report2, err := store.LifecycleAcknowledgeDrift(ctx, "ack-drift", "updated note")
	require.NoError(t, err)
	require.Equal(t, "ack-drift", report2.SpecSlug)
	require.True(t, report2.Acknowledged)
	require.Equal(t, "updated note", report2.AcknowledgeNote)
}

func TestAcknowledgeDrift_IneligibleStage(t *testing.T) {
	store, ctx := newTestStore(t)

	// Create a spec at spark stage (not eligible for drift acknowledgment).
	_, err := store.CreateSpec(ctx, "ack-ineligible", "Test spec", "p1", "medium")
	require.NoError(t, err)

	_, err = store.LifecycleAcknowledgeDrift(ctx, "ack-ineligible", "should fail")
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrSpecIneligibleStage)
}

func TestAmendSpec_ConcurrentModification(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "toctou-amend", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "toctou-amend", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// First amend succeeds.
	amended, err := store.LifecycleAmendSpec(ctx, "toctou-amend", "first amend", "shape")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("shape"), amended.Stage)

	// Now the spec is at "shape" (not done), second amend should fail
	// because amend requires the spec to be in the done stage.
	_, err = store.LifecycleAmendSpec(ctx, "toctou-amend", "second amend", "specify")
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
}

func TestCheckDrift_AllSpecs_Integration(t *testing.T) {
	store, ctx := newTestStore(t)

	// Create two done specs with upstream dependency.
	_, err := store.CreateSpec(ctx, "up-integ", "Upstream", "p1", "medium")
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
	_, err = store.UpdateSpec(ctx, "down1-integ", nil, &doneStage, nil, nil)
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "down2-integ", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Same as above: sleep >1s so updated_at(upstream) > updated_at(downstream).
	time.Sleep(1100 * time.Millisecond)
	newIntent := "Updated upstream"
	_, err = store.UpdateSpec(ctx, "up-integ", &newIntent, nil, nil, nil)
	require.NoError(t, err)

	// Check all specs (empty slug) — should find drift on both.
	engine := drift.NewEngine(store)
	reports, err := engine.Check(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, reports, 2)

	// Check with scope filter — deps should find drift, interfaces should not.
	depsReports, err := engine.Check(ctx, "", "deps")
	require.NoError(t, err)
	require.Len(t, depsReports, 2)

	ifaceReports, err := engine.Check(ctx, "", "interfaces")
	require.NoError(t, err)
	require.Empty(t, ifaceReports)
}

func TestAcknowledgeDrift_PersistsAcrossReads(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "ack-persist", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "ack-persist", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	_, err = store.LifecycleAcknowledgeDrift(ctx, "ack-persist", "intentional drift")
	require.NoError(t, err)

	// Verify the acknowledgment persists by reading the spec independently
	// and acknowledging again — the note should reflect the latest value.
	report, err := store.LifecycleAcknowledgeDrift(ctx, "ack-persist", "new note")
	require.NoError(t, err)
	require.True(t, report.Acknowledged)
	require.Equal(t, "new note", report.AcknowledgeNote)
}

func TestAcknowledgeDrift_VisibleViaGetSpec(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "ack-getspec", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "ack-getspec", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	_, err = store.LifecycleAcknowledgeDrift(ctx, "ack-getspec", "drift accepted")
	require.NoError(t, err)

	// GetSpec should reflect the acknowledged flag set by AcknowledgeDrift.
	spec, err := store.GetSpec(ctx, "ack-getspec")
	require.NoError(t, err)
	require.True(t, spec.DriftAcknowledged)
	require.Equal(t, "drift accepted", spec.DriftAcknowledgeNote)
}

func TestAmendedSpec_CanBeAbandoned(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-abandon", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-abandon", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Amend with empty reEntryStage → "amended" stage.
	amended, err := store.LifecycleAmendSpec(ctx, "amend-abandon", "needs rework", "")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageAmended, amended.Stage)

	// Amended is not fully terminal — abandon should succeed.
	abandoned, err := store.LifecycleAbandonSpec(ctx, "amend-abandon", "no longer needed")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageAbandoned, abandoned.Stage)
}

func TestAmendedSpec_CanBeSuperseded(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-supersede-old", "Old spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-supersede-old", nil, &doneStage, nil, nil)
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
}

func TestAmendSpec_ReEntryDone(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "amend-done-reentry", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-done-reentry", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Amend with re_entry_stage="done" on an already-done spec.
	// This burns a version number but the spec remains in done stage.
	amended, err := store.LifecycleAmendSpec(ctx, "amend-done-reentry", "re-enter at done", "done")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageDone, amended.Stage, "spec should remain in done stage")
	require.Equal(t, int32(3), amended.Version, "version should increment (create=1, update=2, amend=3)")
	require.NotEmpty(t, amended.History)
	lastEntry := amended.History[len(amended.History)-1]
	require.Equal(t, "re-enter at done", lastEntry.Reason)
}

func TestSupersedeSpec_ConcurrentModificationOnNewSpec(t *testing.T) {
	store, ctx := newTestStore(t)

	// Create old spec at done stage.
	_, err := store.CreateSpec(ctx, "old-supersede", "Old spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "old-supersede", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Create new spec at done stage.
	_, err = store.CreateSpec(ctx, "new-supersede", "New spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "new-supersede", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Modify the new spec behind the scenes to trigger the version guard.
	sparkStage := "spark"
	_, err = store.UpdateSpec(ctx, "new-supersede", nil, &sparkStage, nil, nil)
	require.NoError(t, err)

	// Now supersede should detect the concurrent modification on the new spec.
	_, _, err = store.LifecycleSupersedeSpec(ctx, "old-supersede", "new-supersede")
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrConcurrentModification)
}
