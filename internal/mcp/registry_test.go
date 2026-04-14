// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_ToolsFilteredByTier(t *testing.T) {
	r := NewRegistry()
	noop := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return textResult("ok"), nil
	}

	r.AddTool(ToolDef{Name: "spec", Tier: TierCore, Handler: noop})
	r.AddTool(ToolDef{Name: "author", Tier: TierAuthoring, Handler: noop})
	r.AddTool(ToolDef{Name: "claim", Tier: TierExecution, Handler: noop})

	core := r.ToolsForTier(TierCore)
	require.Len(t, core, 1)
	require.Equal(t, "spec", core[0].Name)

	authoring := r.ToolsForTier(TierAuthoring)
	require.Len(t, authoring, 2) // core + authoring
	names := []string{authoring[0].Name, authoring[1].Name}
	require.ElementsMatch(t, []string{"spec", "author"}, names)

	execution := r.ToolsForTier(TierExecution)
	require.Len(t, execution, 3)
}

func TestRegistry_LookupTool(t *testing.T) {
	r := NewRegistry()
	noop := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return textResult("ok"), nil
	}
	r.AddTool(ToolDef{Name: "spec", Tier: TierCore, Handler: noop})

	def, ok := r.LookupTool("spec")
	require.True(t, ok)
	require.Equal(t, "spec", def.Name)

	_, ok = r.LookupTool("missing")
	require.False(t, ok)
}

func TestRegistry_Resources(t *testing.T) {
	r := NewRegistry()
	r.AddResource(ResourceDef{URI: "specgraph://specs", Name: "specs"})
	require.Len(t, r.Resources(), 1)
}

func TestRegistry_Prompts(t *testing.T) {
	r := NewRegistry()
	r.AddPrompt(PromptDef{Name: "spark"})
	require.Len(t, r.Prompts(), 1)
}
