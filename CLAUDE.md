# CLAUDE.md

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — specifications as a queryable graph, not static markdown. Constitution (project ground truth), spec schema, authoring funnel, and storage + query layer feeding execution engines like Gastown.

## Commands

All automation is via Taskfile.dev. Run `task --list` for the full catalog.

### Quality Gates (MUST use)

| Task | When to run | What it does |
|------|-------------|--------------|
| `task check` | Before every `git push` (also runs automatically via pre-push hook) | fmt:check → license:check → lint → build → unit tests (excludes memgraph/Docker) |
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
| `internal/inject/` | Tool injection (AGENTS.md, CLAUDE.md, Cursor) with file locking |
| `internal/server/` | ConnectRPC handlers + proto↔domain converters |
| `internal/storage/` | Storage interfaces (domain types, not protobuf) |
| `internal/storage/memgraph/` | Memgraph implementation (Cypher queries, testcontainers) |
| `internal/storage/memgraph/changelog.go` | ChangeLog node operations (create, list, index, field change marshaling) |
| `internal/storage/memgraph/tx.go` | Transaction support (RunInTransaction, context-threaded tx) |
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
- **Skill personas** — Authoring skills live in `plugin/specgraph/skills/specgraph-*/SKILL.md`. The shared persona source of truth is `plugin/specgraph/skills/specgraph/persona.md`, symlinked into each skill's `references/` directory. When posture system or judgment heuristics change, update `persona.md` at the canonical location — symlinks propagate automatically.

## Gotchas

- **jj-colocated repo** — This repo uses jj with git colocated. Key rules:
  - Always use `jj --no-pager` for all jj commands
  - Always use `-m` with `squash`, `describe`, `commit`, `new` (avoids opening editor)
  - Never use `git push`; use `jj bookmark set <name> -r <rev>` then `jj git push --bookmark <name>`
  - MUST use `jj workspace add` instead of `git worktree` (git worktrees break colocated state)
- **`gen/` is committed** — generated proto code is checked in for Go module compatibility. Run `task proto:check` to verify staleness. Proto sources are in `proto/`, not `gen/`.
- **Proto field removal** — When removing a proto field, use `reserved` for both field number and name in the `.proto` file. Then run `task proto`, update all callers (CLI, handlers, tests), and verify with `go build ./...`.
- **`task proto` is incremental** — fingerprints `.proto` files and skips if unchanged
- **Memgraph integration tests require Docker** — `internal/storage/memgraph/` uses testcontainers
- **Lefthook pre-commit hooks**: license headers (addlicense), golangci-lint, yamlfmt, dprint, rumdl, cog (conventional commits). All run in parallel.
- **Claude Code hooks**: `task lint` runs after Bash, edits to `gen/` are blocked via PreToolUse (edit `.proto` sources instead). Formatting is handled by pre-commit hooks, not Claude Code PostToolUse.
- **ConnectRPC, not plain gRPC** — handlers are in `internal/server/`, proto services generate both `.pb.go` and `.connect.go` files
- **Storage interfaces in `internal/storage/`** — implementations are in subdirectories (currently only `memgraph/`). The interfaces use domain types, not protobuf types.
- **License headers required** — all `.go`, `.sh`, `.py`, `.proto` files need SPDX headers. Run `task license:add` to fix.
- **gosec in test files** — Intentional permission changes (e.g., `os.Chmod(dir, 0o555)` for read-only tests) trigger gosec G302. Add `//nolint:gosec // <reason>` on the same line.
- **Memgraph bolt readiness race** — `wait.ForListeningPort` alone is insufficient; always pair with `wait.ForLog("memgraph entered RUNNING state")` (supervisord log — the platform image does NOT emit "You are running Memgraph" to container stdout) and a connection retry loop (see `newStore` in `memgraph_test.go`)
- **Cypher DELETE + count** — `MATCH ()-[r]->() DELETE r RETURN count(r)` works in Memgraph; `r` was bound pre-deletion. No need to change to `count(*)`.
- **E2E tests use Ginkgo/Gomega** — `e2e/api/` tests run via `go test -tags e2e`; `e2e/docker/` tests require Docker-in-Docker (skipped in CI)
- **Go test glob `./pkg/...` vs `./pkg/`** — ellipsis recurses into subdirs. CI uses `./internal/storage/` (no ellipsis) to avoid pulling in `memgraph/` integration tests into the unit test step
- **Docker compose templates manage DB only** — `internal/docker/compose.go` templates start Memgraph or Postgres containers; the SpecGraph process runs natively and connects to the containerized DB
- **Handler error sanitization** — `stageError` and similar methods sanitize internal errors before returning to clients. Test assertions MUST use error codes (`connect.CodeInternal`, `connect.CodeNotFound`), not error message strings.
- **Mock backends must use sentinel errors** — When handler code uses `errors.Is()` checks (e.g., `storage.ErrSpecNotFound`, `storage.ErrDecisionNotFound`), mock/fake backends must return these sentinel errors, not `fmt.Errorf()`.
- **DECIDED_IN edge direction** — Per ADR-003, DECIDED_IN edges go from spec → decision. In `acceptLinkedDecisions`, `edge.ToID` is the decision slug.
- **HAS_CHANGE edge is internal-only** — `HAS_CHANGE` (Spec → ChangeLog) is not in the proto `EdgeType` enum and is not exposed via `AddEdge`/`RemoveEdge` RPCs. It's created automatically by storage layer mutations.
- **Analytical findings are graph nodes** — `HAS_FINDING` edge (Spec → Finding) is internal, like `HAS_CHANGE`. Not exposed via `AddEdge`/`RemoveEdge`. Pass-specific findings stored via `StoreFindings` RPC in `AnalyticalPassService`, not inline in stage responses. Prompt templates embedded in server binary via `//go:embed`.
- **All multi-query write paths MUST use `RunInTransaction`** (ADR-004) — Pass `txCtx` (not `ctx`) to `executeQuery`, `GetSpec`, `createChangeLog` inside the transaction. Queries automatically join via context. Validation that doesn't hit the DB stays outside to reduce lock time. See `tx.go` for the pattern.
- **Concurrent modifications return `ErrConcurrentModification`** — Mapped to `connect.CodeAborted` (retryable). Version guards in WHERE clauses detect conflicts. First writer wins; second fails fast.
- **`content_hash_at_link` on DEPENDS_ON edges** — DEPENDS_ON edges carry a `content_hash_at_link` property recording the upstream's ContentHash when the dependency was baselined. Set automatically by `AddEdge`, `StoreDecomposeOutput`, and refreshed on done-transition (`RecordCompletion`, `TransitionStage`, `UpdateSpec`) and drift acknowledgment. Drift detection compares this edge hash against the upstream's current ContentHash. Empty edge hash (unmigrated edges) always triggers drift — use `specgraph drift acknowledge <slug> --all` to baseline.
- **Use 4-backtick fences for nested code blocks** — when docs embed files containing ``` fences (e.g., SKILL.md content), use ````markdown for the outer block. Bare ``` nesting creates broken/orphaned fences.

## Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage, claim protocol, execution bundles, CLI, linter
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, exports, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance
