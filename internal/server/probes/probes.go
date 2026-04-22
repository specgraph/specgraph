// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package probes serves plain-HTTP Kubernetes/Knative liveness and readiness
// endpoints (/livez, /readyz) on a listener separate from the main API, so
// kubelet httpGet probes work without ConnectRPC framing.
package probes

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// Pinger is a dependency whose reachability gates readiness.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves /livez and /readyz. Readiness reflects the most recent
// background probe of the Pinger.
type Handler struct {
	ready atomic.Bool
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
	h.ready.Store(pinger.Ping(pingCtx) == nil)
}

// Livez reports liveness. Returns 200 unconditionally — reaching this handler
// means the HTTP goroutine is alive.
func (h *Handler) Livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz reports readiness. 200 when the last Pinger probe succeeded,
// 503 otherwise (including before the first probe completes).
func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	if h.ready.Load() {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
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
