// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// ClaimBackend defines storage operations for spec claims/leases.
type ClaimBackend interface {
	// ClaimSpec creates a CLAIMED_BY relationship between a spec and an agent.
	ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*Claim, error)

	// UnclaimSpec removes the CLAIMED_BY relationship.
	UnclaimSpec(ctx context.Context, slug, agent string) error

	// Heartbeat extends the lease for a claimed spec.
	Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*Claim, error)
}
