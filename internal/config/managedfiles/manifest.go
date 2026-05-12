// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
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
			JSONKeys: []JSONManagedKey{
				{
					Path: "/mcpServers/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"type": "http",
							"url":  ensureMCPSuffix(p.ServerURL),
							"headers": map[string]any{
								"Authorization":       "Bearer ${SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
			},
		},
		{
			Path:     ".cursor/mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessCursor,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/mcpServers/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"url": ensureMCPSuffix(p.ServerURL),
							"headers": map[string]any{
								"Authorization":       "Bearer ${env:SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
			},
		},
		{
			Path:     "opencode.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessOpenCode,
			JSONKeys: []JSONManagedKey{
				{
					Path: "/$schema",
					Mode: KeyManagedValue,
					Value: func(_ ProjectParams) (any, error) {
						return "https://opencode.ai/config.json", nil
					},
				},
				{
					Path: "/mcp/specgraph",
					Mode: KeyManagedValue,
					Value: func(p ProjectParams) (any, error) {
						return map[string]any{
							"type":    "remote",
							"url":     ensureMCPSuffix(p.ServerURL),
							"enabled": true,
							"headers": map[string]any{
								"Authorization":       "Bearer {env:SPECGRAPH_API_KEY}",
								"X-Specgraph-Project": p.Slug,
							},
						}, nil
					},
				},
				{
					Path: "/plugin",
					Mode: KeyManagedArrayUnion,
					Value: func(_ ProjectParams) (any, error) {
						return []any{"./.specgraph/agents/opencode/specgraph.ts"}, nil
					},
				},
			},
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
		{
			Path:     ".specgraph/agents/opencode/specgraph.ts",
			Strategy: StrategyWholeFile,
			Source:   "embedded/opencode/specgraph.ts",
			Comment:  CommentSlash,
			Harness:  HarnessOpenCode,
		},
		{
			Path:           ".cursor/rules/specgraph.mdc",
			Strategy:       StrategyWholeFile,
			Source:         "embedded/cursor/specgraph.mdc",
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			HasFrontmatter: true,
			SupersedesPath: ".cursor/rules/specgraph.md",
		},
		{
			Path:           ".cursor/rules/specgraph-post-stage.mdc",
			Strategy:       StrategyWholeFile,
			Source:         "embedded/cursor/specgraph-post-stage.mdc",
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			HasFrontmatter: true,
			SupersedesPath: ".cursor/rules/post-stage.md",
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
		if err := validateManifestEntry(mf); err != nil {
			panic(err.Error())
		}
	}
}

// validateManifestEntry returns nil if mf satisfies the package's manifest
// invariants, or a descriptive error otherwise. Called from init() at package
// load (where any error panics) and directly from tests that want to
// exercise invariant rules without crashing the test binary.
//
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func validateManifestEntry(mf ManagedFile) error {
	hasBuild := mf.Build != nil
	hasJSONKeys := len(mf.JSONKeys) > 0
	hasSource := mf.Source != ""
	if hasSource && hasBuild {
		return fmt.Errorf("manifest entry %q has both Source and Build", mf.Path)
	}
	if !hasSource && !hasBuild && !hasJSONKeys {
		return fmt.Errorf("manifest entry %q has neither Source nor Build", mf.Path)
	}
	switch mf.Strategy {
	case StrategyJSONKeyMerge:
		if !hasJSONKeys {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy requires JSONKeys", mf.Path)
		}
		if hasBuild {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy must not set Build (use JSONKeys)", mf.Path)
		}
		if hasSource {
			return fmt.Errorf("manifest entry %q: JSONKeyMerge strategy must not set Source", mf.Path)
		}
	case StrategyMarkdownBlock:
		if !hasBuild {
			return fmt.Errorf("manifest entry %q: MarkdownBlock strategy requires Build", mf.Path)
		}
		if hasJSONKeys {
			return fmt.Errorf("manifest entry %q: MarkdownBlock strategy must not set JSONKeys", mf.Path)
		}
	case StrategyWholeFile:
		if !hasSource {
			return fmt.Errorf("manifest entry %q: WholeFile strategy requires Source", mf.Path)
		}
		if hasBuild || hasJSONKeys {
			return fmt.Errorf("manifest entry %q: WholeFile strategy must not set Build or JSONKeys", mf.Path)
		}
		if mf.HasFrontmatter && mf.Comment == CommentNone {
			return fmt.Errorf("manifest entry %q: HasFrontmatter requires non-empty comment syntax", mf.Path)
		}
		// Supported combinations:
		//   CommentNone  + !HasFrontmatter → JSON files (no in-file sentinel)   [PR E]
		//   CommentHash  + !HasFrontmatter → shell / Python / YAML scripts
		//   CommentSlash + !HasFrontmatter → TypeScript / JS plugin source      [PR C]
		//   CommentHTML  + !HasFrontmatter → plain Markdown                     [PR E]
		//   CommentHTML  +  HasFrontmatter → Markdown with leading frontmatter  [PR D]
	}
	if mf.HasFrontmatter && mf.Strategy != StrategyWholeFile {
		return fmt.Errorf("manifest entry %q: HasFrontmatter requires WholeFile strategy, got %s", mf.Path, mf.Strategy)
	}
	if mf.Strategy == StrategyWholeFile && mf.SupersedesPath != "" {
		// PR E Task 9: SupersedesPath entries must have a registered prior
		// canonical hash in the unified priors registry (see priors.go and
		// vestigial_cursor_rules.go). Without one, supersedesGuardedDelete
		// can't safely identify and clean up pre-rename user copies.
		if len(priorsFor(mf.Path)) == 0 {
			return fmt.Errorf("manifest entry %q: SupersedesPath %q requires a registered prior canonical hash for %q in priorsRegistry (vestigial_cursor_rules.go)", mf.Path, mf.SupersedesPath, mf.Path)
		}
	}
	return nil
}

// Build closures — JSON-merge patches.

func ensureMCPSuffix(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	if strings.HasSuffix(trimmed, "/mcp") {
		return trimmed + "/"
	}
	return trimmed + "/mcp/"
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
