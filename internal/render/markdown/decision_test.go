// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package markdown

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestDecision(t *testing.T) {
	d := &specv1.Decision{
		Slug:      "use-rotating-tokens",
		Title:     "Use rotating refresh tokens",
		Status:    specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:  "Use rotating refresh tokens with family revocation.",
		Rationale: "Security audit requires rotation.",
	}
	got := Decision(d)
	if !strings.Contains(got, "# use-rotating-tokens") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "> Use rotating refresh tokens") {
		t.Error("missing title blockquote")
	}
	if !strings.Contains(got, "| Status | accepted |") {
		t.Error("missing status")
	}
	if !strings.Contains(got, "**Decision:**") {
		t.Error("missing decision section")
	}
	if !strings.Contains(got, "**Rationale:**") {
		t.Error("missing rationale section")
	}
}

func TestDecisionSuperseded(t *testing.T) {
	d := &specv1.Decision{
		Slug:         "old-auth",
		Status:       specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
		SupersededBy: "new-auth",
	}
	got := Decision(d)
	if !strings.Contains(got, "| Superseded By | new-auth |") {
		t.Error("missing superseded_by")
	}
}

func TestDecisionNil(t *testing.T) {
	got := Decision(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestDecisionList(t *testing.T) {
	ds := []*specv1.Decision{
		{Slug: "use-memgraph", Status: specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, Title: "Use Memgraph"},
		{Slug: "old-db", Status: specv1.DecisionStatus_DECISION_STATUS_DEPRECATED, Title: "Use Postgres"},
	}
	got := DecisionList(ds)
	if !strings.Contains(got, "| Slug | Status | Title |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| use-memgraph | accepted | Use Memgraph |") {
		t.Error("missing first row")
	}
}

func TestDecisionListEmpty(t *testing.T) {
	got := DecisionList(nil)
	if !strings.Contains(got, "No decisions found.") {
		t.Error("expected empty message")
	}
}

func TestDecisionRender_NewFields(t *testing.T) {
	d := &specv1.Decision{
		Slug:        "use-postgres",
		Title:       "Token storage mechanism",
		Status:      specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:    "Use Postgres",
		Question:    "Where to store refresh tokens?",
		Confidence:  specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Scope:       specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		Tags:        []string{"auth", "storage"},
		OriginSpec:  "login-api",
		OriginStage: "specify",
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "Redis", Reason: "Adds ops complexity"},
			{Option: "DynamoDB", Reason: "Cost prohibitive"},
		},
	}
	got := Decision(d)
	if !strings.Contains(got, "Where to store refresh tokens?") {
		t.Error("missing question")
	}
	if !strings.Contains(got, "HIGH") {
		t.Error("missing confidence")
	}
	if !strings.Contains(got, "PROJECT") {
		t.Error("missing scope")
	}
	if !strings.Contains(got, "auth, storage") {
		t.Error("missing tags")
	}
	if !strings.Contains(got, "login-api") {
		t.Error("missing origin spec")
	}
	if !strings.Contains(got, "specify") {
		t.Error("missing origin stage")
	}
	if !strings.Contains(got, "Redis") {
		t.Error("missing rejected option Redis")
	}
	if !strings.Contains(got, "Adds ops complexity") {
		t.Error("missing rejected reason")
	}
	if !strings.Contains(got, "DynamoDB") {
		t.Error("missing rejected option DynamoDB")
	}
}

func TestDecisionRender_EmptyNewFields(t *testing.T) {
	d := &specv1.Decision{
		Slug:   "minimal",
		Title:  "Minimal decision",
		Status: specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
	}
	got := Decision(d)
	if strings.Contains(got, "Question") {
		t.Error("should not contain Question when empty")
	}
	if strings.Contains(got, "Confidence") {
		t.Error("should not contain Confidence when unspecified")
	}
	if strings.Contains(got, "Rejected") {
		t.Error("should not contain Rejected when empty")
	}
}
