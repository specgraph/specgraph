// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestPromptsToProto(t *testing.T) {
	t.Run("spark stage produces correct proto templates", func(t *testing.T) {
		protos := authoring.PromptsToProto("spark")
		require.NotEmpty(t, protos)
		for _, p := range protos {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPARK, p.Stage)
			require.NotEmpty(t, p.Name)
			require.NotEmpty(t, p.Template)
		}
		// Verify specific prompt names match GetPrompts.
		prompts := authoring.GetPrompts("spark")
		require.Len(t, protos, len(prompts))
		for i, p := range protos {
			require.Equal(t, prompts[i].Name, p.Name)
			require.Equal(t, prompts[i].Template, p.Template)
		}
	})

	t.Run("all stages produce non-nil proto with consistent stage field", func(t *testing.T) {
		for _, stage := range authoring.AllStages() {
			if stage == "approved" {
				// approved has no prompts
				continue
			}
			protos := authoring.PromptsToProto(stage)
			require.NotNilf(t, protos, "stage %q should have protos", stage)
			// All protos for a given stage must share the same non-UNSPECIFIED stage enum.
			for _, p := range protos {
				require.NotEqual(t, specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED, p.Stage,
					"stage %q proto should not be UNSPECIFIED", stage)
				require.Equal(t, protos[0].Stage, p.Stage,
					"all protos for stage %q should have the same stage enum", stage)
			}
		}
	})

	t.Run("unknown stage returns nil", func(t *testing.T) {
		protos := authoring.PromptsToProto("nonexistent")
		require.Nil(t, protos)
	})
}

func TestGetPrompts(t *testing.T) {
	t.Run("spark prompts exist with expected names", func(t *testing.T) {
		prompts := authoring.GetPrompts("spark")
		require.NotEmpty(t, prompts)

		names := make([]string, len(prompts))
		for i, p := range prompts {
			names[i] = p.Name
		}
		require.Contains(t, names, "seed")
		require.Contains(t, names, "signal")
		require.Contains(t, names, "kill_test")
	})

	t.Run("shape prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("shape"))
	})

	t.Run("specify prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("specify"))
	})

	t.Run("decompose prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("decompose"))
	})

	t.Run("nonexistent stage returns empty", func(t *testing.T) {
		prompts := authoring.GetPrompts("nonexistent")
		require.Empty(t, prompts)
	})
}
