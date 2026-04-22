// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPinger struct {
	err error
}

func (p *stubPinger) Ping(_ context.Context) error { return p.err }

func TestStartProbeListener_DisabledWhenEmpty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := startProbeListener(ctx, &stubPinger{}, "", 5*time.Millisecond, 50*time.Millisecond)
	require.NoError(t, err)
	assert.Nil(t, srv, "empty addr disables probes — no listener should be created")
}

func TestStartProbeListener_ServesLivezAndReadyz(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := startProbeListener(ctx, &stubPinger{}, "127.0.0.1:0", 5*time.Millisecond, 50*time.Millisecond)
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

	srv, err := startProbeListener(ctx, &stubPinger{}, addr, 5*time.Millisecond, 50*time.Millisecond)
	assert.Nil(t, srv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe listener bind")
	assert.Contains(t, err.Error(), addr, "bind error must identify which address failed")
}

func TestStartProbeListener_ShutdownClosesListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := startProbeListener(ctx, &stubPinger{}, "127.0.0.1:0", 5*time.Millisecond, 50*time.Millisecond)
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
}

func TestStartProbeListener_ReadyzReportsPingerFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinger := &stubPinger{err: errors.New("db unreachable")}
	srv, err := startProbeListener(ctx, pinger, "127.0.0.1:0", 5*time.Millisecond, 50*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, srv)
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	})

	require.Eventually(t, func() bool {
		r, getErr := http.Get("http://" + srv.Addr + "/readyz") //nolint:noctx // retried
		if getErr != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusServiceUnavailable
	}, 2*time.Second, 10*time.Millisecond, "readyz must reflect pinger failure")
}
