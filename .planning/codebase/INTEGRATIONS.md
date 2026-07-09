# External Integrations

**Analysis Date:** 2026-07-08

## APIs & External Services

**Model Context Protocol (MCP):**
- SpecGraph runs its own MCP server (`internal/mcp/server.go`) using `github.com/mark3labs/mcp-go` v0.45.0, exposing authoring tools, skills (`specgraph_skills_list/get/search`), and resources (`specgraph://skills/<name>`, `specgraph://prime`) to AI coding harnesses.
- Per-harness MCP client configs are written/synced by `specgraph init`: `.mcp.json` (Claude Code), `.cursor/mcp.json` (Cursor), `opencode.json` (OpenCode). See `internal/config/mcpconfigs/`.

**AI Coding Harnesses (consumers, not APIs called by SpecGraph):**
- Claude Code — plugin shim `plugin/specgraph/` (`.claude-plugin/plugin.json`, hooks, `.specgraph/agents/claude/`)
- Cursor — plugin shim `plugin/cursor/` (`.cursor/rules/specgraph.mdc`, `specgraph-post-stage.mdc`)
- OpenCode — plugin shim `plugin/opencode/` (`.opencode/plugins/specgraph.ts`)

**GitHub:**
- `internal/sync/github.go` — `GitHubAdapter` syncs specs to GitHub Issues by shelling out to the `gh` CLI (via `CommandRunner` abstraction in `internal/sync/exec.go`), not a direct REST/GraphQL client.

**Beads (bd):**
- `internal/sync/beads.go` — `BeadsAdapter` syncs specs to the Beads task tracker by shelling out to the `bd` CLI, same `CommandRunner` abstraction.

**AWS SDK (indirect dependency):**
- `github.com/aws/aws-sdk-go-v2*` present as indirect dependencies (pulled in via OTel/autoexport or hashicorp/go-getter for artifact retrieval from S3-style sources); no direct AWS service client code found in `internal/`.

## Data Storage

**Databases:**
- PostgreSQL (with `pgvector` extension) — sole datastore, `internal/storage/postgres/`
  - Driver: `github.com/jackc/pgx/v5` (pgxpool, native driver, no `database/sql`)
  - Migrations: `github.com/pressly/goose/v3`, SQL files in `internal/storage/postgres/migrations/`
  - Connection: `server.postgres.url` config key / `SPECGRAPH_SERVER_POSTGRES_URL` env var / `--pg-url` flag
  - Dev container image: `pgvector/pgvector:pg18` (`internal/docker/compose.go`, testcontainers in `internal/storage/postgres/postgrestest/pool.go`)
  - Graph queries use recursive CTEs (`GetTransitiveDeps`, `GetImpact` with `CYCLE` clause; `GetCriticalPath` with manual path arrays), bounded to 50 hops
  - Instrumented via `github.com/exaring/otelpgx`

**File Storage:**
- Local filesystem only — no object storage (S3/GCS) integration found for application data. `hashicorp/go-getter` is present as a dependency (supports remote fetch of files/archives, e.g. for artifact bundles) but no direct S3/GCS bucket wiring was found in `internal/`.

**Caching:**
- None (no Redis/Memcached client found). A local file-based throttle cache exists for the drift-nudge feature (`xdg.CacheHome()/nudges/`), not a service integration.

## Authentication & Identity

**Auth Provider:**
- OIDC (OpenID Connect) — `internal/auth/` (`oidc_verifier.go`, `loginprovider.go`, `loginsync.go`, `identitystore*.go`)
  - Library: `github.com/coreos/go-oidc/v3`, `golang.org/x/oauth2`, `github.com/go-jose/go-jose/v4`
  - Multiple named OIDC providers configurable under `auth.oidc.providers` in global config (`internal/config/global.go`, `OIDCProviderConfig`), each with `issuer`, `client_id`, and `client_secret_env` (env-var indirection for the secret, e.g. `SPECGRAPH_OIDC_ENTRA_SECRET` — compatible with providers like Entra ID/Azure AD)
  - Browser-based login flow for the CLI (`specgraph login` / `specgraph logout`) and interactive login for the web UI
  - JIT (just-in-time) user provisioning and login-sync — `internal/auth/identitystore_jit_test.go`, `loginsync.go`
- Authorization: Cedar policy engine (`github.com/cedar-policy/cedar-go`) for app-role/permission evaluation, referenced alongside OIDC app-roles.

## Monitoring & Observability

**Error Tracking:**
- No dedicated error-tracking service (e.g. Sentry) integrated. Errors flow through structured logging and OTel traces/logs.

**Logs:**
- Structured logging via `log/slog`, bridged to OTel via `go.opentelemetry.io/contrib/bridges/otelslog` and fanned out with `github.com/samber/slog-multi`. Configurable format (json/text) and output (stdout/stderr) via `log.*` config / `SPECGRAPH_LOG_*` env vars (`internal/config/global.go`).

**Tracing/Metrics:**
- OpenTelemetry SDK (`go.opentelemetry.io/otel*` v1.43.0) — single Init/Shutdown lifecycle in `internal/telemetry/`, no-op when disabled.
- Exporters: OTLP (grpc + http) for traces/metrics/logs, and Prometheus exposition (`go.opentelemetry.io/otel/exporters/prometheus`, `github.com/prometheus/client_golang`) — auto-selected via `go.opentelemetry.io/contrib/exporters/autoexport`.
- ConnectRPC handlers instrumented via `connectrpc.com/otelconnect`; Postgres via `otelpgx`; outbound HTTP via `otelhttp`.

## CI/CD & Deployment

**Hosting:**
- Distributed as a container image (`Dockerfile`, Alpine 3.23 base) exposing port 9090, and as a standalone CLI binary. `deploy/` directory holds deployment assets. No specific cloud provider (AWS/GCP/Azure) hard integration found for hosting the service itself — the app connects outward only to its own Postgres.

**CI Pipeline:**
- GitHub Actions (workflow files not read in this pass, but referenced in project docs — `ci.yml` sets `GO_VERSION`)
- `task check` (fmt, license, lint, build, unit tests) and `task pr-prep` (adds integration + e2e tests, requires Docker) are the local/CI quality gates.
- `codecov.yml` — Codecov coverage reporting integration.

## Environment Configuration

**Required env vars:**
- `SPECGRAPH_SERVER_LISTEN` — server listen address
- `SPECGRAPH_SERVER_POSTGRES_URL` — Postgres connection string
- `SPECGRAPH_LOG_LEVEL` / `SPECGRAPH_LOG_FORMAT` / `SPECGRAPH_LOG_OUTPUT` — logging config
- `SPECGRAPH_CLIENT_DEFAULT_SERVER` — CLI default server target
- `SPECGRAPH_OIDC_<PROVIDER>_SECRET`-style vars (name is user-defined via `client_secret_env` in config) — OIDC client secrets
- `SPECGRAPH_DRIFT_NUDGE=off` — disables the drift-nudge stderr hint

**Secrets location:**
- No secrets committed to the repo. OIDC client secrets are resolved at runtime from environment variables named in `.specgraph.yaml`/global config (`client_secret_env`), never stored inline. `.env`-style files are not used by the project's own config loader.

## Webhooks & Callbacks

**Incoming:**
- OIDC redirect/callback endpoint for browser-based login flows (CLI and web UI) — part of `internal/auth/` login flow. No third-party webhook receivers (e.g. Stripe, GitHub App webhooks) found.

**Outgoing:**
- None found — GitHub and Beads integrations are CLI-shell-based (`gh`, `bd`), not webhook-driven.

---

*Integration audit: 2026-07-08*
