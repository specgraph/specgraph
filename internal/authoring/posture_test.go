// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
)

func TestDetectPosture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []string
		want     specv1.Posture
	}{
		{
			name:     "short vague -> Drive",
			messages: []string{"do it", "yes", "ok"},
			want:     specv1.Posture_POSTURE_DRIVE,
		},
		{
			name: "long detailed -> Support",
			messages: []string{
				strings.Repeat("I want a detailed explanation of the architecture and all subsystems involved. ", 2),
			},
			want: specv1.Posture_POSTURE_SUPPORT,
		},
		{
			name:     "medium -> Partner",
			messages: []string{"Here is a reasonable description of what I need", "And some more context about it"},
			want:     specv1.Posture_POSTURE_PARTNER,
		},
		{
			name:     "empty -> Partner",
			messages: []string{},
			want:     specv1.Posture_POSTURE_PARTNER,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := authoring.DetectPosture(tt.messages)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePosture(t *testing.T) {
	t.Parallel()

	t.Run("explicit DRIVE overrides detection", func(t *testing.T) {
		t.Parallel()
		// Messages would detect as Partner, but explicit wins.
		msgs := []string{"Here is a reasonable description of what I need"}
		got := authoring.ResolvePosture(specv1.Posture_POSTURE_DRIVE, msgs)
		require.Equal(t, specv1.Posture_POSTURE_DRIVE, got)
	})

	t.Run("UNSPECIFIED falls through to detect", func(t *testing.T) {
		t.Parallel()
		msgs := []string{"do it", "yes", "ok"}
		got := authoring.ResolvePosture(specv1.Posture_POSTURE_UNSPECIFIED, msgs)
		require.Equal(t, specv1.Posture_POSTURE_DRIVE, got)
	})
}
