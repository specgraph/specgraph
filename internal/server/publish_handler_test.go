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
	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// --- mock publisher ---

// mockPublisher is a no-op publish.Publisher used for handler tests.
type mockPublisher struct {
	mu          sync.Mutex
	published   []string // slugs that had Publish called
	unpublished []string // slugs that had Unpublish called
	publishErr  error
	unpublishErr error
}

func (m *mockPublisher) Name() string { return "mock" }

func (m *mockPublisher) Publish(_ context.Context, slug string, _ []render.Document) (publish.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return publish.Result{}, m.publishErr
	}
	m.published = append(m.published, slug)
	return publish.Result{}, nil
}

func (m *mockPublisher) Update(_ context.Context, slug string, _ []render.Document, _ *specv1.ChangeLogEntry) (publish.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, slug)
	return publish.Result{}, nil
}

func (m *mockPublisher) Unpublish(_ context.Context, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unpublishErr != nil {
		return m.unpublishErr
	}
	m.unpublished = append(m.unpublished, slug)
	return nil
}

func (m *mockPublisher) Status(_ context.Context, _ string) (publish.Status, error) {
	return publish.Status{}, nil
}

// --- mock feedback source ---

// mockFeedbackSource returns a pre-configured list of feedback entries.
type mockFeedbackSource struct {
	mu       sync.Mutex
	feedback map[string][]publish.Feedback // keyed by spec slug
	pollErr  error
}

func (m *mockFeedbackSource) Poll(_ context.Context, slug string) ([]publish.Feedback, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pollErr != nil {
		return nil, m.pollErr
	}
	return m.feedback[slug], nil
}

// --- mock publish backend ---

// mockPublishBackend embeds stubBackend and overrides publish-related storage methods.
type mockPublishBackend struct {
	stubBackend
	mu       sync.Mutex
	specs    map[string]*storage.Spec
	mappings []*storage.PageMapping
	feedback []*storage.FeedbackEntry
	seq      int
}

func newMockPublishBackend() *mockPublishBackend {
	return &mockPublishBackend{
		specs: make(map[string]*storage.Spec),
	}
}

func (m *mockPublishBackend) addSpec(slug string, withShape bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	now := time.Now().UTC()
	spec := &storage.Spec{
		ID:          fmt.Sprintf("spec-%05d", m.seq),
		Slug:        slug,
		Intent:      "Test intent for " + slug,
		Stage:       storage.SpecStageShape,
		Priority:    storage.SpecPriorityP1,
		Complexity:  storage.SpecComplexityMedium,
		Version:     1,
		ContentHash: strings.Repeat("a", 32),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if withShape {
		spec.ShapeOutput = &storage.ShapeOutput{
			ScopeIn:        []string{"feature X"},
			ChosenApproach: "approach A",
		}
	}
	m.specs[slug] = spec
}

func (m *mockPublishBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return spec, nil
}

func (m *mockPublishBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	// Return no edges for simplicity — tests that need decisions override this.
	return nil, nil
}

func (m *mockPublishBackend) ListPageMappings(_ context.Context, specSlug string) ([]*storage.PageMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*storage.PageMapping
	for _, pm := range m.mappings {
		if specSlug == "" || pm.SpecSlug == specSlug {
			result = append(result, pm)
		}
	}
	return result, nil
}

func (m *mockPublishBackend) DeletePageMappings(_ context.Context, specSlug string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var remaining []*storage.PageMapping
	removed := 0
	for _, pm := range m.mappings {
		if pm.SpecSlug == specSlug {
			removed++
		} else {
			remaining = append(remaining, pm)
		}
	}
	m.mappings = remaining
	return removed, nil
}

func (m *mockPublishBackend) UpsertPageMapping(_ context.Context, pm *storage.PageMapping) (*storage.PageMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mappings = append(m.mappings, pm)
	return pm, nil
}

func (m *mockPublishBackend) StoreFeedback(_ context.Context, entry *storage.FeedbackEntry) (*storage.FeedbackEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feedback = append(m.feedback, entry)
	return entry, nil
}

func (m *mockPublishBackend) CountNewFeedback(_ context.Context, _ string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.feedback), nil
}

func (m *mockPublishBackend) addPageMapping(specSlug string, kind storage.DocumentKind) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mappings = append(m.mappings, &storage.PageMapping{
		SpecSlug:    specSlug,
		DocKind:     kind,
		PageID:      "page-" + string(kind),
		PageVersion: 1,
		SpecVersion: 1,
		State:       storage.PublishStateSynced,
		LastSync:    time.Now().UTC(),
	})
}

// --- setup helpers ---

type publishTestSetup struct {
	client   specgraphv1connect.PublishServiceClient
	backend  *mockPublishBackend
	pub      *mockPublisher
	feedback *mockFeedbackSource
}

func setupPublishServer(t *testing.T) *publishTestSetup {
	t.Helper()
	backend := newMockPublishBackend()
	pub := &mockPublisher{}
	fs := &mockFeedbackSource{feedback: make(map[string][]publish.Feedback)}
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterPublishService(mux, scoper, pub, fs)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return &publishTestSetup{
		client:   specgraphv1connect.NewPublishServiceClient(http.DefaultClient, srv.URL),
		backend:  backend,
		pub:      pub,
		feedback: fs,
	}
}

func setupPublishServerNoFeedback(t *testing.T) *publishTestSetup {
	t.Helper()
	backend := newMockPublishBackend()
	pub := &mockPublisher{}
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterPublishService(mux, scoper, pub, nil) // nil feedback source
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return &publishTestSetup{
		client:  specgraphv1connect.NewPublishServiceClient(http.DefaultClient, srv.URL),
		backend: backend,
		pub:     pub,
	}
}

// --- tests ---

func TestPublishHandler_Publish_Success(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Seed a spec with shape output so PublishAll has something to render.
	ts.backend.addSpec("my-spec", true)

	resp, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	// publisher.Publish was called for the spec (no decisions linked).
	ts.pub.mu.Lock()
	defer ts.pub.mu.Unlock()
	require.Contains(t, ts.pub.published, "my-spec")
}

func TestPublishHandler_Publish_NoShapeOutput(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Spec exists but has no shape output — PublishAll skips publishing,
	// returns empty mappings. Should succeed without error.
	ts.backend.addSpec("bare-spec", false)

	resp, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
		Slug: "bare-spec",
	}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Msg.GetMappings())
}

func TestPublishHandler_Publish_MissingSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
		Slug: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPublishHandler_Publish_SpecNotFound(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
		Slug: "nonexistent-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestPublishHandler_Publish_InvalidSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	tests := []struct {
		name string
		slug string
	}{
		{"path traversal", "../admin"},
		{"uppercase letters", "My-Spec"},
		{"too long", strings.Repeat("a", 257)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
				Slug: tt.slug,
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestPublishHandler_GetPublishStatus_WithMappings(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Add page mappings for two specs.
	ts.backend.addPageMapping("spec-alpha", storage.DocumentKindPRD)
	ts.backend.addPageMapping("spec-alpha", storage.DocumentKindSDD)
	ts.backend.addPageMapping("spec-beta", storage.DocumentKindPRD)

	resp, err := ts.client.GetPublishStatus(ctx, connect.NewRequest(&specv1.GetPublishStatusRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetEntries(), 2)

	// Find the entry for spec-alpha and verify both mappings are grouped.
	var alphaEntry *specv1.PublishStatusEntry
	for _, e := range resp.Msg.GetEntries() {
		if e.GetSpecSlug() == "spec-alpha" {
			alphaEntry = e
			break
		}
	}
	require.NotNil(t, alphaEntry, "expected entry for spec-alpha")
	require.NotNil(t, alphaEntry.GetPrd())
	require.NotNil(t, alphaEntry.GetSdd())
}

func TestPublishHandler_GetPublishStatus_Empty(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	resp, err := ts.client.GetPublishStatus(ctx, connect.NewRequest(&specv1.GetPublishStatusRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.GetEntries())
}

func TestPublishHandler_GetPublishStatus_FilteredBySlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	ts.backend.addPageMapping("spec-alpha", storage.DocumentKindPRD)
	ts.backend.addPageMapping("spec-beta", storage.DocumentKindPRD)

	resp, err := ts.client.GetPublishStatus(ctx, connect.NewRequest(&specv1.GetPublishStatusRequest{
		Slug: "spec-alpha",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetEntries(), 1)
	require.Equal(t, "spec-alpha", resp.Msg.GetEntries()[0].GetSpecSlug())
}

func TestPublishHandler_GetPublishStatus_InvalidSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.GetPublishStatus(ctx, connect.NewRequest(&specv1.GetPublishStatusRequest{
		Slug: "INVALID_SLUG",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPublishHandler_SyncComments_Success(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Add page mappings so the handler knows which slugs to sync.
	ts.backend.addPageMapping("spec-gamma", storage.DocumentKindPRD)

	// Configure feedback to return for spec-gamma.
	ts.feedback.feedback["spec-gamma"] = []publish.Feedback{
		{
			ExternalID: "comment-001",
			Author:     "alice",
			Body:       "Looks good!",
			Timestamp:  time.Now().UTC(),
			Kind:       publish.FeedbackFooter,
		},
	}

	resp, err := ts.client.SyncComments(ctx, connect.NewRequest(&specv1.SyncCommentsRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.GetNewCount())
	require.EqualValues(t, 1, resp.Msg.GetSpecCount())
	require.Len(t, resp.Msg.GetFeedback(), 1)
	require.Equal(t, "alice", resp.Msg.GetFeedback()[0].GetAuthor())
}

func TestPublishHandler_SyncComments_SpecificSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	ts.feedback.feedback["spec-delta"] = []publish.Feedback{
		{
			ExternalID: "comment-002",
			Author:     "bob",
			Body:       "Needs revision",
			Timestamp:  time.Now().UTC(),
			Kind:       publish.FeedbackInline,
			Stage:      "shape",
		},
	}

	resp, err := ts.client.SyncComments(ctx, connect.NewRequest(&specv1.SyncCommentsRequest{
		Slug: "spec-delta",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 1, resp.Msg.GetNewCount())
	require.Len(t, resp.Msg.GetFeedback(), 1)
	require.Equal(t, "bob", resp.Msg.GetFeedback()[0].GetAuthor())
	require.Equal(t, specv1.FeedbackKind_FEEDBACK_KIND_INLINE, resp.Msg.GetFeedback()[0].GetKind())
}

func TestPublishHandler_SyncComments_NoFeedbackSource(t *testing.T) {
	ts := setupPublishServerNoFeedback(t)
	ctx := context.Background()

	_, err := ts.client.SyncComments(ctx, connect.NewRequest(&specv1.SyncCommentsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestPublishHandler_SyncComments_NoMappings_EmptyResult(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// No page mappings — nothing to sync.
	resp, err := ts.client.SyncComments(ctx, connect.NewRequest(&specv1.SyncCommentsRequest{}))
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Msg.GetNewCount())
	require.EqualValues(t, 0, resp.Msg.GetSpecCount())
	require.Empty(t, resp.Msg.GetFeedback())
}

func TestPublishHandler_SyncComments_InvalidSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.SyncComments(ctx, connect.NewRequest(&specv1.SyncCommentsRequest{
		Slug: "../traversal",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPublishHandler_Unpublish_Success(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Seed some page mappings.
	ts.backend.addPageMapping("spec-epsilon", storage.DocumentKindPRD)
	ts.backend.addPageMapping("spec-epsilon", storage.DocumentKindSDD)

	resp, err := ts.client.Unpublish(ctx, connect.NewRequest(&specv1.UnpublishRequest{
		Slug: "spec-epsilon",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 2, resp.Msg.GetPagesRemoved())

	// Verify publisher.Unpublish was called.
	ts.pub.mu.Lock()
	defer ts.pub.mu.Unlock()
	require.Contains(t, ts.pub.unpublished, "spec-epsilon")
}

func TestPublishHandler_Unpublish_NoMappings(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// No mappings exist — should succeed with 0 removed.
	resp, err := ts.client.Unpublish(ctx, connect.NewRequest(&specv1.UnpublishRequest{
		Slug: "spec-zeta",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 0, resp.Msg.GetPagesRemoved())
}

func TestPublishHandler_Unpublish_MissingSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.Unpublish(ctx, connect.NewRequest(&specv1.UnpublishRequest{
		Slug: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPublishHandler_Unpublish_InvalidSlug(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	_, err := ts.client.Unpublish(ctx, connect.NewRequest(&specv1.UnpublishRequest{
		Slug: "INVALID",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestPublishHandler_Publish_PublisherError(t *testing.T) {
	ts := setupPublishServer(t)
	ctx := context.Background()

	// Seed a spec so the handler gets past the GetSpec check.
	ts.backend.addSpec("error-spec", true)

	// Make the mock publisher return an error from Publish.
	ts.pub.mu.Lock()
	ts.pub.publishErr = fmt.Errorf("confluence unavailable")
	ts.pub.mu.Unlock()

	_, err := ts.client.Publish(ctx, connect.NewRequest(&specv1.PublishRequest{
		Slug: "error-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
