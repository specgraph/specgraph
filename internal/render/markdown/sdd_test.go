// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package markdown

import (
	"context"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestRenderSDD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "auth-redesign",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "TokenService", Body: "service TokenService { ... }"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "auth", Description: "Token refresh works under load"},
			},
			Invariants: []string{"Tokens expire within 15 minutes"},
			Touches: []*specv1.FileTouch{
				{Path: "internal/auth/token.go", Purpose: "Token rotation", ChangeType: "modify"},
			},
		},
		DecomposeOutput: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "slice-1", Intent: "Token rotation", Verify: []string{"Tokens rotate"}, Touches: []string{"internal/auth/"}},
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
	body := string(doc.Body)
	if !strings.Contains(body, "# SDD: auth-redesign") {
		t.Error("missing SDD title")
	}
	if !strings.Contains(body, "TokenService") {
		t.Error("missing interface")
	}
	if !strings.Contains(body, "Token refresh works under load") {
		t.Error("missing verify criterion")
	}
	if !strings.Contains(body, "Tokens expire within 15 minutes") {
		t.Error("missing invariant")
	}
	if !strings.Contains(body, "vertical_slice") {
		t.Error("missing strategy")
	}
}

func TestRenderSDDNilSpec(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderSDD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}
