// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestChangeLogEntryToProto(t *testing.T) {
	entry := &storage.ChangeLogEntry{
		ID:          "cl-1",
		Version:     3,
		Stage:       string(storage.SpecStageShape),
		ContentHash: "a1b2c3d4",
		Checkpoint:  true,
		Summary:     "Refined scope",
		Reason:      "Feedback from review",
		Changes: []storage.FieldChange{
			{Field: "intent", OldValue: "Build X", NewValue: "Build X (v2)"},
			{Field: "stage", OldValue: "spark", NewValue: "shape"},
		},
		Date: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
	}

	pb := changeLogEntryToProto(entry)

	assert.Equal(t, "cl-1", pb.Id)
	assert.Equal(t, int32(3), pb.Version)
	assert.Equal(t, "shape", pb.Stage)
	assert.Equal(t, "a1b2c3d4", pb.ContentHash)
	assert.True(t, pb.Checkpoint)
	assert.Equal(t, "Refined scope", pb.Summary)
	assert.Equal(t, "Feedback from review", pb.Reason)
	require.Len(t, pb.Changes, 2)
	assert.Equal(t, "intent", pb.Changes[0].Field)
	assert.Equal(t, "Build X", pb.Changes[0].OldValue)
	assert.Equal(t, "Build X (v2)", pb.Changes[0].NewValue)
	assert.Equal(t, int64(2026), int64(pb.Date.AsTime().Year()))
}

func TestChangeLogEntryToProto_NoChanges(t *testing.T) {
	entry := &storage.ChangeLogEntry{
		ID:      "cl-2",
		Version: 1,
		Stage:   string(storage.SpecStageSpark),
		Date:    time.Now(),
	}

	pb := changeLogEntryToProto(entry)

	assert.Equal(t, "cl-2", pb.Id)
	assert.Empty(t, pb.Changes)
}

func TestChangeLogEntriesToProto_Empty(t *testing.T) {
	pbs := changeLogEntriesToProto(nil)
	assert.Empty(t, pbs)
}
