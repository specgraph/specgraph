// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
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
