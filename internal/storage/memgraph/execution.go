// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

// GenerateBundle assembles a bundle from the spec and its linked decisions.
func (s *Store) GenerateBundle(ctx context.Context, slug string) (*storage.Bundle, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle: %w", err)
	}

	if spec.Stage != storage.SpecStageApproved && string(spec.Stage) != "in_progress" {
		return nil, fmt.Errorf("memgraph: generate bundle for %q: %w", slug, storage.ErrSpecNotApproved)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle decisions: %w", err)
	}

	return &storage.Bundle{
		Version:   1,
		Spec:      spec,
		Decisions: decisions,
	}, nil
}

// RecordProgress stores a progress event from an executing agent.
func (s *Store) RecordProgress(ctx context.Context, slug, agent, message string) error {
	return s.recordClaimedEvent(ctx, slug, agent, "progress", message)
}

// RecordBlocker stores a blocker event from an executing agent.
func (s *Store) RecordBlocker(ctx context.Context, slug, agent, description string) error {
	return s.recordClaimedEvent(ctx, slug, agent, "blocker", description)
}

// RecordCompletion stores a completion event and transitions the spec to done.
func (s *Store) RecordCompletion(ctx context.Context, slug, agent string) error {
	now := s.nowTime()
	id := newID("evt")
	nowStr := now.Format(time.RFC3339Nano)

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Single query: assert claim, create event, transition to done, release claim.
		query := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
			WHERE r.lease_expires >= $now
			CREATE (e:ExecutionEvent {
				id: $id,
				spec_slug: $spec_slug,
				agent: $agent,
				type: "completion",
				message: "",
				created_at: $created_at
			})
			CREATE (s)-[:HAS_EVENT]->(e)
			SET s.stage = "done", s.updated_at = $now, s.version = s.version + 1
			DELETE r
			RETURN e.id
		`
		params := mergeParams(s.projectParam(), map[string]any{
			"slug":       slug,
			"id":         id,
			"spec_slug":  slug,
			"agent":      agent,
			"now":        nowStr,
			"created_at": nowStr,
		})

		records, err := s.executeQuery(txCtx, query, params)
		if err != nil {
			return fmt.Errorf("memgraph: record completion: %w", err)
		}
		if len(records) == 0 {
			return fmt.Errorf("memgraph: record completion: %w", storage.ErrAgentNotClaimOwner)
		}
		if err := s.RefreshDependencyHashes(txCtx, slug); err != nil {
			return fmt.Errorf("memgraph: refresh dependency hashes after completion: %w", err)
		}
		return nil
	})
}

// GetExecutionEvents returns execution events for a spec, ordered by time descending.
func (s *Store) GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*storage.ExecutionEvent, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:HAS_EVENT]->(e:ExecutionEvent)
		RETURN DISTINCT e.id, e.spec_slug, e.agent, e.type, e.message, e.created_at
		ORDER BY e.id DESC
		LIMIT $limit
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":  slug,
		"limit": int64(limit),
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get execution events: %w", err)
	}

	events := make([]*storage.ExecutionEvent, 0, len(records))
	for _, rec := range records {
		evt, err := recordToExecutionEvent(rec)
		if err != nil {
			return nil, err
		}
		events = append(events, evt)
	}
	return events, nil
}

// GetPrimeData returns the data needed to compose a prime response.
func (s *Store) GetPrimeData(ctx context.Context, slug string) (*storage.PrimeData, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get prime data: %w", err)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get prime data decisions: %w", err)
	}

	var constitution *storage.Constitution
	constitution, err = s.GetConstitution(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, fmt.Errorf("memgraph: get prime data constitution: %w", err)
		}
		// No constitution is fine — leave it nil.
	}

	return &storage.PrimeData{
		Spec:         spec,
		Decisions:    decisions,
		Constitution: constitution,
	}, nil
}

// ReleaseExpiredClaims finds and releases all CLAIMED_BY relationships past their lease.
func (s *Store) ReleaseExpiredClaims(ctx context.Context) (int, error) {
	nowStr := s.now()

	query := `
		MATCH ()-[r:CLAIMED_BY]->()
		WHERE r.lease_expires < $now
		DELETE r
		RETURN count(r) AS released
	`
	records, err := s.executeQuery(ctx, query, map[string]any{"now": nowStr})
	if err != nil {
		return 0, fmt.Errorf("memgraph: release expired claims: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	released, err := recordInt64(records[0], 0, "released")
	if err != nil {
		return 0, fmt.Errorf("memgraph: release expired claims count: %w", err)
	}
	return int(released), nil
}

// recordClaimedEvent atomically verifies claim ownership and creates an execution event.
// The claim check and event creation happen in a single Cypher query, preventing TOCTOU races.
func (s *Store) recordClaimedEvent(ctx context.Context, slug, agent, eventType, message string) error {
	now := s.nowTime()
	id := newID("evt")
	nowStr := now.Format(time.RFC3339Nano)

	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:CLAIMED_BY {agent: $agent}]->(a)
		WHERE r.lease_expires >= $now
		CREATE (e:ExecutionEvent {
			id: $id,
			spec_slug: $spec_slug,
			agent: $agent,
			type: $type,
			message: $message,
			created_at: $created_at
		})
		CREATE (s)-[:HAS_EVENT]->(e)
		RETURN e.id
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":       slug,
		"id":         id,
		"spec_slug":  slug,
		"agent":      agent,
		"type":       eventType,
		"message":    message,
		"now":        nowStr,
		"created_at": nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return fmt.Errorf("memgraph: record execution event: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: record %s: %w", eventType, storage.ErrAgentNotClaimOwner)
	}
	return nil
}

// fetchLinkedDecisions retrieves all decisions linked to a spec via DECIDED_IN edges.
func (s *Store) fetchLinkedDecisions(ctx context.Context, slug string) ([]*storage.Decision, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:DECIDED_IN]->(d:Decision)
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at, d.content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, err
	}

	decisions := make([]*storage.Decision, 0, len(records))
	for _, rec := range records {
		d, err := recordToDecision(rec)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

// recordToExecutionEvent converts a neo4j record to a *storage.ExecutionEvent.
func recordToExecutionEvent(rec *neo4j.Record) (*storage.ExecutionEvent, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	specSlug, err := recordString(rec, 1, "spec_slug")
	if err != nil {
		return nil, err
	}
	agent, err := recordString(rec, 2, "agent")
	if err != nil {
		return nil, err
	}
	typeStr, err := recordString(rec, 3, "type")
	if err != nil {
		return nil, err
	}
	message, err := recordString(rec, 4, "message")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, 5, "created_at")
	if err != nil {
		return nil, err
	}

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}

	eventType, err := storage.ParseExecutionEventType(typeStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse event type: %w", err)
	}

	return &storage.ExecutionEvent{
		ID:        id,
		SpecSlug:  specSlug,
		Agent:     agent,
		Type:      eventType,
		Message:   message,
		CreatedAt: createdAt,
	}, nil
}
