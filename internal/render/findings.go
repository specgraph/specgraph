// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Findings renders analytical findings as a markdown table.
func Findings(fs []*specv1.AnalyticalFinding) string {
	if len(fs) == 0 {
		return "No findings.\n"
	}
	headers := []string{"Pass", "Severity", "Summary", "Detail"}
	rows := make([][]string, len(fs))
	for i, f := range fs {
		rows[i] = []string{
			passTypeName(f.GetPassType()),
			findingSeverityName(f.GetSeverity()),
			f.GetSummary(),
			f.GetDetail(),
		}
	}
	return itemTable(headers, rows)
}

func passTypeName(pt specv1.PassType) string {
	s := pt.String()
	return strings.TrimPrefix(s, "PASS_TYPE_")
}

func findingSeverityName(fs specv1.FindingSeverity) string {
	s := fs.String()
	return strings.TrimPrefix(s, "FINDING_SEVERITY_")
}
