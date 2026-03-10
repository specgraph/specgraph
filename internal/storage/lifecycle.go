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
	ErrConcurrentModification = errors.New("concurrent modification detected — retry the operation")
	ErrInvalidReEntryStage    = errors.New("re-entry stage is not allowed for this operation")
	ErrSameSlugs              = errors.New("old and new slugs must differ")
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
	Type            DriftType
	Severity        DriftSeverity
	Description     string
	SpecSlug        string
	UpstreamSlug    string
	ExpectedVersion int32
	ActualVersion   int32
}

// DriftReport aggregates drift items for a spec.
type DriftReport struct {
	SpecSlug            string
	Items               []DriftItem
	Acknowledged        bool
	AcknowledgeNote     string
	ItemsStale          bool
	ErrorMessage        string // non-empty when drift checking failed for this spec
	AckStateUnavailable bool   // true when ack state could not be fetched (e.g. spec deleted)
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
	Error      string // non-empty when linting failed for this spec
}

// LifecycleBackend defines storage operations for spec lifecycle transitions.
type LifecycleBackend interface {
	// GetSpec retrieves a spec by slug.
	// Returns ErrSpecNotFound if the spec does not exist.
	GetSpec(ctx context.Context, slug string) (*Spec, error)

	// BatchGetSpecs retrieves multiple specs by slug in a single round-trip.
	// Missing slugs are silently omitted from the result map.
	BatchGetSpecs(ctx context.Context, slugs []string) (map[string]*Spec, error)

	// LifecycleAmendSpec transitions a done spec back into authoring.
	// Returns ErrSpecNotFound, ErrSpecNotDone, or ErrSpecTerminal.
	LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*Spec, error)

	// LifecycleSupersedeSpec marks old spec superseded and links to new.
	LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*Spec, *Spec, error)

	// LifecycleAbandonSpec transitions a spec to abandoned (terminal).
	LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*Spec, error)

	// LifecycleAcknowledgeDrift marks drift as intentional.
	LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*DriftReport, error)
}
