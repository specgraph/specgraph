// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrSpecNotFound is returned when a spec does not exist.
var ErrSpecNotFound = errors.New("spec not found")

// ErrSpecAlreadyClaimed is returned when a spec has an active claim by another agent.
var ErrSpecAlreadyClaimed = errors.New("spec already claimed")

// ClaimBackend defines storage operations for spec claims/leases.
type ClaimBackend interface {
	// ClaimSpec creates a CLAIMED_BY relationship between a spec and an agent.
	ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*specv1.Claim, error)

	// UnclaimSpec removes the CLAIMED_BY relationship.
	UnclaimSpec(ctx context.Context, slug, agent string) error

	// Heartbeat extends the lease for a claimed spec.
	Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*specv1.Claim, error)
}
