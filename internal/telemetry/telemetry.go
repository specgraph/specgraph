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
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
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
	// mu serializes concurrent Shutdown callers (serve's graceful flush vs.
	// run()'s deferred flush).
	mu            sync.Mutex
	shutdownFuncs []func(context.Context) error

	cfg Config                 // retained for NewLogger
	lp  *sdklog.LoggerProvider // retained for NewLogger (nil when LogsExport off)
}

// Init wires the configured providers. On Enabled=false it returns working
// no-op providers and a plain (cfg.LogHandler) logger; Shutdown is a no-op.
// Init never returns a fatal error for telemetry setup beyond a nil
// LogHandler — provider build failures fall back to no-op (see callers).
func Init(ctx context.Context, cfg Config) (*Telemetry, error) { //nolint:gocritic // Init is the package entrypoint; Config is passed by value as the public API contract.
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
	var initErr error
	defer func() {
		if initErr != nil {
			// Best-effort cleanup: close any providers already registered so a
			// mid-Init failure doesn't leak background processors or leave a
			// live global provider behind (fallback must be a true no-op).
			_ = tel.Shutdown(ctx) //nolint:errcheck // best-effort cleanup; initErr is the meaningful error returned to the caller.
		}
	}()

	res, err := buildResource(ctx, &cfg)
	if err != nil {
		initErr = err
		return nil, initErr
	}

	tp, err := buildTracerProvider(ctx, &cfg, res)
	if err != nil {
		initErr = err
		return nil, initErr
	}
	otel.SetTracerProvider(tp)
	tel.Tracer = tp.Tracer("specgraph")
	tel.shutdownFuncs = append(tel.shutdownFuncs, tp.Shutdown)

	if cfg.Role == RoleServer {
		mp, mErr := buildMeterProvider(ctx, res)
		if mErr != nil {
			initErr = mErr
			return nil, initErr
		}
		otel.SetMeterProvider(mp)
		tel.Meter = mp.Meter("specgraph")
		tel.shutdownFuncs = append(tel.shutdownFuncs, mp.Shutdown)
	} else {
		tel.Meter = metricnoop.NewMeterProvider().Meter("specgraph")
	}

	lp, err := buildLoggerProvider(ctx, &cfg, res)
	if err != nil {
		initErr = err
		return nil, initErr
	}
	if lp != nil {
		tel.shutdownFuncs = append(tel.shutdownFuncs, lp.Shutdown)
	}
	tel.cfg = cfg
	tel.lp = lp

	// Global propagator is TraceContext only (safe at the untrusted edge).
	// Baggage is applied per-transport on internal/loopback hops, not here.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Production logger: enrichHandler → fanout(base, otelslog bridge over lp).
	tel.Logger = buildLogger(&cfg, cfg.LogHandler, lp)

	return tel, nil
}

// NewLogger re-wraps base with the same enrichment + OTLP fanout pipeline as
// the handle's default logger. The server uses this to layer enrichment over
// its configured cfg.Log handler (format/level/output) without a second Init.
// When telemetry is disabled (or t is nil) it returns slog.New(base) unchanged.
func (t *Telemetry) NewLogger(base slog.Handler) *slog.Logger {
	if t == nil || !t.cfg.Enabled {
		return slog.New(base)
	}
	return buildLogger(&t.cfg, base, t.lp)
}

// Shutdown flushes and shuts down each provider in reverse registration
// order, aggregating errors. Idempotent and safe for concurrent use: the
// mutex serializes callers, so a second (possibly concurrent) call blocks
// until the first completes and then is a no-op. This lets serve's
// graceful-shutdown flush and run()'s deferred flush race safely. Bound the
// caller's ctx with a timeout (the caller owns the budget).
func (t *Telemetry) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
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
