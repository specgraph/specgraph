// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import (
	"slices"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// PassName is a typed constant for analytical pass identifiers.
type PassName string

// Known analytical pass names.
const (
	PassConstitutionCheck PassName = "constitution_check"
	PassPeripheralVision  PassName = "peripheral_vision"
	PassRedTeam           PassName = "red_team"
	PassConsistencyCheck  PassName = "consistency_check"
	PassSimplicityCheck   PassName = "simplicity_check"
)

// passConfig describes a single analytical pass, the postures in which it runs
// automatically, and the postures in which it is offered but not auto-run.
type passConfig struct {
	pass      PassName
	autoIn    []specv1.Posture
	offeredIn []specv1.Posture
}

// allPostures is a convenience slice containing every defined posture.
var allPostures = []specv1.Posture{
	specv1.Posture_POSTURE_DRIVE,
	specv1.Posture_POSTURE_PARTNER,
	specv1.Posture_POSTURE_SUPPORT,
}

// passRegistry maps each authoring funnel stage to the analytical passes
// available in that stage, together with posture-aware scheduling rules.
// passRegistry is effectively immutable after init; do not modify at runtime.
var passRegistry = map[Stage][]passConfig{
	StageSpark: {
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageShape: {
		{pass: PassPeripheralVision, autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE}, offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageSpecify: {
		{pass: PassRedTeam, autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE}, offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
		{pass: PassConsistencyCheck, autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE}, offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageDecompose: {
		{pass: PassSimplicityCheck, autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE}, offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
}

// PassesForStage returns the passes that auto-run for the given stage and posture.
func PassesForStage(stage Stage, posture specv1.Posture) []string {
	return collectPasses(stage, posture, func(cfg passConfig) []specv1.Posture { return cfg.autoIn })
}

// OfferedPasses returns the passes that are offered (but not auto-run) for the
// given stage and posture.
func OfferedPasses(stage Stage, posture specv1.Posture) []string {
	return collectPasses(stage, posture, func(cfg passConfig) []specv1.Posture { return cfg.offeredIn })
}

// collectPasses filters pass configs for a stage, returning pass names where
// the posture appears in the slice selected by the selector function.
func collectPasses(stage Stage, posture specv1.Posture, selector func(passConfig) []specv1.Posture) []string {
	configs, ok := passRegistry[stage]
	if !ok {
		return nil
	}
	var result []string
	for _, cfg := range configs {
		if slices.Contains(selector(cfg), posture) {
			result = append(result, string(cfg.pass))
		}
	}
	return result
}
