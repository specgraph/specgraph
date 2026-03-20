// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConstitutionLayerStringToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ConstitutionLayer
	}{
		{"user", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"User", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"USER", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"org", specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
		{"project", specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		{"domain", specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN},
		{"", specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED},
		{"unknown", specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := constitutionLayerStringToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstitutionRefTypeToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ReferenceType
	}{
		{"adr", specv1.ReferenceType_REFERENCE_TYPE_ADR},
		{"ADR", specv1.ReferenceType_REFERENCE_TYPE_ADR},
		{"spec", specv1.ReferenceType_REFERENCE_TYPE_SPEC},
		{"doc", specv1.ReferenceType_REFERENCE_TYPE_DOC},
		{"url", specv1.ReferenceType_REFERENCE_TYPE_URL},
		{"", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
		{"unknown", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := constitutionRefTypeToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConstitutionConfigToProto_BasicFields(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "My Constitution",
		Layer: "project",
		Principles: []config.ConstitutionPrinciple{
			{ID: "p1", Statement: "Keep it simple", Rationale: "YAGNI"},
		},
		Constraints:  []string{"no vendor lock-in"},
		Antipatterns: []config.ConstitutionAntipattern{{Pattern: "god object", Why: "hard to test"}},
		References: []config.ConstitutionReference{
			{Type: "adr", Path: "docs/adr/001.md"},
		},
	}

	pb := constitutionConfigToProto(cc)

	assert.Equal(t, "My Constitution", pb.GetName())
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, pb.GetLayer())
	assert.Equal(t, []string{"no vendor lock-in"}, pb.GetConstraints())

	require := assert.New(t)
	require.Len(pb.GetPrinciples(), 1)
	assert.Equal(t, "p1", pb.GetPrinciples()[0].GetId())
	assert.Equal(t, "Keep it simple", pb.GetPrinciples()[0].GetStatement())

	require.Len(pb.GetAntipatterns(), 1)
	assert.Equal(t, "god object", pb.GetAntipatterns()[0].GetPattern())

	require.Len(pb.GetReferences(), 1)
	assert.Equal(t, specv1.ReferenceType_REFERENCE_TYPE_ADR, pb.GetReferences()[0].GetReferenceType())
	assert.Equal(t, "docs/adr/001.md", pb.GetReferences()[0].GetPath())
}

func TestConstitutionConfigToProto_TechConfig(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "Tech Constitution",
		Layer: "domain",
		Tech: config.ConstitutionTech{
			Languages: config.ConstitutionLangs{
				Primary:   "go",
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"php"},
			},
		},
	}

	pb := constitutionConfigToProto(cc)
	tech := pb.GetTech()
	if tech == nil {
		t.Fatal("expected Tech to be non-nil")
	}
	langs := tech.GetLanguages()
	if langs == nil {
		t.Fatal("expected Languages to be non-nil")
	}
	assert.Equal(t, "go", langs.GetPrimary())
	assert.Equal(t, []string{"go", "python"}, langs.GetAllowed())
	assert.Equal(t, []string{"php"}, langs.GetForbidden())
}

func TestConstitutionConfigToProto_EmptyTech(t *testing.T) {
	cc := &config.ConstitutionConfig{
		Name:  "No Tech",
		Layer: "org",
	}

	pb := constitutionConfigToProto(cc)
	assert.Nil(t, pb.GetTech(), "expected Tech to be nil when no tech fields set")
}
