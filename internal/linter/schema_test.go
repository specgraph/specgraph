// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package linter_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/linter"
	"github.com/specgraph/specgraph/internal/storage"
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

func TestValidateSchema_InvalidComplexity(t *testing.T) {
	spec := &storage.Spec{
		Slug:       "my-spec",
		Intent:     "Do something",
		Stage:      storage.SpecStageSpark,
		Complexity: storage.SpecComplexity("extreme"),
		Version:    1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.enum", violations[0].Rule)
	require.Equal(t, "complexity", violations[0].Location)
}

func TestValidateSchema_DeclaredProvenance(t *testing.T) {
	spec := &storage.Spec{
		Slug:       "declared-spec",
		Intent:     "Always evolving",
		Stage:      storage.SpecStageDone,
		Provenance: storage.SpecProvenanceDeclared,
		Version:    1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func TestValidateSchema_AbandonedStage(t *testing.T) {
	spec := &storage.Spec{
		Slug:    "abandoned-spec",
		Intent:  "No longer needed",
		Stage:   storage.SpecStageAbandoned,
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations, "abandoned stage should pass schema validation without special fields")
}

func TestValidateSchema_VersionZero(t *testing.T) {
	spec := &storage.Spec{
		Slug:    "zero-ver",
		Intent:  "Has version zero",
		Stage:   storage.SpecStageSpark,
		Version: 0,
	}
	violations := linter.ValidateSchema(spec)
	minViolations := filterByRule(violations, "schema.minimum")
	require.Len(t, minViolations, 1)
	require.Contains(t, minViolations[0].Message, "version must be >= 1")
}

func TestValidateSchema_InvalidProvenance(t *testing.T) {
	spec := &storage.Spec{
		Slug:       "bad-provenance",
		Intent:     "Has invalid provenance",
		Stage:      storage.SpecStageSpark,
		Version:    1,
		Provenance: storage.SpecProvenanceType("bogus"),
	}
	violations := linter.ValidateSchema(spec)
	enumViolations := filterByRule(violations, "schema.enum")
	require.Len(t, enumViolations, 1)
	require.Contains(t, enumViolations[0].Message, "invalid provenance")
}

func TestValidateSchema_NilSpec(t *testing.T) {
	violations := linter.ValidateSchema(nil)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.required", violations[0].Rule)
	require.Contains(t, violations[0].Message, "nil")
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
