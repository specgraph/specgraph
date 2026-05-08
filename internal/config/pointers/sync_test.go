// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func defaultOpts() Options {
	return Options{
		ServerURL:   "http://127.0.0.1:7890",
		ProjectSlug: "specgraph",
	}
}

func TestSync_CreatesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	report := Sync(dir, defaultOpts())
	r := report.Agents
	if r.Path != "AGENTS.md" {
		t.Errorf("report.Agents.Path = %q, want AGENTS.md", r.Path)
	}
	if r.Action != ActionCreated {
		t.Errorf("report.Agents.Action = %q, want %q", r.Action, ActionCreated)
	}
	if r.Err != nil {
		t.Errorf("report.Agents.Err = %v, want nil", r.Err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	bs := string(body)
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("AGENTS.md missing start marker:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:end -->") {
		t.Errorf("AGENTS.md missing end marker:\n%s", bs)
	}
	if !strings.Contains(bs, "http://127.0.0.1:7890") {
		t.Errorf("AGENTS.md missing serverURL:\n%s", bs)
	}
	if !strings.HasSuffix(bs, "\n") {
		t.Errorf("AGENTS.md must end with newline; got last bytes %q", bs[max(0, len(bs)-5):])
	}
}

func TestSync_NoOpWhenIdentical(t *testing.T) {
	dir := t.TempDir()
	if r := Sync(dir, defaultOpts()).Agents; r.Action != ActionCreated {
		t.Fatalf("first run: Action = %q, want %q", r.Action, ActionCreated)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionNoOp {
		t.Errorf("second run: Action = %q, want %q", r.Action, ActionNoOp)
	}
}

func TestSync_UpdatesWhenContentDiffers(t *testing.T) {
	dir := t.TempDir()
	Sync(dir, defaultOpts())
	r := Sync(dir, Options{ServerURL: "http://example.com:8080", ProjectSlug: "specgraph"}).Agents
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(body), "http://example.com:8080") {
		t.Errorf("AGENTS.md does not reflect new serverURL:\n%s", body)
	}
}

func TestSync_PreservesUserContentAroundBlock(t *testing.T) {
	dir := t.TempDir()
	const userTop = "# My project\n\nUser preamble.\n\n"
	const userBottom = "\n## My footer\n\nUser tail.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(userTop+userBottom), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.Contains(bs, "User preamble.") || !strings.Contains(bs, "User tail.") {
		t.Errorf("user content not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}

func TestSync_OverwritesUserContentInsideBlock(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nUSER NOTES THAT MUST DISAPPEAR\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	Sync(dir, defaultOpts())
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "USER NOTES THAT MUST DISAPPEAR") {
		t.Errorf("inside-block user content was not overwritten:\n%s", body)
	}
}

func TestSync_AppendsBlockToFileWithoutMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User AGENTS\n\nbody.\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.HasPrefix(bs, "# User AGENTS\n\nbody.\n") {
		t.Errorf("user content not preserved at top:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}

func TestSync_PurgesLegacyInjectBlocks_SimpleSlugs(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:foo:start -->\ndigest A\n<!-- specgraph:foo:end -->\n" +
		"<!-- specgraph:bar-baz:start -->\ndigest B\n<!-- specgraph:bar-baz:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.LegacyBlocksPurged != 2 {
		t.Errorf("LegacyBlocksPurged = %d, want 2", r.LegacyBlocksPurged)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "specgraph:foo:") || strings.Contains(string(body), "specgraph:bar-baz:") {
		t.Errorf("legacy markers still present:\n%s", body)
	}
}

func TestSync_PurgesLegacyInjectBlocks_RealisticSlugs(t *testing.T) {
	dir := t.TempDir()
	// inject's safeSlugPattern allows uppercase, dots, underscores.
	seed := "<!-- specgraph:MySpec.v2:start -->\nA\n<!-- specgraph:MySpec.v2:end -->\n" +
		"<!-- specgraph:my_spec:start -->\nB\n<!-- specgraph:my_spec:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.LegacyBlocksPurged != 2 {
		t.Errorf("LegacyBlocksPurged = %d, want 2", r.LegacyBlocksPurged)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "MySpec.v2") || strings.Contains(string(body), "my_spec") {
		t.Errorf("realistic-slug legacy markers still present:\n%s", body)
	}
}

func TestSync_LegacyMarkerWithInvalidSlugNotPurged(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:has space:start -->\nbody\n<!-- specgraph:has space:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.LegacyBlocksPurged != 0 {
		t.Errorf("LegacyBlocksPurged = %d, want 0", r.LegacyBlocksPurged)
	}
}

func TestSync_DoesNotPurgeInitMarker(t *testing.T) {
	dir := t.TempDir()
	// First create canonical state.
	Sync(dir, defaultOpts())
	// Run again; init block must persist (NoOp).
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionNoOp {
		t.Errorf("Action = %q, want %q", r.Action, ActionNoOp)
	}
	if r.LegacyBlocksPurged != 0 {
		t.Errorf("LegacyBlocksPurged = %d, want 0; the init block must not be matched by the legacy regex", r.LegacyBlocksPurged)
	}
}

func TestSync_LegacyShapedInitMarkerIsCorruption(t *testing.T) {
	dir := t.TempDir()
	// init marker WITHOUT v=1 — corruption rule #4 must fire BEFORE legacy purge.
	seed := "<!-- specgraph:init:start -->\nbody\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "v=1") {
		t.Errorf("Err = %v, want a v=1 error", r.Err)
	}
}

func TestSync_RejectsCorruptedMarkers_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:end -->\n<!-- specgraph:init:start v=1 -->\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_StartWithoutEnd(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_DoubleStart(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:start v=1 -->\nmore\n<!-- specgraph:init:end -->\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsInitMarkerWithoutVersion(t *testing.T) {
	// Same shape as TestSync_LegacyShapedInitMarkerIsCorruption but without
	// the matching end marker — guards rule 4 in isolation.
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start -->\nbody\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "v=1") {
		t.Errorf("Err = %v, want a v=1 error", r.Err)
	}
}

func TestSync_RejectsSymlinkInPath(t *testing.T) {
	dir := t.TempDir()
	// Replace the project dir's AGENTS.md path with a symlink chain by
	// putting AGENTS.md behind a symlinked subdir. We have to seat the
	// symlink at projectDir level since AGENTS.md is at the root; instead
	// symlink projectDir itself so rejectSymlinkComponents triggers when
	// joining its name.
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlink unsupported on this filesystem: %v", err)
	}
	r := Sync(link, defaultOpts()).Agents
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_CreatesCursorRule(t *testing.T) {
	dir := t.TempDir()
	r := Sync(dir, defaultOpts()).Cursor
	if r.Path != cursorRel {
		t.Errorf("Path = %q, want %q", r.Path, cursorRel)
	}
	if r.Action != ActionCreated {
		t.Errorf("Action = %q, want %q", r.Action, ActionCreated)
	}
	body, err := os.ReadFile(filepath.Join(dir, cursorRel))
	if err != nil {
		t.Fatalf("read %s: %v", cursorRel, err)
	}
	bs := string(body)
	if !strings.HasPrefix(bs, "---\n") {
		t.Errorf("missing frontmatter header:\n%s", bs)
	}
	if !strings.Contains(bs, "alwaysApply: true") {
		t.Errorf("alwaysApply: true not in frontmatter:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing in body:\n%s", bs)
	}
}

func TestSync_RefusesCursorRuleWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, cursorRel), []byte("# bare\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Cursor
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_PreservesCursorRuleFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const userFM = "---\ndescription: my custom desc\nalwaysApply: false\nextraField: kept\n---\n\n"
	const userBlock = "<!-- specgraph:init:start v=1 -->\nstale\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, cursorRel), []byte(userFM+userBlock), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Cursor
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, cursorRel))
	bs := string(body)
	if !strings.Contains(bs, "description: my custom desc") {
		t.Errorf("user description not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "alwaysApply: false") {
		t.Errorf("user alwaysApply override not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "extraField: kept") {
		t.Errorf("user extra field not preserved:\n%s", bs)
	}
}

func TestSync_CursorBodyCorruptionErrorMentionsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const fm = "---\ndescription: x\n---\n\n"
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n" // missing end
	if err := os.WriteFile(filepath.Join(dir, cursorRel), []byte(fm+seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts()).Cursor
	if r.Action != ActionError {
		t.Fatalf("Action = %q, want %q", r.Action, ActionError)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "specgraph-bootstrap.md") {
		t.Errorf("expected error to mention bootstrap path; got %v", r.Err)
	}
}

func TestSync_FailureOnOneFileDoesNotAbortOther(t *testing.T) {
	dir := t.TempDir()
	// Corrupt AGENTS.md so it errors; cursor file is fresh and should succeed.
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n" // missing end
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionError {
		t.Errorf("report.Agents (AGENTS.md): Action = %q, want %q", report.Agents.Action, ActionError)
	}
	if report.Cursor.Action != ActionCreated {
		t.Errorf("report.Cursor (cursor): Action = %q, want %q", report.Cursor.Action, ActionCreated)
	}
}

func TestSync_AtomicWriteOnFailure(t *testing.T) {
	dir := t.TempDir()
	// First write a baseline.
	Sync(dir, defaultOpts())
	original, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	// Make the project root read-only so atomicWrite's MkdirAll/Rename fails.
	if err := os.Chmod(dir, 0o555); err != nil { //nolint:gosec // test-only readonly
		t.Skipf("chmod restricted on this fs: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // test-only restore
	// Trigger an update by changing the serverURL.
	r := Sync(dir, Options{ServerURL: "http://example.com:9999", ProjectSlug: "specgraph"}).Agents
	if r.Action != ActionError {
		// The MkdirAll on the parent might succeed if it already exists; on
		// some filesystems chmod 0o555 still allows existing-file writes via
		// rename. Skip rather than false-fail.
		t.Skipf("filesystem permitted write under 0o555 dir; cannot exercise atomic-rename failure: action = %q", r.Action)
	}
	cur, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Equal(original, cur) {
		t.Errorf("AGENTS.md modified despite write failure")
	}
}

func TestSync_ConcurrentInvocations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("acquireFileLock is a no-op on Windows; cross-process concurrency is best-effort there")
	}
	dir := t.TempDir()
	const N = 4
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			Sync(dir, defaultOpts())
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	// Exactly one start marker and one end marker.
	if c := strings.Count(bs, initStart); c != 1 {
		t.Errorf("init start marker count = %d, want 1; concurrent runs interleaved\n%s", c, bs)
	}
	if c := strings.Count(bs, initEnd); c != 1 {
		t.Errorf("init end marker count = %d, want 1; concurrent runs interleaved\n%s", c, bs)
	}
}

func TestSync_PreservesExistingFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	dir := t.TempDir()
	full := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(full, []byte("# user content\n"), 0o644); err != nil { //nolint:gosec // intentional 0644 to verify mode preservation
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(full, 0o644); err != nil { //nolint:gosec // intentional 0644 to verify mode preservation
		t.Fatalf("chmod: %v", err)
	}

	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionUpdated {
		t.Fatalf("report.Agents.Action = %v, want %v (err=%v)", report.Agents.Action, ActionUpdated, report.Agents.Err)
	}

	info, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode after Sync = %o, want 0644 (existing mode preserved)", got)
	}
}

func TestNewOptions_RejectsBadServerURL(t *testing.T) {
	cases := []struct {
		name, url string
	}{
		{"empty", ""},
		{"relative", "/api"},
		{"hostname only", "example.com"},
		{"host:port no scheme", "localhost:3000"},
		{"non-http scheme", "ftp://example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewOptions(c.url, "specgraph")
			if err == nil {
				t.Fatalf("NewOptions(%q) returned nil error; want validation failure", c.url)
			}
		})
	}
}

func TestNewOptions_RejectsBadSlug(t *testing.T) {
	cases := []struct {
		name, slug string
	}{
		{"empty", ""},
		{"leading dot", ".secret"},
		{"contains slash", "foo/bar"},
		{"contains newline", "foo\nbar"},
		{"contains marker tail", "foo --> <!-- specgraph:init:end -->"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewOptions("http://127.0.0.1:7890", c.slug)
			if err == nil {
				t.Fatalf("NewOptions(slug=%q) returned nil error", c.slug)
			}
		})
	}
}

func TestNewOptions_AcceptsValidInputs(t *testing.T) {
	opts, err := NewOptions("https://specgraph.example.com:443", "my_proj.v2")
	if err != nil {
		t.Fatalf("NewOptions: %v", err)
	}
	if opts.ServerURL != "https://specgraph.example.com:443" {
		t.Errorf("ServerURL round-trip failed: got %q", opts.ServerURL)
	}
	if opts.ProjectSlug != "my_proj.v2" {
		t.Errorf("ProjectSlug round-trip failed: got %q", opts.ProjectSlug)
	}
}

// Compile-time guard: ensure errors import is used (used in later tasks).
var _ = errors.New

func TestSync_ReturnsSyncReportStruct(t *testing.T) {
	dir := t.TempDir()
	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionCreated {
		t.Errorf("report.Agents.Action = %v, want %v", report.Agents.Action, ActionCreated)
	}
	if report.Cursor.Action != ActionCreated {
		t.Errorf("report.Cursor.Action = %v, want %v", report.Cursor.Action, ActionCreated)
	}
}

func TestSyncResult_ActionErrorImpliesNonNilErr(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/nonexistent", filepath.Join(dir, "link-to-nowhere")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	report := Sync(filepath.Join(dir, "link-to-nowhere"), defaultOpts())
	if report.Agents.Action != ActionError {
		t.Fatalf("Action = %v, want ActionError", report.Agents.Action)
	}
	if report.Agents.Err == nil {
		t.Errorf("ActionError but Err == nil — invariant broken")
	}
}

func TestSyncResult_NonErrorImpliesNilErr(t *testing.T) {
	dir := t.TempDir()
	report := Sync(dir, defaultOpts())
	if report.Agents.Err != nil {
		t.Errorf("Agents.Err = %v on success path", report.Agents.Err)
	}
	if report.Cursor.Err != nil {
		t.Errorf("Cursor.Err = %v on success path", report.Cursor.Err)
	}
}
