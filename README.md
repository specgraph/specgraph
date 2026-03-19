# SpecGraph

A live spec-driven development framework — specifications as a queryable graph, not static markdown.

SpecGraph provides a **spec schema**, an **authoring funnel**, a **project constitution** (layered ground truth), and a **storage + query layer** that feeds agentic execution systems. It turns design intent into structured, claimable work units that agents and humans can pick up, execute, and verify.

## Why

Existing spec frameworks treat specifications as static documents. This creates gaps:

- **No live query layer** — you can't ask "what specs are blocked?" without parsing files
- **No addressability** — specs reference each other by filename, not stable identity
- **No execution interface** — agents need a task graph, not a folder of markdown
- **No ground truth** — every authoring session starts from scratch

SpecGraph closes these gaps by modeling specs as **nodes in a graph** with first-class edges for dependencies, blocks, and compositions.

## Architecture

```text
┌─────────────────────────────────────────────────┐
│                  Authoring Funnel                │
│  Spark → Shape → Specify → Decompose → Approve  │
└──────────────────────┬──────────────────────────┘
                       │
              ┌────────▼────────┐
              │   Spec Schema   │
              │  (protobuf v1)  │
              └────────┬────────┘
                       │
          ┌────────────▼────────────┐
          │      Constitution       │
          │  User → Org → Project   │
          │       → Domain          │
          └────────────┬────────────┘
                       │
              ┌────────▼────────┐
              │  ConnectRPC API │
              └────────┬────────┘
                       │
              ┌────────▼────────┐
              │  Graph Storage  │
              │   (Memgraph)    │
              └─────────────────┘
```

### Key concepts

- **Specs are graph nodes** with edges for dependencies, blocks, and compositions
- **Constitution** provides layered ground truth — more specific layers override general ones
- **Decisions are first-class nodes** with bidirectional edges to specs (see [ADR-003](docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md))
- **Authoring funnel** guides specs from rough idea to agent-executable work unit

## Getting Started

### Quick start

Initialize a SpecGraph project in your repo:

```bash
specgraph init              # interactive setup (storage backend, deployment mode)
specgraph init --scan       # also scan codebase and draft a constitution
```

The `--scan` flag walks your codebase to detect primary language, frameworks (API, CLI, UI, testing), infrastructure (Docker, Kubernetes), and CI provider, then writes a `constitution.yaml` draft you can refine.

Start the server and begin working with specs:

```bash
specgraph serve             # starts the API and manages Memgraph via Docker Compose
specgraph spec create auth-login --intent "User login flow"
specgraph constitution show # view project ground truth
```

In docker mode (the default), `serve` automatically starts and stops the Memgraph container alongside the ConnectRPC API — no external setup needed.

### Beyond quick start

| Setup | Mode | Description |
|-------|------|-------------|
| **Solo / local** | `docker` (default) | `specgraph serve` manages everything |
| **Team / shared server** | `remote` | Point CLI at a shared SpecGraph instance via config |
| **Production / BYO infra** | `external` | Connect to your own Memgraph or Postgres |

See the [deployment guide](https://seanb4t.github.io/specgraph/deployment/) for team and production configurations.

### CLI overview

| Command | Description |
|---------|-------------|
| **Specs** | |
| `specgraph spec create/list/show/update` | Manage specs |
| `specgraph decision create/list/show` | Manage decisions |
| **Authoring** | |
| `specgraph spark/shape/specify/decompose/approve` | Drive specs through the authoring funnel |
| **Lifecycle** | |
| `specgraph amend/supersede/abandon` | Transition specs through lifecycle states |
| `specgraph drift` | Check for spec drift; `drift ack` to acknowledge |
| `specgraph lint` | Run the spec linter |
| **Graph** | |
| `specgraph edge add/remove/list` | Manage graph edges between specs |
| `specgraph deps/impact/ready/critical-path` | Query the dependency graph |
| **Execution** | |
| `specgraph claim/unclaim` | Claim or release specs for work |
| `specgraph bundle/prime` | Generate execution bundles and prime context |
| `specgraph report-progress/report-blocker/report-completion` | Report execution status |
| **Constitution** | |
| `specgraph constitution show/import/check/emit` | View, import, validate, and emit constitution |
| **Sync & Injection** | |
| `specgraph sync` | Sync specs to Beads or GitHub |
| `specgraph inject` | Inject spec context into tool files (CLAUDE.md, .cursor/rules, AGENTS.md) |
| **Infrastructure** | |
| `specgraph init` | Initialize project config and optionally scan for constitution |
| `specgraph serve` | Start the ConnectRPC API server |
| `specgraph health` | Check server health |
| `specgraph up/down` | Start or stop the database container |

## Contributing

### Prerequisites

- Go 1.25+
- [Buf](https://buf.build/) (protobuf code generation)
- [Task](https://taskfile.dev/) (task runner)
- Docker (for Memgraph integration tests)

### Install development tools

```bash
task tools
```

### Build

```bash
task build
```

This runs protobuf code generation (`buf generate`) then builds the binary.

### Run tests

```bash
task test          # unit tests (excludes integration and e2e)
task test:short    # skip integration tests
task test:e2e      # end-to-end smoke tests
```

### Run the server locally

```bash
task dev                    # development mode with hot reload
```

## Development

### Code generation

Protobuf sources live in `proto/`. Generated code goes to `gen/` (committed for Go module compatibility). Regenerate after changing `.proto` files:

```bash
task proto
```

### Formatting and linting

```bash
task fmt           # format all files (Go, YAML, Markdown)
task lint          # run all linters
```

### Git hooks

The project uses [Lefthook](https://github.com/evilmartians/lefthook) for pre-commit hooks (license headers, golangci-lint, yamlfmt, dprint, conventional commits):

```bash
task hooks:install
```

## Roadmap

| Phase | Focus | Status |
|-------|-------|--------|
| **1 — Foundation** | Spec schema, constitution, storage, claim protocol, execution bundles, CLI, linter | Complete |
| **2 — Authoring & CLI** | Codebase scanner, authoring flow, Claude Code plugin, constitution sync | In progress |
| **3 — Coordination** | Multi-agent leasing, MCP server, drift detection, exports, Gastown integration | Planned |
| **4 — Scale** | Federation, multi-repo, metrics, governance | Planned |

## License

[MIT](LICENSE) — Copyright 2026 Sean Brandt
