// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package emitter_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/emitter"
	"github.com/stretchr/testify/require"
)

func testConstitution() *specv1.Constitution {
	return &specv1.Constitution{
		Name: "test-project",
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
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
		Principles: []*specv1.Principle{
			{
				Id:        "backward-compat",
				Principle: "All API changes must be backward compatible",
				Rationale: "External consumers",
			},
		},
		Constraints: []string{
			"No ORMs",
			"All secrets via Secret Manager",
		},
		Antipatterns: []*specv1.Antipattern{
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
	_, _, err := emitter.Emit(c, "invalid")
	require.Error(t, err)
}
