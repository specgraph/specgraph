// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/authoring"
)

func TestDetectPosture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []string
		want     authoring.Posture
	}{
		{
			name:     "short vague -> Drive",
			messages: []string{"do it", "yes", "ok"},
			want:     authoring.PostureDrive,
		},
		{
			name: "long detailed -> Support",
			messages: []string{
				strings.Repeat("I want a detailed explanation of the architecture and all subsystems involved. ", 2),
			},
			want: authoring.PostureSupport,
		},
		{
			name:     "medium -> Partner",
			messages: []string{"Here is a reasonable description of what I need", "And some more context about it"},
			want:     authoring.PosturePartner,
		},
		{
			name:     "empty -> Partner",
			messages: []string{},
			want:     authoring.PosturePartner,
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

func TestDetectPosture_Boundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []string
		want     authoring.Posture
	}{
		{
			name:     "avg exactly 19 (below driveThreshold) -> Drive",
			messages: []string{strings.Repeat("x", 19)},
			want:     authoring.PostureDrive,
		},
		{
			name:     "avg exactly 20 (at driveThreshold) -> Partner",
			messages: []string{strings.Repeat("x", 20)},
			want:     authoring.PosturePartner,
		},
		{
			name:     "avg exactly 21 (above driveThreshold) -> Partner",
			messages: []string{strings.Repeat("x", 21)},
			want:     authoring.PosturePartner,
		},
		{
			name:     "avg exactly 100 (at supportThreshold) -> Partner",
			messages: []string{strings.Repeat("x", 100)},
			want:     authoring.PosturePartner,
		},
		{
			name:     "avg exactly 101 (above supportThreshold) -> Support",
			messages: []string{strings.Repeat("x", 101)},
			want:     authoring.PostureSupport,
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
		got := authoring.ResolvePosture(authoring.PostureDrive, msgs)
		require.Equal(t, authoring.PostureDrive, got)
	})

	t.Run("UNSPECIFIED falls through to detect", func(t *testing.T) {
		t.Parallel()
		msgs := []string{"do it", "yes", "ok"}
		got := authoring.ResolvePosture(authoring.PostureUnspecified, msgs)
		require.Equal(t, authoring.PostureDrive, got)
	})
}
