// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "fmt"

// strategy is the interface implemented per-strategy.
//
// Inspect classifies the on-disk state for a single ManagedFile.
// Sync writes (or refrains from writing) the canonical content per
// SyncOptions. Both methods MUST be safe to call with mf.Strategy
// matching the dispatched strategy; misuse is a programming error.
type strategy interface {
	Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error)
	Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error)
}

// strategyImpl returns the strategy implementation for s.
//
// Panics if s is not a known Strategy value — that's a programming error
// from a manifest entry with a bogus enum value, not a runtime condition.
func strategyImpl(s Strategy) strategy {
	switch s {
	case StrategyJSONKeyMerge:
		return jsonKeyMergeStrategy{}
	case StrategyMarkdownBlock:
		return markdownBlockStrategy{}
	case StrategyWholeFile:
		return wholeFileStrategy{}
	default:
		panic(fmt.Sprintf("unknown Strategy value: %d", s))
	}
}

type jsonKeyMergeStrategy struct{}

type markdownBlockStrategy struct{}

type wholeFileStrategy struct{}

