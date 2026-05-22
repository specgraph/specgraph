// SPDX-License-Identifier: Apache-2.0
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

	// GetActiveClaim returns the currently active (non-expired) claim for
	// the given spec, or nil if the spec is unclaimed. Used by the prime
	// composer to populate SpecView.Claims; expired claims are filtered
	// out at the storage layer so callers do not have to check lease times.
	GetActiveClaim(ctx context.Context, slug string) (*Claim, error)
}
