// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import "fmt"

// authoringStages defines the ordered authoring funnel stages.
// This is the storage-layer counterpart to authoring.stages.
var authoringStages = []SpecStage{
	SpecStageSpark,
	SpecStageShape,
	SpecStageSpecify,
	SpecStageDecompose,
	SpecStageApproved,
}

// stageIndex returns the position of a stage in the ordered list, or -1 if not found.
func stageIndex(stage SpecStage) int {
	for i, s := range authoringStages {
		if s == stage {
			return i
		}
	}
	return -1
}

// validateStageNames returns an error if from or to are unknown stage names.
// allowEmptyFrom controls whether from=="" is accepted (used for initial transitions).
func validateStageNames(from, to SpecStage, allowEmptyFrom bool) (fromIdx, toIdx int, err error) {
	fromIdx = stageIndex(from)
	toIdx = stageIndex(to)
	var unknowns []string
	if fromIdx < 0 && (!allowEmptyFrom || from != "") {
		unknowns = append(unknowns, string(from))
	}
	if toIdx < 0 {
		unknowns = append(unknowns, string(to))
	}
	if len(unknowns) > 0 {
		return fromIdx, toIdx, fmt.Errorf("unknown stage(s): %q", unknowns)
	}
	return fromIdx, toIdx, nil
}

// ValidateTransition checks whether moving from one authoring stage to another is allowed.
// Forward transitions must follow the defined order (no skipping).
// Backward (amend) transitions are allowed to any earlier stage.
// Same-to-same transitions are not allowed.
//
// IN-04: this is a low-level *structural* validator (pure funnel-order check)
// shared by the general TransitionStage path — including export/import restore,
// which steps specs through the funnel to reconstruct their persisted stage. It
// deliberately does NOT enforce the Phase-7 amend semantics (re-entry allowlist,
// amend-eligibility, claim release). Those live in LifecycleAmendSpec, which is
// the only supported way to rewind a spec through authoring in production. The
// permissive backward branch below is intentionally retained so restore and
// other structural callers are not forced through the amend business logic;
// tightening it here would break the export path and the stage_validation suite
// without closing any real hole, since no in-scope caller performs a backward
// TransitionStage.
func ValidateTransition(from, to SpecStage) error {
	if from == to {
		return fmt.Errorf("transition from %q to %q is a no-op", from, to)
	}
	fromIdx, toIdx, err := validateStageNames(from, to, true)
	if err != nil {
		return err
	}

	// Forward: initial ("" -> first stage) or next stage in sequence.
	if from == "" && toIdx == 0 {
		return nil
	}
	if fromIdx >= 0 && toIdx == fromIdx+1 {
		return nil
	}

	// Backward (amend): to must be at a lower index than from. Intentionally
	// permissive — see the IN-04 note on the function doc comment. Business-rule
	// enforcement of backward movement is LifecycleAmendSpec's responsibility,
	// not this structural validator's.
	if fromIdx >= 0 && toIdx >= 0 && toIdx < fromIdx {
		return nil
	}

	return fmt.Errorf("invalid transition from %q to %q", from, to)
}
