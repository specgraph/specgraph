// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDecisionStatus(t *testing.T) {
	tests := []struct {
		raw      string
		expected storage.DecisionStatus
	}{
		{"DECISION_STATUS_PROPOSED", storage.DecisionStatusProposed},
		{"DECISION_STATUS_ACCEPTED", storage.DecisionStatusAccepted},
		{"DECISION_STATUS_SUPERSEDED", storage.DecisionStatusSuperseded},
		{"DECISION_STATUS_DEPRECATED", storage.DecisionStatusDeprecated},
		{"proposed", storage.DecisionStatusProposed},
		{"accepted", storage.DecisionStatusAccepted},
		{"superseded", storage.DecisionStatusSuperseded},
		{"deprecated", storage.DecisionStatusDeprecated},
		{"unknown", storage.DecisionStatus("unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeDecisionStatus(tt.raw))
		})
	}
}

func TestLegacyDecisionStatus(t *testing.T) {
	tests := []struct {
		status storage.DecisionStatus
		legacy string
	}{
		{storage.DecisionStatusProposed, "DECISION_STATUS_PROPOSED"},
		{storage.DecisionStatusAccepted, "DECISION_STATUS_ACCEPTED"},
		{storage.DecisionStatusSuperseded, "DECISION_STATUS_SUPERSEDED"},
		{storage.DecisionStatusDeprecated, "DECISION_STATUS_DEPRECATED"},
		{storage.DecisionStatus("unknown"), ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.legacy, legacyDecisionStatus(tt.status))
		})
	}
}

// --- marshalRejectedAlts / unmarshalRejectedAlts ---

func TestMarshalRejectedAlts_Empty(t *testing.T) {
	assert.Equal(t, "[]", marshalRejectedAlts(nil))
	assert.Equal(t, "[]", marshalRejectedAlts([]storage.RejectedAlternative{}))
}

func TestMarshalRejectedAlts_Single(t *testing.T) {
	alts := []storage.RejectedAlternative{{Option: "MySQL", Reason: "No graph"}}
	result := marshalRejectedAlts(alts)
	assert.Contains(t, result, `"option":"MySQL"`)
	assert.Contains(t, result, `"reason":"No graph"`)
}

func TestMarshalRejectedAlts_Multiple(t *testing.T) {
	alts := []storage.RejectedAlternative{
		{Option: "MySQL", Reason: "No graph"},
		{Option: "Neo4j", Reason: "Too expensive"},
	}
	result := marshalRejectedAlts(alts)
	assert.Contains(t, result, `"option":"MySQL"`)
	assert.Contains(t, result, `"option":"Neo4j"`)
}

func TestUnmarshalRejectedAlts_Empty(t *testing.T) {
	result, err := unmarshalRejectedAlts("")
	require.NoError(t, err)
	assert.Nil(t, result)

	result, err = unmarshalRejectedAlts("[]")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalRejectedAlts_Valid(t *testing.T) {
	raw := `[{"option":"MySQL","reason":"No graph"},{"option":"Neo4j","reason":"Too expensive"}]`
	result, err := unmarshalRejectedAlts(raw)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "MySQL", result[0].Option)
	assert.Equal(t, "No graph", result[0].Reason)
	assert.Equal(t, "Neo4j", result[1].Option)
	assert.Equal(t, "Too expensive", result[1].Reason)
}

func TestUnmarshalRejectedAlts_InvalidJSON(t *testing.T) {
	_, err := unmarshalRejectedAlts("{not valid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal rejected alternatives")
}

func TestMarshalUnmarshalRejectedAlts_RoundTrip(t *testing.T) {
	alts := []storage.RejectedAlternative{
		{Option: "A", Reason: "reason A"},
		{Option: "B", Reason: "reason B"},
	}
	raw := marshalRejectedAlts(alts)
	got, err := unmarshalRejectedAlts(raw)
	require.NoError(t, err)
	assert.Equal(t, alts, got)
}

// --- marshalTags / unmarshalTags ---

func TestMarshalTags_Empty(t *testing.T) {
	assert.Equal(t, "[]", marshalTags(nil))
	assert.Equal(t, "[]", marshalTags([]string{}))
}

func TestMarshalTags_Values(t *testing.T) {
	result := marshalTags([]string{"auth", "security"})
	assert.Equal(t, `["auth","security"]`, result)
}

func TestUnmarshalTags_Empty(t *testing.T) {
	result, err := unmarshalTags("")
	require.NoError(t, err)
	assert.Nil(t, result)

	result, err = unmarshalTags("[]")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalTags_Valid(t *testing.T) {
	result, err := unmarshalTags(`["auth","security","api"]`)
	require.NoError(t, err)
	assert.Equal(t, []string{"auth", "security", "api"}, result)
}

func TestUnmarshalTags_InvalidJSON(t *testing.T) {
	_, err := unmarshalTags("{bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal tags")
}

func TestMarshalUnmarshalTags_RoundTrip(t *testing.T) {
	tags := []string{"infra", "backend", "go"}
	raw := marshalTags(tags)
	got, err := unmarshalTags(raw)
	require.NoError(t, err)
	assert.Equal(t, tags, got)
}

// --- recordInt64Optional ---

func TestRecordInt64Optional_ValidValue(t *testing.T) {
	rec := &neo4j.Record{Values: []any{int64(42)}}
	v, err := recordInt64Optional(rec, 0, "version")
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)
}

func TestRecordInt64Optional_Nil(t *testing.T) {
	rec := &neo4j.Record{Values: []any{nil}}
	v, err := recordInt64Optional(rec, 0, "version")
	require.NoError(t, err)
	assert.Equal(t, int64(0), v)
}

func TestRecordInt64Optional_OutOfBounds(t *testing.T) {
	rec := &neo4j.Record{Values: []any{}}
	v, err := recordInt64Optional(rec, 5, "version")
	require.NoError(t, err)
	assert.Equal(t, int64(0), v)
}

func TestRecordInt64Optional_WrongType(t *testing.T) {
	rec := &neo4j.Record{Values: []any{"not-an-int"}}
	_, err := recordInt64Optional(rec, 0, "version")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected int64 or nil")
}

// --- toContentHashAlts ---

func TestToContentHashAlts_Nil(t *testing.T) {
	assert.Nil(t, toContentHashAlts(nil))
}

func TestToContentHashAlts_Values(t *testing.T) {
	alts := []storage.RejectedAlternative{
		{Option: "A", Reason: "R1"},
		{Option: "B", Reason: "R2"},
	}
	result := toContentHashAlts(alts)
	require.Len(t, result, 2)
	assert.Equal(t, "A", result[0].Option)
	assert.Equal(t, "R1", result[0].Reason)
	assert.Equal(t, "B", result[1].Option)
	assert.Equal(t, "R2", result[1].Reason)
}
