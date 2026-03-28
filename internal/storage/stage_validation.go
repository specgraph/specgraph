// SPDX-License-Identifier: MIT
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

	// Backward (amend): to must be at a lower index than from.
	if fromIdx >= 0 && toIdx >= 0 && toIdx < fromIdx {
		return nil
	}

	return fmt.Errorf("invalid transition from %q to %q", from, to)
}

// ValidateAmendTransition checks whether an amend (backward) transition is valid.
// It only allows moving to an earlier stage — forward transitions and same-to-same
// are rejected. This is distinct from ValidateTransition which allows both directions.
func ValidateAmendTransition(from, to SpecStage) error {
	if from == to {
		return fmt.Errorf("amend transition from %q to %q is a no-op", from, to)
	}
	fromIdx, toIdx, err := validateStageNames(from, to, false)
	if err != nil {
		return err
	}

	// Only backward transitions are allowed for amend.
	if toIdx < fromIdx {
		return nil
	}

	return fmt.Errorf("amend requires backward transition: %q to %q is not backward", from, to)
}
