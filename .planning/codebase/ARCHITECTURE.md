<!-- refreshed: 2026-07-08 -->
# Architecture

**Analysis Date:** 2026-07-08

## System Overview

```text
┌──────────────────────────────────────────────────────────────────────┐
│                         Clients                                      │
├───────────────┬────────────────────┬─────────────────────────────────┤
│  CLI (cobra)  │  MCP clients        │   Web UI (SvelteKit)            │
│ `cmd/specgraph`│ (Claude/Cursor/    │   `web/src/`                    │
│               │  OpenCode)          │                                 │
└───────┬───────┴──────────┬──────────┴──────────┬──────────────────────┘
        │ ConnectRPC        │ MCP (JSON-RPC        │ ConnectRPC/HTTP
        │ (proto clients)   │ over HTTP, tools+     │ (fetch from svelte
        │                   │ resources+prompts)    │  routes)
        ▼                   ▼                       ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    HTTP mux (`cmd/specgraph/serve.go`)                │
│  ConnectRPC service handlers   `internal/server/*_handler.go`         │
│  Auth interceptor (Cedar)      `internal/auth/interceptor.go`         │
│  MCP HTTP handler              `internal/mcp/server.go`               │
│  Static web UI                 `internal/server/static.go`            │
└───────┬───────────────────┬───────────────┬───────────────┬───────────┘
        │                   │               │               │
        ▼                   ▼               ▼               ▼
┌───────────────┐  ┌─────────────────┐ ┌───────────────┐ ┌──────────────┐
│  authoring     │  │  drift /         │ │  linter       │ │  auth /      │
│ funnel engine  │  │  driftscope      │ │ (spec schema  │ │ identity     │
│`internal/      │  │`internal/drift`, │ │  validation)  │ │`internal/    │
│ authoring/`    │  │`internal/        │ │`internal/     │ │ auth/`       │
│                │  │ driftscope/`     │ │ linter/`      │ │              │
└───────┬────────┘  └────────┬─────────┘ └───────┬───────┘ └──────┬───────┘
        │                    │                    │                │
        └────────────────────┴────────┬───────────┴────────────────┘
                                       ▼
                     ┌─────────────────────────────────────┐
                     │  Storage interface (domain types)     │
                     │  `internal/storage/storage.go`        │
                     │  Backend, SpecReader, Scoper,          │
                     │  TransactionalBackend interfaces       │
                     └───────────────────┬────────────────────┘
                                          ▼
                     ┌─────────────────────────────────────┐
                     │  PostgreSQL implementation             │
                     │  `internal/storage/postgres/`          │
                     │  pgx v5, recursive CTEs, goose migrations,
                     │  testcontainers (pgvector/pg18)        │
                     └─────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| CLI root/commands | Cobra command tree, config loading, telemetry lifecycle | `cmd/specgraph/main.go`, `cmd/specgraph/serve.go`, `cmd/specgraph/up.go` |
| ConnectRPC handlers | Translate proto requests to storage/domain calls, sanitize errors | `internal/server/*_handler.go` |
| Proto↔domain converters | Map generated proto types to internal domain structs and back | `internal/server/convert*.go` |
| Auth engine | Cedar-policy authorization, OIDC login, session/identity store | `internal/auth/` |
| Authoring funnel | Spark→Shape→Specify→Decompose→Approve stage composition and validation | `internal/authoring/` |
| MCP layer | Exposes tools/resources/prompts/skills over MCP protocol, backed by a loopback ConnectRPC client | `internal/mcp/` |
| Drift detection | Compares `content_hash_at_link` on DEPENDS_ON edges against upstream ContentHash | `internal/drift/`, `internal/driftscope/` |
| Linter | Validates spec schema/constitution rules | `internal/linter/` |
| Storage interfaces | Domain-typed Backend/SpecReader/Scoper contracts, not protobuf-shaped | `internal/storage/storage.go`, `internal/storage/*.go` |
| Postgres backend | Concrete storage implementation: pgxpool, recursive CTE graph queries, goose migrations | `internal/storage/postgres/` |
| Config | Global (`~/.config`) and project (`.specgraph.yaml`) config loading | `internal/config/` |
| Managed files/pointers | Init-managed files (AGENTS.md, harness shims, MCP configs) with fencing | `internal/config/pointers/`, `internal/config/managedfiles/` |
| Render | Markdown renderers for CLI table/detail output, one file per entity | `internal/render/` |
| Telemetry | OpenTelemetry trace/metric/log provider lifecycle, context-enriching slog handler | `internal/telemetry/` |
| Sync | External sync adapters (beads, GitHub) via exec runner | `internal/sync/` |
| Docker | Compose templates to start the Postgres container (server itself runs natively) | `internal/docker/` |
| Web UI | SvelteKit SPA, routes per entity type, calls ConnectRPC/HTTP APIs | `web/src/` |
| Plugin shims | Per-harness thin shims (Claude Code, Cursor, OpenCode) that fetch skills via MCP | `plugin/specgraph/`, `plugin/cursor/`, `plugin/opencode/` |

## Pattern Overview

**Overall:** Layered service architecture — CLI/MCP/Web are three front doors into the same ConnectRPC service layer, which in turn depends on a storage interface with a single Postgres implementation. Proto-first design: `.proto` sources under `proto/specgraph/v1/` are the contract; generated code is committed under `gen/specgraph/v1/`.

**Key Characteristics:**
- Proto-defined RPC surface (ConnectRPC, not plain gRPC) with hand-written converters isolating proto types from storage domain types.
- MCP server is a thin adapter: it holds a loopback HTTP ConnectRPC client (`mcpClient`) and re-exposes the same RPC surface as MCP tools/resources/prompts, rather than talking to storage directly.
- Domain-first storage layer: `internal/storage/storage.go` defines interfaces in plain Go types (`Spec`, `NodeRef`, etc.), decoupled from protobuf so storage can be swapped without touching proto.
- Specs form a graph with typed edges (DEPENDS_ON, BLOCKS, COMPOSES, DECIDED_IN, plus internal-only HAS_CHANGE/HAS_FINDING edges), queried via recursive CTEs in Postgres.
- Authoring funnel (Spark→Shape→Specify→Decompose→Approve) is a distinct pipeline (`internal/authoring/`) composing prompts and validating stage transitions before they touch storage.
- Cedar policy engine drives authorization (`internal/auth/cedar_authorizer.go`), evaluated via an interceptor wrapping every ConnectRPC handler.
- Single-process CLI is sequential and mostly single-threaded (see `main.go` comment on `telState`); server mode (`specgraph serve`/`specgraph up`) runs concurrently via net/http.

## Layers

**CLI Layer:**
- Purpose: user-facing commands (spec CRUD, authoring stages, decisions, drift, sync, auth/login, doctor, serve/up)
- Location: `cmd/specgraph/`
- Contains: cobra command definitions, one file per command/command-group (e.g. `decompose.go`, `constitution.go`, `auth.go`, `serve.go`, `up.go`, `prime.go`, `sync.go`)
- Depends on: `internal/config`, `internal/telemetry`, `internal/server` (in-process for `serve`), generated ConnectRPC clients (`gen/specgraph/v1/specgraphv1connect`) for talking to a running server
- Used by: end users, CI, drift-nudge hook

**RPC/Handler Layer:**
- Purpose: implements each proto service (`SpecService`, `LifecycleService`, `AuthoringService`, `GraphService`, `DecisionService`, `ClaimService`, `ConstitutionService`, `AnalyticalPassService`, `ExecutionService`, `SliceService`, `IdentityService`, `ExportService`, `SyncService`, `ServerService`)
- Location: `internal/server/`
- Contains: one `<entity>_handler.go` per service, `convert*.go` proto↔domain mappers, cross-cutting middleware (`cors.go`, `security_headers.go`, `ratelimit.go`, `access_log_interceptor.go`, `request_limits_test.go`)
- Depends on: `internal/storage` (via `Scoper`/`Backend` interfaces), `internal/auth`, `internal/drift`, `internal/linter`
- Used by: `cmd/specgraph/serve.go` (registers all handlers on one mux), MCP server (loopback client)

**MCP Layer:**
- Purpose: exposes the same functionality to AI coding agents (Claude Code, Cursor, OpenCode) via Model Context Protocol
- Location: `internal/mcp/`
- Contains: `server.go` (registry composition), `tools_*.go` (one file per tool group: core, authoring, execution, graph, lifecycle, skills, spec), `resources.go`, `prompts.go`, `client.go` (loopback HTTP client to the ConnectRPC mux), `skills/` (embedded SKILL.md packages)
- Depends on: loopback ConnectRPC client into the same process's HTTP mux (not storage directly)
- Used by: `cmd/specgraph/serve.go` (mounted at `mcpHTTPHandler`), harness plugin shims

**Authoring Layer:**
- Purpose: drives the Spark→Shape→Specify→Decompose→Approve funnel — composes prompts, validates stage transitions, runs analytical passes
- Location: `internal/authoring/`
- Contains: `composer.go` (prompt composition), `stages.go` (stage enum/ordering), `passes.go` (analytical pass execution), `posture.go`, `safety.go`, `validate.go`, `content/` (embedded markdown instructions)
- Depends on: `internal/storage` (via `ComposerBackend` interface), embedded content via `//go:embed`
- Used by: `internal/server/authoring_handler.go`, `internal/mcp/tools_authoring.go`

**Storage Layer:**
- Purpose: persistence contracts and Postgres implementation
- Location: `internal/storage/` (interfaces + domain types), `internal/storage/postgres/` (implementation)
- Contains: `storage.go` (Backend/SpecReader/TransactionalBackend interfaces), per-entity domain files (`spec_domain.go`, `decision.go`, `execution_domain.go`, `claim_domain.go`, etc.), `contenthash/` (content hashing for drift), `postgres/tx.go` (transaction helper)
- Depends on: pgx v5, goose migrations (`internal/storage/postgres/migrations/`)
- Used by: every handler in `internal/server/`, `internal/authoring/`, `internal/drift/`, `internal/linter/`

**Auth Layer:**
- Purpose: authentication (OIDC login, sessions, bearer tokens) and authorization (Cedar policy evaluation)
- Location: `internal/auth/`
- Contains: `interceptor.go` (ConnectRPC interceptor), `cedar_authorizer.go`/`engine.go` (policy evaluation), `identitystore.go` (users/sessions), `oidc_verifier.go`, `loginsync.go`, `policies/` (Cedar policy files), `known_roles.go`
- Depends on: `internal/storage` (identity/session persistence), external OIDC providers
- Used by: `internal/server/*_handler.go` (via interceptor), CLI `login`/`logout` commands

**Web UI:**
- Purpose: browser-based graph/spec/constitution/decision viewer and interactive OIDC login
- Location: `web/src/` (SvelteKit + Svelte 5, Vite, Vitest)
- Contains: `routes/graph`, `routes/spec`, `routes/constitution`, `routes/decision`, `lib/components`, `lib/api`
- Depends on: ConnectRPC/HTTP APIs served by the same Go binary
- Used by: browser clients; served as static assets via `internal/server/static.go`

## Data Flow

### Primary Request Path (CLI → server)

1. User runs a `specgraph` subcommand; cobra parses flags, `nudgePreRun` runs drift-nudge + telemetry init (`cmd/specgraph/main.go:64`)
2. Command builds a ConnectRPC client against `cfg.Client.DefaultServer` and calls the relevant RPC (e.g. `cmd/specgraph/decompose.go`)
3. `internal/server/<entity>_handler.go` receives the request, runs through auth interceptor (`internal/auth/interceptor.go`) for Cedar authorization
4. Handler validates input (`internal/server/validation.go`), converts proto→domain (`convert_*.go`), calls `internal/storage` Backend method
5. Postgres implementation (`internal/storage/postgres/*.go`) executes queries, wraps multi-step writes in `RunInTransaction` (`internal/storage/postgres/tx.go`)
6. Handler converts domain→proto and returns; CLI renders output via `internal/render/` or `printJSON` (`cmd/specgraph/output.go`)

### Authoring Funnel Flow

1. Agent (via MCP tool or CLI) requests a stage prompt: `internal/mcp/tools_authoring.go` or `internal/server/authoring_handler.go`
2. `internal/authoring/composer.go` composes the prompt: fetches spec summary via `ComposerBackend.GetSpecSummary`, embeds stage-specific content from `internal/authoring/content/*.md`
3. Agent's stage output (SparkOutput/ShapeOutput/SpecifyOutput/DecomposeOutput) is submitted back through `StoreXOutput`-style RPCs
4. `internal/authoring/stages.go` validates the stage transition is legal
5. Storage persists the stage output and, on `done` transition, refreshes `content_hash_at_link` on outgoing DEPENDS_ON edges (drift baseline)

### MCP Tool Invocation Flow

1. AI agent (Claude Code/Cursor/OpenCode) calls an MCP tool over HTTP (`mcpHTTPHandler`, `cmd/specgraph/serve.go:230`)
2. `internal/mcp/server.go` dispatches to the matching `tools_*.go` handler
3. Tool handler uses `mcpClient` (a loopback ConnectRPC client back into the same process's mux) to invoke the real RPC — the MCP layer never talks to storage directly
4. Response is converted to MCP tool-result shape (`internal/mcp/convert.go`) and returned

**State Management:**
- No in-memory application state beyond process-local caches (e.g. drift-nudge throttle file, telemetry providers). All durable state lives in Postgres.
- CLI is effectively stateless between invocations except for local config/cache files under XDG dirs (`internal/xdg/`).

## Key Abstractions

**Backend / SpecReader / Scoper / TransactionalBackend:**
- Purpose: storage capability interfaces, composed rather than one monolithic interface
- Examples: `internal/storage/storage.go`, `internal/storage/scoper.go`
- Pattern: handlers depend on the narrowest interface they need (e.g. drift/linter only need `SpecReader`)

**Spec (graph node):**
- Purpose: first-class node type with typed edges (DEPENDS_ON, BLOCKS, COMPOSES, DECIDED_IN) plus internal-only edges (HAS_CHANGE, HAS_FINDING)
- Examples: `internal/storage/spec_domain.go`, `internal/storage/graph.go`
- Pattern: edges carry properties (e.g. `content_hash_at_link`) consumed by drift detection

**Analytical Pass / Finding:**
- Purpose: pass-specific analysis results attached to specs as findings, stored independently of stage output
- Examples: `internal/storage/findings.go`, `internal/server/analytical_pass_handler.go`, `internal/authoring/passes.go`
- Pattern: template-driven prompts with override directory support (`.specgraph/templates/`)

**Managed Files / Pointers:**
- Purpose: files written into end-user projects by `specgraph init` that stay in sync with canonical embedded content (AGENTS.md, harness shims, MCP configs)
- Examples: `internal/config/pointers/`, `internal/config/managedfiles/`
- Pattern: managed-block fencing + file locking so user edits outside the fence survive re-sync; drift status (Synced/Stale/Missing/Drifted) reported by `specgraph doctor`

## Entry Points

**`cmd/specgraph/main.go`:**
- Location: `cmd/specgraph/main.go`
- Triggers: process start (`specgraph <subcommand>`)
- Responsibilities: cobra root command setup, telemetry lifecycle, config file flag wiring, drift-nudge PersistentPreRunE

**`cmd/specgraph/serve.go`:**
- Location: `cmd/specgraph/serve.go`
- Triggers: `specgraph serve` (foreground) or via `specgraph up`/service manager
- Responsibilities: builds the storage backend, registers every ConnectRPC service on one mux (see `NewMux` in `internal/server/server.go` plus `Register*Service` calls at `cmd/specgraph/serve.go:193-209`), mounts the MCP HTTP handler, mounts static web UI, starts `net/http` listener

**`cmd/specgraph/up.go`:**
- Location: `cmd/specgraph/up.go`
- Triggers: `specgraph up`
- Responsibilities: ensures Docker compose stack for Postgres if configured, starts/verifies the service (launchd/systemd) or prints manual-mode instructions, health-checks until the server responds

**MCP HTTP handler:**
- Location: mounted in `cmd/specgraph/serve.go`, implemented in `internal/mcp/server.go`
- Triggers: MCP client (Claude Code, Cursor, OpenCode) tool/resource/prompt calls
- Responsibilities: routes MCP protocol calls to loopback ConnectRPC calls against the same server process

## Architectural Constraints

- **Threading:** CLI execution path (cobra `PersistentPreRunE` → `ExecuteContext` → deferred telemetry shutdown) is strictly sequential on one goroutine (documented at `cmd/specgraph/main.go:47-55`); server mode (`serve`) is a standard concurrent `net/http` server.
- **Global state:** `telState` in `cmd/specgraph/main.go` is a package-level var holding telemetry handle/root span, safe only because of the single-threaded CLI guarantee above. `rootCmd`, `upCmd`, and other cobra command vars are package-level singletons per Go's cobra convention.
- **Circular imports:** None observed; `internal/mcp` avoids a cycle with `internal/server` by using an HTTP loopback client rather than importing server handler code directly.
- **Proto-generated code is committed:** `gen/` must stay in sync with `proto/` (`task proto:check`); editing `gen/` directly is blocked by a Claude Code PreToolUse hook — edit `.proto` sources instead.

## Anti-Patterns

### Editing generated code directly

**What happens:** Occasionally tempting to patch `gen/specgraph/v1/*.pb.go` or `*.connect.go` directly to fix a small issue.
**Why it's wrong:** `task proto` regeneration will silently overwrite the fix, and a PreToolUse hook blocks such edits in Claude Code sessions.
**Do this instead:** Edit the `.proto` source under `proto/specgraph/v1/`, run `task proto`, then update callers.

### Mock backends returning generic errors

**What happens:** Test doubles for `storage.Backend` sometimes return `fmt.Errorf(...)` for not-found conditions.
**Why it's wrong:** Handler code uses `errors.Is()` against sentinel errors (`storage.ErrSpecNotFound`, `storage.ErrDecisionNotFound`); a generic error breaks the `errors.Is` check and error-code mapping to clients.
**Do this instead:** Mock/fake backends must return the exact sentinel error values storage defines.

### Multi-query writes outside a transaction

**What happens:** A handler issues several sequential storage calls to mutate related state (e.g. spec + changelog) without wrapping them.
**Why it's wrong:** Per ADR-004, partial failures leave the graph in an inconsistent state; concurrent writers can interleave.
**Do this instead:** Use `TransactionalBackend.RunInTransaction`, threading `txCtx` (not the outer `ctx`) into `executeQuery`/`GetSpec`/`createChangeLog` — see `internal/storage/postgres/tx.go`.

## Error Handling

**Strategy:** Sentinel errors at the storage layer (`storage.ErrSpecNotFound`, etc.), sanitized at the RPC boundary before reaching clients.

**Patterns:**
- Handlers call sanitization helpers like `stageError` before returning errors to ConnectRPC clients, mapping internal errors to `connect.CodeInternal`/`connect.CodeNotFound`/`connect.CodeAborted` etc. (`internal/server/error_sanitize_test.go`, `internal/server/error_mapper_internal_test.go`)
- Concurrent write conflicts surface as `storage.ErrConcurrentModification` → `connect.CodeAborted` (retryable); version guards in WHERE clauses detect the conflict, first writer wins.
- Test assertions on handler errors check `connect.Code*` values, never raw error message strings (sanitization makes string matching unreliable/unsafe).

## Cross-Cutting Concerns

**Logging:** `log/slog` throughout, with a context-enriching handler wired by `internal/telemetry/` so log records carry trace/span IDs.
**Validation:** Centralized in `internal/server/validation.go` for RPC input; `internal/linter/` validates spec schema/constitution content; `internal/authoring/validate.go` validates stage transitions.
**Authentication/Authorization:** OIDC-based login + Cedar policy authorization via `internal/auth/interceptor.go`, applied uniformly across ConnectRPC handlers; MCP requests carry a Bearer token or project header translated into the same auth context (`cmd/specgraph/serve.go`'s `WithHTTPContextFunc`).

---

*Architecture analysis: 2026-07-08*
</content>
