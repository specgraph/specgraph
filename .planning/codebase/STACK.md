# Technology Stack

**Analysis Date:** 2026-07-08

## Languages

**Primary:**
- Go 1.26.4 - Server, CLI, storage, MCP server (`go.mod`)
- TypeScript 6.x - Web UI (`web/`)
- Protocol Buffers 3 - Service/schema source of truth (`proto/specgraph/v1/`)

**Secondary:**
- Svelte 5 (`.svelte` components) - Web UI framework, `web/src/`
- SQL (goose migrations) - `internal/storage/postgres/migrations/`
- Shell (bash) - dev tooling, Claude/Cursor/OpenCode plugin hooks (`plugin/*/hooks/`)

## Runtime

**Environment:**
- Go 1.26.4 toolchain (compiled binary, no interpreter at runtime)
- Node.js (version not pinned via `.nvmrc`; managed via `web/package.json` + pnpm) for the web UI build

**Package Manager:**
- Go modules (`go.mod` / `go.sum`) - lockfile present
- pnpm (`web/pnpm-lock.yaml`) for the web UI - lockfile present

## Frameworks

**Core:**
- ConnectRPC (`connectrpc.com/connect` v1.19.1) - RPC framework for CLI↔server communication, handlers in `internal/server/`
- Cobra (`github.com/spf13/cobra` v1.10.2) + pflag - CLI command framework, `cmd/specgraph/`
- SvelteKit 2 (`web/`) - Web UI, built as a static adapter (`@sveltejs/adapter-static`)
- mcp-go (`github.com/mark3labs/mcp-go` v0.45.0) - MCP server implementation, `internal/mcp/`

**Testing:**
- Go standard `testing` package + `stretchr/testify` v1.11.1 - unit tests throughout `internal/`
- Ginkgo/Gomega (`github.com/onsi/ginkgo/v2` v2.28.1, `github.com/onsi/gomega` v1.39.1) - E2E tests, `e2e/`
- testcontainers-go v0.41.0 - Postgres integration tests via Docker, `internal/storage/postgres/`
- Vitest v3 - Web UI unit tests, `web/`

**Build/Dev:**
- Taskfile.dev (`Taskfile.yml`) - primary task runner (build, test, lint, fmt, proto codegen)
- Buf (`buf.gen.yaml`, `buf.yaml`) - protobuf lint/generate, local plugins pinned (protoc-gen-go v1.36.11, protoc-gen-connect-go v1.19.1)
- Vite 8 - web UI dev server/bundler
- golangci-lint, yamlfmt, dprint, rumdl, cog (conventional commits), addlicense - lefthook-managed pre-commit/pre-push hooks (`lefthook.yaml`)

## Key Dependencies

**Critical:**
- `github.com/jackc/pgx/v5` v5.9.2 - native Postgres driver (pgxpool, no `database/sql`)
- `github.com/pressly/goose/v3` v3.27.0 - SQL migration runner, `internal/storage/postgres/migrations/`
- `google.golang.org/protobuf` v1.36.11 - protobuf runtime
- `github.com/cedar-policy/cedar-go` v1.7.0 - policy/authorization engine (Cedar language)
- `github.com/coreos/go-oidc/v3` v3.17.0 + `golang.org/x/oauth2` v0.36.0 + `github.com/go-jose/go-jose/v4` v4.1.4 - OIDC authentication, `internal/auth/`
- `github.com/knadh/koanf/v2` v2.3.5 (+ yaml/env/file/confmap/structs providers) - layered configuration loading, `internal/config/`
- `github.com/hashicorp/go-getter` v1.8.6 - remote/local artifact fetching

**Infrastructure:**
- `github.com/exaring/otelpgx` v0.11.1 - OTel instrumentation for pgx
- `connectrpc.com/otelconnect` v0.9.0 - OTel instrumentation for ConnectRPC
- `go.opentelemetry.io/otel*` v1.43.0 family + `go.opentelemetry.io/contrib/exporters/autoexport` v0.68.0 - tracing/metrics/logs, `internal/telemetry/`
- `go.opentelemetry.io/otel/exporters/otlp/*` (grpc/http variants for trace/metric/log) - OTLP export
- `go.opentelemetry.io/otel/exporters/prometheus` v0.65.0 + `github.com/prometheus/client_golang` v1.23.2 - Prometheus metrics exposition
- `go.opentelemetry.io/contrib/bridges/otelslog` v0.18.0 + `github.com/samber/slog-multi` v1.8.0 - structured logging bridged into OTel
- `github.com/oklog/ulid/v2` v2.1.1 - sortable unique IDs
- `github.com/evanphx/json-patch/v5` v5.9.11 - JSON patch support
- `github.com/sergi/go-diff` v1.4.0 - diffing (drift detection)
- `github.com/spaolacci/murmur3` v1.1.0 - hashing (content hash)

## Configuration

**Environment:**
- Layered config loaded via koanf: defaults → YAML file (`~/.config/specgraph/config.yaml`) → `SPECGRAPH_*` environment variables → CLI flags (highest precedence). See `internal/config/global.go`.
- Env vars follow `SPECGRAPH_<SECTION>_<KEY>` naming (e.g. `SPECGRAPH_SERVER_POSTGRES_URL`, `SPECGRAPH_LOG_LEVEL`, `SPECGRAPH_LOG_FORMAT`, `SPECGRAPH_LOG_OUTPUT`, `SPECGRAPH_CLIENT_DEFAULT_SERVER`).
- Per-project config: `.specgraph.yaml` (harness allow-list, drift-nudge quiet flag).
- OIDC client secrets are referenced indirectly via env var name (`client_secret_env` field in provider config, e.g. `SPECGRAPH_OIDC_ENTRA_SECRET`) — never inlined in YAML.
- `.env`-style files were not found in the repo; secrets flow through the `SPECGRAPH_*` env-var indirection above.

**Build:**
- `Taskfile.yml` - all automation entry point (`task build`, `task check`, `task pr-prep`, `task proto`, `task lint`, `task fmt`)
- `buf.yaml` / `buf.gen.yaml` - protobuf generation config
- `dprint.json` - multi-language formatter config
- `cog.toml` - conventional commits config
- `lefthook.yaml` - git hooks (pre-commit, pre-push, commit-msg)
- `codecov.yml` - coverage reporting config

## Platform Requirements

**Development:**
- Go 1.26.4 toolchain
- Docker (for Postgres testcontainers in integration tests and `pgvector/pgvector:pg18` dev database)
- `task tools` installs golangci-lint, buf, lefthook, and other dev tools
- pnpm for web UI (`web/`)

**Production:**
- Distributed as a single static Go binary (`specgraph`), containerized via `Dockerfile` (Alpine 3.23 base)
- Exposes port 9090; runs as non-root `specgraph` user
- Requires an external PostgreSQL instance (pgvector extension, tested against `pgvector/pgvector:pg18`) reachable via `server.postgres.url` / `SPECGRAPH_SERVER_POSTGRES_URL`
- `internal/docker/compose.go` can template a local dev Postgres via Docker Compose (DB only — the SpecGraph process itself runs natively)

---

*Stack analysis: 2026-07-08*
