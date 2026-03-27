// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// SliceDetail renders a single slice as markdown.
func SliceDetail(s *specv1.Slice) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Slug)
	if s.Intent != "" {
		fmt.Fprintf(&b, "> %s\n\n", s.Intent)
	}

	pairs := [][2]string{
		{"Status", sliceStatusString(s.Status)},
		{"Parent", s.ParentSlug},
	}
	if s.AssignedTo != "" {
		pairs = append(pairs, [2]string{"Assigned To", s.AssignedTo})
	}
	b.WriteString(metadataTable(pairs))

	if len(s.Verify) > 0 {
		b.WriteString("\n## Verify\n\n")
		for _, v := range s.Verify {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}
	if len(s.Touches) > 0 {
		b.WriteString("\n## Touches\n\n")
		for _, t := range s.Touches {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	if len(s.DependsOn) > 0 {
		b.WriteString("\n## Depends On\n\n")
		for _, d := range s.DependsOn {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}
	return b.String()
}

// SliceList renders a list of slices as a table.
func SliceList(slices []*specv1.Slice) string {
	if len(slices) == 0 {
		return "No slices found.\n"
	}
	var b strings.Builder
	b.WriteString("| ID | Intent | Status | Assigned To |\n")
	b.WriteString("|------|--------|--------|-------------|\n")
	for _, s := range slices {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			escapeTableCell(s.SliceId), escapeTableCell(truncate(s.Intent, 40)),
			sliceStatusString(s.Status), escapeTableCell(s.AssignedTo))
	}
	return b.String()
}

// escapeTableCell sanitizes a string for use in a Markdown table cell by
// escaping pipe characters and replacing newlines with spaces.
func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func sliceStatusString(s specv1.SliceStatus) string {
	switch s {
	case specv1.SliceStatus_SLICE_STATUS_OPEN:
		return "open"
	case specv1.SliceStatus_SLICE_STATUS_CLAIMED:
		return "claimed"
	case specv1.SliceStatus_SLICE_STATUS_DONE:
		return "done"
	default:
		return "unknown"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
