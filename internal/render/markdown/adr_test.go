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

func TestRenderADR(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:       "use-pgx",
		Title:      "Use pgx/v5 as database driver",
		Status:     specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:   "We will use pgx/v5 directly instead of database/sql",
		Rationale:  "Native PostgreSQL features and better performance",
		Question:   "Which database driver should we use?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "database/sql + pq", Reason: "Missing native PostgreSQL type support"},
			{Option: "sqlx", Reason: "Extra abstraction layer not needed"},
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
		t.Errorf("DecisionID = %q, want %q", doc.DecisionID, "use-pgx")
	}
	body := string(doc.Body)
	if !strings.Contains(body, "# ADR: Use pgx/v5 as database driver") {
		t.Error("missing MADR title")
	}
	if !strings.Contains(body, "## Status") {
		t.Error("missing status section")
	}
	if !strings.Contains(body, "accepted") {
		t.Error("missing status value")
	}
	if !strings.Contains(body, "## Context") {
		t.Error("missing context section")
	}
	if !strings.Contains(body, "## Decision") {
		t.Error("missing decision section")
	}
	if !strings.Contains(body, "## Considered Options") {
		t.Error("missing considered options")
	}
	if !strings.Contains(body, "database/sql + pq") {
		t.Error("missing rejected alternative")
	}
	if !strings.Contains(body, "HIGH") {
		t.Error("missing confidence")
	}
}

func TestRenderADRNilDecision(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderADR(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil decision")
	}
}
