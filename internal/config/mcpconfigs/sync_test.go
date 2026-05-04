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
	gotByPath := map[string]string{}
	for _, r := range results {
		gotByPath[r.Path] = r.Action
	}
	for _, p := range wantPaths {
		if got := gotByPath[p]; got != "created" {
			t.Errorf("%s: action = %q, want %q", p, got, "created")
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
	if got := actionFor(results, ".cursor/mcp.json"); got != "updated" {
		t.Errorf(".cursor/mcp.json action = %q, want %q", got, "updated")
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

func TestSync_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")

	// Run 1: all three files created.
	results1, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 1: %v", err)
	}
	for _, r := range results1 {
		if r.Action != "created" {
			t.Errorf("run 1 %s: action = %q, want %q", r.Path, r.Action, "created")
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
		if r.Action != "no-op" {
			t.Errorf("run 2 %s: action = %q, want %q", r.Path, r.Action, "no-op")
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
	if got := actionFor(results1, ".cursor/mcp.json"); got != "updated" {
		t.Errorf("run 1 .cursor/mcp.json: action = %q, want %q (format normalization)", got, "updated")
	}

	results2, err := Sync(dir, configs)
	if err != nil {
		t.Fatalf("Sync run 2: %v", err)
	}
	if got := actionFor(results2, ".cursor/mcp.json"); got != "no-op" {
		t.Errorf("run 2 .cursor/mcp.json: action = %q, want %q (already canonical)", got, "no-op")
	}
}

// Helper functions used by sync tests.

func actionFor(results []SyncResult, path string) string {
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
