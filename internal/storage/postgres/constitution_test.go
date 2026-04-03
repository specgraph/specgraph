// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestUpdateConstitution_CreatesNew(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	c := &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Name:        "test-constitution",
		Constraints: []string{"no globals", "prefer immutability"},
	}

	got, err := store.UpdateConstitution(ctx, c)
	require.NoError(t, err)
	require.NotEmpty(t, got.ID)
	require.Equal(t, int32(1), got.Version)
	require.Equal(t, storage.ConstitutionLayerProject, got.Layer)
	require.Equal(t, "test-constitution", got.Name)
	require.Equal(t, []string{"no globals", "prefer immutability"}, got.Constraints)
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestUpdateConstitution_IncrementsVersion(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	c := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "versioned",
	}

	v1, err := store.UpdateConstitution(ctx, c)
	require.NoError(t, err)
	require.Equal(t, int32(1), v1.Version)

	c.ID = v1.ID
	c.Name = "versioned-updated"
	v2, err := store.UpdateConstitution(ctx, c)
	require.NoError(t, err)
	require.Equal(t, int32(2), v2.Version)
	require.Equal(t, "versioned-updated", v2.Name)
	require.Equal(t, v1.ID, v2.ID)
}

func TestGetConstitution_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetConstitution(ctx)
	require.ErrorIs(t, err, storage.ErrConstitutionNotFound)
}

func TestGetConstitution_RoundTrip(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	input := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "round-trip",
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary: "Go",
				Allowed: []string{"Go", "Python"},
			},
			Frameworks: map[string]string{"web": "net/http"},
		},
		Principles: []storage.Principle{
			{ID: "p1", Statement: "keep it simple", Rationale: "complexity kills"},
		},
		Process: &storage.ProcessConfig{
			SpecReview: "required",
			Deployment: &storage.DeploymentConfig{
				Strategy: "blue-green",
				Rollback: "automatic",
			},
		},
		Constraints:  []string{"no singletons", "prefer interfaces"},
		Antipatterns: []storage.Antipattern{{Pattern: "god object", Why: "hard to test", Instead: "small focused types"}},
		References:   []storage.Reference{{Type: "adr", Path: "docs/adr/001.md"}},
	}

	stored, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)

	got, err := store.GetConstitution(ctx)
	require.NoError(t, err)

	require.Equal(t, stored.ID, got.ID)
	require.Equal(t, input.Layer, got.Layer)
	require.Equal(t, input.Name, got.Name)
	require.Equal(t, int32(1), got.Version)

	// Tech
	require.NotNil(t, got.Tech)
	require.Equal(t, "Go", got.Tech.Languages.Primary)
	require.Equal(t, []string{"Go", "Python"}, got.Tech.Languages.Allowed)
	require.Equal(t, "net/http", got.Tech.Frameworks["web"])

	// Principles
	require.Len(t, got.Principles, 1)
	require.Equal(t, "p1", got.Principles[0].ID)
	require.Equal(t, "keep it simple", got.Principles[0].Statement)

	// Process
	require.NotNil(t, got.Process)
	require.Equal(t, "required", got.Process.SpecReview)
	require.NotNil(t, got.Process.Deployment)
	require.Equal(t, "blue-green", got.Process.Deployment.Strategy)

	// Constraints
	require.Equal(t, []string{"no singletons", "prefer interfaces"}, got.Constraints)

	// Antipatterns
	require.Len(t, got.Antipatterns, 1)
	require.Equal(t, "god object", got.Antipatterns[0].Pattern)

	// References
	require.Len(t, got.References, 1)
	require.Equal(t, "adr", got.References[0].Type)
}
