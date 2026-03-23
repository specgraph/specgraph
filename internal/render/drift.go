// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// DriftReport renders drift reports as markdown grouped by spec.
func DriftReport(reports []*specv1.DriftReport) string {
	if len(reports) == 0 {
		return "No drift detected.\n"
	}

	var hasContent bool
	for _, r := range reports {
		if len(r.GetItems()) > 0 || r.GetErrorMessage() != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return "No drift detected.\n"
	}

	var b strings.Builder
	for _, r := range reports {
		if len(r.GetItems()) == 0 && r.GetErrorMessage() == "" {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", r.GetSpecSlug())

		if items := r.GetItems(); len(items) > 0 {
			headers := []string{"Type", "Severity", "Description", "Upstream"}
			rows := make([][]string, len(items))
			for i, item := range items {
				rows[i] = []string{
					driftTypeName(item.GetType()),
					driftSeverityName(item.GetSeverity()),
					item.GetDescription(),
					item.GetUpstreamSlug(),
				}
			}
			b.WriteString(itemTable(headers, rows))
		}

		if errMsg := r.GetErrorMessage(); errMsg != "" {
			fmt.Fprintf(&b, "\n**Error:** %s\n", errMsg)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func driftTypeName(dt specv1.DriftType) string {
	s := dt.String()
	return strings.TrimPrefix(s, "DRIFT_TYPE_")
}

func driftSeverityName(ds specv1.DriftSeverity) string {
	s := ds.String()
	return strings.TrimPrefix(s, "DRIFT_SEVERITY_")
}
