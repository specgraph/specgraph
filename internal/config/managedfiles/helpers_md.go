// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"fmt"
	"regexp"
)

// safeSlugPattern matches the slug class accepted by ProjectParams.Validate.
// Lifted verbatim from pointers/sync.go.
var safeSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// defaultCursorFrontmatter is the YAML frontmatter written into a fresh
// .mdc cursor rule. Includes the trailing blank line after the closing
// "---" — this is part of the byte sequence the supersedes prior-canonical
// hash compares against. Lifted verbatim from pointers/cursor.go:18-23.
const defaultCursorFrontmatter = `---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---

`

// Suppress unused lint until the cursor strategy Build closure (added in a later task) references it.
var _ = defaultCursorFrontmatter

// splitFrontmatter splits a Cursor rule file into
// (frontmatter-including-trailing-blank, body). Returns
// ErrFrontmatterMissing if the file does not begin with `---\n` or the
// frontmatter is not closed. Ported from pointers/cursor.go:117-137.
func splitFrontmatter(data []byte) (front, body []byte, err error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf("%w: must start with '---'", ErrFrontmatterMissing)
	}
	rest := data[len("---\n"):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return nil, nil, fmt.Errorf("%w: frontmatter not closed before EOF", ErrFrontmatterMissing)
	}
	end := len("---\n") + idx + len("\n---\n")
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return data[:end], data[end:], nil
}

// initStartLoose matches any "specgraph:init:start" marker, with or
// without a v=N suffix. Used by validateInitMarkers to reject
// naked markers. Ported from pointers/agents.go:25.
var initStartLoose = regexp.MustCompile(`<!--\s*specgraph:init:start\s*-->`)

// initStartAnyVersion matches any well-formed init start marker (v=1, v=2,
// future v=N). Used to anchor "canonical start" positions when checking
// for naked-marker corruption. Replaces the bytes.Index(initStart)
// approach in pointers/agents.go:150 that only recognised v=1.
var initStartAnyVersion = regexp.MustCompile(`<!--\s*specgraph:init:start(\s+v=\d+(\s+sha256=[0-9a-fA-F]+)?(\s+rev=\S+)?)?\s*-->`)

const initEndMarker = "<!-- specgraph:init:end -->"

// validateInitMarkers checks five corruption rules:
//
//	(1) end before start
//	(2) start without end
//	(3) double start
//	(4) naked init start without a v=N suffix
//	(5) unknown version (anything not in ParseSentinel's supported set)
//
// Adapted from pointers/agents.go:134-182. Two adaptations vs. the
// original: version acceptance now delegates to ParseSentinel (which
// supports v=1 and v=2), and the "canonical start" position used in
// Rule 4's exception comes from initStartAnyVersion regex matches,
// not bytes.Index of the v=1 literal.
func validateInitMarkers(displayName string, data []byte) error {
	starts := initStartAnyVersion.FindAllIndex(data, -1)
	ends := bytes.Count(data, []byte(initEndMarker))

	// Rule 5: each well-formed start marker must carry a supported version.
	for _, m := range starts {
		fragment := string(data[m[0]:m[1]])
		s, perr := ParseSentinel(CommentHTML, fragment)
		if perr != nil {
			return fmt.Errorf("%w: %s contains unsupported init start marker at offset %d (%q): %v",
				ErrCorruptedMarkers, displayName, m[0], fragment, perr)
		}
		if s.Version == 0 {
			// initStartAnyVersion matched a start without v=N — Rule 4.
			return fmt.Errorf("%w: %s contains an init start marker without v=N at offset %d (%q); remove or repair manually",
				ErrCorruptedMarkers, displayName, m[0], fragment)
		}
	}

	// Rule 4: catch naked "specgraph:init:start" markers that don't
	// overlap any well-formed start (e.g. a stray comment).
	for _, m := range initStartLoose.FindAllIndex(data, -1) {
		overlap := false
		for _, c := range starts {
			if m[0] == c[0] {
				overlap = true
				break
			}
		}
		if !overlap {
			return fmt.Errorf("%w: %s contains a naked init marker at offset %d", ErrCorruptedMarkers, displayName, m[0])
		}
	}

	switch {
	case len(starts) == 0 && ends == 0:
		return nil
	case len(starts) == 1 && ends == 1:
		startOff := starts[0][0]
		endOff := bytes.Index(data, []byte(initEndMarker))
		if endOff < startOff {
			return fmt.Errorf("%w: %s: init end marker appears before start marker", ErrCorruptedMarkers, displayName)
		}
		return nil
	case len(starts) > 1:
		return fmt.Errorf("%w: %s: more than one init start marker", ErrCorruptedMarkers, displayName)
	case len(starts) == 1 && ends == 0:
		return fmt.Errorf("%w: %s: init start marker without matching end", ErrCorruptedMarkers, displayName)
	default:
		return fmt.Errorf("%w: %s: init end marker without matching start", ErrCorruptedMarkers, displayName)
	}
}
