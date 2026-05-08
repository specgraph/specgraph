# Deprecate `specgraph inject` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `specgraph inject` with two pointer files written by an extended `specgraph init`, and delete inject end-to-end (CLI, RPC, package, callers, docs) in a single PR.

**Architecture:** New `internal/config/pointers/` package implements managed-block-fenced markdown writers for `AGENTS.md` and `.cursor/rules/specgraph-bootstrap.md`. `cmd/specgraph/init.go` calls it after `mcpconfigs.Sync` (only on success). The `Inject` RPC is reserved in proto, the method/handler/permission/CLI are deleted, and every test, doc, and converter that references `InjectTool*` or `InjectToolType` is removed.

**Tech Stack:** Go 1.22+ (existing), `regexp` for managed-block scanning, `os.Rename` for atomic write, `syscall.Flock` (Unix) for serialization. No new third-party dependencies.

**Companion design:** [docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-design.md](2026-05-06-spgr-yyjf-deprecate-inject-design.md)

---

## Pre-flight (read before starting)

- **DCO email:** `Sean Brandt <4678+seanb4t@users.noreply.github.com>`. Every commit message ends with `Signed-off-by: Sean Brandt <...>` (or use `git commit -s` / `jj describe -s`).
- **License header on `.go` files:** match `LICENSE_HEADER` byte-for-byte:

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```

  `task check` runs `addlicense -check -f LICENSE_HEADER`. Run `task license:add` if it complains.

- **Pre-commit ritual** (apply before EVERY `jj commit`):
  1. `jj --no-pager status`
  2. If `.beads/issues.jsonl` is dirty → `jj --no-pager restore --from @- .beads/issues.jsonl`
  3. If `web/src/lib/api/gen/*.ts` is dirty → `jj --no-pager restore --from main --to @ web/src/lib/api/gen/`
  4. After commit, before any subsequent commit: confirm `@` is empty.
- **gh auth note:** before any `jj git push`, run `gh auth switch -u seanb4t -h github.com`.
- **Working directory:** main repo or a `jj workspace add` workspace. This plan assumes the main workspace.
- **Branch / bookmark:** `spgr-yyjf-deprecate-inject`. Create with `jj bookmark create spgr-yyjf-deprecate-inject -r @-` after the first commit, push with `jj git push --bookmark spgr-yyjf-deprecate-inject`.
- **Compile-time gate:** `go build ./...` is the canonical "did I miss a caller?" check after every deletion. Run it after every task that touches Go files.

---

## File structure

| File | Action | Responsibility |
|---|---|---|
| `internal/config/pointers/doc.go` | Create | Package doc comment (revive requires it). |
| `internal/config/pointers/sync.go` | Create | `Sync`, `Options`, `Action`, `SyncResult`, lock helpers, atomic write. |
| `internal/config/pointers/agents.go` | Create | AGENTS.md template, marker scanning, legacy-block purge regex. |
| `internal/config/pointers/cursor.go` | Create | `.cursor/rules/specgraph-bootstrap.md` template, frontmatter handling. |
| `internal/config/pointers/lock_unix.go` | Create | Build-tagged Unix flock helper (lifted from `internal/inject/lock_unix.go`). |
| `internal/config/pointers/lock_windows.go` | Create | Build-tagged no-op (lifted from `internal/inject/lock_windows.go`). |
| `internal/config/pointers/sync_test.go` | Create | All `TestSync_*` cases listed in the design's testing section. |
| `cmd/specgraph/init.go` | Modify | Call `pointers.Sync` after successful `mcpconfigs.Sync`; print results. |
| `cmd/specgraph/init_test.go` | Modify | Add the new `TestInit_*` cases for pointer integration. |
| `cmd/specgraph/inject.go` | Delete | Inject CLI subcommand. |
| `cmd/specgraph/sync_test.go` | Modify | Delete the four `TestInjectCmd_*` tests. |
| `cmd/specgraph/docs.go` | Modify | Drop `"inject"` from the command-list literal. |
| `internal/inject/` | Delete | Entire package: `inject.go`, `inject_test.go`, `lock_unix.go`, `lock_windows.go`. |
| `internal/server/sync_handler.go` | Modify | Delete the `Inject` method and the `internal/inject` import. |
| `internal/server/sync_handler_test.go` | Modify | Delete every `TestSyncHandler_Inject_*` test. |
| `internal/server/convert_constitution.go` | Modify | Delete `injectToolFromProto`. |
| `internal/server/convert_constitution_test.go` | Modify | Delete tests for `injectToolFromProto`. |
| `internal/server/convert_test.go` | Modify | Delete `TestInjectToolFromProto` if present. |
| `internal/storage/sync.go` | Modify | Delete `InjectToolType` and constants. |
| `internal/auth/permissions.go` | Modify | Delete `SyncServiceInjectProcedure` entry. |
| `internal/auth/permissions_test.go` | Modify | Delete the `SyncServiceInjectProcedure` reference. |
| `proto/specgraph/v1/sync.proto` | Modify | Reserve `Inject*` messages, enum, RPC method. |
| `gen/specgraph/v1/sync.pb.go` | Regenerate | Run `task proto`. |
| `gen/specgraph/v1/specgraphv1connect/sync.connect.go` | Regenerate | Run `task proto`. |
| `web/src/lib/api/gen/specgraph/v1/sync_pb.ts` | Regenerate | Run `task proto`. |
| `README.md` | Modify | Remove `specgraph inject` table row. |
| `CLAUDE.md` | Modify | Drop `internal/inject/` row, add `internal/config/pointers/` row, update Gotchas. |
| `site/docs/architecture.md` | Modify | Remove `Inject` Mermaid node, update SyncService row, drop `inject/` from directory tree. |
| `site/docs/cli-reference.md` | Modify | Delete the `### specgraph inject` section. |
| `site/docs/ecosystem.md` | Modify | Remove "Tool Injection" bullet and rephrase line 125. |
| `CHANGELOG.md` | Modify | Add a `## Unreleased` entry for the removal. |

---

## Task ordering

Tasks are ordered to keep the build green at every commit. Each task ends with `go build ./...` (or the equivalent gate) so a regression is caught at its source.

1. **T1** — Build `internal/config/pointers/` package, tests, no callers yet (parallel-safe; pure addition).
2. **T2** — Wire `pointers.Sync` into `cmd/specgraph/init.go` and add init integration tests (depends on T1).
3. **T3** — Remove `Inject` RPC from proto + regenerate (snowplows the deletion train).
4. **T4** — Delete `Inject` server-side: handler method, handler tests, converter, converter tests, storage enum, auth permission and its test.
5. **T5** — Delete `inject` CLI subcommand and its CLI tests; delete `internal/inject/` package.
6. **T6** — Update docs (README, CLAUDE.md, site/docs/*, CHANGELOG).
7. **T7** — Manual smoke test, full `task pr-prep`, push, open PR.

---

## Task 1: Build the `internal/config/pointers/` package

**Files:**

- Create: `internal/config/pointers/doc.go`
- Create: `internal/config/pointers/sync.go`
- Create: `internal/config/pointers/agents.go`
- Create: `internal/config/pointers/cursor.go`
- Create: `internal/config/pointers/lock_unix.go`
- Create: `internal/config/pointers/lock_windows.go`
- Create: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Create the package doc file**

`internal/config/pointers/doc.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package pointers renders and synchronizes managed-block-fenced markdown
// pointer files (AGENTS.md and .cursor/rules/specgraph-bootstrap.md) that
// direct an agent harness at a running SpecGraph MCP server.
//
// The mutation primitive is managed-block fencing: a single project-level
// block delimited by <!-- specgraph:init:start v=1 --> /
// <!-- specgraph:init:end --> markers. Content outside the block is owned
// by the user; content inside is reset to the canonical render every Sync.
//
// Sync also actively purges legacy per-slug blocks that the deprecated
// specgraph inject command used to write into AGENTS.md (markers of the
// shape <!-- specgraph:<slug>:start --> / <!-- specgraph:<slug>:end -->).
package pointers
````

- [ ] **Step 2: Write the public surface in `sync.go` (no internals yet, just types and signatures)**

`internal/config/pointers/sync.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Action describes what Sync did to a single managed pointer file. The string
// values are deliberately identical to mcpconfigs.Action values so init can
// render a unified "<path>: <action>" output.
type Action string

const (
	ActionCreated Action = "created"
	ActionUpdated Action = "updated"
	ActionNoOp    Action = "no-op"
	ActionError   Action = "error"
)

// SyncResult reports what Sync did to a single managed pointer file.
type SyncResult struct {
	Path               string
	Action             Action
	Err                error
	LegacyBlocksPurged int
}

// Options carries the canonical values that init derives once and threads
// into the pointer templates.
type Options struct {
	ServerURL   string
	ProjectSlug string
}

// Sync reconciles all pointer files for the project. Returns a slice with
// one SyncResult per file in the order [AGENTS.md, .cursor/rules/specgraph-bootstrap.md].
//
// If projectDir is missing or not a directory, Sync returns a single-element
// slice with Action == ActionError and Path == "<projectDir>".
//
// A failure on one pointer file is reported via SyncResult.Err with
// Action == ActionError; the other file is still processed. This differs
// from mcpconfigs.Sync, which aborts on first error. The caller (init)
// reconciles by running mcpconfigs first and only invoking pointers.Sync if
// mcpconfigs succeeded.
func Sync(projectDir string, opts Options) []SyncResult {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		msg := fmt.Errorf("projectDir %q is not a directory", projectDir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Errorf("stat %s: %w", projectDir, err)
		}
		return []SyncResult{{Path: projectDir, Action: ActionError, Err: msg}}
	}
	return []SyncResult{
		syncAgents(projectDir, opts),
		syncCursor(projectDir, opts),
	}
}

// errResult is a small convenience for syncAgents / syncCursor.
func errResult(path string, err error) SyncResult {
	return SyncResult{Path: path, Action: ActionError, Err: err}
}

// rejectSymlinkComponents copies the helper from internal/config/mcpconfigs/sync.go
// to keep packages independent. Reproducing it is cheaper than introducing a
// shared util.
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
			return fmt.Errorf("refusing to follow symlink %s", cur)
		}
	}
	return nil
}

// atomicWrite writes data to <fullPath>.tmp.<random> in the same directory
// then renames over fullPath. Removes the temp on failure.
func atomicWrite(fullPath string, data []byte) error {
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
		tmp.Close()        //nolint:errcheck // best-effort cleanup
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("write temp: %w", werr)
	}
	if cerr := tmp.Chmod(0o600); cerr != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("chmod temp: %w", cerr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp: %w", cerr)
	}
	if rerr := os.Rename(tmpName, fullPath); rerr != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename: %w", rerr)
	}
	return nil
}
````

- [ ] **Step 3: Add the Unix and Windows lock files**

`internal/config/pointers/lock_unix.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package pointers

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive advisory lock on a sibling file
// <path>.lock. Returns an unlock function that must be called to release.
// The lock file is intentionally never removed: deleting it between unlock
// and a concurrent open creates a new inode and breaks mutual exclusion.
func acquireFileLock(path string) (func(), error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	fd := int(lockFile.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		lockFile.Close() //nolint:errcheck // lock acquisition failed
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}
	return func() {
		if uerr := syscall.Flock(fd, syscall.LOCK_UN); uerr != nil {
			slog.Error("failed to release file lock", "path", path, "error", uerr)
		}
		lockFile.Close() //nolint:errcheck // best-effort close after unlock
	}, nil
}
````

`internal/config/pointers/lock_windows.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import "log/slog"

// acquireFileLock is a no-op on Windows. ADR posture: native Windows is
// best-effort; concurrent specgraph init runs on Windows are not serialized
// at the lock layer. The atomic-rename in atomicWrite still prevents a
// partially-purged file on disk; the worst case is "last writer wins".
func acquireFileLock(path string) (func(), error) {
	slog.Warn("file locking is not implemented on Windows; concurrent specgraph init runs may race", "path", path)
	return func() {}, nil
}
````

- [ ] **Step 4: Build to verify the skeleton compiles**

Run: `go build ./internal/config/pointers/`
Expected: PASS (zero output, exit 0).

- [ ] **Step 5: Commit the skeleton**

```bash
jj describe -m "feat(pointers): add package skeleton with Sync signature and helpers

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 6: Write the failing AGENTS.md tests**

Add to `internal/config/pointers/sync_test.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultOpts() Options {
	return Options{
		ServerURL:   "http://127.0.0.1:7890",
		ProjectSlug: "specgraph",
	}
}

func TestSync_CreatesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	results := Sync(dir, defaultOpts())
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	r := results[0]
	if r.Path != "AGENTS.md" {
		t.Errorf("results[0].Path = %q, want AGENTS.md", r.Path)
	}
	if r.Action != ActionCreated {
		t.Errorf("results[0].Action = %q, want %q", r.Action, ActionCreated)
	}
	if r.Err != nil {
		t.Errorf("results[0].Err = %v, want nil", r.Err)
	}

	body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	bs := string(body)
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("AGENTS.md missing start marker:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:end -->") {
		t.Errorf("AGENTS.md missing end marker:\n%s", bs)
	}
	if !strings.Contains(bs, "http://127.0.0.1:7890") {
		t.Errorf("AGENTS.md missing serverURL:\n%s", bs)
	}
	if !strings.HasSuffix(bs, "\n") {
		t.Errorf("AGENTS.md must end with newline; got last bytes %q", bs[max(0, len(bs)-5):])
	}
}

func TestSync_NoOpWhenIdentical(t *testing.T) {
	dir := t.TempDir()
	if r := Sync(dir, defaultOpts())[0]; r.Action != ActionCreated {
		t.Fatalf("first run: Action = %q, want %q", r.Action, ActionCreated)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionNoOp {
		t.Errorf("second run: Action = %q, want %q", r.Action, ActionNoOp)
	}
}

func TestSync_UpdatesWhenContentDiffers(t *testing.T) {
	dir := t.TempDir()
	Sync(dir, defaultOpts())
	r := Sync(dir, Options{ServerURL: "http://example.com:8080", ProjectSlug: "specgraph"})[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(body), "http://example.com:8080") {
		t.Errorf("AGENTS.md does not reflect new serverURL:\n%s", body)
	}
}

func TestSync_PreservesUserContentAroundBlock(t *testing.T) {
	dir := t.TempDir()
	const userTop = "# My project\n\nUser preamble.\n\n"
	const userBottom = "\n## My footer\n\nUser tail.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(userTop+userBottom), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.Contains(bs, "User preamble.") || !strings.Contains(bs, "User tail.") {
		t.Errorf("user content not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}

func TestSync_OverwritesUserContentInsideBlock(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nUSER NOTES THAT MUST DISAPPEAR\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	Sync(dir, defaultOpts())
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "USER NOTES THAT MUST DISAPPEAR") {
		t.Errorf("inside-block user content was not overwritten:\n%s", body)
	}
}

func TestSync_AppendsBlockToFileWithoutMarkers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User AGENTS\n\nbody.\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if !strings.HasPrefix(bs, "# User AGENTS\n\nbody.\n") {
		t.Errorf("user content not preserved at top:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}

// Used by TestSync_CreatesAgentsMD's "ends with newline" assertion.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
````

- [ ] **Step 7: Run the failing tests**

Run: `go test ./internal/config/pointers/ -run TestSync_ -v`
Expected: FAIL — `syncAgents` not defined.

- [ ] **Step 8: Implement `syncAgents` and the AGENTS template**

`internal/config/pointers/agents.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const agentsRel = "AGENTS.md"

const initStart = "<!-- specgraph:init:start v=1 -->"
const initEnd = "<!-- specgraph:init:end -->"

// initStartLoose matches an init start marker without the version suffix.
// Catches hand-rolled or pre-spec markers; treated as corruption.
var initStartLoose = regexp.MustCompile(`<!--\s*specgraph:init:start\s*-->`)

// legacyBlock matches inject's per-slug blocks. Slug class mirrors inject's
// safeSlugPattern: `[a-zA-Z0-9][a-zA-Z0-9._-]*`. The (?s) flag lets `.` match
// newlines so the body is captured. The slug is post-filtered to exclude
// the literal `init`.
var legacyBlock = regexp.MustCompile(
	`(?s)<!--\s*specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):start\s*-->.*?<!--\s*specgraph:\1:end\s*-->\n?`,
)

func renderAgentsBlock(opts Options) string {
	var b strings.Builder
	b.WriteString(initStart)
	b.WriteString("\n")
	b.WriteString("# SpecGraph project pointer\n\n")
	fmt.Fprintf(&b, "Server: %s\n", opts.ServerURL)
	fmt.Fprintf(&b, "Project: %s (sent as the X-Specgraph-Project header)\n\n", opts.ProjectSlug)
	b.WriteString("This block is managed by `specgraph init`. Edit content outside the markers.\n")
	b.WriteString("Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.\n")
	b.WriteString(initEnd)
	return b.String()
}

func syncAgents(projectDir string, opts Options) SyncResult {
	if err := rejectSymlinkComponents(projectDir, agentsRel); err != nil {
		return errResult(agentsRel, err)
	}
	full := filepath.Join(projectDir, agentsRel)

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return errResult(agentsRel, lerr)
	}
	defer unlock()

	existing, rerr := os.ReadFile(full)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return errResult(agentsRel, fmt.Errorf("read %s: %w", full, rerr))
	}

	canonical := renderAgentsBlock(opts)

	// Phase 1: validate existing init markers (corruption rules).
	if len(existing) > 0 {
		if err := validateInitMarkers(existing); err != nil {
			return errResult(agentsRel, err)
		}
	}

	// Phase 2: purge legacy per-slug blocks (post-filter excludes "init").
	updated := existing
	purged := 0
	if len(updated) > 0 {
		updated, purged = purgeLegacyBlocks(updated)
	}

	// Phase 3: reconcile init managed block.
	switch {
	case len(updated) == 0:
		updated = []byte(canonical + "\n")
	case bytes.Contains(updated, []byte(initStart)):
		updated = replaceInitBlock(updated, canonical)
	default:
		// File exists, no init block, no markers — append with leading blank line.
		if !bytes.HasSuffix(updated, []byte("\n")) {
			updated = append(updated, '\n')
		}
		updated = append(updated, '\n')
		updated = append(updated, canonical...)
		updated = append(updated, '\n')
	}

	if len(existing) > 0 && bytes.Equal(existing, updated) {
		return SyncResult{Path: agentsRel, Action: ActionNoOp}
	}

	if werr := atomicWrite(full, updated); werr != nil {
		return errResult(agentsRel, werr)
	}

	action := ActionUpdated
	if len(existing) == 0 {
		action = ActionCreated
	}
	return SyncResult{Path: agentsRel, Action: action, LegacyBlocksPurged: purged}
}

// validateInitMarkers returns an error for any of the four corruption rules:
// (1) end before start, (2) start without end, (3) double start, (4)
// init start marker missing the v=1 suffix.
func validateInitMarkers(data []byte) error {
	starts := bytes.Count(data, []byte(initStart))
	ends := bytes.Count(data, []byte(initEnd))

	// Rule 4: init-shaped marker without the v=1 suffix.
	loose := initStartLoose.FindAllIndex(data, -1)
	canonical := bytes.Index(data, []byte(initStart))
	for _, m := range loose {
		// If a loose match overlaps the canonical (versioned) start, skip it.
		if canonical >= 0 && m[0] == canonical {
			continue
		}
		return fmt.Errorf(
			"AGENTS.md contains an init marker without the expected v=1 suffix at offset %d; remove the marker or fix it manually",
			m[0],
		)
	}

	switch {
	case starts == 0 && ends == 0:
		return nil
	case starts == 1 && ends == 1:
		// Verify ordering.
		s := bytes.Index(data, []byte(initStart))
		e := bytes.Index(data, []byte(initEnd))
		if e < s {
			return fmt.Errorf("AGENTS.md: init end marker appears before start marker")
		}
		return nil
	case starts > 1:
		return fmt.Errorf("AGENTS.md: more than one init start marker")
	case starts == 1 && ends == 0:
		return fmt.Errorf("AGENTS.md: init start marker without matching end")
	default: // ends > 0, starts == 0
		return fmt.Errorf("AGENTS.md: init end marker without matching start")
	}
}

func purgeLegacyBlocks(data []byte) ([]byte, int) {
	purged := 0
	out := legacyBlock.ReplaceAllFunc(data, func(match []byte) []byte {
		// Post-filter: never purge the literal slug "init" (handled by phase 1).
		sub := legacyBlock.FindSubmatch(match)
		if len(sub) >= 2 && string(sub[1]) == "init" {
			return match
		}
		purged++
		return nil
	})
	return out, purged
}

func replaceInitBlock(data []byte, canonical string) []byte {
	startIdx := bytes.Index(data, []byte(initStart))
	endIdx := bytes.Index(data, []byte(initEnd))
	if startIdx < 0 || endIdx < startIdx {
		// validateInitMarkers ran first; this should be unreachable.
		return data
	}
	endLen := len(initEnd)
	out := make([]byte, 0, len(data))
	out = append(out, data[:startIdx]...)
	out = append(out, canonical...)
	out = append(out, data[endIdx+endLen:]...)
	return out
}
````

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/config/pointers/ -run TestSync_ -v`
Expected: all six AGENTS tests PASS.

- [ ] **Step 10: Commit AGENTS.md handling**

```bash
jj describe -m "feat(pointers): implement AGENTS.md sync with legacy purge

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 11: Write failing legacy-purge tests**

Add to `internal/config/pointers/sync_test.go`:

````go
func TestSync_PurgesLegacyInjectBlocks_SimpleSlugs(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:foo:start -->\ndigest A\n<!-- specgraph:foo:end -->\n" +
		"<!-- specgraph:bar-baz:start -->\ndigest B\n<!-- specgraph:bar-baz:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.LegacyBlocksPurged != 2 {
		t.Errorf("LegacyBlocksPurged = %d, want 2", r.LegacyBlocksPurged)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "specgraph:foo:") || strings.Contains(string(body), "specgraph:bar-baz:") {
		t.Errorf("legacy markers still present:\n%s", body)
	}
}

func TestSync_PurgesLegacyInjectBlocks_RealisticSlugs(t *testing.T) {
	dir := t.TempDir()
	// inject's safeSlugPattern allows uppercase, dots, underscores.
	seed := "<!-- specgraph:MySpec.v2:start -->\nA\n<!-- specgraph:MySpec.v2:end -->\n" +
		"<!-- specgraph:my_spec:start -->\nB\n<!-- specgraph:my_spec:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.LegacyBlocksPurged != 2 {
		t.Errorf("LegacyBlocksPurged = %d, want 2", r.LegacyBlocksPurged)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(body), "MySpec.v2") || strings.Contains(string(body), "my_spec") {
		t.Errorf("realistic-slug legacy markers still present:\n%s", body)
	}
}

func TestSync_LegacyMarkerWithInvalidSlugNotPurged(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:has space:start -->\nbody\n<!-- specgraph:has space:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.LegacyBlocksPurged != 0 {
		t.Errorf("LegacyBlocksPurged = %d, want 0", r.LegacyBlocksPurged)
	}
}

func TestSync_DoesNotPurgeInitMarker(t *testing.T) {
	dir := t.TempDir()
	// First create canonical state.
	Sync(dir, defaultOpts())
	// Run again; init block must persist (NoOp).
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionNoOp {
		t.Errorf("Action = %q, want %q", r.Action, ActionNoOp)
	}
	if r.LegacyBlocksPurged != 0 {
		t.Errorf("LegacyBlocksPurged = %d, want 0; the init block must not be matched by the legacy regex", r.LegacyBlocksPurged)
	}
}

func TestSync_LegacyShapedInitMarkerIsCorruption(t *testing.T) {
	dir := t.TempDir()
	// init marker WITHOUT v=1 — corruption rule #4 must fire BEFORE legacy purge.
	seed := "<!-- specgraph:init:start -->\nbody\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "v=1") {
		t.Errorf("Err = %v, want a v=1 error", r.Err)
	}
}
````

- [ ] **Step 12: Run the legacy-purge tests**

Run: `go test ./internal/config/pointers/ -run 'TestSync_(Purges|Legacy|DoesNotPurge)' -v`
Expected: PASS.

- [ ] **Step 13: Commit legacy-purge tests**

```bash
jj describe -m "test(pointers): cover legacy block purge and slug-init guard

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 14: Write failing corruption-detection tests**

Add to `internal/config/pointers/sync_test.go`:

````go
func TestSync_RejectsCorruptedMarkers_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:end -->\n<!-- specgraph:init:start v=1 -->\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_StartWithoutEnd(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsCorruptedMarkers_DoubleStart(t *testing.T) {
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:start v=1 -->\nmore\n<!-- specgraph:init:end -->\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_RejectsInitMarkerWithoutVersion(t *testing.T) {
	// Same shape as TestSync_LegacyShapedInitMarkerIsCorruption but without
	// the matching end marker — guards rule 4 in isolation.
	dir := t.TempDir()
	seed := "<!-- specgraph:init:start -->\nbody\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600)
	r := Sync(dir, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
	if r.Err == nil || !strings.Contains(r.Err.Error(), "v=1") {
		t.Errorf("Err = %v, want a v=1 error", r.Err)
	}
}

func TestSync_RejectsSymlinkInPath(t *testing.T) {
	dir := t.TempDir()
	// Replace the project dir's AGENTS.md path with a symlink chain by
	// putting AGENTS.md behind a symlinked subdir. We have to seat the
	// symlink at projectDir level since AGENTS.md is at the root; instead
	// symlink projectDir itself so rejectSymlinkComponents triggers when
	// joining its name.
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("symlink unsupported on this filesystem: %v", err)
	}
	r := Sync(link, defaultOpts())[0]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}
````

- [ ] **Step 15: Run the corruption tests**

Run: `go test ./internal/config/pointers/ -run 'TestSync_Rejects' -v`
Expected: PASS (if any FAIL, fix `validateInitMarkers` or the symlink test guard).

- [ ] **Step 16: Commit corruption tests**

```bash
jj describe -m "test(pointers): cover marker corruption and symlink rejection

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 17: Write failing cursor-rule tests**

Add to `internal/config/pointers/sync_test.go`:

````go
const cursorRel = ".cursor/rules/specgraph-bootstrap.md"

func TestSync_CreatesCursorRule(t *testing.T) {
	dir := t.TempDir()
	r := Sync(dir, defaultOpts())[1]
	if r.Path != cursorRel {
		t.Errorf("Path = %q, want %q", r.Path, cursorRel)
	}
	if r.Action != ActionCreated {
		t.Errorf("Action = %q, want %q", r.Action, ActionCreated)
	}
	body, err := os.ReadFile(filepath.Join(dir, cursorRel))
	if err != nil {
		t.Fatalf("read %s: %v", cursorRel, err)
	}
	bs := string(body)
	if !strings.HasPrefix(bs, "---\n") {
		t.Errorf("missing frontmatter header:\n%s", bs)
	}
	if !strings.Contains(bs, "alwaysApply: true") {
		t.Errorf("alwaysApply: true not in frontmatter:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing in body:\n%s", bs)
	}
}

func TestSync_RefusesCursorRuleWithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, cursorRel), []byte("# bare\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[1]
	if r.Action != ActionError {
		t.Errorf("Action = %q, want %q", r.Action, ActionError)
	}
}

func TestSync_PreservesCursorRuleFrontmatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "rules"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const userFM = "---\ndescription: my custom desc\nalwaysApply: false\nextraField: kept\n---\n\n"
	const userBlock = "<!-- specgraph:init:start v=1 -->\nstale\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, cursorRel), []byte(userFM+userBlock), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r := Sync(dir, defaultOpts())[1]
	if r.Action != ActionUpdated {
		t.Errorf("Action = %q, want %q", r.Action, ActionUpdated)
	}
	body, _ := os.ReadFile(filepath.Join(dir, cursorRel))
	bs := string(body)
	if !strings.Contains(bs, "description: my custom desc") {
		t.Errorf("user description not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "alwaysApply: false") {
		t.Errorf("user alwaysApply override not preserved:\n%s", bs)
	}
	if !strings.Contains(bs, "extraField: kept") {
		t.Errorf("user extra field not preserved:\n%s", bs)
	}
}
````

- [ ] **Step 18: Run cursor tests, see them fail**

Run: `go test ./internal/config/pointers/ -run 'TestSync_(Creates|Refuses|Preserves)Cursor' -v`
Expected: FAIL — `syncCursor` not implemented.

- [ ] **Step 19: Implement `syncCursor`**

`internal/config/pointers/cursor.go`:

````go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const cursorRel = ".cursor/rules/specgraph-bootstrap.md"

const defaultCursorFrontmatter = `---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---

`

func renderCursorBody(opts Options) string {
	return renderAgentsBlock(opts) + "\n"
}

func syncCursor(projectDir string, opts Options) SyncResult {
	if err := rejectSymlinkComponents(projectDir, cursorRel); err != nil {
		return errResult(cursorRel, err)
	}
	full := filepath.Join(projectDir, cursorRel)

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return errResult(cursorRel, lerr)
	}
	defer unlock()

	existing, rerr := os.ReadFile(full)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return errResult(cursorRel, fmt.Errorf("read %s: %w", full, rerr))
	}

	canonicalBody := renderCursorBody(opts)

	if errors.Is(rerr, fs.ErrNotExist) {
		out := []byte(defaultCursorFrontmatter + canonicalBody)
		if werr := atomicWrite(full, out); werr != nil {
			return errResult(cursorRel, werr)
		}
		return SyncResult{Path: cursorRel, Action: ActionCreated}
	}

	frontmatter, body, ferr := splitFrontmatter(existing)
	if ferr != nil {
		return errResult(cursorRel, ferr)
	}

	// Phase 1: corruption check on the body.
	if err := validateInitMarkers(body); err != nil {
		return errResult(cursorRel, err)
	}

	// Reconcile init block in body. (No legacy purge for cursor — inject's
	// per-slug rules lived in separate per-slug files, not inside this one.)
	var newBody []byte
	switch {
	case len(body) == 0:
		newBody = []byte(canonicalBody)
	case bytes.Contains(body, []byte(initStart)):
		newBody = replaceInitBlock(body, renderAgentsBlock(opts))
		if !bytes.HasSuffix(newBody, []byte("\n")) {
			newBody = append(newBody, '\n')
		}
	default:
		newBody = body
		if !bytes.HasSuffix(newBody, []byte("\n")) {
			newBody = append(newBody, '\n')
		}
		newBody = append(newBody, '\n')
		newBody = append(newBody, renderAgentsBlock(opts)...)
		newBody = append(newBody, '\n')
	}

	updated := append([]byte{}, frontmatter...)
	updated = append(updated, newBody...)

	if bytes.Equal(existing, updated) {
		return SyncResult{Path: cursorRel, Action: ActionNoOp}
	}

	if werr := atomicWrite(full, updated); werr != nil {
		return errResult(cursorRel, werr)
	}
	return SyncResult{Path: cursorRel, Action: ActionUpdated}
}

// splitFrontmatter splits a Cursor rule file into (frontmatter-including-trailing-blank, body).
// Returns an error if the file does not begin with a `---` line: an existing
// rule without frontmatter is malformed and we refuse to silently convert it.
func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf(
			"%s: missing YAML frontmatter (must start with '---'); remove the file or add frontmatter manually",
			cursorRel,
		)
	}
	// Find the second `---` line.
	rest := data[len("---\n"):]
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		return nil, nil, fmt.Errorf("%s: frontmatter not closed before EOF", cursorRel)
	}
	end := len("---\n") + idx + len("\n---\n")
	// Include any single trailing blank line.
	if end < len(data) && data[end] == '\n' {
		end++
	}
	return data[:end], data[end:], nil
}
````

Note: `cursorRel` is declared in `cursor.go` and the test refers to a top-level
`const cursorRel = ".cursor/rules/specgraph-bootstrap.md"` in the test file. Resolve
the duplicate by removing the test's `const cursorRel = ...` and importing the
package's value — but the test file is in the same package, so the test can use
the package-level constant directly. Delete the test-file `const cursorRel = ...`
line if it conflicts at compile time.

- [ ] **Step 20: Run cursor tests to verify they pass**

Run: `go test ./internal/config/pointers/ -run 'TestSync_(Creates|Refuses|Preserves)Cursor' -v`
Expected: PASS. If a duplicate `cursorRel` constant error appears, remove the test-file declaration.

- [ ] **Step 21: Run the entire pointers test suite**

Run: `go test ./internal/config/pointers/ -v`
Expected: all tests PASS.

- [ ] **Step 22: Commit cursor support**

```bash
jj describe -m "feat(pointers): implement cursor bootstrap rule sync

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

- [ ] **Step 23: Write failure-isolation, atomic-write, and concurrency tests**

Add to `sync_test.go`:

````go
func TestSync_FailureOnOneFileDoesNotAbortOther(t *testing.T) {
	dir := t.TempDir()
	// Corrupt AGENTS.md so it errors; cursor file is fresh and should succeed.
	seed := "<!-- specgraph:init:start v=1 -->\nbody\n" // missing end
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	results := Sync(dir, defaultOpts())
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Action != ActionError {
		t.Errorf("results[0] (AGENTS.md): Action = %q, want %q", results[0].Action, ActionError)
	}
	if results[1].Action != ActionCreated {
		t.Errorf("results[1] (cursor): Action = %q, want %q", results[1].Action, ActionCreated)
	}
}

func TestSync_AtomicWriteOnFailure(t *testing.T) {
	dir := t.TempDir()
	// First write a baseline.
	Sync(dir, defaultOpts())
	original, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	// Make the project root read-only so atomicWrite's MkdirAll/Rename fails.
	if err := os.Chmod(dir, 0o555); err != nil { //nolint:gosec // test-only readonly
		t.Skipf("chmod restricted on this fs: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:gosec // test-only restore
	// Trigger an update by changing the serverURL.
	r := Sync(dir, Options{ServerURL: "http://example.com:9999", ProjectSlug: "specgraph"})[0]
	if r.Action != ActionError {
		// The MkdirAll on the parent might succeed if it already exists; on
		// some filesystems chmod 0o555 still allows existing-file writes via
		// rename. Skip rather than false-fail.
		t.Skipf("filesystem permitted write under 0o555 dir; cannot exercise atomic-rename failure: action = %q", r.Action)
	}
	cur, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Equal(original, cur) {
		t.Errorf("AGENTS.md modified despite write failure")
	}
}

func TestSync_ConcurrentInvocations(t *testing.T) {
	dir := t.TempDir()
	const N = 4
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			Sync(dir, defaultOpts())
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	// Exactly one start marker and one end marker.
	if c := strings.Count(bs, initStart); c != 1 {
		t.Errorf("init start marker count = %d, want 1; concurrent runs interleaved\n%s", c, bs)
	}
	if c := strings.Count(bs, initEnd); c != 1 {
		t.Errorf("init end marker count = %d, want 1; concurrent runs interleaved\n%s", c, bs)
	}
}
````

(Top of `sync_test.go` needs `import "bytes"` for the new test; add if not yet present.)

- [ ] **Step 24: Run the full test suite**

Run: `go test ./internal/config/pointers/ -race -v`
Expected: PASS. The atomic-write test may `Skip` on some filesystems — that's acceptable.

- [ ] **Step 25: Commit and run `task check`**

```bash
jj describe -m "test(pointers): cover failure isolation, atomic write, concurrency

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

Then run `task check`. Expected: PASS. Fix any license/lint complaints inline.

---

## Task 2: Wire `pointers.Sync` into `specgraph init`

**Files:**

- Modify: `cmd/specgraph/init.go`
- Modify: `cmd/specgraph/init_test.go`

- [ ] **Step 1: Write failing init integration tests**

Find the existing tests in `cmd/specgraph/init_test.go` and append:

````go
func TestInit_FreshProject_WritesPointers(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv("SPECGRAPH_SERVER_URL", "http://127.0.0.1:7890")
	if err := runInit(newCmdWithCtx(), []string{"specgraph"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	for _, p := range []string{"AGENTS.md", ".cursor/rules/specgraph-bootstrap.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestInit_RerunIsNoOp(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv("SPECGRAPH_SERVER_URL", "http://127.0.0.1:7890")
	if err := runInit(newCmdWithCtx(), []string{"specgraph"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	// Capture file mtimes.
	files := []string{
		".mcp.json", ".cursor/mcp.json", "opencode.json",
		"AGENTS.md", ".cursor/rules/specgraph-bootstrap.md",
	}
	mt0 := map[string]time.Time{}
	for _, f := range files {
		fi, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("stat %s: %v", f, err)
		}
		mt0[f] = fi.ModTime()
	}
	// Sleep to make any rewrite detectable via mtime.
	time.Sleep(10 * time.Millisecond)
	if err := runInit(newCmdWithCtx(), []string{"specgraph"}); err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	for _, f := range files {
		fi, _ := os.Stat(filepath.Join(dir, f))
		if !fi.ModTime().Equal(mt0[f]) {
			t.Errorf("%s mtime changed; second run should be no-op", f)
		}
	}
}

func TestInit_PurgesLegacyInjectArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv("SPECGRAPH_SERVER_URL", "http://127.0.0.1:7890")
	seed := "# my AGENTS\n" +
		"<!-- specgraph:foo:start -->\nA\n<!-- specgraph:foo:end -->\n" +
		"<!-- specgraph:My.spec_v2:start -->\nB\n<!-- specgraph:My.spec_v2:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := runInit(newCmdWithCtx(), []string{"specgraph"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	bs := string(body)
	if strings.Contains(bs, "specgraph:foo:") || strings.Contains(bs, "specgraph:My.spec_v2:") {
		t.Errorf("legacy markers not purged:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}
````

If `chdir` and `newCmdWithCtx` don't already exist as helpers, search the file for similar test helpers and adapt. If absent, define minimal versions:

````go
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func newCmdWithCtx() *cobra.Command {
	c := &cobra.Command{}
	c.SetContext(context.Background())
	return c
}
````

(Add `import` lines for `context`, `time`, and `github.com/spf13/cobra` if missing.)

- [ ] **Step 2: Run the new tests, see them fail**

Run: `go test ./cmd/specgraph/ -run 'TestInit_(FreshProject|RerunIsNoOp|PurgesLegacy)' -v`
Expected: FAIL — `runInit` doesn't write the new pointer files yet.

- [ ] **Step 3: Modify `runInit` to call `pointers.Sync`**

In `cmd/specgraph/init.go`, after the existing `mcpconfigs.Sync` block (around the current line 120), add:

````go
	// Pointer files (AGENTS.md, .cursor/rules/specgraph-bootstrap.md).
	// Run only after mcpconfigs succeeded; per-file errors don't abort the
	// pointer phase but do produce a non-zero exit.
	pointerResults := pointers.Sync(cwd, pointers.Options{
		ServerURL:   serverURL,
		ProjectSlug: pc.Slug,
	})
	pointerErr := false
	for _, r := range pointerResults {
		switch r.Action {
		case pointers.ActionError:
			fmt.Printf("%s: error: %v\n", r.Path, r.Err)
			pointerErr = true
		default:
			line := fmt.Sprintf("%s: %s", r.Path, r.Action)
			if r.LegacyBlocksPurged > 0 {
				line += fmt.Sprintf(" (purged %d legacy blocks)", r.LegacyBlocksPurged)
			}
			fmt.Println(line)
		}
	}
	if pointerErr {
		return fmt.Errorf("one or more pointer files failed to sync")
	}
````

Add the import: `"github.com/specgraph/specgraph/internal/config/pointers"`.

- [ ] **Step 4: Run the integration tests, verify they pass**

Run: `go test ./cmd/specgraph/ -run 'TestInit_(FreshProject|RerunIsNoOp|PurgesLegacy)' -v`
Expected: PASS.

- [ ] **Step 5: Run the full init test suite to catch regressions**

Run: `go test ./cmd/specgraph/ -run TestInit -v`
Expected: PASS.

- [ ] **Step 6: Run `task check`**

Run: `task check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj describe -m "feat(init): write AGENTS.md and cursor bootstrap pointers via pointers.Sync

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 3: Reserve `Inject` in proto and regenerate

**Files:**

- Modify: `proto/specgraph/v1/sync.proto`
- Regenerate: `gen/specgraph/v1/sync.pb.go`, `gen/specgraph/v1/specgraphv1connect/sync.connect.go`, `web/src/lib/api/gen/specgraph/v1/sync_pb.ts`

- [ ] **Step 1: Edit `proto/specgraph/v1/sync.proto`**

Replace the `enum InjectTool` block (lines 28–33) with a reserved-numbers stub:

````proto
// Reserved: enum InjectTool removed when specgraph inject was deprecated.
// (Field numbers within the deleted enum are not part of the proto wire
// format, but we keep this comment as a paper trail.)
````

Replace the `InjectRequest` and `InjectResponse` messages (lines 90–100) with reservations:

````proto
// Reserved: InjectRequest and InjectResponse removed when specgraph inject
// was deprecated. See docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-design.md.
````

In the `service SyncService` block, remove the `rpc Inject(...)` line and add a `reserved` field-number declaration. Per Protobuf 3 rules, services don't reserve method names the way messages reserve fields; the reservation here is documentary only:

````proto
service SyncService {
  rpc SyncBeads(SyncBeadsRequest) returns (SyncResponse);
  rpc SyncGitHub(SyncGitHubRequest) returns (SyncResponse);
  rpc GetSyncStatus(SyncStatusRequest) returns (SyncStatusResponse);
  // rpc Inject(InjectRequest) returns (InjectResponse);  // removed 2026-05-06
}
````

- [ ] **Step 2: Regenerate proto**

Run: `task proto`
Expected: completes without errors. The skill `buf-regen/SKILL.md` documents this — invoke it via Read if anything goes wrong.

- [ ] **Step 3: Sanity-check the regen**

Run:

```bash
rg -l '\bInjectRequest\b|\bInjectResponse\b|\bInjectTool\b' gen/ web/src/lib/api/gen/
```

Expected: zero matches in `gen/` and `web/src/lib/api/gen/`.

- [ ] **Step 4: `go build ./...` to find every caller that's now broken**

Run: `go build ./...`
Expected: errors in `internal/server/sync_handler.go` (uses `specv1.InjectRequest` etc.), `internal/server/convert_constitution.go` (`specv1.InjectTool_*`), `internal/auth/permissions.go` (`SyncServiceInjectProcedure`), `cmd/specgraph/inject.go`, and any `_test.go` referencing those symbols.

These errors are **expected** — Tasks 4 and 5 fix them. Don't try to make this commit compile in isolation.

- [ ] **Step 5: Commit the proto + regen as a snapshot**

```bash
jj describe -m "chore(proto): reserve Inject* — Inject RPC and messages removed

Build is intentionally broken at this commit; subsequent commits remove the
in-tree callers. See spgr-yyjf design.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

(If you prefer to keep main green at every commit, do Tasks 3, 4, and 5 as a single atomic change instead. The plan's preference is one commit per task for review readability.)

---

## Task 4: Delete `Inject` server-side

**Files:**

- Modify: `internal/server/sync_handler.go`
- Modify: `internal/server/sync_handler_test.go`
- Modify: `internal/server/convert_constitution.go`
- Modify: `internal/server/convert_constitution_test.go`
- Modify: `internal/server/convert_test.go` (only if a `TestInjectToolFromProto` exists)
- Modify: `internal/storage/sync.go`
- Modify: `internal/auth/permissions.go`
- Modify: `internal/auth/permissions_test.go`

- [ ] **Step 1: Delete the `Inject` handler method**

In `internal/server/sync_handler.go`, delete the entire `func (h *SyncHandler) Inject(...) ...` method (begins around line 243; ends with the closing brace of its function body). Also remove the `"github.com/specgraph/specgraph/internal/inject"` import line at the top of the file.

- [ ] **Step 2: Delete the inject server tests**

In `internal/server/sync_handler_test.go`, delete every `func TestSyncHandler_Inject_*`. There are 11 of them in the working tree on 2026-05-06; use this command to find them and delete each function block:

```bash
rg -n '^func TestSyncHandler_Inject_' internal/server/sync_handler_test.go
```

Keep the unrelated `errorSyncBackend` comment that mentions "inject errors" (different sense of inject) and `TestInjectProject` (unrelated test scoping helper).

- [ ] **Step 3: Delete `injectToolFromProto`**

In `internal/server/convert_constitution.go`, delete the entire `func injectToolFromProto(...) ...` block (around line 267).

- [ ] **Step 4: Delete the converter tests**

In `internal/server/convert_constitution_test.go`, delete the `TestInjectToolFromProto*` functions. Verify with:

```bash
rg -n 'InjectToolFromProto|injectToolFromProto' internal/server/
```

Expected after edits: zero matches.

- [ ] **Step 5: Delete `InjectToolType` and constants from storage**

In `internal/storage/sync.go`, delete lines 31–38 (the `InjectToolType` declaration and the three constants). If anything in the file uses `storage.InjectToolType` after this delete, it's a missed caller and Step 7 will catch it.

- [ ] **Step 6: Delete the auth permission entry and test**

In `internal/auth/permissions.go`, delete the line `specgraphv1connect.SyncServiceInjectProcedure: "sync:write",` (around line 70).

In `internal/auth/permissions_test.go`, delete the line referencing `specgraphv1connect.SyncServiceInjectProcedure` (around line 72).

- [ ] **Step 7: `go build ./...` to verify the server side compiles**

Run: `go build ./...`
Expected: still fails on `cmd/specgraph/inject.go` (next task), but **no** failures in `internal/server/`, `internal/storage/`, or `internal/auth/`.

If `go build` reports failures elsewhere — e.g., a test file or a file under `cmd/specgraph/` other than `inject.go` — it's a missed caller. Find and fix it inline.

- [ ] **Step 8: Run the server tests**

Run: `go test ./internal/server/ ./internal/storage/ ./internal/auth/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
jj describe -m "refactor: remove Inject RPC handler, converter, storage type, auth permission

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 5: Delete the `inject` CLI subcommand and `internal/inject/` package

**Files:**

- Delete: `cmd/specgraph/inject.go`
- Modify: `cmd/specgraph/sync_test.go`
- Modify: `cmd/specgraph/docs.go`
- Delete: `internal/inject/inject.go`
- Delete: `internal/inject/inject_test.go`
- Delete: `internal/inject/lock_unix.go`
- Delete: `internal/inject/lock_windows.go`

- [ ] **Step 1: Delete `cmd/specgraph/inject.go`**

```bash
rm cmd/specgraph/inject.go
```

- [ ] **Step 2: Delete the inject CLI tests from `cmd/specgraph/sync_test.go`**

In `cmd/specgraph/sync_test.go`, delete the four functions `TestInjectCmd_Flags`, `TestInjectCmd_RequiresSlug`, `TestInjectCmd_AcceptsSlug`, and `TestInjectCmd_ToolAliases`. Verify nothing else in the file references them:

```bash
rg -n '\b(injectCmd|runInject|injectTool|injectOutput|TestInjectCmd_)\b' cmd/specgraph/
```

Expected: zero matches after edits.

- [ ] **Step 3: Drop `"inject"` from the command list in `cmd/specgraph/docs.go`**

At line 57, the entry currently reads:

````go
{Name: "Server & Config", Commands: []string{"up", "down", "install", "uninstall", "serve", "status", "health", "init", "prime", "inject", "read-mcp-resource"}},
````

Remove the `"inject",` element so the line becomes:

````go
{Name: "Server & Config", Commands: []string{"up", "down", "install", "uninstall", "serve", "status", "health", "init", "prime", "read-mcp-resource"}},
````

- [ ] **Step 4: Delete the `internal/inject/` package**

```bash
rm -rf internal/inject/
```

- [ ] **Step 5: `go build ./...`**

Run: `go build ./...`
Expected: PASS. If anything still references `internal/inject`, `injectCmd`, or `runInject`, it's a missed caller — fix it inline.

- [ ] **Step 6: Run the full unit test suite**

Run: `go test ./...`
Expected: PASS (excluding postgres integration / Docker-required suites, per `task test` semantics).

- [ ] **Step 7: Run `task check`**

Run: `task check`
Expected: PASS. Common failures at this point: unused imports left behind from earlier deletes (gofmt and goimports catch them), license-header missing on new files (run `task license:add`).

- [ ] **Step 8: Commit**

```bash
jj describe -m "refactor: remove inject CLI subcommand and internal/inject package

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 6: Update documentation

**Files:**

- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `site/docs/architecture.md`
- Modify: `site/docs/cli-reference.md`
- Modify: `site/docs/ecosystem.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Update `README.md`**

In the "Sync & Injection" command-table section (around lines 113–115), delete the inject row and rename the section heading to "Sync":

Search for:

```text
| **Sync & Injection** | |
| --- | --- |
| `specgraph inject` | Inject spec context into tool files (.claude/specs, .cursor/rules, AGENTS.md) |
```

Replace with the inject row removed and the heading renamed:

```text
| **Sync** | |
| --- | --- |
```

(Adjust to actual table structure if it differs; the goal is: delete the inject row.)

- [ ] **Step 2: Update `CLAUDE.md`**

Find line 62 (the architecture table row for `internal/inject/`):

```text
| `internal/inject/` | Tool injection (.claude/specs, .cursor/rules, AGENTS.md) with file locking |
```

Replace with:

```text
| `internal/config/pointers/` | Init-managed pointer files (AGENTS.md, .cursor/rules/specgraph-bootstrap.md) with managed-block fencing and file locking |
```

Search the rest of `CLAUDE.md` for any `inject` references in Gotchas or elsewhere. Remove or rephrase as appropriate.

- [ ] **Step 3: Update `site/docs/architecture.md`**

- Around line 30, remove the Mermaid node:

  ```text
  Inject["Tool Injection<br/>(.claude/specs, .cursor/rules, AGENTS.md)"]
  ```

  And any edges that refer to it.

- Around line 56, edit the SyncService row in the capability table. Current:

  ```text
  | **SyncService** | Push specs to external systems (Beads, GitHub) and inject context into tool files. |
  ```

  Change to:

  ```text
  | **SyncService** | Push specs to external systems (Beads, GitHub). |
  ```

- Around line 211, in the directory tree:

  ```text
  │   ├── inject/             # Tool injection (.claude/specs, .cursor/rules, AGENTS.md)
  ```

  Replace with:

  ```text
  │   ├── config/pointers/    # Init-managed pointer files (AGENTS.md, .cursor/rules/specgraph-bootstrap.md)
  ```

- [ ] **Step 4: Update `site/docs/cli-reference.md`**

Around line 954, delete the entire `### specgraph inject` section, including the synopsis block, all flags, and any examples that follow until the next `###` heading.

- [ ] **Step 5: Update `site/docs/ecosystem.md`**

- Around line 82, delete the bullet:

  ```text
  - **Tool Injection** — `specgraph inject <slug>` writes a spec's interface
  ```

  along with any continuation paragraph.

- Around line 125, locate:

  ```text
  Add tool injection for coding-agent context. No combination is degraded —
  ```

  and rephrase to drop the inject framing. A reasonable replacement:

  ```text
  Wire your harness via `specgraph init`; the MCP integration provides spec
  and constitution resources directly. No combination is degraded —
  ```

- [ ] **Step 6: Add a CHANGELOG entry**

In `CHANGELOG.md`, under the next unreleased version (or create an `## Unreleased` section if none exists), add:

````markdown
## Unreleased

### Removed

- `specgraph inject` CLI subcommand and the `Inject` ConnectRPC method are
  removed. Use `specgraph init` to wire harness configs (.mcp.json,
  .cursor/mcp.json, opencode.json) and pointer files (AGENTS.md,
  .cursor/rules/specgraph-bootstrap.md). Spec content is served live via the
  MCP `specgraph://spec/{slug}` resource.

### Migration

- `specgraph init` automatically purges legacy per-slug blocks
  (`<!-- specgraph:<slug>:start -->` / `<!-- specgraph:<slug>:end -->`) from
  AGENTS.md on next run. The number purged is reported in the init output.
- Orphan files under `.claude/specs/` and per-slug files under
  `.cursor/rules/specgraph-<slug>.md` are **not** removed automatically.
  Delete them manually if desired:

  ```bash
  rm -rf .claude/specs
  rm -f .cursor/rules/specgraph-*.md   # but keep specgraph.md (plugin-shipped)
                                       # and specgraph-bootstrap.md (init-managed)
  ```
````

- [ ] **Step 7: Run `task lint`**

Run: `task lint`
Expected: PASS. Fix any markdown line-length, link, or formatting complaints.

- [ ] **Step 8: Verify no stale `inject` references remain in docs that should be edited**

Run:

```bash
rg -l '\bspecgraph inject\b' README.md CLAUDE.md AGENTS.md site/ CHANGELOG.md
```

Expected: zero matches **except** CHANGELOG.md (which intentionally describes the removal).

For docs under `docs/plans/2026-02-*`, `docs/plans/2026-03-*`, `docs/plans/2026-04-*`, `docs/superpowers/`, references are historical and **stay as-is** per the design's "Bead/CLI references in older plan docs" note.

- [ ] **Step 9: Commit**

```bash
jj describe -m "docs: remove specgraph inject from README, CLAUDE.md, site, add CHANGELOG entry

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"
```

---

## Task 7: Manual smoke test, full pipeline, push, PR

**Files:** none (verification + push)

- [ ] **Step 1: Build a fresh binary**

Run: `task build`
Expected: PASS.

- [ ] **Step 2: Run the manual smoke test on a scratch project**

Set up a scratch project with seeded inject artifacts:

```bash
mkdir -p /tmp/spgr-yyjf-smoke && cd /tmp/spgr-yyjf-smoke
cat > AGENTS.md <<'EOF'
# Scratch project

User content above.

<!-- specgraph:foo:start -->
stale digest A
<!-- specgraph:foo:end -->

<!-- specgraph:My.spec_v2:start -->
stale digest B
<!-- specgraph:My.spec_v2:end -->

User content below.
EOF
mkdir -p .claude/specs && echo "# orphan" > .claude/specs/foo.md
mkdir -p .cursor/rules && echo "# orphan" > .cursor/rules/specgraph-foo.md
```

Run: `<repo-root>/dist/specgraph init`
Expected output includes:

- `.mcp.json: created`
- `.cursor/mcp.json: created`
- `opencode.json: created`
- `AGENTS.md: updated (purged 2 legacy blocks)`
- `.cursor/rules/specgraph-bootstrap.md: created`

Verify:

- `cat AGENTS.md` shows the new init block with v=1 marker, no `specgraph:foo:` or `specgraph:My.spec_v2:` markers, "User content above." and "User content below." preserved.
- `cat .cursor/rules/specgraph-bootstrap.md` shows frontmatter with `alwaysApply: true` and the init block.
- `.claude/specs/foo.md` and `.cursor/rules/specgraph-foo.md` still exist (we don't touch orphans).
- Re-run `<repo-root>/dist/specgraph init`. Expected: every file reports `no-op`, including AGENTS.md (no legacy blocks left to purge).
- Edit a managed block manually (insert garbage between the markers) and re-run init. Expected: AGENTS.md reports `updated`, the garbage is gone, surrounding user content untouched.

- [ ] **Step 3: Run `task pr-prep` (full pipeline including integration + e2e)**

Run: `task pr-prep`
Expected: PASS. Requires Docker.

- [ ] **Step 4: Push and create PR**

```bash
gh auth switch -u seanb4t -h github.com
jj bookmark set spgr-yyjf-deprecate-inject -r @
jj git push --bookmark spgr-yyjf-deprecate-inject
```

Then create the PR:

````bash
gh pr create --title "spgr-yyjf: deprecate specgraph inject in favor of MCP + extended init" --body "$(cat <<'EOF'
## Summary

- Replaces `specgraph inject` with two pointer files (`AGENTS.md`,
  `.cursor/rules/specgraph-bootstrap.md`) written by an extended
  `specgraph init`.
- Deletes the inject CLI subcommand, the `Inject` ConnectRPC method, the
  `internal/inject/` package, the `storage.InjectToolType` enum, the
  `injectToolFromProto` converter, the `SyncServiceInjectProcedure`
  permission, and every test that exercises any of those.
- Active-purge legacy `<!-- specgraph:<slug>:start --> ... :end -->` blocks
  from `AGENTS.md` on first init run. Orphan files under `.claude/specs/`
  and per-slug `.cursor/rules/specgraph-<slug>.md` are flagged in CHANGELOG
  but not auto-deleted.

## Design

- [docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-design.md](docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-design.md)
- [docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-plan.md](docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-plan.md)

## Test plan

- [x] `task check`
- [x] `task pr-prep`
- [x] Manual smoke test on a scratch project with seeded legacy inject blocks
      (purge counted, user content preserved, orphan files untouched, re-run
      is no-op, manual block edit gets restored).

EOF
)"
````

- [ ] **Step 5: Verify CI passes**

Wait for CI checks. If CodeRabbit or DCO fail, fix and push (do not amend a pushed commit; create a new commit).

- [ ] **Step 6: Update bead `spgr-yyjf` to in-review**

```bash
bd update spgr-yyjf --status in-review
```

---

## Self-review

(Run yourself. Fix issues inline. No subagent dispatch.)

**1. Spec coverage:** Walk each section of the design doc and confirm a task implements it.

| Design section | Task |
|---|---|
| §Architecture: `internal/config/pointers/` package | T1 |
| §Architecture: marker contract and v=1 versioning | T1 (steps 8, 14) |
| §Architecture: legacy block purge | T1 (steps 8, 11) |
| §Architecture: cursor frontmatter handling | T1 (steps 17–22) |
| §Architecture: init flow (mcpconfigs first, pointers second, conditional) | T2 |
| §Behavior: idempotency contract (six states) | T1 (steps 6, 11, 14, 17), T2 |
| §Behavior: phase order (validate → purge → reconcile → write) | T1 step 8 (`syncAgents`) |
| §Behavior: corruption rules (4) | T1 (steps 14, 19) |
| §Behavior: failure isolation | T1 (step 23), T2 |
| §Behavior: file locking (Unix flock + Windows no-op) | T1 (step 3) |
| §Behavior: inject removal (concrete deletion list) | T3 (proto), T4 (server), T5 (CLI/package) |
| §Testing: unit tests for pointers | T1 |
| §Testing: integration tests in init_test.go | T2 |
| §Testing: e2e + proto verification | T3, T7 |
| §Testing: manual smoke test | T7 |

No gaps.

**2. Placeholder scan:** Search for these red flags:

```bash
rg -n 'TBD|TODO|implement later|fill in details|appropriate error|handle edge cases|similar to' docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-plan.md
```

Expected: zero matches that aren't in code blocks reproducing existing project conventions. Fix any that show up.

**3. Type consistency:** Check that `Action`, `SyncResult`, `Options`, `Sync` are spelled the same everywhere they appear. Specifically:

- `Action` is `string`-typed, with constants `ActionCreated`, `ActionUpdated`, `ActionNoOp`, `ActionError`.
- `SyncResult` fields are `Path` (string), `Action`, `Err` (error), `LegacyBlocksPurged` (int).
- `Options` fields are `ServerURL` (string), `ProjectSlug` (string).
- `Sync(projectDir string, opts Options) []SyncResult` — no error return.

If any task draft used `File` instead of `Path` or `ProjectID` instead of `ProjectSlug`, fix inline. (Verified: clean.)

---

## Execution Handoff

Plan complete and saved to `docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
