// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestJSONKeyMergeOpencodePluginUnion(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:     "opencode.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		Build: func(_ ProjectParams) ([]byte, error) {
			return []byte(`{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`), nil
		},
	}
	// Seed with a user-added plugin entry.
	seed := []byte(`{"plugin":["./user-plugin.ts"]}`)
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, rerr := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if rerr != nil {
		t.Fatalf("read result: %v", rerr)
	}
	var doc map[string]any
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	plugins, _ := doc["plugin"].([]any)
	if len(plugins) != 2 {
		t.Fatalf("plugin array len = %d, want 2; got: %v", len(plugins), plugins)
	}
	if plugins[0] != "./.specgraph/agents/opencode/specgraph.ts" {
		t.Errorf("[0] = %v, want our managed path first", plugins[0])
	}
	if plugins[1] != "./user-plugin.ts" {
		t.Errorf("[1] = %v, want user path preserved", plugins[1])
	}
}
