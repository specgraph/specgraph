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

// ToolsForProfile returns all tools visible at the given profile.
// Higher profiles include all tools from lower profiles.
func (r *Registry) ToolsForProfile(profile Profile) []ToolDef {
	var out []ToolDef
	for _, def := range r.tools {
		if profile.Includes(def.Profile) {
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
func (r *Registry) AddResource(def ResourceDef) { //nolint:gocritic // value receiver keeps API simple
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
