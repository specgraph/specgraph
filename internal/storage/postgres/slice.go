// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.SliceBackend = (*Store)(nil)

// CreateSlice persists a new slice row with BELONGS_TO and COMPOSES edges.
func (s *Store) CreateSlice(ctx context.Context, sl *storage.Slice) error {
	if sl == nil {
		return fmt.Errorf("postgres: CreateSlice: slice must not be nil")
	}

	now := s.now()

	// Coerce nil slices to empty arrays: columns are NOT NULL TEXT[].
	verify := sl.Verify
	if verify == nil {
		verify = []string{}
	}
	touches := sl.Touches
	if touches == nil {
		touches = []string{}
	}
	dependsOn := sl.DependsOn
	if dependsOn == nil {
		dependsOn = []string{}
	}

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		_, txErr := s.exec(txCtx,
			`INSERT INTO slices
				(slug, project_slug, parent_slug, slice_id, intent, status,
				 assigned_to, verify, touches, depends_on, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, '', $7, $8, $9, $10, $10)`,
			sl.Slug, s.project, sl.ParentSlug, sl.SliceID, sl.Intent,
			string(storage.SliceStatusOpen),
			verify, touches, dependsOn, now,
		)
		if txErr != nil {
			return fmt.Errorf("postgres: create slice %q: %w", sl.Slug, txErr)
		}

		// BELONGS_TO edge: slice -> project.
		_, txErr = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'BELONGS_TO', $3) ON CONFLICT DO NOTHING`,
			sl.Slug, s.project, s.project,
		)
		if txErr != nil {
			return fmt.Errorf("postgres: create BELONGS_TO edge for slice: %w", txErr)
		}

		// COMPOSES edge: slice -> parent spec.
		_, txErr = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'COMPOSES', $3) ON CONFLICT DO NOTHING`,
			sl.Slug, sl.ParentSlug, s.project,
		)
		if txErr != nil {
			return fmt.Errorf("postgres: create COMPOSES edge for slice: %w", txErr)
		}

		return nil
	})
}

// UpdateSlice overwrites the mutable body of an existing slice — Intent,
// Verify, Touches, and DependsOn — leaving status/assigned_to and identity
// columns untouched. It is used by StoreDecomposeOutput to reconcile a
// re-authored decomposition (amend → re-decompose) against pre-existing
// Slice nodes. Returns ErrSliceNotFound if no slice row matches.
func (s *Store) UpdateSlice(ctx context.Context, sl *storage.Slice) error {
	if sl == nil {
		return fmt.Errorf("postgres: UpdateSlice: slice must not be nil")
	}

	// Coerce nil slices to empty arrays: columns are NOT NULL TEXT[].
	verify := sl.Verify
	if verify == nil {
		verify = []string{}
	}
	touches := sl.Touches
	if touches == nil {
		touches = []string{}
	}
	dependsOn := sl.DependsOn
	if dependsOn == nil {
		dependsOn = []string{}
	}

	now := s.now()
	tag, err := s.exec(ctx,
		`UPDATE slices
		 SET intent = $1, verify = $2, touches = $3, depends_on = $4, updated_at = $5
		 WHERE slug = $6 AND project_slug = $7`,
		sl.Intent, verify, touches, dependsOn, now, sl.Slug, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: update slice %q: %w", sl.Slug, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: update slice %q: %w", sl.Slug, storage.ErrSliceNotFound)
	}
	return nil
}

// DeleteSlice removes a slice node and every edge incident to it (BELONGS_TO,
// COMPOSES, and DEPENDS_ON in either direction). It is used by
// StoreDecomposeOutput to prune slices that a re-authored decomposition no
// longer includes. Returns ErrSliceNotFound if no slice row matches.
func (s *Store) DeleteSlice(ctx context.Context, slug string) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Remove all edges incident to the slice first (both directions) so no
		// dangling BELONGS_TO/COMPOSES/DEPENDS_ON edges outlive the node.
		if _, err := s.exec(txCtx,
			`DELETE FROM edges
			 WHERE project_slug = $1 AND (from_slug = $2 OR to_slug = $2)`,
			s.project, slug,
		); err != nil {
			return fmt.Errorf("postgres: delete slice edges %q: %w", slug, err)
		}
		tag, err := s.exec(txCtx,
			`DELETE FROM slices WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: delete slice %q: %w", slug, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgres: delete slice %q: %w", slug, storage.ErrSliceNotFound)
		}
		return nil
	})
}

// ListSlices returns all slices for a parent spec, ordered by creation time.
func (s *Store) ListSlices(ctx context.Context, parentSlug string) ([]*storage.Slice, error) {
	rows, err := s.query(ctx,
		`SELECT slug, parent_slug, slice_id, intent, verify, touches, depends_on,
		        status, assigned_to, created_at, updated_at
		 FROM slices
		 WHERE parent_slug = $1 AND project_slug = $2
		 ORDER BY created_at`,
		parentSlug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list slices for %q: %w", parentSlug, err)
	}
	defer rows.Close()

	slices := make([]*storage.Slice, 0)
	for rows.Next() {
		sl, scanErr := scanSlice(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: list slices: %w", scanErr)
		}
		slices = append(slices, sl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list slices: rows: %w", err)
	}
	return slices, nil
}

// GetSlice returns a single slice by its full slug.
func (s *Store) GetSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	rows, err := s.query(ctx,
		`SELECT slug, parent_slug, slice_id, intent, verify, touches, depends_on,
		        status, assigned_to, created_at, updated_at
		 FROM slices
		 WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get slice %q: %w", slug, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			return nil, fmt.Errorf("postgres: get slice %q: %w", slug, rowsErr)
		}
		return nil, storage.ErrSliceNotFound
	}
	sl, err := scanSlice(rows)
	if err != nil {
		return nil, fmt.Errorf("postgres: get slice %q: %w", slug, err)
	}
	return sl, nil
}

// ClaimSlice transitions a slice from open to claimed status.
func (s *Store) ClaimSlice(ctx context.Context, slug, assignee string) (*storage.Slice, error) {
	now := s.now()
	tag, err := s.exec(ctx,
		`UPDATE slices
		 SET status = $1, assigned_to = $2, updated_at = $3
		 WHERE slug = $4 AND project_slug = $5 AND status = $6`,
		string(storage.SliceStatusClaimed), assignee, now,
		slug, s.project, string(storage.SliceStatusOpen),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: claim slice %q: %w", slug, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, s.sliceNotFoundOrWrongStatus(ctx, slug)
	}
	return s.GetSlice(ctx, slug)
}

// CompleteSlice transitions a slice from claimed to done status.
func (s *Store) CompleteSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	now := s.now()
	tag, err := s.exec(ctx,
		`UPDATE slices
		 SET status = $1, updated_at = $2
		 WHERE slug = $3 AND project_slug = $4 AND status = $5`,
		string(storage.SliceStatusDone), now,
		slug, s.project, string(storage.SliceStatusClaimed),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: complete slice %q: %w", slug, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, s.sliceNotFoundOrWrongStatus(ctx, slug)
	}
	return s.GetSlice(ctx, slug)
}

// sliceNotFoundOrWrongStatus distinguishes ErrSliceNotFound from ErrSliceWrongStatus
// when a conditional UPDATE returns 0 rows.
func (s *Store) sliceNotFoundOrWrongStatus(ctx context.Context, slug string) error {
	_, err := s.GetSlice(ctx, slug)
	if err == nil {
		return fmt.Errorf("postgres: slice %q: %w", slug, storage.ErrSliceWrongStatus)
	}
	if errors.Is(err, storage.ErrSliceNotFound) {
		return fmt.Errorf("postgres: slice %q: %w", slug, storage.ErrSliceNotFound)
	}
	return fmt.Errorf("postgres: slice %q status check: %w", slug, err)
}

// scanSlice reads a Slice from pgx.Rows (column order must match SELECT list).
func scanSlice(rows pgx.Rows) (*storage.Slice, error) {
	var (
		slug       string
		parentSlug string
		sliceID    string
		intent     string
		verify     []string
		touches    []string
		dependsOn  []string
		status     string
		assignedTo string
		createdAt  time.Time
		updatedAt  time.Time
	)
	if err := rows.Scan(
		&slug, &parentSlug, &sliceID, &intent,
		&verify, &touches, &dependsOn,
		&status, &assignedTo, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("postgres: scan slice: %w", err)
	}
	if verify == nil {
		verify = []string{}
	}
	if touches == nil {
		touches = []string{}
	}
	if dependsOn == nil {
		dependsOn = []string{}
	}
	return &storage.Slice{
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
