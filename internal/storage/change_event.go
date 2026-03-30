// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "context"

// ChangeEvent is emitted after a changelog entry is persisted.
type ChangeEvent struct {
	Slug        string
	Version     int32
	Stage       SpecStage
	ContentHash string
	Checkpoint  bool
	Summary     string
	Reason      string
}

// ChangeSubscriber receives notifications after spec changes are committed.
type ChangeSubscriber interface {
	OnSpecChanged(ctx context.Context, event ChangeEvent)
}

// Subscribable is implemented by storage backends that support change notifications.
type Subscribable interface {
	Subscribe(ChangeSubscriber)
}

// changeEventsKey is the context key for stashed change events.
type changeEventsKey struct{}

// InitChangeEvents returns a new context with an empty event slice for stashing.
func InitChangeEvents(ctx context.Context) context.Context {
	events := make([]ChangeEvent, 0, 4)
	return context.WithValue(ctx, changeEventsKey{}, &events)
}

// StashChangeEvent appends an event to the context's event slice.
// No-op if the context has no event slice (non-transactional path).
func StashChangeEvent(ctx context.Context, event ChangeEvent) {
	ptr, ok := ctx.Value(changeEventsKey{}).(*[]ChangeEvent)
	if !ok || ptr == nil {
		return
	}
	*ptr = append(*ptr, event)
}

// DrainChangeEvents returns all stashed events from the context.
func DrainChangeEvents(ctx context.Context) []ChangeEvent {
	ptr, ok := ctx.Value(changeEventsKey{}).(*[]ChangeEvent)
	if !ok || ptr == nil {
		return nil
	}
	return *ptr
}

type graphBackendKey struct{}

// WithGraphBackend returns a context carrying the given GraphBackend.
func WithGraphBackend(ctx context.Context, g GraphBackend) context.Context {
	return context.WithValue(ctx, graphBackendKey{}, g)
}

// GraphBackendFromContext extracts a GraphBackend from context, if present.
func GraphBackendFromContext(ctx context.Context) (GraphBackend, bool) {
	g, ok := ctx.Value(graphBackendKey{}).(GraphBackend)
	return g, ok
}
