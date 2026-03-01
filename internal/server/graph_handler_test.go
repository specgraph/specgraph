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
)

type mockGraphBackend struct {
	mu    sync.Mutex
	edges []mockEdge
	nodes map[string]mockNode
}

type mockEdge struct {
	from, to string
	edgeType specv1.EdgeType
}

type mockNode struct {
	id, slug, label, stage string
}

func newMockGraphBackend() *mockGraphBackend {
	return &mockGraphBackend{
		nodes: map[string]mockNode{
			"spec-a": {id: "spec-00001", slug: "spec-a", label: "Spec", stage: "spark"},
			"spec-b": {id: "spec-00002", slug: "spec-b", label: "Spec", stage: "done"},
			"spec-c": {id: "spec-00003", slug: "spec-c", label: "Spec", stage: "spark"},
		},
	}
}

func (m *mockGraphBackend) AddEdge(_ context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) (*specv1.Edge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	from, ok1 := m.nodes[fromSlug]
	to, ok2 := m.nodes[toSlug]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("node not found")
	}
	m.edges = append(m.edges, mockEdge{from: fromSlug, to: toSlug, edgeType: edgeType})
	return &specv1.Edge{FromId: from.id, ToId: to.id, EdgeType: edgeType}, nil
}

func (m *mockGraphBackend) RemoveEdge(_ context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.edges {
		if e.from == fromSlug && e.to == toSlug && e.edgeType == edgeType {
			m.edges = append(m.edges[:i], m.edges[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockGraphBackend) ListEdges(_ context.Context, slug string, edgeType specv1.EdgeType) ([]*specv1.Edge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*specv1.Edge
	for _, e := range m.edges {
		if e.from != slug && e.to != slug {
			continue
		}
		if edgeType != specv1.EdgeType_EDGE_TYPE_UNSPECIFIED && e.edgeType != edgeType {
			continue
		}
		from := m.nodes[e.from]
		to := m.nodes[e.to]
		result = append(result, &specv1.Edge{FromId: from.id, ToId: to.id, EdgeType: e.edgeType})
	}
	return result, nil
}

func (m *mockGraphBackend) GetDependencies(_ context.Context, slug string) ([]storage.NodeRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var refs []storage.NodeRef
	for _, e := range m.edges {
		if e.from == slug && e.edgeType == specv1.EdgeType_EDGE_TYPE_DEPENDS_ON {
			n := m.nodes[e.to]
			refs = append(refs, storage.NodeRef{ID: n.id, Slug: n.slug, Label: n.label, Stage: n.stage})
		}
	}
	return refs, nil
}

func (m *mockGraphBackend) GetTransitiveDeps(_ context.Context, slug string) ([]storage.NodeRef, error) {
	return m.GetDependencies(context.Background(), slug) // simplified for mock
}

func (m *mockGraphBackend) GetImpact(_ context.Context, slug string) ([]storage.NodeRef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var refs []storage.NodeRef
	for _, e := range m.edges {
		if e.to == slug && e.edgeType == specv1.EdgeType_EDGE_TYPE_DEPENDS_ON {
			n := m.nodes[e.from]
			refs = append(refs, storage.NodeRef{ID: n.id, Slug: n.slug, Label: n.label, Stage: n.stage})
		}
	}
	return refs, nil
}

func (m *mockGraphBackend) GetReady(_ context.Context) ([]storage.NodeRef, error) {
	return []storage.NodeRef{{ID: "spec-00003", Slug: "spec-c", Label: "Spec", Stage: "spark"}}, nil
}

func (m *mockGraphBackend) GetCriticalPath(_ context.Context, slug string) ([]storage.NodeRef, error) {
	return []storage.NodeRef{
		{ID: "spec-00001", Slug: slug, Label: "Spec", Stage: "spark"},
	}, nil
}

func setupGraphServer(t *testing.T) specgraphv1connect.GraphServiceClient {
	t.Helper()
	mb := newMockGraphBackend()
	mux := http.NewServeMux()
	server.RegisterGraphService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewGraphServiceClient(http.DefaultClient, srv.URL)
}

func TestGraphHandler_AddAndListEdges(t *testing.T) {
	client := setupGraphServer(t)
	ctx := context.Background()

	// Add edge
	addResp, err := client.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: "spec-a",
		ToSlug:   "spec-b",
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, addResp.Msg.EdgeType)

	// List edges
	listResp, err := client.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
		Slug: "spec-a",
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Edges, 1)

	// Remove edge
	_, err = client.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
		FromSlug: "spec-a",
		ToSlug:   "spec-b",
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	require.NoError(t, err)
}

func TestGraphHandler_Dependencies(t *testing.T) {
	client := setupGraphServer(t)
	ctx := context.Background()

	// Add dependency
	_, err := client.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: "spec-a",
		ToSlug:   "spec-b",
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	require.NoError(t, err)

	depsResp, err := client.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{
		Slug: "spec-a",
	}))
	require.NoError(t, err)
	require.Len(t, depsResp.Msg.Dependencies, 1)
	require.Equal(t, "spec-b", depsResp.Msg.Dependencies[0].Slug)
}

func TestGraphHandler_ReadyAndImpact(t *testing.T) {
	client := setupGraphServer(t)
	ctx := context.Background()

	readyResp, err := client.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
	require.NoError(t, err)
	require.NotEmpty(t, readyResp.Msg.Ready)

	// Impact
	_, err = client.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: "spec-a",
		ToSlug:   "spec-b",
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	require.NoError(t, err)

	impactResp, err := client.GetImpact(ctx, connect.NewRequest(&specv1.GetImpactRequest{
		Slug: "spec-b",
	}))
	require.NoError(t, err)
	require.Len(t, impactResp.Msg.Impacted, 1)
}
