# Codebase Structure

**Analysis Date:** 2026-07-08

## Directory Layout

```
specgraph/
├── cmd/specgraph/          # CLI entry point — one file per command/command-group, cobra tree
├── proto/specgraph/v1/     # Protobuf service/message definitions (source of truth)
├── gen/specgraph/v1/       # Generated Go code from proto (committed; regen with `task proto`)
├── internal/
│   ├── auth/               # Authn (OIDC), authz (Cedar), sessions, identity store
│   │   └── policies/       # Cedar policy files
│   ├── authoring/          # Spark→Shape→Specify→Decompose→Approve funnel logic
│   │   ├── content/        # Embedded markdown instructions (//go:embed)
│   │   └── testdata/       # Golden test fixtures
│   ├── bootstrap/          # Project bootstrap logic
│   ├── browser/            # Browser-launch helper (OIDC login flow)
│   ├── config/             # Global (~/.config) + project (.specgraph.yaml) config
│   │   ├── managedfiles/   # Embedded canonical harness shim content
│   │   └── pointers/       # Init-managed pointer files (AGENTS.md, .cursor/rules/)
│   ├── constitution/       # Constitution (layered ground truth) domain logic
│   ├── credentials/        # Credential storage helpers
│   ├── diff/               # Version diffing
│   ├── docker/             # Docker compose templates for DB container
│   ├── drift/              # Drift detection engine
│   ├── driftscope/         # Drift scope analysis
│   ├── emitter/            # Event/output emitters
│   ├── export/             # Export format logic
│   ├── linter/             # Spec schema + constitution validation
│   ├── mcp/                # MCP server: tools, resources, prompts
│   │   └── skills/         # Embedded SKILL.md packages (agentskills.io shape)
│   ├── notify/             # Notification helpers
│   ├── prime/              # `specgraph prime` (context-priming) logic
│   ├── render/              # Markdown renderers for CLI output, one file per entity
│   ├── reqctx/             # Request-scoped context helpers
│   ├── server/             # ConnectRPC handlers + proto↔domain converters
│   │   ├── probes/         # Health/readiness probe helpers
│   │   └── templates/      # Analytical pass prompt templates (embedded defaults)
│   ├── service/             # OS service install helpers (launchd/systemd)
│   ├── skillvalidate/       # SKILL.md schema validation
│   ├── storage/             # Storage interfaces (domain types)
│   │   ├── contenthash/    # Content hashing for drift detection
│   │   └── postgres/       # PostgreSQL implementation (pgx v5)
│   │       └── migrations/ # goose SQL migrations
│   ├── sync/                # Sync adapters (beads, GitHub) via exec runner
│   ├── telemetry/           # OpenTelemetry lifecycle (traces/metrics/logs)
│   └── xdg/                 # XDG base directory helpers
├── plugin/                  # Per-harness shims (reverse-symlinks to embedded canonicals)
│   ├── specgraph/           # Claude Code plugin shim
│   ├── cursor/              # Cursor plugin shim
│   └── opencode/            # OpenCode plugin shim
├── skills/                  # Reverse-symlink to internal/mcp/skills/embedded/
├── web/src/                 # SvelteKit web UI
│   ├── routes/               # graph, spec, constitution, decision route pages
│   └── lib/                  # components/, api/
├── e2e/                      # End-to-end tests (Ginkgo/Gomega, require Docker)
│   ├── agent/, api/, cli/, docker/, ui/, testutil/
├── docs/                     # ADRs, design docs, implementation plans
│   ├── decisions/, designs/, plans/, verification/
├── site/docs/                 # Public docs site content
├── demo/                       # Demo assets
├── deploy/memgraph/            # Deployment configs
└── .beads/                     # Beads issue tracker DB
```

## Directory Purposes

**`cmd/specgraph/`:**
- Purpose: CLI entry point; every subcommand is a cobra command
- Contains: one `.go` file per command or command family (`decompose.go`, `specify.go`, `approve.go`, `bundle.go`, `changes.go`, `decision.go`, `edge.go`, `findings.go`, `pass.go`, `slice.go`, `sync.go`, `auth*.go`, `login*.go`, `logout.go`, `up.go`, `serve.go`, `init*.go`, `install*.go`, `prime.go`)
- Key files: `main.go` (root command + telemetry lifecycle), `serve.go` (server bootstrap, registers all RPC/MCP/static handlers), `up.go` (start server as daemon/service), `output.go` (shared `printJSON` helper), `table.go` (legacy `tableWriter`, still used by `sync.go`/`prime.go`)

**`proto/specgraph/v1/`:**
- Purpose: single source of truth for the RPC/message contract
- Contains: `lifecycle.proto`, `spec.proto`, `graph.proto`, `slice.proto`, `authoring.proto`, `constitution.proto`, `claim.proto`, `execution.proto`, `identity.proto`, `sync.proto`, `export.proto`, `analytical_pass.proto`, `server.proto`, `decision.proto`

**`gen/specgraph/v1/`:**
- Purpose: generated Go structs + ConnectRPC client/server code
- Generated: Yes (via `task proto`, buf)
- Committed: Yes (required for Go module compatibility) — never hand-edit

**`internal/server/`:**
- Purpose: implements every proto service as a ConnectRPC handler
- Contains: `<entity>_handler.go` (one per service), `convert*.go` (proto↔domain mapping), cross-cutting middleware (`cors.go`, `security_headers.go`, `ratelimit.go`, `access_log_interceptor.go`), `static.go` (serves web UI build)
- Key files: `server.go` (`NewMux`, base wiring)

**`internal/storage/`:**
- Purpose: storage interface contracts using domain types (not protobuf)
- Contains: `storage.go` (Backend/SpecReader/TransactionalBackend interfaces), per-entity domain files (`spec_domain.go`, `decision.go`, `execution_domain.go`, `claim_domain.go`, `constitution_domain.go`, `users_domain.go`, `web_auth_domain.go`), `errors.go` (sentinel errors)
- `postgres/` subdirectory: concrete implementation, `tx.go` (transaction helper), `migrations/` (goose SQL)

**`internal/authoring/`:**
- Purpose: authoring funnel stage logic (Spark/Shape/Specify/Decompose/Approve)
- Contains: `composer.go`, `stages.go`, `passes.go`, `posture.go`, `safety.go`, `validate.go`, `content/*.md` (embedded via `//go:embed`)

**`internal/auth/`:**
- Purpose: authentication + authorization
- Contains: `interceptor.go`, `cedar_authorizer.go`, `engine.go`, `identitystore.go`, `oidc_verifier.go`, `loginsync.go`, `mint.go` (token minting), `policies/` (Cedar `.cedar` policy files), `known_roles.go`

**`internal/mcp/`:**
- Purpose: MCP protocol server exposing tools/resources/prompts to AI agents
- Contains: `server.go`, `client.go` (loopback ConnectRPC client), `tools_*.go` (core, authoring, execution, graph, lifecycle, skills, spec), `resources.go`, `prompts.go`, `profiles.go`, `skills/` (embedded SKILL.md content)

**`internal/config/`:**
- Purpose: config loading (global + project) and managed-file sync
- Contains: `config.go`, `global.go`, `project.go`, `managedfiles/` (embedded canonical shim content), `pointers/` (writer logic with fencing + locking)

**`e2e/`:**
- Purpose: end-to-end tests requiring a running server and often Docker
- Contains: `agent/` (agent-driven flows), `api/` (Ginkgo/Gomega `-tags e2e`), `cli/`, `docker/` (Docker-in-Docker, skipped in CI), `ui/`, `testutil/`
- Generated: No; Committed: Yes

**`docs/plans/`:**
- Purpose: implementation plan documents (design + rationale before/after major features)
- Naming: `YYYY-MM-DD-<slug>-design.md` or similar dated slugs

**`web/src/`:**
- Purpose: SvelteKit (Svelte 5, Vite, Vitest) web UI
- Contains: `routes/{graph,spec,constitution,decision}/` page routes, `lib/components/`, `lib/api/`

**`plugin/<harness>/` and `skills/`:**
- Purpose: author convenience — reverse-symlinks pointing at the canonical embedded content under `internal/config/managedfiles/embedded/<harness>/` and `internal/mcp/skills/embedded/`
- Generated: No (symlinks); editing through them edits the canonical copy directly

## Key File Locations

**Entry Points:**
- `cmd/specgraph/main.go`: process entry, cobra root command, telemetry lifecycle
- `cmd/specgraph/serve.go`: full server bootstrap (all RPC handlers + MCP + static UI)
- `cmd/specgraph/up.go`: daemon/service startup wrapper

**Configuration:**
- `internal/config/global.go`: `~/.config/specgraph/config.yaml` (or `--config`)
- `internal/config/project.go`: `.specgraph.yaml` (harnesses allow-list, nudges.quiet)
- `.mcp.json`, `.cursor/mcp.json`, `opencode.json`: per-harness MCP server configs, managed by `specgraph init` (writer code in `internal/config/mcpconfigs/`)

**Core Logic:**
- `internal/storage/storage.go`: the storage contract every backend must satisfy
- `internal/storage/postgres/tx.go`: transactional write pattern (ADR-004)
- `internal/authoring/composer.go`: authoring prompt composition
- `internal/auth/interceptor.go`: ConnectRPC authorization interceptor
- `internal/drift/drift.go`, `internal/driftscope/scope.go`: drift detection engine

**Testing:**
- Unit tests: co-located `*_test.go` next to implementation in every `internal/` and `cmd/specgraph/` package
- Integration tests: `internal/storage/postgres/*_test.go` (testcontainers, `//go:build integration` in some suites)
- E2E tests: `e2e/api/` (`-tags e2e`, Ginkgo/Gomega), `e2e/docker/` (Docker-in-Docker)

## Naming Conventions

**Files:**
- `<entity>_handler.go` for ConnectRPC handlers (e.g. `spec_handler.go`, `decision_handler.go`)
- `convert_<entity>.go` for proto↔domain converters in `internal/server/`
- `<entity>_domain.go` for domain type definitions in `internal/storage/` (e.g. `spec_domain.go`, `execution_domain.go`)
- `tools_<group>.go` for MCP tool groups in `internal/mcp/`
- `*_test.go` co-located with the file it tests; `*_internal_test.go` for white-box tests of unexported symbols

**Directories:**
- Lowercase, single-word package names under `internal/` (Go convention — one directory per package)
- `embedded/` subdirectories mark `//go:embed`-sourced content (`internal/mcp/skills/embedded/`, `internal/config/managedfiles/embedded/`)

## Where to Add New Code

**New ConnectRPC service/entity:**
- Proto: add message/service to `proto/specgraph/v1/<entity>.proto`, run `task proto`
- Handler: `internal/server/<entity>_handler.go` (register on mux in `cmd/specgraph/serve.go`)
- Converters: `internal/server/convert_<entity>.go`
- Storage: domain types + interface methods in `internal/storage/<entity>_domain.go`, Postgres impl in `internal/storage/postgres/<entity>.go`
- CLI: new command file in `cmd/specgraph/<entity>.go`
- Tests: co-located `_test.go` files alongside each new file

**New MCP tool:**
- Add to the appropriate `internal/mcp/tools_<group>.go` (or create a new group file), register in `internal/mcp/server.go`/`registry.go`

**New authoring stage behavior:**
- `internal/authoring/stages.go` (stage enum/ordering), `internal/authoring/content/<stage>.md` (embedded instructions), update proto stage-output message and check `TestContentProtoDrift`

**New skill:**
- Add SKILL.md package under `internal/mcp/skills/embedded/<skill-name>/`, validate with `task skills:validate`

**Utilities:**
- Cross-cutting helpers: `internal/reqctx/` (request-scoped context), `internal/xdg/` (base dirs), `internal/emitter/` (output emission)

## Special Directories

**`gen/`:**
- Purpose: protobuf-generated Go code
- Generated: Yes
- Committed: Yes (Go module compatibility)

**`.beads/`:**
- Purpose: beads issue tracker database (`bd` CLI)
- Generated: Yes (managed by `bd`)
- Committed: Yes (dolt-backed, synced via `bd dolt push`)

**`site/docs/`:**
- Purpose: public documentation site content
- Generated: No
- Committed: Yes

**`docs/plans/`:**
- Purpose: dated implementation plan documents; must never embed corporate identifiers (public repo)
- Generated: No
- Committed: Yes

---

*Structure analysis: 2026-07-08*
</content>
