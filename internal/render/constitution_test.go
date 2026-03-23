// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

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
