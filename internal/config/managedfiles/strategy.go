// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "fmt"

// strategy is the interface implemented per-strategy in PR B+. PR A
// registers stubs that return errNotImplemented for both methods.
//
// Inspect classifies the on-disk state for a single ManagedFile.
// Sync writes (or refrains from writing) the canonical content per
// SyncOptions. Both methods MUST be safe to call with mf.Strategy
// matching the dispatched strategy; misuse is a programming error.
type strategy interface {
	Inspect(cwd string, mf ManagedFile) (FileState, error)
	Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error)
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

// PR A stubs. All three return errNotImplemented; PRs B/C/D/E replace
// each one with a real implementation.

type jsonKeyMergeStrategy struct{}

func (jsonKeyMergeStrategy) Inspect(_ string, _ ManagedFile) (FileState, error) {
	return FileState{}, errNotImplemented
}
func (jsonKeyMergeStrategy) Sync(_ string, _ ManagedFile, _ SyncOptions) (SyncResult, error) {
	return SyncResult{}, errNotImplemented
}

type markdownBlockStrategy struct{}

func (markdownBlockStrategy) Inspect(_ string, _ ManagedFile) (FileState, error) {
	return FileState{}, errNotImplemented
}
func (markdownBlockStrategy) Sync(_ string, _ ManagedFile, _ SyncOptions) (SyncResult, error) {
	return SyncResult{}, errNotImplemented
}

type wholeFileStrategy struct{}

func (wholeFileStrategy) Inspect(_ string, _ ManagedFile) (FileState, error) {
	return FileState{}, errNotImplemented
}
func (wholeFileStrategy) Sync(_ string, _ ManagedFile, _ SyncOptions) (SyncResult, error) {
	return SyncResult{}, errNotImplemented
}
