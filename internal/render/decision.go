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
	if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED {
		pairs = append(pairs, [2]string{"Confidence", decisionConfidenceName(d.Confidence)})
	}
	if d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
		pairs = append(pairs, [2]string{"Scope", decisionScopeName(d.Scope)})
	}
	if d.OriginSpec != "" {
		pairs = append(pairs, [2]string{"Origin Spec", d.OriginSpec})
	}
	if d.OriginStage != "" {
		pairs = append(pairs, [2]string{"Origin Stage", d.OriginStage})
	}
	if d.Version != 0 {
		pairs = append(pairs, [2]string{"Version", fmt.Sprintf("%d", d.Version)})
	}
	b.WriteString(metadataTable(pairs))

	if d.Decision != "" {
		fmt.Fprintf(&b, "\n**Decision:** %s\n", d.Decision)
	}
	if d.Rationale != "" {
		fmt.Fprintf(&b, "\n**Rationale:** %s\n", d.Rationale)
	}
	if d.Question != "" {
		fmt.Fprintf(&b, "\n**Question:** %s\n", d.Question)
	}
	if len(d.Tags) > 0 {
		fmt.Fprintf(&b, "\n**Tags:** %s\n", strings.Join(d.Tags, ", "))
	}
	if len(d.RejectedAlternatives) > 0 {
		b.WriteString("\n### Rejected Alternatives\n\n")
		headers := []string{"Option", "Reason"}
		rows := make([][]string, len(d.RejectedAlternatives))
		for i, ra := range d.RejectedAlternatives {
			rows[i] = []string{ra.Option, ra.Reason}
		}
		b.WriteString(itemTable(headers, rows))
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

func decisionConfidenceName(c specv1.DecisionConfidence) string {
	return strings.TrimPrefix(c.String(), "DECISION_CONFIDENCE_")
}

func decisionScopeName(s specv1.DecisionScope) string {
	return strings.TrimPrefix(s.String(), "DECISION_SCOPE_")
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
