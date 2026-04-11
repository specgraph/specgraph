// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// edgeTool tests
// ---------------------------------------------------------------------------

func TestEdgeTool_List(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		listEdges: func(slug string) (*specv1.ListEdgesResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.ListEdgesResponse{
				Edges: []*specv1.Edge{
					{FromId: "spec-a", ToId: "spec-b", EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "list",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec-a")
}

func TestEdgeTool_List_MissingSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestEdgeTool_Add(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		addEdge: func(req *specv1.AddEdgeRequest) (*specv1.AddEdgeResponse, error) {
			require.Equal(t, "spec-a", req.GetFromSlug())
			require.Equal(t, "spec-b", req.GetToSlug())
			require.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, req.GetEdgeType())
			return &specv1.AddEdgeResponse{
				Edge: &specv1.Edge{
					FromId:   "spec-a",
					ToId:     "spec-b",
					EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "add",
		"from_slug": "spec-a",
		"to_slug":   "spec-b",
		"edge_type": "depends_on",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestEdgeTool_Add_MissingParams(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "add",
		// missing from_slug and to_slug
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestEdgeTool_Remove(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		removeEdge: func(req *specv1.RemoveEdgeRequest) (*specv1.RemoveEdgeResponse, error) {
			require.Equal(t, "spec-a", req.GetFromSlug())
			require.Equal(t, "spec-b", req.GetToSlug())
			return &specv1.RemoveEdgeResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "remove",
		"from_slug": "spec-a",
		"to_slug":   "spec-b",
		"edge_type": "depends_on",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestEdgeTool_UnknownAction(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "frobnicate"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "frobnicate")
}

// ---------------------------------------------------------------------------
// graphQueryTool tests
// ---------------------------------------------------------------------------

func TestGraphQueryTool_Dependencies(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getDeps: func(slug string) (*specv1.GetDependenciesResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.GetDependenciesResponse{
				Dependencies: []*specv1.NodeRef{
					{Slug: "spec-b"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "dependencies",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec-b")
}

func TestGraphQueryTool_TransitiveDeps(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getTransDeps: func(slug string) (*specv1.GetTransitiveDepsResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.GetTransitiveDepsResponse{
				Dependencies: []*specv1.NodeRef{
					{Slug: "spec-b"},
					{Slug: "spec-c"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "transitive_deps",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGraphQueryTool_Impact(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getImpact: func(slug string) (*specv1.GetImpactResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.GetImpactResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "impact",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGraphQueryTool_Ready(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getReady: func() (*specv1.GetReadyResponse, error) {
			return &specv1.GetReadyResponse{
				Ready: []*specv1.NodeRef{
					{Slug: "spec-x"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "ready"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec-x")
}

func TestGraphQueryTool_CriticalPath(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getCriticalPath: func(slug string) (*specv1.GetCriticalPathResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.GetCriticalPathResponse{
				Path: []*specv1.NodeRef{
					{Slug: "spec-a"},
					{Slug: "spec-b"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "critical_path",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestGraphQueryTool_FullGraph(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getFullGraph: func() (*specv1.GetFullGraphResponse, error) {
			return &specv1.GetFullGraphResponse{
				Nodes: []*specv1.GraphNode{
					{Slug: "spec-a"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "full"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec-a")
}

func TestGraphQueryTool_UnknownAction(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "invalid"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid")
}

func TestGraphQueryTool_MissingSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	// actions requiring slug fail gracefully when slug is absent
	result, err := tool.Handler(context.Background(), map[string]any{"action": "dependencies"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestGraphQueryTool_TransitiveDeps_MissingSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "transitive_deps"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestGraphQueryTool_Impact_MissingSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "impact"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestGraphQueryTool_CriticalPath_MissingSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("graph_query")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "critical_path"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

// ---------------------------------------------------------------------------
// edgeTool — add/remove missing-param and invalid edge_type tests
// ---------------------------------------------------------------------------

func TestEdgeTool_Add_MissingFromSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":  "add",
		"to_slug": "spec-b",
		// from_slug intentionally absent
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "from_slug")
}

func TestEdgeTool_Add_MissingToSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "add",
		"from_slug": "spec-a",
		// to_slug intentionally absent
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestEdgeTool_Add_InvalidEdgeType(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "add",
		"from_slug": "spec-a",
		"to_slug":   "spec-b",
		"edge_type": "not_a_real_type",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "edge_type")
}

func TestEdgeTool_Remove_MissingFromSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":  "remove",
		"to_slug": "spec-b",
		// from_slug intentionally absent
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestEdgeTool_Remove_MissingToSlug(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "remove",
		"from_slug": "spec-a",
		// to_slug intentionally absent
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestEdgeTool_Remove_InvalidEdgeType(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "remove",
		"from_slug": "spec-a",
		"to_slug":   "spec-b",
		"edge_type": "not_a_real_type",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "edge_type")
}

func TestEdgeTool_List_InvalidEdgeType(t *testing.T) {
	c := &Client{Graph: &mockGraphService{}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "list",
		"slug":      "spec-a",
		"edge_type": "not_a_real_type",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "edge_type")
}

func TestEdgeTool_List_WithValidEdgeType(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		listEdges: func(slug string) (*specv1.ListEdgesResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.ListEdgesResponse{
				Edges: []*specv1.Edge{
					{FromId: "spec-a", ToId: "spec-b", EdgeType: specv1.EdgeType_EDGE_TYPE_BLOCKS},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterGraphTools(r, c)
	tool, ok := r.LookupTool("edge")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "list",
		"slug":      "spec-a",
		"edge_type": "blocks",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// edgeTypeFromString helper tests
// ---------------------------------------------------------------------------

func TestEdgeTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected specv1.EdgeType
	}{
		{"depends_on", specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
		{"blocks", specv1.EdgeType_EDGE_TYPE_BLOCKS},
		{"composes", specv1.EdgeType_EDGE_TYPE_COMPOSES},
		{"relates_to", specv1.EdgeType_EDGE_TYPE_RELATES_TO},
		{"informs", specv1.EdgeType_EDGE_TYPE_INFORMS},
		{"decided_in", specv1.EdgeType_EDGE_TYPE_DECIDED_IN},
		{"supersedes", specv1.EdgeType_EDGE_TYPE_SUPERSEDES},
		{"unknown_value", specv1.EdgeType_EDGE_TYPE_UNSPECIFIED},
		{"", specv1.EdgeType_EDGE_TYPE_UNSPECIFIED},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.expected, edgeTypeFromString(tc.input))
		})
	}
}
