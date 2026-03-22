// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

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
