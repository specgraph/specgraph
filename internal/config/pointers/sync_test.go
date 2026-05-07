// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"os"
	"path/filepath"
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
	results := Sync(dir, defaultOpts())
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	r := results[0]
	if r.Path != "AGENTS.md" {
		t.Errorf("results[0].Path = %q, want AGENTS.md", r.Path)
	}
	if r.Action != ActionCreated {
		t.Errorf("results[0].Action = %q, want %q", r.Action, ActionCreated)
	}
	if r.Err != nil {
		t.Errorf("results[0].Err = %v, want nil", r.Err)
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
	if r := Sync(dir, defaultOpts())[0]; r.Action != ActionCreated {
		t.Fatalf("first run: Action = %q, want %q", r.Action, ActionCreated)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionNoOp {
		t.Errorf("second run: Action = %q, want %q", r.Action, ActionNoOp)
	}
}

func TestSync_UpdatesWhenContentDiffers(t *testing.T) {
	dir := t.TempDir()
	Sync(dir, defaultOpts())
	r := Sync(dir, Options{ServerURL: "http://example.com:8080", ProjectSlug: "specgraph"})[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
	if r.LegacyBlocksPurged != 0 {
		t.Errorf("LegacyBlocksPurged = %d, want 0", r.LegacyBlocksPurged)
	}
}

func TestSync_DoesNotPurgeInitMarker(t *testing.T) {
	dir := t.TempDir()
	// First create canonical state.
	Sync(dir, defaultOpts())
	// Run again; init block must persist (NoOp).
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_StartWithoutEnd(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_DoubleStart(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:start v=1 -->\nmore\n<!-- specgraph:init:end -->\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(dir, defaultOpts())[0]
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
	r := Sync(link, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}
