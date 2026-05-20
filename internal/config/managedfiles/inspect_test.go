// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestInspect_DispatchesToStrategy verifies that Inspect dispatches through
// strategyImpl for WholeFile. PR C implements wholeFileStrategy so a missing
// file now returns StateMissing (not errNotImplemented).
func TestInspect_DispatchesToStrategy(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:     ".specgraph/agents/opencode/specgraph.ts",
		Strategy: StrategyWholeFile,
		Source:   "embedded/opencode/specgraph.ts",
		Comment:  CommentSlash,
		Harness:  HarnessOpenCode,
	}
	state, err := Inspect(dir, mf, ProjectParams{})
	if err != nil {
		t.Errorf("Inspect returned unexpected error: %v", err)
	}
	if state.State != StateMissing {
		t.Errorf("Inspect state = %v, want StateMissing", state.State)
	}
}

// TestInspectAll_NonExistentProjectDir rejects calls against missing
// project dirs; covers validateProjectDir's stat-failure branch.
//
// Uses a child path under t.TempDir() that we intentionally never create,
// so the missing-path is guaranteed-missing AND scoped to test-owned
// territory. Avoids relying on system-level path absence.
func TestInspectAll_NonExistentProjectDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "non-existent-child")
	_, err := InspectAll(missing, []Harness{HarnessOpenCode}, ProjectParams{})
	if err == nil {
		t.Error("expected error for non-existent project dir, got nil")
	}
}

// TestInspectAll_SymlinkProjectDir rejects a project dir that's itself
// a symlink; covers validateProjectDir's symlink-rejection branch.
func TestInspectAll_SymlinkProjectDir(t *testing.T) {
	parent := t.TempDir()
	realDir := filepath.Join(parent, "real")
	if err := os.Mkdir(realDir, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "linky")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skip("symlink creation failed (likely Windows)")
	}
	_, err := InspectAll(link, []Harness{HarnessOpenCode}, ProjectParams{})
	if !errors.Is(err, ErrSymlinkRejected) {
		t.Errorf("expected ErrSymlinkRejected, got %v", err)
	}
}

func TestInspectAll_PopulatesFileStateHarness(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: "test", ServerURL: "https://example.com/mcp/"}
	states, err := InspectAll(dir, []Harness{HarnessClaude}, params)
	if err != nil {
		t.Fatalf("InspectAll: %v", err)
	}
	if len(states) == 0 {
		t.Fatal("expected at least one FileState for HarnessClaude")
	}
	for _, s := range states {
		if s.Harness != HarnessClaude {
			t.Errorf("%s: Harness = %v, want HarnessClaude", s.Path, s.Harness)
		}
	}
}
