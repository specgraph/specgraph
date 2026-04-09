// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
)

// --- Posture conversions ---

// postureToProtoMap maps domain Posture values to proto Posture values.
var postureToProtoMap = map[authoring.Posture]specv1.Posture{
	authoring.PostureUnspecified: specv1.Posture_POSTURE_UNSPECIFIED,
	authoring.PostureDrive:       specv1.Posture_POSTURE_DRIVE,
	authoring.PosturePartner:     specv1.Posture_POSTURE_PARTNER,
	authoring.PostureSupport:     specv1.Posture_POSTURE_SUPPORT,
}

// protoToPostureMap maps proto Posture values to domain Posture values.
var protoToPostureMap = map[specv1.Posture]authoring.Posture{
	specv1.Posture_POSTURE_UNSPECIFIED: authoring.PostureUnspecified,
	specv1.Posture_POSTURE_DRIVE:       authoring.PostureDrive,
	specv1.Posture_POSTURE_PARTNER:     authoring.PosturePartner,
	specv1.Posture_POSTURE_SUPPORT:     authoring.PostureSupport,
}

// postureToProto converts a domain Posture to its proto equivalent.
// Unknown values map to POSTURE_UNSPECIFIED.
func postureToProto(p authoring.Posture) specv1.Posture {
	return postureToProtoMap[p]
}

// protoToPosture converts a proto Posture to its domain equivalent.
// Unknown values map to PostureUnspecified.
func protoToPosture(p specv1.Posture) authoring.Posture {
	return protoToPostureMap[p]
}

// --- Safety result conversions ---

var severityToProtoMap = map[authoring.FindingSeverity]specv1.FindingSeverity{
	authoring.SeverityCritical: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
	authoring.SeverityWarning:  specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
	authoring.SeverityNote:     specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
}

var categoryToProtoMap = map[authoring.SafetyCategory]specv1.SafetyCategory{
	authoring.SafetyCategorySecurity: specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY,
	authoring.SafetyCategoryDataLoss: specv1.SafetyCategory_SAFETY_CATEGORY_DATA_LOSS,
}

// safetyResultsToProto converts domain safety flags to protobuf SafetyFlag messages.
func safetyResultsToProto(flags []authoring.SafetyFlagResult) []*specv1.SafetyFlag {
	out := make([]*specv1.SafetyFlag, len(flags))
	for i, f := range flags {
		protoSev, ok := severityToProtoMap[f.Severity]
		if !ok {
			protoSev = specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
		}
		protoCat, ok := categoryToProtoMap[f.Category]
		if !ok {
			protoCat = specv1.SafetyCategory_SAFETY_CATEGORY_UNSPECIFIED
		}
		out[i] = &specv1.SafetyFlag{
			Category:    protoCat,
			Severity:    protoSev,
			Description: f.Description,
		}
	}
	return out
}

// --- Prompt/stage conversions ---

// stageToProtoEnum is the canonical mapping from domain Stage to proto AuthoringStage.
var stageToProtoEnum = map[authoring.Stage]specv1.AuthoringStage{
	authoring.StageSpark:     specv1.AuthoringStage_AUTHORING_STAGE_SPARK,
	authoring.StageShape:     specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	authoring.StageSpecify:   specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY,
	authoring.StageDecompose: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE,
	authoring.StageApproved:  specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
}

// authoringStageToProto converts a domain Stage to its proto AuthoringStage equivalent.
// Returns AUTHORING_STAGE_UNSPECIFIED if the stage is not recognised.
func authoringStageToProto(stage authoring.Stage) specv1.AuthoringStage {
	return stageToProtoEnum[stage]
}

// promptsToProto converts the prompts for a stage into protobuf PromptTemplate messages.
// Returns nil if no prompts are defined for the stage.
func promptsToProto(stage authoring.Stage) []*specv1.PromptTemplate {
	prompts := authoring.GetPrompts(stage)
	if len(prompts) == 0 {
		return nil
	}
	protoStage := stageToProtoEnum[stage]
	out := make([]*specv1.PromptTemplate, len(prompts))
	for i, p := range prompts {
		out[i] = &specv1.PromptTemplate{
			Stage:    protoStage,
			Name:     p.Name,
			Template: p.Template,
		}
	}
	return out
}
