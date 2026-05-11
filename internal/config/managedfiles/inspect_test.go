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
// strategyImpl and propagates errNotImplemented from the remaining stub
// (WholeFile). JSONKeyMerge and MarkdownBlock are implemented in PR B.
func TestInspect_DispatchesToStrategy(t *testing.T) {
	dir := t.TempDir()
	for _, s := range []Strategy{StrategyWholeFile} {
		mf := ManagedFile{
			Path:     ".specgraph/agents/opencode/nope.ts",
			Strategy: s,
			Comment:  CommentSlash,
		}
		_, err := Inspect(dir, mf, ProjectParams{})
		if !errors.Is(err, errNotImplemented) {
			t.Errorf("Strategy %d: Inspect should propagate errNotImplemented, got %v", s, err)
		}
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
