// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wholeFileJSONCanonical mirrors the bytes of embedded/test/test.json so
// tests can compare against the canonical without re-reading the embed.
// The fixture is dprint-formatted JSON; trailing newline included.
var wholeFileJSONCanonical = []byte("{ \"hello\": \"world\" }\n")

func wholeFileJSONTestEntry() ManagedFile {
	return ManagedFile{
		Path:     "test.json",
		Strategy: StrategyWholeFile,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		Source:   "embedded/test/test.json",
	}
}

func TestWholeFileJSON_Missing(t *testing.T) {
	mf := wholeFileJSONTestEntry()
	dir := t.TempDir()
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("got action %v, want ActionCreated", res.Action)
	}
	got, rerr := os.ReadFile(filepath.Join(dir, "test.json"))
	if rerr != nil {
		t.Fatalf("read written file: %v", rerr)
	}
	if string(got) != string(wholeFileJSONCanonical) {
		t.Errorf("got %q, want %q", got, wholeFileJSONCanonical)
	}
}

func TestWholeFileJSON_Synced(t *testing.T) {
	mf := wholeFileJSONTestEntry()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.json"), wholeFileJSONCanonical, 0o644); err != nil { //nolint:gosec // test fixture in t.TempDir
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	state, err := s.Inspect(dir, mf, ProjectParams{})
	if err != nil {
		t.Fatal(err)
	}
	if state.State != StateSynced {
		t.Errorf("got state %v, want StateSynced", state.State)
	}
}

func TestWholeFileJSON_DriftedUserowned(t *testing.T) {
	mf := wholeFileJSONTestEntry()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"hello":"hacked"}`), 0o644); err != nil { //nolint:gosec // test fixture in t.TempDir
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("got action %v, want ActionSkipped (drifted, user-owned)", res.Action)
	}
	if !strings.Contains(res.Detail, "no sentinel") {
		t.Errorf("expected detail to contain %q, got %q", "no sentinel", res.Detail)
	}
}
