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

func strPtr(s string) *string { return &s }

func TestGetSpecAtVersion(t *testing.T) {
	t.Run("GetCurrentVersion_ReturnsCurrentSpec", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		created, err := store.CreateSpec(ctx, "sv-current", "initial intent", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		result, err := store.GetSpecAtVersion(ctx, "sv-current", created.Version)
		require.NoError(t, err)
		assert.Equal(t, "initial intent", result.Intent)
		assert.Equal(t, created.Version, result.Version)
	})

	t.Run("AfterUpdate_GetV1_ReturnsOriginalIntent", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "sv-history", "original intent", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.UpdateSpec(ctx, "sv-history", strPtr("updated intent"), nil, nil, nil, nil)
		require.NoError(t, err)

		// v1 should have the original intent
		v1, err := store.GetSpecAtVersion(ctx, "sv-history", 1)
		require.NoError(t, err)
		assert.Equal(t, "original intent", v1.Intent)
		assert.Equal(t, int32(1), v1.Version)

		// v2 should have the updated intent
		v2, err := store.GetSpecAtVersion(ctx, "sv-history", 2)
		require.NoError(t, err)
		assert.Equal(t, "updated intent", v2.Intent)
		assert.Equal(t, int32(2), v2.Version)
	})

	t.Run("VersionZero_ReturnsLatest", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "sv-zero", "some intent", "p2", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.UpdateSpec(ctx, "sv-zero", strPtr("newer intent"), nil, nil, nil, nil)
		require.NoError(t, err)

		result, err := store.GetSpecAtVersion(ctx, "sv-zero", 0)
		require.NoError(t, err)
		assert.Equal(t, "newer intent", result.Intent)
		assert.Equal(t, int32(2), result.Version)
	})

	t.Run("NonexistentSpec_ReturnsErrSpecNotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.GetSpecAtVersion(ctx, "does-not-exist", 1)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("VersionTooHigh_ReturnsErrVersionNotFound", func(t *testing.T) {
		store := newStore(t)
		clearDatabase(t, store)
		ctx := context.Background()

		_, err := store.CreateSpec(ctx, "sv-toohigh", "intent", "p1", "medium", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.GetSpecAtVersion(ctx, "sv-toohigh", 999)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrVersionNotFound)
	})
}
