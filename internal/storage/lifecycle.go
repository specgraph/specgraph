// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
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
	// LifecycleAmendSpec transitions an in-flight spec back into authoring.
	// The spec must be in an amend-eligible stage (approved, in_progress, review).
	// reEntryStage is required (spark, shape, specify, decompose).
	// Returns ErrSpecNotFound, ErrSpecNotAmendable, ErrReEntryStageRequired, or ErrSpecTerminal.
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
