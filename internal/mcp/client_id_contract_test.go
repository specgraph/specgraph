// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// TestClientIDContract documents the expected clientInfo.name each platform
// reports during MCP initialize. Update when docs/verification/*.md captures
// empirical findings; regressing any of these without updating the test
// indicates a platform renamed itself and the profile mapping is stale.
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
		{"Polecat", "polecat", ProfileExecution},
		{"Gastown", "gastown", ProfileExecution},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			got := ProfileFromClientInfo(&sdkmcp.Implementation{Name: tc.name})
			if got != tc.profile {
				t.Errorf("%s reports %q → want %v, got %v (did the platform rename? See docs/verification/)",
					tc.platform, tc.name, tc.profile, got)
			}
		})
	}
}
