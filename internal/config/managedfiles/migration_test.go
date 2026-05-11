// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migrationSlug = "dogfood"
const migrationServerURL = "http://localhost:9090"

func TestMigrationV1ToV2Upgrade(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: migrationSlug, ServerURL: migrationServerURL}

	// Seed AGENTS.md with v=1 markers + canonical v=1 body.
	body := renderV1AgentsBlockBody(params)
	agentsSeed := []byte("# User content above\n\n<!-- specgraph:init:start v=1 -->" + string(body) + "<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), agentsSeed, 0o600)

	// Seed .cursor/rules/specgraph-bootstrap.md with frontmatter + v=1 block.
	_ = os.MkdirAll(filepath.Join(dir, ".cursor/rules"), 0o750)
	cursorBody := renderV1CursorBlockBody(params)
	cursorSeed := []byte(defaultCursorFrontmatter + "<!-- specgraph:init:start v=1 -->" + string(cursorBody) + "<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md"), cursorSeed, 0o600)

	results, err := SyncAll(dir, []Harness{HarnessClaude, HarnessCursor, HarnessOpenCode}, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action == ActionError {
			t.Errorf("%s: %v", r.Path, r.Err)
		}
	}

	// AGENTS.md: user content preserved, body unchanged, markers upgraded.
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("# User content above")) {
		t.Error("user content above block was destroyed")
	}
	if !bytes.Contains(got, []byte("v=2 sha256=")) {
		t.Error("markers not upgraded to v=2")
	}
	if bytes.Contains(got, []byte("v=1")) {
		t.Error("v=1 markers still present")
	}
	gotBody, _ := extractManagedBlockBody(got)
	// extractManagedBlockBody strips the leading/trailing newlines adjacent to
	// the markers; trimEdgeNewlines brings the renderer output into the same
	// form for a meaningful comparison.
	if !bytes.Equal(gotBody, trimEdgeNewlines(body)) {
		t.Errorf("body changed during upgrade\n got: %q\nwant: %q", gotBody, trimEdgeNewlines(body))
	}

	// .md must be deleted by supersedes.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md")); !os.IsNotExist(err) {
		t.Error(".md not deleted by supersedes")
	}
	// .mdc must exist with v=2 marker.
	mdc, mdcErr := os.ReadFile(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.mdc"))
	if mdcErr != nil {
		t.Fatalf(".mdc not created: %v", mdcErr)
	}
	if !bytes.Contains(mdc, []byte(defaultCursorFrontmatter)) {
		t.Error(".mdc missing frontmatter")
	}
	if !bytes.Contains(mdc, []byte("v=2 sha256=")) {
		t.Error(".mdc missing v=2 marker")
	}
}

func TestMigrationDriftedV1Refuses(t *testing.T) {
	dir := t.TempDir()
	params := ProjectParams{Slug: migrationSlug, ServerURL: migrationServerURL}

	// Seed AGENTS.md with v=1 markers but mangled body.
	agentsSeed := []byte("<!-- specgraph:init:start v=1 -->\nUSER EDITED — do not lose me\n<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), agentsSeed, 0o600)

	results, _ := SyncAll(dir, []Harness{HarnessClaude}, params, SyncOptions{})
	var agents SyncResult
	for _, r := range results {
		if r.Path == "AGENTS.md" {
			agents = r
		}
	}
	if agents.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped", agents.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(got), "USER EDITED") {
		t.Error("drifted user content was overwritten")
	}
}
