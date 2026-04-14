// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestTierFromClientInfo_Nil(t *testing.T) {
	got := TierFromClientInfo(nil)
	if got != TierCore {
		t.Errorf("TierFromClientInfo(nil) = %v, want %v", got, TierCore)
	}
}

func TestTierFromClientInfo(t *testing.T) {
	tests := []struct {
		name     string
		client   string
		wantTier Tier
	}{
		{"polecat gets execution", "polecat", TierExecution},
		{"gastown gets execution", "gastown", TierExecution},
		{"claude-code gets authoring", "claude-code", TierAuthoring},
		{"cursor gets authoring", "cursor", TierAuthoring},
		{"windsurf gets authoring", "windsurf", TierAuthoring},
		{"unknown gets core", "some-other-client", TierCore},
		{"empty gets core", "", TierCore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := sdkmcp.Implementation{
				Name:    tt.client,
				Version: "1.0.0",
			}
			got := TierFromClientInfo(&info)
			if got != tt.wantTier {
				t.Errorf("TierFromClientInfo(%q) = %v, want %v", tt.client, got, tt.wantTier)
			}
		})
	}
}
