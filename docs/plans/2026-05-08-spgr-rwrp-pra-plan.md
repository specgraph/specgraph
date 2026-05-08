# PR A — managedfiles framework foundation implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `internal/config/managedfiles/` package foundation — types, primitives (hashing, sentinels, locking, atomic writes, symlink rejection), `SupersedesPath` logic, `dev` build tag, and `xdg.CacheHome()` — so subsequent PRs (B–G) can register managed files and execute init/doctor against them. **Zero behaviour change for users in this PR.**

**Architecture:** A single `internal/config/managedfiles/` package owns every "file specgraph injects into a project." It exposes a `ManagedFile` type (path + strategy + content source + harness ownership), a state-machine for drift classification (Missing / Synced / Stale / Drifted), and write/inspect entry points (`SyncAll`, `InspectAll`) that operate over a manifest. PR A wires the skeleton with an empty manifest; PRs B–G register actual files. Safety primitives (file locking, atomic writes, `O_NOFOLLOW` symlink rejection) are **ported** from the existing `internal/config/pointers/` package, not stubbed — they are correctness guarantees, not scaffolding.

**Tech Stack:** Go 1.26.3, standard library `embed`, `crypto/sha256`, `os`, `syscall`. No new third-party dependencies.

**Spec:** `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` — see PR A section + §"Architecture", §"Drift detection".

**Bead:** new `spgr-rwrp-pra` (file as part of Task 0).

---

## Repo conventions to know

- **Build:** `task build`. **Tests:** `task test`. **Quality gate:** `task check` (fmt, license, lint, build, race tests).
- **Lint:** `golangci-lint v2.12.1`, configured in `.golangci.yml`. Common gotchas: `revive` requires package doc comments on the first `.go` file; `wrapcheck` requires errors crossing public boundaries to be wrapped.
- **License headers:** every `.go` file MUST carry `// SPDX-License-Identifier: Apache-2.0` and `// Copyright 2026 Sean Brandt` on the first two lines. Run `task license:add` to fix.
- **Commits:** this repo uses jj-colocated git. `git commit` works (jj picks it up). Conventional Commits + DCO sign-off (`-s`) required. Plan steps use `git commit -s` for compatibility; `jj describe` is also fine.
- **Tests live alongside source:** `foo.go` and `foo_test.go` in the same package. Use `package managedfiles_test` for black-box tests where it improves clarity.
- **No test data outside test files:** if a test needs fixtures, embed them inline as Go strings or use `t.TempDir()`.

---

## File structure (final state after PR A)

```text
internal/config/managedfiles/
├── doc.go                  # package overview comment
├── types.go                # ManagedFile, Strategy, State, Harness, FileState, SyncResult, Options
├── errors.go               # sentinel errors (ErrCorruptedSentinel, ErrSymlinkRejected, ErrPriorCanonicalMismatch, ErrNotImplemented)
├── sentinel.go             # parseSentinel / renderSentinel; comment-syntax dispatch
├── hash.go                 # HashExcludingSentinel
├── lock.go                 # Unlocker type
├── lock_unix.go            # //go:build !windows — acquireFileLock via syscall.Flock
├── lock_windows.go         # //go:build windows  — acquireFileLock via LockFileEx
├── open_unix.go            # //go:build !windows — openExclusiveNoFollow via O_NOFOLLOW
├── open_windows.go         # //go:build windows  — openExclusiveNoFollow Windows fallback
├── atomic.go               # atomicWrite
├── symlink.go              # rejectSymlinkComponents, validateProjectDir
├── inspect.go              # Inspect(cwd, mf), InspectAll(cwd, harnesses)
├── sync.go                 # Sync(cwd, mf, opts), SyncAll(cwd, harnesses, opts)
├── strategy.go             # Strategy interface (Inspect, Render) + per-strategy stubs returning ErrNotImplemented
├── supersedes.go           # supersedesGuardedDelete (hash-check guard)
├── manifest.go             # Manifest() — empty []ManagedFile in PR A
├── source.go               # readSource(mf) — entry point
├── source_release.go       # //go:build !dev — reads from embed.FS
├── source_dev.go           # //go:build dev  — reads from disk + emits stderr banner if isatty
└── *_test.go               # tests per file

internal/xdg/
├── xdg.go                  # MODIFIED: add CacheHome()
└── xdg_test.go             # MODIFIED: add tests for CacheHome
```

The strategies (`StrategyJSONKeyMerge`, `StrategyMarkdownBlock`, `StrategyWholeFile`) **dispatch through stubs** in PR A — they return `ErrNotImplemented`. PR B implements `JSONKeyMerge` and `MarkdownBlock`; PRs C/D/E land `WholeFile` use cases. Manifest is empty in PR A, so the stubs are never invoked end-to-end — they exist for future strategy methods to fill in.

---

## Task 0: Pre-flight + bead creation

**Files:**
- Create: `spgr-rwrp-pra` bead (via `bd create`)

- [ ] **Step 1: Confirm we're on the right branch and clean**

```bash
cd /Volumes/Code/github.com/specgraph
jj --no-pager status | head -5
jj --no-pager log -r 'main..@' --no-graph -T 'commit_id.short() ++ " " ++ description.first_line() ++ "\n"' | head -5
```

Expected: working copy is empty or a benign existing commit; current stack should include the spgr-rwrp design doc + PR 0 commits already.

If your stack diverges, `jj new main` to get a clean starting point.

- [ ] **Step 2: File the bead**

```bash
bd create \
  --type=task \
  --priority=2 \
  --title="PR A: managedfiles framework foundation for spgr-rwrp" \
  --description="Foundation PR for the spgr-rwrp epic. New internal/config/managedfiles/ package with types, primitives (hash, sentinels, locking, atomic writes, symlink rejection), SupersedesPath logic, dev build tag, and xdg.CacheHome(). Zero behaviour change for users; empty manifest. Subsequent PRs (B-G) register managed files and wire init/doctor."
```

Note the returned bead id (e.g., `spgr-XXXX`) — used in commit messages below.

- [ ] **Step 3: Wire dependency to the epic + claim**

```bash
bd dep add spgr-rwrp <new-bead-id>
bd update <new-bead-id> --claim
bd dolt push  # if push fails, run: gh auth switch -u seanb4t && bd dolt push
```

---

## Task 1: Add `xdg.CacheHome()`

**Files:**
- Modify: `internal/xdg/xdg.go`
- Modify: `internal/xdg/xdg_test.go`

The drift-nudge throttle file (PR G) lives at `xdg.CacheHome() + "/nudges/<hash>"`. We add `CacheHome()` now in PR A so PR G can consume it without a separate dependency.

- [ ] **Step 1: Write the failing tests**

Append to `internal/xdg/xdg_test.go`:

```go
func TestCacheHome_Default(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".cache", "specgraph"), xdg.CacheHome())
}

func TestCacheHome_Override(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-test-cache")
	assert.Equal(t, "/tmp/xdg-test-cache/specgraph", xdg.CacheHome())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/xdg/ -run "TestCacheHome" -v
```

Expected: `FAIL` with `xdg.CacheHome undefined`.

- [ ] **Step 3: Add `CacheHome` to `xdg.go`**

In `internal/xdg/xdg.go`, after the existing `StateHome()` function (around line 47), append:

```go
// CacheHome returns XDG_CACHE_HOME/specgraph or ~/.cache/specgraph.
// Cache holds non-essential data that can be regenerated on demand
// (e.g. drift-nudge throttle files for `specgraph doctor`).
func CacheHome() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".cache", appName)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/xdg/ -v
```

Expected: PASS — all xdg tests including the two new ones.

- [ ] **Step 5: Run `task check` to confirm clean**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues. If lint fails on the new doc comment style, match the existing function-doc convention exactly.

- [ ] **Step 6: Commit**

```bash
git add internal/xdg/xdg.go internal/xdg/xdg_test.go
git commit -s -m "feat(xdg): add CacheHome() for XDG_CACHE_HOME

Mirrors the existing ConfigHome/DataHome/StateHome helpers. Consumed
by the drift-nudge throttle file added later in spgr-rwrp PR G.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 2: Bootstrap `managedfiles` package skeleton

**Files:**
- Create: `internal/config/managedfiles/doc.go`
- Create: `internal/config/managedfiles/types.go`
- Create: `internal/config/managedfiles/errors.go`
- Create: `internal/config/managedfiles/types_test.go`

This task introduces the package and its public types but no logic. After this task the package compiles and the types are referenced by tests.

- [ ] **Step 1: Create `doc.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package managedfiles is the framework specgraph init uses to inject
// canonical files into a user's project — and that specgraph doctor
// uses to detect drift.
//
// Every file specgraph writes is registered as a ManagedFile with a
// strategy that determines how the file is read, classified (Synced,
// Stale, Drifted, or Missing), and reconciled. The strategies are:
//
//   - StrategyJSONKeyMerge: managed keys merge into a JSON file;
//     siblings preserved (e.g. .mcp.json's mcpServers.specgraph block).
//   - StrategyMarkdownBlock: a versioned, hash-tracked block fenced by
//     <!-- specgraph:init:start ... --> / <!-- specgraph:init:end -->
//     within an otherwise-user-owned markdown file (e.g. AGENTS.md).
//   - StrategyWholeFile: the entire file is canonical; warn-and-force
//     on drift (e.g. a generated TypeScript plugin).
//
// PR A scaffolds the framework: types, sentinels, hashing, file locking,
// atomic writes, symlink rejection, supersedes-path deletion. Subsequent
// PRs in spgr-rwrp register managed files and implement strategies.
package managedfiles
```

- [ ] **Step 2: Create `errors.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "errors"

// ErrCorruptedSentinel indicates a managed file's sentinel line is
// malformed (unparseable, unknown version, missing required fields).
// The framework refuses to mutate corrupted-sentinel files.
var ErrCorruptedSentinel = errors.New("corrupted managed-file sentinel")

// ErrSymlinkRejected is returned when init/inspect encounters a symlink
// in a managed-file path component. We refuse to follow symlinks to avoid
// confused-deputy attacks (planting a symlink to an unrelated file then
// triggering init).
var ErrSymlinkRejected = errors.New("symlink in managed-file path rejected")

// ErrPriorCanonicalMismatch is returned by supersedesGuardedDelete when
// the on-disk content of a SupersedesPath does NOT match the prior
// canonical the framework would have produced. Indicates user content
// at the old path; init refuses to delete it.
var ErrPriorCanonicalMismatch = errors.New("supersedes-path content differs from prior canonical")

// ErrNotImplemented is returned by strategy stubs in PR A. PRs B and onward
// replace the stubs with real implementations; the manifest is empty in PR A
// so this error is never surfaced end-to-end.
var ErrNotImplemented = errors.New("strategy not implemented in this PR")
```

- [ ] **Step 3: Create `types.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Strategy selects how a ManagedFile is read, classified, and written.
type Strategy int

// Strategy values. Order is fixed; do not reorder (callers may compare
// by value via the iota positions).
const (
	StrategyJSONKeyMerge Strategy = iota
	StrategyMarkdownBlock
	StrategyWholeFile
)

// State is the framework's drift classification for a single managed file.
type State int

// State values. Synced is the only "no-op needed" state; the others all
// imply some action (write, refresh, or surface to the user).
const (
	StateMissing State = iota
	StateSynced
	StateStale   // sentinel hash matches disk content but disk doesn't match canonical
	StateDrifted // disk content does not match the recorded sentinel hash (user-edited)
)

// Harness is the agent-harness a ManagedFile belongs to.
type Harness int

// Harness values.
const (
	HarnessClaude Harness = iota
	HarnessCursor
	HarnessOpenCode
)

// CommentSyntax describes the comment style used to embed a sentinel
// line in a file. Each value maps to a (open, close) pair — close is
// empty for line comments.
type CommentSyntax int

// CommentSyntax values cover the file types managed by the framework.
const (
	CommentNone   CommentSyntax = iota // JSON files: no sentinel possible
	CommentSlash                       // // ...    (TypeScript, Go)
	CommentHash                        // # ...     (shell, YAML)
	CommentHTML                        // <!-- ... --> (markdown, mdc)
)

// ManagedFile describes a single file specgraph manages in a project.
// Construct via the Manifest function; do not build literals at call sites.
type ManagedFile struct {
	// Path is the file location relative to the project root.
	Path string

	// Strategy selects how this file is read, classified, written.
	Strategy Strategy

	// Source is the path within the package's embedded source tree to
	// read the canonical content from. Empty for JSON-key-merge files
	// where the canonical is built programmatically from project config.
	Source string

	// Comment is the comment syntax used for sentinel lines in this file.
	Comment CommentSyntax

	// Harness is which agent-harness this file belongs to. Used to filter
	// the manifest by the user's enabled harnesses.
	Harness Harness

	// SupersedesPath is the project-relative path of an older file that
	// is replaced by this one (e.g. a `.md` cursor rule renamed to `.mdc`).
	// Empty when the file has no predecessor. Init deletes this path
	// after a successful guarded write — see supersedesGuardedDelete.
	SupersedesPath string
}

// FileState is the result of Inspect for a single ManagedFile.
type FileState struct {
	Path         string
	Strategy     Strategy
	State        State
	DiskHash     string // sha256 of current disk content (empty if Missing)
	SentinelHash string // hash recorded in disk sentinel (empty if no sentinel)
	EmbeddedHash string // sha256 of canonical source content
	Detail       string // human-readable explanation, used in doctor output
}

// Action is the outcome of a write attempt for a single ManagedFile.
type Action int

// Action values.
const (
	ActionNoOp      Action = iota // file already Synced; nothing written
	ActionCreated                 // file was Missing; canonical written
	ActionRefreshed               // file was Stale; canonical rewritten with fresh sentinel
	ActionSkipped                 // file was Drifted; init skipped without --force
	ActionForced                  // file was Drifted; --force overwrote
	ActionError                   // some error occurred; see Err
)

// SyncResult reports what Sync did for a single ManagedFile.
type SyncResult struct {
	Path   string
	Action Action
	Err    error
}

// SyncOptions controls Sync behaviour.
type SyncOptions struct {
	// Force overwrites Drifted files with canonical content. Default false.
	Force bool

	// KeepEdits, when used with Force, preserves drifted on-disk content
	// but updates the sentinel hash to match. Default false.
	KeepEdits bool
}
```

- [ ] **Step 4: Create `types_test.go` with a sanity check that the types are usable**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// TestStrategyValuesStable pins the iota positions so future reorderings
// are caught. Callers may compare Strategy values directly.
func TestStrategyValuesStable(t *testing.T) {
	got := []managedfiles.Strategy{
		managedfiles.StrategyJSONKeyMerge,
		managedfiles.StrategyMarkdownBlock,
		managedfiles.StrategyWholeFile,
	}
	want := []managedfiles.Strategy{0, 1, 2}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Strategy[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestStateValuesStable pins the iota positions for State.
func TestStateValuesStable(t *testing.T) {
	got := []managedfiles.State{
		managedfiles.StateMissing,
		managedfiles.StateSynced,
		managedfiles.StateStale,
		managedfiles.StateDrifted,
	}
	want := []managedfiles.State{0, 1, 2, 3}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("State[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 5: Run tests + lint**

```bash
go test ./internal/config/managedfiles/ -v
task check 2>&1 | tail -8
```

Expected: tests PASS, `task check` clean. If lint complains about a missing package doc comment, double-check `doc.go` exists with the comment.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/
git commit -s -m "feat(managedfiles): bootstrap package with types and errors

Introduces internal/config/managedfiles/ — the foundation package for
spgr-rwrp's embed-and-write framework. Defines ManagedFile, Strategy,
State, Harness, CommentSyntax, FileState, Action, SyncResult,
SyncOptions, plus sentinel errors. No logic yet.

Tests pin iota values for Strategy and State so reorderings are caught.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 3: Sentinel parsing + rendering

**Files:**
- Create: `internal/config/managedfiles/sentinel.go`
- Create: `internal/config/managedfiles/sentinel_test.go`

The sentinel encodes `version`, `sha256`, and an optional `rev` field on a single line. For `WholeFile` strategy, the sentinel sits on the file's first line in native comment syntax. For `MarkdownBlock`, the sentinel rides on the start marker. For `JSONKeyMerge`, no sentinel exists.

We support `v=2` going forward; `v=1` is recognized for the upgrade path (no `sha256` field, parsed as `Stale` by Inspect).

- [ ] **Step 1: Write the failing tests**

`internal/config/managedfiles/sentinel_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestRenderSentinel_Slash(t *testing.T) {
	got := RenderSentinel(CommentSlash, Sentinel{Version: 2, SHA256: "abc123", Rev: "cef1ec3a"})
	want := "// specgraph:init v=2 sha256=abc123 rev=cef1ec3a"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSentinel_Hash(t *testing.T) {
	got := RenderSentinel(CommentHash, Sentinel{Version: 2, SHA256: "abc"})
	want := "# specgraph:init v=2 sha256=abc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSentinel_HTMLBlockStart(t *testing.T) {
	got := RenderSentinel(CommentHTML, Sentinel{Version: 2, SHA256: "abc", Rev: "cef"})
	want := "<!-- specgraph:init:start v=2 sha256=abc rev=cef -->"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseSentinel_Slash_v2(t *testing.T) {
	got, err := ParseSentinel(CommentSlash, "// specgraph:init v=2 sha256=abc123 rev=cef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 2 || got.SHA256 != "abc123" || got.Rev != "cef" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSentinel_v1(t *testing.T) {
	// v=1 is recognized for upgrade path (no sha256 field).
	got, err := ParseSentinel(CommentHTML, "<!-- specgraph:init:start v=1 -->")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 || got.SHA256 != "" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSentinel_v3_Rejected(t *testing.T) {
	_, err := ParseSentinel(CommentSlash, "// specgraph:init v=3 sha256=abc")
	if !errors.Is(err, ErrCorruptedSentinel) {
		t.Errorf("want ErrCorruptedSentinel, got %v", err)
	}
}

func TestParseSentinel_NotASentinel(t *testing.T) {
	got, err := ParseSentinel(CommentSlash, "// just a regular comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 0 {
		t.Errorf("expected zero Sentinel for non-sentinel line, got %+v", got)
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	for _, syntax := range []CommentSyntax{CommentSlash, CommentHash, CommentHTML} {
		original := Sentinel{Version: 2, SHA256: "deadbeef", Rev: "abc1234"}
		line := RenderSentinel(syntax, original)
		parsed, err := ParseSentinel(syntax, line)
		if err != nil {
			t.Fatalf("syntax %v: parse error: %v", syntax, err)
		}
		if parsed != original {
			t.Errorf("syntax %v: round-trip mismatch: got %+v, want %+v", syntax, parsed, original)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Sentinel" -v
```

Expected: FAIL with undefined identifiers (`Sentinel`, `RenderSentinel`, `ParseSentinel`).

- [ ] **Step 3: Implement `sentinel.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel is the parsed payload of a managed-file sentinel line.
//
// Version is the marker version (1 or 2). Version 0 indicates a non-sentinel
// line was parsed (treat as "no sentinel present").
//
// SHA256 is empty for v=1 markers (no hash field) and populated for v=2.
//
// Rev is the optional build revision recorded for forensics; not used in
// state classification.
type Sentinel struct {
	Version int
	SHA256  string
	Rev     string
}

// supportedVersions lists marker versions the parser accepts. Anything
// outside this set is treated as ErrCorruptedSentinel by ParseSentinel,
// matching the existing pointers/agents.go corruption-rejection behaviour
// for unknown versions.
var supportedVersions = map[int]bool{1: true, 2: true}

// sentinelMatcher matches both the WholeFile sentinel ("// specgraph:init v=N ...")
// and the MarkdownBlock start marker ("<!-- specgraph:init:start v=N ... -->").
//
// Group 1 captures the version digits.
// Group 2 captures the optional sha256 hex value.
// Group 3 captures the optional rev value.
//
// Both `init` and `init:start` are accepted because the same parser is
// used for both syntaxes; the caller's CommentSyntax decides which form
// RenderSentinel emits.
var sentinelMatcher = regexp.MustCompile(
	`specgraph:init(?::start)?\s+v=(\d+)(?:\s+sha256=([0-9a-fA-F]+))?(?:\s+rev=(\S+?))?\s*(?:-->)?$`,
)

// RenderSentinel formats a Sentinel as a single line in the given comment
// syntax. The returned line includes the comment delimiters but no trailing
// newline.
//
// For CommentNone, returns the empty string (JSON files don't carry sentinels).
//
// For CommentHTML, the rendered line is the START marker only — callers writing
// a MarkdownBlock are responsible for emitting the matching `<!-- specgraph:init:end -->`
// terminator. Keeping this asymmetry inside the strategy implementation avoids
// requiring sentinel.go to know about block structure.
func RenderSentinel(syntax CommentSyntax, s Sentinel) string {
	if syntax == CommentNone || s.Version == 0 {
		return ""
	}
	body := fmt.Sprintf("specgraph:init v=%d", s.Version)
	if s.SHA256 != "" {
		body += " sha256=" + s.SHA256
	}
	if s.Rev != "" {
		body += " rev=" + s.Rev
	}
	switch syntax {
	case CommentSlash:
		return "// " + body
	case CommentHash:
		return "# " + body
	case CommentHTML:
		// Block-strategy start marker. The end marker is emitted separately
		// by the strategy code (it has no payload).
		return "<!-- " + strings.Replace(body, "specgraph:init", "specgraph:init:start", 1) + " -->"
	default:
		return ""
	}
}

// ParseSentinel attempts to parse `line` as a managed-file sentinel.
//
// Returns:
//   - zero Sentinel + nil error if the line is not a sentinel at all (a
//     regular comment, blank, or arbitrary content).
//   - non-zero Sentinel + nil error on a successful parse.
//   - zero Sentinel + ErrCorruptedSentinel if the line *looks* like a
//     sentinel (matches the regex) but carries an unsupported version.
//
// Distinguishing "not a sentinel" from "corrupted sentinel" matters because
// the framework treats absent sentinels as user-owned (Drifted) but
// corrupted sentinels as a hard error (refuse-to-mutate).
func ParseSentinel(syntax CommentSyntax, line string) (Sentinel, error) {
	if syntax == CommentNone {
		return Sentinel{}, nil
	}
	m := sentinelMatcher.FindStringSubmatch(line)
	if m == nil {
		return Sentinel{}, nil
	}
	version, err := strconv.Atoi(m[1])
	if err != nil {
		return Sentinel{}, fmt.Errorf("%w: invalid version %q", ErrCorruptedSentinel, m[1])
	}
	if !supportedVersions[version] {
		return Sentinel{}, fmt.Errorf("%w: unsupported version %d", ErrCorruptedSentinel, version)
	}
	return Sentinel{
		Version: version,
		SHA256:  m[2],
		Rev:     m[3],
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Sentinel" -v
```

Expected: PASS — all 8 sentinel tests.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/sentinel.go internal/config/managedfiles/sentinel_test.go
git commit -s -m "feat(managedfiles): add sentinel parse/render

Sentinels carry version + sha256 + optional rev on a single line in the
file's native comment syntax. RenderSentinel emits the line; ParseSentinel
recovers the payload, distinguishing 'not a sentinel' (regular comment)
from 'corrupted sentinel' (unsupported version).

v=1 is recognized for the upgrade path (no sha256 field). v=3+ is
rejected as ErrCorruptedSentinel.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 4: Hash computation excluding sentinel

**Files:**
- Create: `internal/config/managedfiles/hash.go`
- Create: `internal/config/managedfiles/hash_test.go`

The sentinel must change without affecting the file's content hash, so the framework hashes content with the sentinel line dropped. This way two files with identical body but different `rev=` values hash equal.

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashExcludingSentinel_NoSentinel(t *testing.T) {
	body := "package foo\n\nfunc Bar() {}\n"
	got := HashExcludingSentinel(CommentSlash, []byte(body))
	want := hashOf(body)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestHashExcludingSentinel_WithSlashSentinel(t *testing.T) {
	body := "package foo\n\nfunc Bar() {}\n"
	withSentinel := "// specgraph:init v=2 sha256=abc rev=def\n" + body
	got := HashExcludingSentinel(CommentSlash, []byte(withSentinel))
	if got != hashOf(body) {
		t.Errorf("hash should equal body hash, got %s", got)
	}
}

func TestHashExcludingSentinel_StableAcrossRevChanges(t *testing.T) {
	body := "echo hi\n"
	a := "# specgraph:init v=2 sha256=abc rev=AAA\n" + body
	b := "# specgraph:init v=2 sha256=abc rev=BBB\n" + body
	if HashExcludingSentinel(CommentHash, []byte(a)) != HashExcludingSentinel(CommentHash, []byte(b)) {
		t.Error("hash differed across rev-only changes; should be stable")
	}
}

func TestHashExcludingSentinel_HTMLBlock(t *testing.T) {
	body := "# Title\n\nbody\n"
	withSentinel := "<!-- specgraph:init:start v=2 sha256=abc -->\n" + body + "<!-- specgraph:init:end -->\n"
	got := HashExcludingSentinel(CommentHTML, []byte(withSentinel))
	// For HTML/Markdown-block, BOTH the start AND end markers are dropped
	// before hashing, leaving the inner content.
	if got != hashOf(body) {
		t.Errorf("got %s, want %s", got, hashOf(body))
	}
}

func TestHashExcludingSentinel_NoneStrategy(t *testing.T) {
	body := `{"foo":"bar"}`
	// CommentNone (JSON files): no sentinel logic; hash the bytes as-is.
	got := HashExcludingSentinel(CommentNone, []byte(body))
	if got != hashOf(body) {
		t.Errorf("got %s, want %s", got, hashOf(body))
	}
}

// hashOf is a test helper: returns the hex sha256 of a string.
func hashOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Compile-time assertion that strings.Contains is referenced — silences
// the "imported and not used" lint if all tests are temporarily disabled.
var _ = strings.Contains
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Hash" -v
```

Expected: FAIL with undefined `HashExcludingSentinel`.

- [ ] **Step 3: Implement `hash.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashExcludingSentinel returns the hex-encoded sha256 of the file content
// after stripping the sentinel line(s).
//
// For CommentSlash and CommentHash: the FIRST line is dropped if it parses
// as a sentinel. Other lines are preserved verbatim.
//
// For CommentHTML (markdown-block strategy): every sentinel line in the
// content (start and end markers) is dropped before hashing. This handles
// files like AGENTS.md where the managed block is delimited by a pair.
//
// For CommentNone: the bytes are hashed as-is. JSON files don't carry
// sentinels, so there's nothing to strip.
//
// The hash is computed over the byte stream after sentinel-stripping,
// so two files differing only in their sentinel line hash equal.
func HashExcludingSentinel(syntax CommentSyntax, content []byte) string {
	if syntax == CommentNone {
		return hashBytes(content)
	}

	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))
	for i, line := range lines {
		// For slash/hash syntaxes, only consider the first line a candidate
		// sentinel — body content might legitimately contain "# specgraph:init"
		// inside (e.g., an example in a markdown rule body).
		if (syntax == CommentSlash || syntax == CommentHash) && i > 0 {
			kept = append(kept, line)
			continue
		}
		s, _ := ParseSentinel(syntax, line)
		if s.Version > 0 || strings.Contains(line, "specgraph:init:end") {
			// Drop sentinel start lines AND markdown-block end markers.
			// (The end marker doesn't parse as a Sentinel struct because
			// it has no version, but we drop it from the hash so the
			// framework can replace marker pairs without changing the hash.)
			continue
		}
		kept = append(kept, line)
	}
	return hashBytes([]byte(strings.Join(kept, "\n")))
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Hash" -v
```

Expected: PASS — all 5 hash tests.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/hash.go internal/config/managedfiles/hash_test.go
git commit -s -m "feat(managedfiles): add HashExcludingSentinel

Computes sha256 over file content with the sentinel line(s) stripped,
so the sentinel can change (e.g. rev= updates) without affecting the
hash. For markdown-block strategy both start and end markers are
dropped before hashing.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 5: Port file locking primitives

**Files:**
- Create: `internal/config/managedfiles/lock.go`
- Create: `internal/config/managedfiles/lock_unix.go`
- Create: `internal/config/managedfiles/lock_windows.go`
- Create: `internal/config/managedfiles/lock_test.go`

Port the file-locking primitives from `internal/config/pointers/`. The locking model is unchanged: an `Unlocker` is acquired via `acquireFileLock(path)` and released via the returned closure. The lock file is a sibling at `<path>.lock` and is intentionally never removed (deleting between unlock and a concurrent open creates a new inode and breaks mutual exclusion).

- [ ] **Step 1: Read the existing primitives**

```bash
cat internal/config/pointers/lock_unix.go
cat internal/config/pointers/lock_windows.go
```

Note the existing structure: `acquireFileLock` returns an `Unlocker` (a `func() error`) which releases the flock and closes the file handle.

- [ ] **Step 2: Write the failing tests**

`internal/config/managedfiles/lock_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireFileLock_BasicAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireFileLock(target)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := unlock(); err != nil {
		t.Errorf("unlock: %v", err)
	}
}

func TestAcquireFileLock_ContendedSerializes(t *testing.T) {
	// Two goroutines try to acquire the same file lock; the second must wait.
	// We measure that the second's acquire happened-after the first's release
	// using atomic counters.
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	var (
		firstReleased  atomic.Int64
		secondAcquired atomic.Int64
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1 — holds the lock briefly.
	go func() {
		defer wg.Done()
		unlock, err := acquireFileLock(target)
		if err != nil {
			t.Errorf("g1 acquire: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
		firstReleased.Store(time.Now().UnixNano())
		_ = unlock()
	}()

	// Goroutine 2 — starts slightly later, must wait for g1's release.
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		unlock, err := acquireFileLock(target)
		if err != nil {
			t.Errorf("g2 acquire: %v", err)
			return
		}
		secondAcquired.Store(time.Now().UnixNano())
		_ = unlock()
	}()

	wg.Wait()
	if secondAcquired.Load() < firstReleased.Load() {
		t.Errorf("second acquired before first released: %d < %d",
			secondAcquired.Load(), firstReleased.Load())
	}
}

func TestAcquireFileLock_LockfileSurvivesUnlock(t *testing.T) {
	// The .lock sibling file must not be removed on unlock — see comment
	// in lock_unix.go for why.
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireFileLock(target)
	if err != nil {
		t.Fatal(err)
	}
	_ = unlock()

	if _, err := os.Stat(target + ".lock"); err != nil {
		t.Errorf("lock file should persist after unlock, got: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Lock" -v
```

Expected: FAIL with undefined `acquireFileLock`.

- [ ] **Step 4: Create `lock.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Unlocker releases a file lock acquired via acquireFileLock. Returns
// any errors from the underlying flock LOCK_UN (Unix) or LockFileEx
// release (Windows) plus any error closing the lock-file handle.
//
// Callers MUST invoke the Unlocker via a deferred wrapper that propagates
// the error — leaving a lock unreleased breaks subsequent acquires.
type Unlocker func() error
```

- [ ] **Step 5: Create `lock_unix.go`**

Copy verbatim from `internal/config/pointers/lock_unix.go`, changing only the package declaration:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package managedfiles

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive advisory lock on a sibling file
// <path>.lock. Returns an Unlocker that must be called to release.
// The lock file is intentionally never removed: deleting it between unlock
// and a concurrent open creates a new inode and breaks mutual exclusion.
func acquireFileLock(path string) (Unlocker, error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	fd := int(lockFile.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return nil, errors.Join(
			fmt.Errorf("acquire file lock: %w", err),
			lockFile.Close(),
		)
	}
	return func() error {
		uerr := syscall.Flock(fd, syscall.LOCK_UN)
		cerr := lockFile.Close()
		if uerr != nil || cerr != nil {
			return errors.Join(uerr, cerr)
		}
		return nil
	}, nil
}
```

- [ ] **Step 6: Create `lock_windows.go`**

```bash
cat internal/config/pointers/lock_windows.go
```

Copy verbatim from the existing `internal/config/pointers/lock_windows.go`, change the package declaration to `package managedfiles`. Keep the `//go:build windows` tag.

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Lock" -v
```

Expected: PASS — all 3 lock tests.

- [ ] **Step 8: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues. (License header check should already pass since we copied the headers.)

- [ ] **Step 9: Commit**

```bash
git add internal/config/managedfiles/lock.go internal/config/managedfiles/lock_unix.go internal/config/managedfiles/lock_windows.go internal/config/managedfiles/lock_test.go
git commit -s -m "feat(managedfiles): port file-locking primitives from pointers/

Port acquireFileLock and Unlocker from internal/config/pointers/. Same
semantics: exclusive advisory lock on <path>.lock sibling, lock file
persists across unlocks (deleting between unlock and concurrent open
breaks mutual exclusion).

Tests cover basic acquire/release, contended serialization between two
goroutines, and lockfile-persists-after-unlock invariant.

The pointers/ package will be deleted in PR B; this is the framework's
canonical home for these primitives going forward.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 6: Port atomic write

**Files:**
- Create: `internal/config/managedfiles/atomic.go`
- Create: `internal/config/managedfiles/atomic_test.go`

`atomicWrite` writes to a temp file in the same directory, fsyncs, then renames over the target. The directory is fsynced after the rename so the rename is durable across power loss. Mode is preserved on update.

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := atomicWrite(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", string(got), "hello")
	}
}

func TestAtomicWrite_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(target, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("got %q, want %q", string(got), "new")
	}
}

func TestAtomicWrite_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	if err := atomicWrite(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "out.txt" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only out.txt, got %v", names)
	}
}

func TestAtomicWrite_RespectsMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.sh")
	if err := atomicWrite(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 0o755", info.Mode().Perm())
	}
}

func TestAtomicWrite_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deeper", "out.txt")
	if err := atomicWrite(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Atomic" -v
```

Expected: FAIL with undefined `atomicWrite`.

- [ ] **Step 3: Create `atomic.go`**

Copy verbatim from `internal/config/pointers/sync.go` lines 174–234 (the `atomicWrite` function with its docstring):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to <fullPath>.tmp.<random> in the same directory,
// fsyncs, then renames over fullPath. The directory is fsynced after the
// rename so the rename itself is durable across power loss.
//
// The temp file is chmod'd to mode. On a fresh write, callers pass 0o600.
// On an update, callers should read the existing file's mode and pass it so
// user-set permissions (e.g. 0o644) are preserved.
//
// All cleanup errors are joined to the original error via errors.Join so
// the caller sees both the proximate failure and any stranded-tempfile
// fallout.
func atomicWrite(fullPath string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(fullPath)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, werr := tmp.Write(data); werr != nil {
		return errors.Join(
			fmt.Errorf("write temp: %w", werr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if cerr := tmp.Chmod(mode); cerr != nil {
		return errors.Join(
			fmt.Errorf("chmod temp: %w", cerr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if serr := tmp.Sync(); serr != nil {
		return errors.Join(
			fmt.Errorf("fsync temp: %w", serr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if cerr := tmp.Close(); cerr != nil {
		return errors.Join(
			fmt.Errorf("close temp: %w", cerr),
			os.Remove(tmpName),
		)
	}
	if rerr := os.Rename(tmpName, fullPath); rerr != nil {
		return errors.Join(
			fmt.Errorf("rename: %w", rerr),
			os.Remove(tmpName),
		)
	}
	if dirF, derr := os.Open(dir); derr == nil {
		_ = dirF.Sync()  //nolint:errcheck // best-effort dir fsync; not fatal if unsupported
		_ = dirF.Close() //nolint:errcheck // best-effort cleanup
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Atomic" -v
```

Expected: PASS — all 5 atomic tests.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/atomic.go internal/config/managedfiles/atomic_test.go
git commit -s -m "feat(managedfiles): port atomicWrite from pointers/

Identical semantics: temp file in same dir, fsync, rename over target,
fsync dir for durable rename. Mode preserved on update; cleanup errors
joined via errors.Join.

Tests cover create, replace, no-temp-leftovers, mode preservation, and
parent-dir creation.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 7: Port symlink rejection

**Files:**
- Create: `internal/config/managedfiles/symlink.go`
- Create: `internal/config/managedfiles/symlink_test.go`
- Create: `internal/config/managedfiles/open_unix.go`
- Create: `internal/config/managedfiles/open_windows.go`

`rejectSymlinkComponents` walks each path component looking for symlinks; any symlink in a managed-file path is rejected with `ErrSymlinkRejected`. The `open_*.go` files provide platform-specific `O_NOFOLLOW` open semantics for actual file reads.

- [ ] **Step 1: Read the existing primitives**

```bash
cat internal/config/pointers/open_unix.go
cat internal/config/pointers/open_windows.go
```

- [ ] **Step 2: Write the failing tests**

`internal/config/managedfiles/symlink_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRejectSymlinkComponents_NoSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rejectSymlinkComponents(dir, "nested/foo.txt"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRejectSymlinkComponents_DetectsSymlinkInPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires admin on Windows")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}

	err := rejectSymlinkComponents(dir, "link/foo.txt")
	if !errors.Is(err, ErrSymlinkRejected) {
		t.Errorf("got %v, want ErrSymlinkRejected", err)
	}
}

func TestRejectSymlinkComponents_NonExistentComponentsAllowed(t *testing.T) {
	// Allow paths whose terminal components don't exist yet — init writes
	// new files, so non-existence at the leaf is normal.
	dir := t.TempDir()
	if err := rejectSymlinkComponents(dir, "does/not/exist.txt"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Symlink" -v
```

Expected: FAIL with undefined `rejectSymlinkComponents`.

- [ ] **Step 4: Create `symlink.go`**

Copy from `internal/config/pointers/sync.go` lines 150–172 (the `rejectSymlinkComponents` function), updating package decl:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rejectSymlinkComponents walks the components of relPath under projectDir
// and returns ErrSymlinkRejected on the first symlink encountered. Components
// that don't exist (e.g. a leaf file we haven't written yet) are allowed —
// init writes new files, so non-existence is normal at write time.
//
// Mirrors internal/config/mcpconfigs/sync.go's helper. Reproducing it here
// is cheaper than introducing a shared util package.
func rejectSymlinkComponents(projectDir, relPath string) error {
	cur := projectDir
	for _, part := range strings.Split(filepath.Clean(relPath), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("lstat %s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrSymlinkRejected, cur)
		}
	}
	return nil
}
```

- [ ] **Step 5: Create `open_unix.go`**

Copy from `internal/config/pointers/open_unix.go`, changing only the package decl. Keep the `//go:build !windows` tag:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package managedfiles

import (
	"os"
	"syscall"
)

// openExclusiveNoFollow opens a file for reading with O_NOFOLLOW so a
// symlink at the leaf is detected and rejected via the kernel rather
// than racing with our own pre-check. On Linux/macOS, opening a symlink
// with O_NOFOLLOW returns ELOOP.
//
// Returns (file, ErrSymlinkRejected) if path is a symlink.
func openExclusiveNoFollow(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		// ELOOP indicates symlink encountered.
		if pErr, ok := err.(*os.PathError); ok {
			if errno, ok := pErr.Err.(syscall.Errno); ok && errno == syscall.ELOOP {
				return nil, ErrSymlinkRejected
			}
		}
		return nil, err
	}
	return f, nil
}
```

- [ ] **Step 6: Create `open_windows.go`**

```bash
cat internal/config/pointers/open_windows.go
```

Copy verbatim from `internal/config/pointers/open_windows.go`, changing only the package decl. Keep the `//go:build windows` tag.

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Symlink" -v
```

Expected: PASS — all 3 symlink tests.

- [ ] **Step 8: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 9: Commit**

```bash
git add internal/config/managedfiles/symlink.go internal/config/managedfiles/open_unix.go internal/config/managedfiles/open_windows.go internal/config/managedfiles/symlink_test.go
git commit -s -m "feat(managedfiles): port symlink rejection from pointers/

rejectSymlinkComponents walks managed-file paths rejecting any symlink
component with ErrSymlinkRejected. openExclusiveNoFollow opens with
O_NOFOLLOW (Unix) so the kernel surfaces ELOOP at the leaf.

Tests cover no-symlinks (pass), symlink-in-path (rejected), and
nonexistent-leaf (allowed for init-creates-new-file).

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 8: Source loader with `dev` build tag

**Files:**
- Create: `internal/config/managedfiles/source.go`
- Create: `internal/config/managedfiles/source_release.go`
- Create: `internal/config/managedfiles/source_dev.go`
- Create: `internal/config/managedfiles/source_test.go`

`readSource(mf)` returns the canonical bytes for a `ManagedFile`. The release build reads from a package-level `embed.FS`. The `dev` build reads from disk at canonical source paths (set via `SPECGRAPH_DEV_SOURCE_ROOT` env var, default to `./plugin`). The dev build also emits a stderr banner on interactive invocations.

- [ ] **Step 1: Write the failing tests**

`internal/config/managedfiles/source_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"io/fs"
	"testing"
)

// PR A's manifest is empty, so readSource is exercised with an empty
// embed.FS. We test the not-found path explicitly and rely on PR B+ to
// add tests for actual content.

func TestReadSource_EmptyManifestSourceMissing(t *testing.T) {
	mf := ManagedFile{
		Path:   ".specgraph/agents/opencode/specgraph.ts",
		Source: "opencode/specgraph.ts",
	}
	_, err := readSource(mf)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestReadSource_EmptySourceField(t *testing.T) {
	// JSON-key-merge files set Source="" because their canonical is built
	// programmatically, not embedded. readSource returns nil bytes + nil error
	// to signal "no embedded content; let the strategy build it."
	mf := ManagedFile{
		Path:     ".mcp.json",
		Strategy: StrategyJSONKeyMerge,
		Source:   "",
	}
	got, err := readSource(mf)
	if err != nil {
		t.Errorf("expected nil error for empty Source, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil bytes, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "ReadSource" -v
```

Expected: FAIL with undefined `readSource`.

- [ ] **Step 3: Create `source.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// readSource returns the canonical bytes for a ManagedFile.
//
// Behaviour by build tag:
//   - Default build: reads from the package-level canonicalSources embed.FS
//     populated via //go:embed directives in source_release.go. PR A leaves
//     it empty; PRs C/D/E add directives.
//   - `dev` build tag: reads from disk at SPECGRAPH_DEV_SOURCE_ROOT (default
//     "./plugin") so a developer can edit a source file and re-run init
//     without rebuilding the binary.
//
// Returns (nil, nil) when mf.Source is empty (JSON-key-merge files build
// their canonical programmatically, not from an embedded asset).
//
// Implementation lives in source_release.go (no build tag — default build)
// or source_dev.go (`dev` build tag).
func readSource(mf ManagedFile) ([]byte, error) {
	if mf.Source == "" {
		return nil, nil
	}
	return readSourceImpl(mf)
}
```

- [ ] **Step 4: Create `source_release.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import "embed"

// canonicalSources is populated via //go:embed directives added in PR C+
// when actual managed-file source content lands in the binary. PR A leaves
// the FS empty, which means readSourceImpl returns fs.ErrNotExist for any
// non-empty mf.Source.
//
// The empty embed is intentional: it lets the framework compile and tests
// run even before any //go:embed directive references real files. Adding
// the first directive happens in PR C alongside the OpenCode plugin TS.
var canonicalSources embed.FS

// readSourceImpl reads from the embedded sources tree.
func readSourceImpl(mf ManagedFile) ([]byte, error) {
	return canonicalSources.ReadFile(mf.Source)
}
```

- [ ] **Step 5: Create `source_dev.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build dev

package managedfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/term"
)

// devSourceRoot resolves the directory dev builds read sources from.
// SPECGRAPH_DEV_SOURCE_ROOT overrides the default of "./plugin".
func devSourceRoot() string {
	if v := os.Getenv("SPECGRAPH_DEV_SOURCE_ROOT"); v != "" {
		return v
	}
	return "./plugin"
}

// devBannerOnce ensures the dev banner stderr line is printed at most once
// per process. Loud-but-not-spammy.
var devBannerOnce sync.Once

func emitDevBanner() {
	devBannerOnce.Do(func() {
		// Same isatty gate as the drift-nudge — a dev binary invoked
		// non-interactively (CI, hooks) shouldn't smear the banner across
		// captured stderr.
		if !term.IsTerminal(int(os.Stderr.Fd())) {
			return
		}
		fmt.Fprintf(os.Stderr,
			"specgraph: DEV BUILD — embedded files read from disk at %s\n",
			devSourceRoot())
	})
}

// readSourceImpl reads from disk at <devSourceRoot>/<mf.Source>.
func readSourceImpl(mf ManagedFile) ([]byte, error) {
	emitDevBanner()
	return os.ReadFile(filepath.Join(devSourceRoot(), mf.Source))
}
```

Note: the dev build depends on `golang.org/x/term`. Verify it's already in go.mod:

```bash
grep "golang.org/x/term" go.mod || echo "MISSING — need go get"
```

If missing, run `go get golang.org/x/term@latest` and `go mod tidy` before the next step.

- [ ] **Step 6: Run tests to verify they pass (default build)**

```bash
go test ./internal/config/managedfiles/ -run "ReadSource" -v
```

Expected: PASS.

- [ ] **Step 7: Smoke-test the `dev` build compiles**

```bash
go build -tags dev ./cmd/specgraph
echo "exit=$?"
```

Expected: exit 0. (This catches any compile-time issues in source_dev.go before they bite later.)

- [ ] **Step 8: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 9: Commit**

```bash
git add internal/config/managedfiles/source.go internal/config/managedfiles/source_release.go internal/config/managedfiles/source_dev.go internal/config/managedfiles/source_test.go go.mod go.sum
git commit -s -m "feat(managedfiles): add source loader with dev build tag

readSource returns canonical bytes for a ManagedFile. Default build
reads from the package-level canonicalSources embed.FS (empty in PR A;
populated by //go:embed directives in PRs C+). 'dev' build tag swaps
to disk reads from SPECGRAPH_DEV_SOURCE_ROOT (default ./plugin),
emitting a one-shot stderr banner on interactive invocations.

The dev binary's isatty(stderr) gate matches the drift-nudge: visible
when a developer runs at a terminal, silent in pipes/CI/hooks.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 9: Inspect single file (state classification)

**Files:**
- Create: `internal/config/managedfiles/inspect.go`
- Create: `internal/config/managedfiles/inspect_test.go`

`Inspect(cwd, mf)` reads the on-disk file, parses the sentinel, computes hashes, and returns a `FileState`. This is the central state-machine: the four states (Missing / Synced / Stale / Drifted) are decidable from disk + binary alone.

For PR A, the strategies don't expose their own classification logic yet (they'll be added in PR B+). Inspect handles `WholeFile`-shaped reasoning generically using the sentinel + hash; the strategy-specific paths return `ErrNotImplemented` and are exercised by future PRs.

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspect_MissingFile(t *testing.T) {
	dir := t.TempDir()
	mf := ManagedFile{
		Path:     ".specgraph/agents/opencode/nope.ts",
		Strategy: StrategyWholeFile,
		Source:   "opencode/specgraph.ts",
		Comment:  CommentSlash,
	}
	got, err := Inspect(dir, mf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.State != StateMissing {
		t.Errorf("State = %v, want StateMissing", got.State)
	}
}

func TestInspect_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link.ts")); err != nil {
		t.Skip("symlink creation failed (likely Windows without admin)")
	}
	mf := ManagedFile{
		Path:     "link.ts",
		Strategy: StrategyWholeFile,
		Comment:  CommentSlash,
	}
	if _, err := Inspect(dir, mf); err == nil {
		t.Error("expected ErrSymlinkRejected, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "TestInspect" -v
```

Expected: FAIL with undefined `Inspect`.

- [ ] **Step 3: Create `inspect.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Inspect classifies the on-disk state of a single ManagedFile relative
// to its embedded canonical. Returns a FileState describing the four
// possible outcomes: Missing, Synced, Stale, Drifted.
//
// Returns an error only on operational failures (symlink in path,
// permission denied, etc.). Drift classifications are returned as a
// non-nil State, not as an error.
//
// PR A handles the WholeFile strategy generically using the sentinel +
// hash mechanism. JSONKeyMerge and MarkdownBlock strategy paths are
// reserved for PR B implementations.
func Inspect(cwd string, mf ManagedFile) (FileState, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, err
	}

	full := filepath.Join(cwd, mf.Path)
	disk, readErr := os.ReadFile(full)
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		return FileState{
			Path:     mf.Path,
			Strategy: mf.Strategy,
			State:    StateMissing,
			Detail:   "file does not exist",
		}, nil
	case readErr != nil:
		return FileState{}, fmt.Errorf("read %s: %w", full, readErr)
	}

	canonical, err := readSource(mf)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return FileState{}, fmt.Errorf("read source for %s: %w", mf.Path, err)
	}

	diskHash := HashExcludingSentinel(mf.Comment, disk)
	embeddedHash := ""
	if canonical != nil {
		embeddedHash = HashExcludingSentinel(mf.Comment, canonical)
	}

	// Strategy-specific classification is implemented in PRs B+. Until then
	// we return a Detail noting the file exists but state is undetermined.
	// The empty manifest in PR A means this path is never reached
	// end-to-end.
	return FileState{
		Path:         mf.Path,
		Strategy:     mf.Strategy,
		State:        StateSynced, // placeholder; PR B implements per-strategy
		DiskHash:     diskHash,
		EmbeddedHash: embeddedHash,
		Detail:       "PR A: classification deferred to per-strategy code in PR B",
	}, nil
}

// InspectAll iterates the manifest filtered by the user's enabled harnesses
// and returns a FileState for each. Errors at the per-file level are
// captured in the FileState (not surfaced as an error return) so callers
// see all results, not just the first failure.
//
// In PR A, Manifest() returns an empty slice; InspectAll therefore returns
// an empty slice. PRs B+ populate the manifest.
func InspectAll(cwd string, harnesses []Harness) ([]FileState, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]FileState, 0, len(mfs))
	for _, mf := range mfs {
		fs, err := Inspect(cwd, mf)
		if err != nil {
			out = append(out, FileState{
				Path:     mf.Path,
				Strategy: mf.Strategy,
				State:    StateDrifted, // conservative fallback
				Detail:   fmt.Sprintf("inspect error: %v", err),
			})
			continue
		}
		out = append(out, fs)
	}
	return out, nil
}

// validateProjectDir rejects non-existent dirs, non-dirs, and symlink-rooted
// project directories. Mirrors pointers/sync.go's projectDir guard.
func validateProjectDir(projectDir string) error {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project dir %q is not a directory", projectDir)
	}
	li, lerr := os.Lstat(projectDir)
	if lerr != nil {
		return fmt.Errorf("lstat %s: %w", projectDir, lerr)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlinkRejected, projectDir)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "TestInspect" -v
```

Expected: PASS.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/inspect.go internal/config/managedfiles/inspect_test.go
git commit -s -m "feat(managedfiles): add Inspect/InspectAll skeleton

Inspect classifies a single ManagedFile's on-disk state. Symlink
rejection, missing-file detection, and disk/canonical hash computation
are wired in PR A; per-strategy state classification (Synced/Stale/
Drifted distinction) is reserved for PRs B+ where strategies are
implemented. Empty manifest in PR A means the placeholder Synced
return is never reached end-to-end.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 10: SupersedesPath guarded delete

**Files:**
- Create: `internal/config/managedfiles/supersedes.go`
- Create: `internal/config/managedfiles/supersedes_test.go`

When a manifest entry has `SupersedesPath` set, init deletes the old path after writing the new one — but only if the on-disk content matches the prior canonical (proving we wrote it ourselves, not the user). This guards against deleting hand-edited content during cursor `.md` → `.mdc` and similar renames.

The "prior canonical" computation is supplied by the strategy at PR B time. PR A defines the guarded-delete primitive with an injectable "expected hash" argument.

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSupersedesGuardedDelete_NoFile(t *testing.T) {
	dir := t.TempDir()
	// Old path doesn't exist — guarded delete is a no-op success.
	if err := supersedesGuardedDelete(dir, "missing.md", "anyhash"); err != nil {
		t.Errorf("expected nil for missing file, got %v", err)
	}
}

func TestSupersedesGuardedDelete_HashMatches_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := "old.md"
	full := filepath.Join(dir, oldPath)
	content := "old canonical"
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	expected := HashExcludingSentinel(CommentNone, []byte(content))
	if err := supersedesGuardedDelete(dir, oldPath, expected); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Errorf("file should be deleted, stat err = %v", err)
	}
}

func TestSupersedesGuardedDelete_HashMismatch_LeavesFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := "old.md"
	full := filepath.Join(dir, oldPath)
	if err := os.WriteFile(full, []byte("user-edited content"), 0o644); err != nil {
		t.Fatal(err)
	}

	expected := "0000000000000000000000000000000000000000000000000000000000000000" // wrong
	err := supersedesGuardedDelete(dir, oldPath, expected)
	if !errors.Is(err, ErrPriorCanonicalMismatch) {
		t.Errorf("got %v, want ErrPriorCanonicalMismatch", err)
	}
	if _, statErr := os.Stat(full); statErr != nil {
		t.Errorf("file should NOT be deleted on mismatch, stat err = %v", statErr)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Supersedes" -v
```

Expected: FAIL with undefined `supersedesGuardedDelete`.

- [ ] **Step 3: Create `supersedes.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// supersedesGuardedDelete deletes the file at <projectDir>/<oldPath> only
// if its on-disk content hash matches expectedPriorHash.
//
// expectedPriorHash is the hash this binary's *prior canonical* would have
// produced for oldPath — typically computed by the calling strategy using
// the vestigial v=1 renderer (see §"Drift detection / Vestigial v=1
// renderer" in the spec). The guard prevents init from clobbering user
// content that happens to live at a path being superseded.
//
// Returns nil if the file doesn't exist (nothing to delete) or was deleted
// successfully. Returns ErrPriorCanonicalMismatch if the hash check fails;
// the file is left in place. Returns wrapped errors for other failures
// (lstat, read, remove).
func supersedesGuardedDelete(projectDir, oldPath, expectedPriorHash string) error {
	full := filepath.Join(projectDir, oldPath)

	if err := rejectSymlinkComponents(projectDir, oldPath); err != nil {
		return err
	}

	disk, err := os.ReadFile(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("read %s: %w", full, err)
	}

	// CommentNone is intentional: callers pass the exact bytes the prior
	// canonical would have produced including any sentinel, so we hash
	// the raw bytes here and let the caller-provided hash account for
	// any sentinel-stripping it cares about. This keeps the guard
	// agnostic to comment syntax.
	actual := HashExcludingSentinel(CommentNone, disk)
	if actual != expectedPriorHash {
		return fmt.Errorf("%w: %s (got %s, want %s)",
			ErrPriorCanonicalMismatch, oldPath, actual, expectedPriorHash)
	}

	if rmErr := os.Remove(full); rmErr != nil {
		return fmt.Errorf("remove %s: %w", full, rmErr)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Supersedes" -v
```

Expected: PASS — all 3 supersedes tests.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/supersedes.go internal/config/managedfiles/supersedes_test.go
git commit -s -m "feat(managedfiles): add supersedesGuardedDelete

Deletes a SupersedesPath only when the on-disk content matches the
prior canonical hash supplied by the calling strategy. Mismatch
returns ErrPriorCanonicalMismatch; file is left in place for the
user to triage.

Tests cover missing-file (no-op success), hash-match (deletes), and
hash-mismatch (refuses to delete).

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 11: Strategy interface + dispatch stubs

**Files:**
- Create: `internal/config/managedfiles/strategy.go`
- Create: `internal/config/managedfiles/strategy_test.go`

The `Strategy` enum dispatches through an interface so per-strategy logic lives in dedicated methods in PR B+. PR A defines the interface and registers stubs that return `ErrNotImplemented`.

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestStrategyImpl_Inspect_NotImplemented(t *testing.T) {
	for _, s := range []Strategy{StrategyJSONKeyMerge, StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Inspect("/tmp", ManagedFile{Strategy: s})
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("Strategy %d Inspect should return ErrNotImplemented, got %v", s, err)
		}
	}
}

func TestStrategyImpl_Sync_NotImplemented(t *testing.T) {
	for _, s := range []Strategy{StrategyJSONKeyMerge, StrategyMarkdownBlock, StrategyWholeFile} {
		impl := strategyImpl(s)
		_, err := impl.Sync("/tmp", ManagedFile{Strategy: s}, SyncOptions{})
		if !errors.Is(err, ErrNotImplemented) {
			t.Errorf("Strategy %d Sync should return ErrNotImplemented, got %v", s, err)
		}
	}
}

func TestStrategyImpl_UnknownStrategy_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown strategy")
		}
	}()
	_ = strategyImpl(Strategy(99))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "Strategy" -v
```

Expected: FAIL with undefined `strategyImpl`.

- [ ] **Step 3: Create `strategy.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "fmt"

// strategy is the interface implemented per-strategy in PR B+. PR A
// registers stubs that return ErrNotImplemented for both methods.
//
// Inspect classifies the on-disk state for a single ManagedFile.
// Sync writes (or refrains from writing) the canonical content per
// SyncOptions. Both methods MUST be safe to call with mf.Strategy
// matching the dispatched strategy; misuse is a programming error.
type strategy interface {
	Inspect(cwd string, mf ManagedFile) (FileState, error)
	Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error)
}

// strategyImpl returns the strategy implementation for s.
//
// Panics if s is not a known Strategy value — that's a programming error
// from a manifest entry with a bogus enum value, not a runtime condition.
func strategyImpl(s Strategy) strategy {
	switch s {
	case StrategyJSONKeyMerge:
		return jsonKeyMergeStrategy{}
	case StrategyMarkdownBlock:
		return markdownBlockStrategy{}
	case StrategyWholeFile:
		return wholeFileStrategy{}
	default:
		panic(fmt.Sprintf("unknown Strategy value: %d", s))
	}
}

// PR A stubs. All three return ErrNotImplemented; PRs B/C/D/E replace
// each one with a real implementation.

type jsonKeyMergeStrategy struct{}

func (jsonKeyMergeStrategy) Inspect(cwd string, mf ManagedFile) (FileState, error) {
	return FileState{}, ErrNotImplemented
}
func (jsonKeyMergeStrategy) Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error) {
	return SyncResult{}, ErrNotImplemented
}

type markdownBlockStrategy struct{}

func (markdownBlockStrategy) Inspect(cwd string, mf ManagedFile) (FileState, error) {
	return FileState{}, ErrNotImplemented
}
func (markdownBlockStrategy) Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error) {
	return SyncResult{}, ErrNotImplemented
}

type wholeFileStrategy struct{}

func (wholeFileStrategy) Inspect(cwd string, mf ManagedFile) (FileState, error) {
	return FileState{}, ErrNotImplemented
}
func (wholeFileStrategy) Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error) {
	return SyncResult{}, ErrNotImplemented
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -run "Strategy" -v
```

Expected: PASS — all 3 strategy tests.

- [ ] **Step 5: Run `task check`**

```bash
task check 2>&1 | tail -8
```

Expected: 0 issues. If `unparam` complains about the unused `cwd`/`mf`/`opts` parameters in stubs, ignore — they'll be used in PR B implementations and removing them now would break the interface contract.

- [ ] **Step 6: Commit**

```bash
git add internal/config/managedfiles/strategy.go internal/config/managedfiles/strategy_test.go
git commit -s -m "feat(managedfiles): add strategy interface + ErrNotImplemented stubs

The strategy interface dispatches per-strategy logic for Inspect and
Sync. PR A registers stubs returning ErrNotImplemented for all three
strategies (JSONKeyMerge, MarkdownBlock, WholeFile); PRs B/C/D/E
replace each stub with a real implementation.

Empty manifest in PR A means stubs are never invoked end-to-end.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 12: Manifest, Sync entry point, and end-to-end empty-manifest test

**Files:**
- Create: `internal/config/managedfiles/manifest.go`
- Create: `internal/config/managedfiles/sync.go`
- Create: `internal/config/managedfiles/integration_test.go`

`Manifest()` returns the list of managed files filtered by enabled harnesses. PR A returns an empty slice. `Sync` and `SyncAll` are the write-side entry points; PR A's empty manifest means they're no-ops.

- [ ] **Step 1: Write the failing tests**

`internal/config/managedfiles/integration_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

func TestManifest_EmptyInPRA(t *testing.T) {
	all := managedfiles.Manifest([]managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	})
	if len(all) != 0 {
		t.Errorf("Manifest() should be empty in PR A, got %d entries", len(all))
	}
}

func TestInspectAll_EmptyManifest_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := managedfiles.InspectAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("InspectAll on empty manifest should be empty, got %d", len(got))
	}
}

func TestSyncAll_EmptyManifest_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := managedfiles.SyncAll(dir, []managedfiles.Harness{managedfiles.HarnessOpenCode}, managedfiles.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("SyncAll on empty manifest should be empty, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/managedfiles/ -run "TestManifest|TestInspectAll|TestSyncAll" -v
```

Expected: FAIL with undefined `Manifest`, `SyncAll`.

- [ ] **Step 3: Create `manifest.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Manifest returns the list of ManagedFiles filtered by the given
// harnesses. Order is stable across calls — callers may rely on it for
// deterministic output (e.g. doctor's report).
//
// PR A returns an empty slice; PRs B/C/D/E populate it as they register
// managed files for each harness.
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

// allManagedFiles is the framework's full set, unfiltered by harness.
// PR A returns nil; PRs B+ append entries.
func allManagedFiles() []ManagedFile {
	return nil
}

func harnessSet(harnesses []Harness) map[Harness]bool {
	out := make(map[Harness]bool, len(harnesses))
	for _, h := range harnesses {
		out[h] = true
	}
	return out
}
```

- [ ] **Step 4: Create `sync.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "fmt"

// Sync reconciles a single ManagedFile against its embedded canonical,
// honouring SyncOptions (Force, KeepEdits). Returns a SyncResult
// describing what was done.
//
// PR A dispatches to per-strategy stubs that return ErrNotImplemented;
// the empty manifest means this is never called end-to-end. PRs B/C/D/E
// implement each strategy.
func Sync(cwd string, mf ManagedFile, opts SyncOptions) (SyncResult, error) {
	return strategyImpl(mf.Strategy).Sync(cwd, mf, opts)
}

// SyncAll iterates the manifest filtered by enabled harnesses, calls
// Sync on each, and returns one SyncResult per entry. Per-file errors
// are captured in the SyncResult (Action == ActionError); the iteration
// continues so partial failure produces a complete report.
//
// In PR A, Manifest() is empty, so SyncAll returns an empty slice
// regardless of input.
func SyncAll(cwd string, harnesses []Harness, opts SyncOptions) ([]SyncResult, error) {
	if err := validateProjectDir(cwd); err != nil {
		return nil, err
	}
	mfs := Manifest(harnesses)
	out := make([]SyncResult, 0, len(mfs))
	for _, mf := range mfs {
		r, err := Sync(cwd, mf, opts)
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

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/config/managedfiles/ -v
```

Expected: PASS — all tests across the package.

- [ ] **Step 6: Run full `task check`**

```bash
task check 2>&1 | tail -10
```

Expected: 0 issues. If lint complains about the unused `opts` in `Sync`/`SyncAll`, ignore — it threads through to strategy implementations in PR B.

- [ ] **Step 7: Commit**

```bash
git add internal/config/managedfiles/manifest.go internal/config/managedfiles/sync.go internal/config/managedfiles/integration_test.go
git commit -s -m "feat(managedfiles): add Manifest, Sync, SyncAll plumbing

Manifest returns the list of ManagedFiles filtered by enabled
harnesses (empty in PR A; populated in PRs B/C/D/E). Sync dispatches
to per-strategy implementations; SyncAll iterates with per-file error
capture.

Empty-manifest integration tests prove the plumbing wires through
correctly without exercising any strategy logic.

Refs spgr-XXXX (PR A scope), spgr-rwrp."
```

---

## Task 13: Final verification + close bead

**Files:** none (verification + bookkeeping)

- [ ] **Step 1: Run the full quality gate**

```bash
cd /Volumes/Code/github.com/specgraph
task check 2>&1 | tail -15
```

Expected: 0 issues. All Go tests pass; lint clean; build succeeds.

- [ ] **Step 2: Confirm the dev build still compiles**

```bash
go build -tags dev ./cmd/specgraph
echo "exit=$?"
```

Expected: exit 0.

- [ ] **Step 3: Inspect commit stack**

```bash
jj --no-pager log -r 'main..@' --no-graph -T 'commit_id.short() ++ " " ++ description.first_line() ++ "\n"' | head -20
```

Expected: ~12 commits, one per task. Conventional-commit format. All carry DCO sign-off.

- [ ] **Step 4: Close the bead**

```bash
bd close <pr-a-bead-id> --reason="Foundation merged. internal/config/managedfiles/ + xdg.CacheHome() + dev build tag complete; empty manifest. Unblocks PR B (port existing managed files into the new framework)."
gh auth switch -u seanb4t  # if needed for push
bd dolt push
```

- [ ] **Step 5: Open a PR**

```bash
jj --no-pager bookmark create spgr-rwrp-pra -r @
jj --no-pager git push --bookmark spgr-rwrp-pra
gh pr create --base main --head spgr-rwrp-pra \
  --title "feat(managedfiles): foundation framework (spgr-rwrp PR A)" \
  --body "$(cat <<'EOF'
## Summary

Foundation PR for spgr-rwrp (harness install parity via embed-and-write).

Introduces `internal/config/managedfiles/` with:
- Types: `ManagedFile`, `Strategy`, `State`, `Harness`, `FileState`, `SyncResult`, `SyncOptions`
- Sentinel parse/render for `// # <!-- -->` comment styles, with `v=1` recognition for the upgrade path and `v=3+` rejection
- `HashExcludingSentinel` for content hashing that's stable across sentinel changes
- File locking, atomic writes, symlink rejection (ported verbatim from `internal/config/pointers/`, will be deleted in PR B)
- `SupersedesPath` guarded delete with hash-check guard rails
- `dev` build tag swapping `//go:embed` for disk reads, with `isatty(stderr)`-gated banner
- Strategy interface with `ErrNotImplemented` stubs; PR B replaces JSONKeyMerge + MarkdownBlock; PRs C/D/E land WholeFile use cases
- `Manifest()` filtered by enabled harnesses (returns empty in PR A)
- `xdg.CacheHome()` added to internal/xdg/ for PR G's drift-nudge throttle file

**Zero behaviour change for users.** Empty manifest; `cmd/specgraph/init.go` is untouched. PR B subsumes the existing `mcpconfigs/` and `pointers/` packages and migrates the 5 already-managed files into this framework.

Closes spgr-XXXX (the PR A bead).

## Test plan

- [x] `task check` (fmt, license, lint, skills, build, race tests)
- [x] `go build -tags dev ./cmd/specgraph` succeeds
- [x] All new tests pass
- [ ] CI green
EOF
)"
```

If `gh auth switch` was used, switch back to the working account afterwards.

---

## Self-review (executed already during plan-write)

**Spec coverage:** All PR A scope items from `2026-05-08-spgr-rwrp-harness-install-parity-design.md` §"Children of `spgr-rwrp`, in landing order / PR A":

- Types: ✓ Task 2
- Sentinel parse/write for `ts`/`sh`/`md`/`mdc`: ✓ Task 3
- Hash computation: ✓ Task 4
- Port locking primitives: ✓ Task 5
- Port atomic write: ✓ Task 6 (covered together with locking; see also Task 7's symlink work)
- `O_NOFOLLOW` symlink rejection: ✓ Task 7
- `SupersedesPath` deletion logic: ✓ Task 10
- `InspectAll` and `SyncAll` plumbing: ✓ Tasks 9 and 12
- `xdg.CacheHome()`: ✓ Task 1
- `dev` build tag: ✓ Task 8

**Type consistency:** `ManagedFile` fields align across types.go, source.go, inspect.go, sync.go. `Strategy` enum values are stable per the iota tests. `FileState`/`SyncResult` are constructed identically across call sites.

**No placeholders:** all code blocks are complete; commit messages reference the PR A bead id (placeholder `spgr-XXXX` to be replaced once Task 0 returns the actual id).

---

## Open questions

- **`golang.org/x/term` dependency** for the dev banner's isatty: if go.mod doesn't already include it, Task 8 step 5 adds it via `go get`. Verify before merge that it's the only new dep introduced by PR A.
- **`unparam` complaints** on stub strategy methods: unused `cwd`/`mf`/`opts` parameters in stubs may trigger `unparam` warnings. Acceptable — the parameters become used in PR B implementations and removing them would force interface churn. Add `//nolint:unparam // implementation in PR B uses these` if the linter is strict.
