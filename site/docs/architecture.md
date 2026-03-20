# Architecture

SpecGraph uses a client/server architecture. A single server process owns all
domain logic and storage. Clients connect via ConnectRPC — JSON over HTTP,
gRPC-compatible, protobuf-typed. The server is the single source of truth;
there is no embedded or library mode.

---

## System Diagram

```text
┌─────────────────────────────────────────────────────┐
│  CLIENTS                                             │
│  ├─ specgraph CLI                                   │
│  ├─ Claude Code skills                              │
│  └─ MCP server proxy (planned)                      │
└────────────┬────────────────────────────────────────┘
             │ ConnectRPC (JSON/HTTP)
             ▼
┌─────────────────────────────────────────────────────┐
│  SPECGRAPH SERVER                                    │
│  ├─ Core domain (Spec, Constitution, Authoring)     │
│  ├─ Graph analysis (deps, impact, critical path)    │
│  └─ Storage backend (Memgraph; Postgres planned)    │
└────────────┬────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────┐
│  SYNC ADAPTERS (outbound)                            │
│  ├─ Beads (spec→bead issue)                         │
│  ├─ GitHub Issues                                   │
│  ├─ Linear (planned)                                │
│  └─ Tool Injection (CLAUDE.md, .cursor/rules)        │
└─────────────────────────────────────────────────────┘
```

---

## API Surface

SpecGraph exposes its functionality through ConnectRPC services, each focused on
a single domain concern:

| Service | Description |
|---------|-------------|
| **SpecService** | Create, get, list, and update specs — the primary resource in the graph. |
| **DecisionService** | Create and manage decisions — first-class graph nodes with bidirectional edges to specs. |
| **ConstitutionService** | Layer merging, validation, and queries across the User → Org → Project → Domain hierarchy. |
| **AuthoringService** | Authoring funnel RPCs: Spark, Shape, Specify, Decompose, Approve, Amend, Supersede. |
| **ClaimService** | Claim and release specs for execution. Time-limited leases prevent duplicate work. |
| **GraphService** | Dependency queries, impact analysis, critical-path computation, and ready-spec detection. |
| **LifecycleService** | Lifecycle transitions (amend, supersede, abandon), drift detection, and spec linting. |
| **ExecutionService** | Execution bundles, prime context, and progress/blocker/completion reporting. |
| **SyncService** | Push specs to external systems (Beads, GitHub) and inject context into tool files. |
| **ServerService** | Health checks. |

All services use protobuf message types on the wire and generate both `.pb.go`
and `.connect.go` files from the proto definitions.

---

## Storage

SpecGraph uses a pluggable storage backend behind a `Backend` interface — the
core domain never talks to the database directly.

**Memgraph** (default, shipped) — The only backend in v0.1.0. Native Cypher
queries running in Docker. No extensions required.

**Postgres + AGE** (planned) — Designed but not yet implemented. Cypher via
the Apache AGE extension on standard Postgres.

```go
type Backend interface {
    CreateSpec(ctx context.Context, slug, intent string, ...) (*Spec, error)
    GetSpec(ctx context.Context, slug string) (*Spec, error)
    ListSpecs(ctx context.Context, filters Filters) ([]*Spec, error)
    // ... graph operations, claims, events
}
```

Storage interfaces use domain types, not protobuf types. The ConnectRPC handlers
in `internal/server/` translate between protobuf and domain types before calling
the backend.

---

## Graph Data Model

Specs, constitutions, decisions, and agents are nodes. Relationships between them
are typed edges:

```text
(:Spec) -[:DEPENDS_ON]->  (:Spec)
(:Spec) -[:BLOCKS]->      (:Spec)
(:Spec) -[:COMPOSES]->    (:Spec)
(:Spec) -[:RELATES_TO]->  (:Spec)
(:Spec) -[:DECIDED_IN]->  (:Decision)
(:Decision) -[:INFORMS]-> (:Spec)
(:Spec) -[:SUPERSEDES]->  (:Spec)
```

These edges are first-class — they carry metadata, support traversal queries,
and power the graph analysis operations (impact analysis, critical path, ready
detection).

---

## Why ConnectRPC?

ConnectRPC is browser-compatible (JSON over HTTP) while maintaining gRPC wire
compatibility and protobuf type safety. Plain gRPC cannot be called from
browsers directly. ConnectRPC provides both: structured APIs for tools, human-readable JSON
for debugging.

---

## Code Organization

```text
specgraph/
├── proto/specgraph/v1/     # Protobuf service definitions (source of truth)
├── gen/                    # Generated Go code (committed for module compat)
├── internal/
│   ├── auth/               # Auth interceptor + config-based token store
│   ├── authoring/          # Authoring funnel (stages, postures, passes)
│   ├── config/             # YAML-based server configuration
│   ├── docker/             # Docker Compose templates for DB containers
│   ├── drift/              # Drift detection engine
│   ├── driftscope/         # Drift scope analysis
│   ├── emitter/            # Event/output emitters
│   ├── inject/             # Tool injection (CLAUDE.md, .cursor/rules, AGENTS.md)
│   ├── linter/             # Spec linter (schema, edges, cycles)
│   ├── server/             # ConnectRPC handlers + proto↔domain converters
│   ├── service/            # systemd/launchd integration
│   ├── storage/            # Backend interface + implementations
│   │   └── memgraph/       # Memgraph implementation (Cypher, testcontainers)
│   ├── sync/               # Sync adapters (Beads, GitHub)
│   └── xdg/                # XDG base directory paths
├── cmd/specgraph/          # CLI entry point
├── e2e/                    # End-to-end tests (Ginkgo/Gomega)
├── plugin/                 # Claude Code skills and hooks
└── Taskfile.yml            # Build automation
```

Build automation is via [Taskfile.dev](https://taskfile.dev). Run `task --list`
for the full catalog. The key commands are `task build`, `task test`,
`task proto`, `task lint`, and `task fmt`. Generated code in `gen/` is
committed — regenerate with `task proto` after changing `.proto` files.
