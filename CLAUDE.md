# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — a system for managing software specifications as a queryable graph rather than static markdown files. It provides a constitution (project ground truth), a spec schema, an authoring process with AI collaboration, and a live storage + query layer that feeds execution engines like Gastown.

**Status:** Active implementation. Slices 1 (Spec CRUD) and 2 (Constitution) are implemented.

## Repository Structure

```text
specgraph/
├── cmd/specgraph/          # Cobra CLI (init, serve, spec, constitution, claim, decision, graph)
├── proto/specgraph/v1/     # Protobuf definitions (buf-managed)
├── gen/specgraph/v1/       # Generated Go code (protoc-gen-go + connect)
├── internal/
│   ├── config/             # YAML config + constitution YAML support
│   ├── docker/             # Docker Compose lifecycle
│   ├── emitter/            # Constitution → tool file emitter (CLAUDE.md, .cursorrules, AGENTS.md)
│   ├── scanner/            # Tier 0 codebase scanner
│   ├── server/             # ConnectRPC handlers
│   └── storage/
│       └── memgraph/       # Memgraph (neo4j-go-driver/v5) backend
├── docs/
│   ├── initial-design-session/  # Spec, roadmap, ADRs
│   └── plans/                   # Implementation plans and tracker
└── LICENSE  # MIT
```

## Build & Test Commands

### Prerequisites

buf, golangci-lint, Docker, Go 1.25+, lefthook, cog, dprint, yamlfmt, rumdl, addlicense

### Commands

- `buf generate` — regenerate proto Go code after `.proto` changes
- `go build ./cmd/specgraph/` — build CLI binary
- `go test ./... -count=1 -timeout=120s` — run all tests (integration tests need Docker)
- `go test ./internal/scanner/ ./internal/emitter/ ./internal/server/ -count=1` — unit tests only (no Docker)
- `golangci-lint run ./...` — lint (revive, gosec, gocritic, staticcheck, errcheck, wrapcheck)
- `lefthook install` — install git hooks (run after clone or worktree creation)

## Code Patterns

- **ConnectRPC handlers**: `RegisterXxxService(mux, store)` in `internal/server/`, implement `specgraphv1connect.XxxServiceHandler`
- **Storage interfaces**: Define in `internal/storage/`, implement on `*memgraph.Store`
- **CLI commands**: One file per domain in `cmd/specgraph/`, use `newClient(specgraphv1connect.NewXxxServiceClient)` factory
- **Proto responses**: buf STANDARD lint requires unique response type per RPC — wrap with `GetXxxResponse`, `UpdateXxxResponse`
- **Memgraph storage**: Complex fields stored as JSON strings, use `marshalJSON`/`json.Unmarshal` for round-trip
- **File permissions**: `0o600` for files, `0o750` for directories (gosec G306)
- **Error wrapping**: Always `fmt.Errorf("context: %w", err)` (wrapcheck)
- **License header**: `// SPDX-License-Identifier: MIT` + `// Copyright 2026 Sean Brandt` on every file
- **Integration tests**: Use `setupMemgraph(t)` from `memgraph_test.go` (testcontainers)

## Gotchas

- `go test -short` does NOT skip integration tests — `setupMemgraph` doesn't check `testing.Short()`
- `filepath.Glob` does not support recursive `**` — only matches one directory level
- Proto `gen/` files are auto-generated — never edit manually (blocked by hook)
- Map iteration order is nondeterministic in Go — don't depend on key ordering in tests

## Architecture

### Core Concepts

- **Specs are graph nodes**, not files. Dependencies, blocks, and compositions are first-class edges.
- **Progressive structure**: 3 required fields for solo devs, 30+ for enterprise. Same schema at all scales.
- **Constitution**: Layered project ground truth (User → Org → Project → Domain). More specific layers override general ones.
- **Authoring funnel**: Spark → Shape → Specify → Decompose → Approve. Produces specs clear enough for agents to execute without ambiguity.
- **Execution bundles**: The contract between authoring and execution engines.
- **Decisions are first-class nodes** (ADR-003) with bidirectional edges to specs.

### Storage

Currently Memgraph-only (neo4j-go-driver/v5). Postgres + AGE planned as alternative backend (ADR-001).
SpecGraph sits upstream of Gastown (execution engine). See `docs/initial-design-session/` for full design.

## Implementation Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage backends, claim protocol, execution bundles, core CLI, linter (~17-18 weeks)
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync (~14 weeks, parallel workstreams)
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, document export, issue tracker sync, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance

### Implementation Progress

Work follows vertical slices. See `docs/plans/2026-02-28-implementation-tracker.md` for current status.
