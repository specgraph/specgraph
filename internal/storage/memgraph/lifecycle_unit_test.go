// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordToSpecOffset(t *testing.T) {
	// Build a 36-value record simulating a SupersedeSpec query that returns
	// two specs: old at offset 0, new at offset 18.
	now := "2026-01-15T10:30:00.000000000Z"
	makeSpecValues := func(id, slug, intent, stage, priority, complexity string, version int64, supersededBy, supersedes, contentHash string) []any {
		return []any{
			id, slug, intent, stage, priority, complexity,
			version,  // int64
			now, now, // created_at, updated_at
			"task",         // lifecycle
			supersededBy,   // superseded_by
			supersedes,     // supersedes
			"",             // notes
			contentHash,    // content_hash
			"", "", "", "", // spark_output, shape_output, specify_output, decompose_output
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

	// Parse new spec at offset 18.
	newSpec, err := recordToSpecOffset(rec, 18)
	require.NoError(t, err)
	assert.Equal(t, "id-new", newSpec.ID)
	assert.Equal(t, "new-spec", newSpec.Slug)
	assert.Equal(t, storage.SpecStage("spark"), newSpec.Stage)
	assert.Equal(t, int32(1), newSpec.Version)
	assert.Equal(t, "old-spec", newSpec.Supersedes)
	assert.Equal(t, "def789ghi012def7", newSpec.ContentHash)
}

func TestRecordToSpecOffset_WithConversationCount(t *testing.T) {
	now := "2026-01-15T10:30:00.000000000Z"
	values := []any{
		"id-1", "slug-1", "intent", "spark", "p1", "medium",
		int64(1),   // version
		now, now,   // created_at, updated_at
		"task",     // lifecycle
		"", "",     // superseded_by, supersedes
		"",         // notes
		"abc12345", // content_hash
		"", "", "", "", // spark_output, shape_output, specify_output, decompose_output
		int64(7), // conversation_count
	}
	keys := []string{
		"s.id", "s.slug", "s.intent", "s.stage", "s.priority", "s.complexity",
		"s.version", "s.created_at", "s.updated_at",
		"s.lifecycle", "s.superseded_by", "s.supersedes",
		"s.notes", "s.content_hash",
		"s.spark_output", "s.shape_output", "s.specify_output", "s.decompose_output",
		"conversation_count",
	}
	rec := &neo4j.Record{Values: values, Keys: keys}

	spec, err := recordToSpecOffset(rec, 0)
	require.NoError(t, err)
	assert.Equal(t, "slug-1", spec.Slug)
	assert.Equal(t, 7, spec.ConversationCount)
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
