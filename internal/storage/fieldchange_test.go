// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestComputeFieldDeltas_NoChanges(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	updated := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(&old, &updated)
	assert.Empty(t, deltas)
}

func TestComputeFieldDeltas_IntentChanged(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	updated := storage.SpecFields{Intent: "OAuth2 login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(&old, &updated)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "intent", deltas[0].Field)
	assert.Equal(t, "login", deltas[0].OldValue)
	assert.Equal(t, "OAuth2 login", deltas[0].NewValue)
}

func TestComputeFieldDeltas_MultipleChanges(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "low"}
	updated := storage.SpecFields{Intent: "OAuth2 login", Stage: "shape", Priority: "p1", Complexity: "low"}
	deltas := storage.ComputeFieldDeltas(&old, &updated)
	assert.Len(t, deltas, 3)
	fields := make(map[string]bool)
	for _, d := range deltas {
		fields[d.Field] = true
	}
	assert.True(t, fields["intent"])
	assert.True(t, fields["stage"])
	assert.True(t, fields["priority"])
}

func TestComputeFieldDeltas_AuthoringOutputChanged(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark"}
	updated := storage.SpecFields{Intent: "login", Stage: "spark", SparkOutput: `{"goals":["fast"]}`}
	deltas := storage.ComputeFieldDeltas(&old, &updated)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "spark_output", deltas[0].Field)
	assert.Equal(t, "", deltas[0].OldValue)
	assert.Equal(t, `{"goals":["fast"]}`, deltas[0].NewValue)
}

func TestComputeFieldDeltas_StageTransitionOnly(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	updated := storage.SpecFields{Intent: "login", Stage: "shape", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(&old, &updated)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "stage", deltas[0].Field)
}

func TestComputeDecisionFieldDeltas_NoChanges(t *testing.T) {
	f := &storage.DecisionFields{
		Title: "Use Memgraph", Status: "accepted", Body: "We chose Memgraph",
		Rationale: "Graph-native", Question: "Which DB?", Confidence: "high",
		Scope: "project", Tags: `["db"]`, RejectedAlternatives: `[{"option":"PG","reason":"nope"}]`,
		OriginSpec: "storage", OriginStage: "specify",
	}
	deltas := storage.ComputeDecisionFieldDeltas(f, f)
	assert.Empty(t, deltas)
}

func TestComputeDecisionFieldDeltas_TitleChanged(t *testing.T) {
	old := &storage.DecisionFields{Title: "Old Title", Status: "proposed"}
	updated := &storage.DecisionFields{Title: "New Title", Status: "proposed"}
	deltas := storage.ComputeDecisionFieldDeltas(old, updated)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "title", deltas[0].Field)
	assert.Equal(t, "Old Title", deltas[0].OldValue)
	assert.Equal(t, "New Title", deltas[0].NewValue)
}

func TestComputeDecisionFieldDeltas_MultipleChanges(t *testing.T) {
	old := &storage.DecisionFields{
		Title: "Old", Status: "proposed", Body: "body", Confidence: "low", Scope: "project",
	}
	updated := &storage.DecisionFields{
		Title: "New", Status: "accepted", Body: "updated body", Confidence: "high", Scope: "team",
	}
	deltas := storage.ComputeDecisionFieldDeltas(old, updated)
	assert.Len(t, deltas, 5)
	fields := make(map[string]bool)
	for _, d := range deltas {
		fields[d.Field] = true
	}
	assert.True(t, fields["title"])
	assert.True(t, fields["status"])
	assert.True(t, fields["body"])
	assert.True(t, fields["confidence"])
	assert.True(t, fields["scope"])
}

func TestComputeDecisionFieldDeltas_AllFields(t *testing.T) {
	old := &storage.DecisionFields{}
	updated := &storage.DecisionFields{
		Title: "T", Status: "accepted", Body: "B", Rationale: "R",
		Question: "Q", Confidence: "high", Scope: "org",
		Tags: `["a"]`, RejectedAlternatives: `[{"option":"x","reason":"y"}]`,
		OriginSpec: "spec-1", OriginStage: "shape",
	}
	deltas := storage.ComputeDecisionFieldDeltas(old, updated)
	assert.Len(t, deltas, 11)
}

func TestComputeDecisionFieldDeltas_TagsChanged(t *testing.T) {
	old := &storage.DecisionFields{Tags: `["a","b"]`}
	updated := &storage.DecisionFields{Tags: `["a","c"]`}
	deltas := storage.ComputeDecisionFieldDeltas(old, updated)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "tags", deltas[0].Field)
}
