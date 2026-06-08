// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// enrichHandler wraps a downstream slog.Handler, appending trace correlation
// and request-scoped attributes pulled from context. WithAttrs/WithGroup are
// delegated so grouped/attached attributes are preserved.
type enrichHandler struct {
	next                slog.Handler
	projectFromContext  func(context.Context) (string, bool)
	identityFromContext func(context.Context) (string, bool)
}

func newEnrichHandler(
	next slog.Handler,
	projectFromContext, identityFromContext func(context.Context) (string, bool),
) *enrichHandler {
	return &enrichHandler{
		next:                next,
		projectFromContext:  projectFromContext,
		identityFromContext: identityFromContext,
	}
}

func (h *enrichHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *enrichHandler) Handle(ctx context.Context, rec slog.Record) error { //nolint:gocritic // rec is passed by value as required by the slog.Handler interface.
	// Clone before mutating: a downstream fanout may deliver this record to multiple sinks (slog.Handler contract).
	rec = rec.Clone()
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if h.projectFromContext != nil {
		if p, ok := h.projectFromContext(ctx); ok {
			rec.AddAttrs(slog.String("project", p))
		}
	}
	if h.identityFromContext != nil {
		if id, ok := h.identityFromContext(ctx); ok {
			rec.AddAttrs(slog.String("identity", id))
		}
	}
	return h.next.Handle(ctx, rec) //nolint:wrapcheck // pass-through delegation to the downstream handler.
}

func (h *enrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &enrichHandler{
		next:                h.next.WithAttrs(attrs),
		projectFromContext:  h.projectFromContext,
		identityFromContext: h.identityFromContext,
	}
}

func (h *enrichHandler) WithGroup(name string) slog.Handler {
	return &enrichHandler{
		next:                h.next.WithGroup(name),
		projectFromContext:  h.projectFromContext,
		identityFromContext: h.identityFromContext,
	}
}
