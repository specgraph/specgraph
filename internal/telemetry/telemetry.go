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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
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
	shutdownFuncs []func(context.Context) error
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
