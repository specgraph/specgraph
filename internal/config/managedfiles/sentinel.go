// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"regexp"
	"strconv"
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
// used for both strategies. RenderSentinel always emits the bare `init`
// form; the `:start` form is written inline by markdownblock.go (around
// lines 319 and 380) and never goes through RenderSentinel.
//
// The pattern is start-anchored and requires a recognized comment prefix
// (`//`, `#`, or `<!--`) so a body line containing "specgraph:init v=2 ..."
// inside arbitrary text (e.g. an example in a markdown rule body) is NOT
// matched. Only deliberate sentinel lines are picked up.
var sentinelMatcher = regexp.MustCompile(
	`^\s*(?://|#|<!--)\s+specgraph:init(?::start)?\s+v=(\d+)(?:\s+sha256=([0-9a-fA-F]+))?(?:\s+rev=(\S+?))?\s*(?:-->)?$`,
)

// RenderSentinel formats a Sentinel as a single line in the given comment
// syntax. The returned line includes the comment delimiters but no trailing
// newline.
//
// For CommentNone, returns the empty string (JSON files don't carry sentinels).
//
// CommentHTML renders a standalone whole-file sentinel (e.g.
// "<!-- specgraph:init v=2 sha256=... -->"). The markdown-block strategy
// writes its `:start`/`:end` markers inline and does not call this function.
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
		// Bare form: no `:start` suffix. Whole-file callers (wholefile.go)
		// emit this as a single standalone sentinel line; the markdown-block
		// strategy emits `<!-- specgraph:init:start ... -->` inline via
		// string concatenation in markdownblock.go and does not go through
		// RenderSentinel.
		return "<!-- " + body + " -->"
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
