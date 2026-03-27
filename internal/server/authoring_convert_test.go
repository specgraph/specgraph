// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestSafetyResultsToProto(t *testing.T) {
	t.Run("converts domain flags to proto", func(t *testing.T) {
		flags := []authoring.SafetyFlagResult{
			{
				Category:    authoring.SafetyCategorySecurity,
				Severity:    authoring.SeverityCritical,
				Description: "test security flag",
			},
			{
				Category:    authoring.SafetyCategoryDataLoss,
				Severity:    authoring.SeverityCritical,
				Description: "test data loss flag",
			},
		}
		protos := safetyResultsToProto(flags)
		require.Len(t, protos, 2)
		require.Equal(t, specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY, protos[0].Category)
		require.Equal(t, specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, protos[0].Severity)
		require.Equal(t, "test security flag", protos[0].Description)
		require.Equal(t, specv1.SafetyCategory_SAFETY_CATEGORY_DATA_LOSS, protos[1].Category)
	})

	t.Run("empty input returns empty output", func(t *testing.T) {
		protos := safetyResultsToProto(nil)
		require.Empty(t, protos)
	})
}

func TestPromptsToProto(t *testing.T) {
	t.Run("spark stage produces correct proto templates", func(t *testing.T) {
		protos := promptsToProto(authoring.StageSpark)
		require.NotEmpty(t, protos)
		for _, p := range protos {
			require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPARK, p.Stage)
			require.NotEmpty(t, p.Name)
			require.NotEmpty(t, p.Template)
		}
		// Verify specific prompt names match GetPrompts.
		prompts := authoring.GetPrompts(authoring.StageSpark)
		require.Len(t, protos, len(prompts))
		for i, p := range protos {
			require.Equal(t, prompts[i].Name, p.Name)
			require.Equal(t, prompts[i].Template, p.Template)
		}
	})

	t.Run("all stages produce non-nil proto with consistent stage field", func(t *testing.T) {
		for _, stageStr := range authoring.AllStages() {
			stage := authoring.Stage(stageStr)
			if stage == authoring.StageApproved {
				// approved has no prompts
				continue
			}
			protos := promptsToProto(stage)
			require.NotNilf(t, protos, "stage %q should have protos", stage)
			for _, p := range protos {
				require.NotEqual(t, specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED, p.Stage,
					"stage %q proto should not be UNSPECIFIED", stage)
				require.Equal(t, protos[0].Stage, p.Stage,
					"all protos for stage %q should have the same stage enum", stage)
			}
		}
	})

	t.Run("unknown stage returns nil", func(t *testing.T) {
		protos := promptsToProto("nonexistent")
		require.Nil(t, protos)
	})
}

func TestAuthoringStageToProto(t *testing.T) {
	t.Run("known stages map correctly", func(t *testing.T) {
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPARK, authoringStageToProto(authoring.StageSpark))
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SHAPE, authoringStageToProto(authoring.StageShape))
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY, authoringStageToProto(authoring.StageSpecify))
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE, authoringStageToProto(authoring.StageDecompose))
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, authoringStageToProto(authoring.StageApproved))
	})

	t.Run("unknown stage returns UNSPECIFIED", func(t *testing.T) {
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED, authoringStageToProto("unknown"))
	})
}

func TestPostureConversions(t *testing.T) {
	t.Run("domain to proto roundtrip", func(t *testing.T) {
		for _, p := range []authoring.Posture{
			authoring.PostureUnspecified,
			authoring.PostureDrive,
			authoring.PosturePartner,
			authoring.PostureSupport,
		} {
			proto := postureToProto(p)
			back := protoToPosture(proto)
			require.Equal(t, p, back)
		}
	})

	t.Run("unknown domain posture maps to UNSPECIFIED", func(t *testing.T) {
		require.Equal(t, specv1.Posture_POSTURE_UNSPECIFIED, postureToProto(authoring.Posture(99)))
	})

	t.Run("unknown proto posture maps to PostureUnspecified", func(t *testing.T) {
		require.Equal(t, authoring.PostureUnspecified, protoToPosture(specv1.Posture(99)))
	})
}
