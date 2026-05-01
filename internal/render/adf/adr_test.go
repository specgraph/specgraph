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

func TestADFRenderADRNilDecision(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderADR(context.Background(), nil)
	if err == nil {
		t.Fatal("RenderADR(nil): expected error, got nil")
	}
}

func TestADFRenderADRScopeSetConfidenceUnspecified(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:       "scoped-no-confidence",
		Title:      "Scope Only Decision",
		Status:     specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
		Decision:   "Use approach X",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR(scope only): %v", err)
	}
	body := string(doc.Body)
	// The heading "Confidence & Scope" appears because Scope is set.
	if !strings.Contains(body, "Scope") {
		t.Error("expected 'Scope' in body")
	}
	// Confidence is UNSPECIFIED so only "Scope:" appears in the panel text, not "Confidence:".
	if strings.Contains(body, "Confidence:") {
		t.Error("unexpected 'Confidence:' in panel text when confidence is unspecified")
	}
	// Scope text should appear in the panel.
	if !strings.Contains(body, "Scope: PROJECT") {
		t.Error("expected 'Scope: PROJECT' in panel text")
	}
}

func TestADFRenderADRNoRejectedAlternativesWithDecision(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:                 "no-alternatives",
		Title:                "Simple Decision",
		Status:               specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:             "Use approach Y",
		Rationale:            "It is simpler",
		RejectedAlternatives: nil, // no alternatives
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR(no alternatives): %v", err)
	}
	body := string(doc.Body)
	// "Considered Options" table should still appear because Decision is set.
	if !strings.Contains(body, "Considered Options") {
		t.Error("expected 'Considered Options' when decision is set, even without rejected alternatives")
	}
	if !strings.Contains(body, "Simple Decision") {
		t.Error("expected title in Considered Options table")
	}
}

func TestADFRenderADRNilQuestion(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:     "no-question",
		Title:    "No Context Decision",
		Status:   specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision: "Do X",
		Question: "", // empty question
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR(no question): %v", err)
	}
	body := string(doc.Body)
	// Context section should NOT appear.
	if strings.Contains(body, "\"Context\"") {
		t.Error("unexpected Context section when question is empty")
	}
	if doc.Kind != render.DocumentADR {
		t.Errorf("Kind = %v, want DocumentADR", doc.Kind)
	}
}

func TestADFRenderADR(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:       "use-pgx",
		Title:      "Use pgx/v5 as database driver",
		Status:     specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:   "Use pgx/v5 directly",
		Rationale:  "Native PostgreSQL features",
		Question:   "Which database driver?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		OriginSpec: "auth-redesign",
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "database/sql + pq", Reason: "Missing native types"},
		},
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR() error: %v", err)
	}
	if doc.Kind != render.DocumentADR {
		t.Errorf("Kind = %v, want DocumentADR", doc.Kind)
	}
	if doc.DecisionID != "use-pgx" {
		t.Errorf("DecisionID = %q", doc.DecisionID)
	}
	if doc.SpecSlug != "auth-redesign" {
		t.Errorf("SpecSlug = %q", doc.SpecSlug)
	}
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "Use pgx/v5 as database driver") {
		t.Error("missing title")
	}
	if !strings.Contains(body, "accepted") {
		t.Error("missing status")
	}
	if !strings.Contains(body, "Which database driver") {
		t.Error("missing context/question")
	}
	if !strings.Contains(body, "database/sql + pq") {
		t.Error("missing rejected alternative")
	}
}
