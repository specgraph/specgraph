// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.ChangeLogBackend = (*Store)(nil)

// fieldChangeJSON is the JSON-serializable representation of a field change.
type fieldChangeJSON struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// marshalFieldChanges serializes a slice of FieldChange to JSON bytes for
// storage in the changes JSONB column.
func marshalFieldChanges(changes []storage.FieldChange) ([]byte, error) {
	if len(changes) == 0 {
		return []byte("[]"), nil
	}
	items := make([]fieldChangeJSON, len(changes))
	for i, c := range changes {
		items[i] = fieldChangeJSON{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("postgres: marshal field changes: %w", err)
	}
	return b, nil
}

// unmarshalFieldChanges deserializes JSONB bytes into a slice of FieldChange.
func unmarshalFieldChanges(data []byte) ([]storage.FieldChange, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var items []fieldChangeJSON
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("postgres: unmarshal field changes: %w", err)
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

// createChangeLog inserts a changelog entry and a HAS_CHANGE edge for the given
// spec slug. All writes use s.exec so they participate in the caller's transaction.
//
// The entry.ID is generated here if empty.
func (s *Store) createChangeLog(ctx context.Context, slug string, entry *storage.ChangeLogEntry, changes []storage.FieldChange) error {
	if entry.ID == "" {
		entry.ID = newID("cl")
	}

	changesJSON, err := marshalFieldChanges(changes)
	if err != nil {
		return err
	}

	_, err = s.exec(ctx,
		`INSERT INTO changelog_entries
			(id, spec_slug, project_slug, version, stage, content_hash, checkpoint, summary, reason, changes, date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		entry.ID, slug, s.project,
		entry.Version, entry.Stage, entry.ContentHash,
		entry.Checkpoint, entry.Summary, entry.Reason,
		changesJSON, entry.Date,
	)
	if err != nil {
		return fmt.Errorf("postgres: create changelog entry: %w", err)
	}

	_, err = s.exec(ctx,
		`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
		 VALUES ($1, $2, 'HAS_CHANGE', $3)`,
		slug, entry.ID, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: create HAS_CHANGE edge: %w", err)
	}

	storage.StashChangeEvent(ctx, &storage.ChangeEvent{
		Slug:        slug,
		Version:     entry.Version,
		Stage:       entry.Stage,
		ContentHash: entry.ContentHash,
		Checkpoint:  entry.Checkpoint,
		Summary:     entry.Summary,
		Reason:      entry.Reason,
	})
	return nil
}

// ListChanges returns changelog entries for a spec, ordered by version ascending.
// Returns ErrSpecNotFound if the slug does not exist in either specs or decisions.
// Returns an empty slice (not an error) if the spec has no changelog entries.
func (s *Store) ListChanges(ctx context.Context, slug string, opts storage.ChangeLogFilter) ([]*storage.ChangeLogEntry, error) {
	// Verify the slug exists in specs or decisions.
	var exists int
	err := s.queryRow(ctx,
		`SELECT 1 FROM specs WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	).Scan(&exists)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: list changes: check spec: %w", err)
		}
		// Not found in specs — check decisions.
		err2 := s.queryRow(ctx,
			`SELECT 1 FROM decisions WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&exists)
		if err2 != nil {
			if errors.Is(err2, pgx.ErrNoRows) {
				return nil, fmt.Errorf("postgres: list changes %q: %w", slug, storage.ErrSpecNotFound)
			}
			return nil, fmt.Errorf("postgres: list changes: check decision: %w", err2)
		}
	}

	// Build query with optional filters.
	var whereClauses []string
	args := []any{slug, s.project}

	whereClauses = append(whereClauses, "spec_slug = $1 AND project_slug = $2")

	if opts.CheckpointsOnly {
		whereClauses = append(whereClauses, "checkpoint = true")
	}
	if opts.SinceVersion > 0 {
		args = append(args, opts.SinceVersion)
		whereClauses = append(whereClauses, fmt.Sprintf("version > $%d", len(args)))
	}

	q := `SELECT id, version, stage, content_hash, checkpoint, summary, reason, changes, date
	      FROM changelog_entries
	      WHERE ` + strings.Join(whereClauses, " AND ") + `
	      ORDER BY version ASC`

	if opts.Limit > 0 {
		args = append(args, opts.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: list changes: %w", err)
	}
	defer rows.Close()

	entries := make([]*storage.ChangeLogEntry, 0)
	for rows.Next() {
		entry, scanErr := scanChangeLogEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list changes: iterate: %w", err)
	}
	return entries, nil
}

// ListAllChanges returns all changelog entries across all specs in the project,
// with SpecSlug populated. Ordered by spec_slug, version ascending.
func (s *Store) ListAllChanges(ctx context.Context) ([]*storage.ChangeLogEntry, error) {
	rows, err := s.query(ctx,
		`SELECT id, spec_slug, version, stage, content_hash, checkpoint, summary, reason, changes, date
		 FROM changelog_entries
		 WHERE project_slug = $1
		 ORDER BY spec_slug, version ASC`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list all changes: %w", err)
	}
	defer rows.Close()

	entries := make([]*storage.ChangeLogEntry, 0)
	for rows.Next() {
		var (
			id          string
			specSlug    string
			version     int32
			stage       string
			contentHash string
			checkpoint  bool
			summary     string
			reason      string
			changesJSON []byte
			date        time.Time
		)
		if scanErr := rows.Scan(&id, &specSlug, &version, &stage, &contentHash,
			&checkpoint, &summary, &reason, &changesJSON, &date); scanErr != nil {
			return nil, fmt.Errorf("postgres: list all changes: scan: %w", scanErr)
		}
		changes, err := unmarshalFieldChanges(changesJSON)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &storage.ChangeLogEntry{
			ID:          id,
			SpecSlug:    specSlug,
			Version:     version,
			Stage:       stage,
			ContentHash: contentHash,
			Checkpoint:  checkpoint,
			Summary:     summary,
			Reason:      reason,
			Changes:     changes,
			Date:        date,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list all changes: iterate: %w", err)
	}
	return entries, nil
}

// scanChangeLogEntry scans a single changelog row (without spec_slug).
// Expected column order: id, version, stage, content_hash, checkpoint, summary, reason, changes, date.
func scanChangeLogEntry(rows pgx.Rows) (*storage.ChangeLogEntry, error) {
	var (
		id          string
		version     int32
		stage       string
		contentHash string
		checkpoint  bool
		summary     string
		reason      string
		changesJSON []byte
		date        time.Time
	)
	if err := rows.Scan(&id, &version, &stage, &contentHash,
		&checkpoint, &summary, &reason, &changesJSON, &date); err != nil {
		return nil, fmt.Errorf("postgres: scan changelog entry: %w", err)
	}
	changes, err := unmarshalFieldChanges(changesJSON)
	if err != nil {
		return nil, err
	}
	return &storage.ChangeLogEntry{
		ID:          id,
		Version:     version,
		Stage:       stage,
		ContentHash: contentHash,
		Checkpoint:  checkpoint,
		Summary:     summary,
		Reason:      reason,
		Changes:     changes,
		Date:        date,
	}, nil
}
