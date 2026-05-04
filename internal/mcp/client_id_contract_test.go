// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// TestClientIDContract documents the expected clientInfo.name each platform
// reports during MCP initialize and the profile each name maps to. A failing
// test indicates a platform renamed itself; update the case list here AND
// update ProfileFromClientInfo in profiles.go to match.
func TestClientIDContract(t *testing.T) {
	cases := []struct {
		platform string
		name     string
		profile  Profile
	}{
		{"Claude Code", "claude-code", ProfileAuthoring},
		{"Cursor", "cursor", ProfileAuthoring},
		{"OpenCode", "opencode", ProfileAuthoring},
		{"Codex", "codex", ProfileAuthoring},
		{"Windsurf", "windsurf", ProfileAuthoring},
		{"SpecGraph CLI", "specgraph-cli", ProfileAuthoring},
		{"Polecat", "polecat", ProfileExecution},
		{"Gastown", "gastown", ProfileExecution},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			got := ProfileFromClientInfo(&sdkmcp.Implementation{Name: tc.name})
			if got != tc.profile {
				t.Errorf("%s reports clientInfo.name=%q → want profile %v, got %v (platform may have renamed; update ProfileFromClientInfo and this test)",
					tc.platform, tc.name, tc.profile, got)
			}
		})
	}
}
