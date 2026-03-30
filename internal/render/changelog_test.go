// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestChanges_WithEntries(t *testing.T) {
	entries := []*specv1.ChangeLogEntry{
		{
			Version:     3,
			Stage:       "shape",
			ContentHash: "a1b2c3d4",
			Checkpoint:  true,
			Summary:     "Refined scope",
			Changes: []*specv1.FieldChange{
				{Field: "intent", OldValue: "Build X", NewValue: "Build X (v2)"},
			},
			Date: timestamppb.New(time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)),
		},
		{
			Version:     2,
			Stage:       "spark",
			ContentHash: "e5f6g7h8",
			Date:        timestamppb.New(time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)),
		},
	}

	out := render.Changes(entries)

	if !strings.Contains(out, "v3") {
		t.Error("missing version 3")
	}
	if !strings.Contains(out, "shape") {
		t.Error("missing stage")
	}
	if !strings.Contains(out, "checkpoint") {
		t.Error("missing checkpoint marker")
	}
	if !strings.Contains(out, "Refined scope") {
		t.Error("missing summary")
	}
	if !strings.Contains(out, "intent") {
		t.Error("missing field change")
	}
	if !strings.Contains(out, "## v2 — spark") {
		t.Error("missing version 2 entry header")
	}
}

func TestChanges_Empty(t *testing.T) {
	out := render.Changes(nil)
	if !strings.Contains(out, "No changelog entries") {
		t.Errorf("expected empty message, got %q", out)
	}
}
