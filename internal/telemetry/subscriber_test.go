// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/telemetry"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestMetricsSubscriber_OnSpecChanged(_ *testing.T) {
	sub := telemetry.NewMetricsSubscriber()
	// Must satisfy storage.ChangeSubscriber and not panic.
	var _ storage.ChangeSubscriber = sub
	sub.OnSpecChanged(context.Background(), &storage.ChangeEvent{
		Slug: "x", Stage: "specify", Checkpoint: true,
	})
}

func TestMetricsSubscriber_RecordsSpecChange(t *testing.T) {
	reader := newTestMeter(t)
	sub := telemetry.NewMetricsSubscriber()
	sub.OnSpecChanged(context.Background(), &storage.ChangeEvent{
		Slug: "x", Stage: "specify", Checkpoint: true,
	})

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "specgraph.spec.changes" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("specgraph.spec.changes is %T, want Sum[int64]", m.Data)
			}
			for _, dp := range sum.DataPoints {
				stage, _ := dp.Attributes.Value("stage")
				cp, _ := dp.Attributes.Value("checkpoint")
				if stage.AsString() == "specify" && cp.AsBool() {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("specgraph.spec.changes with stage=specify checkpoint=true not recorded")
	}
}
