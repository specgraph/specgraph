// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "errors"

// ErrFrontmatterMissing is returned by splitFrontmatter when the input
// does not begin with `---\n` or the frontmatter is not closed.
// Ported from pointers/errors.go.
var ErrFrontmatterMissing = errors.New("frontmatter missing or unclosed")

// ErrCorruptedMarkers is returned when validateInitMarkers detects a
// malformed init-block marker pair (count mismatch, ordering, naked
// marker without version, unknown version). Ported from
// pointers/errors.go.
var ErrCorruptedMarkers = errors.New("corrupted specgraph:init markers")
