// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
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
