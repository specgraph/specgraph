// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name:    "SpecGraph",
		Layer:   specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version: 2,
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{Primary: "Go"},
		},
		Principles: []*specv1.Principle{
			{Statement: "Specs are graph nodes"},
		},
		Constraints: []string{"No ORM usage"},
		Antipatterns: []*specv1.Antipattern{
			{Pattern: "God objects", Why: "Violates SRP"},
		},
		References: []*specv1.Reference{
			{ReferenceType: specv1.ReferenceType_REFERENCE_TYPE_ADR, Path: "docs/adr/002-content-hash.md"},
		},
	}
	got := Constitution(c)
	if !strings.Contains(got, "# SpecGraph") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Layer | project |") {
		t.Error("missing layer")
	}
	if !strings.Contains(got, "| Primary Language | Go |") {
		t.Error("missing tech")
	}
	if !strings.Contains(got, "## Principles") {
		t.Error("missing principles section")
	}
	if !strings.Contains(got, "- Specs are graph nodes") {
		t.Error("missing principle")
	}
	if !strings.Contains(got, "## Constraints") {
		t.Error("missing constraints section")
	}
	if !strings.Contains(got, "## Anti-patterns") {
		t.Error("missing antipatterns section")
	}
	if !strings.Contains(got, "- **God objects**: Violates SRP") {
		t.Error("missing antipattern")
	}
	if !strings.Contains(got, "## References") {
		t.Error("missing references section")
	}
	if !strings.Contains(got, "[ADR] docs/adr/002-content-hash.md") {
		t.Error("missing reference")
	}
}

func TestConstitutionNil(t *testing.T) {
	got := Constitution(nil)
	if !strings.Contains(got, "No constitution found.") {
		t.Error("expected empty message")
	}
}

// Renderer invariant: ConstitutionWithProvenance(c, nil) ≡ Constitution(c).
func TestConstitutionWithProvenance_NilProvenance_EquivalentToConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name:        "test",
		Layer:       specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version:     1,
		Constraints: []string{"never use eval"},
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "Prefer explicit"},
		},
	}
	a := Constitution(c)
	b := ConstitutionWithProvenance(c, nil)
	assert.Equal(t, a, b, "with nil provenance, output must be byte-identical to Constitution(c)")
}

func TestConstitutionWithProvenance_EmptyProvenance_EquivalentToConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name:       "test",
		Principles: []*specv1.Principle{{Id: "p1", Statement: "Prefer explicit"}},
	}
	a := Constitution(c)
	b := ConstitutionWithProvenance(c, []*specv1.ProvenanceEntry{})
	assert.Equal(t, a, b, "with empty provenance, output must be byte-identical to Constitution(c)")
}

func TestConstitutionWithProvenance_AnnotatesPrinciples(t *testing.T) {
	c := &specv1.Constitution{
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "First"},
			{Id: "p2", Statement: "Second"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		{Path: "principles[p2]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- First  (set by: project)")
	assert.Contains(t, out, "- Second  (set by: org)")
}

func TestConstitutionWithProvenance_PartialProvenance_OnlyAnnotatesPresent(t *testing.T) {
	c := &specv1.Constitution{
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "First"},
			{Id: "p2", Statement: "Second"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- First  (set by: project)")
	assert.Contains(t, out, "- Second\n")
	assert.NotContains(t, out, "- Second  (set by:")
}

func TestConstitutionWithProvenance_Constraints(t *testing.T) {
	c := &specv1.Constitution{
		Constraints: []string{"never use eval"},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "constraints[never use eval]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- never use eval  (set by: domain)")
}

func TestConstitutionWithProvenance_Antipatterns(t *testing.T) {
	c := &specv1.Constitution{
		Antipatterns: []*specv1.Antipattern{
			{Pattern: "bad-pat", Why: "because"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "antipatterns[bad-pat]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- **bad-pat**: because  (set by: org)")
}

func TestConstitutionWithProvenance_References(t *testing.T) {
	c := &specv1.Constitution{
		References: []*specv1.Reference{
			{ReferenceType: specv1.ReferenceType_REFERENCE_TYPE_URL, Path: "https://example.com"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "references[https://example.com]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- [URL] https://example.com  (set by: user)")
}

func TestConstitutionWithProvenance_PrimaryLanguage(t *testing.T) {
	c := &specv1.Constitution{
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary: "go",
			},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "tech_config.languages.primary", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "| Primary Language | go (set by: project) |",
		"Primary Language metadata cell must carry inline provenance annotation")
}

func TestConstitutionWithProvenance_PrimaryLanguage_NoProvenance(t *testing.T) {
	// When tech_config.languages.primary has no provenance entry, the
	// table cell should render the value alone — no "(set by: )" leak.
	c := &specv1.Constitution{
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary: "go",
			},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		// Provenance for an unrelated field, so the function still runs
		// the with-provenance branch but has no entry for primary lang.
		{Path: "principles[unrelated]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "| Primary Language | go |",
		"missing provenance for primary language must render the bare value")
	assert.NotContains(t, out, "Primary Language | go (set by:")
}

func TestConstitutionWithProvenance_NilConstitution(t *testing.T) {
	out := ConstitutionWithProvenance(nil, []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	})
	assert.Equal(t, "No constitution found.\n", out, "nil constitution falls through to the 'not found' message")
}

// Golden-file byte-stability test for the legacy Constitution renderer.
// If this test fails, downstream scripts/diffs depending on today's
// output bytes will break.
//
// To pin the golden value: run this test (it skips with a logf showing
// the captured output), copy the captured value into legacyGolden, then
// run again — all asserts pass.
func TestConstitution_LegacyGolden(t *testing.T) {
	c := &specv1.Constitution{
		Name:    "golden",
		Layer:   specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version: 1,
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "Be explicit"},
		},
		Constraints: []string{"no-eval"},
	}
	out := Constitution(c)

	const legacyGolden = `# golden

| Field | Value |
|-------|-------|
| Layer | project |
| Version | 1 |

## Principles

- Be explicit

## Constraints

- no-eval
`

	if legacyGolden == "" {
		t.Logf("captured legacy output (pin this in legacyGolden):\n---\n%s---\n(use Go string literal escaping)", out)
		t.Skip("legacyGolden not yet pinned")
	}
	assert.Equal(t, legacyGolden, out,
		"Constitution(c) bytes changed; pin a new golden if the change is intentional and downstream-safe")

	// Use strings.Contains as a sanity check that the test isn't asserting nothing.
	assert.True(t, strings.Contains(out, "golden"))
}
