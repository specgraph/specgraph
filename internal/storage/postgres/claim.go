// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

const defaultLeaseDuration = 15 * time.Minute

// ClaimSpec creates or renews a claim on a spec by the given agent.
// If the spec is already claimed by another agent with a non-expired lease, returns ErrSpecAlreadyClaimed.
func (s *Store) ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*storage.Claim, error) {
	if leaseDuration == 0 {
		leaseDuration = defaultLeaseDuration
	}

	now := s.now()
	leaseExpires := now.Add(leaseDuration)

	var claim *storage.Claim

	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify the spec exists in this project.
		var exists int
		err := s.queryRow(txCtx,
			`SELECT 1 FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: claim spec %q: %w", slug, storage.ErrSpecNotFound)
		}
		if err != nil {
			return fmt.Errorf("postgres: claim spec existence check: %w", err)
		}

		// Delete expired claims first.
		_, err = s.exec(txCtx,
			`DELETE FROM claims WHERE project_slug = $1 AND spec_slug = $2 AND lease_expires < $3`,
			s.project, slug, now,
		)
		if err != nil {
			return fmt.Errorf("postgres: claim spec delete expired: %w", err)
		}

		// Remove CLAIMED_BY edges whose agent no longer has a claim.
		_, err = s.exec(txCtx,
			`DELETE FROM edges
			 WHERE from_slug = $1 AND edge_type = 'CLAIMED_BY' AND project_slug = $2
			   AND NOT EXISTS (
			     SELECT 1 FROM claims
			     WHERE spec_slug = $1 AND project_slug = $2 AND agent = edges.to_slug
			   )`,
			slug, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: claim spec delete orphaned edges: %w", err)
		}

		// Check for an active claim.
		var activeAgent string
		var activeClaimed, activeExpires time.Time
		err = s.queryRow(txCtx,
			`SELECT agent, claimed_at, lease_expires FROM claims WHERE project_slug = $1 AND spec_slug = $2`,
			s.project, slug,
		).Scan(&activeAgent, &activeClaimed, &activeExpires)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: claim spec check active: %w", err)
		}
		if err == nil {
			// Active claim exists.
			if activeAgent != agent {
				return fmt.Errorf("postgres: claim spec %q: %w", slug, storage.ErrSpecAlreadyClaimed)
			}
			// Same agent — refresh the lease.
			var newExpires time.Time
			err = s.queryRow(txCtx,
				`UPDATE claims SET lease_expires = $1 WHERE project_slug = $2 AND spec_slug = $3
				 RETURNING lease_expires`,
				leaseExpires, s.project, slug,
			).Scan(&newExpires)
			if err != nil {
				return fmt.Errorf("postgres: claim spec refresh lease: %w", err)
			}
			claim = &storage.Claim{
				Slug:         slug,
				Agent:        activeAgent,
				ClaimedAt:    activeClaimed,
				LeaseExpires: newExpires,
			}
			return nil
		}

		// No active claim — insert a new one.
		var claimedAt time.Time
		err = s.queryRow(txCtx,
			`INSERT INTO claims (spec_slug, project_slug, agent, lease_expires, claimed_at)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING claimed_at`,
			slug, s.project, agent, leaseExpires, now,
		).Scan(&claimedAt)
		if err != nil {
			return fmt.Errorf("postgres: claim spec insert: %w", err)
		}

		// Insert CLAIMED_BY edge.
		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'CLAIMED_BY', $3)
			 ON CONFLICT (project_slug, from_slug, to_slug, edge_type) DO NOTHING`,
			slug, agent, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: claim spec insert edge: %w", err)
		}

		claim = &storage.Claim{
			Slug:         slug,
			Agent:        agent,
			ClaimedAt:    claimedAt,
			LeaseExpires: leaseExpires,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return claim, nil
}

// UnclaimSpec removes the active claim on a spec. Returns ErrSpecNotClaimed if no claim
// exists, or ErrNotClaimOwner if the claim belongs to a different agent.
func (s *Store) UnclaimSpec(ctx context.Context, slug, agent string) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		var activeAgent string
		err := s.queryRow(txCtx,
			`SELECT agent FROM claims WHERE project_slug = $1 AND spec_slug = $2`,
			s.project, slug,
		).Scan(&activeAgent)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: unclaim spec %q: %w", slug, storage.ErrSpecNotClaimed)
		}
		if err != nil {
			return fmt.Errorf("postgres: unclaim spec check: %w", err)
		}

		if activeAgent != agent {
			return fmt.Errorf("postgres: unclaim spec %q: %w", slug, storage.ErrNotClaimOwner)
		}

		_, err = s.exec(txCtx,
			`DELETE FROM claims WHERE project_slug = $1 AND spec_slug = $2 AND agent = $3`,
			s.project, slug, agent,
		)
		if err != nil {
			return fmt.Errorf("postgres: unclaim spec delete: %w", err)
		}

		_, err = s.exec(txCtx,
			`DELETE FROM edges WHERE project_slug = $1 AND from_slug = $2 AND to_slug = $3 AND edge_type = 'CLAIMED_BY'`,
			s.project, slug, agent,
		)
		if err != nil {
			return fmt.Errorf("postgres: unclaim spec delete edge: %w", err)
		}

		return nil
	})
}

// Heartbeat extends the lease for a spec claimed by the given agent.
// Returns an error if no active claim exists for the agent.
func (s *Store) Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*storage.Claim, error) {
	if extendBy == 0 {
		extendBy = defaultLeaseDuration
	}

	now := s.now()
	newExpires := now.Add(extendBy)

	var claimedAt, leaseExpires time.Time
	err := s.queryRow(ctx,
		`UPDATE claims SET lease_expires = $1
		 WHERE project_slug = $2 AND spec_slug = $3 AND agent = $4 AND lease_expires >= $5
		 RETURNING claimed_at, lease_expires`,
		newExpires, s.project, slug, agent, now,
	).Scan(&claimedAt, &leaseExpires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: heartbeat: no active claim for spec %q by agent %q", slug, agent)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: heartbeat: %w", err)
	}

	return &storage.Claim{
		Slug:         slug,
		Agent:        agent,
		ClaimedAt:    claimedAt,
		LeaseExpires: leaseExpires,
	}, nil
}
