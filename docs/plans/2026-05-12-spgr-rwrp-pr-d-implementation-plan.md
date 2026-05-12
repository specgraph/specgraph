# spgr-rwrp PR D — Cursor rule files implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `.cursor/rules/specgraph.mdc` and `.cursor/rules/specgraph-post-stage.mdc` as init-managed `WholeFile` entries, with frontmatter-aware sentinel positioning and hash-guarded cleanup of pre-rename `.md` copies.

**Architecture:** Extend `internal/config/managedfiles/` with a `HasFrontmatter` flag on `ManagedFile`, a frontmatter-aware whole-file render/classify path, and a sibling hash function. Move canonical content into `internal/config/managedfiles/embedded/cursor/*.mdc` (with the same reverse-symlink pattern PR C established for OpenCode). Preserve pre-rename `.md` bytes as embedded vestigial constants so `supersedesGuardedDelete` can recognize verbatim user copies. Wire two new manifest entries with `SupersedesPath` pointing at the pre-rename `.md` paths.

**Tech Stack:** Go 1.22+, `//go:embed`, `connectrpc` framework, golangci-lint, lefthook hooks, jujutsu (jj-colocated git). Project uses Taskfile.dev — every gate goes through `task check`.

**Design doc:** [`2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md`](2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md)
**Parent epic:** [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md)
**PR C precedent:** [`2026-05-11-spgr-rwrp-pr-c-implementation-plan.md`](2026-05-11-spgr-rwrp-pr-c-implementation-plan.md)

---

## Project conventions (read before starting)

- **License headers:** every new `.go` file MUST start with `// SPDX-License-Identifier: Apache-2.0\n// Copyright 2026 Sean Brandt\n\n`. Use `task license:add` if you miss one.
- **Package doc comments:** new packages need a `// Package foo ...` comment on the first `.go` file or `revive` fails. (This plan adds no new packages.)
- **DCO sign-off:** every commit requires `Signed-off-by:` trailer. Use `git commit -s` or `jj describe` with a trailer.
- **Conventional commits:** subject `feat(scope): summary` or `fix(scope): summary` etc. The `cog` hook enforces this.
- **jj-colocated repo:** prefer `jj describe -m`, `jj commit -m`, `jj new -m`. NEVER `git push`; use `jj bookmark set <name> -r <rev>` then `jj git push --bookmark <name>`. NEVER `git worktree`; use `jj workspace add`.
- **Per-file errors flow through `SyncResult.Err`** with `Action: ActionError`, NOT Go-error returns from Sync. The `//nolint:nilerr` pattern is correct under this framework contract.
- **Test discipline:** all tests are regular unit tests (no `//go:build integration` tag) so `task check` enforces them. Postgres-backed tests are the exception — irrelevant here.
- **bd for task tracking:** do NOT use TodoWrite or TaskCreate. Track progress in the bead at task close.

## Bead

Before starting Task 1, file a child bead under `spgr-rwrp`:

```bash
bd create --parent spgr-rwrp --priority P2 \
  "spgr-rwrp PR D: Cursor rule files via embed-and-write" \
  --description "Add .cursor/rules/specgraph.mdc and specgraph-post-stage.mdc as init-managed WholeFile entries with frontmatter-aware sentinel positioning. See docs/plans/2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md."
bd update <new-id> --claim
```

---

## Task 1: Refactor manifest invariants into a testable function

**Why first:** the existing `init()` block (`manifest.go:83-104`) inlines invariant checks that panic at package load. Subsequent tasks add two new invariant rules; we need a way to test that bad manifests panic without breaking the test binary's load. Extract the inline logic into `validateManifestEntry(mf ManagedFile)` that returns an error; `init()` calls it and panics; tests call it directly.

**Files:**

- Modify: `internal/config/managedfiles/manifest.go:83-104` (extract `init()` body into `validateManifestEntry`)
- Test: `internal/config/managedfiles/manifest_test.go` (add `TestValidateManifestEntry`)

- [ ] **Step 1.1: Write failing tests for `validateManifestEntry`**

Append to `internal/config/managedfiles/manifest_test.go`:

```go
func TestValidateManifestEntry(t *testing.T) {
	cases := []struct {
		name    string
		mf      ManagedFile
		wantErr string
	}{
		{
			name: "both Source and Build set",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source: "s", Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
			wantErr: "has both Source and Build",
		},
		{
			name:    "neither Source nor Build set",
			mf:      ManagedFile{Path: "x", Strategy: StrategyWholeFile},
			wantErr: "has neither Source nor Build",
		},
		{
			name: "JSONKeyMerge without Build",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyJSONKeyMerge,
				Source: "s",
			},
			wantErr: "JSONKeyMerge strategy requires Build",
		},
		{
			name: "WholeFile without Source",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
			wantErr: "WholeFile strategy requires Source",
		},
		{
			name: "valid WholeFile",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source: "s",
			},
		},
		{
			name: "valid JSONKeyMerge",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyJSONKeyMerge,
				Build: func(ProjectParams) ([]byte, error) { return nil, nil },
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateManifestEntry(tc.mf)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
```

Add the imports if missing — `manifest_test.go` already imports `testing`; you'll need `strings` too.

- [ ] **Step 1.2: Run the test, confirm it fails on undefined `validateManifestEntry`**

Run: `cd /Users/SeBrandt/Code/github.com/specgraph && go test -run TestValidateManifestEntry ./internal/config/managedfiles/`
Expected: build failure, "undefined: validateManifestEntry".

- [ ] **Step 1.3: Extract `validateManifestEntry` from `init()`**

Edit `internal/config/managedfiles/manifest.go`. Replace the existing `init()` function (lines 83-104) with:

```go
func init() {
	for _, mf := range allManagedFiles() {
		if err := validateManifestEntry(mf); err != nil {
			panic(err.Error())
		}
	}
}

// validateManifestEntry returns nil if mf satisfies the package's manifest
// invariants, or a descriptive error otherwise. Called from init() at package
// load (where any error panics) and directly from tests that want to
// exercise invariant rules without crashing the test binary.
func validateManifestEntry(mf ManagedFile) error {
	hasSource := mf.Source != ""
	hasBuild := mf.Build != nil
	if hasSource && hasBuild {
		return fmt.Errorf("manifest entry %q has both Source and Build", mf.Path)
	}
	if !hasSource && !hasBuild {
		return fmt.Errorf("manifest entry %q has neither Source nor Build", mf.Path)
	}
	switch mf.Strategy {
	case StrategyJSONKeyMerge, StrategyMarkdownBlock:
		if !hasBuild {
			return fmt.Errorf("manifest entry %q: %v strategy requires Build", mf.Path, mf.Strategy)
		}
	case StrategyWholeFile:
		if !hasSource {
			return fmt.Errorf("manifest entry %q: WholeFile strategy requires Source", mf.Path)
		}
	}
	return nil
}
```

- [ ] **Step 1.4: Run the test, confirm it passes**

Run: `go test -run TestValidateManifestEntry ./internal/config/managedfiles/`
Expected: PASS.

- [ ] **Step 1.5: Run the full package tests to confirm nothing else regressed**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 1.6: Commit**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
jj describe -m "refactor(managedfiles): extract manifest invariants into testable validateManifestEntry

Move the inline checks from init() into validateManifestEntry() so future
invariant rules can be unit-tested without crashing the test binary on bad
fixtures. init() now calls the function and panics on error — observable
behavior at package load is unchanged.

Refs: spgr-rwrp PR D scaffolding

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 2: Drop `:start` suffix from `RenderSentinel(CommentHTML, ...)`

**Why now:** the sentinel form is a building block used by every subsequent test. Get it correct before tests that depend on the rendered bytes are written.

**Files:**

- Modify: `internal/config/managedfiles/sentinel.go:79-82` (`CommentHTML` branch)
- Modify: `internal/config/managedfiles/sentinel_test.go:28-34, 105-115` (assertions on rendered form)

- [ ] **Step 2.1: Read `sentinel_test.go` and identify the assertions to update**

Run: `cd /Users/SeBrandt/Code/github.com/specgraph && grep -n "init:start\|init:start" internal/config/managedfiles/sentinel_test.go`
Expected output: at least line 32-ish (`TestRenderSentinel_CommentHTML`) and any round-trip test that checks the rendered string.

- [ ] **Step 2.2: Update the failing test cases to expect the bare form**

Open `internal/config/managedfiles/sentinel_test.go`. Find the `TestRenderSentinel_CommentHTML` function (around line 27). Update the expected string so the test asserts the bare form `<!-- specgraph:init v=2 sha256=abc rev=cef -->` (NOT `<!-- specgraph:init:start v=2 ... -->`). Update any other assertion in the file that pins the `:start`-suffixed CommentHTML output.

After your edits, add a defense-in-depth test below the existing tests:

```go
func TestParseSentinel_AcceptsLegacyStartForm(t *testing.T) {
	// markdownblock.go writes "<!-- specgraph:init:start v=2 sha256=... -->"
	// inline. Confirm the parser still accepts that form so block-strategy
	// files written by older binaries remain readable.
	legacy := "<!-- specgraph:init:start v=2 sha256=abc -->"
	s, err := ParseSentinel(CommentHTML, legacy)
	if err != nil {
		t.Fatalf("parse legacy form: %v", err)
	}
	if s.Version != 2 || s.SHA256 != "abc" {
		t.Errorf("parsed sentinel = %+v, want {Version:2, SHA256:abc}", s)
	}
}
```

- [ ] **Step 2.3: Run the tests, confirm `TestRenderSentinel_CommentHTML` fails (expected) and `TestParseSentinel_AcceptsLegacyStartForm` passes**

Run: `go test -run 'TestRenderSentinel_CommentHTML|TestParseSentinel_AcceptsLegacyStartForm' -v ./internal/config/managedfiles/`
Expected:

- `TestRenderSentinel_CommentHTML`: FAIL (rendered string still includes `:start`)
- `TestParseSentinel_AcceptsLegacyStartForm`: PASS (parser already accepts both forms)

- [ ] **Step 2.4: Update `RenderSentinel`'s `CommentHTML` branch to emit the bare form**

Edit `internal/config/managedfiles/sentinel.go`. Replace lines 79-82 (the `case CommentHTML:` branch in `RenderSentinel`) with:

```go
	case CommentHTML:
		// Bare form: no `:start` suffix. Whole-file callers (wholefile.go)
		// emit this as a single standalone sentinel line; the markdown-block
		// strategy emits `<!-- specgraph:init:start ... -->` inline via
		// string concatenation in markdownblock.go and does not go through
		// RenderSentinel.
		return "<!-- " + body + " -->"
```

Also update the docstring comment block (around lines 53-62) to remove the now-obsolete claim that `CommentHTML` returns "the START marker only":

```go
// RenderSentinel formats a Sentinel as a single line in the given comment
// syntax. The returned line includes the comment delimiters but no trailing
// newline.
//
// For CommentNone, returns the empty string (JSON files don't carry sentinels).
//
// CommentHTML renders a standalone whole-file sentinel (e.g.
// "<!-- specgraph:init v=2 sha256=... -->"). The markdown-block strategy
// writes its `:start`/`:end` markers inline and does not call this function.
```

- [ ] **Step 2.5: Run the tests, confirm both pass**

Run: `go test -run 'TestRenderSentinel_CommentHTML|TestParseSentinel_AcceptsLegacyStartForm' -v ./internal/config/managedfiles/`
Expected: both PASS.

Then run the whole package:
Run: `go test ./internal/config/managedfiles/`
Expected: all PASS. If any other test in the package depended on the `:start`-suffixed output (most likely a `wholefile_test.go` test on `CommentSlash` is unaffected, but double-check), update its expected string in place — those updates are part of this commit.

- [ ] **Step 2.6: Commit**

```bash
jj describe -m "feat(managedfiles): drop \`:start\` suffix from RenderSentinel CommentHTML output

The \`:start\`/\`:end\` markers carry meaning only in MarkdownBlock context
(where they bracket a managed region). For standalone whole-file sentinels,
the suffix is misleading — there's no matching \`:end\`. RenderSentinel's
only non-test caller is wholefile.go:132; markdownblock.go writes its
inline \`:start\` markers via string concatenation, unaffected by this
change. ParseSentinel accepts both forms (init(?::start)?) so on-disk
back-compat is free.

Refs: spgr-rwrp PR D §3 framework change

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 3: Add `HashExcludingSentinelAfterFrontmatter`

**Why now:** Task 4 (frontmatter-aware classify/render) calls this function. It's an independent unit with a small surface — implement and test in isolation first.

**Files:**

- Modify: `internal/config/managedfiles/hash.go` (add sibling function)
- Modify: `internal/config/managedfiles/hash_test.go` (add test cases)

- [ ] **Step 3.1: Write the failing test**

Append to `internal/config/managedfiles/hash_test.go`:

```go
func TestHashExcludingSentinelAfterFrontmatter(t *testing.T) {
	body := "---\ndescription: x\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=abc -->\n# Heading\n\nbody text\n"

	got, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected hash: same content with the sentinel line removed.
	want := hashBytes([]byte("---\ndescription: x\nalwaysApply: false\n---\n\n# Heading\n\nbody text\n"))
	if got != want {
		t.Errorf("hash mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_StableAcrossRevChanges(t *testing.T) {
	a := "---\ndescription: x\n---\n\n<!-- specgraph:init v=2 sha256=abc -->\nbody\n"
	b := "---\ndescription: x\n---\n\n<!-- specgraph:init v=2 sha256=abc rev=deadbeef -->\nbody\n"
	ha, errA := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(a))
	hb, errB := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(b))
	if errA != nil || errB != nil {
		t.Fatalf("hash errors: %v / %v", errA, errB)
	}
	if ha != hb {
		t.Errorf("hash differs across sentinel-only changes: %s vs %s", ha, hb)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_NoFrontmatter(t *testing.T) {
	body := "no frontmatter here\nbody\n"
	_, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if !errors.Is(err, ErrFrontmatterMissing) {
		t.Errorf("error = %v, want ErrFrontmatterMissing", err)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_NoSentinelOnBodyFirstLine(t *testing.T) {
	// First body line is a heading, not a sentinel. The function should
	// hash the file content unchanged (no line dropped) — the classifier
	// is responsible for treating the absence of a sentinel as Drifted.
	body := "---\ndescription: x\n---\n\n# Heading\nbody\n"
	got, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := hashBytes([]byte(body))
	if got != want {
		t.Errorf("hash mismatch:\n got=%s\nwant=%s", got, want)
	}
}
```

Add the `errors` import to the test file if not present.

- [ ] **Step 3.2: Run the tests, confirm they fail**

Run: `go test -run 'TestHashExcludingSentinelAfterFrontmatter' -v ./internal/config/managedfiles/`
Expected: build failure, "undefined: HashExcludingSentinelAfterFrontmatter".

- [ ] **Step 3.3: Implement `HashExcludingSentinelAfterFrontmatter`**

Append to `internal/config/managedfiles/hash.go` (after `HashExcludingSentinel`):

```go
// HashExcludingSentinelAfterFrontmatter splits leading YAML frontmatter off
// content, removes the sentinel on the first line of the post-frontmatter
// body (if present), and hashes the concatenation of front + remaining body.
//
// Returns ErrFrontmatterMissing if the content does not begin with `---\n` or
// the frontmatter is unclosed — callers (the WholeFile strategy on entries
// with HasFrontmatter==true) treat that as Drifted and refuse to mutate.
//
// If the first body line is not a sentinel (parses to Version 0), the body
// is hashed unchanged — drift classification is the classifier's job, not
// this hash function's.
func HashExcludingSentinelAfterFrontmatter(syntax CommentSyntax, content []byte) (string, error) {
	front, body, err := splitFrontmatter(content)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return hashBytes(front), nil
	}
	firstLine, rest, _ := bytes.Cut(body, []byte("\n"))
	s, perr := ParseSentinel(syntax, string(firstLine))
	if perr != nil {
		// Corrupt sentinel — surface it. Callers should classify the file
		// as Drifted with the parse error in Detail.
		return "", perr
	}
	if s.Version == 0 {
		// No sentinel on body[0]. Hash the body unchanged.
		return hashBytes(append(append([]byte{}, front...), body...)), nil
	}
	// Sentinel present — drop the first line.
	return hashBytes(append(append([]byte{}, front...), rest...)), nil
}
```

Add the `bytes` import to `hash.go` if not already present.

- [ ] **Step 3.4: Run the tests, confirm all pass**

Run: `go test -run 'TestHashExcludingSentinelAfterFrontmatter' -v ./internal/config/managedfiles/`
Expected: all four cases PASS.

- [ ] **Step 3.5: Run the whole package, confirm nothing else regressed**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 3.6: Commit**

```bash
jj describe -m "feat(managedfiles): add HashExcludingSentinelAfterFrontmatter

Sibling to HashExcludingSentinel for files with leading YAML frontmatter.
Splits the frontmatter off, removes the sentinel on the first post-frontmatter
line if present, and hashes (front + remaining-body). Returns
ErrFrontmatterMissing on malformed/unclosed frontmatter.

Used by the WholeFile strategy on entries with HasFrontmatter==true (added
in the next commit). Existing HashExcludingSentinel and its call sites
are unchanged.

Refs: spgr-rwrp PR D §2 framework change

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 4: Extend `wholeFileStrategy` to handle `HasFrontmatter`

**Why now:** the rest of the PR is about manifest entries that use the new flag. The strategy needs to honor it before those entries can be tested end-to-end.

**Files:**

- Modify: `internal/config/managedfiles/types.go` (add `HasFrontmatter bool` field — also accepted by validation in Task 7)
- Modify: `internal/config/managedfiles/wholefile.go` (`wholeFileClassify`, `renderWholeFile`)
- Test: `internal/config/managedfiles/wholefile_test.go` (add new cases)

- [ ] **Step 4.1: Add `HasFrontmatter` field to `ManagedFile`**

Edit `internal/config/managedfiles/types.go`. Find the `ManagedFile` struct definition (around line 70-90). Add `HasFrontmatter bool` after `SupersedesPath`:

```go
type ManagedFile struct {
	Path           string
	Strategy       StrategyKind
	Source         string
	Comment        CommentSyntax
	Harness        Harness
	SupersedesPath string

	// HasFrontmatter, when true, instructs the WholeFile strategy to position
	// the sentinel on the first body line *after* a leading YAML frontmatter
	// block (`---\n...\n---\n`) instead of on line 1. Required for Cursor's
	// .mdc rule format where the frontmatter must occupy line 1.
	//
	// Invariants (enforced by validateManifestEntry):
	//   - HasFrontmatter==true requires Strategy==StrategyWholeFile.
	//   - HasFrontmatter==true requires Comment != CommentNone.
	HasFrontmatter bool

	Build func(ProjectParams) ([]byte, error)
}
```

- [ ] **Step 4.2: Verify the addition compiles**

Run: `go build ./internal/config/managedfiles/`
Expected: success. (Field added with zero-value default; no callers break.)

- [ ] **Step 4.3: Write the failing tests for frontmatter-aware whole-file behavior**

Append to `internal/config/managedfiles/wholefile_test.go`. First add a fixture helper near the existing `testWholeFileMF` (around line 16):

```go
const testMdcPath = ".cursor/rules/test-rule.mdc"

func testMdcCanonical() []byte {
	return []byte("---\ndescription: test rule\nalwaysApply: false\n---\n\n# Test Rule\n\nBody content here.\n")
}

// testMdcMF returns a ManagedFile fixture that points at a test-only
// embedded path. Because the strategy reads from embed.FS via readSource,
// we override the source-read path for these tests via a helper
// (see Step 4.4).
func testMdcMF() ManagedFile {
	return ManagedFile{
		Path:           testMdcPath,
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/test-rule.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
	}
}
```

This points at a `Source` path that won't exist in `embedded/`. We need a way to inject canonical bytes for the test. The simplest approach: add a real fixture file under `internal/config/managedfiles/embedded/cursor/test-rule.mdc` for test consumption (production manifest does not reference it, so it's inert). Do that in the next step.

- [ ] **Step 4.4: Add the test fixture embedded mdc**

Create `internal/config/managedfiles/embedded/cursor/test-rule.mdc` with exactly these bytes (no leading whitespace, single trailing newline):

```text
---
description: test rule
alwaysApply: false
---

# Test Rule

Body content here.
```

Verify with: `cat -A internal/config/managedfiles/embedded/cursor/test-rule.mdc` (should show `$` at end of each line and no `^I` tabs).

Now add the test functions to `wholefile_test.go`:

```go
func TestWholeFileMdc_Missing_WritesSentinelAfterFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Fatalf("action = %v, want ActionCreated", res.Action)
	}
	full := filepath.Join(dir, testMdcPath)
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		t.Fatalf("read written file: %v", rerr)
	}
	got := string(data)
	// Expected layout: frontmatter, blank line (consumed by splitFrontmatter
	// into `front`), sentinel, body. So the bytes immediately after the
	// closing `---\n\n` must be the sentinel.
	if !strings.HasPrefix(got, "---\ndescription: test rule\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=") {
		t.Errorf("file does not start with frontmatter+sentinel:\n%s", got)
	}
	// Body must be present after the sentinel line.
	if !strings.Contains(got, "# Test Rule\n\nBody content here.\n") {
		t.Errorf("body not preserved:\n%s", got)
	}
}

func TestWholeFileMdc_Synced_NoOpOnSecondSync(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionNoOp {
		t.Errorf("second sync action = %v, want ActionNoOp", res.Action)
	}
}

func TestWholeFileMdc_DriftedOnEditedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	s := wholeFileStrategy{}
	params := ProjectParams{Slug: "test", ServerURL: "http://h"}
	if _, err := s.Sync(dir, mf, params, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// User changes alwaysApply.
	full := filepath.Join(dir, testMdcPath)
	data, _ := os.ReadFile(full)
	edited := strings.Replace(string(data), "alwaysApply: false", "alwaysApply: true", 1)
	if err := os.WriteFile(full, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := s.Sync(dir, mf, params, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("action on edited frontmatter = %v, want ActionSkipped", res.Action)
	}
}

func TestWholeFileMdc_DriftedWhenFrontmatterMissing(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	full := filepath.Join(dir, testMdcPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// File exists but has no frontmatter — user broke the shape.
	if err := os.WriteFile(full, []byte("just body content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionSkipped {
		t.Errorf("action on broken frontmatter = %v, want ActionSkipped", res.Action)
	}
	if !strings.Contains(res.Detail, "frontmatter") {
		t.Errorf("Detail should mention frontmatter; got %q", res.Detail)
	}
}

func TestWholeFileMdc_StaleRefreshes(t *testing.T) {
	dir := t.TempDir()
	mf := testMdcMF()
	full := filepath.Join(dir, testMdcPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed: frontmatter + sentinel-hash-matches-stale-body but stale body
	// doesn't match canonical → Stale.
	staleBody := []byte("# Stale Heading\n\nold body\n")
	staleHash := hashBytes(staleBody)
	staleContent := []byte("---\ndescription: test rule\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=" + staleHash + " -->\n# Stale Heading\n\nold body\n")
	if err := os.WriteFile(full, staleContent, 0o600); err != nil {
		t.Fatal(err)
	}
	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionRefreshed {
		t.Errorf("action = %v, want ActionRefreshed", res.Action)
	}
	// Verify the refreshed file has the canonical body.
	got, _ := os.ReadFile(full)
	if !strings.Contains(string(got), "# Test Rule\n\nBody content here.\n") {
		t.Errorf("refresh did not restore canonical body:\n%s", got)
	}
}
```

- [ ] **Step 4.5: Run the tests, confirm they fail (current strategy ignores `HasFrontmatter`)**

Run: `go test -run 'TestWholeFileMdc' -v ./internal/config/managedfiles/`
Expected: most fail with sentinel-on-line-1 or hash mismatches.

- [ ] **Step 4.6: Extend `wholeFileClassify` for `HasFrontmatter`**

Edit `internal/config/managedfiles/wholefile.go`. Replace the entire `wholeFileClassify` function with the frontmatter-aware version. The new version retains identical behavior when `mf.HasFrontmatter == false` and adds the new branch when `true`:

```go
//nolint:gocritic // ManagedFile is the framework's standard parameter shape
func wholeFileClassify(cwd string, mf ManagedFile) (FileState, []byte, []byte, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, nil, nil, err
	}
	full := filepath.Join(cwd, mf.Path)

	canonical, srcErr := readSource(mf)
	if srcErr != nil {
		return FileState{}, nil, nil, fmt.Errorf("read source for %s: %w", mf.Path, srcErr)
	}

	var canonicalHash string
	if mf.HasFrontmatter {
		// Canonical .mdc lives on disk with frontmatter and no sentinel.
		// Use the sibling hash function so canonical_hash on disk is
		// computed the same way as disk_hash on a synced file (sentinel
		// removed from body[0], everything else hashed).
		h, herr := HashExcludingSentinelAfterFrontmatter(mf.Comment, canonical)
		if herr != nil {
			return FileState{}, nil, nil, fmt.Errorf("hash canonical %s: %w", mf.Path, herr)
		}
		canonicalHash = h
	} else {
		canonicalHash = hashBytes(canonical)
	}

	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, EmbeddedHash: canonicalHash}, canonical, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	if mf.HasFrontmatter {
		return classifyMdcWholeFile(mf, existing, canonical, canonicalHash)
	}
	return classifyNonFrontmatterWholeFile(mf, existing, canonical, canonicalHash)
}

// classifyNonFrontmatterWholeFile mirrors the pre-HasFrontmatter behavior:
// sentinel on line 1, hash via HashExcludingSentinel.
//
//nolint:gocritic // see wholeFileClassify
func classifyNonFrontmatterWholeFile(mf ManagedFile, existing, canonical []byte, canonicalHash string) (FileState, []byte, []byte, error) {
	firstLine, _, _ := bytes.Cut(existing, []byte("\n"))
	sentinel, perr := ParseSentinel(mf.Comment, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, fmt.Errorf("parse sentinel for %s: %w", mf.Path, perr)
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	diskHash := HashExcludingSentinel(mf.Comment, existing)
	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}

// classifyMdcWholeFile is the frontmatter-aware variant.
//
//nolint:gocritic // see wholeFileClassify
func classifyMdcWholeFile(mf ManagedFile, existing, canonical []byte, canonicalHash string) (FileState, []byte, []byte, error) {
	_, body, fmErr := splitFrontmatter(existing)
	if fmErr != nil {
		// User broke the frontmatter — refuse to mutate without --force.
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "frontmatter missing or unclosed", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	firstLine, _, _ := bytes.Cut(body, []byte("\n"))
	sentinel, perr := ParseSentinel(mf.Comment, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, fmt.Errorf("parse sentinel for %s: %w", mf.Path, perr)
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	diskHash, hashErr := HashExcludingSentinelAfterFrontmatter(mf.Comment, existing)
	if hashErr != nil {
		return FileState{}, nil, nil, fmt.Errorf("hash %s: %w", mf.Path, hashErr)
	}
	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}
```

- [ ] **Step 4.7: Update `renderWholeFile` to honor `HasFrontmatter`**

In the same file, replace the existing `renderWholeFile` function (currently at the bottom of `wholefile.go`) with:

```go
// renderWholeFile emits canonical content prefixed by a v=2 sentinel line.
// For HasFrontmatter==false (default), the sentinel sits on line 1 of the
// file. For HasFrontmatter==true, the sentinel sits on the first body line
// after the leading YAML frontmatter — Cursor's .mdc parser requires
// `---` on line 1, so the sentinel can't sit above frontmatter.
//
// The hash is computed over the canonical content using the same hashing
// rules as classify: HashExcludingSentinel for line-1 sentinels,
// HashExcludingSentinelAfterFrontmatter when HasFrontmatter==true. On
// re-inspect, disk-hash equals sentinel-hash.
func renderWholeFile(mf ManagedFile, canonical []byte) []byte {
	if mf.HasFrontmatter {
		return renderWholeFileWithFrontmatter(mf, canonical)
	}
	hash := hashBytes(canonical)
	sentinel := RenderSentinel(mf.Comment, Sentinel{Version: 2, SHA256: hash})
	var b bytes.Buffer
	b.WriteString(sentinel)
	b.WriteString("\n")
	b.Write(canonical)
	if len(canonical) == 0 || canonical[len(canonical)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}

func renderWholeFileWithFrontmatter(mf ManagedFile, canonical []byte) []byte {
	front, body, err := splitFrontmatter(canonical)
	if err != nil {
		// Canonical .mdc must have well-formed frontmatter; this is a
		// build-time invariant verified by TestEmbeddedMdcCanonicalSplitsCleanly.
		// If we reach here at runtime, the embedded source is malformed —
		// surface it as a panic rather than write an invalid file.
		panic(fmt.Sprintf("canonical %s has malformed frontmatter: %v", mf.Path, err))
	}
	// Hash inputs: front + body (no sentinel). HashExcludingSentinelAfter-
	// Frontmatter on the rendered output drops the inserted sentinel and
	// returns the same hash, so disk-hash agrees with sentinel-hash.
	hashInput := append(append([]byte{}, front...), body...)
	hash := hashBytes(hashInput)
	sentinel := RenderSentinel(mf.Comment, Sentinel{Version: 2, SHA256: hash})

	var b bytes.Buffer
	b.Write(front)
	b.WriteString(sentinel)
	b.WriteString("\n")
	b.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}
```

Update the callers of `renderWholeFile` (lines 55, 58, 74, 76 of the original `wholefile.go`) to pass `mf` instead of `mf.Comment`. The new signature is `renderWholeFile(mf, canonical)`. Search-and-replace:

```text
renderWholeFile(mf.Comment, canonical)  →  renderWholeFile(mf, canonical)
renderWholeFile(mf.Comment, body)        →  renderWholeFile(mf, body)
```

- [ ] **Step 4.8: Run the new tests, confirm they pass**

Run: `go test -run 'TestWholeFileMdc' -v ./internal/config/managedfiles/`
Expected: all five PASS.

- [ ] **Step 4.9: Run the whole package, confirm nothing else regressed**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS. The existing `TestWholeFileMissing`, `TestWholeFileSynced`, `TestWholeFileStale`, `TestWholeFileDrifted` tests use `HasFrontmatter==false` and should still pass unchanged.

A regression in `TestEmbeddedSourcesHaveLicenseHeader` or similar may surface because we added `embedded/cursor/test-rule.mdc` without a license header. License headers apply to `.go`, `.sh`, `.py`, `.proto` per CLAUDE.md — `.mdc` is exempt. Verify by inspecting any failing test output; if a test asserts a header on `.mdc`, that's an existing test bug worth a side note in the bead, not blocking PR D.

- [ ] **Step 4.10: Commit**

```bash
jj describe -m "feat(managedfiles): frontmatter-aware WholeFile strategy

Adds HasFrontmatter bool to ManagedFile. When true, the WholeFile strategy
positions the sentinel after a leading YAML frontmatter block instead of on
line 1, so Cursor's .mdc rule format (which requires --- on line 1) can be
managed end-to-end. classify and render both branch on the flag; existing
HasFrontmatter==false entries are byte-for-byte unchanged.

Includes a test-only embedded mdc fixture (embedded/cursor/test-rule.mdc)
exercised by five new wholefile tests covering Missing/Synced/Stale/
Drifted-on-frontmatter-edit/Drifted-on-frontmatter-missing.

Refs: spgr-rwrp PR D §2 framework change

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 5: Add canonical `.mdc` files + vestigial pre-rename bytes + prior-hash helper

**Why now:** Task 6 (manifest entries) references both the canonical Source paths and the vestigial-bytes helper.

**Files:**

- Move: `plugin/cursor/.cursor/rules/specgraph.md` → `internal/config/managedfiles/embedded/cursor/specgraph.mdc` (rename + relocate)
- Move: `plugin/cursor/.cursor/rules/post-stage.md` → `internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc` (rename + relocate)
- Copy (byte-exact, pre-move): pre-rename `.md` content into `internal/config/managedfiles/embedded/cursor/vestigial/{specgraph.md,post-stage.md}`
- Copy (byte-exact, pre-move): same content into `internal/config/managedfiles/testdata/cursor-vestigial/{specgraph.md,post-stage.md}` (for Task 9 integration tests; the integration test file lives in an external `_test` package and can't reach the embedded vars)
- Create: `internal/config/managedfiles/vestigial_cursor_rules.go` (embeds + helper)
- Test: `internal/config/managedfiles/vestigial_cursor_rules_test.go` (hash-pinning + embedded-equals-fixture)

- [ ] **Step 5.1: Capture pre-rename canonical bytes into the four destination files (BEFORE rename)**

This step preserves the byte-exact pre-rename content. Do all four copies before doing the rename in Step 5.2 so the source bytes are still available.

```bash
cd /Users/SeBrandt/Code/github.com/specgraph

mkdir -p internal/config/managedfiles/embedded/cursor/vestigial
mkdir -p internal/config/managedfiles/testdata/cursor-vestigial

cp plugin/cursor/.cursor/rules/specgraph.md \
   internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md
cp plugin/cursor/.cursor/rules/post-stage.md \
   internal/config/managedfiles/embedded/cursor/vestigial/post-stage.md

cp plugin/cursor/.cursor/rules/specgraph.md \
   internal/config/managedfiles/testdata/cursor-vestigial/specgraph.md
cp plugin/cursor/.cursor/rules/post-stage.md \
   internal/config/managedfiles/testdata/cursor-vestigial/post-stage.md
```

Verify the four files are byte-identical to the source:

```bash
sha256sum plugin/cursor/.cursor/rules/specgraph.md \
          internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md \
          internal/config/managedfiles/testdata/cursor-vestigial/specgraph.md
```

All three hashes MUST match. Same check for `post-stage.md`.

- [ ] **Step 5.2: Move + rename the authoring sources to the embedded canonical paths**

```bash
mv plugin/cursor/.cursor/rules/specgraph.md \
   internal/config/managedfiles/embedded/cursor/specgraph.mdc
mv plugin/cursor/.cursor/rules/post-stage.md \
   internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc
```

The `.md` files are now gone from `plugin/cursor/.cursor/rules/`. The new `.mdc` files contain real frontmatter — verify:

```bash
head -5 internal/config/managedfiles/embedded/cursor/specgraph.mdc
head -5 internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc
```

Both should show frontmatter starting with `---`.

- [ ] **Step 5.3: Compute the expected hex hashes for hash-pinning**

The hash-pinning test in Step 5.5 needs literal hex constants. Compute them now:

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
go run ./cmd/hash-tool 2>/dev/null || cat <<'EOF' > /tmp/hash_pin.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	for _, path := range os.Args[1:] {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sum := sha256.Sum256(data)
		fmt.Printf("%s  %s\n", hex.EncodeToString(sum[:]), path)
	}
}
EOF
go run /tmp/hash_pin.go \
   internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md \
   internal/config/managedfiles/embedded/cursor/vestigial/post-stage.md
```

Record the two hex constants for Step 5.5. Format: `<hex>  <path>`. Note: `HashExcludingSentinel(CommentNone, b)` is `sha256(b)` per `hash.go:30-32`, so the simple sha256 above is the same value.

- [ ] **Step 5.4: Write the failing tests**

Create `internal/config/managedfiles/vestigial_cursor_rules_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Pinned hashes for the pre-PR-D canonical .md files. Updating either of
// these constants without a deliberate decision breaks SupersedesPath
// cleanup for any user whose .cursor/rules/specgraph.md or post-stage.md
// is a verbatim copy from the pre-PR-D repo. Do not update unless you've
// also confirmed:
//   (a) no current dogfood user has the old bytes on disk, AND
//   (b) the vestigial bytes are being intentionally re-pinned.
const (
	pinnedHashCursorSpecgraphMD = "REPLACE-ME-WITH-STEP-5.3-OUTPUT"
	pinnedHashCursorPostStageMD = "REPLACE-ME-WITH-STEP-5.3-OUTPUT"
)

func TestVestigialCursorRulePriorHashPinned(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{".cursor/rules/specgraph.md", pinnedHashCursorSpecgraphMD},
		{".cursor/rules/post-stage.md", pinnedHashCursorPostStageMD},
	}
	for _, tc := range cases {
		got := vestigialCursorRulePriorHash(tc.path)
		if got != tc.want {
			t.Errorf("vestigialCursorRulePriorHash(%q) = %s, want %s\n\nIf you intentionally changed pre-rename canonical bytes, update the pinnedHash constants in this file — but note this breaks SupersedesPath cleanup for any user with the old verbatim bytes on disk.",
				tc.path, got, tc.want)
		}
	}
}

func TestVestigialCursorRulePriorHash_UnknownPathPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on unknown path; got none")
		}
	}()
	_ = vestigialCursorRulePriorHash("does/not/exist.md")
}

func TestVestigialBytesMatchTestdataFixtures(t *testing.T) {
	// Cross-check the embedded vestigial bytes against the testdata
	// fixture copies used by integration_test.go. If these diverge,
	// integration tests will silently exercise the wrong content.
	cases := []struct {
		fixture string
		embed   []byte
	}{
		{"cursor-vestigial/specgraph.md", vestigialCursorSpecgraphMD},
		{"cursor-vestigial/post-stage.md", vestigialCursorPostStageMD},
	}
	for _, tc := range cases {
		path := filepath.Join("testdata", tc.fixture)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.Equal(got, tc.embed) {
			t.Errorf("%s diverges from embedded var", path)
		}
	}
}
```

- [ ] **Step 5.5: Replace the hex placeholders in the test with real hashes**

Open `internal/config/managedfiles/vestigial_cursor_rules_test.go` and replace `pinnedHashCursorSpecgraphMD` and `pinnedHashCursorPostStageMD` with the hex strings captured in Step 5.3.

- [ ] **Step 5.6: Run the tests, confirm they fail**

Run: `go test -run 'TestVestigial' -v ./internal/config/managedfiles/`
Expected: build failure, "undefined: vestigialCursorRulePriorHash" and "undefined: vestigialCursorSpecgraphMD".

- [ ] **Step 5.7: Create `vestigial_cursor_rules.go` with the embeds and helper**

Create `internal/config/managedfiles/vestigial_cursor_rules.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	_ "embed"
	"fmt"
)

// Vestigial pre-rename Cursor rule bytes. Preserved for hash-guarded
// supersedes cleanup of pre-PR-D `.md` files that users may have copied
// from the repo before PR D landed the embed-and-write managed flow.
//
// These bytes are byte-for-byte copies of what `plugin/cursor/.cursor/rules/`
// shipped before PR D's rename to `.mdc`. They are NOT on the production
// write path; they exist solely so `supersedesGuardedDelete` can recognize
// verbatim user copies and safely remove them while preserving any
// user-edited variants.
//
// Sunset trigger (mirrors renderV1CursorBlockBody): once `task plugin:check`
// reports zero pre-rename `.md` files in the dogfood repo for two
// consecutive releases, both vars and the helper below can be removed.

//go:embed embedded/cursor/vestigial/specgraph.md
var vestigialCursorSpecgraphMD []byte

//go:embed embedded/cursor/vestigial/post-stage.md
var vestigialCursorPostStageMD []byte

// vestigialCursorRulePriorHash returns the expected prior-canonical hash
// for a SupersedesPath value that points at one of the pre-rename Cursor
// rule files. Mirrors computePriorCanonical in markdownblock.go but reads
// from static embedded bytes (not a renderer + ProjectParams).
//
// Panics on unknown supersedesPath — every SupersedesPath in the manifest
// must have a corresponding case here.
func vestigialCursorRulePriorHash(supersedesPath string) string {
	switch supersedesPath {
	case ".cursor/rules/specgraph.md":
		return HashExcludingSentinel(CommentNone, vestigialCursorSpecgraphMD)
	case ".cursor/rules/post-stage.md":
		return HashExcludingSentinel(CommentNone, vestigialCursorPostStageMD)
	default:
		panic(fmt.Sprintf("no vestigial prior-canonical bytes for SupersedesPath %q", supersedesPath))
	}
}
```

- [ ] **Step 5.8: Run the tests, confirm they pass**

Run: `go test -run 'TestVestigial' -v ./internal/config/managedfiles/`
Expected: all three PASS. If the hash-pinning test fails, the hex constants in Step 5.5 are wrong — recompute via Step 5.3 and update.

- [ ] **Step 5.9: Run the whole package, confirm nothing else regressed**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 5.10: Clean up the throw-away `/tmp/hash_pin.go`**

```bash
rm -f /tmp/hash_pin.go
```

- [ ] **Step 5.11: Commit**

```bash
jj describe -m "feat(managedfiles): embed pre-rename cursor-rule bytes for supersedes cleanup

Adds vestigialCursorSpecgraphMD and vestigialCursorPostStageMD (byte-exact
copies of pre-PR-D plugin/cursor/.cursor/rules/{specgraph,post-stage}.md)
and a vestigialCursorRulePriorHash helper that maps SupersedesPath values
to the expected prior-canonical hash. Mirrors computePriorCanonical's role
in markdownblock.go for the new WholeFile supersedes path.

Also relocates the canonical sources to internal/config/managedfiles/
embedded/cursor/{specgraph,specgraph-post-stage}.mdc — the production
managed files will reference these paths via the manifest entries added
in the next commit.

Hash-pinning test locks the pre-rename bytes; updating them breaks
SupersedesPath cleanup and fails CI loudly.

Refs: spgr-rwrp PR D §SupersedesPath

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 6: Add SupersedesPath integration to `wholeFileStrategy.Sync` + the two new manifest entries

**Why now:** the two new entries depend on the strategy honoring SupersedesPath and on the vestigial helper from Task 5. Everything else is in place.

**Files:**

- Modify: `internal/config/managedfiles/wholefile.go` (`Sync`: add supersedes block)
- Modify: `internal/config/managedfiles/manifest.go` (add two entries)
- Test: `internal/config/managedfiles/wholefile_test.go` (supersedes scenarios)
- Test: `internal/config/managedfiles/manifest_test.go` (entry-count and shape — full update happens in Task 7)

- [ ] **Step 6.1: Write the failing supersedes tests**

Append to `internal/config/managedfiles/wholefile_test.go`:

```go
func TestWholeFileMdcSupersedes_DeletesVerbatim(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph.mdc",
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/specgraph.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
		SupersedesPath: ".cursor/rules/specgraph.md",
	}
	// Seed the old path with verbatim pre-rename bytes.
	oldFull := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if err := os.MkdirAll(filepath.Dir(oldFull), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFull, vestigialCursorSpecgraphMD, 0o600); err != nil {
		t.Fatal(err)
	}

	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	// .mdc exists.
	if _, err := os.Stat(filepath.Join(dir, ".cursor/rules/specgraph.mdc")); err != nil {
		t.Errorf("new .mdc missing: %v", err)
	}
	// .md was deleted.
	if _, err := os.Stat(oldFull); !os.IsNotExist(err) {
		t.Errorf(".md still present (stat err = %v)", err)
	}
}

func TestWholeFileMdcSupersedes_PreservesEditedAndAddsDetail(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:           ".cursor/rules/specgraph.mdc",
		Strategy:       StrategyWholeFile,
		Source:         "embedded/cursor/specgraph.mdc",
		Comment:        CommentHTML,
		Harness:        HarnessCursor,
		HasFrontmatter: true,
		SupersedesPath: ".cursor/rules/specgraph.md",
	}
	oldFull := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if err := os.MkdirAll(filepath.Dir(oldFull), 0o750); err != nil {
		t.Fatal(err)
	}
	// Seed an edited variant — append a comment.
	edited := append([]byte{}, vestigialCursorSpecgraphMD...)
	edited = append(edited, []byte("\n<!-- user note -->\n")...)
	if err := os.WriteFile(oldFull, edited, 0o600); err != nil {
		t.Fatal(err)
	}

	s := wholeFileStrategy{}
	res, err := s.Sync(dir, mf, ProjectParams{Slug: "test", ServerURL: "http://h"}, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != ActionCreated {
		t.Errorf("action = %v, want ActionCreated", res.Action)
	}
	// .md is preserved.
	if _, err := os.Stat(oldFull); err != nil {
		t.Errorf(".md should be preserved on edited variant: %v", err)
	}
	if !strings.Contains(res.Detail, `supersedes path ".cursor/rules/specgraph.md" left in place: prior-canonical mismatch`) {
		t.Errorf("Detail should mention prior-canonical mismatch; got %q", res.Detail)
	}
}
```

- [ ] **Step 6.2: Run the tests, confirm they fail (Sync does not yet honor SupersedesPath for WholeFile)**

Run: `go test -run 'TestWholeFileMdcSupersedes' -v ./internal/config/managedfiles/`
Expected: both fail — the `.md` is not deleted (no supersedes call site) and the Detail is empty.

- [ ] **Step 6.3: Add the supersedes block to `wholeFileStrategy.Sync`**

Edit `internal/config/managedfiles/wholefile.go`. Find the `Sync` method (around lines 22-79). It currently returns a `SyncResult` directly from the state switch. Refactor the return to a single `res` variable, then add the supersedes block after the switch and before the return.

Replace the entire `Sync` method body with:

```go
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (wholeFileStrategy) Sync(cwd string, mf ManagedFile, _ ProjectParams, opts SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	full := filepath.Join(cwd, mf.Path)

	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: lerr}, nil
	}
	defer func() {
		if uerr := unlock(); uerr != nil {
			slog.Error("unlock failed", "path", full, "error", uerr)
		}
	}()

	state, canonical, existing, cerr := wholeFileClassify(cwd, mf)
	if cerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: cerr}, nil
	}

	var res SyncResult
	switch state.State {
	case StateSynced:
		res = SyncResult{Path: mf.Path, Action: ActionNoOp}
	case StateMissing:
		res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionCreated, mf.Path)
	case StateStale:
		res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionRefreshed, mf.Path)
	case StateDrifted:
		switch {
		case !opts.Force:
			res = SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}
		case opts.KeepEdits:
			// Strip the first line ONLY when it's an actual sentinel.
			// state.Detail == "no sentinel" or "frontmatter missing or
			// unclosed" means the first line is user content; do not strip.
			body := existing
			if state.Detail != "no sentinel" && state.Detail != "frontmatter missing or unclosed" {
				body = stripFirstLine(existing)
			}
			res = wholeFileWrite(full, renderWholeFile(mf, body), ActionForced, mf.Path)
		default:
			res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionForced, mf.Path)
		}
	default:
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("unhandled state %v", state.State)}, nil
	}

	// Supersedes-guarded delete of any pre-rename path. Runs on the same
	// action set as markdownblock.go:163-176 (Created, Refreshed, Forced,
	// NoOp) so a user who's already at Synced still gets cleanup if they
	// happen to also have the old .md sitting around.
	if mf.SupersedesPath != "" && (res.Action == ActionCreated || res.Action == ActionRefreshed || res.Action == ActionForced || res.Action == ActionNoOp) {
		priorHash := vestigialCursorRulePriorHash(mf.SupersedesPath)
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

	return res, nil
}
```

- [ ] **Step 6.4: Run the supersedes tests, confirm they pass**

Run: `go test -run 'TestWholeFileMdcSupersedes' -v ./internal/config/managedfiles/`
Expected: both PASS.

- [ ] **Step 6.5: Add the two manifest entries**

Edit `internal/config/managedfiles/manifest.go`. Add to the slice returned by `allManagedFiles()` (after the OpenCode entry, around line 71):

```go
		{
			Path:           ".cursor/rules/specgraph.mdc",
			Strategy:       StrategyWholeFile,
			Source:         "embedded/cursor/specgraph.mdc",
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			HasFrontmatter: true,
			SupersedesPath: ".cursor/rules/specgraph.md",
		},
		{
			Path:           ".cursor/rules/specgraph-post-stage.mdc",
			Strategy:       StrategyWholeFile,
			Source:         "embedded/cursor/specgraph-post-stage.mdc",
			Comment:        CommentHTML,
			Harness:        HarnessCursor,
			HasFrontmatter: true,
			SupersedesPath: ".cursor/rules/post-stage.md",
		},
```

- [ ] **Step 6.6: Run the full package, expect entry-count assertions to fail**

Run: `go test ./internal/config/managedfiles/`
Expected: `TestManifestShape` (manifest_test.go:13) and `TestManifest_AllHarnesses` (integration_test.go:18) FAIL with "expected 6 entries, got 8". Other tests should pass. The entry-count bumps land in Task 7.

If any OTHER test fails (e.g., the canonical-mdc file failing `readSource` because `embedded/cursor/specgraph.mdc` wasn't found by `//go:embed`), debug that — the embed pattern likely needs to be checked in source.go or wherever the embed.FS is declared.

- [ ] **Step 6.7: Commit**

```bash
jj describe -m "feat(managedfiles): manage .cursor/rules/specgraph{,-post-stage}.mdc

Adds two WholeFile manifest entries for the Cursor rule files, plus the
SupersedesPath integration in wholeFileStrategy.Sync that hash-guards the
deletion of pre-rename .md user copies. Detail string format matches
markdownblock.go's so doctor (PR G) can find both flavors of orphan with
one regex.

Two tests (TestWholeFileMdcSupersedes_DeletesVerbatim and
TestWholeFileMdcSupersedes_PreservesEditedAndAddsDetail) cover the
verbatim-delete and edited-preserve paths against the vestigial bytes.

Manifest entry-count assertions still fail (6→8 not yet bumped) — fixed
in the next commit.

Refs: spgr-rwrp PR D §3 manifest entries, §SupersedesPath, §Where supersedes is invoked

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 7: Manifest shape tests, invariant tests, back-compat anchor

**Why now:** the manifest is at its final 8-entry shape from Task 6. Bump the counts and lock the invariants.

**Files:**

- Modify: `internal/config/managedfiles/manifest_test.go` (count bump + path map + new invariant cases + back-compat anchor)
- Modify: `internal/config/managedfiles/integration_test.go:18` (count bump)
- Test: same files

- [ ] **Step 7.1: Update `TestManifestShape` to expect 8 entries**

Edit `internal/config/managedfiles/manifest_test.go`. Update the count assertion (line 13) and add the two new paths to the `paths` map (around lines 16-23):

```go
func TestManifestShape(t *testing.T) {
	all := allManagedFiles()
	if len(all) != 8 {
		t.Errorf("expected 8 entries, got %d", len(all))
	}
	paths := map[string]bool{
		".mcp.json":                                    false,
		".cursor/mcp.json":                             false,
		"opencode.json":                                false,
		"AGENTS.md":                                    false,
		".cursor/rules/specgraph-bootstrap.mdc":        false,
		".specgraph/agents/opencode/specgraph.ts":      false,
		".cursor/rules/specgraph.mdc":                  false,
		".cursor/rules/specgraph-post-stage.mdc":       false,
	}
	// ... (rest of function unchanged)
```

- [ ] **Step 7.2: Update `TestManifest_AllHarnesses` to expect 8 entries**

Edit `internal/config/managedfiles/integration_test.go:18-20`. Change the count check from 6 to 8 and update the error message.

```go
	if len(all) != 8 {
		t.Errorf("Manifest() should have 8 entries for all harnesses, got %d entries", len(all))
	}
```

- [ ] **Step 7.3: Add `HasFrontmatter` invariant tests**

Append two cases to the `TestValidateManifestEntry` table in `manifest_test.go` (from Task 1):

```go
		{
			name: "HasFrontmatter on non-WholeFile",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyMarkdownBlock,
				Build:          func(ProjectParams) ([]byte, error) { return nil, nil },
				Comment:        CommentHTML,
				HasFrontmatter: true,
			},
			wantErr: "HasFrontmatter requires WholeFile",
		},
		{
			name: "HasFrontmatter with CommentNone",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source:         "s",
				Comment:        CommentNone,
				HasFrontmatter: true,
			},
			wantErr: "HasFrontmatter requires non-empty comment syntax",
		},
		{
			name: "valid HasFrontmatter entry",
			mf: ManagedFile{
				Path: "x", Strategy: StrategyWholeFile,
				Source:         "s",
				Comment:        CommentHTML,
				HasFrontmatter: true,
			},
		},
```

- [ ] **Step 7.4: Add the back-compat anchor test**

Append to `manifest_test.go`:

```go
// TestNoLegacyWholeFileHTMLSentinels pins the back-compat reasoning
// for PR D's RenderSentinel CommentHTML change. Before PR D, no shipped
// manifest entry combined Strategy==StrategyWholeFile with Comment==
// CommentHTML, so no file on disk anywhere carries the old `:start`-suffixed
// CommentHTML whole-file sentinel form. PR D introduces the combination
// only with HasFrontmatter==true. This test ensures a future entry can't
// silently introduce a WholeFile+HTML+!HasFrontmatter combination, which
// would suddenly produce the bare-`init` form by surprise.
func TestNoLegacyWholeFileHTMLSentinels(t *testing.T) {
	for _, mf := range allManagedFiles() {
		if mf.Strategy == StrategyWholeFile && mf.Comment == CommentHTML && !mf.HasFrontmatter {
			t.Errorf("entry %q: WholeFile+CommentHTML without HasFrontmatter is unsupported (see PR D back-compat anchor)", mf.Path)
		}
	}
}
```

- [ ] **Step 7.5: Add `TestEmbeddedMdcCanonicalSplitsCleanly`**

Append to `manifest_test.go`:

```go
// TestEmbeddedMdcCanonicalSplitsCleanly verifies that every embedded
// canonical for a HasFrontmatter==true entry has well-formed YAML
// frontmatter — splitFrontmatter must succeed and the post-frontmatter
// body must be non-empty. Locks the assumption that renderWholeFile
// never panics on canonical input at runtime.
func TestEmbeddedMdcCanonicalSplitsCleanly(t *testing.T) {
	for _, mf := range allManagedFiles() {
		if !mf.HasFrontmatter {
			continue
		}
		canonical, err := readSource(mf)
		if err != nil {
			t.Errorf("%s: readSource: %v", mf.Path, err)
			continue
		}
		front, body, ferr := splitFrontmatter(canonical)
		if ferr != nil {
			t.Errorf("%s: splitFrontmatter: %v", mf.Path, ferr)
			continue
		}
		if len(front) == 0 {
			t.Errorf("%s: empty frontmatter", mf.Path)
		}
		if len(body) == 0 {
			t.Errorf("%s: empty body after frontmatter", mf.Path)
		}
	}
}
```

- [ ] **Step 7.6: Run the tests, confirm `TestValidateManifestEntry` cases fail (validator doesn't yet enforce HasFrontmatter rules)**

Run: `go test -run 'TestValidateManifestEntry' -v ./internal/config/managedfiles/`
Expected: the two new `HasFrontmatter` cases FAIL. Others pass.

- [ ] **Step 7.7: Add HasFrontmatter invariants to `validateManifestEntry`**

Edit `internal/config/managedfiles/manifest.go`. In `validateManifestEntry` (added in Task 1), insert the HasFrontmatter rules at the end of the function (before `return nil`):

```go
	if mf.HasFrontmatter {
		if mf.Strategy != StrategyWholeFile {
			return fmt.Errorf("manifest entry %q: HasFrontmatter requires WholeFile strategy, got %v", mf.Path, mf.Strategy)
		}
		if mf.Comment == CommentNone {
			return fmt.Errorf("manifest entry %q: HasFrontmatter requires non-empty comment syntax", mf.Path)
		}
	}
	return nil
}
```

- [ ] **Step 7.8: Run all the new tests, confirm they pass**

Run: `go test -run 'TestManifestShape|TestManifest_AllHarnesses|TestValidateManifestEntry|TestNoLegacyWholeFileHTMLSentinels|TestEmbeddedMdcCanonicalSplitsCleanly' -v ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 7.9: Run the whole package, confirm nothing regressed**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 7.10: Commit**

```bash
jj describe -m "test(managedfiles): bump manifest count to 8, add HasFrontmatter invariants

- TestManifestShape and TestManifest_AllHarnesses bumped 6 → 8.
- validateManifestEntry enforces two new HasFrontmatter invariants
  (requires WholeFile, requires non-CommentNone).
- TestNoLegacyWholeFileHTMLSentinels pins the back-compat reasoning
  for the RenderSentinel CommentHTML change.
- TestEmbeddedMdcCanonicalSplitsCleanly locks the invariant that every
  HasFrontmatter canonical has well-formed frontmatter, so renderWholeFile
  never panics at runtime.

Refs: spgr-rwrp PR D §Tests

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 8: Reverse-symlinks under `plugin/cursor/` + symlink-resolve test

**Why now:** dev convenience — developers editing under `plugin/cursor/.cursor/rules/` should land their changes in `embedded/cursor/`. Symmetric to PR C's OpenCode setup.

**Files:**

- Create symlinks: `plugin/cursor/.cursor/rules/specgraph.mdc`, `plugin/cursor/.cursor/rules/specgraph-post-stage.mdc`
- Test: `internal/config/managedfiles/symlink_pluginshim_test.go` (new file)

- [ ] **Step 8.1: Inspect PR C's existing reverse-symlink to confirm the correct relative path**

Run: `cd /Users/SeBrandt/Code/github.com/specgraph && readlink plugin/opencode/.opencode/plugins/specgraph.ts`
Expected output: something like `../../../../internal/config/managedfiles/embedded/opencode/specgraph.ts`.

Now compute the right depth for `plugin/cursor/.cursor/rules/<file>`:

`plugin/cursor/.cursor/rules/` → up four levels → repo root. So the relative target is `../../../../internal/config/managedfiles/embedded/cursor/<file>`.

- [ ] **Step 8.2: Create the two reverse-symlinks**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph/plugin/cursor/.cursor/rules
ln -s ../../../../internal/config/managedfiles/embedded/cursor/specgraph.mdc specgraph.mdc
ln -s ../../../../internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc specgraph-post-stage.mdc
cd /Users/SeBrandt/Code/github.com/specgraph
```

Verify resolution:

```bash
readlink plugin/cursor/.cursor/rules/specgraph.mdc
readlink plugin/cursor/.cursor/rules/specgraph-post-stage.mdc
ls -lL plugin/cursor/.cursor/rules/specgraph.mdc plugin/cursor/.cursor/rules/specgraph-post-stage.mdc
```

Both `ls -lL` should resolve and show file sizes, not "No such file or directory".

- [ ] **Step 8.3: Write the failing test**

Create `internal/config/managedfiles/symlink_pluginshim_test.go`:

```go
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
// is computed from runtime.Caller — see findRepoRoot below.
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

// findRepoRootForTest walks up from the test file's directory looking for
// go.mod. Returns the repo root or fails the test.
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
```

- [ ] **Step 8.4: Run the test, confirm it passes**

Run: `go test -run 'TestPluginCursorSymlinksResolve' -v ./internal/config/managedfiles/`
Expected: PASS. If `readDir` finds zero `.mdc` entries, the symlinks weren't created — re-run Step 8.2.

- [ ] **Step 8.5: Run the whole package**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 8.6: Commit**

```bash
jj describe -m "feat(plugin/cursor): reverse-symlink rule files to embedded canonical

plugin/cursor/.cursor/rules/{specgraph,specgraph-post-stage}.mdc are now
symlinks to internal/config/managedfiles/embedded/cursor/{specgraph,
specgraph-post-stage}.mdc. Mirrors the PR C pattern for OpenCode: a single
authoring source (under embedded/) plus reverse-symlinks under plugin/
for developer convenience. //go:embed reads from embedded/ at compile
time — the symlinks are not in the build path.

A new TestPluginCursorSymlinksResolve unit test fails CI on a dangling
symlink.

Refs: spgr-rwrp PR D §Filesystem moves, §Risks #3

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 9: Integration tests — end-to-end Sync scenarios

**Why now:** the unit tests cover individual transitions; the integration tests verify the cross-strategy + cross-entry interactions through `SyncAll`.

**Files:**

- Modify: `internal/config/managedfiles/integration_test.go` (add three scenarios)

- [ ] **Step 9.1: Write the failing tests**

Append to `internal/config/managedfiles/integration_test.go`:

```go
func TestSyncAll_CursorMdcVerbatimSupersedes(t *testing.T) {
	dir := t.TempDir()
	// Seed both old .md files with verbatim pre-rename bytes (from testdata).
	for _, name := range []string{"specgraph.md", "post-stage.md"} {
		src := filepath.Join("testdata", "cursor-vestigial", name)
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		target := filepath.Join(dir, ".cursor/rules", name)
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
			t.Fatal(mkErr)
		}
		if wErr := os.WriteFile(target, body, 0o600); wErr != nil {
			t.Fatal(wErr)
		}
	}

	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// New .mdc files exist; old .md files are gone.
	for _, p := range []string{".cursor/rules/specgraph.mdc", ".cursor/rules/specgraph-post-stage.mdc"} {
		if _, sErr := os.Stat(filepath.Join(dir, p)); sErr != nil {
			t.Errorf("expected %s to exist: %v", p, sErr)
		}
	}
	for _, p := range []string{".cursor/rules/specgraph.md", ".cursor/rules/post-stage.md"} {
		if _, sErr := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(sErr) {
			t.Errorf("%s should be deleted (stat err = %v)", p, sErr)
		}
	}

	// No result reports an error.
	for _, r := range results {
		if r.Action == managedfiles.ActionError {
			t.Errorf("result for %s reported error: %v", r.Path, r.Err)
		}
	}
}

func TestSyncAll_CursorMdcEditedMdPreserved(t *testing.T) {
	dir := t.TempDir()
	// Seed .cursor/rules/specgraph.md with edited content.
	src := filepath.Join("testdata", "cursor-vestigial", "specgraph.md")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	edited := append([]byte{}, body...)
	edited = append(edited, []byte("\n<!-- user note -->\n")...)
	target := filepath.Join(dir, ".cursor/rules/specgraph.md")
	if mkErr := os.MkdirAll(filepath.Dir(target), 0o750); mkErr != nil {
		t.Fatal(mkErr)
	}
	if wErr := os.WriteFile(target, edited, 0o600); wErr != nil {
		t.Fatal(wErr)
	}

	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Old .md preserved.
	if _, sErr := os.Stat(target); sErr != nil {
		t.Errorf("edited .md should be preserved: %v", sErr)
	}

	// Find the .mdc result and verify its Detail mentions the mismatch.
	found := false
	for _, r := range results {
		if r.Path == ".cursor/rules/specgraph.mdc" {
			found = true
			if !strings.Contains(r.Detail, `supersedes path ".cursor/rules/specgraph.md" left in place: prior-canonical mismatch`) {
				t.Errorf("Detail = %q, expected mismatch message", r.Detail)
			}
		}
	}
	if !found {
		t.Errorf("no SyncResult for .cursor/rules/specgraph.mdc")
	}
}

func TestSyncAll_CursorMdcIdempotent(t *testing.T) {
	dir := t.TempDir()
	params := managedfiles.ProjectParams{Slug: "test", ServerURL: "http://h"}

	if _, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	// Second run: every cursor-managed file reports ActionNoOp.
	results, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessCursor}, params, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action != managedfiles.ActionNoOp {
			t.Errorf("%s: action %v on second sync, want ActionNoOp", r.Path, r.Action)
		}
	}
}
```

Add `strings` to the imports if not already there.

- [ ] **Step 9.2: Run the tests, confirm they pass**

Run: `go test -run 'TestSyncAll_CursorMdc' -v ./internal/config/managedfiles/`
Expected: all three PASS. Failures here indicate a wiring issue in Task 6's supersedes block or Task 5's vestigial mapping.

- [ ] **Step 9.3: Run the whole package**

Run: `go test ./internal/config/managedfiles/`
Expected: all PASS.

- [ ] **Step 9.4: Commit**

```bash
jj describe -m "test(managedfiles): integration scenarios for cursor mdc supersedes

Three SyncAll-driven scenarios covering the verbatim-supersedes,
edited-preserve, and idempotency contracts for .cursor/rules/specgraph.mdc
and specgraph-post-stage.mdc. Seeds the .md from the testdata fixture
(cross-validated against the embedded vestigial bytes in
TestVestigialBytesMatchTestdataFixtures).

Refs: spgr-rwrp PR D §Tests / Integration

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 10: Documentation + SMOKE_TEST.md + final `task check`

**Files:**

- Modify: `plugin/cursor/README.md` (update table rows, note init-managed status)
- Modify: `plugin/specgraph/README.md:58` (path update for Cursor row)
- Create: `plugin/cursor/SMOKE_TEST.md`
- Verify: `task check` passes end-to-end

- [ ] **Step 10.1: Update `plugin/cursor/README.md`**

Open `plugin/cursor/README.md`. Find the file-name table (the existing entries name `.cursor/rules/specgraph.md` and `.cursor/rules/post-stage.md`). Update to the new names AND add a note. Replace the relevant table rows with:

```markdown
| File | Purpose |
|---|---|
| `.cursor/rules/specgraph.mdc` | One-screen routing rule. Written by `specgraph init`; do not edit by hand. |
| `.cursor/rules/specgraph-post-stage.mdc` | Post-stage analytical-pass guidance. Written by `specgraph init`; do not edit by hand. |
| `.cursor/rules/specgraph-bootstrap.mdc` | Project pointer block. Written by `specgraph init` (markdown-block strategy — user owns the rest of the file). |
```

If the README has prose around the table that references the old `.md` names, update it too. Use `grep -n "specgraph\.md\|post-stage\.md" plugin/cursor/README.md` to find references.

- [ ] **Step 10.2: Update `plugin/specgraph/README.md:58`**

Open `plugin/specgraph/README.md`. Find line 58 (the Cursor row, referencing `plugin/cursor/.cursor/rules/post-stage.md`). Update the path:

```markdown
| Cursor | A `.cursor/rules/specgraph-post-stage.mdc` rule that the model consults after stage edits | `plugin/cursor/.cursor/rules/specgraph-post-stage.mdc` |
```

- [ ] **Step 10.3: Create `plugin/cursor/SMOKE_TEST.md`**

Create `plugin/cursor/SMOKE_TEST.md` (mirror PR C's structure, adapted for Cursor):

````markdown
# Cursor plugin smoke test

Manual end-to-end procedure for verifying `.cursor/rules/specgraph.mdc` and
`.cursor/rules/specgraph-post-stage.mdc` against a running Cursor session.
Captures the contract that has no automated test coverage today.

## Prereqs

- Cursor installed (any reasonably recent version with `.cursor/rules/` support)
- `specgraph` binary on `PATH` (`task build && ln -sf $(pwd)/specgraph ~/.local/bin/`)
- `specgraph` server running (`specgraph serve &`) and reachable
- `SPECGRAPH_API_KEY` set in environment
- A fresh project directory (NOT this repo, to avoid dogfood collisions)

## Setup

In a fresh project dir:

```bash
specgraph init --slug smoke-test --server-url http://localhost:9090
```

This writes (among others):

- `.cursor/mcp.json`
- `.cursor/rules/specgraph-bootstrap.mdc`
- `.cursor/rules/specgraph.mdc`
- `.cursor/rules/specgraph-post-stage.mdc`

Verify all three Cursor rules landed:

```bash
ls -la .cursor/rules/
```

Each file should be present. Inspect one:

```bash
head -10 .cursor/rules/specgraph.mdc
```

Expected layout:

```text
---
description: SpecGraph routing — use when the user mentions specs, ...
alwaysApply: false
---

<!-- specgraph:init v=2 sha256=<hex> -->
# SpecGraph Routing

You have access to the SpecGraph MCP server. ...
```

Sentinel line sits between the closing `---` and the `# SpecGraph Routing` heading. Cursor's mdc parser should treat the comment as inert.

## Verification

1. **Open the project in Cursor.** Open Cursor's Rules panel (Settings → Rules, or the editor's rules sidebar).

2. **Confirm both rules appear** with their descriptions:
   - `specgraph.mdc`: "SpecGraph routing — use when the user mentions specs, ..."
   - `specgraph-post-stage.mdc`: "After a SpecGraph stage transition, run the analytical passes ..."

3. **Trigger `specgraph.mdc`** by prompting Cursor's agent with a question about specs, e.g. "What's the next step in the shape stage?". Cursor should apply the rule (visible in the rule-fired indicator if your version of Cursor shows one). The agent should respond with awareness of MCP prompts `spark`/`shape`/`specify`/`decompose`/`approve`.

4. **Trigger `specgraph-post-stage.mdc`** by completing a stage transition via the `author` MCP tool. The agent should follow up by calling `analytical_pass` for each registered pass type.

5. **Inspect the rule preview** in Cursor's Rules panel for both files. The HTML comment with the sentinel SHOULD NOT render as visible content — if it appears in the rendered preview, that's a regression; file a bead.

## Idempotency

Run `specgraph init` again:

```bash
specgraph init
```

Verify no file changed:

```bash
git -C . status   # or jj status — if the test dir is a repo
```

Both rule files should show as untouched (no diff). If a hash byte changed, the canonical content drifted from the embedded source; investigate.

## Cleanup

```bash
rm -rf .cursor/ .specgraph/ opencode.json
```
````

- [ ] **Step 10.4: Run `task check` end-to-end**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
task check
```

Expected: all gates PASS (fmt:check, license:check, lint, build, unit tests).

Common failures and fixes:

- `revive` complains about a missing package doc comment → unlikely, we didn't add new packages.
- `addlicense` complains about a missing header on `vestigial_cursor_rules.go` or `symlink_pluginshim_test.go` → run `task license:add` to fix.
- `wrapcheck` or `gosec` warns on `wholefile.go` — review the warning and either fix or add a targeted `//nolint` with a reason.
- `gofumpt` flags formatting → run `task fmt`.

- [ ] **Step 10.5: Run `task pr-prep` (full pipeline)**

```bash
task pr-prep
```

Expected: passes through unit + integration + e2e. The e2e tests are skipped by default unless Docker-in-Docker is wired; that's fine.

- [ ] **Step 10.6: Sanity-check the dogfood `specgraph init` in this repo**

This repo dogfoods specgraph. Running init should now also write `.cursor/rules/specgraph.mdc` and `.cursor/rules/specgraph-post-stage.mdc` at the repo root:

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
./specgraph init
ls -la .cursor/rules/
```

If `.cursor/rules/specgraph.mdc` is now present at the repo root, verify it's gitignored (init-written, not committed):

```bash
git check-ignore .cursor/rules/specgraph.mdc .cursor/rules/specgraph-post-stage.mdc
```

If `git check-ignore` returns non-zero (i.e., file is NOT ignored), append to `.gitignore`:

```text
.cursor/rules/specgraph.mdc
.cursor/rules/specgraph-post-stage.mdc
```

Stage and amend that into Task 10's commit.

- [ ] **Step 10.7: Commit docs + smoke test**

```bash
jj describe -m "docs(plugin/cursor): SMOKE_TEST.md + README updates for new .mdc rules

- plugin/cursor/README.md: update file-name table to .mdc paths and note
  init-managed status for specgraph.mdc / specgraph-post-stage.mdc.
- plugin/specgraph/README.md: update Cursor row path.
- plugin/cursor/SMOKE_TEST.md: new manual end-to-end procedure mirroring
  PR C's OpenCode SMOKE_TEST shape.
- .gitignore: ignore new init-written cursor rules (if dogfood check
  surfaced them as untracked).

Refs: spgr-rwrp PR D §Documentation updates, §Tests / E2E

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Push + open PR

- [ ] **Step P.1: Set the bookmark and push**

The bead's branch convention is `spgr-rwrp-pr-d-cursor-rules` (matching PR C's `spgr-rwrp-pr-c-opencode-plugin`).

```bash
cd /Users/SeBrandt/Code/github.com/specgraph

# Confirm the right gh account: seanb4t has write access; SeBrandt_geico does not.
gh auth switch --user seanb4t

# Set the bookmark at the current head and push.
jj bookmark set spgr-rwrp-pr-d-cursor-rules -r @
jj git push --bookmark spgr-rwrp-pr-d-cursor-rules
```

If push fails on the pre-push hook, the hook runs `task check`. Re-read Step 10.4 — it should already have passed; if not, fix and re-push.

- [ ] **Step P.2: Open the PR**

```bash
gh pr create --title "spgr-rwrp PR D: Cursor rule files via embed-and-write" --body "$(cat <<'EOF'
## Summary

- `specgraph init` now writes `.cursor/rules/specgraph.mdc` and `.cursor/rules/specgraph-post-stage.mdc` as managed `WholeFile` entries.
- Adds frontmatter-aware sentinel positioning to the WholeFile strategy (new `HasFrontmatter` flag on `ManagedFile`); the sentinel lands after the closing `---` of YAML frontmatter, so Cursor's mdc parser still sees the frontmatter on line 1.
- Hash-guarded supersedes cleanup of pre-rename `.cursor/rules/{specgraph,post-stage}.md` — verbatim copies are deleted; user-edited variants are preserved with a `SyncResult.Detail` mismatch note.
- Drops the misleading `:start` suffix from whole-file `CommentHTML` sentinels (the only caller is wholefile.go; markdownblock emits `:start` inline, unaffected).
- Reverse-symlinks `plugin/cursor/.cursor/rules/{specgraph,specgraph-post-stage}.mdc` to the canonical embedded source, mirroring PR C's OpenCode pattern.

Design: [`docs/plans/2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md`](docs/plans/2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md)
Implementation plan: [`docs/plans/2026-05-12-spgr-rwrp-pr-d-implementation-plan.md`](docs/plans/2026-05-12-spgr-rwrp-pr-d-implementation-plan.md)

## Test plan

- [x] `task check` passes locally
- [x] `task pr-prep` passes locally
- [x] New unit tests cover Missing/Synced/Stale/Drifted (frontmatter-aware) and supersedes verbatim-delete / edited-preserve
- [x] Integration tests cover SyncAll scenarios end-to-end
- [x] Hash-pinning test locks pre-rename canonical bytes
- [x] Symlink-resolve test guards against dangling plugin/cursor/ symlinks
- [x] Back-compat anchor (TestNoLegacyWholeFileHTMLSentinels) prevents future WholeFile+HTML entries from regressing
- [ ] Manual smoke test in a real Cursor session — see plugin/cursor/SMOKE_TEST.md
EOF
)"
```

- [ ] **Step P.3: Update the bead**

```bash
bd update <bead-id> --priority P2 --status in-review
bd dolt push
```

Note the PR URL on the bead's notes for traceability.

---

## Self-review checklist

Run this before saying "plan complete":

**Spec coverage** — does the plan touch every section of the design doc?

- [ ] `HasFrontmatter` field on `ManagedFile` → Task 4
- [ ] `HasFrontmatter` invariants (Strategy + Comment) → Task 7
- [ ] Frontmatter-aware classify + render → Task 4
- [ ] Drop `:start` from RenderSentinel(CommentHTML) → Task 2
- [ ] HashExcludingSentinelAfterFrontmatter → Task 3
- [ ] Two new manifest entries with HasFrontmatter + SupersedesPath → Task 6
- [ ] Move canonical content + vestigial pre-rename bytes + prior-hash helper → Task 5
- [ ] SupersedesPath integration in wholeFileStrategy.Sync → Task 6
- [ ] Reverse-symlinks under plugin/cursor/ → Task 8
- [ ] Hash-pinning test → Task 5
- [ ] Back-compat anchor + embedded-mdc-splits test → Task 7
- [ ] Integration test scenarios → Task 9
- [ ] Documentation updates (READMEs + SMOKE_TEST.md) → Task 10
- [ ] `.gitignore` for new init-written paths → Task 10

**Placeholder scan** — search the plan for: "TBD", "TODO", "REPLACE-ME" (the only legitimate occurrence is in Step 5.4's test code, expected to be filled in Step 5.5).

**Type consistency** —

- `HasFrontmatter` (Task 4) referenced consistently as `HasFrontmatter` everywhere.
- `vestigialCursorSpecgraphMD` / `vestigialCursorPostStageMD` (Task 5) referenced consistently.
- `vestigialCursorRulePriorHash` (Task 5) signature `(supersedesPath string) string`.
- `renderWholeFile(mf, canonical)` (Task 4) new signature; all call sites updated.
- `HashExcludingSentinelAfterFrontmatter(syntax, content) (string, error)` (Task 3) referenced in Task 4.
