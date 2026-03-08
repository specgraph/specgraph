// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package memgraph implements storage backends using Memgraph via the Bolt protocol.
package memgraph

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/seanb4t/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	id := newID("spec")
	nowStr := nowRFC3339()

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
			updated_at: $updated_at,
			lifecycle: $lifecycle,
			history_json: $history_json
		})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	params := map[string]any{
		"id":           id,
		"slug":         slug,
		"intent":       intent,
		"stage":        defaultInitialStage,
		"priority":     priority,
		"complexity":   complexity,
		"version":      int64(1),
		"created_at":   nowStr,
		"updated_at":   nowStr,
		"lifecycle":    "task",
		"history_json": "[]",
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: create spec returned no records")
	}

	return recordToSpec(records[0])
}

// GetSpec retrieves a spec by slug.
func (s *Store) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	query := `
		MATCH (s:Spec {slug: $slug})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	params := map[string]any{"slug": slug}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(records[0])
}

// ListSpecs returns specs matching the given filters.
func (s *Store) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error) {
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
	query += " RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity, s.version, s.created_at, s.updated_at, s.lifecycle, s.superseded_by, s.supersedes, s.history_json"
	query += " ORDER BY s.created_at"
	if limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(limit)
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list specs: %w", err)
	}

	specs := make([]*storage.Spec, 0, len(records))
	for _, rec := range records {
		sp, err := recordToSpec(rec)
		if err != nil {
			return nil, err
		}
		specs = append(specs, sp)
	}
	return specs, nil
}

// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
func (s *Store) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity *string) (*storage.Spec, error) {
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

	nowStr := nowRFC3339()
	setClauses = append(setClauses, "s.version = s.version + 1", "s.updated_at = $updated_at")
	params["updated_at"] = nowStr

	query := fmt.Sprintf(`
		MATCH (s:Spec {slug: $slug})
		SET %s
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`, strings.Join(setClauses, ", "))

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(records[0])
}

// ClearAll removes all nodes and relationships from the graph.
// Intended for test cleanup only.
func (s *Store) ClearAll(ctx context.Context) error {
	_, err := s.executeQuery(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("memgraph: clear all: %w", err)
	}
	return nil
}

// Close releases the driver resources.
func (s *Store) Close(ctx context.Context) error {
	if err := s.driver.Close(ctx); err != nil {
		return fmt.Errorf("memgraph: close: %w", err)
	}
	return nil
}

// newID produces a prefixed ULID: prefix + "-" + ULID.
// ULIDs are 128-bit and lexicographically sortable by timestamp.
func newID(prefix string) string {
	return prefix + "-" + ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// nowRFC3339 returns the current UTC time formatted as an RFC 3339 string.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// parseRFC3339 parses an RFC3339 timestamp string from a memgraph record field.
func parseRFC3339(field, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
	}
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

// safeInt32 clamps an int64 to the int32 range, preventing overflow on conversion.
func safeInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// recordStringOptional extracts a string value from a neo4j record by position,
// returning "" for nil/null values. Use for nullable string fields like superseded_by.
func recordStringOptional(rec *neo4j.Record, pos int) string {
	if pos >= len(rec.Values) || rec.Values[pos] == nil {
		return ""
	}
	s, ok := rec.Values[pos].(string)
	if !ok {
		return ""
	}
	return s
}

// historyEntryJSON is a JSON-serializable form of storage.HistoryEntry.
type historyEntryJSON struct {
	Version int32  `json:"version"`
	Stage   string `json:"stage"`
	Summary string `json:"summary"`
	Reason  string `json:"reason"`
	Date    string `json:"date"`
}

// unmarshalHistory parses a JSON string into a slice of storage.HistoryEntry.
func unmarshalHistory(raw string) ([]storage.HistoryEntry, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var entries []historyEntryJSON
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("memgraph: unmarshal history_json: %w", err)
	}
	result := make([]storage.HistoryEntry, len(entries))
	for i, e := range entries {
		t, err := parseRFC3339("history.date", e.Date)
		if err != nil {
			return nil, err
		}
		result[i] = storage.HistoryEntry{
			Version: e.Version,
			Stage:   e.Stage,
			Summary: e.Summary,
			Reason:  e.Reason,
			Date:    t,
		}
	}
	return result, nil
}

// recordToSpec converts a neo4j record (with positional values) to a *storage.Spec.
func recordToSpec(rec *neo4j.Record) (*storage.Spec, error) {
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

	// New fields at positions 9-12.
	lifecycle := recordStringOptional(rec, 9)
	if lifecycle == "" {
		lifecycle = "task"
	}
	supersededBy := recordStringOptional(rec, 10)
	supersedes := recordStringOptional(rec, 11)
	historyJSON := recordStringOptional(rec, 12)

	history, err := unmarshalHistory(historyJSON)
	if err != nil {
		return nil, err
	}

	return &storage.Spec{
		ID:           id,
		Slug:         slug,
		Intent:       intent,
		Stage:        storage.SpecStage(stage),
		Priority:     storage.SpecPriority(priority),
		Complexity:   complexity,
		Version:      safeInt32(version),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Lifecycle:    lifecycle,
		SupersededBy: supersededBy,
		Supersedes:   supersedes,
		History:      history,
	}, nil
}
