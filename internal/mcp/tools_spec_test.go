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

// ---------------------------------------------------------------------------
// spec tool — additional coverage
// ---------------------------------------------------------------------------

func TestSpecTool_Create_MissingSlug(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "create", "intent": "Something useful"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestSpecTool_Create_MissingIntent(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "create", "slug": "my-feature"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "intent is required")
}

func TestSpecTool_List_WithLimit(t *testing.T) {
	svc := &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return &specv1.ListSpecsResponse{
				Specs: []*specv1.Spec{
					{Slug: "alpha"},
					{Slug: "beta"},
				},
			}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "list", "limit": 10}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "alpha")
}

func TestSpecTool_Update(t *testing.T) {
	svc := &mockSpecService{
		updateSpec: func(req *specv1.UpdateSpecRequest) (*specv1.UpdateSpecResponse, error) {
			require.Equal(t, "auth", req.GetSlug())
			return &specv1.UpdateSpecResponse{Spec: &specv1.Spec{Slug: req.GetSlug()}}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{
		"action": "update",
		"slug":   "auth",
		"stage":  "shape",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "auth")
}

func TestSpecTool_Update_MissingSlug(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "update", "stage": "shape"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestSpecTool_Changes(t *testing.T) {
	svc := &mockSpecService{
		listChanges: func(slug string) (*specv1.ListChangesResponse, error) {
			require.Equal(t, "auth", slug)
			return &specv1.ListChangesResponse{}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "changes", "slug": "auth"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSpecTool_Changes_WithLimit(t *testing.T) {
	svc := &mockSpecService{
		listChanges: func(slug string) (*specv1.ListChangesResponse, error) {
			require.Equal(t, "auth", slug)
			return &specv1.ListChangesResponse{}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "changes", "slug": "auth", "limit": 5}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSpecTool_Changes_MissingSlug(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "changes"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestSpecTool_Compare(t *testing.T) {
	svc := &mockSpecService{
		compareVer: func(req *specv1.CompareVersionsRequest) (*specv1.CompareVersionsResponse, error) {
			require.Equal(t, "auth", req.GetSlug())
			require.Equal(t, int32(1), req.GetFromVersion())
			require.Equal(t, int32(3), req.GetToVersion())
			return &specv1.CompareVersionsResponse{}, nil
		},
	}
	c := &Client{Spec: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{
		"action":       "compare",
		"slug":         "auth",
		"from_version": 1,
		"to_version":   3,
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSpecTool_Compare_MissingSlug(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "compare", "from_version": 1, "to_version": 3}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestSpecTool_Compare_MissingVersions(t *testing.T) {
	c := &Client{Spec: &mockSpecService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("spec")
	require.True(t, ok)

	params := map[string]any{"action": "compare", "slug": "auth"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "from_version and to_version are required")
}

// ---------------------------------------------------------------------------
// decision tool — additional coverage
// ---------------------------------------------------------------------------

func TestDecisionTool_Create(t *testing.T) {
	svc := &mockDecisionService{
		createDecision: func(req *specv1.CreateDecisionRequest) (*specv1.CreateDecisionResponse, error) {
			require.Equal(t, "adr-003", req.GetSlug())
			require.Equal(t, "Use pgx v5", req.GetTitle())
			return &specv1.CreateDecisionResponse{
				Decision: &specv1.Decision{Slug: req.GetSlug(), Title: req.GetTitle()},
			}, nil
		},
	}
	c := &Client{Decision: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{
		"action":   "create",
		"slug":     "adr-003",
		"title":    "Use pgx v5",
		"decision": "We will use pgx v5 as the Postgres driver.",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "adr-003")
}

func TestDecisionTool_Create_MissingSlug(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{
		"action":   "create",
		"title":    "Use pgx v5",
		"decision": "We will use pgx v5.",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestDecisionTool_Create_MissingTitle(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{
		"action":   "create",
		"slug":     "adr-003",
		"decision": "We will use pgx v5.",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "title is required")
}

func TestDecisionTool_Update(t *testing.T) {
	svc := &mockDecisionService{
		updateDecision: func(req *specv1.UpdateDecisionRequest) (*specv1.UpdateDecisionResponse, error) {
			require.Equal(t, "adr-001", req.GetSlug())
			return &specv1.UpdateDecisionResponse{
				Decision: &specv1.Decision{Slug: req.GetSlug()},
			}, nil
		},
	}
	c := &Client{Decision: svc}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{
		"action":    "update",
		"slug":      "adr-001",
		"rationale": "Better performance",
	}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "adr-001")
}

func TestDecisionTool_Update_MissingSlug(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{}}
	r := NewRegistry()
	RegisterSpecTools(r, c)

	tool, ok := r.LookupTool("decision")
	require.True(t, ok)

	params := map[string]any{"action": "update", "rationale": "Better performance"}
	result, err := tool.Handler(context.Background(), params)
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}
