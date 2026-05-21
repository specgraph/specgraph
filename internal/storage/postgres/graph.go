// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.GraphBackend = (*Store)(nil)

// AddEdge creates a typed relationship between two nodes (by slug).
// For DEPENDS_ON edges, captures the upstream's content_hash at link time.
// The node existence check, hash lookup, and INSERT are wrapped in a single
// transaction so they are atomic.
func (s *Store) AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType storage.EdgeType) (*storage.Edge, error) {
	if !edgeType.IsValid() {
		return nil, fmt.Errorf("unsupported edge type: %q", edgeType)
	}

	var result *storage.Edge
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify both nodes exist in the project.
		fromExists, err := s.nodeExists(txCtx, fromSlug)
		if err != nil {
			return fmt.Errorf("postgres: add edge: check from node: %w", err)
		}
		toExists, err := s.nodeExists(txCtx, toSlug)
		if err != nil {
			return fmt.Errorf("postgres: add edge: check to node: %w", err)
		}
		if !fromExists || !toExists {
			return fmt.Errorf("postgres: one or both nodes not found (from=%q, to=%q)", fromSlug, toSlug)
		}

		var contentHashAtLink string
		if edgeType == storage.EdgeTypeDependsOn {
			// Look up upstream's content_hash.
			var hashErr error
			contentHashAtLink, hashErr = s.lookupContentHash(txCtx, toSlug)
			if hashErr != nil {
				return fmt.Errorf("postgres: add edge: lookup content hash: %w", hashErr)
			}
		}

		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug, content_hash_at_link)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (project_slug, from_slug, to_slug, edge_type) DO NOTHING`,
			fromSlug, toSlug, string(edgeType), s.project, contentHashAtLink,
		)
		if err != nil {
			return fmt.Errorf("postgres: add edge: %w", err)
		}

		result = &storage.Edge{
			FromID:            fromSlug,
			ToID:              toSlug,
			EdgeType:          edgeType,
			ContentHashAtLink: contentHashAtLink,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RemoveEdge removes a typed relationship between two nodes.
func (s *Store) RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType storage.EdgeType) error {
	if !edgeType.IsValid() {
		return fmt.Errorf("unsupported edge type: %q", edgeType)
	}

	_, err := s.exec(ctx,
		`DELETE FROM edges
		 WHERE from_slug = $1 AND to_slug = $2 AND edge_type = $3 AND project_slug = $4`,
		fromSlug, toSlug, string(edgeType), s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: remove edge: %w", err)
	}
	return nil
}

// ListEdges returns edges for a node, optionally filtered by type.
// Bidirectional: returns both outgoing and incoming edges.
// Excludes internal edge types when no specific type filter is given.
func (s *Store) ListEdges(ctx context.Context, slug string, edgeType storage.EdgeType) ([]*storage.Edge, error) {
	var rows interface {
		Next() bool
		Scan(dest ...any) error
		Err() error
		Close()
	}
	var err error

	if edgeType != "" {
		rows, err = s.query(ctx,
			`SELECT from_slug, to_slug, edge_type FROM edges
			 WHERE from_slug = $1 AND edge_type = $2 AND project_slug = $3
			 UNION ALL
			 SELECT from_slug, to_slug, edge_type FROM edges
			 WHERE to_slug = $1 AND edge_type = $2 AND project_slug = $3`,
			slug, string(edgeType), s.project,
		)
	} else {
		rows, err = s.query(ctx,
			`SELECT from_slug, to_slug, edge_type FROM edges
			 WHERE from_slug = $1 AND project_slug = $2
			   AND edge_type NOT IN ('BELONGS_TO','HAS_CHANGE','HAS_FINDING','HAS_EVENT','CLAIMED_BY','AUTHORED_VIA','CONTINUES','EXPLAINS')
			 UNION ALL
			 SELECT from_slug, to_slug, edge_type FROM edges
			 WHERE to_slug = $1 AND project_slug = $2
			   AND edge_type NOT IN ('BELONGS_TO','HAS_CHANGE','HAS_FINDING','HAS_EVENT','CLAIMED_BY','AUTHORED_VIA','CONTINUES','EXPLAINS')`,
			slug, s.project,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: list edges: %w", err)
	}
	defer rows.Close()

	var edges []*storage.Edge
	for rows.Next() {
		var from, to, et string
		if err := rows.Scan(&from, &to, &et); err != nil {
			return nil, fmt.Errorf("postgres: list edges: scan: %w", err)
		}
		edges = append(edges, &storage.Edge{
			FromID:   from,
			ToID:     to,
			EdgeType: storage.EdgeType(et),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list edges: rows: %w", err)
	}
	if edges == nil {
		edges = []*storage.Edge{}
	}
	return edges, nil
}

// GetDependencies returns direct dependencies of a node.
// Includes both DEPENDS_ON targets and BLOCKS sources.
func (s *Store) GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	rows, err := s.query(ctx,
		`SELECT e.to_slug AS slug
		 FROM edges e
		 WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		 UNION
		 SELECT e.from_slug AS slug
		 FROM edges e
		 WHERE e.to_slug = $1 AND e.edge_type = 'BLOCKS' AND e.project_slug = $2`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get dependencies: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var depSlug string
		if err := rows.Scan(&depSlug); err != nil {
			return nil, fmt.Errorf("postgres: get dependencies: scan: %w", err)
		}
		slugs = append(slugs, depSlug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get dependencies: rows: %w", err)
	}

	return s.resolveNodeRefs(ctx, slugs)
}

// GetDependenciesWithEdgeData returns DEPENDS_ON dependencies with edge properties.
// Used by drift detection to compare content hashes. Does NOT include BLOCKS edges.
func (s *Store) GetDependenciesWithEdgeData(ctx context.Context, slug string) ([]storage.DependencyRef, error) {
	rows, err := s.query(ctx,
		`SELECT e.to_slug,
		        COALESCE(e.content_hash_at_link, '') AS content_hash_at_link,
		        COALESCE(
		            COALESCE(sp.content_hash, d.content_hash),
		            ''
		        ) AS upstream_content_hash
		 FROM edges e
		 LEFT JOIN specs sp ON sp.slug = e.to_slug AND sp.project_slug = e.project_slug
		 LEFT JOIN decisions d ON d.slug = e.to_slug AND d.project_slug = e.project_slug
		 WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get dependencies with edge data: %w", err)
	}
	defer rows.Close()

	var refs []storage.DependencyRef
	for rows.Next() {
		var toSlug, hashAtLink, upstreamHash string
		if err := rows.Scan(&toSlug, &hashAtLink, &upstreamHash); err != nil {
			return nil, fmt.Errorf("postgres: get dependencies with edge data: scan: %w", err)
		}
		nr, nrErr := s.resolveNodeRef(ctx, toSlug)
		if nrErr != nil {
			return nil, fmt.Errorf("postgres: get dependencies with edge data: resolve %q: %w", toSlug, nrErr)
		}
		refs = append(refs, storage.DependencyRef{
			NodeRef:             nr,
			ContentHashAtLink:   hashAtLink,
			UpstreamContentHash: upstreamHash,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get dependencies with edge data: rows: %w", err)
	}
	if refs == nil {
		refs = []storage.DependencyRef{}
	}
	return refs, nil
}

// RefreshDependencyHashes updates content_hash_at_link on all outgoing
// DEPENDS_ON edges for a spec, setting them to each upstream's current content_hash.
func (s *Store) RefreshDependencyHashes(ctx context.Context, slug string) error {
	_, err := s.exec(ctx,
		`UPDATE edges e
		 SET content_hash_at_link = COALESCE(upstream.content_hash, '')
		 FROM (
		     SELECT slug, content_hash FROM specs WHERE project_slug = $2
		     UNION ALL
		     SELECT slug, content_hash FROM decisions WHERE project_slug = $2
		 ) upstream
		 WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON'
		   AND e.project_slug = $2 AND upstream.slug = e.to_slug`,
		slug, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: refresh dependency hashes: %w", err)
	}
	return nil
}

// refreshInboundDependencyHashes updates content_hash_at_link on all incoming
// DEPENDS_ON edges that point to slug, setting them to slug's current content_hash.
// Called when a spec transitions to done so downstream specs see the refreshed baseline.
func (s *Store) refreshInboundDependencyHashes(ctx context.Context, slug string) error {
	_, err := s.exec(ctx,
		`UPDATE edges e
		 SET content_hash_at_link = COALESCE(upstream.content_hash, '')
		 FROM (
		     SELECT slug, content_hash FROM specs
		     WHERE slug = $1 AND project_slug = $2
		 ) upstream
		 WHERE e.to_slug = $1 AND e.edge_type = 'DEPENDS_ON'
		   AND e.project_slug = $2`,
		slug, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: refresh inbound dependency hashes: %w", err)
	}
	return nil
}

// GetTransitiveDeps returns all transitive dependencies of a node.
// Uses a recursive CTE bounded to 50 hops with Postgres CYCLE detection.
func (s *Store) GetTransitiveDeps(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	rows, err := s.query(ctx,
		`WITH RECURSIVE transitive AS (
		     SELECT e.to_slug, 1 AS depth
		     FROM edges e
		     WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     UNION ALL
		     SELECT e.to_slug, t.depth + 1
		     FROM transitive t
		     JOIN edges e ON e.from_slug = t.to_slug
		                 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     WHERE t.depth < 50
		 ) CYCLE to_slug SET is_cycle USING path
		 SELECT DISTINCT to_slug FROM transitive WHERE NOT is_cycle`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get transitive deps: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var depSlug string
		if err := rows.Scan(&depSlug); err != nil {
			return nil, fmt.Errorf("postgres: get transitive deps: scan: %w", err)
		}
		slugs = append(slugs, depSlug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get transitive deps: rows: %w", err)
	}

	return s.resolveNodeRefs(ctx, slugs)
}

// GetImpact returns all nodes transitively depending on this node.
// Mirror of GetTransitiveDeps, following edges backward.
func (s *Store) GetImpact(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	rows, err := s.query(ctx,
		`WITH RECURSIVE impact AS (
		     SELECT e.from_slug AS slug, 1 AS depth
		     FROM edges e
		     WHERE e.to_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     UNION ALL
		     SELECT e.from_slug, i.depth + 1
		     FROM impact i
		     JOIN edges e ON e.to_slug = i.slug
		                 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     WHERE i.depth < 50
		 ) CYCLE slug SET is_cycle USING path
		 SELECT DISTINCT slug FROM impact WHERE NOT is_cycle`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get impact: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var impactSlug string
		if err := rows.Scan(&impactSlug); err != nil {
			return nil, fmt.Errorf("postgres: get impact: scan: %w", err)
		}
		slugs = append(slugs, impactSlug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get impact: rows: %w", err)
	}

	return s.resolveNodeRefs(ctx, slugs)
}

// GetReady returns specs with all dependencies "done" or no dependencies.
// A spec is blocked if it has unfinished DEPENDS_ON targets or unfinished BLOCKS sources.
//
// By design, the dependency/blocker checks only consider Spec nodes (not decisions or
// slices). GetReady finds ready *specs*; cross-type edges (e.g., spec → decision) are
// not part of the readiness model. This matches the Memgraph implementation which
// matches only (dep:Spec) and (blocker:Spec) patterns.
func (s *Store) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
	rows, err := s.query(ctx,
		`SELECT s.slug, 'Spec' AS label, s.stage
		 FROM specs s
		 WHERE s.project_slug = $1
		   AND s.stage = 'approved'
		   AND s.provenance_type = 'authored'
		   AND NOT EXISTS (
		       -- Active claim by any agent
		       SELECT 1 FROM claims c
		       WHERE c.project_slug = $1 AND c.spec_slug = s.slug
		         AND c.lease_expires > NOW()
		   )
		   AND NOT EXISTS (
		       -- Only Spec deps are checked by design; see function comment.
		       SELECT 1 FROM edges e
		       JOIN specs dep ON dep.slug = e.to_slug AND dep.project_slug = $1
		       WHERE e.from_slug = s.slug AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $1
		         AND dep.stage <> 'done'
		   )
		   AND NOT EXISTS (
		       -- Only Spec blockers are checked by design; see function comment.
		       SELECT 1 FROM edges e
		       JOIN specs blocker ON blocker.slug = e.from_slug AND blocker.project_slug = $1
		       WHERE e.to_slug = s.slug AND e.edge_type = 'BLOCKS' AND e.project_slug = $1
		         AND blocker.stage <> 'done'
		   )`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get ready: %w", err)
	}
	defer rows.Close()

	var refs []storage.NodeRef
	for rows.Next() {
		var slug, label, stage string
		if err := rows.Scan(&slug, &label, &stage); err != nil {
			return nil, fmt.Errorf("postgres: get ready: scan: %w", err)
		}
		refs = append(refs, storage.NodeRef{
			Slug:  slug,
			Label: storage.NodeLabel(label),
			Stage: stage,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get ready: rows: %w", err)
	}
	if refs == nil {
		refs = []storage.NodeRef{}
	}
	return refs, nil
}

// GetCriticalPath returns the longest dependency chain starting at a node.
func (s *Store) GetCriticalPath(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	rows, err := s.query(ctx,
		`WITH RECURSIVE chains AS (
		     SELECT $1::text AS current_slug, ARRAY[$1::text] AS path, 0 AS depth
		     UNION ALL
		     SELECT e.to_slug, c.path || e.to_slug, c.depth + 1
		     FROM chains c
		     JOIN edges e ON e.from_slug = c.current_slug
		                 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     WHERE c.depth < 50 AND NOT e.to_slug = ANY(c.path)
		 ),
		 leaf_paths AS (
		     SELECT path FROM chains c
		     WHERE NOT EXISTS (
		         SELECT 1 FROM edges e
		         WHERE e.from_slug = c.current_slug
		           AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
		     )
		     ORDER BY array_length(path, 1) DESC LIMIT 1
		 )
		 SELECT node_slug, ordinality
		 FROM leaf_paths, unnest(path) WITH ORDINALITY AS t(node_slug, ordinality)
		 ORDER BY ordinality`,
		slug, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get critical path: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var nodeSlug string
		var ordinality int
		if err := rows.Scan(&nodeSlug, &ordinality); err != nil {
			return nil, fmt.Errorf("postgres: get critical path: scan: %w", err)
		}
		slugs = append(slugs, nodeSlug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get critical path: rows: %w", err)
	}

	return s.resolveNodeRefs(ctx, slugs)
}

// GetFullGraph returns all spec, decision, and slice nodes with all user-facing edges.
func (s *Store) GetFullGraph(ctx context.Context) (*storage.FullGraph, error) {
	// Query 1: All nodes (Spec + Decision + Slice).
	nodeRows, err := s.query(ctx,
		`SELECT slug, 'Spec' AS label,
		        COALESCE(stage, '') AS stage,
		        COALESCE(intent, '') AS intent,
		        COALESCE(priority, '') AS priority
		 FROM specs WHERE project_slug = $1
		 UNION ALL
		 SELECT slug, 'Decision' AS label,
		        COALESCE(status, '') AS stage,
		        COALESCE(title, '') AS intent,
		        '' AS priority
		 FROM decisions WHERE project_slug = $1
		 UNION ALL
		 SELECT slug, 'Slice' AS label,
		        COALESCE(status, '') AS stage,
		        COALESCE(intent, '') AS intent,
		        '' AS priority
		 FROM slices WHERE project_slug = $1`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get full graph nodes: %w", err)
	}
	defer nodeRows.Close()

	var nodes []storage.GraphNode
	seen := make(map[string]bool)
	for nodeRows.Next() {
		var slug, label, stage, intent, priority string
		if scanErr := nodeRows.Scan(&slug, &label, &stage, &intent, &priority); scanErr != nil {
			return nil, fmt.Errorf("postgres: get full graph nodes: scan: %w", scanErr)
		}
		if seen[slug] {
			slog.Warn("duplicate node slug in project — data integrity issue", "slug", slug, "project", s.project)
			continue
		}
		seen[slug] = true
		nodes = append(nodes, storage.GraphNode{
			Slug:     slug,
			Label:    storage.NodeLabel(label),
			Stage:    stage,
			Intent:   intent,
			Priority: priority,
		})
	}
	if rowsErr := nodeRows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("postgres: get full graph nodes: rows: %w", rowsErr)
	}

	// Query 2: All user-facing edges.
	edgeRows, err := s.query(ctx,
		`SELECT from_slug, to_slug, edge_type FROM edges
		 WHERE project_slug = $1
		   AND edge_type NOT IN ('BELONGS_TO','HAS_CHANGE','HAS_FINDING','HAS_EVENT','CLAIMED_BY','AUTHORED_VIA','CONTINUES','EXPLAINS')`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get full graph edges: %w", err)
	}
	defer edgeRows.Close()

	var edges []*storage.Edge
	for edgeRows.Next() {
		var from, to, et string
		if err := edgeRows.Scan(&from, &to, &et); err != nil {
			return nil, fmt.Errorf("postgres: get full graph edges: scan: %w", err)
		}
		edges = append(edges, &storage.Edge{
			FromID:   from,
			ToID:     to,
			EdgeType: storage.EdgeType(et),
		})
	}
	if err := edgeRows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get full graph edges: rows: %w", err)
	}

	if nodes == nil {
		nodes = []storage.GraphNode{}
	}
	if edges == nil {
		edges = []*storage.Edge{}
	}

	return &storage.FullGraph{Nodes: nodes, Edges: edges}, nil
}

// nodeExists checks whether a slug exists in any node table for this project.
func (s *Store) nodeExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := s.queryRow(ctx,
		`SELECT EXISTS(
		     SELECT 1 FROM specs WHERE slug = $1 AND project_slug = $2
		     UNION ALL
		     SELECT 1 FROM decisions WHERE slug = $1 AND project_slug = $2
		     UNION ALL
		     SELECT 1 FROM slices WHERE slug = $1 AND project_slug = $2
		 )`,
		slug, s.project,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: node exists: %w", err)
	}
	return exists, nil
}

// lookupContentHash finds the content_hash for a slug across specs and decisions.
// Returns ("", nil) if the slug has no content hash or does not exist.
func (s *Store) lookupContentHash(ctx context.Context, slug string) (string, error) {
	var hash string
	err := s.queryRow(ctx,
		`SELECT COALESCE(content_hash, '') FROM (
		     SELECT content_hash FROM specs WHERE slug = $1 AND project_slug = $2
		     UNION ALL
		     SELECT content_hash FROM decisions WHERE slug = $1 AND project_slug = $2
		 ) sub LIMIT 1`,
		slug, s.project,
	).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("postgres: lookup content hash: %w", err)
	}
	return hash, nil
}

// resolveNodeRef resolves a single slug to its NodeRef by checking node tables.
// Returns a minimal NodeRef (slug only) if the slug is not found.
// Returns an error for actual DB failures.
func (s *Store) resolveNodeRef(ctx context.Context, slug string) (storage.NodeRef, error) {
	var label, stage string
	err := s.queryRow(ctx,
		`SELECT label, stage FROM (
		     SELECT 'Spec' AS label, COALESCE(stage, '') AS stage
		     FROM specs WHERE slug = $1 AND project_slug = $2
		     UNION ALL
		     SELECT 'Decision' AS label, COALESCE(status, '') AS stage
		     FROM decisions WHERE slug = $1 AND project_slug = $2
		     UNION ALL
		     SELECT 'Slice' AS label, COALESCE(status, '') AS stage
		     FROM slices WHERE slug = $1 AND project_slug = $2
		 ) sub LIMIT 1`,
		slug, s.project,
	).Scan(&label, &stage)
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.NodeRef{Slug: slug}, nil
	}
	if err != nil {
		return storage.NodeRef{}, fmt.Errorf("postgres: resolve node ref %q: %w", slug, err)
	}
	return storage.NodeRef{
		Slug:  slug,
		Label: storage.NodeLabel(label),
		Stage: stage,
	}, nil
}

// resolveNodeRefs resolves a list of slugs to NodeRefs using a single batch query.
// Preserves order of the input slugs.
func (s *Store) resolveNodeRefs(ctx context.Context, slugs []string) ([]storage.NodeRef, error) {
	if len(slugs) == 0 {
		return []storage.NodeRef{}, nil
	}

	rows, err := s.query(ctx,
		`SELECT slug, label, stage FROM (
		     SELECT slug, 'Spec' AS label, COALESCE(stage, '') AS stage
		     FROM specs WHERE slug = ANY($1) AND project_slug = $2
		     UNION ALL
		     SELECT slug, 'Decision' AS label, COALESCE(status, '') AS stage
		     FROM decisions WHERE slug = ANY($1) AND project_slug = $2
		     UNION ALL
		     SELECT slug, 'Slice' AS label, COALESCE(status, '') AS stage
		     FROM slices WHERE slug = ANY($1) AND project_slug = $2
		 ) sub`,
		slugs, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: resolve node refs: %w", err)
	}
	defer rows.Close()

	lookup := make(map[string]storage.NodeRef, len(slugs))
	for rows.Next() {
		var slug, label, stage string
		if err := rows.Scan(&slug, &label, &stage); err != nil {
			return nil, fmt.Errorf("postgres: resolve node refs: scan: %w", err)
		}
		if _, exists := lookup[slug]; !exists {
			lookup[slug] = storage.NodeRef{
				Slug:  slug,
				Label: storage.NodeLabel(label),
				Stage: stage,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: resolve node refs: rows: %w", err)
	}

	// Preserve input order.
	refs := make([]storage.NodeRef, 0, len(slugs))
	for _, slug := range slugs {
		if ref, ok := lookup[slug]; ok {
			refs = append(refs, ref)
		} else {
			refs = append(refs, storage.NodeRef{Slug: slug})
		}
	}
	return refs, nil
}
