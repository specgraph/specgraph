// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestConstitution_GetNotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetConstitution(ctx)
	require.Error(t, err)
	require.True(t, errors.Is(err, storage.ErrConstitutionNotFound))
}

func TestConstitution_UpdateAndGet(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	input := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "specgraph-project",
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:   "Go",
				Allowed:   []string{"Go", "TypeScript"},
				Forbidden: []string{"PHP"},
			},
		},
		Principles: []storage.Principle{
			{
				ID:        "p1",
				Statement: "Keep it simple",
				Rationale: "Simple is maintainable",
			},
		},
		Constraints:  []string{"no global state", "no shared mutable state"},
		Antipatterns: []storage.Antipattern{{Pattern: "god object", Why: "too complex", Instead: "small focused types"}},
		References:   []storage.Reference{{Type: "adr", Path: "docs/adr-001.md"}},
	}

	got, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotEmpty(t, got.ID)
	require.Equal(t, storage.ConstitutionLayerProject, got.Layer)
	require.Equal(t, "specgraph-project", got.Name)
	require.Equal(t, int32(1), got.Version)
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())

	// Verify Tech round-trips.
	require.NotNil(t, got.Tech)
	require.NotNil(t, got.Tech.Languages)
	require.Equal(t, "Go", got.Tech.Languages.Primary)
	require.Equal(t, []string{"Go", "TypeScript"}, got.Tech.Languages.Allowed)

	// Verify Principles.
	require.Len(t, got.Principles, 1)
	require.Equal(t, "p1", got.Principles[0].ID)
	require.Equal(t, "Keep it simple", got.Principles[0].Statement)

	// Verify Constraints.
	require.Equal(t, []string{"no global state", "no shared mutable state"}, got.Constraints)

	// Verify Antipatterns.
	require.Len(t, got.Antipatterns, 1)
	require.Equal(t, "god object", got.Antipatterns[0].Pattern)

	// Verify References.
	require.Len(t, got.References, 1)
	require.Equal(t, "adr", got.References[0].Type)

	// Fetch via GetConstitution and confirm same data.
	fetched, err := store.GetConstitution(ctx)
	require.NoError(t, err)
	require.Equal(t, got.ID, fetched.ID)
	require.Equal(t, got.Name, fetched.Name)
	require.Equal(t, got.Version, fetched.Version)

	// Second update bumps version.
	input.Name = "specgraph-project-v2"
	updated, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int32(2), updated.Version)
	require.Equal(t, "specgraph-project-v2", updated.Name)
}

func TestConstitution_MinimalRoundTrip(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	input := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "minimal",
	}

	got, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "minimal", got.Name)
	require.Nil(t, got.Tech)
	require.Nil(t, got.Process)
	require.Empty(t, got.Principles)
	require.Empty(t, got.Constraints)
	require.Empty(t, got.Antipatterns)
	require.Empty(t, got.References)

	// Round-trip via Get
	fetched, err := store.GetConstitution(ctx)
	require.NoError(t, err)
	require.Equal(t, "minimal", fetched.Name)
	require.Nil(t, fetched.Tech)
	require.Empty(t, fetched.Principles)
}

func TestConstitution_CheckViolation(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// CheckViolation on non-existent spec returns ErrSpecNotFound.
	_, err = store.CheckViolation(ctx, "nonexistent-spec")
	require.Error(t, err)
	require.True(t, errors.Is(err, storage.ErrSpecNotFound))

	// Create a spec.
	_, err = store.CreateSpec(ctx, "auth-api", "Implement auth API", "p1", "medium")
	require.NoError(t, err)

	// CheckViolation without a constitution returns ErrConstitutionNotFound.
	_, err = store.CheckViolation(ctx, "auth-api")
	require.Error(t, err)
	require.True(t, errors.Is(err, storage.ErrConstitutionNotFound))

	// Store a constitution.
	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Name:        "test-project",
		Constraints: []string{"no globals"},
	})
	require.NoError(t, err)

	// Now CheckViolation should succeed and return empty violations.
	violations, err := store.CheckViolation(ctx, "auth-api")
	require.NoError(t, err)
	require.Empty(t, violations)
}
