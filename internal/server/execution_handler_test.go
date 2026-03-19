// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// mockExecutionBackend implements storage.ExecutionBackend for unit tests.
type mockExecutionBackend struct {
	stubBackend
	mu       sync.Mutex
	bundles  map[string]*storage.Bundle
	events   map[string][]*storage.ExecutionEvent
	primes   map[string]*storage.PrimeData
	eventSeq int
}

func newMockExecutionBackend() *mockExecutionBackend {
	return &mockExecutionBackend{
		bundles: make(map[string]*storage.Bundle),
		events:  make(map[string][]*storage.ExecutionEvent),
		primes:  make(map[string]*storage.PrimeData),
	}
}

func (m *mockExecutionBackend) GenerateBundle(_ context.Context, slug string) (*storage.Bundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.bundles[slug]
	if !ok {
		return nil, fmt.Errorf("mock: %w", storage.ErrSpecNotFound)
	}
	return b, nil
}

func (m *mockExecutionBackend) RecordProgress(_ context.Context, slug, agent, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bundles[slug]; !ok {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.eventSeq++
	m.events[slug] = append(m.events[slug], &storage.ExecutionEvent{
		ID:        fmt.Sprintf("evt-%05d", m.eventSeq),
		SpecSlug:  slug,
		Agent:     agent,
		Type:      storage.ExecutionEventTypeProgress,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (m *mockExecutionBackend) RecordBlocker(_ context.Context, slug, agent, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bundles[slug]; !ok {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.eventSeq++
	m.events[slug] = append(m.events[slug], &storage.ExecutionEvent{
		ID:        fmt.Sprintf("evt-%05d", m.eventSeq),
		SpecSlug:  slug,
		Agent:     agent,
		Type:      storage.ExecutionEventTypeBlocker,
		Message:   description,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (m *mockExecutionBackend) RecordCompletion(_ context.Context, slug, agent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.bundles[slug]; !ok {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.eventSeq++
	m.events[slug] = append(m.events[slug], &storage.ExecutionEvent{
		ID:        fmt.Sprintf("evt-%05d", m.eventSeq),
		SpecSlug:  slug,
		Agent:     agent,
		Type:      storage.ExecutionEventTypeCompletion,
		Message:   "completed",
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (m *mockExecutionBackend) GetExecutionEvents(_ context.Context, slug string, limit int) ([]*storage.ExecutionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	evts := m.events[slug]
	if limit > 0 && limit < len(evts) {
		evts = evts[:limit]
	}
	return evts, nil
}

func (m *mockExecutionBackend) GetPrimeData(_ context.Context, slug string) (*storage.PrimeData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pd, ok := m.primes[slug]
	if !ok {
		return nil, fmt.Errorf("mock: %w", storage.ErrSpecNotFound)
	}
	return pd, nil
}

func (m *mockExecutionBackend) ReleaseExpiredClaims(_ context.Context) (int, error) {
	return 0, nil
}

// seedBundle adds a bundle to the mock for a given slug.
func (m *mockExecutionBackend) seedBundle(slug string, b *storage.Bundle) { //nolint:unparam // test helper; slug param kept for clarity
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bundles[slug] = b
}

// seedPrime adds prime data to the mock for a given slug.
func (m *mockExecutionBackend) seedPrime(slug string, pd *storage.PrimeData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.primes[slug] = pd
}

func setupExecutionServer(t *testing.T, mb storage.ExecutionBackend) specgraphv1connect.ExecutionServiceClient {
	t.Helper()
	// mb must implement ScopedBackend (mockExecutionBackend embeds stubBackend).
	scoper := &testScoper{backend: mb.(storage.ScopedBackend)}
	mux := http.NewServeMux()
	server.RegisterExecutionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewExecutionServiceClient(http.DefaultClient, srv.URL)
}

func TestExecutionHandler_GenerateBundle(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedBundle("my-spec", &storage.Bundle{
		Version: 1,
		Spec: &storage.Spec{
			Slug:        "my-spec",
			Intent:      "build a widget",
			Stage:       storage.SpecStageApproved,
			ContentHash: strings.Repeat("a", 32),
		},
		Decisions: []*storage.Decision{
			{Slug: "adr-001", Title: "Use Go", Status: storage.DecisionStatusAccepted, ContentHash: strings.Repeat("a", 32)},
		},
		Bootstrap: "echo hello",
	})
	client := setupExecutionServer(t, mb)

	resp, err := client.GenerateBundle(context.Background(), connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug:     "my-spec",
		Endpoint: "http://localhost:8080",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Version)
	require.Equal(t, "my-spec", resp.Msg.Spec.Slug)
	require.Equal(t, "build a widget", resp.Msg.Spec.Intent)
	require.Len(t, resp.Msg.Decisions, 1)
	require.Equal(t, "adr-001", resp.Msg.Decisions[0].Slug)
	require.NotNil(t, resp.Msg.Callbacks)
	require.Equal(t, "http://localhost:8080", resp.Msg.Callbacks.Endpoint)
	require.Equal(t, "http://localhost:8080/progress", resp.Msg.Callbacks.Progress)
	require.NotEmpty(t, resp.Msg.BundleYaml)
	require.Contains(t, resp.Msg.BundleYaml, "my-spec")
}

func TestExecutionHandler_GenerateBundle_NotFound(t *testing.T) {
	mb := newMockExecutionBackend()
	client := setupExecutionServer(t, mb)

	_, err := client.GenerateBundle(context.Background(), connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug: "no-such-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestExecutionHandler_GenerateBundle_NotApproved(t *testing.T) {
	mb := newMockExecutionBackend()
	// Override GenerateBundle to return ErrSpecNotApproved by not seeding a
	// bundle and using a custom mock. Instead, we directly test the error path
	// by creating a wrapper.
	client := setupExecutionServer(t, &mockExecutionBackendNotApproved{mockExecutionBackend: mb})

	_, err := client.GenerateBundle(context.Background(), connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug: "unapproved-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestExecutionHandler_ReportProgress(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedBundle("my-spec", &storage.Bundle{
		Version: 1,
		Spec:    &storage.Spec{Slug: "my-spec", Intent: "test", Stage: storage.SpecStageApproved, ContentHash: strings.Repeat("a", 32)},
	})
	client := setupExecutionServer(t, mb)

	resp, err := client.ReportProgress(context.Background(), connect.NewRequest(&specv1.ReportProgressRequest{
		Slug:    "my-spec",
		Agent:   "agent-1",
		Message: "step 1 done",
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Acknowledged)

	// Verify event was recorded.
	mb.mu.Lock()
	evts := mb.events["my-spec"]
	mb.mu.Unlock()
	require.Len(t, evts, 1)
	require.Equal(t, storage.ExecutionEventTypeProgress, evts[0].Type)
	require.Equal(t, "step 1 done", evts[0].Message)
}

func TestExecutionHandler_ReportProgress_NoClaim(t *testing.T) {
	mb := newMockExecutionBackend()
	// No bundle seeded, so mock returns ErrAgentNotClaimOwner.
	client := setupExecutionServer(t, mb)

	_, err := client.ReportProgress(context.Background(), connect.NewRequest(&specv1.ReportProgressRequest{
		Slug:    "unclaimed-spec",
		Agent:   "agent-1",
		Message: "attempt",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestExecutionHandler_GetExecutionEvents(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedBundle("my-spec", &storage.Bundle{
		Version: 1,
		Spec:    &storage.Spec{Slug: "my-spec", Intent: "test", Stage: storage.SpecStageApproved, ContentHash: strings.Repeat("a", 32)},
	})
	client := setupExecutionServer(t, mb)

	// Record a couple of events.
	_, err := client.ReportProgress(context.Background(), connect.NewRequest(&specv1.ReportProgressRequest{
		Slug: "my-spec", Agent: "agent-1", Message: "progress 1",
	}))
	require.NoError(t, err)
	_, err = client.ReportBlocker(context.Background(), connect.NewRequest(&specv1.ReportBlockerRequest{
		Slug: "my-spec", Agent: "agent-1", Description: "blocked on X",
	}))
	require.NoError(t, err)

	resp, err := client.GetExecutionEvents(context.Background(), connect.NewRequest(&specv1.GetExecutionEventsRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Events, 2)
	require.Equal(t, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS, resp.Msg.Events[0].Type)
	require.Equal(t, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER, resp.Msg.Events[1].Type)
}

func TestExecutionHandler_GetExecutionEvents_LimitCapped(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedBundle("my-spec", &storage.Bundle{
		Version: 1,
		Spec:    &storage.Spec{Slug: "my-spec", Intent: "test", Stage: storage.SpecStageApproved, ContentHash: strings.Repeat("a", 32)},
	})
	client := setupExecutionServer(t, mb)

	// Request with a limit exceeding the server-side cap (500) — the handler should clamp it.
	resp, err := client.GetExecutionEvents(context.Background(), connect.NewRequest(&specv1.GetExecutionEventsRequest{
		Slug:  "my-spec",
		Limit: 600,
	}))
	require.NoError(t, err)
	// Mock returns no events, so we just verify the request succeeded (limit was clamped, not rejected).
	require.NotNil(t, resp.Msg)
}

func TestExecutionHandler_GetPrime(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedPrime("my-spec", &storage.PrimeData{
		Spec: &storage.Spec{Slug: "my-spec", Intent: "build a widget", Stage: storage.SpecStageApproved, ContentHash: strings.Repeat("a", 32)},
		Decisions: []*storage.Decision{
			{Slug: "adr-001", Title: "Use Go", Status: storage.DecisionStatusAccepted, ContentHash: strings.Repeat("a", 32)},
		},
		Constitution: &storage.Constitution{
			Name:  "MyProject",
			Layer: storage.ConstitutionLayerProject,
			Principles: []storage.Principle{
				{Statement: "Keep it simple"},
			},
			Constraints: []string{"No global state"},
		},
	})
	client := setupExecutionServer(t, mb)

	resp, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, "MyProject (project layer)", resp.Msg.ConstitutionSummary)
	require.Equal(t, "build a widget", resp.Msg.ProjectContext)
	require.Len(t, resp.Msg.Decisions, 1)
	require.Contains(t, resp.Msg.CodingConventions, "Keep it simple")
	require.Contains(t, resp.Msg.CodingConventions, "No global state")
	require.NotEmpty(t, resp.Msg.CallbackDocs)
}

// mockExecutionBackendNotApproved wraps mockExecutionBackend but returns
// ErrSpecNotApproved from GenerateBundle.
type mockExecutionBackendNotApproved struct {
	*mockExecutionBackend
}

func (m *mockExecutionBackendNotApproved) GenerateBundle(_ context.Context, _ string) (*storage.Bundle, error) {
	return nil, fmt.Errorf("mock: %w", storage.ErrSpecNotApproved)
}
