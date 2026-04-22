// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package probes_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer wraps bytes.Buffer with a mutex so the probe goroutine writing
// via slog and the test goroutine reading via String don't race.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

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

func TestReadyz_BodyCarriesReason(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &fakePinger{}
	pinger.setErr(errors.New("postgres: ping: dial tcp: connect: connection refused"))
	h := probes.New(ctx, pinger, 10*time.Millisecond, 50*time.Millisecond)

	waitFor(t, func() bool { return pinger.calls.Load() >= 1 })

	// Poll until the cache has flipped to the failing state so the reason
	// string reflects the stored error rather than the pre-probe marker.
	waitFor(t, func() bool {
		w := httptest.NewRecorder()
		h.Readyz(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		return w.Code == http.StatusServiceUnavailable &&
			bytes.Contains(w.Body.Bytes(), []byte("connection refused"))
	})

	w := httptest.NewRecorder()
	h.Readyz(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Contains(t, w.Body.String(), "not ready:")
	assert.Contains(t, w.Body.String(), "connection refused")
}

func TestReadyz_BodyReportsPrePingState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Blocking pinger that never returns until ctx expires — first probe
	// will time out and populate lastErr with context.DeadlineExceeded.
	pinger := blockingPinger{block: make(chan struct{})}
	h := probes.New(ctx, pinger, time.Hour, 10*time.Millisecond)

	// Before the first probe window elapses the handler reports the
	// pre-probe marker; after the timeout it reports the deadline error.
	// Either message is acceptable; what we assert is that the body is
	// never empty on 503.
	waitFor(t, func() bool {
		w := httptest.NewRecorder()
		h.Readyz(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		return w.Code == http.StatusServiceUnavailable && w.Body.Len() > 0
	})
}

// slog.SetDefault is process-global; do not add t.Parallel() to these
// logger-capturing tests.
func TestProbe_LogsTransitionsOnly(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &fakePinger{}
	h := probes.New(ctx, pinger, 5*time.Millisecond, 50*time.Millisecond)

	// First healthy probe is silent (happy-path boot); subsequent healthy
	// probes stay silent too.
	waitFor(t, func() bool { return pinger.calls.Load() >= 3 })
	assert.NotContains(t, buf.String(), "readiness probe", "steady-state healthy must not log")

	pinger.setErr(errors.New("synthetic failure"))
	waitFor(t, func() bool { return strings.Contains(buf.String(), "readiness probe failed") })
	assert.Contains(t, buf.String(), "synthetic failure")

	failCallsBefore := pinger.calls.Load()
	waitFor(t, func() bool { return pinger.calls.Load() > failCallsBefore+2 })
	assert.Equal(t, 1, strings.Count(buf.String(), "readiness probe failed"),
		"failing steady state must not flood logs")

	// Error shape changes mid-outage must re-log so pod logs mirror what
	// /readyz now returns.
	pinger.setErr(errors.New("different cause"))
	waitFor(t, func() bool { return strings.Contains(buf.String(), "readiness probe error changed") })
	assert.Contains(t, buf.String(), "different cause")

	pinger.setErr(nil)
	waitFor(t, func() bool { return strings.Contains(buf.String(), "readiness probe recovered") })
	assert.Equal(t, 1, strings.Count(buf.String(), "readiness probe recovered"))

	// After recovery /readyz is 200 and body reasons are silent — lastErr
	// must be cleared so operators don't see a stale cause.
	waitFor(t, func() bool {
		w := httptest.NewRecorder()
		h.Readyz(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		return w.Code == http.StatusOK
	})
}

// slog.SetDefault is process-global; do not add t.Parallel() here.
func TestProbe_LogsFirstProbeFailure(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &fakePinger{}
	pinger.setErr(errors.New("db unreachable on boot"))
	probes.New(ctx, pinger, time.Hour, 50*time.Millisecond)

	// First-probe-failing must produce a log line; a pod that never
	// becomes ready would otherwise leave operators without any log
	// signal that readiness is permanently failing.
	waitFor(t, func() bool {
		return strings.Contains(buf.String(), "readiness probe failed on first attempt")
	})
	assert.Contains(t, buf.String(), "db unreachable on boot")
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
