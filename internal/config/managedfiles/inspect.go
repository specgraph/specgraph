// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"os"
)

// Inspect classifies the on-disk state of a single ManagedFile relative
// to its embedded canonical. Returns a FileState describing the four
// possible outcomes: Missing, Synced, Stale, Drifted.
//
// Returns an error only on operational failures (symlink in path,
// permission denied, etc.). Drift classifications are returned as a
// non-nil State, not as an error.
//
// Strategy implementations own all classification logic; this function
// dispatches to the appropriate strategy.
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the public API
func Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	state, err := strategyImpl(mf.Strategy).Inspect(cwd, mf, params)
	if err != nil {
		return state, fmt.Errorf("strategy inspect %s: %w", mf.Path, err)
	}
	return state, nil
}

// InspectAll iterates the manifest filtered by the user's enabled harnesses
// and returns a FileState for each. Errors at the per-file level are
// captured in the FileState (not surfaced as an error return) so callers
// see all results, not just the first failure.
//
// In PR A, Manifest() returns an empty slice; InspectAll therefore returns
// an empty slice. PRs B+ populate the manifest.
func InspectAll(cwd string, harnesses []Harness, params ProjectParams) ([]FileState, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]FileState, 0, len(mfs))
	for _, mf := range mfs {
		state, err := Inspect(cwd, mf, params)
		if err != nil {
			out = append(out, FileState{
				Path:     mf.Path,
				Strategy: mf.Strategy,
				Harness:  mf.Harness,
				State:    StateDrifted,
				Detail:   fmt.Sprintf("inspect error: %v", err),
			})
			continue
		}
		// Strategy literals don't know which harness owns the manifest
		// entry; overwrite here so callers (doctor's --harness filter,
		// JSON output, etc.) see the attribution. Per design §Managed files.
		state.Harness = mf.Harness
		out = append(out, state)
	}
	return out, nil
}

// validateProjectDir rejects non-existent dirs, non-dirs, and symlink-rooted
// project directories. Mirrors pointers/sync.go's projectDir guard.
func validateProjectDir(projectDir string) error {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project dir %q is not a directory", projectDir)
	}
	li, lerr := os.Lstat(projectDir)
	if lerr != nil {
		return fmt.Errorf("lstat %s: %w", projectDir, lerr)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkRejected, projectDir)
	}
	return nil
}
