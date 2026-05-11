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
	if len(all) != 5 {
		t.Errorf("Manifest() should have 5 entries for all harnesses, got %d entries", len(all))
	}
}

func TestInspectAll_SingleHarness_ReturnsFiltered(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	got, err := managedfiles.InspectAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, params)
	if err != nil {
		t.Fatal(err)
	}
	// HarnessOpenCode has 1 file: opencode.json
	if len(got) != 1 {
		t.Fatalf("InspectAll for OpenCode should return 1 entry, got %d", len(got))
	}
	if got[0].Path != "opencode.json" {
		t.Errorf("InspectAll[0].Path = %q, want \"opencode.json\"", got[0].Path)
	}
}

func TestSyncAll_SingleHarness_ReturnsFiltered(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	got, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// HarnessOpenCode has 1 file: opencode.json
	if len(got) != 1 {
		t.Fatalf("SyncAll for OpenCode should return 1 entry, got %d", len(got))
	}
	if got[0].Path != "opencode.json" {
		t.Errorf("SyncAll[0].Path = %q, want \"opencode.json\"", got[0].Path)
	}
	if got[0].Action == managedfiles.ActionError {
		t.Errorf("SyncAll[0].Action = ActionError, err = %v", got[0].Err)
	}
}
