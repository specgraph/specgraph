// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstitutionToProto_Full(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	c := &storage.Constitution{
		ID:      "const-1",
		Layer:   storage.ConstitutionLayerProject,
		Name:    "acme-project",
		Version: 2,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:          "go",
				Allowed:          []string{"go", "typescript"},
				Forbidden:        []string{"perl"},
				ForbiddenReasons: map[string]string{"perl": "unmaintainable"},
			},
			Frameworks:     map[string]string{"web": "connectrpc"},
			Infrastructure: map[string]string{"db": "memgraph"},
			APIStandards:   map[string]string{"rpc": "connectrpc"},
			Data:           map[string]string{"graph": "memgraph"},
		},
		Principles: []storage.Principle{
			{ID: "p1", Statement: "keep it simple", Rationale: "complexity kills", Exceptions: "perf-critical paths"},
		},
		Process: &storage.ProcessConfig{
			SpecReview:     "required",
			SecurityReview: &storage.SecurityReviewConfig{When: "always"},
			Deployment:     &storage.DeploymentConfig{Strategy: "rolling", Rollback: "automatic"},
			Documentation:  &storage.DocumentationConfig{APIDocs: "required", Runbook: "required"},
		},
		Constraints:  []string{"no-external-deps"},
		Antipatterns: []storage.Antipattern{{Pattern: "god object", Why: "coupling", Instead: "decompose"}},
		References:   []storage.Reference{{Type: "adr", Path: "docs/adr-001.md"}},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	pb := constitutionToProto(c)
	require.NotNil(t, pb)

	assert.Equal(t, "const-1", pb.Id)
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, pb.Layer)
	assert.Equal(t, "acme-project", pb.Name)
	assert.Equal(t, int32(2), pb.Version)
	assert.Equal(t, []string{"no-external-deps"}, pb.Constraints)
	assert.Equal(t, now.Unix(), pb.CreatedAt.AsTime().Unix())
	assert.Equal(t, now.Unix(), pb.UpdatedAt.AsTime().Unix())

	// Tech stack
	require.NotNil(t, pb.Tech)
	require.NotNil(t, pb.Tech.Languages)
	assert.Equal(t, "go", pb.Tech.Languages.Primary)
	assert.Equal(t, []string{"go", "typescript"}, pb.Tech.Languages.Allowed)
	assert.Equal(t, []string{"perl"}, pb.Tech.Languages.Forbidden)
	assert.Equal(t, map[string]string{"perl": "unmaintainable"}, pb.Tech.Languages.ForbiddenReasons)
	assert.Equal(t, map[string]string{"web": "connectrpc"}, pb.Tech.Frameworks)

	// Principles
	require.Len(t, pb.Principles, 1)
	assert.Equal(t, "p1", pb.Principles[0].Id)
	assert.Equal(t, "keep it simple", pb.Principles[0].Statement)
	assert.Equal(t, "complexity kills", pb.Principles[0].Rationale)
	assert.Equal(t, "perf-critical paths", pb.Principles[0].Exceptions)

	// Process
	require.NotNil(t, pb.Process)
	assert.Equal(t, "required", pb.Process.SpecReview)
	require.NotNil(t, pb.Process.SecurityReview)
	assert.Equal(t, "always", pb.Process.SecurityReview.When)
	require.NotNil(t, pb.Process.Deployment)
	assert.Equal(t, "rolling", pb.Process.Deployment.Strategy)
	assert.Equal(t, "automatic", pb.Process.Deployment.Rollback)
	require.NotNil(t, pb.Process.Documentation)
	assert.Equal(t, "required", pb.Process.Documentation.ApiDocs)
	assert.Equal(t, "required", pb.Process.Documentation.Runbook)

	// Antipatterns
	require.Len(t, pb.Antipatterns, 1)
	assert.Equal(t, "god object", pb.Antipatterns[0].Pattern)
	assert.Equal(t, "coupling", pb.Antipatterns[0].Why)
	assert.Equal(t, "decompose", pb.Antipatterns[0].Instead)

	// References
	require.Len(t, pb.References, 1)
	assert.Equal(t, specv1.ReferenceType_REFERENCE_TYPE_ADR, pb.References[0].ReferenceType)
	assert.Equal(t, "docs/adr-001.md", pb.References[0].Path)
}

func TestConstitutionToProto_Nil(t *testing.T) {
	assert.Nil(t, constitutionToProto(nil))
}

func TestConstitutionToProto_Minimal(t *testing.T) {
	c := &storage.Constitution{ID: "const-min", Layer: storage.ConstitutionLayerUser, Name: "minimal"}
	pb := constitutionToProto(c)
	require.NotNil(t, pb)
	assert.Equal(t, "const-min", pb.Id)
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER, pb.Layer)
	assert.Nil(t, pb.Tech)
	assert.Nil(t, pb.Process)
	assert.Empty(t, pb.Principles)
	assert.Empty(t, pb.Antipatterns)
	assert.Empty(t, pb.References)
}

func TestConstitutionRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	original := &storage.Constitution{
		ID:      "const-rt",
		Layer:   storage.ConstitutionLayerDomain,
		Name:    "round-trip",
		Version: 3,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:          "rust",
				Allowed:          []string{"rust", "go"},
				Forbidden:        []string{"c++"},
				ForbiddenReasons: map[string]string{"c++": "safety"},
			},
			Frameworks:     map[string]string{"rpc": "tonic"},
			Infrastructure: map[string]string{"kv": "redis"},
			APIStandards:   map[string]string{"format": "grpc"},
			Data:           map[string]string{"store": "sled"},
		},
		Principles: []storage.Principle{
			{ID: "p1", Statement: "safety first", Rationale: "memory bugs", Exceptions: "ffi"},
		},
		Process: &storage.ProcessConfig{
			SpecReview:     "mandatory",
			SecurityReview: &storage.SecurityReviewConfig{When: "pr"},
			Deployment:     &storage.DeploymentConfig{Strategy: "canary", Rollback: "manual"},
			Documentation:  &storage.DocumentationConfig{APIDocs: "swagger", Runbook: "wiki"},
		},
		Constraints:  []string{"no-unsafe"},
		Antipatterns: []storage.Antipattern{{Pattern: "raw pointers", Why: "UB", Instead: "smart ptrs"}},
		References:   []storage.Reference{{Type: "spec", Path: "specs/core.md"}, {Type: "url", Path: "https://example.com"}},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	pb := constitutionToProto(original)
	back, err := constitutionFromProto(pb)
	require.NoError(t, err)

	assert.Equal(t, original.ID, back.ID)
	assert.Equal(t, original.Layer, back.Layer)
	assert.Equal(t, original.Name, back.Name)
	assert.Equal(t, original.Version, back.Version)
	assert.Equal(t, original.Constraints, back.Constraints)
	assert.Equal(t, original.CreatedAt.Unix(), back.CreatedAt.Unix())
	assert.Equal(t, original.UpdatedAt.Unix(), back.UpdatedAt.Unix())

	// Tech round-trip
	require.NotNil(t, back.Tech)
	require.NotNil(t, back.Tech.Languages)
	assert.Equal(t, original.Tech.Languages.Primary, back.Tech.Languages.Primary)
	assert.Equal(t, original.Tech.Languages.Allowed, back.Tech.Languages.Allowed)
	assert.Equal(t, original.Tech.Frameworks, back.Tech.Frameworks)

	// Principles round-trip
	require.Len(t, back.Principles, 1)
	assert.Equal(t, original.Principles[0].ID, back.Principles[0].ID)
	assert.Equal(t, original.Principles[0].Statement, back.Principles[0].Statement)

	// Process round-trip
	require.NotNil(t, back.Process)
	assert.Equal(t, original.Process.SpecReview, back.Process.SpecReview)
	assert.Equal(t, original.Process.SecurityReview.When, back.Process.SecurityReview.When)
	assert.Equal(t, original.Process.Deployment.Strategy, back.Process.Deployment.Strategy)
	assert.Equal(t, original.Process.Documentation.APIDocs, back.Process.Documentation.APIDocs)

	// Antipatterns round-trip
	require.Len(t, back.Antipatterns, 1)
	assert.Equal(t, original.Antipatterns[0].Pattern, back.Antipatterns[0].Pattern)

	// References round-trip
	require.Len(t, back.References, 2)
	assert.Equal(t, "spec", back.References[0].Type)
	assert.Equal(t, "url", back.References[1].Type)
}

func TestConstitutionFromProto_Nil(t *testing.T) {
	got, err := constitutionFromProto(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestConstitutionFromProto_NilTimestamps(t *testing.T) {
	pb := &specv1.Constitution{
		Id:    "const-no-ts",
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
		Name:  "no-timestamps",
	}
	c, err := constitutionFromProto(pb)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.True(t, c.CreatedAt.IsZero())
	assert.True(t, c.UpdatedAt.IsZero())
}

func TestConstitutionLayerMapping(t *testing.T) {
	tests := []struct {
		domain storage.ConstitutionLayer
		proto  specv1.ConstitutionLayer
	}{
		{storage.ConstitutionLayerUser, specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{storage.ConstitutionLayerOrg, specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
		{storage.ConstitutionLayerProject, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		{storage.ConstitutionLayerDomain, specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			// Forward
			assert.Equal(t, tt.proto, constitutionLayerToProtoMap[tt.domain])
			// Reverse
			assert.Equal(t, tt.domain, constitutionLayerFromProtoMap[tt.proto])
		})
	}
}

func TestReferenceTypeToProto(t *testing.T) {
	tests := []struct {
		domain string
		proto  specv1.ReferenceType
	}{
		{"adr", specv1.ReferenceType_REFERENCE_TYPE_ADR},
		{"spec", specv1.ReferenceType_REFERENCE_TYPE_SPEC},
		{"doc", specv1.ReferenceType_REFERENCE_TYPE_DOC},
		{"url", specv1.ReferenceType_REFERENCE_TYPE_URL},
		{"unknown", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
		{"", specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			assert.Equal(t, tt.proto, referenceTypeToProto(tt.domain))
		})
	}
}

func TestReferenceTypeFromProto(t *testing.T) {
	tests := []struct {
		proto  specv1.ReferenceType
		domain string
	}{
		{specv1.ReferenceType_REFERENCE_TYPE_ADR, "adr"},
		{specv1.ReferenceType_REFERENCE_TYPE_SPEC, "spec"},
		{specv1.ReferenceType_REFERENCE_TYPE_DOC, "doc"},
		{specv1.ReferenceType_REFERENCE_TYPE_URL, "url"},
		{specv1.ReferenceType_REFERENCE_TYPE_UNSPECIFIED, ""},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			assert.Equal(t, tt.domain, referenceTypeFromProto(tt.proto))
		})
	}
}

func TestTechStackToProto_NilLanguages(t *testing.T) {
	ts := &storage.TechStack{
		Frameworks: map[string]string{"web": "gin"},
	}
	pb := techStackToProto(ts)
	assert.Nil(t, pb.Languages)
	assert.Equal(t, map[string]string{"web": "gin"}, pb.Frameworks)
}

func TestTechStackRoundTrip(t *testing.T) {
	original := &storage.TechStack{
		Languages: &storage.Languages{
			Primary: "python",
			Allowed: []string{"python", "go"},
		},
		Frameworks:     map[string]string{"web": "fastapi"},
		Infrastructure: map[string]string{"queue": "redis"},
		APIStandards:   map[string]string{"rest": "openapi"},
		Data:           map[string]string{"db": "postgres"},
	}
	pb := techStackToProto(original)
	back := techStackFromProto(pb)
	assert.Equal(t, original.Languages.Primary, back.Languages.Primary)
	assert.Equal(t, original.Frameworks, back.Frameworks)
	assert.Equal(t, original.Infrastructure, back.Infrastructure)
	assert.Equal(t, original.APIStandards, back.APIStandards)
	assert.Equal(t, original.Data, back.Data)
}

func TestProcessConfigToProto_PartialFields(t *testing.T) {
	t.Run("only spec_review", func(t *testing.T) {
		p := &storage.ProcessConfig{SpecReview: "optional"}
		pb := processConfigToProto(p)
		assert.Equal(t, "optional", pb.SpecReview)
		assert.Nil(t, pb.SecurityReview)
		assert.Nil(t, pb.Deployment)
		assert.Nil(t, pb.Documentation)
	})

	t.Run("all sub-configs set", func(t *testing.T) {
		p := &storage.ProcessConfig{
			SpecReview:     "required",
			SecurityReview: &storage.SecurityReviewConfig{When: "pr"},
			Deployment:     &storage.DeploymentConfig{Strategy: "blue-green", Rollback: "auto"},
			Documentation:  &storage.DocumentationConfig{APIDocs: "openapi", Runbook: "confluence"},
		}
		pb := processConfigToProto(p)
		require.NotNil(t, pb.SecurityReview)
		assert.Equal(t, "pr", pb.SecurityReview.When)
		require.NotNil(t, pb.Deployment)
		assert.Equal(t, "blue-green", pb.Deployment.Strategy)
		require.NotNil(t, pb.Documentation)
		assert.Equal(t, "openapi", pb.Documentation.ApiDocs)
	})
}

func TestProcessConfigRoundTrip(t *testing.T) {
	original := &storage.ProcessConfig{
		SpecReview:     "required",
		SecurityReview: &storage.SecurityReviewConfig{When: "merge"},
		Deployment:     &storage.DeploymentConfig{Strategy: "canary", Rollback: "auto"},
		Documentation:  &storage.DocumentationConfig{APIDocs: "swagger", Runbook: "wiki"},
	}
	pb := processConfigToProto(original)
	back := processConfigFromProto(pb)
	assert.Equal(t, original.SpecReview, back.SpecReview)
	assert.Equal(t, original.SecurityReview.When, back.SecurityReview.When)
	assert.Equal(t, original.Deployment.Strategy, back.Deployment.Strategy)
	assert.Equal(t, original.Deployment.Rollback, back.Deployment.Rollback)
	assert.Equal(t, original.Documentation.APIDocs, back.Documentation.APIDocs)
	assert.Equal(t, original.Documentation.Runbook, back.Documentation.Runbook)
}

func TestOutputFormatToString(t *testing.T) {
	assert.Equal(t, "claude-md", outputFormatToString[specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD])
	assert.Equal(t, "cursorrules", outputFormatToString[specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES])
	assert.Equal(t, "agents-md", outputFormatToString[specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD])
}

func TestConstitutionFromProto_UnknownLayer(t *testing.T) {
	pb := &specv1.Constitution{
		Id:    "const-bad",
		Layer: specv1.ConstitutionLayer(99),
		Name:  "bad-layer",
	}
	_, err := constitutionFromProto(pb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported constitution layer")
}

func TestConstitutionFromProto_UnknownReferenceType(t *testing.T) {
	pb := &specv1.Constitution{
		Id:    "const-bad-ref",
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
		Name:  "bad-ref",
		References: []*specv1.Reference{
			{ReferenceType: specv1.ReferenceType(99), Path: "bad.md"},
		},
	}
	_, err := constitutionFromProto(pb)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported reference type")
}

func TestInjectToolFromProto_Unknown(t *testing.T) {
	_, err := injectToolFromProto(specv1.InjectTool(99))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown inject tool")
}
