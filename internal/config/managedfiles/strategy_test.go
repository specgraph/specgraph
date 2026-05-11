// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestStrategyImpl_Inspect_NotImplemented(t *testing.T) {
	// StrategyJSONKeyMerge is implemented in PR B; only the remaining stubs are checked here.
	for _, s := range []Strategy{StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Inspect("/tmp", ManagedFile{Strategy: s}, ProjectParams{})
		if !errors.Is(err, errNotImplemented) {
			t.Errorf("Strategy %d Inspect should return errNotImplemented, got %v", s, err)
		}
	}
}

func TestStrategyImpl_Sync_NotImplemented(t *testing.T) {
	// StrategyJSONKeyMerge is implemented in PR B; only the remaining stubs are checked here.
	for _, s := range []Strategy{StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Sync("/tmp", ManagedFile{Strategy: s}, ProjectParams{}, SyncOptions{})
		if !errors.Is(err, errNotImplemented) {
			t.Errorf("Strategy %d Sync should return errNotImplemented, got %v", s, err)
		}
	}
}

func TestStrategyImpl_UnknownStrategy_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown strategy")
		}
	}()
	_ = strategyImpl(Strategy(99))
}

// TestSync_DispatchesAndWraps verifies the public Sync entry point
// dispatches through strategyImpl and propagates errNotImplemented from
// the remaining stubs (MarkdownBlock, WholeFile). JSONKeyMerge is
// implemented in PR B and excluded from this check.
func TestSync_DispatchesAndWraps(t *testing.T) {
	for _, s := range []Strategy{StrategyMarkdownBlock, StrategyWholeFile} {
		_, err := Sync("/tmp", ManagedFile{Path: "x", Strategy: s}, ProjectParams{}, SyncOptions{})
		if !errors.Is(err, errNotImplemented) {
			t.Errorf("Strategy %d: Sync should propagate errNotImplemented, got %v", s, err)
		}
	}
}
