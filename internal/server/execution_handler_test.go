// SPDX-License-Identifier: Apache-2.0
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
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// stubSkillsSource is a minimal skills.Source for tests that exercise
// project-scope GetPrime (which surfaces a skills count). List returns
// a fixed, configurable slice; Get/Search are unused by Composer.Project.
type stubSkillsSource struct {
	metas []skills.Meta
	err   error
}

func (s *stubSkillsSource) List(context.Context) ([]skills.Meta, error) {
	return s.metas, s.err
}

func (*stubSkillsSource) Get(context.Context, string) (skills.Skill, error) {
	return skills.Skill{}, skills.ErrNotFound
}

func (*stubSkillsSource) Search(context.Context, string, skills.SearchOptions) ([]skills.Meta, error) {
	return nil, nil
}

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

// GetSpec serves the Spec inside the seeded PrimeData for slug, so the
// new Composer-driven GetPrime spec-scope path can reach the legacy
// summary fields without additional seeding.
func (m *mockExecutionBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pd, ok := m.primes[slug]
	if !ok || pd == nil || pd.Spec == nil {
		return nil, fmt.Errorf("mock: %w", storage.ErrSpecNotFound)
	}
	return pd.Spec, nil
}

// GetMergedConstitution returns the seeded constitution (wrapped in a
// trivial MergedResult) for the most recently seeded prime, so
// Composer.Spec can populate the SpecView constitution.
func (m *mockExecutionBackend) GetMergedConstitution(_ context.Context) (*storage.MergedResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pd := range m.primes {
		if pd != nil && pd.Constitution != nil {
			return &storage.MergedResult{Constitution: pd.Constitution}, nil
		}
	}
	// Soft empty state — no error so Composer.Spec doesn't bail.
	return nil, storage.ErrConstitutionNotFound
}

// ListSlices returns no slices for any spec — Composer.Spec then leaves
// SpecView.Slices empty. Overridden so we don't fall through to the
// stubBackend's errNotImplemented.
func (m *mockExecutionBackend) ListSlices(context.Context, string) ([]*storage.Slice, error) {
	return nil, nil
}

// ListSpecs returns all specs seeded via seedPrime. Composer.Project
// reads through this method to bucket specs by stage; without the
// override the embedded stubBackend returns errNotImplemented.
func (m *mockExecutionBackend) ListSpecs(context.Context, string, string, int) ([]*storage.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*storage.Spec, 0, len(m.primes))
	for _, pd := range m.primes {
		if pd != nil && pd.Spec != nil {
			out = append(out, pd.Spec)
		}
	}
	return out, nil
}

// GetReady returns no ready specs by default — sufficient for the
// project-scope tests in this file.
func (m *mockExecutionBackend) GetReady(context.Context) ([]storage.NodeRef, error) {
	return nil, nil
}

// ListAllFindings returns no findings by default.
func (m *mockExecutionBackend) ListAllFindings(context.Context) ([]*storage.AnalyticalFinding, error) {
	return nil, nil
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
	return setupExecutionServerWithSkills(t, mb, nil)
}

// setupExecutionServerWithSkills wires the ExecutionService with a
// caller-supplied skills.Source. Tests that exercise project-scope
// GetPrime must pass a non-nil source because Composer.Project lists
// the catalog.
func setupExecutionServerWithSkills(t *testing.T, mb storage.ExecutionBackend, src skills.Source) specgraphv1connect.ExecutionServiceClient {
	t.Helper()
	// mb must implement ScopedBackend (mockExecutionBackend embeds stubBackend).
	scoper := &testScoper{backend: mb.(storage.ScopedBackend)}
	mux := http.NewServeMux()
	server.RegisterExecutionService(mux, scoper, src)
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
			Provenance:  storage.SpecProvenanceAuthored,
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
	require.Equal(t, int32(1), resp.Msg.GetBundle().GetVersion())
	require.Equal(t, "my-spec", resp.Msg.GetBundle().GetSpec().GetSlug())
	require.Equal(t, "build a widget", resp.Msg.GetBundle().GetSpec().GetIntent())
	require.Len(t, resp.Msg.GetBundle().GetDecisions(), 1)
	require.Equal(t, "adr-001", resp.Msg.GetBundle().GetDecisions()[0].GetSlug())
	require.NotNil(t, resp.Msg.GetBundle().GetCallbacks())
	require.Equal(t, "http://localhost:8080", resp.Msg.GetBundle().GetCallbacks().GetEndpoint())
	require.Equal(t, "http://localhost:8080/progress", resp.Msg.GetBundle().GetCallbacks().GetProgress())
	require.NotEmpty(t, resp.Msg.GetBundle().GetBundleContent())
	require.Contains(t, resp.Msg.GetBundle().GetBundleContent(), "my-spec")
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
		Spec: &storage.Spec{Slug: "my-spec", Intent: "build a widget", Stage: storage.SpecStageApproved, Provenance: storage.SpecProvenanceAuthored, ContentHash: strings.Repeat("a", 32)},
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

	// Legacy summary fields (1–5) — preserved for backward compatibility
	// with older polecat consumers.
	require.Equal(t, "MyProject (project layer)", resp.Msg.GetConstitutionSummary())
	require.Equal(t, "build a widget", resp.Msg.GetProjectContext())
	require.Len(t, resp.Msg.GetDecisions(), 1)
	require.Contains(t, resp.Msg.GetCodingConventions(), "Keep it simple")
	require.Contains(t, resp.Msg.GetCodingConventions(), "No global state")
	require.NotEmpty(t, resp.Msg.GetCallbackDocs())

	// New view oneof — spec_view populated for spec-scope responses.
	require.NotNil(t, resp.Msg.GetSpecView(), "spec-scope response must populate spec_view oneof")
	require.Nil(t, resp.Msg.GetProjectView(), "spec-scope response must not populate project_view oneof")
	require.Equal(t, "my-spec", resp.Msg.GetSpecView().GetSpec().GetSlug())
}

func TestGetPrime_EmptySlug_ProjectView(t *testing.T) {
	mb := newMockExecutionBackend()
	src := &stubSkillsSource{
		metas: []skills.Meta{
			{Name: "skill-a", URI: "specgraph://skills/skill-a"},
			{Name: "skill-b", URI: "specgraph://skills/skill-b"},
		},
	}
	client := setupExecutionServerWithSkills(t, mb, src)

	resp, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
		Slug: "",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.GetProjectView(), "empty-slug response must populate project_view oneof")
	require.Nil(t, resp.Msg.GetSpecView(), "empty-slug response must not populate spec_view oneof")
	require.Equal(t, int32(2), resp.Msg.GetProjectView().GetSkillsCount())

	// Legacy summary fields are intentionally left zero on project-scope
	// responses (design Section 10).
	require.Empty(t, resp.Msg.GetConstitutionSummary())
	require.Empty(t, resp.Msg.GetProjectContext())
	require.Empty(t, resp.Msg.GetCodingConventions())
	require.Empty(t, resp.Msg.GetCallbackDocs())
	require.Empty(t, resp.Msg.GetDecisions())
}

func TestGetPrime_NonEmptySlug_SpecView_PopulatesLegacyFields(t *testing.T) {
	mb := newMockExecutionBackend()
	mb.seedPrime("widget", &storage.PrimeData{
		Spec: &storage.Spec{Slug: "widget", Intent: "build a widget", Stage: storage.SpecStageApproved, Provenance: storage.SpecProvenanceAuthored, ContentHash: strings.Repeat("a", 32)},
		Decisions: []*storage.Decision{
			{Slug: "adr-001", Title: "Use Go", Status: storage.DecisionStatusAccepted, ContentHash: strings.Repeat("a", 32)},
		},
		Constitution: &storage.Constitution{
			Name:        "MyProject",
			Layer:       storage.ConstitutionLayerProject,
			Principles:  []storage.Principle{{Statement: "Keep it simple"}},
			Constraints: []string{"No global state"},
		},
	})
	client := setupExecutionServer(t, mb)

	resp, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
		Slug: "widget",
	}))
	require.NoError(t, err)

	// spec_view populated.
	sv := resp.Msg.GetSpecView()
	require.NotNil(t, sv)
	require.Equal(t, "widget", sv.GetSpec().GetSlug())

	// Legacy summary fields populated (back-compat).
	require.Equal(t, "MyProject (project layer)", resp.Msg.GetConstitutionSummary())
	require.Equal(t, "build a widget", resp.Msg.GetProjectContext())
	require.Len(t, resp.Msg.GetDecisions(), 1)
	require.Contains(t, resp.Msg.GetCodingConventions(), "Keep it simple")
	require.NotEmpty(t, resp.Msg.GetCallbackDocs())
}

func TestGetPrime_ViewOneofInvariant(t *testing.T) {
	t.Run("empty slug -> exactly project_view", func(t *testing.T) {
		mb := newMockExecutionBackend()
		client := setupExecutionServerWithSkills(t, mb, &stubSkillsSource{})
		resp, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
			Slug: "",
		}))
		require.NoError(t, err)
		assertExactlyOneView(t, resp.Msg, viewKindProject)
	})

	t.Run("non-empty slug -> exactly spec_view", func(t *testing.T) {
		mb := newMockExecutionBackend()
		mb.seedPrime("widget", &storage.PrimeData{
			Spec: &storage.Spec{Slug: "widget", Intent: "x", Stage: storage.SpecStageApproved, Provenance: storage.SpecProvenanceAuthored, ContentHash: strings.Repeat("a", 32)},
		})
		client := setupExecutionServer(t, mb)
		resp, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
			Slug: "widget",
		}))
		require.NoError(t, err)
		assertExactlyOneView(t, resp.Msg, viewKindSpec)
	})
}

func TestGetPrime_UnknownSlug_NotFound(t *testing.T) {
	mb := newMockExecutionBackend()
	client := setupExecutionServer(t, mb)

	_, err := client.GetPrime(context.Background(), connect.NewRequest(&specv1.GetPrimeRequest{
		Slug: "no-such-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

type viewKind int

const (
	viewKindProject viewKind = iota
	viewKindSpec
)

// assertExactlyOneView asserts that resp.View is set to exactly the
// expected oneof arm: type-switching on the typed View field ensures
// the other arm is nil.
func assertExactlyOneView(t *testing.T, resp *specv1.PrimeResponse, want viewKind) {
	t.Helper()
	switch v := resp.GetView().(type) {
	case *specv1.PrimeResponse_ProjectView:
		require.Equal(t, viewKindProject, want, "got project_view, expected spec_view")
		require.NotNil(t, v.ProjectView)
		require.Nil(t, resp.GetSpecView())
	case *specv1.PrimeResponse_SpecView:
		require.Equal(t, viewKindSpec, want, "got spec_view, expected project_view")
		require.NotNil(t, v.SpecView)
		require.Nil(t, resp.GetProjectView())
	case nil:
		t.Fatalf("view oneof must be set, got nil")
	default:
		t.Fatalf("unexpected view oneof arm: %T", v)
	}
}

// mockExecutionBackendNotApproved wraps mockExecutionBackend but returns
// ErrSpecNotApproved from GenerateBundle.
type mockExecutionBackendNotApproved struct {
	*mockExecutionBackend
}

func (m *mockExecutionBackendNotApproved) GenerateBundle(_ context.Context, _ string) (*storage.Bundle, error) {
	return nil, fmt.Errorf("mock: %w", storage.ErrSpecNotApproved)
}
