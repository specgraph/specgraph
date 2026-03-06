// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/seanb4t/specgraph/internal/authoring"
)

func TestPassesForStage(t *testing.T) {
	tests := []struct {
		name    string
		stage   authoring.Stage
		posture authoring.Posture
		want    []string
	}{
		{
			name:    "spark/DRIVE returns constitution_check",
			stage:   "spark",
			posture: authoring.PostureDrive,
			want:    []string{"constitution_check"},
		},
		{
			name:    "shape/DRIVE returns peripheral_vision and constitution_check",
			stage:   "shape",
			posture: authoring.PostureDrive,
			want:    []string{"peripheral_vision", "constitution_check"},
		},
		{
			name:    "shape/PARTNER returns constitution_check only",
			stage:   "shape",
			posture: authoring.PosturePartner,
			want:    []string{"constitution_check"},
		},
		{
			name:    "specify/DRIVE returns all three passes",
			stage:   "specify",
			posture: authoring.PostureDrive,
			want:    []string{"red_team", "consistency_check", "constitution_check"},
		},
		{
			name:    "decompose/DRIVE returns simplicity_check and constitution_check",
			stage:   "decompose",
			posture: authoring.PostureDrive,
			want:    []string{"simplicity_check", "constitution_check"},
		},
		{
			name:    "shape/SUPPORT returns constitution_check only",
			stage:   "shape",
			posture: authoring.PostureSupport,
			want:    []string{"constitution_check"},
		},
		{
			name:    "specify/SUPPORT returns constitution_check only (red_team and consistency_check are offered)",
			stage:   "specify",
			posture: authoring.PostureSupport,
			want:    []string{"constitution_check"},
		},
		{
			name:    "decompose/SUPPORT returns constitution_check only (simplicity_check is offered)",
			stage:   "decompose",
			posture: authoring.PostureSupport,
			want:    []string{"constitution_check"},
		},
		{
			name:    "unknown stage returns nil",
			stage:   "nonexistent",
			posture: authoring.PostureDrive,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authoring.PassesForStage(tt.stage, tt.posture)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOfferedPasses(t *testing.T) {
	tests := []struct {
		name    string
		stage   authoring.Stage
		posture authoring.Posture
		want    []string
	}{
		{
			name:    "shape/PARTNER offers peripheral_vision",
			stage:   "shape",
			posture: authoring.PosturePartner,
			want:    []string{"peripheral_vision"},
		},
		{
			name:    "shape/DRIVE offers nothing (already auto)",
			stage:   "shape",
			posture: authoring.PostureDrive,
			want:    nil,
		},
		{
			name:    "specify/SUPPORT offers red_team and consistency_check",
			stage:   "specify",
			posture: authoring.PostureSupport,
			want:    []string{"red_team", "consistency_check"},
		},
		{
			name:    "decompose/SUPPORT offers simplicity_check",
			stage:   "decompose",
			posture: authoring.PostureSupport,
			want:    []string{"simplicity_check"},
		},
		{
			name:    "unknown stage returns nil",
			stage:   "nonexistent",
			posture: authoring.PosturePartner,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authoring.OfferedPasses(tt.stage, tt.posture)
			require.Equal(t, tt.want, got)
		})
	}
}
