// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
	"github.com/stretchr/testify/require"
)

// TestDogfood_CheckedInConfigsAreCanonical guards the spgr-7htb dogfood
// cutover: the three managed MCP config files at the repo root must stay
// in the canonical form Sync produces from the SpecGraph dogfood slug +
// server URL. A future change to ManagedConfigs (key shape, env-var syntax,
// harness-specific fields) or a manual edit that drifts a file from
// canonical would silently bit-rot the committed configs until someone
// re-ran specgraph init manually; this test fails loud instead.
//
// The slug + URL constants below are the ground truth for this repo and are
// independent of the contributor's global config — the test must NOT depend
// on ~/.config/specgraph/config.yaml because CI agents may have a different
// or absent default. .specgraph.yaml deliberately omits a `server:` field
// (its presence would break startFakeServer-using tests by overriding the
// fake server URL via ProjectConfig.Server), so the dogfood URL is pinned
// here in the test instead.
//
// Implementation: copy the three checked-in files into a temp dir and run
// Sync there. If everything is already canonical, all three actions are
// no-op. The temp-dir copy keeps Sync read-only against the working tree.
func TestDogfood_CheckedInConfigsAreCanonical(t *testing.T) {
	const (
		dogfoodSlug      = "specgraph"
		dogfoodServerURL = "http://127.0.0.1:9090"
	)

	root, err := config.FindProjectRoot(".")
	require.NoError(t, err, "find project root from cmd/specgraph")

	pc, err := config.LoadProject(root)
	require.NoError(t, err, "load .specgraph.yaml")
	require.Equal(t, dogfoodSlug, pc.Slug, ".specgraph.yaml slug should be %q", dogfoodSlug)

	managedPaths := []string{".cursor/mcp.json", ".mcp.json", "opencode.json"}
	tmp := t.TempDir()
	for _, p := range managedPaths {
		src := filepath.Join(root, p)
		dst := filepath.Join(tmp, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755), "mkdir %s", filepath.Dir(dst)) //nolint:gosec // 0755 is intentional for test fixture dirs
		data, readErr := os.ReadFile(src)
		require.NoError(t, readErr, "read checked-in %s", p)
		require.NoError(t, os.WriteFile(dst, data, 0o600), "write fixture %s", dst) //nolint:gosec // dst is t.TempDir() joined with a literal path
	}

	params := managedfiles.ProjectParams{Slug: dogfoodSlug, ServerURL: dogfoodServerURL}
	harnesses := []managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	}
	results, err := managedfiles.SyncAll(tmp, harnesses, params, managedfiles.SyncOptions{})
	require.NoError(t, err, "SyncAll against fixture copy")

	// Filter to the three JSONKeyMerge paths under check; the manifest also
	// includes AGENTS.md and the cursor .mdc which aren't part of the
	// checked-in JSON-canonicalisation guard.
	jsonResults := make(map[string]managedfiles.SyncResult, len(managedPaths))
	for _, r := range results {
		for _, p := range managedPaths {
			if r.Path == p {
				jsonResults[r.Path] = r
			}
		}
	}
	require.Len(t, jsonResults, len(managedPaths),
		"expected one result per managed JSON path; manifest returned a different set")

	for _, p := range managedPaths {
		r := jsonResults[p]
		if r.Action != managedfiles.ActionNoOp {
			t.Errorf("checked-in %s drifted from canonical form: action = %v, want %v. "+
				"Run `specgraph init` from the repo root to re-canonicalize.",
				r.Path, managedfiles.ActionName(r.Action), managedfiles.ActionName(managedfiles.ActionNoOp))
		}
	}
}
