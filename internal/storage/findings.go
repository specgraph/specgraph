// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// PassType is a typed string for analytical pass identifiers.
type PassType string

// PassType values.
const (
	PassTypeConstitutionCheck PassType = "constitution_check"
	PassTypePeripheralVision  PassType = "peripheral_vision"
	PassTypeRedTeam           PassType = "red_team"
	PassTypeConsistencyCheck  PassType = "consistency_check"
	PassTypeSimplicityCheck   PassType = "simplicity_check"
	PassTypeApproveRejected   PassType = "approve-rejected"
)

// ValidPassType reports whether pt is a known pass type.
func ValidPassType(pt PassType) bool {
	switch pt {
	case PassTypeConstitutionCheck, PassTypePeripheralVision,
		PassTypeRedTeam, PassTypeConsistencyCheck, PassTypeSimplicityCheck,
		PassTypeApproveRejected:
		return true
	}
	return false
}

// AnalyticalFindingInput contains the fields required to create a finding.
// PassType is derived from the method-level parameter, not per-finding.
type AnalyticalFindingInput struct {
	Severity   FindingSeverity
	Summary    string
	Detail     string
	Constraint string
	Resolution string
}

// AnalyticalFinding records a finding produced by an analytical pass (read-side).
type AnalyticalFinding struct {
	ID         string
	SpecSlug   string // populated by ListAllFindings for export
	PassType   PassType
	Severity   FindingSeverity
	Summary    string
	Detail     string
	Constraint string
	Resolution string
	Version    int32
	CreatedAt  time.Time
}

// FindingsWriter stores analytical pass findings.
type FindingsWriter interface {
	// StoreFindings replaces all findings for the given (slug, passType) pair
	// and returns the IDs assigned to the persisted findings.
	StoreFindings(ctx context.Context, slug string, passType PassType, findings []AnalyticalFindingInput) ([]string, error)
}

// FindingsReader retrieves analytical pass findings.
type FindingsReader interface {
	// ListFindings returns findings for a spec, optionally filtered by pass type.
	// Returns ErrSpecNotFound if the spec does not exist.
	// An empty passType returns all findings across all pass types.
	ListFindings(ctx context.Context, slug string, passType PassType) ([]AnalyticalFinding, error)

	// ListAllFindings returns all findings across all specs, with SpecSlug populated.
	ListAllFindings(ctx context.Context) ([]*AnalyticalFinding, error)
}

// FindingsBackend combines finding read and write operations.
type FindingsBackend interface {
	FindingsWriter
	FindingsReader
}
