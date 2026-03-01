// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultLeaseDuration = 15 * time.Minute

// ClaimSpec creates a CLAIMED_BY relationship between a spec and an agent.
// If the spec is already claimed by another agent (with a non-expired lease), it returns an error.
func (s *Store) ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*specv1.Claim, error) {
	if leaseDuration == 0 {
		leaseDuration = defaultLeaseDuration
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expiresStr := now.Add(leaseDuration).Format(time.RFC3339)

	// Delete any expired claims first, then check for active ones
	query := `
		MATCH (s:Spec {slug: $slug})
		OPTIONAL MATCH (s)-[old:CLAIMED_BY]->()
		WHERE old.lease_expires < $now
		DELETE old
		WITH s
		OPTIONAL MATCH (s)-[active:CLAIMED_BY]->(existing)
		WHERE active.lease_expires >= $now
		WITH s, active
		WHERE active IS NULL
		CREATE (s)-[r:CLAIMED_BY {agent: $agent, claimed_at: $claimed_at, lease_expires: $lease_expires}]->(a:Agent {name: $agent})
		RETURN r.agent, r.claimed_at, r.lease_expires
	`
	params := map[string]any{
		"slug":          slug,
		"agent":         agent,
		"now":           nowStr,
		"claimed_at":    nowStr,
		"lease_expires": expiresStr,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: claim spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q not found or already claimed", slug)
	}

	return recordToClaim(slug, result.Records[0])
}

// UnclaimSpec removes the CLAIMED_BY relationship for a specific agent.
func (s *Store) UnclaimSpec(ctx context.Context, slug, agent string) error {
	query := `
		MATCH (s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
		DELETE r, a
	`
	params := map[string]any{"slug": slug, "agent": agent}

	_, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: unclaim spec: %w", err)
	}
	return nil
}

// Heartbeat extends the lease for a claimed spec.
func (s *Store) Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*specv1.Claim, error) {
	if extendBy == 0 {
		extendBy = defaultLeaseDuration
	}

	newExpires := time.Now().UTC().Add(extendBy).Format(time.RFC3339)

	query := `
		MATCH (s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
		SET r.lease_expires = $lease_expires
		RETURN r.agent, r.claimed_at, r.lease_expires
	`
	params := map[string]any{
		"slug":          slug,
		"agent":         agent,
		"lease_expires": newExpires,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: heartbeat: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: no active claim for spec %q by agent %q", slug, agent)
	}

	return recordToClaim(slug, result.Records[0])
}

func recordToClaim(slug string, rec *neo4j.Record) (*specv1.Claim, error) {
	agent := recordString(rec, 0)
	claimedAtStr := recordString(rec, 1)
	leaseExpiresStr := recordString(rec, 2)

	claimedAt, err := time.Parse(time.RFC3339, claimedAtStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse claimed_at %q: %w", claimedAtStr, err)
	}
	leaseExpires, err := time.Parse(time.RFC3339, leaseExpiresStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse lease_expires %q: %w", leaseExpiresStr, err)
	}

	return &specv1.Claim{
		SpecSlug:     slug,
		Agent:        agent,
		ClaimedAt:    timestamppb.New(claimedAt),
		LeaseExpires: timestamppb.New(leaseExpires),
	}, nil
}
