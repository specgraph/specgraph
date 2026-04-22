// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package probes_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePinger returns the value stored in err and counts calls.
type fakePinger struct {
	calls atomic.Int32
	err   atomic.Pointer[error]
}

func (p *fakePinger) setErr(e error) {
	p.err.Store(&e)
}

func (p *fakePinger) Ping(_ context.Context) error {
	p.calls.Add(1)
	if e := p.err.Load(); e != nil {
		return *e
	}
	return nil
}

// waitFor polls cond until it returns true or the deadline elapses.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func TestLivez_AlwaysOK(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pinger permanently failing — /livez must still return 200.
	pinger := &fakePinger{}
	pinger.setErr(errors.New("db down"))
	h := probes.New(ctx, pinger, 10*time.Millisecond, 50*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/livez", http.NoBody)
	w := httptest.NewRecorder()
	h.Livez(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadyz_ReflectsPingerState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &fakePinger{}
	pinger.setErr(errors.New("db down"))
	h := probes.New(ctx, pinger, 10*time.Millisecond, 50*time.Millisecond)

	// First probe runs synchronously-ish at startup; wait until it's been called.
	waitFor(t, func() bool { return pinger.calls.Load() >= 1 })

	// Pinger failing → readyz 503.
	req := httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody)
	w := httptest.NewRecorder()
	h.Readyz(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Flip pinger to healthy and wait for a fresh probe.
	callsBefore := pinger.calls.Load()
	pinger.setErr(nil)
	waitFor(t, func() bool { return pinger.calls.Load() > callsBefore })

	// Poll until cache flips (atomic update visible to another goroutine).
	waitFor(t, func() bool {
		w2 := httptest.NewRecorder()
		h.Readyz(w2, req)
		return w2.Code == http.StatusOK
	})
}

func TestReadyz_NotReadyBeforeFirstProbe(t *testing.T) {
	// Pinger that blocks forever — first probe never completes; readyz must be 503.
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	pinger := blockingPinger{block: block}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := probes.New(ctx, pinger, time.Hour, 10*time.Millisecond)
	// probeTimeout will elapse quickly and Ping will return ctx.DeadlineExceeded,
	// which counts as a failed probe → ready=false.
	waitFor(t, func() bool {
		w := httptest.NewRecorder()
		h.Readyz(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		return w.Code == http.StatusServiceUnavailable
	})
}

func TestMux_RoutesBothEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &fakePinger{}
	h := probes.New(ctx, pinger, 10*time.Millisecond, 50*time.Millisecond)
	waitFor(t, func() bool { return pinger.calls.Load() >= 1 })

	mux := h.Mux()

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/livez")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	waitFor(t, func() bool {
		r, getErr := http.Get(srv.URL + "/readyz")
		if getErr != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusOK
	})
}

type blockingPinger struct{ block <-chan struct{} }

func (p blockingPinger) Ping(ctx context.Context) error {
	select {
	case <-p.block:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
