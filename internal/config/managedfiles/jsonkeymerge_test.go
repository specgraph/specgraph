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
	got, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var parsed map[string]any
	_ = json.Unmarshal(got, &parsed)
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
	info, _ := os.Stat(filepath.Join(dir, "config.json"))
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
