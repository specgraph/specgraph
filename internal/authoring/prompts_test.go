// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestGetPrompts(t *testing.T) {
	t.Run("spark prompts exist with expected names", func(t *testing.T) {
		prompts := authoring.GetPrompts("spark")
		require.NotEmpty(t, prompts)

		names := make([]string, len(prompts))
		for i, p := range prompts {
			names[i] = p.Name
		}
		require.Contains(t, names, "seed")
		require.Contains(t, names, "signal")
		require.Contains(t, names, "kill_test")
	})

	t.Run("shape prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("shape"))
	})

	t.Run("specify prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("specify"))
	})

	t.Run("decompose prompts are non-empty", func(t *testing.T) {
		require.NotEmpty(t, authoring.GetPrompts("decompose"))
	})

	t.Run("nonexistent stage returns empty", func(t *testing.T) {
		prompts := authoring.GetPrompts("nonexistent")
		require.Empty(t, prompts)
	})
}
