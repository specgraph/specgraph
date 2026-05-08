// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestStrategyImpl_Inspect_NotImplemented(t *testing.T) {
	for _, s := range []Strategy{StrategyJSONKeyMerge, StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Inspect("/tmp", ManagedFile{Strategy: s})
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("Strategy %d Inspect should return ErrNotImplemented, got %v", s, err)
		}
	}
}

func TestStrategyImpl_Sync_NotImplemented(t *testing.T) {
	for _, s := range []Strategy{StrategyJSONKeyMerge, StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Sync("/tmp", ManagedFile{Strategy: s}, SyncOptions{})
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("Strategy %d Sync should return ErrNotImplemented, got %v", s, err)
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
