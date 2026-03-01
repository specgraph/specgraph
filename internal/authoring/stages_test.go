// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from  string
		to    string
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
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			err := authoring.ValidateTransition(tt.from, tt.to)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestAllStages(t *testing.T) {
	got := authoring.AllStages()
	expected := []string{"spark", "shape", "specify", "decompose", "approved"}
	require.Equal(t, expected, got)

	// Verify it returns a copy, not the original slice.
	got[0] = "mutated"
	require.Equal(t, expected, authoring.AllStages())
}
