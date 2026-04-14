// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides a Model Context Protocol server that translates
// MCP tool calls into ConnectRPC RPCs against a running SpecGraph server.
package mcp

import "context"

// Tier controls which tools are visible to an MCP client.
type Tier int

const (
	// TierCore exposes read-heavy tools: specs, graph queries, constitution.
	TierCore Tier = iota
	// TierAuthoring adds authoring funnel, decisions, drift, analytical passes.
	TierAuthoring
	// TierExecution adds claims, slices, progress reporting, bundles.
	TierExecution
)

// String returns the tier name used in capability negotiation.
func (t Tier) String() string {
	switch t {
	case TierCore:
		return "core"
	case TierAuthoring:
		return "authoring"
	case TierExecution:
		return "execution"
	default:
		return "core"
	}
}

// ParseTier converts a string to a Tier. Returns TierCore for unknown values.
func ParseTier(s string) Tier {
	switch s {
	case "authoring":
		return TierAuthoring
	case "execution":
		return TierExecution
	default:
		return TierCore
	}
}

// Includes reports whether tier t includes all tools visible at tier other.
func (t Tier) Includes(other Tier) bool {
	return t >= other
}

// ToolDef defines an MCP tool in SpecGraph's own types (no SDK dependency).
type ToolDef struct {
	Name        string
	Description string
	Tier        Tier
	Schema      map[string]any // JSON Schema for parameters
	Handler     ToolHandler
}

// ToolHandler processes an MCP tool call. params is the deserialized arguments map.
type ToolHandler func(ctx context.Context, params map[string]any) (*ToolResult, error)

// ToolResult is the response from a tool handler.
type ToolResult struct {
	Content []Content
	IsError bool
}

// Content is a single content block in a tool result.
type Content struct {
	Type string // "text"
	Text string
}

// ResourceDef defines an MCP resource.
type ResourceDef struct {
	URI         string // Exact URI or template pattern
	Name        string
	Description string
	MimeType    string
	IsTemplate  bool
	Handler     ResourceHandler
}

// ResourceHandler reads a resource. uri is the resolved URI.
type ResourceHandler func(ctx context.Context, uri string) ([]ResourceContent, error)

// ResourceContent is a single content block in a resource response.
type ResourceContent struct {
	URI      string
	MimeType string
	Text     string
}

// PromptDef defines an MCP prompt.
type PromptDef struct {
	Name        string
	Description string
	Arguments   []PromptArgument
	Handler     PromptHandler
}

// PromptArgument describes a prompt parameter.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptHandler renders a prompt. args is the argument map.
type PromptHandler func(ctx context.Context, args map[string]string) (*PromptResult, error)

// PromptResult is the response from a prompt handler.
type PromptResult struct {
	Description string
	Messages    []PromptMessage
}

// PromptMessage is a single message in a prompt result.
type PromptMessage struct {
	Role    string // "user" or "assistant"
	Content string
}
