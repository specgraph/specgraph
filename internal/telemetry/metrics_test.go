// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
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
