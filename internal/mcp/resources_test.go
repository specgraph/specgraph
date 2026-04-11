// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// specResourceHandler tests
// ---------------------------------------------------------------------------

func TestSpecResource(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
			require.Equal(t, "oauth-refresh", slug)
			return &specv1.GetSpecResponse{
				Spec: &specv1.Spec{Slug: "oauth-refresh", Intent: "Rotate tokens safely"},
			}, nil
		},
	}}

	handler := specResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://spec/oauth-refresh")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://spec/oauth-refresh", contents[0].URI)
	require.Contains(t, contents[0].Text, "oauth-refresh")
}

func TestSpecResource_Error(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("not found"))
		},
	}}

	handler := specResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://spec/missing")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// specListResourceHandler tests
// ---------------------------------------------------------------------------

func TestSpecListResource(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return &specv1.ListSpecsResponse{
				Specs: []*specv1.Spec{
					{Slug: "spec-a", Stage: "spark"},
					{Slug: "spec-b", Stage: "shape"},
				},
			}, nil
		},
	}}

	handler := specListResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://specs")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://specs", contents[0].URI)
	require.Contains(t, contents[0].Text, "spec-a")
	require.Contains(t, contents[0].Text, "spec-b")
}

func TestSpecListResource_Error(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listSpecs: func() (*specv1.ListSpecsResponse, error) {
			return nil, fmt.Errorf("storage unavailable")
		},
	}}

	handler := specListResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://specs")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// decisionResourceHandler tests
// ---------------------------------------------------------------------------

func TestDecisionResource(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{
		getDecision: func(slug string) (*specv1.GetDecisionResponse, error) {
			require.Equal(t, "adr-001", slug)
			return &specv1.GetDecisionResponse{
				Decision: &specv1.Decision{Slug: "adr-001", Title: "Use PostgreSQL"},
			}, nil
		},
	}}

	handler := decisionResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://decision/adr-001")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://decision/adr-001", contents[0].URI)
	require.Contains(t, contents[0].Text, "adr-001")
}

func TestDecisionResource_Error(t *testing.T) {
	c := &Client{Decision: &mockDecisionService{
		getDecision: func(_ string) (*specv1.GetDecisionResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("not found"))
		},
	}}

	handler := decisionResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://decision/missing")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// constitutionResourceHandler tests
// ---------------------------------------------------------------------------

func TestConstitutionResource(t *testing.T) {
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

	handler := constitutionResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://constitution")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://constitution", contents[0].URI)
	require.Contains(t, contents[0].Text, "project-constitution")
}

func TestConstitutionResource_Error(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return nil, fmt.Errorf("storage error")
		},
	}}

	handler := constitutionResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://constitution")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// constitutionLayerResourceHandler tests
// ---------------------------------------------------------------------------

func TestConstitutionLayerResource(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{
				Constitution: &specv1.Constitution{
					Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
				},
			}, nil
		},
	}}

	handler := constitutionLayerResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://constitution/org")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://constitution/org", contents[0].URI)
	require.Contains(t, contents[0].Text, "CONSTITUTION_LAYER_ORG")
}

func TestConstitutionLayerResource_Error(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return nil, fmt.Errorf("storage error")
		},
	}}

	handler := constitutionLayerResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://constitution/project")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// graphResourceHandler tests
// ---------------------------------------------------------------------------

func TestGraphResource(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getFullGraph: func() (*specv1.GetFullGraphResponse, error) {
			return &specv1.GetFullGraphResponse{
				Nodes: []*specv1.GraphNode{
					{Slug: "spec-a"},
					{Slug: "spec-b"},
				},
			}, nil
		},
	}}

	handler := graphResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://graph")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://graph", contents[0].URI)
	require.Contains(t, contents[0].Text, "spec-a")
}

func TestGraphResource_Error(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getFullGraph: func() (*specv1.GetFullGraphResponse, error) {
			return nil, fmt.Errorf("storage error")
		},
	}}

	handler := graphResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://graph")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// readyResourceHandler tests
// ---------------------------------------------------------------------------

func TestReadyResource(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getReady: func() (*specv1.GetReadyResponse, error) {
			return &specv1.GetReadyResponse{
				Ready: []*specv1.NodeRef{
					{Slug: "spec-ready"},
				},
			}, nil
		},
	}}

	handler := readyResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://graph/ready")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://graph/ready", contents[0].URI)
	require.Contains(t, contents[0].Text, "spec-ready")
}

func TestReadyResource_Error(t *testing.T) {
	c := &Client{Graph: &mockGraphService{
		getReady: func() (*specv1.GetReadyResponse, error) {
			return nil, fmt.Errorf("storage error")
		},
	}}

	handler := readyResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://graph/ready")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// findingsResourceHandler tests
// ---------------------------------------------------------------------------

func TestFindingsResource(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
			return &specv1.ListFindingsResponse{
				Findings: []*specv1.AnalyticalFinding{
					{Id: "finding-1", Summary: "missing constraint"},
				},
			}, nil
		},
	}}

	handler := findingsResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://findings")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://findings", contents[0].URI)
	require.Contains(t, contents[0].Text, "finding-1")
}

func TestFindingsResource_Error(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
			return nil, fmt.Errorf("storage error")
		},
	}}

	handler := findingsResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://findings")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// changesResourceHandler tests
// ---------------------------------------------------------------------------

func TestChangesResource(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listChanges: func(slug string) (*specv1.ListChangesResponse, error) {
			require.Equal(t, "oauth-refresh", slug)
			return &specv1.ListChangesResponse{
				Entries: []*specv1.ChangeLogEntry{
					{Id: "v1", Summary: "initial version"},
				},
			}, nil
		},
	}}

	handler := changesResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://spec/oauth-refresh/changes")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)
	require.Equal(t, "specgraph://spec/oauth-refresh/changes", contents[0].URI)
	require.Contains(t, contents[0].Text, "initial version")
}

func TestChangesResource_Error(t *testing.T) {
	c := &Client{Spec: &mockSpecService{
		listChanges: func(_ string) (*specv1.ListChangesResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("not found"))
		},
	}}

	handler := changesResourceHandler(c)
	_, err := handler(context.Background(), "specgraph://spec/missing/changes")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// RegisterResources tests
// ---------------------------------------------------------------------------

func TestRegisterResources_Count(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterResources(r, c)
	require.Len(t, r.Resources(), 9)
}

func TestRegisterResources_Templates(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterResources(r, c)

	templateURIs := map[string]bool{}
	exactURIs := map[string]bool{}
	for _, res := range r.Resources() {
		if res.IsTemplate {
			templateURIs[res.URI] = true
		} else {
			exactURIs[res.URI] = true
		}
	}

	// Templates: spec/{slug}, decision/{slug}, constitution/{layer}, spec/{slug}/changes
	require.True(t, templateURIs["specgraph://spec/{slug}"], "spec template missing")
	require.True(t, templateURIs["specgraph://decision/{slug}"], "decision template missing")
	require.True(t, templateURIs["specgraph://constitution/{layer}"], "constitution layer template missing")
	require.True(t, templateURIs["specgraph://spec/{slug}/changes"], "changes template missing")

	// Exact URIs
	require.True(t, exactURIs["specgraph://specs"], "specs exact URI missing")
	require.True(t, exactURIs["specgraph://constitution"], "constitution exact URI missing")
	require.True(t, exactURIs["specgraph://graph"], "graph exact URI missing")
	require.True(t, exactURIs["specgraph://graph/ready"], "graph/ready exact URI missing")
	require.True(t, exactURIs["specgraph://findings"], "findings exact URI missing")
}

// ---------------------------------------------------------------------------
// extractSlugFromURI tests
// ---------------------------------------------------------------------------

func TestExtractSlugFromURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"specgraph://spec/oauth-refresh", "oauth-refresh"},
		{"specgraph://decision/adr-001", "adr-001"},
		{"specgraph://constitution/org", "org"},
	}
	for _, tt := range tests {
		got := extractSlugFromURI(tt.uri)
		require.Equal(t, tt.want, got, "URI: %s", tt.uri)
	}
}
