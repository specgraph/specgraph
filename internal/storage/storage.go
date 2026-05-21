// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package storage defines backend interfaces for SpecGraph persistence.
package storage

import "context"

// TransactionalBackend provides atomic multi-operation support.
// Implementations wrap multiple storage calls in a single transaction so that
// either all succeed or none take effect.
type TransactionalBackend interface {
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// SpecReader is the read-only subset of storage used by drift and linter engines.
type SpecReader interface {
	GetSpec(ctx context.Context, slug string) (*Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]NodeRef, error)
}

// Backend is the interface that all storage backends must implement.
type Backend interface {
	// CreateSpec stores a new spec and returns it with generated ID and timestamps.
	// provenance, detail, and stage outputs support all three creation flows:
	// AUTHORED (spark only), RETROACTIVE_FROM_PR, and DECLARED (born at done).
	CreateSpec(
		ctx context.Context,
		slug, intent, priority, complexity string,
		provenance SpecProvenanceType,
		detail SpecProvenanceDetail,
		spark *SparkOutput,
		shape *ShapeOutput,
		specify *SpecifyOutput,
		decompose *DecomposeOutput,
	) (*Spec, error)

	// GetSpec retrieves a spec by slug.
	GetSpec(ctx context.Context, slug string) (*Spec, error)

	// ListSpecs returns specs matching the given filters.
	// Empty filter values mean "no filter".
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*Spec, error)

	// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
	// Returns the updated spec with bumped version and updated timestamp.
	UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*Spec, error)

	// Close releases any resources held by the backend.
	Close(ctx context.Context) error
}
