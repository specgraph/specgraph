// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

func TestManifest_EmptyInPRA(t *testing.T) {
	all := managedfiles.Manifest([]managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	})
	if len(all) != 0 {
		t.Errorf("Manifest() should be empty in PR A, got %d entries", len(all))
	}
}

func TestInspectAll_EmptyManifest_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := managedfiles.InspectAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, managedfiles.ProjectParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("InspectAll on empty manifest should be empty, got %d", len(got))
	}
}

func TestSyncAll_EmptyManifest_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, managedfiles.ProjectParams{}, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("SyncAll on empty manifest should be empty, got %d", len(got))
	}
}
