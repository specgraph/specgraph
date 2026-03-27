// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockSliceBackend struct {
	stubBackend
	mu             sync.Mutex
	slices         map[string]*storage.Slice
	seq            int
	listSlicesErr  error // injected error for ListSlices
	getSliceErr    error // injected error for GetSlice
	claimSliceErr  error // injected error for ClaimSlice
	completeSliceErr error // injected error for CompleteSlice
}

func newMockSliceBackend() *mockSliceBackend {
	return &mockSliceBackend{slices: make(map[string]*storage.Slice)}
}

func (m *mockSliceBackend) CreateSlice(_ context.Context, sl *storage.Slice) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	sl.ID = fmt.Sprintf("slc-%05d", m.seq)
	sl.Status = storage.SliceStatusOpen
	sl.CreatedAt = time.Now().UTC()
	sl.UpdatedAt = sl.CreatedAt
	m.slices[sl.Slug] = sl
	return nil
}

func (m *mockSliceBackend) ListSlices(_ context.Context, parentSlug string) ([]*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listSlicesErr != nil {
		return nil, m.listSlicesErr
	}
	var result []*storage.Slice
	for _, s := range m.slices {
		if s.ParentSlug == parentSlug {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSliceBackend) GetSlice(_ context.Context, slug string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getSliceErr != nil {
		return nil, m.getSliceErr
	}
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	return s, nil
}

func (m *mockSliceBackend) ClaimSlice(_ context.Context, slug, assignee string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimSliceErr != nil {
		return nil, m.claimSliceErr
	}
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	if s.Status != storage.SliceStatusOpen {
		return nil, storage.ErrSliceWrongStatus
	}
	s.Status = storage.SliceStatusClaimed
	s.AssignedTo = assignee
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func (m *mockSliceBackend) CompleteSlice(_ context.Context, slug string) (*storage.Slice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.completeSliceErr != nil {
		return nil, m.completeSliceErr
	}
	s, ok := m.slices[slug]
	if !ok {
		return nil, storage.ErrSliceNotFound
	}
	if s.Status != storage.SliceStatusClaimed {
		return nil, storage.ErrSliceWrongStatus
	}
	s.Status = storage.SliceStatusDone
	s.UpdatedAt = time.Now().UTC()
	return s, nil
}

func setupSliceServer(t *testing.T) (specgraphv1connect.SliceServiceClient, *mockSliceBackend) {
	t.Helper()
	mb := newMockSliceBackend()
	return setupSliceServerWithBackend(t, mb), mb
}

func setupSliceServerWithBackend(t *testing.T, mb *mockSliceBackend) specgraphv1connect.SliceServiceClient {
	t.Helper()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterSliceService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewSliceServiceClient(http.DefaultClient, srv.URL)
}

// seedSlice creates a slice in the mock backend for testing.
func seedSlice(t *testing.T, mb *mockSliceBackend, parentSlug, sliceID, intent string) string {
	t.Helper()
	slug := parentSlug + "/" + sliceID
	err := mb.CreateSlice(context.Background(), &storage.Slice{
		Slug:       slug,
		ParentSlug: parentSlug,
		SliceID:    sliceID,
		Intent:     intent,
	})
	require.NoError(t, err)
	return slug
}

func TestSliceHandler_ListSlices(t *testing.T) {
	client, mb := setupSliceServer(t)
	ctx := context.Background()

	seedSlice(t, mb, "parent", "a", "First")
	seedSlice(t, mb, "parent", "b", "Second")
	seedSlice(t, mb, "other", "c", "Other parent")

	resp, err := client.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: "parent",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Slices, 2)
}

func TestSliceHandler_ListSlices_MissingParentSlug(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_GetSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Test slice")

	resp, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: slug,
	}))
	require.NoError(t, err)
	require.Equal(t, "Test slice", resp.Msg.Slice.Intent)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_OPEN, resp.Msg.Slice.Status)
}

func TestSliceHandler_GetSlice_NotFound(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Claimable")

	resp, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     slug,
		Assignee: "alice",
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_CLAIMED, resp.Msg.Slice.Status)
	require.Equal(t, "alice", resp.Msg.Slice.AssignedTo)
}

func TestSliceHandler_ClaimSlice_WrongStatus(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Already claimed")
	_, _ = mb.ClaimSlice(context.Background(), slug, "bob")

	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     slug,
		Assignee: "alice",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice_MissingAssignee(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Missing assignee")

	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug: slug,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_CompleteSlice(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Completable")
	_, _ = mb.ClaimSlice(context.Background(), slug, "alice")

	resp, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: slug,
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.SliceStatus_SLICE_STATUS_DONE, resp.Msg.Slice.Status)
}

func TestSliceHandler_CompleteSlice_WrongStatus(t *testing.T) {
	client, mb := setupSliceServer(t)
	slug := seedSlice(t, mb, "parent", "a", "Not yet claimed")

	_, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: slug,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// --- Internal error tests (CodeInternal paths) ---

func TestSliceHandler_ListSlices_InternalError(t *testing.T) {
	mb := &mockSliceBackend{
		slices:        make(map[string]*storage.Slice),
		listSlicesErr: fmt.Errorf("database unavailable"),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: "parent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSliceHandler_GetSlice_InternalError(t *testing.T) {
	mb := &mockSliceBackend{
		slices:      make(map[string]*storage.Slice),
		getSliceErr: fmt.Errorf("database unavailable"),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: "any-slug",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice_InternalError(t *testing.T) {
	mb := &mockSliceBackend{
		slices:        make(map[string]*storage.Slice),
		claimSliceErr: fmt.Errorf("database unavailable"),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     "any-slug",
		Assignee: "alice",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSliceHandler_CompleteSlice_InternalError(t *testing.T) {
	mb := &mockSliceBackend{
		slices:           make(map[string]*storage.Slice),
		completeSliceErr: fmt.Errorf("database unavailable"),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: "any-slug",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- Missing slug validation tests ---

func TestSliceHandler_GetSlice_MissingSlug(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_ClaimSlice_MissingSlug(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Assignee: "alice",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSliceHandler_CompleteSlice_MissingSlug(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- Not-found tests for Claim/Complete ---

func TestSliceHandler_ClaimSlice_NotFound(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.ClaimSlice(context.Background(), connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     "nonexistent",
		Assignee: "alice",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSliceHandler_CompleteSlice_NotFound(t *testing.T) {
	client, _ := setupSliceServer(t)
	_, err := client.CompleteSlice(context.Background(), connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Proto conversion error tests (invalid status triggers sliceToProto failure) ---

func TestSliceHandler_GetSlice_ConversionError(t *testing.T) {
	mb := newMockSliceBackend()
	// Seed a slice with an invalid status that sliceStatusToProto can't map.
	mb.slices["parent/bad"] = &storage.Slice{
		ID: "slc-bad", Slug: "parent/bad", ParentSlug: "parent",
		SliceID: "bad", Intent: "bad status", Status: storage.SliceStatus("invalid"),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.GetSlice(context.Background(), connect.NewRequest(&specv1.GetSliceRequest{
		Slug: "parent/bad",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSliceHandler_ListSlices_ConversionError(t *testing.T) {
	mb := newMockSliceBackend()
	mb.slices["parent/bad"] = &storage.Slice{
		ID: "slc-bad", Slug: "parent/bad", ParentSlug: "parent",
		SliceID: "bad", Intent: "bad status", Status: storage.SliceStatus("invalid"),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	client := setupSliceServerWithBackend(t, mb)
	_, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: "parent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// --- Missing project header test (scopeStore error) ---

func TestSliceHandler_ListSlices_MissingProject(t *testing.T) {
	mb := newMockSliceBackend()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterSliceService(mux, scoper)
	// No wrapTestProject — project header is missing.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSliceServiceClient(http.DefaultClient, srv.URL)
	_, err := client.ListSlices(context.Background(), connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: "parent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
