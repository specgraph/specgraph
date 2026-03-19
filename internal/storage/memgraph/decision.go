// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/contenthash"
)

// CreateDecision stores a new decision node in Memgraph.
func (s *Store) CreateDecision(ctx context.Context, slug, title, body, rationale string) (*storage.Decision, error) {
	now := s.nowTime()
	id := newID("dec")
	nowStr := now.Format(time.RFC3339)
	initialStatus := string(storage.DecisionStatusProposed)
	ch := contenthash.Decision(title, initialStatus, body, rationale)

	query := `
		MATCH (p:Project {slug: $project})
		CREATE (p)<-[:BELONGS_TO]-(d:Decision {
			id: $id,
			slug: $slug,
			title: $title,
			status: $status,
			decision: $decision,
			rationale: $rationale,
			superseded_by: $superseded_by,
			created_at: $created_at,
			updated_at: $updated_at,
			content_hash: $content_hash
		})
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at, d.content_hash
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"id":            id,
		"slug":          slug,
		"title":         title,
		"status":        initialStatus,
		"decision":      body,
		"rationale":     rationale,
		"superseded_by": "",
		"created_at":    nowStr,
		"updated_at":    nowStr,
		"content_hash":  ch,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create decision: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: create decision returned no records")
	}

	return recordToDecision(records[0])
}

// GetDecision retrieves a decision by slug.
func (s *Store) GetDecision(ctx context.Context, slug string) (*storage.Decision, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at, d.content_hash
	`
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get decision: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: decision %q: %w", slug, storage.ErrDecisionNotFound)
	}

	return recordToDecision(records[0])
}

// ListDecisions returns decisions matching the given filters.
func (s *Store) ListDecisions(ctx context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error) {
	var clauses []string
	params := s.projectParam()

	if status != "" {
		clauses = append(clauses, "d.status = $status")
		params["status"] = string(status)
	}

	query := "MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision)"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale, d.superseded_by, d.created_at, d.updated_at, d.content_hash"
	query += " ORDER BY d.created_at"
	if limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(limit)
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list decisions: %w", err)
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

// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
func (s *Store) UpdateDecision(ctx context.Context, slug string, title *string, status *storage.DecisionStatus, body, rationale, supersededBy *string) (*storage.Decision, error) {
	if status != nil && *status == storage.DecisionStatusSuperseded {
		if supersededBy == nil || *supersededBy == "" {
			return nil, storage.ErrSupersededByRequired
		}
	}

	var setClauses []string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if title != nil {
		setClauses = append(setClauses, "d.title = $title")
		params["title"] = *title
	}
	if status != nil {
		setClauses = append(setClauses, "d.status = $status")
		params["status"] = string(*status)
	}
	if body != nil {
		setClauses = append(setClauses, "d.decision = $decision")
		params["decision"] = *body
	}
	if rationale != nil {
		setClauses = append(setClauses, "d.rationale = $rationale")
		params["rationale"] = *rationale
	}
	if supersededBy != nil {
		setClauses = append(setClauses, "d.superseded_by = $superseded_by")
		params["superseded_by"] = *supersededBy
	}

	if len(setClauses) == 0 {
		return s.GetDecision(ctx, slug)
	}

	nowStr := s.nowTime().Format(time.RFC3339)
	setClauses = append(setClauses, "d.updated_at = $updated_at")
	params["updated_at"] = nowStr

	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})
		SET %s
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at, d.content_hash
	`, strings.Join(setClauses, ", "))

	var result *storage.Decision
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		if qErr != nil {
			return fmt.Errorf("memgraph: update decision: %w", qErr)
		}
		if len(records) == 0 {
			return fmt.Errorf("memgraph: decision %q: %w", slug, storage.ErrDecisionNotFound)
		}

		dec, parseErr := recordToDecision(records[0])
		if parseErr != nil {
			return parseErr
		}
		ch := contenthash.Decision(dec.Title, string(dec.Status), dec.Body, dec.Rationale)

		hashQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})
			SET d.content_hash = $content_hash
		`
		if _, hashErr := s.executeQuery(txCtx, hashQuery, mergeParams(s.projectParam(), map[string]any{
			"slug":         slug,
			"content_hash": ch,
		})); hashErr != nil {
			return fmt.Errorf("memgraph: update decision content_hash: %w", hashErr)
		}
		dec.ContentHash = ch
		result = dec
		return nil
	})
	return result, err
}

func recordToDecision(rec *neo4j.Record) (*storage.Decision, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	slug, err := recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}
	title, err := recordString(rec, 2, "title")
	if err != nil {
		return nil, err
	}
	statusStr, err := recordString(rec, 3, "status")
	if err != nil {
		return nil, err
	}
	body, err := recordString(rec, 4, "decision")
	if err != nil {
		return nil, err
	}
	rationale, err := recordString(rec, 5, "rationale")
	if err != nil {
		return nil, err
	}
	supersededBy, err := recordString(rec, 6, "superseded_by")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, 7, "created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordString(rec, 8, "updated_at")
	if err != nil {
		return nil, err
	}
	contentHash, err := recordStringOptional(rec, 9, "content_hash")
	if err != nil {
		return nil, err
	}

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}

	status := storage.DecisionStatus(statusStr)
	switch status {
	case storage.DecisionStatusProposed, storage.DecisionStatusAccepted,
		storage.DecisionStatusSuperseded, storage.DecisionStatusDeprecated:
		// valid
	default:
		if statusStr == "DECISION_STATUS_UNSPECIFIED" || statusStr == "" {
			status = storage.DecisionStatusProposed
		} else {
			return nil, fmt.Errorf("memgraph: unknown decision status %q", statusStr)
		}
	}

	return &storage.Decision{
		ID:           id,
		Slug:         slug,
		Title:        title,
		Status:       status,
		Body:         body,
		Rationale:    rationale,
		SupersededBy: supersededBy,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		ContentHash:  contentHash,
	}, nil
}
