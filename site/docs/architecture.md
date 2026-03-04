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
│  ├─ MCP server proxy                                │
│  └─ Tauri+Svelte UI (future)                        │
└────────────┬────────────────────────────────────────┘
             │ ConnectRPC (JSON/HTTP)
             ▼
┌─────────────────────────────────────────────────────┐
│  SPECGRAPH SERVER                                    │
│  ├─ Core domain (Spec, Constitution, Authoring)     │
│  ├─ Graph analysis (deps, impact, critical path)    │
│  └─ Storage backend (Memgraph | Postgres+AGE)       │
└────────────┬────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────┐
│  SYNC ADAPTERS (outbound)                            │
│  ├─ Beads (spec→bead issue)                         │
│  ├─ GitHub Issues                                   │
│  ├─ Linear                                          │
│  └─ Tool Injection (CLAUDE.md, .cursorrules)        │
└─────────────────────────────────────────────────────┘
```

---

## API Surface

SpecGraph exposes its functionality through ConnectRPC services, each focused on
a single domain concern:

| Service | Description |
|---------|-------------|
| **SpecService** | CRUD for specs — create, get, list, update. The primary resource in the graph. |
| **DecisionService** | CRUD for decisions. ADRs are first-class graph nodes with bidirectional edges to the specs they affect. |
| **ConstitutionService** | Constitution management — layer merging, validation, and queries across the User → Org → Project → Domain hierarchy. |
| **AuthoringService** | The authoring funnel RPCs: Spark, Shape, Specify, Decompose, Approve. Drives specs from rough idea to execution-ready. |
| **ClaimService** | Claim and unclaim specs for execution. Manages leases so multiple agents don't collide on the same work. |
| **GraphService** | Dependency queries, impact analysis, critical-path computation, and ready-spec detection. |
| **HealthService** | Server health checks. |

All services use protobuf message types on the wire and generate both `.pb.go`
and `.connect.go` files from the proto definitions.

---

## Storage

SpecGraph supports two pluggable storage backends. Both implement the same
`Backend` interface — the core domain never talks to the database directly.

**Memgraph** (default) — Native Cypher queries running in Docker. Good for solo
developers and teams. Provides native graph operations without extensions.

**Postgres + AGE** (alternative) — Cypher via the Apache AGE extension on
standard Postgres. Good for teams with existing Postgres infrastructure. Falls
back to recursive CTEs if AGE is not available.

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
(:Spec) -[:GOVERNED_BY]-> (:Constitution)
(:Spec) -[:DECIDED_IN]->  (:Decision)
(:Spec) -[:CLAIMED_BY]->  (:Agent)
```

These edges are first-class — they carry metadata, support traversal queries,
and power the graph analysis operations (impact analysis, critical path, ready
detection).

---

## Why ConnectRPC?

ConnectRPC is browser-compatible (JSON over HTTP) while maintaining gRPC wire
compatibility and protobuf type safety. Plain gRPC cannot be called from
browsers directly. ConnectRPC gives both worlds — structured APIs for tools,
human-readable JSON for debugging.

---

## Code Organization

```text
specgraph/
├── proto/specgraph/v1/     # Protobuf service definitions
├── gen/                    # Generated code (gitignored)
├── internal/
│   ├── server/             # ConnectRPC handlers
│   ├── storage/            # Backend interface + implementations
│   │   └── memgraph/       # Memgraph implementation
│   ├── authoring/          # Domain logic (stages, postures, passes)
│   ├── scanner/            # Codebase context gathering
│   └── config/             # Server configuration
├── cmd/specgraph/          # CLI entry point
├── docker/                 # Docker Compose for dev
└── Taskfile.yml            # Build automation
```

Build automation is via [Taskfile.dev](https://taskfile.dev). Run `task --list`
for the full catalog. The key commands are `task build`, `task test`,
`task proto`, `task lint`, and `task fmt`. Generated code in `gen/` is
gitignored — run `task proto` after clone to regenerate it.
