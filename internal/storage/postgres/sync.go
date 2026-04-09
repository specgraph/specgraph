// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.SyncBackend = (*Store)(nil)

// isUniqueViolation reports whether the pgconn error is a unique-constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateSyncMapping stores a new sync mapping between a spec and an external reference.
// Returns ErrSpecNotFound if the spec does not exist, ErrSyncMappingExists on conflict.
func (s *Store) CreateSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, externalID string) (*storage.SyncMapping, error) {
	// Verify spec exists.
	spec, err := s.GetSpec(ctx, specSlug)
	if err != nil {
		return nil, err
	}

	now := s.now()
	_, err = s.exec(ctx,
		`INSERT INTO sync_mappings
			(spec_slug, project_slug, adapter, external_id, state, error_message, last_sync, created_at)
		 VALUES ($1, $2, $3, $4, $5, '', $6, $6)`,
		specSlug, s.project, string(adapter), externalID,
		string(storage.SyncStateSynced), now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("postgres: create sync mapping %q/%s: %w", specSlug, adapter, storage.ErrSyncMappingExists)
		}
		return nil, fmt.Errorf("postgres: create sync mapping: %w", err)
	}

	return &storage.SyncMapping{
		SpecID:       spec.ID,
		SpecSlug:     specSlug,
		Adapter:      adapter,
		ExternalID:   externalID,
		State:        storage.SyncStateSynced,
		ErrorMessage: "",
		LastSync:     now,
		CreatedAt:    now,
	}, nil
}

// UpdateSyncState updates the sync state and last_sync timestamp for an existing mapping.
// Returns ErrSyncMappingNotFound if no mapping exists.
func (s *Store) UpdateSyncState(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, state storage.SyncStateType, errorMessage string) (*storage.SyncMapping, error) {
	now := s.now()
	tag, err := s.exec(ctx,
		`UPDATE sync_mappings
		 SET state = $1, error_message = $2, last_sync = $3
		 WHERE spec_slug = $4 AND project_slug = $5 AND adapter = $6`,
		string(state), errorMessage, now,
		specSlug, s.project, string(adapter),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: update sync state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("postgres: update sync state %q/%s: %w", specSlug, adapter, storage.ErrSyncMappingNotFound)
	}
	return s.GetSyncMapping(ctx, specSlug, adapter)
}

// GetSyncMapping retrieves a sync mapping by spec slug and adapter.
// Returns ErrSyncMappingNotFound if no mapping exists.
func (s *Store) GetSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType) (*storage.SyncMapping, error) {
	row := s.queryRow(ctx,
		`SELECT sm.spec_slug, sm.adapter, sm.external_id, sm.state,
		        sm.error_message, sm.last_sync, sm.created_at
		 FROM sync_mappings sm
		 WHERE sm.spec_slug = $1 AND sm.project_slug = $2 AND sm.adapter = $3`,
		specSlug, s.project, string(adapter),
	)

	var (
		slug         string
		adapterStr   string
		externalID   string
		state        string
		errorMessage string
		lastSync     time.Time
		createdAt    time.Time
	)
	err := row.Scan(&slug, &adapterStr, &externalID, &state, &errorMessage, &lastSync, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: get sync mapping %q/%s: %w", specSlug, adapter, storage.ErrSyncMappingNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get sync mapping: %w", err)
	}

	return &storage.SyncMapping{
		SpecSlug:     slug,
		Adapter:      storage.SyncAdapterType(adapterStr),
		ExternalID:   externalID,
		State:        storage.SyncStateType(state),
		ErrorMessage: errorMessage,
		LastSync:     lastSync,
		CreatedAt:    createdAt,
	}, nil
}

// ListSyncMappings returns all sync mappings, optionally filtered by adapter or spec slug.
func (s *Store) ListSyncMappings(ctx context.Context, adapter storage.SyncAdapterType, specSlug string) ([]*storage.SyncMapping, error) {
	var conditions []string
	args := []any{s.project}
	idx := 2

	if specSlug != "" {
		conditions = append(conditions, fmt.Sprintf("sm.spec_slug = $%d", idx))
		args = append(args, specSlug)
		idx++
	}
	if adapter != "" {
		conditions = append(conditions, fmt.Sprintf("sm.adapter = $%d", idx))
		args = append(args, string(adapter))
	}

	where := "sm.project_slug = $1"
	if len(conditions) > 0 {
		where += " AND " + strings.Join(conditions, " AND ")
	}

	rows, err := s.query(ctx,
		fmt.Sprintf(
			`SELECT sm.spec_slug, sm.adapter, sm.external_id, sm.state,
			        sm.error_message, sm.last_sync, sm.created_at
			 FROM sync_mappings sm
			 WHERE %s
			 ORDER BY sm.last_sync DESC`,
			where,
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list sync mappings: %w", err)
	}
	defer rows.Close()

	mappings := make([]*storage.SyncMapping, 0)
	for rows.Next() {
		var (
			slug         string
			adapterStr   string
			externalID   string
			state        string
			errorMessage string
			lastSync     time.Time
			createdAt    time.Time
		)
		if scanErr := rows.Scan(&slug, &adapterStr, &externalID, &state, &errorMessage, &lastSync, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("postgres: list sync mappings: scan: %w", scanErr)
		}
		mappings = append(mappings, &storage.SyncMapping{
			SpecSlug:     slug,
			Adapter:      storage.SyncAdapterType(adapterStr),
			ExternalID:   externalID,
			State:        storage.SyncStateType(state),
			ErrorMessage: errorMessage,
			LastSync:     lastSync,
			CreatedAt:    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list sync mappings: rows: %w", err)
	}
	return mappings, nil
}

// DeleteSyncMapping removes a sync mapping. Idempotent: no error if not found.
func (s *Store) DeleteSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType) error {
	_, err := s.exec(ctx,
		`DELETE FROM sync_mappings WHERE spec_slug = $1 AND project_slug = $2 AND adapter = $3`,
		specSlug, s.project, string(adapter),
	)
	if err != nil {
		return fmt.Errorf("postgres: delete sync mapping: %w", err)
	}
	return nil
}
