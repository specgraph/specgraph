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
	h, _ := meter().Float64Histogram("specgraph.storage.transaction.duration", metric.WithUnit("s")) //nolint:errcheck // a failed instrument is a valid no-op, never nil
	h.Record(ctx, d.Seconds())
}

// RecordDrift records a drift-detection pass duration.
func RecordDrift(ctx context.Context, d time.Duration) {
	h, _ := meter().Float64Histogram("specgraph.drift.detect.duration", metric.WithUnit("s")) //nolint:errcheck // a failed instrument is a valid no-op, never nil
	h.Record(ctx, d.Seconds())
}

// RecordEdgeMutation counts an edge add/remove by edge_type + op (bounded).
func RecordEdgeMutation(ctx context.Context, edgeType, op string) {
	c, _ := meter().Int64Counter("specgraph.edge.mutations") //nolint:errcheck // a failed instrument is a valid no-op, never nil
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("edge_type", edgeType),
		attribute.String("op", op),
	))
}

// RecordStageTransition counts a stage transition by from→to (bounded).
func RecordStageTransition(ctx context.Context, from, to string) {
	c, _ := meter().Int64Counter("specgraph.stage.transitions") //nolint:errcheck // a failed instrument is a valid no-op, never nil
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("from", from),
		attribute.String("to", to),
	))
}

// RecordFindingsStored counts stored findings by pass_type (bounded).
func RecordFindingsStored(ctx context.Context, passType string, n int64) {
	c, _ := meter().Int64Counter("specgraph.findings.stored") //nolint:errcheck // a failed instrument is a valid no-op, never nil
	c.Add(ctx, n, metric.WithAttributes(attribute.String("pass_type", passType)))
}

// RecordSpecChange counts committed spec changes by stage + checkpoint (bounded).
func RecordSpecChange(ctx context.Context, stage string, checkpoint bool) {
	c, _ := meter().Int64Counter("specgraph.spec.changes") //nolint:errcheck // a failed instrument is a valid no-op, never nil
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("stage", stage),
		attribute.Bool("checkpoint", checkpoint),
	))
}

// RecordStartup records server startup duration.
func RecordStartup(ctx context.Context, d time.Duration) {
	h, _ := meter().Float64Histogram("specgraph.startup.duration", metric.WithUnit("s")) //nolint:errcheck // a failed instrument is a valid no-op, never nil
	h.Record(ctx, d.Seconds())
}
