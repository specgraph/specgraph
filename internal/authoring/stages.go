// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// Stage is the typed authoring-stage value used by Composer and the funnel
// helpers. It shares an underlying string with storage.SpecStage so values
// flow through proto-string fields unchanged, but the defined type prevents
// implicit assignment of lifecycle-only storage values (in_progress, done,
// superseded, …) into authoring APIs. Use StageFromStorage to narrow a
// storage value into an authoring Stage; use Stage.AsStorage() for the
// reverse. The pair AsStorage / StageFromStorage are the only legitimate
// crossings between the two types.
type Stage string

// Authoring funnel stage constants. Most map 1:1 to storage.SpecStage* values;
// StageApprove is the one exception — it is the composer's content-routing key
// and is never written to storage (storage uses StageApproved).
const (
	StageSpark     Stage = "spark"
	StageShape     Stage = "shape"
	StageSpecify   Stage = "specify"
	StageDecompose Stage = "decompose"
	// StageApprove is the authoring-funnel verb stage. Used only as a
	// content-file routing key (stage-approve.md) and as a validStages
	// member; never written to storage or returned in proto responses
	// (storage uses StageApproved for the post-funnel state).
	StageApprove Stage = "approve"
	// StageApproved is the post-funnel storage state. Equal to
	// storage.SpecStageApproved by value.
	StageApproved Stage = "approved"
)

// authoringStages defines the ordered authoring funnel stages used for
// transition validation and storage-side operations. Note: this list uses
// StageApproved ("approved"), not StageApprove ("approve") — the latter is
// the composer's content-routing key, not a storage stage value.
var authoringStages = []Stage{StageSpark, StageShape, StageSpecify, StageDecompose, StageApproved}

// AsStorage returns s as a storage.SpecStage. Always safe — the underlying
// string is identical; the conversion only re-types the value for storage
// call sites.
func (s Stage) AsStorage() storage.SpecStage { return storage.SpecStage(s) }

// StageFromStorage narrows a storage.SpecStage to an authoring.Stage,
// returning (Stage, false) when s is not one of the funnel stages
// (i.e., lifecycle-only values like "in_progress", "done", "superseded").
func StageFromStorage(s storage.SpecStage) (Stage, bool) {
	candidate := Stage(s)
	for _, stage := range authoringStages {
		if candidate == stage {
			return candidate, true
		}
	}
	return "", false
}

// IsAuthoringStage reports whether s is one of the authoring funnel stages.
// Thin wrapper over StageFromStorage; kept for backward-compatible callers
// that only need a boolean.
func IsAuthoringStage(s storage.SpecStage) bool {
	_, ok := StageFromStorage(s)
	return ok
}

// AllStages returns a copy of the ordered stage list as strings (for backward compat with callers that need []string).
func AllStages() []string {
	out := make([]string, len(authoringStages))
	for i, s := range authoringStages {
		out[i] = string(s)
	}
	return out
}

// validateStageNames returns an error if from or to are unknown authoring stage names.
// allowEmptyFrom controls whether from=="" is accepted (used for initial transitions).
func validateStageNames(from, to Stage, allowEmptyFrom bool) (fromIdx, toIdx int, err error) {
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
func ValidateTransition(from, to Stage) error {
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
func ValidateAmendTransition(from, to Stage) error {
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
func indexOf(stage Stage) int {
	for i, s := range authoringStages {
		if s == stage {
			return i
		}
	}
	return -1
}
