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

func TestADFRenderSDDNilSpec(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderSDD(context.Background(), nil)
	if err == nil {
		t.Fatal("RenderSDD(nil): expected error, got nil")
	}
}

func TestADFRenderSDDDecomposeOnly(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "decompose-only",
		DecomposeOutput: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "Build the whole thing", Verify: []string{"it works"}},
			},
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD(decompose only): %v", err)
	}
	if doc.Kind != render.DocumentSDD {
		t.Errorf("Kind = %v, want DocumentSDD", doc.Kind)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "single_unit") {
		t.Error("expected 'single_unit' strategy in body")
	}
	if !strings.Contains(body, "Build the whole thing") {
		t.Error("expected slice intent in body")
	}
	// Should not contain Interface or Criteria sections.
	if strings.Contains(body, "Interface Contracts") {
		t.Error("unexpected 'Interface Contracts' when SpecifyOutput is nil")
	}
}

func TestADFRenderSDDSpecifyOnly(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "specify-only",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "MyService", Body: "rpc Ping() returns (Pong)"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "smoke", Description: "basic ping works"},
			},
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD(specify only): %v", err)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "MyService") {
		t.Error("expected interface name in body")
	}
	if !strings.Contains(body, "basic ping works") {
		t.Error("expected verify criterion in body")
	}
	// Decompose section should not appear.
	if strings.Contains(body, "Decomposition Strategy") {
		t.Error("unexpected 'Decomposition Strategy' when DecomposeOutput is nil")
	}
}

func TestADFRenderSDDEmptyInterfacesAndCriteria(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "empty-lists",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces:     []*specv1.InterfaceSection{},  // empty
			VerifyCriteria: []*specv1.VerifyCriterion{},   // empty
			Invariants:     []string{},                    // empty
			Touches:        []*specv1.FileTouch{},         // empty
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD(empty lists): %v", err)
	}
	body := string(doc.Body)
	// Sections with empty lists should not be rendered.
	if strings.Contains(body, "Interface Contracts") {
		t.Error("unexpected 'Interface Contracts' section with empty interfaces")
	}
	if strings.Contains(body, "Acceptance Criteria") {
		t.Error("unexpected 'Acceptance Criteria' section with empty criteria")
	}
	if doc.Kind != render.DocumentSDD {
		t.Errorf("Kind = %v, want DocumentSDD", doc.Kind)
	}
}

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
