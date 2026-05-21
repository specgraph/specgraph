# spgr-rwrp PR G — `specgraph doctor` + drift nudge + dogfood plumbing

**Date:** 2026-05-20
**Bead:** spgr-hdki
**Status:** Design (post-brainstorm; pre-implementation)
**Parent design:** [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md) — §"Doctor + drift nudge" (lines 205-258), §"PR G" (lines 373-379), §"Dogfood discipline".
**Predecessors:** PRs 0/A/B/C/D/E/F — Claude API verification, `internal/config/managedfiles/` foundation, mcpconfigs/pointers port, OpenCode plugin, Cursor rule files, Claude plugin shim, skills via MCP.

## Problem

After PR F, SpecGraph ships 14 managed files across three harnesses (Claude, Cursor, OpenCode) with three strategies (`JSONKeyMerge`, `MarkdownBlock`, `WholeFile`). The framework can `Inspect` any of them and classify into `Missing`/`Synced`/`Stale`/`Drifted`, and `Sync` either creates fresh canonical content or surfaces user edits for review. But none of that capacity is exposed to end users today:

1. There is no `specgraph doctor` — a user whose `.claude/settings.json` got hand-edited has no way to learn that the SpecGraph blocks are now out of date without re-reading the design docs.
2. There is no drift signal at all on day-to-day specgraph subcommand invocations — a user can run `specgraph spec list` for weeks while their managed files silently rot relative to a newer binary version.
3. The existing `specgraph health` command is server-only and disconnected from this framework; its exit codes are part of a script contract but the surface is too narrow for the post-PR-F integration story.
4. The dogfood loop is honor-system: a contributor who edits `plugin/<harness>/...` canonicals must remember to `task build` + `specgraph init` before testing in the harness. Forgotten rebuilds rot silently between PRs.

PR G closes all four loops in one PR.

## Approach

Land in eight focused commits, each green under `task check`. Two pieces of plumbing the parent epic mentions are already shipped (`xdg.CacheHome` at `internal/xdg/xdg.go:54`, and the `dev` build-tag with `source_release.go`/`source_dev.go` reading from `SPECGRAPH_DEV_SOURCE_ROOT`) so PR G consumes them rather than landing them.

1. Extend `ProjectConfig` (`internal/config/project.go`) with `Harnesses []string` and `Nudges struct { Quiet bool }` keys, plus strict YAML decoding (`yaml.Decoder.KnownFields(true)`) so unknown top-level keys surface as errors. Update `cmd/specgraph/init.go:117` to read `ProjectConfig.Harnesses` when set, falling back to the hard-coded all-three list otherwise. Adds `FileState.Harness Harness` to `internal/config/managedfiles/types.go` so doctor's `--harness` filter and JSON output can attribute each row.
2. Add `--check` flag to `specgraph init` that combines with the existing dry-run behaviour: with `--check` (and no actual write), exit non-zero if any managed file would change. Required by `task plugin:check`.
3. Scaffold `specgraph doctor` cobra command with the Binary group only. Establishes the `DoctorReport` struct, compact-vs-expanded text rendering, `--exit-zero`/`--verbose`/`--json` flags, and the exit-code policy.
4. Add the Project config group: parse `.specgraph.yaml` via the new strict path; each entry in `Harnesses:` must resolve to a known `Harness` enum.
5. Add the Server group: Connect Health RPC + MCP Streamable-HTTP initialize handshake + `specgraph_skills_list` count, with a `--timeout` flag (default 2s) shared by the Connect and MCP clients (each built fresh; ConnectRPC and MCP use different endpoints/transports — the "single client" notion was sloppy).
6. Add the Managed files group via `InspectAll` + `--fix` for Stale/Missing + guidance lines for Drifted + `--harness` narrowing. Doctor classifies rows as host-pinned vs SpecGraph-owned by **path prefix** (anything under `.specgraph/agents/` is SpecGraph-owned; everything else is host-pinned) — no new field on `ManagedFile`, no manifest validator ripple.
7. Deprecate `specgraph health` as a thin alias for `specgraph doctor server` that prints a deprecation notice to stderr and dispatches. Preserves the existing health exit codes for scripts. (`specgraph doctor server` itself never prints the notice — direct users of the new path see clean output.)
8. Add the drift-nudge `PersistentPreRun` on the root cobra command. Reads `ProjectConfig` once to get the harness list and `nudges.quiet`. Uses `xdg.CacheHome()/nudges/<sha256(EvalSymlinks(projectRoot))>-<binaryVersionHash>` as the 24h-throttle file. Subcommand allow-list, isatty gate, opportunistic GC of >30d entries, unthrottled fallback on any internal error. Also adds `task plugin:refresh` and `task plugin:check` to `Taskfile.yml` and inserts `plugin:check` into the `task check` cmds sequence between `lint` and `skills:validate`.

The 14-file managed-file surface is fully introspected by PR A's `InspectAll` and PR E/F's strategy machinery — PR G is essentially a UI layer on top of existing infrastructure plus the `PersistentPreRun` hook + `ProjectConfig` schema extension.

## Public surface

```text
specgraph doctor [flags]
  --json              Machine-readable output (full structure, never compacted)
  --fix               Auto-init for Stale/Missing; print guidance for Drifted
  --harness <name>    Narrow Managed Files group to one harness (claude | cursor | opencode)
  --verbose           Force per-row expansion of all four groups
  --exit-zero         Always exit 0 (advisory-only mode)
  --timeout <dur>     Per-RPC timeout for the Server group (default 2s)

specgraph doctor server    Run only the Server group (used by `health` alias)
specgraph health           Deprecated alias for `specgraph doctor server`
                           (keeps exit codes for existing scripts)
```

**Exit codes** (for `specgraph doctor` and its subcommands, unless `--exit-zero` is passed):

| Code | Meaning |
|------|---------|
| 0 | All groups healthy. Every managed file Synced. Server reachable. Project config parses. |
| 1 | At least one group reports a problem: any file Missing/Stale/Drifted, server unreachable, MCP handshake fails, config invalid, or a `harnesses:` entry doesn't resolve to a known `Harness`. |
| 2 | Infrastructure failure: can't read the project root, can't dial the server, `~/.cache/` unwritable for non-throttle reasons, etc. Distinct from "user has drift" so CI can tell apart "user fix needed" from "environment broken". |

`--scope` flag is **deferred to a follow-up bead** per parent design (user/all scopes not yet specified).

## Output format

**Compact-when-green** by default; expanded-when-problems automatically; `--verbose` forces full expansion; `--json` always emits the full structure.

### Clean run (compact)

```text
specgraph doctor
Binary:         OK (v0.7.3, built 2026-05-20T14:30:00Z from 6af3b015)
Server:         OK (reachable v0.7.3 · MCP handshake OK · 6 skills)
Project config: OK (3 harnesses enabled: claude, cursor, opencode)
Managed files:  14/14 synced
```

Exit 0. Five lines total.

### Problem run (mixed, partial expansion)

```text
specgraph doctor
Binary:         OK (v0.7.3, built 2026-05-20T14:30:00Z from 6af3b015)
Server:         OK (reachable v0.7.3 · MCP handshake OK · 6 skills)
Project config: OK (3 harnesses enabled)
Managed files:  12/14 synced — 1 stale, 1 drifted

  Strategy        Path                                                       State    Detail
  ─────────────── ────────────────────────────────────────────────────────── ──────── ──────────────────────────────────
  WholeFile       .specgraph/agents/claude/.claude-plugin/marketplace.json   Stale    sentinel matches; canonical newer
  MarkdownBlock   AGENTS.md                                                  Drifted  block bytes diverge from sentinel

Run `specgraph doctor --fix` to refresh stale/missing files.
For drifted files, run `specgraph init --force --keep-edits <path>` (keep your changes)
or `specgraph init --force <path>` (discard your changes) per file.
```

Exit 1. Only the Managed files section expands; the three healthy groups stay compact.

### `--verbose` (forced full expansion)

Every group prints its detail rows even when healthy. Useful for CI logs that want a per-row record.

### `--json` (machine-readable)

Always full structure. Example:

```json
{
  "exitCode": 0,
  "groups": {
    "binary": {
      "ok": true,
      "version": "0.7.3",
      "builtAt": "2026-05-20T14:30:00Z",
      "commit": "6af3b015"
    },
    "server": {
      "ok": true,
      "reachable": true,
      "version": "0.7.3",
      "mcpHandshake": "ok",
      "skillsCount": 6
    },
    "projectConfig": {
      "ok": true,
      "harnesses": ["claude", "cursor", "opencode"]
    },
    "managedFiles": {
      "ok": true,
      "synced": 14,
      "total": 14,
      "files": [
        {"path": "...", "strategy": "WholeFile", "state": "Synced", "harness": "claude"},
        ...
      ]
    }
  }
}
```

Schema is stable; new fields may be added but existing ones won't change shape. The Go struct producing this JSON lives at `cmd/specgraph/doctor.go:DoctorReport`.

## Check group details

### Binary

Inputs: build-time `ldflags`-injected `Version`, `BuildTime`, `Commit` (already populated for `specgraph health`'s version field). No external dependencies.

Reports `ok: false` only if any of the three are empty (unlikely in practice; would mean a build that bypassed the normal Taskfile). Compact line never wraps.

### Server

Three sub-checks, run sequentially. ConnectRPC and MCP use different endpoints/transports, so each sub-check builds its own client (no connection state is shared between Connect and MCP — earlier prose calling this "a single client" was wrong). The MCP handshake and Skills count share one mcp-go client. If reachability fails, the other two are skipped (no point doing MCP handshake against an unreachable server). All three sub-checks honor the shared `--timeout` flag (default 2s).

1. **Connect Health RPC**: calls `ServerServiceClient.Health(ctx, &HealthRequest{})`. Matches today's `specgraph health` exactly. Returns `reachable: false` on dial error, RPC error, or timeout.
2. **MCP Streamable-HTTP initialize handshake**: opens a `mcp-go` Streamable-HTTP client against `<serverURL>/mcp` using the same construction pattern PR F's `e2e/api/skills_test.go` uses (`client.NewStreamableHttpClient`); calls `Initialize(ctx, ...)` with a stub `clientInfo`. Reports `mcpHandshake: "ok"` if the server returns capabilities; `"failed"` with the error message otherwise.
3. **Skills count**: invokes `specgraph_skills_list` tool via the same mcp-go client used in sub-check 2, parses the JSON array, reports `skillsCount: N`. If the tool call fails or the JSON doesn't parse as `[]{name,summary,uri}`, reports `skillsCount: -1` and `ok: false` for the Server group.

The compact line collapses all three sub-checks into `Server: OK (reachable v<X> · MCP handshake OK · <N> skills)`. Any failure expands the Server group to show each sub-check on its own line with the specific failure.

`specgraph doctor server` runs only this group and emits compact form unless `--verbose`.

### Project config

Inputs: project-root-relative `.specgraph.yaml`. If absent, the group reports `ok: true` with `harnesses: []` (treating "no config" as "no project-level customization", which is valid — the binary works without it).

`ProjectConfig` today (`internal/config/project.go`) carries only `Slug` and `Server`. Commit 1 extends it with `Harnesses []string` and `Nudges struct { Quiet bool }`, and updates `init.go:117`'s currently-hard-coded `harnesses :=` block to read from `cfg.Harnesses` (falling back to all three when the list is empty).

Loading stays lenient (`yaml.Unmarshal`) so existing `.specgraph.yaml` files with future-reserved or typo'd keys don't break `init` or any other subcommand at decode time. Strict validation is moved into the **doctor's Project config check** instead — doctor uses a second `yaml.Decoder.KnownFields(true)` pass and reports unknown top-level keys as a per-key problem in the expanded view. This keeps the regression surface minimal: pre-existing configs keep working everywhere, doctor surfaces the strictness as drift, and the user can fix at their own pace.

Doctor's Project config group runs these checks:

1. `LoadProject(cwd)` returns without error (same call init uses; lenient).
2. A new helper `ValidateProjectStrict(path string) error` in `internal/config/project.go` re-reads the raw bytes from disk at `path` (the file `FindProjectRoot` returns) and runs them through `yaml.Decoder.KnownFields(true)`. Any unknown top-level key surfaces as drift. The strict helper is doctor's only caller; init keeps using `LoadProject`. Adds a one-line `os.ReadFile` + decoder construction — cheap.
3. Each entry in `Harnesses:` resolves to a known `Harness` enum value (`claude`, `cursor`, `opencode`). Unknown values surface as a per-entry error in the expanded view.

Compact line: `Project config: OK (3 harnesses enabled)`. Expanded shows each `Harnesses:` entry on its own line.

### Managed files

Delegates to `managedfiles.InspectAll(cwd, harnesses, params)`. The `harnesses` argument is the union of `ProjectConfig.Harnesses` and any `--harness` flag override (override wins if specified).

`InspectAll` today returns `[]FileState`, but `FileState` (`types.go:108`) doesn't carry which harness produced each row — that's needed for both the `--harness` filter and the JSON output's per-row `harness` attribution. Commit 1 adds `Harness Harness` to `FileState`.

**Population happens in `InspectAll`, not in each strategy's `Inspect`.** The strategy implementations (`jsonkeymerge.go:30,45,47`; `markdownblock.go:210,239,281,283`) construct `FileState{...}` literals that don't know which harness owns the manifest entry. Rather than thread `Harness` through every strategy method (invasive, four-file change), `InspectAll` does `state.Harness = mf.Harness` after the strategy returns — one line in `inspect.go`'s loop, zero changes to the strategy implementations or their literal constructions. The existing `FileState{}` literal sites can stay as-is; their zero-value `Harness` (which would be `HarnessClaude`) is overwritten before the caller sees it.

Compact line: `Managed files: 14/14 synced` if all green; `Managed files: 12/14 synced — 1 stale, 1 drifted` if mixed.

Expanded view (auto-triggered when any file is not Synced, or always with `--verbose`): a single table grouped by `host-pinned` vs `SpecGraph-owned`. The grouping is **derived from path prefix** at render time, not stored on `ManagedFile`:

- **Host-pinned**: files whose path does NOT start with `.specgraph/agents/`. These are the JSON-key-merge entries (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`, `.claude/settings.json`), the cursor `.mdc` rules, and the `AGENTS.md` markdown-block entry — paths the harnesses specify; SpecGraph cooperates.
- **SpecGraph-owned**: files under `.specgraph/agents/<harness>/`. SpecGraph owns the whole path tree; nothing else should write there.

Compact form omits the grouping; expanded form prints `## Host-pinned` and `## SpecGraph-owned` headers between the rows. Path-prefix derivation avoids ripple into `ManagedFile`, the manifest validator (`validateManifestEntry`), and `TestManifestShape` that adding a `HostPinned bool` field would cause.

`--fix` semantics:

- **Stale / Missing**: for each such row, call `Sync(cwd, mf, params, SyncOptions{})` (same options as `specgraph init`'s default — no `--force`, no `--keep-edits`). The strategy decides what to write.
- **Drifted**: never auto-touched. Print one guidance line per file:

  ```text
  .claude/settings.json (drifted): run `specgraph init --force --keep-edits .claude/settings.json`
  to keep your changes, or `specgraph init --force .claude/settings.json` to discard them.
  ```

  The user makes the call. This matches the existing framework's discipline of treating Drifted as "user has uncommitted state".

## Drift-nudge

A `PersistentPreRun` hook on the root cobra command. Calls `managedfiles.InspectAll(cwd, harnesses, params)` and emits one stderr line if any file is not Synced:

```text
note: 2 managed files out of date with this binary (1 stale, 1 drifted); run `specgraph init` to refresh, `specgraph doctor` for details
```

### Skip gates (in evaluation order)

The hook walks from the invoked cobra `cmd` up to the **top-level command directly under `rootCmd`** (so for `specgraph slice list`, the top-level is `slice`; for `specgraph init`, it's `init` itself). It then matches that top-level name against the allow-list. The walk:

```go
top := cmd
for top.HasParent() && top.Parent() != rootCmd {
    top = top.Parent()
}
// top.Name() now is the top-level group, e.g. "slice", "edge", "init", "doctor".
```

This was wrong in the previous draft (a `cmd.Name()`-only match would let the nudge fire for `slice list`, `edge add`, `graph deps`, `findings list`, etc. — the highest-frequency interactive commands). Using the top-level group catches every subtree by construction.

1. **Subcommand allow-list** (matched on the top-level group's name): skip for `init`, `doctor`, `health`, `read-mcp-resource`, `serve`, `version`, `bundle`, `up`, `confluence`. Any future `mcp` parent command would join by extension. These either handle reporting themselves, run in non-interactive contexts, or are short-lived utility subcommands. **Everything else gets the nudge** — including `slice list`, `edge add`, `deps`, `ready`, `list`, `show`, `create`, `update`, `findings`, `decision`, `changes`, `claim`, `amend`, `drift`, etc. (the cobra tree today registers most subcommands directly under `rootCmd` rather than under grouped parents, so the walk usually exits at the first hop; verify against the live tree before pinning test expectations).
2. **`isatty(stderr) == false`**: primary gate. Uses `golang.org/x/term`'s `IsTerminal(int(os.Stderr.Fd()))` (the same package `internal/config/managedfiles/source_dev.go:13,38` already uses for the dev-banner gate — one import, one implementation). Catches Claude session-start hook, MCP server, shell pipes, CI. Per parent design, without this, the Claude hook would inject the nudge into the prime payload the model reads — a class-of-bug closed at the framework level rather than via subcommand allow-list maintenance.
3. **`SPECGRAPH_DRIFT_NUDGE=off`** env var (user-level mute).
4. **`nudges.quiet: true`** in `.specgraph.yaml` (project-level mute; read from the same `ProjectConfig.Nudges` field commit 1 adds).
5. **Throttle** (see below).

If any gate triggers, the hook returns immediately — no inspection done, no IO performed.

### Project root, harness list, and ProjectParams for the nudge's `InspectAll`

The hook must call `InspectAll(root, harnesses, params)` — and "root" matters. `InspectAll` does **not** walk up to find the project root on its own (`internal/config/managedfiles/inspect.go:61-74` only validates the given path is a directory, then resolves manifest paths relative to it). If the nudge passed the bare `cwd`, every managed-file path would appear `Missing` whenever the user ran a subcommand from a project subdirectory — the nudge would fire spuriously on every invocation.

The hook therefore does, in order:

1. `root, err := config.FindProjectRoot(cwd)` — same call init uses at `init.go:55`. If `err == ErrProjectNotFound`, the hook returns silently (no project, no managed files, no nudge — this is the common case for `specgraph` invocations outside a project tree).
2. `cfg, err := config.LoadProject(root)` — lenient decode; on any other error, the hook returns silently (advisory feature, never fail the CLI).
3. From `cfg`, build the harness slice (`cfg.Harnesses` mapped to `[]Harness`, falling back to all three when empty) and the `ProjectParams`. The server URL **must** go through `globalCfg.ResolveServer(cfg.Slug, cfg.Server)` — the same path `init.go` takes — so the sentinel hashes the framework computes match what init wrote. Skipping the resolver and using raw `cfg.Server` would yield a different `ProjectParams.ServerURL` than init used, which flows into JSONKeyMerge canonical bytes, which changes the embedded sentinel hash, which makes every JSONKeyMerge entry flip `Stale` even on a freshly-init'd project. Pass-4 review caught this; the cost of the extra call is one map lookup.
4. `InspectAll(root, harnesses, params)` and check whether any state is not `Synced`.

`LoadProject` + `FindProjectRoot` together parse YAML and walk the directory tree on every CLI invocation that survives the earlier skip gates. The cost is sub-millisecond on warm cache and only paid by interactive invocations from inside a project root (the vast majority of CLI calls hit the isatty or allow-list gate first and skip these calls entirely).

### Cheap-gate optimization

`InspectAll` walks the manifest (8-14 entries per harness filter) and stats each path. On every passing-all-gates CLI invocation this is acceptable (one stat per file, no reads when the path is missing); the calls together take sub-millisecond on warm cache. No cheaper gate is needed; the throttle catches the repeat case.

### Throttle path

`xdg.CacheHome() + "/nudges/" + sha256(EvalSymlinks(projectRoot)) + "-" + binaryVersionHash`

Default expansion: `~/.cache/specgraph/nudges/<project-hash>-<version-hash>`.

The binary version is part of the **path**, not the contents — that way two binaries on PATH (e.g., a stable install and a dev-tagged build) each track their own throttle independently and can't ping-pong each other into re-emitting on every alternation.

File contents are empty; mtime drives the throttle window.

### Throttle policy

| Condition | Action |
|---|---|
| First nudge ever for `(project, binary-version)` | Emit, create file |
| Same `(project, binary-version)` within 24h | Skip |
| Same project, different binary version | Emit (binary upgraded; user sees one fresh nudge per version bump) |
| Older than 24h, same version | Emit, refresh mtime |

### Opportunistic GC

On each emit, scan `xdg.CacheHome() + "/nudges/"` for entries with `mtime > 30 days`; delete them. One `readdir` is well within the per-invocation budget. Prevents indefinite accumulation as users move/rename projects.

### Fallback

For unexpected errors (cache home unwritable, `EvalSymlinks` errors on a deleted cwd, throttle file write fails): emit the nudge unthrottled. Minor over-emission, not broken. The CLI never fails because of an advisory feature.

## Already-shipped plumbing PR G consumes

Two pieces the parent epic frames as part of PR G are **already in main**; PR G consumes them rather than landing them:

- **`xdg.CacheHome()`** is at `internal/xdg/xdg.go:54-60` with cache-dir-appropriate docs ("non-essential data that can be regenerated on demand", "not pre-created — callers create lazily at use site"). The drift-nudge calls it directly. The pass-1 reviewer caught my mistake of listing this as new work.
- **`dev` build tag** is implemented as paired files `internal/config/managedfiles/source_release.go` (`//go:build !dev`) and `internal/config/managedfiles/source_dev.go` (`//go:build dev`). The dev variant reads from `SPECGRAPH_DEV_SOURCE_ROOT` (defaulting to `./plugin`) via `os.ReadFile`, emits a one-shot dev banner gated on `isatty(stderr)` via `golang.org/x/term`, and exposes the same `readSourceImpl(mf ManagedFile) ([]byte, error)` signature the release path uses. The symbol is `readSourceImpl` (not `embeddedFS`) and there is no `embed.FS`-vs-`fs.FS` type-swap to engineer — the two implementations have matching signatures by construction. PR G does **not** need a paired dev file for `internal/mcp/skills/embedded.go` because that package's `//go:embed embedded/*/SKILL.md` directive already reads from real on-disk files inside the package (PR F's pattern); there is no on-disk-vs-binary divergence to bridge for skills.

## Dogfood tasks (PR G work)

### `task plugin:refresh`

```yaml
plugin:refresh:
  desc: Rebuild specgraph and re-run init against the current project to pick up plugin canonical edits
  cmds:
    - task: build
    - go run ./cmd/specgraph init --quiet
```

The `--quiet` flag on init is added alongside `--check` in commit 2 (suppresses the per-file action lines so the refresh output stays terse).

### `task plugin:check`

```yaml
plugin:check:
  desc: Verify the embedded canonicals match what specgraph init would write
  cmds:
    - task: build
    - go run ./cmd/specgraph init --check
```

Exits non-zero if any managed file would change. Wired into `task check` by inserting `- task: plugin:check` into its `cmds:` sequence between `- task: lint` and `- task: skills:validate` so a contributor who edited `plugin/<harness>/...` without rebuilding sees the failure during the same `task check` they're running pre-push. (`task check` uses a `cmds:` sequence, not `deps:` — earlier wording was wrong.)

The `--check` flag combines with init's existing dry-run-style behaviour: with `--check`, init performs the same inspection it would for a real run, exits 0 if every managed file is Synced, and exits non-zero if any would change. No separate `--dry-run` flag is added.

## Implementation layout

```text
cmd/specgraph/
├── doctor.go              # NEW — top-level command, DoctorReport struct, render dispatch
├── doctor_test.go         # NEW — table-driven state-machine tests
├── doctor_server.go       # NEW — server subcommand + Server group implementation
├── doctor_managed.go      # NEW — managed-files group + --fix + --harness
├── doctor_binary.go       # NEW — binary group
├── doctor_config.go       # NEW — project-config group
├── doctor_render.go       # NEW — compact-vs-expanded text rendering + JSON marshal
├── nudge.go               # NEW — PersistentPreRun hook + throttle + allow-list + GC
├── nudge_test.go          # NEW — table-driven on the four states + throttle + skips
├── health.go              # MODIFIED — deprecated alias for `doctor server`
├── init.go                # MODIFIED — adds --check + --quiet flags; reads cfg.Harnesses
└── root.go                # MODIFIED — wires PersistentPreRun, adds doctor cmd

internal/config/
├── project.go             # MODIFIED — Harnesses []string + Nudges struct{Quiet bool}; strict YAML
└── project_test.go        # MODIFIED — strict-decoder rejection + new fields' decoding

internal/config/managedfiles/
├── types.go               # MODIFIED — adds Harness Harness to FileState
├── inspect.go             # MODIFIED — populates FileState.Harness from the manifest
└── inspect_test.go        # MODIFIED — asserts FileState.Harness round-trip

Taskfile.yml               # MODIFIED — adds plugin:refresh + plugin:check; wires plugin:check into the check cmds sequence (between lint and skills:validate)
```

Not in scope (already shipped, see "Already-shipped plumbing PR G consumes"): `internal/xdg/xdg.go` (already has `CacheHome()`), `internal/config/managedfiles/source_dev.go` (already exists), and any dev-tag work in `internal/mcp/skills/`.

## Tests

### Unit — `cmd/specgraph/nudge_test.go`

Table-driven across the four `State` values and the skip gates:

- `TestNudge_SkippedFor*`: each subcommand in the allow-list returns nil from the hook with no IO.
- `TestNudge_SkippedWhenStderrNotTTY`: with `os.Stderr` redirected to a pipe, hook returns silently.
- `TestNudge_SkippedByEnvVar`: `SPECGRAPH_DRIFT_NUDGE=off`.
- `TestNudge_SkippedByConfig`: `nudges: { quiet: true }` in test fixture.
- `TestNudge_ThrottledWithinWindow`: write a fresh mtime throttle file, run hook, assert no emit.
- `TestNudge_EmittedAfterWindow`: backdate throttle file mtime by 25h, run, assert emit + mtime refresh.
- `TestNudge_EmittedFirstTime`: no throttle file, run, assert emit + file created.
- `TestNudge_GarbageCollectsOldEntries`: create three throttle files with mtime > 30d in a temp xdg cache, run, assert deletion of those three but not a fresh one.
- `TestNudge_FallbackOnCacheUnwritable`: simulate unwritable cache (chmod 0o400 a temp dir), assert emit happens anyway.
- `TestNudge_SkippedWhenNoProject`: when `FindProjectRoot(cwd)` returns `ErrProjectNotFound` (e.g., running specgraph from `/tmp` with no `.specgraph.yaml` anywhere up the tree), hook returns silently with no IO, no inspection, no nudge.

### Unit — `cmd/specgraph/doctor_test.go`

- `TestDoctorReport_AllHealthy_Compact`: 5-line output, exit 0.
- `TestDoctorReport_OneStale_ManagedFilesExpanded`: only managed files section expands.
- `TestDoctorReport_ServerUnreachable_ServerExpanded`: only server section expands.
- `TestDoctorReport_Verbose_AllExpanded`: all four sections expanded.
- `TestDoctorReport_JSON_StableSchema`: JSON unmarshal into `DoctorReport`, key-by-key assertion.
- `TestDoctorReport_ExitCodes`: all healthy → 0; any unhealthy → 1; infrastructure failure → 2.
- `TestDoctorReport_ExitZeroForcesZero`: with `--exit-zero`, an unhealthy run that would otherwise return 1 returns 0; the report body still shows the problem rows. Pin the advisory-mode contract.
- `TestInit_CheckFlag_ExitsNonZeroOnDiff`: `init --check` against a project where a managed file is Stale exits non-zero; against a fully-Synced project exits 0. Pins `task plugin:check`'s exit contract independently of the e2e.
- `TestDoctorFix_StaleAndMissing_Refreshed`: fixture with 1 stale + 1 missing; `--fix` calls Sync for both; final inspect shows both Synced.
- `TestDoctorFix_Drifted_GuidanceOnly`: drifted file in fixture; `--fix` does NOT call Sync; guidance line in output names the file + both recovery commands.
- `TestHealthAlias_DispatchesToDoctorServer`: `specgraph health` runs, prints a deprecation notice on stderr, then produces the same `Server: OK` line `specgraph doctor server` does. (The assertion direction is health → doctor server, not the reverse — health is the deprecated wrapper that delegates.)

### Unit — `internal/config/project_test.go` (extended)

- `TestProjectConfig_DecodesNewFields`: YAML with `harnesses: [claude, cursor]` and `nudges: { quiet: true }` round-trips into `cfg.Harnesses` and `cfg.Nudges.Quiet`.
- `TestProjectConfig_RejectsUnknownTopLevelKey`: a fixture with `fnord: 42` returns a decode error (strict mode catches it).
- `TestProjectConfig_EmptyHarnessesAcceptedAsLegacy`: missing or empty `harnesses:` decodes as empty slice, matching the pre-PR-G no-key behaviour init.go falls back on.

### Unit — `internal/config/managedfiles/inspect_test.go` (extended)

- `TestInspectAll_PopulatesFileStateHarness`: invoke `InspectAll` with `[]Harness{HarnessClaude}`, assert every returned `FileState.Harness == HarnessClaude`.

### E2E — `e2e/api/doctor_test.go`

A Ginkgo test that runs `specgraph init` followed by `specgraph doctor` and asserts the doctor output:

- Contains `14/14 synced` (assuming a fresh project install)
- Contains the `Server: OK` line with the skills count
- Exits with code 0

A second `It` block runs `specgraph init`, then manually corrupts one managed file (writes garbage), then runs `specgraph doctor --fix` and asserts:

- The corrupted file is detected as Drifted (since the sentinel hash no longer matches disk)
- `--fix` does NOT modify it (Drifted is opt-in)
- The guidance line names the file + both recovery commands
- Exit code is 1

A third `It` block runs `specgraph doctor --json` and parses the output as `DoctorReport`. Schema check.

## Sequencing

Nine commits (1 and 1a split for narrower blast radius per pass-2 review), each green under `task check`:

| # | Commit | Description |
|---|--------|-------------|
| 1 | `feat(config): ProjectConfig.Harnesses + Nudges.Quiet; init.go fallback to cfg.Harnesses` | `internal/config/project.go` gains `Harnesses []string` and `Nudges struct{Quiet bool}`; YAML decode stays lenient (`yaml.Unmarshal` is unchanged). `init.go:117`'s hard-coded harness slice now falls back from `cfg.Harnesses`. Project tests extended for the new field round-trips. |
| 1a | `feat(managedfiles): FileState.Harness populated by InspectAll after strategy dispatch` | `internal/config/managedfiles/types.go` adds `Harness Harness` to `FileState`; `inspect.go`'s `InspectAll` loop writes `state.Harness = mf.Harness` after the strategy returns (strategies unchanged). Inspect tests extended. Split out from commit 1 so each commit makes one schema migration in isolation. |
| 2 | `feat(init): add --check (exit non-zero if any managed file would change) + --quiet` | `cmd/specgraph/init.go`. Required by `task plugin:check` in commit 8. `--check` performs inspection but no writes; `--quiet` suppresses per-file action lines for `task plugin:refresh`. |
| 3 | `feat(doctor): scaffold cobra command + Binary group + DoctorReport rendering` | `cmd/specgraph/doctor.go`, `doctor_binary.go`, `doctor_render.go`, minimal `doctor_test.go`. Establishes the `DoctorReport` struct, compact-vs-expanded rendering, exit-code policy (0/1/2 + `--exit-zero`), `--json`/`--verbose` flags. |
| 4 | `feat(doctor): Project config group (strict decode + harnesses: resolution)` | `doctor_config.go` + tests. Re-uses commit 1's `ProjectConfig`. Reports per-entry errors in expanded view. |
| 5 | `feat(doctor): Server group (Health RPC + MCP handshake + Skills count) + --timeout` | `doctor_server.go` + tests. New `doctor server` subcommand. Adds `--timeout` flag (default 2s) shared by Connect and mcp-go clients (one of each, fresh per invocation). |
| 6 | `feat(doctor): Managed files group + --fix + --harness + --verbose` | `doctor_managed.go` + tests. Wires `--fix` (Stale/Missing only) + guidance text for Drifted. Path-prefix grouping for host-pinned vs SpecGraph-owned (no manifest field added). |
| 7 | `refactor(health): deprecate health command as alias for doctor server` | `cmd/specgraph/health.go` becomes a thin wrapper that prints a deprecation notice on stderr and dispatches to `doctor server`. Preserves exit codes for existing scripts. |
| 8 | `feat(cmd): drift-nudge PersistentPreRun + task plugin:refresh/plugin:check + docs` | `cmd/specgraph/nudge.go` + `nudge_test.go`. Wires into `rootCmd.PersistentPreRun`. Reads `ProjectConfig.Harnesses` and `Nudges.Quiet`. Throttle file path uses `xdg.CacheHome()` directly (already shipped). Adds `task plugin:refresh` + `task plugin:check` to `Taskfile.yml`; inserts `- task: plugin:check` into the `check:` cmds sequence between `- task: lint` and `- task: skills:validate`. Folds in the documentation updates: `CLAUDE.md` "Doctor + drift-nudge" subsection, `plugin/specgraph/routing-guide.md` entry point note, top-level README check, and `.specgraph.yaml` schema documentation for the new `harnesses:` / `nudges:` fields. |

E2E test (`e2e/api/doctor_test.go`) lands in commit 6 — the first commit that exposes the full multi-group doctor surface the test asserts against (Binary from 3, Project config from 4, Server from 5, Managed files from 6).

## Documentation updates included in this PR

- `CLAUDE.md` — add a "Doctor + drift-nudge" subsection under "Project Overview" or the "Commands" table; document `task plugin:refresh` and `task plugin:check`; describe the `dev` build tag.
- `plugin/specgraph/routing-guide.md` — mention `specgraph doctor` as the entry point for "is something wrong?".
- The existing top-level README — add `specgraph doctor` to any command listing (verify; edit only if a listing exists).

## Deliberate departures from the parent epic

- **`--scope` flag deferred.** Parent design's example signature includes `--scope <s>`. PR G ships without that flag; the design notes user/all scopes as future work. Reduces surface area while still matching the parent epic's "v1 = project" intent.
- **Subcommand allow-list extended.** Parent design lists `init`, `doctor`, `health`, `read-mcp-resource`, `serve`, `mcp/*`, `version`. PR G also adds `bundle`, `up`, `confluence` — short-lived utility subcommands where the nudge clutters output more than it informs. Documented in the nudge skip rules.

## Out of scope

- **`task plugin:watch`** — file watcher that runs `task plugin:refresh` on changes. Belongs in dev tooling, not the binary. Parent design explicitly defers it; PR G follows.
- **`--scope` flag for user/all scopes.** See departure above.
- **Doctor checking remote/server-side health beyond Connect + MCP handshake.** E.g., DB connectivity check on the server's behalf. The server reports its own health via the existing `Health` RPC; doctor doesn't need to second-guess.
- **Auto-fixing Drifted files.** Per the framework's existing discipline: Drifted means the user has uncommitted state. `--fix` won't touch them.
- **Throttle period tunable.** 24h is fixed in PR G. A config knob can land later if anyone asks.

## Risks

- **`isatty(stderr)` false-positives on `specgraph list | less`.** Parent design notes this; nudge can smear across pager output. Escape hatch is `SPECGRAPH_DRIFT_NUDGE=off`. Documented limitation, not blocker.
- **`InspectAll` on every CLI invocation has a cost.** 8-14 stats per call (depending on `--harness` narrowing); sub-millisecond on warm cache. Acceptable. If users report perceived lag, a cheaper "is there probably drift?" gate (e.g., comparing the mtime of `.specgraph/` to the binary's build time) can land as a follow-up.
- **Project config strict-decode is doctor-only.** Loading via `yaml.Unmarshal` stays lenient everywhere (init, nudge, all other subcommands). Strict (`KnownFields(true)`) is run only inside doctor's Project config check, so unknown top-level keys surface as drift the user sees in `specgraph doctor` rather than as a hard `init` failure. Earlier drafts proposed flipping global decode to strict; pass-2 review flagged that as a breaking migration. The doctor-only path is the equivalent diagnostic without the regression.
- **`PersistentPreRun` runs before subcommand setup.** Cobra's `PersistentPreRun(E)` runs after flag parsing but before the subcommand's own `RunE`. The nudge hook can therefore freely use the parsed cobra context, but must not depend on per-subcommand state (e.g., the `--server` flag of `up`). The hook does its own minimal config load and never sees subcommand flags.
- **Throttle file under jj-colocated workspaces.** `EvalSymlinks(projectRoot)` resolves each jj workspace's working directory to a distinct physical path (`/Users/.../specgraph` vs `/Users/.../specgraph-pr-g` etc.), so two workspaces of the same repo get separate throttle files. Acceptable: the user sees one nudge per 24h per workspace. (Earlier prose claimed they would share a throttle file — that was wrong; `jj workspace add` creates a separate physical directory, not a symlink.)
- **Project config field migration.** Adding `Harnesses []string` and `Nudges struct{Quiet bool}` to `ProjectConfig` extends the user-facing config schema. Anyone with a `.specgraph.yaml` using neither key continues to work (empty values are valid). Documenting the new keys in the project README / CLAUDE.md is included in this PR's docs commit (folded into commit 8).

## Open questions

None outstanding. The brainstorming round closed:

- Scope — full parent-epic scope (4 check groups + health alias + dev tooling).
- Exit codes — non-zero on unhealthy by default; `--exit-zero` for advisory mode.
- Server group depth — three sub-checks (Health RPC + MCP handshake + Skills count) under one group.
- Healthy-state output — compact-when-green, expanded-when-problems; `--verbose` forces expansion.

## References

- Parent epic design: [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md), §"Doctor + drift nudge" (lines 205-258), §"PR G" (lines 373-379), §"Dogfood discipline".
- Existing health command: [`cmd/specgraph/health.go`](../../cmd/specgraph/health.go).
- Existing InspectAll: [`internal/config/managedfiles/inspect.go`](../../internal/config/managedfiles/inspect.go).
- Existing xdg helpers: [`internal/xdg/xdg.go`](../../internal/xdg/xdg.go).
- Predecessor design (PR F): [`2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md`](2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md).
- agentskills.io SKILL.md format — the `summary:` extension from PR F that informs the doctor's reporting style.
