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

func TestADFRenderPRD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:   "auth-redesign",
		Intent: "Redesign authentication system",
		Stage:  "shape",
		SparkOutput: &specv1.SparkOutput{
			Seed:     "Auth is brittle",
			Signal:   "Three incidents",
			KillTest: "If compliance approves",
		},
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn:       []string{"OAuth2 refresh"},
			ScopeOut:      []string{"SSO"},
			SuccessMust:   []string{"Token rotation"},
			SuccessShould: []string{"Better UX"},
			SuccessWont:   []string{"SSO support"},
			Risks:         []string{"Timeline"},
			Approaches: []*specv1.Approach{
				{Name: "Rewrite", Description: "From scratch", Tradeoffs: []string{"Clean", "Risky"}},
			},
			ChosenApproach: "Rewrite",
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
		t.Errorf("SpecSlug = %q", doc.SpecSlug)
	}
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	if m["type"] != TypeDoc {
		t.Errorf("root type = %v, want %q", m["type"], TypeDoc)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "auth-redesign") {
		t.Error("missing slug in ADF")
	}
	if !strings.Contains(body, "Auth is brittle") {
		t.Error("missing spark seed")
	}
	if !strings.Contains(body, "OAuth2 refresh") {
		t.Error("missing scope")
	}
	if !strings.Contains(body, "Token rotation") {
		t.Error("missing success criteria")
	}
}

func TestADFRenderPRDNil(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderPRD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}
