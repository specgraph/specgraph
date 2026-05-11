// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Vestigial v=1 renderers. Reproduce the body bytes the deleted
// pointers/ package would have written inside <!-- specgraph:init:start
// v=1 --> ... <!-- specgraph:init:end --> markers.
//
// Used only by:
//
//	(a) markdownBlockStrategy's v=1 → v=2 upgrade hash-check
//	(b) supersedesGuardedDelete's prior-canonical computation for
//	    .cursor/rules/specgraph-bootstrap.md
//
// Not on the production write path — new writes emit v=2 with hash
// sentinels.
//
// Sunset trigger: parent design's "zero v=1 files for two consecutive
// releases" AND "6 months elapsed since v=2 rollout" — see spec
// §"Helpers ported" / "Sunset trigger correction."
package managedfiles

import (
	"fmt"
	"strings"
)

// renderV1AgentsBlockBody returns the body bytes (between markers,
// no markers themselves) for AGENTS.md's v=1 block. Verbatim port
// of pointers/agents.go:41-52 minus the start/end marker writes.
func renderV1AgentsBlockBody(p ProjectParams) []byte {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("# SpecGraph project pointer\n\n")
	fmt.Fprintf(&b, "Server: %s\n", p.ServerURL)
	fmt.Fprintf(&b, "Project: %s (sent as the X-Specgraph-Project header)\n\n", p.ProjectSlug())
	b.WriteString("This block is managed by `specgraph init`. Edit content outside the markers.\n")
	b.WriteString("Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.\n")
	return []byte(b.String())
}

// renderV1CursorBlockBody returns the body bytes for the v=1 cursor
// bootstrap rule's block. pointers/cursor.go:25-27 delegates to
// renderAgentsBlock; we preserve that identity.
func renderV1CursorBlockBody(p ProjectParams) []byte {
	return renderV1AgentsBlockBody(p)
}

// ProjectSlug returns p.Slug; method exists because the pointers/
// helpers referenced opts.ProjectSlug as a field. Keeping the method
// name preserves the verbatim character of the port.
func (p ProjectParams) ProjectSlug() string { return p.Slug }
