// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestDispatchChangeEvents_NoShared(_ *testing.T) {
	s := &Store{} // shared is nil
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "x"})
	// Should not panic.
	s.dispatchChangeEvents(ctx)
}

func TestDispatchChangeEvents_NoEvents(t *testing.T) {
	var called atomic.Bool
	s := &Store{shared: &sharedState{
		subscribers: []storage.ChangeSubscriber{&funcSubscriber{fn: func(_ context.Context, _ *storage.ChangeEvent) {
			called.Store(true)
		}}},
	}}
	ctx := storage.InitChangeEvents(context.Background())
	// No events stashed.
	s.dispatchChangeEvents(ctx)
	assert.False(t, called.Load(), "subscriber should not be called with no events")
}

func TestDispatchChangeEvents_FiresSubscriber(t *testing.T) {
	var received []*storage.ChangeEvent
	s := &Store{shared: &sharedState{
		subscribers: []storage.ChangeSubscriber{&funcSubscriber{fn: func(_ context.Context, e *storage.ChangeEvent) {
			received = append(received, e)
		}}},
	}}
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "spec-1", Version: 1})
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "spec-2", Version: 2})

	s.dispatchChangeEvents(ctx)

	assert.Len(t, received, 2)
	assert.Equal(t, "spec-1", received[0].Slug)
	assert.Equal(t, "spec-2", received[1].Slug)
}

func TestDispatchChangeEvents_PanicRecovery(t *testing.T) {
	var secondCalled atomic.Bool
	s := &Store{shared: &sharedState{
		subscribers: []storage.ChangeSubscriber{
			&funcSubscriber{fn: func(_ context.Context, _ *storage.ChangeEvent) {
				panic("boom")
			}},
			&funcSubscriber{fn: func(_ context.Context, _ *storage.ChangeEvent) {
				secondCalled.Store(true)
			}},
		},
	}}
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "x"})

	// Should not panic — recovery should catch it.
	s.dispatchChangeEvents(ctx)

	assert.True(t, secondCalled.Load(), "second subscriber should still fire after first panics")
}

func TestDispatchChangeEvents_InjectsGraphBackend(t *testing.T) {
	var gotBackend bool
	s := &Store{shared: &sharedState{
		subscribers: []storage.ChangeSubscriber{&funcSubscriber{fn: func(ctx context.Context, _ *storage.ChangeEvent) {
			_, gotBackend = storage.GraphBackendFromContext(ctx)
		}}},
	}}
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "x"})

	s.dispatchChangeEvents(ctx)

	assert.True(t, gotBackend, "dispatch context should carry GraphBackend")
}

func TestSubscribe(t *testing.T) {
	s := &Store{shared: &sharedState{}}
	assert.Empty(t, s.shared.subscribers)

	sub := &funcSubscriber{}
	s.Subscribe(sub)
	assert.Len(t, s.shared.subscribers, 1)

	s.Subscribe(sub)
	assert.Len(t, s.shared.subscribers, 2)
}

type funcSubscriber struct {
	fn func(context.Context, *storage.ChangeEvent)
}

func (s *funcSubscriber) OnSpecChanged(ctx context.Context, event *storage.ChangeEvent) {
	if s.fn != nil {
		s.fn(ctx, event)
	}
}
