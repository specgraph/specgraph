// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter

import (
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// ValidateSchema validates a spec against the spec schema rules.
// Returns lint violations using domain types.
func ValidateSchema(spec *storage.Spec) []storage.LintViolation {
	if spec == nil {
		return []storage.LintViolation{{
			Rule:     "schema.required",
			Severity: storage.LintSeverityError,
			Message:  "spec is nil",
		}}
	}
	var violations []storage.LintViolation

	// Required fields
	if spec.Slug == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "slug is required", Location: "slug",
		})
	}
	if spec.Intent == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "intent is required", Location: "intent",
		})
	}
	if spec.Stage == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "stage is required", Location: "stage",
		})
	}

	// Validate stage enum
	if spec.Stage != "" && !spec.Stage.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid stage %q", spec.Stage),
			Location: "stage",
		})
	}

	// Validate priority enum
	if spec.Priority != "" && !spec.Priority.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid priority %q", spec.Priority),
			Location: "priority",
		})
	}

	// Validate complexity enum
	if spec.Complexity != "" && !spec.Complexity.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid complexity %q", spec.Complexity),
			Location: "complexity",
		})
	}

	// Validate lifecycle enum
	if spec.Lifecycle != "" && !spec.Lifecycle.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid lifecycle %q", spec.Lifecycle),
			Location: "lifecycle",
		})
	}

	// Conditional: superseded spec must have superseded_by
	if spec.Stage == storage.SpecStageSuperseded && spec.SupersededBy == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.conditional", Severity: storage.LintSeverityError,
			Message:  "superseded spec must have superseded_by field set",
			Location: "superseded_by",
		})
	}

	// Version minimum
	if spec.Version < 1 {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.minimum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("version must be >= 1, got %d", spec.Version),
			Location: "version",
		})
	}

	return violations
}
