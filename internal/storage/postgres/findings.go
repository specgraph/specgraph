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
var _ storage.FindingsBackend = (*Store)(nil)

// StoreFindings atomically replaces all findings for (slug, passType) and
// inserts HAS_FINDING edges for each new finding. Returns the generated IDs.
func (s *Store) StoreFindings(ctx context.Context, slug string, passType storage.PassType, findings []storage.AnalyticalFindingInput) ([]string, error) {
	var ids []string
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify spec exists and capture its version.
		spec, txErr := s.GetSpec(txCtx, slug)
		if txErr != nil {
			return txErr
		}

		// Delete existing findings for (slug, passType) and collect their IDs.
		deletedRows, txErr := s.query(txCtx,
			`DELETE FROM findings WHERE spec_slug = $1 AND pass_type = $2 AND project_slug = $3
			 RETURNING id`,
			slug, string(passType), s.project,
		)
		if txErr != nil {
			return fmt.Errorf("postgres: delete existing findings: %w", txErr)
		}
		var deletedIDs []string
		for deletedRows.Next() {
			var deletedID string
			if scanErr := deletedRows.Scan(&deletedID); scanErr != nil {
				deletedRows.Close()
				return fmt.Errorf("postgres: delete existing findings: scan: %w", scanErr)
			}
			deletedIDs = append(deletedIDs, deletedID)
		}
		deletedRows.Close()
		if rowsErr := deletedRows.Err(); rowsErr != nil {
			return fmt.Errorf("postgres: delete existing findings: rows: %w", rowsErr)
		}

		// Delete only the HAS_FINDING edges for the deleted findings.
		if len(deletedIDs) > 0 {
			_, txErr = s.exec(txCtx,
				`DELETE FROM edges WHERE from_slug = $1 AND edge_type = 'HAS_FINDING' AND project_slug = $2
				 AND to_slug = ANY($3)`,
				slug, s.project, deletedIDs,
			)
			if txErr != nil {
				return fmt.Errorf("postgres: delete HAS_FINDING edges: %w", txErr)
			}
		}

		now := s.now()
		ids = make([]string, 0, len(findings))
		for i := range findings {
			f := &findings[i]
			id := newID("finding")

			_, txErr = s.exec(txCtx,
				`INSERT INTO findings
					(id, spec_slug, project_slug, pass_type, severity, summary, detail,
					 constraint_, resolution, version, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				id, slug, s.project, string(passType),
				string(f.Severity), f.Summary, f.Detail,
				f.Constraint, f.Resolution,
				spec.Version, now,
			)
			if txErr != nil {
				return fmt.Errorf("postgres: insert finding: %w", txErr)
			}

			// Insert HAS_FINDING edge: spec -> finding.
			_, txErr = s.exec(txCtx,
				`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
				 VALUES ($1, $2, 'HAS_FINDING', $3)
				 ON CONFLICT DO NOTHING`,
				slug, id, s.project,
			)
			if txErr != nil {
				return fmt.Errorf("postgres: insert HAS_FINDING edge: %w", txErr)
			}

			ids = append(ids, id)
		}
		return nil
	})
	return ids, err
}

// ListFindings returns findings for a spec, optionally filtered by passType.
// Returns ErrSpecNotFound if the spec does not exist.
func (s *Store) ListFindings(ctx context.Context, slug string, passType storage.PassType) ([]storage.AnalyticalFinding, error) {
	// Verify spec exists (check specs, then decisions).
	if _, err := s.GetSpec(ctx, slug); err != nil {
		if !errors.Is(err, storage.ErrSpecNotFound) {
			return nil, err
		}
		// Check decisions as fallback.
		var exists int
		checkErr := s.queryRow(ctx,
			`SELECT 1 FROM decisions WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&exists)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: spec %q: %w", slug, storage.ErrSpecNotFound)
		}
		if checkErr != nil {
			return nil, fmt.Errorf("postgres: check decision existence: %w", checkErr)
		}
	}

	var (
		rows pgx.Rows
		err  error
	)
	if passType != "" {
		rows, err = s.query(ctx,
			`SELECT id, pass_type, severity, summary, detail, constraint_, resolution, version, created_at
			 FROM findings
			 WHERE spec_slug = $1 AND project_slug = $2 AND pass_type = $3
			 ORDER BY created_at`,
			slug, s.project, string(passType),
		)
	} else {
		rows, err = s.query(ctx,
			`SELECT id, pass_type, severity, summary, detail, constraint_, resolution, version, created_at
			 FROM findings
			 WHERE spec_slug = $1 AND project_slug = $2
			 ORDER BY created_at`,
			slug, s.project,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: list findings: %w", err)
	}
	defer rows.Close()

	findings := make([]storage.AnalyticalFinding, 0)
	for rows.Next() {
		var (
			id          string
			pt          string
			severity    string
			summary     string
			detail      string
			constraintV string
			resolution  string
			version     int32
			createdAt   time.Time
		)
		if scanErr := rows.Scan(&id, &pt, &severity, &summary, &detail, &constraintV, &resolution, &version, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("postgres: list findings: scan: %w", scanErr)
		}
		findings = append(findings, storage.AnalyticalFinding{
			ID:         id,
			PassType:   storage.PassType(pt),
			Severity:   storage.FindingSeverity(severity),
			Summary:    summary,
			Detail:     detail,
			Constraint: constraintV,
			Resolution: resolution,
			Version:    version,
			CreatedAt:  createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list findings: rows: %w", err)
	}
	return findings, nil
}

// ListAllFindings returns all findings across all specs in the project,
// with SpecSlug populated.
func (s *Store) ListAllFindings(ctx context.Context) ([]*storage.AnalyticalFinding, error) {
	rows, err := s.query(ctx,
		`SELECT id, spec_slug, pass_type, severity, summary, detail, constraint_, resolution, version, created_at
		 FROM findings
		 WHERE project_slug = $1
		 ORDER BY spec_slug, created_at`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list all findings: %w", err)
	}
	defer rows.Close()

	findings := make([]*storage.AnalyticalFinding, 0)
	for rows.Next() {
		var (
			id          string
			specSlug    string
			pt          string
			severity    string
			summary     string
			detail      string
			constraintV string
			resolution  string
			version     int32
			createdAt   time.Time
		)
		if scanErr := rows.Scan(&id, &specSlug, &pt, &severity, &summary, &detail, &constraintV, &resolution, &version, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("postgres: list all findings: scan: %w", scanErr)
		}
		findings = append(findings, &storage.AnalyticalFinding{
			ID:         id,
			SpecSlug:   specSlug,
			PassType:   storage.PassType(pt),
			Severity:   storage.FindingSeverity(severity),
			Summary:    summary,
			Detail:     detail,
			Constraint: constraintV,
			Resolution: resolution,
			Version:    version,
			CreatedAt:  createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list all findings: rows: %w", err)
	}
	return findings, nil
}
