# CLI Lifecycle Split — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the `specgraph down` data-loss bug and split the CLI surface so registration (`install`/`uninstall`) is orthogonal to runtime (`up`/`down`), with a single confirmation-guarded destructive flag (`down --purge`).

**Design spec:** `docs/plans/2026-04-22-cli-lifecycle-split-design.md`

**Sequencing:**

- **Phase 1** is a self-contained bug fix and should land as its own PR **ahead of Phase 2**. It stops the bleeding (data loss on every `down`) without any CLI surface change.
- **Phase 2** is the breaking CLI redesign. It depends on Phase 1 but is otherwise orthogonal.
- **Phase 3** is filed as a beads follow-up (per-workspace XDG isolation) and is **not** implemented in this branch.

**Tech Stack:** Go, cobra (existing), `docker compose` shell-out (existing), `golang.org/x/term` (new direct dep; already transitive).

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/docker/compose.go` | Drop `-v` / `--force-recreate -V` / `-V`. Add `ComposeStop` + `ComposeDownWithVolumes`. |
| Create | `internal/docker/compose_test.go` | Argv assertion tests for every wrapper. |
| Modify | `cmd/specgraph/serve.go` | Swap `defer ComposeDown` → `defer ComposeStop`. |
| Modify | `cmd/specgraph/down.go` | Retire `--rm`. Add `--purge` + `--yes`. Wire TTY prompt. |
| Create | `cmd/specgraph/down_test.go` | Flag retirement, TTY/non-TTY branches, wrapper call assertions. |
| Create | `cmd/specgraph/install.go` | New verb: registration + docker up + partial-failure unwind. |
| Create | `cmd/specgraph/install_test.go` | Happy path, mode-mismatch error, partial-failure unwind. |
| Create | `cmd/specgraph/uninstall.go` | New verb: registration removal + `ComposeStop`. |
| Create | `cmd/specgraph/uninstall_test.go` | Happy path, mode-mismatch error, compose-file preservation. |
| Modify | `cmd/specgraph/up.go` | Remove service-install. Use `service.IsInstalled`. |
| Modify | `cmd/specgraph/up_test.go` (or add if absent) | New branch coverage. |
| Create | `cmd/specgraph/confirm.go` | TTY prompt + `--yes` helper (shared by `down --purge` and any future destructive command). |
| Create | `cmd/specgraph/confirm_test.go` | TTY branch via io.Reader injection, non-TTY error path. |
| Modify | `cmd/specgraph/docs.go:57` | Add `install` + `uninstall` to "Server & Config" `commandGroups` entry. |
| Modify | `site/docs/cli-reference.md` | Regenerated via `task docs:cli`. |
| Modify | `README.md:122` | Update command table. |
| Modify | `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md` | Cross-link to this design; mark superseded for CLI surface. |
| Modify | `docs/superpowers/specs/2026-03-21-bootstrap-funnel-demo-design.md:149` | Replace `specgraph down --rm` with correct verb. |
| Modify | `go.mod` | Promote `golang.org/x/term` to direct require. |

---

## Phase 1 — Bug Fix (shippable PR, no CLI surface change)

### Task 1.1 — Add argv-assertion tests for current wrappers

**Files:** Create `internal/docker/compose_test.go`.

Write tests **first** that fail against the current code, by asserting the argv each wrapper *should* produce. This is the TDD red step.

- [ ] **Step 1:** Extract argv generation behind a seam. Refactor `ComposeUp` and `ComposeDown` so the `exec.Command` args are produced by a pure function (e.g., `composeUpArgs(composeFile string) []string`). Callers still invoke via `exec.Command("docker", args...)`.
- [ ] **Step 2:** Write failing tests:

```go
func TestComposeUpArgs(t *testing.T) {
    got := composeUpArgs("/tmp/compose.yaml")
    want := []string{"compose", "-f", "/tmp/compose.yaml", "up", "-d", "--wait"}
    if !reflect.DeepEqual(got, want) { t.Fatalf("got %v, want %v", got, want) }
}

func TestComposeDownArgs(t *testing.T) {
    got := composeDownArgs("/tmp/compose.yaml")
    want := []string{"compose", "-f", "/tmp/compose.yaml", "down", "--timeout", "10"}
    // NOTE: no -v. Regression guard for the data-loss bug.
    if !reflect.DeepEqual(got, want) { t.Fatalf("got %v, want %v", got, want) }
}
```

- [ ] **Step 3:** Run `go test ./internal/docker/...` — confirm both tests fail with the specific expected-vs-got mismatch. Red state confirmed.

### Task 1.2 — Fix `ComposeUp` and `ComposeDown`

**Files:** Modify `internal/docker/compose.go`.

- [ ] **Step 1:** Drop `--force-recreate` and `-V` from `composeUpArgs`. Keep `-d --wait`.
- [ ] **Step 2:** Drop `-v` from `composeDownArgs`. Keep `--timeout 10`.
- [ ] **Step 3:** Run `go test ./internal/docker/...` — tests from 1.1 now pass. Green state.
- [ ] **Step 4:** Smoke test manually: `go run ./cmd/specgraph up; go run ./cmd/specgraph down; go run ./cmd/specgraph up` — verify Postgres volume survives the cycle (`docker volume inspect specgraph_specgraph-data` shows same `CreatedAt` before and after).

### Task 1.3 — Add `ComposeStop`

**Files:** Modify `internal/docker/compose.go`, update `internal/docker/compose_test.go`.

- [ ] **Step 1:** Write failing test for `composeStopArgs`:

```go
func TestComposeStopArgs(t *testing.T) {
    got := composeStopArgs("/tmp/compose.yaml")
    want := []string{"compose", "-f", "/tmp/compose.yaml", "stop", "--timeout", "10"}
    if !reflect.DeepEqual(got, want) { t.Fatalf(...) }
}
```

- [ ] **Step 2:** Implement `ComposeStop` (mirror `ComposeDown` structure; call `composeStopArgs`).
- [ ] **Step 3:** Godoc: `// ComposeStop halts the stack without removing containers or volumes. Idempotent — already-stopped stacks exit 0.`
- [ ] **Step 4:** Run `task lint` and `go test ./internal/docker/...`.

### Task 1.4 — Swap `serve.go` defer

**Files:** Modify `cmd/specgraph/serve.go`.

- [ ] **Step 1:** Locate the `defer` block at `serve.go:83-87`.
- [ ] **Step 2:** Replace `docker.ComposeDown(composeFile)` with `docker.ComposeStop(composeFile)`. Keep the warning-log pattern.
- [ ] **Step 3:** Verify no other callers of `ComposeDown` in `cmd/` (grep `ComposeDown`).
- [ ] **Step 4:** Smoke test: `go run ./cmd/specgraph serve` in a terminal, Ctrl-C, then `docker ps -a` — confirm container is `Exited (0)` not removed, and `docker volume inspect` shows the same volume.

### Task 1.5 — Run full quality gate

- [ ] **Step 1:** `task check` (fmt, license, lint, build, unit tests).
- [ ] **Step 2:** Skim `jj diff` — only four files changed (`internal/docker/compose.go`, new `compose_test.go`, `cmd/specgraph/serve.go`). No CLI surface change. No doc changes.
- [ ] **Step 3:** Commit with Phase 1 scope: `fix(docker): preserve postgres volume across down/up cycles`.
- [ ] **Step 4:** Open PR. **This PR is independently mergeable.** Phase 2 can start after it lands (or on a rebase branch in parallel).

---

## Phase 2 — CLI Surface Redesign (breaking, builds on Phase 1)

### Task 2.1 — Confirmation helper

**Files:** Create `cmd/specgraph/confirm.go`, `cmd/specgraph/confirm_test.go`.

Small, shared, testable. Pulled out so `down --purge` isn't the only site that has to reason about TTY detection.

- [ ] **Step 1:** Write failing tests first:

```go
// confirm_test.go
func TestConfirmNonTTYRequiresYes(t *testing.T) {
    // reader with no data simulates non-TTY stdin; isTTY forced false via test seam.
    err := confirmDestructive(strings.NewReader(""), false, false /*yes*/, "msg")
    if err == nil || !strings.Contains(err.Error(), "--yes") { t.Fatalf(...) }
}

func TestConfirmNonTTYWithYesPasses(t *testing.T) {
    err := confirmDestructive(strings.NewReader(""), false, true, "msg")
    if err != nil { t.Fatalf(...) }
}

func TestConfirmTTYPromptsAndAcceptsY(t *testing.T) {
    err := confirmDestructive(strings.NewReader("y\n"), true, false, "msg")
    if err != nil { t.Fatalf(...) }
}

func TestConfirmTTYPromptsAndRejectsDefault(t *testing.T) {
    err := confirmDestructive(strings.NewReader("\n"), true, false, "msg")
    if err == nil || !strings.Contains(err.Error(), "aborted") { t.Fatalf(...) }
}
```

- [ ] **Step 2:** Implement:

```go
// confirm.go

// confirmDestructive prompts on a TTY or requires --yes otherwise.
// Returns nil on confirmation, an error (with a user-facing message) on abort or missing --yes.
func confirmDestructive(in io.Reader, isTTY, yes bool, prompt string) error {
    if yes { return nil }
    if !isTTY {
        return errors.New("destructive operation requires --yes when not on a TTY")
    }
    fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
    scanner := bufio.NewScanner(in)
    if !scanner.Scan() { return errors.New("aborted — no input") }
    ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
    if ans != "y" && ans != "yes" {
        return errors.New("aborted — no changes made")
    }
    return nil
}
```

- [ ] **Step 3:** In production, `isTTY` is computed via `term.IsTerminal(int(os.Stdin.Fd()))`. Add `golang.org/x/term` to `go.mod` as a direct require.

### Task 2.2 — Retire `--rm`, add `--purge` + `--yes` on `down`

**Files:** Modify `cmd/specgraph/down.go`, create `cmd/specgraph/down_test.go`.

- [ ] **Step 1:** Remove the `--rm` flag registration. Add `--purge` and `--yes`.
- [ ] **Step 2:** Add a pre-run check: if `os.Args` contains `--rm`, exit with the retirement message from the design doc §Migration. Cobra's built-in `MarkDeprecated` prints a warning and still accepts the flag — wrong behavior here. Instead, define `--rm` as a hidden bool flag whose handler immediately errors. This keeps cobra's parser happy and gives us the exact retirement message.

```go
var downRMRetired bool
downCmd.Flags().BoolVar(&downRMRetired, "rm", false, "")
_ = downCmd.Flags().MarkHidden("rm")

// in runDown:
if downRMRetired {
    return errors.New(`--rm has been retired.
  To remove the service definition:   specgraph uninstall
  To remove containers and data:       specgraph down --purge`)
}
```

- [ ] **Step 3:** Wire the new logic:

```go
if downPurge {
    isTTY := term.IsTerminal(int(os.Stdin.Fd()))
    msg := "Destroy all data in specgraph-data volume? All SpecGraph workspaces on this machine share this volume."
    if err := confirmDestructive(os.Stdin, isTTY, downYes, msg); err != nil {
        return err
    }
    // ... docker.ComposeDownWithVolumes ...
}
```

- [ ] **Step 4:** Replace the unconditional `service.Uninstall` path (current behavior tied to `--rm`) with `service.Stop` only. Uninstall is no longer a side effect of `down`.
- [ ] **Step 5:** Replace `docker.ComposeDown` (non-purge) with `docker.ComposeStop` per §File-by-File caller map.
- [ ] **Step 6:** Tests in `down_test.go`:
  - `--rm` errors with retirement message, no docker call.
  - `--purge` on non-TTY without `--yes` errors, no docker call.
  - `--purge --yes` calls `ComposeDownWithVolumes`.
  - No flags calls `ComposeStop` and `service.Stop`.
  - Mock `service` and `docker` via small interface seams — don't shell out in tests.

### Task 2.3 — Add `ComposeDownWithVolumes`

**Files:** Modify `internal/docker/compose.go`, update `internal/docker/compose_test.go`.

- [ ] **Step 1:** Write failing test for argv: expect `["compose", "-f", <path>, "down", "--timeout", "10", "-v"]`.
- [ ] **Step 2:** Implement wrapper. Godoc: `// ComposeDownWithVolumes tears down the stack AND removes named volumes. Destructive — callers must confirm with the user.`
- [ ] **Step 3:** Only caller is `cmd/specgraph/down.go` under the `--purge` branch. Verify via grep.

### Task 2.4 — Create `install` command

**Files:** Create `cmd/specgraph/install.go`, `cmd/specgraph/install_test.go`.

- [ ] **Step 1:** Write failing tests first (install_test.go):
  - Mode-mismatch: `server.mode == "manual"` returns error with actionable hint.
  - Idempotent: `service.IsInstalled` returning true short-circuits (ensure loaded, docker up, exit 0).
  - Happy path: calls `ComposeUp` then `service.Install`.
  - Partial-failure: `ComposeUp` fails → no `service.Install` call, error surfaces.
  - Partial-failure: `service.Install` fails after `ComposeUp` success → `ComposeStop` called, definition file removed, error surfaces.
- [ ] **Step 2:** Extract the service-install block from `up.go:64-93` (the `case "service":` branch) into `install.go`. Adjust:
  - Add the idempotency check via `service.IsInstalled`.
  - Reorder: `ComposeUp` first, then `service.Install`, so unwind is clean.
  - Add the partial-failure unwind (delete the definition file on `service.Install` error; call `ComposeStop` to avoid orphans).
- [ ] **Step 3:** Register `installCmd` on `rootCmd` in `init()`.

### Task 2.5 — Create `uninstall` command

**Files:** Create `cmd/specgraph/uninstall.go`, `cmd/specgraph/uninstall_test.go`.

- [ ] **Step 1:** Tests first:
  - Mode-mismatch errors (manual mode has nothing to uninstall).
  - Not installed: prints notice, exits 0 idempotently.
  - Happy path: calls `service.Uninstall` then `docker.ComposeStop`.
  - Compose file is **not** removed (assert file still exists after uninstall).
- [ ] **Step 2:** Implement mirroring the design §uninstall spec. The prior `down --rm` uninstall path from `down.go` moves here.
- [ ] **Step 3:** Register on `rootCmd`.

### Task 2.6 — Refactor `up.go`

**Files:** Modify `cmd/specgraph/up.go`.

- [ ] **Step 1:** Remove the entire `case "service":` block that calls `service.Generate` and `service.Install` (lines 65-89 currently).
- [ ] **Step 2:** Replace with a service-state check using `service.IsInstalled`:

```go
case "service":
    installed, err := service.IsInstalled(defPath)
    if err != nil { return fmt.Errorf("check install state: %w", err) }
    if installed {
        if err := service.Load(defPath); err != nil { return fmt.Errorf("load service: %w", err) }
    } else {
        fmt.Println("Service not installed — running container only. `specgraph install` to enable auto-start.")
    }
```

- [ ] **Step 3:** Verify `service.Load` exists or add it as a thin wrapper (today `service.Install` bundles generate+load; we need load-only for the `up` code path when definition already exists). Check `internal/service/` for this function and add if missing.
- [ ] **Step 4:** Tests: add branch coverage for the "installed → load" and "not installed → hint" paths.

### Task 2.7 — Register commands in `docs.go`

**Files:** Modify `cmd/specgraph/docs.go:57`.

- [ ] **Step 1:** Update `commandGroups` entry:

```go
{Name: "Server & Config", Commands: []string{"up", "down", "install", "uninstall", "serve", "mcp", "status", "health", "init", "prime", "inject"}},
```

- [ ] **Step 2:** Regenerate: `task docs:cli` (or whatever the Taskfile target is for `cmd/specgraph/docs.go`'s generator — check `Taskfile.yml`).
- [ ] **Step 3:** Confirm `site/docs/cli-reference.md` now contains the new commands.

### Task 2.8 — Doc updates

- [ ] **Step 1:** `README.md:122` — add rows for `install`/`uninstall`; update `down` row to mention `--purge`.
- [ ] **Step 2:** `site/docs/cli-reference.md:856-869` — extend `up`/`down` sections; regenerate from cobra via Task 2.7.
- [ ] **Step 3:** `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md:151-193` — prepend a note: "Superseded for the CLI surface by `2026-04-22-cli-lifecycle-split-design.md`. Kept for historical record."
- [ ] **Step 4:** `docs/superpowers/specs/2026-03-21-bootstrap-funnel-demo-design.md:149` — replace `specgraph down --rm` with the correct verb (confirm with stakeholder; likely `specgraph uninstall` for tutorial cleanup, possibly `task dev:reset`).
- [ ] **Step 5:** Audit `site/docs/quickstart.md` for any implicit assumption that `down` wipes data.

### Task 2.9 — File a beads issue for Phase 3 (XDG per-workspace isolation)

- [ ] **Step 1:** `bd create --title="Per-workspace XDG isolation to prevent shared-volume footgun across jj workspaces" --description=... --type=feature --priority=3`
- [ ] **Step 2:** Reference this design doc in the issue description.

### Task 2.10 — Full quality gate

- [ ] **Step 1:** `task check` passes.
- [ ] **Step 2:** `task pr-prep` passes (requires Docker — exercises integration + e2e).
- [ ] **Step 3:** Manual smoke test matrix:
  - `specgraph install` on fresh machine → service registered, container up, `specgraph health` green.
  - `specgraph uninstall` → service gone, compose file preserved, volume preserved.
  - `specgraph install` again → picks up the preserved volume; data from prior session intact.
  - `specgraph down` → container stopped, volume preserved.
  - `specgraph up` → container back, data intact.
  - `specgraph down --rm` → retirement error, no state changed.
  - `specgraph down --purge` on a TTY → prompt appears, typing `n` aborts cleanly.
  - `specgraph down --purge --yes` on non-TTY → proceeds, volume removed.
- [ ] **Step 4:** Commit with Phase 2 scope: `feat(cli)!: split install/uninstall from up/down; retire --rm`. (Conventional-commit breaking-change marker.)

---

## Out of Scope (Phase 3, filed as beads)

- Per-workspace XDG data dir (design doc §Shared-State Caveat).
- Active-RPC draining on `uninstall` (design doc §Partial Failure Semantics). Reassess if/when long-lived streaming RPCs land.
- Windows support for `install`/`uninstall` (inherits current platform coverage).

## References

- Design: `docs/plans/2026-04-22-cli-lifecycle-split-design.md`
- Superseded CLI surface section: `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md:151-193`
- Triggering bug in code: `internal/docker/compose.go:31` (`-v` on `ComposeDown`)
- Second data-loss path: `cmd/specgraph/serve.go:83-87` (destructive defer)
