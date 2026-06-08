// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package probes_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHandler_LivezUpBeforeProbing locks in the core liveness-decoupling
// guarantee: a Handler that has NOT been Started answers /livez with 200 (so
// kubelet never kills the pod while the gated dependency connects) and /readyz
// with a silent 503 — without spawning a probe goroutine or logging anything.
func TestNewHandler_LivezUpBeforeProbing(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	h := probes.NewHandler()

	livez := httptest.NewRecorder()
	h.Livez(livez, httptest.NewRequest(http.MethodGet, "/livez", http.NoBody))
	assert.Equal(t, http.StatusOK, livez.Code,
		"/livez must be 200 before any probe runs — liveness is storage-independent")

	readyz := httptest.NewRecorder()
	h.Readyz(readyz, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
	assert.Equal(t, http.StatusServiceUnavailable, readyz.Code,
		"/readyz must be 503 until the first probe completes")
	assert.Contains(t, readyz.Body.String(), "probe has not yet completed")

	assert.NotContains(t, buf.String(), "readiness probe",
		"a bound-but-not-started handler must be silent — no spurious readiness log on boot")
}

// TestHandler_Start_HealthyPinger_FlipsReadyzSilently proves Start begins
// probing and, when the pinger is already healthy (happy path: store set before
// Start), flips /readyz to 200 without logging a spurious failure.
func TestHandler_Start_HealthyPinger_FlipsReadyzSilently(t *testing.T) {
	buf := &syncBuffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := probes.NewHandler()
	h.Start(ctx, &fakePinger{}, 5*time.Millisecond, 50*time.Millisecond)

	require.Eventually(t, func() bool {
		rec := httptest.NewRecorder()
		h.Readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		return rec.Code == http.StatusOK
	}, 2*time.Second, 5*time.Millisecond, "Start must probe and flip /readyz to 200 when the pinger is healthy")

	assert.NotContains(t, buf.String(), "readiness probe failed",
		"a healthy first probe must stay silent — no spurious WARN")
}
