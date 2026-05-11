// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// TestStrategyValuesStable pins the iota positions so future reorderings
// are caught. Callers may compare Strategy values directly.
func TestStrategyValuesStable(t *testing.T) {
	got := []managedfiles.Strategy{
		managedfiles.StrategyJSONKeyMerge,
		managedfiles.StrategyMarkdownBlock,
		managedfiles.StrategyWholeFile,
	}
	want := []managedfiles.Strategy{0, 1, 2}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Strategy[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestStateValuesStable pins the iota positions for State.
func TestStateValuesStable(t *testing.T) {
	got := []managedfiles.State{
		managedfiles.StateMissing,
		managedfiles.StateSynced,
		managedfiles.StateStale,
		managedfiles.StateDrifted,
	}
	want := []managedfiles.State{0, 1, 2, 3}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("State[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
