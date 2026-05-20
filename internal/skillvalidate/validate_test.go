// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skillvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill is a test helper that creates a SKILL.md file at a synthetic
// skill path with the given content. The directory name becomes the
// expected `name` value for validation.
func writeSkill(t *testing.T, root, dir, content string) {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestValidateRoots_AcceptsValidSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "good-skill", `---
name: good-skill
summary: A perfectly fine summary.
description: A perfectly fine skill description.
---

Body content here.
`)

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].OK {
		t.Fatalf("expected OK, got reasons: %v", results[0].Reasons)
	}
}

func TestValidateRoots_RejectsMissingFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "bare-skill", "Just a body, no frontmatter.\n")

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected one failure, got %+v", results)
	}
}

func TestValidateRoots_RejectsNameDirMismatch(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "actual-dir", `---
name: different-name
description: A description.
---

Body.
`)

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected mismatch failure, got %+v", results)
	}
	found := false
	for _, r := range results[0].Reasons {
		if strings.Contains(r, "must match directory name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'must match directory name' reason, got %v", results[0].Reasons)
	}
}

func TestValidateRoots_RejectsMissingDescription(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "no-desc-skill", `---
name: no-desc-skill
---

Body.
`)

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected description failure, got %+v", results)
	}
}

func TestValidateRoots_AcceptsValidSkillWithoutTrailingNewline(t *testing.T) {
	// Regression: bufio.Reader.ReadString('\n') returns the partial line plus
	// io.EOF when the delimiter is missing at end-of-file. Ensure a SKILL.md
	// whose body ends without a trailing newline still validates — the
	// closing `---` of the frontmatter is what matters, not whether the body
	// happens to end with \n.
	root := t.TempDir()
	writeSkill(t, root, "no-trailing-nl-skill", "---\n"+
		"name: no-trailing-nl-skill\n"+
		"summary: A perfectly fine summary.\n"+
		"description: A skill whose body does not end with a newline.\n"+
		"---\n"+
		"Body without trailing newline.")

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].OK {
		t.Fatalf("expected OK, got reasons: %v", results[0].Reasons)
	}
}

func TestValidateRoots_RejectsTruncatedFrontmatter(t *testing.T) {
	// EOF inside the frontmatter (no closing ---) must still fail clearly.
	root := t.TempDir()
	writeSkill(t, root, "truncated-skill", "---\nname: truncated-skill\ndescription: missing closer\n")

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected failure for truncated frontmatter, got %+v", results)
	}
	found := false
	for _, r := range results[0].Reasons {
		if strings.Contains(r, "frontmatter not closed") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'frontmatter not closed' reason, got %v", results[0].Reasons)
	}
}

func TestValidateRoots_RejectsTooLongDescription(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a", maxDesc+1)
	writeSkill(t, root, "long-desc-skill", `---
name: long-desc-skill
description: `+long+`
---

Body.
`)

	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected too-long failure, got %+v", results)
	}
}

func TestValidateRoots_RejectsMissingSummary(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "no-summary", `---
name: no-summary
description: A perfectly fine skill description.
---

Body.
`)
	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected failure, got %+v", results)
	}
	joined := strings.Join(results[0].Reasons, "; ")
	if !strings.Contains(joined, "summary") {
		t.Errorf("expected 'summary' in failure reasons; got %q", joined)
	}
}

func TestValidateRoots_RejectsOverlongSummary_FlowScalar(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a", 121) // single-line, 121 chars
	writeSkill(t, root, "overlong-flow", `---
name: overlong-flow
summary: `+long+`
description: ok
---
Body.
`)
	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected failure, got %+v", results)
	}
}

func TestValidateRoots_RejectsOverlongSummary_BlockScalar(t *testing.T) {
	root := t.TempDir()
	// Block-scalar source bytes are < 120 (four 25-char lines = 100) but
	// decoded value (newlines fold to spaces) is > 120.
	writeSkill(t, root, "overlong-block", `---
name: overlong-block
summary: >
  aaaaaaaaaaaaaaaaaaaaaaaaa
  bbbbbbbbbbbbbbbbbbbbbbbbb
  ccccccccccccccccccccccccc
  ddddddddddddddddddddddddd
  eeeeeeeeeeeeeeeeeeeeeeeee
description: ok
---
Body.
`)
	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected failure for decoded-length>120; got %+v", results)
	}
}

func TestValidateRoots_RejectsNonKebabName(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "Foo_Bar", `---
name: Foo_Bar
summary: A skill with a non-kebab name.
description: ok
---
Body.
`)
	results, err := ValidateRoots([]string{root})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected failure, got %+v", results)
	}
	joined := strings.Join(results[0].Reasons, "; ")
	if !strings.Contains(joined, "kebab") && !strings.Contains(joined, "name") {
		t.Errorf("expected 'kebab' or 'name' in failure reasons; got %q", joined)
	}
}

func TestValidateRoots_FollowsRepoSymlink(t *testing.T) {
	// Stand in for the <repo>/skills symlink created in Task 1.
	target := t.TempDir()
	writeSkill(t, target, "valid-skill", `---
name: valid-skill
summary: A perfectly fine summary.
description: A perfectly fine skill description.
---
Body.
`)

	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "skills-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	results, err := ValidateRoots([]string{link})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("expected pass through symlink, got %+v", results)
	}
}

func TestValidateRoots_RealSkills(t *testing.T) {
	// Sanity check: validate the in-tree skills/ directory if present.
	if _, err := os.Stat("../../skills"); err != nil {
		t.Skip("skills/ not present at test working dir")
	}
	results, err := ValidateRoots([]string{"../../skills"})
	if err != nil {
		t.Fatalf("ValidateRoots: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one skill, got 0")
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("real skill failed validation: %s — %v", r.Path, r.Reasons)
		}
	}
}
