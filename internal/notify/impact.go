// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package notify provides subscribers for storage change events.
package notify

import (
	"context"
	"log/slog"

	"github.com/specgraph/specgraph/internal/storage"
)

// ImpactLogger logs which specs are impacted when an upstream spec changes.
// Stateless — extracts the scoped GraphBackend from context at dispatch time.
type ImpactLogger struct{}

// NewImpactLogger creates an ImpactLogger.
func NewImpactLogger() *ImpactLogger {
	return &ImpactLogger{}
}

// OnSpecChanged implements storage.ChangeSubscriber.
func (l *ImpactLogger) OnSpecChanged(ctx context.Context, event *storage.ChangeEvent) {
	graph, ok := storage.GraphBackendFromContext(ctx)
	if !ok {
		return
	}

	refs, err := graph.GetImpact(ctx, event.Slug)
	if err != nil {
		slog.Warn("impact analysis failed",
			"slug", event.Slug,
			"error", err.Error(),
		)
		return
	}

	slugs := make([]string, len(refs))
	for i, r := range refs {
		slugs[i] = r.Slug
	}

	slog.Info("spec change impact",
		"slug", event.Slug,
		"version", event.Version,
		"impacted_count", len(refs),
		"impacted", slugs,
	)
}
