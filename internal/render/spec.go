// SPDX-License-Identifier: MIT
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
		{"Lifecycle", lifecycleString(s.Lifecycle)},
	}
	b.WriteString(metadataTable(pairs))

	b.WriteString(section(2, "Notes", s.Notes))

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

func lifecycleString(lc specv1.SpecLifecycle) string {
	switch lc {
	case specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK:
		return "task"
	case specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING:
		return "living"
	default:
		return "task"
	}
}
