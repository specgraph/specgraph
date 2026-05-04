// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEnsureMCPSuffix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890/mcp/"},
		{"http://127.0.0.1:7890/", "http://127.0.0.1:7890/mcp/"},
		{"http://127.0.0.1:7890///", "http://127.0.0.1:7890/mcp/"},   // multiple trailing slashes
		{"http://127.0.0.1:7890/mcp", "http://127.0.0.1:7890/mcp/"},  // suffix without trailing slash
		{"http://127.0.0.1:7890/mcp/", "http://127.0.0.1:7890/mcp/"}, // suffix already present
		{"https://specgraph.example.com", "https://specgraph.example.com/mcp/"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ensureMCPSuffix(tc.in)
			if got != tc.want {
				t.Errorf("ensureMCPSuffix(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestManagedConfigs_PathsAndCount(t *testing.T) {
	configs := ManagedConfigs("specgraph", "http://127.0.0.1:7890")
	if got, want := len(configs), 3; got != want {
		t.Fatalf("ManagedConfigs returned %d entries, want %d", got, want)
	}
	wantPaths := map[string]bool{
		".cursor/mcp.json": false,
		".mcp.json":        false,
		"opencode.json":    false,
	}
	for _, c := range configs {
		if _, ok := wantPaths[c.Path]; !ok {
			t.Errorf("unexpected path %q", c.Path)
			continue
		}
		wantPaths[c.Path] = true
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Errorf("missing path %q", path)
		}
	}
}

func TestManagedConfigs_Cursor(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), ".cursor/mcp.json")
	want := map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Cursor patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_ClaudeCode(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), ".mcp.json")
	want := map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  "http://127.0.0.1:7890/mcp/",
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Claude Code patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_OpenCode(t *testing.T) {
	got := patchFor(t, ManagedConfigs("specgraph", "http://127.0.0.1:7890"), "opencode.json")
	want := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     "http://127.0.0.1:7890/mcp/",
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": "specgraph",
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OpenCode patch mismatch.\n got: %v\nwant: %v", got, want)
	}
}

func TestManagedConfigs_SlugFlowsThrough(t *testing.T) {
	configs := ManagedConfigs("my-other-project", "http://127.0.0.1:7890")
	for _, c := range configs {
		var m map[string]any
		if err := json.Unmarshal(c.Patch, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", c.Path, err)
		}
		// Walk to the headers; both shapes (mcpServers.specgraph.headers and
		// mcp.specgraph.headers) end at headers.X-Specgraph-Project.
		var server map[string]any
		switch c.Path {
		case ".cursor/mcp.json", ".mcp.json":
			server = m["mcpServers"].(map[string]any)["specgraph"].(map[string]any)
		case "opencode.json":
			server = m["mcp"].(map[string]any)["specgraph"].(map[string]any)
		}
		headers := server["headers"].(map[string]any)
		if got := headers["X-Specgraph-Project"]; got != "my-other-project" {
			t.Errorf("%s: X-Specgraph-Project = %v, want %q", c.Path, got, "my-other-project")
		}
	}
}

// patchFor decodes the patch for the named harness path and returns it as a
// generic map. Fails the test if the path isn't present.
func patchFor(t *testing.T, configs []ManagedConfig, path string) map[string]any {
	t.Helper()
	for _, c := range configs {
		if c.Path != path {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(c.Patch, &m); err != nil {
			t.Fatalf("unmarshal %s patch: %v", path, err)
		}
		return m
	}
	t.Fatalf("path %q not found in configs", path)
	return nil
}
