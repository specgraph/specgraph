// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package memgraph implements storage backends using Memgraph via the Bolt protocol.
package memgraph

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

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

// CreateSpec stores a new spec node in Memgraph and returns it.
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error) {
	id := generateID(slug)
	now := time.Now().UTC()
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
		"stage":      "spark",
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
		return nil, fmt.Errorf("memgraph: spec %q not found", slug)
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
		query += fmt.Sprintf(" LIMIT %d", limit)
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
		return nil, fmt.Errorf("memgraph: spec %q not found", slug)
	}

	return recordToSpec(result.Records[0])
}

// Close releases the driver resources.
func (s *Store) Close(ctx context.Context) error {
	return fmt.Errorf("memgraph: close: %w", s.driver.Close(ctx))
}

// generateID produces "spec-" + first 7 hex chars of sha256(slug + now).
func generateID(slug string) string {
	h := sha256.Sum256([]byte(slug + time.Now().String()))
	return fmt.Sprintf("spec-%x", h[:4])[:12] // "spec-" (5) + 7 hex chars = 12
}

// recordString extracts a string value from a neo4j record at the given index.
func recordString(rec *neo4j.Record, i int) string {
	v, ok := rec.Values[i].(string)
	if !ok {
		return ""
	}
	return v
}

// recordInt64 extracts an int64 value from a neo4j record at the given index.
func recordInt64(rec *neo4j.Record, i int) int64 {
	v, ok := rec.Values[i].(int64)
	if !ok {
		return 0
	}
	return v
}

// recordToSpec converts a neo4j record (with positional values) to a *specv1.Spec.
func recordToSpec(rec *neo4j.Record) (*specv1.Spec, error) {
	createdAtStr := recordString(rec, 7)
	updatedAtStr := recordString(rec, 8)

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse created_at %q: %w", createdAtStr, err)
	}
	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse updated_at %q: %w", updatedAtStr, err)
	}

	version := recordInt64(rec, 6)
	if version > int64(math.MaxInt32) {
		version = int64(math.MaxInt32)
	}

	return &specv1.Spec{
		Id:         recordString(rec, 0),
		Slug:       recordString(rec, 1),
		Intent:     recordString(rec, 2),
		Stage:      recordString(rec, 3),
		Priority:   recordString(rec, 4),
		Complexity: recordString(rec, 5),
		Version:    int32(version),
		CreatedAt:  timestamppb.New(createdAt),
		UpdatedAt:  timestamppb.New(updatedAt),
	}, nil
}
