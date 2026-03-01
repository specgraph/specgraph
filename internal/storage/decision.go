// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// DecisionBackend defines storage operations for Decision entities.
type DecisionBackend interface {
	CreateDecision(ctx context.Context, slug, title, decision, rationale string) (*specv1.Decision, error)
	GetDecision(ctx context.Context, slug string) (*specv1.Decision, error)
	ListDecisions(ctx context.Context, status specv1.DecisionStatus, limit int) ([]*specv1.Decision, error)
	UpdateDecision(ctx context.Context, slug string, title *string, status *specv1.DecisionStatus, decision, rationale, supersededBy *string) (*specv1.Decision, error)
}
