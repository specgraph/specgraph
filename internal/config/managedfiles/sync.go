// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "fmt"

// Sync reconciles a single ManagedFile against its embedded canonical,
// honouring SyncOptions (Force, KeepEdits). Returns a SyncResult
// describing what was done.
//
// PR A dispatches to per-strategy stubs that return errNotImplemented;
// the empty manifest means this is never called end-to-end. PRs B/C/D/E
// implement each strategy.
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the public API
func Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error) {
	r, err := strategyImpl(mf.Strategy).Sync(cwd, mf, params, opts)
	if err != nil {
		return r, fmt.Errorf("strategy sync: %w", err)
	}
	return r, nil
}

// SyncAll iterates the manifest filtered by enabled harnesses, calls
// Sync on each, and returns one SyncResult per entry. Per-file errors
// are captured in the SyncResult (Action == ActionError); the iteration
// continues so partial failure produces a complete report.
//
// In PR A, Manifest() is empty, so SyncAll returns an empty slice
// regardless of input.
func SyncAll(cwd string, harnesses []Harness, params ProjectParams, opts SyncOptions) ([]SyncResult, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]SyncResult, 0, len(mfs))
	for _, mf := range mfs {
		r, err := Sync(cwd, mf, params, opts)
		if err != nil {
			out = append(out, SyncResult{
				Path:   mf.Path,
				Action: ActionError,
				Err:    fmt.Errorf("sync %s: %w", mf.Path, err),
			})
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
