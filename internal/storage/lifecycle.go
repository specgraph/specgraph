// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// Lifecycle-specific sentinel errors.
var (
	ErrSpecNotDone     = errors.New("spec must be in done stage to amend")
	ErrSpecTerminal    = errors.New("spec is in a terminal state (superseded or abandoned)")
	ErrNewSpecNotFound = errors.New("replacement spec not found")
)

// DriftType identifies the category of drift detected.
type DriftType string

const (
	DriftTypeDependency DriftType = "dependency"
	DriftTypeInterface  DriftType = "interface"
	DriftTypeVerify     DriftType = "verify"
)

// DriftSeverity indicates drift urgency.
type DriftSeverity string

const (
	DriftSeverityHigh   DriftSeverity = "high"
	DriftSeverityMedium DriftSeverity = "medium"
	DriftSeverityLow    DriftSeverity = "low"
	DriftSeverityInfo   DriftSeverity = "info"
)

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
	SpecSlug        string
	Items           []DriftItem
	Acknowledged    bool
	AcknowledgeNote string
}

// LintSeverity indicates lint violation urgency.
type LintSeverity string

const (
	LintSeverityError   LintSeverity = "error"
	LintSeverityWarning LintSeverity = "warning"
	LintSeverityInfo    LintSeverity = "info"
)

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
}

// LifecycleBackend defines storage operations for spec lifecycle transitions.
type LifecycleBackend interface {
	// AmendSpec transitions a done spec back into authoring.
	AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*Spec, error)

	// SupersedeSpec marks old spec superseded and links to new.
	SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*Spec, *Spec, error)

	// AbandonSpec transitions a spec to abandoned (terminal).
	AbandonSpec(ctx context.Context, slug, reason string) (*Spec, error)

	// CheckDrift runs drift detection for a single spec or all eligible specs.
	CheckDrift(ctx context.Context, slug, scope string) ([]DriftReport, error)

	// AcknowledgeDrift marks drift as intentional.
	AcknowledgeDrift(ctx context.Context, slug, note string) (*DriftReport, error)
}
