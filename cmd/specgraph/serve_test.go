// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer protects a bytes.Buffer so concurrent slog Writes from any
// probe or background goroutine do not race with the test goroutine reading
// via String.
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

func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return buf
}

func TestIsLoopbackAddr(t *testing.T) {
	tests := []struct {
		addr     string
		loopback bool
	}{
		{"127.0.0.1:7890", true},
		{"localhost:7890", true},
		{"::1:7890", false}, // net.SplitHostPort sees "::1" as invalid host:port — edge case
		{"[::1]:7890", true},
		{"0.0.0.0:7890", false},
		{"192.168.1.100:7890", false},
		{"10.0.0.1:9090", false},
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			assert.Equal(t, tt.loopback, isLoopbackAddr(tt.addr))
		})
	}
}

func TestValidateServerConfig_RejectsInvalidProbes(t *testing.T) {
	cfg := &config.GlobalConfig{
		Server: config.ServerSection{
			Probes: config.ProbesConfig{Interval: -1 * time.Second},
		},
	}
	_, err := validateServerConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid probes config")
	assert.Contains(t, err.Error(), "probes.interval must be non-negative")
}

func TestValidateServerConfig_ResolvesValidProbes(t *testing.T) {
	cfg := &config.GlobalConfig{
		Server: config.ServerSection{
			Probes: config.ProbesConfig{Listen: "127.0.0.1:9091"},
		},
	}
	probesCfg, err := validateServerConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9091", probesCfg.Listen)
	assert.Equal(t, config.DefaultProbeInterval, probesCfg.Interval)
	assert.Equal(t, config.DefaultProbeTimeout, probesCfg.Timeout)
}

func TestWarnIfNoAuthOnPublicListen(t *testing.T) {
	cases := []struct {
		name     string
		listen   string
		hasAuth  bool
		wantWarn bool
	}{
		{name: "default listen without auth fires", listen: "0.0.0.0:9090", hasAuth: false, wantWarn: true},
		{name: "default listen with auth silent", listen: "0.0.0.0:9090", hasAuth: true, wantWarn: false},
		{name: "loopback without auth silent", listen: "127.0.0.1:9090", hasAuth: false, wantWarn: false},
		{name: "loopback with auth silent", listen: "127.0.0.1:9090", hasAuth: true, wantWarn: false},
		{name: "non-loopback IPv4 without auth fires", listen: "10.0.0.5:9090", hasAuth: false, wantWarn: true},
		{name: "empty listen treated as non-loopback", listen: "", hasAuth: false, wantWarn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureSlog(t)
			warnIfNoAuthOnPublicListen(tc.listen, tc.hasAuth)
			out := buf.String()
			if tc.wantWarn {
				assert.Contains(t, out, "server listening without authentication on non-loopback interface")
				assert.Contains(t, out, "level=WARN")
				if tc.listen != "" {
					assert.Contains(t, out, "addr="+tc.listen)
				}
			} else {
				assert.NotContains(t, out, "server listening without authentication")
			}
		})
	}
}

func TestWarnIfNoAuthOnPublicListen_EmitsOnceForDefault(t *testing.T) {
	buf := captureSlog(t)
	warnIfNoAuthOnPublicListen("0.0.0.0:9090", false)
	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "server listening without authentication"),
		"warning must fire exactly once per call")
}
