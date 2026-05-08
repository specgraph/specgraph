# PR #940 Review-Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address every Critical, Important, and Suggestion-tier finding from the multi-agent review of PR #940 (`spgr-yyjf-deprecate-inject`), so the `internal/config/pointers/` package and its `cmd/specgraph/init` caller are durable, race-tolerant, well-typed, and well-tested.

**Architecture:** Phased TDD. Phase 1 hardens the existing `atomicWrite` primitive (the single most-relied-upon function). Phase 2 refactors the API shape (`SyncReport` struct, `NewOptions` constructor, sentinel errors) so subsequent test additions land on the final API. Phase 3 fixes the file-lock contract on both platforms. Phase 4 closes the symlink-TOCTOU window. Phase 5 fills behavioral test gaps. Phase 6 fixes the init-caller hygiene findings. Phase 7 fixes comment rot. Phase 8 lands hygiene + low-priority suggestions.

**Tech Stack:** Go 1.23, `pgx` v5 unrelated, `protoc-gen-go` for regen (none touched here), `golang.org/x/sys/windows` (added in Phase 3), `jj` for VCS, Taskfile for `task check` quality gate.

**Branch context:** Working in jj workspace `deprecate-inject` at `~/Code/github.com/specgraph-deprecate-inject`, descended from bookmark `spgr-yyjf-deprecate-inject` (PR #940). Each phase produces one or more commits via `jj commit -m`.

**Commit-message convention:** Conventional commits + DCO sign-off (`Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>`). The pre-commit hooks enforce both.

---

## Phase 0: Setup & Issue Tracking

### Task 0.1: Pull latest beads issue store and create tracking issue

**Files:** none (operates on `.beads/`)

- [ ] **Step 1:** Pull beads updates so issue numbering is current.

Run: `bd dolt pull`

- [ ] **Step 2:** Create the umbrella issue.

Run:

```bash
bd create \
  --title="PR #940 review fixes (criticals + importants + suggestions)" \
  --description="Addresses all findings from the 5-agent review of PR #940 (spgr-yyjf-deprecate-inject). See docs/plans/2026-05-07-pr940-review-fixes-plan.md for the breakdown." \
  --type=task \
  --priority=1
```

- [ ] **Step 3:** Claim it.

Run: `bd update <id> --claim` (use the id printed by step 2).

---

## Phase 1: `atomicWrite` Hardening

Closes Critical findings 1 (fsync), 4 (errors.Join cleanup), 6 (close-error in failure paths) and Test-Critical 6 (file-mode preservation).

### Task 1.1: Test that an existing file's permissions survive a managed update

**Files:**

- Test: `internal/config/pointers/sync_test.go` (append)

- [ ] **Step 1: Write the failing test.**

Append to `internal/config/pointers/sync_test.go`:

```go
func TestSync_PreservesExistingFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	dir := t.TempDir()
	// Seed AGENTS.md with mode 0644 (typical user file).
	full := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(full, []byte("# user content\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(full, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	results := Sync(dir, defaultOpts())
	if results[0].Action != ActionUpdated {
		t.Fatalf("results[0].Action = %v, want %v (err=%v)", results[0].Action, ActionUpdated, results[0].Err)
	}

	info, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode after Sync = %o, want 0644 (existing mode preserved)", got)
	}
}
```

- [ ] **Step 2: Run test, expect fail.**

Run: `cd ~/Code/github.com/specgraph-deprecate-inject && go test ./internal/config/pointers/ -run TestSync_PreservesExistingFileMode -v`

Expected: FAIL with `file mode after Sync = 600, want 0644`.

- [ ] **Step 3: Modify `atomicWrite` to accept an explicit mode.**

Edit `internal/config/pointers/sync.go` lines 101-132. Replace the function:

```go
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
	// fsync the directory so the rename is durable. Best-effort: a failure
	// here cannot be undone but does not corrupt the file.
	if dirF, derr := os.Open(dir); derr == nil {
		_ = dirF.Sync()
		_ = dirF.Close()
	}
	return nil
}
```

- [ ] **Step 4: Update callers in `agents.go` and `cursor.go`.**

In `internal/config/pointers/agents.go`, replace line 101:

```go
		mode := os.FileMode(0o600)
		if info, statErr := os.Stat(full); statErr == nil {
			mode = info.Mode().Perm()
		}
		if werr := atomicWrite(full, updated, mode); werr != nil {
			return errResult(agentsRel, werr)
		}
```

In `internal/config/pointers/cursor.go`, replace lines 52-54 (fresh-create branch):

```go
		if werr := atomicWrite(full, out, 0o600); werr != nil {
			return errResult(cursorRel, werr)
		}
```

And replace lines 96-98 (update branch):

```go
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if werr := atomicWrite(full, updated, mode); werr != nil {
		return errResult(cursorRel, werr)
	}
```

- [ ] **Step 5: Run targeted test, expect pass.**

Run: `go test ./internal/config/pointers/ -run TestSync_PreservesExistingFileMode -v`

Expected: PASS.

- [ ] **Step 6: Run full pointers test suite, confirm no regressions.**

Run: `go test ./internal/config/pointers/ -v`

Expected: All previously-passing tests still pass.

- [ ] **Step 7: Commit.**

Run:

```bash
jj commit -m "$(cat <<'EOF'
fix(pointers): harden atomicWrite with fsync, errors.Join, mode preservation

- fsync the temp file before close and the parent directory after
  rename so the on-disk state survives a power loss.
- Compose all cleanup errors with errors.Join instead of dropping
  via //nolint:errcheck so stranded-tempfile signals reach the caller.
- Take an explicit os.FileMode parameter; agents.go and cursor.go
  read the existing file's mode and pass it through. Eliminates the
  silent 0644 -> 0600 mode change on update paths.

Closes review findings: Critical-1 (fsync), Critical-4 (errors.Join),
Critical-6 (close-error in failure paths), Test-Critical-6 (mode).

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 2: Type-Design Refactor

Closes Important findings 15 (`Options` validation), 16 (`SyncReport` struct), 17 (`ActionError` ↔ `Err` invariant) and unblocks subsequent phases by landing the final API shape early.

### Task 2.1: Add `NewOptions` constructor with validation

**Files:**

- Modify: `internal/config/pointers/sync.go`
- Test: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Write failing tests.**

Append to `internal/config/pointers/sync_test.go`:

```go
func TestNewOptions_RejectsBadServerURL(t *testing.T) {
	cases := []struct {
		name, url string
	}{
		{"empty", ""},
		{"relative", "/api"},
		{"hostname only", "example.com"},
		{"host:port no scheme", "localhost:3000"},
		{"non-http scheme", "ftp://example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewOptions(c.url, "specgraph")
			if err == nil {
				t.Fatalf("NewOptions(%q) returned nil error; want validation failure", c.url)
			}
		})
	}
}

func TestNewOptions_RejectsBadSlug(t *testing.T) {
	cases := []struct {
		name, slug string
	}{
		{"empty", ""},
		{"leading dot", ".secret"},
		{"contains slash", "foo/bar"},
		{"contains newline", "foo\nbar"},
		{"contains marker tail", "foo --> <!-- specgraph:init:end -->"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewOptions("http://127.0.0.1:7890", c.slug)
			if err == nil {
				t.Fatalf("NewOptions(slug=%q) returned nil error", c.slug)
			}
		})
	}
}

func TestNewOptions_AcceptsValidInputs(t *testing.T) {
	opts, err := NewOptions("https://specgraph.example.com:443", "my_proj.v2")
	if err != nil {
		t.Fatalf("NewOptions: %v", err)
	}
	if opts.ServerURL != "https://specgraph.example.com:443" {
		t.Errorf("ServerURL round-trip failed: got %q", opts.ServerURL)
	}
	if opts.ProjectSlug != "my_proj.v2" {
		t.Errorf("ProjectSlug round-trip failed: got %q", opts.ProjectSlug)
	}
}
```

- [ ] **Step 2: Run, expect fail.**

Run: `go test ./internal/config/pointers/ -run TestNewOptions -v`

Expected: FAIL with `undefined: NewOptions`.

- [ ] **Step 3: Implement `NewOptions`.**

In `internal/config/pointers/sync.go`, replace the existing `Options` doc comment block (lines 36-41) and add a constructor + validator:

```go
// Options carries the canonical values that init derives once and threads
// into the pointer templates. Construct via NewOptions to validate inputs.
type Options struct {
	ServerURL   string
	ProjectSlug string
}

// safeSlugPattern mirrors the inject-era slug class:
// `[a-zA-Z0-9][a-zA-Z0-9._-]*`. Slugs flow into both the X-Specgraph-Project
// header text and the rendered managed block; rejecting newlines and
// marker-shaped fragments keeps the block syntactically inviolable.
var safeSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// NewOptions validates serverURL and projectSlug then returns a usable
// Options. serverURL must be an absolute http or https URL with a non-empty
// host. projectSlug must match the inject-era slug pattern.
func NewOptions(serverURL, projectSlug string) (Options, error) {
	parsed, perr := url.Parse(serverURL)
	if perr != nil {
		return Options{}, fmt.Errorf("server URL %q: %w", serverURL, perr)
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Options{}, fmt.Errorf("server URL %q must be an absolute http or https URL", serverURL)
	}
	if !safeSlugPattern.MatchString(projectSlug) {
		return Options{}, fmt.Errorf("project slug %q does not match %s", projectSlug, safeSlugPattern)
	}
	return Options{ServerURL: serverURL, ProjectSlug: projectSlug}, nil
}
```

Add `"net/url"` and `"regexp"` to the import block at the top of `sync.go` (sorted).

- [ ] **Step 4: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -run TestNewOptions -v`

Expected: PASS (all 12 subtests).

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
feat(pointers): add NewOptions constructor with URL + slug validation

Options now has a validating constructor that rejects malformed server
URLs (relative paths, hostname-only, host:port without scheme,
non-http(s) schemes) and slugs that don't match the inject-era
[a-zA-Z0-9][a-zA-Z0-9._-]* pattern. Slug validation prevents newline
or marker-tail injection into the rendered managed block.

Closes review finding Important-15.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 2.2: Replace `[]SyncResult` with `SyncReport` struct

**Files:**

- Modify: `internal/config/pointers/sync.go`, `agents.go`, `cursor.go`, `sync_test.go`
- Modify: `cmd/specgraph/init.go`, `init_test.go`

- [ ] **Step 1: Write a failing test for the new shape.**

Append to `internal/config/pointers/sync_test.go`:

```go
func TestSync_ReturnsSyncReportStruct(t *testing.T) {
	dir := t.TempDir()
	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionCreated {
		t.Errorf("report.Agents.Action = %v, want %v", report.Agents.Action, ActionCreated)
	}
	if report.Cursor.Action != ActionCreated {
		t.Errorf("report.Cursor.Action = %v, want %v", report.Cursor.Action, ActionCreated)
	}
}
```

- [ ] **Step 2: Run, expect compile failure.**

Run: `go test ./internal/config/pointers/ -run TestSync_ReturnsSyncReportStruct`

Expected: build error (`Sync returns []SyncResult, not struct`).

- [ ] **Step 3: Define `SyncReport` and change `Sync`'s signature.**

In `internal/config/pointers/sync.go`, replace `Sync` and surrounding doc:

```go
// SyncReport is the per-file outcome of a Sync. The two fields are
// always populated except in the projectDir-level early-error case where
// Agents carries the projectDir error and Cursor is the zero value.
type SyncReport struct {
	Agents SyncResult
	Cursor SyncResult
}

// IsErr reports whether either pointer file failed.
func (r SyncReport) IsErr() bool {
	return r.Agents.Action == ActionError || r.Cursor.Action == ActionError
}

// Sync reconciles all pointer files for the project. Returns a SyncReport
// with one SyncResult per pointer file.
//
// If projectDir is missing, is not a directory, or is itself a symlink,
// Sync short-circuits: report.Agents carries the projectDir-level error
// (Path == projectDir, Action == ActionError) and report.Cursor is the
// zero value.
//
// A failure on one pointer file is reported via the corresponding
// SyncResult.Err with Action == ActionError; the other file is still
// processed. This differs from mcpconfigs.Sync which aborts on first
// error. The init caller reconciles by running mcpconfigs first and only
// invoking pointers.Sync if mcpconfigs succeeded.
func Sync(projectDir string, opts Options) SyncReport {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		msg := fmt.Errorf("projectDir %q is not a directory", projectDir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Errorf("stat %s: %w", projectDir, err)
		}
		return SyncReport{Agents: errResult(projectDir, msg)}
	}
	li, lerr := os.Lstat(projectDir)
	if lerr != nil {
		return SyncReport{Agents: errResult(projectDir, fmt.Errorf("lstat %s: %w", projectDir, lerr))}
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return SyncReport{Agents: errResult(projectDir, fmt.Errorf("refusing to follow symlink %s", projectDir))}
	}
	return SyncReport{
		Agents: syncAgents(projectDir, opts),
		Cursor: syncCursor(projectDir, opts),
	}
}
```

(Note: this also fixes Important-14 — the `os.Lstat` error swallowing.)

- [ ] **Step 4: Update `cmd/specgraph/init.go` to consume `SyncReport`.**

Replace lines 133-153:

```go
	pointerReport := pointers.Sync(cwd, pointers.Options{
		ServerURL:   serverURL,
		ProjectSlug: pc.Slug,
	})
	var failedPaths []string
	for _, r := range []pointers.SyncResult{pointerReport.Agents, pointerReport.Cursor} {
		if r.Path == "" {
			continue // zero-value (projectDir-level error case)
		}
		switch r.Action {
		case pointers.ActionError:
			fmt.Fprintf(os.Stderr, "%s: error: %v\n", r.Path, r.Err)
			failedPaths = append(failedPaths, r.Path)
		default:
			line := fmt.Sprintf("%s: %s", r.Path, r.Action)
			if r.LegacyBlocksPurged > 0 {
				line += fmt.Sprintf(" (purged %d legacy blocks)", r.LegacyBlocksPurged)
			}
			fmt.Println(line)
		}
	}
	if len(failedPaths) > 0 {
		return fmt.Errorf("sync pointer files: %d failed: %s", len(failedPaths), strings.Join(failedPaths, ", "))
	}
```

(This change also addresses Important-10 — errors now go to stderr.)

- [ ] **Step 5: Update `internal/config/pointers/sync_test.go` references.**

Replace every `results := Sync(...)` / `results[0]` / `results[1]` / `len(results) != 2` usage with `report := Sync(...)` / `report.Agents` / `report.Cursor`. Specific edits — search and replace:

- `results := Sync(` → `report := Sync(`
- `results[0]` → `report.Agents`
- `results[1]` → `report.Cursor`
- Delete every `if len(results) != 2 { ... }` block (the type guarantees two results).

(Use the Edit tool with `replace_all=true` for each pattern, then visually scan for orphan `results[` and `len(results)` references.)

- [ ] **Step 6: Update `cmd/specgraph/init_test.go` references the same way if any tests call `pointers.Sync` directly.**

Run: `grep -n 'pointers.Sync' cmd/specgraph/init_test.go`. Update any direct callers.

- [ ] **Step 7: Run full `pointers` suite and `cmd/specgraph` suite.**

Run:

```bash
go test ./internal/config/pointers/ ./cmd/specgraph/ -v
```

Expected: all green.

- [ ] **Step 8: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
refactor(pointers): replace []SyncResult with SyncReport struct

Sync now returns a named SyncReport{Agents, Cursor} struct instead of
a positional []SyncResult slice. Callers stop indexing by [0]/[1]; the
two-file contract is encoded in the type. The projectDir-level error
case populates SyncReport.Agents and leaves SyncReport.Cursor zero.

Also surfaces an os.Lstat error on projectDir (previously swallowed
when lerr != nil) and routes init's per-file error output to stderr.

Closes review findings: Important-16 (struct), Important-14 (Lstat),
Important-10 (stderr).

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 2.3: Couple `ActionError` ↔ `Err` via constructors

**Files:**

- Modify: `internal/config/pointers/sync.go`
- Test: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Write a failing test that asserts the invariant.**

Append to `sync_test.go`:

```go
func TestSyncResult_ActionErrorImpliesNonNilErr(t *testing.T) {
	// Drive an error via a known-failing input.
	dir := t.TempDir()
	if err := os.Symlink("/nonexistent", filepath.Join(dir, "link-to-nowhere")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	report := Sync(filepath.Join(dir, "link-to-nowhere"), defaultOpts())
	if report.Agents.Action != ActionError {
		t.Fatalf("Action = %v, want ActionError", report.Agents.Action)
	}
	if report.Agents.Err == nil {
		t.Errorf("ActionError but Err == nil — invariant broken")
	}
}

func TestSyncResult_NonErrorImpliesNilErr(t *testing.T) {
	dir := t.TempDir()
	report := Sync(dir, defaultOpts())
	if report.Agents.Err != nil {
		t.Errorf("Agents.Err = %v on success path", report.Agents.Err)
	}
	if report.Cursor.Err != nil {
		t.Errorf("Cursor.Err = %v on success path", report.Cursor.Err)
	}
}
```

- [ ] **Step 2: Run.** Likely passes already if no bugs; we keep the tests as regression pins. Confirm:

Run: `go test ./internal/config/pointers/ -run TestSyncResult -v`

Expected: PASS.

- [ ] **Step 3: Add explicit constructor helpers and a validation comment.**

In `internal/config/pointers/sync.go`, replace `errResult` with three constructors and document the invariant on `SyncResult`:

```go
// SyncResult reports what Sync did to a single managed pointer file.
//
// Invariant: Action == ActionError ⇔ Err != nil. Constructed via
// errResult / okResult / noopResult; do not build SyncResult literals
// outside this package.
//
// LegacyBlocksPurged is the number of pre-init per-slug blocks removed
// from AGENTS.md. Always 0 for the cursor pointer file. Meaningful only
// when Action == ActionUpdated or ActionCreated.
type SyncResult struct {
	Path               string
	Action             Action
	Err                error
	LegacyBlocksPurged int
}

func errResult(path string, err error) SyncResult {
	return SyncResult{Path: path, Action: ActionError, Err: err}
}

func okResult(path string, action Action, purged int) SyncResult {
	return SyncResult{Path: path, Action: action, LegacyBlocksPurged: purged}
}

func noopResult(path string) SyncResult {
	return SyncResult{Path: path, Action: ActionNoOp}
}
```

- [ ] **Step 4: Replace inline `SyncResult{...}` literals in agents.go and cursor.go with the new constructors.**

In `internal/config/pointers/agents.go`:

- Line 98 `return SyncResult{Path: agentsRel, Action: ActionNoOp}` → `return noopResult(agentsRel)`
- Line 109 `return SyncResult{Path: agentsRel, Action: action, LegacyBlocksPurged: purged}` → `return okResult(agentsRel, action, purged)`

In `internal/config/pointers/cursor.go`:

- Line 55 `return SyncResult{Path: cursorRel, Action: ActionCreated}` → `return okResult(cursorRel, ActionCreated, 0)`
- Line 93 `return SyncResult{Path: cursorRel, Action: ActionNoOp}` → `return noopResult(cursorRel)`
- Line 99 `return SyncResult{Path: cursorRel, Action: ActionUpdated}` → `return okResult(cursorRel, ActionUpdated, 0)`

- [ ] **Step 5: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 6: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
refactor(pointers): construct SyncResult via okResult/errResult/noopResult

Documents the Action == ActionError ⇔ Err != nil invariant and routes
all SyncResult construction through three helpers so the invariant is
maintained centrally. Also documents that LegacyBlocksPurged is
meaningful only on AGENTS.md update/create paths.

Closes review findings: Important-17 (invariant), Comment-3 (field doc).

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 2.4: Define sentinel errors

**Files:**

- New: `internal/config/pointers/errors.go`
- Modify: `agents.go`, `cursor.go`, `sync.go`
- Test: `sync_test.go`

- [ ] **Step 1: Write failing test that exercises `errors.Is` against sentinels.**

Append to `sync_test.go`:

```go
func TestSync_SentinelErrors(t *testing.T) {
	t.Run("corrupted markers", func(t *testing.T) {
		dir := t.TempDir()
		full := filepath.Join(dir, "AGENTS.md")
		if err := os.WriteFile(full, []byte(initStart+"\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		report := Sync(dir, defaultOpts())
		if !errors.Is(report.Agents.Err, ErrCorruptedMarkers) {
			t.Errorf("err = %v; want errors.Is ErrCorruptedMarkers", report.Agents.Err)
		}
	})

	t.Run("symlink rejected", func(t *testing.T) {
		dir := t.TempDir()
		// Make AGENTS.md a symlink — blocked by rejectSymlinkComponents.
		target := filepath.Join(t.TempDir(), "elsewhere")
		os.WriteFile(target, []byte("x"), 0o644)
		os.Symlink(target, filepath.Join(dir, "AGENTS.md"))
		report := Sync(dir, defaultOpts())
		if !errors.Is(report.Agents.Err, ErrSymlinkRejected) {
			t.Errorf("err = %v; want errors.Is ErrSymlinkRejected", report.Agents.Err)
		}
	})

	t.Run("missing frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		full := filepath.Join(dir, ".cursor", "rules", "specgraph-bootstrap.md")
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte("no frontmatter\n"), 0o644)
		report := Sync(dir, defaultOpts())
		if !errors.Is(report.Cursor.Err, ErrFrontmatterMissing) {
			t.Errorf("err = %v; want errors.Is ErrFrontmatterMissing", report.Cursor.Err)
		}
	})
}
```

Add `"errors"` to the imports if not already present.

- [ ] **Step 2: Run, expect fail.**

Run: `go test ./internal/config/pointers/ -run TestSync_SentinelErrors -v`

Expected: build fails on `ErrCorruptedMarkers`, etc.

- [ ] **Step 3: Create `internal/config/pointers/errors.go`.**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import "errors"

// Sentinel errors for callers that want to branch on failure mode.
// Use errors.Is, not == comparison: each Sync call wraps these with
// fmt.Errorf("...: %w", err) for a contextual top-level message.
var (
	// ErrCorruptedMarkers indicates managed-block markers in the target
	// file violate one of the four corruption rules in validateInitMarkers.
	// The caller should refuse to proceed; the user must repair the file
	// manually.
	ErrCorruptedMarkers = errors.New("corrupted init markers")

	// ErrSymlinkRejected indicates a path component (or the target itself)
	// is a symlink. Sync refuses to follow symlinks because the user owns
	// the file and may not own its symlink target.
	ErrSymlinkRejected = errors.New("refusing to follow symlink")

	// ErrFrontmatterMissing indicates the cursor rule file does not begin
	// with `---\n`. Sync refuses to silently convert it; the user must add
	// frontmatter or remove the file.
	ErrFrontmatterMissing = errors.New("missing YAML frontmatter")
)
```

- [ ] **Step 4: Wrap existing errors with the sentinels.**

In `internal/config/pointers/agents.go`, modify `validateInitMarkers` to wrap each return:

Replace each `return fmt.Errorf("%s ...", ...)` inside `validateInitMarkers` with `return fmt.Errorf("%w: %s ...", ErrCorruptedMarkers, ...)`. Specific edits — five returns at lines 127-131, 142, 146, 148, 150.

In `internal/config/pointers/sync.go`, modify `rejectSymlinkComponents`:

Replace `return fmt.Errorf("refusing to follow symlink %s", cur)` (line 95) with `return fmt.Errorf("%w: %s", ErrSymlinkRejected, cur)`.

And in `Sync`, line for projectDir symlink:
Replace `fmt.Errorf("refusing to follow symlink %s", projectDir)` with `fmt.Errorf("%w: %s", ErrSymlinkRejected, projectDir)`.

In `internal/config/pointers/cursor.go`, modify `splitFrontmatter`:

Replace the missing-frontmatter error at lines 107-111 with:

```go
return nil, nil, fmt.Errorf("%w: %s (must start with '---'); remove the file or add frontmatter manually", ErrFrontmatterMissing, cursorRel)
```

- [ ] **Step 5: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 6: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
feat(pointers): introduce sentinel errors for caller branching

Adds ErrCorruptedMarkers, ErrSymlinkRejected, ErrFrontmatterMissing
in a new errors.go file. Each is wrapped via fmt.Errorf("%w: ...")
at the callsites where the failure originates. Callers (today only
init; future TUIs or CI integrations) can use errors.Is to branch
on failure mode and surface targeted remediation hints.

Closes review suggestion: Type-design sentinel-errors.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 3: File-Lock Hardening

Closes Critical-3 (Windows lock no-op), Critical-4 / Important-13 (unlock errors invisible), Type-Design-6 (`Unlocker` named type).

### Task 3.1: Lock function returns `Unlocker = func() error`

**Files:**

- Modify: `lock_unix.go`, `lock_windows.go`, `agents.go`, `cursor.go`
- Test: `sync_test.go`

- [ ] **Step 1: Define the named type and change signatures.**

In `internal/config/pointers/sync.go` (top-level type declarations), add:

```go
// Unlocker releases a file lock acquired via acquireFileLock. It returns
// any error from the underlying flock LOCK_UN (Unix) or LockFileEx
// release (Windows) plus any error closing the lock-file handle. Callers
// MUST invoke Unlocker via a deferred wrapper that propagates the error.
type Unlocker func() error
```

In `internal/config/pointers/lock_unix.go`, replace lines 19-36:

```go
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

Add `"errors"` to imports.

- [ ] **Step 2: Update Windows stub temporarily** (proper impl in Task 3.2):

In `internal/config/pointers/lock_windows.go`:

```go
func acquireFileLock(path string) (Unlocker, error) {
	slog.Warn("file locking is not implemented on Windows; concurrent specgraph init runs may race", "path", path)
	return func() error { return nil }, nil
}
```

- [ ] **Step 3: Update callers in `agents.go` and `cursor.go`** to thread the unlock error into the function's return.

In `internal/config/pointers/agents.go`, replace the `defer unlock()` pattern starting at line 54-58:

```go
	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return errResult(agentsRel, lerr)
	}
	defer func() {
		if uerr := unlock(); uerr != nil {
			slog.Error("unlock failed", "path", full, "error", uerr)
		}
	}()
```

Add `"log/slog"` to imports.

(Decision: log unlock errors via slog rather than mutate the result, because the SyncResult is already fully populated by the time defer fires. Slog is a deliberate trade-off vs returning the error — it surfaces the signal to callers running with a configured slog handler without complicating the pure-result API. We accept this trade-off since unlock-after-success is rare and non-corrupting.)

Same pattern for `cursor.go` lines 37-41.

- [ ] **Step 4: Run.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
refactor(pointers): make Unlocker return error and propagate via slog

acquireFileLock now returns a named type Unlocker = func() error that
composes the LOCK_UN error and the lock-file close error via
errors.Join. Callers in agents.go / cursor.go invoke unlock via a
deferred wrapper that logs failures through slog.Error. Replaces the
previous bare func() that silently dropped both classes of error.

Closes review findings: Critical-4, Important-13, Type-Design-6.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 3.2: Implement real Windows file locking

**Files:**

- Modify: `lock_windows.go`
- New (maybe): test under `//go:build windows`
- Modify: `go.mod`, `go.sum` (via `go get`)

- [ ] **Step 1: Add the dependency.**

Run: `go get golang.org/x/sys/windows`

(This affects `go.sum`; `go.mod` already has `golang.org/x/sys` per existing transitive deps — verify with `grep golang.org/x/sys go.mod`. If not present, add it.)

- [ ] **Step 2: Replace `lock_windows.go` with a real implementation.**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// acquireFileLock acquires an exclusive lock on a sibling file <path>.lock
// via LockFileEx. Returns an Unlocker that calls UnlockFileEx and closes
// the handle. The lock file is intentionally not removed (see lock_unix.go
// for the rationale).
func acquireFileLock(path string) (Unlocker, error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	handle := windows.Handle(lockFile.Fd())
	var ol windows.Overlapped
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		return nil, errors.Join(
			fmt.Errorf("acquire file lock: %w", err),
			lockFile.Close(),
		)
	}
	return func() error {
		var ol windows.Overlapped
		uerr := windows.UnlockFileEx(handle, 0, 1, 0, &ol)
		cerr := lockFile.Close()
		if uerr != nil || cerr != nil {
			return errors.Join(uerr, cerr)
		}
		return nil
	}, nil
}
```

- [ ] **Step 3: Smoke-test compile via build tag.**

Run: `GOOS=windows go build ./internal/config/pointers/`

Expected: build succeeds.

- [ ] **Step 4: Verify the unix path still builds + tests pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 5: Update the doc comment to remove the misleading "best-effort" framing.**

Already replaced in step 2 — verify no stale text remains.

- [ ] **Step 6: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
fix(pointers): implement real Windows file locking via LockFileEx

Replaces the slog.Warn-and-no-op stub with a LockFileEx-based
exclusive lock from golang.org/x/sys/windows, mirroring the Unix
flock semantics. Concurrent specgraph init runs on Windows are now
serialized at the lock layer.

Closes review finding Critical-3.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 4: Symlink TOCTOU + Lstat Error Handling

Closes Critical-2 (TOCTOU), already-handled Important-14 (in Phase 2), Test-Important-4 (intermediate-component coverage).

### Task 4.1: Add test for symlink at intermediate path component

**Files:**

- Test: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Write the failing test.**

Append:

```go
func TestSync_RejectsSymlinkAtIntermediateComponent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows; covered on Unix only")
	}
	dir := t.TempDir()
	other := t.TempDir()
	// Make .cursor a symlink to a directory outside the project.
	if err := os.Symlink(other, filepath.Join(dir, ".cursor")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	report := Sync(dir, defaultOpts())
	if report.Cursor.Action != ActionError {
		t.Fatalf("Cursor.Action = %v, want ActionError (err=%v)", report.Cursor.Action, report.Cursor.Err)
	}
	if !errors.Is(report.Cursor.Err, ErrSymlinkRejected) {
		t.Errorf("Cursor.Err = %v; want errors.Is ErrSymlinkRejected", report.Cursor.Err)
	}
	// AGENTS.md should still be created — failure isolation between files.
	if report.Agents.Action != ActionCreated {
		t.Errorf("Agents.Action = %v, want ActionCreated despite cursor symlink", report.Agents.Action)
	}
}
```

Add `"errors"` to imports if not already.

- [ ] **Step 2: Run, expect pass** (the existing `rejectSymlinkComponents` walk should already cover this; this is a regression pin).

Run: `go test ./internal/config/pointers/ -run TestSync_RejectsSymlinkAtIntermediateComponent -v`

Expected: PASS.

- [ ] **Step 3: Commit (behavior pin only — no impl change needed).**

```bash
jj commit -m "$(cat <<'EOF'
test(pointers): pin intermediate-component symlink rejection for cursor

Symlinks at .cursor or .cursor/rules would let the rejection check be
bypassed by future refactors that take a single Lstat shortcut. Adds
a behavioral pin that also asserts AGENTS.md still gets written
(failure-isolation regression guard).

Closes review finding Test-Important-4.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 4.2: Reduce TOCTOU window with `O_NOFOLLOW` reads on Unix

**Files:**

- Modify: `internal/config/pointers/agents.go`, `cursor.go`
- New: `internal/config/pointers/open_unix.go`, `open_windows.go`

- [ ] **Step 1: Document the security boundary explicitly in `doc.go`.**

Edit `internal/config/pointers/doc.go`. Append before the closing `package pointers`:

```go
//
// Symlink rejection is best-effort, not a security boundary. The
// rejectSymlinkComponents walk and the subsequent open are not atomic;
// a process that can write to the project directory between the walk
// and the open could swap a path component for a symlink. On Unix we
// reduce the window by opening read targets with O_NOFOLLOW; on
// Windows we rely on the walk only. Treat the project directory as a
// trust boundary and run specgraph init from a directory you own.
```

- [ ] **Step 2: Add a tiny platform-shimmed `openNoFollow` helper.**

Create `internal/config/pointers/open_unix.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package pointers

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// readFileNoFollow is os.ReadFile with O_NOFOLLOW. On a symlink it returns
// an error wrapping syscall.ELOOP; on file-not-found it returns an error
// satisfying errors.Is(_, fs.ErrNotExist).
func readFileNoFollow(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// noFollowIsSymlink reports whether err arose from O_NOFOLLOW refusing
// to traverse a symlink. Used by callers to translate ELOOP into
// ErrSymlinkRejected.
func noFollowIsSymlink(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}

var _ = fs.ErrNotExist // referenced by readFile callers' errors.Is
```

Create `internal/config/pointers/open_windows.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import "os"

// readFileNoFollow falls back to os.ReadFile on Windows; symlink rejection
// is handled by the rejectSymlinkComponents walk only. Documented in doc.go
// as best-effort, not a security boundary.
func readFileNoFollow(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func noFollowIsSymlink(err error) bool { return false }
```

- [ ] **Step 3: Replace `os.ReadFile` calls in `agents.go:60` and `cursor.go:43` with `readFileNoFollow`.**

In `internal/config/pointers/agents.go`, change line 60:

```go
	existing, rerr := readFileNoFollow(full)
```

In `internal/config/pointers/cursor.go`, change line 43:

```go
	existing, rerr := readFileNoFollow(full)
```

Add a translation block right after each, before the existing `errors.Is(rerr, fs.ErrNotExist)` check:

In `agents.go`:

```go
	if rerr != nil && noFollowIsSymlink(rerr) {
		return errResult(agentsRel, fmt.Errorf("%w: %s", ErrSymlinkRejected, full))
	}
```

In `cursor.go`:

```go
	if rerr != nil && noFollowIsSymlink(rerr) {
		return errResult(cursorRel, fmt.Errorf("%w: %s", ErrSymlinkRejected, full))
	}
```

- [ ] **Step 4: Add a test that races a symlink swap against the read.**

Append to `sync_test.go`:

```go
func TestSync_NoFollowReadRejectsSymlinkSwappedAfterCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("O_NOFOLLOW is Unix-only; Windows path documented as best-effort")
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "decoy")
	if err := os.WriteFile(target, []byte("decoy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Plant a symlink AT the target path before any check runs.
	// rejectSymlinkComponents will catch this; the test pins the secondary
	// O_NOFOLLOW defense as well.
	if err := os.Symlink(target, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	report := Sync(dir, defaultOpts())
	if !errors.Is(report.Agents.Err, ErrSymlinkRejected) {
		t.Errorf("err = %v; want errors.Is ErrSymlinkRejected", report.Agents.Err)
	}
}
```

- [ ] **Step 5: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 6: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
fix(pointers): read existing pointer files with O_NOFOLLOW on Unix

Reduces the TOCTOU window between rejectSymlinkComponents and the
subsequent read. A symlink swapped in after the walk is now refused
at open time on Unix (ELOOP -> ErrSymlinkRejected). Windows falls
back to plain os.ReadFile and remains best-effort, documented as such
in doc.go.

Adds an explicit security-boundary disclaimer: the project directory
is a trust boundary; specgraph init does not defend against an
attacker who can write to it.

Closes review finding Critical-2.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 5: Behavioral Test Gap Closure

Closes Test-Critical-1 (orphan files), Test-Critical-3 (resolved-serverURL wiring), Test-Important-5 (init pointer-error propagation), Test-Important-7 (cursor-fails / agents-succeeds), Test-Important-8 (read-only dir), Important-11 (Created vs Updated empty file), Important-12 (mismatched-slug legacy block reporting).

### Task 5.1: Pin orphan-file behavior — inject artifacts left untouched

**Files:**

- Test: `cmd/specgraph/init_test.go` (append)

- [ ] **Step 1: Write failing test.**

Append to `cmd/specgraph/init_test.go`:

```go
func TestInit_LeavesLegacyOrphanFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Seed orphan inject artifacts that the PR description explicitly
	// promises to leave alone: per-slug AGENTS.md blocks were purged
	// in-place, but per-slug files under .claude/specs/ and
	// .cursor/rules/specgraph-<slug>.md (other than specgraph-bootstrap.md)
	// must survive.
	orphans := map[string][]byte{
		filepath.Join(".claude", "specs", "old-spec.md"):                  []byte("# orphan claude spec\n"),
		filepath.Join(".cursor", "rules", "specgraph-old-feature.md"):     []byte("---\n---\n# orphan per-slug rule\n"),
		filepath.Join(".cursor", "rules", "specgraph.md"):                 []byte("# plugin-shipped rule (must survive)\n"),
	}
	for rel, content := range orphans {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := runInitInDirWithGlobalCfg(dir, "specgraph", "http://127.0.0.1:7890"); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	for rel, want := range orphans {
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Errorf("orphan %s: read failed: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("orphan %s mutated\n  got:  %q\n  want: %q", rel, got, want)
		}
	}
}
```

(Imports needed: `bytes`, `os`, `path/filepath`, `testing`. The helper `runInitInDirWithGlobalCfg` must already exist in the test file — verify via `grep -n runInitInDirWithGlobalCfg cmd/specgraph/init_test.go`. If not, the closest existing helper is what we'll use; adapt the call.)

- [ ] **Step 2: Run, expect pass** (no impl change should be needed; this pins existing behavior).

Run: `go test ./cmd/specgraph/ -run TestInit_LeavesLegacyOrphanFilesUntouched -v`

Expected: PASS.

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(init): pin "orphan inject artifacts left untouched" behavior

The PR description and CHANGELOG explicitly promise that per-slug
.claude/specs/*.md and .cursor/rules/specgraph-*.md (excluding
specgraph-bootstrap.md and the plugin-shipped specgraph.md) are not
auto-deleted. Without a behavioral pin, a future "helpful cleanup" PR
could silently delete user files.

Closes review finding Test-Critical-1.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.2: Pin resolved-serverURL flow into AGENTS.md

**Files:**

- Test: `cmd/specgraph/init_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestRunInit_ResolvedServerURLFlowsIntoAgentsMD(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	const customServer = "http://7.7.7.7:7890"
	if err := runInitInDirWithGlobalCfg(dir, "specgraph", customServer); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(customServer)) {
		t.Errorf("AGENTS.md does not contain resolved server URL %q\n%s", customServer, got)
	}
}
```

- [ ] **Step 2: Run.** Confirm pass; this is a regression pin.

Run: `go test ./cmd/specgraph/ -run TestRunInit_ResolvedServerURLFlowsIntoAgentsMD -v`

Expected: PASS.

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(init): pin resolved server URL flowing into AGENTS.md pointer

Mirrors TestRunInit_ResolvedServerURLFlowsIntoConfigs but for the
pointer-file phase. A regression that passed pc.Server (raw) instead
of the globally-resolved URL to pointers.Sync would otherwise ship
undetected.

Closes review finding Test-Critical-3.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.3: Pin init pointer-error propagation

**Files:**

- Test: `cmd/specgraph/init_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestRunInit_PropagatesPointerSyncErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Seed AGENTS.md with corrupted markers (start without end).
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"),
		[]byte("<!-- specgraph:init:start v=1 -->\nbody without end\n"),
		0o644); err != nil {
		t.Fatal(err)
	}

	err := runInitInDirWithGlobalCfg(dir, "specgraph", "http://127.0.0.1:7890")
	if err == nil {
		t.Fatal("runInit returned nil; want non-nil pointer-sync error")
	}
	if !strings.Contains(err.Error(), "sync pointer files") {
		t.Errorf("err = %q; want substring 'sync pointer files'", err.Error())
	}
}
```

Add `"strings"` import if not present.

- [ ] **Step 2: Run, expect pass** (existing init logic at line 151-153 already returns this error — pin only).

Run: `go test ./cmd/specgraph/ -run TestRunInit_PropagatesPointerSyncErrors -v`

Expected: PASS.

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(init): pin pointer-sync error propagation to runInit return

Mirrors TestRunInit_NoSuccessBannerWhenSyncFails but for the pointer
phase. Seeds AGENTS.md with a missing-end-marker corruption, runs
runInit, asserts the returned error mentions 'sync pointer files'.

Closes review finding Test-Important-5.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.4: Fix `Created` vs `Updated` for empty existing AGENTS.md

**Files:**

- Modify: `internal/config/pointers/agents.go`
- Test: `sync_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestSync_EmptyFileReportsUpdatedNotCreated(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(full, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionUpdated {
		t.Errorf("Agents.Action = %v, want %v (file existed but was empty)", report.Agents.Action, ActionUpdated)
	}
}
```

- [ ] **Step 2: Run, expect fail.**

Run: `go test ./internal/config/pointers/ -run TestSync_EmptyFileReportsUpdatedNotCreated -v`

Expected: FAIL with `Agents.Action = created`.

- [ ] **Step 3: Implement fix.** In `internal/config/pointers/agents.go`, replace lines 60-63 + lines 105-108. Track whether the file existed using `errors.Is(rerr, fs.ErrNotExist)`:

Replace `existing, rerr := readFileNoFollow(full)` block:

```go
	existing, rerr := readFileNoFollow(full)
	if rerr != nil && noFollowIsSymlink(rerr) {
		return errResult(agentsRel, fmt.Errorf("%w: %s", ErrSymlinkRejected, full))
	}
	preexisted := !errors.Is(rerr, fs.ErrNotExist)
	if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
		return errResult(agentsRel, fmt.Errorf("read %s: %w", full, rerr))
	}
```

Replace lines 105-108 (action determination):

```go
	action := ActionUpdated
	if !preexisted {
		action = ActionCreated
	}
	return okResult(agentsRel, action, purged)
```

- [ ] **Step 4: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: all green.

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
fix(pointers): empty AGENTS.md reports Updated, not Created

The previous len(existing) == 0 ⇒ ActionCreated heuristic conflated
"file does not exist" with "file exists but is empty". Tracking
existence via errors.Is(rerr, fs.ErrNotExist) gives the correct
distinction. A user-touched zero-byte AGENTS.md no longer reports a
misleading "created".

Closes review finding Important-11.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.5: Surface mismatched-slug legacy blocks via a new `SyncResult` field

**Files:**

- Modify: `sync.go` (`SyncResult`), `agents.go` (`purgeLegacyBlocks`)
- Test: `sync_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestSync_MismatchedSlugLegacyBlocksReported(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "AGENTS.md")
	// Slug-start "foo" paired with slug-end "bar" — corrupt legacy block.
	corrupt := "<!-- specgraph:foo:start -->\nbody\n<!-- specgraph:bar:end -->\n"
	if err := os.WriteFile(full, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Sync(dir, defaultOpts())
	if report.Agents.LegacyBlocksSkippedMalformed == 0 {
		t.Errorf("LegacyBlocksSkippedMalformed = 0; want >= 1")
	}
}
```

- [ ] **Step 2: Run, expect compile fail.**

Run: `go test ./internal/config/pointers/ -run TestSync_MismatchedSlugLegacyBlocks -v`

Expected: FAIL — undefined field `LegacyBlocksSkippedMalformed`.

- [ ] **Step 3: Add the field and increment in `purgeLegacyBlocks`.**

In `internal/config/pointers/sync.go`, add to `SyncResult`:

```go
	// LegacyBlocksSkippedMalformed counts legacy slug-pair blocks that
	// were detected but not purged because the start and end slugs
	// differ. They remain in the file; the user must repair manually.
	LegacyBlocksSkippedMalformed int
```

Update `okResult` to take a fourth arg:

```go
func okResult(path string, action Action, purged, skipped int) SyncResult {
	return SyncResult{Path: path, Action: action, LegacyBlocksPurged: purged, LegacyBlocksSkippedMalformed: skipped}
}
```

In `internal/config/pointers/agents.go`, change `purgeLegacyBlocks` signature and impl:

```go
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

Update `syncAgents` to thread the new return:

```go
	updated := existing
	purged := 0
	skipped := 0
	if len(updated) > 0 {
		updated, purged, skipped = purgeLegacyBlocks(updated)
	}
```

And the success-path return:

```go
	return okResult(agentsRel, action, purged, skipped)
```

Also `cursor.go` callers of `okResult` need a 0 added:

- Line 55: `return okResult(cursorRel, ActionCreated, 0, 0)`
- Line 99: `return okResult(cursorRel, ActionUpdated, 0, 0)`

And `init.go`'s render line should mention it:

```go
			line := fmt.Sprintf("%s: %s", r.Path, r.Action)
			if r.LegacyBlocksPurged > 0 {
				line += fmt.Sprintf(" (purged %d legacy blocks)", r.LegacyBlocksPurged)
			}
			if r.LegacyBlocksSkippedMalformed > 0 {
				line += fmt.Sprintf(" (skipped %d malformed legacy blocks; repair manually)", r.LegacyBlocksSkippedMalformed)
			}
			fmt.Println(line)
```

- [ ] **Step 4: Run, expect pass.**

Run: `go test ./internal/config/pointers/ ./cmd/specgraph/ -v`

Expected: all green.

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
feat(pointers): report malformed legacy blocks via SyncResult field

Legacy blocks where the start/end slugs differ are detected by
purgeLegacyBlocks but cannot be purged automatically. They were
previously left in place silently. Now each detection increments
SyncResult.LegacyBlocksSkippedMalformed and init renders
"(skipped N malformed legacy blocks; repair manually)" so the user
gets a clear remediation cue.

Closes review finding Important-12.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.6: Pin "cursor fails, AGENTS.md succeeds" failure isolation (inverse direction)

**Files:**

- Test: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestSync_CursorFailureDoesNotAbortAgents(t *testing.T) {
	dir := t.TempDir()
	// Seed cursor with no frontmatter — splitFrontmatter will reject.
	cursorPath := filepath.Join(dir, ".cursor", "rules", "specgraph-bootstrap.md")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorPath, []byte("body without frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := Sync(dir, defaultOpts())
	if report.Cursor.Action != ActionError {
		t.Fatalf("Cursor.Action = %v, want ActionError", report.Cursor.Action)
	}
	if report.Agents.Action != ActionCreated {
		t.Errorf("Agents.Action = %v, want ActionCreated despite cursor failure", report.Agents.Action)
	}
}
```

- [ ] **Step 2: Run, expect pass** (regression pin only).

Run: `go test ./internal/config/pointers/ -run TestSync_CursorFailureDoesNotAbortAgents -v`

Expected: PASS.

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(pointers): pin cursor-fails / AGENTS.md-succeeds failure isolation

The existing TestSync_FailureOnOneFileDoesNotAbortOther covers AGENTS.md
failure leaving cursor still attempted. Adds the inverse direction so a
future reorder of syncAgents/syncCursor invocation in Sync doesn't
silently regress the contract.

Closes review finding Test-Important-7.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 5.7: Pin behavior on read-only project directory

**Files:**

- Test: `internal/config/pointers/sync_test.go`

- [ ] **Step 1: Write failing test.** Append:

```go
func TestSync_ReadOnlyProjectDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dir mode 0555 does not block writes the same way on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionError {
		t.Errorf("Agents.Action = %v, want ActionError on read-only dir", report.Agents.Action)
	}
	if report.Cursor.Action != ActionError {
		t.Errorf("Cursor.Action = %v, want ActionError on read-only dir", report.Cursor.Action)
	}
	// macOS APFS may permit some operations even with 0555 — accept either
	// failure source as long as both files report ActionError.
}
```

- [ ] **Step 2: Run.**

Run: `go test ./internal/config/pointers/ -run TestSync_ReadOnlyProjectDir -v`

Expected: on Linux: PASS. On macOS APFS: may fail or skip. If the test fails on macOS, add a `runtime.GOOS == "darwin"` skip with a TODO note (we accept this is a Linux-CI-only assertion).

- [ ] **Step 3: If macOS skip needed, add it. Then commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(pointers): pin behavior on read-only project directory

Both pointer files must report ActionError when projectDir is mode
0555. macOS APFS does not enforce dir-write permission the same way
Linux ext4/xfs do, so the test skips there with a TODO. The Linux CI
job exercises the path.

Closes review finding Test-Important-8.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 6: Init Caller Hygiene

Closes Important-8 (gitignore), Suggestion-trim-URL-comment, Suggestion-drop-obvious-comment.

### Task 6.1: Add lock-file pattern to `.gitignore`

**Files:**

- Modify: `.gitignore`

- [ ] **Step 1:** Edit `.gitignore`. Find the section after the `# Skip dotenv` block (around line 47) and add:

```gitignore

# Pointer-file lock siblings (managed by internal/config/pointers).
# Persistent by design — see lock_unix.go for the rationale.
*.md.lock
```

- [ ] **Step 2: Verify.**

Run: `cd ~/Code/github.com/specgraph-deprecate-inject && git check-ignore -v AGENTS.md.lock .cursor/rules/specgraph-bootstrap.md.lock 2>&1 || true`

Expected output: paths matched against the new pattern.

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
chore: gitignore *.md.lock pointer-file lock siblings

internal/config/pointers/acquireFileLock writes <path>.lock siblings
that are intentionally never removed (deletion between unlock and
concurrent open would create a new inode and break mutual exclusion).
Without a gitignore entry, every git status in a project that has
run specgraph init shows two untracked .md.lock files.

Closes review finding Important-8.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 6.2: Tighten `cmd/specgraph/init.go` URL-validation comment

**Files:**

- Modify: `cmd/specgraph/init.go`

- [ ] **Step 1:** Replace lines 89-96 with a 2-line comment:

```go
	// Reject malformed/relative server URLs before any writes. url.Parse
	// is lenient — bare "/api", "example.com", and "localhost:3000" all
	// parse — so we require Scheme ∈ {http,https} AND non-empty Host.
```

- [ ] **Step 2:** Drop the redundant "Run only after mcpconfigs succeeded" comment at line 130-131. Replace with a one-liner:

```go
	// Pointer-file errors don't abort the pointer phase, but produce a
	// non-zero exit so CI surfaces them.
```

- [ ] **Step 3:** Drop the obvious "Only emit the success banner after Sync succeeds" comment at line 155-157.

- [ ] **Step 4:** Run.

Run: `go build ./cmd/specgraph/ && go test ./cmd/specgraph/ -v`

Expected: still green.

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
docs(init): trim multi-paragraph comments down to load-bearing WHY

Three comments in cmd/specgraph/init.go restated structure already
visible in the code (banner-after-sync ordering, pointer-runs-after-
mcpconfigs sequencing) and one paragraph essay explained url.Parse's
known leniency in eight lines where two suffice.

Closes review suggestions: trim-URL-validation-comment, drop-obvious-
comments.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 7: Comment Accuracy Fixes

Closes Comment-1 (mcpconfigs.Action mirror claim), Comment-2 (Sync doc symlink short-circuit — already fixed in Phase 2), Comment-4 (lock_windows.go ADR posture), Comment-5 (proto reserved comments).

### Task 7.1: Tighten `mcpconfigs.Action mirror` claim

**Files:**

- Modify: `internal/config/pointers/sync.go`

- [ ] **Step 1:** Replace the comment at lines 15-20 with:

```go
// Action describes what Sync did to a single managed pointer file.
// ActionCreated/ActionUpdated/ActionNoOp string values overlap with
// mcpconfigs.Action so init can render a unified "<path>: <action>"
// line. ActionError is pointers-only — mcpconfigs aborts on first
// error rather than per-file reporting.
type Action string

const (
	ActionCreated Action = "created"
	ActionUpdated Action = "updated"
	ActionNoOp    Action = "no-op"
	ActionError   Action = "error"
)
```

- [ ] **Step 2: Build, commit.**

Run: `go build ./...`

```bash
jj commit -m "$(cat <<'EOF'
docs(pointers): correct Action-mirror comment to match reality

ActionError is pointers-only (mcpconfigs aborts instead of reporting
per-file). The previous "values mirror mcpconfigs.Action string tags"
claim was true for three of four constants only.

Closes review finding Comment-1.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 7.2: Replace "ADR posture" reference in lock_windows.go

**Note:** Phase 3 already replaced lock_windows.go entirely with a real implementation, so this finding is implicitly resolved. Verify and skip if so.

- [ ] **Step 1: Verify.** Run: `grep -n "ADR posture" internal/config/pointers/lock_windows.go || echo "resolved by Phase 3"`

Expected: `resolved by Phase 3`. If the grep finds anything, replace it with concrete reference to the design doc.

- [ ] **Step 2:** No commit needed if already resolved.

### Task 7.3: Tighten proto `reserved` comments

**Files:**

- Modify: `proto/specgraph/v1/sync.proto`

- [ ] **Step 1:** Replace the multi-line "paper trail" comments at the affected `reserved` blocks with single-line historical pointers. Find each block via:

Run: `grep -n "paper trail\|InjectTool\|InjectRequest\|InjectResponse" proto/specgraph/v1/sync.proto`

For each match, edit the multi-line comment down to one line of the form:

```proto
// Removed in spgr-yyjf: <symbol>. Reusing the name later is harmless
// on the wire; this comment is a historical pointer only.
```

- [ ] **Step 2: Regenerate proto** (per CLAUDE.md the gen/ dir is committed).

Run: `task proto`

- [ ] **Step 3: Verify generated code compiles.**

Run: `go build ./...`

- [ ] **Step 4: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
docs(proto): drop self-undermining "paper trail" framing on reserved

The previous comments explained why they were kept despite arguably
being noise. Single-line "Removed in spgr-yyjf: <symbol>" pointers
serve the same purpose without the meta-justification.

Closes review finding Comment-5.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 8: Suggestions

Closes Suggestion-forward-compat-marker-version, Suggestion-lift-fslock-shared (deferred — note in CHANGELOG), Suggestion-splitFrontmatter-trailing-blank (deferred), Suggestion-cursor-body-no-markers test (small).

### Task 8.1: Forward-compat init marker regex

**Files:**

- Modify: `internal/config/pointers/agents.go`
- Test: `sync_test.go`

- [ ] **Step 1: Write a failing test.** Append:

```go
func TestSync_HypotheticalV2MarkerIsCorruption(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "AGENTS.md")
	// Future v=2 marker that today's code doesn't emit but might one day.
	bogus := "<!-- specgraph:init:start v=2 -->\nbody\n<!-- specgraph:init:end -->\n"
	if err := os.WriteFile(full, []byte(bogus), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Sync(dir, defaultOpts())
	if report.Agents.Action != ActionError {
		t.Errorf("Action = %v; want ActionError on unknown init version", report.Agents.Action)
	}
	if !errors.Is(report.Agents.Err, ErrCorruptedMarkers) {
		t.Errorf("Err = %v; want errors.Is ErrCorruptedMarkers", report.Agents.Err)
	}
}
```

- [ ] **Step 2: Run, expect fail** (today the v=2 marker is invisible to both `initStart` and `initStartLoose` so Sync silently appends a duplicate v=1 block).

Run: `go test ./internal/config/pointers/ -run TestSync_HypotheticalV2MarkerIsCorruption -v`

Expected: FAIL.

- [ ] **Step 3: Add a "any-version" pattern and a fifth corruption rule.**

In `internal/config/pointers/agents.go`, after `initStartLoose`, add:

```go
// initStartAnyVersion matches any specgraph:init:start marker, with or
// without a v=N suffix. Used to detect future-versioned markers we don't
// know how to migrate yet.
var initStartAnyVersion = regexp.MustCompile(`<!--\s*specgraph:init:start(\s+v=\d+)?\s*-->`)
```

In `validateInitMarkers`, just before the existing rule-4 loop, add:

```go
	// Rule 5: any specgraph:init:start marker that isn't the canonical
	// v=1 form is treated as an unknown version we can't migrate.
	for _, m := range initStartAnyVersion.FindAllIndex(data, -1) {
		fragment := data[m[0]:m[1]]
		if !bytes.Equal(fragment, []byte(initStart)) {
			return fmt.Errorf("%w: %s contains a non-v=1 init start marker at offset %d (%q); migrate or remove manually",
				ErrCorruptedMarkers, displayName, m[0], string(fragment))
		}
	}
```

- [ ] **Step 4: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -v`

Expected: green (the new test passes; the existing v=1 happy path still passes since `bytes.Equal(fragment, []byte(initStart))` matches).

- [ ] **Step 5: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
fix(pointers): treat unknown-version init markers as corruption

A hypothetical future <!-- specgraph:init:start v=2 --> marker was
previously invisible to both initStart and initStartLoose, so Sync
silently appended a duplicate v=1 block. The new initStartAnyVersion
pattern catches anything specgraph:init:start* that isn't canonical
v=1 and surfaces it via ErrCorruptedMarkers with an offset.

Closes review suggestion: forward-compat-marker-version.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 8.2: Pin cursor "frontmatter present, body has no markers" branch

**Files:**

- Test: `sync_test.go`

- [ ] **Step 1: Write test.** Append:

```go
func TestSync_CursorAppendsBlockWhenBodyHasNoMarkers(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, ".cursor", "rules", "specgraph-bootstrap.md")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := "---\ndescription: pre-existing rule\nalwaysApply: true\n---\n\n# user notes\n"
	if err := os.WriteFile(cursorPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	report := Sync(dir, defaultOpts())
	if report.Cursor.Action != ActionUpdated {
		t.Fatalf("Cursor.Action = %v, want ActionUpdated (err=%v)", report.Cursor.Action, report.Cursor.Err)
	}
	got, _ := os.ReadFile(cursorPath)
	if !bytes.Contains(got, []byte("# user notes")) {
		t.Errorf("user notes lost\n%s", got)
	}
	if !bytes.Contains(got, []byte(initStart)) || !bytes.Contains(got, []byte(initEnd)) {
		t.Errorf("init markers not appended\n%s", got)
	}
}
```

- [ ] **Step 2: Run, expect pass.**

Run: `go test ./internal/config/pointers/ -run TestSync_CursorAppendsBlockWhenBodyHasNoMarkers -v`

Expected: PASS (regression pin).

- [ ] **Step 3: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
test(pointers): pin cursor "frontmatter present, body lacks markers" path

cursor.go's else-branch (line 79) appends a leading blank line and the
managed block when frontmatter is valid but the body doesn't contain
the init markers. Previously not directly tested in isolation.

Closes review suggestion: cursor body branch coverage.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

---

## Phase 9: Final Verification & Push

### Task 9.1: Full quality gate

- [ ] **Step 1:** Run the project's pre-push gate.

Run: `cd ~/Code/github.com/specgraph-deprecate-inject && task check`

Expected: green (fmt, license, lint, build, unit tests).

- [ ] **Step 2:** Integration tests (Docker-dependent).

Run: `task test:integration`

Expected: green.

- [ ] **Step 3:** E2E.

Run: `task test:e2e:api && task test:e2e:cli`

Expected: green.

### Task 9.2: Update CHANGELOG

**Files:**

- Modify: `CHANGELOG.md`

- [ ] **Step 1:** Append a new section under the existing `## Unreleased` (or the spgr-yyjf section) describing the hardening pass. Keep it short:

```markdown
### Fixed (post-PR-940 hardening)

- `internal/config/pointers/atomicWrite` now `fsync`s the temp file and the
  parent directory before returning, preserves the existing file's mode on
  update, and composes cleanup errors via `errors.Join` instead of
  `//nolint:errcheck`.
- File-lock contract: `acquireFileLock` returns `Unlocker = func() error`
  so unlock failures reach the caller. Windows now uses `LockFileEx` (no
  more silent no-op).
- `Sync` returns `SyncReport{Agents, Cursor}` instead of `[]SyncResult`;
  callers stop indexing positionally. `pointers.NewOptions` validates
  server URL and project slug. Sentinel errors `ErrCorruptedMarkers`,
  `ErrSymlinkRejected`, `ErrFrontmatterMissing` enable caller branching.
- `*.md.lock` sibling lock files now ignored by `.gitignore`.
```

- [ ] **Step 2: Commit.**

```bash
jj commit -m "$(cat <<'EOF'
docs(changelog): record post-PR-940 hardening pass

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>
EOF
)"
```

### Task 9.3: Advance bookmark and push

- [ ] **Step 1:** Advance the bookmark to the tip of the workspace's lineage.

Run: `jj --no-pager bookmark set spgr-yyjf-deprecate-inject -r @-`

(`@-` because `@` is the empty working-copy commit; the last real commit is its parent.)

- [ ] **Step 2:** Push to remote.

Run: `jj --no-pager git push --bookmark spgr-yyjf-deprecate-inject`

Expected: push succeeds.

- [ ] **Step 3:** Close the beads issue.

Run: `bd close <id>`

- [ ] **Step 4:** Run `bd dolt push` to sync issue store.

Run: `bd dolt push`

---

## Self-Review

**Spec coverage:**

| Review finding | Phase | Task |
|---|---|---|
| Critical-1 (fsync) | 1 | 1.1 |
| Critical-2 (TOCTOU) | 4 | 4.2 |
| Critical-3 (Windows lock no-op) | 3 | 3.2 |
| Critical-4 (errors.Join cleanup) | 1 | 1.1 |
| Critical-5 (subsumed by 4) | 1 | 1.1 |
| Critical-6 (close-error in failure paths) | 1 | 1.1 |
| Test-Critical-1 (orphan files) | 5 | 5.1 |
| Test-Critical-2 (file-mode) | 1 | 1.1 |
| Test-Critical-3 (resolved-serverURL wiring) | 5 | 5.2 |
| Important-8 (gitignore) | 6 | 6.1 |
| Important-9 (bisect-broken commit) | n/a — historical, noted only |  |
| Important-10 (stderr) | 2 | 2.2 |
| Important-11 (Created vs Updated empty) | 5 | 5.4 |
| Important-12 (mismatched-slug legacy) | 5 | 5.5 |
| Important-13 (lock unlock errors) | 3 | 3.1 |
| Important-14 (Lstat error swallow) | 2 | 2.2 |
| Important-15 (Options validation) | 2 | 2.1 |
| Important-16 (SyncReport struct) | 2 | 2.2 |
| Important-17 (ActionError ↔ Err) | 2 | 2.3 |
| Test-Important-4 (intermediate symlink) | 4 | 4.1 |
| Test-Important-5 (init pointer-error) | 5 | 5.3 |
| Test-Important-6 (atomic-write skip macOS) | n/a — accepted as Linux-CI-only |  |
| Test-Important-7 (cursor-fails / agents-succeeds) | 5 | 5.6 |
| Test-Important-8 (read-only dir) | 5 | 5.7 |
| Comment-1 (mcpconfigs.Action mirror) | 7 | 7.1 |
| Comment-2 (Sync doc symlink) | 2 | 2.2 (subsumed) |
| Comment-3 (LegacyBlocksPurged field doc) | 2 | 2.3 (subsumed) |
| Comment-4 (lock_windows.go ADR posture) | 3 | 3.2 (subsumed) |
| Comment-5 (proto reserved paper-trail) | 7 | 7.3 |
| Suggestion forward-compat marker | 8 | 8.1 |
| Suggestion lift-fslock-shared | n/a — file follow-up bead, do not block |  |
| Suggestion sentinel errors | 2 | 2.4 |
| Suggestion trim URL comment | 6 | 6.2 |
| Suggestion drop "paper trail" framing | 7 | 7.3 (subsumed) |
| Suggestion concurrent test forks process | n/a — defer (TODO comment) |  |
| Suggestion splitFrontmatter trailing blank | n/a — cosmetic, defer |  |
| Suggestion cursor body no-markers test | 8 | 8.2 |

**Items deferred to follow-up beads (will be filed in Phase 9 close-out):**

- Important-9: historical commit broken — note in CONTRIBUTING.md follow-up.
- Test-Important-6: macOS-skip on `TestSync_AtomicWriteOnFailure` — needs FS-injection test harness.
- Suggestion lift-fslock-shared: extract `acquireFileLock` to `internal/fslock/`.
- Suggestion concurrent test forks process: replace 4-goroutine test with `os/exec` fork.
- Suggestion splitFrontmatter trailing-blank normalization.

**Placeholder scan:** None. Every test has full code; every implementation step shows the diff.

**Type consistency:**

- `Sync` signature: `func Sync(projectDir string, opts Options) SyncReport` everywhere from Phase 2 onward.
- `SyncResult` constructors: `errResult`, `okResult(path, action, purged, skipped)`, `noopResult` after Phase 5.5.
- `Unlocker = func() error` after Phase 3.1.
- `atomicWrite(fullPath, data, mode)` after Phase 1.1.

---

## Execution Handoff

**Plan complete and saved to `docs/plans/2026-05-07-pr940-review-fixes-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per phase (or per task within long phases), review between tasks, fast iteration. Best for the multi-phase scope; isolates context per logical change.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review. Best if you want continuous narration.

**Which approach?**
