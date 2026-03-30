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
