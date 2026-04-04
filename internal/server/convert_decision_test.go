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

func TestDecisionConfidenceToProto(t *testing.T) {
	tests := []struct {
		domain storage.DecisionConfidence
		want   specv1.DecisionConfidence
	}{
		{storage.DecisionConfidenceHigh, specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH},
		{storage.DecisionConfidenceMedium, specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM},
		{storage.DecisionConfidenceLow, specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			assert.Equal(t, tt.want, decisionConfidenceToProto(tt.domain))
		})
	}

	t.Run("empty returns UNSPECIFIED", func(t *testing.T) {
		assert.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED, decisionConfidenceToProto(""))
	})
	t.Run("unknown returns UNSPECIFIED", func(t *testing.T) {
		assert.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED, decisionConfidenceToProto("extreme"))
	})
}

func TestDecisionConfidenceFromProto(t *testing.T) {
	tests := []struct {
		proto specv1.DecisionConfidence
		want  storage.DecisionConfidence
	}{
		{specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH, storage.DecisionConfidenceHigh},
		{specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM, storage.DecisionConfidenceMedium},
		{specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW, storage.DecisionConfidenceLow},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			assert.Equal(t, tt.want, decisionConfidenceFromProto(tt.proto))
		})
	}

	t.Run("UNSPECIFIED returns empty", func(t *testing.T) {
		assert.Equal(t, storage.DecisionConfidence(""), decisionConfidenceFromProto(specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED))
	})
}

func TestDecisionConfidenceRoundTrip(t *testing.T) {
	values := []storage.DecisionConfidence{
		storage.DecisionConfidenceHigh,
		storage.DecisionConfidenceMedium,
		storage.DecisionConfidenceLow,
	}
	for _, v := range values {
		got := decisionConfidenceFromProto(decisionConfidenceToProto(v))
		assert.Equal(t, v, got, "round-trip failed for %q", v)
	}
}

func TestDecisionScopeToProto(t *testing.T) {
	tests := []struct {
		domain storage.DecisionScope
		want   specv1.DecisionScope
	}{
		{storage.DecisionScopeProject, specv1.DecisionScope_DECISION_SCOPE_PROJECT},
		{storage.DecisionScopeTeam, specv1.DecisionScope_DECISION_SCOPE_TEAM},
		{storage.DecisionScopeOrg, specv1.DecisionScope_DECISION_SCOPE_ORG},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			assert.Equal(t, tt.want, decisionScopeToProto(tt.domain))
		})
	}

	t.Run("empty returns UNSPECIFIED", func(t *testing.T) {
		assert.Equal(t, specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED, decisionScopeToProto(""))
	})
	t.Run("unknown returns UNSPECIFIED", func(t *testing.T) {
		assert.Equal(t, specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED, decisionScopeToProto("global"))
	})
}

func TestDecisionScopeFromProto(t *testing.T) {
	tests := []struct {
		proto specv1.DecisionScope
		want  storage.DecisionScope
	}{
		{specv1.DecisionScope_DECISION_SCOPE_PROJECT, storage.DecisionScopeProject},
		{specv1.DecisionScope_DECISION_SCOPE_TEAM, storage.DecisionScopeTeam},
		{specv1.DecisionScope_DECISION_SCOPE_ORG, storage.DecisionScopeOrg},
	}
	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			assert.Equal(t, tt.want, decisionScopeFromProto(tt.proto))
		})
	}

	t.Run("UNSPECIFIED returns empty", func(t *testing.T) {
		assert.Equal(t, storage.DecisionScope(""), decisionScopeFromProto(specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED))
	})
}

func TestDecisionScopeRoundTrip(t *testing.T) {
	values := []storage.DecisionScope{
		storage.DecisionScopeProject,
		storage.DecisionScopeTeam,
		storage.DecisionScopeOrg,
	}
	for _, v := range values {
		got := decisionScopeFromProto(decisionScopeToProto(v))
		assert.Equal(t, v, got, "round-trip failed for %q", v)
	}
}

func TestRejectedAltsToProto(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, rejectedAltsToProto(nil))
	})
	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, rejectedAltsToProto([]storage.RejectedAlternative{}))
	})
	t.Run("populated", func(t *testing.T) {
		alts := []storage.RejectedAlternative{
			{Option: "PostgreSQL", Reason: "Not graph-native"},
			{Option: "Neo4j", Reason: "Too expensive"},
		}
		pbs := rejectedAltsToProto(alts)
		require.Len(t, pbs, 2)
		assert.Equal(t, "PostgreSQL", pbs[0].Option)
		assert.Equal(t, "Not graph-native", pbs[0].Reason)
		assert.Equal(t, "Neo4j", pbs[1].Option)
		assert.Equal(t, "Too expensive", pbs[1].Reason)
	})
}

func TestRejectedAltsFromProto(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, rejectedAltsFromProto(nil))
	})
	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, rejectedAltsFromProto([]*specv1.RejectedAlternative{}))
	})
	t.Run("populated", func(t *testing.T) {
		pbs := []*specv1.RejectedAlternative{
			{Option: "PostgreSQL", Reason: "Not graph-native"},
		}
		alts := rejectedAltsFromProto(pbs)
		require.Len(t, alts, 1)
		assert.Equal(t, "PostgreSQL", alts[0].Option)
		assert.Equal(t, "Not graph-native", alts[0].Reason)
	})
}

func TestRejectedAltsRoundTrip(t *testing.T) {
	original := []storage.RejectedAlternative{
		{Option: "Option A", Reason: "Too slow"},
		{Option: "Option B", Reason: "Too complex"},
	}
	result := rejectedAltsFromProto(rejectedAltsToProto(original))
	assert.Equal(t, original, result)
}

func TestDecisionToProto_AllNewFields(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	d := &storage.Decision{
		ID:        "dec-full",
		Slug:      "use-memgraph",
		Title:     "Use Memgraph",
		Status:    storage.DecisionStatusAccepted,
		Body:      "We chose Memgraph",
		Rationale: "Graph-native queries",
		Question:  "Which database to use?",
		RejectedAlternatives: []storage.RejectedAlternative{
			{Option: "PostgreSQL", Reason: "Not graph-native"},
		},
		Confidence:  storage.DecisionConfidenceHigh,
		Tags:        []string{"database", "infrastructure"},
		Scope:       storage.DecisionScopeProject,
		OriginSpec:  "storage-design",
		OriginStage: "specify",
		Version:     3,
		ContentHash: "abc123",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	pb, err := decisionToProto(d)
	require.NoError(t, err)

	assert.Equal(t, "dec-full", pb.Id)
	assert.Equal(t, "use-memgraph", pb.Slug)
	assert.Equal(t, "Use Memgraph", pb.Title)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, pb.Status)
	assert.Equal(t, "We chose Memgraph", pb.Decision)
	assert.Equal(t, "Graph-native queries", pb.Rationale)
	assert.Equal(t, "Which database to use?", pb.Question)
	require.Len(t, pb.RejectedAlternatives, 1)
	assert.Equal(t, "PostgreSQL", pb.RejectedAlternatives[0].Option)
	assert.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH, pb.Confidence)
	assert.Equal(t, []string{"database", "infrastructure"}, pb.Tags)
	assert.Equal(t, specv1.DecisionScope_DECISION_SCOPE_PROJECT, pb.Scope)
	assert.Equal(t, "storage-design", pb.OriginSpec)
	assert.Equal(t, "specify", pb.OriginStage)
	assert.Equal(t, int32(3), pb.Version)
	assert.Equal(t, "abc123", pb.ContentHash)
}

func TestDecisionToProto_EmptyNewFields(t *testing.T) {
	d := &storage.Decision{
		ID:     "dec-empty",
		Slug:   "minimal",
		Title:  "Minimal",
		Status: storage.DecisionStatusProposed,
	}
	pb, err := decisionToProto(d)
	require.NoError(t, err)

	assert.Equal(t, "", pb.Question)
	assert.Nil(t, pb.RejectedAlternatives)
	assert.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED, pb.Confidence)
	assert.Nil(t, pb.Tags)
	assert.Equal(t, specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED, pb.Scope)
	assert.Equal(t, "", pb.OriginSpec)
	assert.Equal(t, "", pb.OriginStage)
	assert.Equal(t, int32(0), pb.Version)
}
