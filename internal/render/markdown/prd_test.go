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

func TestRenderPRD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:   "auth-redesign",
		Intent: "Redesign authentication system",
		Stage:  "shape",
		SparkOutput: &specv1.SparkOutput{
			Seed:     "Auth is brittle and needs rework",
			Signal:   "Three incidents in two weeks",
			KillTest: "If compliance approves current system",
		},
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn:  []string{"OAuth2 refresh rotation", "Session management"},
			ScopeOut: []string{"SSO integration", "MFA"},
			Approaches: []*specv1.Approach{
				{Name: "Full rewrite", Description: "Start from scratch", Tradeoffs: []string{"Clean design", "High risk"}},
			},
			ChosenApproach: "Full rewrite",
			SuccessMust:    []string{"Token rotation works"},
			SuccessShould:  []string{"Session UI improved"},
			SuccessWont:    []string{"SSO support"},
			Risks:          []string{"Timeline risk"},
		},
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD() error: %v", err)
	}
	if doc.Kind != render.DocumentPRD {
		t.Errorf("Kind = %v, want DocumentPRD", doc.Kind)
	}
	if doc.SpecSlug != "auth-redesign" {
		t.Errorf("SpecSlug = %q, want %q", doc.SpecSlug, "auth-redesign")
	}
	body := string(doc.Body)
	if !strings.Contains(body, "# PRD: auth-redesign") {
		t.Error("missing PRD title")
	}
	if !strings.Contains(body, "Redesign authentication system") {
		t.Error("missing intent")
	}
	if !strings.Contains(body, "Auth is brittle") {
		t.Error("missing spark seed")
	}
	if !strings.Contains(body, "OAuth2 refresh rotation") {
		t.Error("missing scope in")
	}
	if !strings.Contains(body, "SSO integration") {
		t.Error("missing scope out")
	}
	if !strings.Contains(body, "MUST") {
		t.Error("missing success criteria")
	}
}

func TestRenderPRDNilSpec(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderPRD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}

func TestRenderPRDNoSparkOutput(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:        "no-spark",
		Intent:      "Some intent",
		Stage:       "shape",
		SparkOutput: nil,
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn: []string{"something"},
		},
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD: %v", err)
	}
	body := string(doc.Body)
	if strings.Contains(body, "## Intent") {
		t.Error("body should not contain 'Intent' section when SparkOutput is nil")
	}
	if strings.Contains(body, "Context & Signal") {
		t.Error("body should not contain 'Context & Signal' section when SparkOutput is nil")
	}
}

func TestRenderPRDNoShapeOutput(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:  "no-shape",
		Stage: "shape",
		SparkOutput: &specv1.SparkOutput{
			Seed: "Some seed idea",
		},
		ShapeOutput: nil,
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD: %v", err)
	}
	body := string(doc.Body)
	if strings.Contains(body, "## Scope") {
		t.Error("body should not contain 'Scope' section when ShapeOutput is nil")
	}
	if strings.Contains(body, "## Approaches") {
		t.Error("body should not contain 'Approaches' section when ShapeOutput is nil")
	}
}

func TestRenderPRDEmptyShapeFields(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:  "empty-shape",
		Stage: "shape",
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn:     []string{},
			ScopeOut:    []string{},
			Approaches:  []*specv1.Approach{},
			SuccessMust: []string{},
		},
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD: %v", err)
	}
	body := string(doc.Body)
	if strings.Contains(body, "## Scope") {
		t.Error("body should not contain 'Scope' section when ScopeIn/ScopeOut are empty")
	}
	if strings.Contains(body, "## Approaches") {
		t.Error("body should not contain 'Approaches' section when Approaches is empty")
	}
	if strings.Contains(body, "## Success Criteria") {
		t.Error("body should not contain 'Success Criteria' section when SuccessMust is empty")
	}
}
