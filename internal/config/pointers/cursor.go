// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const cursorRel = ".cursor/rules/specgraph-bootstrap.md"

const defaultCursorFrontmatter = `---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---

`

func renderCursorBody(opts Options) string {
	return renderAgentsBlock(opts) + "\n"
}

func syncCursor(projectDir string, opts Options) SyncResult {
	if err := rejectSymlinkComponents(projectDir, cursorRel); err != nil {
		return errResult(cursorRel, err)
	}
	full := filepath.Join(projectDir, cursorRel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return errResult(cursorRel, fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err))
	}

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return errResult(cursorRel, lerr)
	}
	defer unlock()

	existing, rerr := os.ReadFile(full)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return errResult(cursorRel, fmt.Errorf("read %s: %w", full, rerr))
	}

	canonicalBody := renderCursorBody(opts)

	if errors.Is(rerr, fs.ErrNotExist) {
		out := []byte(defaultCursorFrontmatter + canonicalBody)
		if werr := atomicWrite(full, out, 0o600); werr != nil {
			return errResult(cursorRel, werr)
		}
		return okResult(cursorRel, ActionCreated, 0)
	}

	frontmatter, body, ferr := splitFrontmatter(existing)
	if ferr != nil {
		return errResult(cursorRel, ferr)
	}

	// Phase 1: corruption check on the body.
	if err := validateInitMarkers(cursorRel, body); err != nil {
		return errResult(cursorRel, err)
	}

	// Reconcile init block in body. (No legacy purge for cursor — inject's
	// per-slug rules lived in separate per-slug files, not inside this one.)
	var newBody []byte
	switch {
	case len(body) == 0:
		newBody = []byte(canonicalBody)
	case bytes.Contains(body, []byte(initStart)):
		newBody = replaceInitBlock(body, renderAgentsBlock(opts))
		if !bytes.HasSuffix(newBody, []byte("\n")) {
			newBody = append(newBody, '\n')
		}
	default:
		newBody = body
		if !bytes.HasSuffix(newBody, []byte("\n")) {
			newBody = append(newBody, '\n')
		}
		newBody = append(newBody, '\n')
		newBody = append(newBody, renderAgentsBlock(opts)...)
		newBody = append(newBody, '\n')
	}

	updated := append([]byte{}, frontmatter...)
	updated = append(updated, newBody...)

	if bytes.Equal(existing, updated) {
		return noopResult(cursorRel)
	}

	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if werr := atomicWrite(full, updated, mode); werr != nil {
		return errResult(cursorRel, werr)
	}
	return okResult(cursorRel, ActionUpdated, 0)
}

// splitFrontmatter splits a Cursor rule file into (frontmatter-including-trailing-blank, body).
// Returns an error if the file does not begin with a `---` line: an existing
// rule without frontmatter is malformed and we refuse to silently convert it.
func splitFrontmatter(data []byte) (front, body []byte, err error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf(
			"%w: %s (must start with '---'); remove the file or add frontmatter manually",
			ErrFrontmatterMissing,
			cursorRel,
		)
	}
	// Find the second `---` line.
	rest := data[len("---\n"):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return nil, nil, fmt.Errorf("%w: %s: frontmatter not closed before EOF", ErrFrontmatterMissing, cursorRel)
	}
	end := len("---\n") + idx + len("\n---\n")
	// Include any single trailing blank line.
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return data[:end], data[end:], nil
}
