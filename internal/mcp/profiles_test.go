// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestProfileFromClientInfo_Nil(t *testing.T) {
	got := ProfileFromClientInfo(nil)
	if got != ProfileCore {
		t.Errorf("ProfileFromClientInfo(nil) = %v, want %v", got, ProfileCore)
	}
}

func TestProfileFromClientInfo(t *testing.T) {
	tests := []struct {
		name        string
		client      string
		wantProfile Profile
	}{
		{"polecat gets execution", "polecat", ProfileExecution},
		{"gastown gets execution", "gastown", ProfileExecution},
		{"claude-code gets authoring", "claude-code", ProfileAuthoring},
		{"cursor gets authoring", "cursor", ProfileAuthoring},
		{"windsurf gets authoring", "windsurf", ProfileAuthoring},
		{"opencode gets authoring", "opencode", ProfileAuthoring},
		{"codex gets authoring", "codex", ProfileAuthoring},
		{"unknown gets core", "some-other-client", ProfileCore},
		{"empty gets core", "", ProfileCore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := sdkmcp.Implementation{
				Name:    tt.client,
				Version: "1.0.0",
			}
			got := ProfileFromClientInfo(&info)
			if got != tt.wantProfile {
				t.Errorf("ProfileFromClientInfo(%q) = %v, want %v", tt.client, got, tt.wantProfile)
			}
		})
	}
}
