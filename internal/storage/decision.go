// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"
)

// ErrDecisionNotFound is returned when a decision does not exist.
var ErrDecisionNotFound = errors.New("decision not found")

// ErrSupersededByRequired is returned when status is superseded but superseded_by is not provided.
var ErrSupersededByRequired = errors.New("superseded_by is required when status is superseded")

// DecisionStatus represents the lifecycle state of a decision.
type DecisionStatus string

// Decision status lifecycle values.
const (
	DecisionStatusProposed   DecisionStatus = "DECISION_STATUS_PROPOSED"
	DecisionStatusAccepted   DecisionStatus = "DECISION_STATUS_ACCEPTED"
	DecisionStatusSuperseded DecisionStatus = "DECISION_STATUS_SUPERSEDED"
	DecisionStatusDeprecated DecisionStatus = "DECISION_STATUS_DEPRECATED"
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

// Decision is the storage-layer domain type for architectural decisions.
type Decision struct {
	ID           string
	Slug         string
	Title        string
	Status       DecisionStatus
	Body         string
	Rationale    string
	SupersededBy string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DecisionBackend defines storage operations for Decision entities.
type DecisionBackend interface {
	// CreateDecision stores a new decision.
	CreateDecision(ctx context.Context, slug, title, body, rationale string) (*Decision, error)
	// GetDecision retrieves a decision by slug.
	GetDecision(ctx context.Context, slug string) (*Decision, error)
	// ListDecisions returns decisions matching the given filters.
	ListDecisions(ctx context.Context, status DecisionStatus, limit int) ([]*Decision, error)
	// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
	UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus, body, rationale, supersededBy *string) (*Decision, error)
}
