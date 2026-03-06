// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrDecisionNotFound is returned when a decision does not exist.
var ErrDecisionNotFound = errors.New("decision not found")

// ErrSupersededByRequired is returned when status is superseded but superseded_by is not provided.
var ErrSupersededByRequired = errors.New("superseded_by is required when status is superseded")

// DecisionStatus represents the lifecycle state of a decision.
type DecisionStatus string

const (
	DecisionStatusProposed   DecisionStatus = "DECISION_STATUS_PROPOSED"
	DecisionStatusAccepted   DecisionStatus = "DECISION_STATUS_ACCEPTED"
	DecisionStatusSuperseded DecisionStatus = "DECISION_STATUS_SUPERSEDED"
	DecisionStatusDeprecated DecisionStatus = "DECISION_STATUS_DEPRECATED"
)

// Decision is the storage-layer domain type for architectural decisions.
type Decision struct {
	ID           string
	Slug         string
	Title        string
	Status       DecisionStatus
	Decision     string
	Rationale    string
	SupersededBy string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DecisionBackend defines storage operations for Decision entities.
type DecisionBackend interface {
	// CreateDecision stores a new decision.
	CreateDecision(ctx context.Context, slug, title, decision, rationale string) (*specv1.Decision, error)
	// GetDecision retrieves a decision by slug.
	GetDecision(ctx context.Context, slug string) (*specv1.Decision, error)
	// ListDecisions returns decisions matching the given filters.
	ListDecisions(ctx context.Context, status specv1.DecisionStatus, limit int) ([]*specv1.Decision, error)
	// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
	UpdateDecision(ctx context.Context, slug string, title *string, status *specv1.DecisionStatus, decision, rationale, supersededBy *string) (*specv1.Decision, error)
}
