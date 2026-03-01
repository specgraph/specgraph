# CLAUDE.md

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — specifications as a queryable graph, not static markdown. Constitution (project ground truth), spec schema, authoring funnel, and storage + query layer feeding execution engines like Gastown.

**Status:** Phase 1 in progress. Slice 2 (Constitution) is implemented.

## Commands

All automation is via Taskfile.dev. Run `task --list` for the full catalog. Key commands: `task build`, `task test`, `task proto`, `task lint`, `task fmt`.

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

## Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage, claim protocol, execution bundles, CLI, linter
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, exports, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance
