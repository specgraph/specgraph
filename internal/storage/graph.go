// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "context"

// NodeLabel represents the type of a graph node.
type NodeLabel string

// Node label values.
const (
	NodeLabelSpec     NodeLabel = "Spec"
	NodeLabelDecision NodeLabel = "Decision"
)

// IsValid reports whether l is a known node label.
func (l NodeLabel) IsValid() bool {
	switch l {
	case NodeLabelSpec, NodeLabelDecision:
		return true
	default:
		return false
	}
}

// NodeRef is a lightweight reference to a graph node.
type NodeRef struct {
	ID    string
	Slug  string
	Label NodeLabel
	Stage string
}

// EdgeType represents the kind of relationship between nodes.
type EdgeType string

// Edge type relationship values.
const (
	EdgeTypeDependsOn EdgeType = "DEPENDS_ON"
	EdgeTypeBlocks    EdgeType = "BLOCKS"
	EdgeTypeComposes  EdgeType = "COMPOSES"
	EdgeTypeRelatesTo EdgeType = "RELATES_TO"
	EdgeTypeInforms   EdgeType = "INFORMS"
	EdgeTypeDecidedIn EdgeType = "DECIDED_IN"
)

// IsValid reports whether e is a known edge type.
func (e EdgeType) IsValid() bool {
	switch e {
	case EdgeTypeDependsOn, EdgeTypeBlocks, EdgeTypeComposes,
		EdgeTypeRelatesTo, EdgeTypeInforms, EdgeTypeDecidedIn:
		return true
	default:
		return false
	}
}

// Edge represents a typed relationship between two graph nodes.
type Edge struct {
	FromID   string
	ToID     string
	EdgeType EdgeType
}

// GraphBackend defines storage operations for graph edges and queries.
type GraphBackend interface {
	// AddEdge creates a typed relationship between two nodes (by slug).
	AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType EdgeType) (*Edge, error)

	// RemoveEdge removes a typed relationship between two nodes.
	RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType EdgeType) error

	// ListEdges returns edges for a node, optionally filtered by type.
	ListEdges(ctx context.Context, slug string, edgeType EdgeType) ([]*Edge, error)

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
