// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspect_MissingFile(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:     ".specgraph/agents/opencode/nope.ts",
		Strategy: StrategyWholeFile,
		Source:   "opencode/specgraph.ts",
		Comment:  CommentSlash,
	}
	got, err := Inspect(dir, mf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.State != StateMissing {
		t.Errorf("State = %v, want StateMissing", got.State)
	}
}

func TestInspect_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link.ts")); err != nil {
		t.Skip("symlink creation failed (likely Windows without admin)")
	}
	mf := ManagedFile{
		Path:     "link.ts",
		Strategy: StrategyWholeFile,
		Comment:  CommentSlash,
	}
	if _, err := Inspect(dir, mf); err == nil {
		t.Error("expected ErrSymlinkRejected, got nil")
	}
}
