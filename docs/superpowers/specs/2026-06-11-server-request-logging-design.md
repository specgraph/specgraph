# Per-request access logging

**Date:** 2026-06-11
**Status:** Design approved, revised after adversarial review, pending spec review

## Problem

The server emits almost no per-request signal. The only request-scoped log
is `mcpHeaderLogger`, which records `/mcp/` header *names* at `DEBUG`.
`otelhttp`/`otelconnect` produce spans and metrics, but no human-readable
access log. Operators have no idiomatic "one line per request" view of
traffic, latency, or error outcomes.

We want a standard access log — one line per request with method, path,
status, duration, and (for RPCs) procedure + connect code — emitted at a
level that reflects the outcome, with the high-frequency probe endpoints
gated behind a config flag so kubelet traffic doesn't flood the logs.

## Goals

- One access-log line per request covering **all** main-listener traffic:
  ConnectRPC, REST handlers, `/mcp/`, and the static web UI.
- RPC lines additionally carry the `procedure` and connect `code`.
- Outcome-based level so errors are filterable/alertable (`2xx/3xx` INFO,
  `4xx` WARN, `5xx` ERROR; connect codes mapped similarly).
- Probe-endpoint logging **off by default**, enabled via config for debugging.
- A master switch to disable main access logging (volume/cost control).
- No sensitive data in logs (no `Authorization`, no query strings, no bodies).

## Non-goals

- Request/response **body** logging.
- Sampling / rate-limiting of access logs (deferred; see "Static-asset volume").
- Changing the telemetry (trace/metric) pipeline. Access logging is additive
  and reuses the existing trace-enriched slog handler.
- A second, separate RPC log line. The interceptor enriches the single edge
  line rather than emitting its own.

## Decisions (from brainstorming)

| Question | Decision |
|----------|----------|
| Logging layer | Edge HTTP middleware **and** a Connect interceptor, composed as one line |
| Composition | Single enriched line via a per-request carrier (interceptor fills it) |
| Probe gating | Probe-listener logging **off by default**, config flag to enable |
| Field set | Standard idiomatic set (below); query/auth/bodies never logged |
| Level | Outcome-based (status + connect code → INFO/WARN/ERROR) |
| Config shape | `log.requests` (default true) + `server.probes.log_requests` (default false) |

## Field ownership (resolves the enrichment/duplication question)

The telemetry-enriched slog handler (`internal/telemetry/logging.go`) already
adds attributes to **every** log record whose context carries them:
`trace_id`, `span_id` (from the span), `project` (via
`server.ProjectFromContext`), and `identity` (via `auth.IdentityFromContext`).
To avoid **duplicate keys**, `AccessLog` must NOT re-add anything the enrich
handler adds for a context it can see. Concretely:

| Field | Source | Present when telemetry OFF? |
|-------|--------|------------------------------|
| `method`, `path`, `status`, `duration_ms`, `bytes`, `remote_ip` | `AccessLog` (always) | yes |
| `procedure`, `code` | carrier (filled by interceptor) → `AccessLog` | yes |
| `identity` | carrier (filled by auth) → `AccessLog` | yes |
| `project` | enrich handler (context already has it at the edge) | **no** |
| `trace_id`, `span_id` | enrich handler (span in context) | **no** |

Rationale for the split:

- `project` lives in the context at the edge (`ProjectMiddleware` is *outer*
  to `AccessLog`), so the enrich handler adds it; `AccessLog` adding it too
  would emit a duplicate `project` key (slog does not dedupe). When telemetry
  is off, `project` is simply absent — acceptable for the dev/degraded path.
- `identity` is set **downstream** of `AccessLog` (inside the auth
  interceptor / `RequireAuth`), so it is NOT in `AccessLog`'s context and the
  enrich handler never adds it to the access line. `AccessLog` therefore emits
  `identity` itself, sourced from the carrier — no duplication, and it works
  telemetry-off. Key name is `identity` to match the enrich handler.

## Architecture

### 1. Per-request carrier

A small mutable struct behind a context key in `internal/server`:

```go
type requestInfo struct {
    procedure string // RPC procedure, e.g. /specgraph.v1.GraphService/AddEdge
    code      string // connect code string: "ok", "invalid_argument", ...
    identity  string // authenticated principal, set by the auth layer
}
```

- `AccessLog` allocates a `requestInfo`, keeps a **local pointer**, and seeds
  that same pointer into the request context before calling `next`.
- Inner layers look it up via `requestInfoFromContext(ctx) *requestInfo` and
  mutate it **through the pointer**. Connect's handler context descends from
  `r.Context()` (verified), so the pointer is visible to the interceptor and
  to the auth layer. `AccessLog` reads its local pointer after `next` returns —
  pointer mutation is visible without re-injection.
- `requestInfoFromContext` returns `nil` when absent (e.g. on the probe
  listener, which runs no interceptor and seeds no carrier unless logging is
  enabled).

### 2. Edge middleware: `AccessLog`

`internal/server/access_log.go`:

```go
func AccessLog(next http.Handler) http.Handler
```

Per request:

1. Allocate `info := &requestInfo{}`; seed it into the context.
2. Wrap the `ResponseWriter` with a lightweight recorder (status defaulting to
   200, plus a byte counter), composed via `httpsnoop.Wrap` so optional
   interfaces (`http.Flusher`/`http.Hijacker`) are preserved — required for the
   `/mcp/` streamable endpoint.
3. In a `defer` (so it runs during a panic unwind), emit **one** line with
   `msg="request"`. The line MUST be emitted with the post-`next`
   **`r.Context()`** (via `slog.LogAttrs(r.Context(), …)`), not
   `context.Background()` — otherwise the enrich handler can't read the span or
   project and `project`/`trace_id`/`span_id` silently vanish. `duration_ms` is
   measured from `AccessLog` entry (before `next`), so it excludes outer
   otelhttp/CORS overhead. Fields:
   - `method`, `path` (`r.URL.Path` only — query string stripped),
     `status`, `duration_ms`, `bytes`, `remote_ip`.
   - When the carrier is populated: `procedure`, `code`, `identity`.
   - **Not** `project`/`trace_id`/`span_id` — those come from the enrich
     handler (see "Field ownership").
4. **Panic handling:** the `defer` calls `recover()`. If a panic occurred, log
   the line at ERROR with `status=500`, then re-`panic(rec)` so net/http's
   per-connection recovery still tears the connection down. (`httpsnoop`'s
   return-value `CaptureMetrics` form is NOT used: it returns metrics only on
   normal completion, so a panic would lose the captured status — hence the
   explicit recorder + `recover`.)
5. Level chosen per "Level mapping".

`remote_ip`: first hop of `X-Forwarded-For` when present, else the host part of
`RemoteAddr`. **This value is client-spoofable** unless a trusted L7 proxy
overwrites `X-Forwarded-For`; it is informational, not an authentication or
audit source. Behind a mesh, `RemoteAddr` is the sidecar.

The middleware is only added to the chain when `log.requests` is true.

### 3. Connect interceptor: `AccessLogInterceptor`

A `connect.Interceptor` (unary + streaming) that, around each call:

- sets `info.procedure` from the request/connection `Spec().Procedure`,
- after the inner call, sets `info.code = connectCode(err)`, where
  `connectCode` returns `"ok"` for `err == nil` and otherwise
  `connect.CodeOf(err).String()` (note: `connect.CodeOf(nil)` is `Unknown`, so
  the `nil` check must come first; a plain non-connect handler error yields
  `Unknown` → ERROR, which is the desired "unexpected failure" signal).

It looks up the carrier via `requestInfoFromContext`; if absent it is a no-op.
It **never logs** — it only enriches. **It MUST be registered as the OUTERMOST
interceptor** (prepended to the list), because connect runs the first
interceptor outermost (see `cmd/specgraph/serve.go`, where the otel interceptor
is prepended "outermost: before auth"). The auth interceptor returns
`Unauthenticated`/`PermissionDenied` **without calling `next`**, so an
innermost access-log interceptor would never run for rejected RPCs — exactly
the security-relevant denials we most want logged. Outermost placement
guarantees it runs and observes the final outcome code.

### 4. Identity capture (auth layer)

`identity` is populated through the carrier by the code that resolves the
principal, best-effort:

- the auth **interceptor** (RPC path), after it resolves the identity, and
- `RequireAuth` (REST/MCP path),

each do a guarded `if info := requestInfoFromContext(ctx); info != nil { info.identity = id.Subject }`.
No auth signatures change; this is a single additive write on each path. When a
request is unauthenticated/rejected, `identity` is simply empty.

**Scope note:** the auth interceptor is a `connect.UnaryInterceptorFunc`, so it
covers unary RPCs only. Streaming RPCs are not authenticated by it today (a
pre-existing fact) and therefore carry no `identity`. This is acceptable and
out of scope to change here.

### 5. Chain placement

Actual main-handler wiring is `SecurityHeaders(ProjectMiddleware(mux))`, then
optionally wrapped by `otelhttp` (telemetry on) and `CORSMiddleware` (when a
CORS origin is set). `AccessLog` is inserted **innermost, around the mux**:

```text
[CORS] → [otelhttp] → SecurityHeaders → ProjectMiddleware → AccessLog → mux(+interceptors)
```

(`[...]` = applied conditionally.) i.e. `SecurityHeaders(ProjectMiddleware(AccessLog(mux)))`.

This placement gives `AccessLog`'s logging context the span (`trace_id`, since
otelhttp is outer) and the project (since `ProjectMiddleware` is outer, so the
enrich handler adds `project`), while keeping `AccessLog` outside the mux so
the carrier pointer flows into the connect interceptors. The
`AccessLogInterceptor` is **prepended** to the interceptor list (see §3).

### 6. Probe listener

The probe listener (`startProbeListener`) is a separate `http.Server` whose
handler is `probeHandler.Mux()`. When `server.probes.log_requests` is true, its
mux is wrapped with `AccessLog`; otherwise it is left bare (today's silent
behavior). Probe requests run no interceptor and no `ProjectMiddleware`/otelhttp
wrap, so a probe line carries `method/path/status/duration_ms/bytes/remote_ip`
only (no `procedure`/`code`/`identity`/`project`/`trace_id`). `startProbeListener`
receives the flag (via the resolved `ProbesConfig`).

### 7. Level mapping

Level is the more severe of the HTTP-status level and the connect-code level:

- HTTP: `< 400` → INFO, `4xx` → WARN, `≥ 500` → ERROR.
- Connect code: `Internal`, `Unknown`, `Unavailable`, `DataLoss`,
  `DeadlineExceeded` → ERROR; other non-`ok` codes
  (`InvalidArgument`, `NotFound`, `AlreadyExists`, `PermissionDenied`,
  `Unauthenticated`, `FailedPrecondition`, `ResourceExhausted`, …) → WARN;
  `ok`/absent → defer to the HTTP-status level.

The connect-code dimension matters because gRPC/gRPC-Web clients return
**HTTP 200** with the error only in the trailer; without it, a denied RPC would
be logged INFO. Because lines are leveled, raising `log.level` to `warn` keeps
only the problem requests.

## Config

`internal/config/global.go`:

- `LogConfig.Requests bool` — `yaml:"requests" koanf:"requests"`. Defaulted to
  **true** in `globalDefaults()`. An explicit `log.requests: false` overrides
  (koanf last-write-wins), matching the established `Server.Docker` default-true
  pattern.
- `ProbesConfig.LogRequests bool` — `yaml:"log_requests" koanf:"log_requests"`.
  Zero value **false**; no default entry needed.

No new validation is required; both are plain booleans.

## Sensitive data

Never logged:

- the `Authorization` header (and any other header values),
- the URL query string (credential params such as `access_token`/`token`
  ride there — see `internal/constitution/fetch`),
- request or response bodies.

Only `r.URL.Path` (path, no query) is logged. `remote_ip` is informational and
spoofable (see §2) — not for audit.

## Edge cases

- **MCP / REST / static get no `procedure`/`code`.** `/mcp/` is a plain
  `http.Handler` chain (not a connect handler), and REST/static likewise run no
  connect interceptor, so those lines carry HTTP fields only — by construction.
- **MCP SSE / long-lived GET stream**: the line emits when `ServeHTTP` returns
  (stream close), so an in-flight stream produces no line until then;
  `duration_ms` reflects the stream lifetime, bounded by the main server's
  `WriteTimeout` (60s). `otelhttp` already excludes `/mcp/` GET from spans, so
  those lines also lack `trace_id`. Acceptable; can be excluded later if noisy.
- **Panic in a handler**: handled by the `recover()` in `AccessLog`'s defer —
  logs ERROR/`status=500`, then re-panics so existing connection teardown is
  unchanged (§2.4). connect-go does NOT recover handler panics by default
  (`WithRecover` is opt-in and unused here), so this path is real for RPC
  handlers too. Two caveats: (a) the `info.code` assignment runs *after*
  `next`, so a panic line carries `procedure` but an **empty `code`** — status
  500 still drives the ERROR level correctly; (b) net/http's per-connection
  recovery emits its own `http: panic serving` line to `http.Server.ErrorLog`
  (currently the default std logger → plain-text stderr), so a panic yields the
  structured `AccessLog` line **plus** that secondary plain line. Optionally set
  `srv.ErrorLog` to a slog-backed writer for a single structured stream.
- **CORS preflight**: `CORSMiddleware` is outermost and may answer `OPTIONS`
  preflights without calling `next`, so `AccessLog` won't log them. CORS is
  dev-only (gated on `--cors-origin`); acceptable.
- **Telemetry disabled**: `project`/`trace_id`/`span_id` are absent (no enrich
  handler, no otelhttp span); the line still carries the HTTP fields plus
  `procedure`/`code`/`identity` from the carrier.
- **Static-asset volume**: `AccessLog` logs every static UI asset (JS/CSS/img)
  on the main listener; a dashboard page view can produce many lines. The
  master switch (`log.requests`) is the volume control; per-path sampling is a
  deferred non-goal.

## Testing

Unit (slog captured to a buffer + `httptest`):

- Emits exactly one `msg="request"` line per request.
- Level mapping table: `200`→INFO, `404`→WARN, `500`→ERROR; connect
  `internal`→ERROR, `invalid_argument`→WARN, `ok`→status-driven; and the
  `nil`-error (→`ok`) vs non-connect-error (→`unknown`→ERROR) boundary.
- Carrier enrichment: an inner handler/interceptor that fills the carrier
  yields `procedure`/`code`/`identity`; absent carrier → no RPC/identity fields.
- **No duplicate `project`**: with a project in context and the enrich handler
  active, exactly one `project` key appears.
- **Interceptor runs on auth denial**: with the access-log interceptor
  outermost and an auth interceptor that rejects without calling `next`, the
  `code` is still captured (e.g. `permission_denied`) and the level is WARN.
- **Panic → 500**: a handler that panics yields one ERROR line with
  `status=500` and the panic still propagates.
- Redaction: query string omitted from `path`; no `Authorization` value
  anywhere in the line.
- `trace_id` present when the context carries a span; absent telemetry-off.

Config:

- `log.requests` defaults true; explicit `false` honored.
- `server.probes.log_requests` defaults false; explicit `true` honored.

Probe listener (via `startProbeListener` + `httptest`):

- Default (flag off): `/livez` produces no access line.
- Flag on: `/livez` produces a plain HTTP access line (no `procedure`/`code`).

## Files touched

- `internal/server/access_log.go` (new) — `requestInfo` carrier, accessor,
  `AccessLog` middleware (recorder + recover-based panic path), level mapping.
- `internal/server/access_log_interceptor.go` (new) — `AccessLogInterceptor`
  - `connectCode` helper.
- `internal/auth/` — one guarded carrier write each in the auth interceptor and
  `RequireAuth` to populate `identity` (no signature changes).
- `internal/config/global.go` — `LogConfig.Requests` (default true) +
  `ProbesConfig.LogRequests`.
- `cmd/specgraph/serve.go` — **prepend** the interceptor to the list; wrap the
  main chain with `AccessLog` when `log.requests`; pass the probe flag into
  `startProbeListener` and wrap the probe mux when enabled.
- `go.mod` — `github.com/felixge/httpsnoop` moves from indirect to direct.
- New tests alongside `access_log.go`, the interceptor, and the config/serve
  changes.

No proto, storage-interface, handler, or connect-interceptor **signature**
changes.
