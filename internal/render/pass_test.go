// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/assert"
)

func TestAnalyticalPass_RendersMarkdown(t *testing.T) {
	resp := &specv1.RunAnalyticalPassResponse{
		PassType:       specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		PromptTemplate: "You are a constitution compliance analyst.\n\nEvaluate the spec.",
		Tools: []*specv1.ToolReference{
			{Name: "show_spec", Command: `specgraph show "my-spec"`, Description: "Read the spec"},
			{Name: "show_constitution", Command: "specgraph constitution show", Description: "Read the constitution"},
		},
		InitialMessage: "Run the constitution_check pass on spec \"my-spec\" (stage: shape).",
		Stage:          "shape",
	}

	got := AnalyticalPass(resp, "my-spec")

	assert.Contains(t, got, "# Constitution Check -- my-spec")
	assert.Contains(t, got, "You are a constitution compliance analyst.")
	assert.Contains(t, got, "## Tools")
	assert.Contains(t, got, "### show_spec")
	assert.Contains(t, got, `specgraph show "my-spec"`)
	assert.Contains(t, got, "### show_constitution")
	assert.Contains(t, got, "## Instructions")
	assert.Contains(t, got, "constitution_check")
}

func TestAnalyticalPass_NilResponse(t *testing.T) {
	assert.Empty(t, AnalyticalPass(nil, "slug"))
}

func TestAnalyticalPass_NoTools(t *testing.T) {
	resp := &specv1.RunAnalyticalPassResponse{
		PassType:       specv1.PassType_PASS_TYPE_RED_TEAM,
		PromptTemplate: "You are a red team analyst.",
		InitialMessage: "Run red_team pass.",
		Stage:          "specify",
	}

	got := AnalyticalPass(resp, "my-spec")

	assert.Contains(t, got, "# Red Team -- my-spec")
	assert.Contains(t, got, "You are a red team analyst.")
	assert.NotContains(t, got, "## Tools")
}
