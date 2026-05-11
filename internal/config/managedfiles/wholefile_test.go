// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testWholeFilePath = ".specgraph/agents/opencode/specgraph.ts"

func testWholeFileMF() ManagedFile {
	return ManagedFile{
		Path:     testWholeFilePath,
		Strategy: StrategyWholeFile,
		Source:   "embedded/opencode/specgraph.ts",
		Comment:  CommentSlash,
		Harness:  HarnessOpenCode,
	}
}

func TestWholeFileMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read written file: %v", rerr)
	}
	if !strings.HasPrefix(string(data), "// specgraph:init v=2 sha256=") {
		t.Errorf("first line missing v=2 sentinel:\n%s", data)
	}
}

func TestWholeFileSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestWholeFileStale(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed with a v=2 sentinel that hashes the stale body (so disk
	// matches sentinel) but the body doesn't match canonical.
	canonical, _ := readSource(mf)
	canonHash := hashBytes(canonical)
	staleBody := []byte("// stale content not matching canonical\n")
	staleSentinelHash := hashBytes(staleBody)
	if canonHash == staleSentinelHash {
		t.Skip("synthetic stale body collided with canonical hash")
	}
	staleFile := []byte("// specgraph:init v=2 sha256=" + staleSentinelHash + "\n" + string(staleBody))
	if err := os.WriteFile(full, staleFile, 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	// File now matches canonical hash.
	data, _ := os.ReadFile(full)
	if !strings.Contains(string(data), "sha256="+canonHash) {
		t.Errorf("refreshed file missing canonical hash; got:\n%s", data)
	}
}

func TestWholeFileDriftedUserEdited(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// First write produces a v=2 file with the canonical hash.
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Now corrupt the body so the sentinel hash != actual body hash.
	data, _ := os.ReadFile(full)
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	corrupted := []byte(firstLine + "\n" + "USER EDITED BODY\n")
	if err := os.WriteFile(full, corrupted, 0o600); err != nil { //nolint:gosec // full is constructed from t.TempDir() + constant path; no taint
		t.Fatal(err)
	}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (drifted)", res.Action)
	}
	after, _ := os.ReadFile(full)
	if !strings.Contains(string(after), "USER EDITED BODY") {
		t.Error("drifted user content was overwritten")
	}
}

func TestWholeFileNoSentinel(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// user-authored file with no sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (no sentinel)", res.Action)
	}
	if res.Detail != "no sentinel" {
		t.Errorf("detail = %q, want \"no sentinel\"", res.Detail)
	}
}

func TestWholeFileCorruptedSentinel(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// specgraph:init v=99 sha256=abc\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionError {
		t.Errorf("action = %v, want ActionError (corrupted)", res.Action)
	}
	if !errors.Is(res.Err, ErrCorruptedSentinel) {
		t.Errorf("err = %v, want ErrCorruptedSentinel", res.Err)
	}
}

func TestWholeFileModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("// placeholder\n"), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(full)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}
