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

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockBackend implements storage.Backend with an in-memory map.
type mockBackend struct {
	mu    sync.Mutex
	specs map[string]*specv1.Spec
	seq   int
}

func newMockBackend() *mockBackend {
	return &mockBackend{specs: make(map[string]*specv1.Spec)}
}

func (m *mockBackend) CreateSpec(_ context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	now := timestamppb.Now()
	spec := &specv1.Spec{
		Id:         fmt.Sprintf("spec-%05d", m.seq),
		Slug:       slug,
		Intent:     intent,
		Stage:      "spark",
		Priority:   priority,
		Complexity: complexity,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	m.specs[slug] = spec
	return spec, nil
}

func (m *mockBackend) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return spec, nil
}

func (m *mockBackend) ListSpecs(_ context.Context, stage, priority string, limit int) ([]*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*specv1.Spec
	for _, s := range m.specs {
		if stage != "" && s.Stage != stage {
			continue
		}
		if priority != "" && s.Priority != priority {
			continue
		}
		result = append(result, s)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockBackend) UpdateSpec(_ context.Context, slug string, intent, stage, priority, complexity *string) (*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	if intent != nil {
		spec.Intent = *intent
	}
	if stage != nil {
		spec.Stage = *stage
	}
	if priority != nil {
		spec.Priority = *priority
	}
	if complexity != nil {
		spec.Complexity = *complexity
	}
	spec.Version++
	spec.UpdatedAt = timestamppb.Now()
	return spec, nil
}

func (m *mockBackend) Close(_ context.Context) error {
	return nil
}

func TestSpecHandler_CreateAndGet(t *testing.T) {
	mb := newMockBackend()
	srv := httptest.NewServer(server.NewMux(mb))
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	// Create a spec.
	createResp, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   "oauth-refresh",
		Intent: "Implement OAuth refresh token rotation",
	}))
	require.NoError(t, err)
	require.Equal(t, "oauth-refresh", createResp.Msg.Slug)
	require.Equal(t, "Implement OAuth refresh token rotation", createResp.Msg.Intent)
	require.Equal(t, "p2", createResp.Msg.Priority)       // defaulted
	require.Equal(t, "medium", createResp.Msg.Complexity)  // defaulted
	require.Equal(t, "spark", createResp.Msg.Stage)
	require.NotEmpty(t, createResp.Msg.Id)

	// Get it back.
	getResp, err := client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
		Slug: "oauth-refresh",
	}))
	require.NoError(t, err)
	require.Equal(t, createResp.Msg.Id, getResp.Msg.Id)
	require.Equal(t, "oauth-refresh", getResp.Msg.Slug)

	// Get non-existent returns not found.
	_, err = client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
		Slug: "does-not-exist",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSpecHandler_UpdateSpec(t *testing.T) {
	mb := newMockBackend()
	srv := httptest.NewServer(server.NewMux(mb))
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	// Create a spec first.
	_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   "update-test",
		Intent: "Original intent",
	}))
	require.NoError(t, err)

	// Update intent only.
	newIntent := "Updated intent"
	updateResp, err := client.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
		Slug:   "update-test",
		Intent: &newIntent,
	}))
	require.NoError(t, err)
	require.Equal(t, "Updated intent", updateResp.Msg.Intent)
	require.Equal(t, int32(2), updateResp.Msg.Version)
	require.Equal(t, "p2", updateResp.Msg.Priority) // unchanged

	// Update non-existent spec.
	_, err = client.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
		Slug:   "no-such-spec",
		Intent: &newIntent,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSpecHandler_ListSpecs(t *testing.T) {
	mb := newMockBackend()
	srv := httptest.NewServer(server.NewMux(mb))
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	// Create two specs.
	_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   "spec-alpha",
		Intent: "Alpha feature",
	}))
	require.NoError(t, err)

	_, err = client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   "spec-beta",
		Intent: "Beta feature",
	}))
	require.NoError(t, err)

	// List all.
	listResp, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Specs, 2)
}
