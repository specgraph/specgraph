// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashExcludingSentinel returns the hex-encoded sha256 of the file content
// after stripping the sentinel line(s).
//
// For CommentSlash and CommentHash: the FIRST line is dropped if it parses
// as a sentinel. Other lines are preserved verbatim — the sentinel is
// always line 0 for these syntaxes; a body line containing
// "specgraph:init" (e.g., an example inside a rule body) is not a sentinel.
//
// For CommentHTML (markdown-block strategy): every sentinel line in the
// content (start and end markers) is dropped before hashing. This handles
// files like AGENTS.md where the managed block is delimited by a pair.
//
// For CommentNone: the bytes are hashed as-is. JSON files don't carry
// sentinels, so there's nothing to strip.
//
// The hash is computed over the byte stream after sentinel-stripping,
// so two files differing only in their sentinel line hash equal.
func HashExcludingSentinel(syntax CommentSyntax, content []byte) string {
	if syntax == CommentNone {
		return hashBytes(content)
	}

	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		// For slash/hash syntaxes, only consider the first line a candidate
		// sentinel — body content might legitimately contain "# specgraph:init"
		// inside (e.g., an example in a markdown rule body).
		if (syntax == CommentSlash || syntax == CommentHash) && i > 0 {
			kept = append(kept, line)
			continue
		}
		s, err := ParseSentinel(syntax, line)
		if (err == nil && s.Version > 0) || strings.Contains(line, "specgraph:init:end") {
			// Drop sentinel start lines AND markdown-block end markers.
			// (The end marker doesn't parse as a Sentinel struct because
			// it has no version, but we drop it from the hash so the
			// framework can replace marker pairs without changing the hash.)
			continue
		}
		kept = append(kept, line)
	}
	return hashBytes([]byte(strings.Join(kept, "\n")))
}

// HashExcludingSentinelAfterFrontmatter is the counterpart of
// HashExcludingSentinel for files with leading YAML frontmatter — use it
// when ManagedFile.HasFrontmatter is true.
//
// HashExcludingSentinelAfterFrontmatter splits leading YAML frontmatter off
// content, removes the sentinel on the first line of the post-frontmatter
// body (if present), and hashes the concatenation of front + remaining body.
//
// Returns ErrFrontmatterMissing if the content does not begin with `---\n` or
// the frontmatter is unclosed — callers (the WholeFile strategy on entries
// with HasFrontmatter==true) treat that as Drifted and refuse to mutate.
//
// If the first body line is not a sentinel (parses to Version 0), the body
// is hashed unchanged — drift classification is the classifier's job, not
// this hash function's.
func HashExcludingSentinelAfterFrontmatter(syntax CommentSyntax, content []byte) (string, error) {
	front, body, err := splitFrontmatter(content)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return hashBytes(front), nil
	}
	firstLine, rest, _ := bytes.Cut(body, []byte("\n"))
	s, perr := ParseSentinel(syntax, string(firstLine))
	if perr != nil {
		// Corrupt sentinel — surface it. Callers should classify the file
		// as Drifted with the parse error in Detail.
		return "", perr
	}
	if s.Version == 0 {
		// No sentinel on body[0]. Hash the body unchanged.
		return hashBytes(bytes.Join([][]byte{front, body}, nil)), nil
	}
	// Sentinel present — drop the first line.
	return hashBytes(bytes.Join([][]byte{front, rest}, nil)), nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
