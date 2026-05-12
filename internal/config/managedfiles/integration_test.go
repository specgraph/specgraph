// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

func TestManifest_AllHarnesses(t *testing.T) {
	all := managedfiles.Manifest([]managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	})
	if len(all) != 8 {
		t.Errorf("Manifest() should have 8 entries for all harnesses, got %d entries", len(all))
	}
}

func TestInspectAll_SingleHarness_ReturnsFiltered(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	got, err := managedfiles.InspectAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, params)
	if err != nil {
		t.Fatal(err)
	}
	// HarnessOpenCode has 2 files: opencode.json + .specgraph/agents/opencode/specgraph.ts
	if len(got) != 2 {
		t.Fatalf("InspectAll for OpenCode should return 2 entries, got %d", len(got))
	}
	paths := map[string]bool{}
	for _, f := range got {
		paths[f.Path] = true
	}
	if !paths["opencode.json"] {
		t.Error("InspectAll missing opencode.json")
	}
	if !paths[".specgraph/agents/opencode/specgraph.ts"] {
		t.Error("InspectAll missing .specgraph/agents/opencode/specgraph.ts")
	}
}

func TestSyncAll_SingleHarness_ReturnsFiltered(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	got, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// HarnessOpenCode has 2 files: opencode.json + .specgraph/agents/opencode/specgraph.ts
	if len(got) != 2 {
		t.Fatalf("SyncAll for OpenCode should return 2 entries, got %d", len(got))
	}
	for _, r := range got {
		if r.Action == managedfiles.ActionError {
			t.Errorf("SyncAll[%s].Action = ActionError, err = %v", r.Path, r.Err)
		}
	}
}

func TestSyncAll_CursorMdcVerbatimSupersedes(t *testing.T) {
	dir := t.TempDir()
	// Seed both old .md files with verbatim pre-rename bytes (from testdata).
	for _, name := range []string{"specgraph.md", "post-stage.md"} {
		src := filepath.Join("testdata", "cursor-vestigial", name)
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		target := filepath.Join(dir, ".cursor/rules", name)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
			t.Fatal(mkErr)
		}
		if wErr := os.WriteFile(target, body, 0o600); wErr != nil { //nolint:gosec // test fixture seeding in t.TempDir; path is constructed, not user-supplied
			t.Fatal(wErr)
		}
	}

	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// New .mdc files exist; old .md files are gone.
	for _, p := range []string{".cursor/rules/specgraph.mdc", ".cursor/rules/specgraph-post-stage.mdc"} {
		if _, sErr := os.Stat(filepath.Join(dir, p)); sErr != nil {
			t.Errorf("expected %s to exist: %v", p, sErr)
		}
	}
	for _, p := range []string{".cursor/rules/specgraph.md", ".cursor/rules/post-stage.md"} {
		if _, sErr := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(sErr) {
			t.Errorf("%s should be deleted (stat err = %v)", p, sErr)
		}
	}

	// No result reports an error.
	for _, r := range results {
		if r.Action == managedfiles.ActionError {
			t.Errorf("result for %s reported error: %v", r.Path, r.Err)
		}
	}
}

func TestSyncAll_CursorMdcEditedMdPreserved(t *testing.T) {
	dir := t.TempDir()
	// Seed .cursor/rules/specgraph.md with edited content.
	src := filepath.Join("testdata", "cursor-vestigial", "specgraph.md")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	edited := append([]byte{}, body...)
	edited = append(edited, []byte("\n<!-- user note -->\n")...)
	target := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}
	if wErr := os.WriteFile(target, edited, 0o600); wErr != nil { //nolint:gosec // test fixture seeding in t.TempDir; path is constructed, not user-supplied
		t.Fatal(wErr)
	}

	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Old .md preserved.
	if _, sErr := os.Stat(target); sErr != nil {
		t.Errorf("edited .md should be preserved: %v", sErr)
	}

	// Find the .mdc result and verify its Detail mentions the mismatch.
	found := false
	for _, r := range results {
		if r.Path == ".cursor/rules/specgraph.mdc" {
			found = true
			if !strings.Contains(r.Detail, `supersedes path ".cursor/rules/specgraph.md" left in place: prior-canonical mismatch`) {
				t.Errorf("Detail = %q, expected mismatch message", r.Detail)
			}
		}
	}
	if !found {
		t.Errorf("no SyncResult for .cursor/rules/specgraph.mdc")
	}
}

func TestSyncAll_CursorMdcIdempotent(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}

	if _, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second run: every cursor-managed file reports ActionNoOp.
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action != managedfiles.ActionNoOp {
			t.Errorf("%s: action %v on second sync, want ActionNoOp", r.Path, r.Action)
		}
	}
}
