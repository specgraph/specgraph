// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	v1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// healthStub is a trivial ServerService implementation: it answers Health and
// inherits CodeUnimplemented for every other method via the embedded
// Unimplemented type.
type healthStub struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (healthStub) Health(_ context.Context, _ *connect.Request[v1.HealthRequest]) (*connect.Response[v1.HealthResponse], error) {
	return connect.NewResponse(&v1.HealthResponse{}), nil
}

// TestPropagationRoundTrip proves the otelconnect client interceptor injects a
// traceparent that the otelconnect server interceptor extracts, so the server
// span is a child of the client span: one connected trace.
func TestPropagationRoundTrip(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	prop := propagation.TraceContext{}

	// One interceptor instance, backed by the SAME provider + propagator,
	// drives both the handler and the client paths. WithTrustRemote makes the
	// server span a true child of the incoming client span (by default
	// otelconnect links untrusted remote spans instead of parenting them),
	// which is what lets us assert direct parent linkage below.
	ic, err := otelconnect.NewInterceptor(
		otelconnect.WithTracerProvider(tp),
		otelconnect.WithPropagator(prop),
		otelconnect.WithTrustRemote(),
	)
	if err != nil {
		t.Fatalf("NewInterceptor: %v", err)
	}

	path, h := specgraphv1connect.NewServerServiceHandler(healthStub{}, connect.WithInterceptors(ic))
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewServerServiceClient(srv.Client(), srv.URL, connect.WithInterceptors(ic))
	if _, err := client.Health(context.Background(), connect.NewRequest(&v1.HealthRequest{})); err != nil {
		t.Fatalf("Health RPC: %v", err)
	}

	spans := rec.Ended()
	if len(spans) < 2 {
		t.Fatalf("expected >= 2 spans (client + server), got %d", len(spans))
	}

	var clientSpan, serverSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		switch s.SpanKind() {
		case trace.SpanKindClient:
			clientSpan = s
		case trace.SpanKindServer:
			serverSpan = s
		}
	}
	if clientSpan == nil {
		t.Fatalf("no client-kind span recorded")
	}
	if serverSpan == nil {
		t.Fatalf("no server-kind span recorded")
	}

	if serverSpan.Parent().SpanID() != clientSpan.SpanContext().SpanID() {
		t.Fatalf("server span parent SpanID = %s, want client SpanID %s",
			serverSpan.Parent().SpanID(), clientSpan.SpanContext().SpanID())
	}
	if serverSpan.SpanContext().TraceID() != clientSpan.SpanContext().TraceID() {
		t.Fatalf("server TraceID = %s, client TraceID = %s: spans are not in one trace",
			serverSpan.SpanContext().TraceID(), clientSpan.SpanContext().TraceID())
	}
}
