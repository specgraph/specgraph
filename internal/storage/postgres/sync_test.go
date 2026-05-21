// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestCreateSyncMapping(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "sync-spec", "Sync intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	m, err := store.CreateSyncMapping(ctx, "sync-spec", storage.SyncAdapterBeads, "bead-42")
	require.NoError(t, err)
	require.Equal(t, "sync-spec", m.SpecSlug)
	require.Equal(t, storage.SyncAdapterBeads, m.Adapter)
	require.Equal(t, "bead-42", m.ExternalID)
	require.Equal(t, storage.SyncStateSynced, m.State)
	require.False(t, m.CreatedAt.IsZero())
}

func TestCreateSyncMapping_Duplicate(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "sync-dup", "Dup", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "sync-dup", storage.SyncAdapterBeads, "ext-1")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "sync-dup", storage.SyncAdapterBeads, "ext-2")
	require.ErrorIs(t, err, storage.ErrSyncMappingExists)
}

func TestUpdateSyncState(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "sync-update", "Update", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "sync-update", storage.SyncAdapterGitHub, "gh-99")
	require.NoError(t, err)

	updated, err := store.UpdateSyncState(ctx, "sync-update", storage.SyncAdapterGitHub, storage.SyncStateError, "timeout")
	require.NoError(t, err)
	require.Equal(t, storage.SyncStateError, updated.State)
	require.Equal(t, "timeout", updated.ErrorMessage)
}

func TestGetSyncMapping_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetSyncMapping(ctx, "no-spec", storage.SyncAdapterBeads)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}

func TestListSyncMappings_Filter(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "list-sync-a", "A", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "list-sync-b", "B", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "list-sync-a", storage.SyncAdapterBeads, "bead-a")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-sync-b", storage.SyncAdapterBeads, "bead-b")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-sync-a", storage.SyncAdapterGitHub, "gh-a")
	require.NoError(t, err)

	// Filter by adapter beads — two results.
	beads, err := store.ListSyncMappings(ctx, storage.SyncAdapterBeads, "")
	require.NoError(t, err)
	require.Len(t, beads, 2)

	// Filter by spec slug — two results (beads + github for list-sync-a).
	specA, err := store.ListSyncMappings(ctx, "", "list-sync-a")
	require.NoError(t, err)
	require.Len(t, specA, 2)

	// Filter by both — one result.
	one, err := store.ListSyncMappings(ctx, storage.SyncAdapterGitHub, "list-sync-a")
	require.NoError(t, err)
	require.Len(t, one, 1)
	require.Equal(t, "gh-a", one[0].ExternalID)

	// No filter — all three.
	all, err := store.ListSyncMappings(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, all, 3)
}

func TestDeleteSyncMapping_Idempotent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "del-sync", "Del", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "del-sync", storage.SyncAdapterBeads, "bead-del")
	require.NoError(t, err)

	// First delete.
	require.NoError(t, store.DeleteSyncMapping(ctx, "del-sync", storage.SyncAdapterBeads))

	// Second delete — idempotent, no error.
	require.NoError(t, store.DeleteSyncMapping(ctx, "del-sync", storage.SyncAdapterBeads))

	// Mapping gone.
	_, err = store.GetSyncMapping(ctx, "del-sync", storage.SyncAdapterBeads)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}
