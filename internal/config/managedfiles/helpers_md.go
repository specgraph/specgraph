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
