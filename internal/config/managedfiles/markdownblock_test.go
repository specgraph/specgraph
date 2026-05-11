// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testMDMF(path string, harness Harness) ManagedFile { //nolint:unparam // path may vary in future tests; keep the parameter explicit.
	return ManagedFile{
		Path:     path,
		Strategy: StrategyMarkdownBlock,
		Comment:  CommentHTML,
		Harness:  harness,
		Build: func(p ProjectParams) ([]byte, error) {
			return []byte("\n# block body for " + p.Slug + "\n"), nil
		},
	}
}

var testMDParams = ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}

func TestMarkdownBlockMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	res, err := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("specgraph:init:start v=2 sha256=")) {
		t.Errorf("v=2 marker missing in output:\n%s", got)
	}
}

func TestMarkdownBlockSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	// First sync creates.
	if _, err := s.Sync(dir, mf, testMDParams, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second sync no-op.
	res, _ := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestMarkdownBlockStaleV1Upgrade(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	// Seed with v=1 markers wrapping the same body the test Build would emit.
	body, _ := mf.Build(testMDParams)
	seed := []byte("<!-- specgraph:init:start v=1 -->" + string(body) + "<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)

	res, err := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed (v=1 upgrade)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("v=2 sha256=")) {
		t.Errorf("v=2 marker missing after upgrade:\n%s", got)
	}
}

func TestMarkdownBlockDriftedV1(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	// Seed with v=1 markers but mangled body (does NOT match what Build emits).
	seed := []byte("<!-- specgraph:init:start v=1 -->\nUSER EDIT — do not overwrite\n<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)

	res, _ := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (drifted, no --force)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("USER EDIT")) {
		t.Error("drifted user content was overwritten")
	}
}

func TestMarkdownBlockOutsideMarkerEditsPreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	if _, err := s.Sync(dir, mf, testMDParams, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// User appends prose after the block.
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	withProse := append(got, []byte("\nUser prose after the block.\n")...)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), withProse, 0o600) //nolint:gosec // test reads then rewrites under t.TempDir()

	// Re-sync; should still classify Synced (outside-block edits don't drift).
	res, _ := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp (outside-block edits ignored)", res.Action)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(after, []byte("User prose after the block.")) {
		t.Error("outside-block user content was destroyed")
	}
}

func TestMarkdownBlockNoMarkersAppends(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	// User-authored content; no specgraph markers.
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User content\n"), 0o600)
	res, _ := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated (block created, file existed)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("# User content")) {
		t.Error("user content was destroyed")
	}
	if !bytes.Contains(got, []byte("specgraph:init:start v=2")) {
		t.Error("init block not appended")
	}
}

func TestMarkdownBlockPurgesLegacy(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	seed := []byte("<!-- specgraph:foo:start -->\nold\n<!-- specgraph:foo:end -->\n# User text\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)
	res, _ := s.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Detail != "purged 1 legacy block" {
		t.Errorf("Detail = %q, want \"purged 1 legacy block\"", res.Detail)
	}
}

func TestMarkdownBlockModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	s := markdownBlockStrategy{}
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(""), 0o644) //nolint:gosec // test asserts mode preservation requires 0o644
	_, _ = s.Sync(dir, mf, testMDParams, SyncOptions{})
	info, _ := os.Stat(filepath.Join(dir, "AGENTS.md"))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}
