// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/specgraph/specgraph/internal/server/probes"
)

// errStoreNotReady is returned by readinessPinger.Ping before a live store has
// been installed, so /readyz reports 503 during the degraded window.
var errStoreNotReady = errors.New("storage not ready: postgres connection not yet established")

// readinessPinger adapts a not-yet-available backing pinger (the *postgres.Store)
// to probes.Pinger. The inner pinger lives behind an atomic pointer so the
// background connector can publish the live store without locking the probe
// goroutine. The wrapper struct is required because atomic.Pointer needs a
// concrete element type.
type readinessPinger struct {
	inner atomic.Pointer[livePinger]
}

type livePinger struct{ p probes.Pinger }

func newReadinessPinger() *readinessPinger { return &readinessPinger{} }

// set publishes the live backing pinger. *postgres.Store satisfies probes.Pinger.
func (rp *readinessPinger) set(p probes.Pinger) { rp.inner.Store(&livePinger{p: p}) }

// Ping reports errStoreNotReady until a backing pinger is set, then delegates.
func (rp *readinessPinger) Ping(ctx context.Context) error {
	lp := rp.inner.Load()
	if lp == nil || lp.p == nil {
		return errStoreNotReady
	}
	return lp.p.Ping(ctx)
}

// atomicHandler is a fixed http.Handler whose delegate is swapped atomically.
// srv.Handler is set to the atomicHandler once and never reassigned; only the
// internal pointer changes, so concurrent swaps never race the http.Server's
// per-request Handler read.
type atomicHandler struct {
	h atomic.Pointer[http.Handler]
}

func newAtomicHandler(initial http.Handler) *atomicHandler {
	a := &atomicHandler{}
	a.set(initial)
	return a
}

func (a *atomicHandler) set(h http.Handler) { a.h.Store(&h) }

func (a *atomicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*a.h.Load()).ServeHTTP(w, r)
}

// notReadyHandler responds 503 to every request while Postgres is unavailable.
func notReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, werr := io.WriteString(w, "storage not ready\n"); werr != nil {
			slog.LogAttrs(r.Context(), slog.LevelDebug, "write not-ready body", slog.Any("error", werr))
		}
	})
}
