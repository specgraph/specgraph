// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// AnalyticalPass renders a RunAnalyticalPassResponse as markdown optimized
// for LLM consumption. The slug is included in the title for context.
// Returns empty string for nil.
func AnalyticalPass(resp *specv1.RunAnalyticalPassResponse, slug string) string {
	if resp == nil {
		return ""
	}

	var b strings.Builder

	title := passTypeTitle(resp.GetPassType())
	fmt.Fprintf(&b, "# %s -- %s\n\n", title, slug)

	fmt.Fprintf(&b, "## Prompt\n\n%s\n\n", resp.GetPromptTemplate())

	if len(resp.GetTools()) > 0 {
		b.WriteString("## Tools\n\n")
		for _, tool := range resp.GetTools() {
			fmt.Fprintf(&b, "### %s\n", tool.GetName())
			fmt.Fprintf(&b, "%s\n", tool.GetDescription())
			fmt.Fprintf(&b, "```\n%s\n```\n\n", tool.GetCommand())
		}
	}

	fmt.Fprintf(&b, "## Instructions\n\n%s\n", resp.GetInitialMessage())

	return b.String()
}

// passTypeTitle converts a PassType enum to a human-readable title.
func passTypeTitle(pt specv1.PassType) string {
	s := pt.String()
	s = strings.TrimPrefix(s, "PASS_TYPE_")
	parts := strings.Split(strings.ToLower(s), "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, " ")
}
