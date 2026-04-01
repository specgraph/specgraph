// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// DecisionStatus represents the lifecycle state of a decision.
type DecisionStatus string

// Decision status lifecycle values.
const (
	DecisionStatusProposed   DecisionStatus = "proposed"
	DecisionStatusAccepted   DecisionStatus = "accepted"
	DecisionStatusSuperseded DecisionStatus = "superseded"
	DecisionStatusDeprecated DecisionStatus = "deprecated"
)

// IsValid reports whether s is a known decision status.
func (s DecisionStatus) IsValid() bool {
	switch s {
	case DecisionStatusProposed, DecisionStatusAccepted,
		DecisionStatusSuperseded, DecisionStatusDeprecated:
		return true
	default:
		return false
	}
}

// DecisionConfidence represents the confidence level of a decision.
type DecisionConfidence string

// Decision confidence values.
const (
	DecisionConfidenceHigh   DecisionConfidence = "high"
	DecisionConfidenceMedium DecisionConfidence = "medium"
	DecisionConfidenceLow    DecisionConfidence = "low"
)

// IsValid reports whether c is a known decision confidence level.
func (c DecisionConfidence) IsValid() bool {
	switch c {
	case DecisionConfidenceHigh, DecisionConfidenceMedium, DecisionConfidenceLow:
		return true
	default:
		return false
	}
}

// DecisionScope represents the organizational scope of a decision.
type DecisionScope string

// Decision scope values.
const (
	DecisionScopeProject DecisionScope = "project"
	DecisionScopeTeam    DecisionScope = "team"
	DecisionScopeOrg     DecisionScope = "org"
)

// IsValid reports whether s is a known decision scope.
func (s DecisionScope) IsValid() bool {
	switch s {
	case DecisionScopeProject, DecisionScopeTeam, DecisionScopeOrg:
		return true
	default:
		return false
	}
}

// RejectedAlternative records an option that was considered but not chosen.
type RejectedAlternative struct {
	Option string
	Reason string
}

// Decision is the storage-layer domain type for architectural decisions.
type Decision struct {
	ID                   string
	Slug                 string
	Title                string
	Status               DecisionStatus
	Body                 string
	Rationale            string
	SupersededBy         string
	Question             string
	RejectedAlternatives []RejectedAlternative
	Confidence           DecisionConfidence
	Tags                 []string
	Scope                DecisionScope
	OriginSpec           string
	OriginStage          string
	Version              int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ContentHash          string
}

// DecisionBackend defines storage operations for Decision entities.
type DecisionBackend interface {
	// CreateDecision stores a new decision.
	CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
		rejectedAlts []RejectedAlternative, confidence DecisionConfidence,
		tags []string, scope DecisionScope, originSpec, originStage string) (*Decision, error)
	// GetDecision retrieves a decision by slug.
	GetDecision(ctx context.Context, slug string) (*Decision, error)
	// ListDecisions returns decisions matching the given filters.
	ListDecisions(ctx context.Context, status DecisionStatus, limit int) ([]*Decision, error)
	// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
	UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus,
		body, rationale, supersededBy, question *string,
		rejectedAlts *[]RejectedAlternative, confidence *DecisionConfidence,
		tags *[]string, scope *DecisionScope, originSpec, originStage *string) (*Decision, error)
}
