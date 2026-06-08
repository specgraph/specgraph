# Observability with OpenTelemetry

SpecGraph ships native [OpenTelemetry](https://opentelemetry.io/)
instrumentation for traces, metrics, and logs across both the server
(`specgraph serve`) and the CLI. Telemetry is **off by default** and
**zero-overhead when disabled** — with the master switch off the binary is
silent, installs no exporters, wires no per-request instrumentation, and
takes the original code path.

## Enable Telemetry

The master switch is a **persistent root flag**, so it works on every
command — `specgraph serve`, `specgraph spec list`, and the rest — not just
`serve`:

```bash
# via flag
specgraph --otel serve

# via env
SPECGRAPH_OTEL_ENABLED=true specgraph serve
```

When the switch is off all providers are no-ops and no wiring happens. When
it is on, SpecGraph installs the OTel SDK and emits the signals described
below.

## SpecGraph-Native Knobs

Three knobs are owned by SpecGraph. Precedence is **flag (if set) >
`SPECGRAPH_OTEL_*` env > default**:

| Knob | Env | Flag | Default | Effect |
|------|-----|------|---------|--------|
| Master enable | `SPECGRAPH_OTEL_ENABLED` | `--otel` | `false` | off ⇒ all providers no-op, no wiring |
| Trace sample ratio | `SPECGRAPH_OTEL_SAMPLE_RATIO` | `--otel-sample-ratio` | `1.0` | `ParentBased(TraceIDRatioBased(r))` |
| Log export toggle | `SPECGRAPH_OTEL_LOGS` | `--otel-logs` | `true` (when enabled) | `false` ⇒ traces + metrics export, logs stay local but still trace-enriched |

These are all persistent root flags.

## Standard OTEL_* Environment

Transport and resource configuration is handled by the OpenTelemetry SDK
itself — SpecGraph does not re-implement it. The standard `OTEL_*`
environment variables are honored as-is:

| Variable | Purpose |
|----------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | Headers (e.g. auth) for the OTLP exporter |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` or `grpc` |
| `OTEL_SERVICE_NAME` | Service name on emitted resources |
| `OTEL_RESOURCE_ATTRIBUTES` | Extra resource attributes |
| `OTEL_TRACES_EXPORTER` | Per-signal selector (e.g. `none` to disable, `otlp` to export) |
| `OTEL_METRICS_EXPORTER` | Per-signal selector for metrics |
| `OTEL_LOGS_EXPORTER` | Per-signal selector for logs |

### Minimal Example

Enable telemetry and point it at a local collector:

```bash
SPECGRAPH_OTEL_ENABLED=true \
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
specgraph serve
```

!!! note "Enabling `--otel` on the CLI needs a reachable collector"
    The CLI flushes its spans synchronously on exit. With `--otel` on but no
    collector reachable at the configured (or default) OTLP endpoint, a CLI
    command can pause on exit until the export attempt times out. Point
    `OTEL_EXPORTER_OTLP_ENDPOINT` at a running collector, or set
    `OTEL_TRACES_EXPORTER=none` to keep telemetry on without exporting.

## Server vs CLI Roles

The two binaries emit different signal sets:

| Role | Traces | Metrics | Logs |
|------|--------|---------|------|
| Server (`specgraph serve`) | ✅ | ✅ | ✅ |
| CLI (every other command) | ✅ (+ context propagation) | — | ✅ |

- **Server logs** go to the configured log stream (stdout by default,
  honoring `--log-format` / `--log-level` / `--log-output`).
- **CLI logs** go to **STDERR** so `--json` output on stdout stays clean.

The CLI emits no metrics — only traces, context propagation, and logs.

## Context-Aware Logging

When telemetry is enabled, every log line automatically gains correlation
fields with **no code changes**:

- `trace_id` / `span_id` — present when the log is emitted inside a span.
- `project` — the request-scoped project.
- `identity` — the authenticated subject (e.g. `apikey:…` / `oidc:…`).

Logs are also exported over OTLP when `--otel-logs` is on. With
`--otel-logs=false`, logs stay local (written to the normal stream) but
remain trace-enriched — only traces and metrics are exported.

## Emitted Signal Inventory

### Traces (spans)

- **HTTP server span** (otelhttp) — excludes the long-lived `/mcp/` `GET`
  SSE stream.
- **Per-RPC spans** (otelconnect) — both client and server; server spans
  are children of the client span.
- **Per-query DB spans** (otelpgx) — SQL is trimmed from the span name and
  query parameters are **not** included.
- `storage.transaction`
- `drift.detect`
- **MCP spans** — `mcp.tool/<name>`, `mcp.resource/<uri>`,
  `mcp.resource_template/<uri>`, `mcp.prompt/<name>`.
- **CLI command root span** — `cli <command path>`.

### Metrics (server only)

- **RPC server metrics** (otelconnect)
- **HTTP server metrics** (otelhttp)
- **DB pool / query metrics** (otelpgx)
- Histograms: `specgraph.storage.transaction.duration`,
  `specgraph.drift.detect.duration`, `specgraph.startup.duration`
- Bounded-cardinality counters:

  | Counter | Attributes |
  |---------|------------|
  | `specgraph.edge.mutations` | `edge_type`, `op` |
  | `specgraph.stage.transitions` | `from`, `to` |
  | `specgraph.findings.stored` | `pass_type` |
  | `specgraph.spec.changes` | `stage`, `checkpoint` |

Metric attributes are deliberately bounded — there are **no slug, id, or
free-text dimensions**.

### Logs

Structured slog records, trace-correlated, optionally exported over OTLP.

## Security Notes

- **Bearer tokens and identity** are never placed in spans or trace
  baggage.
- **DB query parameter values** are not captured — only the `$1`
  placeholders appear.
- At the **public edge** only the W3C `traceparent` header is trusted and
  propagated; baggage is restricted to internal loopback hops.
