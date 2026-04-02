// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.ConversationBackend = (*Store)(nil)

// exchangeJSON is the JSON-serializable representation of a conversation exchange.
type exchangeJSON struct {
	Role          string `json:"role"`
	Content       string `json:"content"`
	Stage         string `json:"stage"`
	Sequence      int32  `json:"sequence"`
	DecisionPoint bool   `json:"decision_point,omitempty"`
}

// marshalExchanges serializes exchanges to JSON bytes for storage in the exchanges JSONB column.
func marshalExchanges(exchanges []storage.ConversationExchange) ([]byte, error) {
	if len(exchanges) == 0 {
		return []byte("[]"), nil
	}
	items := make([]exchangeJSON, len(exchanges))
	for i, e := range exchanges {
		items[i] = exchangeJSON{
			Role:          string(e.Role),
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("postgres: marshal exchanges: %w", err)
	}
	return b, nil
}

// unmarshalExchanges deserializes JSONB bytes into a slice of ConversationExchange.
func unmarshalExchanges(data []byte) ([]storage.ConversationExchange, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var items []exchangeJSON
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("postgres: unmarshal exchanges: %w", err)
	}
	result := make([]storage.ConversationExchange, len(items))
	for i, item := range items {
		result[i] = storage.ConversationExchange{
			Role:          storage.ConversationRole(item.Role),
			Content:       item.Content,
			Stage:         item.Stage,
			Sequence:      item.Sequence,
			DecisionPoint: item.DecisionPoint,
		}
	}
	return result, nil
}

// RecordConversation stores a conversation log entry for a spec stage.
// Links to the most recent ChangeLog via an EXPLAINS edge (if one exists).
// Extends the CONTINUES chain from the previous conversation log (if one exists).
// Returns ErrSpecNotFound if the spec slug does not exist.
func (s *Store) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) { //nolint:gocritic // hugeParam: entry is a value type by interface contract
	var result *storage.ConversationLogEntry

	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// 1. Verify spec exists and get current version.
		var version int32
		err := s.queryRow(txCtx,
			`SELECT version FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&version)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("postgres: record conversation %q: %w", slug, storage.ErrSpecNotFound)
			}
			return fmt.Errorf("postgres: record conversation: verify spec: %w", err)
		}

		// 2. Find the most recent changelog entry for this stage+version (for EXPLAINS edge).
		var changeLogID string
		clErr := s.queryRow(txCtx,
			`SELECT id FROM changelog_entries
			 WHERE spec_slug = $1 AND project_slug = $2 AND stage = $3 AND version = $4
			 ORDER BY date DESC
			 LIMIT 1`,
			slug, s.project, string(entry.Stage), version,
		).Scan(&changeLogID)
		if clErr != nil && !errors.Is(clErr, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: record conversation: find changelog: %w", clErr)
		}

		// 3. Find the most recent conversation log for this spec (tail of CONTINUES chain).
		var prevID string
		prevErr := s.queryRow(txCtx,
			`SELECT id FROM conversation_logs
			 WHERE spec_slug = $1 AND project_slug = $2
			 ORDER BY date DESC
			 LIMIT 1`,
			slug, s.project,
		).Scan(&prevID)
		if prevErr != nil && !errors.Is(prevErr, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: record conversation: find prev: %w", prevErr)
		}

		// 4. Insert the conversation_logs row.
		id := newID("conv")
		now := s.now()
		exchangesJSON, mErr := marshalExchanges(entry.Exchanges)
		if mErr != nil {
			return mErr
		}

		_, insertErr := s.exec(txCtx,
			`INSERT INTO conversation_logs
				(id, spec_slug, project_slug, stage, version, is_amend, exchanges, exchange_count, date)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			id, slug, s.project,
			string(entry.Stage), version, entry.IsAmend,
			exchangesJSON, entry.ExchangeCount, now,
		)
		if insertErr != nil {
			return fmt.Errorf("postgres: record conversation: insert: %w", insertErr)
		}

		// 5a. AUTHORED_VIA edge — only if this is the first conversation log for this spec.
		if prevID == "" {
			_, edgeErr := s.exec(txCtx,
				`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
				 VALUES ($1, $2, 'AUTHORED_VIA', $3)`,
				slug, id, s.project,
			)
			if edgeErr != nil {
				return fmt.Errorf("postgres: record conversation: create AUTHORED_VIA: %w", edgeErr)
			}
		}

		// 5b. CONTINUES edge — from previous log to this one.
		if prevID != "" {
			_, contErr := s.exec(txCtx,
				`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
				 VALUES ($1, $2, 'CONTINUES', $3)`,
				prevID, id, s.project,
			)
			if contErr != nil {
				return fmt.Errorf("postgres: record conversation: create CONTINUES: %w", contErr)
			}
		}

		// 5c. EXPLAINS edge — from this conversation log to the matching changelog entry.
		if changeLogID != "" {
			_, explErr := s.exec(txCtx,
				`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
				 VALUES ($1, $2, 'EXPLAINS', $3)`,
				id, changeLogID, s.project,
			)
			if explErr != nil {
				return fmt.Errorf("postgres: record conversation: create EXPLAINS: %w", explErr)
			}
		}

		result = &storage.ConversationLogEntry{
			ID:            id,
			SpecSlug:      slug,
			Stage:         entry.Stage,
			Version:       version,
			IsAmend:       entry.IsAmend,
			Exchanges:     entry.Exchanges,
			ExchangeCount: entry.ExchangeCount,
			Date:          now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListConversations returns conversation logs for a spec in date order (chain order).
// If stage is non-empty, filters to that stage only.
// Returns an empty slice (not an error) if no conversation logs exist.
// Returns ErrSpecNotFound if the spec slug does not exist.
func (s *Store) ListConversations(ctx context.Context, slug, stage string) ([]*storage.ConversationLogEntry, error) {
	// Verify spec exists.
	var exists int
	err := s.queryRow(ctx,
		`SELECT 1 FROM specs WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: list conversations %q: %w", slug, storage.ErrSpecNotFound)
		}
		return nil, fmt.Errorf("postgres: list conversations: check spec: %w", err)
	}

	var (
		rows     pgx.Rows
		queryErr error
	)
	if stage != "" {
		rows, queryErr = s.query(ctx,
			`SELECT id, stage, version, is_amend, exchanges, exchange_count, date
			 FROM conversation_logs
			 WHERE spec_slug = $1 AND project_slug = $2 AND stage = $3
			 ORDER BY date ASC`,
			slug, s.project, stage,
		)
	} else {
		rows, queryErr = s.query(ctx,
			`SELECT id, stage, version, is_amend, exchanges, exchange_count, date
			 FROM conversation_logs
			 WHERE spec_slug = $1 AND project_slug = $2
			 ORDER BY date ASC`,
			slug, s.project,
		)
	}
	if queryErr != nil {
		return nil, fmt.Errorf("postgres: list conversations: %w", queryErr)
	}
	defer rows.Close()

	entries := make([]*storage.ConversationLogEntry, 0)
	for rows.Next() {
		entry, scanErr := scanConversationLogEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entry.SpecSlug = slug
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list conversations: iterate: %w", err)
	}
	return entries, nil
}

// ListAllConversations returns all conversation logs across all specs in the project,
// with SpecSlug populated. Ordered by spec_slug, date ascending.
func (s *Store) ListAllConversations(ctx context.Context) ([]*storage.ConversationLogEntry, error) {
	rows, err := s.query(ctx,
		`SELECT id, spec_slug, stage, version, is_amend, exchanges, exchange_count, date
		 FROM conversation_logs
		 WHERE project_slug = $1
		 ORDER BY spec_slug, date ASC`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list all conversations: %w", err)
	}
	defer rows.Close()

	entries := make([]*storage.ConversationLogEntry, 0)
	for rows.Next() {
		var (
			id            string
			specSlug      string
			stageStr      string
			version       int32
			isAmend       bool
			exchangesJSON []byte
			exchangeCount int32
			date          time.Time
		)
		if scanErr := rows.Scan(&id, &specSlug, &stageStr, &version, &isAmend,
			&exchangesJSON, &exchangeCount, &date); scanErr != nil {
			return nil, fmt.Errorf("postgres: list all conversations: scan: %w", scanErr)
		}
		exchanges, uErr := unmarshalExchanges(exchangesJSON)
		if uErr != nil {
			return nil, uErr
		}
		entries = append(entries, &storage.ConversationLogEntry{
			ID:            id,
			SpecSlug:      specSlug,
			Stage:         storage.SpecStage(stageStr),
			Version:       version,
			IsAmend:       isAmend,
			Exchanges:     exchanges,
			ExchangeCount: exchangeCount,
			Date:          date,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list all conversations: iterate: %w", err)
	}
	return entries, nil
}

// scanConversationLogEntry scans a single conversation log row (without spec_slug).
// Expected column order: id, stage, version, is_amend, exchanges, exchange_count, date.
func scanConversationLogEntry(rows pgx.Rows) (*storage.ConversationLogEntry, error) {
	var (
		id            string
		stageStr      string
		version       int32
		isAmend       bool
		exchangesJSON []byte
		exchangeCount int32
		date          time.Time
	)
	if err := rows.Scan(&id, &stageStr, &version, &isAmend,
		&exchangesJSON, &exchangeCount, &date); err != nil {
		return nil, fmt.Errorf("postgres: scan conversation log entry: %w", err)
	}
	exchanges, err := unmarshalExchanges(exchangesJSON)
	if err != nil {
		return nil, err
	}
	return &storage.ConversationLogEntry{
		ID:            id,
		Stage:         storage.SpecStage(stageStr),
		Version:       version,
		IsAmend:       isAmend,
		Exchanges:     exchanges,
		ExchangeCount: exchangeCount,
		Date:          date,
	}, nil
}
