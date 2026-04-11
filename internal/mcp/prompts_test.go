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
// TestSparkPrompt
// ---------------------------------------------------------------------------

func TestSparkPrompt_WithTopicAndContext(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPARK, req.GetStage())
			return &specv1.GetPromptsResponse{
				Prompts: []*specv1.PromptTemplate{
					{Stage: specv1.AuthoringStage_AUTHORING_STAGE_SPARK, Name: "spark", Template: "Generate a spec idea"},
				},
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var sparkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "spark" {
			sparkDef = &prompts[i]
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
	require.Contains(t, result.Messages[0].Content, "Generate a spec idea")
	require.Contains(t, result.Messages[0].Content, "OAuth token rotation")
	require.Contains(t, result.Messages[0].Content, "Used by mobile apps")
}

func TestSparkPrompt_TopicOnly(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(_ *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return &specv1.GetPromptsResponse{
				Prompts: []*specv1.PromptTemplate{
					{Template: "Spark template text"},
				},
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var sparkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "spark" {
			sparkDef = &prompts[i]
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
	// No context provided; should not contain "Context:" line
	require.NotContains(t, result.Messages[0].Content, "Context:")
}

func TestSparkPrompt_NoTemplates(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(_ *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return &specv1.GetPromptsResponse{Prompts: nil}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var sparkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "spark" {
			sparkDef = &prompts[i]
			break
		}
	}
	require.NotNil(t, sparkDef)

	result, err := sparkDef.Handler(context.Background(), map[string]string{
		"topic": "something",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Messages)
	require.Contains(t, result.Messages[0].Content, "spark")
}

func TestSparkPrompt_RPCError(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(_ *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("server error"))
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var sparkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "spark" {
			sparkDef = &prompts[i]
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
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SHAPE, req.GetStage())
			return &specv1.GetPromptsResponse{
				Prompts: []*specv1.PromptTemplate{
					{Template: "Shape this spec into a problem statement"},
				},
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var shapeDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "shape" {
			shapeDef = &prompts[i]
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
	require.Contains(t, result.Messages[0].Content, "Shape this spec into a problem statement")
}

func TestShapePrompt_NoTemplates(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(_ *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			return &specv1.GetPromptsResponse{}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var shapeDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "shape" {
			shapeDef = &prompts[i]
			break
		}
	}
	require.NotNil(t, shapeDef)

	result, err := shapeDef.Handler(context.Background(), map[string]string{
		"spec_slug": "oauth-refresh",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Messages[0].Content, "shape")
}

// ---------------------------------------------------------------------------
// TestSpecifyPrompt
// ---------------------------------------------------------------------------

func TestSpecifyPrompt(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY, req.GetStage())
			return &specv1.GetPromptsResponse{
				Prompts: []*specv1.PromptTemplate{
					{Template: "Add full specification detail"},
				},
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var specifyDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "specify" {
			specifyDef = &prompts[i]
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
	require.Contains(t, result.Messages[0].Content, "Add full specification detail")
}

// ---------------------------------------------------------------------------
// TestDecomposePrompt
// ---------------------------------------------------------------------------

func TestDecomposePrompt(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		getPrompts: func(req *specv1.GetPromptsRequest) (*specv1.GetPromptsResponse, error) {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE, req.GetStage())
			return &specv1.GetPromptsResponse{
				Prompts: []*specv1.PromptTemplate{
					{Template: "Break into work slices"},
				},
			}, nil
		},
	}}

	r := NewRegistry()
	RegisterPrompts(r, c)
	prompts := r.Prompts()
	var decomposeDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "decompose" {
			decomposeDef = &prompts[i]
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
	require.Contains(t, result.Messages[0].Content, "Break into work slices")
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
	prompts := r.Prompts()
	var checkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "constitution_check" {
			checkDef = &prompts[i]
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
	prompts := r.Prompts()
	var checkDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "constitution_check" {
			checkDef = &prompts[i]
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
	prompts := r.Prompts()
	var depDef *PromptDef
	for i := range prompts {
		if prompts[i].Name == "dependency_review" {
			depDef = &prompts[i]
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
