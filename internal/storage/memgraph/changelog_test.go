// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSpec_CreatesChangeLogEntry(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-changelog", "test intent", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-changelog", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, int32(1), entry.Version)
	assert.Equal(t, string(storage.SpecStageSpark), entry.Stage)
	assert.True(t, entry.Checkpoint, "initial creation should be a checkpoint")
	assert.NotEmpty(t, entry.ContentHash)
	assert.NotEmpty(t, entry.ID)
}

func TestUpdateSpec_CreatesChangeLogEntry(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-update-cl", "initial intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "updated intent"
	_, err = store.UpdateSpec(ctx, "test-update-cl", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-update-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	update := entries[1]
	assert.Equal(t, int32(2), update.Version)
	assert.False(t, update.Checkpoint)
	assert.Len(t, update.Changes, 1)
	assert.Equal(t, "intent", update.Changes[0].Field)
	assert.Equal(t, "initial intent", update.Changes[0].OldValue)
	assert.Equal(t, "updated intent", update.Changes[0].NewValue)
}

func TestTransitionStage_CreatesCheckpointChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-transition-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "test-transition-cl", "spark", "shape")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-transition-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	transition := entries[1]
	assert.True(t, transition.Checkpoint, "stage transition should be checkpoint")
	assert.Equal(t, string(storage.SpecStageShape), transition.Stage)
	assert.Contains(t, transition.Summary, "Stage transition")
	require.Len(t, transition.Changes, 1)
	assert.Equal(t, "stage", transition.Changes[0].Field)
	assert.Equal(t, "spark", transition.Changes[0].OldValue)
	assert.Equal(t, "shape", transition.Changes[0].NewValue)
}

func TestStoreSparkOutput_CreatesChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-spark-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "test-spark-cl", &storage.SparkOutput{Seed: "fast login", Signal: "user request"})
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-spark-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	sparkEntry := entries[1]
	assert.False(t, sparkEntry.Checkpoint)
	assert.Contains(t, sparkEntry.Summary, "spark_output")
	// The field delta should include spark_output.
	found := false
	for _, c := range sparkEntry.Changes {
		if c.Field == "spark_output" {
			found = true
			break
		}
	}
	assert.True(t, found, "should have spark_output field change")
}

func TestStoreShapeOutput_CreatesChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-shape-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.StoreShapeOutput(ctx, "test-shape-cl", &storage.ShapeOutput{
		ScopeIn:        []string{"auth module"},
		ChosenApproach: "OAuth2",
	})
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-shape-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	shapeEntry := entries[1]
	assert.False(t, shapeEntry.Checkpoint)
	assert.Contains(t, shapeEntry.Summary, "shape_output")
	found := false
	for _, c := range shapeEntry.Changes {
		if c.Field == "shape_output" {
			found = true
			break
		}
	}
	assert.True(t, found, "should have shape_output field change")
}

func TestStoreSpecifyOutput_CreatesChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-specify-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.StoreSpecifyOutput(ctx, "test-specify-cl", &storage.SpecifyOutput{
		VerifyCriteria: []storage.VerifyCriterion{{Category: "performance", Description: "must be fast"}},
	})
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-specify-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	specifyEntry := entries[1]
	assert.False(t, specifyEntry.Checkpoint)
	assert.Contains(t, specifyEntry.Summary, "specify_output")
}

func TestStoreDecomposeOutput_CreatesChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-decompose-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	_, err = store.StoreDecomposeOutput(ctx, "test-decompose-cl", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "slice-a", Intent: "first slice"},
		},
	})
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-decompose-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	decomposeEntry := entries[1]
	assert.False(t, decomposeEntry.Checkpoint)
	assert.Contains(t, decomposeEntry.Summary, "decompose_output")
}

func TestLifecycleAmendSpec_CreatesCheckpointChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-amend-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	// Set stage to "done" directly via UpdateSpec (matches existing lifecycle_test.go pattern).
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "test-amend-cl", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.LifecycleAmendSpec(ctx, "test-amend-cl", "needs rework", "shape")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-amend-cl", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	last := entries[len(entries)-1]
	assert.True(t, last.Checkpoint)
	assert.Contains(t, last.Summary, "Amended")
	assert.Equal(t, "needs rework", last.Reason)
}

func TestLifecycleAbandonSpec_CreatesCheckpointChangeLog(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-abandon-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	_, err = store.LifecycleAbandonSpec(ctx, "test-abandon-cl", "no longer needed")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-abandon-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	abandonEntry := entries[1]
	assert.True(t, abandonEntry.Checkpoint)
	assert.Equal(t, string(storage.SpecStageAbandoned), abandonEntry.Stage)
	assert.Equal(t, "Spec abandoned", abandonEntry.Summary)
	assert.Equal(t, "no longer needed", abandonEntry.Reason)
}

func TestLifecycleSupersedeSpec_CreatesCheckpointChangeLogs(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-supersede-old-cl", "old intent", "p2", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "test-supersede-new-cl", "new intent", "p2", "medium")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "test-supersede-old-cl", "test-supersede-new-cl")
	require.NoError(t, err)

	// Old spec should have 2 entries: creation + superseded checkpoint.
	oldEntries, err := store.ListChanges(ctx, "test-supersede-old-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, oldEntries, 2)
	assert.True(t, oldEntries[1].Checkpoint)
	assert.Equal(t, string(storage.SpecStageSuperseded), oldEntries[1].Stage)
	assert.Equal(t, "Spec superseded", oldEntries[1].Summary)

	// New spec should have 2 entries: creation + supersedes predecessor checkpoint.
	newEntries, err := store.ListChanges(ctx, "test-supersede-new-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, newEntries, 2)
	assert.True(t, newEntries[1].Checkpoint)
	assert.Equal(t, "Supersedes predecessor", newEntries[1].Summary)
}

func TestListChanges_CheckpointsOnly(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-filter-cp", "intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "updated"
	_, err = store.UpdateSpec(ctx, "test-filter-cp", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	all, err := store.ListChanges(ctx, "test-filter-cp", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	cps, err := store.ListChanges(ctx, "test-filter-cp", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	assert.Len(t, cps, 1)
}

func TestListChanges_SinceVersion(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-filter-ver", "intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "v2"
	_, err = store.UpdateSpec(ctx, "test-filter-ver", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-filter-ver", storage.ChangeLogFilter{SinceVersion: 1})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, int32(2), entries[0].Version)
}

func TestListChanges_Limit(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-filter-lim", "intent", "p2", "medium")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		v := fmt.Sprintf("intent-%d", i)
		_, err = store.UpdateSpec(ctx, "test-filter-lim", &v, nil, nil, nil, nil)
		require.NoError(t, err)
	}

	entries, err := store.ListChanges(ctx, "test-filter-lim", storage.ChangeLogFilter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, int32(1), entries[0].Version)
}

func TestListChanges_SpecNotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.ListChanges(ctx, "nonexistent", storage.ChangeLogFilter{})
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestUpdateSpec_NoChangeLogOnNoOp(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-noop-cl", "same intent", "p2", "medium")
	require.NoError(t, err)

	notes := "just a note"
	_, err = store.UpdateSpec(ctx, "test-noop-cl", nil, nil, nil, nil, &notes)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-noop-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1) // only creation, no update changelog
}

// changeCapture is a minimal ChangeSubscriber that records received events.
type changeCapture struct {
	mu     sync.Mutex
	events []*storage.ChangeEvent
}

func (c *changeCapture) OnSpecChanged(_ context.Context, event *storage.ChangeEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *event
	c.events = append(c.events, &cp)
}

func (c *changeCapture) received() []*storage.ChangeEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*storage.ChangeEvent, len(c.events))
	copy(out, c.events)
	return out
}

func TestSubscribe_FiresOnCreateSpec(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	cap := &changeCapture{}
	store.Subscribe(cap)

	_, err = store.CreateSpec(ctx, "sub-fire-test", "Test intent", "p2", "medium")
	require.NoError(t, err)

	events := cap.received()
	require.NotEmpty(t, events, "subscriber should have received at least one event after CreateSpec")
	assert.Equal(t, "sub-fire-test", events[0].Slug)
}

func TestSubscribe_GraphBackendAvailableInContext(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	var gotBackend storage.GraphBackend
	sub := &contextCheckSubscriber{
		onChanged: func(notifyCtx context.Context, _ *storage.ChangeEvent) {
			gotBackend, _ = storage.GraphBackendFromContext(notifyCtx)
		},
	}
	store.Subscribe(sub)

	_, err = store.CreateSpec(ctx, "sub-ctx-test", "intent", "p2", "medium")
	require.NoError(t, err)

	assert.NotNil(t, gotBackend, "GraphBackend should be present in subscriber context")
}

func TestSubscribe_PanicInSubscriberDoesNotPropagate(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	panicSub := &panicSubscriber{}
	store.Subscribe(panicSub)

	// CreateSpec must succeed even though the subscriber panics.
	_, err = store.CreateSpec(ctx, "sub-panic-test", "intent", "p2", "medium")
	assert.NoError(t, err, "transaction should not fail when subscriber panics")
}

// contextCheckSubscriber calls a provided function with the notification context.
type contextCheckSubscriber struct {
	onChanged func(ctx context.Context, event *storage.ChangeEvent)
}

func (s *contextCheckSubscriber) OnSpecChanged(ctx context.Context, event *storage.ChangeEvent) {
	if s.onChanged != nil {
		s.onChanged(ctx, event)
	}
}

// panicSubscriber deliberately panics to verify dispatchChangeEvents recovery.
type panicSubscriber struct{}

func (panicSubscriber) OnSpecChanged(_ context.Context, _ *storage.ChangeEvent) {
	panic("deliberate test panic")
}
