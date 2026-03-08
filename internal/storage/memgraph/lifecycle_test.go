// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"testing"
	"time"

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

	amended, err := store.AmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("amended"), amended.Stage)
	require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3
	require.NotEmpty(t, amended.History)
	require.Equal(t, "Mobile needs offline refresh", amended.History[len(amended.History)-1].Reason)
}

func TestAmendSpec_NotDone(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "not-done", "Test spec", "p1", "medium")
	require.NoError(t, err)
	// Spec is at "spark" — not done, so amend should fail.
	_, err = store.AmendSpec(ctx, "not-done", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
}

func TestAmendSpec_LifecycleNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.AmendSpec(ctx, "nonexistent", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_HappyPath(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium")
	require.NoError(t, err)

	old, newSpec, err := store.SupersedeSpec(ctx, "old-lifecycle", "new-lifecycle")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageSuperseded, old.Stage)
	require.Equal(t, "new-lifecycle", old.SupersededBy)
	require.Equal(t, "old-lifecycle", newSpec.Supersedes)
}

func TestSupersedeSpec_OldNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "exists-new", "New spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.SupersedeSpec(ctx, "nonexistent-old", "exists-new")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_NewNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "exists-old", "Old spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.SupersedeSpec(ctx, "exists-old", "nonexistent-new")
	require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
}

func TestAbandonSpec_HappyPath(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "abandon-me", "Test spec", "p1", "medium")
	require.NoError(t, err)

	abandoned, err := store.AbandonSpec(ctx, "abandon-me", "no longer needed")
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

	_, err = store.AbandonSpec(ctx, "abandon-twice", "first abandon")
	require.NoError(t, err)

	// Second abandon should fail — already terminal.
	_, err = store.AbandonSpec(ctx, "abandon-twice", "second abandon")
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

	// Wait briefly so updated_at differs.
	time.Sleep(1100 * time.Millisecond)

	// Update upstream to bump its updated_at.
	newIntent := "Upstream updated"
	_, err = store.UpdateSpec(ctx, "upstream-spec", &newIntent, nil, nil, nil)
	require.NoError(t, err)

	// Check drift on downstream — should detect dependency drift.
	reports, err := store.CheckDrift(ctx, "downstream-spec", "")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "downstream-spec", reports[0].SpecSlug)
	require.NotEmpty(t, reports[0].Items)
	require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
	require.Equal(t, "upstream-spec", reports[0].Items[0].UpstreamSlug)
}

func TestAcknowledgeDrift(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "ack-drift", "Test spec", "p1", "medium")
	require.NoError(t, err)

	report, err := store.AcknowledgeDrift(ctx, "ack-drift", "drift is intentional")
	require.NoError(t, err)
	require.Equal(t, "ack-drift", report.SpecSlug)
	require.True(t, report.Acknowledged)
	require.Equal(t, "drift is intentional", report.AcknowledgeNote)
}
