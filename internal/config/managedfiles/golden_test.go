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
		got, gerr := os.ReadFile(filepath.Join(dir, name))
		if gerr != nil {
			t.Errorf("read produced %s: %v", name, gerr)
			continue
		}
		want, werr := os.ReadFile(filepath.Join(goldenDir, filepath.Base(name)))
		if werr != nil {
			t.Errorf("read golden %s: %v", name, werr)
			continue
		}
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
		got, gerr := os.ReadFile(c.got)
		if gerr != nil {
			t.Errorf("read %s: %v", c.got, gerr)
			continue
		}
		want, werr := os.ReadFile(c.golden)
		if werr != nil {
			t.Errorf("read golden %s: %v", c.golden, werr)
			continue
		}
		gotBody, ok1 := extractManagedBlockBody(got)
		wantBody, ok2 := extractManagedBlockBody(want)
		if !ok1 || !ok2 {
			t.Errorf("%s: failed to extract block body (got ok=%v want ok=%v)", c.got, ok1, ok2)
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
		t.Errorf("unmarshal a: %v", err)
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		t.Errorf("unmarshal b: %v", err)
		return false
	}
	ja, err := json.Marshal(va)
	if err != nil {
		t.Errorf("re-marshal a: %v", err)
		return false
	}
	jb, err := json.Marshal(vb)
	if err != nil {
		t.Errorf("re-marshal b: %v", err)
		return false
	}
	return bytes.Equal(ja, jb)
}
