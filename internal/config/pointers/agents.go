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
	"regexp"
	"strings"
)

const agentsRel = "AGENTS.md"

const initStart = "<!-- specgraph:init:start v=1 -->"
const initEnd = "<!-- specgraph:init:end -->"

// initStartLoose matches an init start marker without the version suffix.
// Catches hand-rolled or pre-spec markers; treated as corruption.
var initStartLoose = regexp.MustCompile(`<!--\s*specgraph:init:start\s*-->`)

// legacyBlock matches inject's per-slug blocks. Slug class mirrors inject's
// safeSlugPattern: `[a-zA-Z0-9][a-zA-Z0-9._-]*`. The (?s) flag lets `.` match
// newlines so the body is captured. Start and end slugs must match (Go regexp
// has no backrefs; ReplaceAllFunc verifies submatch equality). The literal
// slug `init` is never purged here.
var legacyBlock = regexp.MustCompile(
	`(?s)<!--\s*specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):start\s*-->(.*?)<!--\s*specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):end\s*-->\n?`,
)

func renderAgentsBlock(opts Options) string {
	var b strings.Builder
	b.WriteString(initStart)
	b.WriteString("\n")
	b.WriteString("# SpecGraph project pointer\n\n")
	fmt.Fprintf(&b, "Server: %s\n", opts.ServerURL)
	fmt.Fprintf(&b, "Project: %s (sent as the X-Specgraph-Project header)\n\n", opts.ProjectSlug)
	b.WriteString("This block is managed by `specgraph init`. Edit content outside the markers.\n")
	b.WriteString("Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.\n")
	b.WriteString(initEnd)
	return b.String()
}

func syncAgents(projectDir string, opts Options) SyncResult {
	if err := rejectSymlinkComponents(projectDir, agentsRel); err != nil {
		return errResult(agentsRel, err)
	}
	full := filepath.Join(projectDir, agentsRel)

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return errResult(agentsRel, lerr)
	}
	defer unlock()

	existing, rerr := os.ReadFile(full)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return errResult(agentsRel, fmt.Errorf("read %s: %w", full, rerr))
	}

	canonical := renderAgentsBlock(opts)

	// Phase 1: validate existing init markers (corruption rules).
	if len(existing) > 0 {
		if err := validateInitMarkers(agentsRel, existing); err != nil {
			return errResult(agentsRel, err)
		}
	}

	// Phase 2: purge legacy per-slug blocks (post-filter excludes "init").
	updated := existing
	purged := 0
	if len(updated) > 0 {
		updated, purged = purgeLegacyBlocks(updated)
	}

	// Phase 3: reconcile init managed block.
	switch {
	case len(updated) == 0:
		updated = []byte(canonical + "\n")
	case bytes.Contains(updated, []byte(initStart)):
		updated = replaceInitBlock(updated, canonical)
	default:
		// File exists, no init block, no markers — append with leading blank line.
		if !bytes.HasSuffix(updated, []byte("\n")) {
			updated = append(updated, '\n')
		}
		updated = append(updated, '\n')
		updated = append(updated, canonical...)
		updated = append(updated, '\n')
	}

	if len(existing) > 0 && bytes.Equal(existing, updated) {
		return SyncResult{Path: agentsRel, Action: ActionNoOp}
	}

	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if werr := atomicWrite(full, updated, mode); werr != nil {
		return errResult(agentsRel, werr)
	}

	action := ActionUpdated
	if len(existing) == 0 {
		action = ActionCreated
	}
	return SyncResult{Path: agentsRel, Action: action, LegacyBlocksPurged: purged}
}

// validateInitMarkers returns an error for any of the four corruption rules:
// (1) end before start, (2) start without end, (3) double start, (4)
// init start marker missing the v=1 suffix.
func validateInitMarkers(displayName string, data []byte) error {
	starts := bytes.Count(data, []byte(initStart))
	ends := bytes.Count(data, []byte(initEnd))

	// Rule 4: init-shaped marker without the v=1 suffix.
	loose := initStartLoose.FindAllIndex(data, -1)
	canonical := bytes.Index(data, []byte(initStart))
	for _, m := range loose {
		// If a loose match overlaps the canonical (versioned) start, skip it.
		if canonical >= 0 && m[0] == canonical {
			continue
		}
		return fmt.Errorf(
			"%s contains an init marker without the expected v=1 suffix at offset %d; remove the marker or fix it manually",
			displayName,
			m[0],
		)
	}

	switch {
	case starts == 0 && ends == 0:
		return nil
	case starts == 1 && ends == 1:
		// Verify ordering.
		s := bytes.Index(data, []byte(initStart))
		e := bytes.Index(data, []byte(initEnd))
		if e < s {
			return fmt.Errorf("%s: init end marker appears before start marker", displayName)
		}
		return nil
	case starts > 1:
		return fmt.Errorf("%s: more than one init start marker", displayName)
	case starts == 1 && ends == 0:
		return fmt.Errorf("%s: init start marker without matching end", displayName)
	default: // ends > 0, starts == 0
		return fmt.Errorf("%s: init end marker without matching start", displayName)
	}
}

func purgeLegacyBlocks(data []byte) (out []byte, purged int) {
	out = legacyBlock.ReplaceAllFunc(data, func(match []byte) []byte {
		sub := legacyBlock.FindSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		slugStart, slugEnd := string(sub[1]), string(sub[3])
		if slugStart != slugEnd {
			return match
		}
		if slugStart == "init" {
			return match
		}
		purged++
		return nil
	})
	return out, purged
}

func replaceInitBlock(data []byte, canonical string) []byte {
	startIdx := bytes.Index(data, []byte(initStart))
	endIdx := bytes.Index(data, []byte(initEnd))
	if startIdx < 0 || endIdx < startIdx {
		// validateInitMarkers ran first; this should be unreachable.
		return data
	}
	endLen := len(initEnd)
	out := make([]byte, 0, len(data))
	out = append(out, data[:startIdx]...)
	out = append(out, canonical...)
	out = append(out, data[endIdx+endLen:]...)
	return out
}
