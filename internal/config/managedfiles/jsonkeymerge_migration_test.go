// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigratedMCPJSONsMatchLegacyOutput(t *testing.T) {
	params := ProjectParams{
		Slug:      "test-project",
		ServerURL: "https://specgraph.example.com",
	}
	cases := []struct {
		path  string
		entry func() ManagedFile
		// goldenBuild is the EXACT bytes the pre-migration Build closure
		// produced for these params. Captured by running the legacy
		// closure once and freezing the output.
		goldenBuild []byte
	}{
		{
			path:        ".mcp.json",
			entry:       func() ManagedFile { return findManifestEntry(t, ".mcp.json") },
			goldenBuild: legacyBuildClaudeMCPJSON(params),
		},
		{
			path:        ".cursor/mcp.json",
			entry:       func() ManagedFile { return findManifestEntry(t, ".cursor/mcp.json") },
			goldenBuild: legacyBuildCursorMCPJSON(params),
		},
		{
			path:        "opencode.json",
			entry:       func() ManagedFile { return findManifestEntry(t, "opencode.json") },
			goldenBuild: legacyBuildOpenCodeJSON(params),
		},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			full := filepath.Join(dir, tc.path)
			_ = os.MkdirAll(filepath.Dir(full), 0o755) //nolint:gosec // test directory creation with permissive mode is intentional
			mf := tc.entry()
			s := jsonKeyMergeStrategy{}
			res, err := s.Sync(dir, mf, params, SyncOptions{})
			if err != nil {
				t.Fatalf("sync: %v", err)
			}
			if res.Err != nil {
				t.Fatalf("sync result error: %v", res.Err)
			}
			got, rerr := os.ReadFile(full)
			if rerr != nil {
				t.Fatalf("read %s: %v", full, rerr)
			}
			var gotDoc, wantDoc any
			if jerr := json.Unmarshal(got, &gotDoc); jerr != nil {
				t.Fatalf("unmarshal got: %v", jerr)
			}
			if jerr := json.Unmarshal(legacyPatchBytes(tc.goldenBuild), &wantDoc); jerr != nil {
				t.Fatalf("unmarshal want: %v", jerr)
			}
			gotCanon, merr := json.Marshal(gotDoc)
			if merr != nil {
				t.Fatalf("marshal got: %v", merr)
			}
			wantCanon, merr := json.Marshal(wantDoc)
			if merr != nil {
				t.Fatalf("marshal want: %v", merr)
			}
			if !bytes.Equal(gotCanon, wantCanon) {
				t.Errorf("migrated output differs from legacy:\n got:  %s\n want: %s", gotCanon, wantCanon)
			}
		})
	}
}

// legacyBuildClaudeMCPJSON, legacyBuildCursorMCPJSON, legacyBuildOpenCodeJSON,
// and legacyPatchBytes are kept ONLY in this test file as regression oracles
// (exact copies of the pre-migration Build closures / helpers). Delete all
// four once the migration has been merged for one release cycle and we're
// confident in the new output.
func legacyBuildClaudeMCPJSON(p ProjectParams) []byte {
	b, _ := json.Marshal(map[string]any{
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
	return b
}

func legacyBuildCursorMCPJSON(p ProjectParams) []byte {
	b, _ := json.Marshal(map[string]any{
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
	return b
}

// legacyBuildOpenCodeJSON mirrors the pre-migration buildOpenCodeJSON
// closure for use as the regression oracle. When called via
// TestMigratedMCPJSONsMatchLegacyOutput with no pre-existing file, the
// legacy path's unionPluginArray hook is a no-op (canonical wins) and
// the new KeyManagedArrayUnion mode produces the same plugin slice
// because existingDoc is nil. Both paths therefore yield the same
// semantic document.
func legacyBuildOpenCodeJSON(p ProjectParams) []byte {
	b, _ := json.Marshal(map[string]any{
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
		"plugin": []any{"./.specgraph/agents/opencode/specgraph.ts"},
	})
	return b
}

// TestMigratedOpenCodeJSON_PreservesPluginUnion exercises the critical
// regression: user-installed plugins in /plugin must survive an init
// sync. Under the new KeyManagedArrayUnion mode this is provided by
// jsonKeyMergeCanonicalFromKeys (phase 3); the path-keyed
// unionPluginArray post-merge hook is gone.
func TestMigratedOpenCodeJSON_PreservesPluginUnion(t *testing.T) {
	params := ProjectParams{Slug: "test", ServerURL: "https://x.example"}
	dir := t.TempDir()
	full := filepath.Join(dir, "opencode.json")
	// Pre-existing opencode.json with a user-added plugin.
	_ = os.WriteFile(full, []byte(`{"plugin":["./user-plugin.ts"]}`), 0o644) //nolint:gosec // intentional permissive mode for permission-preservation test
	mf := findManifestEntry(t, "opencode.json")
	s := jsonKeyMergeStrategy{}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	got, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read %s: %v", full, rerr)
	}
	// Assert array membership structurally — the legacy unionPluginArray
	// hook prepended canonical entries; KeyManagedArrayUnion (post-PR E)
	// appends them. Both preserve user plugins, which is the user-visible
	// contract. Order is implementation-defined and covered by the doc
	// comment on KeyManagedArrayUnion.
	var doc struct {
		Plugin []string `json:"plugin"`
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, want := range []string{
		"./user-plugin.ts",
		"./.specgraph/agents/opencode/specgraph.ts",
	} {
		if !slicesContains(doc.Plugin, want) {
			t.Errorf("plugin array missing %q: %v", want, doc.Plugin)
		}
	}
}

// slicesContains is a tiny local helper avoiding the slices import for
// a single comparison so this test file stays self-contained.
func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// legacyPatchBytes returns the legacy Build closure's patch bytes
// verbatim. Used as the regression oracle in golden comparisons;
// since both .mcp.json and .cursor/mcp.json closures produced
// top-level objects, MergePatch({}, patch) is identical to patch,
// so no merge step is needed here.
func legacyPatchBytes(patch []byte) []byte {
	return patch
}

func findManifestEntry(t *testing.T, path string) ManagedFile {
	t.Helper()
	for _, mf := range allManagedFiles() {
		if mf.Path == path {
			return mf
		}
	}
	t.Fatalf("manifest entry %q not found", path)
	return ManagedFile{}
}
