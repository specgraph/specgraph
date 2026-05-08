// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel is the parsed payload of a managed-file sentinel line.
//
// Version is the marker version (1 or 2). Version 0 indicates a non-sentinel
// line was parsed (treat as "no sentinel present").
//
// SHA256 is empty for v=1 markers (no hash field) and populated for v=2.
//
// Rev is the optional build revision recorded for forensics; not used in
// state classification.
type Sentinel struct {
	Version int
	SHA256  string
	Rev     string
}

// supportedVersions lists marker versions the parser accepts. Anything
// outside this set is treated as ErrCorruptedSentinel by ParseSentinel,
// matching the existing pointers/agents.go corruption-rejection behaviour
// for unknown versions.
var supportedVersions = map[int]bool{1: true, 2: true}

// sentinelMatcher matches both the WholeFile sentinel ("// specgraph:init v=N ...")
// and the MarkdownBlock start marker ("<!-- specgraph:init:start v=N ... -->").
//
// Group 1 captures the version digits.
// Group 2 captures the optional sha256 hex value.
// Group 3 captures the optional rev value.
//
// Both `init` and `init:start` are accepted because the same parser is
// used for both syntaxes; the caller's CommentSyntax decides which form
// RenderSentinel emits.
var sentinelMatcher = regexp.MustCompile(
	`specgraph:init(?::start)?\s+v=(\d+)(?:\s+sha256=([0-9a-fA-F]+))?(?:\s+rev=(\S+?))?\s*(?:-->)?$`,
)

// RenderSentinel formats a Sentinel as a single line in the given comment
// syntax. The returned line includes the comment delimiters but no trailing
// newline.
//
// For CommentNone, returns the empty string (JSON files don't carry sentinels).
//
// For CommentHTML, the rendered line is the START marker only — callers writing
// a MarkdownBlock are responsible for emitting the matching `<!-- specgraph:init:end -->`
// terminator. Keeping this asymmetry inside the strategy implementation avoids
// requiring sentinel.go to know about block structure.
func RenderSentinel(syntax CommentSyntax, s Sentinel) string {
	if syntax == CommentNone || s.Version == 0 {
		return ""
	}
	body := fmt.Sprintf("specgraph:init v=%d", s.Version)
	if s.SHA256 != "" {
		body += " sha256=" + s.SHA256
	}
	if s.Rev != "" {
		body += " rev=" + s.Rev
	}
	switch syntax {
	case CommentSlash:
		return "// " + body
	case CommentHash:
		return "# " + body
	case CommentHTML:
		// Block-strategy start marker. The end marker is emitted separately
		// by the strategy code (it has no payload).
		return "<!-- " + strings.Replace(body, "specgraph:init", "specgraph:init:start", 1) + " -->"
	default:
		return ""
	}
}

// ParseSentinel attempts to parse `line` as a managed-file sentinel.
//
// Returns:
//   - zero Sentinel + nil error if the line is not a sentinel at all (a
//     regular comment, blank, or arbitrary content).
//   - non-zero Sentinel + nil error on a successful parse.
//   - zero Sentinel + ErrCorruptedSentinel if the line *looks* like a
//     sentinel (matches the regex) but carries an unsupported version.
//
// Distinguishing "not a sentinel" from "corrupted sentinel" matters because
// the framework treats absent sentinels as user-owned (Drifted) but
// corrupted sentinels as a hard error (refuse-to-mutate).
func ParseSentinel(syntax CommentSyntax, line string) (Sentinel, error) {
	if syntax == CommentNone {
		return Sentinel{}, nil
	}
	m := sentinelMatcher.FindStringSubmatch(line)
	if m == nil {
		return Sentinel{}, nil
	}
	version, err := strconv.Atoi(m[1])
	if err != nil {
		return Sentinel{}, fmt.Errorf("%w: invalid version %q", ErrCorruptedSentinel, m[1])
	}
	if !supportedVersions[version] {
		return Sentinel{}, fmt.Errorf("%w: unsupported version %d", ErrCorruptedSentinel, version)
	}
	return Sentinel{
		Version: version,
		SHA256:  m[2],
		Rev:     m[3],
	}, nil
}
