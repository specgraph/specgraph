// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestSparkSection_Nil(t *testing.T) {
	assert.Empty(t, render.SparkSection(nil))
}

func TestSparkSection_Full(t *testing.T) {
	out := &specv1.SparkOutput{
		Seed:       "Build a widget factory",
		Signal:     "High customer demand",
		ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
		KillTest:   "No migration needed",
		Questions:  []string{"What throughput?", "Which types?"},
	}
	result := render.SparkSection(out)
	assert.Contains(t, result, "## Spark")
	assert.Contains(t, result, "Build a widget factory")
	assert.Contains(t, result, "High customer demand")
	assert.Contains(t, result, "medium")
	assert.Contains(t, result, "No migration needed")
	assert.Contains(t, result, "What throughput?")
}

func TestShapeSection_Nil(t *testing.T) {
	assert.Empty(t, render.ShapeSection(nil))
}

func TestShapeSection_Full(t *testing.T) {
	out := &specv1.ShapeOutput{
		ScopeIn:        []string{"API", "Storage"},
		ScopeOut:       []string{"Web UI"},
		ChosenApproach: "Plugin arch",
		Approaches: []*specv1.Approach{
			{Name: "Plugin arch", Description: "Modular plugins", Tradeoffs: []string{"Complex"}},
			{Name: "Monolith", Description: "Single service"},
		},
		Risks:         []string{"Performance under load"},
		SuccessMust:   []string{"CRUD via API"},
		SuccessShould: []string{"Batch ops"},
		SuccessWont:   []string{"Real-time collab"},
	}
	result := render.ShapeSection(out)
	assert.Contains(t, result, "## Shape")
	assert.Contains(t, result, "API")
	assert.Contains(t, result, "Web UI")
	assert.Contains(t, result, "Plugin arch")
	assert.Contains(t, result, "(chosen)")
	assert.Contains(t, result, "Performance under load")
	assert.Contains(t, result, "CRUD via API")
}

func TestSpecifySection_Nil(t *testing.T) {
	assert.Empty(t, render.SpecifySection(nil))
}

func TestSpecifySection_Full(t *testing.T) {
	out := &specv1.SpecifyOutput{
		Interfaces: []*specv1.InterfaceSection{
			{Name: "WidgetService", Body: "CreateWidget, GetWidget RPCs"},
		},
		VerifyCriteria: []*specv1.VerifyCriterion{
			{Category: "functional", Description: "Create returns valid ID"},
		},
		Invariants: []string{"Widget IDs are globally unique"},
		Touches: []*specv1.FileTouch{
			{Path: "internal/widget/service.go", Purpose: "Widget service", ChangeType: "create"},
		},
	}
	result := render.SpecifySection(out)
	assert.Contains(t, result, "## Specify")
	assert.Contains(t, result, "WidgetService")
	assert.Contains(t, result, "Create returns valid ID")
	assert.Contains(t, result, "Widget IDs are globally unique")
	assert.Contains(t, result, "internal/widget/service.go")
}

func TestDecomposeSection_Nil(t *testing.T) {
	assert.Empty(t, render.DecomposeSection(nil))
}

func TestDecomposeSection_Full(t *testing.T) {
	out := &specv1.DecomposeOutput{
		Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
		Slices: []*specv1.DecompositionSlice{
			{
				Id:        "slice-1",
				Intent:    "Core widget CRUD",
				Verify:    []string{"Create returns 201"},
				DependsOn: []string{},
			},
			{
				Id:        "slice-2",
				Intent:    "Batch operations",
				DependsOn: []string{"slice-1"},
			},
		},
	}
	result := render.DecomposeSection(out)
	assert.Contains(t, result, "## Decompose")
	assert.Contains(t, result, "vertical_slice")
	assert.Contains(t, result, "slice-1")
	assert.Contains(t, result, "Core widget CRUD")
	assert.Contains(t, result, "slice-2")
	assert.Contains(t, result, "slice-1") // dependency reference
}
