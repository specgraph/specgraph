// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/memgraph"
)

// faultingSliceOps injects errors into SliceBackend operations for testing
// StoreDecomposeOutput error paths against a real database.
type faultingSliceOps struct {
	getSliceErr    error
	createSliceErr error
}

func (f *faultingSliceOps) CreateSlice(context.Context, *storage.Slice) error {
	return f.createSliceErr
}

func (f *faultingSliceOps) GetSlice(context.Context, string) (*storage.Slice, error) {
	if f.getSliceErr != nil {
		return nil, f.getSliceErr
	}
	// Default: slice not found (triggers CreateSlice path).
	return nil, storage.ErrSliceNotFound
}

func (f *faultingSliceOps) ListSlices(context.Context, string) ([]*storage.Slice, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *faultingSliceOps) ClaimSlice(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}

func (f *faultingSliceOps) CompleteSlice(context.Context, string) error {
	return fmt.Errorf("not implemented")
}

func TestStoreDecomposeOutput_GetSliceError(t *testing.T) {
	clearDatabase(t)
	ctx := context.Background()

	store, err := newStore(ctx, boltURI, memgraph.WithSliceOps(&faultingSliceOps{
		getSliceErr: fmt.Errorf("connection reset"),
	}))
	require.NoError(t, err)
	defer store.Close(ctx) //nolint:errcheck

	_, err = store.CreateSpec(ctx, "fault-parent", "Fault test", "p1", "medium")
	require.NoError(t, err)

	_, err = store.StoreDecomposeOutput(ctx, "fault-parent", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "a", Intent: "test slice"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check slice")
	require.Contains(t, err.Error(), "connection reset")
}

func TestStoreDecomposeOutput_CreateSliceError(t *testing.T) {
	clearDatabase(t)
	ctx := context.Background()

	store, err := newStore(ctx, boltURI, memgraph.WithSliceOps(&faultingSliceOps{
		createSliceErr: fmt.Errorf("disk full"),
	}))
	require.NoError(t, err)
	defer store.Close(ctx) //nolint:errcheck

	_, err = store.CreateSpec(ctx, "fault-parent-2", "Fault test 2", "p1", "medium")
	require.NoError(t, err)

	_, err = store.StoreDecomposeOutput(ctx, "fault-parent-2", &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "a", Intent: "test slice"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create slice")
	require.Contains(t, err.Error(), "disk full")
}
