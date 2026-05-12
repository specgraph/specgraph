// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

func TestManifest_AllHarnesses(t *testing.T) {
	all := managedfiles.Manifest([]managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	})
	if len(all) != 6 {
		t.Errorf("Manifest() should have 6 entries for all harnesses, got %d entries", len(all))
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
