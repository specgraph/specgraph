// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// resolveEdge maps an edge type to its Cypher relation name.
// EdgeType string values are the Cypher relationship names directly.
func resolveEdge(fromSlug, toSlug string, edgeType storage.EdgeType) (rel, from, to string, err error) {
	if !edgeType.IsValid() {
		return "", "", "", fmt.Errorf("memgraph: unknown edge type %v", edgeType)
	}
	return string(edgeType), fromSlug, toSlug, nil
}

// AddEdge creates a typed relationship between two nodes.
func (s *Store) AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType storage.EdgeType) (*storage.Edge, error) {
	relType, actualFrom, actualTo, err := resolveEdge(fromSlug, toSlug, edgeType)
	if err != nil {
		return nil, err
	}

	var query string
	if edgeType == storage.EdgeTypeDependsOn {
		query = fmt.Sprintf(`
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $from}),
			      (p)<-[:BELONGS_TO]-(b {slug: $to})
			MERGE (a)-[r:%s]->(b)
			ON CREATE SET r.content_hash_at_link = COALESCE(b.content_hash, "")
			RETURN a.slug, b.slug
		`, relType)
	} else {
		query = fmt.Sprintf(`
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $from}),
			      (p)<-[:BELONGS_TO]-(b {slug: $to})
			MERGE (a)-[r:%s]->(b)
			RETURN a.slug, b.slug
		`, relType)
	}
	params := mergeParams(s.projectParam(), map[string]any{"from": actualFrom, "to": actualTo})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: add edge: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: one or both nodes not found (from=%q, to=%q)", fromSlug, toSlug)
	}

	fromSlugVal, err := recordString(records[0], 0, "from_slug")
	if err != nil {
		return nil, err
	}
	toSlugVal, err := recordString(records[0], 1, "to_slug")
	if err != nil {
		return nil, err
	}

	return &storage.Edge{
		FromID:   fromSlugVal,
		ToID:     toSlugVal,
		EdgeType: edgeType,
	}, nil
}

// RemoveEdge removes a typed relationship between two nodes.
func (s *Store) RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType storage.EdgeType) error {
	relType, actualFrom, actualTo, err := resolveEdge(fromSlug, toSlug, edgeType)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $from}),
		      (p)<-[:BELONGS_TO]-(b {slug: $to}),
		      (a)-[r:%s]->(b)
		DELETE r
	`, relType)
	params := mergeParams(s.projectParam(), map[string]any{"from": actualFrom, "to": actualTo})

	if _, err = neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer); err != nil {
		return fmt.Errorf("memgraph: remove edge: %w", err)
	}
	return nil
}

// ListEdges returns edges for a node, optionally filtered by type.
func (s *Store) ListEdges(ctx context.Context, slug string, edgeType storage.EdgeType) ([]*storage.Edge, error) {
	var query string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if edgeType != "" {
		relType := string(edgeType)
		query = fmt.Sprintf(`
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[r:%s]->(b)
			RETURN a.slug AS from_slug, b.slug AS to_slug, type(r) AS rel_type
			UNION
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})<-[r:%s]-(b)
			RETURN b.slug AS from_slug, a.slug AS to_slug, type(r) AS rel_type
		`, relType, relType)
	} else {
		// Exclude internal infrastructure edges (BELONGS_TO, HAS_CHANGE) from
		// unfiltered listing — these are not user-facing edge types.
		query = `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[r]->(b)
			WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE"
			RETURN a.slug AS from_slug, b.slug AS to_slug, type(r) AS rel_type
			UNION
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})<-[r]-(b)
			WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE"
			RETURN b.slug AS from_slug, a.slug AS to_slug, type(r) AS rel_type
		`
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list edges: %w", err)
	}

	edges := make([]*storage.Edge, 0, len(records))
	for _, rec := range records {
		// Use named access via aliases to avoid Memgraph UNION column reordering.
		from, _ := rec.Get("from_slug")
		to, _ := rec.Get("to_slug")
		rt, _ := rec.Get("rel_type")
		edgeType, err := relNameToEdgeType(stringVal(rt))
		if err != nil {
			return nil, fmt.Errorf("ListEdges: %w", err)
		}
		edges = append(edges, &storage.Edge{
			FromID:   stringVal(from),
			ToID:     stringVal(to),
			EdgeType: edgeType,
		})
	}
	return edges, nil
}

// GetDependencies returns direct dependencies of a node.
// A node's dependencies include both nodes it DEPENDS_ON and nodes that BLOCK it.
func (s *Store) GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[:DEPENDS_ON]->(n)
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
		UNION
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug}),
		      (n)-[:BLOCKS]->(a)
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
}

// GetDependenciesWithEdgeData returns DEPENDS_ON dependencies with edge properties.
// Only queries DEPENDS_ON edges (not BLOCKS) — drift is a content concern, not scheduling.
func (s *Store) GetDependenciesWithEdgeData(ctx context.Context, slug string) ([]storage.DependencyRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[dep:DEPENDS_ON]->(n)
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label,
		       COALESCE(n.stage, n.status, "") AS stage,
		       COALESCE(dep.content_hash_at_link, "") AS content_hash_at_link
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: get dependencies with edge data: %w", err)
	}

	refs := make([]storage.DependencyRef, 0, len(records))
	for _, rec := range records {
		id, _ := rec.Get("id")
		sl, _ := rec.Get("slug")
		label, _ := rec.Get("label")
		stage, _ := rec.Get("stage")
		hash, _ := rec.Get("content_hash_at_link")
		refs = append(refs, storage.DependencyRef{
			NodeRef: storage.NodeRef{
				ID:    stringVal(id),
				Slug:  stringVal(sl),
				Label: storage.NodeLabel(stringVal(label)),
				Stage: stringVal(stage),
			},
			ContentHashAtLink: stringVal(hash),
		})
	}
	return refs, nil
}

// GetTransitiveDeps returns all transitive dependencies of a node.
func (s *Store) GetTransitiveDeps(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[:DEPENDS_ON*]->(b)
		RETURN DISTINCT b.id AS id, b.slug AS slug, labels(b)[0] AS label, COALESCE(b.stage, b.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
}

// GetImpact returns all nodes transitively depending on this node.
func (s *Store) GetImpact(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})<-[:DEPENDS_ON*]-(b)
		RETURN DISTINCT b.id AS id, b.slug AS slug, labels(b)[0] AS label, COALESCE(b.stage, b.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
}

// GetReady returns specs with all dependencies "done" or no dependencies.
// A spec is blocked if it has unfinished DEPENDS_ON targets or unfinished BLOCKS sources.
func (s *Store) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)
		WHERE s.stage <> "done"
		OPTIONAL MATCH (s)-[:DEPENDS_ON]->(dep:Spec)
		WHERE dep.stage <> "done"
		WITH s, collect(dep) AS unfinished_deps
		WHERE size(unfinished_deps) = 0
		OPTIONAL MATCH (blocker:Spec)-[:BLOCKS]->(s)
		WHERE blocker.stage <> "done"
		WITH s, collect(blocker) AS active_blockers
		WHERE size(active_blockers) = 0
		RETURN s.id AS id, s.slug AS slug, labels(s)[0] AS label, s.stage AS stage
	`
	return s.queryNodeRefs(ctx, query, s.projectParam())
}

// GetCriticalPath returns the longest dependency chain ending at a node.
func (s *Store) GetCriticalPath(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (proj:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})
		MATCH p = (a)-[:DEPENDS_ON*]->(b)
		OPTIONAL MATCH (b)-[:DEPENDS_ON]->(c)
		WITH p, b, c
		WHERE c IS NULL
		WITH p ORDER BY size(nodes(p)) DESC LIMIT 1
		UNWIND nodes(p) AS n
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
}

func (s *Store) queryNodeRefs(ctx context.Context, query string, params map[string]any) ([]storage.NodeRef, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: graph query: %w", err)
	}

	refs := make([]storage.NodeRef, 0, len(result.Records))
	for _, rec := range result.Records {
		// Use named access via aliases to avoid Memgraph UNION column reordering.
		id, _ := rec.Get("id")
		slug, _ := rec.Get("slug")
		label, _ := rec.Get("label")
		stage, _ := rec.Get("stage")
		refs = append(refs, storage.NodeRef{
			ID:    stringVal(id),
			Slug:  stringVal(slug),
			Label: storage.NodeLabel(stringVal(label)),
			Stage: stringVal(stage),
		})
	}
	return refs, nil
}

// stringVal safely converts an any value to string, returning "" for non-strings.
func stringVal(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func relNameToEdgeType(relType string) (storage.EdgeType, error) {
	et := storage.EdgeType(relType)
	if !et.IsValid() {
		return "", fmt.Errorf("unknown edge relation type: %q", relType)
	}
	return et, nil
}
