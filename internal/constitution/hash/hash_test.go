// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package hash_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/hash"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestHash_Deterministic(t *testing.T) {
	c := &storage.Constitution{
		Name:  "test",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "Prefer explicit"},
		},
	}
	h1, err := hash.Hash(c)
	require.NoError(t, err)
	h2, err := hash.Hash(c)
	require.NoError(t, err)
	assert.Equal(t, h1, h2, "hash must be stable across calls")
}

func TestHash_DifferentContent_DifferentHash(t *testing.T) {
	c1 := &storage.Constitution{
		Name: "a",
		Principles: []storage.Principle{{ID: "p1", Statement: "S1"}},
	}
	c2 := &storage.Constitution{
		Name: "a",
		Principles: []storage.Principle{{ID: "p1", Statement: "S2"}},
	}
	h1, _ := hash.Hash(c1)
	h2, _ := hash.Hash(c2)
	assert.NotEqual(t, h1, h2, "different content produces different hash")
}

func TestHash_ListOrderMatters(t *testing.T) {
	c1 := &storage.Constitution{
		Principles: []storage.Principle{
			{ID: "p1", Statement: "S1"},
			{ID: "p2", Statement: "S2"},
		},
	}
	c2 := &storage.Constitution{
		Principles: []storage.Principle{
			{ID: "p2", Statement: "S2"},
			{ID: "p1", Statement: "S1"},
		},
	}
	h1, _ := hash.Hash(c1)
	h2, _ := hash.Hash(c2)
	assert.NotEqual(t, h1, h2,
		"list reordering produces different hash (intentional)")
}

func TestHash_EmptyStruct_Stable(t *testing.T) {
	c := &storage.Constitution{Name: "empty"}
	h1, err := hash.Hash(c)
	require.NoError(t, err)
	h2, err := hash.Hash(c)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
}

func TestHash_HexLength(t *testing.T) {
	c := &storage.Constitution{Name: "x"}
	h, err := hash.Hash(c)
	require.NoError(t, err)
	assert.Len(t, h, 32, "Murmur3-128 hex must be 32 chars")
}

// TestHash_FixedExpected guards against Go-version and encoding/json
// behavior shifts. The sentinel must always hash to the same value.
// On first run, capture the computed hash and pin it below.
func TestHash_FixedExpected(t *testing.T) {
	sentinel := &storage.Constitution{
		Name:  "sentinel",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "fixed"},
		},
		Constraints: []string{"never use eval"},
	}
	h, err := hash.Hash(sentinel)
	require.NoError(t, err)

	const expected = "3586b5e1e08a894b620887aea4c67f65"
	assert.Equal(t, expected, h,
		"regression: encoding/json or constitution struct shape changed")
}
