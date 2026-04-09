// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "errors"

// --- Spec errors ---

// ErrSpecNotFound is returned when a spec does not exist.
var ErrSpecNotFound = errors.New("spec not found")

// ErrSpecAlreadyExists is returned when creating a spec with a slug that already exists.
var ErrSpecAlreadyExists = errors.New("spec already exists")

// ErrSpecAlreadyApproved is returned when attempting to modify an already-approved spec.
var ErrSpecAlreadyApproved = errors.New("spec is already approved")

// ErrSpecSuperseded is returned when attempting to amend a spec that has been superseded.
var ErrSpecSuperseded = errors.New("spec has been superseded and cannot be amended")

// ErrInvalidStageTransition is returned when a stage transition violates funnel rules.
var ErrInvalidStageTransition = errors.New("invalid stage transition")

// --- Claim errors ---

// ErrSpecAlreadyClaimed is returned when a spec has an active claim by another agent.
var ErrSpecAlreadyClaimed = errors.New("spec already claimed")

// ErrNotClaimOwner is returned when the agent does not own the claim.
var ErrNotClaimOwner = errors.New("agent does not own the claim")

// ErrSpecNotClaimed is returned when the spec is not claimed.
var ErrSpecNotClaimed = errors.New("spec is not claimed")

// --- Execution errors ---

// ErrSpecNotApproved is returned when a bundle is requested for a spec not in an executable stage.
var ErrSpecNotApproved = errors.New("spec is not in an approved or in_progress stage")

// ErrAgentNotClaimOwner is returned when an agent reports an event but does not hold the claim.
var ErrAgentNotClaimOwner = errors.New("agent does not hold the claim for this spec")

// --- Lifecycle errors ---

var (
	// ErrSpecNotDone is returned when a lifecycle operation requires done stage.
	ErrSpecNotDone = errors.New("spec must be in done stage")
	// ErrSpecIneligibleStage is returned when a spec's stage does not support the requested operation.
	ErrSpecIneligibleStage = errors.New("spec is not in an eligible stage for this operation")
	// ErrSpecIneligibleForDrift is returned when a spec cannot be drift-checked.
	ErrSpecIneligibleForDrift = errors.New("spec is not eligible for drift checking (must be done)")
	// ErrSpecTerminal is returned when a spec is superseded or abandoned.
	ErrSpecTerminal = errors.New("spec is in a terminal state (superseded or abandoned)")
	// ErrNewSpecNotFound is returned when a replacement spec does not exist.
	ErrNewSpecNotFound = errors.New("replacement spec not found")
	// ErrNewSpecTerminal is returned when a replacement spec is in a terminal state.
	ErrNewSpecTerminal = errors.New("replacement spec is in a terminal state")
	// ErrConcurrentModification is returned when an optimistic-lock version guard fails.
	ErrConcurrentModification = errors.New("concurrent modification detected — retry the operation")
	// ErrInternalGuardFailure signals an unexpected precondition violation.
	ErrInternalGuardFailure = errors.New("internal guard failure — unexpected precondition violation")
	// ErrInvalidReEntryStage is returned when the requested re-entry stage is disallowed.
	ErrInvalidReEntryStage = errors.New("re-entry stage is not allowed for this operation")
	// ErrSameSlugs is returned when old and new slugs are identical in a supersede operation.
	ErrSameSlugs = errors.New("old and new slugs must differ")
	// ErrEdgeNotFound is returned when no matching dependency edge exists.
	ErrEdgeNotFound = errors.New("no matching dependency edge found")
	// ErrSpecNotAmendable is returned when amend is attempted on a spec not in an eligible stage.
	ErrSpecNotAmendable = errors.New("spec must be in approved, in_progress, or review stage to amend")
	// ErrReEntryStageRequired is returned when amend is called without a re-entry stage.
	ErrReEntryStageRequired = errors.New("re_entry_stage is required for amend")
)

// --- Version errors ---

// ErrVersionNotFound is returned when a requested version does not exist.
var ErrVersionNotFound = errors.New("version not found")

// --- Decision errors ---

// ErrDecisionNotFound is returned when a decision does not exist.
var ErrDecisionNotFound = errors.New("decision not found")

// ErrDecisionAlreadyExists is returned when creating a decision with a slug that already exists.
var ErrDecisionAlreadyExists = errors.New("decision already exists")

// ErrSupersededByRequired is returned when status is superseded but superseded_by is not provided.
var ErrSupersededByRequired = errors.New("superseded_by is required when status is superseded")

// --- Constitution errors ---

// ErrConstitutionNotFound is returned when no constitution exists.
var ErrConstitutionNotFound = errors.New("constitution not found")

// --- Project errors ---

// ErrProjectNotFound is returned when no project exists with the given slug.
var ErrProjectNotFound = errors.New("project not found")

// --- Slice errors ---

var (
	// ErrSliceNotFound is returned when a slice lookup finds no matching node.
	ErrSliceNotFound = errors.New("slice not found")
	// ErrSliceWrongStatus is returned when a status transition is invalid.
	ErrSliceWrongStatus = errors.New("slice status precondition not met")
)

// --- Sync errors ---

var (
	// ErrSyncMappingNotFound is returned when a sync mapping does not exist.
	ErrSyncMappingNotFound = errors.New("sync mapping not found")
	// ErrSyncMappingExists is returned when a sync mapping already exists for the spec+adapter pair.
	ErrSyncMappingExists = errors.New("sync mapping already exists for this spec and adapter")
)
