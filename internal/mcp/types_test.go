// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierCore, "core"},
		{TierAuthoring, "authoring"},
		{TierExecution, "execution"},
		// Unknown values fall back to "core" per the current implementation.
		{Tier(99), "core"},
		{Tier(-1), "core"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Tier(%d)", tt.tier), func(t *testing.T) {
			require.Equal(t, tt.want, tt.tier.String())
		})
	}
}

func TestParseTier(t *testing.T) {
	tests := []struct {
		input string
		want  Tier
	}{
		{"core", TierCore},
		{"authoring", TierAuthoring},
		{"execution", TierExecution},
		// Unknown strings default to TierCore.
		{"unknown", TierCore},
		{"", TierCore},
		{"CORE", TierCore},
		{"Authoring", TierCore},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("ParseTier(%q)", tt.input), func(t *testing.T) {
			require.Equal(t, tt.want, ParseTier(tt.input))
		})
	}
}

func TestTierIncludes(t *testing.T) {
	tests := []struct {
		t     Tier
		other Tier
		want  bool
	}{
		// Core includes only core.
		{TierCore, TierCore, true},
		{TierCore, TierAuthoring, false},
		{TierCore, TierExecution, false},
		// Authoring includes core and authoring.
		{TierAuthoring, TierCore, true},
		{TierAuthoring, TierAuthoring, true},
		{TierAuthoring, TierExecution, false},
		// Execution includes all tiers.
		{TierExecution, TierCore, true},
		{TierExecution, TierAuthoring, true},
		{TierExecution, TierExecution, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Tier(%d).Includes(Tier(%d))", tt.t, tt.other), func(t *testing.T) {
			require.Equal(t, tt.want, tt.t.Includes(tt.other))
		})
	}
}
