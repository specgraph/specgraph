// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// expectedProjectMatchLegacy is the byte-stable rendering of a ProjectView
// equivalent to today's specgraph://prime output for a representative
// fixture. Pinning the literal here guards the rewrite invariant from
// Section 14 of the spgr-8ar design.
const expectedProjectMatchLegacy = `# SpecGraph Session Prime

## Constitution

Primary language: Go

Top constraints:
- No ORM usage
- Specs are graph nodes

Full at ` + "`specgraph://constitution`" + `.

## Graph Overview

- spark: 2
- shape: 1
- specify: 3

## Ready to Work

- ` + "`auth-rework`" + ` (shape)
- ` + "`pay-flow`" + ` (specify)

Full list at ` + "`specgraph://graph/ready`" + `.

## Open Findings

- FINDING_SEVERITY_CRITICAL: 1
- FINDING_SEVERITY_WARNING: 2

Full at ` + "`specgraph://findings`" + `.

## Skills

7 skills exposed via MCP. Use ` + "`specgraph_skills_list`" + ` to see the catalog, ` + "`specgraph_skills_search`" + ` to find one by keyword, and ` + "`specgraph_skills_get`" + ` / ` + "`specgraph://skills/<name>`" + ` to fetch a specific skill. Start here: ` + "`specgraph_skills_list`" + ` the catalog, then ` + "`specgraph_skills_get name=specgraph-constitution`" + ` or ` + "`specgraph-authoring`" + ` to author the constitution or a spec.

`

func TestRenderProjectMarkdown_EmptyConstitution(t *testing.T) {
	v := &specv1.ProjectView{}
	got := RenderProjectMarkdown(v, RenderOpts{})

	require.Contains(t, got, ConstitutionEmptyHint)
	require.Contains(t, got, "`constitution` MCP tool")
	require.Contains(t, got, "specgraph-constitution")
	require.NotContains(t, got, "specgraph constitution set")
	require.NotContains(t, got, "Top constraints:")
}

func TestRenderSpecMarkdown_EmptyConstitution(t *testing.T) {
	v := &specv1.SpecView{
		Spec: &specv1.Spec{Slug: "pay-flow", Stage: "spark"},
	}
	got := RenderSpecMarkdown(v, RenderOpts{})

	require.Contains(t, got, ConstitutionEmptyHint)
	require.Contains(t, got, "`constitution` MCP tool")
	require.Contains(t, got, "specgraph-constitution")
	// D-10: an MCP-only agent that sparks a spec first must not be routed to a
	// CLI it may not have.
	require.NotContains(t, got, "specgraph constitution set")
	require.NotContains(t, got, "specgraph constitution")
}

func TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout(t *testing.T) {
	v := fixtureProjectView()
	got := RenderProjectMarkdown(v, RenderOpts{})

	require.Equal(t, expectedProjectMatchLegacy, got)
}

func TestRenderProjectMarkdown_WithProvenance(t *testing.T) {
	v := fixtureProjectView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
		{Path: "constraints[No ORM usage]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}

	got := RenderProjectMarkdown(v, RenderOpts{ShowProvenance: true})

	require.Contains(t, got, "(set by:")
	require.Contains(t, got, "Primary language: Go (set by: org)")
	require.Contains(t, got, "- No ORM usage  (set by: project)")
}

func TestRenderProjectMarkdown_AllSectionsPopulated(t *testing.T) {
	v := fixtureProjectView()
	got := RenderProjectMarkdown(v, RenderOpts{})

	for _, want := range []string{
		"# SpecGraph Session Prime",
		"## Constitution",
		"## Graph Overview",
		"## Ready to Work",
		"## Open Findings",
		"## Skills",
	} {
		require.Contains(t, got, want, "missing section %q", want)
	}
}

func TestRenderSpecMarkdown_Basic(t *testing.T) {
	v := fixtureSpecView()
	got := RenderSpecMarkdown(v, RenderOpts{})

	require.Contains(t, got, "# Prime: pay-flow")
	require.Contains(t, got, "## Spec")
	require.Contains(t, got, "Stage: specify")
	require.Contains(t, got, "Priority: high")
	require.Contains(t, got, "## Constitution")
	require.Contains(t, got, "## Decisions")
	require.Contains(t, got, "- [adr-007] Use Stripe")
	require.Contains(t, got, "## Slices")
	require.Contains(t, got, "- `slice-a`")
	require.Contains(t, got, "## Claims")
	require.Contains(t, got, "Active claim: claude-1")
	require.Contains(t, got, "## Blockers")
	require.Contains(t, got, "- Webhook flaky")
}

func TestRenderSpecMarkdown_WithProvenance(t *testing.T) {
	v := fixtureSpecView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}
	got := RenderSpecMarkdown(v, RenderOpts{ShowProvenance: true})
	require.Contains(t, got, "Primary language: Go (set by: project)")
}

func TestRenderSpecMarkdown_NilView(t *testing.T) {
	require.Equal(t, "", RenderSpecMarkdown(nil, RenderOpts{}))
}

func TestRenderProjectJSON_OmitsProvenanceByDefault(t *testing.T) {
	v := fixtureProjectView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}

	data, err := RenderProjectJSON(v, RenderOpts{})
	require.NoError(t, err)
	s := string(data)
	require.NotContains(t, s, "constitution_provenance")
	require.NotContains(t, s, "constitutionProvenance")
}

func TestRenderProjectJSON_IncludesProvenanceWhenAsked(t *testing.T) {
	v := fixtureProjectView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}

	data, err := RenderProjectJSON(v, RenderOpts{ShowProvenance: true})
	require.NoError(t, err)
	require.Contains(t, string(data), "constitutionProvenance")

	// The original ProjectView must still carry provenance — we should not
	// have mutated the caller's value.
	require.Len(t, v.GetConstitutionProvenance(), 1)
}

func TestRenderSpecJSON_OmitsProvenanceByDefault(t *testing.T) {
	v := fixtureSpecView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}

	data, err := RenderSpecJSON(v, RenderOpts{})
	require.NoError(t, err)
	s := string(data)
	require.NotContains(t, s, "constitution_provenance")
	require.NotContains(t, s, "constitutionProvenance")
}

func TestRenderSpecJSON_IncludesProvenanceWhenAsked(t *testing.T) {
	v := fixtureSpecView()
	v.ConstitutionProvenance = []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}

	data, err := RenderSpecJSON(v, RenderOpts{ShowProvenance: true})
	require.NoError(t, err)
	require.Contains(t, string(data), "constitutionProvenance")
}

func TestRenderProjectMarkdown_GraphOverviewFunnelOrder(t *testing.T) {
	v := &specv1.ProjectView{
		GraphOverview: &specv1.GraphOverview{
			CountsByStage: map[string]int32{
				"specify":   1,
				"spark":     1,
				"shape":     1,
				"in_review": 1, // unknown stage → alphabetical leftover
			},
		},
	}
	got := RenderProjectMarkdown(v, RenderOpts{})

	// Funnel-order stages appear before alphabetical leftovers.
	sparkIdx := strings.Index(got, "- spark:")
	shapeIdx := strings.Index(got, "- shape:")
	specifyIdx := strings.Index(got, "- specify:")
	leftoverIdx := strings.Index(got, "- in_review:")
	require.NotEqual(t, -1, sparkIdx)
	require.NotEqual(t, -1, leftoverIdx)
	require.True(t, sparkIdx < shapeIdx)
	require.True(t, shapeIdx < specifyIdx)
	require.True(t, specifyIdx < leftoverIdx)
}

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

func fixtureProjectView() *specv1.ProjectView {
	return &specv1.ProjectView{
		Constitution: &specv1.Constitution{
			Name:    "SpecGraph",
			Layer:   specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			Version: 1,
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{Primary: "Go"},
			},
			Constraints: []string{"No ORM usage", "Specs are graph nodes"},
		},
		GraphOverview: &specv1.GraphOverview{
			CountsByStage: map[string]int32{
				"spark":   2,
				"shape":   1,
				"specify": 3,
			},
		},
		Ready: []*specv1.Spec{
			{Slug: "auth-rework", Stage: "shape"},
			{Slug: "pay-flow", Stage: "specify"},
		},
		FindingsBySeverity: map[int32]int32{
			int32(specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL): 1,
			int32(specv1.FindingSeverity_FINDING_SEVERITY_WARNING):  2,
		},
		SkillsCount: 7,
	}
}

func fixtureSpecView() *specv1.SpecView {
	expires := timestamppb.New(time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	createdAt := timestamppb.New(time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC))
	return &specv1.SpecView{
		Spec: &specv1.Spec{
			Slug:     "pay-flow",
			Stage:    "specify",
			Priority: "high",
			Intent:   "Implement subscription billing using Stripe.",
		},
		Constitution: &specv1.Constitution{
			Name: "SpecGraph",
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{Primary: "Go"},
			},
			Constraints: []string{"No ORM usage"},
		},
		Decisions: []*specv1.Decision{
			{Slug: "adr-007", Title: "Use Stripe"},
		},
		Slices: []*specv1.Slice{
			{Slug: "slice-a", Intent: "Set up Stripe SDK."},
		},
		Claims: []*specv1.Claim{
			{Agent: "claude-1", LeaseExpires: expires},
		},
		Blockers: []*specv1.ExecutionEvent{
			{
				Message:   "Webhook flaky",
				Agent:     "claude-1",
				CreatedAt: createdAt,
				Type:      specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER,
			},
		},
	}
}
