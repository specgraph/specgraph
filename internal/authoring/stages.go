// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import "fmt"

// Stage name constants used across the authoring funnel.
const (
	StageSpark     = "spark"
	StageShape     = "shape"
	StageSpecify   = "specify"
	StageDecompose = "decompose"
	StageApproved  = "approved"
)

// stages defines the ordered authoring funnel stages.
var stages = []string{StageSpark, StageShape, StageSpecify, StageDecompose, StageApproved}

// AllStages returns a copy of the ordered stage list.
func AllStages() []string {
	out := make([]string, len(stages))
	copy(out, stages)
	return out
}

// ValidateTransition checks whether moving from one stage to another is allowed.
// Forward transitions must follow the defined order (no skipping).
// Backward (amend) transitions are allowed to any earlier stage.
// Same-to-same transitions are not allowed.
func ValidateTransition(from, to string) error {
	if from == to {
		return fmt.Errorf("transition from %q to %q is a no-op", from, to)
	}

	fromIdx := indexOf(from)
	toIdx := indexOf(to)

	// Validate both stages are known.
	var unknowns []string
	if fromIdx < 0 && from != "" {
		unknowns = append(unknowns, from)
	}
	if toIdx < 0 {
		unknowns = append(unknowns, to)
	}
	if len(unknowns) > 0 {
		return fmt.Errorf("unknown stage(s): %v", unknowns)
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
func ValidateAmendTransition(from, to string) error {
	if from == to {
		return fmt.Errorf("amend transition from %q to %q is a no-op", from, to)
	}

	fromIdx := indexOf(from)
	toIdx := indexOf(to)

	// Validate both stages are known.
	var unknowns []string
	if fromIdx < 0 {
		unknowns = append(unknowns, from)
	}
	if toIdx < 0 {
		unknowns = append(unknowns, to)
	}
	if len(unknowns) > 0 {
		return fmt.Errorf("unknown stage(s): %v", unknowns)
	}

	// Only backward transitions are allowed for amend.
	if toIdx < fromIdx {
		return nil
	}

	return fmt.Errorf("amend requires backward transition: %q to %q is not backward", from, to)
}

// indexOf returns the position of a stage in the ordered list, or -1 if not found.
func indexOf(stage string) int {
	for i, s := range stages {
		if s == stage {
			return i
		}
	}
	return -1
}
