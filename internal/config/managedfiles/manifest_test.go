// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"strings"
	"testing"
)

func TestManifestShape(t *testing.T) {
	all := allManagedFiles()
	if len(all) != 13 {
		t.Errorf("expected 13 entries, got %d", len(all))
	}
	paths := map[string]bool{
		".mcp.json":                                                        false,
		".cursor/mcp.json":                                                 false,
		"opencode.json":                                                    false,
		"AGENTS.md":                                                        false,
		".cursor/rules/specgraph-bootstrap.mdc":                            false,
		".specgraph/agents/opencode/specgraph.ts":                          false,
		".cursor/rules/specgraph.mdc":                                      false,
		".cursor/rules/specgraph-post-stage.mdc":                           false,
		".specgraph/agents/claude/.claude-plugin/plugin.json":              false,
		".specgraph/agents/claude/.claude-plugin/marketplace.json":         false,
		".specgraph/agents/claude/hooks/specgraph-session-start.sh":        false,
		".specgraph/agents/claude/hooks/specgraph-post-stage.sh":           false,
		".specgraph/agents/claude/routing-guide.md":                        false,
	}
	for _, mf := range all {
		if _, ok := paths[mf.Path]; !ok {
			t.Errorf("unexpected path %q", mf.Path)
		}
		paths[mf.Path] = true
		// Source-xor-Build-xor-JSONKeys invariant.
		if mf.Source != "" && mf.Build != nil {
			t.Errorf("%q: both Source and Build set", mf.Path)
		}
		if mf.Source == "" && mf.Build == nil && len(mf.JSONKeys) == 0 {
			t.Errorf("%q: neither Source nor Build nor JSONKeys set", mf.Path)
		}
	}
	for path, seen := range paths {
		if !seen {
			t.Errorf("manifest missing %q", path)
		}
	}
}

func TestManifestBuildPurity(t *testing.T) {
	params := ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	for _, mf := range allManagedFiles() {
		if mf.Build == nil {
			continue
		}
		a, err1 := mf.Build(params)
		b, err2 := mf.Build(params)
		if err1 != nil || err2 != nil {
			t.Errorf("%q: build error: %v / %v", mf.Path, err1, err2)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%q: Build not pure (two calls returned different bytes)", mf.Path)
		}
	}
}

func TestValidateManifestEntry(t *testing.T) {
	cases := []struct {
		name    string
		mf      ManagedFile
		wantErr string
	}{
		{
			name: "both Source and Build set",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source: "s", Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
			wantErr: "has both Source and Build",
		},
		{
			name:    "neither Source nor Build set",
			mf:      ManagedFile{Path: "x", Strategy: StrategyWholeFile},
			wantErr: "has neither Source nor Build",
		},
		{
			name: "JSONKeyMerge without Build",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyJSONKeyMerge,
				Source: "s",
			},
			wantErr: "JSONKeyMerge strategy requires JSONKeys",
		},
		{
			name: "MarkdownBlock without Build",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyMarkdownBlock,
				Source: "s",
			},
			wantErr: "MarkdownBlock strategy requires Build",
		},
		{
			name: "valid MarkdownBlock",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyMarkdownBlock,
				Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
		},
		{
			name: "WholeFile without Source",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
			wantErr: "WholeFile strategy requires Source",
		},
		{
			name: "valid WholeFile",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source: "s",
			},
		},
		{
			name: "valid JSONKeyMerge",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyJSONKeyMerge,
				JSONKeys: []JSONManagedKey{
					{Path: "/foo", Mode: KeyManagedValue, Value: func(_ ProjectParams) (any, error) { return "bar", nil }},
				},
			},
		},
		{
			name: "HasFrontmatter on non-WholeFile",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyMarkdownBlock,
				Build:          func(ProjectParams) ([]byte, error) { return nil, nil },
				Comment:        CommentHTML,
				HasFrontmatter: true,
			},
			wantErr: "HasFrontmatter requires WholeFile",
		},
		{
			name: "HasFrontmatter with CommentNone",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source:         "s",
				Comment:        CommentNone,
				HasFrontmatter: true,
			},
			wantErr: "HasFrontmatter requires non-empty comment syntax",
		},
		{
			name: "valid HasFrontmatter entry",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source:         "s",
				Comment:        CommentHTML,
				HasFrontmatter: true,
			},
		},
		{
			name: "WholeFile with unregistered SupersedesPath",
			mf: ManagedFile{
				Path:           "x.mdc",
				Strategy:       StrategyWholeFile,
				Source:         "embedded/cursor/x.mdc",
				Comment:        CommentHTML,
				Harness:        HarnessCursor,
				HasFrontmatter: true,
				SupersedesPath: ".cursor/rules/unknown.md",
			},
			wantErr: "requires a registered prior canonical hash",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateManifestEntry(tc.mf)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidator_JSONKeyMergeRequiresJSONKeys(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		Build:    func(_ ProjectParams) ([]byte, error) { return []byte(`{}`), nil },
	}
	if err := validateManifestEntry(mf); err == nil {
		t.Error("expected validator to reject JSONKeyMerge with Build (post-PR-E)")
	}
}

func TestValidator_JSONKeyMergeAcceptsJSONKeys(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.json",
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessClaude,
		JSONKeys: []JSONManagedKey{
			{Path: "/foo", Mode: KeyManagedValue, Value: func(_ ProjectParams) (any, error) { return "bar", nil }},
		},
	}
	if err := validateManifestEntry(mf); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidator_JSONKeysOnWholeFileRejected(t *testing.T) {
	mf := ManagedFile{
		Path:     "x.md",
		Strategy: StrategyWholeFile,
		Comment:  CommentHTML,
		Harness:  HarnessClaude,
		Source:   "embedded/x.md",
		JSONKeys: []JSONManagedKey{{Path: "/x", Mode: KeyManagedValue, Value: func(_ ProjectParams) (any, error) { return "", nil }}},
	}
	if err := validateManifestEntry(mf); err == nil {
		t.Error("expected validator to reject JSONKeys on WholeFile")
	}
}

func TestManifest_ClaudePluginShimEntries(t *testing.T) {
	wantPaths := []string{
		".specgraph/agents/claude/.claude-plugin/plugin.json",
		".specgraph/agents/claude/.claude-plugin/marketplace.json",
		".specgraph/agents/claude/hooks/specgraph-session-start.sh",
		".specgraph/agents/claude/hooks/specgraph-post-stage.sh",
		".specgraph/agents/claude/routing-guide.md",
	}
	present := map[string]bool{}
	for _, mf := range allManagedFiles() {
		present[mf.Path] = true
	}
	for _, p := range wantPaths {
		if !present[p] {
			t.Errorf("manifest missing entry %q", p)
		}
	}
}

// TestManifestValidator_WholeFileSupportedCombinations enumerates the five
// supported WholeFile (Comment, HasFrontmatter) combinations as of PR E.
// Every WholeFile manifest entry must match one of these. Replaces the
// PR D back-compat anchor that rejected CommentHTML+!HasFrontmatter —
// that combination is now first-class (plain Markdown like routing-guide.md).
//
//	CommentNone  + !HasFrontmatter → JSON files (no in-file sentinel)   [PR E]
//	CommentHash  + !HasFrontmatter → shell / Python / YAML scripts
//	CommentSlash + !HasFrontmatter → TypeScript / JS plugin source      [PR C]
//	CommentHTML  + !HasFrontmatter → plain Markdown                     [PR E]
//	CommentHTML  +  HasFrontmatter → Markdown with leading frontmatter  [PR D]
func TestManifestValidator_WholeFileSupportedCombinations(t *testing.T) {
	type combo struct {
		comment CommentSyntax
		hasFm   bool
	}
	supported := map[combo]bool{
		{CommentNone, false}:  true,
		{CommentHash, false}:  true,
		{CommentSlash, false}: true,
		{CommentHTML, false}:  true,
		{CommentHTML, true}:   true,
	}
	for _, mf := range allManagedFiles() {
		if mf.Strategy != StrategyWholeFile {
			continue
		}
		k := combo{comment: mf.Comment, hasFm: mf.HasFrontmatter}
		if !supported[k] {
			t.Errorf("entry %q: unsupported WholeFile combo Comment=%v HasFrontmatter=%v",
				mf.Path, mf.Comment, mf.HasFrontmatter)
		}
	}
}
