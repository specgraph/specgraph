// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPublishStateString(t *testing.T) {
	tests := []struct {
		state specv1.PublishState
		want  string
	}{
		{specv1.PublishState_PUBLISH_STATE_SYNCED, "synced"},
		{specv1.PublishState_PUBLISH_STATE_DRAFT, "draft"},
		{specv1.PublishState_PUBLISH_STATE_ERROR, "error"},
		{specv1.PublishState_PUBLISH_STATE_UNPUBLISHED, "unpublished"},
		{specv1.PublishState_PUBLISH_STATE_UNSPECIFIED, "-"},
		// Use a numeric cast for an unknown value outside the defined enum range.
		{specv1.PublishState(999), "-"},
	}
	for _, tt := range tests {
		got := publishStateString(tt.state)
		if got != tt.want {
			t.Errorf("publishStateString(%v) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestRenderPublishStatusEmpty(t *testing.T) {
	got := renderPublishStatus(nil)
	if got != "No published specs.\n" {
		t.Errorf("renderPublishStatus(nil) = %q, want 'No published specs.'", got)
	}

	got = renderPublishStatus([]*specv1.PublishStatusEntry{})
	if got != "No published specs.\n" {
		t.Errorf("renderPublishStatus([]) = %q, want 'No published specs.'", got)
	}
}

func TestRenderPublishStatusSingleEntry(t *testing.T) {
	ts := timestamppb.Now()
	entries := []*specv1.PublishStatusEntry{
		{
			SpecSlug: "my-prd",
			Prd: &specv1.PageMapping{
				State: specv1.PublishState_PUBLISH_STATE_SYNCED,
			},
			Sdd: &specv1.PageMapping{
				State: specv1.PublishState_PUBLISH_STATE_DRAFT,
			},
			Adrs:        []*specv1.PageMapping{},
			LastSync:    ts,
			NewComments: 3,
		},
	}
	got := renderPublishStatus(entries)
	if !strings.Contains(got, "my-prd") {
		t.Errorf("renderPublishStatus: missing slug, got %q", got)
	}
	if !strings.Contains(got, "synced") {
		t.Errorf("renderPublishStatus: missing PRD state 'synced', got %q", got)
	}
	if !strings.Contains(got, "draft") {
		t.Errorf("renderPublishStatus: missing SDD state 'draft', got %q", got)
	}
	if !strings.Contains(got, "3 new") {
		t.Errorf("renderPublishStatus: missing comment count '3 new', got %q", got)
	}
}

func TestRenderPublishStatusMultipleEntries(t *testing.T) {
	entries := []*specv1.PublishStatusEntry{
		{
			SpecSlug: "spec-alpha",
			Prd: &specv1.PageMapping{
				State: specv1.PublishState_PUBLISH_STATE_SYNCED,
			},
			Adrs: []*specv1.PageMapping{
				{State: specv1.PublishState_PUBLISH_STATE_SYNCED},
				{State: specv1.PublishState_PUBLISH_STATE_SYNCED},
			},
			NewComments: 0,
		},
		{
			SpecSlug:    "spec-beta",
			NewComments: 1,
		},
	}
	got := renderPublishStatus(entries)
	if !strings.Contains(got, "spec-alpha") {
		t.Errorf("missing spec-alpha in output: %q", got)
	}
	if !strings.Contains(got, "spec-beta") {
		t.Errorf("missing spec-beta in output: %q", got)
	}
	// ADRs count for spec-alpha — verify it's in a table cell, not just any '2'.
	if !strings.Contains(got, "| 2 |") {
		t.Errorf("missing ADR count '| 2 |' in table output: %q", got)
	}
}

func TestRenderPublishStatusNilPRDAndSDD(t *testing.T) {
	// Entry where PRD and SDD are nil — should show "-" for each.
	entries := []*specv1.PublishStatusEntry{
		{
			SpecSlug:    "bare-spec",
			NewComments: 0,
		},
	}
	got := renderPublishStatus(entries)
	if !strings.Contains(got, "bare-spec") {
		t.Errorf("missing bare-spec in output: %q", got)
	}
	// Both PRD and SDD show "-" as table cells.
	if !strings.Contains(got, "| - |") {
		t.Errorf("expected '| - |' for nil PRD/SDD table cells, got: %q", got)
	}
}

func TestRenderPublishStatusNoLastSync(t *testing.T) {
	entries := []*specv1.PublishStatusEntry{
		{
			SpecSlug:    "no-sync",
			LastSync:    nil,
			NewComments: 0,
		},
	}
	got := renderPublishStatus(entries)
	// When LastSync is nil the lastSync column should show "-" as a table cell.
	if !strings.Contains(got, "| - |") {
		t.Errorf("expected '| - |' for nil LastSync table cell, got: %q", got)
	}
}
