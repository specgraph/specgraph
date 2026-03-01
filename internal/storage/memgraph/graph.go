// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

var edgeTypeToRel = map[specv1.EdgeType]string{
	specv1.EdgeType_EDGE_TYPE_DEPENDS_ON: "DEPENDS_ON",
	specv1.EdgeType_EDGE_TYPE_BLOCKS:     "BLOCKS",
	specv1.EdgeType_EDGE_TYPE_COMPOSES:   "COMPOSES",
	specv1.EdgeType_EDGE_TYPE_RELATES_TO: "RELATES_TO",
	specv1.EdgeType_EDGE_TYPE_INFORMS:    "INFORMS",
}

// resolveEdge maps an edge type to its Cypher relation name.
// All edge types are stored as-is: (from)-[:REL]->(to).
func resolveEdge(fromSlug, toSlug string, edgeType specv1.EdgeType) (rel, from, to string, err error) {
	rel, ok := edgeTypeToRel[edgeType]
	if !ok {
		return "", "", "", fmt.Errorf("memgraph: unknown edge type %v", edgeType)
	}
	return rel, fromSlug, toSlug, nil
}

// AddEdge creates a typed relationship between two nodes.
func (s *Store) AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) (*specv1.Edge, error) {
	relType, actualFrom, actualTo, err := resolveEdge(fromSlug, toSlug, edgeType)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		MATCH (a {slug: $from}), (b {slug: $to})
		MERGE (a)-[r:%s]->(b)
		RETURN a.slug, b.slug
	`, relType)
	params := map[string]any{"from": actualFrom, "to": actualTo}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: add edge: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: one or both nodes not found (from=%q, to=%q)", fromSlug, toSlug)
	}

	fromSlugVal, err := recordString(result.Records[0], 0, "from_slug")
	if err != nil {
		return nil, err
	}
	toSlugVal, err := recordString(result.Records[0], 1, "to_slug")
	if err != nil {
		return nil, err
	}

	return &specv1.Edge{
		FromId:   fromSlugVal,
		ToId:     toSlugVal,
		EdgeType: edgeType,
	}, nil
}

// RemoveEdge removes a typed relationship between two nodes.
func (s *Store) RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) error {
	relType, actualFrom, actualTo, err := resolveEdge(fromSlug, toSlug, edgeType)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
		MATCH (a {slug: $from})-[r:%s]->(b {slug: $to})
		DELETE r
	`, relType)
	params := map[string]any{"from": actualFrom, "to": actualTo}

	if _, err = neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer); err != nil {
		return fmt.Errorf("memgraph: remove edge: %w", err)
	}
	return nil
}

// ListEdges returns edges for a node, optionally filtered by type.
func (s *Store) ListEdges(ctx context.Context, slug string, edgeType specv1.EdgeType) ([]*specv1.Edge, error) {
	var query string
	params := map[string]any{"slug": slug}

	if edgeType != specv1.EdgeType_EDGE_TYPE_UNSPECIFIED {
		relType := edgeTypeToRel[edgeType]
		query = fmt.Sprintf(`
			MATCH (a {slug: $slug})-[r:%s]->(b)
			RETURN a.slug AS from_slug, b.slug AS to_slug, type(r) AS rel_type
			UNION
			MATCH (a {slug: $slug})<-[r:%s]-(b)
			RETURN b.slug AS from_slug, a.slug AS to_slug, type(r) AS rel_type
		`, relType, relType)
	} else {
		query = `
			MATCH (a {slug: $slug})-[r]->(b)
			RETURN a.slug AS from_slug, b.slug AS to_slug, type(r) AS rel_type
			UNION
			MATCH (a {slug: $slug})<-[r]-(b)
			RETURN b.slug AS from_slug, a.slug AS to_slug, type(r) AS rel_type
		`
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list edges: %w", err)
	}

	edges := make([]*specv1.Edge, 0, len(result.Records))
	for _, rec := range result.Records {
		// Use named access via aliases to avoid Memgraph UNION column reordering.
		from, _ := rec.Get("from_slug")
		to, _ := rec.Get("to_slug")
		rt, _ := rec.Get("rel_type")
		edges = append(edges, &specv1.Edge{
			FromId:   stringVal(from),
			ToId:     stringVal(to),
			EdgeType: relNameToEdgeType(stringVal(rt)),
		})
	}
	return edges, nil
}

// GetDependencies returns direct dependencies of a node.
// A node's dependencies include both nodes it DEPENDS_ON and nodes that BLOCK it.
func (s *Store) GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})-[:DEPENDS_ON]->(n)
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
		UNION
		MATCH (n)-[:BLOCKS]->(a {slug: $slug})
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetTransitiveDeps returns all transitive dependencies of a node.
func (s *Store) GetTransitiveDeps(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})-[:DEPENDS_ON*]->(b)
		RETURN DISTINCT b.id AS id, b.slug AS slug, labels(b)[0] AS label, COALESCE(b.stage, b.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetImpact returns all nodes transitively depending on this node.
func (s *Store) GetImpact(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})<-[:DEPENDS_ON*]-(b)
		RETURN DISTINCT b.id AS id, b.slug AS slug, labels(b)[0] AS label, COALESCE(b.stage, b.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetReady returns specs with all dependencies "done" or no dependencies.
// A spec is blocked if it has unfinished DEPENDS_ON targets or unfinished BLOCKS sources.
func (s *Store) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
	query := `
		MATCH (s:Spec)
		WHERE s.stage <> "done"
		  AND NOT EXISTS {
			MATCH (s)-[:DEPENDS_ON]->(dep:Spec)
			WHERE dep.stage <> "done"
		  }
		  AND NOT EXISTS {
			MATCH (blocker:Spec)-[:BLOCKS]->(s)
			WHERE blocker.stage <> "done"
		  }
		RETURN s.id AS id, s.slug AS slug, labels(s)[0] AS label, s.stage AS stage
	`
	return s.queryNodeRefs(ctx, query, map[string]any{})
}

// GetCriticalPath returns the longest dependency chain ending at a node.
func (s *Store) GetCriticalPath(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH p = (a {slug: $slug})-[:DEPENDS_ON*]->(b)
		WHERE NOT (b)-[:DEPENDS_ON]->()
		WITH p ORDER BY length(p) DESC LIMIT 1
		UNWIND nodes(p) AS n
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label, COALESCE(n.stage, n.status, "") AS stage
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
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
			Label: stringVal(label),
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

func relNameToEdgeType(relType string) specv1.EdgeType {
	switch relType {
	case "DEPENDS_ON":
		return specv1.EdgeType_EDGE_TYPE_DEPENDS_ON
	case "BLOCKS":
		return specv1.EdgeType_EDGE_TYPE_BLOCKS
	case "COMPOSES":
		return specv1.EdgeType_EDGE_TYPE_COMPOSES
	case "RELATES_TO":
		return specv1.EdgeType_EDGE_TYPE_RELATES_TO
	case "INFORMS":
		return specv1.EdgeType_EDGE_TYPE_INFORMS
	default:
		return specv1.EdgeType_EDGE_TYPE_UNSPECIFIED
	}
}
