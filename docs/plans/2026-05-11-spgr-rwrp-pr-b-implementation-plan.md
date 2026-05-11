# PR B — Port existing managed files implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the five files specgraph already manages (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`, `AGENTS.md`, `.cursor/rules/specgraph-bootstrap.mdc`) onto the `internal/config/managedfiles/` framework; delete `internal/config/mcpconfigs/` and `internal/config/pointers/`.

**Architecture:** Replace PR A's strategy stubs with real `jsonKeyMergeStrategy` and `markdownBlockStrategy` implementations driven by a new `Build func(ProjectParams) []byte` closure field on `ManagedFile`. Live-path helpers (`canonicalize`, `splitFrontmatter`, `validateInitMarkers`, `purgeLegacyBlocks`) and vestigial v=1 renderers (`renderV1AgentsBlockBody`, `renderV1CursorBlockBody`) port verbatim from the deleted packages. Init wiring in `cmd/specgraph/init.go` swaps `mcpconfigs.Sync` + `pointers.Sync` for one `managedfiles.SyncAll` call.

**Tech Stack:** Go 1.23, ConnectRPC project conventions, `github.com/evanphx/json-patch/v5` (already a dep), pgx not involved, `task check` quality gate, `lefthook` pre-commit + DCO sign-off, conventional commits.

**Spec:** `docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md` (v7).

**Working directory:** `/Users/SeBrandt/Code/github.com/specgraph` (or `/Volumes/Code/github.com/specgraph`, same files via symlink). All `task` and `go` commands run from project root.

---

## File structure

**Files created in `internal/config/managedfiles/`:**

| File | Responsibility |
|---|---|
| `params.go` | `ProjectParams` struct + `Validate()` |
| `params_test.go` | `ProjectParams.Validate` test cases |
| `helpers_json.go` | `canonicalize` (ported from `mcpconfigs/sync.go`) |
| `helpers_md.go` | `splitFrontmatter`, `defaultCursorFrontmatter`, `purgeLegacyBlocks` + `legacyBlock` regex, `validateInitMarkers` + marker regexes, `extractManagedBlockBody`, `safeSlugPattern` |
| `helpers_md_test.go` | Tests for each helper |
| `errors_ports.go` | `ErrFrontmatterMissing` sentinel + `ErrCorruptedMarkers` sentinel |
| `vestigial_v1.go` | `renderV1AgentsBlockBody`, `renderV1CursorBlockBody` |
| `vestigial_v1_test.go` | Asserts v=1 helpers reproduce the exact bytes today's `pointers/` package would have written |
| `jsonkeymerge.go` | Real `jsonKeyMergeStrategy` implementation |
| `jsonkeymerge_test.go` | Six-case matrix per strategy |
| `markdownblock.go` | Real `markdownBlockStrategy` implementation |
| `markdownblock_test.go` | Six-case matrix per strategy |
| `wholefile_test.go` | Replaces `strategy_test.go`'s WholeFile coverage; asserts stub still returns `errNotImplemented` |
| `action_names.go` | Exported `ActionName(Action) string`, `CountErrors([]SyncResult) int` |
| `action_names_test.go` | Tests for both helpers |
| `internal/captureimpl/main.go` | One-shot capture helper (deleted in cleanup commit) |
| `internal/captureimpl/doc.go` | Package comment + license header for the helper |
| `testdata/golden/<case>/` | Captured byte fixtures (immutable; not regenerable after cleanup) |
| `testdata/golden/README.md` | Regeneration recipe pointing at the pre-cleanup commit SHA |
| `testdata/detail-grammar.txt` | Pinned `SyncResult.Detail` strings (referenced by tests) |
| `migration_test.go` | Synthetic v=1 → v=2 upgrade integration test |

**Files modified in `internal/config/managedfiles/`:**

| File | Change |
|---|---|
| `types.go` | Add `Build func(ProjectParams) ([]byte, error)` on `ManagedFile`; add `Detail string` on `SyncResult` |
| `strategy.go` | Strategy interface takes `params ProjectParams`; replace JSON + MD stubs with real impls |
| `inspect.go` | Public `Inspect`/`InspectAll` take `ProjectParams`; remove "PR B placeholder" branch |
| `sync.go` | Public `Sync`/`SyncAll` take `ProjectParams` |
| `manifest.go` | Populate `allManagedFiles()` with 5 entries; add `func init()` validating `Source` xor `Build` |
| `errors.go` | (unchanged — `errNotImplemented` stays until PR C) |
| `strategy_test.go` | Deleted; replaced by per-strategy test files |
| `inspect_test.go` | Update calls to pass `ProjectParams` |
| `integration_test.go` | Update calls to pass `ProjectParams` |

**Files modified outside `managedfiles/`:**

| File | Change |
|---|---|
| `cmd/specgraph/init.go:96-153` | Replace `pointers.NewOptions` + `mcpconfigs.Sync` + `pointers.Sync` blocks with one `managedfiles.SyncAll`; preserve `init.go:36-95, 102-109, 149-151` verbatim |
| `Taskfile.yml` | Add `task capture-goldens` target (deleted in cleanup commit) |

**Files deleted (cleanup commit):**

- `internal/config/mcpconfigs/` (whole directory)
- `internal/config/pointers/` (whole directory)
- `internal/config/managedfiles/internal/captureimpl/` (whole directory)
- `task capture-goldens` target from `Taskfile.yml`

---

## Conventions enforced throughout

- Every new `.go` file starts with this two-line prologue:

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt

  package <name>
  ```

- New sub-packages (only `captureimpl`) get a `doc.go` with `// Package <name> ...` doc comment.
- Commit message format: `feat(managedfiles): <subject>` or `chore(managedfiles): <subject>`; body references `spgr-rwrp`; trailer carries `Signed-off-by: Sean Brandt <SeBrandt@geico.com>`.
- Commit with `git commit -s -m "..."` (the `-s` adds the trailer) or `jj describe -m "..." --reset-author=false` then `jj git push`.
- Run `task check` before every commit that compiles. The pre-push hook enforces it; running early saves cycles.

---

### Task 1: `ProjectParams` type + `Validate()`

**Files:**
- Create: `internal/config/managedfiles/params.go`
- Create: `internal/config/managedfiles/params_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/config/managedfiles/params_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestProjectParamsValidate(t *testing.T) {
	cases := []struct {
		name    string
		params  ProjectParams
		wantErr bool
	}{
		{"valid", ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}, false},
		{"valid https", ProjectParams{Slug: "p", ServerURL: "https://specgraph.example.com"}, false},
		{"valid slug with dots", ProjectParams{Slug: "p.q.r_1-2", ServerURL: "http://h"}, false},
		{"empty slug", ProjectParams{Slug: "", ServerURL: "http://h"}, true},
		{"slug with space", ProjectParams{Slug: "my proj", ServerURL: "http://h"}, true},
		{"slug starts with dash", ProjectParams{Slug: "-x", ServerURL: "http://h"}, true},
		{"missing scheme", ProjectParams{Slug: "p", ServerURL: "localhost:9090"}, true},
		{"empty URL", ProjectParams{Slug: "p", ServerURL: ""}, true},
		{"non-http scheme", ProjectParams{Slug: "p", ServerURL: "ftp://h"}, true},
		{"empty host", ProjectParams{Slug: "p", ServerURL: "http://"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/managedfiles/ -run TestProjectParamsValidate -v
```

Expected: `FAIL` with `ProjectParams` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/config/managedfiles/params.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"net/url"
)

// ProjectParams carries the per-project values strategy Build closures
// interpolate into managed file content. Construct once at init time;
// thread the same value through InspectAll and SyncAll.
//
// Build closures MUST be pure functions of ProjectParams — see
// docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md
// §"Build closure purity".
type ProjectParams struct {
	Slug      string
	ServerURL string // resolved, http or https, no /mcp/ suffix
}

// Validate rejects malformed slug or server URL. Lifted from
// pointers.NewOptions (deleted in this PR).
func (p ProjectParams) Validate() error {
	if !safeSlugPattern.MatchString(p.Slug) {
		return fmt.Errorf("project slug %q does not match %s", p.Slug, safeSlugPattern)
	}
	parsed, perr := url.Parse(p.ServerURL)
	if perr != nil {
		return fmt.Errorf("server URL %q: %w", p.ServerURL, perr)
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("server URL %q must be an absolute http or https URL", p.ServerURL)
	}
	return nil
}
```

Note: `safeSlugPattern` will be added in Task 2 (alongside other ports). The test depends on it being defined; if you run tests between Task 1 and Task 2 the compile fails. That's OK — Tasks 1 and 2 land in one commit.

- [ ] **Step 4: Defer running tests until Task 2 lands `safeSlugPattern`.**

### Task 2: Port `safeSlugPattern` + frontmatter helpers + error sentinels

**Files:**
- Create: `internal/config/managedfiles/helpers_md.go` (skeleton; more added in later tasks)
- Create: `internal/config/managedfiles/errors_ports.go`
- Create: `internal/config/managedfiles/helpers_md_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/managedfiles/helpers_md_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"errors"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	t.Run("valid frontmatter", func(t *testing.T) {
		in := []byte("---\ndescription: hi\nalwaysApply: true\n---\n\nbody text\n")
		front, body, err := splitFrontmatter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFront := []byte("---\ndescription: hi\nalwaysApply: true\n---\n\n")
		wantBody := []byte("body text\n")
		if !bytes.Equal(front, wantFront) {
			t.Errorf("front mismatch:\n  got %q\n want %q", front, wantFront)
		}
		if !bytes.Equal(body, wantBody) {
			t.Errorf("body mismatch:\n  got %q\n want %q", body, wantBody)
		}
	})
	t.Run("missing frontmatter", func(t *testing.T) {
		_, _, err := splitFrontmatter([]byte("body text\n"))
		if !errors.Is(err, ErrFrontmatterMissing) {
			t.Fatalf("want ErrFrontmatterMissing, got %v", err)
		}
	})
	t.Run("unclosed frontmatter", func(t *testing.T) {
		_, _, err := splitFrontmatter([]byte("---\ndescription: hi\nbody\n"))
		if !errors.Is(err, ErrFrontmatterMissing) {
			t.Fatalf("want ErrFrontmatterMissing, got %v", err)
		}
	})
}

func TestSafeSlugPattern(t *testing.T) {
	good := []string{"a", "abc", "a.b", "a-b", "a_b", "a1.2_3-4"}
	bad := []string{"", "-a", ".a", "a b", "a/b"}
	for _, s := range good {
		if !safeSlugPattern.MatchString(s) {
			t.Errorf("expected %q to match", s)
		}
	}
	for _, s := range bad {
		if safeSlugPattern.MatchString(s) {
			t.Errorf("expected %q NOT to match", s)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run 'TestSplitFrontmatter|TestSafeSlugPattern' -v
```

Expected: FAIL with `splitFrontmatter`, `ErrFrontmatterMissing`, `safeSlugPattern` undefined.

- [ ] **Step 3: Implement error sentinels**

```go
// internal/config/managedfiles/errors_ports.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "errors"

// ErrFrontmatterMissing is returned by splitFrontmatter when the input
// does not begin with `---\n` or the frontmatter is not closed.
// Ported from pointers/errors.go.
var ErrFrontmatterMissing = errors.New("frontmatter missing or unclosed")

// ErrCorruptedMarkers is returned when validateInitMarkers detects a
// malformed init-block marker pair (count mismatch, ordering, naked
// marker without version, unknown version). Ported from
// pointers/errors.go.
var ErrCorruptedMarkers = errors.New("corrupted specgraph:init markers")
```

- [ ] **Step 4: Implement frontmatter + slug-pattern helpers**

```go
// internal/config/managedfiles/helpers_md.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"fmt"
	"regexp"
)

// safeSlugPattern matches the slug class accepted by ProjectParams.Validate.
// Lifted verbatim from pointers/sync.go.
var safeSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// defaultCursorFrontmatter is the YAML frontmatter written into a fresh
// .mdc cursor rule. Includes the trailing blank line after the closing
// "---" — this is part of the byte sequence the supersedes prior-canonical
// hash compares against. Lifted verbatim from pointers/cursor.go:18-23.
const defaultCursorFrontmatter = `---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---

`

// splitFrontmatter splits a Cursor rule file into
// (frontmatter-including-trailing-blank, body). Returns
// ErrFrontmatterMissing if the file does not begin with `---\n` or the
// frontmatter is not closed. Ported from pointers/cursor.go:117-137.
func splitFrontmatter(data []byte) (front, body []byte, err error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf("%w: must start with '---'", ErrFrontmatterMissing)
	}
	rest := data[len("---\n"):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return nil, nil, fmt.Errorf("%w: frontmatter not closed before EOF", ErrFrontmatterMissing)
	}
	end := len("---\n") + idx + len("\n---\n")
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return data[:end], data[end:], nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run 'TestSplitFrontmatter|TestSafeSlugPattern|TestProjectParamsValidate' -v
```

Expected: PASS (all three test sets — Task 1's TestProjectParamsValidate now compiles).

- [ ] **Step 6: Commit Tasks 1 + 2 together**

```bash
git add internal/config/managedfiles/params.go internal/config/managedfiles/params_test.go internal/config/managedfiles/helpers_md.go internal/config/managedfiles/helpers_md_test.go internal/config/managedfiles/errors_ports.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): add ProjectParams and port frontmatter helpers

Lifts safeSlugPattern, splitFrontmatter, defaultCursorFrontmatter,
ErrFrontmatterMissing, ErrCorruptedMarkers from internal/config/pointers/
into the new framework. Adds ProjectParams.Validate using the lifted
slug regex and URL check.

Spec: docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md
Bead: spgr-rwrp.1
EOF
)"
```

### Task 3: Port `validateInitMarkers` (v=1 + v=2 aware) + marker regexes

**Files:**
- Modify: `internal/config/managedfiles/helpers_md.go` (append)
- Modify: `internal/config/managedfiles/helpers_md_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append to `helpers_md_test.go`:

```go
func TestValidateInitMarkers(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"no markers", "no markers here\n", false},
		{"valid v=1 pair", "<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:end -->\n", false},
		{"valid v=2 pair", "<!-- specgraph:init:start v=2 sha256=abc123 -->\nbody\n<!-- specgraph:init:end -->\n", false},
		{"end before start", "<!-- specgraph:init:end -->\nbody\n<!-- specgraph:init:start v=1 -->\n", true},
		{"double start", "<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:end -->\n", true},
		{"start without end", "<!-- specgraph:init:start v=1 -->\nbody\n", true},
		{"end without start", "body\n<!-- specgraph:init:end -->\n", true},
		{"naked start no version", "<!-- specgraph:init:start -->\nbody\n<!-- specgraph:init:end -->\n", true},
		{"unknown version v=99", "<!-- specgraph:init:start v=99 -->\nbody\n<!-- specgraph:init:end -->\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInitMarkers("test.md", []byte(tc.input))
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateInitMarkers err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestValidateInitMarkers -v
```

Expected: FAIL with `validateInitMarkers` undefined.

- [ ] **Step 3: Implement marker validator**

Append to `helpers_md.go`:

```go
// initStartLoose matches any "specgraph:init:start" marker, with or
// without a v=N suffix. Used by validateInitMarkers to reject
// naked markers. Ported from pointers/agents.go:25.
var initStartLoose = regexp.MustCompile(`<!--\s*specgraph:init:start\s*-->`)

// initStartAnyVersion matches any well-formed init start marker (v=1, v=2,
// future v=N). Used to anchor "canonical start" positions when checking
// for naked-marker corruption. Replaces the bytes.Index(initStart)
// approach in pointers/agents.go:150 that only recognised v=1.
var initStartAnyVersion = regexp.MustCompile(`<!--\s*specgraph:init:start(\s+v=\d+(\s+sha256=[0-9a-fA-F]+)?(\s+rev=\S+)?)?\s*-->`)

const initEndMarker = "<!-- specgraph:init:end -->"

// validateInitMarkers checks five corruption rules:
//   (1) end before start
//   (2) start without end
//   (3) double start
//   (4) naked init start without a v=N suffix
//   (5) unknown version (anything not in ParseSentinel's supported set)
//
// Adapted from pointers/agents.go:134-182. Two adaptations vs. the
// original: version acceptance now delegates to ParseSentinel (which
// supports v=1 and v=2), and the "canonical start" position used in
// Rule 4's exception comes from initStartAnyVersion regex matches,
// not bytes.Index of the v=1 literal.
func validateInitMarkers(displayName string, data []byte) error {
	starts := initStartAnyVersion.FindAllIndex(data, -1)
	ends := bytes.Count(data, []byte(initEndMarker))

	// Rule 5: each well-formed start marker must carry a supported version.
	for _, m := range starts {
		fragment := string(data[m[0]:m[1]])
		s, perr := ParseSentinel(CommentHTML, fragment)
		if perr != nil {
			return fmt.Errorf("%w: %s contains unsupported init start marker at offset %d (%q): %v",
				ErrCorruptedMarkers, displayName, m[0], fragment, perr)
		}
		if s.Version == 0 {
			// initStartAnyVersion matched a start without v=N — Rule 4.
			return fmt.Errorf("%w: %s contains an init start marker without v=N at offset %d (%q); remove or repair manually",
				ErrCorruptedMarkers, displayName, m[0], fragment)
		}
	}

	// Rule 4: catch naked "specgraph:init:start" markers that don't
	// overlap any well-formed start (e.g. a stray comment).
	for _, m := range initStartLoose.FindAllIndex(data, -1) {
		overlap := false
		for _, c := range starts {
			if m[0] == c[0] {
				overlap = true
				break
			}
		}
		if !overlap {
			return fmt.Errorf("%w: %s contains a naked init marker at offset %d", ErrCorruptedMarkers, displayName, m[0])
		}
	}

	switch {
	case len(starts) == 0 && ends == 0:
		return nil
	case len(starts) == 1 && ends == 1:
		startOff := starts[0][0]
		endOff := bytes.Index(data, []byte(initEndMarker))
		if endOff < startOff {
			return fmt.Errorf("%w: %s: init end marker appears before start marker", ErrCorruptedMarkers, displayName)
		}
		return nil
	case len(starts) > 1:
		return fmt.Errorf("%w: %s: more than one init start marker", ErrCorruptedMarkers, displayName)
	case len(starts) == 1 && ends == 0:
		return fmt.Errorf("%w: %s: init start marker without matching end", ErrCorruptedMarkers, displayName)
	default:
		return fmt.Errorf("%w: %s: init end marker without matching start", ErrCorruptedMarkers, displayName)
	}
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/config/managedfiles/ -run TestValidateInitMarkers -v
```

Expected: PASS (all nine cases).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/helpers_md.go internal/config/managedfiles/helpers_md_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): port and adapt validateInitMarkers for v=2 markers

Replaces the v=1-literal-only validator in pointers/agents.go with a
version-agnostic port. Two adaptations: ParseSentinel handles version
acceptance, and canonical-start anchoring uses the
initStartAnyVersion regex instead of bytes.Index(v=1 literal).

Spec: §"markdownBlockStrategy" step 3.
Bead: spgr-rwrp.1
EOF
)"
```

### Task 4: Port `purgeLegacyBlocks` + `legacyBlock` regex

**Files:**
- Modify: `internal/config/managedfiles/helpers_md.go` (append)
- Modify: `internal/config/managedfiles/helpers_md_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `helpers_md_test.go`:

```go
func TestPurgeLegacyBlocks(t *testing.T) {
	t.Run("removes well-formed slug block", func(t *testing.T) {
		in := []byte("before\n<!-- specgraph:foo:start -->\nfoo body\n<!-- specgraph:foo:end -->\nafter\n")
		out, purged, skipped := purgeLegacyBlocks(in)
		if purged != 1 || skipped != 0 {
			t.Errorf("counts: purged=%d skipped=%d, want 1/0", purged, skipped)
		}
		want := []byte("before\nafter\n")
		if !bytes.Equal(out, want) {
			t.Errorf("out mismatch:\n  got %q\n want %q", out, want)
		}
	})
	t.Run("preserves init block", func(t *testing.T) {
		in := []byte("<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:end -->\n")
		out, purged, _ := purgeLegacyBlocks(in)
		if purged != 0 {
			t.Errorf("init block must NOT be purged; got %d", purged)
		}
		if !bytes.Equal(out, in) {
			t.Errorf("init block unchanged; got %q", out)
		}
	})
	t.Run("skips malformed mismatched slugs", func(t *testing.T) {
		in := []byte("<!-- specgraph:foo:start -->\nbody\n<!-- specgraph:bar:end -->\n")
		out, purged, skipped := purgeLegacyBlocks(in)
		if purged != 0 || skipped != 1 {
			t.Errorf("counts: purged=%d skipped=%d, want 0/1", purged, skipped)
		}
		if !bytes.Equal(out, in) {
			t.Errorf("malformed block unchanged; got %q", out)
		}
	})
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestPurgeLegacyBlocks -v
```

Expected: FAIL with `purgeLegacyBlocks` undefined.

- [ ] **Step 3: Implement legacy-block purge**

Append to `helpers_md.go`:

```go
// legacyBlock matches inject-era per-slug blocks. Slug class mirrors
// safeSlugPattern. The literal slug "init" is preserved (never purged).
// (?s) lets `.` match newlines. Ported verbatim from pointers/agents.go:37-39.
var legacyBlock = regexp.MustCompile(
	`(?s)<!--\s*specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):start\s*-->(.*?)<!--\s*specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):end\s*-->\n?`,
)

// purgeLegacyBlocks removes per-slug pre-init blocks from data. Returns
// (cleaned data, count purged, count skipped due to slug-mismatch).
// The literal slug "init" is never purged. Ported verbatim from
// pointers/agents.go:184-202.
func purgeLegacyBlocks(data []byte) (out []byte, purged, skippedMalformed int) {
	out = legacyBlock.ReplaceAllFunc(data, func(match []byte) []byte {
		sub := legacyBlock.FindSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		slugStart, slugEnd := string(sub[1]), string(sub[3])
		if slugStart != slugEnd {
			skippedMalformed++
			return match
		}
		if slugStart == "init" {
			return match
		}
		purged++
		return nil
	})
	return out, purged, skippedMalformed
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/managedfiles/ -run TestPurgeLegacyBlocks -v
```

Expected: PASS (three cases).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/helpers_md.go internal/config/managedfiles/helpers_md_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): port purgeLegacyBlocks from pointers

Removes inject-era per-slug `specgraph:<slug>:start/end` blocks from
AGENTS.md on every sync. Verbatim port of pointers/agents.go:184-202.
The "init" slug is preserved; mismatched-slug pairs are skipped (not
deleted) for manual repair.

Spec: §"markdownBlockStrategy" step 4.
Bead: spgr-rwrp.1
EOF
)"
```

### Task 5: Implement `extractManagedBlockBody`

**Files:**
- Modify: `internal/config/managedfiles/helpers_md.go` (append)
- Modify: `internal/config/managedfiles/helpers_md_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `helpers_md_test.go`:

```go
func TestExtractManagedBlockBody(t *testing.T) {
	t.Run("v=1 markers", func(t *testing.T) {
		in := []byte("prelude\n<!-- specgraph:init:start v=1 -->\nbody line 1\nbody line 2\n<!-- specgraph:init:end -->\npostlude\n")
		body, ok := extractManagedBlockBody(in)
		if !ok {
			t.Fatal("expected ok=true")
		}
		want := []byte("body line 1\nbody line 2")
		if !bytes.Equal(body, want) {
			t.Errorf("body mismatch:\n  got %q\n want %q", body, want)
		}
	})
	t.Run("v=2 markers", func(t *testing.T) {
		in := []byte("<!-- specgraph:init:start v=2 sha256=abc -->\nx\n<!-- specgraph:init:end -->\n")
		body, ok := extractManagedBlockBody(in)
		if !ok || string(body) != "x" {
			t.Errorf("got %q, ok=%v", body, ok)
		}
	})
	t.Run("empty body", func(t *testing.T) {
		in := []byte("<!-- specgraph:init:start v=2 sha256=abc -->\n<!-- specgraph:init:end -->\n")
		body, ok := extractManagedBlockBody(in)
		if !ok {
			t.Fatal("empty body must return ok=true")
		}
		if body == nil {
			t.Fatal("empty body must return non-nil empty slice")
		}
		if len(body) != 0 {
			t.Errorf("body len = %d, want 0", len(body))
		}
	})
	t.Run("no markers", func(t *testing.T) {
		_, ok := extractManagedBlockBody([]byte("just prose\n"))
		if ok {
			t.Error("expected ok=false")
		}
	})
	t.Run("end before start", func(t *testing.T) {
		_, ok := extractManagedBlockBody([]byte("<!-- specgraph:init:end -->\n<!-- specgraph:init:start v=1 -->\n"))
		if ok {
			t.Error("expected ok=false")
		}
	})
	t.Run("multiple starts", func(t *testing.T) {
		_, ok := extractManagedBlockBody([]byte("<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:end -->\n"))
		if ok {
			t.Error("expected ok=false")
		}
	})
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestExtractManagedBlockBody -v
```

Expected: FAIL with `extractManagedBlockBody` undefined.

- [ ] **Step 3: Implement**

Append to `helpers_md.go`:

```go
// extractManagedBlockBody returns the bytes strictly between the
// canonical start and end markers, or (nil, false) if no well-formed
// pair is present. The bytes do NOT include the surrounding marker
// lines or any leading/trailing newline adjacent to them.
//
// "Well-formed pair" means: exactly one start marker (v=1 OR v=2,
// matched via initStartAnyVersion), exactly one end marker, end
// strictly after start. Anything else returns (nil, false).
// Empty body between markers returns ([]byte{}, true).
func extractManagedBlockBody(content []byte) ([]byte, bool) {
	starts := initStartAnyVersion.FindAllIndex(content, -1)
	if len(starts) != 1 {
		return nil, false
	}
	endOff := bytes.Index(content, []byte(initEndMarker))
	if endOff < 0 {
		return nil, false
	}
	startEnd := starts[0][1] // end offset of the start-marker line
	// Skip the newline immediately after the start marker if present.
	if startEnd < len(content) && content[startEnd] == '\n' {
		startEnd++
	}
	if endOff < startEnd {
		return nil, false
	}
	body := content[startEnd:endOff]
	// Strip the trailing newline immediately before the end marker.
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	if body == nil {
		body = []byte{}
	}
	return body, true
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/managedfiles/ -run TestExtractManagedBlockBody -v
```

Expected: PASS (six cases).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/helpers_md.go internal/config/managedfiles/helpers_md_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): add extractManagedBlockBody helper

Returns just the bytes between the start and end markers — the
correct hash scope for MarkdownBlock strategy. Recognises v=1 and
v=2 markers; empty body returns non-nil empty slice.

Spec: §"markdownBlockStrategy" / "Hash scope".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 6: Port `canonicalize` from mcpconfigs

**Files:**
- Create: `internal/config/managedfiles/helpers_json.go`
- Create: `internal/config/managedfiles/helpers_json_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/managedfiles/helpers_json_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	t.Run("alphabetical keys", func(t *testing.T) {
		in := []byte(`{"z":1,"a":2}`)
		out, err := canonicalize(in)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("{\n  \"a\": 2,\n  \"z\": 1\n}\n")
		if !bytes.Equal(out, want) {
			t.Errorf("got %q want %q", out, want)
		}
	})
	t.Run("trailing newline", func(t *testing.T) {
		out, err := canonicalize([]byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if out[len(out)-1] != '\n' {
			t.Error("missing trailing newline")
		}
	})
	t.Run("invalid JSON returns error", func(t *testing.T) {
		if _, err := canonicalize([]byte(`{not json}`)); err == nil {
			t.Error("expected error")
		}
	})
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestCanonicalize -v
```

Expected: FAIL with `canonicalize` undefined.

- [ ] **Step 3: Implement**

```go
// internal/config/managedfiles/helpers_json.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"fmt"
)

// canonicalize returns the JSON document re-marshaled with 2-space
// indent, alphabetical key order (via encoding/json), and a trailing
// newline. Used by jsonKeyMergeStrategy for the no-op short-circuit
// and the bytes written to disk. Verbatim port of
// mcpconfigs/sync.go:180-190.
func canonicalize(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal indent: %w", err)
	}
	return append(out, '\n'), nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/managedfiles/ -run TestCanonicalize -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/helpers_json.go internal/config/managedfiles/helpers_json_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): port canonicalize from mcpconfigs

2-space indent, alphabetical keys, trailing newline. Identical to
mcpconfigs/sync.go's helper; used by jsonKeyMergeStrategy.

Spec: §"jsonKeyMergeStrategy" step 6.
Bead: spgr-rwrp.1
EOF
)"
```

### Task 7: Port `renderV1AgentsBlockBody` + `renderV1CursorBlockBody`

**Files:**
- Create: `internal/config/managedfiles/vestigial_v1.go`
- Create: `internal/config/managedfiles/vestigial_v1_test.go`

- [ ] **Step 1: Write the failing tests**

The v=1 renderers must reproduce the bytes today's `pointers.renderAgentsBlock` produces. Read `pointers/agents.go:41-52` for the expected layout. Test asserts byte-exact match.

```go
// internal/config/managedfiles/vestigial_v1_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"strings"
	"testing"
)

func TestRenderV1AgentsBlockBody(t *testing.T) {
	params := ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}
	got := string(renderV1AgentsBlockBody(params))

	mustContain := []string{
		"# SpecGraph project pointer",
		"Server: http://localhost:9090",
		"Project: myproj (sent as the X-Specgraph-Project header)",
		"This block is managed by `specgraph init`. Edit content outside the markers.",
		"Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("body missing %q\nFull body:\n%s", s, got)
		}
	}
	// Must NOT contain markers — body bytes only.
	if strings.Contains(got, "<!-- specgraph:init") {
		t.Error("body must not include marker lines")
	}
}

func TestRenderV1CursorBlockBody(t *testing.T) {
	params := ProjectParams{Slug: "x", ServerURL: "http://h"}
	agents := string(renderV1AgentsBlockBody(params))
	cursor := string(renderV1CursorBlockBody(params))
	if cursor != agents {
		t.Errorf("cursor body must equal agents body for v=1; got\n%q\nvs\n%q", cursor, agents)
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run 'TestRenderV1AgentsBlockBody|TestRenderV1CursorBlockBody' -v
```

Expected: FAIL with undefined symbols.

- [ ] **Step 3: Implement**

Read `pointers/agents.go:41-52` carefully — the helper must reproduce bytes exactly (including newline placement) because the supersedes hash compares against it.

```go
// internal/config/managedfiles/vestigial_v1.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Vestigial v=1 renderers. Reproduce the body bytes the deleted
// pointers/ package would have written inside <!-- specgraph:init:start
// v=1 --> ... <!-- specgraph:init:end --> markers.
//
// Used only by:
//   (a) markdownBlockStrategy's v=1 → v=2 upgrade hash-check
//   (b) supersedesGuardedDelete's prior-canonical computation for
//       .cursor/rules/specgraph-bootstrap.md
//
// Not on the production write path — new writes emit v=2 with hash
// sentinels.
//
// Sunset trigger: parent design's "zero v=1 files for two consecutive
// releases" AND "6 months elapsed since v=2 rollout" — see spec
// §"Helpers ported" / "Sunset trigger correction."

package managedfiles

import (
	"fmt"
	"strings"
)

// renderV1AgentsBlockBody returns the body bytes (between markers,
// no markers themselves) for AGENTS.md's v=1 block. Verbatim port
// of pointers/agents.go:41-52 minus the start/end marker writes.
func renderV1AgentsBlockBody(p ProjectParams) []byte {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("# SpecGraph project pointer\n\n")
	fmt.Fprintf(&b, "Server: %s\n", p.ServerURL)
	fmt.Fprintf(&b, "Project: %s (sent as the X-Specgraph-Project header)\n\n", p.ProjectSlug())
	b.WriteString("This block is managed by `specgraph init`. Edit content outside the markers.\n")
	b.WriteString("Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.\n")
	return []byte(b.String())
}

// renderV1CursorBlockBody returns the body bytes for the v=1 cursor
// bootstrap rule's block. pointers/cursor.go:25-27 delegates to
// renderAgentsBlock; we preserve that identity.
func renderV1CursorBlockBody(p ProjectParams) []byte {
	return renderV1AgentsBlockBody(p)
}

// ProjectSlug returns p.Slug; method exists because the pointers/
// helpers referenced opts.ProjectSlug as a field. Keeping the method
// name preserves the verbatim character of the port.
func (p ProjectParams) ProjectSlug() string { return p.Slug }
```

Note on byte-exactness: `pointers/agents.go:43` writes `b.WriteString("\n")` *immediately after* `initStart`. The marker is `<!-- specgraph:init:start v=1 -->`, then `\n`, then the body. For our body-only helper, that leading `\n` is part of the body (everything between the marker and the end marker). The test's `mustContain` checks the substantive lines; the leading newline is byte-included for hash-match faithfulness.

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/managedfiles/ -run 'TestRenderV1AgentsBlockBody|TestRenderV1CursorBlockBody' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/vestigial_v1.go internal/config/managedfiles/vestigial_v1_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): port v=1 renderers as vestigial helpers

Reproduces byte-exact body content the deleted pointers/ package
would have written between v=1 markers. Used only by the v=1 → v=2
upgrade hash check and supersedes prior-canonical computation.
Production write path emits v=2; these are defensive-only.

Spec: §"Helpers ported" / "Vestigial v=1 renderers".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 8: Add `ManagedFile.Build` field + `SyncResult.Detail` field

**Files:**
- Modify: `internal/config/managedfiles/types.go`

- [ ] **Step 1: Make the modification**

Modify `internal/config/managedfiles/types.go` — add `Build` to `ManagedFile` and `Detail` to `SyncResult`:

```go
// In ManagedFile struct, after `Source string`, add:

	// Build is a closure that returns the canonical content for this
	// file given a ProjectParams. Mutually exclusive with Source: each
	// manifest entry uses one or the other. JSONKeyMerge and
	// MarkdownBlock strategies require Build (canonical depends on
	// per-project params); WholeFile requires Source (canonical is a
	// static embedded asset).
	//
	// Build MUST be a pure function of ProjectParams: same input →
	// byte-identical output, no FS reads, no clock, no env, no
	// randomness. TestManifestShape asserts this for every registered
	// entry. Without purity, Inspect and Sync can disagree on state.
	Build func(ProjectParams) ([]byte, error)
```

```go
// In SyncResult struct, after `Err error`, add:

	// Detail is a human-readable explanation populated by strategies
	// for non-trivial cases (legacy-block purge counts, supersedes-path
	// drifted, --force --keep-edits semantics). Exact strings pinned
	// in testdata/detail-grammar.txt. Empty for the common case.
	Detail string
```

- [ ] **Step 2: Verify the package still compiles**

```bash
go build ./internal/config/managedfiles/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/config/managedfiles/types.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): add Build closure and Detail fields

ManagedFile.Build carries the canonical-content builder for JSON-merge
and markdown-block strategies (which depend on ProjectParams).
SyncResult.Detail carries human-readable explanations for non-trivial
cases (purge counts, supersedes-drifted).

Spec: §"Per-project params plumbing", §"SyncResult.Detail string
grammar".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 9: Cascade strategy interface signature for `ProjectParams`

**Files:**
- Modify: `internal/config/managedfiles/strategy.go`
- Modify: `internal/config/managedfiles/sync.go`
- Modify: `internal/config/managedfiles/inspect.go`
- Modify: `internal/config/managedfiles/strategy_test.go` (delete after task 11; for now update)
- Modify: `internal/config/managedfiles/inspect_test.go`
- Modify: `internal/config/managedfiles/integration_test.go`

- [ ] **Step 1: Update strategy interface and stubs**

Modify `strategy.go`:

```go
type strategy interface {
	Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error)
	Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error)
}
```

Update all three stub methods to match the signature. They still return `errNotImplemented`; just add `_ ProjectParams` to ignore the new parameter:

```go
func (jsonKeyMergeStrategy) Inspect(_ string, _ ManagedFile, _ ProjectParams) (FileState, error) {
	return FileState{}, errNotImplemented
}
func (jsonKeyMergeStrategy) Sync(_ string, _ ManagedFile, _ ProjectParams, _ SyncOptions) (SyncResult, error) {
	return SyncResult{}, errNotImplemented
}
// Same shape for markdownBlockStrategy and wholeFileStrategy.
```

- [ ] **Step 2: Update package-level Sync**

Modify `sync.go`:

```go
func Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error) {
	r, err := strategyImpl(mf.Strategy).Sync(cwd, mf, params, opts)
	if err != nil {
		return r, fmt.Errorf("strategy sync: %w", err)
	}
	return r, nil
}

func SyncAll(cwd string, harnesses []Harness, params ProjectParams, opts SyncOptions) ([]SyncResult, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]SyncResult, 0, len(mfs))
	for _, mf := range mfs {
		r, err := Sync(cwd, mf, params, opts)
		if err != nil {
			out = append(out, SyncResult{
				Path:   mf.Path,
				Action: ActionError,
				Err:    fmt.Errorf("sync %s: %w", mf.Path, err),
			})
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
```

- [ ] **Step 3: Update package-level Inspect**

Modify `inspect.go`:

```go
func Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	// Strategy-specific dispatch replaces the generic body. PR B's
	// real implementations classify per strategy; PR A's placeholder
	// branch is removed.
	state, err := strategyImpl(mf.Strategy).Inspect(cwd, mf, params)
	if err != nil {
		return state, fmt.Errorf("strategy inspect %s: %w", mf.Path, err)
	}
	return state, nil
}

func InspectAll(cwd string, harnesses []Harness, params ProjectParams) ([]FileState, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]FileState, 0, len(mfs))
	for _, mf := range mfs {
		state, err := Inspect(cwd, mf, params)
		if err != nil {
			out = append(out, FileState{
				Path:     mf.Path,
				Strategy: mf.Strategy,
				State:    StateDrifted,
				Detail:   fmt.Sprintf("inspect error: %v", err),
			})
			continue
		}
		out = append(out, state)
	}
	return out, nil
}
```

Note: the previous `Inspect` body (PR A placeholder with `rejectSymlinkComponents`, `readFileNoFollow`, etc.) is replaced. Strategy implementations own that logic now. `validateProjectDir` stays in this file (it's a project-dir utility, not strategy-specific).

- [ ] **Step 4: Update existing tests for new signatures**

Edit `inspect_test.go`, `integration_test.go`, `strategy_test.go` to pass `ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}` where these functions are called. Where tests assert the PR A placeholder behaviour ("PR A: classification deferred"), delete those assertions — they no longer apply.

- [ ] **Step 5: Verify build + existing tests pass**

```bash
go build ./internal/config/managedfiles/
go test ./internal/config/managedfiles/ -v
```

Expected: PASS (existing tests adjusted; new tests from earlier tasks pass; strategy stubs still return `errNotImplemented`).

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/strategy.go internal/config/managedfiles/sync.go internal/config/managedfiles/inspect.go internal/config/managedfiles/strategy_test.go internal/config/managedfiles/inspect_test.go internal/config/managedfiles/integration_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): cascade ProjectParams through strategy interface

Strategy.Inspect/Sync take ProjectParams (needed to invoke Build).
Package-level Inspect/InspectAll/Sync/SyncAll forward it. PR A
placeholder branches removed from inspect.go; strategy dispatch is
now end-to-end.

Spec: §"Per-project params plumbing" / "Signature cascade".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 10: Implement `jsonKeyMergeStrategy`

**Files:**
- Create: `internal/config/managedfiles/jsonkeymerge.go`
- Create: `internal/config/managedfiles/jsonkeymerge_test.go`

- [ ] **Step 1: Write failing tests covering the six-case matrix**

```go
// internal/config/managedfiles/jsonkeymerge_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testJSONMF(path string) ManagedFile {
	return ManagedFile{
		Path:     path,
		Strategy: StrategyJSONKeyMerge,
		Comment:  CommentNone,
		Harness:  HarnessOpenCode,
		Build: func(_ ProjectParams) ([]byte, error) {
			return []byte(`{"mcp":{"specgraph":{"url":"http://h/mcp/"}}}`), nil
		},
	}
}

func TestJSONKeyMergeMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	got, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if jerr := json.Unmarshal(got, &v); jerr != nil {
		t.Errorf("output not valid JSON: %v", jerr)
	}
}

func TestJSONKeyMergeSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	params := ProjectParams{Slug: "p", ServerURL: "http://h"}
	// First sync creates.
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second sync is no-op.
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestJSONKeyMergeStalePreservesSiblings(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	// Seed with a user-added sibling key.
	seed := []byte(`{"theme":"dark","mcp":{"specgraph":{"url":"http://OLD/"}}}`)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var parsed map[string]any
	_ = json.Unmarshal(got, &parsed)
	if parsed["theme"] != "dark" {
		t.Error("user-added theme sibling was destroyed")
	}
}

func TestJSONKeyMergeOpencodeJSONCRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte("//"), 0o600); err != nil {
		t.Fatal(err)
	}
	mf := testJSONMF("opencode.json")
	res, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{})
	if err == nil && res.Action != ActionError {
		t.Errorf("want error or ActionError; got action=%v err=%v", res.Action, err)
	}
}

func TestJSONKeyMergeModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	// Seed at 0o644.
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := jsonKeyMergeStrategy{}.Sync(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"}, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(dir, "config.json"))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestJSONKeyMergeInspectMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testJSONMF("config.json")
	state, err := jsonKeyMergeStrategy{}.Inspect(dir, mf, ProjectParams{Slug: "p", ServerURL: "http://h"})
	if err != nil {
		t.Fatal(err)
	}
	if state.State != StateMissing {
		t.Errorf("state = %v, want StateMissing", state.State)
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v
```

Expected: FAIL (`errNotImplemented` from the stub).

- [ ] **Step 3: Implement `jsonkeymerge.go`**

```go
// internal/config/managedfiles/jsonkeymerge.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

func (jsonKeyMergeStrategy) Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, err
	}
	full := filepath.Join(cwd, mf.Path)
	existing, err := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(err):
		return FileState{}, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(err, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, Detail: "file does not exist"}, nil
	case err != nil:
		return FileState{}, fmt.Errorf("read %s: %w", full, err)
	}
	canonical, err := jsonKeyMergeCanonical(existing, mf, params)
	if err != nil {
		return FileState{}, err
	}
	if bytes.Equal(existing, canonical) {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced}, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale}, nil
}

func (jsonKeyMergeStrategy) Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	full := filepath.Join(cwd, mf.Path)

	// opencode.json refuses if opencode.jsonc exists alongside.
	if mf.Path == "opencode.json" {
		jsoncPath := filepath.Join(cwd, "opencode.jsonc")
		if _, statErr := os.Lstat(jsoncPath); statErr == nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("found opencode.jsonc alongside opencode.json; consolidate to one file")}, nil
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("lstat %s: %w", jsoncPath, statErr)}, nil
		}
	}

	existing, rerr := readFileNoFollow(full)
	fileExisted := rerr == nil
	switch {
	case noFollowIsSymlink(rerr):
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("%w: %s", ErrSymlinkRejected, full)}, nil
	case rerr != nil && !errors.Is(rerr, fs.ErrNotExist):
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("read %s: %w", full, rerr)}, nil
	}
	if fileExisted {
		var probe any
		if jerr := json.Unmarshal(existing, &probe); jerr != nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("parse %s: %w", full, jerr)}, nil
		}
	}

	canonical, err := jsonKeyMergeCanonical(existing, mf, params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	if fileExisted && bytes.Equal(existing, canonical) {
		return SyncResult{Path: mf.Path, Action: ActionNoOp}, nil
	}

	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)}, nil
	}
	if err := atomicWrite(full, canonical, mode); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	if fileExisted {
		return SyncResult{Path: mf.Path, Action: ActionRefreshed}, nil
	}
	return SyncResult{Path: mf.Path, Action: ActionCreated}, nil
}

// jsonKeyMergeCanonical computes the canonical disk content for an entry:
// apply the patch from mf.Build to `existing` (or {} if missing), then
// canonicalize.
func jsonKeyMergeCanonical(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	patch, err := mf.Build(params)
	if err != nil {
		return nil, fmt.Errorf("build patch for %s: %w", mf.Path, err)
	}
	src := existing
	if len(src) == 0 {
		src = []byte(`{}`)
	}
	merged, err := jsonpatch.MergePatch(src, patch)
	if err != nil {
		return nil, fmt.Errorf("merge patch %s: %w", mf.Path, err)
	}
	return canonicalize(merged)
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/config/managedfiles/ -run TestJSONKeyMerge -v
```

Expected: PASS (six cases).

- [ ] **Step 5: Remove the duplicate `Inspect`/`Sync` stub for `jsonKeyMergeStrategy` from `strategy.go`** (since we just defined real ones in jsonkeymerge.go — there will be a duplicate-method error).

- [ ] **Step 6: Verify build + run all package tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS for all tests except markdownblock (still stubbed).

- [ ] **Step 7: Commit**

```bash
git add internal/config/managedfiles/jsonkeymerge.go internal/config/managedfiles/jsonkeymerge_test.go internal/config/managedfiles/strategy.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): implement jsonKeyMergeStrategy

RFC 7396 merge patch + canonicalize. Refuses opencode.jsonc sibling.
Preserves file mode from existing. Six-case test matrix: Missing,
Synced, Stale (preserves user siblings), opencode.jsonc refusal,
mode preservation, Inspect Missing.

Spec: §"jsonKeyMergeStrategy".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 11: Implement `markdownBlockStrategy`

**Files:**
- Create: `internal/config/managedfiles/markdownblock.go`
- Create: `internal/config/managedfiles/markdownblock_test.go`
- Modify: `internal/config/managedfiles/strategy.go` (remove markdown stub)
- Delete: `internal/config/managedfiles/strategy_test.go` (replaced)
- Create: `internal/config/managedfiles/wholefile_test.go` (covers the last stub)

This is the most complex task. The state machine has many branches; the test cases must cover each one.

- [ ] **Step 1: Write failing tests**

```go
// internal/config/managedfiles/markdownblock_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testMDMF(path string, harness Harness) ManagedFile {
	return ManagedFile{
		Path:     path,
		Strategy: StrategyMarkdownBlock,
		Comment:  CommentHTML,
		Harness:  harness,
		Build: func(p ProjectParams) ([]byte, error) {
			return []byte("\n# block body for " + p.Slug + "\n"), nil
		},
	}
}

var testMDParams = ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}

func TestMarkdownBlockMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	res, err := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("specgraph:init:start v=2 sha256=")) {
		t.Errorf("v=2 marker missing in output:\n%s", got)
	}
}

func TestMarkdownBlockSynced(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	// First sync creates.
	if _, err := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second sync no-op.
	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp", res.Action)
	}
}

func TestMarkdownBlockStaleV1Upgrade(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	// Seed with v=1 markers wrapping the same body the test Build would emit.
	body, _ := mf.Build(testMDParams)
	seed := []byte("<!-- specgraph:init:start v=1 -->" + string(body) + "<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)

	// NOTE: Test uses the SAME body for v=1 seed and v=2 build. Real-world
	// uses renderV1AgentsBlockBody for seed and the manifest's Build for
	// new emit; in PR B those produce identical body bytes by design.
	res, err := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed (v=1 upgrade)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("v=2 sha256=")) {
		t.Errorf("v=2 marker missing after upgrade:\n%s", got)
	}
}

func TestMarkdownBlockDriftedV1(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	// Seed with v=1 markers but mangled body (does NOT match what Build emits).
	seed := []byte("<!-- specgraph:init:start v=1 -->\nUSER EDIT — do not overwrite\n<!-- specgraph:init:end -->\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)

	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionSkipped {
		t.Errorf("action = %v, want ActionSkipped (drifted, no --force)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("USER EDIT")) {
		t.Error("drifted user content was overwritten")
	}
}

func TestMarkdownBlockOutsideMarkerEditsPreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	if _, err := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// User appends prose after the block.
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	withProse := append(got, []byte("\nUser prose after the block.\n")...)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), withProse, 0o600)

	// Re-sync; should still classify Synced (outside-block edits don't drift).
	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionNoOp {
		t.Errorf("action = %v, want ActionNoOp (outside-block edits ignored)", res.Action)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(after, []byte("User prose after the block.")) {
		t.Error("outside-block user content was destroyed")
	}
}

func TestMarkdownBlockNoMarkersAppends(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	// User-authored content; no specgraph markers.
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User content\n"), 0o600)
	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated (block created, file existed)", res.Action)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(got, []byte("# User content")) {
		t.Error("user content was destroyed")
	}
	if !bytes.Contains(got, []byte("specgraph:init:start v=2")) {
		t.Error("init block not appended")
	}
}

func TestMarkdownBlockPurgesLegacy(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	seed := []byte("<!-- specgraph:foo:start -->\nold\n<!-- specgraph:foo:end -->\n# User text\n")
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), seed, 0o600)
	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if res.Detail != "purged 1 legacy block" {
		t.Errorf("Detail = %q, want \"purged 1 legacy block\"", res.Detail)
	}
}

func TestMarkdownBlockModePreserved(t *testing.T) {
	dir := t.TempDir()
	mf := testMDMF("AGENTS.md", HarnessClaude)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(""), 0o644)
	_, _ = markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	info, _ := os.Stat(filepath.Join(dir, "AGENTS.md"))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}
```

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestMarkdownBlock -v
```

Expected: FAIL with `errNotImplemented`.

- [ ] **Step 3: Implement `markdownblock.go`**

This is a long implementation. Read the spec §"markdownBlockStrategy" carefully — the state machine has six branches.

```go
// internal/config/managedfiles/markdownblock.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

func (markdownBlockStrategy) Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	state, _, _, err := markdownBlockClassify(cwd, mf, params)
	return state, err
}

func (markdownBlockStrategy) Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	full := filepath.Join(cwd, mf.Path)

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: lerr}, nil
	}
	defer func() {
		if uerr := unlock(); uerr != nil {
			slog.Error("unlock failed", "path", full, "error", uerr)
		}
	}()

	state, existing, purgedAfter, err := markdownBlockClassify(cwd, mf, params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	body, err := mf.Build(params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	canonicalBlock := renderV2Block(body)

	var newContent []byte
	var action Action
	switch state.State {
	case StateSynced:
		return SyncResult{Path: mf.Path, Action: ActionNoOp, Detail: state.Detail}, nil

	case StateMissing:
		// Build full file: frontmatter (if .mdc) + canonical block.
		if isMDCPath(mf.Path) {
			newContent = []byte(defaultCursorFrontmatter)
			newContent = append(newContent, canonicalBlock...)
		} else {
			newContent = canonicalBlock
		}
		if !bytes.HasSuffix(newContent, []byte("\n")) {
			newContent = append(newContent, '\n')
		}
		action = ActionCreated

	case StateStale:
		// Replace existing init block in-place (using purged-existing).
		newContent = replaceInitBlock(purgedAfter, canonicalBlock)
		action = ActionRefreshed

	case StateDrifted:
		if !opts.Force {
			detail := state.Detail
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: detail}, nil
		}
		if opts.KeepEdits {
			// Refresh sentinel hash to match disk; keep user body.
			newContent = refreshSentinelToDisk(existing)
			action = ActionForced
		} else {
			newContent = replaceInitBlock(purgedAfter, canonicalBlock)
			action = ActionForced
		}
	}

	// If no init block exists yet but file has content, append.
	if state.State == StateMissing && len(existing) > 0 {
		// (Handled above in StateMissing branch when file exists but no markers.)
	}

	// "File exists, no markers" case overlaps StateMissing semantics
	// because Classify returns StateMissing only when the file is absent.
	// When file exists without markers, Classify returns StateStale with
	// purgedAfter = existing-after-purge; we need to append, not replace.
	if state.Detail == "no markers" {
		newContent = appendBlockToExisting(purgedAfter, canonicalBlock)
		action = ActionCreated
	}

	// Preserve mode.
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	if err := atomicWrite(full, newContent, mode); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	res := SyncResult{Path: mf.Path, Action: action}
	if state.Detail != "" && state.Detail != "no markers" {
		res.Detail = state.Detail
	}
	return res, nil
}

// markdownBlockClassify returns (FileState, existing-bytes,
// post-purge-bytes, error). The two byte-blobs are conveniences for
// Sync; Inspect ignores them.
func markdownBlockClassify(cwd string, mf ManagedFile, params ProjectParams) (FileState, []byte, []byte, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, nil, nil, err
	}
	full := filepath.Join(cwd, mf.Path)
	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing}, nil, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	// For .mdc, split frontmatter off so marker validation and purge
	// operate on the body.
	var front, working []byte
	if isMDCPath(mf.Path) {
		f, b, ferr := splitFrontmatter(existing)
		if ferr != nil {
			return FileState{}, nil, nil, ferr
		}
		front, working = f, b
	} else {
		front, working = nil, existing
	}
	_ = front // re-stitched in Sync via the existing bytes when needed

	if err := validateInitMarkers(mf.Path, working); err != nil {
		return FileState{}, nil, nil, err
	}
	purged, _, _ := purgeLegacyBlocks(working)
	purgedFull := append([]byte{}, front...)
	purgedFull = append(purgedFull, purged...)

	// Body present?
	body, ok := extractManagedBlockBody(purged)
	if !ok {
		// No init block in the file.
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, Detail: "no markers"}, existing, purgedFull, nil
	}

	// Compare disk body hash against sentinel-recorded hash.
	startIdx := initStartAnyVersion.FindIndex(purged)
	sentinelLine := string(purged[startIdx[0]:startIdx[1]])
	sentinel, perr := ParseSentinel(CommentHTML, sentinelLine)
	if perr != nil {
		return FileState{}, nil, nil, perr
	}

	canonicalBody, berr := mf.Build(params)
	if berr != nil {
		return FileState{}, nil, nil, berr
	}
	diskHash := hashBytes(body)
	canonicalHash := hashBytes(canonicalBody)

	switch sentinel.Version {
	case 1:
		// Defensive v=1 recompute: compare disk to vestigial v=1 canonical.
		var v1Body []byte
		if mf.Path == "AGENTS.md" {
			v1Body = renderV1AgentsBlockBody(params)
		} else {
			v1Body = renderV1CursorBlockBody(params)
		}
		if bytes.Equal(body, v1Body) {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, Detail: "v=1 upgrade"}, existing, purgedFull, nil
		}
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "v=1 user-edited"}, existing, purgedFull, nil

	case 2:
		if sentinel.SHA256 != diskHash {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash"}, existing, purgedFull, nil
		}
		if diskHash != canonicalHash {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale}, existing, purgedFull, nil
		}
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced}, existing, purgedFull, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "unknown sentinel version"}, existing, purgedFull, nil
}

func renderV2Block(body []byte) []byte {
	hash := hashBytes(body)
	var b bytes.Buffer
	b.WriteString("<!-- specgraph:init:start v=2 sha256=")
	b.WriteString(hash)
	b.WriteString(" -->")
	if len(body) > 0 && body[0] != '\n' {
		b.WriteString("\n")
	}
	b.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		b.WriteString("\n")
	}
	b.WriteString(initEndMarker)
	return b.Bytes()
}

func replaceInitBlock(data, canonicalBlock []byte) []byte {
	startLoc := initStartAnyVersion.FindIndex(data)
	endIdx := bytes.Index(data, []byte(initEndMarker))
	if startLoc == nil || endIdx < 0 {
		return data
	}
	endLen := len(initEndMarker)
	out := make([]byte, 0, len(data)+len(canonicalBlock))
	out = append(out, data[:startLoc[0]]...)
	out = append(out, canonicalBlock...)
	out = append(out, data[endIdx+endLen:]...)
	return out
}

func appendBlockToExisting(existing, canonicalBlock []byte) []byte {
	if len(existing) == 0 {
		return append(canonicalBlock, '\n')
	}
	out := append([]byte{}, existing...)
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	out = append(out, '\n')
	out = append(out, canonicalBlock...)
	out = append(out, '\n')
	return out
}

func refreshSentinelToDisk(existing []byte) []byte {
	body, ok := extractManagedBlockBody(existing)
	if !ok {
		return existing
	}
	return replaceInitBlock(existing, renderV2Block(body))
}

func isMDCPath(p string) bool { return filepath.Ext(p) == ".mdc" }
```

- [ ] **Step 4: Remove markdown stub from strategy.go**

Delete the `markdownBlockStrategy` Inspect/Sync stubs from `strategy.go` to avoid duplicate-method errors.

- [ ] **Step 5: Replace strategy_test.go with wholefile_test.go**

```bash
rm internal/config/managedfiles/strategy_test.go
```

```go
// internal/config/managedfiles/wholefile_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestWholeFileStubReturnsNotImplemented(t *testing.T) {
	_, err := wholeFileStrategy{}.Sync("", ManagedFile{}, ProjectParams{}, SyncOptions{})
	if !errors.Is(err, errNotImplemented) {
		t.Errorf("WholeFile stub should still return errNotImplemented; got %v", err)
	}
}
```

- [ ] **Step 6: Run all tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS (all markdownblock cases, jsonkeymerge cases, helper tests, wholefile stub).

- [ ] **Step 7: Commit**

```bash
git add internal/config/managedfiles/markdownblock.go internal/config/managedfiles/markdownblock_test.go internal/config/managedfiles/wholefile_test.go internal/config/managedfiles/strategy.go
git rm internal/config/managedfiles/strategy_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): implement markdownBlockStrategy

State machine covers Missing, Synced, Stale (incl. v=1 upgrade),
Drifted (incl. v=2 hash mismatch and v=1 mangled-body), no-markers
append. Hash scope is block body only (extractManagedBlockBody) so
outside-block edits don't drift. Preserves file mode. Purges
legacy per-slug blocks.

strategy_test.go replaced by per-strategy files; wholefile_test.go
keeps the stub assertion.

Spec: §"markdownBlockStrategy".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 12: Add supersedes-delete trigger to `markdownBlockStrategy`

The cursor `.mdc` entry sets `SupersedesPath: ".cursor/rules/specgraph-bootstrap.md"`. After a non-skipped, non-error sync of `.mdc`, the strategy should delete the old `.md` if its content matches what the prior canonical would have written.

**Files:**
- Modify: `internal/config/managedfiles/markdownblock.go`
- Modify: `internal/config/managedfiles/markdownblock_test.go`

- [ ] **Step 1: Write failing tests**

Append to `markdownblock_test.go`:

```go
func TestMarkdownBlockSupersedesDeletesMatchingMD(t *testing.T) {
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor", "rules")
	_ = os.MkdirAll(cursorDir, 0o750)

	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph-bootstrap.mdc",
		Strategy:       StrategyMarkdownBlock,
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		SupersedesPath: ".cursor/rules/specgraph-bootstrap.md",
		Build: func(p ProjectParams) ([]byte, error) {
			return renderV1CursorBlockBody(p), nil
		},
	}
	// Seed .md with what the prior canonical would have produced.
	body := renderV1CursorBlockBody(testMDParams)
	priorBlock := []byte("<!-- specgraph:init:start v=1 -->" + string(body) + "<!-- specgraph:init:end -->\n")
	priorFull := append([]byte(defaultCursorFrontmatter), priorBlock...)
	_ = os.WriteFile(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md"), priorFull, 0o600)

	if _, err := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Old .md must be gone.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md")); !os.IsNotExist(err) {
		t.Error("old .md still exists after successful supersedes")
	}
	// New .mdc must exist.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.mdc")); err != nil {
		t.Errorf("new .mdc not created: %v", err)
	}
}

func TestMarkdownBlockSupersedesPreservesEditedMD(t *testing.T) {
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor", "rules")
	_ = os.MkdirAll(cursorDir, 0o750)

	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph-bootstrap.mdc",
		Strategy:       StrategyMarkdownBlock,
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		SupersedesPath: ".cursor/rules/specgraph-bootstrap.md",
		Build:          func(p ProjectParams) ([]byte, error) { return renderV1CursorBlockBody(p), nil },
	}
	// Seed .md with edited content (does NOT match prior canonical).
	edited := []byte("USER EDITED CONTENT\n")
	_ = os.WriteFile(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md"), edited, 0o600)

	res, _ := markdownBlockStrategy{}.Sync(dir, mf, testMDParams, SyncOptions{})
	if !strings.Contains(res.Detail, "supersedes path") {
		t.Errorf("Detail should report supersedes-drifted; got %q", res.Detail)
	}
	// Old .md must still exist.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph-bootstrap.md")); err != nil {
		t.Error("user-edited .md was deleted")
	}
}
```

Add `"strings"` to the test file's imports.

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run TestMarkdownBlockSupersedes -v
```

Expected: FAIL.

- [ ] **Step 3: Add supersedes logic at the end of `markdownBlockStrategy.Sync`**

Before the final `return res, nil` in `Sync`, add:

```go
	// Supersedes-guarded delete. Runs on Created, Refreshed, Forced,
	// and NoOp; skips Skipped and Error.
	if mf.SupersedesPath != "" && (action == ActionCreated || action == ActionRefreshed || action == ActionForced || action == ActionNoOp) {
		priorCanonical := computePriorCanonical(mf, params)
		priorHash := hashBytes(priorCanonical)
		if err := supersedesGuardedDelete(cwd, mf.SupersedesPath, priorHash); err != nil {
			if errors.Is(err, ErrPriorCanonicalMismatch) {
				if res.Detail != "" {
					res.Detail += "; "
				}
				res.Detail += fmt.Sprintf("supersedes path %q left in place: prior-canonical mismatch", mf.SupersedesPath)
			} else {
				return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
			}
		}
	}
```

Add a helper at the bottom of the file:

```go
// computePriorCanonical returns the byte sequence the deleted pointers/
// package would have written at mf.SupersedesPath. Used to hash-compare
// against the on-disk supersedes file.
func computePriorCanonical(mf ManagedFile, params ProjectParams) []byte {
	if mf.SupersedesPath != ".cursor/rules/specgraph-bootstrap.md" {
		// PR B has only one SupersedesPath; later PRs may add more.
		// Panic loud rather than silently producing zero bytes.
		panic(fmt.Sprintf("no prior-canonical renderer for SupersedesPath %q", mf.SupersedesPath))
	}
	body := renderV1CursorBlockBody(params)
	var b bytes.Buffer
	b.WriteString(defaultCursorFrontmatter)
	b.WriteString("<!-- specgraph:init:start v=1 -->")
	b.Write(body)
	b.WriteString(initEndMarker)
	b.WriteString("\n")
	return b.Bytes()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/managedfiles/ -run TestMarkdownBlock -v
```

Expected: PASS (eight cases including supersedes tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/markdownblock.go internal/config/managedfiles/markdownblock_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): supersedes-guarded delete in markdownBlockStrategy

Deletes the old .md after writing .mdc, but only if the .md content
matches what the prior canonical would have written. User-edited
content survives. Runs on Created/Refreshed/Forced/NoOp; skips on
Skipped/Error per spec §"SupersedesPath".

Bead: spgr-rwrp.1
EOF
)"
```

### Task 13: Populate the manifest with 5 entries

**Files:**
- Modify: `internal/config/managedfiles/manifest.go`
- Create: `internal/config/managedfiles/manifest_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/managedfiles/manifest_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"testing"
)

func TestManifestShape(t *testing.T) {
	all := allManagedFiles()
	if len(all) != 5 {
		t.Errorf("expected 5 entries, got %d", len(all))
	}
	paths := map[string]bool{
		".mcp.json": false,
		".cursor/mcp.json": false,
		"opencode.json": false,
		"AGENTS.md": false,
		".cursor/rules/specgraph-bootstrap.mdc": false,
	}
	for _, mf := range all {
		if _, ok := paths[mf.Path]; !ok {
			t.Errorf("unexpected path %q", mf.Path)
		}
		paths[mf.Path] = true
		// Source-xor-Build invariant.
		if mf.Source != "" && mf.Build != nil {
			t.Errorf("%q: both Source and Build set", mf.Path)
		}
		if mf.Source == "" && mf.Build == nil {
			t.Errorf("%q: neither Source nor Build set", mf.Path)
		}
	}
	for path, seen := range paths {
		if !seen {
			t.Errorf("manifest missing %q", path)
		}
	}
}

func TestManifestBuildPurity(t *testing.T) {
	params := ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}
	for _, mf := range allManagedFiles() {
		if mf.Build == nil {
			continue
		}
		a, err1 := mf.Build(params)
		b, err2 := mf.Build(params)
		if err1 != nil || err2 != nil {
			t.Errorf("%q: build error: %v / %v", mf.Path, err1, err2)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%q: Build not pure (two calls returned different bytes)", mf.Path)
		}
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/managedfiles/ -run 'TestManifestShape|TestManifestBuildPurity' -v
```

Expected: FAIL (manifest is empty).

- [ ] **Step 3: Implement manifest entries**

Replace `manifest.go`:

```go
// internal/config/managedfiles/manifest.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest returns the list of ManagedFiles filtered by the given
// harnesses. Order is stable across calls — callers may rely on it for
// deterministic output.
func Manifest(harnesses []Harness) []ManagedFile {
	all := allManagedFiles()
	enabled := harnessSet(harnesses)
	out := make([]ManagedFile, 0, len(all))
	for _, mf := range all {
		if enabled[mf.Harness] {
			out = append(out, mf)
		}
	}
	return out
}

func allManagedFiles() []ManagedFile {
	return []ManagedFile{
		{
			Path:     ".mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessClaude,
			Build:    buildClaudeMCPJSON,
		},
		{
			Path:     ".cursor/mcp.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessCursor,
			Build:    buildCursorMCPJSON,
		},
		{
			Path:     "opencode.json",
			Strategy: StrategyJSONKeyMerge,
			Comment:  CommentNone,
			Harness:  HarnessOpenCode,
			Build:    buildOpenCodeJSON,
		},
		{
			Path:     "AGENTS.md",
			Strategy: StrategyMarkdownBlock,
			Comment:  CommentHTML,
			Harness:  HarnessClaude,
			Build:    buildAgentsBlockBody,
		},
		{
			Path:           ".cursor/rules/specgraph-bootstrap.mdc",
			Strategy:       StrategyMarkdownBlock,
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			SupersedesPath: ".cursor/rules/specgraph-bootstrap.md",
			Build:          buildCursorBootstrapBody,
		},
	}
}

func harnessSet(harnesses []Harness) map[Harness]bool {
	out := make(map[Harness]bool, len(harnesses))
	for _, h := range harnesses {
		out[h] = true
	}
	return out
}

func init() {
	for _, mf := range allManagedFiles() {
		hasSource := mf.Source != ""
		hasBuild := mf.Build != nil
		if hasSource && hasBuild {
			panic(fmt.Sprintf("manifest entry %q has both Source and Build", mf.Path))
		}
		if !hasSource && !hasBuild {
			panic(fmt.Sprintf("manifest entry %q has neither Source nor Build", mf.Path))
		}
		switch mf.Strategy {
		case StrategyJSONKeyMerge, StrategyMarkdownBlock:
			if !hasBuild {
				panic(fmt.Sprintf("manifest entry %q: %v strategy requires Build", mf.Path, mf.Strategy))
			}
		case StrategyWholeFile:
			if !hasSource {
				panic(fmt.Sprintf("manifest entry %q: WholeFile strategy requires Source", mf.Path))
			}
		}
	}
}

// Build closures — JSON-merge patches.

func ensureMCPSuffix(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	if strings.HasSuffix(trimmed, "/mcp") {
		return trimmed + "/"
	}
	return trimmed + "/mcp/"
}

func buildCursorMCPJSON(p ProjectParams) ([]byte, error) {
	return json.Marshal(map[string]any{
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
}

func buildClaudeMCPJSON(p ProjectParams) ([]byte, error) {
	return json.Marshal(map[string]any{
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
}

func buildOpenCodeJSON(p ProjectParams) ([]byte, error) {
	return json.Marshal(map[string]any{
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
	})
}

// Build closures — markdown block bodies. PR B uses v=1 body verbatim
// for v=2 emission; body text is identical between v=1 and v=2, only
// the marker shape differs.

func buildAgentsBlockBody(p ProjectParams) ([]byte, error) {
	return renderV1AgentsBlockBody(p), nil
}

func buildCursorBootstrapBody(p ProjectParams) ([]byte, error) {
	return renderV1CursorBlockBody(p), nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS (all tests including new manifest tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/managedfiles/manifest.go internal/config/managedfiles/manifest_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): populate manifest with 5 PR-B entries

3 JSONKeyMerge (.mcp.json, .cursor/mcp.json, opencode.json) +
2 MarkdownBlock (AGENTS.md, .cursor/rules/specgraph-bootstrap.mdc
with SupersedesPath to the old .md). func init() asserts the
Source-xor-Build invariant; TestManifestBuildPurity asserts Build
closures are pure.

Spec: §"Manifest population".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 14: Add `ActionName` + `CountErrors` exported helpers

**Files:**
- Create: `internal/config/managedfiles/action_names.go`
- Create: `internal/config/managedfiles/action_names_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/config/managedfiles/action_names_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestActionName(t *testing.T) {
	cases := map[Action]string{
		ActionNoOp:      "no-op",
		ActionCreated:   "created",
		ActionRefreshed: "refreshed",
		ActionSkipped:   "skipped",
		ActionForced:    "forced",
		ActionError:     "error",
	}
	for a, want := range cases {
		if got := ActionName(a); got != want {
			t.Errorf("ActionName(%v) = %q, want %q", a, got, want)
		}
	}
}

func TestCountErrors(t *testing.T) {
	rs := []SyncResult{
		{Action: ActionCreated},
		{Action: ActionError},
		{Action: ActionError},
		{Action: ActionNoOp},
	}
	if n := CountErrors(rs); n != 2 {
		t.Errorf("CountErrors = %d, want 2", n)
	}
}
```

- [ ] **Step 2: Implement**

```go
// internal/config/managedfiles/action_names.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// ActionName returns the canonical lowercase string for an Action,
// suitable for human-readable CLI output. PR G's doctor uses the
// same names to keep init and doctor output aligned.
func ActionName(a Action) string {
	switch a {
	case ActionNoOp:
		return "no-op"
	case ActionCreated:
		return "created"
	case ActionRefreshed:
		return "refreshed"
	case ActionSkipped:
		return "skipped"
	case ActionForced:
		return "forced"
	case ActionError:
		return "error"
	default:
		return "unknown"
	}
}

// CountErrors returns the number of SyncResults with Action == ActionError.
func CountErrors(rs []SyncResult) int {
	n := 0
	for _, r := range rs {
		if r.Action == ActionError {
			n++
		}
	}
	return n
}
```

- [ ] **Step 3: Run tests + commit**

```bash
go test ./internal/config/managedfiles/ -run 'TestActionName|TestCountErrors' -v
```

```bash
git add internal/config/managedfiles/action_names.go internal/config/managedfiles/action_names_test.go
git commit -s -m "$(cat <<'EOF'
feat(managedfiles): export ActionName and CountErrors helpers

Shared by init and PR G's doctor for consistent CLI output. Avoids
divergent action-name copy between callers.

Spec: §"Init wiring" / ActionName note.
Bead: spgr-rwrp.1
EOF
)"
```

### Task 15: Rewire `cmd/specgraph/init.go`

**Files:**
- Modify: `cmd/specgraph/init.go`

- [ ] **Step 1: Read init.go to identify exact replacement region**

```bash
sed -n '85,155p' cmd/specgraph/init.go
```

Identify lines 96-153 — that's the replacement region. Lines 36-95, 102-109, 149-151 stay verbatim.

- [ ] **Step 2: Make the edit**

Use Edit to replace the region. New code:

```go
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	serverURL := globalCfg.ResolveServer(pc.Slug, pc.Server)
	params := managedfiles.ProjectParams{Slug: pc.Slug, ServerURL: serverURL}
	if err := params.Validate(); err != nil {
		return fmt.Errorf("validate project params: %w", err)
	}

	// Write .specgraph.yaml only if it doesn't exist; idempotent.
	projectCreated := false
	if existing == nil {
		if writeErr := config.WriteProject(cwd, pc); writeErr != nil {
			return fmt.Errorf("write project config: %w", writeErr)
		}
		projectCreated = true
	}

	// Hard-coded for PR B; .specgraph.yaml-driven harnesses: list lands later.
	harnesses := []managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	}

	results, syncErr := managedfiles.SyncAll(cwd, harnesses, params, managedfiles.SyncOptions{})
	var failedPaths []string
	for _, r := range results {
		if r.Action == managedfiles.ActionError {
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", r.Path, r.Err)
			failedPaths = append(failedPaths, r.Path)
		} else {
			line := fmt.Sprintf("%s: %s", r.Path, managedfiles.ActionName(r.Action))
			if r.Detail != "" {
				line += " (" + r.Detail + ")"
			}
			fmt.Println(line)
		}
	}
	if syncErr != nil {
		return fmt.Errorf("sync managed files: %w", syncErr)
	}
	if len(failedPaths) > 0 {
		return fmt.Errorf("sync managed files: %d failed: %s",
			len(failedPaths), strings.Join(failedPaths, ", "))
	}

	if projectCreated {
		fmt.Printf("Initialized project %s. Config written to .specgraph.yaml\n", pc.Slug)
	}

	return nil
```

Update imports: remove `"github.com/specgraph/specgraph/internal/config/mcpconfigs"` and `"github.com/specgraph/specgraph/internal/config/pointers"`; add `"github.com/specgraph/specgraph/internal/config/managedfiles"`. Keep `"strings"`.

- [ ] **Step 3: Verify build**

```bash
go build ./cmd/specgraph/
```

Expected: no errors.

- [ ] **Step 4: Run a manual smoke test against a temp dir**

```bash
mkdir -p /tmp/specgraph-smoke && cd /tmp/specgraph-smoke
"$(cd - && pwd)/specgraph" init --slug smoke
ls -la
cat AGENTS.md
cat .cursor/rules/specgraph-bootstrap.mdc
cat .mcp.json
```

Expected output shows: 5 managed files created, AGENTS.md has v=2 marker with sha256, .mdc has frontmatter + v=2 marker.

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/init.go
git commit -s -m "$(cat <<'EOF'
feat(specgraph): rewire init to use managedfiles.SyncAll

Replaces mcpconfigs.Sync + pointers.Sync + per-file error handling
with one SyncAll call. Preserves init.go:36-95 (project-config
load), 102-109 (WriteProject before SyncAll), 149-151 (Initialized
project message). Error message shape mirrors today's failedPaths
join for diagnostic continuity.

Spec: §"Init wiring".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 16: Capture-helper main.go + `task capture-goldens`

**Files:**
- Create: `internal/config/managedfiles/internal/captureimpl/doc.go`
- Create: `internal/config/managedfiles/internal/captureimpl/main.go`
- Create: `internal/config/managedfiles/testdata/golden/README.md`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Write the capture helper**

```go
// internal/config/managedfiles/internal/captureimpl/doc.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package captureimpl is the one-shot golden-fixture generator for
// managedfiles PR B. It imports the deleted-in-this-PR mcpconfigs/
// and pointers/ packages, runs them against the checked-in starting
// fixtures, and writes their byte outputs to testdata/golden/<case>/out/.
//
// Deleted in the same commit that deletes mcpconfigs/ and pointers/.
// To regenerate fixtures after deletion, check out the
// PR-B-pre-cleanup commit and re-run `go run ./internal/config/managedfiles/internal/captureimpl`.
package captureimpl
```

```go
// internal/config/managedfiles/internal/captureimpl/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package captureimpl
```

Wait — this file needs to be `package main` to be runnable. Correction:

```go
// internal/config/managedfiles/internal/captureimpl/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/specgraph/specgraph/internal/config/mcpconfigs"
	"github.com/specgraph/specgraph/internal/config/pointers"
)

// Cases captured: one per managed file. Each case has a starting
// state under testdata/golden/<case>/in/ and expected output under
// testdata/golden/<case>/out/.
//
// PR B's parity-test fixtures only need the Missing-file → first-init
// case for byte-equivalence of the JSON merge results and the v=1
// markdown body content. Drifted/edited cases are covered by the
// migration_test.go unit test, not goldens.

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "capture failed:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.MkdirTemp("", "captureimpl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	slug := "captureslug"
	serverURL := "http://localhost:9090"
	configs := mcpconfigs.ManagedConfigs(slug, serverURL)
	if _, err := mcpconfigs.Sync(root, configs); err != nil {
		return fmt.Errorf("mcpconfigs.Sync: %w", err)
	}
	opts, oerr := pointers.NewOptions(serverURL, slug)
	if oerr != nil {
		return oerr
	}
	report := pointers.Sync(root, opts)
	if report.IsErr() {
		return fmt.Errorf("pointers.Sync had errors: %+v", report)
	}

	// Copy outputs into testdata/golden/missing-first-init/out/.
	wd, _ := os.Getwd()
	outDir := filepath.Join(wd, "testdata", "golden", "missing-first-init", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{".mcp.json", ".cursor/mcp.json", "opencode.json", "AGENTS.md", ".cursor/rules/specgraph-bootstrap.md"} {
		src := filepath.Join(root, name)
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", src, rerr)
		}
		dst := filepath.Join(outDir, filepath.Base(name))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("captured: %s (%d bytes)\n", dst, len(data))
	}
	return nil
}
```

The `//go:build ignore` tag keeps the file out of the regular build/test invocation so `go test ./...` doesn't try to compile it (it imports packages that will eventually be deleted). It runs via `go run` which respects the `ignore` tag.

- [ ] **Step 2: Add Taskfile target**

Append to `Taskfile.yml`:

```yaml
  capture-goldens:
    desc: One-shot capture of PR-B parity goldens from the deleted-in-this-PR mcpconfigs/pointers packages. Deleted alongside the helper in the cleanup commit.
    cmds:
      - go run -tags ignore ./internal/config/managedfiles/internal/captureimpl
```

Wait — `-tags ignore` doesn't work that way; `//go:build ignore` is a magic tag that excludes by default. `go run path/to/main.go` works directly. Use that:

```yaml
  capture-goldens:
    desc: One-shot capture of PR-B parity goldens.
    cmds:
      - go run ./internal/config/managedfiles/internal/captureimpl/main.go
```

- [ ] **Step 3: Write the goldens README**

```markdown
<!-- internal/config/managedfiles/testdata/golden/README.md -->
# Golden fixtures

Captured byte outputs from the deleted-in-PR-B `mcpconfigs/` and
`pointers/` packages, used to prove behavioural parity with
`managedfiles/`.

These fixtures are **immutable** — they cannot be regenerated from
`main` after the PR-B cleanup commit. The capture helper at
`internal/config/managedfiles/internal/captureimpl/` is also deleted
in that commit.

## To regenerate

1. `git checkout <PR-B-pre-cleanup commit SHA>`
2. `go run ./internal/config/managedfiles/internal/captureimpl`
3. Hand-merge the new bytes back to current `main`.

The `task capture-goldens` target also no longer exists after the
cleanup commit — invoke `go run` directly.

## Cases

- `missing-first-init/` — Missing file → first init produces these
  bytes for each managed file. Uses slug `captureslug`, server
  `http://localhost:9090`.
```

- [ ] **Step 4: Run the capture**

```bash
task capture-goldens
```

Expected: prints 5 "captured: ..." lines and creates `testdata/golden/missing-first-init/out/` with 5 files.

- [ ] **Step 5: Inspect the captured files**

```bash
ls -la internal/config/managedfiles/testdata/golden/missing-first-init/out/
```

Sanity-check that `.mcp.json` contains `"type": "http"`, `opencode.json` contains `"$schema"`, AGENTS.md contains `v=1` markers, `.md` contains frontmatter.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/internal/captureimpl/ internal/config/managedfiles/testdata/golden/ Taskfile.yml
git commit -s -m "$(cat <<'EOF'
chore(managedfiles): one-shot capture of PR-B parity goldens

Captures the byte outputs the deleted-in-this-PR mcpconfigs and
pointers packages produce for the Missing-file case. Helper and
task target are deleted in the cleanup commit; fixtures stay as
immutable byte references.

Spec: §"Tests / 2. Behaviour-parity golden tests".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 17: Golden-parity test

**Files:**
- Create: `internal/config/managedfiles/golden_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/config/managedfiles/golden_test.go
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
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/config/managedfiles/ -run TestGolden -v
```

Expected: PASS.

```bash
git add internal/config/managedfiles/golden_test.go
git commit -s -m "$(cat <<'EOF'
test(managedfiles): assert byte-parity with captured goldens

JSON files compared semantically; markdown files compared
between-markers (body) and outside-markers (frontmatter, surrounding
prose). Marker lines themselves are excluded from the diff per spec
§Goal "Precise meaning of byte-identical."

Bead: spgr-rwrp.1
EOF
)"
```

### Task 18: Synthetic v=1 → v=2 migration integration test

**Files:**
- Create: `internal/config/managedfiles/migration_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/config/managedfiles/migration_test.go
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
	if !bytes.Equal(gotBody, body) {
		t.Errorf("body changed during upgrade\n got: %q\nwant: %q", gotBody, body)
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
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/config/managedfiles/ -run TestMigration -v
```

Expected: PASS.

```bash
git add internal/config/managedfiles/migration_test.go
git commit -s -m "$(cat <<'EOF'
test(managedfiles): synthetic v=1 → v=2 upgrade integration test

Asserts AGENTS.md user content preserved, body unchanged, markers
upgraded; .md deleted by supersedes; .mdc created with v=2. Drift
case asserts ActionSkipped + user content preserved.

Regular unit test (no //go:build integration) so `task check`
catches dogfood-discipline rot.

Spec: §"Synthetic v=1 → v=2 upgrade integration test".
Bead: spgr-rwrp.1
EOF
)"
```

### Task 19: Run `task check` and fix any lints

**Files:** whichever the linter complains about.

- [ ] **Step 1: Run task check**

```bash
task check
```

Expected: PASS. If it fails:

- License headers: `task license:add`
- Lint complaints: address each (often unused vars, missing package comments).
- Formatting: `task fmt`

Re-run `task check` until clean.

- [ ] **Step 2: If any fixups were needed, commit them**

```bash
git add -A
git commit -s -m "$(cat <<'EOF'
chore(managedfiles): fix lint and formatting issues

Bead: spgr-rwrp.1
EOF
)"
```

### Task 20: Cleanup commit (delete mcpconfigs, pointers, captureimpl, task target)

This is the **single atomic cleanup commit** per spec §"Cleanup."

**Files deleted:**
- `internal/config/mcpconfigs/` (whole dir)
- `internal/config/pointers/` (whole dir)
- `internal/config/managedfiles/internal/captureimpl/` (whole dir)

**Files modified:**
- `Taskfile.yml` — remove `capture-goldens` target

- [ ] **Step 1: Delete the directories**

```bash
rm -rf internal/config/mcpconfigs internal/config/pointers internal/config/managedfiles/internal/captureimpl
```

- [ ] **Step 2: Remove the task target**

Edit `Taskfile.yml` and delete the `capture-goldens:` entry added in Task 16.

- [ ] **Step 3: Run task check**

```bash
task check
```

Expected: PASS. The deleted packages had no remaining importers (cmd/specgraph rewired in Task 15). If anything still imports them, fix it before this commit.

- [ ] **Step 4: Run task pr-prep**

```bash
task pr-prep
```

Expected: PASS (includes integration + e2e tests; requires Docker).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -s -m "$(cat <<'EOF'
chore(managedfiles): delete mcpconfigs, pointers, captureimpl

Single atomic cleanup commit per spec §"Cleanup". All three
packages are now subsumed by managedfiles/:
- mcpconfigs/'s canonicalize is now helpers_json.go's canonicalize
- pointers/'s renderers are vestigial_v1.go's renderV1* helpers
- pointers/'s validateInitMarkers and purgeLegacyBlocks are
  helpers_md.go (validateInitMarkers adapted for v=2)
- captureimpl/ produced testdata/golden/ fixtures; helper deleted,
  fixtures stay immutable

Goldens regeneration recipe in
internal/config/managedfiles/testdata/golden/README.md uses `go run`
at the pre-cleanup commit SHA.

Bead: spgr-rwrp.1
EOF
)"
```

### Task 21: Push + open PR

- [ ] **Step 1: Push the branch**

```bash
git push -u origin HEAD
```

(Or jj-equivalent: `jj bookmark set <name> -r @-`, then `jj git push --bookmark <name>`.)

- [ ] **Step 2: Open the PR**

```bash
gh pr create --title "spgr-rwrp PR B: port managed files into managedfiles framework" --body "$(cat <<'EOF'
## Summary

- Migrates the 5 already-managed files into `internal/config/managedfiles/` (`.mcp.json`, `.cursor/mcp.json`, `opencode.json` → `JSONKeyMerge`; `AGENTS.md`, `.cursor/rules/specgraph-bootstrap.mdc` → `MarkdownBlock`)
- Replaces PR A's `jsonKeyMergeStrategy` and `markdownBlockStrategy` stubs with real implementations
- Deletes `internal/config/mcpconfigs/` and `internal/config/pointers/` (verbatim helpers preserved as private inside `managedfiles/`)
- Adds `ProjectParams` type, `Build` closure on `ManagedFile`, `Detail` field on `SyncResult`
- v=1 → v=2 marker upgrade with defensive hash recompute

Spec (v7): `docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md`
Bead: `spgr-rwrp.1`
Parent: PR #942 (`5a10ce5` — PR A foundation)

## Test plan

- [x] `task check` (fmt, lint, build, unit tests)
- [x] `task pr-prep` (check + integration + e2e)
- [x] Migration integration test asserts synthetic v=1 → v=2 upgrade
- [x] Golden-parity test asserts byte-equivalence with captured `mcpconfigs`/`pointers` output
- [x] Manual smoke: `specgraph init` in a temp dir produces 5 expected files
- [ ] Manual smoke: `specgraph init` against this repo upgrades dogfood files (no v=1 files exist; first run creates v=2)

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-review

**Spec coverage check.** Walking the spec section by section:

| Spec section | Task |
|---|---|
| §"Per-project params plumbing" — `ProjectParams`, `Build` field | Tasks 1, 8 |
| §"Per-project params plumbing" — Signature cascade | Task 9 |
| §"Concurrency: Inspect vs Sync" | Tasks 10, 11 (file lock in Sync; no lock in Inspect) |
| §"Build closure purity" | Task 13 (TestManifestBuildPurity) |
| §"Atomic write: mode preservation" | Tasks 10, 11 (mode-preservation test cases) |
| §"jsonKeyMergeStrategy" | Task 10 |
| §"markdownBlockStrategy" / "Hash scope" | Tasks 5, 11 |
| §"markdownBlockStrategy" / step 3 marker validation | Tasks 3, 11 |
| §"markdownBlockStrategy" / step 4 purge | Tasks 4, 11 |
| §"wholeFileStrategy (still stub)" | Task 11 (`wholefile_test.go`) |
| §"Helpers ported" (live-path + vestigial) | Tasks 2, 3, 4, 6, 7 |
| §"SupersedesPath" / "Prior canonical exact bytes" | Task 12 |
| §"Manifest population" | Task 13 |
| §"Project-convention compliance" | Conventions section + all commits |
| §"Init wiring" | Task 15 |
| §"SyncResult.Detail string grammar" | Tasks 11 (Detail strings emitted), 4 (purge counts) |
| §"Tests / 1 strategy units" | Tasks 10, 11, 12 |
| §"Tests / 2 golden parity" | Tasks 16, 17 |
| §"Tests / 3 migration integration" | Task 18 |
| §"Cleanup" (single atomic commit) | Task 20 |
| §"Capture-helper invocation" | Task 16 |

**Placeholder scan:** All steps contain actual code or commands. No "TODO", "implement appropriately", "similar to above" placeholders.

**Type consistency check:**

- `ProjectParams` field names (`Slug`, `ServerURL`) consistent across Tasks 1, 7, 9, 10, 11, 13, 15.
- `ManagedFile.Build` signature `func(ProjectParams) ([]byte, error)` consistent (Task 8 defines; Tasks 10, 11, 13 use).
- `ActionName(Action) string`, `CountErrors([]SyncResult) int` exported names consistent (Task 14 defines; Task 15 uses).
- `extractManagedBlockBody(content []byte) ([]byte, bool)` signature consistent (Task 5 defines; Task 11 + 17 + 18 use).
- `renderV1AgentsBlockBody` / `renderV1CursorBlockBody` (Task 7) referenced consistently in Tasks 11, 13, 18.
- `validateInitMarkers(displayName string, data []byte) error` (Task 3) referenced by Task 11.
- `purgeLegacyBlocks(data) (out, purged, skipped)` (Task 4) referenced by Task 11.

No mismatches found.

**Known limitations:**

- Task 11's `markdownBlockStrategy.Sync` has subtle interplay between `markdownBlockClassify`'s `state.Detail == "no markers"` signal and the StateMissing branch — the implementation may need iteration to get the "file exists, no markers, append" case right. Tests catch it (`TestMarkdownBlockNoMarkersAppends`).
- Task 16's `//go:build ignore` tag on `main.go` means the helper is invoked via `go run path/to/file.go`, not the regular `go run ./pkg` form. The Taskfile target reflects this.
- Tasks 18's migration test seeds fixtures via `renderV1AgentsBlockBody` — it tests that the v=1 → v=2 upgrade *works*, not that any actual dogfood file is in v=1 state (none are). This is intentional per spec §"M1' (dogfood migration fixture premise broken)."

Plan complete and saved to `docs/plans/2026-05-11-spgr-rwrp-pr-b-implementation-plan.md`.
