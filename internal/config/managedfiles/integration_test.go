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
		t.Errorf("InspectAll for OpenCode should return 1 entry, got %d", len(got))
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
		t.Errorf("SyncAll for OpenCode should return 1 entry, got %d", len(got))
	}
}
