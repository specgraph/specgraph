// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// syncFixtures returns the three canonical-content map[string]any documents
// the Sync function should emit for slug=specgraph, serverURL=http://127.0.0.1:7890.
func syncFixtures() (cursor, claude, opencode map[string]any) {
	cursor = map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	claude = map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	opencode = map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     "http://127.0.0.1:7890/mcp/",
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	return
}

func TestSync_CreatesMissingFiles(t *testing.T) {
	dir := t.TempDir()
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")

	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	wantPaths := []string{".cursor/mcp.json", ".mcp.json", "opencode.json"}
	gotByPath := map[string]Action{}
	for _, r := range results {
		gotByPath[r.Path] = r.Action
	}
	for _, p := range wantPaths {
		if got := gotByPath[p]; got != ActionCreated {
			t.Errorf("%s: action = %q, want %q", p, got, ActionCreated)
		}
	}

	cursor, claude, opencode := syncFixtures()
	assertFileEquals(t, filepath.Join(dir, ".cursor/mcp.json"), cursor)
	assertFileEquals(t, filepath.Join(dir, ".mcp.json"), claude)
	assertFileEquals(t, filepath.Join(dir, "opencode.json"), opencode)
}

func TestSync_PreservesOtherServers_Cursor(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": {"CONTEXT7_API_KEY": "${env:CONTEXT7}"}
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := actionFor(results, ".cursor/mcp.json"); got != ActionUpdated {
		t.Errorf(".cursor/mcp.json action = %q, want %q", got, ActionUpdated)
	}

	got := readJSON(t, cursorPath)
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Error("context7 server was not preserved")
	}
	if _, ok := servers["specgraph"]; !ok {
		t.Error("specgraph server was not added")
	}
}

func TestSync_PreservesOtherServers_ClaudeCode(t *testing.T) {
	// Claude Code uses the same `mcpServers` (plural) top-level key as
	// Cursor — but it's a separate file with separate parsing, so an
	// off-by-one in claudeCodeConfig would silently nuke a user's other
	// Claude Code servers without this test.
	dir := t.TempDir()
	claudePath := filepath.Join(dir, ".mcp.json")
	existing := []byte(`{
  "mcpServers": {
    "filesystem": {
      "type": "http",
      "url": "https://example.com/fs"
    }
  }
}
`)
	if err := os.WriteFile(claudePath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := actionFor(results, ".mcp.json"); got != ActionUpdated {
		t.Errorf(".mcp.json action = %q, want %q", got, ActionUpdated)
	}

	got := readJSON(t, claudePath)
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["filesystem"]; !ok {
		t.Error("filesystem server was not preserved")
	}
	if _, ok := servers["specgraph"]; !ok {
		t.Error("specgraph server was not added")
	}
}

func TestSync_PreservesOtherServers_OpenCode(t *testing.T) {
	// OpenCode wraps servers under `mcp` (SINGULAR), not `mcpServers`. The
	// merge patch builder must target the correct top-level key; otherwise
	// it would silently nuke a user's other OpenCode servers, leak under
	// the wrong key, or both.
	dir := t.TempDir()
	opencodePath := filepath.Join(dir, "opencode.json")
	existing := []byte(`{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "context7": {
      "type": "remote",
      "url": "https://mcp.context7.com/mcp",
      "enabled": true
    }
  }
}
`)
	if err := os.WriteFile(opencodePath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if got := actionFor(results, "opencode.json"); got != ActionUpdated {
		t.Errorf("opencode.json action = %q, want %q", got, ActionUpdated)
	}

	got := readJSON(t, opencodePath)
	servers := got["mcp"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Error("context7 server was not preserved")
	}
	if _, ok := servers["specgraph"]; !ok {
		t.Error("specgraph server was not added")
	}
	// And the merge must NOT have also written the entry under the wrong
	// "mcpServers" plural key by accident.
	if _, ok := got["mcpServers"]; ok {
		t.Error("opencode.json gained an unwanted mcpServers (plural) key")
	}
}

func TestSync_UpdatesStaleSpecgraphEntry(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "specgraph": {
      "url": "http://old.host:1234/mcp/",
      "headers": {
        "Authorization": "Bearer stale",
        "X-Specgraph-Project": "old-slug"
      }
    },
    "atlassian": {
      "url": "https://mcp.atlassian.com",
      "headers": {"Authorization": "Bearer foo"}
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if _, err := Sync(dir, configs); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readJSON(t, cursorPath)
	servers := got["mcpServers"].(map[string]any)
	specgraph := servers["specgraph"].(map[string]any)
	if specgraph["url"] != "http://127.0.0.1:7890/mcp/" {
		t.Errorf("url not updated: %v", specgraph["url"])
	}
	headers := specgraph["headers"].(map[string]any)
	if headers["X-Specgraph-Project"] != "specgraph" {
		t.Errorf("project not updated: %v", headers["X-Specgraph-Project"])
	}
	if headers["Authorization"] != "Bearer ${env:SPECGRAPH_API_KEY}" {
		t.Errorf("auth not updated: %v", headers["Authorization"])
	}
	if _, ok := servers["atlassian"]; !ok {
		t.Error("atlassian server was not preserved")
	}
}

func TestSync_PreservesUserCustomizationsUnderSpecgraph(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	existing := []byte(`{
  "mcpServers": {
    "specgraph": {
      "url": "http://old.host:1234/mcp/",
      "headers": {
        "Authorization": "Bearer stale",
        "X-Specgraph-Project": "old-slug",
        "X-User-Custom": "preserve-me"
      },
      "comment": "my dev notes"
    }
  }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if _, err := Sync(dir, configs); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	got := readJSON(t, cursorPath)
	specgraph := got["mcpServers"].(map[string]any)["specgraph"].(map[string]any)
	if specgraph["comment"] != "my dev notes" {
		t.Errorf("user comment was not preserved: %v", specgraph["comment"])
	}
	headers := specgraph["headers"].(map[string]any)
	if headers["X-User-Custom"] != "preserve-me" {
		t.Errorf("user custom header was not preserved: %v", headers["X-User-Custom"])
	}
	// And managed fields are still updated to canonical values.
	if specgraph["url"] != "http://127.0.0.1:7890/mcp/" {
		t.Errorf("managed url was not updated: %v", specgraph["url"])
	}
	if headers["Authorization"] != "Bearer ${env:SPECGRAPH_API_KEY}" {
		t.Errorf("managed auth was not updated: %v", headers["Authorization"])
	}
	if headers["X-Specgraph-Project"] != "specgraph" {
		t.Errorf("managed project was not updated: %v", headers["X-Specgraph-Project"])
	}
}

func TestSync_RefusesOnOpencodeJSONCSibling(t *testing.T) {
	dir := t.TempDir()
	jsoncPath := filepath.Join(dir, "opencode.jsonc")
	if err := os.WriteFile(jsoncPath, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	wantSubstr := "opencode.jsonc"
	if got := err.Error(); !strings.Contains(got, wantSubstr) {
		t.Errorf("error %q does not contain %q", got, wantSubstr)
	}

	// Sync stops at OpenCode (the third config); the first two (Cursor,
	// Claude Code) should have completed before the error.
	gotPaths := make([]string, 0, len(results))
	for _, r := range results {
		gotPaths = append(gotPaths, r.Path)
	}
	wantPaths := []string{".cursor/mcp.json", ".mcp.json"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("partial results paths = %v, want %v", gotPaths, wantPaths)
	}
}

func TestSync_RefusesOnBrokenOpencodeJSONCSymlink(t *testing.T) {
	// A dangling opencode.jsonc symlink is still a directory entry the
	// user has set up; refusing on it prevents Sync from creating
	// opencode.json next to a broken sibling that the user (or another
	// tool) is presumably about to fix. Probing with os.Stat would
	// follow the link, see ErrNotExist, and write opencode.json — the
	// behavior os.Lstat fixes.
	dir := t.TempDir()
	jsoncPath := filepath.Join(dir, "opencode.jsonc")
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), jsoncPath); err != nil {
		t.Skipf("os.Symlink not supported here: %v", err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	_, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error for broken opencode.jsonc symlink, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "opencode.jsonc") {
		t.Errorf("error %q should mention opencode.jsonc", got)
	}
	// opencode.json must NOT have been written.
	if _, statErr := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(statErr) {
		t.Errorf("opencode.json should not exist when a (broken) jsonc sibling is present: %v", statErr)
	}
}

func TestSync_RefusesOnMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorPath, []byte(`{not valid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "parse") || !strings.Contains(got, ".cursor/mcp.json") {
		t.Errorf("error %q should mention parse failure for .cursor/mcp.json", got)
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty (Sync stopped at first config)", results)
	}

	// File should be untouched.
	got, err := os.ReadFile(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{not valid json` {
		t.Errorf("file was modified: %q", got)
	}
}

func TestSync_RefusesSymlinkAtTargetFile(t *testing.T) {
	// If .cursor/mcp.json is itself a symlink, follow-ing it on write
	// would overwrite whatever the symlink points to (potentially outside
	// projectDir). Sync must refuse before any FS write.
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"untouched":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cursorPath := filepath.Join(cursorDir, "mcp.json")
	if err := os.Symlink(outside, cursorPath); err != nil {
		t.Skipf("os.Symlink not supported here: %v", err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error for symlinked target, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "symlink") {
		t.Errorf("error %q should mention symlink refusal", got)
	}
	// Sync stopped at .cursor/mcp.json (the first config); no results.
	if len(results) != 0 {
		t.Errorf("results = %v, want empty", results)
	}
	// And the file outside projectDir is untouched.
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"untouched":true}` {
		t.Errorf("symlink target was modified: %q", got)
	}
}

func TestSync_RefusesSymlinkAtParentDir(t *testing.T) {
	// Same threat model as the file case, but the symlink is one
	// component up: .cursor/ -> /elsewhere. A naive os.MkdirAll on
	// .cursor/mcp.json would create files inside /elsewhere.
	dir := t.TempDir()
	elsewhere := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.Symlink(elsewhere, cursorDir); err != nil {
		t.Skipf("os.Symlink not supported here: %v", err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error for symlinked parent dir, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "symlink") {
		t.Errorf("error %q should mention symlink refusal", got)
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty", results)
	}
	// Nothing got written into the symlink target.
	entries, err := os.ReadDir(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("symlink target dir was written into: %v", entries)
	}
}

func TestSync_PartialResults_WhenLaterFileFails(t *testing.T) {
	// Pins the partial-results contract: when a later file fails for a
	// non-jsonc-sibling reason (here, malformed JSON in .mcp.json), the
	// already-synced earlier file (.cursor/mcp.json) must be reported in
	// results so callers know its state. Without this test, a refactor
	// could legitimately drop earlier results on any error and tests would
	// still pass against the only existing case (first-file failure).
	dir := t.TempDir()
	claudePath := filepath.Join(dir, ".mcp.json")
	if err := os.WriteFile(claudePath, []byte(`{not valid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results, err := Sync(dir, configs)
	if err == nil {
		t.Fatal("expected error from malformed .mcp.json, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "parse") || !strings.Contains(got, ".mcp.json") {
		t.Errorf("error %q should mention parse failure for .mcp.json", got)
	}

	// .cursor/mcp.json (first config) should be reported as created.
	if len(results) != 1 {
		t.Fatalf("results = %v, want exactly one entry for the cursor config", results)
	}
	if results[0].Path != ".cursor/mcp.json" {
		t.Errorf("results[0].Path = %q, want %q", results[0].Path, ".cursor/mcp.json")
	}
	if results[0].Action != ActionCreated {
		t.Errorf("results[0].Action = %q, want %q", results[0].Action, ActionCreated)
	}

	// Cursor config was actually written; OpenCode (third) was NOT touched.
	if _, statErr := os.Stat(filepath.Join(dir, ".cursor/mcp.json")); statErr != nil {
		t.Errorf(".cursor/mcp.json should exist (reported as created): %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(statErr) {
		t.Errorf("opencode.json should NOT exist (Sync stopped at .mcp.json): %v", statErr)
	}
}

func TestSync_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")

	// Run 1: all three files created.
	results1, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 1: %v", err)
	}
	for _, r := range results1 {
		if r.Action != ActionCreated {
			t.Errorf("run 1 %s: action = %q, want %q", r.Path, r.Action, ActionCreated)
		}
	}

	// Snapshot bytes after run 1.
	snapshots := map[string][]byte{}
	for _, c := range configs {
		data, readErr := os.ReadFile(filepath.Join(dir, c.Path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		snapshots[c.Path] = data
	}

	// Run 2: all three should be no-ops, file bytes unchanged.
	results2, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 2: %v", err)
	}
	for _, r := range results2 {
		if r.Action != ActionNoOp {
			t.Errorf("run 2 %s: action = %q, want %q", r.Path, r.Action, ActionNoOp)
		}
	}
	for _, c := range configs {
		got, readErr := os.ReadFile(filepath.Join(dir, c.Path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, snapshots[c.Path]) {
			t.Errorf("%s: file bytes changed between run 1 and run 2", c.Path)
		}
	}
}

func TestSync_Idempotent_ReformatsThenStable(t *testing.T) {
	// Existing file is valid JSON but not in canonical 2-space-indent form.
	// Run 1 should rewrite it (action "updated"); run 2 should be no-op.
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor/mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	// 4-space-indent variant of the canonical specgraph entry — semantically
	// equivalent, format different.
	existing := []byte(`{
    "mcpServers": {
        "specgraph": {
            "url": "http://127.0.0.1:7890/mcp/",
            "headers": {
                "Authorization": "Bearer ${env:SPECGRAPH_API_KEY}",
                "X-Specgraph-Project": "specgraph"
            }
        }
    }
}
`)
	if err := os.WriteFile(cursorPath, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	results1, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 1: %v", err)
	}
	if got := actionFor(results1, ".cursor/mcp.json"); got != ActionUpdated {
		t.Errorf("run 1 .cursor/mcp.json: action = %q, want %q (format normalization)", got, ActionUpdated)
	}

	results2, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 2: %v", err)
	}
	if got := actionFor(results2, ".cursor/mcp.json"); got != ActionNoOp {
		t.Errorf("run 2 .cursor/mcp.json: action = %q, want %q (already canonical)", got, ActionNoOp)
	}
}

// Helper functions used by sync tests.

func actionFor(results []SyncResult, path string) Action {
	for _, r := range results {
		if r.Path == path {
			return r.Action
		}
	}
	return ""
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func assertFileEquals(t *testing.T, path string, want map[string]any) {
	t.Helper()
	got := readJSON(t, path)
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", "  ")
		wj, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("%s mismatch.\n got: %s\nwant: %s", path, gj, wj)
	}
}
