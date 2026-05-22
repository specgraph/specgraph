// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
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

func TestGetConstitutionLayer_RoundTripAllFields(t *testing.T) {
	// Migrated from the pre-Piece-D TestGetConstitution_RoundTrip: exercises
	// full field round-trip through UpdateConstitution + GetConstitutionLayer
	// for the org layer specifically. Field-level assertions are unique to
	// this test (other layer-aware tests check structure but not every field).
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

	got, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerOrg)
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

func TestGetAllLayers_ReturnsOrdered(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "org-layer",
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "project-layer",
	})
	require.NoError(t, err)

	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 2)
	assert.Equal(t, storage.ConstitutionLayerOrg, layers[0].Layer)
	assert.Equal(t, storage.ConstitutionLayerProject, layers[1].Layer)
}

func TestGetConstitutionLayer_ReturnsSpecific(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:       storage.ConstitutionLayerOrg,
		Name:        "org-layer",
		Constraints: []string{"org-constraint"},
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Name:        "project-layer",
		Constraints: []string{"project-constraint"},
	})
	require.NoError(t, err)

	orgLayer, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerOrg)
	require.NoError(t, err)
	assert.Equal(t, storage.ConstitutionLayerOrg, orgLayer.Layer)
	assert.Equal(t, "org-layer", orgLayer.Name)
	assert.Equal(t, []string{"org-constraint"}, orgLayer.Constraints)

	projectLayer, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerProject)
	require.NoError(t, err)
	assert.Equal(t, storage.ConstitutionLayerProject, projectLayer.Layer)
	assert.Equal(t, "project-layer", projectLayer.Name)
	assert.Equal(t, []string{"project-constraint"}, projectLayer.Constraints)
}

func TestUpdateConstitution_IndependentLayers(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "org-original",
	})
	require.NoError(t, err)

	project, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Name:        "project-original",
		Constraints: []string{"project-only"},
	})
	require.NoError(t, err)
	projectVersion := project.Version
	projectContent := project.Constraints

	// Update org layer
	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "org-updated",
	})
	require.NoError(t, err)

	// Verify project layer is unchanged
	got, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerProject)
	require.NoError(t, err)
	assert.Equal(t, projectVersion, got.Version)
	assert.Equal(t, projectContent, got.Constraints)
}

func TestUpdateConstitution_UpsertSameLayer(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	v1, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "org-v1",
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), v1.Version)

	v2, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "org-v2",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), v2.Version)

	// Only one row should exist for this layer
	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, storage.ConstitutionLayerOrg, layers[0].Layer)
	assert.Equal(t, "org-v2", layers[0].Name)
}

func TestGetConstitutionLayer_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerDomain)
	require.ErrorIs(t, err, storage.ErrConstitutionNotFound)
}

func TestGetAllLayers_Empty(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	assert.Empty(t, layers)
}
