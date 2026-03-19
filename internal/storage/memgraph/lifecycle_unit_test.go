// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
	roundTripped, err := unmarshalHistory("test-spec", jsonStr)
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

	roundTripped, err := unmarshalHistory("test-spec", jsonStr)
	require.NoError(t, err)
	assert.Len(t, roundTripped, maxHistoryEntries)
	assert.Equal(t, int32(1), roundTripped[0].Version)
}

func TestUnmarshalHistory_InvalidJSON(t *testing.T) {
	_, err := unmarshalHistory("test-spec", "{not valid json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal history_json")
}

func TestUnmarshalHistory_EmptyAndNil(t *testing.T) {
	entries, err := unmarshalHistory("test-spec", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NotNil(t, entries, "should return empty slice, not nil")

	entries, err = unmarshalHistory("test-spec", "[]")
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NotNil(t, entries, "should return empty slice, not nil")
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

	roundTripped, err := unmarshalHistory("test-spec", jsonStr)
	require.NoError(t, err)
	assert.Len(t, roundTripped, maxHistoryEntries)
	// Oldest entry (version 1) should have been trimmed.
	assert.Equal(t, int32(2), roundTripped[0].Version)
	// Newest entry should be present at the end.
	assert.Equal(t, int32(maxHistoryEntries+1), roundTripped[len(roundTripped)-1].Version)
	assert.Equal(t, storage.SpecStageDone, roundTripped[len(roundTripped)-1].Stage)
}

func TestRecordToSpecOffset(t *testing.T) {
	// Build a 34-value record simulating a SupersedeSpec query that returns
	// two specs: old at offset 0, new at offset 17.
	now := "2026-01-15T10:30:00.000000000Z"
	makeSpecValues := func(id, slug, intent, stage, priority, complexity string, version int64, supersededBy, supersedes, contentHash string) []any {
		return []any{
			id, slug, intent, stage, priority, complexity,
			version,  // int64
			now, now, // created_at, updated_at
			"task",       // lifecycle
			supersededBy, // superseded_by
			supersedes,   // supersedes
			"[]",         // history_json (empty array)
			false,        // drift_acknowledged
			nil,          // drift_acknowledge_note
			"",           // notes
			contentHash,  // content_hash
		}
	}

	oldVals := makeSpecValues("id-old", "old-spec", "Original intent", "superseded", "high", "medium", int64(2), "new-spec", "", "abc123def456abc1")
	newVals := makeSpecValues("id-new", "new-spec", "Replacement intent", "spark", "high", "low", int64(1), "", "old-spec", "def789ghi012def7")

	rec := &neo4j.Record{Values: append(oldVals, newVals...)}

	// Parse old spec at offset 0.
	oldSpec, err := recordToSpecOffset(rec, 0)
	require.NoError(t, err)
	assert.Equal(t, "id-old", oldSpec.ID)
	assert.Equal(t, "old-spec", oldSpec.Slug)
	assert.Equal(t, storage.SpecStage("superseded"), oldSpec.Stage)
	assert.Equal(t, int32(2), oldSpec.Version)
	assert.Equal(t, "new-spec", oldSpec.SupersededBy)
	assert.Equal(t, "abc123def456abc1", oldSpec.ContentHash)

	// Parse new spec at offset 17.
	newSpec, err := recordToSpecOffset(rec, 17)
	require.NoError(t, err)
	assert.Equal(t, "id-new", newSpec.ID)
	assert.Equal(t, "new-spec", newSpec.Slug)
	assert.Equal(t, storage.SpecStage("spark"), newSpec.Stage)
	assert.Equal(t, int32(1), newSpec.Version)
	assert.Equal(t, "old-spec", newSpec.Supersedes)
	assert.Equal(t, "def789ghi012def7", newSpec.ContentHash)
}

func TestSortableRFC3339Nano_LexicographicOrdering(t *testing.T) {
	earlier := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	later := time.Date(2026, 3, 10, 12, 0, 1, 0, time.UTC)

	earlierStr := earlier.Format(sortableRFC3339Nano)
	laterStr := later.Format(sortableRFC3339Nano)

	assert.Less(t, earlierStr, laterStr,
		"sortableRFC3339Nano strings should sort chronologically")

	// Mixed format: old RFC3339 (no nanos) vs new sortableRFC3339Nano.
	oldFormatEarlier := earlier.Format(time.RFC3339)
	assert.Less(t, oldFormatEarlier, laterStr,
		"old RFC3339 format should still sort before newer sortableRFC3339Nano")
}

func TestUnmarshalHistory_UnparseableDate(t *testing.T) {
	raw := `[{"version":1,"stage":"amended","summary":"test","reason":"r","date":"not-a-date"}]`
	_, err := unmarshalHistory("test-spec", raw)
	require.Error(t, err, "unparseable date should return error, not silently produce zero timestamp")
	assert.Contains(t, err.Error(), "parse")
}

func TestUnmarshalHistory_UnknownStageAccepted(t *testing.T) {
	raw := `[{"version":1,"stage":"nonexistent_stage","summary":"test","reason":"r","date":"2026-01-01T00:00:00.000000000Z"}]`
	entries, err := unmarshalHistory("test-spec", raw)
	require.NoError(t, err, "unknown stages should be accepted in history (immutable audit log)")
	require.Len(t, entries, 1)
	require.Equal(t, storage.SpecStage("nonexistent_stage"), entries[0].Stage)
}
