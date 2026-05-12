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

const testMdcPath = ".cursor/rules/test-rule.mdc"

func testMdcCanonical() []byte {
	return []byte("---\ndescription: test rule\nalwaysApply: false\n---\n\n# Test Rule\n\nBody content here.\n")
}

func testMdcMF() ManagedFile {
	return ManagedFile{
		Path:           testMdcPath,
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/test-rule.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
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
	canonical, srcErr := readSource(mf)
	if srcErr != nil {
		t.Fatalf("readSource: %v", srcErr)
	}
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
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read refreshed file: %v", rerr)
	}
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
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read initial file: %v", rerr)
	}
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	corrupted := []byte(firstLine + "\n" + "USER EDITED BODY\n")
	if err := os.WriteFile(full, corrupted, 0o600); err != nil { //nolint:gosec // full is constructed from t.TempDir() + constant path; no taint
		t.Fatal(err)
	}
	res, _ := s.Sync(dir, mf, params, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (drifted)", res.Action)
	}
	after, aerr := os.ReadFile(full)
	if aerr != nil {
		t.Fatalf("read post-skip file: %v", aerr)
	}
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
	info, serr := os.Stat(full)
	if serr != nil {
		t.Fatalf("stat full: %v", serr)
	}
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
	disk, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read initial file: %v", rerr)
	}
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
	canonical, srcErr := readSource(mf)
	if srcErr != nil {
		t.Fatalf("readSource: %v", srcErr)
	}
	canonHash := hashBytes(canonical)
	after, aerr := os.ReadFile(full)
	if aerr != nil {
		t.Fatalf("read post-force file: %v", aerr)
	}
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
	after, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read post-keep-edits file: %v", rerr)
	}
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
	disk, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read initial file: %v", rerr)
	}
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
	after, aerr := os.ReadFile(full)
	if aerr != nil {
		t.Fatalf("read post-keep-edits file: %v", aerr)
	}
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

func TestWholeFileMdc_Missing_WritesSentinelAfterFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Fatalf("action = %v, want ActionCreated", res.Action)
	}
	full := filepath.Join(dir, testMdcPath)
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read written file: %v", rerr)
	}
	got := string(data)
	if !strings.HasPrefix(got, "---\ndescription: test rule\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=") {
		t.Errorf("file does not start with frontmatter+sentinel:\n%s", got)
	}
	if !strings.Contains(got, "# Test Rule\n\nBody content here.\n") {
		t.Errorf("body not preserved:\n%s", got)
	}
}

func TestWholeFileMdc_Synced_NoOpOnSecondSync(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("second sync action = %v, want ActionNoOp", res.Action)
	}
}

func TestWholeFileMdc_DriftedOnEditedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(dir, testMdcPath)
	data, _ := os.ReadFile(full)
	edited := strings.Replace(string(data), "alwaysApply: false", "alwaysApply: true", 1)
	if err := os.WriteFile(full, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("action on edited frontmatter = %v, want ActionSkipped", res.Action)
	}
	if !strings.Contains(res.Detail, "sentinel hash") {
		t.Errorf("Detail should indicate sentinel-hash mismatch; got %q", res.Detail)
	}
}

func TestWholeFileMdc_DriftedWhenFrontmatterMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	full := filepath.Join(dir, testMdcPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("just body content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("action on broken frontmatter = %v, want ActionSkipped", res.Action)
	}
	if !strings.Contains(res.Detail, "frontmatter") {
		t.Errorf("Detail should mention frontmatter; got %q", res.Detail)
	}
}

func TestWholeFileMdc_StaleRefreshes(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	full := filepath.Join(dir, testMdcPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed: a v=2 sentinel whose hash matches the disk content's
	// HashExcludingSentinelAfterFrontmatter (front + body, sentinel stripped).
	// The hash inputs are the frontmatter bytes (including the trailing blank
	// line that splitFrontmatter consumes into `front`) concatenated with the
	// post-sentinel body. Disk hash matches sentinel hash, but neither matches
	// canonical hash — classifier returns Stale.
	frontBytes := []byte("---\ndescription: test rule\nalwaysApply: false\n---\n\n")
	postSentinelBody := []byte("# Stale Heading\n\nold body\n")
	staleHash := hashBytes(append(append([]byte{}, frontBytes...), postSentinelBody...))
	staleContent := []byte("---\ndescription: test rule\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=" + staleHash + " -->\n# Stale Heading\n\nold body\n")
	if err := os.WriteFile(full, staleContent, 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), "# Test Rule\n\nBody content here.\n") {
		t.Errorf("refresh did not restore canonical body:\n%s", got)
	}
}

// TestEmbeddedMdcCanonicalSplitsCleanly verifies that every embedded
// canonical for a HasFrontmatter==true entry has well-formed YAML
// frontmatter — splitFrontmatter must succeed and the post-frontmatter
// body must be non-empty. Locks the assumption that renderWholeFile
// never panics on canonical input at runtime.
func TestEmbeddedMdcCanonicalSplitsCleanly(t *testing.T) {
	// Cover the test fixture explicitly (it lives outside the manifest
	// since production wouldn't sync test-rule.mdc to a project).
	t.Run("testFixture", func(t *testing.T) {
		mf := testMdcMF()
		assertCanonicalSplitsCleanly(t, mf)
	})

	// Cover every production manifest entry with HasFrontmatter==true.
	for _, mf := range allManagedFiles() {
		if !mf.HasFrontmatter {
			continue
		}
		mf := mf
		t.Run(mf.Path, func(t *testing.T) {
			assertCanonicalSplitsCleanly(t, mf)
		})
	}
}

func assertCanonicalSplitsCleanly(t *testing.T, mf ManagedFile) {
	t.Helper()
	canonical, err := readSource(mf)
	if err != nil {
		t.Fatalf("readSource(%s): %v", mf.Path, err)
	}
	front, body, ferr := splitFrontmatter(canonical)
	if ferr != nil {
		t.Fatalf("splitFrontmatter(%s): %v", mf.Path, ferr)
	}
	if len(front) == 0 {
		t.Errorf("%s: empty frontmatter", mf.Path)
	}
	if len(body) == 0 {
		t.Errorf("%s: empty body after frontmatter", mf.Path)
	}
}

func TestWholeFileMdc_ForceKeepEdits_PreservesUserBody(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	// First sync: write canonical.
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(dir, testMdcPath)
	// User edits the body (everything after the sentinel).
	data, _ := os.ReadFile(full)
	edited := strings.Replace(string(data), "Body content here.", "USER REWROTE THIS.", 1)
	if err := os.WriteFile(full, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	// Force + KeepEdits: should preserve the user body and refresh the sentinel.
	res, err := s.Sync(dir, mf, params, SyncOptions{Force: true, KeepEdits: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionForced {
		t.Errorf("action = %v, want ActionForced", res.Action)
	}
	out, _ := os.ReadFile(full)
	if !strings.Contains(string(out), "USER REWROTE THIS.") {
		t.Errorf("KeepEdits dropped user edits:\n%s", out)
	}
	// Frontmatter must still be valid.
	if !strings.HasPrefix(string(out), "---\ndescription: test rule\n") {
		t.Errorf("KeepEdits broke frontmatter:\n%s", out)
	}
	// Sentinel must still be present on the first body line.
	if !strings.Contains(string(out), "<!-- specgraph:init v=2 sha256=") {
		t.Errorf("KeepEdits dropped sentinel:\n%s", out)
	}
}

func TestWholeFileMdc_ForceKeepEdits_BrokenFrontmatterSkips(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	full := filepath.Join(dir, testMdcPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("just body content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := wholeFileStrategy{}.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{Force: true, KeepEdits: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("action on broken frontmatter + KeepEdits = %v, want ActionSkipped", res.Action)
	}
	if !strings.Contains(res.Detail, "malformed frontmatter") {
		t.Errorf("Detail should mention malformed frontmatter; got %q", res.Detail)
	}
	_ = s
}

func TestWholeFileMdcSupersedes_DeletesVerbatim(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph.mdc",
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/specgraph.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
		SupersedesPath: ".cursor/rules/specgraph.md",
	}
	// Seed the old path with verbatim pre-rename bytes.
	oldFull := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if err := os.MkdirAll(filepath.Dir(oldFull), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFull, vestigialCursorSpecgraphMD, 0o600); err != nil {
		t.Fatal(err)
	}

	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	// .mdc exists.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph.mdc")); err != nil {
		t.Errorf("new .mdc missing: %v", err)
	}
	// .md was deleted.
	if _, err := os.Stat(oldFull); !os.IsNotExist(err) {
		t.Errorf(".md still present (stat err = %v)", err)
	}
}

func TestWholeFileMdcSupersedes_PreservesEditedAndAddsDetail(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph.mdc",
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/specgraph.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
		SupersedesPath: ".cursor/rules/specgraph.md",
	}
	oldFull := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if err := os.MkdirAll(filepath.Dir(oldFull), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed an edited variant — append a comment.
	edited := append([]byte{}, vestigialCursorSpecgraphMD...)
	edited = append(edited, []byte("\n<!-- user note -->\n")...)
	if err := os.WriteFile(oldFull, edited, 0o600); err != nil {
		t.Fatal(err)
	}

	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	// .md is preserved.
	if _, err := os.Stat(oldFull); err != nil {
		t.Errorf(".md should be preserved on edited variant: %v", err)
	}
	if !strings.Contains(res.Detail, `supersedes path ".cursor/rules/specgraph.md" left in place: prior-canonical mismatch`) {
		t.Errorf("Detail should mention prior-canonical mismatch; got %q", res.Detail)
	}
}

func TestWholeFileMdcSupersedes_NoOpStillCleansUpLateAppearingVerbatim(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph.mdc",
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/specgraph.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
		SupersedesPath: ".cursor/rules/specgraph.md",
	}
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}

	// First sync: writes the .mdc cleanly with no .md present.
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}

	// A user (or stale dotfiles backup) drops a verbatim .md back in
	// between syncs. The next sync sees the .mdc as already-Synced
	// (ActionNoOp), but supersedes cleanup should still fire.
	oldFull := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if err := os.WriteFile(oldFull, vestigialCursorSpecgraphMD, 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp (the .mdc itself is synced)", res.Action)
	}
	if _, sErr := os.Stat(oldFull); !os.IsNotExist(sErr) {
		t.Errorf("late-appearing verbatim .md should have been deleted (stat err = %v)", sErr)
	}
}
