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

// EnsureProject implements storage.ProjectBackend.
func (s *Store) EnsureProject(ctx context.Context, slug string) (*storage.Project, error) {
	_, err := s.exec(ctx,
		`INSERT INTO projects (slug) VALUES ($1) ON CONFLICT (slug) DO NOTHING`,
		slug,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: ensure project %q: insert: %w", slug, err)
	}

	return s.GetProject(ctx, slug)
}

// GetProject implements storage.ProjectBackend.
func (s *Store) GetProject(ctx context.Context, slug string) (*storage.Project, error) {
	row := s.queryRow(ctx,
		`SELECT slug, sync_adapters, github_repo, created_at, updated_at
		 FROM projects WHERE slug = $1`,
		slug,
	)
	p, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: get project %q: %w", slug, storage.ErrProjectNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get project %q: %w", slug, err)
	}
	return p, nil
}

// UpdateProject implements storage.ProjectBackend.
func (s *Store) UpdateProject(ctx context.Context, slug string, adapters []string, ghRepo string) (*storage.Project, error) {
	now := s.now()
	row := s.queryRow(ctx,
		`UPDATE projects
		 SET sync_adapters = $1, github_repo = $2, updated_at = $3
		 WHERE slug = $4
		 RETURNING slug, sync_adapters, github_repo, created_at, updated_at`,
		adapters, ghRepo, now, slug,
	)
	p, err := scanProject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: update project %q: %w", slug, storage.ErrProjectNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: update project %q: %w", slug, err)
	}
	return p, nil
}

// ListProjects implements storage.ProjectBackend.
func (s *Store) ListProjects(ctx context.Context) ([]*storage.Project, error) {
	rows, err := s.query(ctx,
		`SELECT slug, sync_adapters, github_repo, created_at, updated_at
		 FROM projects ORDER BY slug`,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list projects: %w", err)
	}
	defer rows.Close()

	var projects []*storage.Project
	for rows.Next() {
		var (
			slug         string
			syncAdapters []string
			githubRepo   string
			createdAt    time.Time
			updatedAt    time.Time
		)
		if err := rows.Scan(&slug, &syncAdapters, &githubRepo, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("postgres: list projects: scan: %w", err)
		}
		if syncAdapters == nil {
			syncAdapters = []string{}
		}
		projects = append(projects, &storage.Project{
			Slug:         slug,
			SyncAdapters: syncAdapters,
			GitHubRepo:   githubRepo,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list projects: rows: %w", err)
	}
	return projects, nil
}

// WipeProjectData deletes all data for s.project in FK-safe order.
// The project row itself is preserved.
func (s *Store) WipeProjectData(ctx context.Context) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		tables := []string{
			"edges",
			"sync_mappings",
			"execution_events",
			"claims",
			"findings",
			"conversation_logs",
			"changelog_entries",
			"constitutions",
			"slices",
			"decisions",
			"specs",
		}
		for _, tbl := range tables {
			q := "DELETE FROM " + tbl + " WHERE project_slug = $1"
			if _, err := s.exec(txCtx, q, s.project); err != nil {
				return fmt.Errorf("postgres: wipe project data: delete %s: %w", tbl, err)
			}
		}
		return nil
	})
}

// scanProject reads a Project from a single pgx.Row.
func scanProject(row pgx.Row) (*storage.Project, error) {
	var (
		slug         string
		syncAdapters []string
		githubRepo   string
		createdAt    time.Time
		updatedAt    time.Time
	)
	if err := row.Scan(&slug, &syncAdapters, &githubRepo, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("postgres: scan project: %w", err)
	}
	if syncAdapters == nil {
		syncAdapters = []string{}
	}
	return &storage.Project{
		Slug:         slug,
		SyncAdapters: syncAdapters,
		GitHubRepo:   githubRepo,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}
