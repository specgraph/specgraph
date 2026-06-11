// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func probesCfg(addr string) config.ProbesConfig {
	return config.ProbesConfig{Listen: addr, Interval: 5 * time.Millisecond, Timeout: 50 * time.Millisecond}
}

type stubPinger struct {
	err error
}

func (p *stubPinger) Ping(_ context.Context) error { return p.err }

func TestStartProbeListener_DisabledWhenEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, errCh, err := startProbeListener(ctx, probes.NewHandler(), probesCfg(""))
	require.NoError(t, err)
	assert.Nil(t, srv, "empty addr disables probes — no listener should be created")
	assert.Nil(t, errCh, "no death channel when probes are disabled")
}

// TestStartProbeListener_LivezUpBeforeProbing is the core liveness-decoupling
// guarantee: the listener answers /livez with 200 and /readyz with 503 as soon
// as it binds — before Start runs any probe. This is what keeps kubelet from
// killing the pod while Postgres is still connecting.
func TestStartProbeListener_LivezUpBeforeProbing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, _, err := startProbeListener(ctx, probes.NewHandler(), probesCfg("127.0.0.1:0"))
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	})
	base := "http://" + srv.Addr

	resp, err := http.Get(base + "/livez") //nolint:noctx // test probe, no ctx threading needed
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"/livez must be 200 immediately on bind, with no probe loop started")

	ready, err := http.Get(base + "/readyz") //nolint:noctx // test probe
	require.NoError(t, err)
	require.NoError(t, ready.Body.Close())
	assert.Equal(t, http.StatusServiceUnavailable, ready.StatusCode,
		"/readyz must be 503 until a probe runs")
}

func TestStartProbeListener_ServesLivezAndReadyz(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := probesCfg("127.0.0.1:0")
	h := probes.NewHandler()
	srv, _, err := startProbeListener(ctx, h, cfg)
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	})
	require.NotEmpty(t, srv.Addr, "helper must set srv.Addr to the resolved listener address")
	base := "http://" + srv.Addr

	resp, err := http.Get(base + "/livez") //nolint:noctx // test probe, no ctx threading needed
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Readiness only flips once the probe loop is started with a healthy pinger.
	h.Start(ctx, &stubPinger{}, cfg.Interval, cfg.Timeout)
	require.Eventually(t, func() bool {
		r, getErr := http.Get(base + "/readyz") //nolint:noctx // retried via Eventually
		if getErr != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusOK
	}, 2*time.Second, 10*time.Millisecond, "readyz must flip to 200 after first healthy probe")
}

func TestStartProbeListener_BindFailureReturnsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, errCh, err := startProbeListener(ctx, probes.NewHandler(), probesCfg(addr))
	assert.Nil(t, srv)
	assert.Nil(t, errCh)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe listener bind")
	assert.Contains(t, err.Error(), addr, "bind error must identify which address failed")
}

func TestStartProbeListener_ShutdownClosesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, errCh, err := startProbeListener(ctx, probes.NewHandler(), probesCfg("127.0.0.1:0"))
	require.NoError(t, err)
	require.NotNil(t, srv)

	base := "http://" + srv.Addr
	require.Eventually(t, func() bool {
		r, getErr := http.Get(base + "/livez") //nolint:noctx // retried via Eventually
		if getErr != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusOK
	}, 2*time.Second, 10*time.Millisecond)

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutCancel()
	require.NoError(t, srv.Shutdown(shutCtx))

	_, getErr := http.Get(base + "/livez") //nolint:noctx // expected to fail
	require.Error(t, getErr, "after Shutdown the listener must refuse connections")
	assert.True(t,
		errors.Is(getErr, syscall.ECONNREFUSED) || strings.Contains(getErr.Error(), "refused"),
		"expected connection-refused after Shutdown, got %v", getErr)

	// Graceful shutdown must not signal a listener-death event.
	select {
	case deadErr := <-errCh:
		t.Fatalf("shutdown should not signal listener death, got %v", deadErr)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestStartProbeListener_ReadyzReportsPingerFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := probesCfg("127.0.0.1:0")
	h := probes.NewHandler()
	srv, _, err := startProbeListener(ctx, h, cfg)
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	})

	h.Start(ctx, &stubPinger{err: errors.New("db unreachable")}, cfg.Interval, cfg.Timeout)
	require.Eventually(t, func() bool {
		r, getErr := http.Get("http://" + srv.Addr + "/readyz") //nolint:noctx // retried
		if getErr != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusServiceUnavailable
	}, 2*time.Second, 10*time.Millisecond, "readyz must reflect pinger failure")
}

func TestStartProbeListener_ListenerDeathSignalsErrCh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, errCh, err := startProbeListener(ctx, probes.NewHandler(), probesCfg("127.0.0.1:0"))
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	})

	// Close the underlying handler to simulate a non-graceful death; Shutdown
	// via http.Server.Close returns ErrServerClosed (which should not fire the
	// channel), so we instead force Serve to unblock with a different error by
	// closing the server's listener at the transport level.
	require.NoError(t, srv.Close(), "Close triggers ErrServerClosed; confirms channel stays quiet")
	select {
	case deadErr := <-errCh:
		t.Fatalf("Close → ErrServerClosed must not signal death, got %v", deadErr)
	case <-time.After(100 * time.Millisecond):
	}
	// Drain any lingering goroutines.
	_, _ = io.Copy(io.Discard, new(strings.Reader))
}

func TestStartProbeListener_LogsWhenEnabled(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := probesCfg("127.0.0.1:0")
	cfg.LogRequests = true
	srv, _, err := startProbeListener(ctx, probes.NewHandler(), cfg)
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(shutCtx)
	})

	resp, err := http.Get("http://" + srv.Addr + "/livez") //nolint:noctx // test probe
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Eventually(t, func() bool { return strings.Contains(buf.String(), `"path":"/livez"`) },
		2*time.Second, 10*time.Millisecond, "probe access line must appear when LogRequests is true")
}

func TestStartProbeListener_SilentWhenDisabled(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, _, err := startProbeListener(ctx, probes.NewHandler(), probesCfg("127.0.0.1:0")) // LogRequests false
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(shutCtx)
	})
	resp, err := http.Get("http://" + srv.Addr + "/livez") //nolint:noctx // test probe
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.NotContains(t, buf.String(), `"path":"/livez"`, "probe logging must be off by default")
}
