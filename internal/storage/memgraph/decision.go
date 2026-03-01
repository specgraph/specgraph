// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"strings"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateDecision stores a new decision node in Memgraph.
func (s *Store) CreateDecision(ctx context.Context, slug, title, decision, rationale string) (*specv1.Decision, error) {
	now := time.Now().UTC()
	id := generateID("dec", slug, now)
	nowStr := now.Format(time.RFC3339)

	query := `
		CREATE (d:Decision {
			id: $id,
			slug: $slug,
			title: $title,
			status: $status,
			decision: $decision,
			rationale: $rationale,
			superseded_by: $superseded_by,
			created_at: $created_at,
			updated_at: $updated_at
		})
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at
	`
	params := map[string]any{
		"id":            id,
		"slug":          slug,
		"title":         title,
		"status":        specv1.DecisionStatus_DECISION_STATUS_PROPOSED.String(),
		"decision":      decision,
		"rationale":     rationale,
		"superseded_by": "",
		"created_at":    nowStr,
		"updated_at":    nowStr,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create decision: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: create decision returned no records")
	}

	return recordToDecision(result.Records[0])
}

// GetDecision retrieves a decision by slug.
func (s *Store) GetDecision(ctx context.Context, slug string) (*specv1.Decision, error) {
	query := `
		MATCH (d:Decision {slug: $slug})
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at
	`
	params := map[string]any{"slug": slug}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get decision: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: decision %q not found", slug)
	}

	return recordToDecision(result.Records[0])
}

// ListDecisions returns decisions matching the given filters.
func (s *Store) ListDecisions(ctx context.Context, status specv1.DecisionStatus, limit int) ([]*specv1.Decision, error) {
	var clauses []string
	params := map[string]any{}

	if status != specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED {
		clauses = append(clauses, "d.status = $status")
		params["status"] = status.String()
	}

	query := "MATCH (d:Decision)"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale, d.superseded_by, d.created_at, d.updated_at"
	query += " ORDER BY d.created_at"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list decisions: %w", err)
	}

	decisions := make([]*specv1.Decision, 0, len(result.Records))
	for _, rec := range result.Records {
		d, err := recordToDecision(rec)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
func (s *Store) UpdateDecision(ctx context.Context, slug string, title *string, status *specv1.DecisionStatus, decision, rationale, supersededBy *string) (*specv1.Decision, error) {
	var setClauses []string
	params := map[string]any{"slug": slug}

	if title != nil {
		setClauses = append(setClauses, "d.title = $title")
		params["title"] = *title
	}
	if status != nil {
		setClauses = append(setClauses, "d.status = $status")
		params["status"] = status.String()
	}
	if decision != nil {
		setClauses = append(setClauses, "d.decision = $decision")
		params["decision"] = *decision
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

	nowStr := time.Now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "d.updated_at = $updated_at")
	params["updated_at"] = nowStr

	query := fmt.Sprintf(`
		MATCH (d:Decision {slug: $slug})
		SET %s
		RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		       d.superseded_by, d.created_at, d.updated_at
	`, strings.Join(setClauses, ", "))

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update decision: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: decision %q not found", slug)
	}

	return recordToDecision(result.Records[0])
}

func recordToDecision(rec *neo4j.Record) (*specv1.Decision, error) {
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
	decision, err := recordString(rec, 4, "decision")
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

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}

	statusVal, ok := specv1.DecisionStatus_value[statusStr]
	if !ok {
		statusVal = int32(specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED)
	}

	return &specv1.Decision{
		Id:           id,
		Slug:         slug,
		Title:        title,
		Status:       specv1.DecisionStatus(statusVal),
		Decision:     decision,
		Rationale:    rationale,
		SupersededBy: supersededBy,
		CreatedAt:    timestamppb.New(createdAt),
		UpdatedAt:    timestamppb.New(updatedAt),
	}, nil
}
