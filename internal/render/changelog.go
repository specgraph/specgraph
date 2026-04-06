// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// formatInlineHunks renders a slice of InlineDiff hunks as text with
// [-deleted-] and {+inserted+} markers for deleted and inserted text.
func formatInlineHunks(hunks []*specv1.InlineDiff) string {
	var b strings.Builder
	for _, h := range hunks {
		switch h.Op {
		case specv1.InlineDiff_DELETE:
			b.WriteString("[-")
			b.WriteString(h.Text)
			b.WriteString("-]")
		case specv1.InlineDiff_INSERT:
			b.WriteString("{+")
			b.WriteString(h.Text)
			b.WriteString("+}")
		default:
			b.WriteString(h.Text)
		}
	}
	return b.String()
}

// Changes renders changelog entries as a markdown timeline.
func Changes(entries []*specv1.ChangeLogEntry) string {
	if len(entries) == 0 {
		return "No changelog entries found.\n"
	}
	var b strings.Builder
	for _, e := range entries {
		header := fmt.Sprintf("## v%d — %s", e.Version, e.Stage)
		if e.Checkpoint {
			header += " (checkpoint)"
		}
		fmt.Fprintln(&b, header)
		date := ""
		if e.Date != nil {
			date = e.Date.AsTime().Format("2006-01-02")
		}
		fmt.Fprintf(&b, "**%s** | Hash: %s\n", date, e.ContentHash)
		if e.Reason != "" {
			fmt.Fprintf(&b, "Reason: %s\n", e.Reason)
		}
		if e.Summary != "" {
			fmt.Fprintf(&b, "\n%s\n", e.Summary)
		}
		if len(e.Changes) > 0 {
			fmt.Fprintln(&b)
			headers := []string{"Field", "Old", "New"}
			rows := make([][]string, len(e.Changes))
			for i, c := range e.Changes {
				rows[i] = []string{escapePipe(c.Field), escapePipe(c.OldValue), escapePipe(c.NewValue)}
			}
			fmt.Fprint(&b, itemTable(headers, rows))
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// ChangesWithDiff renders changelog entries with inline word-level diffs.
// hunksProvider computes inline diffs for a pair of old/new values.
func ChangesWithDiff(entries []*specv1.ChangeLogEntry, hunksProvider func(oldVal, newVal string) []*specv1.InlineDiff) string {
	if len(entries) == 0 {
		return "No changelog entries found.\n"
	}
	var b strings.Builder
	for _, e := range entries {
		header := fmt.Sprintf("## v%d — %s", e.Version, e.Stage)
		if e.Checkpoint {
			header += " (checkpoint)"
		}
		fmt.Fprintln(&b, header)
		date := ""
		if e.Date != nil {
			date = e.Date.AsTime().Format("2006-01-02")
		}
		fmt.Fprintf(&b, "**%s** | Hash: %s\n", date, e.ContentHash)
		if e.Reason != "" {
			fmt.Fprintf(&b, "Reason: %s\n", e.Reason)
		}
		if e.Summary != "" {
			fmt.Fprintf(&b, "\n%s\n", e.Summary)
		}
		if len(e.Changes) > 0 {
			fmt.Fprintln(&b)
			for _, c := range e.Changes {
				hunks := hunksProvider(c.OldValue, c.NewValue)
				if len(hunks) == 0 {
					fmt.Fprintf(&b, "  %s: %s → %s\n", c.Field, c.OldValue, c.NewValue)
					continue
				}
				fmt.Fprintf(&b, "  %s: %s\n", c.Field, formatInlineHunks(hunks))
			}
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// VersionComparison renders a CompareVersionsResponse as inline diff text.
func VersionComparison(resp *specv1.CompareVersionsResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Comparing v%d → v%d\n", resp.FromVersion, resp.ToVersion)
	if resp.FromStage != "" || resp.ToStage != "" {
		fmt.Fprintf(&b, "Stage: %s → %s\n", resp.FromStage, resp.ToStage)
	}
	if len(resp.Diffs) == 0 {
		fmt.Fprintln(&b, "\nNo differences found.")
		return b.String()
	}
	fmt.Fprintln(&b)
	for _, d := range resp.Diffs {
		fmt.Fprintf(&b, "  %s: %s\n", d.Field, formatInlineHunks(d.Hunks))
	}
	fmt.Fprintln(&b)
	return b.String()
}
