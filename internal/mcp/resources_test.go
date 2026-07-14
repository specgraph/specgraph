// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
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
					Name:        "project-constitution",
					Layer:       specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
					Constraints: []string{"no circular deps"},
				},
			}, nil
		},
	}}

	handler := constitutionResourceHandler(c)
	contents, err := handler(context.Background(), "specgraph://constitution")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Equal(t, "specgraph://constitution", contents[0].URI)
	require.Contains(t, contents[0].Text, "project-constitution")
	require.Contains(t, contents[0].Text, "## Constraints")
	require.Contains(t, contents[0].Text, "no circular deps")
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
	require.Contains(t, contents[0].Text, "`constitution` MCP tool")
	require.Contains(t, contents[0].Text, "specgraph-constitution")
	require.NotContains(t, contents[0].Text, "specgraph constitution set")
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
	require.Contains(t, contents[0].Text, "`constitution` MCP tool")
	require.Contains(t, contents[0].Text, "specgraph-constitution")
	require.NotContains(t, contents[0].Text, "specgraph constitution set")
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
	RegisterResources(r, c, &fakeSource{})
	// 11 original resources + the new templated specgraph://prime/spec/{slug}.
	require.Len(t, r.Resources(), 12)
}

func TestRegisterResources_Templates(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterResources(r, c, &fakeSource{})

	templateURIs := map[string]bool{}
	exactURIs := map[string]bool{}
	for _, res := range r.Resources() {
		if res.IsTemplate {
			templateURIs[res.URI] = true
		} else {
			exactURIs[res.URI] = true
		}
	}

	// Templates: spec/{slug}, decision/{slug}, constitution/{layer}, spec/{slug}/changes, skills/{name}, prime/spec/{slug}
	require.True(t, templateURIs["specgraph://spec/{slug}"], "spec template missing")
	require.True(t, templateURIs["specgraph://decision/{slug}"], "decision template missing")
	require.True(t, templateURIs["specgraph://constitution/{layer}"], "constitution layer template missing")
	require.True(t, templateURIs["specgraph://spec/{slug}/changes"], "changes template missing")
	require.True(t, templateURIs["specgraph://skills/{name}"], "skills template missing")
	require.True(t, templateURIs["specgraph://prime/spec/{slug}"], "prime spec template missing")
	// Prime exact should NOT be registered as a template.
	require.False(t, templateURIs["specgraph://prime"], "prime should be exact, not templated")

	// Exact URIs
	require.True(t, exactURIs["specgraph://specs"], "specs exact URI missing")
	require.True(t, exactURIs["specgraph://constitution"], "constitution exact URI missing")
	require.True(t, exactURIs["specgraph://graph"], "graph exact URI missing")
	require.True(t, exactURIs["specgraph://graph/ready"], "graph/ready exact URI missing")
	require.True(t, exactURIs["specgraph://findings"], "findings exact URI missing")
	require.True(t, exactURIs["specgraph://prime"], "prime exact URI missing")
}

// ---------------------------------------------------------------------------
// skillsResourceHandler tests
// ---------------------------------------------------------------------------

func TestSkillsResourceHandler_KnownAndUnknown(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var skillsHandler ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://skills/{name}" {
			skillsHandler = res.Handler
			break
		}
	}
	if skillsHandler == nil {
		t.Fatal("skills resource not registered")
	}

	contents, err := skillsHandler(context.Background(), "specgraph://skills/alpha")
	if err != nil {
		t.Fatalf("known: %v", err)
	}
	if len(contents) != 1 || !strings.Contains(contents[0].Text, "body-a") {
		t.Errorf("expected body-a; got %+v", contents)
	}
	if contents[0].MimeType != "text/markdown" {
		t.Errorf("expected text/markdown; got %q", contents[0].MimeType)
	}

	_, err = skillsHandler(context.Background(), "specgraph://skills/no-such")
	if err == nil {
		t.Error("expected error for unknown name")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound for unknown name; got %v", connect.CodeOf(err))
	}
}

func TestSkillsResourceHandler_RejectsMalformedURI(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var h ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://skills/{name}" {
			h = res.Handler
			break
		}
	}
	if h == nil {
		t.Fatal("skills resource not registered")
	}

	rejects := []string{
		"specgraph://skills",
		"specgraph://skills/",
		"specgraph://skills//",
		"specgraph://skills/foo/",
		"specgraph://skills/foo/bar",
		"specgraph://SKILLS/foo",
		"specgraph://skills/Foo",
		"specgraph://skills/foo%20bar",
	}
	for _, uri := range rejects {
		_, err := h(context.Background(), uri)
		if err == nil {
			t.Errorf("expected reject for %q", uri)
			continue
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("expected CodeInvalidArgument for %q; got %v", uri, connect.CodeOf(err))
		}
	}

	if _, err := h(context.Background(), "specgraph://skills/alpha"); err != nil {
		t.Errorf("expected accept for /alpha; got %v", err)
	}
}

func TestSkillsResourceHandler_UnknownNameReturnsCodeNotFound(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var h ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://skills/{name}" {
			h = res.Handler
			break
		}
	}

	// "no-such-skill" is well-formed kebab-case but not in the fake.
	_, err := h(context.Background(), "specgraph://skills/no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound; got %v", connect.CodeOf(err))
	}
}

// (legacy prime per-section error-rendering tests removed — composition
// and error handling now live in the server-side prime.Composer; see
// internal/prime and internal/server tests for that coverage.)

// ---------------------------------------------------------------------------
// primeResourceHandler / specSpecificPrimeResourceHandler tests
// ---------------------------------------------------------------------------

// primeClient builds a Client whose ExecutionService returns the supplied
// response (and optional error). All other RPC clients are left nil since
// the prime handlers should not touch them.
func primeClient(resp *specv1.PrimeResponse, callErr error) *Client {
	return &Client{
		Execution: &mockExecutionService{
			getPrime: func(_ *specv1.GetPrimeRequest) (*specv1.PrimeResponse, error) {
				if callErr != nil {
					return nil, callErr
				}
				return resp, nil
			},
		},
	}
}

func projectViewFixture() *specv1.ProjectView {
	return &specv1.ProjectView{
		Constitution: &specv1.Constitution{
			Constraints: []string{"no GPL"},
		},
		ConstitutionProvenance: []*specv1.ProvenanceEntry{
			{Path: "constraints[no GPL]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		},
		GraphOverview: &specv1.GraphOverview{CountsByStage: map[string]int32{"spark": 1}},
		Ready:         []*specv1.Spec{{Slug: "spec-a", Stage: "spark"}},
		FindingsBySeverity: map[int32]int32{
			int32(specv1.FindingSeverity_FINDING_SEVERITY_WARNING): 1,
		},
		SkillsCount: 7,
	}
}

func specViewFixture(slug string) *specv1.SpecView {
	return &specv1.SpecView{
		Spec: &specv1.Spec{Slug: slug, Stage: "spark"},
		Constitution: &specv1.Constitution{
			Constraints: []string{"no GPL"},
		},
		ConstitutionProvenance: []*specv1.ProvenanceEntry{
			{Path: "constraints[no GPL]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		},
	}
}

func TestPrimeResource_ProjectMarkdown_Default(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{ProjectView: projectViewFixture()},
	}, nil)

	contents, err := primeResourceHandler(c)(context.Background(), "specgraph://prime")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "specgraph://prime", contents[0].URI)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Contains(t, contents[0].Text, "# SpecGraph Session Prime")
	// Default markdown should NOT include provenance annotations.
	require.NotContains(t, contents[0].Text, "(set by:")
}

func TestPrimeResource_ProjectJSON(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{ProjectView: projectViewFixture()},
	}, nil)

	contents, err := primeResourceHandler(c)(context.Background(), "specgraph://prime?format=json")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(contents[0].Text), &parsed))
	// ShowProvenance=false strips constitution_provenance.
	_, hasProv := parsed["constitutionProvenance"]
	require.False(t, hasProv, "constitutionProvenance must be omitted when provenance flag is off")
}

func TestPrimeResource_ProjectMarkdown_WithProvenance(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{ProjectView: projectViewFixture()},
	}, nil)

	contents, err := primeResourceHandler(c)(context.Background(), "specgraph://prime?provenance=true")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	require.Contains(t, contents[0].Text, "(set by:",
		"provenance flag should surface (set by: <layer>) annotations from the renderer")
}

func TestPrimeResource_Spec_Markdown(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_SpecView{SpecView: specViewFixture("foo")},
	}, nil)

	contents, err := specSpecificPrimeResourceHandler(c)(context.Background(), "specgraph://prime/spec/foo")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "specgraph://prime/spec/foo", contents[0].URI)
	require.Equal(t, "text/markdown", contents[0].MimeType)
	// Spec markdown header includes the slug ("# Prime: foo").
	require.Contains(t, contents[0].Text, "# Prime: foo")
	require.NotContains(t, contents[0].Text, "(set by:")
}

func TestPrimeResource_Spec_JSON_WithProvenance(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_SpecView{SpecView: specViewFixture("foo")},
	}, nil)

	contents, err := specSpecificPrimeResourceHandler(c)(context.Background(), "specgraph://prime/spec/foo?format=json&provenance=true")
	require.NoError(t, err)
	require.Len(t, contents, 1)
	require.Equal(t, "application/json", contents[0].MimeType)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(contents[0].Text), &parsed))
	// provenance=true keeps the constitution_provenance field in the JSON payload.
	_, hasProv := parsed["constitutionProvenance"]
	require.True(t, hasProv, "constitutionProvenance must be present when provenance flag is true")
}

func TestPrimeResource_Spec_UnknownSlug(t *testing.T) {
	c := primeClient(nil, connect.NewError(connect.CodeNotFound, errors.New("spec not found")))
	_, err := specSpecificPrimeResourceHandler(c)(context.Background(), "specgraph://prime/spec/missing")
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err),
		"NotFound from the RPC must propagate through the handler")
}

// TestPrimeResource_Spec_ExactURIvsTemplated verifies the registry exposes both
// shapes: the exact specgraph://prime handler resolves project-scope and the
// templated specgraph://prime/spec/{slug} handler resolves spec-scope. Tests
// match handlers by URI and confirm each refuses the other's path.
func TestPrimeResource_Spec_ExactURIvsTemplated(t *testing.T) {
	r := NewRegistry()
	RegisterResources(r, &Client{}, &fakeSource{})

	var exactDef, tmplDef *ResourceDef
	for i := range r.Resources() {
		res := &r.resources[i]
		switch res.URI {
		case "specgraph://prime":
			exactDef = res
		case "specgraph://prime/spec/{slug}":
			tmplDef = res
		}
	}
	require.NotNil(t, exactDef, "exact specgraph://prime resource must be registered")
	require.NotNil(t, tmplDef, "templated specgraph://prime/spec/{slug} resource must be registered")
	require.False(t, exactDef.IsTemplate, "specgraph://prime must NOT be a template")
	require.True(t, tmplDef.IsTemplate, "specgraph://prime/spec/{slug} must be a template")
}

// TestPrimeResource_MalformedURIs verifies bad query strings and malformed
// spec URIs surface as CodeInvalidArgument.
func TestPrimeResource_MalformedURIs(t *testing.T) {
	c := primeClient(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{ProjectView: projectViewFixture()},
	}, nil)

	// Bad suffix on the exact URI (not "" and not "?...") is rejected.
	_, err := primeResourceHandler(c)(context.Background(), "specgraph://prime/extra")
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// Empty slug on the templated URI is rejected.
	_, err = specSpecificPrimeResourceHandler(c)(context.Background(), "specgraph://prime/spec/")
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	// Invalid format value is rejected.
	_, err = primeResourceHandler(c)(context.Background(), "specgraph://prime?format=xml")
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
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
