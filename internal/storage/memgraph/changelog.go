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

// Compile-time interface assertion.
var _ storage.ChangeLogBackend = (*Store)(nil)

// changeLogFieldChangeJSON is the JSON-serializable representation of a field change.
type changeLogFieldChangeJSON struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// marshalFieldChanges serializes a slice of FieldChange to a JSON string
// for storage as a property on the ChangeLog node.
func marshalFieldChanges(changes []storage.FieldChange) (string, error) {
	if len(changes) == 0 {
		return "[]", nil
	}
	items := make([]changeLogFieldChangeJSON, len(changes))
	for i, c := range changes {
		items[i] = changeLogFieldChangeJSON{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("memgraph: marshal field changes: %w", err)
	}
	return string(b), nil
}

// unmarshalFieldChanges deserializes a JSON string into a slice of FieldChange.
func unmarshalFieldChanges(raw string) ([]storage.FieldChange, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []changeLogFieldChangeJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("memgraph: unmarshal field changes: %w", err)
	}
	changes := make([]storage.FieldChange, len(items))
	for i, item := range items {
		changes[i] = storage.FieldChange{
			Field:    item.Field,
			OldValue: item.OldValue,
			NewValue: item.NewValue,
		}
	}
	return changes, nil
}

// recordBool extracts a bool value from a neo4j record by position.
// It returns an error if the value is nil or not a bool.
func recordBool(rec *neo4j.Record, pos int, name string) (bool, error) {
	if pos >= len(rec.Values) || rec.Values[pos] == nil {
		return false, fmt.Errorf("memgraph: %s at position %d is nil", name, pos)
	}
	v, ok := rec.Values[pos].(bool)
	if !ok {
		return false, fmt.Errorf("memgraph: %s at position %d: expected bool, got %T", name, pos, rec.Values[pos])
	}
	return v, nil
}

// createChangeLog creates a ChangeLog node and links it to a Spec via a HAS_CHANGE edge.
// The entry's ID is generated here if empty. The changes slice is serialized to JSON
// and stored as a property on the node.
//
// A version guard ensures the spec's current version matches entry.Version at
// write time. If another writer modified the spec between the mutation and this
// call, the guard returns 0 rows and we return ErrConcurrentModification.
func (s *Store) createChangeLog(ctx context.Context, slug string, entry *storage.ChangeLogEntry, changes []storage.FieldChange) error {
	if entry.ID == "" {
		entry.ID = newID("cl")
	}
	dateStr := entry.Date.UTC().Format(sortableRFC3339Nano)
	changesJSON, err := marshalFieldChanges(changes)
	if err != nil {
		return err
	}

	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		WHERE s.version = $expected_version
		CREATE (s)-[:HAS_CHANGE]->(cl:ChangeLog {
			id: $id,
			version: $version,
			stage: $stage,
			content_hash: $content_hash,
			checkpoint: $checkpoint,
			summary: $summary,
			reason: $reason,
			changes_json: $changes_json,
			date: $date
		})
		RETURN cl.id
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"slug":             slug,
		"expected_version": int64(entry.Version),
		"id":               entry.ID,
		"version":          int64(entry.Version),
		"stage":            string(entry.Stage),
		"content_hash":     entry.ContentHash,
		"checkpoint":       entry.Checkpoint,
		"summary":          entry.Summary,
		"reason":           entry.Reason,
		"changes_json":     changesJSON,
		"date":             dateStr,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return fmt.Errorf("memgraph: create changelog: %w", err)
	}
	if len(records) == 0 {
		// Version guard returned 0 rows. The caller already verified the spec
		// exists (via the mutation query that matched it), so 0 rows here means
		// the version changed between the mutation and this call.
		return fmt.Errorf("memgraph: create changelog for %q (version %d): %w", slug, entry.Version, storage.ErrConcurrentModification)
	}
	return nil
}

// ListChanges returns changelog entries for a spec, ordered by version ascending.
// Returns ErrSpecNotFound if the spec slug does not exist. Returns an empty
// slice (not an error) if the spec has no changelog entries.
func (s *Store) ListChanges(ctx context.Context, slug string, opts storage.ChangeLogFilter) ([]*storage.ChangeLogEntry, error) {
	// First verify the spec exists.
	checkQuery := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.slug`
	checkRecords, err := s.executeQuery(ctx, checkQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: list changes: check spec: %w", err)
	}
	if len(checkRecords) == 0 {
		return nil, fmt.Errorf("memgraph: list changes %q: %w", slug, storage.ErrSpecNotFound)
	}

	// Build the changelog query with optional filters.
	var whereClauses []string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if opts.CheckpointsOnly {
		whereClauses = append(whereClauses, "cl.checkpoint = true")
	}
	if opts.SinceVersion > 0 {
		whereClauses = append(whereClauses, "cl.version > $since_version")
		params["since_version"] = int64(opts.SinceVersion)
	}

	query := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:HAS_CHANGE]->(cl:ChangeLog)`
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	query += `
		RETURN cl.id, cl.version, cl.stage, cl.content_hash,
		       cl.checkpoint, cl.summary, cl.reason, cl.changes_json, cl.date
		ORDER BY cl.version
	`
	if opts.Limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(opts.Limit)
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list changes: %w", err)
	}

	entries := make([]*storage.ChangeLogEntry, 0, len(records))
	for _, rec := range records {
		entry, parseErr := recordToChangeLogEntry(rec)
		if parseErr != nil {
			return nil, parseErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// recordToChangeLogEntry parses a neo4j record into a ChangeLogEntry.
// Expected positional columns:
//
//	0: cl.id, 1: cl.version, 2: cl.stage, 3: cl.content_hash,
//	4: cl.checkpoint, 5: cl.summary, 6: cl.reason, 7: cl.changes_json, 8: cl.date
func recordToChangeLogEntry(rec *neo4j.Record) (*storage.ChangeLogEntry, error) {
	id, err := recordString(rec, 0, "cl.id")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 1, "cl.version")
	if err != nil {
		return nil, err
	}
	stage, err := recordString(rec, 2, "cl.stage")
	if err != nil {
		return nil, err
	}
	contentHash, err := recordString(rec, 3, "cl.content_hash")
	if err != nil {
		return nil, err
	}
	checkpoint, err := recordBool(rec, 4, "cl.checkpoint")
	if err != nil {
		return nil, err
	}
	summary, err := recordString(rec, 5, "cl.summary")
	if err != nil {
		return nil, err
	}
	reason, err := recordStringOptional(rec, 6, "cl.reason")
	if err != nil {
		return nil, err
	}
	changesJSON, err := recordStringOptional(rec, 7, "cl.changes_json")
	if err != nil {
		return nil, err
	}
	dateStr, err := recordString(rec, 8, "cl.date")
	if err != nil {
		return nil, err
	}

	date, err := parseRFC3339("cl.date", dateStr)
	if err != nil {
		return nil, err
	}

	changes, err := unmarshalFieldChanges(changesJSON)
	if err != nil {
		return nil, err
	}

	return &storage.ChangeLogEntry{
		ID:          id,
		Version:     safeInt32(version),
		Stage:       storage.SpecStage(stage),
		ContentHash: contentHash,
		Checkpoint:  checkpoint,
		Summary:     summary,
		Reason:      reason,
		Changes:     changes,
		Date:        date,
	}, nil
}

// ListAllChanges returns all changelog entries across all specs in the project.
// SpecSlug is populated from the spec_slug column for each entry.
func (s *Store) ListAllChanges(ctx context.Context) ([]*storage.ChangeLogEntry, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(spec:Spec)-[:HAS_CHANGE]->(cl:ChangeLog)
		RETURN cl.id, spec.slug AS spec_slug, cl.version, cl.stage, cl.content_hash,
		       cl.checkpoint, cl.summary, cl.reason, cl.changes_json, cl.date
		ORDER BY spec.slug, cl.version
	`
	records, err := s.executeQuery(ctx, query, s.projectParam())
	if err != nil {
		return nil, fmt.Errorf("memgraph: list all changes: %w", err)
	}

	entries := make([]*storage.ChangeLogEntry, 0, len(records))
	for _, rec := range records {
		id, err := recordString(rec, 0, "cl.id")
		if err != nil {
			return nil, err
		}
		specSlug, err := recordString(rec, 1, "spec_slug")
		if err != nil {
			return nil, err
		}
		version, err := recordInt64(rec, 2, "cl.version")
		if err != nil {
			return nil, err
		}
		stage, err := recordString(rec, 3, "cl.stage")
		if err != nil {
			return nil, err
		}
		contentHash, err := recordString(rec, 4, "cl.content_hash")
		if err != nil {
			return nil, err
		}
		checkpoint, err := recordBool(rec, 5, "cl.checkpoint")
		if err != nil {
			return nil, err
		}
		summary, err := recordString(rec, 6, "cl.summary")
		if err != nil {
			return nil, err
		}
		reason, err := recordStringOptional(rec, 7, "cl.reason")
		if err != nil {
			return nil, err
		}
		changesJSON, err := recordStringOptional(rec, 8, "cl.changes_json")
		if err != nil {
			return nil, err
		}
		dateStr, err := recordString(rec, 9, "cl.date")
		if err != nil {
			return nil, err
		}
		date, err := parseRFC3339("cl.date", dateStr)
		if err != nil {
			return nil, err
		}
		changes, err := unmarshalFieldChanges(changesJSON)
		if err != nil {
			return nil, err
		}

		entries = append(entries, &storage.ChangeLogEntry{
			ID:          id,
			SpecSlug:    specSlug,
			Version:     safeInt32(version),
			Stage:       storage.SpecStage(stage),
			ContentHash: contentHash,
			Checkpoint:  checkpoint,
			Summary:     summary,
			Reason:      reason,
			Changes:     changes,
			Date:        date,
		})
	}
	return entries, nil
}

// EnsureChangeLogIndexes creates indexes on ChangeLog nodes for efficient queries.
// Called from ensureIndexes during Store initialization.
func (s *Store) EnsureChangeLogIndexes(ctx context.Context) error {
	indexes := []string{
		"CREATE INDEX ON :ChangeLog(version)",
		"CREATE INDEX ON :ChangeLog(date)",
	}
	for _, stmt := range indexes {
		session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
		_, runErr := session.Run(ctx, stmt, nil)
		closeErr := session.Close(ctx)
		if runErr != nil && !strings.Contains(runErr.Error(), "already exists") {
			if closeErr != nil {
				return errors.Join(
					fmt.Errorf("create changelog index %q: %w", stmt, runErr),
					fmt.Errorf("close session: %w", closeErr),
				)
			}
			return fmt.Errorf("create changelog index %q: %w", stmt, runErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close session after changelog index %q: %w", stmt, closeErr)
		}
	}
	return nil
}
