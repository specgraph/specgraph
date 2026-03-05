// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
)

func TestPassesForStage(t *testing.T) {
	tests := []struct {
		name    string
		stage   authoring.Stage
		posture specv1.Posture
		want    []string
	}{
		{
			name:    "spark/DRIVE returns constitution_check",
			stage:   "spark",
			posture: specv1.Posture_POSTURE_DRIVE,
			want:    []string{"constitution_check"},
		},
		{
			name:    "shape/DRIVE returns peripheral_vision and constitution_check",
			stage:   "shape",
			posture: specv1.Posture_POSTURE_DRIVE,
			want:    []string{"peripheral_vision", "constitution_check"},
		},
		{
			name:    "shape/PARTNER returns constitution_check only",
			stage:   "shape",
			posture: specv1.Posture_POSTURE_PARTNER,
			want:    []string{"constitution_check"},
		},
		{
			name:    "specify/DRIVE returns all three passes",
			stage:   "specify",
			posture: specv1.Posture_POSTURE_DRIVE,
			want:    []string{"red_team", "consistency_check", "constitution_check"},
		},
		{
			name:    "decompose/DRIVE returns simplicity_check and constitution_check",
			stage:   "decompose",
			posture: specv1.Posture_POSTURE_DRIVE,
			want:    []string{"simplicity_check", "constitution_check"},
		},
		{
			name:    "shape/SUPPORT returns constitution_check only",
			stage:   "shape",
			posture: specv1.Posture_POSTURE_SUPPORT,
			want:    []string{"constitution_check"},
		},
		{
			name:    "specify/SUPPORT returns constitution_check only (red_team and consistency_check are offered)",
			stage:   "specify",
			posture: specv1.Posture_POSTURE_SUPPORT,
			want:    []string{"constitution_check"},
		},
		{
			name:    "decompose/SUPPORT returns constitution_check only (simplicity_check is offered)",
			stage:   "decompose",
			posture: specv1.Posture_POSTURE_SUPPORT,
			want:    []string{"constitution_check"},
		},
		{
			name:    "unknown stage returns nil",
			stage:   "nonexistent",
			posture: specv1.Posture_POSTURE_DRIVE,
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
		posture specv1.Posture
		want    []string
	}{
		{
			name:    "shape/PARTNER offers peripheral_vision",
			stage:   "shape",
			posture: specv1.Posture_POSTURE_PARTNER,
			want:    []string{"peripheral_vision"},
		},
		{
			name:    "shape/DRIVE offers nothing (already auto)",
			stage:   "shape",
			posture: specv1.Posture_POSTURE_DRIVE,
			want:    nil,
		},
		{
			name:    "specify/SUPPORT offers red_team and consistency_check",
			stage:   "specify",
			posture: specv1.Posture_POSTURE_SUPPORT,
			want:    []string{"red_team", "consistency_check"},
		},
		{
			name:    "decompose/SUPPORT offers simplicity_check",
			stage:   "decompose",
			posture: specv1.Posture_POSTURE_SUPPORT,
			want:    []string{"simplicity_check"},
		},
		{
			name:    "unknown stage returns nil",
			stage:   "nonexistent",
			posture: specv1.Posture_POSTURE_PARTNER,
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
