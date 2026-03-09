// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestSpecStage_ExcludesReEntry(t *testing.T) {
	terminal := []storage.SpecStage{
		storage.SpecStageAmended,
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
		storage.SpecStageDone,
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
		{storage.SpecStageAmended, false},
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
		storage.SpecStageAmended,
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
	assert.Equal(t, "medium", spec.Complexity)
	assert.Equal(t, int32(1), spec.Version)
	assert.Equal(t, now, spec.CreatedAt)
	assert.Equal(t, now, spec.UpdatedAt)
}
