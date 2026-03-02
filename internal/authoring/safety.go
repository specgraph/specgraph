// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import (
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// SafetyInput holds the text to scan for safety concerns.
type SafetyInput struct {
	Intent     string
	Scope      []string
	Invariants []string
}

// SafetyCategory identifies a class of safety concern.
type SafetyCategory string

// Safety category constants.
const (
	SafetyCategorySecurity SafetyCategory = "security"
	SafetyCategoryDataLoss SafetyCategory = "data_loss"
)

// SafetyFlagResult is the domain-level result of a safety net check.
type SafetyFlagResult struct {
	Category    SafetyCategory
	Severity    specv1.FindingSeverity
	Description string
}

var securityPatterns = []string{
	"plaintext",
	"hardcoded secret",
	"hardcoded password",
	"disable auth",
	"skip validation",
	"no encryption",
	"credential",
	"injection",
	"eval(",
	"exec(",
}

var dataLossPatterns = []string{
	"drop table",
	"drop all",
	"delete all",
	"truncate",
	"without migration",
	"without backup",
	"no rollback",
	"rm -rf",
	"force delete",
	"purge",
}

// RunSafetyNet scans the input for known dangerous patterns and returns flags.
func RunSafetyNet(input *SafetyInput) []SafetyFlagResult {
	parts := make([]string, 0, 1+len(input.Scope)+len(input.Invariants))
	parts = append(parts, input.Intent)
	parts = append(parts, input.Scope...)
	parts = append(parts, input.Invariants...)
	combined := strings.ToLower(strings.Join(parts, " "))

	var flags []SafetyFlagResult

	type patternGroup struct {
		category SafetyCategory
		patterns []string
	}

	groups := []patternGroup{
		{SafetyCategorySecurity, securityPatterns},
		{SafetyCategoryDataLoss, dataLossPatterns},
	}
	for _, g := range groups {
		for _, p := range g.patterns {
			if strings.Contains(combined, p) {
				flags = append(flags, SafetyFlagResult{
					Category:    g.category,
					Severity:    specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
					Description: "matched pattern: " + p,
				})
			}
		}
	}

	return flags
}

// SafetyResultsToProto converts domain safety flags to protobuf SafetyFlag messages.
func SafetyResultsToProto(flags []SafetyFlagResult) []*specv1.SafetyFlag {
	out := make([]*specv1.SafetyFlag, len(flags))
	for i, f := range flags {
		out[i] = &specv1.SafetyFlag{
			Category:    string(f.Category),
			Severity:    f.Severity,
			Description: f.Description,
		}
	}
	return out
}
