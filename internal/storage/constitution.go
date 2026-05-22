// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
)

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitutionLayer returns a single layer's raw constitution data.
	// Returns ErrConstitutionNotFound if the layer does not exist.
	GetConstitutionLayer(ctx context.Context, layer ConstitutionLayer) (*Constitution, error)

	// GetAllLayers returns all constitution layers for the project,
	// ordered by precedence (user, org, project, domain).
	// Returns an empty slice (not error) if no layers exist.
	GetAllLayers(ctx context.Context) ([]*Constitution, error)

	// GetMergedConstitution returns all layers composed into a single
	// constitution plus per-field provenance. The single source of truth
	// for "the effective constitution."
	//
	// Returns ErrConstitutionNotFound if no layers exist.
	GetMergedConstitution(ctx context.Context) (*MergedResult, error)

	// UpdateConstitution stores or replaces a constitution layer,
	// bumping its version. The layer is determined by constitution.Layer.
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
}
