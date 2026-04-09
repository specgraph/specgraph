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

func TestCreateSlice(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "parent-spec", "Parent intent", "", "")
	require.NoError(t, err)

	sl := &storage.Slice{
		Slug:       "parent-spec/sl-1",
		ParentSlug: "parent-spec",
		SliceID:    "sl-1",
		Intent:     "Do something useful",
		Verify:     []string{"tests pass", "lint clean"},
		Touches:    []string{"internal/foo/bar.go"},
		DependsOn:  []string{},
	}
	err = store.CreateSlice(ctx, sl)
	require.NoError(t, err)

	got, err := store.GetSlice(ctx, "parent-spec/sl-1")
	require.NoError(t, err)
	require.Equal(t, "parent-spec/sl-1", got.Slug)
	require.Equal(t, "parent-spec", got.ParentSlug)
	require.Equal(t, "sl-1", got.SliceID)
	require.Equal(t, "Do something useful", got.Intent)
	require.Equal(t, storage.SliceStatusOpen, got.Status)
	require.Equal(t, []string{"tests pass", "lint clean"}, got.Verify)
	require.Equal(t, []string{"internal/foo/bar.go"}, got.Touches)
	require.Empty(t, got.AssignedTo)
}

func TestListSlices(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "list-parent", "Parent", "", "")
	require.NoError(t, err)

	for _, id := range []string{"sl-a", "sl-b", "sl-c"} {
		sl := &storage.Slice{
			Slug:       "list-parent/" + id,
			ParentSlug: "list-parent",
			SliceID:    id,
			Intent:     "intent " + id,
		}
		require.NoError(t, store.CreateSlice(ctx, sl))
	}

	slices, err := store.ListSlices(ctx, "list-parent")
	require.NoError(t, err)
	require.Len(t, slices, 3)
}

func TestGetSlice_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetSlice(ctx, "no-such/slice")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)
}

func TestClaimSlice(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "claim-parent", "Parent", "", "")
	require.NoError(t, err)

	sl := &storage.Slice{
		Slug:       "claim-parent/sl-claim",
		ParentSlug: "claim-parent",
		SliceID:    "sl-claim",
		Intent:     "claim me",
	}
	require.NoError(t, store.CreateSlice(ctx, sl))

	claimed, err := store.ClaimSlice(ctx, "claim-parent/sl-claim", "agent-007")
	require.NoError(t, err)
	require.Equal(t, storage.SliceStatusClaimed, claimed.Status)
	require.Equal(t, "agent-007", claimed.AssignedTo)

	// Claiming again should return ErrSliceWrongStatus.
	_, err = store.ClaimSlice(ctx, "claim-parent/sl-claim", "agent-008")
	require.ErrorIs(t, err, storage.ErrSliceWrongStatus)
}

func TestCompleteSlice(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "complete-parent", "Parent", "", "")
	require.NoError(t, err)

	sl := &storage.Slice{
		Slug:       "complete-parent/sl-done",
		ParentSlug: "complete-parent",
		SliceID:    "sl-done",
		Intent:     "complete me",
	}
	require.NoError(t, store.CreateSlice(ctx, sl))

	// Must claim before completing.
	_, err = store.CompleteSlice(ctx, "complete-parent/sl-done")
	require.ErrorIs(t, err, storage.ErrSliceWrongStatus)

	_, err = store.ClaimSlice(ctx, "complete-parent/sl-done", "agent-1")
	require.NoError(t, err)

	done, err := store.CompleteSlice(ctx, "complete-parent/sl-done")
	require.NoError(t, err)
	require.Equal(t, storage.SliceStatusDone, done.Status)
}

func TestGetSlice_NotFound_ExplicitCheck(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetSlice(ctx, "definitely/missing")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)
}

func TestClaimSlice_NotFound_ReturnsNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.ClaimSlice(ctx, "no-parent/no-slice", "agent-x")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)
}

func TestCompleteSlice_NotFound_ReturnsNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CompleteSlice(ctx, "no-parent/no-slice")
	require.ErrorIs(t, err, storage.ErrSliceNotFound)
}

func TestClaimSlice_AlreadyDone_ReturnsWrongStatus(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "done-parent", "Parent", "", "")
	require.NoError(t, err)

	sl := &storage.Slice{
		Slug:       "done-parent/sl-done2",
		ParentSlug: "done-parent",
		SliceID:    "sl-done2",
		Intent:     "will be done",
	}
	require.NoError(t, store.CreateSlice(ctx, sl))

	// Claim, then complete.
	_, err = store.ClaimSlice(ctx, "done-parent/sl-done2", "agent-1")
	require.NoError(t, err)
	_, err = store.CompleteSlice(ctx, "done-parent/sl-done2")
	require.NoError(t, err)

	// Trying to claim a done slice should return ErrSliceWrongStatus.
	_, err = store.ClaimSlice(ctx, "done-parent/sl-done2", "agent-2")
	require.ErrorIs(t, err, storage.ErrSliceWrongStatus)
}
