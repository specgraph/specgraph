// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

// Registry holds all MCP tool, resource, and prompt definitions.
type Registry struct {
	tools     []ToolDef
	toolIndex map[string]int // name → index in tools
	resources []ResourceDef
	prompts   []PromptDef
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		toolIndex: make(map[string]int),
	}
}

// AddTool registers a tool definition.
func (r *Registry) AddTool(def ToolDef) {
	r.toolIndex[def.Name] = len(r.tools)
	r.tools = append(r.tools, def)
}

// ToolsForTier returns all tools visible at the given tier.
// Higher tiers include all tools from lower tiers.
func (r *Registry) ToolsForTier(tier Tier) []ToolDef {
	var out []ToolDef
	for _, def := range r.tools {
		if tier.Includes(def.Tier) {
			out = append(out, def)
		}
	}
	return out
}

// LookupTool finds a tool by name.
func (r *Registry) LookupTool(name string) (ToolDef, bool) {
	idx, ok := r.toolIndex[name]
	if !ok {
		return ToolDef{}, false
	}
	return r.tools[idx], true
}

// AddResource registers a resource definition.
func (r *Registry) AddResource(def ResourceDef) {
	r.resources = append(r.resources, def)
}

// Resources returns all registered resource definitions.
func (r *Registry) Resources() []ResourceDef {
	return r.resources
}

// AddPrompt registers a prompt definition.
func (r *Registry) AddPrompt(def PromptDef) {
	r.prompts = append(r.prompts, def)
}

// Prompts returns all registered prompt definitions.
func (r *Registry) Prompts() []PromptDef {
	return r.prompts
}
