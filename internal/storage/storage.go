// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package storage defines backend interfaces for SpecGraph persistence.
package storage

import (
	"context"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// Backend is the interface that all storage backends must implement.
type Backend interface {
	// CreateSpec stores a new spec and returns it with generated ID and timestamps.
	CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*specv1.Spec, error)

	// GetSpec retrieves a spec by slug.
	GetSpec(ctx context.Context, slug string) (*specv1.Spec, error)

	// ListSpecs returns specs matching the given filters.
	// Empty filter values mean "no filter".
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error)

	// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
	// Returns the updated spec with bumped version and updated timestamp.
	UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity *string) (*specv1.Spec, error)

	// Close releases any resources held by the backend.
	Close(ctx context.Context) error
}
