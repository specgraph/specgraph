// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package telemetry

import (
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

// loopbackPropagator carries traceparent AND baggage; used only on internal
// hops (server↔server loopback), never at the public edge.
func loopbackPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// ServerInterceptor returns the otelconnect server interceptor, or nil when
// telemetry is disabled (callers must skip nil; connect.WithInterceptors
// tolerates none but we gate at the call site for zero overhead).
func ServerInterceptor(enabled bool) (connect.Interceptor, error) {
	if !enabled {
		return nil, nil
	}
	ic, err := otelconnect.NewInterceptor()
	if err != nil {
		return nil, fmt.Errorf("telemetry: otelconnect server interceptor: %w", err)
	}
	return ic, nil
}

// ClientInterceptor returns the otelconnect interceptor for outbound RPC
// clients, or nil when disabled. It is a separate named seam from
// ServerInterceptor purely for call-site clarity; both rely on the global
// TraceContext-only propagator (set in Init), so neither injects baggage at
// the public edge. Baggage rides only the loopback hop via LoopbackTransport.
func ClientInterceptor(enabled bool) (connect.Interceptor, error) {
	if !enabled {
		return nil, nil
	}
	ic, err := otelconnect.NewInterceptor()
	if err != nil {
		return nil, fmt.Errorf("telemetry: otelconnect client interceptor: %w", err)
	}
	return ic, nil
}

// LoopbackTransport wraps base with otelhttp using the loopback propagator
// (traceparent + baggage). Returns base unchanged when disabled.
func LoopbackTransport(enabled bool, base http.RoundTripper) http.RoundTripper {
	if !enabled {
		return base
	}
	return otelhttp.NewTransport(base, otelhttp.WithPropagators(loopbackPropagator()))
}

// WrapHTTPHandler wraps h with otelhttp to produce the root server HTTP span.
// The long-lived /mcp/ GET notification stream (SSE) is excluded from span
// creation: its span would stay open for the whole session and never export
// under a batch processor, inflating active-request counts. otelhttp wraps the
// ResponseWriter via httpsnoop, which preserves http.Flusher/Hijacker, so the
// /mcp/ streamable endpoint keeps working.
func WrapHTTPHandler(h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, "specgraph.http",
		otelhttp.WithFilter(func(r *http.Request) bool {
			if strings.HasPrefix(r.URL.Path, "/mcp/") && r.Method == http.MethodGet {
				return false // long-lived SSE notification stream
			}
			return true
		}),
	)
}
