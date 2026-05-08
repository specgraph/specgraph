// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import "errors"

// Sentinel errors for callers that want to branch on failure mode.
// Use errors.Is, not == comparison: each Sync call wraps these with
// fmt.Errorf("...: %w", err) for a contextual top-level message.
var (
	// ErrCorruptedMarkers indicates managed-block markers in the target
	// file violate one of the four corruption rules in validateInitMarkers.
	// The caller should refuse to proceed; the user must repair the file
	// manually.
	ErrCorruptedMarkers = errors.New("corrupted init markers")

	// ErrSymlinkRejected indicates a path component (or the target itself)
	// is a symlink. Sync refuses to follow symlinks because the user owns
	// the file and may not own its symlink target.
	ErrSymlinkRejected = errors.New("refusing to follow symlink")

	// ErrFrontmatterMissing indicates the cursor rule file does not begin
	// with `---\n`. Sync refuses to silently convert it; the user must add
	// frontmatter or remove the file.
	ErrFrontmatterMissing = errors.New("missing YAML frontmatter")
)
