# CLAUDE.md

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — specifications as a queryable graph, not static markdown. Constitution (project ground truth), spec schema, authoring funnel, and storage + query layer feeding execution engines like Gastown.

## Commands

All automation is via Taskfile.dev. Run `task --list` for the full catalog.

### Quality Gates (MUST use)

| Task | When to run | What it does |
|------|-------------|--------------|
| `task check` | Before every `git push` (also runs automatically via pre-push hook) | fmt:check → license:check → lint → build → unit tests (excludes postgres integration/Docker) |
| `task pr-prep` | Before opening/updating a PR (requires Docker) | check → test:integration → test:e2e |

- **MUST run `task check`** after finishing a batch of changes and before pushing. The pre-push hook enforces this automatically, but run it manually first to catch issues early.
- **SHOULD run `task pr-prep`** before opening a PR or marking it ready for review. This is the full pipeline including e2e tests.
- **Do NOT rely solely on lefthook pre-commit** for lint validation — it only checks staged files per commit and misses cross-scope issues like `govet -shadow` and `wrapcheck`.

### Common Commands

| Task | Purpose |
|------|---------|
| `task build` | Build the specgraph binary (runs `task proto` first) |
| `task test` | Default `go test ./...` — skips `//go:build integration` and `//go:build e2e` suites |
| `task test:short` | Run short tests only |
| `task proto` | Generate Go code from protobuf (incremental, skips if unchanged) |
| `task lint` | Run all linters (Go, Markdown, YAML) |
| `task fmt` | Format all files (Go, YAML, Markdown, dprint) |

### Dev Setup (fresh clone)

```bash
task tools          # Install all dev tools (golangci-lint, buf, lefthook, etc.)
task hooks:install  # Install git hooks (pre-commit, pre-push, commit-msg)
task build          # Generate proto + build binary
```

### Doctor + drift-nudge

`specgraph doctor` reports four check groups (Binary, Server, Project
config, Managed files). Default output is compact when everything is
green; sections expand when problems exist. `--json` for
machine-readable output; `--fix` auto-init's Stale/Missing rows and
prints guidance for Drifted; `--harness <name>` narrows; `--exit-zero`
suppresses non-zero exit for advisory use.

Every CLI invocation runs a drift-nudge in `PersistentPreRun` that
emits one stderr line if any managed file is non-Synced. Skip gates:
the subcommand allow-list (`init`, `doctor`, `health`, etc.),
`isatty(stderr)`, `SPECGRAPH_DRIFT_NUDGE=off`, `.specgraph.yaml`'s
`nudges.quiet: true`, and a 24h throttle file at
`xdg.CacheHome()/nudges/`.

`task plugin:refresh` rebuilds + re-init's against the current
project. `task plugin:check` runs `init --check` and exits non-zero
if any managed file would be modified — it's wired into `task check`
so a contributor who edited `plugin/<harness>/...` without rebuilding
sees the failure during the same `task check` they run pre-push.

`.specgraph.yaml` gains two fields:

- `harnesses: [claude, cursor, opencode]` — per-project allow-list
  (empty = all three, matching the legacy behaviour).
- `nudges: { quiet: true }` — project-level mute for the drift-nudge.

## Domain Concepts

- **Specs are graph nodes** with first-class edges (dependencies, blocks, compositions)
- **Constitution**: Layered ground truth (User → Org → Project → Domain). More specific layers override general ones.
- **Authoring funnel**: Spark → Shape → Specify → Decompose → Approve
- **Decisions are first-class nodes** (ADR-003) with bidirectional edges to specs
- **SpecGraph is upstream of Gastown** — SpecGraph does design; Gastown does execution via polecats (ephemeral worker agents)

## Architecture

| Directory | Contents |
|-----------|----------|
| `cmd/specgraph/` | CLI entry point |
| `proto/specgraph/v1/` | Protobuf service definitions (source of truth) |
| `gen/specgraph/v1/` | Generated Go code from proto (committed; regenerate with `task proto`) |
| `internal/authoring/` | Authoring funnel stage logic |
| `internal/config/` | Config loading and validation (YAML-based) |
| `internal/docker/` | Docker compose templates for DB containers |
| `internal/drift/` | Drift detection engine |
| `internal/driftscope/` | Drift scope analysis |
| `internal/emitter/` | Event/output emitters |
| `internal/config/pointers/` | Init-managed pointer files (AGENTS.md, .cursor/rules/specgraph-bootstrap.md) with managed-block fencing and file locking |
| `internal/server/` | ConnectRPC handlers + proto↔domain converters |
| `internal/storage/` | Storage interfaces (domain types, not protobuf) |
| `internal/storage/postgres/` | PostgreSQL implementation (pgx v5, recursive CTEs, testcontainers) |
| `internal/storage/postgres/migrations/` | goose SQL migrations (schema DDL) |
| `internal/storage/postgres/tx.go` | Transaction support (RunInTransaction, context-threaded tx via pgx) |
| `internal/render/` | Markdown renderers for CLI output — one file per entity type, functions accept proto types and return strings |
| `internal/sync/` | Sync adapters (beads, GitHub) with exec runner |
| `e2e/` | End-to-end tests (Ginkgo/Gomega, require Docker) |
| `docs/plans/` | Implementation plan documents |

## Jujutsu Workspaces

Use `jj workspace` commands instead of git worktrees. Workspaces share a single repo
store but provide independent working-copy commits in the DAG.

### Key Commands

- `jj workspace add ../dir-name` — create a new workspace (no branch-lock issues)
- `jj workspace list` — show all workspaces
- `jj workspace forget <name>` — untrack a workspace (manually delete the dir after)
- `jj workspace update-stale` — rebase working-copy commit if ancestry changed elsewhere
- `jj workspace root` — print the workspace root path

### When to Use

- Need two changes **on disk simultaneously** (e.g., running tests in one, coding in another)
- For simple context switching, prefer `jj edit <change-id>` or `jj new` — no workspace needed
- Multiple workspaces can operate on the same bookmark lineage (no branch-locking)

### Workflow

```sh
# create workspace for parallel work
jj workspace add ../project-creds
cd ../project-creds
jj edit <change-id>        # point at existing work
# ...work here, auto-snapshotted...

# back in main workspace, sync if needed
cd ../project-main
jj workspace update-stale
```

### Notes

- Conflicts from workspace updates are materialized (not blocked) — resolve at leisure
- `jj workspace forget` does NOT delete the directory on disk
- Workspaces are rarely needed for solo work; `jj edit` covers most context-switching

## Documentation

- **Example spec** — `site/docs/concepts/example-spec.md` is the canonical example spec on the public site. When proto messages for authoring stages change (`SparkOutput`, `ShapeOutput`, `SpecifyOutput`, `DecomposeOutput`), check if the example spec needs updating.
- **Authoring content** — workflow guidance (persona, orchestration, stage-specific instructions) lives in `internal/authoring/content/*.md`, embedded via `//go:embed` and composed into MCP prompt responses by `internal/authoring/composer.go`. When proto stage-output messages change (`ShapeOutput`, `SpecifyOutput`, `DecomposeOutput` in `proto/specgraph/v1/authoring.proto`), update both the proto AND any field references in `internal/authoring/content/stage-*.md`. The `TestContentProtoDrift` CI test catches drift for backticked snake_case tokens.
- **Plugin shims** — `plugin/specgraph/` (Claude Code), `plugin/cursor/` (Cursor), and `plugin/opencode/` (OpenCode) are thin per-harness shims that consume SpecGraph's skills via MCP fetch. Skills live in the CLI binary as embedded canonicals (`internal/mcp/skills/embedded/`) and are served via three tools (`specgraph_skills_list`, `specgraph_skills_get`, `specgraph_skills_search`) plus a templated resource (`specgraph://skills/<name>`). No on-disk skill copies in end-user projects. All three harnesses' canonical shim content lives under `internal/config/managedfiles/embedded/<harness>/` and is written to end-user projects by `specgraph init` (Cursor → `.cursor/rules/`, OpenCode → `.specgraph/agents/opencode/`, Claude → `.specgraph/agents/claude/` plus `.claude/settings.json`). The Claude shim has `.claude-plugin/plugin.json` (hooks array inlined; no separate `hooks.json`), `hooks/specgraph-session-start.sh` (reads `specgraph://prime` via the `specgraph read-mcp-resource` CLI subcommand), `hooks/specgraph-post-stage.sh` (PostToolUse on `mcp__specgraph__author` that surfaces analytical passes), and `routing-guide.md` (one-screen pointer). Hook scripts carry a `specgraph-` prefix to avoid colliding with user-added hooks. The Cursor shim has `.cursor/rules/specgraph.mdc` + `.cursor/rules/specgraph-post-stage.mdc`. The OpenCode shim has `.opencode/plugins/specgraph.ts` using `experimental.chat.system.transform` (prime injected once per session (cached after first CLI fetch)) + `tool.execute.after` (post-stage nudge queued for the next turn's system prompt). The `plugin/<harness>/` directories are reverse-symlinks for author convenience — editing via the symlinks edits the canonical embedded copy directly. See `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` for the parity contract.
- **Shared skills** — Six skills (`specgraph-authoring`, `specgraph-graph-query`, `specgraph-analytical-passes`, `specgraph-drift`, `specgraph-conventions`, `specgraph-troubleshooting`) live as agentskills.io-shape SKILL.md packages under `internal/mcp/skills/embedded/`. The repo-root `skills/` is a reverse-symlink to that directory so authors and the GitHub UI keep using the familiar path. Skills are served at runtime via MCP: `specgraph_skills_list` for the catalog, `specgraph_skills_get` (or the `specgraph://skills/<name>` resource) for a specific skill, `specgraph_skills_search` for keyword/regex lookup. `task skills:validate` (wired into `task check`) gates each SKILL.md against the schema, including the SpecGraph-local `summary:` extension and the kebab-case `name` regex (`skillvalidate.NameRegex`).
- **Per-harness MCP configs** — `.cursor/mcp.json` (Cursor), `.mcp.json` (Claude Code), and `opencode.json` (OpenCode) are managed by `specgraph init`. `init` syncs them on every run regardless of project state: missing files are created, existing files have managed fields (URL, Authorization header, X-Specgraph-Project header, harness-specific shape) reset to canonical values while siblings and user-added fields under the `specgraph` entry are preserved, and byte-equal files are reported as no-ops. See `internal/config/mcpconfigs/` for the writer code and `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md` for the design rationale.

## Gotchas

- **jj-colocated repo** — This repo uses jj with git colocated. Key rules:
  - Always use `jj --no-pager` for all jj commands
  - Always use `-m` with `squash`, `describe`, `commit`, `new` (avoids opening editor)
  - Never use `git push`; use `jj bookmark set <name> -r <rev>` then `jj git push --bookmark <name>`
  - MUST use `jj workspace add` instead of `git worktree` (git worktrees break colocated state)
- **`gen/` is committed** — generated proto code is checked in for Go module compatibility. Run `task proto:check` to verify staleness. Proto sources are in `proto/`, not `gen/`.
- **Proto field removal** — When removing a proto field, use `reserved` for both field number and name in the `.proto` file. Then run `task proto`, update all callers (CLI, handlers, tests), and verify with `go build ./...`.
- **`task proto` is incremental** — fingerprints `.proto` files and skips if unchanged
- **Postgres integration tests require Docker** — `internal/storage/postgres/` uses testcontainers with `pgvector/pgvector:pg18`. Wait strategy: `ForLog("database system is ready").WithOccurrence(2)`
- **Lefthook pre-commit hooks**: license headers (addlicense), golangci-lint, yamlfmt, dprint, rumdl, cog (conventional commits). All run in parallel.
- **Lefthook commit-msg hooks**: `cog` (conventional commits) and DCO sign-off check. All commits require `Signed-off-by:` trailer — use `git commit -s` or `jj describe` with trailer.
- **Claude Code hooks**: `task lint` runs after Bash, edits to `gen/` are blocked via PreToolUse (edit `.proto` sources instead). Formatting is handled by pre-commit hooks, not Claude Code PostToolUse.
- **ConnectRPC, not plain gRPC** — handlers are in `internal/server/`, proto services generate both `.pb.go` and `.connect.go` files
- **Storage interfaces in `internal/storage/`** — implementations are in subdirectories (currently only `postgres/`). The interfaces use domain types, not protobuf types.
- **License headers required** — all `.go`, `.sh`, `.py`, `.proto` files need `SPDX-License-Identifier: Apache-2.0` headers. Run `task license:add` to fix.
- **DCO required** — all commits must carry a `Signed-off-by:` trailer per the Developer Certificate of Origin. See `CONTRIBUTING.md`.
- **`revive` requires package comments** — new Go packages need a `// Package foo ...` doc comment on the first `.go` file or `revive` linter fails in `task check`.
- **`cmd/specgraph/table.go` still used** — `sync.go` and `prime.go` depend on `tableWriter`. Don't delete when migrating other commands to the render package.
- **`cmd/specgraph/output.go`** — shared `printJSON(proto.Message)` helper for `--json` flag output. All read commands use it.
- **CLI `--pass-type` uses friendly names** — `findings list --pass-type constitution-check`, not raw proto enum strings. Follows the `driftScopeToProtoMap` pattern in `lifecycle.go`.
- **gosec in test files** — Intentional permission changes (e.g., `os.Chmod(dir, 0o555)` for read-only tests) trigger gosec G302. Add `//nolint:gosec // <reason>` on the same line.
- **Postgres recursive CTEs for graph traversals** — `GetTransitiveDeps`, `GetImpact` use `CYCLE` clause (PG14+). `GetCriticalPath` uses manual path array + `unnest WITH ORDINALITY`. All bounded to 50 hops.
- **pgx v5 native driver** — use pgxpool, not database/sql. JSONB auto-marshals Go structs. `pgx.CollectRows` with `RowToStructByName` for scanning. goose migrations use `pgx/v5/stdlib` shim.
- **E2E tests use Ginkgo/Gomega** — `e2e/api/` tests run via `go test -tags e2e`; `e2e/docker/` tests require Docker-in-Docker (skipped in CI)
- **Go test glob `./pkg/...` vs `./pkg/`** — ellipsis recurses into subdirs. CI uses `./internal/storage/` (no ellipsis) to avoid pulling in `postgres/` integration tests into the unit test step
- **Docker compose templates manage DB only** — `internal/docker/compose.go` templates start Postgres containers; the SpecGraph process runs natively and connects to the containerized DB
- **Handler error sanitization** — `stageError` and similar methods sanitize internal errors before returning to clients. Test assertions MUST use error codes (`connect.CodeInternal`, `connect.CodeNotFound`), not error message strings.
- **Mock backends must use sentinel errors** — When handler code uses `errors.Is()` checks (e.g., `storage.ErrSpecNotFound`, `storage.ErrDecisionNotFound`), mock/fake backends must return these sentinel errors, not `fmt.Errorf()`.
- **DECIDED_IN edge direction** — Per ADR-003, DECIDED_IN edges go from spec → decision. In `acceptLinkedDecisions`, `edge.ToID` is the decision slug.
- **HAS_CHANGE edge is internal-only** — `HAS_CHANGE` (Spec → ChangeLog) is not in the proto `EdgeType` enum and is not exposed via `AddEdge`/`RemoveEdge` RPCs. It's created automatically by storage layer mutations.
- **Analytical findings are graph nodes** — `HAS_FINDING` edge (Spec → Finding) is internal, like `HAS_CHANGE`. Not exposed via `AddEdge`/`RemoveEdge`. Pass-specific findings stored via `StoreFindings` RPC in `AnalyticalPassService`, not inline in stage responses. Prompt templates embedded in server binary via `//go:embed`.
- **Analytical pass template overrides** — Place `<pass_type>.md` files (e.g., `constitution_check.md`) in a local directory and pass its path as `templateOverrideDir` to `RegisterAnalyticalPassService`. The handler checks the override directory first, then falls back to the embedded default. Typical convention: `.specgraph/templates/`.
- **All multi-query write paths MUST use `RunInTransaction`** (ADR-004) — Pass `txCtx` (not `ctx`) to `executeQuery`, `GetSpec`, `createChangeLog` inside the transaction. Queries automatically join via context. Validation that doesn't hit the DB stays outside to reduce lock time. See `tx.go` for the pattern.
- **Concurrent modifications return `ErrConcurrentModification`** — Mapped to `connect.CodeAborted` (retryable). Version guards in WHERE clauses detect conflicts. First writer wins; second fails fast.
- **`content_hash_at_link` on DEPENDS_ON edges** — DEPENDS_ON edges carry a `content_hash_at_link` property recording the upstream's ContentHash when the dependency was baselined. Set automatically by `AddEdge`, `StoreDecomposeOutput`, and refreshed on done-transition (`RecordCompletion`, `TransitionStage`, `UpdateSpec`) and drift acknowledgment. Drift detection compares this edge hash against the upstream's current ContentHash. Empty edge hash (unmigrated edges) always triggers drift — use `specgraph drift acknowledge <slug> --all` to baseline.
- **Use 4-backtick fences for nested code blocks** — when docs embed files containing ``` fences (e.g., implementation plan documents in `docs/plans/`), use ````markdown for the outer block. Bare ``` nesting creates broken/orphaned fences.

## Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage, claim protocol, execution bundles, CLI, linter
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, exports, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:

   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```

5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
