# OpenTelemetry Instrumentation: Traces, Metrics & Context-Aware Logging

**Status:** Design approved; revised after three adversarial review rounds (2026-06-05). Round 3 reworked the CLI bootstrap (single Init site, config resolver, handle handoff).
**Date:** 2026-06-05
**Scope:** Client (CLI) and server end-to-end observability via OpenTelemetry — traces, metrics, and logs — with context propagation across the boundary and centralized context-aware logging.

## Goal

Add proper observability seams and instrumentation to SpecGraph for both the CLI client and the server:

- All three OpenTelemetry signals: **traces**, **metrics**, **logs**.
- A **working, configurable OTLP export pipeline** (env-driven, no-op by default).
- **Context propagation** across the client→server boundary (W3C `traceparent`/`baggage`) and consistent `context.Context` threading internally (already strong in this codebase).
- **All logging leverages context** — every existing `slog` call automatically gains trace correlation (`trace_id`/`span_id`) and request-scoped fields (`project`, `identity`) with no call-site changes, and logs also export over OTLP.

Default behavior is **off**: the binary is silent and zero-overhead unless telemetry is explicitly enabled. Telemetry is best-effort and must never break or block the application.

## Current State (research summary)

- **Logging:** Go stdlib `log/slog`. Configured via `internal/config/global.go:73-129` (`LogConfig.Build()`); the only production `slog.SetDefault` is in `cmd/specgraph/serve.go:79-83`. Server code already passes `ctx` to log calls (`h.logger.ErrorContext(ctx, ...)`, `slog.LogAttrs(ctx, ...)`), but stdlib handlers do **not** extract anything from `ctx`. The CLI does not initialize slog; user-facing output is `fmt.Fprintln`/`printJSON`.
- **Context:** `context.Context` is threaded consistently as the first argument across handlers → scoper → storage → pgx. Existing context-value patterns: `txKey{}` (`storage/postgres/tx.go`), `projectKey{}` (`server/project.go:22`), `auth.WithBearerToken`/`WithProject`/`WithIdentity`, and the change-event slice (`storage.InitChangeEvents`).
- **OTel today:** none in application code. All `go.opentelemetry.io/*` entries in `go.mod` are `// indirect` (pulled by testcontainers/GCP/grpc). No `connectrpc.com/otelconnect`.
- **RPC boundary:** CLI is a ConnectRPC client over HTTP/1.1. Server interceptor wiring at `serve.go:250,260-261,271`; only one interceptor today (server-side unary auth, `internal/auth/interceptor.go:19`). The CLI passes **no** client options — `newClient[C]` (`cmd/specgraph/client.go:133,144`) has an unused `opts ...connect.ClientOption` seam. Header injection is done via custom `http.RoundTripper`s (`client.go:22-61`).
- **HTTP server:** `http.Server` at `serve.go:342-349`, listen at `serve.go:398`. Middleware chain `server.SecurityHeaders(server.ProjectMiddleware(mux))` at `serve.go:332`. Separate probe listener (`/livez`/`/readyz`) at `serve.go:469-503`.
- **MCP server:** mounted at `/mcp/` (`serve.go:322`) on the same mux; uses a loopback Connect client `mcppkg.NewClient(newHTTPClient(""), ...)` (`serve.go:298`). All tool/resource/prompt calls funnel through `wrapToolHandler`/`wrapResourceHandler`/`wrapPromptHandler` (`internal/mcp/server.go:140-183`), each receiving a `ctx`.
- **Change events:** `storage.ChangeSubscriber` interface (`OnSpecChanged(ctx, *ChangeEvent)`) with `Subscribe(...)`, already consumed by `notify.ImpactLogger`. `ChangeEvent` carries `Slug`, `Version`, `Stage`, `ContentHash`, `Checkpoint`, `Summary`, `Reason` (`internal/storage/change_event.go`).
- **Module:** `github.com/specgraph/specgraph`, Go 1.26.4. Build version via `buildVersion()` (`cmd/specgraph/main.go:26`).

## Architecture

A single cohesive package `internal/telemetry/` owns the entire observability lifecycle. Both the server and the CLI call the same `Init`/`Shutdown`, differing only by `Config`. When disabled, every provider is the OTel built-in no-op and the logger is the plain base handler (server stdout / CLI stderr) — zero hot-path overhead.

### Package layout

```text
internal/telemetry/
  telemetry.go      // Config, Telemetry struct, Init(ctx, Config) (*Telemetry, error), Shutdown(ctx)
  config.go         // standalone post-parse flag+env resolver (SpecGraph knobs) + OTEL_* via SDK; no koanf/yaml dependency
  providers.go      // TracerProvider / MeterProvider / LoggerProvider builders + no-op fallbacks
  logging.go        // context-enriching slog.Handler + fanout (base handler[server stdout / CLI stderr] + otelslog bridge)
  ctxfields.go      // injected context-accessor closures (project[both keys]/identity) — no server/auth import
  connect.go        // otelconnect interceptor helpers (client + server)
  pgx.go            // otelpgx tracer wiring for the pgxpool
  metrics.go        // app-level instruments (startup/shutdown, tx, drift, MCP, domain) + registration
  subscriber.go     // MetricsSubscriber implementing storage.ChangeSubscriber (Tier-2 domain metrics)
  telemetry_test.go // no-op default, context enrichment, propagation round-trip, metric seams
```

### Public seam

```go
type Config struct {
    Enabled     bool         // master switch (SPECGRAPH_OTEL_ENABLED); off => all no-op
    ServiceName string       // OTEL_SERVICE_NAME fallback
    Role        string       // "server" | "cli" (resource attr + which signals start)
    SampleRatio float64      // SpecGraph-native trace sampler arg (default 1.0)
    LogsExport  bool         // OTLP log export toggle (traces/metrics unaffected)
    LogHandler  slog.Handler // base stdout/stderr handler to fan into; server=cfg.Log.Build() (stdout), CLI=JSON handler on STDERR. Must be non-nil (no nil-wrap).
    Version     string       // service.version (from buildVersion())
}

type Telemetry struct {
    Logger *slog.Logger
    Tracer trace.Tracer
    Meter  metric.Meter
    // unexported providers for shutdown
}

func Init(ctx context.Context, cfg Config) (*Telemetry, error)
func (t *Telemetry) Shutdown(ctx context.Context) error
```

### Integration points (full edit surface)

- `cmd/specgraph/serve.go`: **consume the shared `*Telemetry` from the bootstrap holder** (initialized once in `nudgePreRun` with `Role=server` — serve does **not** call `telemetry.Init` again, avoiding a double-Init that would overwrite the OTel globals and double-Shutdown). `slog.SetDefault(t.Logger)`; **when `Enabled`**, add the otelconnect **server** interceptor to the existing `opts` (`serve.go:261`) and wrap the outer HTTP handler with `otelhttp` (outside `SecurityHeaders`/`ProjectMiddleware`, **excluding** the probe listener `serve.go:478`); emit `server.startup`/`server.shutdown` spans; wire `t.Shutdown` into the graceful-shutdown path (the CLI holder's `run()` defer is a no-op second `Shutdown` on the already-shut-down providers — `Shutdown` is idempotent, but serve owns the primary shutdown so flush happens before process exit). The interceptor/otelhttp wiring is **gated on `Enabled`** so the disabled binary takes the original code path (see No-op section).
- `internal/mcp/client.go`: the MCP loopback client `NewClient(httpClient, baseURL)` (`client.go:31`) has **no options seam today**. To correlate loopback calls, either (a) wrap `newHTTPClient("")` with `otelhttp.NewTransport` (preferred — no signature change, gated on `Enabled`), or (b) add `...connect.ClientOption` to `NewClient` and thread it to all 13 inner `specgraphv1connect.NewXxxClient` calls (`client.go:33-45`). Plan uses (a).
- `internal/mcp/server.go`: wrap **all four** dispatch adapters — `wrapToolHandler` (:140), `wrapResourceHandler` (:152), `wrapResourceTemplateHandler` (:164, serves the templated `specgraph://skills/<name>` resource), `wrapPromptHandler` (:175) — for spans + metrics.
- `cmd/specgraph/main.go` / `client.go` / root command (CLI seam — refactor `main`): replace the current `func main(){ if err := rootCmd.Execute(); … os.Exit }` (`main.go:80-88`) with `func main(){ os.Exit(run()) }` where `run() int` calls `rootCmd.ExecuteContext(ctx)`, then on a `defer` (so it runs on **every** branch) reads the bootstrap holder to `span.End()` then `t.Shutdown(ctx)`, and maps the error to an exit code.
  - **Why Init can't happen in `run()` before `ExecuteContext`:** cobra parses flags and resolves the target command *inside* `Execute`/`ExecuteContext`. Before that call the persistent-flag vars hold defaults, so `run()` cannot see `--otel*` or know which command (and thus which `Role`) is running. Therefore **telemetry is initialized inside `nudgePreRun`** (the root `PersistentPreRunE`, `main.go:49`), which cobra runs *after* flag parsing and command resolution. Verified: **no subcommand defines its own `PersistentPreRunE`**, so cobra does not shadow the root's — `nudgePreRun` fires for every command (`serve` included).
  - **Handle handoff PreRun → `run()`:** cobra's `PreRunE` signature `(*cobra.Command, []string) error` cannot return the `*Telemetry`/root-span handles, and `cmd.Context()` only flows *downward* into subcommands (it is not retrievable upward after `ExecuteContext` returns). The handoff therefore uses a **single package-level holder** in `cmd/specgraph` (`var tel *telemetry.Telemetry; var rootSpan trace.Span`), set in `nudgePreRun` and read by `run()`'s defer. This is **race-free**: the CLI is single-threaded and `PersistentPreRunE → ExecuteContext-body → run()-defer` is strictly sequential on one goroutine. (The earlier "avoid package-global mutable state" goal is dropped — it is the wrong constraint for a cobra PreRun→main handoff.)
  - **`nudgePreRun` body (composed, not replaced):** resolve telemetry config from the now-parsed persistent flags + `SPECGRAPH_OTEL_*` env (via `telemetry/config.go`, see Configuration) → choose `Role` from `cmd.Name()`/command path (`serve` ⇒ `server`, else `cli`) → `telemetry.Init` **once**, storing the result + a started root command span in the holder → set `cmd.SetContext(ctxWithRootSpan)` so subcommand/RPC spans are children → run the existing nudge logic. Because Init happens here, **`serve.go` does NOT call `telemetry.Init` again** — it consumes the shared holder (see serve integration point); this avoids the double-Init / global-overwrite that a second Init would cause.
  - **`span.End()` before `Shutdown` is mandatory:** `TracerProvider.Shutdown` only exports *ended* spans, so an un-ended root span (and its orphaned RPC children) would be dropped on error paths. The `run()` defer ends the span first, then shuts down.
  - Add the otelconnect **client** interceptor via the unused `opts ...connect.ClientOption` in the generic `newClient[C]`/`newClientWithProject` ctors (`client.go:133,144`, applied at call sites `:139,153`), gated on `Enabled`.
- `internal/storage/postgres/postgres.go`: `New()` currently calls `pgxpool.New(ctx, connString)` (`postgres.go:74`) — no `Config` exists. Refactor to `pgxpool.ParseConfig(connString)` → set `cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())` (only when telemetry enabled) → `pgxpool.NewWithConfig(ctx, cfg)`.
- Storage wiring: register `telemetry.MetricsSubscriber` via the existing `Subscribe(...)` seam alongside `notify.ImpactLogger`.

Existing slog call sites stay **unchanged** — they gain trace correlation through the new handler.

**Layering / import-cycle constraint:** `internal/telemetry` MUST NOT import `internal/server` or `internal/auth`. The project/identity context keys are **unexported** in those packages, so telemetry cannot read them directly anyway. Resolution: **inject accessor closures at `Init`** — `cmd/specgraph` (which already imports both `server` and `auth`) passes `func(context.Context)(string,bool)` closures over `server.ProjectFromContext` + `auth.ProjectFromContext` (and identity) into `telemetry.Init`. The enrich handler calls the injected closures; `telemetry` imports neither package. `ctxfields.go` holds the closure types and the wiring helper, *inside* `internal/telemetry` (it does not define or relocate the keys). `internal/storage/postgres` importing `internal/telemetry` for Tier-3 counters creates no cycle as long as `telemetry` never imports `postgres` (it doesn't — it only imports the leaf `internal/storage` for `ChangeSubscriber`).

## Configuration

OTel SDK reads standard `OTEL_*` env for transport (via `autoexport`); SpecGraph adds three native knobs resolved by a **standalone resolver in `telemetry/config.go`** with precedence **flag (if `cmd.Flags().Changed(...)`) > `SPECGRAPH_OTEL_*` env > built-in default**.

> **Why a standalone resolver, not koanf:** the existing koanf machinery (`loadGlobalCfg`, `internal/config/global.go`) runs **only** inside `serve` (`serve.go:70`); its `flagKeyMap` is explicitly serve-scoped (`global.go:42-50`) and `GlobalConfig` has **no `otel` section**. Non-`serve` commands never load config at all. So the otel knobs cannot ride koanf/`.specgraph.yaml` without adding a new koanf section *and* a CLI load path; the design instead uses a self-contained flag+env reader that works identically for every command. (No `.specgraph.yaml` layer for the otel knobs — that would be unbacked.)

**Standard `OTEL_*` (honored by the SDK, not re-implemented):** `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`.

**SpecGraph-native knobs** — exposed as **persistent root flags** (`rootCmd.PersistentFlags()`, `main.go:43-50`, alongside the existing `--config`) so they work for **every** command (CLI reads `specgraph spec list --otel`, not just `serve`), each resolved against its `SPECGRAPH_OTEL_*` env by `telemetry/config.go` (flags are read in `nudgePreRun`, *after* cobra parses them):

| Knob | Env | Flag (persistent, root) | Default | Effect |
|------|-----|------|---------|--------|
| Master enable | `SPECGRAPH_OTEL_ENABLED` | `--otel` | `false` | off ⇒ all providers no-op, no wiring, logger = plain stdout (server) / plain stderr (CLI) |
| Trace sample ratio | `SPECGRAPH_OTEL_SAMPLE_RATIO` | `--otel-sample-ratio` | `1.0` | `ParentBased(TraceIDRatioBased(r))` |
| Log export toggle | `SPECGRAPH_OTEL_LOGS` | `--otel-logs` | `true` (when enabled) | `false` ⇒ traces+metrics export, logs stay local (still trace-enriched) |

> Flags are **root-persistent**, not `serve`-only — otherwise enablement for every non-`serve` command would be env-only (`loadGlobalCfg`/koanf runs only in `serve`). The standalone `telemetry/config.go` resolver gives a uniform flag > env > default story across all commands. Server-specific transport defaults (e.g. endpoint) still come from `OTEL_*` env.

The master switch gates everything: `Enabled=false` short-circuits `Init` to the no-op path regardless of `OTEL_*` env, so the binary is silent and opt-in only. The SpecGraph `--otel-sample-ratio` is the single sampling knob — we construct the sampler ourselves rather than deferring to env samplers, documented as the source of truth.

## Provider Lifecycle & Propagation

**Resource:** built once via `resource.New` merging SDK detectors (env, process, host) + explicit `service.name` (`specgraph-server` / `specgraph-cli` from `Role`), `service.version` (`buildVersion()`), `service.instance.id`.

**`Init` flow (Enabled=true):**

1. Build `Resource`.
2. **Tracer:** `sdktrace.NewTracerProvider` with batch span processor over `autoexport.NewSpanExporter(ctx)`; sampler `ParentBased(TraceIDRatioBased(SampleRatio))`. (CLI role: use a **simple/synchronous** span processor, not batch — a short-lived process flushing a batch processor still works but a simple processor avoids a buffering layer that can drop spans if shutdown races.)
3. **Meter:** `sdkmetric.NewMeterProvider` with periodic reader over `autoexport.NewMetricReader(ctx)`. (CLI role: skipped — CLI is spans + propagation only.)
4. **Logger provider:** if `LogsExport`, `sdklog.NewLoggerProvider` with batch processor over `autoexport.NewLogExporter(ctx)`; else nil.
5. Set globals: `otel.SetTracerProvider`, `otel.SetMeterProvider`, `otellog/global.SetLoggerProvider`. **Propagator:** the *global* `otel.SetTextMapPropagator` is set to `TraceContext` only (safe at the untrusted external edge). `Baggage` is **not** applied globally — it is enabled only on internal/loopback hops by passing an explicit composite (`TraceContext{}, Baggage{}`) to the loopback `otelhttp.NewTransport(base, otelhttp.WithPropagators(composite))` (plural) and the otelconnect client interceptor `otelconnect.WithPropagator(composite)` (singular). (Option names per otelhttp v0.67.0 / otelconnect v0.9.0 — **verify the exact symbols against `go.mod` at impl time**; the capability exists in both, only the spelling must be confirmed.) This gives the edge-vs-internal asymmetry the security section requires; a single global composite cannot (it would extract attacker-supplied baggage at the public edge).
6. Build the enriched `*slog.Logger`; register app instruments and the metrics change-subscriber.
7. Return `*Telemetry`.

**`Init` flow (Enabled=false / no-op):** no providers created; OTel globals stay at built-in no-ops; `Logger = slog.New(cfg.LogHandler)` (plain base handler — server stdout / CLI stderr — no enrichment, no trace fields); `Shutdown` is a no-op. Critically, the **interceptor / `otelhttp` / `otelpgx` wiring is also skipped** when disabled (gated at the call sites in `serve.go`/`client.go`/`postgres.go`), so the disabled binary takes the exact original code path with no per-request OTel allocations.

> **Overhead note (corrected from review):** "Zero overhead when disabled" only holds if the *wiring itself* is gated on `Enabled`. If interceptors/`otelhttp` were left always-on with no-op providers, they would still run per request (propagator extraction, ResponseWriter wrapping, non-recording `Tracer.Start`) — small but non-zero. The plan therefore gates wiring, not just providers.

**Propagation across the boundary:**

- **Client:** the otelconnect client interceptor injects only `traceparent` (the **external** CLI client uses the *global* propagator, which is `TraceContext`-only — see Lifecycle step 5; it does **not** inject `baggage`). Only the **loopback** client adds `baggage`, via its explicit `TraceContext{}, Baggage{}` composite. Injection coexists with the existing `clientTransport` RoundTripper, which `req.Clone`s and only *adds* auth/project headers (`client.go:27-45`) — different layers, injected headers survive, no conflict (verified).
- **Server:** `otelhttp` + the otelconnect server interceptor extract incoming context so the server span is a child of the client span. `SecurityHeaders` (`security_headers.go:11-15`) and `ProjectMiddleware` (`project.go:33-41`) both forward the request context unchanged, so the span context reaches the interceptor (verified).

**Shutdown / flush:** `Shutdown(ctx)` flushes + shuts down each non-nil provider in reverse order, aggregating errors, bounded by a context timeout (default 5s), and is **idempotent** (a second call is a no-op). Server: wired into the existing signal-based graceful shutdown (the primary flush). CLI: because `main.go:80-88` calls `os.Exit` directly (bypassing defers) and cobra's `PersistentPostRunE` does not run on error paths, the CLI uses the `func main(){ os.Exit(run()) }` / `run() int` shape (see Integration points). `run()`'s `defer` reads the bootstrap holder set in `nudgePreRun` and runs `span.End()` → `Shutdown` on **every** branch. Only *ended* spans export, so the explicit `End` before `Shutdown` is mandatory or failure traces (the most valuable ones) are lost. For `serve`, the holder's deferred `Shutdown` is a harmless idempotent no-op after serve's own graceful-shutdown flush.

## Context-Aware Logging

Goal: every existing `slog` call (already ctx-passing) automatically gains trace correlation + request-scoped fields with no call-site changes, and logs also export over OTLP.

**Handler composition (`logging.go`):**

```text
slog.Logger
  └─ enrichHandler (context extractor)        ← adds attrs from ctx
       └─ fanout (slog-multi)
            ├─ base handler (server stdout / CLI stderr; existing cfg.LogHandler)  ← always
            └─ otelslog bridge → LoggerProvider                ← only if LogsExport
```

**`enrichHandler`** wraps a downstream handler; in `Handle(ctx, record)` it appends attributes when present (omitted otherwise, so startup/shutdown logs stay clean):

- `trace_id`, `span_id` — from `trace.SpanContextFromContext(ctx)` when the span is valid.
- `project` — read via the neutral accessor seam, which must check **both** project context keys: `server.projectKey{}` (set on the request path, `project.go:37`) **and** `auth.WithProject`/`auth.ProjectFromContext` (set on the MCP path, `serve.go:317`, a *different* key in `auth/context.go`). Reading only one drops `project` on MCP-path logs.
- `identity` — from the `auth` identity context value (via the accessor seam, not a direct `internal/auth` import — see layering constraint).

**Handler contract (corrected from review):** `enrichHandler` must correctly delegate `Enabled`, `WithAttrs`, and `WithGroup` to its downstream — not only override `Handle` — or grouped/`With`-attached attributes are silently dropped. A unit test asserts attr/group fidelity through the full `enrichHandler → fanout → {stdout, otelslog}` chain. There is exactly **one** `LoggerProvider` sink (the fanned-in otelslog bridge); no other otelslog handler is installed, avoiding double-emit.

**Fanout** uses `github.com/samber/slog-multi` so one logical record reaches both stdout and the OTLP bridge. The `otelslog` bridge converts records to OTel log records carrying the same trace context, correlating logs and traces in the backend.

**Toggle interaction:** `LogsExport=false` ⇒ fanout collapses to the base handler only (still trace-enriched). `Enabled=false` ⇒ no enrichment handler at all, plain base handler.

**CLI (corrected from review — critical):** the CLI gets the enriched logger too, but its base handler MUST write to **stderr**, never stdout. `--json` commands emit machine-readable protojson to **stdout** via `printJSON(cmd.OutOrStdout(), …)` (`output.go:15-24`); a single `slog.Warn/Error` on stdout would corrupt `specgraph … --json | jq`. The CLI is safe today only because it never `SetDefault`s slog. So: CLI `LogHandler` = a JSON handler bound to `os.Stderr` (non-nil), and `slog.SetDefault` uses the enriched-on-stderr logger. User-facing output (`fmt.Fprintln`/`printJSON` → stdout) is untouched — logging (stderr) and results (stdout) stay on separate streams.

## Instrumentation Coverage

Every seam emits **both a span and a metric** (and a trace-correlated log where it is a logged operation). Hand-rolled seams share one helper pattern: start span, defer a `recordDuration`/`recordOutcome` that writes to the seam's instrument using the same `ctx` and attributes. Instruments live in `metrics.go`, registered once at `Init`.

### Auto-instrumentation

- **RPC boundary** — `connectrpc.com/otelconnect`: server interceptor on the existing `opts` (`serve.go:261`); client interceptor on the unused `opts ...connect.ClientOption` (`client.go`). Per-RPC spans + standard RPC metrics on the server; traceparent propagation both ways. **Interceptor order:** the otelconnect interceptor must be **outermost** (first in `WithInterceptors`, before the auth interceptor at `serve.go:250`) so auth-rejected calls still appear in RPC spans/metrics and the span parents correctly.
- **HTTP server** — `otelhttp.NewHandler` wraps the outer handler (outside `SecurityHeaders`/`ProjectMiddleware`), producing the root server HTTP span. A span-name formatter keeps `/mcp/` legible. Probe listener (`/livez`/`/readyz`) is a **separate `http.Server`** (`serve.go:478`) and is **not** wrapped (verified — avoids health-check noise). Static web-asset requests (`serve.go:330`) should be filtered (`otelhttp.WithFilter`) to avoid a span per asset. (otelhttp v0.67.0 wraps the ResponseWriter via `httpsnoop`, which **preserves** `http.Flusher`/`Hijacker`, so wrapping does **not** break the `/mcp/` SSE/streamable endpoint — verified against mcp-go's `Flusher` assertion.)
- **Database** — `otelpgx` tracer on `pgxpool.Config` in `postgres.go`: each query a child span (sanitized SQL + table attrs), nested under the RPC span, following the same ctx-threaded tx (`tx.go`).

### Hand-rolled spans + metrics

| Seam | Span | Metric(s) |
|------|------|-----------|
| HTTP server (`otelhttp`) | server HTTP span | `http.server.request.duration`, active requests |
| RPC boundary (`otelconnect`) | RPC span | `rpc.server.duration`, request/response counts by procedure + status |
| DB / pgx (`otelpgx`) | per-query span | query duration histogram + pool gauges (`acquired`/`idle`/`total` from `pgxpool.Stat()`) |
| Storage tx (`RunInTransaction`, `tx.go`) | `storage.transaction` | `specgraph.storage.transaction.duration` histogram + `...transaction.retries` counter |
| Drift detection (`internal/drift`) | `drift.detect` | `specgraph.drift.detect.duration` histogram |
| Server startup/shutdown (`serve.go`) | `server.startup` / `server.shutdown` | `specgraph.startup.duration` histogram + `specgraph.uptime` observable gauge |
| MCP tool/resource/prompt (`wrap*Handler`) | `mcp.tool/<name>` etc. | `mcp.tool.duration` histogram + call counter by tool/outcome |

### MCP server coverage

- HTTP span free via `otelhttp` (same mux). **Latency nuance:** POST tool/resource/prompt calls return `application/json` and produce correct **per-call** HTTP spans. The MCP **GET notification stream** is long-lived (SSE) — its otelhttp span stays open for the whole session and won't export until disconnect under a batch processor, inflating `http.server.active_requests`. Treat the per-operation `mcp.*` spans/metrics as the latency signal; exclude `/mcp/` from the HTTP duration histogram (or filter the GET stream).
- MCP operation spans + metrics added at **all four** `wrap*Handler` chokepoints (`internal/mcp/server.go:140-183`): `wrapToolHandler`, `wrapResourceHandler`, `wrapResourceTemplateHandler`, `wrapPromptHandler` — one edit per adapter, not per tool. Spans `mcp.tool/<name>`, `mcp.resource/<uri>`, `mcp.resource_template/<uri>`, `mcp.prompt/<name>` with name + outcome attrs; `mcp.tool.duration` histogram + call counter. **Coverage scope:** protocol-level ops (`initialize`, `ping`, `tools/list`, `resources/list`, `prompts/list`, notifications) are `MCPServer` methods that bypass the four adapters — they get the outer `/mcp/` HTTP span only, **no `mcp.*` operation span**. This is deliberate (low-value, high-frequency); the per-item invocations are what's instrumented.
- **Loopback correlation:** the loopback HTTP client (`newHTTPClient("")`, `serve.go:298`) is wrapped with `otelhttp.NewTransport` (gated on `Enabled`) so outgoing loopback requests carry `traceparent` and re-enter as child spans. (The `mcp.NewClient` ctor has no options seam — see Integration points.) Result: one connected trace `MCP HTTP → mcp.tool/author → loopback RPC author → storage tx → pgx query`. No infinite-nesting risk: the call graph is a finite DAG (tools → RPC → storage, never back into MCP). Note this yields **two server-side HTTP spans per MCP call** in the same process (external `/mcp/` + loopback `/specgraph.v1.*`) plus the RPC span — verbose but correct.
- MCP-path logs carry `trace_id`/`project` via the existing MCP context func (`serve.go:301-320`) feeding the enriched logger — provided the enrich handler reads the `auth.WithProject` key as well as `server.projectKey` (see Logging).

### CRUD / domain-operation metrics

**Tier 1 — RPC-level CRUD (free):** otelconnect server metrics count every procedure (`CreateSpec`/`UpdateSpec`/`DeleteSpec`/`GetSpec`/`AddEdge`/`TransitionStage`, …) by name + status + duration.

**Tier 2 — domain-dimensioned mutation metrics:** a `telemetry.MetricsSubscriber` implementing `storage.ChangeSubscriber`, registered via the existing `Subscribe(...)` seam (alongside `notify.ImpactLogger`). On each committed change it increments `specgraph.spec.changes` (counter) by `stage` + `checkpoint`. Zero handler edits; rides the existing `ChangeEvent`. **Note:** subscriber dispatch uses a detached `context.Background()` (`tx.go:114,127`), so Tier-2 metrics carry **no span context / exemplars** — a plain counter only. This is acceptable for a counter but means Tier-2 is the one seam not trace-correlated; the trace-level record of the mutation is the RPC/tx span instead.

**Tier 3 — curated domain counters at storage seams:**

- `specgraph.edge.mutations` by `edge_type` + `op` (add/remove)
- `specgraph.stage.transitions` by `from`→`to`
- `specgraph.findings.stored` by `pass_type`

**Deferred:** `specgraph.specs.total` / `decisions.total` observable gauges (need periodic count queries) — follow-up.

**Cardinality rule (hard):** metric attributes are restricted to bounded values (`stage`, `type`, `op`, `status`, `pass_type`, `procedure`). Never `slug`/`id`/free-text as a metric dimension — those remain span attributes and log fields only.

**Span-name cardinality (otelpgx):** otelpgx defaults the **span name to the SQL text**, producing a distinct span name per statement (high cardinality in trace backends). Mitigate with `otelpgx.WithTrimSQLInSpanName()` (or a span-name formatter). The `/mcp/` HTTP span-name formatter must NOT be reused as a metric route tag.

### Security / PII

- **DB query parameters:** pgx uses parameterized queries, so `db.statement` carries `$1` placeholders, not bound values. Keep `otelpgx.WithIncludeQueryParameters()` **off** so parameter values never reach spans.
- **Bearer token / identity:** the token stays in the `Authorization` header via `clientTransport` (`client.go:42-44`) and is never placed in spans or baggage. The enrich handler's `identity` is a local log attribute only — not propagated over the wire.
- **Baggage trust boundary:** the global propagator is `TraceContext`-only, so the public edge does **not** extract attacker-supplied `baggage` (a cardinality/PII injection vector). `Baggage` is applied only on internal/loopback hops via an explicit composite on the loopback transport + client interceptor (see Lifecycle step 5). Policy: never put identity/token/PII in baggage. **Sampling caveat:** because the edge still extracts inbound `traceparent`, an authenticated caller can inject a trace ID and force a sampling decision; bounded by `RequireAuth` on `/mcp/` (`serve.go:322`), documented as accepted.

### CLI role

Spans + propagation only. No MeterProvider, so CLI command/RPC seams emit spans (and flush on exit) but no metrics. The otelconnect **client** interceptor *is* wired on the CLI and is capable of emitting metrics, but with no MeterProvider set its instruments resolve against the global **no-op** meter — they record nothing and never error (valid instruments, no output). Full span+metric+log treatment is server-role only.

## Error Handling (telemetry must never break the app)

- `telemetry.Init` failures are **non-fatal**: log a warning, fall back to no-op providers, continue running.
- Exporters use the SDK's async batch processors (server role), so a slow/down collector never blocks request handling. The CLI role uses a simple/sync span processor and flushes on exit.
- `otel.SetErrorHandler` routes internal SDK errors to slog at `warn` — visible but quiet.
- `Shutdown` errors are aggregated and logged, never fatal.

## Testing Strategy

All telemetry tests are unit-level (no Docker) so they run in `task check`. otelpgx behavior is covered by the existing postgres integration suite (`task pr-prep`).

- **No-op default:** `Init` with `Enabled=false` yields working no-op providers + plain base-handler logger (CLI variant on stderr); assert no exporter activity and identical behavior, and that interceptor/otelhttp wiring is skipped.
- **Context enrichment:** ctx with valid span + project + identity ⇒ `trace_id`/`span_id`/`project`/`identity` on the record; bare ctx ⇒ omitted. Include a case where `project` is set via the **`auth.WithProject`** key (MCP path) to prove both keys are read.
- **Handler fidelity:** assert `WithAttrs`/`WithGroup` attributes survive the full `enrichHandler → fanout → {stdout, otelslog}` chain.
- **Propagation round-trip:** in-memory `tracetest.SpanRecorder`; fire a client→server RPC through both interceptors; assert server span parent == client span (one connected trace).
- **Metric seams:** `metric/metrictest` manual reader; exercise tx, drift, MCP tool, and a `ChangeEvent`; assert instruments recorded with the right bounded attributes.
- **Cardinality guard:** assert domain metric attribute sets never include slug/id.

## Dependencies

> **Corrected from review — this is NOT a simple "promote indirect to direct."** The first implementation step MUST run `go mod tidy` with pinned candidate versions and verify the resolved matrix compiles, then re-validate against the pinned GCP exporter stack (`opentelemetry-operations-go/exporter/metric v0.55.0`, `contrib/detectors/gcp v1.42.0`) and the existing `otlptrace`/`otlptracehttp` pins.

**Already present as `// indirect` (GA, v1.43.0 — safe to promote):**

- `go.opentelemetry.io/otel`, `.../otel/sdk`, `.../otel/sdk/metric`, `.../otel/trace`, `.../otel/metric`
- `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` (v0.67.0)

**Absent from `go.mod`/`go.sum` — net-new subtrees (must add + pin):**

- `go.opentelemetry.io/contrib/exporters/autoexport` — pulls **13+ new modules** and **bumps** the existing `otlptrace`/`otlptracehttp` (v1.40.0/v1.41.0) up to the otel-core-aligned version (≈v1.43.0); re-validate the GCP stack after.
- `go.opentelemetry.io/otel/log`, `go.opentelemetry.io/otel/log/global` (logs API used by `global.SetLoggerProvider` and the bridge)
- `go.opentelemetry.io/otel/sdk/log` — **BETA (v0.x, ~v0.18-0.20)**, API churns between minors.
- `go.opentelemetry.io/contrib/bridges/otelslog` — **BETA**, version-locked to a specific `otel/log` minor. The chosen `otelslog` version's required `otel/log` MUST equal autoexport's required `otel/log` (they share the module), or it won't compile. Pin both deliberately.

**Beta-risk decision (accepted):** OTLP **log export** rides the beta logs stack above. Per design decision, we **ship it now**, but: (a) it sits behind the `--otel-logs` toggle so traces/metrics keep working if logs break; (b) stdout log **context-enrichment** (`trace_id`/`span_id`/`project`/`identity`) depends on NONE of the beta modules and ships regardless; (c) the plan pins exact versions and documents the beta risk. If `go mod tidy` cannot resolve a clean, GCP-compatible matrix, fall back to enrichment-only logs and defer OTLP log export.

**New direct adds (GA, MVS-compatible with the repo — verified):**

- `connectrpc.com/otelconnect` (v0.9.0; requires connect ≥v1.17.0, otel ≥v1.29.0 — repo's v1.19.1/v1.43.0 satisfy)
- `github.com/exaring/otelpgx` (**pin v0.11.1**, otel-v1.43.0-aligned). Note: otelpgx v0.11.1 requires `pgx/v5 v5.9.2`, so `go mod tidy` will **bump the repo's pgx v5.9.1 → v5.9.2** (one patch; pin it and run the postgres integration suite to confirm).
- `github.com/samber/slog-multi`

Pin every added version explicitly. The new package needs a `// Package telemetry ...` doc comment (revive). License headers required on all new files. `task proto` unaffected. **Lint hot spots (caught by `task check`, not pre-commit):** the `Init` fallback and `Shutdown` aggregation return raw `go.opentelemetry.io/otel/sdk` errors — wrap them (or targeted `//nolint:wrapcheck`); the multi-provider sequences reuse `err`, so write them to avoid `govet -shadow`.

## Documentation

- A `docs/` section on enabling telemetry: `SPECGRAPH_OTEL_ENABLED`, the three SpecGraph knobs, standard `OTEL_*` env, and the emitted span/metric/log inventory.
- Update `AGENTS.md` Architecture table with the new `internal/telemetry/` row.

## Out of Scope (follow-ups)

- `*.total` observable gauges (specs/decisions/edges current counts).
- CLI metrics (CLI stays spans + propagation only).
- Ready-made collector/dashboards (Jaeger/Prometheus/Grafana compose).

**Not applicable:** there are **no streaming RPCs** in the codebase (verified — no `ServerStream`/`ClientStream`/`BidiStream` in `gen/`, no `stream` in `proto/`), so the existing unary-only interceptor pattern and otelconnect's unary path cover everything. No streaming-interceptor work is needed.

## Implementation Phasing

This design spans four independently-testable sub-areas; the implementation plan should sequence them so the dependency-matrix risk surfaces first:

1. **Deps + SDK lifecycle + no-op + config** — resolve the full module matrix (`go mod tidy`, incl. beta logs + GCP re-validation); `telemetry.go`/`config.go`/`providers.go`; no-op path; config knobs. **Gate:** clean build + no-op default test.
2. **Traces + propagation** — otelconnect server/client, otelhttp wrap, otelpgx pool refactor (`postgres.go` `ParseConfig`/`NewWithConfig`), MCP loopback transport wrap + four `wrap*Handler` spans, CLI bootstrap (`nudgePreRun` Init + holder + `run()` defer flush; `Execute`→`ExecuteContext`). **Gate:** propagation round-trip test.
3. **Context-aware logging** — enrich handler (both project keys, `WithAttrs`/`WithGroup` delegation) + slog-multi fanout + otelslog bridge (beta), neutral accessor seam. **Gate:** enrichment + attr-fidelity tests.
4. **Metrics** — auto (otelhttp/otelconnect/otelpgx) + Tier-2 subscriber + Tier-3 storage counters. **Gate:** metric-seam + cardinality-guard tests.
