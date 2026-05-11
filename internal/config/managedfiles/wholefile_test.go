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

// TestWholeFileForceRestoresCanonical covers the StateDrifted +
// Force=true, KeepEdits=false path. Seeds a file whose body is
// user-edited (sentinel hash != disk hash); after --force, the file
// must match canonical content with a fresh canonical sentinel.
func TestWholeFileForceRestoresCanonical(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// First sync produces a canonical v=2 file.
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// User edits the body — sentinel hash no longer matches.
	disk, _ := os.ReadFile(full)
	firstLine := strings.SplitN(string(disk), "\n", 2)[0]
	corrupted := []byte(firstLine + "\n// USER EDIT\n")
	if err := os.WriteFile(full, corrupted, 0o600); err != nil { //nolint:gosec // full is filepath.Join(t.TempDir(), ...)
		t.Fatal(err)
	}
	// --force without --keep-edits must restore canonical.
	res, _ := s.Sync(dir, mf, params, SyncOptions{Force: true})
	if res.Action != ActionForced {
		t.Errorf("action = %v, want ActionForced", res.Action)
	}
	canonical, _ := readSource(mf)
	canonHash := hashBytes(canonical)
	after, _ := os.ReadFile(full)
	if strings.Contains(string(after), "USER EDIT") {
		t.Error("--force without --keep-edits preserved user edits (should have restored canonical)")
	}
	if !strings.Contains(string(after), "sha256="+canonHash) {
		t.Errorf("restored file missing canonical hash; got:\n%s", after)
	}
}

// TestWholeFileForceKeepEditsOnNoSentinelPreservesAllContent covers
// the keep-edits path when the existing file has no sentinel at all
// (e.g., a user-authored file the framework hasn't claimed yet). The
// fix is that we MUST NOT strip the first line in this case — it's
// user content, not a sentinel. Stripping would silently drop the
// first line of the user's file.
func TestWholeFileForceKeepEditsOnNoSentinelPreservesAllContent(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// User-authored file, no sentinel — every line is user content.
	userContent := "// Line 1 — user wrote this\n// Line 2 — also user\n// Line 3 — and this\n"
	if err := os.WriteFile(full, []byte(userContent), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, _ := s.Sync(dir, mf, params, SyncOptions{Force: true, KeepEdits: true})
	if res.Action != ActionForced {
		t.Errorf("action = %v, want ActionForced", res.Action)
	}
	after, _ := os.ReadFile(full)
	// All three user lines must survive — stripping the first line
	// would have dropped "Line 1".
	for _, want := range []string{"Line 1", "Line 2", "Line 3"} {
		if !strings.Contains(string(after), want) {
			t.Errorf("--force --keep-edits on no-sentinel file dropped %q; got:\n%s", want, after)
		}
	}
}

// TestWholeFileForceKeepEditsPreservesUserBody covers the
// StateDrifted + Force=true, KeepEdits=true path. The user's body
// content survives; the sentinel hash is refreshed to match.
func TestWholeFileForceKeepEditsPreservesUserBody(t *testing.T) {
	dir := t.TempDir()
	mf := testWholeFileMF()
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	full := filepath.Join(dir, ".specgraph/agents/opencode/specgraph.ts")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Replace the body with user content (keep the old sentinel —
	// makes the file Drifted).
	disk, _ := os.ReadFile(full)
	firstLine := strings.SplitN(string(disk), "\n", 2)[0]
	userBody := "// USER CONTENT KEPT\n"
	corrupted := []byte(firstLine + "\n" + userBody)
	if err := os.WriteFile(full, corrupted, 0o600); err != nil { //nolint:gosec // full is filepath.Join(t.TempDir(), ...)
		t.Fatal(err)
	}
	res, _ := s.Sync(dir, mf, params, SyncOptions{Force: true, KeepEdits: true})
	if res.Action != ActionForced {
		t.Errorf("action = %v, want ActionForced", res.Action)
	}
	after, _ := os.ReadFile(full)
	if !strings.Contains(string(after), "USER CONTENT KEPT") {
		t.Errorf("--force --keep-edits dropped user body; got:\n%s", after)
	}
	// Re-inspect: state is Stale (disk diverges from canonical), but
	// the sentinel hash now matches the disk hash — confirming
	// keep-edits refreshed the sentinel over the user body. Future
	// inits will still see Stale on every inspect, which is the
	// intended UX (user knows their file diverges from canonical;
	// re-applying --keep-edits or accepting --force will rewrite).
	state, _ := s.Inspect(dir, mf, params)
	if state.State != StateStale {
		t.Errorf("after force+keep-edits, state = %v, want StateStale (diverges from canonical)", state.State)
	}
	if state.SentinelHash != state.DiskHash {
		t.Errorf("sentinel hash %q != disk hash %q; keep-edits should have refreshed sentinel", state.SentinelHash, state.DiskHash)
	}
}
