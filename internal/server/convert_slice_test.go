// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSliceToProto(t *testing.T) {
	now := time.Date(2026, 3, 28, 14, 0, 0, 0, time.UTC)
	s := &storage.Slice{
		ID:         "slice-1",
		Slug:       "login/s1",
		ParentSlug: "login",
		SliceID:    "s1",
		Intent:     "auth endpoint",
		Verify:     []string{"login returns 200"},
		Touches:    []string{"auth.go"},
		DependsOn:  []string{},
		Status:     storage.SliceStatusOpen,
		AssignedTo: "",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	pb, err := sliceToProto(s)
	require.NoError(t, err)
	assert.Equal(t, "slice-1", pb.Id)
	assert.Equal(t, "login/s1", pb.Slug)
	assert.Equal(t, "login", pb.ParentSlug)
	assert.Equal(t, "s1", pb.SliceId)
	assert.Equal(t, "auth endpoint", pb.Intent)
	assert.Equal(t, []string{"login returns 200"}, pb.Verify)
	assert.Equal(t, []string{"auth.go"}, pb.Touches)
	assert.Empty(t, pb.DependsOn)
	assert.Equal(t, specv1.SliceStatus_SLICE_STATUS_OPEN, pb.Status)
	assert.Empty(t, pb.AssignedTo)
	assert.Equal(t, now.Unix(), pb.CreatedAt.AsTime().Unix())
}

func TestSliceStatusToProto(t *testing.T) {
	tests := []struct {
		domain storage.SliceStatus
		proto  specv1.SliceStatus
	}{
		{storage.SliceStatusOpen, specv1.SliceStatus_SLICE_STATUS_OPEN},
		{storage.SliceStatusClaimed, specv1.SliceStatus_SLICE_STATUS_CLAIMED},
		{storage.SliceStatusDone, specv1.SliceStatus_SLICE_STATUS_DONE},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			got, err := sliceStatusToProto(tt.domain)
			require.NoError(t, err)
			assert.Equal(t, tt.proto, got)
		})
	}

	_, err := sliceStatusToProto("bogus")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown slice status")
}

func TestSlicesToProto(t *testing.T) {
	slices := []*storage.Slice{
		{ID: "a", Slug: "s/a", Status: storage.SliceStatusOpen},
		{ID: "b", Slug: "s/b", Status: storage.SliceStatusDone},
	}
	pbs, err := slicesToProto(slices)
	require.NoError(t, err)
	require.Len(t, pbs, 2)
	assert.Equal(t, "s/a", pbs[0].Slug)
	assert.Equal(t, "s/b", pbs[1].Slug)
}

func TestSlicesToProto_InvalidStatus(t *testing.T) {
	slices := []*storage.Slice{
		{ID: "bad", Slug: "s/bad", Status: "invalid"},
	}
	_, err := slicesToProto(slices)
	assert.Error(t, err)
}
