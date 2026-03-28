// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
)

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	GetConstitution(ctx context.Context) (*Constitution, error)

	// UpdateConstitution stores or replaces the constitution, bumping its version.
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
}
