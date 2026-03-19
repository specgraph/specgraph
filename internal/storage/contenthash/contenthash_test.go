// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package contenthash_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/storage/contenthash"
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
	h1 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	h2 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32)
}

func TestDecisionHash_ChangesOnFieldChange(t *testing.T) {
	base := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	changed := contenthash.Decision("Use Memgraph", "superseded", "We chose Memgraph", "Fast graph queries")
	require.NotEqual(t, base, changed)
}
