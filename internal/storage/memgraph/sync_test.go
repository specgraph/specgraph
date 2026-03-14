// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestSync_CreateMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec first
	_, err = store.CreateSpec(ctx, "sync-test-spec", "Test spec for sync", "p2", "medium")
	require.NoError(t, err)

	// Create sync mapping
	mapping, err := store.CreateSyncMapping(ctx, "sync-test-spec", storage.SyncAdapterGitHub, "github-issue-42")
	require.NoError(t, err)
	require.Equal(t, "sync-test-spec", mapping.SpecSlug)
	require.Equal(t, storage.SyncAdapterGitHub, mapping.Adapter)
	require.Equal(t, "github-issue-42", mapping.ExternalID)
	require.Equal(t, storage.SyncStateSynced, mapping.State)
	require.NotEmpty(t, mapping.SpecID)
	require.False(t, mapping.LastSync.IsZero())
	require.False(t, mapping.CreatedAt.IsZero())
}

func TestSync_CreateMappingDuplicate(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "dup-spec", "Test spec", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "dup-spec", storage.SyncAdapterBeads, "beads-abc123")
	require.NoError(t, err)

	// Duplicate should fail
	_, err = store.CreateSyncMapping(ctx, "dup-spec", storage.SyncAdapterBeads, "beads-xyz789")
	require.ErrorIs(t, err, storage.ErrSyncMappingExists)
}

func TestSync_CreateMappingSpecNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSyncMapping(ctx, "nonexistent", storage.SyncAdapterGitHub, "gh-1")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSync_UpdateState(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "state-spec", "Test spec", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "state-spec", storage.SyncAdapterGitHub, "gh-99")
	require.NoError(t, err)

	// Update state to error
	updated, err := store.UpdateSyncState(ctx, "state-spec", storage.SyncAdapterGitHub, storage.SyncStateError, "rate limit exceeded")
	require.NoError(t, err)
	require.Equal(t, storage.SyncStateError, updated.State)
	require.Equal(t, "rate limit exceeded", updated.ErrorMessage)

	// Update state back to synced
	updated, err = store.UpdateSyncState(ctx, "state-spec", storage.SyncAdapterGitHub, storage.SyncStateSynced, "")
	require.NoError(t, err)
	require.Equal(t, storage.SyncStateSynced, updated.State)
	require.Empty(t, updated.ErrorMessage)
}

func TestSync_UpdateStateNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.UpdateSyncState(ctx, "no-spec", storage.SyncAdapterGitHub, storage.SyncStateSynced, "")
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}

func TestSync_GetMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "get-spec", "Test spec", "p1", "high")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "get-spec", storage.SyncAdapterBeads, "beads-get123")
	require.NoError(t, err)

	got, err := store.GetSyncMapping(ctx, "get-spec", storage.SyncAdapterBeads)
	require.NoError(t, err)
	require.Equal(t, "beads-get123", got.ExternalID)

	// Not found
	_, err = store.GetSyncMapping(ctx, "get-spec", storage.SyncAdapterGitHub)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}

func TestSync_ListMappings(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "list-a", "Spec A", "p2", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "list-b", "Spec B", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "list-a", storage.SyncAdapterGitHub, "gh-1")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-b", storage.SyncAdapterGitHub, "gh-2")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-a", storage.SyncAdapterBeads, "beads-1")
	require.NoError(t, err)

	// List all — empty adapter and empty slug means no filter
	all, err := store.ListSyncMappings(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Filter by adapter
	ghOnly, err := store.ListSyncMappings(ctx, storage.SyncAdapterGitHub, "")
	require.NoError(t, err)
	require.Len(t, ghOnly, 2)

	// Filter by spec slug
	specA, err := store.ListSyncMappings(ctx, "", "list-a")
	require.NoError(t, err)
	require.Len(t, specA, 2)

	// Filter by both
	specific, err := store.ListSyncMappings(ctx, storage.SyncAdapterBeads, "list-a")
	require.NoError(t, err)
	require.Len(t, specific, 1)
	require.Equal(t, "beads-1", specific[0].ExternalID)
}

func TestSync_DeleteMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "del-spec", "Spec to delete sync", "p2", "low")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "del-spec", storage.SyncAdapterGitHub, "gh-del")
	require.NoError(t, err)

	err = store.DeleteSyncMapping(ctx, "del-spec", storage.SyncAdapterGitHub)
	require.NoError(t, err)

	_, err = store.GetSyncMapping(ctx, "del-spec", storage.SyncAdapterGitHub)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)

	// Delete nonexistent — should not error (idempotent)
	err = store.DeleteSyncMapping(ctx, "del-spec", storage.SyncAdapterGitHub)
	require.NoError(t, err)
}

func TestSync_DeleteMappingCleansUpExternalRef(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "cleanup-spec", "Spec for cleanup test", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "cleanup-spec", storage.SyncAdapterGitHub, "gh-cleanup-42")
	require.NoError(t, err)

	// Verify ExternalRef exists before deletion
	driver, dErr := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
	require.NoError(t, dErr)
	defer driver.Close(ctx)

	session := driver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	result, qErr := session.Run(ctx,
		`MATCH (e:ExternalRef {external_id: $eid, adapter: $adapter}) RETURN count(e) AS cnt`,
		map[string]any{"eid": "gh-cleanup-42", "adapter": string(storage.SyncAdapterGitHub)})
	require.NoError(t, qErr)
	rec, rErr := result.Single(ctx)
	require.NoError(t, rErr)
	cnt, _ := rec.Get("cnt")
	require.Equal(t, int64(1), cnt.(int64), "ExternalRef should exist before deletion") //nolint:forcetypeassert // test assertion

	// Delete the mapping
	err = store.DeleteSyncMapping(ctx, "cleanup-spec", storage.SyncAdapterGitHub)
	require.NoError(t, err)

	// Verify ExternalRef was cleaned up
	result2, qErr2 := session.Run(ctx,
		`MATCH (e:ExternalRef {external_id: $eid, adapter: $adapter}) RETURN count(e) AS cnt`,
		map[string]any{"eid": "gh-cleanup-42", "adapter": string(storage.SyncAdapterGitHub)})
	require.NoError(t, qErr2)
	rec2, rErr2 := result2.Single(ctx)
	require.NoError(t, rErr2)
	cnt2, _ := rec2.Get("cnt")
	require.Equal(t, int64(0), cnt2.(int64), "ExternalRef should be cleaned up after deletion") //nolint:forcetypeassert // test assertion
}
