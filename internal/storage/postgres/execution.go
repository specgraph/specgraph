// SPDX-License-Identifier: Apache-2.0
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

// Compile-time interface assertion.
var _ storage.ExecutionBackend = (*Store)(nil)

// GenerateBundle assembles a bundle from the spec and its linked decisions,
// active claim, and upstream dependencies with drift state.
func (s *Store) GenerateBundle(ctx context.Context, slug string) (*storage.Bundle, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: generate bundle: %w", err)
	}

	if spec.Stage != storage.SpecStageApproved && string(spec.Stage) != "in_progress" {
		return nil, fmt.Errorf("postgres: generate bundle for %q: %w", slug, storage.ErrSpecNotApproved)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: generate bundle decisions: %w", err)
	}

	claim, err := s.fetchActiveClaim(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: generate bundle claim: %w", err)
	}

	deps, err := s.fetchBundleDependencies(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: generate bundle dependencies: %w", err)
	}

	return &storage.Bundle{
		Version:      2,
		Spec:         spec,
		Decisions:    decisions,
		Claim:        claim,
		Dependencies: deps,
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
// The entire operation is atomic: claim verification, event insertion, spec
// stage transition, content hash recomputation, changelog checkpoint, claim
// deletion, and dependency hash refresh all occur in a single transaction.
func (s *Store) RecordCompletion(ctx context.Context, slug, agent string) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.assertActiveClaim(txCtx, slug, agent); err != nil {
			return err
		}

		// Insert completion event.
		eventID := newID("evt")
		now := s.now()
		_, err := s.exec(txCtx,
			`INSERT INTO execution_events (id, spec_slug, project_slug, agent, event_type, message, created_at)
			 VALUES ($1, $2, $3, $4, 'completion', '', $5)`,
			eventID, slug, s.project, agent, now,
		)
		if err != nil {
			return fmt.Errorf("postgres: record completion event: %w", err)
		}

		// Insert HAS_EVENT edge.
		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'HAS_EVENT', $3)`,
			slug, eventID, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: record completion HAS_EVENT edge: %w", err)
		}

		// Read current spec to get stage and version for changelog.
		var stage string
		var version int32
		err = s.queryRow(txCtx,
			`SELECT stage, version
			 FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&stage, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: record completion: spec %q: %w", slug, storage.ErrSpecNotFound)
		}
		if err != nil {
			return fmt.Errorf("postgres: record completion: read spec: %w", err)
		}

		newStage := "done"

		// Transition spec to done, bump version.
		tag, err := s.exec(txCtx,
			`UPDATE specs SET stage = 'done', version = version + 1,
			     updated_at = $1
			 WHERE slug = $2 AND project_slug = $3`,
			now, slug, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: record completion: update spec: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgres: record completion: spec %q not updated", slug)
		}

		// Recompute content hash including authoring outputs.
		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return fmt.Errorf("postgres: record completion: %w", hashErr)
		}

		// Re-read spec to get updated content hash for changelog.
		var ch string
		err = s.queryRow(txCtx,
			`SELECT content_hash FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&ch)
		if err != nil {
			return fmt.Errorf("postgres: record completion: read content_hash: %w", err)
		}

		// Create checkpoint changelog entry.
		oldFields := &storage.SpecFields{Stage: stage}
		newFields := &storage.SpecFields{Stage: newStage}
		deltas := storage.ComputeFieldDeltas(oldFields, newFields)
		clEntry := &storage.ChangeLogEntry{
			Version:     version + 1,
			Stage:       newStage,
			ContentHash: ch,
			Checkpoint:  true,
			Summary:     "Spec completed",
			Date:        now,
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}

		// Delete the claim.
		_, err = s.exec(txCtx,
			`DELETE FROM claims WHERE project_slug = $1 AND spec_slug = $2 AND agent = $3`,
			s.project, slug, agent,
		)
		if err != nil {
			return fmt.Errorf("postgres: record completion: delete claim: %w", err)
		}

		// Delete CLAIMED_BY edge.
		_, err = s.exec(txCtx,
			`DELETE FROM edges WHERE project_slug = $1 AND from_slug = $2 AND to_slug = $3 AND edge_type = 'CLAIMED_BY'`,
			s.project, slug, agent,
		)
		if err != nil {
			return fmt.Errorf("postgres: record completion: delete CLAIMED_BY edge: %w", err)
		}

		// Refresh dependency hashes on all specs that depend on this one.
		if err := s.RefreshDependencyHashes(txCtx, slug); err != nil {
			return fmt.Errorf("postgres: record completion: refresh dependency hashes: %w", err)
		}

		return nil
	})
}

// GetExecutionEvents returns execution events for a spec, ordered by time descending.
// When limit is 0, all events are returned (no LIMIT clause).
func (s *Store) GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*storage.ExecutionEvent, error) {
	rows, err := s.query(ctx,
		`SELECT id, spec_slug, agent, event_type, message, created_at
		 FROM execution_events
		 WHERE spec_slug = $1 AND project_slug = $2
		 ORDER BY created_at DESC, id DESC
		 LIMIT CASE WHEN $3 > 0 THEN $3 END`,
		slug, s.project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get execution events: %w", err)
	}
	defer rows.Close()

	var events []*storage.ExecutionEvent
	for rows.Next() {
		var (
			id        string
			specSlug  string
			agent     string
			typeStr   string
			message   string
			createdAt time.Time
		)
		if scanErr := rows.Scan(&id, &specSlug, &agent, &typeStr, &message, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("postgres: get execution events: scan: %w", scanErr)
		}
		eventType, parseErr := storage.ParseExecutionEventType(typeStr)
		if parseErr != nil {
			return nil, fmt.Errorf("postgres: get execution events: parse type: %w", parseErr)
		}
		events = append(events, &storage.ExecutionEvent{
			ID:        id,
			SpecSlug:  specSlug,
			Agent:     agent,
			Type:      eventType,
			Message:   message,
			CreatedAt: createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get execution events: iterate: %w", err)
	}
	return events, nil
}

// GetPrimeData returns the data needed to compose a prime response.
func (s *Store) GetPrimeData(ctx context.Context, slug string) (*storage.PrimeData, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prime data: %w", err)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prime data decisions: %w", err)
	}

	var constitution *storage.Constitution
	constitution, err = s.GetConstitution(ctx)
	if err != nil {
		if !errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, fmt.Errorf("postgres: get prime data constitution: %w", err)
		}
	}

	return &storage.PrimeData{
		Spec:         spec,
		Decisions:    decisions,
		Constitution: constitution,
	}, nil
}

// ReleaseExpiredClaims finds and releases all claims past their lease expiry.
// Returns the count of released claims.
func (s *Store) ReleaseExpiredClaims(ctx context.Context) (int, error) {
	var count int
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		now := s.now()

		rows, qErr := s.query(txCtx,
			`DELETE FROM claims
			 WHERE project_slug = $1 AND lease_expires < $2
			 RETURNING spec_slug, agent`,
			s.project, now,
		)
		if qErr != nil {
			return fmt.Errorf("postgres: release expired claims: %w", qErr)
		}
		defer rows.Close()

		type claimRow struct{ specSlug, agent string }
		var expired []claimRow
		for rows.Next() {
			var cr claimRow
			if scanErr := rows.Scan(&cr.specSlug, &cr.agent); scanErr != nil {
				return fmt.Errorf("postgres: release expired claims: scan: %w", scanErr)
			}
			expired = append(expired, cr)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return fmt.Errorf("postgres: release expired claims: iterate: %w", rowsErr)
		}

		for _, cr := range expired {
			_, delErr := s.exec(txCtx,
				`DELETE FROM edges WHERE project_slug = $1 AND from_slug = $2 AND to_slug = $3 AND edge_type = 'CLAIMED_BY'`,
				s.project, cr.specSlug, cr.agent,
			)
			if delErr != nil {
				return fmt.Errorf("postgres: release expired claims: delete edge for %q: %w", cr.specSlug, delErr)
			}
		}

		count = len(expired)
		return nil
	})
	return count, err
}

// fetchActiveClaim returns the active (non-expired) claim for a spec, or nil if unclaimed.
func (s *Store) fetchActiveClaim(ctx context.Context, slug string) (*storage.Claim, error) {
	now := s.now()
	var agent string
	var claimedAt, leaseExpires time.Time

	err := s.queryRow(ctx,
		`SELECT agent, claimed_at, lease_expires
		 FROM claims
		 WHERE project_slug = $1 AND spec_slug = $2 AND lease_expires >= $3`,
		s.project, slug, now,
	).Scan(&agent, &claimedAt, &leaseExpires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: fetch active claim: %w", err)
	}

	return &storage.Claim{
		Slug:         slug,
		Agent:        agent,
		ClaimedAt:    claimedAt,
		LeaseExpires: leaseExpires,
	}, nil
}

// fetchBundleDependencies returns dependency info with drift state for the bundle.
func (s *Store) fetchBundleDependencies(ctx context.Context, slug string) ([]storage.DependencyInfo, error) {
	refs, err := s.GetDependenciesWithEdgeData(ctx, slug)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}

	deps := make([]storage.DependencyInfo, 0, len(refs))
	for _, ref := range refs {
		drifted := ref.ContentHashAtLink == "" || ref.ContentHashAtLink != ref.UpstreamContentHash
		var note string
		if drifted {
			if ref.ContentHashAtLink == "" {
				note = "dependency not yet baselined"
			} else {
				note = "content changed since baseline"
			}
		}
		deps = append(deps, storage.DependencyInfo{
			Slug:    ref.Slug,
			Stage:   storage.SpecStage(ref.Stage),
			Drifted: drifted,
			Note:    note,
		})
	}
	return deps, nil
}

// recordClaimedEvent verifies claim ownership and atomically inserts an execution event
// and its HAS_EVENT edge within a single transaction.
func (s *Store) recordClaimedEvent(ctx context.Context, slug, agent, eventType, message string) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.assertActiveClaim(txCtx, slug, agent); err != nil {
			return err
		}

		eventID := newID("evt")
		now := s.now()

		_, err := s.exec(txCtx,
			`INSERT INTO execution_events (id, spec_slug, project_slug, agent, event_type, message, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			eventID, slug, s.project, agent, eventType, message, now,
		)
		if err != nil {
			return fmt.Errorf("postgres: record %s event: %w", eventType, err)
		}

		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'HAS_EVENT', $3)`,
			slug, eventID, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: record %s HAS_EVENT edge: %w", eventType, err)
		}

		return nil
	})
}

// assertActiveClaim checks that the given agent holds a non-expired claim on slug.
// Returns ErrAgentNotClaimOwner if the claim does not exist or belongs to someone else.
func (s *Store) assertActiveClaim(ctx context.Context, slug, agent string) error {
	now := s.now()
	var found string
	err := s.queryRow(ctx,
		`SELECT agent FROM claims
		 WHERE project_slug = $1 AND spec_slug = $2 AND agent = $3 AND lease_expires >= $4`,
		s.project, slug, agent, now,
	).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("postgres: %w", storage.ErrAgentNotClaimOwner)
	}
	if err != nil {
		return fmt.Errorf("postgres: assert active claim: %w", err)
	}
	return nil
}

// fetchLinkedDecisions retrieves all decisions linked to a spec via DECIDED_IN edges.
func (s *Store) fetchLinkedDecisions(ctx context.Context, slug string) ([]*storage.Decision, error) {
	rows, err := s.query(ctx,
		`SELECT to_slug FROM edges
		 WHERE from_slug = $1 AND edge_type = 'DECIDED_IN' AND project_slug = $2`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: fetch linked decisions: %w", err)
	}
	defer rows.Close()

	var decisionSlugs []string
	for rows.Next() {
		var toSlug string
		if scanErr := rows.Scan(&toSlug); scanErr != nil {
			return nil, fmt.Errorf("postgres: fetch linked decisions: scan: %w", scanErr)
		}
		decisionSlugs = append(decisionSlugs, toSlug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: fetch linked decisions: iterate: %w", err)
	}

	decisions := make([]*storage.Decision, 0, len(decisionSlugs))
	for _, dSlug := range decisionSlugs {
		d, err := s.GetDecision(ctx, dSlug)
		if err != nil {
			return nil, fmt.Errorf("postgres: fetch linked decisions: get %q: %w", dSlug, err)
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}
