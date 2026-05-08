# Harness install parity via embed-and-write — design

- **Bead:** `spgr-rwrp`
- **First child landing this:** `spgr-zqpb` (OpenCode plugin embed-and-write)
- **Predecessor:** `spgr-cceg` (harness parity epic — established the three-shim pattern)
- **Date:** 2026-05-08

## Context

`specgraph init` today manages MCP wiring for all three harnesses (Claude Code, Cursor, OpenCode) but only **partially** manages each harness's integration surface:

- **Claude:** `.mcp.json` + `AGENTS.md` (pointer). Plugin manifest, hooks, routing-guide left to a future marketplace install or manual setup.
- **Cursor:** `.cursor/mcp.json` + `.cursor/rules/specgraph-bootstrap.md` (pointer). Main rule files (`specgraph.md`, `post-stage.md`) live in the repo's `plugin/cursor/` shim and never reach end-user projects.
- **OpenCode:** `opencode.json`'s MCP block. Plugin TypeScript file is a manual README install step.
- **Skills (cross-harness):** symlinked from each shim to the in-tree `<repo>/skills/` tree. Broken for end users without a repo checkout.

The result: end users get an inconsistent setup story per harness, and the harness-parity promise from `spgr-cceg` is incomplete.

This epic adopts a uniform **embed-and-write** pattern: harness shim content is bundled into the specgraph CLI binary via `//go:embed`, and `specgraph init` writes canonical files into the user's project on every run. Drift between binary and on-disk files is detectable and surfaced via a new `specgraph doctor` command and a low-overhead nudge on every CLI invocation.

## Decisions

| Question | Decision |
|---|---|
| Scope | Cross-harness pattern, not per-harness one-offs. First implementation lands in `spgr-zqpb`; the framework is reusable. |
| Drift policy | Per-strategy. JSON keys → merge. Markdown blocks → fence (versioned, hash-tracked). Whole-file → warn-and-force unless `--force`. (StrategicMerge dropped — YAGNI; reintroduce when a concrete file needs it.) |
| Marker shape | Sentinel comment in the file's native comment syntax (`//`, `#`, `<!-- -->`), carrying version + sha256. No sidecar files, no project-level state file. |
| Staleness UX | Cheap drift check on every CLI invocation emits a one-line stderr nudge if any managed file is not synced. Plus a new `specgraph doctor` command for the deep-dive. |
| Skills delivery | Universal MCP-resource fetch (`specgraph://skills/<name>`); zero on-disk skill files in any harness. |
| Architecture | Single unified `internal/config/managedfiles/` package with a strategy enum. Existing `internal/config/mcpconfigs/` and `internal/config/pointers/` get folded in and deleted. |
| Layout | SpecGraph-owned files consolidate under `.specgraph/agents/<harness>/`. Harness-pinned files (Cursor rules, MCP configs, AGENTS.md, settings.json) stay at their conventional paths. |
| Existing-user migration | Not applicable — no existing users. Design forward, implement forward. |
| Dogfood | This repo runs the same flow as end users. Authoring sources at `plugin/<harness>/...` are committed; init-written destinations are gitignored. |

## Architecture

A single package `internal/config/managedfiles/` is the home for everything specgraph injects into a project.

```go
type ManagedFile struct {
    Path           string         // relative to project root
    Strategy       Strategy
    Source         embed.FS       // canonical content
    Comment        CommentSyntax  // ts | sh | md | mdc | jsonc | yaml — for sentinel
    Harness        Harness        // claude | cursor | opencode
    SupersedesPath string         // optional: if set, init deletes this path after writing Path
                                  //   (used for `.md` → `.mdc` cursor-rule rename)
}

type Strategy int
const (
    StrategyJSONKeyMerge  Strategy = iota // managed keys merge into a JSON file; siblings preserved
    StrategyMarkdownBlock                 // versioned, hash-tracked block in a markdown file
    StrategyWholeFile                     // entire file is canonical; warn-and-force on drift
    // (StrategicMerge dropped — YAGNI. Reintroduce when a concrete file needs it.)
)

type State int
const (
    StateMissing State = iota
    StateSynced
    StateStale       // sentinel hash matches disk, but disk doesn't match canonical (older specgraph wrote it)
    StateDrifted     // disk doesn't match sentinel hash (user-edited)
)

func InspectAll(cwd string, harnesses []Harness) ([]FileState, error)
func SyncAll(cwd string, harnesses []Harness, opts SyncOptions) ([]SyncResult, error)
```

The existing `mcpconfigs/` and `pointers/` packages are subsumed as `JSONKeyMerge` and `MarkdownBlock` strategies, then deleted. Call sites that talk to this one package: `cmd/specgraph/init.go`, the new `cmd/specgraph/doctor.go` (with `cmd/specgraph/health.go` shimming through doctor as a deprecated alias), and the drift-nudge in the root command's `PersistentPreRun`.

## Drift detection

### Sentinel format

The marker namespace stays `specgraph:init:` to preserve continuity with the existing `internal/config/pointers/` markers (`agents.go:20-21`, `cursor.go:16`). The version bumps from `v=1` to `v=2` to add the hash payload.

For `WholeFile` strategy (TypeScript / shell / markdown / mdc), a single sentinel line:

```text
// specgraph:init v=2 sha256=abc123... rev=cef1ec3a
```

`sha256` is computed over canonical content **excluding the sentinel line itself**, so the sentinel can change without affecting the hash. `rev` is forensic only.

For `MarkdownBlock` strategy, the sentinel rides on the existing start/end markers:

```text
<!-- specgraph:init:start v=2 sha256=abc... -->
...
<!-- specgraph:init:end -->
```

**Upgrade path from v=1 markers to v=2.** Existing files in this repo carry `<!-- specgraph:init:start v=1 -->` (no hash). The framework's classification of a `v=1` file uses a defensive check, not blind trust:

1. Read disk content between `v=1` markers.
2. Compute what *this binary's* `v=1` canonical for that path would have written, using a preserved-private rendering helper (see "Vestigial v=1 renderer" below).
3. If the disk content matches that canonical: classify as `Stale` and refresh in-place to `v=2` with the new hash. No user content was at risk.
4. If the disk content does NOT match: classify as `Drifted`. The user (or a prior dev) hand-edited within the managed block. Refuse to overwrite without `--force`.

This guards against silently destroying hand-edited content during the v=1 → v=2 upgrade. The check is dogfood-only (no external users), but it's the right primitive for the framework regardless.

**Vestigial v=1 renderer (sunset policy).** PR B preserves the existing `pointers/` rendering logic as an unexported helper inside `internal/config/managedfiles/` (e.g. `classifyV1Disk`). The helper exists *only* to compute what a v=1 canonical would have written for the v=1 → v=2 hash-check above. It is not on the production write path — new writes always emit v=2. The helper is removed in a follow-up bead once `task plugin:check` reports zero `v=1` files in the dogfood repo for two consecutive releases. Same policy applies to `SupersedesPath` "prior canonical" computations (e.g., the old `.md` cursor-rule path's pre-PR-D rendering): preserve as a private helper, sunset on the same trigger.

**Forward-compatibility:** the marker parser accepts `v=1` and `v=2`, rejects `v=3+` as `ErrCorruptedMarkers` (matches today's `pointers/agents.go` corruption-rejection behaviour for unknown versions). A future binary downgrade scenario surfaces as corruption rather than silent data loss.

`JSONKeyMerge` does not need sentinels — the strategy already knows which keys it owns and overwrites them on every init. There's no `Drifted` state for merge-strategy files because managed keys are not user-owned by definition. **One exception:** `.claude/settings.json`'s `enabledPlugins["specgraph@specgraph-local"]` preserves a user-set `false` value (so `claude /plugin disable specgraph` survives subsequent `init` runs — see "Open questions / Resolved"). That key is "managed-presence" rather than "managed-value": init ensures the key exists; the value follows the user's last choice. It's the only such key in the manifest; the exception is local, not a general policy.

### State decision

| Disk state | Sentinel hash matches disk content? | Sentinel hash matches canonical hash? | State |
|---|---|---|---|
| File missing | — | — | `Missing` |
| Sentinel absent | — | — | `Drifted` (treat as user-owned, skip without `--force`) |
| present | yes | yes | `Synced` |
| present | yes | no | `Stale` (older specgraph wrote it; safe to refresh) |
| present | no | — | `Drifted` (user modified after init) |

### Init's behaviour per state

- `Missing` → write canonical with fresh sentinel (silent)
- `Synced` → no-op (silent)
- `Stale` → rewrite canonical, update sentinel (one line: `path: refreshed (vN.M.K → vN.M.K')`)
- `Drifted` → emit warning, skip; `--force` to overwrite, `--force --keep-edits` to preserve content but update the sentinel hash to match disk

### Cost

The check is intentionally tiny: one `os.Stat` per file, read first ~200 bytes for the sentinel, compute SHA-256 over the canonical embed (already in memory). For a 14-file manifest, total wall time is sub-millisecond on a warm cache. Suitable for running on every CLI invocation.

The four states are decidable from disk + binary alone — no ambient state files. `rm -rf .specgraph/` produces a clean recovery: doctor reports Missing, init recreates.

## Skills delivery

Skills live in the CLI binary (`//go:embed skills/...`) and are exposed via two MCP surfaces:

- **Resource:** `specgraph://skills/<name>` returns the SKILL.md body. Same fetch mechanism as `specgraph://prime`.
- **Tools:** `specgraph_skills_list` returns `[{name, description, when_to_use}]` from frontmatter; `specgraph_skills_get` is a thin tool wrapper over the resource fetch.

The prime resource grows a "Skills available" catalog block listing names and descriptions, telling the model to fetch on demand rather than pre-load.

Per-harness consumption becomes uniform:

- **OpenCode:** `experimental.chat.system.transform` already injects prime; the catalog comes through automatically.
- **Cursor:** the bootstrap rule references prime via `read-mcp-resource`; the prime now includes the catalog.
- **Claude:** the existing `hooks/specgraph-session-start.sh` already injects prime via `specgraph read-mcp-resource specgraph://prime`. No new hook needed; the catalog rides the existing path.

**Trade-off — explicit Claude UX regression:** Claude Code's native skills mechanism auto-loads from `.claude/skills/` and surfaces in the user's UI; switching to MCP-fetch means (a) the model has to *choose* to fetch based on the prime catalog descriptions, (b) no skill-card UI in Claude, (c) skills can't trigger via Claude's own skill-routing heuristics. This is a deliberate trade for cross-harness uniformity, accepted upfront.

PR F lands MCP-fetch as the **chosen mechanism**, not a parallel-run with disk symlinks. The existing in-repo `plugin/<harness>/skills` symlinks (a dev-time artifact only — end users never had them) are removed in PR F itself; `task plugin:sync` is also removed. Running two delivery paths simultaneously would create two sources of truth the model can see (auto-loaded from disk vs MCP-fetched), with no clean way to measure which was actually used. Cleaner to commit.

**Fallback (if needed):** if user feedback indicates the Claude UX regression is unacceptable, a follow-up bead adds a Claude-specific manifest section that writes skills to `.claude/skills/<name>/SKILL.md` (`WholeFile` strategy) — a chosen-mechanism for Claude only, decided on real feedback rather than a parallel-run measurement window. OpenCode and Cursor stay on MCP-fetch unconditionally.

The repo-root `<repo>/skills/` tree remains the authoring source; the embedded canonical lives in the binary; the MCP resource handler serves it.

## Per-harness manifest

14 managed files total. SpecGraph-owned files consolidate under `.specgraph/agents/<harness>/`; harness-pinned files stay at their conventional paths.

| # | Path | Harness | Strategy | Notes |
|---|---|---|---|---|
| 1 | `opencode.json` | OpenCode | `JSONKeyMerge` | Managed keys: `mcp.specgraph.{enabled,headers,type,url}`. Also ensures our plugin path (`./.specgraph/agents/opencode/specgraph.ts`) is present in the `plugin` array via union-merge — adds if missing, leaves any other user-added entries alone. |
| 2 | `.specgraph/agents/opencode/specgraph.ts` | OpenCode | `WholeFile` | Plugin TS, referenced from `opencode.json`'s `plugin` array. |
| 3 | `.cursor/mcp.json` | Cursor | `JSONKeyMerge` | Init emits warning if `~/.cursor/mcp.json` has a `specgraph` entry (global collision). |
| 4 | `.cursor/rules/specgraph-bootstrap.mdc` | Cursor | `MarkdownBlock` | |
| 5 | `.cursor/rules/specgraph.mdc` | Cursor | `WholeFile` | |
| 6 | `.cursor/rules/specgraph-post-stage.mdc` | Cursor | `WholeFile` | |
| 7 | `.mcp.json` | Claude | `JSONKeyMerge` | |
| 8 | `AGENTS.md` | Claude | `MarkdownBlock` | |
| 9 | `.claude/settings.json` | Claude | `JSONKeyMerge` | Managed keys: `extraKnownMarketplaces.specgraph-local` (with `source: { type: "directory", path: "./.specgraph/agents/claude" }` — note: the path is the **marketplace root**, the dir containing `.claude-plugin/`, NOT `.claude-plugin/` itself; verified in PR 0), `enabledPlugins["specgraph@specgraph-local"]`. |
| 10 | `.specgraph/agents/claude/.claude-plugin/plugin.json` | Claude | `WholeFile` | |
| 11 | `.specgraph/agents/claude/.claude-plugin/marketplace.json` | Claude | `WholeFile` | Single-plugin local marketplace. `plugins[].source` is the directory path `"./"` (co-located plugin shares the marketplace root); NOT `"./.claude-plugin/plugin.json"`. `..` traversal is blocked, so any future multi-plugin layout must keep all plugins inside the marketplace root. |
| 12 | `.specgraph/agents/claude/hooks/specgraph-session-start.sh` | Claude | `WholeFile` | |
| 13 | `.specgraph/agents/claude/hooks/specgraph-post-stage.sh` | Claude | `WholeFile` | |
| 14 | `.specgraph/agents/claude/routing-guide.md` | Claude | `WholeFile` | |

Hook paths inside `plugin.json` use `${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-{session-start,post-stage}.sh`, which Claude resolves against the plugin root (`.specgraph/agents/claude/`).

`.claude/settings.json` writes `autoUpdate: false` for the local marketplace. Drift control is `specgraph init`'s job, not Claude's auto-updater.

### Repo-internal source renames

- `plugin/cursor/.cursor/rules/specgraph.md` → `plugin/cursor/.cursor/rules/specgraph.mdc`
- `plugin/cursor/.cursor/rules/post-stage.md` → `plugin/cursor/.cursor/rules/specgraph-post-stage.mdc`
- `plugin/specgraph/hooks/session-start.sh` → `plugin/specgraph/hooks/specgraph-session-start.sh`
- `plugin/specgraph/hooks/post-stage.sh` → `plugin/specgraph/hooks/specgraph-post-stage.sh`
- `plugin/specgraph/.claude-plugin/plugin.json` references update to match renamed hook paths

### Per-harness opt-in

`.specgraph.yaml` gains a `harnesses:` field accepting:

- `auto` (default): detect from disk on first init (presence of `.opencode/` → opencode, `.cursor/` → cursor, `.claude-plugin/` or `CLAUDE.md` → claude), then pin the detected list
- `all`: write managed files for every supported harness regardless of disk presence
- explicit list, e.g. `[opencode, cursor]`: write only the named harnesses
- empty/none: skip all harness file management

`specgraph init --harness <name>` and `--harness all` flags override for one-off runs.

### Excluded from the manifest

- **Skills**: served via `specgraph://skills/<name>`; no per-project files.
- **User-level installs** (`~/.config/opencode/...`, `~/.cursor/mcp.json`, Cursor User Rules): project-level wins on conflict; documented as opt-in manual setup for power users.
- **`.specgraph.yaml`**: user-owned project config; init creates with the project slug if missing, never overwrites.

## Doctor + drift nudge

### `specgraph doctor`

```text
specgraph doctor [flags]

Flags:
  --json              Machine-readable output
  --fix               Auto-fix Stale/Missing; print guidance for Drifted
  --harness <name>    Narrow to one harness
  --scope <s>         project (default; user/all not implemented in v1)
```

Four check groups: **Binary** (version, build), **Server** (subsumes `specgraph health`: reachability + MCP transport handshake), **Project config** (`.specgraph.yaml` parses, `harnesses:` resolves), **Managed files** (14-file table grouped by host-pinned vs SpecGraph-owned).

`specgraph health` becomes a deprecated alias for `specgraph doctor server`, keeping its current exit codes for scripts.

### Drift-nudge on every CLI invocation

A `PersistentPreRun` hook on the root cobra command runs `InspectAll` and emits one stderr line if any file is not Synced:

```text
note: 2 managed files out of date with this binary (1 stale, 1 drifted); run `specgraph init` to refresh, `specgraph doctor` for details
```

**Skip rules — primary gate is `isatty(stderr)`.** If stderr isn't a terminal, the nudge never fires. This catches every non-interactive invocation by construction: the Claude session-start hook (`specgraph read-mcp-resource specgraph://prime`), the MCP server (`specgraph serve`), shell-script consumers piping stdout/stderr, CI runs. Without this guard, the Claude hook would inject the nudge text **into the prime payload the model reads**, actively poisoning the harness — a class-of-bug we close once at the framework level rather than maintaining a brittle subcommand allow-list.

The `isatty(stderr)` gate has one well-known limitation: `specgraph list | less` keeps stderr as a TTY, so the nudge can smear across pager output. The escape hatch is the `SPECGRAPH_DRIFT_NUDGE=off` env var or `nudges: { quiet: true }` in `.specgraph.yaml`. Acceptable trade-off; documented for users who run pager pipelines frequently.

Additional skip conditions (belt-and-suspenders):

- Subcommands that handle reporting themselves: `init`, `doctor`, `health`, `read-mcp-resource`, `serve`, anything under `mcp` or `version`
- `SPECGRAPH_DRIFT_NUDGE=off` env var (user-level mute)
- `nudges: { quiet: true }` in `.specgraph.yaml` (project-level mute)
- Throttle (see below)

**Throttle path:** `xdg.CacheHome() + "/nudges/" + sha256(EvalSymlinks(projectRoot)) + "-" + binaryVersionHash`. Default expansion: `~/.cache/specgraph/nudges/<project-hash>-<version-hash>`. The version is part of the **path**, not the contents — that way two binaries on PATH (e.g., a stable install and a dev-tagged build) each track their own throttle independently and can't ping-pong each other into re-emitting on every alternation.

The file's contents are empty; mtime drives the throttle window.

**Throttle policy:**

- First nudge ever for `(project, binary-version)`: emit, create file
- Same `(project, binary-version)` within 24h: skip
- Same project, *different* binary version: emit (binary upgraded; user sees one fresh nudge for the new version) — that path's throttle file is independent
- Older than 24h, same version: emit again, refresh mtime

**Opportunistic GC:** on each nudge, scan `xdg.CacheHome() + "/nudges/"` for entries with mtime > 30 days; delete them. One `readdir` is well within the per-invocation budget and prevents indefinite accumulation as users move/rename projects.

**Fallback for unknown errors** (e.g., `~/.cache/` not writable, `EvalSymlinks` errors on a deleted cwd): emit the nudge unthrottled — minor over-emission, not broken. Never make the CLI fail because of an advisory feature.

`internal/xdg/` adds a `CacheHome()` function alongside the existing `ConfigHome()` / `DataHome()` / `StateHome()`, mirroring the spec one-for-one.

## Migration plan + sequencing

### Children of `spgr-rwrp`, in landing order

```text
0 (Claude API verification spike)
     ↓
A (foundation) → B (port existing) → ┬→ C (OpenCode)
                                     ├→ D (Cursor)
                                     ├→ E (Claude)
                                     ├→ F (Skills via MCP)
                                     └→ G (Doctor + nudge)
```

PR 0 runs before PR A — it's a verification spike, not implementation. PR A and PR B are sequential. C, D, E, F, G can land in any order or in parallel after B.

#### PR 0 — Claude plugin API verification spike (gates PR A)

Three claims in §"Per-harness manifest" and §"Claude" hinge on Claude Code behaviour we have asserted but not verified end-to-end. Verify each with a minimal in-tree test before PR A locks the framework's design assumptions:

1. **`extraKnownMarketplaces` accepts `source: { type: "directory", path: "<relative>" }`** — write a throwaway `.claude/settings.json` with a relative path, run `claude /plugin list`, confirm the plugin shows up.
2. **`autoUpdate: false` is honoured at the marketplace-entry level** — toggle it, observe Claude does not auto-update the plugin from the directory source. Cite the exact docs anchor that documents the field.
3. **`${CLAUDE_PLUGIN_ROOT}` resolves to the plugin root, not the marketplace root, for `directory`-source local marketplaces** — write a hook script that prints `pwd` and `$CLAUDE_PLUGIN_ROOT`, fire it via session-start, observe the resolved path.

If any claim fails, the spec changes before PR A. Most likely fallout if a claim fails:

- (1) fails → use absolute paths (resolved at init time) instead of relative; spec's relative-path examples become absolute; `dev` build tag and dogfood story unaffected
- (2) fails → drop `autoUpdate: false` from managed keys; rely on the local marketplace's default behaviour
- (3) fails → re-architect Claude layout: the plugin root might need to be `.specgraph/agents/claude/.claude-plugin/` (one level deeper) so the marketplace root and plugin root coincide, breaking the "siblings to .claude-plugin/" model in §"Per-harness manifest"

**Output:** a verification report at `docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md` (date-prefixed slug, consistent with existing `docs/plans/` convention). The report greens or reds each of the three claims with a captured terminal transcript or test artifact.

**Gating PR A:**

- PR A's commit message includes `Depends on: docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md (all three claims green)`
- PR A's pull request description repeats the dependency
- Where feasible, PR A includes a `testdata/` fixture distilled from PR 0 that fails the build if any claim regresses (e.g., a `marketplace.json` + `plugin.json` pair tested via a synthetic Claude harness — if the harness API exists). If a build-time test isn't feasible (because Claude isn't available in CI), the dependency is honor-system but at least visibly recorded.

#### PR A — `internal/config/managedfiles/` foundation

- New package with types: `ManagedFile` (including `SupersedesPath`), `Strategy`, `State`, `FileState`, `SyncResult`
- Sentinel parsing + writing for `ts` / `sh` / `md` / `mdc` comment styles
- Hash computation (canonical content excluding sentinel line)
- **Port (not stub) the safety primitives from existing `internal/config/pointers/`**: file locking (`lock_unix.go`, `lock_windows.go`), `O_NOFOLLOW` symlink rejection, atomic writes via `O_CREATE|O_EXCL` + rename. These are non-negotiable correctness guarantees, not scaffolding — the new framework must match the existing surface from day one.
- Supersedes-path deletion logic with guard rails:
  - When a manifest entry sets `SupersedesPath`, init computes the hash of the on-disk old file (excluding any sentinel) and compares it against what *this binary's previous canonical for that old path* would have written
  - If hashes match: delete the old path after the new file is written and fsync'd (clean rename)
  - If hashes don't match: leave the old file alone, classify the old path as `Drifted` in doctor output (so users see the file and can decide). Doctor's `--fix` does NOT auto-delete drifted supersedes-path files
  - Manifest invariant: no two entries may declare the same `SupersedesPath` (validated at registry construction; panic if violated, since it's a static manifest error)
  - Deletion failures (permission denied, etc.) are logged and reported by doctor but don't block the new-file write
- `InspectAll` and `SyncAll` plumbing with empty manifests
- `xdg.CacheHome()` added to `internal/xdg/`
- `dev` build tag: swap `//go:embed` for `os.ReadFile` against canonical source paths. Standard Go pattern for source-tree iteration. See "Dogfood discipline" below for the safety mitigations that ship alongside.
- Tests cover sentinel round-tripping, hash stability, comment-syntax variants, locking under concurrent writers, symlink rejection, atomic-write semantics
- **Zero behaviour change for users.** Pure scaffolding (correctly defined: same semantics as today's `pointers/`, generalized over more file types).

#### PR B — Port existing managed files into the framework

Migrate the 5 already-managed files into the new framework:

- `.mcp.json`, `.cursor/mcp.json`, `opencode.json` → `JSONKeyMerge`
- `AGENTS.md`, `.cursor/rules/specgraph-bootstrap.mdc` → `MarkdownBlock` (with `SupersedesPath: ".cursor/rules/specgraph-bootstrap.md"` so the old `.md` is deleted when the new `.mdc` is written)

Existing `mcpconfigs/` and `pointers/` packages get folded in (their tests come along, ported to use the new types). Both packages are deleted at the end of this PR — but their *rendering algorithms* are preserved as private helpers inside `internal/config/managedfiles/` for the v=1 → v=2 hash-check and `SupersedesPath` "prior canonical" comparisons. Those helpers sunset per the policy described in §"Drift detection / Vestigial v=1 renderer."

`MarkdownBlock` strategy extends today's `<!-- specgraph:init:start v=1 -->` markers to `<!-- specgraph:init:start v=2 sha256=... -->`. The state machine recognizes `v=1` as `Stale` (no hash to check; trust-and-refresh, atomic-write `v=2`). `JSONKeyMerge` strategy needs no sentinel (managed keys are always overwritten on init).

#### PR C — OpenCode plugin under `.specgraph/agents/opencode/` (delivers `spgr-zqpb`)

- Embed `plugin/opencode/.opencode/plugins/specgraph.ts` into binary
- Add `.specgraph/agents/opencode/specgraph.ts` to manifest as `WholeFile`
- Add `plugin` array key to `opencode.json` managed keys
- Remove `.opencode/plugins/specgraph.ts` from the dogfood repo (was added in PR #941)
- Remove the `plugin` array entry pointing at the old path from the dogfood `opencode.json`
- Re-run `plugin/opencode/SMOKE_TEST.md` end-to-end against the new path

#### PR D — Cursor rule files at `.cursor/rules/`

- Repo-internal renames: source `.md` → `.mdc`, `post-stage.mdc` → `specgraph-post-stage.mdc`
- Embed renamed sources into binary
- Add 2 entries to manifest as `WholeFile`
- Test: open in real Cursor, verify rules appear under Rules panel and apply on prompts

#### PR E — Claude plugin shim under `.specgraph/agents/claude/`

- Repo-internal renames: hook scripts get `specgraph-` prefix; `plugin.json` references update
- Embed plugin shim sources (5 files) into binary
- Add 5 entries to manifest as `WholeFile` under `.specgraph/agents/claude/`
- Add `.claude/settings.json` to manifest with managed keys: `extraKnownMarketplaces.specgraph-local`, `enabledPlugins["specgraph@specgraph-local"]`
- **Verified by PR 0** (see `2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`):
  - `extraKnownMarketplaces` accepts `source: { type: "directory", path: "<relative>" }` with a project-relative path. ✓ GREEN
  - `autoUpdate: false` is honoured at the marketplace-entry level. ✓ GREEN with caveat — registry version is pinned, but for `directory`-source marketplaces Claude reads files live (no cache copy), so file-content drift is not blocked by the flag. Acceptable for our model: every `specgraph init` refresh is supposed to be visible to the next session.
  - `${CLAUDE_PLUGIN_ROOT}` resolves to the plugin's own root directory regardless of marketplace shape. ✓ GREEN
- Test: spin up `claude` in a fresh project, run `specgraph init`, verify the plugin shows in `/plugin list` and hooks fire on session start
- Author `plugin/specgraph/SMOKE_TEST.md` analog to OpenCode's

#### PR F — Skills via MCP resource (chosen mechanism; symlinks removed)

Independent of A/B/C/D/E:

- Embed `<repo>/skills/` tree into the CLI binary
- Add MCP resource handler for `specgraph://skills/<name>`
- Add MCP tools: `specgraph_skills_list`, `specgraph_skills_get`
- Update prime template to include the skills catalog
- Delete the `plugin/<harness>/skills` symlinks in all three shims
- Delete `task plugin:sync`
- Doctor's "Integration" section grows a "Skills (N served via MCP)" line
- **Documentation cleanup** (must land alongside the symlink deletion or downstream readers see stale info):
  - `CLAUDE.md`: rewrite the "Plugin shims" and "Shared skills" paragraphs to reflect MCP-fetch delivery; remove `task plugin:sync` references
  - `plugin/claude/README.md`, `plugin/cursor/README.md`, `plugin/opencode/README.md`: remove any "skills are symlinked" language; describe the MCP-resource fetch model
  - Top-level `README.md` if it references the symlink/sync pattern (verify during PR F)

PR F commits to MCP-fetch as the chosen delivery mechanism (per §"Skills delivery"). If real-user feedback later indicates a Claude UX regression is unacceptable, a follow-up bead adds Claude-specific on-disk skill writes — but PR F itself is not a parallel-run experiment.

#### PR G — `specgraph doctor` + drift nudge

- Implement `specgraph doctor` cobra command
- Implement `PersistentPreRun` drift-nudge with the throttle file at `xdg.CacheHome()/nudges/<hash>`
- Honour `SPECGRAPH_DRIFT_NUDGE=off` and `nudges: { quiet: true }`
- `--fix` mode: auto-runs init for Stale/Missing; prints guidance for Drifted
- Tests: state-machine table-driven tests for the four states, throttle test with synthetic mtime, e2e test that runs init then doctor and verifies clean output

### Dogfood discipline

This repo runs the same install flow as end users. The authoring source `plugin/<harness>/...` is the only canonical file; `.specgraph/agents/<harness>/...` and `.cursor/rules/specgraph-*.mdc` are init-written destinations and are gitignored. Dev cycle:

1. Edit `plugin/<harness>/...` source
2. `task build` (re-embeds)
3. `specgraph init` (re-writes destinations)
4. Test in the harness

Four tooling pieces, available to anyone (not dogfood-only):

- **`task plugin:refresh`**: combines build + init.
- **`task plugin:check`**: runs `task build && specgraph init --dry-run --check` and exits non-zero if any managed file would change. PR G adds it to the `task check` chain so CI catches "I edited a shim source and forgot to rebuild + re-init." Without this, the dogfood discipline is honor-system and rots the second someone is in a hurry. (Today's `Taskfile.yml` doesn't include this target — it's introduced in PR G.)
- **`dev` build tag**: swaps `//go:embed` for `os.ReadFile` against canonical source paths; `go build -tags dev ./cmd/specgraph` produces a binary that reads-from-disk on every init. Standard Go pattern.
- **`task plugin:watch`** (defer): file watcher that runs `task plugin:refresh` on changes under `plugin/<harness>/`. Belongs in dev tooling, not the binary.

**`dev` build tag safety mitigations:**

- The dev binary emits a stderr banner on **interactive** invocations (gated on `isatty(stderr)`, same primitive as the drift-nudge): `specgraph: DEV BUILD — embedded files read from disk at <path>`. Loud and unmissable when a developer runs the binary at a terminal; silent when the binary is invoked by tooling, CI, or hooks. This preserves the safety property (devs see the warning every time they touch the binary) without polluting captured stderr in scripts. Dev binaries aren't shipped to CI in normal flows; if they are, the `isatty` gate avoids smearing the banner across captured logs.
- Version string discipline: dev builds carry a `+dev.<short-sha>` suffix on the version (e.g., `0.2.1+dev.cef1ec3a`), never a clean semver. `specgraph version` prints it loudly. This prevents "I shipped a release but the binary I tested was tagged dev."
- CI assertion: a release-pipeline check greps the embedded build info (`runtime/debug.ReadBuildInfo()`) on every release artifact and fails if the `dev` tag is present. Belt-and-suspenders against goreleaser misconfiguration.

### Test strategy

- **Unit:** each strategy implementation × ~6 cases (missing/synced/stale/drifted/sentinel-absent/sentinel-malformed)
- **Integration:** `internal/config/managedfiles/` exercised against temp dirs with realistic content. Cross-strategy interaction in one Sync call. Partial-failure semantics. Idempotency.
- **E2E:** per-harness manual smoke procedures, all checked in (`plugin/<harness>/SMOKE_TEST.md`). Each verifies the harness loads what init wrote. Run on demand; CI runs unit + integration.

## What's NOT in this epic

- **`spgr-eo4n`** (Claude marketplace publish) — stays parallel. Once published, init writes a slightly different `extraKnownMarketplaces` entry pointing at the marketplace instead of the local directory.
- **`spgr-sa95`** (OpenCode npm publish) — same; parallel, optional.
- **User-level installs** (`~/.config/opencode/`, `~/.cursor/mcp.json`, Cursor User Rules) — out of scope. Project-level wins on conflict per harness docs.
- **Auto-init on binary upgrade** — Section 5's nudge surfaces staleness; we don't silently re-init.

## Open questions

(None remaining at design time — both prior open questions resolved during v2 review.)

### Resolved (during adversarial review)

- **`dev` build tag location:** PR A. Small addition, immediately useful for PRs C/D/E iteration.
- **`enabledPlugins` semantics:** preserve user-set `false`. If a user runs `/plugin disable specgraph`, init must NOT silently re-enable on next run. Init only ensures the key *exists*; it never overwrites a user-flipped `false`.
- **Orphans under `.specgraph/agents/<harness>/`** (e.g., a harness gets removed from the user's `harnesses:` list, or an old binary's manifest entry is dropped from a newer binary): doctor surfaces them in a dedicated "Orphans" section of the report. The orphan bar is **two-pronged**:
  - The file lives under `.specgraph/agents/<harness>/` and is **not** in the embedded manifest for the current binary.
  - The file carries a `specgraph:init:` sentinel.

  Both conditions must hold. Files under `.specgraph/agents/<harness>/` *without* a sentinel are user content (e.g., a developer's `notes.md`); doctor surfaces them as informational only and never offers to delete. `specgraph init --remove-orphans` only deletes sentinel-carrying files whose paths are absent from the current manifest — never user-authored content lacking a sentinel.
- **Skills engagement measurement** (was: what threshold triggers the Claude-on-disk fallback): obsoleted by the v2 decision to commit to MCP-fetch as the chosen mechanism in PR F. No measurement window. If user feedback later indicates the regression is unacceptable, we add Claude on-disk writes as a chosen change, not an experimental one.

## References

- Predecessor epic: `spgr-cceg` — harness parity (the three-shim pattern this epic completes the install story for)
- First child: `spgr-zqpb` — OpenCode plugin embed-and-write (delivered by PR C)
- Surfaced this epic: `spgr-f0di` — OpenCode plugin validation (PR #941 fixed the literal-vs-structural tool match; smoke test exposed install-vector gap)
- Marketplace publish (parallel, optional): `spgr-eo4n` — Claude Code marketplace
- npm publish (parallel, optional): `spgr-sa95` — `@specgraph/opencode-plugin`

### External

- Claude Code plugins: <https://code.claude.com/docs/en/plugins>, <https://code.claude.com/docs/en/discover-plugins>, <https://code.claude.com/docs/en/plugins-reference>, <https://code.claude.com/docs/en/settings#configuration-scopes>
- OpenCode: <https://opencode.ai/docs/plugins>, <https://opencode.ai/docs/config>, <https://opencode.ai/docs/rules>
- Cursor: <https://cursor.com/docs/context/rules>, <https://cursor.com/docs/context/mcp>
- XDG Base Directory Spec: <https://specifications.freedesktop.org/basedir/latest/>

## Revision history

- **2026-05-08 v1**: initial design from brainstorming session
- **2026-05-08 v2**: first adversarial review applied. Major revisions:
  - **C1**: marker namespace stays `specgraph:init:` (not renamed to `specgraph:managed-block`); v=1 → v=2 upgrade path defined
  - **C2**: `SupersedesPath` field on `ManagedFile` for `.md` → `.mdc` cleanup
  - **C3**: drift-nudge gates on `isatty(stderr)`, not just a subcommand allow-list — closes the "Claude session-start hook injects nudge into prime payload" hole at the framework level
  - **S1**: skills-via-MCP regression on Claude called out explicitly; PR F initially proposed additive-only landing
  - **S2**: `StrategicMerge` strategy dropped (YAGNI)
  - **S3**: `dev` build tag mitigations (stderr banner, version-string discipline, CI release check)
  - **S4**: throttle file opportunistic GC; per-binary-version 24h throttle (not per-minute)
  - **S6**: `task plugin:check` wired into `task check` to enforce dogfood discipline
  - **M1**: PR A explicitly ports (not stubs) locking, symlink rejection, atomic-write primitives
  - **M4**: preserve user-set `enabledPlugins: false` (don't silently re-enable)
- **2026-05-08 v5**: PR 0 (Claude API verification spike) completed. All three claims green; two schema corrections folded in:
  - **Spec correction 1:** `extraKnownMarketplaces.<name>.source.path` points at the **marketplace root** (the dir containing `.claude-plugin/`), NOT at `.claude-plugin/` itself. Manifest entry 9's note updated to clarify the path value is `./.specgraph/agents/claude` (not `./.specgraph/agents/claude/.claude-plugin`).
  - **Spec correction 2:** `marketplace.json`'s `plugins[].source` is a directory path with mandatory `./` prefix (e.g. `"./"` for a co-located plugin), NOT a path to plugin.json. Manifest entry 11's note updated. `..` traversal is blocked, so any future multi-plugin layout must keep all plugins inside the marketplace root.
  - **Bonus finding (informational):** `autoUpdate: false` is honoured at the registry layer (version stays pinned in `installed_plugins.json`) but for `directory`-source marketplaces Claude reads files live with no cache copy, so file-content drift is not gated. Acceptable for our model — every `specgraph init` refresh should be visible to the next session.
  - **Bonus finding (informational):** Claude resolves symlinks on the plugin path. Spec's choice of `EvalSymlinks(projectRoot)` for the throttle file key matches Claude's internal behaviour.
  - PR E's "verify before committing" subsection is now "verified by PR 0," referencing the verification report.
- **2026-05-08 v4**: third adversarial review applied (diminishing-returns review; no critical findings). Revisions:
  - **v3-1 (vestigial v=1 renderer sunset)**: `mcpconfigs/` and `pointers/` rendering algorithms preserved as private helpers in `internal/config/managedfiles/` for the v=1 → v=2 hash-check and `SupersedesPath` "prior canonical" comparisons. Sunset triggers when `task plugin:check` reports zero v=1 files for two consecutive releases.
  - **v3-2 (PR F documentation cleanup)**: PR F now explicitly updates `CLAUDE.md`, `plugin/<harness>/README.md` files, and the top-level `README.md` to remove symlink + `task plugin:sync` references. Avoids stale docs after the symlink deletion lands.
  - **PR 0 nits**: explicit output filename (`docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`); PR A dependency recorded in commit message + PR description; build-time test fixture where feasible.
  - **`JSONKeyMerge` exception**: `enabledPlugins["specgraph@specgraph-local"]` is "managed-presence not managed-value" — explicit caveat added to the strategy description so a fresh reader doesn't trip on the seeming contradiction with "managed keys are always overwritten."
  - **Orphan definition tightened**: orphan = sentinel-carrying file under `.specgraph/agents/<harness>/` whose path is absent from the current manifest. User-authored files lacking a sentinel are surfaced as informational only; never deleted by `--remove-orphans`.
- **2026-05-08 v3**: second adversarial review applied. Major revisions:
  - **v2-C1 (corruption-rejection collision)**: v=1 → v=2 upgrade now hashes disk content against this binary's prior canonical; mismatch is `Drifted`, not `Stale`. Forward-incompat (`v=3+`) explicitly rejected as corruption
  - **v2-C2 (deferred verification on critical path)**: added PR 0 — Claude API verification spike that gates PR A. Three claims (`directory` source with relative path, `autoUpdate: false` honoured, `${CLAUDE_PLUGIN_ROOT}` resolves to plugin root) are verified before the framework's design assumptions lock
  - **v2-S1 (`SupersedesPath` guard rails)**: hash-check the old file before deleting; mismatch leaves it alone and surfaces as `Drifted` on the old path. Manifest invariant: no two entries declare the same `SupersedesPath`
  - **v2-S3 (throttle ping-pong between binaries)**: binary version moved from file contents into the path itself (`<project-hash>-<version-hash>`) so two binaries on PATH track independently
  - **v2-S4 (PR F dual sources of truth)**: PR F now commits to MCP-fetch as the chosen mechanism; symlinks and `task plugin:sync` removed in PR F itself, not deferred. Claude-specific on-disk fallback is a chosen-change follow-up if user feedback warrants, not a measurement-window experiment
  - **v2-S5 (dev banner has no isatty gate)**: dev banner now also gates on `isatty(stderr)` — visible interactively, silent in pipes/CI
  - **v2-S2, v2-M2, v2-M3, v2-M4**: small wording fixes; orphan-file handling resolved in Open Questions (doctor surfaces, `init --remove-orphans` deletes)
