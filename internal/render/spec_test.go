// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestSpec(t *testing.T) {
	s := &specv1.Spec{
		Slug:       "login-api",
		Intent:     "Implement OAuth2 login flow",
		Stage:      "specify",
		Priority:   "p1",
		Complexity: "medium",
		Version:    3,
		Lifecycle:  specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK,
	}
	got := Spec(s)
	if !strings.Contains(got, "# login-api") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "> Implement OAuth2 login flow") {
		t.Error("missing blockquote intent")
	}
	if !strings.Contains(got, "| Stage | specify |") {
		t.Error("missing stage row")
	}
	if !strings.Contains(got, "| Priority | p1 |") {
		t.Error("missing priority row")
	}
	if !strings.Contains(got, "| Lifecycle | task |") {
		t.Error("missing lifecycle row")
	}
}

func TestSpecWithNotes(t *testing.T) {
	s := &specv1.Spec{
		Slug:    "test-spec",
		Intent:  "test",
		Stage:   "spark",
		Version: 1,
		Notes:   "Some context notes",
	}
	got := Spec(s)
	if !strings.Contains(got, "## Notes") {
		t.Error("missing notes section")
	}
	if !strings.Contains(got, "Some context notes") {
		t.Error("missing notes content")
	}
}

func TestSpecNil(t *testing.T) {
	got := Spec(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestSpecList(t *testing.T) {
	specs := []*specv1.Spec{
		{Slug: "login-api", Stage: "specify", Priority: "p1", Intent: "OAuth2 login"},
		{Slug: "webhook", Stage: "shape", Priority: "p2", Intent: "Webhooks"},
	}
	got := SpecList(specs)
	if !strings.Contains(got, "| Slug | Stage | Priority | Intent |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| login-api | specify | p1 | OAuth2 login |") {
		t.Error("missing first row")
	}
}

func TestSpecListEmpty(t *testing.T) {
	got := SpecList(nil)
	if !strings.Contains(got, "No specs found.") {
		t.Error("expected empty message")
	}
}
