// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

// CreateSlice persists a new :Slice node with BELONGS_TO and COMPOSES edges.
func (s *Store) CreateSlice(ctx context.Context, sl *storage.Slice) error {
	if sl == nil {
		return fmt.Errorf("memgraph: CreateSlice: slice must not be nil")
	}

	id := newID("slc")
	nowStr := s.now()

	verifyJSON := marshalStringSlice(sl.Verify)
	touchesJSON := marshalStringSlice(sl.Touches)
	dependsOnJSON := marshalStringSlice(sl.DependsOn)

	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(parent:Spec {slug: $parent_slug})
		CREATE (sl:Slice {
			id: $id,
			slug: $slug,
			parent_slug: $parent_slug,
			slice_id: $slice_id,
			intent: $intent,
			verify_json: $verify_json,
			touches_json: $touches_json,
			depends_on_json: $depends_on_json,
			status: $status,
			assigned_to: '',
			created_at: $now,
			updated_at: $now
		})
		CREATE (sl)-[:BELONGS_TO]->(p)
		CREATE (sl)-[:COMPOSES]->(parent)
		RETURN sl.id
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"id":              id,
		"slug":            sl.Slug,
		"parent_slug":     sl.ParentSlug,
		"slice_id":        sl.SliceID,
		"intent":          sl.Intent,
		"verify_json":     string(verifyJSON),
		"touches_json":    string(touchesJSON),
		"depends_on_json": string(dependsOnJSON),
		"status":          string(storage.SliceStatusOpen),
		"now":             nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return fmt.Errorf("memgraph: create slice %q: %w", sl.Slug, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: create slice %q: parent spec %q not found", sl.Slug, sl.ParentSlug)
	}
	return nil
}

// ListSlices returns all slices for a parent spec, ordered by creation time.
func (s *Store) ListSlices(ctx context.Context, parentSlug string) ([]*storage.Slice, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(sl:Slice)-[:COMPOSES]->(parent:Spec {slug: $parent_slug})
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent,
		       sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status,
		       sl.assigned_to, sl.created_at, sl.updated_at
		ORDER BY sl.created_at
	`
	params := mergeParams(s.projectParam(), map[string]any{"parent_slug": parentSlug})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list slices for %q: %w", parentSlug, err)
	}

	slices := make([]*storage.Slice, 0, len(records))
	for _, rec := range records {
		sl, err := recordToSlice(rec)
		if err != nil {
			return nil, err
		}
		slices = append(slices, sl)
	}
	return slices, nil
}

// GetSlice returns a single slice by its full slug.
func (s *Store) GetSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(sl:Slice {slug: $slug})
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent,
		       sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status,
		       sl.assigned_to, sl.created_at, sl.updated_at
	`
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get slice %q: %w", slug, err)
	}
	if len(records) == 0 {
		return nil, storage.ErrSliceNotFound
	}
	return recordToSlice(records[0])
}

// ClaimSlice transitions a slice to claimed status and records the assignee.
func (s *Store) ClaimSlice(ctx context.Context, slug, assignee string) (*storage.Slice, error) {
	if strings.TrimSpace(assignee) == "" {
		return nil, fmt.Errorf("memgraph: claim slice %q: assignee must not be blank", slug)
	}
	nowStr := s.now()
	// Use OPTIONAL MATCH + WITH to distinguish "not found" from "wrong status".
	// If the slice doesn't exist, sl is NULL. If it exists but has wrong status,
	// sl is non-NULL but the WHERE filters it out of the SET.
	query := `
		MATCH (p:Project {slug: $project})
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(sl:Slice {slug: $slug})
		WITH sl, CASE WHEN sl IS NULL THEN 'missing' WHEN sl.status <> $expected_status THEN 'wrong_status' ELSE 'ok' END AS check
		WHERE check = 'ok'
		SET sl.status = $new_status, sl.assigned_to = $assignee, sl.updated_at = $now
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent, sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status, sl.assigned_to, sl.created_at, sl.updated_at, check
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":            slug,
		"expected_status": string(storage.SliceStatusOpen),
		"new_status":      string(storage.SliceStatusClaimed),
		"assignee":        assignee,
		"now":             nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: claim slice %q: %w", slug, err)
	}
	if len(records) == 0 {
		// check = 'ok' matched nothing. Determine why with a simple existence check.
		return nil, s.sliceNotFoundOrWrongStatus(ctx, slug)
	}
	return recordToSlice(records[0])
}

// CompleteSlice transitions a slice to done status.
func (s *Store) CompleteSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})
		OPTIONAL MATCH (p)<-[:BELONGS_TO]-(sl:Slice {slug: $slug})
		WITH sl, CASE WHEN sl IS NULL THEN 'missing' WHEN sl.status <> $expected_status THEN 'wrong_status' ELSE 'ok' END AS check
		WHERE check = 'ok'
		SET sl.status = $new_status, sl.updated_at = $now
		RETURN sl.id, sl.slug, sl.parent_slug, sl.slice_id, sl.intent, sl.verify_json, sl.touches_json, sl.depends_on_json, sl.status, sl.assigned_to, sl.created_at, sl.updated_at, check
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":            slug,
		"expected_status": string(storage.SliceStatusClaimed),
		"new_status":      string(storage.SliceStatusDone),
		"now":             nowStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: complete slice %q: %w", slug, err)
	}
	if len(records) == 0 {
		return nil, s.sliceNotFoundOrWrongStatus(ctx, slug)
	}
	return recordToSlice(records[0])
}

// sliceNotFoundOrWrongStatus checks whether a slice exists to distinguish
// ErrSliceNotFound from ErrSliceWrongStatus when a conditional update returns 0 rows.
func (s *Store) sliceNotFoundOrWrongStatus(ctx context.Context, slug string) error {
	_, err := s.GetSlice(ctx, slug)
	if err == nil {
		// Slice exists but status didn't match the WHERE condition.
		return fmt.Errorf("memgraph: slice %q: %w", slug, storage.ErrSliceWrongStatus)
	}
	if errors.Is(err, storage.ErrSliceNotFound) {
		return fmt.Errorf("memgraph: slice %q: %w", slug, storage.ErrSliceNotFound)
	}
	// Real backend error — propagate it, don't mask as not-found.
	return fmt.Errorf("memgraph: slice %q status check: %w", slug, err)
}

// marshalStringSlice marshals a []string to JSON. This cannot fail because
// []string contains no unmarshalable types (channels, funcs, complex numbers).
func marshalStringSlice(s []string) []byte {
	b, _ := json.Marshal(s) //nolint:errcheck // []string marshal is infallible
	return b
}

// recordToSlice converts a neo4j record to a *storage.Slice.
func recordToSlice(rec *neo4j.Record) (*storage.Slice, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	slug, err := recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}
	parentSlug, err := recordString(rec, 2, "parent_slug")
	if err != nil {
		return nil, err
	}
	sliceID, err := recordString(rec, 3, "slice_id")
	if err != nil {
		return nil, err
	}
	intent, err := recordString(rec, 4, "intent")
	if err != nil {
		return nil, err
	}
	verifyJSON, err := recordStringOptional(rec, 5, "verify_json")
	if err != nil {
		return nil, err
	}
	touchesJSON, err := recordStringOptional(rec, 6, "touches_json")
	if err != nil {
		return nil, err
	}
	dependsOnJSON, err := recordStringOptional(rec, 7, "depends_on_json")
	if err != nil {
		return nil, err
	}
	status, err := recordString(rec, 8, "status")
	if err != nil {
		return nil, err
	}
	assignedTo, err := recordStringOptional(rec, 9, "assigned_to")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, 10, "created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordString(rec, 11, "updated_at")
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

	var verify []string
	if verifyJSON != "" {
		if err := json.Unmarshal([]byte(verifyJSON), &verify); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal verify_json: %w", err)
		}
	}
	var touches []string
	if touchesJSON != "" {
		if err := json.Unmarshal([]byte(touchesJSON), &touches); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal touches_json: %w", err)
		}
	}
	var dependsOn []string
	if dependsOnJSON != "" {
		if err := json.Unmarshal([]byte(dependsOnJSON), &dependsOn); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal depends_on_json: %w", err)
		}
	}

	return &storage.Slice{
		ID:         id,
		Slug:       slug,
		ParentSlug: parentSlug,
		SliceID:    sliceID,
		Intent:     intent,
		Verify:     verify,
		Touches:    touches,
		DependsOn:  dependsOn,
		Status:     storage.SliceStatus(status),
		AssignedTo: assignedTo,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}
