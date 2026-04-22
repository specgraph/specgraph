// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package probes serves plain-HTTP Kubernetes/Knative liveness and readiness
// endpoints (/livez, /readyz) on a listener separate from the main API, so
// kubelet httpGet probes work without ConnectRPC framing.
package probes

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// Pinger is a dependency whose reachability gates readiness.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves /livez and /readyz. Readiness reflects the most recent
// background probe of the Pinger. The most recent probe error (if any)
// is retained so /readyz bodies surface the failure cause rather than an
// empty 503 — operators curling the endpoint get a reason string without
// having to correlate with pod logs.
type Handler struct {
	ready   atomic.Bool
	probed  atomic.Bool
	lastErr atomic.Pointer[error]
}

// New starts a background goroutine that probes pinger every interval using
// probeTimeout per call. The goroutine exits when ctx is cancelled.
// The first probe runs immediately so readiness reflects current state
// without waiting a full interval.
func New(ctx context.Context, pinger Pinger, interval, probeTimeout time.Duration) *Handler {
	h := &Handler{}
	go h.run(ctx, pinger, interval, probeTimeout)
	return h
}

func (h *Handler) run(ctx context.Context, pinger Pinger, interval, probeTimeout time.Duration) {
	h.probe(ctx, pinger, probeTimeout)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.probe(ctx, pinger, probeTimeout)
		}
	}
}

func (h *Handler) probe(ctx context.Context, pinger Pinger, timeout time.Duration) {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := pinger.Ping(pingCtx)
	// Write lastErr before flipping ready so a Readyz racing with this
	// probe sees an accurate reason for its 503.
	if err != nil {
		h.lastErr.Store(&err)
	} else {
		h.lastErr.Store(nil)
	}
	prev := h.ready.Swap(err == nil)
	// The first probe has no prior state — zero-value ready=false looks
	// like "failing" but it's just uninitialized, so treat the flip as
	// informational and skip the transition log.
	if !h.probed.Swap(true) {
		return
	}
	switch {
	case err != nil && prev:
		slog.Warn("readiness probe failed", "error", err)
	case err == nil && !prev:
		slog.Info("readiness probe recovered")
	}
}

// Livez reports liveness. Returns 200 unconditionally — reaching this handler
// means the HTTP goroutine is alive.
func (h *Handler) Livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz reports readiness. 200 when the last Pinger probe succeeded,
// 503 otherwise (including before the first probe completes). The 503
// response body carries the retained probe error (or "not yet probed"
// before the first probe) so operators curling /readyz see the cause
// without tailing logs.
func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	if h.ready.Load() {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	reason := "not ready: probe has not yet completed"
	if e := h.lastErr.Load(); e != nil && *e != nil {
		reason = fmt.Sprintf("not ready: %s", *e)
	}
	// A failed body write here means the client has already disconnected;
	// there is nothing left to tell them, so drop the error rather than
	// taking on a log line on every half-open connection.
	_, _ = fmt.Fprintln(w, reason) //nolint:errcheck // client gone, nothing to do
}

// Mux returns a fresh http.Handler wiring /livez and /readyz. Callers wanting
// additional routes on the same listener should mount this handler under
// their own mux rather than relying on the concrete type.
func (h *Handler) Mux() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/livez", h.Livez)
	m.HandleFunc("/readyz", h.Readyz)
	return m
}
