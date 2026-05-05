// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"
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

func TestConstitutionResource_NotFoundRendersHint(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("constitution not found"))
		},
	}}

	handler := constitutionResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://constitution")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Equal(t, "specgraph://constitution", contents[0].URI)
	require.Contains(t, contents[0].Text, "No constitution configured")
	require.Contains(t, contents[0].Text, "specgraph constitution set")
}

func TestConstitutionResource_SlugRequiredRendersHint(t *testing.T) {
	c := &Client{Constitution: &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
		},
	}}

	handler := constitutionResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://constitution")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Equal(t, "specgraph://constitution", contents[0].URI)
	require.Contains(t, contents[0].Text, "No constitution configured")
	require.Contains(t, contents[0].Text, "specgraph constitution set")
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
		listProjectFindings: func(req *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
			require.Equal(t, specv1.PassType_PASS_TYPE_UNSPECIFIED, req.GetPassType())
			return &specv1.ListProjectFindingsResponse{
				Findings: []*specv1.AnalyticalFinding{
					{Id: "finding-1", SpecSlug: "spec-a", Summary: "missing constraint"},
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
		listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
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
	require.Len(t, r.Resources(), 10)
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
	require.True(t, exactURIs["specgraph://prime"], "prime exact URI missing")
}

// ---------------------------------------------------------------------------
// primeResourceHandler tests
// ---------------------------------------------------------------------------

func TestPrimeResource(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return &specv1.GetConstitutionResponse{
					Constitution: &specv1.Constitution{
						Constraints: []string{"no GPL", "no circular deps"},
					},
				}, nil
			},
		},
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return &specv1.ListSpecsResponse{
					Specs: []*specv1.Spec{
						{Slug: "spec-a", Stage: "spark"},
						{Slug: "spec-b", Stage: "shape"},
					},
				}, nil
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return &specv1.GetReadyResponse{
					Ready: []*specv1.NodeRef{
						{Slug: "spec-a", Stage: "spark"},
					},
				}, nil
			},
		},
		AnalyticalPass: &mockAnalyticalPassService{
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return &specv1.ListProjectFindingsResponse{}, nil
			},
		},
	}

	handler := primeResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Equal(t, "specgraph://prime", contents[0].URI)

	text := contents[0].Text
	for _, marker := range []string{"Constitution", "Graph", "Ready"} {
		require.Contains(t, text, marker)
	}
	// Should contain constraint text
	require.Contains(t, text, "no GPL")
	// Should reference the full resources
	require.Contains(t, text, "specgraph://constitution")
	require.Contains(t, text, "specgraph://graph/ready")
	// B.3: Assert aggregated stage counts from the two specs in the fake.
	// spec-a is stage "spark", spec-b is stage "shape" → each count 1.
	require.Contains(t, text, "spark: 1")
	require.Contains(t, text, "shape: 1")
}

func TestPrimeResource_FindingsSection(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		AnalyticalPass: &mockAnalyticalPassService{
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return &specv1.ListProjectFindingsResponse{
					Findings: []*specv1.AnalyticalFinding{
						{Id: "f1", SpecSlug: "spec-a", Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL},
						{Id: "f2", SpecSlug: "spec-b", Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL},
						{Id: "f3", SpecSlug: "spec-c", Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING},
					},
				}, nil
			},
		},
	}

	handler := primeResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.Len(t, contents, 1)

	text := contents[0].Text
	require.Contains(t, text, "Open Findings")
	require.Contains(t, text, "specgraph://findings")
	// B.3: Assert per-severity counts rendered by the handler (f.GetSeverity().String()).
	require.Contains(t, text, "FINDING_SEVERITY_CRITICAL: 2")
	require.Contains(t, text, "FINDING_SEVERITY_WARNING: 1")
	// Failed sections render visible error markers (behavior change from PR #924 fixup):
	// each section heading IS present, followed by an unable-to-load line.
	require.Contains(t, text, "## Constitution")
	require.Contains(t, text, "## Graph Overview")
	require.Contains(t, text, "## Ready to Work")
	require.Contains(t, text, "(unable to load")
}

// TestPrimeResource_SeverityOrdering verifies findings are rendered
// in CRITICAL → WARNING → NOTE order (not alphabetical).
func TestPrimeResource_SeverityOrdering(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		AnalyticalPass: &mockAnalyticalPassService{
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return &specv1.ListProjectFindingsResponse{
					Findings: []*specv1.AnalyticalFinding{
						{Id: "f1", SpecSlug: "spec-a", Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE},
						{Id: "f2", SpecSlug: "spec-b", Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL},
						{Id: "f3", SpecSlug: "spec-c", Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING},
					},
				}, nil
			},
		},
	}

	handler := primeResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.Len(t, contents, 1)

	text := contents[0].Text
	// Severity ordering: critical findings should render before warnings, warnings before notes.
	criticalIdx := strings.Index(text, "FINDING_SEVERITY_CRITICAL")
	warningIdx := strings.Index(text, "FINDING_SEVERITY_WARNING")
	noteIdx := strings.Index(text, "FINDING_SEVERITY_NOTE")
	require.Greater(t, warningIdx, criticalIdx, "WARNING should appear after CRITICAL")
	require.Greater(t, noteIdx, warningIdx, "NOTE should appear after WARNING")
}

// TestPrimeResource_StageOrdering verifies that funnel stages are rendered
// in spark → shape → specify → decompose → approved order, not alphabetically.
func TestPrimeResource_StageOrdering(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return &specv1.ListSpecsResponse{
					Specs: []*specv1.Spec{
						{Slug: "spec-a", Stage: "spark"},
						{Slug: "spec-b", Stage: "decompose"},
						{Slug: "spec-c", Stage: "shape"},
						{Slug: "spec-d", Stage: "specify"},
					},
				}, nil
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return nil, fmt.Errorf("unavailable")
			},
		},
		AnalyticalPass: defaultAnalyticalPassMock(),
	}

	handler := primeResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.Len(t, contents, 1)

	text := contents[0].Text
	sparkIdx := strings.Index(text, "spark:")
	shapeIdx := strings.Index(text, "shape:")
	specifyIdx := strings.Index(text, "specify:")
	decomposeIdx := strings.Index(text, "decompose:")
	require.Greater(t, shapeIdx, sparkIdx, "shape should appear after spark")
	require.Greater(t, specifyIdx, shapeIdx, "specify should appear after shape")
	require.Greater(t, decomposeIdx, specifyIdx, "decompose should appear after specify")
}

// TestPrimeResource_RPCFailureRendersErrorMarker verifies that when an RPC fails
// the section heading is still rendered with a visible error marker rather than
// silently omitting the section.
func TestPrimeResource_RPCFailureRendersErrorMarker(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, fmt.Errorf("backend connection refused")
			},
		},
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return nil, fmt.Errorf("backend connection refused")
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return nil, fmt.Errorf("backend connection refused")
			},
		},
		AnalyticalPass: &mockAnalyticalPassService{
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return nil, fmt.Errorf("backend connection refused")
			},
		},
	}

	handler := primeResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://prime")
	// The handler itself must not error — it returns the partial digest.
	require.NoError(t, err)
	require.Len(t, contents, 1)

	text := contents[0].Text
	// All four section headings must appear.
	require.Contains(t, text, "## Constitution")
	require.Contains(t, text, "## Graph Overview")
	require.Contains(t, text, "## Ready to Work")
	require.Contains(t, text, "## Open Findings")
	// Failures are visible to the user.
	require.Contains(t, text, "_(unable to load:")
}

// ---------------------------------------------------------------------------
// C.1 — primeResourceHandler empty Graph Overview guard
// ---------------------------------------------------------------------------

// defaultAnalyticalPassMock returns minimal findings mocks that return no findings.
func defaultAnalyticalPassMock() *mockAnalyticalPassService {
	return &mockAnalyticalPassService{
		listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
			return &specv1.ListFindingsResponse{}, nil
		},
		listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
			return &specv1.ListProjectFindingsResponse{}, nil
		},
	}
}

// TestPrimeResource_EmptyGraphSkipped verifies that when ListSpecs succeeds but
// returns an empty list, the "## Graph Overview" heading is NOT emitted.
// The Ready and Findings sections were already guarded; Graph Overview must match.
func TestPrimeResource_EmptyGraphSkipped(t *testing.T) {
	c := &Client{
		Constitution: defaultConstitutionMock(), // succeeds — header rendered
		Spec: &mockSpecService{
			listSpecs: func() (*specv1.ListSpecsResponse, error) {
				return &specv1.ListSpecsResponse{}, nil // empty list, no error
			},
		},
		Graph: &mockGraphService{
			getReady: func() (*specv1.GetReadyResponse, error) {
				return &specv1.GetReadyResponse{}, nil // also empty
			},
		},
		AnalyticalPass: defaultAnalyticalPassMock(), // empty findings
	}
	content, err := primeResourceHandler(c)(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.NotEmpty(t, content)
	text := content[0].Text
	require.NotContains(t, text, "## Graph Overview",
		"Graph Overview heading should not render for empty spec list")
}

// TestPrimeResource_ConstitutionNotFound_RendersHint verifies that when
// GetConstitution fails with connect.CodeNotFound (the expected fresh-project
// state), the prime body renders a heading + actionable hint instead of the
// loud "_(unable to load: ...)_" marker reserved for genuine RPC failures.
func TestPrimeResource_ConstitutionNotFound_RendersHint(t *testing.T) {
	c := &Client{
		Constitution: &mockConstitutionService{
			getConstitution: func() (*specv1.GetConstitutionResponse, error) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("constitution not found"))
			},
		},
		Spec:           &mockSpecService{listSpecs: func() (*specv1.ListSpecsResponse, error) { return &specv1.ListSpecsResponse{}, nil }},
		Graph:          &mockGraphService{getReady: func() (*specv1.GetReadyResponse, error) { return &specv1.GetReadyResponse{}, nil }},
		AnalyticalPass: defaultAnalyticalPassMock(),
	}

	content, err := primeResourceHandler(c)(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.NotEmpty(t, content)
	text := content[0].Text

	require.Contains(t, text, "## Constitution",
		"Constitution heading should render so the agent knows the slot exists")
	require.Contains(t, text, "specgraph constitution set",
		"NotFound state should hint at the command that populates the constitution")
	require.NotContains(t, text, "unable to load",
		"NotFound is an expected empty state, not an RPC failure")
}

func TestPrimeResource_ProjectFindingsDoesNotCallPerSpecListWithoutSlug(t *testing.T) {
	c := &Client{
		Constitution: defaultConstitutionMock(),
		Spec:         &mockSpecService{listSpecs: func() (*specv1.ListSpecsResponse, error) { return &specv1.ListSpecsResponse{}, nil }},
		Graph:        &mockGraphService{getReady: func() (*specv1.GetReadyResponse, error) { return &specv1.GetReadyResponse{}, nil }},
		AnalyticalPass: &mockAnalyticalPassService{
			listFindings: func(_ string) (*specv1.ListFindingsResponse, error) {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
			},
			listProjectFindings: func(_ *specv1.ListProjectFindingsRequest) (*specv1.ListProjectFindingsResponse, error) {
				return &specv1.ListProjectFindingsResponse{
					Findings: []*specv1.AnalyticalFinding{
						{Id: "f1", SpecSlug: "spec-a", Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING},
					},
				}, nil
			},
		},
	}

	content, err := primeResourceHandler(c)(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.NotEmpty(t, content)
	text := content[0].Text

	require.Contains(t, text, "## Open Findings")
	require.Contains(t, text, "FINDING_SEVERITY_WARNING: 1")
	require.NotContains(t, text, "slug is required",
		"prime should use project-wide findings instead of per-spec ListFindings without a slug")
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
