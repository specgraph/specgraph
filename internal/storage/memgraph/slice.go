// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// CreateSlice persists a new Slice node. Full implementation in spgr-6sw.2.
func (s *Store) CreateSlice(_ context.Context, _ *storage.Slice) error {
	return fmt.Errorf("memgraph: CreateSlice not yet implemented")
}

// ListSlices returns slices for a parent spec. Full implementation in spgr-6sw.2.
func (s *Store) ListSlices(_ context.Context, _ string) ([]*storage.Slice, error) {
	return nil, fmt.Errorf("memgraph: ListSlices not yet implemented")
}

// GetSlice returns a single slice by slug. Full implementation in spgr-6sw.2.
func (s *Store) GetSlice(_ context.Context, _ string) (*storage.Slice, error) {
	return nil, fmt.Errorf("memgraph: GetSlice not yet implemented")
}

// ClaimSlice transitions a slice to claimed. Full implementation in spgr-6sw.2.
func (s *Store) ClaimSlice(_ context.Context, _, _ string) error {
	return fmt.Errorf("memgraph: ClaimSlice not yet implemented")
}

// CompleteSlice transitions a slice to done. Full implementation in spgr-6sw.2.
func (s *Store) CompleteSlice(_ context.Context, _ string) error {
	return fmt.Errorf("memgraph: CompleteSlice not yet implemented")
}
