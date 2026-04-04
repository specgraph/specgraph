// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestStashAndDrainChangeEvents(t *testing.T) {
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "spec-a", Version: 1})
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{Slug: "spec-b", Version: 2})

	events := storage.DrainChangeEvents(ctx)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Slug != "spec-a" {
		t.Errorf("events[0].Slug = %q, want spec-a", events[0].Slug)
	}
	if events[1].Slug != "spec-b" {
		t.Errorf("events[1].Slug = %q, want spec-b", events[1].Slug)
	}
}

func TestDrainChangeEvents_NoInit(t *testing.T) {
	events := storage.DrainChangeEvents(context.Background())
	if len(events) != 0 {
		t.Fatalf("got %d events from un-initialized context, want 0", len(events))
	}
}

func TestStashChangeEvent_NoInit(_ *testing.T) {
	// Should not panic on un-initialized context.
	storage.StashChangeEvent(context.Background(), &storage.ChangeEvent{Slug: "x"})
}

func TestChangeEventPreservesAllFields(t *testing.T) {
	ctx := storage.InitChangeEvents(context.Background())
	storage.StashChangeEvent(ctx, &storage.ChangeEvent{
		Slug:        "s1",
		Version:     5,
		Stage:       string(storage.SpecStageShape),
		ContentHash: "abc123",
		Checkpoint:  true,
		Summary:     "test summary",
		Reason:      "test reason",
	})

	events := storage.DrainChangeEvents(ctx)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Slug != "s1" || e.Version != 5 || e.Stage != string(storage.SpecStageShape) ||
		e.ContentHash != "abc123" || !e.Checkpoint || e.Summary != "test summary" || e.Reason != "test reason" {
		t.Errorf("field mismatch: %+v", e)
	}
}

// minimalGraphBackend is a minimal implementation of storage.GraphBackend used
// only to test context round-trip threading — all methods are no-ops.
type minimalGraphBackend struct{}

func (minimalGraphBackend) AddEdge(_ context.Context, _, _ string, _ storage.EdgeType) (*storage.Edge, error) {
	return nil, nil
}

func (minimalGraphBackend) RemoveEdge(_ context.Context, _, _ string, _ storage.EdgeType) error {
	return nil
}

func (minimalGraphBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	return nil, nil
}

func (minimalGraphBackend) GetDependencies(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (minimalGraphBackend) GetTransitiveDeps(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (minimalGraphBackend) GetImpact(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (minimalGraphBackend) GetReady(_ context.Context) ([]storage.NodeRef, error) {
	return nil, nil
}

func (minimalGraphBackend) GetCriticalPath(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (minimalGraphBackend) GetDependenciesWithEdgeData(_ context.Context, _ string) ([]storage.DependencyRef, error) {
	return nil, nil
}
func (minimalGraphBackend) RefreshDependencyHashes(_ context.Context, _ string) error { return nil }
func (minimalGraphBackend) GetFullGraph(_ context.Context) (*storage.FullGraph, error) {
	return nil, nil
}

func TestWithGraphBackend_RoundTrip(t *testing.T) {
	ctx := context.Background()
	_, ok := storage.GraphBackendFromContext(ctx)
	if ok {
		t.Error("expected no GraphBackend in empty context")
	}

	var backend storage.GraphBackend = minimalGraphBackend{}
	ctx = storage.WithGraphBackend(ctx, backend)
	got, ok := storage.GraphBackendFromContext(ctx)
	if !ok {
		t.Fatal("expected GraphBackend in context")
	}
	if got != backend {
		t.Error("got different backend than stored")
	}
}
