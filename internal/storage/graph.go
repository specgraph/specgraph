// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// NodeRef is a lightweight reference to a graph node.
type NodeRef struct {
	ID    string
	Slug  string
	Label string // Valid values: "Spec" or "Decision"
	Stage string
}

// EdgeType represents the kind of relationship between nodes.
type EdgeType string

const (
	EdgeTypeDependsOn EdgeType = "DEPENDS_ON"
	EdgeTypeBlocks    EdgeType = "BLOCKS"
	EdgeTypeComposes  EdgeType = "COMPOSES"
	EdgeTypeRelatesTo EdgeType = "RELATES_TO"
	EdgeTypeInforms   EdgeType = "INFORMS"
	EdgeTypeDecidedIn EdgeType = "DECIDED_IN"
)

// Edge represents a typed relationship between two graph nodes.
type Edge struct {
	FromID   string
	ToID     string
	EdgeType EdgeType
}

// GraphBackend defines storage operations for graph edges and queries.
type GraphBackend interface {
	// AddEdge creates a typed relationship between two nodes (by slug).
	AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) (*specv1.Edge, error)

	// RemoveEdge removes a typed relationship between two nodes.
	RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType specv1.EdgeType) error

	// ListEdges returns edges for a node, optionally filtered by type.
	ListEdges(ctx context.Context, slug string, edgeType specv1.EdgeType) ([]*specv1.Edge, error)

	// GetDependencies returns direct dependencies of a node.
	GetDependencies(ctx context.Context, slug string) ([]NodeRef, error)

	// GetTransitiveDeps returns all transitive dependencies of a node.
	GetTransitiveDeps(ctx context.Context, slug string) ([]NodeRef, error)

	// GetImpact returns all nodes transitively depending on this node.
	GetImpact(ctx context.Context, slug string) ([]NodeRef, error)

	// GetReady returns specs with all dependencies in "done" stage (or no dependencies).
	GetReady(ctx context.Context) ([]NodeRef, error)

	// GetCriticalPath returns the longest dependency chain ending at a node.
	GetCriticalPath(ctx context.Context, slug string) ([]NodeRef, error)
}
