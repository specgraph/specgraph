// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testJSONMF(path string) ManagedFile {
	return ManagedFile{
		Path:     path,
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		Build: func(_ ProjectParams) ([]byte, error) {
			return []byte(`{"mcp":{"specgraph":{"url":"http://h/mcp/"}}}`), nil
		},
	}
}

func TestJSONKeyMergeMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	s := jsonKeyMergeStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	got, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if jerr := json.Unmarshal(got, &v); jerr != nil {
		t.Errorf("output not valid JSON: %v", jerr)
	}
}

func TestJSONKeyMergeSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	params := ProjectParams{Slug: "p", ServerURL: "http://h"}
	s := jsonKeyMergeStrategy{}
	// First sync creates.
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second sync is no-op.
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestJSONKeyMergeStalePreservesSiblings(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	// Seed with a user-added sibling key.
	seed := []byte(`{"theme":"dark","mcp":{"specgraph":{"url":"http://OLD/"}}}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	got, rerr := os.ReadFile(filepath.Join(dir, "config.json"))
	if rerr != nil {
		t.Fatalf("read config.json: %v", rerr)
	}
	var parsed map[string]any
	if jerr := json.Unmarshal(got, &parsed); jerr != nil {
		t.Fatalf("unmarshal config.json: %v", jerr)
	}
	if parsed["theme"] != "dark" {
		t.Error("user-added theme sibling was destroyed")
	}
}

func TestJSONKeyMergeOpencodeJSONCRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte("//"), 0o600); err != nil {
		t.Fatal(err)
	}
	mf := testJSONMF("opencode.json")
	s := jsonKeyMergeStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err == nil && res.Action != ActionError {
		t.Errorf("want error or ActionError; got action=%v err=%v", res.Action, err)
	}
}

func TestJSONKeyMergeModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	// Seed at 0o644.
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	info, serr := os.Stat(filepath.Join(dir, "config.json"))
	if serr != nil {
		t.Fatalf("stat config.json: %v", serr)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestJSONKeyMergeInspectMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	s := jsonKeyMergeStrategy{}
	state, err := s.Inspect(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"})
	if err != nil {
		t.Fatal(err)
	}
	if state.State != StateMissing {
		t.Errorf("state = %v, want StateMissing", state.State)
	}
}

func TestJSONKeyMerge_KeyManagedValue_Basic(t *testing.T) {
	mf := ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/managed/value",
				Mode: KeyManagedValue,
				Value: func(_ ProjectParams) (any, error) {
					return "canonical", nil
				},
			},
		},
	}
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	if err := os.WriteFile(full, []byte(`{"unrelated":"keep","managed":{"value":"old"}}`), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("got action %v, want Refreshed", res.Action)
	}
	got, _ := os.ReadFile(full)
	var doc map[string]any
	_ = json.Unmarshal(got, &doc)
	if doc["unrelated"] != "keep" {
		t.Errorf("unrelated key clobbered: %v", doc)
	}
	if m, _ := doc["managed"].(map[string]any); m["value"] != "canonical" {
		t.Errorf("managed key not refreshed: %v", doc)
	}
}

func TestJSONKeyMerge_KeyManagedPresence_WriteIfAbsent(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	if err := os.WriteFile(full, []byte(`{}`), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := readEnabledPlugin(t, full); !ok || v != true {
		t.Errorf("expected enabledPlugins[\"specgraph@specgraph-local\"]=true on first init, got %v (present=%v)", v, ok)
	}
}

func TestJSONKeyMerge_KeyManagedPresence_PreservesUserFalse(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	if err := os.WriteFile(full, []byte(`{"enabledPlugins":{"specgraph@specgraph-local":false}}`), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := readEnabledPlugin(t, full); !ok || v != false {
		t.Errorf("expected user's false to be preserved, got %v (present=%v)", v, ok)
	}
}

func TestJSONKeyMerge_KeyManagedPresence_PreservesUserCustomValue(t *testing.T) {
	mf := presenceTestEntry(t, true)
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	if err := os.WriteFile(full, []byte(`{"enabledPlugins":{"specgraph@specgraph-local":"custom"}}`), 0o644); err != nil { //nolint:gosec // intentional permissive mode for permission-preservation test
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := readEnabledPlugin(t, full); !ok || v != "custom" {
		t.Errorf("expected user's custom value to be preserved, got %v (present=%v)", v, ok)
	}
}

// readEnabledPlugin loads the JSON at path, navigates enabledPlugins, and
// returns the value at specgraph@specgraph-local plus a presence flag.
// Structural lookup so tests don't break on JSON formatting changes.
func readEnabledPlugin(t *testing.T, path string) (any, bool) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	enabled, ok := doc["enabledPlugins"].(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := enabled["specgraph@specgraph-local"]
	return v, ok
}

func presenceTestEntry(t *testing.T, defaultValue bool) ManagedFile {
	t.Helper()
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/enabledPlugins/specgraph@specgraph-local",
				Mode: KeyManagedPresence,
				Value: func(_ ProjectParams) (any, error) {
					return defaultValue, nil
				},
			},
		},
	}
}

func TestJSONKeyMerge_KeyManagedArrayUnion_AbsentArray(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{}`), 0o644) //nolint:gosec // intentional permissive mode for permission-preservation test
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"./.specgraph/agents/opencode/specgraph.ts"`) {
		t.Errorf("expected canonical element written; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedArrayUnion_DisjointUnion(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	_ = os.WriteFile(full, []byte(`{"plugin":["./user-plugin.ts"]}`), 0o644) //nolint:gosec // intentional permissive mode for permission-preservation test
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), `"./user-plugin.ts"`) ||
		!strings.Contains(string(got), `"./.specgraph/agents/opencode/specgraph.ts"`) {
		t.Errorf("expected both elements present; got %s", got)
	}
}

func TestJSONKeyMerge_KeyManagedArrayUnion_DedupesOverlap(t *testing.T) {
	mf := arrayUnionTestEntry()
	dir := t.TempDir()
	full := filepath.Join(dir, "test.json")
	seed := []byte(`{"plugin":["./.specgraph/agents/opencode/specgraph.ts","./user.ts"]}`)
	_ = os.WriteFile(full, seed, 0o644) //nolint:gosec // intentional permissive mode for permission-preservation test
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(full)
	var doc struct {
		Plugin []string `json:"plugin"`
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Plugin) != 2 {
		t.Errorf("expected 2 unique elements, got %d: %v", len(doc.Plugin), doc.Plugin)
	}
}

func arrayUnionTestEntry() ManagedFile {
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		JSONKeys: []JSONManagedKey{
			{
				Path: "/plugin",
				Mode: KeyManagedArrayUnion,
				Value: func(_ ProjectParams) (any, error) {
					return []any{"./.specgraph/agents/opencode/specgraph.ts"}, nil
				},
			},
		},
	}
}

// TestJSONKeyMergeOpencodePluginUnion was removed: the path-keyed
// unionPluginArray hook it exercised has been deleted, and its
// behavior (union of canonical + existing plugin entries) is now
// covered generically by TestJSONKeyMerge_KeyManagedArrayUnion_*
// and TestMigratedOpenCodeJSON_PreservesPluginUnion.

func TestClaudeSettingsJSON_FreshInit(t *testing.T) {
	dir := t.TempDir()
	mf := findManifestEntry(t, ".claude/settings.json")
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{Slug: "x"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dir, ".claude/settings.json")
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read %s: %v", settingsPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ekm, ok := doc["extraKnownMarketplaces"].(map[string]any)
	if !ok {
		t.Fatalf("extraKnownMarketplaces missing or wrong type: %s", got)
	}
	entry, ok := ekm["specgraph-local"].(map[string]any)
	if !ok {
		t.Fatalf("extraKnownMarketplaces.specgraph-local missing or wrong type: %s", got)
	}
	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatalf("extraKnownMarketplaces.specgraph-local.source missing or wrong type: %s", got)
	}
	if source["path"] != "./.specgraph/agents/claude" {
		t.Errorf("marketplace path = %v, want %q", source["path"], "./.specgraph/agents/claude")
	}
	if v, ok := readEnabledPlugin(t, settingsPath); !ok || v != true {
		t.Errorf("expected enabledPlugins[\"specgraph@specgraph-local\"]=true on fresh init, got %v (present=%v)", v, ok)
	}
}

func TestClaudeSettingsJSON_PreservesUserDisable(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	_ = os.MkdirAll(settingsDir, 0o755)                                                                                                       //nolint:gosec // test directory creation with permissive mode is intentional
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"enabledPlugins":{"specgraph@specgraph-local":false}}`), 0o644)    //nolint:gosec // intentional permissive mode for permission-preservation test
	mf := findManifestEntry(t, ".claude/settings.json")
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{Slug: "x"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if v, ok := readEnabledPlugin(t, filepath.Join(dir, ".claude/settings.json")); !ok || v != false {
		t.Errorf("user's disable was overwritten, got %v (present=%v)", v, ok)
	}
}
