// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// constitutionTool tests
// ---------------------------------------------------------------------------

func TestConstitutionTool_Get(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{
				Constitution: &specv1.Constitution{
					Name:  "project-constitution",
					Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "get"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "project-constitution")
}

func TestConstitutionTool_Get_WithLayer(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{
				Constitution: &specv1.Constitution{
					Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "get",
		"layer":  "org",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestConstitutionTool_Update_RoundTrip(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		updateConstitution: func(req *specv1.UpdateConstitutionRequest) (*specv1.UpdateConstitutionResponse, error) {
			require.NotNil(t, req.Constitution)
			require.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, req.Constitution.Layer)
			require.Equal(t, "my-project", req.Constitution.Name)
			require.Len(t, req.Constitution.Constraints, 1)
			require.Equal(t, "no vendor lock-in", req.Constitution.Constraints[0])
			return &specv1.UpdateConstitutionResponse{
				Constitution: req.Constitution,
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	// Simulate round-trip: pass friendly YAML (as the skills teach)
	constitutionYAML := "layer: project\nname: my-project\nconstraints:\n  - \"no vendor lock-in\"\n"
	result, err := tool.Handler(context.Background(), map[string]any{
		"action":       "update",
		"constitution": constitutionYAML,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-project")
}

func TestConstitutionTool_Update_InvalidInput(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":       "update",
		"constitution": "layer: bogus-layer\nname: x\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid constitution input")
	// Sanitized: no raw parser internals leaked (T-06-03).
	require.NotContains(t, result.Content[0].Text, "unknown constitution layer")
}

func TestConstitutionTool_Update_EmptyLayer(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	// Valid YAML but no `layer:` — explicit-layer guard must reject it.
	result, err := tool.Handler(context.Background(), map[string]any{
		"action":       "update",
		"constitution": "name: my-project\nconstraints:\n  - \"no vendor lock-in\"\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "layer is required")
}

func TestConstitutionTool_UnknownAction(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("constitution")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "delete"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// findingsTool tests
// ---------------------------------------------------------------------------

func TestFindingsTool_List(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		listFindings: func(slug string) (*specv1.ListFindingsResponse, error) {
			require.Equal(t, "spec-a", slug)
			return &specv1.ListFindingsResponse{
				Findings: []*specv1.AnalyticalFinding{
					{
						Id:      "finding-1",
						Summary: "missing constraint",
					},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("findings")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "list",
		"slug":   "spec-a",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "finding-1")
}

func TestFindingsTool_List_WithPassType(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
			return &specv1.ListFindingsResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("findings")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "list",
		"slug":      "spec-a",
		"pass_type": "constitution-check",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestFindingsTool_List_MissingSlug(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("findings")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestFindingsTool_UnknownAction(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("findings")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "delete"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// healthTool tests
// ---------------------------------------------------------------------------

func TestHealthTool(t *testing.T) {
	c := &Client{Health: &mockHealthService{
		health: func() (*specv1.HealthResponse, error) {
			return &specv1.HealthResponse{
				Status:  "ok",
				Version: "v1.2.3",
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("health")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "ok")
}

func TestHealthTool_Error(t *testing.T) {
	c := &Client{Health: &mockHealthService{
		health: func() (*specv1.HealthResponse, error) {
			return nil, fmt.Errorf("server unavailable")
		},
	}}
	r := NewRegistry()
	RegisterCoreTools(r, c)
	tool, ok := r.LookupTool("health")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	// Non-connect errors (no code) become tool error results
	require.NoError(t, err)
	require.True(t, result.IsError)
}
