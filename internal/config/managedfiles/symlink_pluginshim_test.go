// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPluginCursorSymlinksResolve verifies that the reverse-symlinks under
// plugin/cursor/.cursor/rules/ point at real files under embedded/cursor/.
// The symlinks are author-convenience: developers editing under plugin/
// land their changes in the embedded canonical the binary reads via
// //go:embed. A broken symlink would mean an editor opens a dangling file.
//
// This test is build-tag-free; it runs in `task check`. The repo-root path
// is computed by walking up from the test's working directory looking for
// go.mod.
func TestPluginCursorSymlinksResolve(t *testing.T) {
	root := findRepoRootForTest(t)
	rulesDir := filepath.Join(root, "plugin", "cursor", ".cursor", "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatalf("read %s: %v", rulesDir, err)
	}
	mdcCount := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".mdc") {
			continue
		}
		mdcCount++
		full := filepath.Join(rulesDir, e.Name())
		resolved, rerr := filepath.EvalSymlinks(full)
		if rerr != nil {
			t.Errorf("%s: EvalSymlinks: %v", e.Name(), rerr)
			continue
		}
		// Must resolve to a file under internal/config/managedfiles/embedded/cursor/
		if !strings.Contains(resolved, filepath.Join("internal", "config", "managedfiles", "embedded", "cursor")) {
			t.Errorf("%s: resolves to %s, expected under embedded/cursor/", e.Name(), resolved)
		}
	}
	if mdcCount == 0 {
		t.Errorf("no .mdc files found under %s", rulesDir)
	}
}

// findRepoRootForTest walks up from the test's working directory looking
// for go.mod. Returns the repo root or fails the test.
func findRepoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}
