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

// supersedesGuardedDelete deletes the file at <projectDir>/<oldPath> only
// if its on-disk content hash matches expectedPriorHash.
//
// expectedPriorHash is the hash this binary's *prior canonical* would have
// produced for oldPath — typically computed by the calling strategy using
// the vestigial v=1 renderer (see §"Drift detection / Vestigial v=1
// renderer" in the design doc). The guard prevents init from clobbering
// user content that happens to live at a path being superseded.
//
// Returns nil if the file doesn't exist (nothing to delete) or was deleted
// successfully. Returns ErrPriorCanonicalMismatch if the hash check fails;
// the file is left in place. Returns wrapped errors for other failures
// (lstat, read, remove).
func supersedesGuardedDelete(projectDir, oldPath, expectedPriorHash string) error {
	full := filepath.Join(projectDir, oldPath)

	if err := rejectSymlinkComponents(projectDir, oldPath); err != nil {
		return err
	}

	disk, err := os.ReadFile(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("read %s: %w", full, err)
	}

	// CommentNone is intentional: callers pass the exact bytes the prior
	// canonical would have produced including any sentinel, so we hash
	// the raw bytes here and let the caller-provided hash account for
	// any sentinel-stripping it cares about. This keeps the guard
	// agnostic to comment syntax.
	actual := HashExcludingSentinel(CommentNone, disk)
	if actual != expectedPriorHash {
		return fmt.Errorf("%w: %s (got %s, want %s)",
			ErrPriorCanonicalMismatch, oldPath, actual, expectedPriorHash)
	}

	if rmErr := os.Remove(full); rmErr != nil {
		return fmt.Errorf("remove %s: %w", full, rmErr)
	}
	return nil
}
