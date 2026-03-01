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
	specv1.EdgeType_EDGE_TYPE_BLOCKS:     "DEPENDS_ON", // stored as inverse DEPENDS_ON
	specv1.EdgeType_EDGE_TYPE_COMPOSES:   "COMPOSES",
	specv1.EdgeType_EDGE_TYPE_RELATES_TO: "RELATES_TO",
	specv1.EdgeType_EDGE_TYPE_INFORMS:    "INFORMS",
}

// resolveEdge maps an edge type to its Cypher relation and resolves direction.
// BLOCKS is stored as inverse DEPENDS_ON: if A blocks B, then B DEPENDS_ON A.
func resolveEdge(fromSlug, toSlug string, edgeType specv1.EdgeType) (rel, from, to string, err error) {
	rel, ok := edgeTypeToRel[edgeType]
	if !ok {
		return "", "", "", fmt.Errorf("memgraph: unknown edge type %v", edgeType)
	}
	from, to = fromSlug, toSlug
	if edgeType == specv1.EdgeType_EDGE_TYPE_BLOCKS {
		from, to = toSlug, fromSlug
	}
	return rel, from, to, nil
}

// AddEdge creates a typed relationship between two nodes.
func (s *Store) AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) (*specv1.Edge, error) {
	relType, actualFrom, actualTo, err := resolveEdge(fromSlug, toSlug, edgeType)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		MATCH (a {slug: $from}), (b {slug: $to})
		CREATE (a)-[r:%s]->(b)
		RETURN a.id, b.id
	`, relType)
	params := map[string]any{"from": actualFrom, "to": actualTo}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: add edge: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: one or both nodes not found (from=%q, to=%q)", fromSlug, toSlug)
	}

	fromID, err := recordString(result.Records[0], 0, "from_id")
	if err != nil {
		return nil, err
	}
	toID, err := recordString(result.Records[0], 1, "to_id")
	if err != nil {
		return nil, err
	}

	return &specv1.Edge{
		FromId:   fromID,
		ToId:     toID,
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
			RETURN a.id, b.id, type(r)
			UNION
			MATCH (a {slug: $slug})<-[r:%s]-(b)
			RETURN b.id, a.id, type(r)
		`, relType, relType)
	} else {
		query = `
			MATCH (a {slug: $slug})-[r]->(b)
			RETURN a.id, b.id, type(r)
			UNION
			MATCH (a {slug: $slug})<-[r]-(b)
			RETURN b.id, a.id, type(r)
		`
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list edges: %w", err)
	}

	edges := make([]*specv1.Edge, 0, len(result.Records))
	for _, rec := range result.Records {
		fromID, err := recordString(rec, 0, "from_id")
		if err != nil {
			return nil, err
		}
		toID, err := recordString(rec, 1, "to_id")
		if err != nil {
			return nil, err
		}
		relType, err := recordString(rec, 2, "type")
		if err != nil {
			return nil, err
		}
		edges = append(edges, &specv1.Edge{
			FromId:   fromID,
			ToId:     toID,
			EdgeType: relNameToEdgeType(relType),
		})
	}
	return edges, nil
}

// GetDependencies returns direct dependencies of a node.
func (s *Store) GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})-[:DEPENDS_ON]->(b)
		RETURN b.id, b.slug, labels(b)[0], COALESCE(b.stage, b.status, "")
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetTransitiveDeps returns all transitive dependencies of a node.
func (s *Store) GetTransitiveDeps(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})-[:DEPENDS_ON*]->(b)
		RETURN DISTINCT b.id, b.slug, labels(b)[0], COALESCE(b.stage, b.status, "")
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetImpact returns all nodes transitively depending on this node.
func (s *Store) GetImpact(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	query := `
		MATCH (a {slug: $slug})<-[:DEPENDS_ON*]-(b)
		RETURN DISTINCT b.id, b.slug, labels(b)[0], COALESCE(b.stage, b.status, "")
	`
	return s.queryNodeRefs(ctx, query, map[string]any{"slug": slug})
}

// GetReady returns specs with all dependencies "done" or no dependencies.
func (s *Store) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
	query := `
		MATCH (s:Spec)
		WHERE s.stage <> "done"
		  AND NOT EXISTS {
			MATCH (s)-[:DEPENDS_ON]->(dep:Spec)
			WHERE dep.stage <> "done"
		  }
		RETURN s.id, s.slug, labels(s)[0], s.stage
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
		RETURN n.id, n.slug, labels(n)[0], COALESCE(n.stage, n.status, "")
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
		id, err := recordString(rec, 0, "id")
		if err != nil {
			return nil, err
		}
		slug, err := recordString(rec, 1, "slug")
		if err != nil {
			return nil, err
		}
		label, err := recordString(rec, 2, "label")
		if err != nil {
			return nil, err
		}
		stage, err := recordString(rec, 3, "stage")
		if err != nil {
			return nil, err
		}
		refs = append(refs, storage.NodeRef{
			ID:    id,
			Slug:  slug,
			Label: label,
			Stage: stage,
		})
	}
	return refs, nil
}

func relNameToEdgeType(relType string) specv1.EdgeType {
	switch relType {
	case "DEPENDS_ON":
		return specv1.EdgeType_EDGE_TYPE_DEPENDS_ON
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
