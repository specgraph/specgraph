// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/telemetry"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// forbiddenAttrKeys are high-cardinality dimensions that must NEVER appear on
// a metric (they belong on spans/logs only).
var forbiddenAttrKeys = []string{"slug", "id", "uri", "name", "trace_id"}

func TestDomainMetrics_NoHighCardinalityAttrs(t *testing.T) {
	reader := newTestMeter(t)

	ctx := context.Background()
	telemetry.RecordEdgeMutation(ctx, "DEPENDS_ON", "add")
	telemetry.RecordStageTransition(ctx, "shape", "specify")
	telemetry.RecordFindingsStored(ctx, "constitution-check", 3)
	telemetry.RecordSpecChange(ctx, "specify", true)

	// expected guards against the test passing vacuously: if a counter were
	// renamed/dropped or degraded to a no-op, its name would never be seen and
	// the post-scan assertion below would fail.
	expected := map[string]bool{
		"specgraph.edge.mutations":    false,
		"specgraph.stage.transitions": false,
		"specgraph.findings.stored":   false,
		"specgraph.spec.changes":      false,
	}

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
			if _, want := expected[m.Name]; want {
				expected[m.Name] = true
			}
			for _, dp := range sum.DataPoints {
				for _, kv := range dp.Attributes.ToSlice() {
					key := strings.ToLower(string(kv.Key))
					for _, token := range strings.Split(key, "_") {
						for _, bad := range forbiddenAttrKeys {
							if token == bad {
								t.Errorf("metric %s carries forbidden high-cardinality attr %q (token %q)", m.Name, key, token)
							}
						}
					}
				}
			}
		}
	}

	for name, seen := range expected {
		if !seen {
			t.Errorf("expected domain counter %q was not recorded", name)
		}
	}
}
