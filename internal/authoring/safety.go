// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
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
// Text accepts any stage-appropriate content:
//   - Spark: seed idea text
//   - Shape: risks list (joined)
//   - Specify: interface_contract
//   - Decompose: slice intents (joined)
type SafetyInput struct {
	Text       string
	Scope      []string
	Invariants []string
}

// Validate returns an error if the SafetyInput has no scannable content.
// An empty Text with populated Scope or Invariants is accepted because
// those fields contribute to the combined scan text.
func (s *SafetyInput) Validate() error {
	if s.Text == "" && len(s.Scope) == 0 && len(s.Invariants) == 0 {
		return fmt.Errorf("safety input has no scannable content")
	}
	return nil
}

// SafetyCategory identifies a class of safety concern.
type SafetyCategory int

// String returns the human-readable name of the safety category.
func (c SafetyCategory) String() string {
	switch c {
	case SafetyCategorySecurity:
		return "security"
	case SafetyCategoryDataLoss:
		return "data_loss"
	default:
		return "unknown"
	}
}

// Safety category constants.
const (
	SafetyCategorySecurity SafetyCategory = iota + 1
	SafetyCategoryDataLoss
)

// SafetyFlagResult is the domain-level result of a safety net check.
type SafetyFlagResult struct {
	Category    SafetyCategory
	Severity    FindingSeverity
	Description string
}

// safetyPattern pairs a compiled regex with its severity level and a
// human-readable label used in API responses (never exposes the raw regex).
type safetyPattern struct {
	re       *regexp.Regexp
	severity FindingSeverity
	label    string
}

// buildPatterns compiles pattern strings into word-boundary-aware regexes.
// Patterns containing spaces, parens, or hyphens use substring match (high
// confidence); single-word patterns use word-boundary anchors to reduce
// false positives. label is the human-readable description used in
// SafetyFlagResult.Description.
func buildPatterns(label string, patterns []string, severity FindingSeverity) []safetyPattern {
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
		out[i] = safetyPattern{re: regexp.MustCompile(expr), severity: severity, label: label}
	}
	return out
}

// High-confidence patterns that warrant CRITICAL severity.
var criticalSecurityPatterns = buildPatterns(
	"hardcoded credentials or disabled security control detected",
	[]string{
		"hardcoded secret",
		"hardcoded password",
		"disable auth",
		"skip validation",
		"no encryption",
		"rm -rf",
	},
	SeverityCritical,
)

// Ambiguous patterns that may appear in legitimate specs — WARNING severity.
var warningSecurityPatterns = buildPatterns(
	"potentially sensitive security-related term detected",
	[]string{
		"credential",
		"injection",
		"eval(",
		"exec(",
		"plaintext",
	},
	SeverityWarning,
)

var criticalDataLossPatterns = buildPatterns(
	"irreversible data destruction operation detected",
	[]string{
		"drop table",
		"drop all",
		"delete all",
		"without migration",
		"without backup",
		"no rollback",
		"force delete",
	},
	SeverityCritical,
)

var warningDataLossPatterns = buildPatterns(
	"potentially destructive data operation detected",
	[]string{
		"truncate",
		"purge",
	},
	SeverityWarning,
)

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
// so that a CRITICAL pattern is never masked by a co-occurring WARNING match.
func RunSafetyNet(input *SafetyInput) []SafetyFlagResult {
	if input == nil {
		return nil
	}
	parts := make([]string, 0, 1+len(input.Scope)+len(input.Invariants))
	parts = append(parts, input.Text)
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
					Description: fmt.Sprintf("[%s] %s", g.category, sp.label),
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

	flags := make([]SafetyFlagResult, 0, len(best))
	for _, f := range best {
		flags = append(flags, f)
	}
	// Sort by category for deterministic output.
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Category < flags[j].Category
	})

	return flags
}

var severityToProto = map[FindingSeverity]specv1.FindingSeverity{
	SeverityCritical: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
	SeverityWarning:  specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
	SeverityNote:     specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
}

var categoryToProto = map[SafetyCategory]specv1.SafetyCategory{
	SafetyCategorySecurity: specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY,
	SafetyCategoryDataLoss: specv1.SafetyCategory_SAFETY_CATEGORY_DATA_LOSS,
}

// SafetyResultsToProto converts domain safety flags to protobuf SafetyFlag messages.
func SafetyResultsToProto(flags []SafetyFlagResult) []*specv1.SafetyFlag {
	out := make([]*specv1.SafetyFlag, len(flags))
	for i, f := range flags {
		protoSev, ok := severityToProto[f.Severity]
		if !ok {
			protoSev = specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
		}
		protoCat, ok := categoryToProto[f.Category]
		if !ok {
			protoCat = specv1.SafetyCategory_SAFETY_CATEGORY_UNSPECIFIED
		}
		out[i] = &specv1.SafetyFlag{
			Category:    protoCat,
			Severity:    protoSev,
			Description: f.Description,
		}
	}
	return out
}

// severityToStorage maps authoring.FindingSeverity (ordered int) to
// storage.FindingSeverity (string, used for JSON persistence).
// The two types are intentionally different: authoring uses ordered ints
// for severity comparison; storage uses strings for human-readable JSON output.
var severityToStorage = map[FindingSeverity]storage.FindingSeverity{
	SeverityCritical: storage.SeverityCritical,
	SeverityWarning:  storage.SeverityWarning,
	SeverityNote:     storage.SeverityNote,
}

// ToStorageSeverity converts an authoring FindingSeverity to the storage
// representation. Unknown values map to the empty string.
func ToStorageSeverity(s FindingSeverity) storage.FindingSeverity {
	return severityToStorage[s]
}
