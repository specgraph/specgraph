// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

func TestPostureToString(t *testing.T) {
	tests := []struct {
		name string
		in   specv1.Posture
		want string
	}{
		{"unspecified → empty", specv1.Posture_POSTURE_UNSPECIFIED, ""},
		{"drive", specv1.Posture_POSTURE_DRIVE, "drive"},
		{"partner", specv1.Posture_POSTURE_PARTNER, "partner"},
		{"support", specv1.Posture_POSTURE_SUPPORT, "support"},
		{"unknown int → empty", specv1.Posture(999), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := postureToString(tt.in)
			require.Equal(t, tt.want, got)
		})
	}
}
