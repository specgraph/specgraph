// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// ErrSpecNotApproved is returned when a bundle is requested for a spec not in an executable stage.
var ErrSpecNotApproved = errors.New("spec is not in an approved or in_progress stage")

// ErrAgentNotClaimOwner is returned when an agent reports an event but does not hold the claim.
var ErrAgentNotClaimOwner = errors.New("agent does not hold the claim for this spec")

// ExecutionBackend defines storage operations for execution bundles and agent callbacks.
type ExecutionBackend interface {
	// GenerateBundle assembles a bundle from the spec, its decisions, and the constitution.
	GenerateBundle(ctx context.Context, slug string) (*Bundle, error)

	// RecordProgress stores a progress event from an executing agent.
	RecordProgress(ctx context.Context, slug, agent, message string) error

	// RecordBlocker stores a blocker event from an executing agent.
	RecordBlocker(ctx context.Context, slug, agent, description string) error

	// RecordCompletion stores a completion event and transitions spec to done.
	RecordCompletion(ctx context.Context, slug, agent string) error

	// GetExecutionEvents returns execution events for a spec, ordered by time descending.
	GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*ExecutionEvent, error)

	// GetPrimeData returns the data needed to compose a prime response.
	GetPrimeData(ctx context.Context, slug string) (*PrimeData, error)

	// ReleaseExpiredClaims finds and releases all CLAIMED_BY relationships past their lease.
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}
