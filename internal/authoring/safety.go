// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import (
	"regexp"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// FindingSeverity indicates how severe a safety finding is.
// Lower values are more severe.
type FindingSeverity int

const (
	SeverityCritical FindingSeverity = 1
	SeverityWarning  FindingSeverity = 2
	SeverityNote     FindingSeverity = 3
)

// SafetyInput holds the text to scan for safety concerns.
// Intent accepts any text for pattern scanning — its name reflects the Spark
// use case, but other handlers pass stage-appropriate text (e.g., risks in Shape).
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
	Severity    FindingSeverity
	Description string
}

// safetyPattern pairs a compiled regex with its severity level.
type safetyPattern struct {
	re       *regexp.Regexp
	severity FindingSeverity
}

// buildPatterns compiles pattern strings into word-boundary-aware regexes.
// Multi-word phrases use substring match (high confidence); single short
// words use word-boundary anchors to reduce false positives.
func buildPatterns(patterns []string, severity FindingSeverity) []safetyPattern {
	out := make([]safetyPattern, len(patterns))
	for i, p := range patterns {
		var expr string
		if strings.ContainsAny(p, " (-") {
			// Multi-word or punctuated patterns: substring match is precise enough.
			expr = regexp.QuoteMeta(p)
		} else {
			// Single-word patterns: require word boundary to reduce false positives.
			expr = `\b` + regexp.QuoteMeta(p) + `\b`
		}
		out[i] = safetyPattern{re: regexp.MustCompile(expr), severity: severity}
	}
	return out
}

// High-confidence patterns that warrant CRITICAL severity.
var criticalSecurityPatterns = buildPatterns([]string{
	"hardcoded secret",
	"hardcoded password",
	"disable auth",
	"skip validation",
	"no encryption",
	"rm -rf",
}, SeverityCritical)

// Ambiguous patterns that may appear in legitimate specs — WARNING severity.
var warningSecurityPatterns = buildPatterns([]string{
	"credential",
	"injection",
	"eval(",
	"exec(",
	"plaintext",
}, SeverityWarning)

var criticalDataLossPatterns = buildPatterns([]string{
	"drop table",
	"drop all",
	"delete all",
	"without migration",
	"without backup",
	"no rollback",
	"force delete",
}, SeverityCritical)

var warningDataLossPatterns = buildPatterns([]string{
	"truncate",
	"purge",
}, SeverityWarning)

type patternGroup struct {
	category SafetyCategory
	patterns []safetyPattern
}

var allPatternGroups = []patternGroup{
	{SafetyCategorySecurity, criticalSecurityPatterns},
	{SafetyCategorySecurity, warningSecurityPatterns},
	{SafetyCategoryDataLoss, criticalDataLossPatterns},
	{SafetyCategoryDataLoss, warningDataLossPatterns},
}

// RunSafetyNet scans the input for known dangerous patterns and returns flags.
// Per category, it emits only the highest-severity match (lowest enum value wins)
// so that a CRITICAL pattern is never suppressed by an earlier WARNING match.
func RunSafetyNet(input *SafetyInput) []SafetyFlagResult {
	parts := make([]string, 0, 1+len(input.Scope)+len(input.Invariants))
	parts = append(parts, input.Intent)
	parts = append(parts, input.Scope...)
	parts = append(parts, input.Invariants...)
	combined := strings.ToLower(strings.Join(parts, " "))

	// Collect every matching pattern across all groups.
	var allMatches []SafetyFlagResult
	for _, g := range allPatternGroups {
		for _, sp := range g.patterns {
			if sp.re.MatchString(combined) {
				allMatches = append(allMatches, SafetyFlagResult{
					Category:    g.category,
					Severity:    sp.severity,
					Description: "matched pattern: " + sp.re.String(),
				})
			}
		}
	}

	// Deduplicate: keep only the highest-severity (lowest enum value) per category.
	best := make(map[SafetyCategory]SafetyFlagResult)
	for _, m := range allMatches {
		existing, ok := best[m.Category]
		if !ok || m.Severity < existing.Severity {
			best[m.Category] = m
		}
	}

	var flags []SafetyFlagResult
	for _, f := range best {
		flags = append(flags, f)
	}

	return flags
}

var severityToProto = map[FindingSeverity]specv1.FindingSeverity{
	SeverityCritical: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
	SeverityWarning:  specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
	SeverityNote:     specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
}

// SafetyResultsToProto converts domain safety flags to protobuf SafetyFlag messages.
func SafetyResultsToProto(flags []SafetyFlagResult) []*specv1.SafetyFlag {
	out := make([]*specv1.SafetyFlag, len(flags))
	for i, f := range flags {
		out[i] = &specv1.SafetyFlag{
			Category:    string(f.Category),
			Severity:    severityToProto[f.Severity],
			Description: f.Description,
		}
	}
	return out
}
