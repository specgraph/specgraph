# spgr-rwrp PR F — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve the six SpecGraph skills via MCP resource + tools instead of an on-disk symlink. Embed canonicals into the binary, expose `specgraph://skills/<name>` plus `specgraph_skills_list`/`_get`/`_search`, share a `NameRegex` between the validator and the resource handler, and delete the now-vestigial `plugin/specgraph/skills` symlink and `task plugin:sync`.

**Architecture:** A new `internal/mcp/skills/` subpackage owns a small read-only `Source` interface (`List`/`Get`/`Search`) with one implementation, `embeddedSource`, backed by `//go:embed embedded/*/SKILL.md`. Canonicals relocate from `<repo>/skills/` to `internal/mcp/skills/embedded/`; the repo-root `<repo>/skills` becomes a reverse-symlink. `internal/mcp/server.go` constructs the source once at startup, hands it to `RegisterResources` (signature grows), `RegisterSkillTools` (new), and `primeResourceHandler` (signature grows). Tool schemas use the existing `objectSchema(props{...}, required...)` builder convention — no new typed-struct surface.

**Tech Stack:** Go 1.24, `embed.FS`, `regexp` (RE2), Taskfile.dev. Tests use stdlib `testing` plus `testify/require` as the existing MCP tests do. E2E uses Ginkgo/Gomega under `//go:build e2e`.

**Spec:** [`2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md`](2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md)

**Bead:** spgr-i3he

---

## File Structure

### New files

- `internal/mcp/skills/doc.go` — package comment: `Source` contract, the SpecGraph-local `summary:` extension, future-source playbook.
- `internal/mcp/skills/skills.go` — `Source` interface; `Meta`, `Skill`, `SearchOptions`, `SearchMode`, `SearchField` types; `ErrNotFound`, `ErrInvalidQuery`.
- `internal/mcp/skills/embedded.go` — `//go:embed` directive; `NewEmbedded()` constructor; `embeddedSource` struct; `List`, `Get`, `Search` method bodies.
- `internal/mcp/skills/embedded_test.go` — FS listing, parse-and-validate, List/Get, Search (text+regex+fields+limit+stable order).
- `internal/mcp/skills/skills_symlink_test.go` — `TestSkillsSymlink` asserting `<repo>/skills` resolves to `internal/mcp/skills/embedded/`.
- `internal/mcp/tools_skills.go` — `RegisterSkillTools(r *Registry, src skills.Source)`; three tool handlers.
- `internal/mcp/tools_skills_test.go` — handler tests with an in-package `fakeSource`.
- `internal/skillvalidate/name.go` — `var NameRegex` (one regex, three callers).
- `e2e/api/skills_test.go` — E2E test of `specgraph://skills/<name>` and the three tools through a real server.

### Modified files

- `internal/skillvalidate/validate.go` — require `summary` (≤120 chars after YAML decode); enforce `NameRegex` on `parsed.Name`.
- `internal/skillvalidate/validate_test.go` — new fixtures for missing/overlong-block/overlong-flow summary + non-kebab name + follows-repo-symlink.
- `internal/mcp/resources.go` — `RegisterResources` signature grows; new templated entry for `specgraph://skills/{name}`; new `skillsResourceHandler` + `extractSkillName` helper; `primeResourceHandler` signature grows; prime body adds the `## Skills` pointer section.
- `internal/mcp/resources_test.go` — `TestRegisterResources_Count`, `TestRegisterResources_Templates`, and the prime tests updated for the new signature/count and the pointer section.
- `internal/mcp/server.go` — construct `src := skills.NewEmbedded()` once; thread it through `RegisterResources(reg, client, src)` and into `RegisterSkillTools(reg, src)`.
- `Taskfile.yml` — delete `plugin:sync` target.
- `CLAUDE.md` — rewrite "Plugin shims" and "Shared skills" paragraphs.
- `plugin/specgraph/README.md` — remove symlink language; describe MCP-fetch.
- `plugin/specgraph/routing-guide.md` — name the three new tools.
- `README.md` — verify; edit only if it mentions the symlink/`plugin:sync` pattern.
- All six `SKILL.md` files under `internal/mcp/skills/embedded/<name>/` — add `summary:` frontmatter field.

### Deleted files

- `plugin/specgraph/skills` (symlink).

### Relocations (the `git mv` in Task 1)

- `skills/specgraph-authoring/SKILL.md` → `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md`
- `skills/specgraph-graph-query/SKILL.md` → `internal/mcp/skills/embedded/specgraph-graph-query/SKILL.md`
- `skills/specgraph-analytical-passes/SKILL.md` → `internal/mcp/skills/embedded/specgraph-analytical-passes/SKILL.md`
- `skills/specgraph-drift/SKILL.md` → `internal/mcp/skills/embedded/specgraph-drift/SKILL.md`
- `skills/specgraph-conventions/SKILL.md` → `internal/mcp/skills/embedded/specgraph-conventions/SKILL.md`
- `skills/specgraph-troubleshooting/SKILL.md` → `internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md`

### Symlink created

- `<repo>/skills → internal/mcp/skills/embedded` (single directory symlink, replaces the moved directory).

---

## Task 1: Relocate skills/ canonicals + create repo-root reverse-symlink

**Files:**

- Move: six directories `skills/specgraph-*/` → `internal/mcp/skills/embedded/specgraph-*/`
- Create: `<repo>/skills → internal/mcp/skills/embedded` (directory symlink)
- Create: `internal/mcp/skills/skills_symlink_test.go`

- [ ] **Step 1: Verify starting state**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-f
ls -d skills/specgraph-* | wc -l
```

Expected: `6`

- [ ] **Step 2: Move canonicals + create symlink in one snapshot**

Per design §Sequencing (commit 1), use plain `mv` + `ln -s` and let jj snapshot. Do **not** use `git mv` (redundant) and do **not** modify the `Taskfile.yml`/`validate_test.go` paths (they flow through the new symlink).

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-f
mkdir -p internal/mcp/skills/embedded
mv skills/* internal/mcp/skills/embedded/
rmdir skills
ln -s internal/mcp/skills/embedded skills
```

- [ ] **Step 3: Verify the move**

```bash
ls -d internal/mcp/skills/embedded/specgraph-* | wc -l
readlink skills
ls skills/specgraph-authoring/SKILL.md
```

Expected:

```text
6
internal/mcp/skills/embedded
internal/mcp/skills/embedded/specgraph-authoring/SKILL.md   (resolves via symlink)
```

- [ ] **Step 4: Write the symlink test**

Create `internal/mcp/skills/skills_symlink_test.go` (parallels `internal/config/managedfiles/symlink_pluginshim_test.go`):

```go
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
	// repoRoot = three levels up from internal/mcp/skills/
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
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
```

- [ ] **Step 5: Run task check**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-f
task check
```

Expected: PASS. `task skills:validate` (inside `task check`) resolves `./skills` through the symlink and validates the six canonical SKILL.md files unchanged. `TestValidateRoots_RealSkills` (using `../../skills`) similarly passes.

- [ ] **Step 6: Verify jj snapshot looks right**

```bash
jj --no-pager diff --summary
```

Expected: shows six SKILL.md paths *renamed* (`R skills/... -> internal/mcp/skills/embedded/...`), the new `skills` symlink, and the new `skills_symlink_test.go` — not deletes plus adds.

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "refactor(skills): relocate canonicals into internal/mcp/skills/embedded/

Move the six skill canonicals from <repo>/skills/ to
internal/mcp/skills/embedded/ so //go:embed can read them from inside
the package. Replace <repo>/skills with a reverse-symlink so existing
consumers (task skills:validate ./skills, the GitHub UI, validator
test fixture at ../../skills) keep working unchanged.

Adds TestSkillsSymlink to pin the link target — a future change that
deletes the symlink without redirecting consumers would otherwise
silently break task skills:validate.

Per design §Sequencing (commit 1) at
docs/plans/2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 2: Add `summary` frontmatter field to all six SKILL.md files

**Files:**

- Modify: all six `internal/mcp/skills/embedded/specgraph-*/SKILL.md`

- [ ] **Step 1: Pre-flight read of one file**

```bash
head -10 internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
```

Expected: shows current YAML frontmatter with `name`, `description`, `license`, `metadata`.

- [ ] **Step 2: Edit each SKILL.md to insert `summary:` between `name:` and `description:`**

Use the following exact values (each ≤120 chars):

| File | New `summary:` line |
|------|---------------------|
| `specgraph-authoring/SKILL.md` | `summary: Author a SpecGraph spec through the spark → shape → specify → decompose → approve funnel.` |
| `specgraph-graph-query/SKILL.md` | `summary: Query specs by relationships, status, or stage — ready work, blocked work, impact of a change.` |
| `specgraph-analytical-passes/SKILL.md` | `summary: Run analytical passes (constitution check, dependency check, decision capture) on a spec.` |
| `specgraph-drift/SKILL.md` | `summary: Detect, acknowledge, or fix drift between specs and their upstream dependencies.` |
| `specgraph-conventions/SKILL.md` | `summary: SpecGraph slug, edge-type, and authoring conventions the model should follow.` |
| `specgraph-troubleshooting/SKILL.md` | `summary: Diagnose stuck specs, drift loops, MCP errors, and other SpecGraph runtime issues.` |

For each file, insert the corresponding `summary:` line as a new line after the `name:` line. Example for `specgraph-authoring/SKILL.md`:

```yaml
---
name: specgraph-authoring
summary: Author a SpecGraph spec through the spark → shape → specify → decompose → approve funnel.
description: Use when the user wants to author or update a SpecGraph spec...
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---
```

- [ ] **Step 3: Verify every file has a `summary:` and the value is ≤120 chars**

```bash
for f in internal/mcp/skills/embedded/specgraph-*/SKILL.md; do
  s=$(awk '/^summary:/{sub(/^summary: */, ""); print; exit}' "$f")
  echo "$(echo -n "$s" | wc -c) chars : $f"
done
```

Expected: six lines, each showing a positive char count ≤ 120.

- [ ] **Step 4: Run `task check`**

```bash
task check
```

Expected: PASS (`task skills:validate` doesn't yet require `summary` — that lands in Task 3 — so any value is fine here).

- [ ] **Step 5: Commit**

```bash
jj --no-pager describe -m "feat(skills): add summary frontmatter field to all 6 SKILL.md

Adds a new 'summary' field (≤120 chars after YAML decode) to each of
the six SpecGraph SKILL.md files. The summary is what
specgraph_skills_list and specgraph_skills_search return per skill
(rows of {name, summary, uri}); the longer 'description' paragraph
stays as-is in the SKILL.md body.

The field is a SpecGraph-local extension to the agentskills.io
schema; documented in the upcoming internal/mcp/skills package
comment and required by skillvalidate in commit 3.

Per design §SKILL.md schema change.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 3: Add `skillvalidate.NameRegex` + enforce kebab-case name + `summary` validation

**Files:**

- Create: `internal/skillvalidate/name.go`
- Modify: `internal/skillvalidate/validate.go` (require `summary`, enforce `NameRegex`)
- Modify: `internal/skillvalidate/validate_test.go` (5 new tests + extend `AcceptsValidSkill` fixture)

- [ ] **Step 1: Write the failing tests**

Append to `internal/skillvalidate/validate_test.go`:

```go
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
```

Also extend the existing `TestValidateRoots_AcceptsValidSkill` fixture (currently at `validate_test.go:30-46`) so its frontmatter includes `summary: A perfectly fine summary.` between `name:` and `description:`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/skillvalidate/ -run 'TestValidateRoots_(Rejects|Follows|Accepts)' -v
```

Expected: the four `Rejects*` tests fail with no validation failure (validator accepts); `Follows*` passes (already works via `os.Stat` resolving the symlink); `AcceptsValidSkill` may pass (existing logic) or fail depending on whether the YAML decoder rejects the duplicate keys — verify before fixing.

- [ ] **Step 3: Create `internal/skillvalidate/name.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skillvalidate

import "regexp"

// NameRegex is the canonical pattern for skill directory and frontmatter
// names. Kebab-case ASCII: lowercase alphanumerics with single hyphens
// between segments. Used by:
//   - the validator (this package) to reject malformed names at build time
//   - internal/mcp/skills.NewEmbedded() to reject malformed names at server
//     startup
//   - internal/mcp/resources.go's extractSkillName helper to reject malformed
//     URIs at request time
//
// One regex, three callers — keeps "what counts as a valid skill name" in
// one place.
var NameRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
```

- [ ] **Step 4: Add `Summary` to `Frontmatter` struct + enforce in `validateFile`**

In `internal/skillvalidate/validate.go`:

Add the new field to the `Frontmatter` struct (around L30):

```go
type Frontmatter struct {
	Name        string `yaml:"name"`
	Summary     string `yaml:"summary"`
	Description string `yaml:"description"`
}
```

Add the summary constant alongside `minDesc`/`maxDesc` (around L45-48):

```go
const (
	minDesc    = 1
	maxDesc    = 1024
	maxSummary = 120
)
```

After the existing `parsed.Name != dirName` check (around L106-111), add the kebab-case check:

```go
	if parsed.Name != "" && !NameRegex.MatchString(parsed.Name) {
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("frontmatter.name=%q is not kebab-case ASCII (regex: %s)", parsed.Name, NameRegex.String()))
	}
```

After the description checks (around L116-123 — the existing `case len(desc) > maxDesc:` block), add the summary checks:

```go
	summary := strings.TrimSpace(parsed.Summary)
	switch {
	case summary == "":
		res.Reasons = append(res.Reasons, "frontmatter.summary is required")
	case len([]rune(summary)) > maxSummary:
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("frontmatter.summary too long (%d > %d chars after YAML decode)", len([]rune(summary)), maxSummary))
	}
```

Use `len([]rune(...))` rather than `len(...)` so multi-byte characters (e.g., `→` in `specgraph-authoring`'s summary) count as one char.

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/skillvalidate/ -v
```

Expected: all `Rejects*` / `Follows*` / `Accepts*` tests PASS.

- [ ] **Step 6: Run task check to confirm real skills validate**

```bash
task check
```

Expected: PASS. The six real SKILL.md files (each with a `summary` ≤ 120 chars added in Task 2 and kebab-case names by convention) validate.

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(skillvalidate): require summary (≤120 chars) + enforce kebab-case name regex

Two invariants added to internal/skillvalidate:

1. Every SKILL.md frontmatter must include a 'summary' field. Decoded
   length (not source bytes) must be ≤ 120 chars, so block-scalars
   that look short in source but expand under YAML decoding still
   reject. Multi-byte characters count as one rune.

2. The 'name' frontmatter field must match the canonical kebab-case
   regex ^[a-z0-9]+(?:-[a-z0-9]+)*\$. The regex is exported as
   skillvalidate.NameRegex so downstream consumers (the upcoming
   internal/mcp/skills package and the specgraph://skills/<name>
   resource handler) import the same var — one regex, three callers,
   no drift.

Tests cover present/missing summary, overlong flow scalar, overlong
block scalar, non-kebab name, and validation through a directory
symlink (the <repo>/skills reverse-symlink introduced in commit 1).

Per design §Sequencing (commit 3) and §Public surface details.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 4: Skills package skeleton — types, errors, interface

**Files:**

- Create: `internal/mcp/skills/doc.go`
- Create: `internal/mcp/skills/skills.go`

- [ ] **Step 1: Create `doc.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package skills serves the SpecGraph SKILL.md packages embedded in the
// CLI binary. The package exposes a small read-only Source interface
// (List, Get, Search) with one implementation, embeddedSource, backed by
// //go:embed embedded/*/SKILL.md.
//
// SKILL.md schema follows agentskills.io with one SpecGraph-local
// extension: a required 'summary' field (≤120 chars after YAML decode)
// that the catalog tools surface separately from the longer
// 'description' paragraph. The skillvalidate package owns the
// schema invariants — see skillvalidate.NameRegex for the canonical
// skill-name pattern that this package imports rather than redefining.
//
// Source is read-only by design. Future implementations (a dirSource
// reading .specgraph/skills/, a remoteSource fetching from a registry,
// a compositeSource fanning out across both) plug in without touching
// the MCP handlers in internal/mcp. See
// docs/plans/2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md for the
// full design.
package skills
```

- [ ] **Step 2: Create `skills.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"errors"
)

// Source is the read-only catalog interface for SKILL.md packages.
type Source interface {
	List(ctx context.Context) ([]Meta, error)
	Get(ctx context.Context, name string) (Skill, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]Meta, error)
}

// Meta is one catalog row: what specgraph_skills_list and
// specgraph_skills_search return per skill.
type Meta struct {
	Name    string
	Summary string
	URI     string // canonical fetch URI, e.g. "specgraph://skills/specgraph-authoring"
}

// Skill is the full payload returned by Source.Get and by the
// specgraph://skills/<name> resource handler.
type Skill struct {
	Meta
	Body []byte // verbatim SKILL.md bytes (frontmatter + content)
}

// SearchOptions tune Source.Search. Zero value = case-insensitive
// substring search across name, summary, and body, no result cap.
type SearchOptions struct {
	Mode   SearchMode    // Text (default) or Regex
	Fields []SearchField // empty = all of {Name, Summary, Body}
	Limit  int           // 0 = no cap
}

// SearchMode controls how Source.Search interprets the query.
type SearchMode int

const (
	// SearchText is case-insensitive substring matching (default).
	SearchText SearchMode = iota
	// SearchRegex compiles the query as an RE2 regex.
	SearchRegex
)

// SearchField restricts Source.Search to specific fields.
type SearchField int

const (
	// FieldName scans Meta.Name.
	FieldName SearchField = iota
	// FieldSummary scans Meta.Summary.
	FieldSummary
	// FieldBody scans the SKILL.md body bytes.
	FieldBody
)

// ErrNotFound is returned by Source.Get when the requested name is not
// in the catalog. Mapped to connect.CodeNotFound at the handler boundary.
var ErrNotFound = errors.New("skill not found")

// ErrInvalidQuery is returned by Source.Search when the query is empty
// or, in SearchRegex mode, fails to compile. Mapped to
// connect.CodeInvalidArgument at the handler boundary.
var ErrInvalidQuery = errors.New("invalid query")
```

- [ ] **Step 3: Verify the package compiles**

```bash
go build ./internal/mcp/skills/...
```

Expected: build succeeds (no test files yet; no implementation yet — interfaces and types only).

- [ ] **Step 4: Commit (no test commit yet — implementation lands in Task 5)**

Do NOT commit yet. Continue to Task 5 — Tasks 4 and 5 land as one commit.

---

## Task 5: `embeddedSource` constructor + `List` + `Get`

**Files:**

- Create: `internal/mcp/skills/embedded.go`
- Create: `internal/mcp/skills/embedded_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mcp/skills/embedded_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"strings"
	"testing"
)

// Expected names match the six embedded canonicals (relocated in commit 1).
var wantNames = []string{
	"specgraph-analytical-passes",
	"specgraph-authoring",
	"specgraph-conventions",
	"specgraph-drift",
	"specgraph-graph-query",
	"specgraph-troubleshooting",
}

func TestNewEmbedded_LoadsAllSixSkills(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	metas, err := src.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != len(wantNames) {
		t.Fatalf("got %d skills, want %d", len(metas), len(wantNames))
	}
	for i, m := range metas {
		if m.Name != wantNames[i] {
			t.Errorf("[%d] name = %q, want %q (List must be sorted)", i, m.Name, wantNames[i])
		}
		if m.Summary == "" {
			t.Errorf("[%d] %s has empty summary", i, m.Name)
		}
		wantURI := "specgraph://skills/" + m.Name
		if m.URI != wantURI {
			t.Errorf("[%d] URI = %q, want %q", i, m.URI, wantURI)
		}
	}
}

func TestEmbedded_Get_Known(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	sk, err := src.Get(context.Background(), "specgraph-authoring")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sk.Name != "specgraph-authoring" {
		t.Errorf("Name = %q, want %q", sk.Name, "specgraph-authoring")
	}
	if !strings.Contains(string(sk.Body), "name: specgraph-authoring") {
		t.Errorf("body missing name line; got first 200 bytes: %q", string(sk.Body[:min(200, len(sk.Body))]))
	}
}

func TestEmbedded_Get_Unknown(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	_, err = src.Get(context.Background(), "no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !errorsIs(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// Helpers — kept local to avoid pulling in extra imports for tiny needs.
func errorsIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		type unwrap interface{ Unwrap() error }
		u, ok := e.(unwrap)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/mcp/skills/ -run 'TestNewEmbedded|TestEmbedded_Get' -v
```

Expected: FAIL with "undefined: NewEmbedded" — the constructor doesn't exist yet.

- [ ] **Step 3: Create `embedded.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/specgraph/specgraph/internal/skillvalidate"
	"gopkg.in/yaml.v3"
)

//go:embed embedded/*/SKILL.md
var embeddedFS embed.FS

type embeddedSource struct {
	byName map[string]Skill
	order  []string // sorted skill names
}

// NewEmbedded loads and validates every embedded SKILL.md once. Returns
// a Source whose List/Get/Search read from the prebuilt in-memory
// catalog. Any malformed skill (missing required frontmatter, summary
// > 120 chars after YAML decode, non-kebab name, invalid YAML) returns
// a precise error and causes `specgraph serve` startup to fail loudly.
func NewEmbedded() (Source, error) {
	src := &embeddedSource{byName: map[string]Skill{}}

	entries, err := fs.ReadDir(embeddedFS, "embedded")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !skillvalidate.NameRegex.MatchString(name) {
			return nil, fmt.Errorf("skill directory %q is not kebab-case ASCII (regex: %s)",
				name, skillvalidate.NameRegex.String())
		}
		body, err := fs.ReadFile(embeddedFS, path.Join("embedded", name, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read %s/SKILL.md: %w", name, err)
		}
		meta, err := parseFrontmatter(name, body)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", name, err)
		}
		src.byName[name] = Skill{Meta: meta, Body: body}
		src.order = append(src.order, name)
	}
	sort.Strings(src.order)
	return src, nil
}

func (s *embeddedSource) List(ctx context.Context) ([]Meta, error) {
	out := make([]Meta, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.byName[name].Meta)
	}
	return out, nil
}

func (s *embeddedSource) Get(ctx context.Context, name string) (Skill, error) {
	sk, ok := s.byName[name]
	if !ok {
		return Skill{}, ErrNotFound
	}
	return sk, nil
}

// parseFrontmatter extracts the YAML frontmatter, validates required
// fields (name match, summary present + ≤120 chars decoded), and
// returns the Meta. Body bytes are unmodified.
func parseFrontmatter(dirName string, body []byte) (Meta, error) {
	const fence = "---\n"
	bs := string(body)
	if !strings.HasPrefix(bs, fence) {
		return Meta{}, fmt.Errorf("missing leading YAML frontmatter fence")
	}
	rest := bs[len(fence):]
	end := strings.Index(rest, "\n"+fence[:len(fence)-1])
	if end < 0 {
		return Meta{}, fmt.Errorf("unterminated YAML frontmatter")
	}
	var fm struct {
		Name    string `yaml:"name"`
		Summary string `yaml:"summary"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return Meta{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if fm.Name != dirName {
		return Meta{}, fmt.Errorf("frontmatter.name=%q must match dirname %q", fm.Name, dirName)
	}
	if !skillvalidate.NameRegex.MatchString(fm.Name) {
		return Meta{}, fmt.Errorf("frontmatter.name=%q is not kebab-case", fm.Name)
	}
	summary := strings.TrimSpace(fm.Summary)
	if summary == "" {
		return Meta{}, fmt.Errorf("frontmatter.summary is required")
	}
	if len([]rune(summary)) > 120 {
		return Meta{}, fmt.Errorf("frontmatter.summary too long (%d > 120 chars after YAML decode)",
			len([]rune(summary)))
	}
	return Meta{
		Name:    fm.Name,
		Summary: summary,
		URI:     "specgraph://skills/" + fm.Name,
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/mcp/skills/ -v
```

Expected: PASS. `TestNewEmbedded_LoadsAllSixSkills`, `TestEmbedded_Get_Known`, `TestEmbedded_Get_Unknown`, and `TestSkillsSymlink` (from Task 1) all pass.

- [ ] **Step 5: Run task check**

```bash
task check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(mcp/skills): add Source interface + embedded source with eager parse + validate at constructor time

New internal/mcp/skills/ subpackage:

- skills.go defines the read-only Source interface (List, Get,
  Search), Meta, Skill, SearchOptions, and the ErrNotFound +
  ErrInvalidQuery sentinels.
- embedded.go provides NewEmbedded() which loads every embedded
  SKILL.md via //go:embed embedded/*/SKILL.md, validates each
  against skillvalidate.NameRegex (the kebab-case rule shared with
  the validator), parses frontmatter, enforces summary present and
  ≤120 chars after YAML decode, and stores results in a prebuilt
  catalog keyed by name with a sorted name slice for stable List
  order.
- embedded_test.go covers List, Get-known, Get-unknown.

Search lands in commit 5; the three MCP tools and resource handler
land in commits 6-7. Source is read-only by design — future
sources (dirSource, remoteSource, compositeSource) plug into the
same interface without touching the MCP handlers.

Per design §Source interface and §embeddedSource.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 6: Implement `embeddedSource.Search`

**Files:**

- Modify: `internal/mcp/skills/embedded.go` (add Search method)
- Modify: `internal/mcp/skills/embedded_test.go` (add search tests)

- [ ] **Step 1: Write the failing tests**

Append to `internal/mcp/skills/embedded_test.go`:

```go
func TestSearch_TextMatchesAcrossFields(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	// "drift" appears in the drift skill's name and in other bodies.
	results, err := src.Search(context.Background(), "drift", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one match for 'drift'")
	}
	var found bool
	for _, m := range results {
		if m.Name == "specgraph-drift" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected specgraph-drift in results; got %+v", results)
	}
}

func TestSearch_TextCaseInsensitive(t *testing.T) {
	src, _ := NewEmbedded()
	lower, _ := src.Search(context.Background(), "drift", SearchOptions{})
	upper, _ := src.Search(context.Background(), "DRIFT", SearchOptions{})
	if len(lower) != len(upper) {
		t.Errorf("case sensitivity: lower=%d, upper=%d", len(lower), len(upper))
	}
}

func TestSearch_RegexAnchors(t *testing.T) {
	src, _ := NewEmbedded()
	// \bdrift\b matches "drift" but not "drifted" — pins regex mode.
	results, err := src.Search(context.Background(), `\bdrift\b`,
		SearchOptions{Mode: SearchRegex})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected matches for \\bdrift\\b")
	}
}

func TestSearch_RegexInvalidReturnsErrInvalidQuery(t *testing.T) {
	src, _ := NewEmbedded()
	_, err := src.Search(context.Background(), `[unclosed`,
		SearchOptions{Mode: SearchRegex})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !errorsIs(err, ErrInvalidQuery) {
		t.Errorf("got %v, want ErrInvalidQuery", err)
	}
}

func TestSearch_FieldsRestriction(t *testing.T) {
	src, _ := NewEmbedded()
	// Restrict to FieldName: a query that matches body but not name
	// must return zero rows.
	results, err := src.Search(context.Background(), "funnel",
		SearchOptions{Fields: []SearchField{FieldName}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, m := range results {
		if !strings.Contains(strings.ToLower(m.Name), "funnel") {
			t.Errorf("FieldName restriction matched a non-name field: %s", m.Name)
		}
	}
}

func TestSearch_LimitClamps(t *testing.T) {
	src, _ := NewEmbedded()
	// A broad query that matches all six skills.
	results, err := src.Search(context.Background(), "spec",
		SearchOptions{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Limit=2 not honored; got %d rows", len(results))
	}
}

func TestSearch_StableOrder(t *testing.T) {
	src, _ := NewEmbedded()
	a, _ := src.Search(context.Background(), "spec", SearchOptions{})
	b, _ := src.Search(context.Background(), "spec", SearchOptions{})
	if len(a) != len(b) {
		t.Fatalf("len differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Errorf("[%d] order differs: %q vs %q", i, a[i].Name, b[i].Name)
		}
	}
}

func TestSearch_EmptyQueryReturnsErrInvalidQuery(t *testing.T) {
	src, _ := NewEmbedded()
	_, err := src.Search(context.Background(), "", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !errorsIs(err, ErrInvalidQuery) {
		t.Errorf("got %v, want ErrInvalidQuery", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/mcp/skills/ -run TestSearch -v
```

Expected: FAIL with `embeddedSource does not implement Source (missing Search method)` or similar.

- [ ] **Step 3: Implement `Search`**

Append to `internal/mcp/skills/embedded.go` (add `"regexp"` to the import block):

```go
func (s *embeddedSource) Search(ctx context.Context, query string, opts SearchOptions) ([]Meta, error) {
	if query == "" {
		return nil, ErrInvalidQuery
	}
	fields := opts.Fields
	if len(fields) == 0 {
		fields = []SearchField{FieldName, FieldSummary, FieldBody}
	}

	matchText := func(haystack, needle string) bool {
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}

	var rx *regexp.Regexp
	if opts.Mode == SearchRegex {
		var err error
		rx, err = regexp.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
		}
	}

	matches := func(text string) bool {
		if rx != nil {
			return rx.MatchString(text)
		}
		return matchText(text, query)
	}

	matchesSkill := func(sk Skill) bool {
		for _, f := range fields {
			switch f {
			case FieldName:
				if matches(sk.Name) {
					return true
				}
			case FieldSummary:
				if matches(sk.Summary) {
					return true
				}
			case FieldBody:
				if matches(string(sk.Body)) {
					return true
				}
			}
		}
		return false
	}

	out := make([]Meta, 0, len(s.order))
	for _, name := range s.order {
		sk := s.byName[name]
		if !matchesSkill(sk) {
			continue
		}
		out = append(out, sk.Meta)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/mcp/skills/ -v
```

Expected: all `TestSearch_*` PASS along with the earlier Task-5 tests.

- [ ] **Step 5: `task check`**

```bash
task check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(mcp/skills): implement Search (text + regex) on embedded source

embeddedSource.Search walks the prebuilt catalog in sorted order
applying a predicate built from SearchOptions:

- SearchText (default): case-insensitive substring (strings.Contains
  on lowercased fields).
- SearchRegex: regexp.Compile (RE2 — no catastrophic backtracking);
  invalid pattern returns ErrInvalidQuery wrapping the compile error.

SearchOptions.Fields restricts which of {Name, Summary, Body} get
scanned (default: all three). SearchOptions.Limit clamps the result
slice (0 = no cap). Empty query returns ErrInvalidQuery.

Tests cover all branches: text matches across fields, case
insensitivity, regex anchors (\\bdrift\\b matches drift not drifted),
invalid regex, fields restriction, limit clamping, stable order
across repeated queries, and the empty-query error.

Per design §embeddedSource (Search section).

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 7: `tools_skills.go` — register and handle the three skill tools

**Files:**

- Create: `internal/mcp/tools_skills.go`
- Create: `internal/mcp/tools_skills_test.go`
- Modify: `internal/mcp/server.go` (introduce `src`, call `RegisterSkillTools`)

- [ ] **Step 1: Write the failing tests**

Create `internal/mcp/tools_skills_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/mcp/skills"
)

// fakeSource is an in-package test double satisfying skills.Source.
// Used by the handler tests so they don't depend on the embedded
// canonicals (which are exercised separately in internal/mcp/skills).
type fakeSource struct {
	entries []skills.Skill
}

func (f *fakeSource) List(ctx context.Context) ([]skills.Meta, error) {
	out := make([]skills.Meta, len(f.entries))
	for i, e := range f.entries {
		out[i] = e.Meta
	}
	return out, nil
}

func (f *fakeSource) Get(ctx context.Context, name string) (skills.Skill, error) {
	for _, e := range f.entries {
		if e.Name == name {
			return e, nil
		}
	}
	return skills.Skill{}, skills.ErrNotFound
}

func (f *fakeSource) Search(ctx context.Context, query string, opts skills.SearchOptions) ([]skills.Meta, error) {
	if query == "" {
		return nil, skills.ErrInvalidQuery
	}
	var out []skills.Meta
	for _, e := range f.entries {
		if strings.Contains(strings.ToLower(e.Name+" "+e.Summary), strings.ToLower(query)) {
			out = append(out, e.Meta)
		}
	}
	return out, nil
}

func twoSkillFake() *fakeSource {
	return &fakeSource{entries: []skills.Skill{
		{Meta: skills.Meta{Name: "alpha", Summary: "first", URI: "specgraph://skills/alpha"}, Body: []byte("---\nname: alpha\n---\nbody-a")},
		{Meta: skills.Meta{Name: "beta", Summary: "second", URI: "specgraph://skills/beta"}, Body: []byte("---\nname: beta\n---\nbody-b")},
	}}
}

func TestRegisterSkillTools_RegistersThreeTools(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	for _, name := range []string{"specgraph_skills_list", "specgraph_skills_get", "specgraph_skills_search"} {
		if _, ok := r.LookupTool(name); !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestSpecgraphSkillsList_ReturnsCatalog(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_list")
	res, err := def.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !strings.Contains(res.Text(), `"alpha"`) || !strings.Contains(res.Text(), `"beta"`) {
		t.Errorf("expected both skill names in output; got %s", res.Text())
	}
}

func TestSpecgraphSkillsGet_KnownAndUnknown(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_get")

	res, err := def.Handler(context.Background(), map[string]any{"name": "alpha"})
	if err != nil {
		t.Fatalf("handler(alpha): %v", err)
	}
	if !strings.Contains(res.Text(), "body-a") {
		t.Errorf("expected body-a in payload; got %s", res.Text())
	}

	_, err = def.Handler(context.Background(), map[string]any{"name": "no-such"})
	if err == nil {
		t.Error("expected error for unknown name")
	}

	_, err = def.Handler(context.Background(), map[string]any{"name": ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestSpecgraphSkillsSearch_TextAndRegex(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_search")

	res, err := def.Handler(context.Background(), map[string]any{"query": "first"})
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if !strings.Contains(res.Text(), "alpha") || strings.Contains(res.Text(), "beta") {
		t.Errorf("text search should match alpha only; got %s", res.Text())
	}

	_, err = def.Handler(context.Background(), map[string]any{"query": ""})
	if err == nil {
		t.Error("expected error for empty query")
	}
	if !errors.Is(err, skills.ErrInvalidQuery) {
		t.Errorf("got %v, want ErrInvalidQuery", err)
	}
}
```

(Note: `ToolResult.Text()` is the existing helper used by other tool tests in this package — see e.g. `tools_authoring_test.go`. If it's spelled differently in the codebase, match the existing convention.)

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/mcp/ -run 'TestRegisterSkillTools|TestSpecgraphSkills' -v
```

Expected: FAIL with `undefined: RegisterSkillTools`.

- [ ] **Step 3: Create `tools_skills.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/specgraph/specgraph/internal/mcp/skills"
)

// RegisterSkillTools registers the three MCP tools that expose the skills
// catalog to model-side callers: specgraph_skills_list,
// specgraph_skills_get, and specgraph_skills_search. The src argument is
// the live skills.Source built once at server startup (see server.go).
//
// Tool schemas use the existing objectSchema/stringProp/boolProp builders
// from helpers.go; handlers read parameters via stringParam/boolParam.
// No typed-struct args — matches the convention used by every other
// tools_*.go file in this package.
func RegisterSkillTools(r *Registry, src skills.Source) {
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_list",
		Description: "List the available SpecGraph skills (one-line summary per skill).",
		Profile:     ProfileCore,
		Schema:      objectSchema(props{}),
		Handler:     skillsListHandler(src),
	})
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_get",
		Description: "Fetch a single SpecGraph skill's SKILL.md body by name.",
		Profile:     ProfileCore,
		Schema: objectSchema(
			props{"name": stringProp("Skill name (kebab-case, e.g. specgraph-authoring)")},
			"name",
		),
		Handler: skillsGetHandler(src),
	})
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_search",
		Description: "Search the SpecGraph skills catalog by keyword (substring) or RE2 regex.",
		Profile:     ProfileCore,
		Schema: objectSchema(
			props{
				"query": stringProp("Substring (default) or RE2 regex"),
				"regex": boolProp("Treat query as RE2 regex; defaults to false. Must be a JSON boolean, not the string \"true\""),
			},
			"query",
		),
		Handler: skillsSearchHandler(src),
	})
}

func skillsListHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		metas, err := src.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list skills: %w", err)
		}
		body, err := json.MarshalIndent(metas, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		return TextResult(string(body)), nil
	}
}

func skillsGetHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		name := stringParam(params, "name")
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
		}
		sk, err := src.Get(ctx, name)
		if err != nil {
			if errors.Is(err, skills.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, fmt.Errorf("get %s: %w", name, err)
		}
		return TextResult(string(sk.Body)), nil
	}
}

func skillsSearchHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		query := stringParam(params, "query")
		if query == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, skills.ErrInvalidQuery)
		}
		mode := skills.SearchText
		if boolParam(params, "regex") {
			mode = skills.SearchRegex
		}
		metas, err := src.Search(ctx, query, skills.SearchOptions{Mode: mode})
		if err != nil {
			if errors.Is(err, skills.ErrInvalidQuery) {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			return nil, fmt.Errorf("search %q: %w", query, err)
		}
		body, err := json.MarshalIndent(metas, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		return TextResult(string(body)), nil
	}
}
```

(Note: `TextResult` and `ToolHandler` are the existing helpers/types in this package — see `tools_authoring.go` for the actual spellings. If they differ slightly from the spellings above, update to match.)

- [ ] **Step 4: Wire the source into `server.go`**

In `internal/mcp/server.go`, find the existing block calling `RegisterResources(reg, client)` (around L41-42). Replace with:

```go
	skillsSrc, err := skills.NewEmbedded()
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}

	RegisterSpecTools(reg, client)
	RegisterGraphTools(reg, client)
	RegisterCoreTools(reg, client)
	RegisterAuthoringTools(reg, client)
	RegisterLifecycleTools(reg, client)
	RegisterExecutionTools(reg, client)
	RegisterResources(reg, client)
	RegisterPrompts(reg, client)
	RegisterSkillTools(reg, skillsSrc)
```

Add the import:

```go
	"github.com/specgraph/specgraph/internal/mcp/skills"
```

The `RegisterResources` signature stays `(reg, client)` in this commit — it grows in Task 8 (commit 7). Confirm by checking the `internal/mcp/server.go` function the calls sit inside returns an `error` so the new fatal path compiles; if not, route the err appropriately (e.g., `log.Fatalf` if the function returns nothing).

- [ ] **Step 5: Run tests**

```bash
go test ./internal/mcp/ -run 'TestRegisterSkillTools|TestSpecgraphSkills' -v
```

Expected: PASS.

- [ ] **Step 6: `task check`**

```bash
task check
```

Expected: PASS. The pre-existing `TestRegisterResources_Count` still passes (it asserts `Len == 10`; we haven't added the skills resource yet — that's Task 8).

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(mcp): add specgraph_skills_list, _get, _search tools

New internal/mcp/tools_skills.go with RegisterSkillTools(r, src),
following the existing per-domain pattern (tools_authoring.go,
tools_core.go, etc.). Three tools:

- specgraph_skills_list: returns the full catalog as JSON rows
  ({name, summary, uri}).
- specgraph_skills_get: returns the verbatim SKILL.md body for a
  named skill. ErrNotFound maps to CodeNotFound; empty name maps
  to CodeInvalidArgument.
- specgraph_skills_search: text (default) or RE2 regex search via
  the optional 'regex' bool arg. ErrInvalidQuery maps to
  CodeInvalidArgument.

Tool schemas use objectSchema(props{...}, required...) +
stringProp/boolProp from helpers.go; handlers read params via
stringParam/boolParam. No typed-struct args — matches every other
tools_*.go file.

internal/mcp/server.go constructs skills.NewEmbedded() once at
startup, treats a non-nil err as fatal (the embedded catalog
should always parse — if it doesn't, the binary is broken), and
hands the Source to RegisterSkillTools. RegisterResources stays
unchanged in this commit; its signature grows in commit 7.

Per design §Sequencing (commit 6) and §Public surface details.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 8: `specgraph://skills/<name>` resource handler

**Files:**

- Modify: `internal/mcp/resources.go` (add `extractSkillName`, `skillsResourceHandler`, templated registration; grow `RegisterResources` signature)
- Modify: `internal/mcp/resources_test.go` (update `TestRegisterResources_Count`, `TestRegisterResources_Templates`; add `TestSkillsResourceHandler_*` tests)
- Modify: `internal/mcp/server.go` (pass `skillsSrc` to `RegisterResources`)

- [ ] **Step 1: Update existing tests + add new tests (TDD)**

In `internal/mcp/resources_test.go`:

Update both existing callers at L386 and L393:

```go
func TestRegisterResources_Count(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterResources(r, c, &fakeSource{}) // signature grows; fakeSource defined in tools_skills_test.go
	require.Len(t, r.Resources(), 11)      // +1 for skills templated entry
}

func TestRegisterResources_Templates(t *testing.T) {
	c := &Client{}
	r := NewRegistry()
	RegisterResources(r, c, &fakeSource{})
	// ... existing body; add `specgraph://skills/{name}` to templateURIs map.
}
```

Append new handler tests:

```go
func TestSkillsResourceHandler_KnownAndUnknown(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var skillsHandler ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://skills/{name}" {
			skillsHandler = res.Handler
			break
		}
	}
	if skillsHandler == nil {
		t.Fatal("skills resource not registered")
	}

	contents, err := skillsHandler(context.Background(), "specgraph://skills/alpha")
	if err != nil {
		t.Fatalf("known: %v", err)
	}
	if len(contents) != 1 || !strings.Contains(string(contents[0].Text), "body-a") {
		t.Errorf("expected body-a; got %+v", contents)
	}

	_, err = skillsHandler(context.Background(), "specgraph://skills/no-such")
	if err == nil {
		t.Error("expected error for unknown name")
	}
}

func TestSkillsResourceHandler_RejectsMalformedURI(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var h ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://skills/{name}" {
			h = res.Handler
			break
		}
	}

	rejects := []string{
		"specgraph://skills",
		"specgraph://skills/",
		"specgraph://skills//",
		"specgraph://skills/foo/",
		"specgraph://skills/foo/bar",
		"specgraph://SKILLS/foo",
		"specgraph://skills/Foo",
		"specgraph://skills/foo%20bar",
	}
	for _, uri := range rejects {
		if _, err := h(context.Background(), uri); err == nil {
			t.Errorf("expected reject for %q", uri)
		}
	}

	if _, err := h(context.Background(), "specgraph://skills/alpha"); err != nil {
		t.Errorf("expected accept for /alpha; got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/mcp/ -run 'TestRegisterResources|TestSkillsResourceHandler' -v
```

Expected: FAIL with compile errors (signature mismatch + undefined `extractSkillName`).

- [ ] **Step 3: Add `extractSkillName` + `skillsResourceHandler` to `resources.go`**

Add the new helper next to `extractSlugFromURI`:

```go
// extractSkillName parses a specgraph://skills/<name> URI and validates
// the name against skillvalidate.NameRegex. Returns the validated name
// or an error mapped at the call site to connect.CodeNotFound.
//
// Strict by design: rejects subpaths (specgraph://skills/foo/bar),
// trailing slashes, empty names, mixed-case scheme segment, and any
// name failing the kebab-case regex. URL-encoded names are rejected
// (skill names never need encoding by convention).
func extractSkillName(uri string) (string, error) {
	rest := strings.TrimPrefix(uri, "specgraph://")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed skills URI %q", uri)
	}
	if parts[0] != "skills" {
		return "", fmt.Errorf("not a skills URI: %q", uri)
	}
	if !skillvalidate.NameRegex.MatchString(parts[1]) {
		return "", fmt.Errorf("invalid skill name %q in URI", parts[1])
	}
	return parts[1], nil
}
```

Add the handler near `primeResourceHandler`:

```go
// skillsResourceHandler — specgraph://skills/{name}
func skillsResourceHandler(src skills.Source) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		name, err := extractSkillName(uri)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		sk, err := src.Get(ctx, name)
		if err != nil {
			if errors.Is(err, skills.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, fmt.Errorf("get %s: %w", name, err)
		}
		return []ResourceContent{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     string(sk.Body),
		}}, nil
	}
}
```

Grow `RegisterResources` to accept `skills.Source` and register the new templated entry. The signature change touches the existing function definition and the place where the resources slice is built. Add a new entry to the slice (matching the existing template-entry shape used by `specgraph://spec/{slug}` at L369-373):

```go
func RegisterResources(r *Registry, c *Client, src skills.Source) {
	// ... existing entries unchanged ...
	r.AddResource(ResourceDef{
		URI:         "specgraph://skills/{name}",
		Name:        "SpecGraph skill",
		Description: "A single SpecGraph SKILL.md package by name.",
		MIMEType:    "text/markdown",
		IsTemplate:  true,
		Handler:     skillsResourceHandler(src),
	})
}
```

Add imports as needed (`errors`, `github.com/specgraph/specgraph/internal/mcp/skills`, `github.com/specgraph/specgraph/internal/skillvalidate`).

- [ ] **Step 4: Update the caller in `server.go`**

```go
RegisterResources(reg, client, skillsSrc)
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/mcp/ -v
```

Expected: PASS (`TestRegisterResources_Count` now asserts 11; `TestSkillsResourceHandler_*` tests pass; old tests continue to pass).

- [ ] **Step 6: `task check`**

```bash
task check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "feat(mcp): add specgraph://skills/<name> resource handler

New templated resource specgraph://skills/{name} returns the verbatim
SKILL.md bytes for a named skill. The handler:

- parses the URI through a new extractSkillName helper (a strict
  sibling of the existing extractSlugFromURI — rejects subpaths,
  trailing slashes, empty names, mixed-case scheme, URL-encoded
  names, and any name failing skillvalidate.NameRegex);
- calls src.Get, maps ErrNotFound to connect.CodeNotFound;
- returns ResourceContent with MIMEType text/markdown.

RegisterResources signature grows from (r, c) to (r, c, src
skills.Source). The single caller in server.go reuses the same
src constructed in commit 6's RegisterSkillTools call (no new
NewEmbedded). TestRegisterResources_Count moves to expect 11
entries; TestRegisterResources_Templates picks up the new
templated URI; new tests cover known/unknown name and the
malformed-URI rejection enumeration from the design.

Per design §Sequencing (commit 7) and §Resource —
specgraph://skills/<name>.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 9: Prime template — `## Skills` pointer section

**Files:**

- Modify: `internal/mcp/resources.go` (grow `primeResourceHandler` signature; insert section in body)
- Modify: `internal/mcp/resources_test.go` (update prime tests for new signature; add section assertion)
- Modify: `internal/mcp/server.go` (no change — wiring already established)

- [ ] **Step 1: Update prime tests (TDD)**

Find the existing `TestPrimeResource_*` tests in `resources_test.go` (around L465+ per design). Update the handler-construction call(s) from `primeResourceHandler(c)` to `primeResourceHandler(c, src)` where `src` is a `fakeSource` constructed locally with one or more entries.

Append a new test:

```go
func TestPrime_IncludesSkillsPointer(t *testing.T) {
	src := twoSkillFake()
	r := NewRegistry()
	RegisterResources(r, &Client{}, src)

	var primeHandler ResourceHandler
	for _, res := range r.Resources() {
		if res.URI == "specgraph://prime" {
			primeHandler = res.Handler
			break
		}
	}
	if primeHandler == nil {
		t.Fatal("prime resource not registered")
	}
	contents, err := primeHandler(context.Background(), "specgraph://prime")
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	body := string(contents[0].Text)

	for _, want := range []string{
		"## Skills",
		"specgraph_skills_list",
		"specgraph_skills_search",
		"specgraph_skills_get",
		"specgraph://skills/",
		"2 skills", // live count from fake
	} {
		if !strings.Contains(body, want) {
			t.Errorf("prime missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/mcp/ -run 'TestPrime' -v
```

Expected: FAIL (compile error: signature mismatch + missing assertion).

- [ ] **Step 3: Grow `primeResourceHandler` signature + add the section**

In `internal/mcp/resources.go`, change:

```go
func primeResourceHandler(c *Client) ResourceHandler {
```

to:

```go
func primeResourceHandler(c *Client, src skills.Source) ResourceHandler {
```

Inside the closure body, after the existing "Ready to Work" section (around L300 per design notes), splice in the new pointer block:

```go
		metas, err := src.List(ctx)
		switch {
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "skills"),
				slog.String("err", err.Error()))
			b.WriteString("## Skills\n\n_(unable to load: " + err.Error() + ")_\n\n")
		case len(metas) > 0:
			fmt.Fprintf(&b, "## Skills\n\n%d skills exposed via MCP. ", len(metas))
			b.WriteString("Use `specgraph_skills_list` to see the catalog, ")
			b.WriteString("`specgraph_skills_search` to find one by keyword, ")
			b.WriteString("and `specgraph_skills_get` / `specgraph://skills/<name>` ")
			b.WriteString("to fetch a specific skill.\n\n")
		}
```

Update the `RegisterResources` call that wires the prime handler so it passes `src`:

```go
		Handler: primeResourceHandler(c, src),
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/mcp/ -v
```

Expected: PASS, including the new `TestPrime_IncludesSkillsPointer`.

- [ ] **Step 5: `task check`**

```bash
task check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "feat(mcp): update prime template with skills pointer section

primeResourceHandler signature grows from (c) to (c, src
skills.Source) so the handler can read the live catalog count from
src.List(ctx) and template it into the digest. Adds a new '## Skills'
section after 'Ready to Work':

  ## Skills

  6 skills exposed via MCP. Use specgraph_skills_list to see the
  catalog, specgraph_skills_search to find one by keyword, and
  specgraph_skills_get / specgraph://skills/<name> to fetch a
  specific skill.

Pointer-only by design — no per-skill enumeration (see design
§Deliberate departures from the parent epic). The skill count
updates automatically as the embedded catalog grows. List errors
emit the standard '_(unable to load: ...)_' pattern matching the
other prime sections.

Per design §Sequencing (commit 8) and §Resource —
specgraph://prime (modified).

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 10: Delete `plugin/specgraph/skills` symlink + `task plugin:sync`

**Files:**

- Delete: `plugin/specgraph/skills` (symlink)
- Modify: `Taskfile.yml` (remove `plugin:sync` target)
- Search: any other references to `plugin:sync` repo-wide

- [ ] **Step 1: Confirm the symlink and target exist**

```bash
ls -la plugin/specgraph/skills
```

Expected: shows a symlink (`l...`) pointing at `../../skills`.

- [ ] **Step 2: Delete the symlink**

```bash
rm plugin/specgraph/skills
```

- [ ] **Step 3: Remove the `plugin:sync` target from `Taskfile.yml`**

Find the `plugin:sync:` block in `Taskfile.yml` and delete it (the entire `desc:` + `cmds:` block under the key). Also search for references:

```bash
grep -rn "plugin:sync\|task plugin:sync" Taskfile.yml CONTRIBUTING.md docs/ README.md
```

Update or remove any prose hits. Do **not** touch `CLAUDE.md` or `plugin/specgraph/README.md` — those are rewritten in Task 11.

- [ ] **Step 4: `task --list` to confirm the target is gone**

```bash
task --list 2>&1 | grep plugin:sync
```

Expected: no output (target removed).

- [ ] **Step 5: `task check`**

```bash
task check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe -m "chore: delete plugin/specgraph/skills symlink and task plugin:sync

plugin/specgraph/skills was a dev-time symlink that pointed at the
repo-root skills/ tree. It existed only so authors editing through
plugin/specgraph/ saw skill content; end-user projects never had
this symlink. With PR F serving skills via MCP fetch
(specgraph://skills/<name> + the three tools), the symlink is
vestigial.

task plugin:sync's only job was refreshing this symlink — also
removed.

References to 'task plugin:sync' in prose (CONTRIBUTING.md, docs,
top-level README) cleaned up. CLAUDE.md and plugin/specgraph/README.md
are rewritten in commit 10.

Per design §Sequencing (commit 9) and §Cleanup landed in this PR.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 11: Documentation — CLAUDE.md, plugin/specgraph/README.md, routing-guide.md, top-level README

**Files:**

- Modify: `CLAUDE.md` (rewrite "Plugin shims" + "Shared skills" paragraphs)
- Modify: `plugin/specgraph/README.md` (replace symlink language; describe MCP fetch)
- Modify: `plugin/specgraph/routing-guide.md` (name the three new tools)
- Modify: `README.md` (top-level — verify, edit only if needed)

- [ ] **Step 1: Rewrite CLAUDE.md "Plugin shims" paragraph**

In `CLAUDE.md`, find the paragraph starting with `**Plugin shims** —` (around L116 per current state). Replace the sentence about symlinks and `task plugin:sync` with prose describing MCP-fetch delivery. The rewrite should:

- State that skills live in the binary (`//go:embed`) and are served via MCP.
- Name the three tools (`specgraph_skills_list`, `_get`, `_search`) and the resource URI (`specgraph://skills/<name>`).
- Drop the line about "All three symlink `skills/` to the in-tree `skills/`".
- Drop the `task plugin:sync` reference.

Example replacement text (adapt the surrounding sentence flow to match the existing paragraph):

```markdown
**Plugin shims** — `plugin/specgraph/` (Claude Code), `plugin/cursor/`
(Cursor), and `plugin/opencode/` (OpenCode) are thin per-harness shims
that consume SpecGraph's skills via MCP fetch. Skills live in the CLI
binary as embedded canonicals (`internal/mcp/skills/embedded/`) and are
served via three tools (`specgraph_skills_list`, `specgraph_skills_get`,
`specgraph_skills_search`) plus a templated resource
(`specgraph://skills/<name>`). No on-disk skill copies in end-user
projects. See `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
for the parity contract.
```

- [ ] **Step 2: Rewrite CLAUDE.md "Shared skills" paragraph**

Find the `**Shared skills** —` paragraph (next paragraph in the same section). Replace to describe MCP-fetch delivery instead of in-tree symlinks. Keep the reference to `task skills:validate` (which still works via the new repo-root symlink). Example:

```markdown
**Shared skills** — Six skills (`specgraph-authoring`,
`specgraph-graph-query`, `specgraph-analytical-passes`,
`specgraph-drift`, `specgraph-conventions`,
`specgraph-troubleshooting`) live as agentskills.io-shape SKILL.md
packages under `internal/mcp/skills/embedded/`. The repo-root
`skills/` is a reverse-symlink to that directory so authors and the
GitHub UI keep using the familiar path. Skills are served at runtime
via MCP: `specgraph_skills_list` for the catalog,
`specgraph_skills_get` (or the `specgraph://skills/<name>` resource)
for a specific skill, `specgraph_skills_search` for keyword/regex
lookup. `task skills:validate` (wired into `task check`) gates each
SKILL.md against the schema, including the SpecGraph-local
`summary:` extension and the kebab-case `name` regex
(`skillvalidate.NameRegex`).
```

- [ ] **Step 3: Rewrite plugin/specgraph/README.md**

In `plugin/specgraph/README.md`, remove any text mentioning the now-deleted `plugin/specgraph/skills` symlink or `task plugin:sync`. Replace with a short note pointing at the MCP tools/resource. Keep the rest of the README (plugin manifest, hooks, etc.) untouched.

- [ ] **Step 4: Update plugin/specgraph/routing-guide.md**

Add a section (or extend an existing one) naming the three new tools. Example addition:

```markdown
## Loading a SpecGraph skill

When the conversation calls for SpecGraph guidance, fetch the skill
via MCP rather than relying on local files:

- `specgraph_skills_list` — see what's available.
- `specgraph_skills_search` — keyword/regex lookup
  (`{"query": "drift"}` or `{"query": "\\bdrift\\b", "regex": true}`).
- `specgraph_skills_get` — fetch a specific skill by name
  (`{"name": "specgraph-authoring"}`).
- `specgraph://skills/<name>` — the same payload as `_get` via the
  resource URI.
```

- [ ] **Step 5: Top-level README.md check**

```bash
grep -n "plugin:sync\|plugin/specgraph/skills" README.md
```

Expected: no hits (nothing to edit). If hits exist, update them to MCP-fetch language matching CLAUDE.md.

- [ ] **Step 6: `task check`**

```bash
task check
```

Expected: PASS (linters check markdown but `task check` doesn't gate on prose accuracy; this step confirms no formatting regressions).

- [ ] **Step 7: Commit**

```bash
jj --no-pager describe -m "docs: rewrite CLAUDE.md plugin shims + shared skills sections

PR F replaces in-tree skill symlinks with MCP-fetch delivery; the
prose in CLAUDE.md, plugin/specgraph/README.md, and routing-guide.md
needs to match.

- CLAUDE.md 'Plugin shims' paragraph: drop the 'All three symlink
  skills/ to the in-tree skills/' line and the task plugin:sync
  reference. Describe MCP-fetch delivery (three tools + resource
  URI).
- CLAUDE.md 'Shared skills' paragraph: rewrite to describe
  embedded canonicals + the repo-root reverse-symlink + the runtime
  MCP fetch surface. Keep the task skills:validate reference (it
  still works via the new symlink) and note the kebab-case name
  regex.
- plugin/specgraph/README.md: drop symlink language; describe
  MCP-fetch.
- plugin/specgraph/routing-guide.md: add a 'Loading a SpecGraph
  skill' section naming specgraph_skills_list,
  specgraph_skills_search, specgraph_skills_get, and
  specgraph://skills/<name>.
- README.md (top-level): no edits needed (no skill/plugin:sync
  references).

Per design §Sequencing (commit 10) and §Documentation updates
included in this PR.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 12: E2E test — full skills surface via real server

**Files:**

- Create: `e2e/api/skills_test.go`

- [ ] **Step 1: Write the E2E test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Skills via MCP", func() {
	It("exposes specgraph://skills/specgraph-authoring with the SKILL.md body", func() {
		body := mustReadResource("specgraph://skills/specgraph-authoring")
		Expect(body).To(ContainSubstring("name: specgraph-authoring"))
		Expect(body).To(ContainSubstring("summary:"))
	})

	It("lists six skills via specgraph_skills_list", func() {
		out := mustCallTool("specgraph_skills_list", map[string]any{})
		// JSON array contains six rows; assert all expected names.
		for _, name := range []string{
			"specgraph-authoring",
			"specgraph-graph-query",
			"specgraph-analytical-passes",
			"specgraph-drift",
			"specgraph-conventions",
			"specgraph-troubleshooting",
		} {
			Expect(out).To(ContainSubstring(name))
		}
	})

	It("finds the drift skill via specgraph_skills_search", func() {
		out := mustCallTool("specgraph_skills_search", map[string]any{"query": "drift"})
		Expect(out).To(ContainSubstring("specgraph-drift"))
	})

	It("rejects malformed skill URIs with CodeNotFound", func() {
		_, err := tryReadResource("specgraph://skills/foo/bar")
		Expect(err).To(HaveOccurred())
		Expect(strings.ToLower(err.Error())).To(SatisfyAny(
			ContainSubstring("not found"),
			ContainSubstring("notfound"),
		))
	})
})
```

Note: `mustReadResource`, `mustCallTool`, and `tryReadResource` are the existing helpers used by other tests in `e2e/api/` (see `claude_plugin_test.go` for the pattern). If the helper names differ, match the existing convention. Same for `testutil.NewCLIRunner` if your test prefers the CLI shell-out approach.

- [ ] **Step 2: Run the e2e suite**

```bash
go test -tags e2e ./e2e/api/... -run Skills -v
```

Expected: PASS. If e2e setup requires Docker and the local environment lacks it, run `task pr-prep` instead per `CLAUDE.md`.

- [ ] **Step 3: `task check`**

```bash
task check
```

Expected: PASS (`task check` doesn't run e2e; this confirms unit tests are still green).

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "test(e2e): skills surface via real server

Adds e2e/api/skills_test.go (Ginkgo) covering the full skills
surface PR F introduces:

- specgraph://skills/specgraph-authoring resource returns the SKILL.md
  body containing the canonical name line and summary frontmatter.
- specgraph_skills_list tool returns all six skill names.
- specgraph_skills_search tool finds specgraph-drift when queried
  for 'drift'.
- Malformed skill URIs (specgraph://skills/foo/bar) reject with
  CodeNotFound — pins the extractSkillName strictness.

Bead: spgr-i3he

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Test plan (full PR)

After all 12 tasks land:

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-pr-f
task check         # fmt → license → lint → build → unit tests; must PASS
task pr-prep       # task check + integration + e2e; must PASS (requires Docker)
```

Also run targeted checks:

```bash
# All new tests
go test -v ./internal/mcp/skills/... \
            ./internal/skillvalidate/... \
            ./internal/mcp/ -run 'TestRegisterSkillTools|TestSpecgraphSkills|TestSkillsResourceHandler|TestPrime'

# Symlink invariant
go test -v ./internal/mcp/skills/ -run TestSkillsSymlink

# E2E
go test -tags e2e -v ./e2e/api/... -run Skills
```

---

## Self-review checklist

**Spec coverage:**

- ✓ Canonical relocation + reverse-symlink → Task 1
- ✓ `summary:` field on all 6 SKILL.md → Task 2
- ✓ Shared `NameRegex` + validator enforcement → Task 3
- ✓ `Source` interface + types + `embeddedSource` (List/Get) → Tasks 4-5
- ✓ `Search` (text + regex + fields + limit) → Task 6
- ✓ Three MCP tools with `objectSchema`/`stringProp`/`boolProp` → Task 7
- ✓ Templated resource handler with `extractSkillName` strictness → Task 8
- ✓ Prime pointer section → Task 9
- ✓ Delete `plugin/specgraph/skills` symlink + `task plugin:sync` → Task 10
- ✓ CLAUDE.md / plugin README / routing-guide / top-level README → Task 11
- ✓ E2E test → Task 12

**Placeholder scan:** Each step contains concrete code or a concrete command, no "TBD", no "similar to above", no "add appropriate error handling".

**Type / signature consistency across tasks:**

- `Source` interface defined in Task 4 ↔ implemented in Tasks 5-6 ↔ consumed in Tasks 7-9 ↔ test-faked in Task 7 (`fakeSource` reused by Tasks 8-9).
- `extractSkillName` (Task 8) returns `(string, error)`; tested in Task 8's `TestSkillsResourceHandler_RejectsMalformedURI`.
- `NameRegex` defined in Task 3 (`skillvalidate.NameRegex`); imported by Task 5 (`embedded.go`) and Task 8 (`extractSkillName`).
- `RegisterResources` signature change is coordinated across `resources.go` (Task 8), `resources_test.go` (Task 8), and `server.go` (Task 8) in one commit.
- `primeResourceHandler` signature change is coordinated across `resources.go` (Task 9) and `resources_test.go` (Task 9) in one commit.

---

## Summary

11 commits (Tasks 4+5 pair into one commit; the other 10 tasks each land their own commit), atop main in the `spgr-rwrp-pr-f` jj workspace. Expands the design's 10-commit sequence by one: Task 12 (E2E test) is broken out as its own commit for cleaner plan-task isolation rather than folded into Task 8 as the design suggested. Each commit must stay green under `task check`. Follow the order strictly — Tasks 4 → 8 each consume the previous task's surface, and Tasks 3 → 5 → 8 share `skillvalidate.NameRegex`. Once landed, push the bookmark with `jj git push --bookmark spgr-rwrp-pr-f` and open the PR following the same flow PRs C/D/E used.
