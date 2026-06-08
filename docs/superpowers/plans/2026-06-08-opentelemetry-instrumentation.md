# OpenTelemetry Instrumentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add OpenTelemetry traces, metrics, and context-aware logging to the SpecGraph CLI client and server, with W3C context propagation across the boundary, OTLP export, and a no-op-by-default master switch.

**Architecture:** One cohesive `internal/telemetry/` package owns the entire observability lifecycle via a single `Init(ctx, Config) (*Telemetry, error)` / `Shutdown(ctx) error`. Server and CLI call the same API, differing only by `Config.Role`. When `Enabled=false`, every provider is the OTel built-in no-op and all per-request wiring is skipped, so the disabled binary takes the original code path. The CLI initializes telemetry inside the root `PersistentPreRunE` (after cobra parses flags) and hands the `*Telemetry` + root span to `main()` via a single package-level holder; `main()` becomes `func main(){ os.Exit(run()) }` so deferred flush runs on every exit branch.

**Tech Stack:** Go 1.26.4, ConnectRPC (`connectrpc.com/connect` v1.19.1), `connectrpc.com/otelconnect`, `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`, `github.com/exaring/otelpgx`, `go.opentelemetry.io/contrib/exporters/autoexport`, `go.opentelemetry.io/contrib/bridges/otelslog` (beta logs), `github.com/samber/slog-multi`, pgx v5, stdlib `log/slog`, cobra.

**Design doc:** `docs/superpowers/specs/2026-06-05-opentelemetry-instrumentation-design.md` (read it before starting — this plan implements it).

---

## Conventions (apply to EVERY new file)

- First two lines of every new `.go` file:

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt
  ```

- New package `internal/telemetry` needs a `// Package telemetry ...` doc comment on the first declaration in the first file (`telemetry.go`) or `revive` fails.
- Commit messages are Conventional Commits with a DCO sign-off. Use `git commit -s -m "..."`. (Repo is jj-colocated; `git commit` works through the colocation. Do NOT `git push`.)
- After each phase, run `task check` (fmt:check → license:check → lint → build → unit tests). Fix lint before moving on. `task license:add` fixes missing headers; `task fmt` fixes formatting.
- Run a single test with: `go test ./internal/telemetry/ -run TestName -v`.

---

## Phase 1 — Dependencies, SDK lifecycle, no-op, config

**Outcome:** `internal/telemetry` compiles, `Init(Enabled=false)` returns working no-op providers + a plain logger, `Shutdown` is a no-op, and the config resolver reads flags+env. No wiring into serve/CLI yet.

### Task 1: Add and pin dependencies

**Files:**

- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the GA direct deps**

Run (one module at a time so a bad resolve is obvious):

```bash
go get connectrpc.com/otelconnect@v0.9.0
go get github.com/exaring/otelpgx@v0.11.1
go get github.com/samber/slog-multi@latest
go get go.opentelemetry.io/contrib/exporters/autoexport@latest
```

- [ ] **Step 2: Add the beta logs stack (versions must be mutually consistent)**

The `otelslog` bridge is version-locked to a specific `otel/log` minor, and that minor MUST equal the one `autoexport` pulls. Let `go get` resolve them together, then verify they share one `otel/log` version:

```bash
go get go.opentelemetry.io/contrib/bridges/otelslog@latest
go get go.opentelemetry.io/otel/log@latest
go get go.opentelemetry.io/otel/sdk/log@latest
```

- [ ] **Step 3: Tidy and verify the matrix resolves**

Run:

```bash
go mod tidy
go build ./...
```

Expected: clean build. If `go mod tidy` cannot resolve a GCP-compatible matrix (conflict against `opentelemetry-operations-go/exporter/metric v0.55.0` or `contrib/detectors/gcp v1.42.0`), STOP and fall back to enrichment-only logs (skip `autoexport`'s log exporter + `sdk/log` + `otelslog`); record the decision in the commit message and proceed — Phase 3's OTLP-log-export step becomes a no-op behind `--otel-logs`.

- [ ] **Step 4: Confirm the expected version bumps happened**

Run:

```bash
go list -m github.com/jackc/pgx/v5 go.opentelemetry.io/otel
```

Expected: `pgx/v5` is now `v5.9.2` (bumped from v5.9.1 by otelpgx v0.11.1); `otel` is `v1.43.0`. The otel core modules (`otel`, `otel/sdk`, `otel/sdk/metric`, `otel/trace`, `otel/metric`, `otelhttp`) should now be direct (no `// indirect`).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -s -m "build(deps): add OpenTelemetry, otelconnect, otelpgx, slog-multi"
```

### Task 2: Telemetry package skeleton (Config, Telemetry struct)

**Files:**

- Create: `internal/telemetry/telemetry.go`

- [ ] **Step 1: Write the skeleton with the public seam**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package telemetry owns SpecGraph's OpenTelemetry lifecycle: trace, metric,
// and log providers plus a context-enriching slog handler. A single
// Init(ctx, Config) wires every signal; Shutdown(ctx) flushes them. When
// Config.Enabled is false every provider is the OTel built-in no-op and the
// returned logger is the plain base handler, so the disabled binary keeps the
// original hot path.
package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Role selects which signals Init starts and the service.name resource attr.
type Role string

const (
	// RoleServer enables traces + metrics + logs.
	RoleServer Role = "server"
	// RoleCLI enables traces + propagation + logs only (no MeterProvider).
	RoleCLI Role = "cli"
)

// Config controls Init. Construct it from ResolveConfig (config.go).
type Config struct {
	Enabled     bool         // master switch; false => all no-op
	ServiceName string       // OTEL_SERVICE_NAME fallback
	Role        Role         // "server" | "cli"
	SampleRatio float64      // trace sampler arg (default 1.0)
	LogsExport  bool         // OTLP log export toggle (traces/metrics unaffected)
	LogHandler  slog.Handler // base handler to fan into; MUST be non-nil
	Version     string       // service.version (from buildVersion())

	// Context accessors injected by cmd/specgraph to avoid importing
	// internal/server and internal/auth (whose context keys are unexported).
	// All may be nil; a nil accessor contributes no attribute.
	ProjectFromContext  func(context.Context) (string, bool)
	IdentityFromContext func(context.Context) (string, bool)
}

// Telemetry is the handle returned by Init. Logger is always non-nil.
type Telemetry struct {
	Logger *slog.Logger
	Tracer trace.Tracer
	Meter  metric.Meter

	// shutdownFuncs are called in reverse order by Shutdown. Empty when no-op.
	shutdownFuncs []func(context.Context) error
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./internal/telemetry/`
Expected: success (unused imports `metric`/`trace` are referenced in the struct, so no error).

- [ ] **Step 3: Commit**

```bash
git add internal/telemetry/telemetry.go
git commit -s -m "feat(telemetry): add package skeleton and public Config seam"
```

### Task 3: Config resolver (flag + env, standalone)

**Files:**

- Create: `internal/telemetry/config.go`
- Test: `internal/telemetry/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"testing"

	"github.com/spf13/pflag"
)

func newFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	return fs
}

func TestResolveConfig_DefaultsOff(t *testing.T) {
	fs := newFlagSet()
	got := ResolveConfig(fs, func(string) string { return "" })
	if got.Enabled {
		t.Fatalf("Enabled = true, want false by default")
	}
	if got.SampleRatio != 1.0 {
		t.Fatalf("SampleRatio = %v, want 1.0", got.SampleRatio)
	}
	if !got.LogsExport {
		t.Fatalf("LogsExport = false, want true default")
	}
}

func TestResolveConfig_FlagBeatsEnv(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse([]string{"--otel", "--otel-sample-ratio", "0.25"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"SPECGRAPH_OTEL_ENABLED":      "false",
		"SPECGRAPH_OTEL_SAMPLE_RATIO": "0.9",
	}
	got := ResolveConfig(fs, func(k string) string { return env[k] })
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true (flag set beats env)")
	}
	if got.SampleRatio != 0.25 {
		t.Fatalf("SampleRatio = %v, want 0.25 (flag wins)", got.SampleRatio)
	}
}

func TestResolveConfig_EnvWhenFlagUnset(t *testing.T) {
	fs := newFlagSet()
	env := map[string]string{
		"SPECGRAPH_OTEL_ENABLED": "true",
		"SPECGRAPH_OTEL_LOGS":    "false",
	}
	got := ResolveConfig(fs, func(k string) string { return env[k] })
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true from env")
	}
	if got.LogsExport {
		t.Fatalf("LogsExport = true, want false from env")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -run TestResolveConfig -v`
Expected: FAIL — `RegisterFlags`/`ResolveConfig` undefined.

- [ ] **Step 3: Write the resolver**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"strconv"

	"github.com/spf13/pflag"
)

// Flag names registered on the root persistent flag set.
const (
	flagEnabled     = "otel"
	flagSampleRatio = "otel-sample-ratio"
	flagLogs        = "otel-logs"
)

// Env names checked when the corresponding flag is not explicitly set.
const (
	envEnabled     = "SPECGRAPH_OTEL_ENABLED"
	envSampleRatio = "SPECGRAPH_OTEL_SAMPLE_RATIO"
	envLogs        = "SPECGRAPH_OTEL_LOGS"
)

// RegisterFlags adds the three SpecGraph-native telemetry knobs to fs.
// Call on rootCmd.PersistentFlags() so every command accepts them.
func RegisterFlags(fs *pflag.FlagSet) {
	fs.Bool(flagEnabled, false, "Enable OpenTelemetry export (env: "+envEnabled+")")
	fs.Float64(flagSampleRatio, 1.0, "Trace sample ratio 0.0-1.0 (env: "+envSampleRatio+")")
	fs.Bool(flagLogs, true, "Export logs over OTLP when telemetry is enabled (env: "+envLogs+")")
}

// ResolveConfig builds a partial Config from flags + env with precedence
// flag(if Changed) > env > default. getenv is injected for testability
// (pass os.Getenv in production). Role, LogHandler, Version, ServiceName,
// and the context accessors are filled in by the caller.
func ResolveConfig(fs *pflag.FlagSet, getenv func(string) string) Config {
	return Config{
		Enabled:     resolveBool(fs, flagEnabled, envEnabled, getenv, false),
		SampleRatio: resolveFloat(fs, flagSampleRatio, envSampleRatio, getenv, 1.0),
		LogsExport:  resolveBool(fs, flagLogs, envLogs, getenv, true),
	}
}

func resolveBool(fs *pflag.FlagSet, flag, env string, getenv func(string) string, def bool) bool {
	if fs.Changed(flag) {
		v, _ := fs.GetBool(flag)
		return v
	}
	if s := getenv(env); s != "" {
		if v, err := strconv.ParseBool(s); err == nil {
			return v
		}
	}
	return def
}

func resolveFloat(fs *pflag.FlagSet, flag, env string, getenv func(string) string, def float64) float64 {
	if fs.Changed(flag) {
		v, _ := fs.GetFloat64(flag)
		return v
	}
	if s := getenv(env); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
	}
	return def
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/telemetry/ -run TestResolveConfig -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/config.go internal/telemetry/config_test.go
git commit -s -m "feat(telemetry): add standalone flag+env config resolver"
```

### Task 4: Providers and resource builders

**Files:**

- Create: `internal/telemetry/providers.go`

> **Beta-API note:** the `sdk/log` and `otelslog` symbols below are from beta modules. If a symbol name differs in the resolved version, adjust to the equivalent constructor — the shape (NewLoggerProvider + batch processor + exporter) is stable. Do NOT invent fields; check the godoc of the pinned version.

- [ ] **Step 1: Write the builders**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// buildResource merges SDK detectors (env/process/host) with explicit
// service.name/version. service.name is "specgraph-server" or "specgraph-cli".
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	name := "specgraph-" + string(cfg.Role)
	if cfg.ServiceName != "" {
		name = cfg.ServiceName
	}
	r, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(name),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}
	return r, nil
}

// buildTracerProvider returns a TracerProvider. CLI role uses a synchronous
// span processor (short-lived process); server role uses a batch processor.
func buildTracerProvider(ctx context.Context, cfg Config, r *resource.Resource) (*sdktrace.TracerProvider, error) {
	exp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: span exporter: %w", err)
	}
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))
	var proc sdktrace.SpanProcessor
	if cfg.Role == RoleCLI {
		proc = sdktrace.NewSimpleSpanProcessor(exp)
	} else {
		proc = sdktrace.NewBatchSpanProcessor(exp)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sampler),
		sdktrace.WithSpanProcessor(proc),
	), nil
}

// buildMeterProvider returns a MeterProvider with a periodic reader.
func buildMeterProvider(ctx context.Context, r *resource.Resource) (*sdkmetric.MeterProvider, error) {
	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: metric reader: %w", err)
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(r),
		sdkmetric.WithReader(reader),
	), nil
}

// buildLoggerProvider returns a LoggerProvider with a batch processor, or
// nil if log export is disabled.
func buildLoggerProvider(ctx context.Context, cfg Config, r *resource.Resource) (*sdklog.LoggerProvider, error) {
	if !cfg.LogsExport {
		return nil, nil
	}
	exp, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: log exporter: %w", err)
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(r),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	), nil
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `go build ./internal/telemetry/`
Expected: success. If `autoexport.NewLogExporter` or `sdklog` symbols differ in the resolved beta version, fix the names per godoc (`go doc go.opentelemetry.io/otel/sdk/log`).

- [ ] **Step 3: Commit**

```bash
git add internal/telemetry/providers.go
git commit -s -m "feat(telemetry): add resource/tracer/meter/logger provider builders"
```

### Task 5: Init / Shutdown with no-op path

**Files:**

- Modify: `internal/telemetry/telemetry.go`
- Create: `internal/telemetry/telemetry_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestInit_DisabledIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	tel, err := Init(context.Background(), Config{
		Enabled:    false,
		Role:       RoleCLI,
		LogHandler: base,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if tel.Logger == nil {
		t.Fatal("Logger is nil")
	}
	// Global TracerProvider must remain the no-op (Init did not set one).
	if _, ok := otel.GetTracerProvider().(noop.TracerProvider); !ok {
		t.Fatalf("disabled Init set a non-noop global TracerProvider: %T", otel.GetTracerProvider())
	}
	if err := tel.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestInit_NilLogHandlerRejected(t *testing.T) {
	_, err := Init(context.Background(), Config{Enabled: false, Role: RoleCLI, LogHandler: nil})
	if err == nil {
		t.Fatal("expected error for nil LogHandler")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -run TestInit -v`
Expected: FAIL — `Init`/`Shutdown` undefined.

- [ ] **Step 3: Implement Init and Shutdown**

Append to `internal/telemetry/telemetry.go`:

```go
import (
	// add to the existing import block:
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Init wires the configured providers. On Enabled=false it returns working
// no-op providers and a plain (cfg.LogHandler) logger; Shutdown is a no-op.
// Init never returns a fatal error for telemetry setup beyond a nil
// LogHandler — provider build failures fall back to no-op (see callers).
func Init(ctx context.Context, cfg Config) (*Telemetry, error) {
	if cfg.LogHandler == nil {
		return nil, errors.New("telemetry: Config.LogHandler must be non-nil")
	}
	if !cfg.Enabled {
		return &Telemetry{
			Logger: slog.New(cfg.LogHandler),
			Tracer: tracenoop.NewTracerProvider().Tracer("specgraph"),
			Meter:  metricnoop.NewMeterProvider().Meter("specgraph"),
		}, nil
	}

	tel := &Telemetry{}
	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tp, err := buildTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tp)
	tel.Tracer = tp.Tracer("specgraph")
	tel.shutdownFuncs = append(tel.shutdownFuncs, tp.Shutdown)

	if cfg.Role == RoleServer {
		mp, mErr := buildMeterProvider(ctx, res)
		if mErr != nil {
			return nil, mErr
		}
		otel.SetMeterProvider(mp)
		tel.Meter = mp.Meter("specgraph")
		tel.shutdownFuncs = append(tel.shutdownFuncs, mp.Shutdown)
	} else {
		tel.Meter = metricnoop.NewMeterProvider().Meter("specgraph")
	}

	lp, err := buildLoggerProvider(ctx, cfg, res)
	if err != nil {
		return nil, err
	}
	if lp != nil {
		tel.shutdownFuncs = append(tel.shutdownFuncs, lp.Shutdown)
	}

	// Global propagator is TraceContext only (safe at the untrusted edge).
	// Baggage is applied per-transport on internal/loopback hops, not here.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Logger is wired in Phase 3 (enrichHandler + fanout over lp). For now
	// the base handler is used so Phase 1/2 have a working logger.
	tel.Logger = slog.New(cfg.LogHandler)

	return tel, nil
}

// Shutdown flushes and shuts down each provider in reverse registration
// order, aggregating errors. Idempotent: a second call is a no-op. Bound the
// caller's ctx with a timeout (the caller owns the budget).
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(t.shutdownFuncs) - 1; i >= 0; i-- {
		if err := t.shutdownFuncs[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	t.shutdownFuncs = nil
	if len(errs) > 0 {
		return fmt.Errorf("telemetry: shutdown: %w", errors.Join(errs...))
	}
	return nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/telemetry/ -run TestInit -v`
Expected: PASS (both).

- [ ] **Step 5: Run the full package + lint**

Run: `go test ./internal/telemetry/ && task lint`
Expected: PASS. Fix `wrapcheck`/`govet -shadow` hits per AGENTS.md (the `Init` provider-build returns are already wrapped with `telemetry:` prefixes; reused `err` is fine here because each is checked immediately).

- [ ] **Step 6: Commit**

```bash
git add internal/telemetry/telemetry.go internal/telemetry/telemetry_test.go
git commit -s -m "feat(telemetry): add Init/Shutdown with no-op default path"
```

**Phase 1 gate:** `task check` passes; `Init(Enabled=false)` test green; clean build with the full dependency matrix.

---

## Phase 2 — Traces and propagation

**Outcome:** Client→server RPCs produce one connected trace through otelconnect (both interceptors), otelhttp (server edge), otelpgx (DB), and the MCP loopback. The CLI initializes telemetry in `nudgePreRun`, hands the handle to `main()` via a package holder, and flushes on every exit branch.

### Task 6: otelconnect server interceptor (gated)

**Files:**

- Create: `internal/telemetry/connect.go`
- Test: `internal/telemetry/connect_test.go`

- [ ] **Step 1: Write the interceptor helpers**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"fmt"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
)

// loopbackPropagator carries traceparent AND baggage; used only on internal
// hops (server↔server loopback), never at the public edge.
func loopbackPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// ServerInterceptor returns the otelconnect server interceptor, or nil when
// telemetry is disabled (callers must skip nil; connect.WithInterceptors
// tolerates none but we gate at the call site for zero overhead).
// NOTE: verify the exact otelconnect constructor/option names against the
// pinned v0.9.0 godoc at impl time (otelconnect.NewInterceptor).
func ServerInterceptor(enabled bool) (connect.Interceptor, error) {
	if !enabled {
		return nil, nil
	}
	ic, err := otelconnect.NewInterceptor()
	if err != nil {
		return nil, fmt.Errorf("telemetry: otelconnect server interceptor: %w", err)
	}
	return ic, nil
}

// ClientInterceptor returns the otelconnect client interceptor (uses the
// GLOBAL TraceContext-only propagator — injects traceparent only). nil when
// disabled.
func ClientInterceptor(enabled bool) (connect.Interceptor, error) {
	if !enabled {
		return nil, nil
	}
	ic, err := otelconnect.NewInterceptor()
	if err != nil {
		return nil, fmt.Errorf("telemetry: otelconnect client interceptor: %w", err)
	}
	return ic, nil
}

// LoopbackTransport wraps base with otelhttp using the loopback propagator
// (traceparent + baggage). Returns base unchanged when disabled.
// NOTE: verify otelhttp.WithPropagators (plural) against v0.67.0 godoc.
func LoopbackTransport(enabled bool, base http.RoundTripper) http.RoundTripper {
	if !enabled {
		return base
	}
	return otelhttp.NewTransport(base, otelhttp.WithPropagators(loopbackPropagator()))
}
```

- [ ] **Step 2: Write the propagation round-trip test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"testing"
)

func TestServerInterceptor_DisabledReturnsNil(t *testing.T) {
	ic, err := ServerInterceptor(false)
	if err != nil {
		t.Fatal(err)
	}
	if ic != nil {
		t.Fatal("disabled ServerInterceptor should be nil")
	}
}

func TestServerInterceptor_EnabledNonNil(t *testing.T) {
	ic, err := ServerInterceptor(true)
	if err != nil {
		t.Fatal(err)
	}
	if ic == nil {
		t.Fatal("enabled ServerInterceptor should be non-nil")
	}
}
```

(The full client→server connected-trace assertion lives in Task 12's integration test, after both interceptors are wired and a `tracetest.SpanRecorder` is installed.)

- [ ] **Step 3: Run to verify**

Run: `go test ./internal/telemetry/ -run TestServerInterceptor -v`
Expected: PASS. Fix the otelconnect constructor name if `go build` reports it differs from `NewInterceptor`.

- [ ] **Step 4: Commit**

```bash
git add internal/telemetry/connect.go internal/telemetry/connect_test.go
git commit -s -m "feat(telemetry): add gated otelconnect interceptors + loopback transport"
```

### Task 7: Wire otelconnect client interceptor into CLI clients (gated)

**Files:**

- Modify: `cmd/specgraph/client.go:133-154` (the `newClient`/`newClientWithProject` ctors)
- Modify: `cmd/specgraph/main.go` (add a package var for the resolved enabled flag — set in Task 8)

- [ ] **Step 1: Add a package-level telemetry-enabled accessor**

In `cmd/specgraph/main.go`, add near the `cfgFile` var (line 41):

```go
// telState holds the process-wide telemetry handle and root span, set once
// in nudgePreRun (after cobra parses flags) and read by run()'s defer and by
// the client ctors. Single-threaded CLI: PreRun → ExecuteContext → run-defer
// is strictly sequential on one goroutine, so plain vars are race-free.
var telState struct {
	tel      *telemetry.Telemetry
	rootSpan trace.Span
	enabled  bool
}
```

Add imports `"github.com/specgraph/specgraph/internal/telemetry"` and `"go.opentelemetry.io/otel/trace"` to `main.go`.

- [ ] **Step 2: Thread the client interceptor through the ctors**

Edit `cmd/specgraph/client.go`. Add imports `"connectrpc.com/connect"` (already present) and `"github.com/specgraph/specgraph/internal/telemetry"`. Replace the two `ctor(...)` calls:

```go
// newClient creates a ConnectRPC client using the configured base URL.
func newClient[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C) (C, error) {
	baseURL, project, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	return ctor(newAuthenticatedHTTPClient(baseURL, project), baseURL, clientOpts()...), nil
}

// newClientWithProject creates a ConnectRPC client using an explicit project slug.
func newClientWithProject[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C, project string) (C, error) {
	baseURL, derivedProject, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	if project != "" {
		derivedProject = project
	}
	return ctor(newAuthenticatedHTTPClient(baseURL, derivedProject), baseURL, clientOpts()...), nil
}

// clientOpts returns the connect client options, including the otelconnect
// interceptor when telemetry is enabled. With no MeterProvider on the CLI,
// the interceptor's metric instruments resolve against the global no-op
// meter — they record nothing and never error.
func clientOpts() []connect.ClientOption {
	ic, err := telemetry.ClientInterceptor(telState.enabled)
	if err != nil || ic == nil {
		return nil
	}
	return []connect.ClientOption{connect.WithInterceptors(ic)}
}
```

- [ ] **Step 3: Build**

Run: `go build ./cmd/specgraph/`
Expected: success (telState.enabled is false until Task 8 sets it; clients still construct).

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/client.go cmd/specgraph/main.go
git commit -s -m "feat(cli): wire gated otelconnect client interceptor into RPC ctors"
```

### Task 8: CLI bootstrap — holder, nudgePreRun Init, run() refactor

**Files:**

- Modify: `cmd/specgraph/main.go:43-50` (register flags), `cmd/specgraph/main.go:80-89` (`main`→`run`)
- Modify: `cmd/specgraph/nudge.go:49` (`nudgePreRun` — prepend telemetry init)

- [ ] **Step 1: Register the persistent telemetry flags**

In `main.go` `init()` (after the `--config` flag at line 46), add:

```go
	telemetry.RegisterFlags(rootCmd.PersistentFlags())
```

- [ ] **Step 2: Initialize telemetry at the top of nudgePreRun**

In `cmd/specgraph/nudge.go`, prepend to the body of `nudgePreRun` (before the existing "// 1. Subcommand allow-list" block at line 50):

```go
	// Telemetry bootstrap: flags are parsed by now (cobra parses before
	// PersistentPreRunE) and the command/role is known. Init once here; the
	// handle + root span are stashed in telState for run()'s deferred flush.
	initTelemetry(cmd)
```

Then add this function to `nudge.go` (below `nudgePreRun`):

```go
// initTelemetry resolves telemetry config from the parsed persistent flags +
// env, initializes the providers, starts a root command span, and stashes the
// result in telState. Best-effort: any failure logs a warning and continues.
func initTelemetry(cmd *cobra.Command) {
	cfg := telemetry.ResolveConfig(rootCmd.PersistentFlags(), os.Getenv)
	cfg.Role = telemetry.RoleCLI
	cfg.Version = buildVersion()
	// CLI base handler writes to STDERR so --json output on stdout stays clean.
	cfg.LogHandler = slog.NewJSONHandler(os.Stderr, nil)
	cfg.ProjectFromContext = telemetryProjectAccessor
	cfg.IdentityFromContext = telemetryIdentityAccessor

	tel, err := telemetry.Init(cmd.Context(), cfg)
	if err != nil {
		// Fall back to a plain stderr logger; never block the CLI.
		fmt.Fprintln(os.Stderr, "warning: telemetry init failed:", err)
		return
	}
	telState.tel = tel
	telState.enabled = cfg.Enabled
	slog.SetDefault(tel.Logger)

	ctx, span := tel.Tracer.Start(cmd.Context(), "cli "+cmd.CommandPath())
	telState.rootSpan = span
	cmd.SetContext(ctx) // children (RPC spans) parent off the root span
}
```

Add imports to `nudge.go`: `"log/slog"`, `"github.com/specgraph/specgraph/internal/telemetry"`. (`fmt`, `os`, `cobra` already imported.)

- [ ] **Step 3: Add the accessor closures (defined fully in Task 13; stubs now)**

For Phase 2 the accessors can be nil-returning stubs (enrichment lands in Phase 3). Add to `nudge.go`:

```go
// telemetryProjectAccessor / telemetryIdentityAccessor bridge server/auth
// context keys into telemetry without telemetry importing those packages.
// Phase 3 fills these in; Phase 2 uses no-op stubs.
func telemetryProjectAccessor(ctx context.Context) (string, bool)  { return "", false }
func telemetryIdentityAccessor(ctx context.Context) (string, bool) { return "", false }
```

Add import `"context"` to `nudge.go`.

- [ ] **Step 4: Refactor main() → run() with deferred flush**

Replace `main.go:80-89`:

```go
func main() {
	os.Exit(run())
}

// run executes the root command and returns the process exit code. Telemetry
// is initialized in nudgePreRun (after flag parse); here we only ensure the
// root span is ended and providers are flushed on EVERY exit branch — only
// ended spans export, so End must precede Shutdown or failure traces are lost.
func run() int {
	ctx := context.Background()
	defer func() {
		if telState.rootSpan != nil {
			telState.rootSpan.End()
		}
		if telState.tel != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := telState.tel.Shutdown(shutdownCtx); err != nil {
				fmt.Fprintln(os.Stderr, "warning: telemetry shutdown:", err)
			}
		}
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
```

Add imports to `main.go`: `"context"`, `"time"`.

- [ ] **Step 5: Build and smoke-test disabled path**

Run:

```bash
go build -o /tmp/specgraph ./cmd/specgraph/ && /tmp/specgraph version
```

Expected: prints version, exit 0, NO telemetry output (disabled by default).

- [ ] **Step 6: Verify existing CLI tests still pass**

Run: `go test ./cmd/specgraph/ -run TestNudge -v`
Expected: PASS — the nudge tests stub `nudgeIsTerminal`; telemetry init with `Enabled=false` is a cheap no-op that doesn't change nudge behavior.

- [ ] **Step 7: Commit**

```bash
git add cmd/specgraph/main.go cmd/specgraph/nudge.go
git commit -s -m "feat(cli): bootstrap telemetry in nudgePreRun with run()-deferred flush"
```

### Task 9: serve consumes the shared handle (no re-Init) + server interceptor + otelhttp

**Files:**

- Modify: `cmd/specgraph/serve.go:79-90` (logger), `:261` (interceptor opts), `:332` (handler wrap), `:360-393` (shutdown)

- [ ] **Step 1: Re-Init telemetry with server role + metrics inside runServe**

The CLI bootstrap Init'd with `Role=cli` (no MeterProvider). For `serve` we need the server role (metrics + stdout logger). Because `serve` is on the drift-nudge allow-list, `nudgePreRun` returns early BEFORE its `initTelemetry` call only if that call is placed after the allow-list check — but Task 8 placed `initTelemetry` FIRST, so it runs for serve too with `Role=cli`. Fix: make `initTelemetry` role-aware.

Edit `nudge.go` `initTelemetry` to choose role from the top-level command:

```go
	cfg.Role = telemetry.RoleCLI
	top := cmd
	for top.HasParent() && top.Parent() != rootCmd {
		top = top.Parent()
	}
	if top.Name() == "serve" {
		cfg.Role = telemetry.RoleServer
		// Server logs go to the configured stream (default stdout), not the
		// CLI stderr JSON handler. serve overrides slog.SetDefault with its
		// own cfg.Log.Build() handler below; we hand telemetry that handler
		// via the holder so enrichment wraps it (Phase 3). For now use stdout.
		cfg.LogHandler = slog.NewJSONHandler(os.Stdout, nil)
	}
```

- [ ] **Step 2: Consume the shared handle in runServe; do NOT call telemetry.Init again**

In `serve.go runServe`, after `cfg.Log.Build()` (line 79-83), replace the logger wiring so the enriched logger (Phase 3) is preferred but for Phase 2 we keep serve's logger and reuse the holder's providers:

```go
	logger, err := cfg.Log.Build()
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	slog.SetDefault(logger)
	// telState.tel was initialized in nudgePreRun with Role=server. serve
	// consumes it (no second telemetry.Init — a re-Init would overwrite the
	// OTel globals and double-shutdown). Phase 3 swaps slog.SetDefault to the
	// enriched logger.
	tel := telState.tel
```

- [ ] **Step 3: Add the server interceptor outermost (before auth)**

Replace `serve.go:261`:

```go
	maxBytes := connect.WithReadMaxBytes(4 << 20) // 4 MiB request body limit
	interceptors := []connect.Interceptor{}
	if otelIC, otelErr := telemetry.ServerInterceptor(telState.enabled); otelErr != nil {
		return fmt.Errorf("otel interceptor: %w", otelErr)
	} else if otelIC != nil {
		interceptors = append(interceptors, otelIC) // outermost: before auth
	}
	interceptors = append(interceptors, interceptor) // existing auth interceptor
	opts := connect.WithInterceptors(interceptors...)
```

Add import `"github.com/specgraph/specgraph/internal/telemetry"` to `serve.go`.

- [ ] **Step 4: Wrap the outer handler with otelhttp (excluding probes + static + /mcp/ histogram)**

Replace `serve.go:332`:

```go
	handler := server.SecurityHeaders(server.ProjectMiddleware(mux))
	if telState.enabled {
		handler = telemetry.WrapHTTPHandler(handler)
	}
```

Add to `internal/telemetry/connect.go`:

```go
// WrapHTTPHandler wraps h with otelhttp for the server edge. Static assets
// and the long-lived /mcp/ GET stream are filtered out of span creation to
// avoid per-asset noise and never-closing SSE spans.
func WrapHTTPHandler(h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, "specgraph.http",
		otelhttp.WithFilter(func(r *http.Request) bool {
			// Skip static web assets (everything not under an API/MCP path).
			p := r.URL.Path
			if strings.HasPrefix(p, "/mcp/") && r.Method == http.MethodGet {
				return false // long-lived SSE notification stream
			}
			return true
		}),
	)
}
```

Add imports `"strings"` to `connect.go`.

- [ ] **Step 5: Wire telemetry shutdown into the server graceful-shutdown path**

In the shutdown goroutine (`serve.go:392`, after `wg.Wait()`), add:

```go
		wg.Wait()
		if tel != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tel.Shutdown(shutdownCtx); err != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "telemetry shutdown", slog.Any("error", err))
			}
		}
```

(The `run()` defer's second `Shutdown` is then an idempotent no-op.)

- [ ] **Step 6: Build + existing serve tests**

Run: `go build ./cmd/specgraph/ && go test ./cmd/specgraph/ -run TestValidateServerConfig -v`
Expected: build success; existing serve validation test passes.

- [ ] **Step 7: Commit**

```bash
git add cmd/specgraph/serve.go cmd/specgraph/nudge.go internal/telemetry/connect.go
git commit -s -m "feat(server): consume shared telemetry handle, add server interceptor + otelhttp"
```

### Task 10: otelpgx pool refactor

**Files:**

- Modify: `internal/storage/postgres/postgres.go:64-89` (`New`)
- Modify: `cmd/specgraph/serve.go:127` (pass enabled flag into postgres.New)

- [ ] **Step 1: Add a telemetry option to postgres.New**

In `postgres.go`, add an option (near `WithProject` at line 50):

```go
// WithTracing enables the otelpgx query tracer on the pool. Off by default.
func WithTracing(enabled bool) Option {
	return func(s *Store) { s.tracingEnabled = enabled }
}
```

Add the `tracingEnabled bool` field to the `Store` struct (find the struct definition; add alongside `project string`).

- [ ] **Step 2: Refactor New to use ParseConfig + NewWithConfig**

Replace `postgres.go:74-77`:

```go
	poolCfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	if s.tracingEnabled {
		poolCfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
```

Add import `"github.com/exaring/otelpgx"` to `postgres.go`.

- [ ] **Step 3: Pass the flag from serve**

In `serve.go:127`, change:

```go
	s, pgErr := postgres.New(ctx, connURL, postgres.WithProject("_server"), postgres.WithTracing(telState.enabled))
```

- [ ] **Step 4: Build + postgres unit tests (non-integration)**

Run: `go build ./... && go test ./internal/storage/postgres/ -short`
Expected: success. (otelpgx behavior is validated by the integration suite under `task pr-prep`; note this in the PR.)

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/postgres.go cmd/specgraph/serve.go
git commit -s -m "feat(storage): add gated otelpgx query tracer to the pool"
```

### Task 11: MCP loopback transport wrap + four wrap*Handler spans

**Files:**

- Modify: `cmd/specgraph/serve.go:298` (loopback client transport)
- Modify: `internal/mcp/server.go:139-183` (four wrap adapters)

- [ ] **Step 1: Wrap the loopback HTTP client with the baggage-carrying transport**

`newHTTPClient("")` returns a `*http.Client` with a `clientTransport` base (`client.go:112-117`). Wrap its transport. Add a CLI helper in `serve.go` (or inline):

```go
	loopbackClient := newHTTPClient("")
	loopbackClient.Transport = telemetry.LoopbackTransport(telState.enabled, loopbackClient.Transport)
	mcpClient := mcppkg.NewClient(loopbackClient, selfBaseURL(cfg.Server.Listen))
```

(Replaces `serve.go:298`.)

- [ ] **Step 2: Add span wrapping to the four MCP adapters**

In `internal/mcp/server.go`, wrap each adapter body in a span. Add a small helper at the top of the file:

```go
// mcpSpan starts a span named kind+"/"+name and returns the child ctx + an
// end func that records the outcome. Uses the global tracer (no-op when
// telemetry disabled), so this is always safe to call.
func mcpSpan(ctx context.Context, kind, name string) (context.Context, func(err error)) {
	ctx, span := otel.Tracer("specgraph.mcp").Start(ctx, "mcp."+kind+"/"+name)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}
```

Add imports `"go.opentelemetry.io/otel"` and `"go.opentelemetry.io/otel/codes"` to `server.go`.

Then edit each adapter. `wrapToolHandler` (line 140):

```go
func wrapToolHandler(h ToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		ctx, end := mcpSpan(ctx, "tool", req.Params.Name)
		params := fromSDKParams(&req)
		result, err := h(ctx, params)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResult(result), nil
	}
}
```

`wrapResourceHandler` (line 152) and `wrapResourceTemplateHandler` (line 164) — use `req.Params.URI` as the name and kind `"resource"` / `"resource_template"`:

```go
func wrapResourceHandler(h ResourceHandler) server.ResourceHandlerFunc {
	return func(ctx context.Context, req sdkmcp.ReadResourceRequest) ([]sdkmcp.ResourceContents, error) {
		ctx, end := mcpSpan(ctx, "resource", req.Params.URI)
		contents, err := h(ctx, req.Params.URI)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResourceContents(contents), nil
	}
}

func wrapResourceTemplateHandler(h ResourceHandler) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, req sdkmcp.ReadResourceRequest) ([]sdkmcp.ResourceContents, error) {
		ctx, end := mcpSpan(ctx, "resource_template", req.Params.URI)
		contents, err := h(ctx, req.Params.URI)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResourceContents(contents), nil
	}
}
```

`wrapPromptHandler` (line 175) — name `req.Params.Name`, kind `"prompt"`:

```go
func wrapPromptHandler(h PromptHandler) server.PromptHandlerFunc {
	return func(ctx context.Context, req sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		ctx, end := mcpSpan(ctx, "prompt", req.Params.Name)
		result, err := h(ctx, req.Params.Arguments)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKPromptResult(result), nil
	}
}
```

- [ ] **Step 3: Build + existing mcp tests**

Run: `go build ./... && go test ./internal/mcp/ -v`
Expected: success; existing MCP tests unaffected (spans are no-op under the default global tracer).

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go internal/mcp/server.go
git commit -s -m "feat(mcp): add loopback trace propagation + per-operation spans"
```

### Task 12: Propagation round-trip integration test

**Files:**

- Create: `internal/telemetry/propagation_test.go`

- [ ] **Step 1: Write the connected-trace test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestPropagationRoundTrip asserts that a traceparent injected by the client
// interceptor is extracted by the server interceptor so the server span's
// parent is the client span (one connected trace). Uses an in-memory
// SpanRecorder and a real httptest server fronting a trivial Connect handler.
func TestPropagationRoundTrip(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prop := propagation.TraceContext{}

	serverIC, err := otelconnect.NewInterceptor(
		otelconnect.WithTracerProvider(tp),
		otelconnect.WithPropagator(prop),
	)
	if err != nil {
		t.Fatal(err)
	}
	// Build a minimal Connect handler with the server interceptor, fire a
	// client call through the client interceptor, then assert parent linkage.
	// Wire the generated ServerService.Health RPC (the simplest — no project
	// header required): specgraphv1connect.NewServerServiceHandler(stub,
	// connect.WithInterceptors(serverIC)) where stub embeds
	// specgraphv1connect.UnimplementedServerServiceHandler and overrides
	// Health to return an empty *connect.Response[v1.HealthResponse]. Call it
	// via specgraphv1connect.NewServerServiceClient(httpClient, srv.URL,
	// connect.WithInterceptors(clientIC)).Health(ctx, connect.NewRequest(&v1.HealthRequest{})).
	_ = serverIC
	_ = httptest.NewServer
	_ = http.MethodGet
	_ = context.Background
	_ = connect.WithInterceptors

	// After the round trip:
	spans := rec.Ended()
	if len(spans) < 2 {
		t.Fatalf("expected >=2 spans (client+server), got %d", len(spans))
	}
	// Find the server span and assert its parent TraceID == client span TraceID.
	// (Concrete assertion completed once the ServerService.Health handler is wired.)
}
```

> **Implementer note:** complete the handler/client wiring using `gen/specgraph/v1/specgraphv1connect` — the `ServerService.Health` RPC (`/specgraph.v1.ServerService/Health`, request `*v1.HealthRequest`) is the simplest, requiring no project header. Embed `specgraphv1connect.UnimplementedServerServiceHandler` in your stub and override only `Health`. The assertion to finish: locate the server-kind span and the client-kind span via `span.SpanKind()`, then assert `serverSpan.Parent().SpanID() == clientSpan.SpanContext().SpanID()` and equal `TraceID()`. The `otelconnect.WithTracerProvider`/`WithPropagator` option names must be verified against v0.9.0.
>
> **TDD note:** the skeleton above (no wiring) compiles but produces 0 spans, so it FAILS `len(spans) < 2` — that is the intended red step. Run it once to confirm the failure, then add the handler/client wiring to make it pass.

- [ ] **Step 2: Finish wiring + run**

Run: `go test ./internal/telemetry/ -run TestPropagationRoundTrip -v`
Expected: PASS — one connected trace, server span parented to client span.

- [ ] **Step 3: Commit**

```bash
git add internal/telemetry/propagation_test.go
git commit -s -m "test(telemetry): assert client→server trace propagation round-trip"
```

**Phase 2 gate:** propagation round-trip test green; `task check` passes; disabled binary takes the original code path (no interceptors/otelhttp/otelpgx wired when `--otel` is off).

---

## Phase 3 — Context-aware logging

**Outcome:** Every existing ctx-passing `slog` call gains `trace_id`/`span_id`/`project`/`identity` with no call-site changes, and (when `--otel-logs`) logs export over OTLP via the otelslog bridge. CLI logs go to stderr; server logs to its configured stream.

### Task 13: Context accessor closures (real implementations)

**Files:**

- Modify: `cmd/specgraph/nudge.go` (replace the Task 8 stubs)

- [ ] **Step 1: Implement the accessors over both project keys + identity**

Replace the stubs from Task 8 Step 3:

```go
// telemetryProjectAccessor reads the project slug from EITHER context key:
// server.ProjectFromContext (request path) or auth.ProjectFromContext (MCP
// path). Reading only one drops `project` on MCP-path logs.
func telemetryProjectAccessor(ctx context.Context) (string, bool) {
	if slug := server.ProjectFromContext(ctx); slug != "" {
		return slug, true
	}
	if slug, ok := auth.ProjectFromContext(ctx); ok && slug != "" {
		return slug, true
	}
	return "", false
}

// telemetryIdentityAccessor reads the authenticated identity, if any.
func telemetryIdentityAccessor(ctx context.Context) (string, bool) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok || id == nil {
		return "", false
	}
	return id.Subject, true // verify the human-readable field on *auth.Identity
}
```

Add imports `"github.com/specgraph/specgraph/internal/server"` and `"github.com/specgraph/specgraph/internal/auth"` to `nudge.go`.

> **Implementer note:** confirm the display field on `*auth.Identity` (e.g. `Subject`, `ID`, or `Email`) via `go doc github.com/specgraph/specgraph/internal/auth.Identity`; pick the stable, non-PII-sensitive identifier. Keep `identity` a LOCAL log attribute only — never propagated over the wire (security section).

- [ ] **Step 2: Build**

Run: `go build ./cmd/specgraph/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/nudge.go
git commit -s -m "feat(cli): implement telemetry context accessors (both project keys)"
```

### Task 14: enrichHandler (Handle + Enabled + WithAttrs + WithGroup)

**Files:**

- Create: `internal/telemetry/logging.go`
- Test: `internal/telemetry/logging_test.go`

- [ ] **Step 1: Write the failing test (enrichment + attr/group fidelity)**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func decode(t *testing.T, line []byte) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("unmarshal %q: %v", line, err)
	}
	return m
}

func TestEnrichHandler_AddsTraceAndProject(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := newEnrichHandler(base, Config{
		ProjectFromContext: func(context.Context) (string, bool) { return "acme", true },
	})
	logger := slog.New(h)

	// Context with a valid span.
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x02},
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	logger.InfoContext(ctx, "hello")

	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if m["trace_id"] == nil || m["span_id"] == nil {
		t.Fatalf("missing trace_id/span_id: %v", m)
	}
	if m["project"] != "acme" {
		t.Fatalf("project = %v, want acme", m["project"])
	}
}

func TestEnrichHandler_BareContextOmits(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), Config{})
	slog.New(h).InfoContext(context.Background(), "hi")
	m := decode(t, bytes.TrimSpace(buf.Bytes()))
	if _, ok := m["trace_id"]; ok {
		t.Fatalf("trace_id present on bare ctx: %v", m)
	}
}

func TestEnrichHandler_WithAttrsAndGroupSurvive(t *testing.T) {
	var buf bytes.Buffer
	h := newEnrichHandler(slog.NewJSONHandler(&buf, nil), Config{})
	logger := slog.New(h).With("svc", "x").WithGroup("g").With("k", "v")
	logger.InfoContext(context.Background(), "msg")
	out := buf.String()
	if !strings.Contains(out, `"svc":"x"`) {
		t.Fatalf("WithAttrs dropped: %s", out)
	}
	if !strings.Contains(out, `"g":{"k":"v"}`) {
		t.Fatalf("WithGroup dropped: %s", out)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -run TestEnrichHandler -v`
Expected: FAIL — `newEnrichHandler` undefined.

- [ ] **Step 3: Implement enrichHandler (delegates Enabled/WithAttrs/WithGroup)**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// enrichHandler wraps a downstream slog.Handler, appending trace correlation
// and request-scoped attributes pulled from context. WithAttrs/WithGroup are
// delegated so grouped/attached attributes are preserved.
type enrichHandler struct {
	next                slog.Handler
	projectFromContext  func(context.Context) (string, bool)
	identityFromContext func(context.Context) (string, bool)
}

func newEnrichHandler(next slog.Handler, cfg Config) *enrichHandler {
	return &enrichHandler{
		next:                next,
		projectFromContext:  cfg.ProjectFromContext,
		identityFromContext: cfg.IdentityFromContext,
	}
}

func (h *enrichHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *enrichHandler) Handle(ctx context.Context, rec slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if h.projectFromContext != nil {
		if p, ok := h.projectFromContext(ctx); ok {
			rec.AddAttrs(slog.String("project", p))
		}
	}
	if h.identityFromContext != nil {
		if id, ok := h.identityFromContext(ctx); ok {
			rec.AddAttrs(slog.String("identity", id))
		}
	}
	return h.next.Handle(ctx, rec)
}

func (h *enrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &enrichHandler{
		next:                h.next.WithAttrs(attrs),
		projectFromContext:  h.projectFromContext,
		identityFromContext: h.identityFromContext,
	}
}

func (h *enrichHandler) WithGroup(name string) slog.Handler {
	return &enrichHandler{
		next:                h.next.WithGroup(name),
		projectFromContext:  h.projectFromContext,
		identityFromContext: h.identityFromContext,
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/telemetry/ -run TestEnrichHandler -v`
Expected: PASS (all three). Note: `trace_id`/`span_id` are added at `Handle` time (post-group), so they appear at the record's group level — that is fine; the test asserts presence, not nesting.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/logging.go internal/telemetry/logging_test.go
git commit -s -m "feat(telemetry): add context-enriching slog handler"
```

### Task 15: Fanout (slog-multi) + otelslog bridge

**Files:**

- Modify: `internal/telemetry/logging.go`
- Modify: `internal/telemetry/telemetry.go` (build the logger in Init)

- [ ] **Step 1: Add the logger builder**

Append to `internal/telemetry/logging.go`:

```go
import (
	// add to the existing import block:
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	otellog "go.opentelemetry.io/otel/log"
)

// buildLogger composes: enrichHandler → fanout(base, otelslog bridge).
// When lp is nil (LogsExport off) the fanout collapses to base only. The
// returned logger is always non-nil. lpName is the instrumentation scope.
func buildLogger(cfg Config, base slog.Handler, lp loggerProvider) *slog.Logger {
	handlers := []slog.Handler{base}
	if lp != nil {
		handlers = append(handlers, otelslog.NewHandler("specgraph", otelslog.WithLoggerProvider(lp)))
	}
	fan := slogmulti.Fanout(handlers...)
	return slog.New(newEnrichHandler(fan, cfg))
}

// loggerProvider is the subset of *sdklog.LoggerProvider that otelslog needs.
// Declared as an interface so buildLogger stays testable and providers.go owns
// the concrete type. otelslog.WithLoggerProvider takes otellog.LoggerProvider.
type loggerProvider = otellog.LoggerProvider
```

> **Beta-API note:** verify `otelslog.NewHandler(name, otelslog.WithLoggerProvider(lp))` against the pinned version (`go doc go.opentelemetry.io/contrib/bridges/otelslog`). Some versions take the scope name positionally and the provider via option; adjust if the signature differs. `*sdklog.LoggerProvider` implements `otellog.LoggerProvider`.

- [ ] **Step 2: Wire buildLogger into Init (replace the placeholder logger)**

In `internal/telemetry/telemetry.go` Init, replace the Phase-1 line `tel.Logger = slog.New(cfg.LogHandler)` (in the Enabled branch) with:

```go
	tel.Logger = buildLogger(cfg, cfg.LogHandler, lp)
```

(Keep the disabled branch as `slog.New(cfg.LogHandler)` — no enrichment when disabled.)

- [ ] **Step 2b: Expose a logger-rebuild seam on the handle**

`nudgePreRun` builds the server logger over a stdout-JSON base because `cfg.Log` isn't loaded yet. To let `serve` honor `--log-format`/`--log-output`/`--log-level` AND keep enrichment + OTLP export, the handle must re-wrap an arbitrary base handler with the same enrich+fanout pipeline. The handle already retains its `Config` and `LoggerProvider` (`lp`) — add to `telemetry.go`:

```go
// NewLogger re-wraps base with the same enrichment + OTLP fanout pipeline as
// the handle's default logger. Used by the server to layer enrichment over its
// configured cfg.Log handler (format/level/output) without a second Init.
// When telemetry is disabled this returns slog.New(base) unchanged.
func (t *Telemetry) NewLogger(base slog.Handler) *slog.Logger {
	if t == nil || !t.cfg.Enabled {
		return slog.New(base)
	}
	return buildLogger(t.cfg, base, t.lp)
}
```

(Ensure `Init` stores `t.cfg = cfg` and `t.lp = lp` on the handle so this method can reuse them.)

- [ ] **Step 3: Build + full package tests**

Run: `go build ./internal/telemetry/ && go test ./internal/telemetry/ -v`
Expected: success; enrichment + fidelity tests still pass (they construct `newEnrichHandler` directly, unaffected).

- [ ] **Step 4: Commit**

```bash
git add internal/telemetry/logging.go internal/telemetry/telemetry.go
git commit -s -m "feat(telemetry): fan logs to stdout + otelslog OTLP bridge"
```

### Task 16: Activate the enriched logger on server + CLI

**Files:**

- Modify: `cmd/specgraph/serve.go:79-83` (use enriched logger)
- Modify: `cmd/specgraph/nudge.go` (server LogHandler = cfg.Log handler)

- [ ] **Step 1: Hand serve's configured log handler to telemetry**

Problem: `nudgePreRun` runs before `runServe` loads `cfg.Log`, so `initTelemetry` can't see the server's configured handler. Resolution: telemetry's default logger uses a stdout-JSON base (set in Task 9 Step 1); once `cfg.Log` is loaded, `runServe` re-wraps the server's CONFIGURED handler with telemetry's enrich+OTLP pipeline via `NewLogger` (Task 15 Step 2b), preserving `--log-format`/`--log-level`/`--log-output`. Replace `serve.go:79-83`:

```go
	logger, lErr := cfg.Log.Build()
	if lErr != nil {
		return fmt.Errorf("configure logger: %w", lErr)
	}
	if telState.tel != nil {
		// Layer enrichment (trace_id/span_id/project/identity) + OTLP log
		// export over the server's configured handler, honoring cfg.Log knobs.
		slog.SetDefault(telState.tel.NewLogger(logger.Handler()))
	} else {
		// Telemetry init failed; use the plain configured logger.
		slog.SetDefault(logger)
	}
	tel := telState.tel
```

> **Note:** `NewLogger` honors all `cfg.Log` knobs (level/format/output) AND adds enrichment + OTLP log export, so there is no logging-config regression. When telemetry is disabled, `tel` is nil and the plain configured logger is used unchanged. The base handler from `cfg.Log.Build().Handler()` is what enrichment wraps, so `--log-output stderr`/`--log-format text` flow through correctly.

- [ ] **Step 2: Build + serve test**

Run: `go build ./cmd/specgraph/ && go test ./cmd/specgraph/ -run TestValidateServerConfig`
Expected: success.

- [ ] **Step 3: Manual smoke test — CLI logs to stderr, --json clean on stdout**

Run:

```bash
go build -o /tmp/specgraph ./cmd/specgraph/
SPECGRAPH_OTEL_ENABLED=true OTEL_TRACES_EXPORTER=none OTEL_METRICS_EXPORTER=none OTEL_LOGS_EXPORTER=none /tmp/specgraph version 2>/tmp/err.log 1>/tmp/out.log
cat /tmp/out.log   # version only, no log lines
cat /tmp/err.log   # any slog output lands here
```

Expected: stdout has only command output; logs (if any) are on stderr.

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go cmd/specgraph/nudge.go
git commit -s -m "feat(server): activate enriched logger; keep CLI logs on stderr"
```

**Phase 3 gate:** enrichment + attr-fidelity tests green; manual smoke shows CLI logs on stderr, `--json` clean on stdout; `task check` passes.

---

## Phase 4 — Metrics

**Outcome:** Server emits RPC/HTTP/DB auto-metrics (free via the Phase-2 interceptors/wrappers) plus hand-rolled tx, drift, startup, and curated domain counters (Tier-2 subscriber + Tier-3 storage counters). All metric attributes are cardinality-safe.

### Task 17: Metrics registry with lazy global instruments

**Files:**

- Create: `internal/telemetry/metrics.go`
- Test: `internal/telemetry/metrics_test.go`

Design: package-level record helpers (`RecordTransaction`, `RecordDrift`, `RecordEdgeMutation`, `RecordStageTransition`, `RecordFindingsStored`, `RecordSpecChange`, `RecordStartup`) that resolve `otel.Meter("specgraph")` and create the instrument **on each call**. This is deliberate: the SDK meter caches instruments by identity (same name/kind/unit returns the same instrument), so per-call creation is cheap and idempotent, AND it always honors the *current* global `MeterProvider`. A cached `sync.Once` binding would lock instruments to whichever provider was global at first call — fine in production (provider set in `Init` before any record) but a test-isolation hazard when multiple tests install different `ManualReader` providers in the same package run. When telemetry is disabled the global meter is the no-op, so the helpers record nothing. This keeps `internal/storage/postgres` decoupled — it imports `internal/telemetry` (no cycle, since telemetry never imports postgres) and calls these helpers.

> **Note:** there is intentionally NO transaction-retry counter. `RunInTransaction` (`tx.go:38-64`) has no retry loop (first-writer-wins → `ErrConcurrentModification`), so a retries instrument would be permanently zero. `RecordTransaction` records duration only.

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/telemetry"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestMeter installs a fresh ManualReader provider for ONE test and
// restores the no-op provider on cleanup, so package-level test ordering
// cannot leak providers between tests (per-call meter resolution makes this
// fully isolated).
func newTestMeter(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(noop.NewMeterProvider()) })
	return reader
}

func TestRecordEdgeMutation_RecordsBoundedAttrs(t *testing.T) {
	reader := newTestMeter(t)

	telemetry.RecordEdgeMutation(context.Background(), "DEPENDS_ON", "add")
	telemetry.RecordTransaction(context.Background(), 5*time.Millisecond)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "specgraph.edge.mutations" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("specgraph.edge.mutations not recorded")
	}
}
```

(Add `"go.opentelemetry.io/otel/metric/noop"` to the test imports.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -run TestRecordEdgeMutation -v`
Expected: FAIL — record helpers undefined.

- [ ] **Step 3: Implement metrics.go**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meter resolves the current global meter. Instruments are created per-call;
// the SDK caches them by identity, so this is cheap and always honors the
// active MeterProvider (set in Init, or the no-op when disabled).
func meter() metric.Meter { return otel.Meter("specgraph") }

// RecordTransaction records a storage transaction's duration. (No retry
// counter: RunInTransaction has no retry loop.)
func RecordTransaction(ctx context.Context, d time.Duration) {
	h, _ := meter().Float64Histogram("specgraph.storage.transaction.duration", metric.WithUnit("s"))
	h.Record(ctx, d.Seconds())
}

// RecordDrift records a drift-detection pass duration.
func RecordDrift(ctx context.Context, d time.Duration) {
	h, _ := meter().Float64Histogram("specgraph.drift.detect.duration", metric.WithUnit("s"))
	h.Record(ctx, d.Seconds())
}

// RecordEdgeMutation counts an edge add/remove by edge_type + op (bounded).
func RecordEdgeMutation(ctx context.Context, edgeType, op string) {
	c, _ := meter().Int64Counter("specgraph.edge.mutations")
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("edge_type", edgeType),
		attribute.String("op", op),
	))
}

// RecordStageTransition counts a stage transition by from→to (bounded).
func RecordStageTransition(ctx context.Context, from, to string) {
	c, _ := meter().Int64Counter("specgraph.stage.transitions")
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("from", from),
		attribute.String("to", to),
	))
}

// RecordFindingsStored counts stored findings by pass_type (bounded).
func RecordFindingsStored(ctx context.Context, passType string, n int64) {
	c, _ := meter().Int64Counter("specgraph.findings.stored")
	c.Add(ctx, n, metric.WithAttributes(attribute.String("pass_type", passType)))
}

// RecordSpecChange counts committed spec changes by stage + checkpoint (bounded).
func RecordSpecChange(ctx context.Context, stage string, checkpoint bool) {
	c, _ := meter().Int64Counter("specgraph.spec.changes")
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("stage", stage),
		attribute.Bool("checkpoint", checkpoint),
	))
}

// RecordStartup records server startup duration.
func RecordStartup(ctx context.Context, d time.Duration) {
	h, _ := meter().Float64Histogram("specgraph.startup.duration", metric.WithUnit("s"))
	h.Record(ctx, d.Seconds())
}
```

> **Lint note:** ignoring the instrument-construction `error` (`c, _ := ...`) is intentional — a failed instrument returns a valid no-op, never nil. If `errcheck`/`wrapcheck` flags these, add `//nolint:errcheck // instrument errors yield no-op instruments` on each line, or assign to a discard handled once.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/telemetry/ -run TestRecordEdgeMutation -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/metrics.go internal/telemetry/metrics_test.go
git commit -s -m "feat(telemetry): add app metric instruments + record helpers"
```

### Task 18: Hand-rolled tx, drift, and startup metrics + spans

**Files:**

- Modify: `internal/storage/postgres/tx.go:38-64` (`RunInTransaction`)
- Modify: `internal/drift/drift.go:49` (`Check`)
- Modify: `cmd/specgraph/serve.go` (startup timing)

- [ ] **Step 1: Instrument RunInTransaction with a span + duration metric**

Edit `tx.go RunInTransaction`. Wrap the non-nested path (after the `if _, ok := txFromContext(ctx); ok` early return) with a span + timer:

```go
func (s *Store) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	ctx, span := otel.Tracer("specgraph.storage").Start(ctx, "storage.transaction")
	start := time.Now()
	defer func() {
		telemetry.RecordTransaction(ctx, time.Since(start))
		span.End()
	}()

	ctx = storage.InitChangeEvents(ctx)
	// ... rest unchanged (Begin/fn/Commit/dispatch) ...
}
```

Add imports to `tx.go`: `"time"`, `"go.opentelemetry.io/otel"`, `"github.com/specgraph/specgraph/internal/telemetry"`.

> **Cycle check:** `internal/storage/postgres` → `internal/telemetry` is fine (telemetry imports only `internal/storage` for `ChangeSubscriber`, never `postgres`). Verify with `go build ./...`.

- [ ] **Step 2: Instrument drift Check**

Edit `internal/drift/drift.go Check` (line 49):

```go
func (e *Engine) Check(ctx context.Context, slug, scope string) (*CheckResult, error) {
	ctx, span := otel.Tracer("specgraph.drift").Start(ctx, "drift.detect")
	start := time.Now()
	defer func() {
		telemetry.RecordDrift(ctx, time.Since(start))
		span.End()
	}()
	// ... existing body ...
}
```

Add imports `"time"`, `"go.opentelemetry.io/otel"`, `"github.com/specgraph/specgraph/internal/telemetry"` to `drift.go`.

- [ ] **Step 3: Record server startup duration**

In `serve.go runServe`, capture a start time at the very top of the function and record it once the server is listening. Add near line 65:

```go
	startupBegin := time.Now()
```

And after `slog.LogAttrs(... "server listening" ...)` (line 396):

```go
	if tel != nil {
		telemetry.RecordStartup(ctx, time.Since(startupBegin))
	}
```

- [ ] **Step 4: Build + tests**

Run: `go build ./... && go test ./internal/drift/ ./internal/storage/postgres/ -short`
Expected: success.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/tx.go internal/drift/drift.go cmd/specgraph/serve.go
git commit -s -m "feat: add tx/drift/startup spans and duration metrics"
```

### Task 19: Tier-2 metrics subscriber

**Files:**

- Create: `internal/telemetry/subscriber.go`
- Test: `internal/telemetry/subscriber_test.go`
- Modify: `cmd/specgraph/serve.go:144` (register subscriber)

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/telemetry"
)

func TestMetricsSubscriber_OnSpecChanged(t *testing.T) {
	sub := telemetry.NewMetricsSubscriber()
	// Must not panic and must satisfy storage.ChangeSubscriber.
	var _ storage.ChangeSubscriber = sub
	sub.OnSpecChanged(context.Background(), &storage.ChangeEvent{
		Slug: "x", Stage: "specify", Checkpoint: true,
	})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/telemetry/ -run TestMetricsSubscriber -v`
Expected: FAIL — `NewMetricsSubscriber` undefined.

- [ ] **Step 3: Implement the subscriber**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"

	"github.com/specgraph/specgraph/internal/storage"
)

// MetricsSubscriber implements storage.ChangeSubscriber, incrementing the
// spec-change counter on each committed change. Dispatch uses a detached
// context (tx.go), so there is no span correlation here — a plain counter.
type MetricsSubscriber struct{}

// NewMetricsSubscriber returns a ChangeSubscriber that records Tier-2 metrics.
func NewMetricsSubscriber() *MetricsSubscriber { return &MetricsSubscriber{} }

// OnSpecChanged records a spec change by stage + checkpoint.
func (m *MetricsSubscriber) OnSpecChanged(ctx context.Context, e *storage.ChangeEvent) {
	RecordSpecChange(ctx, e.Stage, e.Checkpoint)
}
```

- [ ] **Step 4: Register it in serve (gated)**

In `serve.go:144`, after `store.Subscribe(notify.NewImpactLogger())`:

```go
	store.Subscribe(notify.NewImpactLogger())
	if telState.enabled {
		store.Subscribe(telemetry.NewMetricsSubscriber())
	}
```

- [ ] **Step 5: Run to verify it passes + build**

Run: `go test ./internal/telemetry/ -run TestMetricsSubscriber -v && go build ./cmd/specgraph/`
Expected: PASS + build success.

- [ ] **Step 6: Commit**

```bash
git add internal/telemetry/subscriber.go internal/telemetry/subscriber_test.go cmd/specgraph/serve.go
git commit -s -m "feat(telemetry): add Tier-2 change-event metrics subscriber"
```

### Task 20: Tier-3 curated storage counters

**Files:**

- Modify: `internal/storage/postgres/graph.go:24` (AddEdge), `:79` (RemoveEdge)
- Modify: `internal/storage/postgres/authoring.go:26` (TransitionStage)
- Modify: `internal/storage/postgres/findings.go:22` (StoreFindings)

- [ ] **Step 1: Record edge mutations**

In `graph.go AddEdge`, at the post-commit success return (line 75, `return result, nil`):

```go
	telemetry.RecordEdgeMutation(ctx, string(edgeType), "add")
	return result, nil
```

In `RemoveEdge`, before the post-commit success return (line 92, `return nil`):

```go
	telemetry.RecordEdgeMutation(ctx, string(edgeType), "remove")
	return nil
```

Add import `"github.com/specgraph/specgraph/internal/telemetry"` to `graph.go`.

> **Implementer note:** both success returns are OUTSIDE the transaction closure (post-commit), which is the correct counting point — `result, nil` at AddEdge:75 and `nil` at RemoveEdge:92. The counter has no span/exemplar dependency. `RemoveEdge` is a `DELETE ... ON` with no row-count check (idempotent); still count it — the `op` dimension is bounded and a removed-nothing is rare.

- [ ] **Step 2: Record stage transitions**

`TransitionStage` ends with a bare `return s.RunInTransaction(...)`. Refactor that tail to capture the error and count only on commit success:

```go
	if err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// ... existing closure body unchanged ...
	}); err != nil {
		return err
	}
	telemetry.RecordStageTransition(ctx, string(from), string(to))
	return nil
```

Add the telemetry import to `authoring.go`.

- [ ] **Step 3: Record findings stored**

`StoreFindings` ends with `return ids, err` (findings.go:101). Count post-commit, guarded on success:

```go
	if err != nil {
		return ids, err
	}
	telemetry.RecordFindingsStored(ctx, string(passType), int64(len(findings)))
	return ids, nil
```

Add the telemetry import to `findings.go`. (Signature: `StoreFindings(ctx, slug string, passType storage.PassType, findings []storage.AnalyticalFindingInput) ([]string, error)`.)

- [ ] **Step 4: Build + storage short tests**

Run: `go build ./... && go test ./internal/storage/postgres/ -short`
Expected: success. (Full counter behavior is covered by the integration suite under `task pr-prep`.)

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/graph.go internal/storage/postgres/authoring.go internal/storage/postgres/findings.go
git commit -s -m "feat(storage): add Tier-3 curated domain counters"
```

### Task 21: Cardinality guard test + metric-seam integration test

**Files:**

- Create: `internal/telemetry/cardinality_test.go`

- [ ] **Step 1: Write the cardinality guard**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// forbiddenAttrKeys are high-cardinality dimensions that must NEVER appear on
// a metric (they belong on spans/logs only).
var forbiddenAttrKeys = []string{"slug", "id", "uri", "name", "trace_id"}

func TestDomainMetrics_NoHighCardinalityAttrs(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(noop.NewMeterProvider()) })

	ctx := context.Background()
	telemetry.RecordEdgeMutation(ctx, "DEPENDS_ON", "add")
	telemetry.RecordStageTransition(ctx, "shape", "specify")
	telemetry.RecordFindingsStored(ctx, "constitution-check", 3)
	telemetry.RecordSpecChange(ctx, "specify", true)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				for _, kv := range dp.Attributes.ToSlice() {
					key := strings.ToLower(string(kv.Key))
					for _, bad := range forbiddenAttrKeys {
						if key == bad {
							t.Errorf("metric %s carries forbidden high-cardinality attr %q", m.Name, key)
						}
					}
				}
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it passes**

Run: `go test ./internal/telemetry/ -run TestDomainMetrics_NoHighCardinality -v`
Expected: PASS — none of the domain counters carry slug/id/uri/name.

- [ ] **Step 3: Commit**

```bash
git add internal/telemetry/cardinality_test.go
git commit -s -m "test(telemetry): guard domain metrics against high-cardinality attrs"
```

**Phase 4 gate:** metric-seam + cardinality-guard tests green; `task check` passes.

---

## Final verification

- [ ] **Step 1: Full quality gate**

Run: `task check`
Expected: fmt:check → license:check → lint → build → unit tests all pass.

- [ ] **Step 2: Integration + e2e (requires Docker)**

Run: `task pr-prep`
Expected: check → integration (validates otelpgx + Tier-3 counters against real Postgres) → e2e pass.

- [ ] **Step 3: Disabled-overhead sanity**

Run:

```bash
go build -o /tmp/specgraph ./cmd/specgraph/
/tmp/specgraph spec list 2>&1 | head   # no otel output, original behavior
```

Expected: identical to pre-telemetry behavior; no exporter attempts.

- [ ] **Step 4: Update docs**

- Add a `docs/` section documenting `SPECGRAPH_OTEL_ENABLED`, `--otel`/`--otel-sample-ratio`/`--otel-logs`, the standard `OTEL_*` env, and the emitted span/metric/log inventory.
- Add the `internal/telemetry/` row to the `AGENTS.md` Architecture table.

```bash
git add docs/ AGENTS.md
git commit -s -m "docs: document OpenTelemetry configuration and signal inventory"
```

---

## Spec-coverage map

| Spec section | Task(s) |
|--------------|---------|
| Package layout / public seam | 2, 3, 4, 5 |
| Config knobs (flag/env, persistent root) | 3, 8 |
| No-op default + gated wiring | 5, 6, 7, 9, 10 |
| otelconnect server/client | 6, 7, 9 |
| CLI bootstrap (holder, nudgePreRun, run(), ExecuteContext) | 8 |
| serve consumes shared handle (no double-Init) | 9 |
| otelhttp edge + filter (probes/static/SSE) | 9 |
| otelpgx pool refactor (ParseConfig/NewWithConfig) | 10 |
| MCP loopback propagation + four wrap*Handler spans | 11 |
| Propagation round-trip + asymmetric baggage | 6, 12 |
| Context accessors (both project keys) | 13 |
| enrichHandler (Handle/Enabled/WithAttrs/WithGroup) | 14 |
| Fanout + otelslog bridge (beta, gated) | 15 |
| CLI stderr / server stdout logging | 8, 16 |
| Tier-1 RPC metrics (free) | 6, 9 |
| Tier-2 change subscriber | 19 |
| Tier-3 curated counters | 20 |
| tx/drift/startup spans+metrics | 18 |
| Cardinality rule | 17, 21 |
| Security/PII (no params, no baggage at edge) | 6, 10 |
| Dependencies (pin + tidy + GCP re-validate) | 1 |

---

## Execution notes for the implementer

- **Beta-API verification points** (flagged inline): otelconnect constructor/option names (`NewInterceptor`, `WithTracerProvider`, `WithPropagator`), otelhttp `WithPropagators`/`WithFilter`, `sdk/log` + `otelslog` symbols, `*auth.Identity` display field. Verify each against the pinned version's godoc before relying on it; the code shapes are correct but exact symbol names in beta modules can drift.
- **Import-cycle invariant:** `internal/telemetry` must never import `internal/server`, `internal/auth`, or `internal/storage/postgres`. It imports only `internal/storage` (for `ChangeSubscriber`). `internal/storage/postgres` and `internal/drift` import `internal/telemetry` (no cycle). Run `go build ./...` after Tasks 18 and 20 to confirm.
- **Single Init site:** telemetry is initialized exactly once, in `nudgePreRun`. `serve` consumes the holder. Never add a second `telemetry.Init`.
- **Gating discipline:** every per-request wiring (interceptors, otelhttp, otelpgx, subscriber) is gated on `telState.enabled`. The record helpers (`metrics.go`) are always safe to call (no-op meter when disabled), so storage seams don't need explicit gating.
