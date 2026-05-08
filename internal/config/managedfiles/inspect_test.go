// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
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
	_, err := Inspect(dir, mf)
	if !errors.Is(err, ErrSymlinkRejected) {
		t.Errorf("expected ErrSymlinkRejected, got %v", err)
	}
}

// TestInspect_FileExistsHappyPath exercises Inspect's main body: file
// reads successfully, disk hash is computed, an embedded source miss is
// tolerated (canonical hash stays empty), and the placeholder StateSynced
// is returned. PR A's empty manifest means this path is never reached
// end-to-end via InspectAll, but the unit test pins the contract for PRs
// B+ which will replace the placeholder with strategy-specific logic.
func TestInspect_FileExistsHappyPath(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, ".specgraph", "agents", "opencode")
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(subdir, "specgraph.ts")
	const body = "// hello\nconst x = 1\n"
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	mf := ManagedFile{
		Path:     ".specgraph/agents/opencode/specgraph.ts",
		Strategy: StrategyWholeFile,
		Source:   "opencode/specgraph.ts", // PR A's embed.FS is empty, so this misses
		Comment:  CommentSlash,
	}
	got, err := Inspect(dir, mf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != mf.Path {
		t.Errorf("Path = %q, want %q", got.Path, mf.Path)
	}
	if got.DiskHash == "" {
		t.Error("DiskHash should be non-empty for an existing file")
	}
	if got.EmbeddedHash != "" {
		t.Errorf("EmbeddedHash should be empty (no canonical embedded in PR A), got %q", got.EmbeddedHash)
	}
	// PR A returns StateSynced as a placeholder; PR B replaces with per-strategy logic.
	if got.State != StateSynced {
		t.Errorf("State = %v, want StateSynced (PR A placeholder)", got.State)
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
	_, err := InspectAll(missing, []Harness{HarnessOpenCode})
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
	_, err := InspectAll(link, []Harness{HarnessOpenCode})
	if !errors.Is(err, ErrSymlinkRejected) {
		t.Errorf("expected ErrSymlinkRejected, got %v", err)
	}
}
