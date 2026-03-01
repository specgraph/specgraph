// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package memgraph implements storage backends using Memgraph via the Bolt protocol.
package memgraph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Store implements storage.Backend using Memgraph (Bolt protocol).
type Store struct {
	driver neo4j.DriverWithContext
}

// New creates a new Memgraph-backed Store and verifies connectivity.
func New(ctx context.Context, boltURI string) (*Store, error) {
	driver, err := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("memgraph: create driver: %w", err)
	}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("memgraph: verify connectivity: %w", err)
	}
	return &Store{driver: driver}, nil
}

const defaultInitialStage = "spark"

// CreateSpec stores a new spec node in Memgraph and returns it.
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error) {
	now := time.Now().UTC()
	id := generateID("spec", slug, now)
	nowStr := now.Format(time.RFC3339)

	query := `
		CREATE (s:Spec {
			id: $id,
			slug: $slug,
			intent: $intent,
			stage: $stage,
			priority: $priority,
			complexity: $complexity,
			version: $version,
			created_at: $created_at,
			updated_at: $updated_at
		})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at
	`
	params := map[string]any{
		"id":         id,
		"slug":       slug,
		"intent":     intent,
		"stage":      defaultInitialStage,
		"priority":   priority,
		"complexity": complexity,
		"version":    int64(1),
		"created_at": nowStr,
		"updated_at": nowStr,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: create spec returned no records")
	}

	return recordToSpec(result.Records[0])
}

// GetSpec retrieves a spec by slug.
func (s *Store) GetSpec(ctx context.Context, slug string) (*specv1.Spec, error) {
	query := `
		MATCH (s:Spec {slug: $slug})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at
	`
	params := map[string]any{"slug": slug}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(result.Records[0])
}

// ListSpecs returns specs matching the given filters.
func (s *Store) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error) {
	var clauses []string
	params := map[string]any{}

	if stage != "" {
		clauses = append(clauses, "s.stage = $stage")
		params["stage"] = stage
	}
	if priority != "" {
		clauses = append(clauses, "s.priority = $priority")
		params["priority"] = priority
	}

	query := "MATCH (s:Spec)"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity, s.version, s.created_at, s.updated_at"
	query += " ORDER BY s.created_at"
	if limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(limit)
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list specs: %w", err)
	}

	specs := make([]*specv1.Spec, 0, len(result.Records))
	for _, rec := range result.Records {
		sp, err := recordToSpec(rec)
		if err != nil {
			return nil, err
		}
		specs = append(specs, sp)
	}
	return specs, nil
}

// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
func (s *Store) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity *string) (*specv1.Spec, error) {
	var setClauses []string
	params := map[string]any{"slug": slug}

	if intent != nil {
		setClauses = append(setClauses, "s.intent = $intent")
		params["intent"] = *intent
	}
	if stage != nil {
		setClauses = append(setClauses, "s.stage = $stage")
		params["stage"] = *stage
	}
	if priority != nil {
		setClauses = append(setClauses, "s.priority = $priority")
		params["priority"] = *priority
	}
	if complexity != nil {
		setClauses = append(setClauses, "s.complexity = $complexity")
		params["complexity"] = *complexity
	}

	if len(setClauses) == 0 {
		return s.GetSpec(ctx, slug)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	setClauses = append(setClauses, "s.version = s.version + 1", "s.updated_at = $updated_at")
	params["updated_at"] = nowStr

	query := fmt.Sprintf(`
		MATCH (s:Spec {slug: $slug})
		SET %s
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at
	`, strings.Join(setClauses, ", "))

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(result.Records[0])
}

// Close releases the driver resources.
func (s *Store) Close(ctx context.Context) error {
	if err := s.driver.Close(ctx); err != nil {
		return fmt.Errorf("memgraph: close: %w", err)
	}
	return nil
}

// generateID produces a prefixed ID: prefix + "-" + first 7 hex chars of sha256(slug + now).
func generateID(prefix, slug string, now time.Time) string {
	h := sha256.Sum256([]byte(slug + now.String()))
	return prefix + "-" + hex.EncodeToString(h[:])[:7]
}

// parseRFC3339 parses an RFC3339 timestamp string from a memgraph record field.
func parseRFC3339(field, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("memgraph: parse %s %q: %w", field, value, err)
	}
	return t, nil
}

// recordString extracts a string value from a neo4j record by position.
// It returns an error if the value is not a string, preventing silent data corruption.
func recordString(rec *neo4j.Record, pos int, field string) (string, error) {
	v, ok := rec.Values[pos].(string)
	if !ok {
		return "", fmt.Errorf("memgraph: field %q at position %d: expected string, got %T", field, pos, rec.Values[pos])
	}
	return v, nil
}

// recordInt64 extracts an int64 value from a neo4j record by position.
// It returns an error if the value is not an int64, preventing silent data corruption.
func recordInt64(rec *neo4j.Record, pos int, field string) (int64, error) {
	v, ok := rec.Values[pos].(int64)
	if !ok {
		return 0, fmt.Errorf("memgraph: field %q at position %d: expected int64, got %T", field, pos, rec.Values[pos])
	}
	return v, nil
}

// recordToSpec converts a neo4j record (with positional values) to a *specv1.Spec.
func recordToSpec(rec *neo4j.Record) (*specv1.Spec, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	slug, err := recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}
	intent, err := recordString(rec, 2, "intent")
	if err != nil {
		return nil, err
	}
	stage, err := recordString(rec, 3, "stage")
	if err != nil {
		return nil, err
	}
	priority, err := recordString(rec, 4, "priority")
	if err != nil {
		return nil, err
	}
	complexity, err := recordString(rec, 5, "complexity")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 6, "version")
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

	return &specv1.Spec{
		Id:         id,
		Slug:       slug,
		Intent:     intent,
		Stage:      stage,
		Priority:   priority,
		Complexity: complexity,
		Version:    int32(version),
		CreatedAt:  timestamppb.New(createdAt),
		UpdatedAt:  timestamppb.New(updatedAt),
	}, nil
}
