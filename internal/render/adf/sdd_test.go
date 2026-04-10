// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package adf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestADFRenderSDD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "auth-redesign",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "TokenService", Body: "service TokenService { ... }"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "auth", Description: "Token refresh works"},
			},
			Invariants: []string{"Tokens expire in 15min"},
			Touches: []*specv1.FileTouch{
				{Path: "internal/auth/token.go", Purpose: "Rotation", ChangeType: "modify"},
			},
		},
		DecomposeOutput: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "Token rotation", Verify: []string{"Rotates"}, DependsOn: []string{}},
			},
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD() error: %v", err)
	}
	if doc.Kind != render.DocumentSDD {
		t.Errorf("Kind = %v, want DocumentSDD", doc.Kind)
	}
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "TokenService") {
		t.Error("missing interface")
	}
	if !strings.Contains(body, "Token refresh works") {
		t.Error("missing verify criterion")
	}
	if !strings.Contains(body, "Tokens expire in 15min") {
		t.Error("missing invariant")
	}
}
