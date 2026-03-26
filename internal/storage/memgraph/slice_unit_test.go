// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestSliceStubs_ReturnNotImplemented(t *testing.T) {
	s := &Store{}
	ctx := context.Background()

	assert.Error(t, s.CreateSlice(ctx, &storage.Slice{}))

	_, err := s.ListSlices(ctx, "any")
	assert.Error(t, err)

	_, err = s.GetSlice(ctx, "any")
	assert.Error(t, err)

	assert.Error(t, s.ClaimSlice(ctx, "any", "alice"))
	assert.Error(t, s.CompleteSlice(ctx, "any"))
}
