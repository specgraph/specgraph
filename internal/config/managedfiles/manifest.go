// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest returns the list of ManagedFiles filtered by the given
// harnesses. Order is stable across calls — callers may rely on it for
// deterministic output (e.g. doctor's report).
func Manifest(harnesses []Harness) []ManagedFile {
	all := allManagedFiles()
	enabled := harnessSet(harnesses)
	out := make([]ManagedFile, 0, len(all))
	for _, mf := range all {
		if enabled[mf.Harness] {
			out = append(out, mf)
		}
	}
	return out
}

func allManagedFiles() []ManagedFile {
	return []ManagedFile{
		{
			Path:     ".mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			Build:    buildClaudeMCPJSON,
		},
		{
			Path:     ".cursor/mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessCursor,
			Build:    buildCursorMCPJSON,
		},
		{
			Path:     "opencode.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessOpenCode,
			Build:    buildOpenCodeJSON,
		},
		{
			Path:     "AGENTS.md",
			Strategy: StrategyMarkdownBlock,
			Comment:  CommentHTML,
			Harness:  HarnessClaude,
			Build:    buildAgentsBlockBody,
		},
		{
			Path:           ".cursor/rules/specgraph-bootstrap.mdc",
			Strategy:       StrategyMarkdownBlock,
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			SupersedesPath: ".cursor/rules/specgraph-bootstrap.md",
			Build:          buildCursorBootstrapBody,
		},
	}
}

func harnessSet(harnesses []Harness) map[Harness]bool {
	out := make(map[Harness]bool, len(harnesses))
	for _, h := range harnesses {
		out[h] = true
	}
	return out
}

func init() {
	for _, mf := range allManagedFiles() {
		hasSource := mf.Source != ""
		hasBuild := mf.Build != nil
		if hasSource && hasBuild {
			panic(fmt.Sprintf("manifest entry %q has both Source and Build", mf.Path))
		}
		if !hasSource && !hasBuild {
			panic(fmt.Sprintf("manifest entry %q has neither Source nor Build", mf.Path))
		}
		switch mf.Strategy {
		case StrategyJSONKeyMerge, StrategyMarkdownBlock:
			if !hasBuild {
				panic(fmt.Sprintf("manifest entry %q: %v strategy requires Build", mf.Path, mf.Strategy))
			}
		case StrategyWholeFile:
			if !hasSource {
				panic(fmt.Sprintf("manifest entry %q: WholeFile strategy requires Source", mf.Path))
			}
		}
	}
}

// Build closures — JSON-merge patches.

func ensureMCPSuffix(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	if strings.HasSuffix(trimmed, "/mcp") {
		return trimmed + "/"
	}
	return trimmed + "/mcp/"
}

func buildCursorMCPJSON(p ProjectParams) ([]byte, error) {
	b, err := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"url": ensureMCPSuffix(p.ServerURL),
				"headers": map[string]any{
					"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cursor MCP JSON: %w", err)
	}
	return b, nil
}

func buildClaudeMCPJSON(p ProjectParams) ([]byte, error) {
	b, err := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"specgraph": map[string]any{
				"type": "http",
				"url":  ensureMCPSuffix(p.ServerURL),
				"headers": map[string]any{
					"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal claude MCP JSON: %w", err)
	}
	return b, nil
}

func buildOpenCodeJSON(p ProjectParams) ([]byte, error) {
	b, err := json.Marshal(map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{
			"specgraph": map[string]any{
				"type":    "remote",
				"url":     ensureMCPSuffix(p.ServerURL),
				"enabled": true,
				"headers": map[string]any{
					"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
					"X-Specgraph-Project": p.Slug,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal opencode JSON: %w", err)
	}
	return b, nil
}

// Build closures — markdown block bodies. PR B uses v=1 body verbatim
// for v=2 emission; body text is identical between v=1 and v=2, only
// the marker shape differs.

func buildAgentsBlockBody(p ProjectParams) ([]byte, error) {
	return renderV1AgentsBlockBody(p), nil
}

func buildCursorBootstrapBody(p ProjectParams) ([]byte, error) {
	return renderV1CursorBlockBody(p), nil
}
