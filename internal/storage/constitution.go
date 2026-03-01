// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrConstitutionNotFound is returned when no constitution exists.
var ErrConstitutionNotFound = errors.New("constitution not found")

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	GetConstitution(ctx context.Context) (*specv1.Constitution, error)

	// UpdateConstitution stores or replaces the constitution, bumping its version.
	UpdateConstitution(ctx context.Context, constitution *specv1.Constitution) (*specv1.Constitution, error)

	// CheckViolation checks a spec against constitution constraints.
	// Returns a list of violations (empty if compliant).
	CheckViolation(ctx context.Context, specSlug string) ([]*specv1.Violation, error)
}
