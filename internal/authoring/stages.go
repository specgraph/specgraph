// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import "fmt"

// stages defines the ordered authoring funnel stages.
var stages = []string{"spark", "shape", "specify", "decompose", "approved"}

// forwardTransitions maps each stage to its next valid forward stage.
var forwardTransitions = map[string]string{
	"":          "spark",
	"spark":     "shape",
	"shape":     "specify",
	"specify":   "decompose",
	"decompose": "approved",
}

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

	// Check forward transition: must be the immediate next stage.
	if next, ok := forwardTransitions[from]; ok && next == to {
		return nil
	}

	// Check backward (amend) transition: to must be at a lower index than from.
	fromIdx := indexOf(from)
	toIdx := indexOf(to)

	if fromIdx < 0 && from != "" {
		return fmt.Errorf("unknown stage %q", from)
	}
	if toIdx < 0 {
		return fmt.Errorf("unknown stage %q", to)
	}

	if fromIdx >= 0 && toIdx >= 0 && toIdx < fromIdx {
		return nil
	}

	return fmt.Errorf("invalid transition from %q to %q", from, to)
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
