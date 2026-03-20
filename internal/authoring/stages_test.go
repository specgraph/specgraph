// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from  authoring.Stage
		to    authoring.Stage
		valid bool
	}{
		// Valid forward transitions.
		{"", "spark", true},
		{"spark", "shape", true},
		{"shape", "specify", true},
		{"specify", "decompose", true},
		{"decompose", "approved", true},

		// Valid backward (amend) transitions.
		{"shape", "spark", true},
		{"specify", "shape", true},
		{"decompose", "specify", true},
		{"approved", "decompose", true},
		{"approved", "spark", true},

		// Invalid transitions (skip forward or same-to-same).
		{"spark", "specify", false},
		{"spark", "approved", false},
		{"shape", "decompose", false},
		{"spark", "spark", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := authoring.ValidateTransition(tt.from, tt.to)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateTransition_UnknownStages(t *testing.T) {
	t.Run("unknown from-stage", func(t *testing.T) {
		err := authoring.ValidateTransition("nonexistent", "shape")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown stage")
		require.Contains(t, err.Error(), "nonexistent")
	})

	t.Run("unknown to-stage", func(t *testing.T) {
		err := authoring.ValidateTransition("spark", "nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown stage")
		require.Contains(t, err.Error(), "nonexistent")
	})

	t.Run("both stages unknown", func(t *testing.T) {
		err := authoring.ValidateTransition("foo", "bar")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown stage")
		require.Contains(t, err.Error(), "foo")
		require.Contains(t, err.Error(), "bar")
	})

	t.Run("empty from with valid to (initial transition)", func(t *testing.T) {
		err := authoring.ValidateTransition("", "spark")
		require.NoError(t, err)
	})

	t.Run("empty from with non-first stage is invalid", func(t *testing.T) {
		err := authoring.ValidateTransition("", "shape")
		require.Error(t, err)
	})
}

func TestValidateAmendTransition(t *testing.T) {
	tests := []struct {
		from  authoring.Stage
		to    authoring.Stage
		valid bool
	}{
		// Valid backward transitions.
		{"shape", "spark", true},
		{"specify", "shape", true},
		{"specify", "spark", true},
		{"decompose", "specify", true},
		{"decompose", "spark", true},
		{"approved", "decompose", true},
		{"approved", "spark", true},

		// Invalid: forward transitions must be rejected.
		{"spark", "shape", false},
		{"shape", "specify", false},
		{"specify", "decompose", false},
		{"decompose", "approved", false},

		// Invalid: same-to-same.
		{"spark", "spark", false},
		{"approved", "approved", false},

		// Invalid: initial transition (empty from) is not a backward transition.
		{"", "spark", false},

		// Invalid: unknown stages.
		{"nonexistent", "spark", false},
		{"spark", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			err := authoring.ValidateAmendTransition(tt.from, tt.to)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestStageToStorage(t *testing.T) {
	t.Run("known stages convert without error", func(t *testing.T) {
		known := []authoring.Stage{
			authoring.StageSpark,
			authoring.StageShape,
			authoring.StageSpecify,
			authoring.StageDecompose,
			authoring.StageApproved,
		}
		for _, s := range known {
			v, err := s.ToStorage()
			require.NoError(t, err)
			require.Equal(t, string(s), string(v))
		}
	})

	t.Run("unknown stage returns error", func(t *testing.T) {
		_, err := authoring.Stage("typo").ToStorage()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown authoring stage")
		require.Contains(t, err.Error(), "typo")
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, err := authoring.Stage("").ToStorage()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown authoring stage")
	})
}

func TestAllStages(t *testing.T) {
	got := authoring.AllStages()
	expected := []string{"spark", "shape", "specify", "decompose", "approved"}
	require.Equal(t, expected, got)

	// Verify it returns a copy, not the original slice.
	got[0] = "mutated"
	require.Equal(t, expected, authoring.AllStages())
}
