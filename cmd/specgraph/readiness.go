// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
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
