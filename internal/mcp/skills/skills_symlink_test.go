// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSkillsSymlink asserts that <repo>/skills resolves to the canonical
// embedded directory inside the skills package. The symlink lets
// `task skills:validate ./skills` and the GitHub UI continue to find skill
// content at <repo>/skills/<name>/SKILL.md while //go:embed reads from
// the real files inside the package.
func TestSkillsSymlink(t *testing.T) {
	repoRoot := repoRootForTest(t)
	link := filepath.Join(repoRoot, "skills")

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat %s: %v", link, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", link)
	}

	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	const want = "internal/mcp/skills/embedded"
	if target != want {
		t.Fatalf("symlink target = %q, want %q", target, want)
	}

	// And it must resolve to a real directory containing SKILL.md files.
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	for _, name := range []string{
		"specgraph-authoring",
		"specgraph-graph-query",
		"specgraph-analytical-passes",
		"specgraph-drift",
		"specgraph-conventions",
		"specgraph-troubleshooting",
	} {
		path := filepath.Join(resolved, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
}

// repoRootForTest walks up from the test's working directory looking for
// go.mod. Returns the repo root or fails the test.
func repoRootForTest(t *testing.T) string {
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
