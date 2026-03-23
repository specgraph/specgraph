// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestDriftReport(t *testing.T) {
	reports := []*specv1.DriftReport{
		{
			SpecSlug: "login-api",
			Items: []*specv1.DriftItem{
				{
					Type:         specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
					Severity:     specv1.DriftSeverity_DRIFT_SEVERITY_HIGH,
					Description:  "upstream token-storage changed",
					UpstreamSlug: "token-storage",
				},
			},
		},
	}
	got := DriftReport(reports)
	if !strings.Contains(got, "## login-api") {
		t.Error("missing spec heading")
	}
	if !strings.Contains(got, "| DEPENDENCY | HIGH | upstream token-storage changed | token-storage |") {
		t.Error("missing drift item row")
	}
}

func TestDriftReportWithError(t *testing.T) {
	reports := []*specv1.DriftReport{
		{SpecSlug: "broken", ErrorMessage: "storage unavailable"},
	}
	got := DriftReport(reports)
	if !strings.Contains(got, "**Error:** storage unavailable") {
		t.Error("missing error message")
	}
}

func TestDriftReportEmpty(t *testing.T) {
	got := DriftReport(nil)
	if !strings.Contains(got, "No drift detected.") {
		t.Error("expected empty message")
	}
}
