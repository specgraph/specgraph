// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package markdown

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// EdgeList renders edges for a slug as a markdown table with direction.
func EdgeList(slug string, edges []*specv1.Edge) string {
	if len(edges) == 0 {
		return "No edges found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Edges for %s\n\n", slug)

	headers := []string{"Type", "Direction", "Target"}
	rows := make([][]string, len(edges))
	for i, e := range edges {
		dir, target := edgeDirection(slug, e)
		rows[i] = []string{edgeTypeName(e.EdgeType), dir, target}
	}
	b.WriteString(ItemTable(headers, rows))
	return b.String()
}

func edgeDirection(slug string, e *specv1.Edge) (direction, target string) {
	if e.FromId == slug {
		return "outgoing", e.ToId
	}
	return "incoming", e.FromId
}

func edgeTypeName(et specv1.EdgeType) string {
	s := et.String()
	return strings.TrimPrefix(s, "EDGE_TYPE_")
}
