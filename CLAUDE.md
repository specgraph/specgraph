# CLAUDE.md

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — specifications as a queryable graph, not static markdown. Constitution (project ground truth), spec schema, authoring funnel, and storage + query layer feeding execution engines like Gastown.

## Commands

All automation is via Taskfile.dev. Run `task --list` for the full catalog.

### Quality Gates (MUST use)

| Task | When to run | What it does |
|------|-------------|--------------|
| `task check` | Before every `git push` (also runs automatically via pre-push hook) | fmt:check → license:check → lint → build → unit tests (excludes memgraph/Docker) |
| `task pr-prep` | Before opening/updating a PR (requires Docker) | clean → generate → check → e2e tests |

- **MUST run `task check`** after finishing a batch of changes and before pushing. The pre-push hook enforces this automatically, but run it manually first to catch issues early.
- **SHOULD run `task pr-prep`** before opening a PR or marking it ready for review. This is the full pipeline including e2e tests.
- **Do NOT rely solely on lefthook pre-commit** for lint validation — it only checks staged files per commit and misses cross-scope issues like `govet -shadow` and `wrapcheck`.

### Common Commands

| Task | Purpose |
|------|---------|
| `task build` | Build the specgraph binary (runs `task proto` first) |
| `task test` | Run all tests including memgraph integration (requires Docker) |
| `task test:short` | Run short tests only |
| `task proto` | Generate Go code from protobuf (incremental, skips if unchanged) |
| `task lint` | Run all linters (Go, Markdown, YAML) |
| `task fmt` | Format all files (Go, YAML, Markdown, dprint) |

## Domain Concepts

- **Specs are graph nodes** with first-class edges (dependencies, blocks, compositions)
- **Constitution**: Layered ground truth (User → Org → Project → Domain). More specific layers override general ones.
- **Authoring funnel**: Spark → Shape → Specify → Decompose → Approve
- **Decisions are first-class nodes** (ADR-003) with bidirectional edges to specs
- **SpecGraph is upstream of Gastown** — SpecGraph does design; Gastown does execution via polecats (ephemeral worker agents)

## Gotchas

- **`gen/` is gitignored** — run `task proto` (or `task build`, which depends on it) after clone. Proto sources are in `proto/`, not `gen/`.
- **`task proto` is incremental** — fingerprints `.proto` files and skips if unchanged
- **Memgraph integration tests require Docker** — `internal/storage/memgraph/` uses testcontainers
- **Lefthook pre-commit hooks**: license headers (addlicense), golangci-lint, yamlfmt, dprint, rumdl, cog (conventional commits). All run in parallel.
- **Claude Code hooks**: `task fmt` runs after Edit/Write, `task lint` runs after Bash, edits to `gen/` are blocked via PreToolUse
- **ConnectRPC, not plain gRPC** — handlers are in `internal/server/`, proto services generate both `.pb.go` and `.connect.go` files
- **Storage interfaces in `internal/storage/`** — implementations are in subdirectories (currently only `memgraph/`). The interfaces use domain types, not protobuf types.
- **License headers required** — all `.go`, `.sh`, `.py`, `.proto` files need SPDX headers. Run `task license:add` to fix.
- **Memgraph bolt readiness race** — `wait.ForListeningPort` alone is insufficient; always pair with `wait.ForLog("You are running Memgraph")` and a connection retry loop (see `newStore` in `memgraph_test.go`)
- **E2E tests use Ginkgo/Gomega** — `e2e/api/` tests run via `go test -tags e2e`; `e2e/docker/` tests require Docker-in-Docker (skipped in CI)
- **Go test glob `./pkg/...` vs `./pkg/`** — ellipsis recurses into subdirs. CI uses `./internal/storage/` (no ellipsis) to avoid pulling in `memgraph/` integration tests into the unit test step
- **Docker compose templates manage DB only** — `internal/docker/compose.go` templates start Memgraph or Postgres containers; the SpecGraph process runs natively and connects to the containerized DB
- **Handler error sanitization** — `stageError` and similar methods sanitize internal errors before returning to clients. Test assertions MUST use error codes (`connect.CodeInternal`, `connect.CodeNotFound`), not error message strings.
- **Mock backends must use sentinel errors** — When handler code uses `errors.Is()` checks (e.g., `storage.ErrSpecNotFound`, `storage.ErrDecisionNotFound`), mock/fake backends must return these sentinel errors, not `fmt.Errorf()`.
- **DECIDED_IN edge direction** — Per ADR-003, DECIDED_IN edges go from spec → decision. In `acceptLinkedDecisions`, `edge.ToID` is the decision slug.
- **Lefthook pre-commit is file-scoped** — only lints staged files per commit. Cross-scope linters (`govet -shadow`, `wrapcheck`) need full package context. Use `task check` (see Quality Gates above) to catch what pre-commit misses.

## Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage, claim protocol, execution bundles, CLI, linter
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, exports, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance
