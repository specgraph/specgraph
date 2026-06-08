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
