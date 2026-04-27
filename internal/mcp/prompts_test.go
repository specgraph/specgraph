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

// newComposerClient builds a Client pre-wired with the three services that the
// composerBackend calls: Constitution, Spec, and Graph.
func newComposerClient(
	constitution *mockConstitutionService,
	spec *mockSpecService,
	graph *mockGraphService,
) *Client {
	return &Client{
		Constitution: constitution,
		Spec:         spec,
		Graph:        graph,
	}
}

// defaultConstitutionMock returns a minimal GetConstitution mock.
func defaultConstitutionMock() *mockConstitutionService {
	return &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return &specv1.GetConstitutionResponse{}, nil
		},
	}
}

// defaultSpecMock returns a minimal GetSpec mock for the given slug.
func defaultSpecMock(slug string) *mockSpecService {
	return &mockSpecService{
		getSpec: func(s string) (*specv1.GetSpecResponse, error) {
			if s == slug {
				return &specv1.GetSpecResponse{
					Spec: &specv1.Spec{Slug: slug, Intent: "test intent", Stage: "shape"},
				}, nil
			}
			return &specv1.GetSpecResponse{}, nil
		},
	}
}

// defaultGraphMock returns an empty GetDependencies mock.
func defaultGraphMock() *mockGraphService {
	return &mockGraphService{
		getDeps: func(_ string) (*specv1.GetDependenciesResponse, error) {
			return &specv1.GetDependenciesResponse{}, nil
		},
	}
}

// ---------------------------------------------------------------------------
// TestSparkPrompt
// ---------------------------------------------------------------------------

func TestSparkPrompt_WithTopicAndContext(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var sparkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "spark" {
			d := p
			sparkDef = &d
			break
		}
	}
	require.NotNil(t, sparkDef)

	result, err := sparkDef.Handler(context.Background(), map[string]string{
		"topic":   "OAuth token rotation",
		"context": "Used by mobile apps",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Messages)
	require.Equal(t, "user", result.Messages[0].Role)
	// Composer produces rich content — verify stage marker and topic appendix.
	require.Contains(t, result.Messages[0].Content, "# Persona")
	require.Contains(t, result.Messages[0].Content, "OAuth token rotation")
	require.Contains(t, result.Messages[0].Content, "Used by mobile apps")
}

func TestSparkPrompt_TopicOnly(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var sparkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "spark" {
			d := p
			sparkDef = &d
			break
		}
	}
	require.NotNil(t, sparkDef)

	result, err := sparkDef.Handler(context.Background(), map[string]string{
		"topic": "rate limiting",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Messages[0].Content, "rate limiting")
	// No context provided; should not contain Additional Context section.
	require.NotContains(t, result.Messages[0].Content, "# Additional Context")
}

func TestSparkPrompt_RPCError(t *testing.T) {
	constitutionMock := &mockConstitutionService{
		getConstitution: func() (*specv1.GetConstitutionResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("server error"))
		},
	}
	c := newComposerClient(constitutionMock, defaultSpecMock(""), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var sparkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "spark" {
			d := p
			sparkDef = &d
			break
		}
	}
	require.NotNil(t, sparkDef)

	_, err := sparkDef.Handler(context.Background(), map[string]string{"topic": "test"})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// TestShapePrompt
// ---------------------------------------------------------------------------

func TestShapePrompt(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock("oauth-refresh"), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var shapeDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "shape" {
			d := p
			shapeDef = &d
			break
		}
	}
	require.NotNil(t, shapeDef)

	result, err := shapeDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user", result.Messages[0].Role)
	// Verify composer markers are present.
	require.Contains(t, result.Messages[0].Content, "# Persona")
	require.Contains(t, result.Messages[0].Content, "# Stage:")
}

// ---------------------------------------------------------------------------
// TestSpecifyPrompt
// ---------------------------------------------------------------------------

func TestSpecifyPrompt(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock("oauth-refresh"), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var specifyDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "specify" {
			d := p
			specifyDef = &d
			break
		}
	}
	require.NotNil(t, specifyDef)

	result, err := specifyDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user", result.Messages[0].Role)
	require.Contains(t, result.Messages[0].Content, "# Persona")
	require.Contains(t, result.Messages[0].Content, "# Stage:")
}

// ---------------------------------------------------------------------------
// TestDecomposePrompt
// ---------------------------------------------------------------------------

func TestDecomposePrompt(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock("oauth-refresh"), defaultGraphMock())

	r := NewRegistry()
	RegisterPrompts(r, c)
	var decomposeDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "decompose" {
			d := p
			decomposeDef = &d
			break
		}
	}
	require.NotNil(t, decomposeDef)

	result, err := decomposeDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user", result.Messages[0].Role)
	require.Contains(t, result.Messages[0].Content, "# Persona")
	require.Contains(t, result.Messages[0].Content, "# Stage:")
}

// ---------------------------------------------------------------------------
// TestConstitutionCheckPrompt
// ---------------------------------------------------------------------------

func TestConstitutionCheckPrompt(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		runAnalyticalPass: func(req *specv1.RunAnalyticalPassRequest) (*specv1.RunAnalyticalPassResponse, error) {
			require.Equal(t, "oauth-refresh", req.GetSlug())
			require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, req.GetPassType())
			return &specv1.RunAnalyticalPassResponse{
				PromptTemplate: "Check spec against constitution rules",
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	var checkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "constitution_check" {
			d := p
			checkDef = &d
			break
		}
	}
	require.NotNil(t, checkDef)

	result, err := checkDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user", result.Messages[0].Role)
	require.Contains(t, result.Messages[0].Content, "Check spec against constitution rules")
}

func TestConstitutionCheckPrompt_RPCError(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		runAnalyticalPass: func(_ *specv1.RunAnalyticalPassRequest) (*specv1.RunAnalyticalPassResponse, error) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("not found"))
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	var checkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "constitution_check" {
			d := p
			checkDef = &d
			break
		}
	}
	require.NotNil(t, checkDef)

	_, err := checkDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// TestDependencyReviewPrompt
// ---------------------------------------------------------------------------

func TestDependencyReviewPrompt(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		runAnalyticalPass: func(req *specv1.RunAnalyticalPassRequest) (*specv1.RunAnalyticalPassResponse, error) {
			require.Equal(t, "oauth-refresh", req.GetSlug())
			require.Equal(t, specv1.PassType_PASS_TYPE_PERIPHERAL_VISION, req.GetPassType())
			return &specv1.RunAnalyticalPassResponse{
				PromptTemplate: "Review dependency health",
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	var depDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "dependency_review" {
			d := p
			depDef = &d
			break
		}
	}
	require.NotNil(t, depDef)

	result, err := depDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user", result.Messages[0].Role)
	require.Contains(t, result.Messages[0].Content, "Review dependency health")
}

// ---------------------------------------------------------------------------
// TestRegisterPrompts_Count
// ---------------------------------------------------------------------------

func TestRegisterPrompts_Count(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterPrompts(r, c)
	require.Len(t, r.Prompts(), 6)
}

func TestRegisterPrompts_Names(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterPrompts(r, c)

	names := map[string]bool{}
	for _, p := range r.Prompts() {
		names[p.Name] = true
	}
	require.True(t, names["spark"], "spark prompt missing")
	require.True(t, names["shape"], "shape prompt missing")
	require.True(t, names["specify"], "specify prompt missing")
	require.True(t, names["decompose"], "decompose prompt missing")
	require.True(t, names["constitution_check"], "constitution_check prompt missing")
	require.True(t, names["dependency_review"], "dependency_review prompt missing")
}

func TestSparkPrompt_MissingTopic(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())
	r := NewRegistry()
	RegisterPrompts(r, c)
	var sparkDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "spark" {
			d := p
			sparkDef = &d
			break
		}
	}
	require.NotNil(t, sparkDef)
	_, err := sparkDef.Handler(context.Background(), map[string]string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "topic is required")
}

func TestShapePrompt_MissingSpecSlug(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())
	r := NewRegistry()
	RegisterPrompts(r, c)
	var shapeDef *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "shape" {
			d := p
			shapeDef = &d
			break
		}
	}
	require.NotNil(t, shapeDef)
	_, err := shapeDef.Handler(context.Background(), map[string]string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec_slug is required")
}

func TestConstitutionCheckPrompt_MissingSpecSlug(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterPrompts(r, c)
	var def *PromptDef
	for _, p := range r.Prompts() {
		if p.Name == "constitution_check" {
			d := p
			def = &d
			break
		}
	}
	require.NotNil(t, def)
	_, err := def.Handler(context.Background(), map[string]string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec_slug is required")
}

func TestRegisterPrompts_Arguments(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterPrompts(r, c)

	for _, p := range r.Prompts() {
		switch p.Name {
		case "spark":
			// spark has topic (required) and context (optional)
			argMap := map[string]PromptArgument{}
			for _, a := range p.Arguments {
				argMap[a.Name] = a
			}
			require.True(t, argMap["topic"].Required, "spark: topic should be required")
			require.False(t, argMap["context"].Required, "spark: context should be optional")
		case "shape", "specify", "decompose", "constitution_check", "dependency_review":
			// all have spec_slug (required)
			require.NotEmpty(t, p.Arguments, "%s should have arguments", p.Name)
			var found bool
			for _, a := range p.Arguments {
				if a.Name == "spec_slug" && a.Required {
					found = true
					break
				}
			}
			require.True(t, found, "%s: spec_slug should be required", p.Name)
		}
	}
}
