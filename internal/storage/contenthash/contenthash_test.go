// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package contenthash_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage/contenthash"
	"github.com/stretchr/testify/require"
)

func TestSpecHash_Deterministic(t *testing.T) {
	h1 := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	h2 := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32) // 128 bits = 32 hex chars
}

func TestSpecHash_ChangesOnFieldChange(t *testing.T) {
	base := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	changed := contenthash.Spec("implement login", "shape", "p1", "medium", nil)
	require.NotEqual(t, base, changed)
}

func TestSpecHash_IncludesAuthoringOutputs(t *testing.T) {
	noOutputs := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	withOutputs := contenthash.Spec("implement login", "spark", "p1", "medium",
		map[string]string{"spark_output": `{"seed":"test"}`})
	require.NotEqual(t, noOutputs, withOutputs)
}

func TestDecisionHash_Deterministic(t *testing.T) {
	h1 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries", "", "", "", "", "", nil, nil)
	h2 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries", "", "", "", "", "", nil, nil)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32)
}

func TestDecisionHash_ChangesOnFieldChange(t *testing.T) {
	base := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries", "", "", "", "", "", nil, nil)
	changed := contenthash.Decision("Use Memgraph", "superseded", "We chose Memgraph", "Fast graph queries", "", "", "", "", "", nil, nil)
	require.NotEqual(t, base, changed)
}

func TestDecision_NewFieldsChangeHash(t *testing.T) {
	base := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", nil, nil)

	withQuestion := contenthash.Decision("T", "accepted", "D", "R", "Why?", "", "", "", "", nil, nil)
	require.NotEqual(t, base, withQuestion)

	withConfidence := contenthash.Decision("T", "accepted", "D", "R", "", "high", "", "", "", nil, nil)
	require.NotEqual(t, base, withConfidence)

	withScope := contenthash.Decision("T", "accepted", "D", "R", "", "", "project", "", "", nil, nil)
	require.NotEqual(t, base, withScope)

	withTags := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", []string{"go"}, nil)
	require.NotEqual(t, base, withTags)

	withAlts := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", nil,
		[]contenthash.RejectedAlt{{Option: "PostgreSQL", Reason: "No graph"}})
	require.NotEqual(t, base, withAlts)
}

func TestDecision_TagsSortedForDeterminism(t *testing.T) {
	h1 := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", []string{"b", "a", "c"}, nil)
	h2 := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", []string{"c", "a", "b"}, nil)
	require.Equal(t, h1, h2)
}

func TestDecision_RejectedAltsSortedForDeterminism(t *testing.T) {
	alts1 := []contenthash.RejectedAlt{
		{Option: "PostgreSQL", Reason: "No graph support"},
		{Option: "ArangoDB", Reason: "Licensing"},
	}
	alts2 := []contenthash.RejectedAlt{
		{Option: "ArangoDB", Reason: "Licensing"},
		{Option: "PostgreSQL", Reason: "No graph support"},
	}
	h1 := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", nil, alts1)
	h2 := contenthash.Decision("T", "accepted", "D", "R", "", "", "", "", "", nil, alts2)
	require.Equal(t, h1, h2)
}
