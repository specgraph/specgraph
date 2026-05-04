// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// ProfileFromClientInfo determines the appropriate profile for an MCP client
// based on its reported Implementation name. Execution engines (polecat,
// gastown) get full access; authoring IDEs get authoring + core; everything
// else gets core only.
func ProfileFromClientInfo(info *sdkmcp.Implementation) Profile {
	if info == nil {
		return ProfileCore
	}
	switch info.Name {
	case "polecat", "gastown":
		return ProfileExecution
	case "claude-code", "cursor", "cursor-vscode", "windsurf", "opencode", "codex", "specgraph-cli":
		return ProfileAuthoring
	default:
		return ProfileCore
	}
}
