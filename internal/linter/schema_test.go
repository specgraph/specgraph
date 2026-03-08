// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema_ValidSpec(t *testing.T) {
	spec := &storage.Spec{
		Slug:    "my-spec",
		Intent:  "Do something useful",
		Stage:   storage.SpecStageSpark,
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func TestValidateSchema_MissingRequired(t *testing.T) {
	spec := &storage.Spec{}
	violations := linter.ValidateSchema(spec)

	// slug, intent, stage are required; version < 1 also fires
	requiredViolations := filterByRule(violations, "schema.required")
	require.Len(t, requiredViolations, 3, "expected 3 required-field violations")

	locations := make(map[string]bool)
	for _, v := range requiredViolations {
		locations[v.Location] = true
		require.Equal(t, storage.LintSeverityError, v.Severity)
	}
	require.True(t, locations["slug"])
	require.True(t, locations["intent"])
	require.True(t, locations["stage"])
}

func TestValidateSchema_InvalidStage(t *testing.T) {
	spec := &storage.Spec{
		Slug:    "my-spec",
		Intent:  "Do something",
		Stage:   storage.SpecStage("bogus"),
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.enum", violations[0].Rule)
	require.Equal(t, "stage", violations[0].Location)
}

func TestValidateSchema_SupersededWithoutBy(t *testing.T) {
	spec := &storage.Spec{
		Slug:    "old-spec",
		Intent:  "Legacy thing",
		Stage:   storage.SpecStageSuperseded,
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.conditional", violations[0].Rule)
	require.Equal(t, "superseded_by", violations[0].Location)
}

func TestValidateSchema_SupersededWithBy(t *testing.T) {
	spec := &storage.Spec{
		Slug:         "old-spec",
		Intent:       "Legacy thing",
		Stage:        storage.SpecStageSuperseded,
		SupersededBy: "new-spec",
		Version:      1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func TestValidateSchema_InvalidPriority(t *testing.T) {
	spec := &storage.Spec{
		Slug:     "my-spec",
		Intent:   "Do something",
		Stage:    storage.SpecStageSpark,
		Priority: storage.SpecPriority("p9"),
		Version:  1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.enum", violations[0].Rule)
	require.Equal(t, "priority", violations[0].Location)
}

func TestValidateSchema_LivingLifecycle(t *testing.T) {
	spec := &storage.Spec{
		Slug:      "living-spec",
		Intent:    "Always evolving",
		Stage:     storage.SpecStageApproved,
		Lifecycle: "living",
		Version:   1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func filterByRule(violations []storage.LintViolation, rule string) []storage.LintViolation {
	var filtered []storage.LintViolation
	for _, v := range violations {
		if v.Rule == rule {
			filtered = append(filtered, v)
		}
	}
	return filtered
}
