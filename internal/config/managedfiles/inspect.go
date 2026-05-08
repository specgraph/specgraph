// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Inspect classifies the on-disk state of a single ManagedFile relative
// to its embedded canonical. Returns a FileState describing the four
// possible outcomes: Missing, Synced, Stale, Drifted.
//
// Returns an error only on operational failures (symlink in path,
// permission denied, etc.). Drift classifications are returned as a
// non-nil State, not as an error.
//
// PR A handles the WholeFile strategy generically using the sentinel +
// hash mechanism. JSONKeyMerge and MarkdownBlock strategy paths are
// reserved for PR B implementations.
func Inspect(cwd string, mf ManagedFile) (FileState, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, err
	}

	full := filepath.Join(cwd, mf.Path)
	disk, readErr := os.ReadFile(full)
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		return FileState{
			Path:     mf.Path,
			Strategy: mf.Strategy,
			State:    StateMissing,
			Detail:   "file does not exist",
		}, nil
	case readErr != nil:
		return FileState{}, fmt.Errorf("read %s: %w", full, readErr)
	}

	canonical, err := readSource(mf)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return FileState{}, fmt.Errorf("read source for %s: %w", mf.Path, err)
	}

	diskHash := HashExcludingSentinel(mf.Comment, disk)
	embeddedHash := ""
	if canonical != nil {
		embeddedHash = HashExcludingSentinel(mf.Comment, canonical)
	}

	// Strategy-specific classification is implemented in PRs B+. Until then
	// we return a Detail noting the file exists but state is undetermined.
	// The empty manifest in PR A means this path is never reached
	// end-to-end.
	return FileState{
		Path:         mf.Path,
		Strategy:     mf.Strategy,
		State:        StateSynced, // placeholder; PR B implements per-strategy
		DiskHash:     diskHash,
		EmbeddedHash: embeddedHash,
		Detail:       "PR A: classification deferred to per-strategy code in PR B",
	}, nil
}

// validateProjectDir rejects non-existent dirs, non-dirs, and symlink-rooted
// project directories. Mirrors pointers/sync.go's projectDir guard.
//
//nolint:unused // consumed by InspectAll/SyncAll added in Task 12
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
