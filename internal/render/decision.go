// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Decision renders a single decision as markdown.
func Decision(d *specv1.Decision) string {
	if d == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", d.Slug)
	if d.Title != "" {
		fmt.Fprintf(&b, "> %s\n\n", d.Title)
	}

	pairs := [][2]string{
		{"Status", decisionStatusString(d.Status)},
	}
	if d.SupersededBy != "" {
		pairs = append(pairs, [2]string{"Superseded By", d.SupersededBy})
	}
	b.WriteString(metadataTable(pairs))

	if d.Decision != "" {
		fmt.Fprintf(&b, "\n**Decision:** %s\n", d.Decision)
	}
	if d.Rationale != "" {
		fmt.Fprintf(&b, "\n**Rationale:** %s\n", d.Rationale)
	}

	return b.String()
}

// DecisionList renders a list of decisions as a markdown table.
func DecisionList(ds []*specv1.Decision) string {
	if len(ds) == 0 {
		return "No decisions found.\n"
	}
	headers := []string{"Slug", "Status", "Title"}
	rows := make([][]string, len(ds))
	for i, d := range ds {
		rows[i] = []string{d.Slug, decisionStatusString(d.Status), d.Title}
	}
	return itemTable(headers, rows)
}

func decisionStatusString(s specv1.DecisionStatus) string {
	switch s {
	case specv1.DecisionStatus_DECISION_STATUS_PROPOSED:
		return "proposed"
	case specv1.DecisionStatus_DECISION_STATUS_ACCEPTED:
		return "accepted"
	case specv1.DecisionStatus_DECISION_STATUS_DEPRECATED:
		return "deprecated"
	case specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED:
		return "superseded"
	default:
		return "unknown"
	}
}
