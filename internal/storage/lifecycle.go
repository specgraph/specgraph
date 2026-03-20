// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// Lifecycle-specific sentinel errors.
var (
	ErrSpecNotDone            = errors.New("spec must be in done stage")
	ErrSpecIneligibleStage    = errors.New("spec is not in an eligible stage for this operation")
	ErrSpecIneligibleForDrift = errors.New("spec is not eligible for drift checking (must be done or amended)")
	ErrSpecTerminal           = errors.New("spec is in a terminal state (superseded or abandoned)")
	ErrNewSpecNotFound        = errors.New("replacement spec not found")
	ErrNewSpecTerminal        = errors.New("replacement spec is in a terminal state")
	ErrConcurrentModification = errors.New("concurrent modification detected — retry the operation")
	ErrInternalGuardFailure   = errors.New("internal guard failure — unexpected precondition violation")
	ErrInvalidReEntryStage    = errors.New("re-entry stage is not allowed for this operation")
	ErrSameSlugs              = errors.New("old and new slugs must differ")
	ErrEdgeNotFound           = errors.New("no matching dependency edge found")
)

// DriftType identifies the category of drift detected.
type DriftType string

// DriftType values.
const (
	DriftTypeDependency DriftType = "dependency"
	DriftTypeInterface  DriftType = "interface"
	DriftTypeVerify     DriftType = "verify"
)

// DriftSeverity indicates drift urgency.
type DriftSeverity string

// DriftSeverity values.
const (
	DriftSeverityHigh   DriftSeverity = "high"
	DriftSeverityMedium DriftSeverity = "medium"
	DriftSeverityLow    DriftSeverity = "low"
	DriftSeverityInfo   DriftSeverity = "info"
)

// IsValid reports whether t is a known drift type.
func (t DriftType) IsValid() bool {
	switch t {
	case DriftTypeDependency, DriftTypeInterface, DriftTypeVerify:
		return true
	default:
		return false
	}
}

// IsValid reports whether s is a known drift severity.
func (s DriftSeverity) IsValid() bool {
	switch s {
	case DriftSeverityHigh, DriftSeverityMedium, DriftSeverityLow, DriftSeverityInfo:
		return true
	default:
		return false
	}
}

// DriftItem is a single drift finding.
type DriftItem struct {
	Type         DriftType
	Severity     DriftSeverity
	Description  string
	SpecSlug     string
	UpstreamSlug string
	ExpectedHash string // edge's content_hash_at_link
	ActualHash   string // upstream's current ContentHash
}

// DriftReport aggregates drift items for a spec.
type DriftReport struct {
	SpecSlug     string
	Items        []DriftItem
	ErrorMessage string
}

// LintSeverity indicates lint violation urgency.
type LintSeverity string

// LintSeverity values.
const (
	LintSeverityError   LintSeverity = "error"
	LintSeverityWarning LintSeverity = "warning"
	LintSeverityInfo    LintSeverity = "info"
)

// IsValid reports whether s is a known lint severity.
func (s LintSeverity) IsValid() bool {
	switch s {
	case LintSeverityError, LintSeverityWarning, LintSeverityInfo:
		return true
	default:
		return false
	}
}

// LintViolation is a single lint finding.
type LintViolation struct {
	Rule     string
	Severity LintSeverity
	Message  string
	Location string
}

// LintResult holds lint results for a single spec.
type LintResult struct {
	SpecSlug   string
	Violations []LintViolation
	Passed     bool
	Error      string // non-empty when linting failed for this spec (for proto)
}

// LifecycleBackend defines storage operations for spec lifecycle transitions.
type LifecycleBackend interface {
	// LifecycleAmendSpec transitions a done spec back into authoring.
	// Returns ErrSpecNotFound, ErrSpecNotDone, or ErrSpecTerminal.
	LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*Spec, error)

	// LifecycleSupersedeSpec marks old spec superseded and links to new.
	LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*Spec, *Spec, error)

	// LifecycleAbandonSpec transitions a spec to abandoned (terminal).
	LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*Spec, error)

	// LifecycleAcknowledgeDrift marks drift as intentional.
	// When upstreamSlug is non-empty, updates the specific DEPENDS_ON edge's hash.
	// When upstreamSlug is empty, updates all outgoing DEPENDS_ON edges (blanket ack).
	// Returns ErrEdgeNotFound if upstreamSlug is specified but no matching edge exists.
	LifecycleAcknowledgeDrift(ctx context.Context, slug, upstreamSlug, note string) error
}
