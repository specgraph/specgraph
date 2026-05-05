// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package mcpconfigs renders and synchronizes per-harness MCP configuration
// files (Cursor .cursor/mcp.json, Claude Code .mcp.json, OpenCode
// opencode.json) from a (slug, serverURL) pair.
//
// The mutation primitive is RFC 7396 JSON Merge Patch (via
// github.com/evanphx/json-patch/v5). Each per-harness builder produces a
// patch document that names only the fields specgraph manages — url,
// Authorization header, X-Specgraph-Project header, and harness-specific
// shape fields like type and enabled. Applying the patch to an existing
// file updates managed keys and preserves all siblings and user
// customizations under the specgraph entry.
package mcpconfigs

import (
	"encoding/json"
	"strings"
)

// mustMarshal marshals v to JSON, panicking if marshaling fails. It is used
// for static map literals whose contents are known to be marshallable.
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("mcpconfigs: unexpected marshal failure: " + err.Error())
	}
	return b
}

// ManagedConfig pairs a project-relative file path with the JSON Merge Patch
// document specgraph manages for that file's specgraph server entry.
type ManagedConfig struct {
	// Path is the file path relative to the project root (e.g. ".cursor/mcp.json").
	Path string

	// Patch is the RFC 7396 JSON Merge Patch document. It contains only
	// fields specgraph manages; applying it preserves siblings and
	// user-added fields.
	Patch json.RawMessage
}

// ManagedConfigs returns the patches for the three currently-supported
// harnesses (Cursor, Claude Code, OpenCode). slug is the project slug from
// .specgraph.yaml; serverURL is the resolved server base URL (without /mcp/
// suffix; the helper appends it).
func ManagedConfigs(slug, serverURL string) []ManagedConfig {
	mcpURL := ensureMCPSuffix(serverURL)
	return []ManagedConfig{
		cursorConfig(slug, mcpURL),
		claudeCodeConfig(slug, mcpURL),
		openCodeConfig(slug, mcpURL),
	}
}

// ensureMCPSuffix returns serverURL with a trailing "/mcp/" segment, leaving
// the URL unchanged if it already ends with that suffix.
func ensureMCPSuffix(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	if strings.HasSuffix(trimmed, "/mcp") {
		return trimmed + "/"
	}
	return trimmed + "/mcp/"
}

// cursorConfig returns the merge patch for .cursor/mcp.json. Cursor uses
// ${env:NAME} env-var substitution.
func cursorConfig(slug, mcpURL string) ManagedConfig {
	return ManagedConfig{
		Path: ".cursor/mcp.json",
		Patch: mustMarshal(map[string]any{
			"mcpServers": map[string]any{
				"specgraph": map[string]any{
					"url": mcpURL,
					"headers": map[string]any{
						"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
						"X-Specgraph-Project": slug,
					},
				},
			},
		}),
	}
}

// claudeCodeConfig returns the merge patch for .mcp.json. Claude Code uses
// ${NAME} env-var substitution and requires "type": "http" for HTTP MCP.
func claudeCodeConfig(slug, mcpURL string) ManagedConfig {
	return ManagedConfig{
		Path: ".mcp.json",
		Patch: mustMarshal(map[string]any{
			"mcpServers": map[string]any{
				"specgraph": map[string]any{
					"type": "http",
					"url":  mcpURL,
					"headers": map[string]any{
						"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
						"X-Specgraph-Project": slug,
					},
				},
			},
		}),
	}
}

// openCodeConfig returns the merge patch for opencode.json. OpenCode uses
// {env:NAME} env-var substitution (no leading $), wraps servers under "mcp"
// (singular), and requires "type": "remote" for HTTP MCP. The top-level
// "$schema" sibling tells OpenCode which schema to validate against.
func openCodeConfig(slug, mcpURL string) ManagedConfig {
	return ManagedConfig{
		Path: "opencode.json",
		Patch: mustMarshal(map[string]any{
			"$schema": "https://opencode.ai/config.json",
			"mcp": map[string]any{
				"specgraph": map[string]any{
					"type":    "remote",
					"url":     mcpURL,
					"enabled": true,
					"headers": map[string]any{
						"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
						"X-Specgraph-Project": slug,
					},
				},
			},
		}),
	}
}
