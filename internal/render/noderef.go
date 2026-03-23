// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// NodeRefList renders a titled list of node references as a markdown table.
func NodeRefList(title string, refs []*specv1.NodeRef) string {
	if len(refs) == 0 {
		return "None.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)

	headers := []string{"Slug", "Stage"}
	rows := make([][]string, len(refs))
	for i, r := range refs {
		rows[i] = []string{r.Slug, r.Stage}
	}
	b.WriteString(itemTable(headers, rows))
	return b.String()
}
