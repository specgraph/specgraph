// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
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

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
