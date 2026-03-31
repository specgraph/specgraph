// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package notify_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/notify"
	"github.com/specgraph/specgraph/internal/storage"
)

type mockGraphBackend struct {
	refs []storage.NodeRef
	err  error
}

func (m *mockGraphBackend) GetImpact(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return m.refs, m.err
}

// Stub implementations to satisfy storage.GraphBackend.
func (m *mockGraphBackend) AddEdge(_ context.Context, _, _ string, _ storage.EdgeType) (*storage.Edge, error) {
	return nil, nil
}
func (m *mockGraphBackend) RemoveEdge(_ context.Context, _, _ string, _ storage.EdgeType) error {
	return nil
}
func (m *mockGraphBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	return nil, nil
}
func (m *mockGraphBackend) GetDependencies(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}
func (m *mockGraphBackend) GetTransitiveDeps(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}
func (m *mockGraphBackend) GetReady(_ context.Context) ([]storage.NodeRef, error) {
	return nil, nil
}
func (m *mockGraphBackend) GetCriticalPath(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}
func (m *mockGraphBackend) GetDependenciesWithEdgeData(_ context.Context, _ string) ([]storage.DependencyRef, error) {
	return nil, nil
}
func (m *mockGraphBackend) RefreshDependencyHashes(_ context.Context, _ string) error { return nil }
func (m *mockGraphBackend) GetFullGraph(_ context.Context) (*storage.FullGraph, error) {
	return nil, nil
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestImpactLogger_LogsImpactedSpecs(t *testing.T) {
	buf := captureLog(t)

	mock := &mockGraphBackend{
		refs: []storage.NodeRef{
			{Slug: "downstream-a"},
			{Slug: "downstream-b"},
		},
	}

	ctx := storage.WithGraphBackend(context.Background(), mock)
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(ctx, &storage.ChangeEvent{Slug: "upstream-spec", Version: 3})

	out := buf.String()
	if !strings.Contains(out, "upstream-spec") {
		t.Errorf("log missing slug, got: %s", out)
	}
	if !strings.Contains(out, "downstream-a") {
		t.Errorf("log missing impacted spec, got: %s", out)
	}
	if !strings.Contains(out, "impacted_count=2") {
		t.Errorf("expected impacted_count=2, got: %s", out)
	}
}

func TestImpactLogger_NoImpact(t *testing.T) {
	buf := captureLog(t)

	mock := &mockGraphBackend{refs: nil}
	ctx := storage.WithGraphBackend(context.Background(), mock)
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(ctx, &storage.ChangeEvent{Slug: "isolated-spec", Version: 1})

	out := buf.String()
	if !strings.Contains(out, "isolated-spec") {
		t.Errorf("log missing slug, got: %s", out)
	}
	if !strings.Contains(out, "impacted_count=0") {
		t.Errorf("expected impacted_count=0, got: %s", out)
	}
}

func TestImpactLogger_NoGraphBackend(_ *testing.T) {
	// Should not panic when no GraphBackend in context.
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(context.Background(), &storage.ChangeEvent{Slug: "x"})
}

func TestImpactLogger_GetImpactError(t *testing.T) {
	buf := captureLog(t)

	mock := &mockGraphBackend{err: errors.New("connection lost")}
	ctx := storage.WithGraphBackend(context.Background(), mock)
	sub := notify.NewImpactLogger()
	sub.OnSpecChanged(ctx, &storage.ChangeEvent{Slug: "error-spec", Version: 2})

	out := buf.String()
	if !strings.Contains(out, "impact analysis failed") {
		t.Errorf("expected warning log, got: %s", out)
	}
	if !strings.Contains(out, "connection lost") {
		t.Errorf("expected error detail, got: %s", out)
	}
}
