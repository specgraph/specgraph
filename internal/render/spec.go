// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Spec renders a single spec as markdown. Returns empty string for nil.
func Spec(s *specv1.Spec) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", s.Slug)
	if s.Intent != "" {
		fmt.Fprintf(&b, "> %s\n\n", s.Intent)
	}

	pairs := [][2]string{
		{"Stage", s.Stage},
		{"Priority", s.Priority},
		{"Complexity", s.Complexity},
		{"Version", fmt.Sprintf("%d", s.Version)},
		{"Provenance", provenanceString(s.GetProvenanceType())},
	}
	b.WriteString(metadataTable(pairs))

	b.WriteString(section(2, "Notes", s.Notes))
	b.WriteString(SparkSection(s.SparkOutput))
	b.WriteString(ShapeSection(s.ShapeOutput))
	b.WriteString(SpecifySection(s.SpecifyOutput))
	b.WriteString(DecomposeSection(s.DecomposeOutput))
	b.WriteString(ConversationLogList(s.ConversationLogs))

	return b.String()
}

// SpecList renders a list of specs as a markdown table.
func SpecList(specs []*specv1.Spec) string {
	if len(specs) == 0 {
		return "No specs found.\n"
	}
	headers := []string{"Slug", "Stage", "Priority", "Intent"}
	rows := make([][]string, len(specs))
	for i, s := range specs {
		rows[i] = []string{s.Slug, s.Stage, s.Priority, s.Intent}
	}
	return itemTable(headers, rows)
}

func provenanceString(p specv1.SpecProvenance) string {
	switch p {
	case specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED:
		return "AUTHORED"
	case specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR:
		return "RETROACTIVE_FROM_PR"
	case specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED:
		return "DECLARED"
	default:
		return "UNSPECIFIED"
	}
}

// Note: a richer renderProvenanceBlock that includes RETROACTIVE/DECLARED
// detail was prototyped but not wired into the metadata table. The table
// format displays only the provenance type via provenanceString; the
// structured detail surfaces in the corresponding stage outputs and via
// the spec's notes. Re-introduce a block renderer if a future use case
// needs the detail rendered separately from the table.
