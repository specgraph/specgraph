// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalHistory_TrimsOldEntries(t *testing.T) {
	entries := make([]storage.HistoryEntry, maxHistoryEntries+10)
	for i := range entries {
		entries[i] = storage.HistoryEntry{
			Version: int32(i + 1),
			Stage:   storage.SpecStageDone,
			Summary: "entry",
			Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	jsonStr, err := marshalHistory(entries)
	require.NoError(t, err)

	// Unmarshal to verify only the last maxHistoryEntries remain.
	roundTripped, err := unmarshalHistory(jsonStr)
	require.NoError(t, err)
	assert.Len(t, roundTripped, maxHistoryEntries)
	// First entry should be #11 (oldest 10 trimmed).
	assert.Equal(t, int32(11), roundTripped[0].Version)
	// Last entry should be #110.
	assert.Equal(t, int32(maxHistoryEntries+10), roundTripped[len(roundTripped)-1].Version)
}

func TestMarshalHistory_ExactLimit(t *testing.T) {
	entries := make([]storage.HistoryEntry, maxHistoryEntries)
	for i := range entries {
		entries[i] = storage.HistoryEntry{
			Version: int32(i + 1),
			Stage:   storage.SpecStageSpark,
			Summary: "entry",
			Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	jsonStr, err := marshalHistory(entries)
	require.NoError(t, err)

	roundTripped, err := unmarshalHistory(jsonStr)
	require.NoError(t, err)
	assert.Len(t, roundTripped, maxHistoryEntries)
	assert.Equal(t, int32(1), roundTripped[0].Version)
}

func TestUnmarshalHistory_InvalidJSON(t *testing.T) {
	_, err := unmarshalHistory("{not valid json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal history_json")
}

func TestUnmarshalHistory_EmptyAndNil(t *testing.T) {
	entries, err := unmarshalHistory("")
	require.NoError(t, err)
	assert.Nil(t, entries)

	entries, err = unmarshalHistory("[]")
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestAppendHistory_TrimsOldestWhenFull(t *testing.T) {
	existing := make([]storage.HistoryEntry, maxHistoryEntries)
	for i := range existing {
		existing[i] = storage.HistoryEntry{
			Version: int32(i + 1),
			Stage:   storage.SpecStageSpark,
			Summary: "old entry",
			Date:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	newest := &storage.HistoryEntry{
		Version: int32(maxHistoryEntries + 1),
		Stage:   storage.SpecStageDone,
		Summary: "newest entry",
		Date:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	jsonStr, err := appendHistory(existing, newest)
	require.NoError(t, err)

	roundTripped, err := unmarshalHistory(jsonStr)
	require.NoError(t, err)
	assert.Len(t, roundTripped, maxHistoryEntries)
	// Oldest entry (version 1) should have been trimmed.
	assert.Equal(t, int32(2), roundTripped[0].Version)
	// Newest entry should be present at the end.
	assert.Equal(t, int32(maxHistoryEntries+1), roundTripped[len(roundTripped)-1].Version)
	assert.Equal(t, storage.SpecStageDone, roundTripped[len(roundTripped)-1].Stage)
}

func TestUnmarshalHistory_UnknownStageReturnsError(t *testing.T) {
	raw := `[{"version":1,"stage":"nonexistent_stage","summary":"test","reason":"r","date":"2026-01-01T00:00:00.000000000Z"}]`
	_, err := unmarshalHistory(raw)
	require.Error(t, err, "unknown stage should return an error")
	require.Contains(t, err.Error(), "nonexistent_stage")
}
