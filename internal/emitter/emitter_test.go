// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package emitter_test

import (
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/emitter"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func testConstitution() *storage.Constitution {
	return &storage.Constitution{
		Name: "test-project",
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary:   "go",
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"java"},
				ForbiddenReasons: map[string]string{
					"java": "No Java expertise",
				},
			},
			Frameworks: map[string]string{
				"api":     "ConnectRPC",
				"testing": "testify",
			},
			Infrastructure: map[string]string{
				"runtime": "Docker",
				"ci":      "GitHub Actions",
			},
		},
		Principles: []storage.Principle{
			{
				ID:        "backward-compat",
				Statement: "All API changes must be backward compatible",
				Rationale: "External consumers",
			},
		},
		Constraints: []string{
			"No ORMs",
			"All secrets via Secret Manager",
		},
		Antipatterns: []storage.Antipattern{
			{
				Pattern: "Shared mutable state",
				Why:     "Caused cascading failure",
				Instead: "Event-driven",
			},
		},
	}
}

func TestEmit_ClaudeMD(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "claude-md")
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", filename)
	require.Contains(t, content, "go")
	require.Contains(t, content, "ConnectRPC")
	require.Contains(t, content, "No ORMs")
	require.Contains(t, content, "backward compatible")
	require.Contains(t, content, "Shared mutable state")
}

func TestEmit_Cursorrules(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "cursorrules")
	require.NoError(t, err)
	require.Equal(t, ".cursorrules", filename)
	require.Contains(t, content, "go")
}

func TestEmit_AgentsMD(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "agents-md")
	require.NoError(t, err)
	require.Equal(t, "AGENTS.md", filename)
	require.Contains(t, content, "go")
}

func TestEmit_InvalidFormat(t *testing.T) {
	c := testConstitution()
	_, _, err := emitter.Emit(c, "invalid-format")
	require.Error(t, err)
}

func TestEmit_NilConstitution(t *testing.T) {
	formats := []string{
		"claude-md",
		"cursorrules",
		"agents-md",
	}
	for _, format := range formats {
		content, _, err := emitter.Emit(nil, format)
		require.NoError(t, err, "format %s should not error with nil constitution", format)
		require.NotEmpty(t, content, "format %s should return header even for nil constitution", format)
	}
}

func TestEmit_EmptyConstitution(t *testing.T) {
	c := &storage.Constitution{}
	content, filename, err := emitter.Emit(c, "claude-md")
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", filename)
	require.True(t, strings.HasPrefix(content, "# Project Constitution"), "should start with header")
	require.NotContains(t, content, "## Tech Stack")
	require.NotContains(t, content, "## Principles")
	require.NotContains(t, content, "## Constraints")
	require.NotContains(t, content, "## Anti-patterns")
}

func TestEmit_PartialConstitution_TechOnly(t *testing.T) {
	c := &storage.Constitution{
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary: "go",
			},
		},
	}
	content, _, err := emitter.Emit(c, "claude-md")
	require.NoError(t, err)
	require.Contains(t, content, "## Tech Stack")
	require.Contains(t, content, "go")
	require.NotContains(t, content, "## Principles")
	require.NotContains(t, content, "## Constraints")
	require.NotContains(t, content, "## Anti-patterns")
}

func TestEmit_NilTech(t *testing.T) {
	c := &storage.Constitution{
		Tech:        nil,
		Constraints: []string{"No ORMs"},
	}
	content, _, err := emitter.Emit(c, "claude-md")
	require.NoError(t, err)
	require.NotContains(t, content, "## Tech Stack")
	require.Contains(t, content, "## Constraints")
	require.Contains(t, content, "No ORMs")
}

func TestEmit_MapOrdering(t *testing.T) {
	// Run multiple times to catch nondeterministic map iteration.
	c := testConstitution()
	var prev string
	for i := 0; i < 20; i++ {
		content, _, err := emitter.Emit(c, "claude-md")
		require.NoError(t, err)
		if prev != "" {
			require.Equal(t, prev, content, "output must be deterministic across runs")
		}
		prev = content
	}
}
