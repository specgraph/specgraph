// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// TierFromClientInfo determines the appropriate tier for an MCP client based
// on its reported Implementation name. Execution engines (polecat, gastown)
// get full access; authoring IDEs get authoring + core; everything else gets
// core only.
func TierFromClientInfo(info *sdkmcp.Implementation) Tier {
	if info == nil {
		return TierCore
	}
	switch info.Name {
	case "polecat", "gastown":
		return TierExecution
	case "claude-code", "cursor", "windsurf":
		return TierAuthoring
	default:
		return TierCore
	}
}
