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

// probeState is a consistent snapshot of the last probe's outcome. Storing
// it behind a single atomic pointer means readers see a coherent triple
// (probed, ready, err) without per-field ordering hazards, and the writer
// publishes the entire transition in one Store.
type probeState struct {
	ready bool
	err   error
}

// Handler serves /livez and /readyz. Readiness reflects the most recent
// background probe of the Pinger; the last probe's error is retained for
// the /readyz body (see Readyz).
type Handler struct {
	state atomic.Pointer[probeState]
}

// NewHandler returns a Handler that serves /livez (always 200) and /readyz
// (503 until the first probe completes). It does NOT probe — bind it to a
// listener to bring liveness up immediately, independent of the gated
// dependency, then call Start once the dependency wiring is ready. Until Start
// runs, /readyz is a truthful, silent 503 ("probe has not yet completed").
func NewHandler() *Handler {
	return &Handler{}
}

// Start launches the background probe loop: it probes pinger every interval
// using probeTimeout per call, updating /readyz. The first probe runs
// immediately so readiness reflects current state without waiting a full
// interval. The goroutine exits when ctx is cancelled. Call Start at most once.
func (h *Handler) Start(ctx context.Context, pinger Pinger, interval, probeTimeout time.Duration) {
	go h.run(ctx, pinger, interval, probeTimeout)
}

// New binds and starts in one call: NewHandler followed immediately by Start.
// Retained for callers whose pinger is already live when the listener binds.
func New(ctx context.Context, pinger Pinger, interval, probeTimeout time.Duration) *Handler {
	h := NewHandler()
	h.Start(ctx, pinger, interval, probeTimeout)
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
	if ctx.Err() != nil {
		// Parent ctx cancelled — we're shutting down. State updates and
		// log lines would only add noise; the goroutine's next loop
		// iteration will observe ctx.Done and exit.
		return
	}
	prev := h.state.Load()
	h.state.Store(&probeState{ready: err == nil, err: err})

	switch {
	case prev == nil && err != nil:
		// Pod starting against a dead dependency: log once so operators
		// tailing for readiness failures see something. A silent boot
		// against a permanently-down DB would otherwise reveal the
		// failure only via 503 bodies on /readyz.
		slog.LogAttrs(ctx, slog.LevelWarn, "readiness probe failed on first attempt", slog.Any("error", err))
	case prev == nil:
		// First probe is healthy — happy path, stay silent.
	case err != nil && prev.ready:
		slog.LogAttrs(ctx, slog.LevelWarn, "readiness probe failed", slog.Any("error", err))
	case err == nil && !prev.ready:
		slog.LogAttrs(ctx, slog.LevelInfo, "readiness probe recovered")
	case err != nil && prev.err != nil && prev.err.Error() != err.Error():
		// Mid-outage error shape changed (e.g., connection-refused →
		// auth-failed). Re-log so logs mirror what /readyz now returns.
		slog.LogAttrs(ctx, slog.LevelWarn, "readiness probe error changed", slog.Any("error", err))
	}
}

// Livez reports liveness. Returns 200 unconditionally — reaching this handler
// means the HTTP goroutine is alive.
func (h *Handler) Livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz reports readiness. 200 when the last probe succeeded, 503 otherwise
// (including before the first probe completes). The 503 body carries the
// retained probe error so operators curling /readyz see the cause without
// tailing pod logs.
func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	s := h.state.Load()
	if s != nil && s.ready {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	reason := "not ready: probe has not yet completed"
	if s != nil && s.err != nil {
		reason = fmt.Sprintf("not ready: %s", s.err)
	}
	// A failed write here means the peer already disconnected; logging per
	// half-open connection would flood during an outage.
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
