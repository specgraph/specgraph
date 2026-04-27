// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/specgraph/specgraph/internal/storage"
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

func TestIsAuthoringStage(t *testing.T) {
	t.Run("known authoring stages return true", func(t *testing.T) {
		known := []storage.SpecStage{
			storage.SpecStageSpark,
			storage.SpecStageShape,
			storage.SpecStageSpecify,
			storage.SpecStageDecompose,
			storage.SpecStageApproved,
		}
		for _, s := range known {
			require.True(t, authoring.IsAuthoringStage(s), "expected %q to be an authoring stage", s)
		}
	})

	t.Run("non-authoring stages return false", func(t *testing.T) {
		nonAuthoring := []storage.SpecStage{
			storage.SpecStageInProgress,
			storage.SpecStageReview,
			storage.SpecStageDone,
			storage.SpecStageSuperseded,
			storage.SpecStageAbandoned,
		}
		for _, s := range nonAuthoring {
			require.False(t, authoring.IsAuthoringStage(s), "expected %q to not be an authoring stage", s)
		}
	})

	t.Run("unknown stage returns false", func(t *testing.T) {
		require.False(t, authoring.IsAuthoringStage(storage.SpecStage("typo")))
	})

	t.Run("empty string returns false", func(t *testing.T) {
		require.False(t, authoring.IsAuthoringStage(storage.SpecStage("")))
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

func TestStageFromStorage(t *testing.T) {
	cases := []struct {
		name      string
		in        storage.SpecStage
		wantStage authoring.Stage
		wantOk    bool
	}{
		{"spark funnel value", storage.SpecStageSpark, authoring.StageSpark, true},
		{"shape funnel value", storage.SpecStageShape, authoring.StageShape, true},
		{"specify funnel value", storage.SpecStageSpecify, authoring.StageSpecify, true},
		{"decompose funnel value", storage.SpecStageDecompose, authoring.StageDecompose, true},
		{"approved funnel value", storage.SpecStageApproved, authoring.StageApproved, true},
		{"in_progress lifecycle value rejected", storage.SpecStageInProgress, authoring.Stage(""), false},
		{"done lifecycle value rejected", storage.SpecStageDone, authoring.Stage(""), false},
		{"superseded lifecycle value rejected", storage.SpecStageSuperseded, authoring.Stage(""), false},
		{"abandoned lifecycle value rejected", storage.SpecStageAbandoned, authoring.Stage(""), false},
		{"review lifecycle value rejected", storage.SpecStageReview, authoring.Stage(""), false},
		{"empty rejected", storage.SpecStage(""), authoring.Stage(""), false},
		{"unknown rejected", storage.SpecStage("bogus"), authoring.Stage(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStage, gotOk := authoring.StageFromStorage(tc.in)
			if gotStage != tc.wantStage || gotOk != tc.wantOk {
				t.Errorf("StageFromStorage(%q) = (%q, %v), want (%q, %v)",
					string(tc.in), string(gotStage), gotOk, string(tc.wantStage), tc.wantOk)
			}
		})
	}
}

func TestStage_AsStorage_RoundTrip(t *testing.T) {
	for _, s := range []authoring.Stage{
		authoring.StageSpark,
		authoring.StageShape,
		authoring.StageSpecify,
		authoring.StageDecompose,
		authoring.StageApproved,
	} {
		narrow, ok := authoring.StageFromStorage(s.AsStorage())
		if !ok || narrow != s {
			t.Errorf("round-trip Stage(%q).AsStorage().StageFromStorage = (%q, %v), want (%q, true)",
				string(s), string(narrow), ok, string(s))
		}
	}
}
