// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// Stage is an alias for storage.SpecStage, kept for backward compatibility
// within the authoring package. New code should use storage.SpecStage directly.
type Stage = storage.SpecStage

// Stage name constants used across the authoring funnel.
// These are aliases for the corresponding storage.SpecStage* constants.
const (
	StageSpark     = storage.SpecStageSpark
	StageShape     = storage.SpecStageShape
	StageSpecify   = storage.SpecStageSpecify
	StageDecompose = storage.SpecStageDecompose
	StageApproved  = storage.SpecStageApproved
)

// authoringStages defines the ordered authoring funnel stages.
var authoringStages = []storage.SpecStage{StageSpark, StageShape, StageSpecify, StageDecompose, StageApproved}

// AllStages returns a copy of the ordered stage list as strings (for backward compat with callers that need []string).
func AllStages() []string {
	out := make([]string, len(authoringStages))
	for i, s := range authoringStages {
		out[i] = string(s)
	}
	return out
}

// IsAuthoringStage reports whether the given SpecStage is one of the five
// authoring funnel stages (spark, shape, specify, decompose, approved).
func IsAuthoringStage(s storage.SpecStage) bool {
	return indexOf(s) >= 0
}

// validateStageNames returns an error if from or to are unknown authoring stage names.
// allowEmptyFrom controls whether from=="" is accepted (used for initial transitions).
func validateStageNames(from, to storage.SpecStage, allowEmptyFrom bool) (fromIdx, toIdx int, err error) {
	fromIdx = indexOf(from)
	toIdx = indexOf(to)
	var unknowns []string
	if fromIdx < 0 && (!allowEmptyFrom || from != "") {
		unknowns = append(unknowns, string(from))
	}
	if toIdx < 0 {
		unknowns = append(unknowns, string(to))
	}
	if len(unknowns) > 0 {
		return fromIdx, toIdx, fmt.Errorf("unknown stage(s): %v", unknowns)
	}
	return fromIdx, toIdx, nil
}

// ValidateTransition checks whether moving from one stage to another is allowed.
// Forward transitions must follow the defined order (no skipping).
// Backward (amend) transitions are allowed to any earlier stage.
// Same-to-same transitions are not allowed.
func ValidateTransition(from, to storage.SpecStage) error {
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
func ValidateAmendTransition(from, to storage.SpecStage) error {
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

// indexOf returns the position of a stage in the ordered authoring funnel list, or -1 if not found.
func indexOf(stage storage.SpecStage) int {
	for i, s := range authoringStages {
		if s == stage {
			return i
		}
	}
	return -1
}
