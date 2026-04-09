// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package render converts protobuf types into markdown strings for CLI output.
package render

import (
	"fmt"
	"strings"
)

// metadataTable renders a two-column | Field | Value | markdown table.
// Returns empty string if pairs is empty.
func metadataTable(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	for _, p := range pairs {
		fmt.Fprintf(&b, "| %s | %s |\n", p[0], p[1])
	}
	return b.String()
}

// itemTable renders a multi-column markdown table.
// Returns empty string if rows is empty.
func itemTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "| %s |\n", strings.Join(headers, " | "))
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Fprintf(&b, "| %s |\n", strings.Join(seps, " | "))
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s |\n", strings.Join(row, " | "))
	}
	return b.String()
}

// section renders a markdown heading with body. Returns empty string if body is empty.
func section(level int, title, body string) string {
	if body == "" {
		return ""
	}
	prefix := strings.Repeat("#", level)
	return fmt.Sprintf("%s %s\n\n%s\n", prefix, title, body)
}
