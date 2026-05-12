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
	if len(all) != 8 {
		t.Errorf("expected 8 entries, got %d", len(all))
	}
	paths := map[string]bool{
		".mcp.json":                                    false,
		".cursor/mcp.json":                             false,
		"opencode.json":                                false,
		"AGENTS.md":                                    false,
		".cursor/rules/specgraph-bootstrap.mdc":        false,
		".specgraph/agents/opencode/specgraph.ts":      false,
		".cursor/rules/specgraph.mdc":                  false,
		".cursor/rules/specgraph-post-stage.mdc":       false,
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
			wantErr: "is not registered in vestigialCursorRulePriorHash",
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

// TestNoLegacyWholeFileHTMLSentinels pins the back-compat reasoning for
// PR D's RenderSentinel CommentHTML change. Before PR D, no shipped
// manifest entry combined Strategy==StrategyWholeFile with Comment==
// CommentHTML, so no file on disk anywhere carries the old `:start`-suffixed
// CommentHTML whole-file sentinel form. PR D introduces the combination
// only with HasFrontmatter==true. This test ensures a future entry can't
// silently introduce a WholeFile+HTML+!HasFrontmatter combination, which
// would suddenly produce the bare-`init` form by surprise.
func TestNoLegacyWholeFileHTMLSentinels(t *testing.T) {
	for _, mf := range allManagedFiles() {
		if mf.Strategy == StrategyWholeFile && mf.Comment == CommentHTML && !mf.HasFrontmatter {
			t.Errorf("entry %q: WholeFile+CommentHTML without HasFrontmatter is unsupported (see PR D back-compat anchor)", mf.Path)
		}
	}
}
