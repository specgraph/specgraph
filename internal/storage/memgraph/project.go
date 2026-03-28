// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

// EnsureProject implements storage.ProjectBackend.
func (s *Store) EnsureProject(ctx context.Context, slug string) (*storage.Project, error) {
	nowStr := s.now()

	records, err := s.executeQuery(ctx,
		`MERGE (p:Project {slug: $slug})
		 ON CREATE SET p.created_at = $now, p.updated_at = $now,
		               p.sync_adapters = [], p.github_repo = ""
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{"slug": slug, "now": nowStr},
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: ensure project: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: ensure project %q: no record returned", slug)
	}

	return recordToProject(records[0])
}

// GetProject implements storage.ProjectBackend.
func (s *Store) GetProject(ctx context.Context, slug string) (*storage.Project, error) {
	records, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $slug})
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{"slug": slug},
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get project: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: get project %q: %w", slug, storage.ErrProjectNotFound)
	}

	return recordToProject(records[0])
}

// UpdateProject implements storage.ProjectBackend.
func (s *Store) UpdateProject(ctx context.Context, slug string, adapters []string, ghRepo string) (*storage.Project, error) {
	nowStr := s.now()

	adaptersAny := make([]any, len(adapters))
	for i, a := range adapters {
		adaptersAny[i] = a
	}

	records, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $slug})
		 SET p.sync_adapters = $adapters,
		     p.github_repo = $github_repo,
		     p.updated_at = $now
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo`,
		map[string]any{
			"slug":        slug,
			"adapters":    adaptersAny,
			"github_repo": ghRepo,
			"now":         nowStr,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update project: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: update project %q: %w", slug, storage.ErrProjectNotFound)
	}

	return recordToProject(records[0])
}

// ListProjects implements storage.ProjectBackend.
func (s *Store) ListProjects(ctx context.Context) ([]*storage.Project, error) {
	records, err := s.executeQuery(ctx,
		`MATCH (p:Project)
		 RETURN p.slug, p.created_at, p.updated_at, p.sync_adapters, p.github_repo
		 ORDER BY p.slug`,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list projects: %w", err)
	}

	projects := make([]*storage.Project, 0, len(records))
	for i, rec := range records {
		p, err := recordToProject(rec)
		if err != nil {
			return nil, fmt.Errorf("memgraph: list projects: scan index %d: %w", i, err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// WipeProjectData deletes all nodes belonging to the project (everything with a
// BELONGS_TO edge pointing to the Project node). The Project node itself is preserved.
func (s *Store) WipeProjectData(ctx context.Context) error {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(n)
		DETACH DELETE n
	`
	_, err := s.executeQuery(ctx, query, map[string]any{"project": s.project})
	return err
}

func recordToProject(rec *neo4j.Record) (*storage.Project, error) {
	p := &storage.Project{}

	var err error
	p.Slug, err = recordString(rec, 0, "slug")
	if err != nil {
		return nil, err
	}

	createdStr, err := recordString(rec, 1, "created_at")
	if err != nil {
		return nil, err
	}
	p.CreatedAt, err = parseRFC3339("created_at", createdStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse created_at: %w", err)
	}

	updatedStr, err := recordString(rec, 2, "updated_at")
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, err = parseRFC3339("updated_at", updatedStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse updated_at: %w", err)
	}

	raw := rec.Values[3]
	if raw != nil {
		if list, ok := raw.([]any); ok {
			p.SyncAdapters = make([]string, len(list))
			for i, v := range list {
				s, ok := v.(string)
				if !ok {
					return nil, fmt.Errorf("memgraph: sync_adapters[%d] is not a string", i)
				}
				p.SyncAdapters[i] = s
			}
		}
	}
	if p.SyncAdapters == nil {
		p.SyncAdapters = []string{}
	}

	p.GitHubRepo, err = recordString(rec, 4, "github_repo")
	if err != nil {
		return nil, err
	}

	return p, nil
}
