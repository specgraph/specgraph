// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.PublishBackend = (*Store)(nil)

// UpsertPageMapping inserts or updates a page mapping record.
func (s *Store) UpsertPageMapping(ctx context.Context, m *storage.PageMapping) (*storage.PageMapping, error) {
	now := time.Now()
	err := s.queryRow(ctx, `
		INSERT INTO page_mappings (spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (spec_slug, doc_kind, decision_slug)
		DO UPDATE SET page_id = $4, page_version = $5, spec_version = $6, state = $7, error_message = $8, last_sync = $9
		RETURNING last_sync, created_at`,
		m.SpecSlug, m.DocKind, m.DecisionSlug, m.PageID, m.PageVersion, m.SpecVersion, m.State, m.ErrorMessage, now, now,
	).Scan(&m.LastSync, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert page mapping: %w", err)
	}
	return m, nil
}

// GetPageMapping retrieves a page mapping by composite key. Returns nil, nil if not found.
func (s *Store) GetPageMapping(ctx context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error) {
	var m storage.PageMapping
	err := s.queryRow(ctx, `
		SELECT spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at
		FROM page_mappings
		WHERE spec_slug = $1 AND doc_kind = $2 AND decision_slug = $3`,
		specSlug, kind, decisionSlug,
	).Scan(&m.SpecSlug, &m.DocKind, &m.DecisionSlug, &m.PageID, &m.PageVersion, &m.SpecVersion, &m.State, &m.ErrorMessage, &m.LastSync, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page mapping: %w", err)
	}
	return &m, nil
}

// ListPageMappings returns all page mappings, optionally filtered by specSlug.
func (s *Store) ListPageMappings(ctx context.Context, specSlug string) ([]*storage.PageMapping, error) {
	query := `SELECT spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at FROM page_mappings`
	var args []any
	if specSlug != "" {
		query += ` WHERE spec_slug = $1`
		args = append(args, specSlug)
	}
	query += ` ORDER BY spec_slug, doc_kind, decision_slug`
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list page mappings: %w", err)
	}
	defer rows.Close()
	var mappings []*storage.PageMapping
	for rows.Next() {
		var m storage.PageMapping
		if err := rows.Scan(&m.SpecSlug, &m.DocKind, &m.DecisionSlug, &m.PageID, &m.PageVersion, &m.SpecVersion, &m.State, &m.ErrorMessage, &m.LastSync, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan page mapping: %w", err)
		}
		mappings = append(mappings, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list page mappings rows: %w", err)
	}
	return mappings, nil
}

// DeletePageMappings removes all page mappings for a spec slug. Returns count deleted.
func (s *Store) DeletePageMappings(ctx context.Context, specSlug string) (int, error) {
	tag, err := s.exec(ctx, `DELETE FROM page_mappings WHERE spec_slug = $1`, specSlug)
	if err != nil {
		return 0, fmt.Errorf("delete page mappings: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// StoreFeedback inserts a feedback entry, ignoring duplicates by external_id.
func (s *Store) StoreFeedback(ctx context.Context, entry *storage.FeedbackEntry) (*storage.FeedbackEntry, error) {
	if entry.ID == "" {
		entry.ID = "fb-" + ulid.Make().String()
	}
	now := time.Now()
	tag, err := s.exec(ctx, `
		INSERT INTO feedback_entries (id, external_id, spec_slug, author, body, timestamp, kind, stage, is_question, parent_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (external_id) DO NOTHING`,
		entry.ID, entry.ExternalID, entry.SpecSlug, entry.Author, entry.Body, entry.Timestamp, entry.Kind, entry.Stage, entry.IsQuestion, entry.ParentID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("store feedback: %w", err)
	}
	if tag.RowsAffected() > 0 {
		entry.CreatedAt = now
	}
	return entry, nil
}

// ListFeedback returns feedback entries for a spec, optionally filtering to entries
// created after the entry with the given external ID.
func (s *Store) ListFeedback(ctx context.Context, specSlug, sinceExternalID string) ([]*storage.FeedbackEntry, error) {
	query := `SELECT id, external_id, spec_slug, author, body, timestamp, kind, stage, is_question, parent_id, created_at
		FROM feedback_entries WHERE spec_slug = $1`
	args := []any{specSlug}
	if sinceExternalID != "" {
		query += ` AND created_at > COALESCE((SELECT created_at FROM feedback_entries WHERE external_id = $2), '1970-01-01'::timestamptz)`
		args = append(args, sinceExternalID)
	}
	query += ` ORDER BY timestamp`
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	defer rows.Close()
	var entries []*storage.FeedbackEntry
	for rows.Next() {
		var e storage.FeedbackEntry
		if err := rows.Scan(&e.ID, &e.ExternalID, &e.SpecSlug, &e.Author, &e.Body, &e.Timestamp, &e.Kind, &e.Stage, &e.IsQuestion, &e.ParentID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan feedback: %w", err)
		}
		entries = append(entries, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list feedback rows: %w", err)
	}
	return entries, nil
}

// CountNewFeedback returns the total number of feedback entries for a spec.
func (s *Store) CountNewFeedback(ctx context.Context, specSlug string) (int, error) {
	var count int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM feedback_entries WHERE spec_slug = $1`, specSlug).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count feedback: %w", err)
	}
	return count, nil
}
