// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package diff_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/diff"
)

func TestComputeHunks_NoChange(t *testing.T) {
	hunks := diff.ComputeHunks("hello world", "hello world")
	if len(hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}
	for _, h := range hunks {
		if h.Op != diff.OpEqual {
			t.Errorf("expected all hunks to be OpEqual, got %v", h.Op)
		}
	}
}

func TestComputeHunks_Insertion(t *testing.T) {
	hunks := diff.ComputeHunks("hello", "hello world")
	hasInsert := false
	for _, h := range hunks {
		if h.Op == diff.OpInsert {
			hasInsert = true
			break
		}
	}
	if !hasInsert {
		t.Error("expected an INSERT hunk")
	}
}

func TestComputeHunks_Deletion(t *testing.T) {
	hunks := diff.ComputeHunks("hello world", "hello")
	hasDelete := false
	for _, h := range hunks {
		if h.Op == diff.OpDelete {
			hasDelete = true
			break
		}
	}
	if !hasDelete {
		t.Error("expected a DELETE hunk")
	}
}

func TestComputeHunks_Replacement(t *testing.T) {
	hunks := diff.ComputeHunks("foo bar", "foo baz")
	hasDelete := false
	hasInsert := false
	for _, h := range hunks {
		if h.Op == diff.OpDelete {
			hasDelete = true
		}
		if h.Op == diff.OpInsert {
			hasInsert = true
		}
	}
	if !hasDelete {
		t.Error("expected a DELETE hunk for replacement")
	}
	if !hasInsert {
		t.Error("expected an INSERT hunk for replacement")
	}
}

func TestComputeHunks_EmptyOld(t *testing.T) {
	hunks := diff.ComputeHunks("", "hello")
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Op != diff.OpInsert {
		t.Errorf("expected OpInsert, got %v", hunks[0].Op)
	}
	if hunks[0].Text != "hello" {
		t.Errorf("expected text 'hello', got %q", hunks[0].Text)
	}
}

func TestComputeHunks_EmptyNew(t *testing.T) {
	hunks := diff.ComputeHunks("hello", "")
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Op != diff.OpDelete {
		t.Errorf("expected OpDelete, got %v", hunks[0].Op)
	}
	if hunks[0].Text != "hello" {
		t.Errorf("expected text 'hello', got %q", hunks[0].Text)
	}
}

func TestComputeHunks_BothEmpty(t *testing.T) {
	hunks := diff.ComputeHunks("", "")
	if hunks != nil {
		t.Errorf("expected nil for both-empty input, got %v", hunks)
	}
}

func TestFormatInline_DeletedAndInserted(t *testing.T) {
	hunks := []diff.Hunk{
		{Op: diff.OpEqual, Text: "foo "},
		{Op: diff.OpDelete, Text: "bar"},
		{Op: diff.OpInsert, Text: "baz"},
	}
	result := diff.FormatInline(hunks)
	expected := "foo [-bar-]{+baz+}"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatInline_NoChanges(t *testing.T) {
	hunks := []diff.Hunk{
		{Op: diff.OpEqual, Text: "just text"},
	}
	result := diff.FormatInline(hunks)
	if result != "just text" {
		t.Errorf("expected 'just text', got %q", result)
	}
}
