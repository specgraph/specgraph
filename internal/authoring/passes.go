// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import "slices"

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
	autoIn    []Posture
	offeredIn []Posture
}

// allPostures is a convenience slice containing every defined posture.
var allPostures = []Posture{
	PostureDrive,
	PosturePartner,
	PostureSupport,
}

// passRegistry maps each authoring funnel stage to the analytical passes
// available in that stage, together with posture-aware scheduling rules.
// passRegistry is effectively immutable after init; do not modify at runtime.
var passRegistry = map[Stage][]passConfig{
	StageSpark: {
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageShape: {
		{pass: PassPeripheralVision, autoIn: []Posture{PostureDrive}, offeredIn: []Posture{PosturePartner}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageSpecify: {
		{pass: PassRedTeam, autoIn: []Posture{PostureDrive}, offeredIn: []Posture{PosturePartner, PostureSupport}},
		{pass: PassConsistencyCheck, autoIn: []Posture{PostureDrive}, offeredIn: []Posture{PosturePartner, PostureSupport}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
	StageDecompose: {
		{pass: PassSimplicityCheck, autoIn: []Posture{PostureDrive}, offeredIn: []Posture{PosturePartner, PostureSupport}},
		{pass: PassConstitutionCheck, autoIn: allPostures},
	},
}

// PassesForStage returns the passes that auto-run for the given stage and posture.
func PassesForStage(stage Stage, posture Posture) []string {
	return collectPasses(stage, posture, func(cfg passConfig) []Posture { return cfg.autoIn })
}

// OfferedPasses returns the passes that are offered (but not auto-run) for the
// given stage and posture.
func OfferedPasses(stage Stage, posture Posture) []string {
	return collectPasses(stage, posture, func(cfg passConfig) []Posture { return cfg.offeredIn })
}

// collectPasses filters pass configs for a stage, returning pass names where
// the posture appears in the slice selected by the selector function.
func collectPasses(stage Stage, posture Posture, selector func(passConfig) []Posture) []string {
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
