// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

const defaultLeaseDuration = 15 * time.Minute

// ClaimSpec creates a CLAIMED_BY relationship between a spec and an agent.
// If the spec is already claimed by another agent (with a non-expired lease), it returns an error.
func (s *Store) ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*storage.Claim, error) {
	if leaseDuration == 0 {
		leaseDuration = defaultLeaseDuration
	}

	now := s.nowTime()
	nowStr := now.Format(time.RFC3339)
	expiresStr := now.Add(leaseDuration).Format(time.RFC3339)

	var claim *storage.Claim

	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify the spec exists inside the transaction.
		existsRecords, err := s.executeQuery(txCtx,
			`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.slug`,
			mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if err != nil {
			return fmt.Errorf("memgraph: claim spec existence check: %w", err)
		}
		if len(existsRecords) == 0 {
			return fmt.Errorf("memgraph: claim spec %q: %w", slug, storage.ErrSpecNotFound)
		}

		// Delete expired claims, check for active ones, and create new claim atomically.
		query := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			OPTIONAL MATCH (s)-[old:CLAIMED_BY]->()
			WHERE old.lease_expires < $now
			DELETE old
			WITH DISTINCT s
			OPTIONAL MATCH (s)-[active:CLAIMED_BY]->(existing)
			WHERE active.lease_expires >= $now
			WITH s, active
			WHERE active IS NULL
			MERGE (a:Agent {name: $agent})
			CREATE (s)-[r:CLAIMED_BY {agent: $agent, claimed_at: $claimed_at, lease_expires: $lease_expires}]->(a)
			RETURN r.agent, r.claimed_at, r.lease_expires
		`
		params := mergeParams(s.projectParam(), map[string]any{
			"slug":          slug,
			"agent":         agent,
			"now":           nowStr,
			"claimed_at":    nowStr,
			"lease_expires": expiresStr,
		})

		records, err := s.executeQuery(txCtx, query, params)
		if err != nil {
			return fmt.Errorf("memgraph: claim spec: %w", err)
		}
		if len(records) == 0 {
			return fmt.Errorf("memgraph: claim spec %q: %w", slug, storage.ErrSpecAlreadyClaimed)
		}

		claim, err = recordToClaim(slug, records[0])
		return err
	})
	if err != nil {
		return nil, err
	}

	return claim, nil
}

// UnclaimSpec removes the CLAIMED_BY relationship for a specific agent.
// Returns ErrSpecNotClaimed if no claim exists, ErrNotClaimOwner if another agent owns the claim.
func (s *Store) UnclaimSpec(ctx context.Context, slug, agent string) error {
	// First check if any claim exists and who owns it.
	checkQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		OPTIONAL MATCH (s)-[r:CLAIMED_BY]->(a)
		RETURN r.agent AS claim_agent
	`
	checkRecords, err := s.executeQuery(ctx, checkQuery,
		mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return fmt.Errorf("memgraph: unclaim spec check: %w", err)
	}

	if len(checkRecords) == 0 {
		return fmt.Errorf("memgraph: unclaim spec %q: %w", slug, storage.ErrSpecNotClaimed)
	}

	rec := checkRecords[0]
	claimAgentVal, _ := rec.Get("claim_agent")
	if claimAgentVal == nil {
		return fmt.Errorf("memgraph: unclaim spec %q: %w", slug, storage.ErrSpecNotClaimed)
	}

	claimAgent, ok := claimAgentVal.(string)
	if !ok || claimAgent == "" {
		return fmt.Errorf("memgraph: unclaim spec %q: %w", slug, storage.ErrSpecNotClaimed)
	}

	if claimAgent != agent {
		return fmt.Errorf("memgraph: unclaim spec %q: %w", slug, storage.ErrNotClaimOwner)
	}

	// Agent matches — delete the claim relationship.
	deleteQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
		DELETE r
	`
	_, err = s.executeQuery(ctx, deleteQuery,
		mergeParams(s.projectParam(), map[string]any{"slug": slug, "agent": agent}))
	if err != nil {
		return fmt.Errorf("memgraph: unclaim spec: %w", err)
	}
	return nil
}

// Heartbeat extends the lease for a claimed spec.
func (s *Store) Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*storage.Claim, error) {
	if extendBy == 0 {
		extendBy = defaultLeaseDuration
	}

	newExpires := s.nowTime().Add(extendBy).Format(time.RFC3339)

	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
		SET r.lease_expires = $lease_expires
		RETURN r.agent, r.claimed_at, r.lease_expires
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":          slug,
		"agent":         agent,
		"lease_expires": newExpires,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: heartbeat: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: no active claim for spec %q by agent %q", slug, agent)
	}

	return recordToClaim(slug, records[0])
}

func recordToClaim(slug string, rec *neo4j.Record) (*storage.Claim, error) {
	agent, err := recordString(rec, 0, "agent")
	if err != nil {
		return nil, err
	}
	claimedAtStr, err := recordString(rec, 1, "claimed_at")
	if err != nil {
		return nil, err
	}
	leaseExpiresStr, err := recordString(rec, 2, "lease_expires")
	if err != nil {
		return nil, err
	}

	claimedAt, err := parseRFC3339("claimed_at", claimedAtStr)
	if err != nil {
		return nil, err
	}
	leaseExpires, err := parseRFC3339("lease_expires", leaseExpiresStr)
	if err != nil {
		return nil, err
	}

	return &storage.Claim{
		Slug:         slug,
		Agent:        agent,
		ClaimedAt:    claimedAt,
		LeaseExpires: leaseExpires,
	}, nil
}
