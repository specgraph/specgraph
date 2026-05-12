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
			got, _ := os.ReadFile(full)
			var gotDoc, wantDoc any
			_ = json.Unmarshal(got, &gotDoc)
			_ = json.Unmarshal(legacyPatchBytes(tc.goldenBuild), &wantDoc)
			gotCanon, _ := json.Marshal(gotDoc)
			wantCanon, _ := json.Marshal(wantDoc)
			if !bytes.Equal(gotCanon, wantCanon) {
				t.Errorf("migrated output differs from legacy:\n got:  %s\n want: %s", gotCanon, wantCanon)
			}
		})
	}
}

// legacyBuildClaudeMCPJSON / legacyBuildCursorMCPJSON are exact copies of
// the pre-migration Build closures, kept ONLY in this test file as the
// regression oracle. Delete them once the migration has been merged for
// one release cycle and we're confident in the new output.
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
