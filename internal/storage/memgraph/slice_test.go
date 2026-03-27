// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndListSlices(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create parent spec.
	_, err = store.CreateSpec(ctx, "parent-spec", "Parent intent", "p2", "medium")
	require.NoError(t, err)

	// Create two slices.
	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "parent-spec/backend",
		ParentSlug: "parent-spec",
		SliceID:    "backend",
		Intent:     "Implement backend API",
		Verify:     []string{"API tests pass"},
		Touches:    []string{"internal/server/"},
	})
	require.NoError(t, err)

	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "parent-spec/frontend",
		ParentSlug: "parent-spec",
		SliceID:    "frontend",
		Intent:     "Implement frontend UI",
		Verify:     []string{"UI renders"},
		Touches:    []string{"web/src/"},
	})
	require.NoError(t, err)

	// List slices.
	slices, err := store.ListSlices(ctx, "parent-spec")
	require.NoError(t, err)
	require.Len(t, slices, 2)

	assert.Equal(t, "parent-spec/backend", slices[0].Slug)
	assert.Equal(t, "backend", slices[0].SliceID)
	assert.Equal(t, "Implement backend API", slices[0].Intent)
	assert.Equal(t, storage.SliceStatusOpen, slices[0].Status)
	assert.Equal(t, []string{"API tests pass"}, slices[0].Verify)
	assert.Equal(t, []string{"internal/server/"}, slices[0].Touches)

	assert.Equal(t, "parent-spec/frontend", slices[1].Slug)
}

func TestGetSlice(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "get-slice-parent", "Parent", "p2", "medium")
	require.NoError(t, err)

	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "get-slice-parent/api",
		ParentSlug: "get-slice-parent",
		SliceID:    "api",
		Intent:     "Build the API",
		Verify:     []string{"test passes"},
	})
	require.NoError(t, err)

	sl, err := store.GetSlice(ctx, "get-slice-parent/api")
	require.NoError(t, err)
	assert.Equal(t, "get-slice-parent/api", sl.Slug)
	assert.Equal(t, "get-slice-parent", sl.ParentSlug)
	assert.Equal(t, "api", sl.SliceID)
	assert.Equal(t, "Build the API", sl.Intent)
	assert.Equal(t, storage.SliceStatusOpen, sl.Status)
	assert.NotZero(t, sl.CreatedAt)
	assert.NotZero(t, sl.UpdatedAt)
}

func TestGetSlice_NotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetSlice(ctx, "nonexistent/slice")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)
}

func TestClaimSlice(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "claim-parent", "Parent", "p2", "medium")
	require.NoError(t, err)

	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "claim-parent/work",
		ParentSlug: "claim-parent",
		SliceID:    "work",
		Intent:     "Do the work",
	})
	require.NoError(t, err)

	// Claim it.
	err = store.ClaimSlice(ctx, "claim-parent/work", "alice")
	require.NoError(t, err)

	// Verify status changed.
	sl, err := store.GetSlice(ctx, "claim-parent/work")
	require.NoError(t, err)
	assert.Equal(t, storage.SliceStatusClaimed, sl.Status)
	assert.Equal(t, "alice", sl.AssignedTo)

	// Claiming again should fail (not open) with sentinel error.
	err = store.ClaimSlice(ctx, "claim-parent/work", "bob")
	require.ErrorIs(t, err, storage.ErrSliceWrongStatus)
}

func TestCompleteSlice(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "complete-parent", "Parent", "p2", "medium")
	require.NoError(t, err)

	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "complete-parent/task",
		ParentSlug: "complete-parent",
		SliceID:    "task",
		Intent:     "A task",
	})
	require.NoError(t, err)

	// Must claim before completing — should return sentinel error.
	err = store.CompleteSlice(ctx, "complete-parent/task")
	require.ErrorIs(t, err, storage.ErrSliceWrongStatus)

	// Claim then complete.
	err = store.ClaimSlice(ctx, "complete-parent/task", "alice")
	require.NoError(t, err)

	err = store.CompleteSlice(ctx, "complete-parent/task")
	require.NoError(t, err)

	sl, err := store.GetSlice(ctx, "complete-parent/task")
	require.NoError(t, err)
	assert.Equal(t, storage.SliceStatusDone, sl.Status)
}

func TestCreateSlice_ParentNotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	err = store.CreateSlice(ctx, &storage.Slice{
		Slug:       "ghost/slice",
		ParentSlug: "ghost",
		SliceID:    "slice",
		Intent:     "orphan",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
