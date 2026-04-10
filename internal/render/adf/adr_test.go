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
