// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestFindings(t *testing.T) {
	fs := []*specv1.AnalyticalFinding{
		{
			PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
			Summary:  "Missing constraint coverage",
			Detail:   "Spec does not address constraint C3.",
		},
		{
			PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
			Summary:  "Edge case unhandled",
		},
	}
	got := Findings(fs)
	if !strings.Contains(got, "| Pass | Severity | Summary |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "CONSTITUTION_CHECK") {
		t.Error("missing pass type")
	}
	if !strings.Contains(got, "CRITICAL") {
		t.Error("missing severity")
	}
}

func TestFindingsEmpty(t *testing.T) {
	got := Findings(nil)
	if !strings.Contains(got, "No findings.") {
		t.Error("expected empty message")
	}
}
