// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// spec tool tests
// ---------------------------------------------------------------------------

func TestSpecTool_Get(t *testing.T) {
	svc := &mockSpecService{
		getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
			require.Equal(t, "oauth-refresh", slug)
			return &specv1.GetSpecResponse{
				Spec: &specv1.Spec{Slug: "oauth-refresh", Intent: "Rotate tokens safely"},
			}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "get", "slug": "oauth-refresh"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "oauth-refresh")
}

func TestSpecTool_GetNotFound(t *testing.T) {
	svc := &mockSpecService{
		getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, nil)
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "get", "slug": "no-such-spec"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestSpecTool_List(t *testing.T) {
	svc := &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return &specv1.ListSpecsResponse{
				Specs: []*specv1.Spec{
					{Slug: "spec-a", Stage: "spark"},
					{Slug: "spec-b", Stage: "shape"},
				},
			}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "list"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec-a")
	require.Contains(t, result.Content[0].Text, "spec-b")
}

func TestSpecTool_Create(t *testing.T) {
	svc := &mockSpecService{
		createSpec: func(slug, intent string) (*specv1.CreateSpecResponse, error) {
			require.Equal(t, "new-feature", slug)
			require.Equal(t, "Build new feature", intent)
			return &specv1.CreateSpecResponse{
				Spec: &specv1.Spec{Slug: "new-feature", Intent: "Build new feature", Stage: "spark"},
			}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{
		"action": "create",
		"slug":   "new-feature",
		"intent": "Build new feature",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "new-feature")
}

func TestSpecTool_UnknownAction(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "frobnicate"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "unknown action")
}

// ---------------------------------------------------------------------------
// decision tool tests
// ---------------------------------------------------------------------------

func TestDecisionTool_Get(t *testing.T) {
	svc := &mockDecisionService{
		getDecision: func(slug string) (*specv1.GetDecisionResponse, error) {
			require.Equal(t, "adr-001", slug)
			return &specv1.GetDecisionResponse{
				Decision: &specv1.Decision{Slug: "adr-001", Title: "Use PostgreSQL"},
			}, nil
		},
	}
	c := &Client{Decision: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{"action": "get", "slug": "adr-001"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "adr-001")
}

func TestDecisionTool_List(t *testing.T) {
	svc := &mockDecisionService{
		listDecisions: func() (*specv1.ListDecisionsResponse, error) {
			return &specv1.ListDecisionsResponse{
				Decisions: []*specv1.Decision{
					{Slug: "adr-001", Title: "Use PostgreSQL"},
					{Slug: "adr-002", Title: "Use ConnectRPC"},
				},
			}, nil
		},
	}
	c := &Client{Decision: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{"action": "list"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "adr-001")
	require.Contains(t, result.Content[0].Text, "adr-002")
}
