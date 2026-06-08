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

var _ storage.ChangeSubscriber = (*MetricsSubscriber)(nil)

// NewMetricsSubscriber returns a ChangeSubscriber that records Tier-2 metrics.
func NewMetricsSubscriber() *MetricsSubscriber { return &MetricsSubscriber{} }

// OnSpecChanged records a spec change by stage + checkpoint.
func (m *MetricsSubscriber) OnSpecChanged(ctx context.Context, e *storage.ChangeEvent) {
	RecordSpecChange(ctx, e.Stage, e.Checkpoint)
}
