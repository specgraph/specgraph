// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestGetMergedConstitution_Empty(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetMergedConstitution(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrConstitutionNotFound),
		"empty project must return ErrConstitutionNotFound, got %v", err)
}

func TestGetMergedConstitution_SingleLayer(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "test",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "Prefer explicit over implicit"},
		},
	})
	require.NoError(t, err)

	result, err := store.GetMergedConstitution(ctx)

	require.NoError(t, err)
	require.NotNil(t, result.Constitution)
	assert.Equal(t, "test", result.Constitution.Name,
		"merge must preserve scalar metadata from the single layer")
	assert.Equal(t, storage.ConstitutionLayerProject, result.Constitution.Layer,
		"merged Layer reflects the highest-precedence input layer")
	assert.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p1]"],
		"provenance must attribute p1 to project layer")
}

func TestGetMergedConstitution_MultiLayer(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "org",
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p-org", Statement: "Org rule"},
			{ID: "p-shared", Statement: "Org's version"},
		},
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "project",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p-proj", Statement: "Project rule"},
			{ID: "p-shared", Statement: "Project's version"},
		},
	})
	require.NoError(t, err)

	result, err := store.GetMergedConstitution(ctx)

	require.NoError(t, err)
	require.NotNil(t, result.Constitution)
	assert.Len(t, result.Constitution.Principles, 3, "merge must yield org+proj unique + shared key")

	// Provenance attributes shared key to highest-precedence layer (project)
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p-shared]"],
		"shared key must be attributed to project (highest precedence)")
	assert.Equal(t, storage.ConstitutionLayerOrg,
		result.Provenance["principles[p-org]"])
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p-proj]"])
}
