# CLI Lifecycle Split — Design

> **Date**: 2026-04-22
> **Status**: Draft (revised after independent review)
> **Phase**: 2 (Authoring & CLI Integration)
> **Triggering issue**: `specgraph down` followed by `specgraph up` silently wipes all PostgreSQL data.

## Overview

Today `specgraph up` and `specgraph down` each do two orthogonal things — **runtime lifecycle** (start/stop containers + running service) and **registration** (install/uninstall the launchd plist or systemd unit). The overload forced `--rm` on `down` to mean "uninstall the service definition," which left no clean flag for "destroy data" — and in practice the code path unconditionally runs `docker compose down -v`, destroying the named Postgres volume on every `down`.

This design splits the two concerns into dedicated verbs:

- `install` / `uninstall` — service registration (plist/systemd) only
- `up` / `down` — runtime lifecycle only; data is preserved by default
- `down --purge` — opt-in destructive teardown of containers and volumes, guarded by a confirmation prompt on TTY or `--yes` in scripts

As a side benefit, the same pass addresses a second data-loss path: the `defer ComposeDown(...)` at the end of `specgraph serve` (serve.go:83-87), which currently wipes data whenever the foreground server exits. It is replaced with a non-destructive `ComposeStop`.

### Goals

- Make `specgraph down` non-destructive by default; data survives any number of `up`/`down` cycles.
- Give service registration a first-class, reversible command pair.
- Make destructive teardown a single, clearly-named, confirmation-guarded operation.
- Replace `serve`'s data-destroying exit path with a container-stop that preserves data and cleans up live processes for interactive users.
- Keep `task dev:reset` as the canonical "nuke everything and rebuild" developer path, unchanged.

### Non-Goals

- Changing the `server.mode` config model (`service` vs `manual` stays the same).
- Introducing a new destructive command (`dev:reset` already fills that role).
- Changing the compose file schema or Postgres image.
- Adding new flags to `up`.
- Windows support — `cmd/specgraph/up.go:121-132` has only `darwin` and `default: linux` branches today. Windows is explicitly out of scope; the new `install`/`uninstall` commands inherit the same platform coverage.

## Problem Details

### Bug 1 — `ComposeDown` unconditionally passes `-v`

`internal/docker/compose.go:31`:

```go
cmd := exec.Command("docker", "compose", "-f", composeFile, "down", "--timeout", "10", "-v")
```

`docker compose down -v` removes **named volumes declared in the compose file**. The Postgres data volume is named `specgraph-data` (compose.go:72-81), so every `specgraph down` drops it. The original design doc for `specgraph down` (`docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md:193`) specified `down --timeout 10` with no `-v` — the code regressed from the spec.

### Bug 2 — `serve.go` destroys data on foreground exit

`cmd/specgraph/serve.go:83-87`:

```go
defer func() {
    if downErr := docker.ComposeDown(composeFile); downErr != nil {
        fmt.Fprintf(os.Stderr, "warning: compose down: %v\n", downErr)
    }
}()
```

Every clean exit of `specgraph serve` calls the buggy `ComposeDown` and wipes data. This fires on normal Ctrl-C, not just on errors. `specgraph up` does not install a matching defer, so the two "start" paths have inconsistent teardown semantics.

### Bug 3 — `--force-recreate -V` on every `up`

`internal/docker/compose.go:20`:

```go
cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--wait", "--force-recreate", "-V")
```

`-V` (`--renew-anon-volumes`) is harmless for our named volume but signals the wrong intent. `--force-recreate` unconditionally tears down and recreates the container even when nothing changed, adding latency and spurious churn on every `up`. Not a data-loss bug on its own, but inconsistent with a "preserve state" default.

### Flag overload on `down --rm`

`cmd/specgraph/down.go:28`:

```go
downCmd.Flags().BoolVar(&downRM, "rm", false, "uninstall the service definition after stopping")
```

`--rm` currently means "uninstall the launchd/systemd service definition" — install state, not data. Users reaching for `docker compose down --volumes`-style intuition find nothing; `--rm` is already taken for a different concept. Separating service-def management into its own verb frees `--rm` to be retired, replaced by a single unambiguous `--purge`.

## Proposed Model

### Command matrix

| Command | Touches service def? | Touches containers? | Touches volumes? |
|---------|----------------------|---------------------|------------------|
| `specgraph install` | Register + start | Start (via service/docker) | No |
| `specgraph uninstall` | Stop + unregister | Stop | No |
| `specgraph up` | No | Start | No |
| `specgraph down` | No (but stops running service) | Stop | **No** |
| `specgraph down --purge` | No | Stop + remove | **Yes (data wipe, confirmation-guarded)** |
| `specgraph serve` | No | Start (foreground); exit calls `ComposeStop` | No |
| `task dev:reset` | Unchanged | Stop + remove | Yes (wipe + rebuild) |

### Semantic rules

1. **Registration and runtime are orthogonal.** `install`/`uninstall` manage only the plist/systemd unit file. `up`/`down` manage only running processes and containers.
2. **`up`/`down` in `service` mode use `service.IsInstalled` to detect registration state.** `internal/service/service.go` already exposes this predicate; `up` loads the service iff installed, `down` stops it iff installed. No file-stat probing in command handlers.
3. **Data survives every non-`--purge` path.** A user who never types `--purge` can never lose their graph through lifecycle commands.
4. **`--purge` on `down` is the single, clearly-named, opt-in destructive switch for data.** Retired flag name (`--rm`) is not reintroduced for any future meaning.
5. **`--purge` is confirmation-guarded.** On a TTY, `down --purge` prompts `Destroy all data in specgraph-data volume? [y/N]`. Non-TTY (CI, scripts) requires `--yes` or the command errors without destroying anything.
6. **`serve` stops its container on exit.** A `ComposeStop` defer replaces today's destructive `ComposeDown` defer. Containers are halted (not removed), volumes are untouched, the next `up` or `serve` restarts quickly.

### Command details

#### `specgraph install`

```text
specgraph install
  1. Require server.mode == "service" (error with actionable hint if "manual").
  2. If already installed (service.IsInstalled): ensure loaded and docker is up, exit 0 idempotently.
  3. Generate service definition under destDir (reuses service.Generate).
  4. If server.docker: ComposeUp.
     - On ComposeUp failure: abort before loading the service. Nothing registered yet, nothing to unwind.
  5. service.Install(defPath) — register and load.
     - On service.Install failure: call ComposeStop to avoid orphan containers, delete the definition file, return error.
  6. Health-check loop (reuse existing logic).
  7. Print "SpecGraph installed and running at <url>".
```

Idempotent on success. Ordered so that the cheapest-to-unwind step runs last: once the service is registered, launchd/systemd will restart the binary on failure, so we only register after the container is confirmed healthy.

#### `specgraph uninstall`

```text
specgraph uninstall
  1. Require server.mode == "service" (error if "manual" — nothing to uninstall).
  2. If not installed: print notice, exit 0 idempotently.
  3. service.Uninstall(defPath) — stops service and removes definition.
  4. If server.docker: ComposeStop (preserve container + volume for fast re-install).
  5. Print "SpecGraph uninstalled (data preserved; run `specgraph down --purge` to remove volumes)".
```

The compose file is **not** removed by `uninstall` — preserving it alongside the preserved volume is the whole point of non-destructive uninstall. A subsequent `install` finds the same compose file pointing at the same volume and resumes exactly where the user left off. (Closes Open Question 2 from the draft.)

#### `specgraph up`

```text
specgraph up
  1. Idempotency check: if already healthy, exit 0.
  2. If server.docker: ComposeUp (non-destructive — no --force-recreate, no -V).
  3. If server.mode == "service":
     - If service.IsInstalled: load it (launchctl/systemctl start).
     - Else: print one-line hint "Service not installed — running container only. `specgraph install` to enable auto-start.", continue.
  4. If server.mode == "manual": print "Manual mode: run `specgraph serve` in another terminal."
  5. Health-check loop.
```

Key change: `up` no longer *generates* the service definition or calls `service.Install`. Those moved to `specgraph install`. The "service registered but not running" state is handled explicitly via `service.IsInstalled`; file-stat probes are gone.

#### `specgraph down`

```text
specgraph down [--purge [--yes]]
  1. If server.mode == "service" and service.IsInstalled: service.Stop() (do NOT remove definition).
  2. Without --purge:
     - If server.docker: ComposeStop (halt container, preserve volumes).
     - Print "SpecGraph stopped".
  3. With --purge:
     - If TTY: prompt "Destroy all data in specgraph-data volume? [y/N]". Non-y answer aborts.
     - If non-TTY: require --yes or error ("--purge requires --yes when not on a TTY").
     - If server.docker: ComposeDownWithVolumes (docker compose down -v).
     - Print "SpecGraph stopped and volumes removed".
```

#### `specgraph serve`

Replace the `defer ComposeDown(composeFile)` at serve.go:83-87 with a `defer ComposeStop(composeFile)`. Rationale:

- `serve` in manual mode is often the process the user Ctrl-Cs — with the defer removed entirely, interactive users would accumulate orphan containers across sessions.
- `ComposeStop` halts the container (fast) without removing it or touching volumes. Next `serve` or `up` restarts the container rather than recreating it.
- On start-path failures (ComposeUp fails), there is nothing to stop; the defer is a no-op in that case (`docker compose stop` on an already-stopped stack is idempotent).

### `ComposeUp`/`ComposeStop`/`ComposeDown` changes

`internal/docker/compose.go`:

```go
// ComposeUp: drop --force-recreate and -V.
cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--wait")

// ComposeStop: new — non-destructive pause. Halts containers, keeps them + volumes.
cmd := exec.Command("docker", "compose", "-f", composeFile, "stop", "--timeout", "10")

// ComposeDown: drop -v. Removes containers; keeps named volumes.
cmd := exec.Command("docker", "compose", "-f", composeFile, "down", "--timeout", "10")

// ComposeDownWithVolumes: new — explicit, destructive. Called only by `down --purge`.
cmd := exec.Command("docker", "compose", "-f", composeFile, "down", "--timeout", "10", "-v")
```

Caller map:

| Caller | Function |
|--------|----------|
| `specgraph up` | `ComposeUp` |
| `specgraph install` | `ComposeUp` |
| `specgraph down` (no flag) | `ComposeStop` |
| `specgraph uninstall` | `ComposeStop` |
| `specgraph serve` (defer) | `ComposeStop` |
| `specgraph down --purge` | `ComposeDownWithVolumes` |
| `task dev:reset` | Unchanged — still invokes `docker compose ... down -v` directly in the Taskfile |

## Partial Failure Semantics

Summarized to one place since `install` and the destructive `down --purge` path both have multi-step sequences:

- **`install` failure modes.**
  - `ComposeUp` fails: nothing registered yet, surface the error and exit. User re-runs `install` after fixing Docker.
  - `service.Install` fails after successful `ComposeUp`: call `ComposeStop` so the container doesn't stay running under a half-installed service, delete the generated definition file, return error.
  - Health-check fails after both registered and container up: leave the install in place (the service will keep trying), return error with a diagnostic suggesting `specgraph logs` (or `journalctl --user -u specgraph` on Linux).
- **`down --purge` confirmation aborted.** Nothing is destroyed; command exits 0 with "aborted — no changes made."
- **`down --purge` after partial `ComposeDown`.** `docker compose down -v` is atomic enough at the volume level — either the volume is gone or it isn't. No mid-state to recover.
- **`uninstall` during active RPCs.** No explicit connection draining. The service manager (launchd/systemd) sends SIGTERM via `service.Uninstall`; the server process must exit cleanly on SIGTERM within the standard service-manager grace period. In-flight RPCs may be cut. Acceptable for now — this server has no long-lived streaming RPCs; re-evaluate if/when MCP or watch endpoints land. Tracked as a follow-up consideration, not a blocker.

## Shared-State Caveat Across jj Workspaces

`$XDG_DATA_HOME/specgraph/` is **per-user, not per-workspace**. Two jj workspaces of this repo — which CLAUDE.md explicitly prescribes for parallel work — share a single compose file and a single Postgres volume. Consequences:

- `specgraph up` in workspace A and workspace B connect to the same database by default.
- `specgraph down --purge` in workspace A destroys the data workspace B is using.
- `specgraph uninstall` in workspace A stops the service workspace B was relying on.

This is unchanged from today's behavior; the design neither makes it better nor worse. It is called out here because:

1. The destructive path (`--purge`) is newly-named and newly-guarded, so users may reach for it more willingly than the current opaque `--rm`.
2. The confirmation prompt should make the shared-state footgun visible: wording is `Destroy all data in specgraph-data volume? All SpecGraph workspaces on this machine share this volume. [y/N]`. The second sentence is the user-visible mitigation for the jj case.
3. Per-workspace isolation (separate XDG roots per repo checkout) is a follow-up, tracked as a beads issue from this design — not a blocker.

## Migration & Backward Compatibility

### Single breaking change: retire `--rm`, introduce `--purge`

`--rm` on `down` is **removed entirely** in this change and never reinstated. Running `specgraph down --rm` after this change prints:

```text
Error: --rm has been retired.
  To remove the service definition:   specgraph uninstall
  To remove containers and data:       specgraph down --purge
Run `specgraph --help` for details.
```

The error is permanent — `--rm` never resurfaces with a new meaning. This avoids the silent-reversal footgun (which a four-phase flip-with-alias would have carried) at the cost of a loud, one-line error that points users at the two correct new commands.

Rationale for rejecting the multi-phase flip-and-rename-back:

- Pre-1.0 project; no tests depend on `--rm`'s destructive side effect (verified — no test file references `ComposeDown` or `ComposeUp`).
- `docker compose down -v` uses `--volumes`, not `--rm` — so the "Docker muscle memory" argument actually *weakens* the case for preserving `--rm` as a destructive flag.
- Three known documentation references to `--rm` exist (slice-7 design, bootstrap-funnel-demo design, CLI reference), one of which was ambiguously/incorrectly worded anyway. Updating three doc sites is lower effort than shepherding users through a multi-release flip.

### Non-breaking

- `server.mode` and `server.docker` config stay as-is.
- `task dev:reset` is unchanged and continues to be the canonical wipe.
- `specgraph up` with no flags behaves the same on the happy path for users who haven't typed `install` — the service auto-install step is gone, but manual-mode and docker-only users see no difference.

### Users who relied on `up` auto-installing the service

`up` no longer calls `service.Install`. Users who relied on the implicit install now need to run `install` once. Mitigation:

- First-run detection via `service.IsInstalled`: if `server.mode == "service"` and not installed, `up` prints a one-line hint.
- Release notes + docs call this out explicitly.

## File-by-File Impact

### Code

| File | Change |
|------|--------|
| `internal/docker/compose.go` | Add `ComposeStop` and `ComposeDownWithVolumes`. Drop `-v` from `ComposeDown`. Drop `--force-recreate -V` from `ComposeUp`. |
| `cmd/specgraph/up.go` | Remove service-definition generation + `service.Install` call. Use `service.IsInstalled` for the branch. |
| `cmd/specgraph/down.go` | Retire `--rm` (permanent error). Add `--purge` and `--yes` flags. Wire TTY/non-TTY confirmation. Wire `ComposeStop`/`ComposeDownWithVolumes`. |
| `cmd/specgraph/install.go` | **New.** Owns service definition generation + `service.Install` + `ComposeUp` + partial-failure unwinds per §"Partial Failure Semantics". |
| `cmd/specgraph/uninstall.go` | **New.** Owns `service.Uninstall` + `ComposeStop`. Compose file is intentionally preserved. |
| `cmd/specgraph/serve.go` | Swap `defer ComposeDown` at lines 83-87 to `defer ComposeStop`. |
| `cmd/specgraph/docs.go:57` | Add `install` and `uninstall` to the `commandGroups` "Server & Config" entry so `task docs:cli` regenerates `site/docs/cli-reference.md` with the new commands. |

### Docs

| File | Change |
|------|--------|
| `README.md:122` | Expand the command table: `up`/`down` row + new `install`/`uninstall` row. |
| `site/docs/cli-reference.md:856-869` | Expand `up`/`down`. Add `install`/`uninstall` sections. Document `--purge` new meaning and confirmation semantics. Document `--rm` retirement. |
| `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md:151-193` | Note: superseded for the CLI surface; cross-link to this design. Leave the original text for historical record. |
| `docs/superpowers/specs/2026-03-21-bootstrap-funnel-demo-design.md:149` | Replace `specgraph down --rm` with the correct verb — `specgraph uninstall` for tutorial cleanup, `task dev:reset` if a full wipe is intended. Confirm during implementation. |
| `site/docs/quickstart.md:70` and related | Audit for "wipe between runs" language that assumed the old destructive behavior. |

### Tests

No existing tests exercise the `ComposeDown -v` path (verified via grep). New tests:

- `cmd/specgraph/install_test.go` + `uninstall_test.go` — happy path, mode-mismatch errors, `install` partial-failure unwind (mock `service.Install` returning error after a fake `ComposeUp` success; assert `ComposeStop` was called and definition file was removed).
- `cmd/specgraph/down_test.go` — `--rm` prints retirement error and exits non-zero without touching docker; `--purge` without `--yes` on non-TTY errors; `--purge --yes` on non-TTY calls `ComposeDownWithVolumes`; TTY path covered via a PTY helper or mocked stdin.
- `internal/docker/compose_test.go` — assert the exact `exec.Command` argv produced by each wrapper (catches future regressions of the `-v` kind).

## Rejected Alternatives

### A. Minimal fix: drop `-v`, leave everything else

Correctly rejected as the *complete* fix, but the Plan section ships the minimal fix first (as Phase 1) so the reported bug is resolved ahead of the ergonomic redesign. The full design is an ergonomics cleanup built on top of the bug fix, not a gate in front of it.

### B. Keep `--rm` = uninstall, add `--purge` for data

Non-breaking but leaves `up`/`down` still doing two jobs each and wastes the clearest flag name (`--rm`) on service-def removal. The install/uninstall split is what makes the CLI surface comprehensible; keeping `--rm` as an alias for `uninstall` would undercut that.

### C. Add `up --no-install` as a mirror to "don't auto-install on up"

Rejected. Once `install` is a separate verb, `up` never installs, so `--no-install` is a negation of a default that no longer exists.

### D. Four-phase `--rm` migration with transitional `--purge-data` alias

Rejected in the revision. Four phases across unspecified release cadence, with a hidden alias that exists for one release and then vanishes, adds friction for users without meaningfully reducing surprise. A single permanent retirement of `--rm` is louder but cleaner.

### E. Remove `serve`'s exit defer entirely

Rejected in the revision. Leaves orphan containers after every interactive manual-mode session. Replaced with `ComposeStop` (this design's proposal), which halts without destroying and preserves fast restart.

## Open Questions

1. **Does `install` start the service immediately, or only register it?** Current spec: register + start (matches `brew services start`). Alternative: register only; require `up` to actually launch. The former is more ergonomic for the primary use case and consistent with the existing `specgraph up` one-command experience. Recommend keeping as spec'd.
2. ~~**Does `uninstall` remove the compose file?**~~ **Resolved**: no. The compose file is preserved so a subsequent `install` resumes against the same preserved volume. See `uninstall` spec.
3. **Which teardown does the funnel-demo spec actually want?** `docs/superpowers/specs/2026-03-21-bootstrap-funnel-demo-design.md:149` says `specgraph down --rm` for "teardown … clean up." Confirm during implementation whether the intent is service-def removal (`uninstall`), data wipe (`down --purge`), or a full dev reset (`task dev:reset`). Affects one doc line and potentially the tutorial's observable behavior.

## Plan

A companion `2026-04-22-cli-lifecycle-split-plan.md` will sequence the implementation. **Phase 1 is shippable on its own and resolves the reported data-loss bug ahead of the larger redesign.**

1. **Phase 1 — Bug fix (shippable immediately, self-contained PR).**
   - Drop `-v` from `ComposeDown`.
   - Drop `--force-recreate -V` from `ComposeUp`.
   - Swap `serve.go:83-87` defer from `ComposeDown` to `ComposeStop` (adds `ComposeStop` as the new non-destructive wrapper).
   - Add `internal/docker/compose_test.go` asserting argv for each wrapper.
   - No CLI surface change. No breaking change. Fixes the reported bug.
2. **Phase 2 — CLI surface redesign (breaking, builds on Phase 1).**
   - Add `install`/`uninstall` commands.
   - Retire `--rm` on `down` with the permanent error; add `--purge` + `--yes`.
   - Implement TTY-aware confirmation prompt.
   - Move service-install logic out of `up`; add `service.IsInstalled` branch.
   - Add `ComposeDownWithVolumes` as a second wrapper (only `down --purge` uses it).
   - Doc updates across README, CLI reference, and the slice-7 design cross-link.
3. **Phase 3 — Follow-up (separate change, not in this branch).**
   - Beads issue: investigate per-workspace XDG isolation to fix the shared-volume footgun across jj workspaces (documented here; no behavior change in this branch).
