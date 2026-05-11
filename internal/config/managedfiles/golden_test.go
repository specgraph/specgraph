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

func TestGoldenMissingFirstInit(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: "captureslug", ServerURL: "http://localhost:9090"}
	harnesses := []Harness{HarnessClaude, HarnessCursor, HarnessOpenCode}

	results, err := SyncAll(dir, harnesses, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action == ActionError {
			t.Errorf("%s: %v", r.Path, r.Err)
		}
	}

	goldenDir := "testdata/golden/missing-first-init/out"

	// JSON files: byte-identical.
	jsonFiles := []string{".mcp.json", ".cursor/mcp.json", "opencode.json"}
	for _, name := range jsonFiles {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		want, _ := os.ReadFile(filepath.Join(goldenDir, filepath.Base(name)))
		if !bytesEqualJSON(t, got, want) {
			t.Errorf("%s mismatch\n got: %q\nwant: %q", name, got, want)
		}
	}

	// Markdown files: between-markers body identical; outside-markers
	// identical; markers themselves replaced (v=1 → v=2).
	mdCases := []struct {
		got, golden string
	}{
		{filepath.Join(dir, "AGENTS.md"), filepath.Join(goldenDir, "AGENTS.md")},
		{filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.mdc"), filepath.Join(goldenDir, "specgraph-bootstrap.md")},
	}
	for _, c := range mdCases {
		got, _ := os.ReadFile(c.got)
		want, _ := os.ReadFile(c.golden)
		gotBody, ok1 := extractManagedBlockBody(got)
		wantBody, ok2 := extractManagedBlockBody(want)
		if !ok1 || !ok2 {
			t.Errorf("%s: failed to extract block body", c.got)
			continue
		}
		if !bytes.Equal(gotBody, wantBody) {
			t.Errorf("%s body mismatch\n got: %q\nwant: %q", c.got, gotBody, wantBody)
		}
	}
}

// bytesEqualJSON compares two JSON byte slices semantically (alphabetical
// key order via re-marshalling). This guards against incidental key-order
// differences from different Go versions or jsonpatch behaviour.
func bytesEqualJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return bytes.Equal(ja, jb)
}
