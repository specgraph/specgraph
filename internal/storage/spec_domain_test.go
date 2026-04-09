// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecStage_ExcludesReEntry(t *testing.T) {
	terminal := []storage.SpecStage{
		storage.SpecStageDone,
		storage.SpecStageSuperseded,
		storage.SpecStageAbandoned,
	}
	for _, s := range terminal {
		assert.True(t, s.ExcludesReEntry(), "stage %q should exclude re-entry", s)
	}

	nonTerminal := []storage.SpecStage{
		storage.SpecStageSpark,
		storage.SpecStageShape,
		storage.SpecStageSpecify,
		storage.SpecStageDecompose,
		storage.SpecStageApproved,
		storage.SpecStageInProgress,
		storage.SpecStageReview,
	}
	for _, s := range nonTerminal {
		assert.False(t, s.ExcludesReEntry(), "stage %q should not exclude re-entry", s)
	}
}

func TestSpecStage_IsFullyTerminal(t *testing.T) {
	tests := []struct {
		stage    storage.SpecStage
		expected bool
	}{
		{storage.SpecStageSuperseded, true},
		{storage.SpecStageAbandoned, true},
		{storage.SpecStageDone, false},
		{storage.SpecStageSpark, false},
		{storage.SpecStageShape, false},
		{storage.SpecStageSpecify, false},
		{storage.SpecStageDecompose, false},
		{storage.SpecStageApproved, false},
		{storage.SpecStageInProgress, false},
		{storage.SpecStageReview, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.stage), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.stage.IsFullyTerminal())
		})
	}
}

func TestFullyTerminalStages(t *testing.T) {
	stages := storage.FullyTerminalStages()
	assert.ElementsMatch(t, []storage.SpecStage{
		storage.SpecStageSuperseded,
		storage.SpecStageAbandoned,
	}, stages)
}

func TestSpecStage_IsValid(t *testing.T) {
	valid := []storage.SpecStage{
		storage.SpecStageSpark,
		storage.SpecStageShape,
		storage.SpecStageSpecify,
		storage.SpecStageDecompose,
		storage.SpecStageApproved,
		storage.SpecStageInProgress,
		storage.SpecStageReview,
		storage.SpecStageDone,
		storage.SpecStageSuperseded,
		storage.SpecStageAbandoned,
	}
	for _, s := range valid {
		t.Run(string(s), func(t *testing.T) {
			assert.True(t, s.IsValid(), "stage %q should be valid", s)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		assert.False(t, storage.SpecStage("bogus").IsValid())
	})
}

func TestSpecStage_IsAmendEligible(t *testing.T) {
	eligible := []storage.SpecStage{
		storage.SpecStageApproved,
		storage.SpecStageInProgress,
		storage.SpecStageReview,
	}
	for _, s := range eligible {
		require.True(t, s.IsAmendEligible(), "stage %q should be amend-eligible", s)
	}

	ineligible := []storage.SpecStage{
		storage.SpecStageSpark,
		storage.SpecStageShape,
		storage.SpecStageSpecify,
		storage.SpecStageDecompose,
		storage.SpecStageDone,
		storage.SpecStageSuperseded,
		storage.SpecStageAbandoned,
	}
	for _, s := range ineligible {
		require.False(t, s.IsAmendEligible(), "stage %q should not be amend-eligible", s)
	}
}

func TestSpecStage_PrecedingAuthStage(t *testing.T) {
	tests := []struct {
		stage    storage.SpecStage
		expected storage.SpecStage
	}{
		// Each authoring stage maps to the one before it.
		{storage.SpecStageSpark, storage.SpecStageSpark},       // first stage: returns self
		{storage.SpecStageShape, storage.SpecStageSpark},       // shape ← spark
		{storage.SpecStageSpecify, storage.SpecStageShape},     // specify ← shape
		{storage.SpecStageDecompose, storage.SpecStageSpecify}, // decompose ← specify
		{storage.SpecStageApproved, storage.SpecStageDecompose}, // approved ← decompose
		// Non-authoring stages: not in authoringStages sequence, return self.
		{storage.SpecStageInProgress, storage.SpecStageInProgress},
		{storage.SpecStageReview, storage.SpecStageReview},
		{storage.SpecStageDone, storage.SpecStageDone},
		{storage.SpecStageSuperseded, storage.SpecStageSuperseded},
		{storage.SpecStageAbandoned, storage.SpecStageAbandoned},
	}
	for _, tc := range tests {
		t.Run(string(tc.stage), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.stage.PrecedingAuthStage())
		})
	}
}

func TestSpecLifecycle_IsValid(t *testing.T) {
	tests := []struct {
		lifecycle storage.SpecLifecycle
		expected  bool
	}{
		{storage.SpecLifecycleTask, true},
		{storage.SpecLifecycleLiving, true},
		{storage.SpecLifecycle("bogus"), false},
		{storage.SpecLifecycle(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.lifecycle), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.lifecycle.IsValid())
		})
	}
}

func TestSpecPriority_IsValid(t *testing.T) {
	tests := []struct {
		priority storage.SpecPriority
		expected bool
	}{
		{storage.SpecPriorityP0, true},
		{storage.SpecPriorityP1, true},
		{storage.SpecPriorityP2, true},
		{storage.SpecPriorityP3, true},
		{storage.SpecPriority("p4"), false},
		{storage.SpecPriority(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.priority), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.priority.IsValid())
		})
	}
}

func TestSpecComplexity_IsValid(t *testing.T) {
	valid := []storage.SpecComplexity{
		storage.SpecComplexityLow,
		storage.SpecComplexityMedium,
		storage.SpecComplexityHigh,
	}
	for _, c := range valid {
		t.Run(string(c), func(t *testing.T) {
			assert.True(t, c.IsValid(), "complexity %q should be valid", c)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		assert.False(t, storage.SpecComplexity("bogus").IsValid())
	})
	t.Run("empty", func(t *testing.T) {
		assert.False(t, storage.SpecComplexity("").IsValid())
	})
}

func TestNewSpec(t *testing.T) {
	now := time.Now().UTC()
	spec := &storage.Spec{
		ID:         "spec-abc1234",
		Slug:       "login-api",
		Intent:     "REST endpoint for OAuth2",
		Stage:      storage.SpecStageSpark,
		Priority:   storage.SpecPriorityP1,
		Complexity: "medium",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	assert.Equal(t, "spec-abc1234", spec.ID)
	assert.Equal(t, "login-api", spec.Slug)
	assert.Equal(t, "REST endpoint for OAuth2", spec.Intent)
	assert.Equal(t, storage.SpecStageSpark, spec.Stage)
	assert.Equal(t, storage.SpecPriorityP1, spec.Priority)
	assert.Equal(t, storage.SpecComplexityMedium, spec.Complexity)
	assert.Equal(t, int32(1), spec.Version)
	assert.Equal(t, now, spec.CreatedAt)
	assert.Equal(t, now, spec.UpdatedAt)
}
