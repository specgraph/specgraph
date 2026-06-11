// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLevel(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
		want   slog.Level
	}{
		{"2xx no code", 200, "", slog.LevelInfo},
		{"3xx", 302, "", slog.LevelInfo},
		{"4xx", 404, "", slog.LevelWarn},
		{"5xx", 500, "", slog.LevelError},
		{"ok code on 200", 200, "ok", slog.LevelInfo},
		{"invalid_argument on 200", 200, "invalid_argument", slog.LevelWarn},
		{"internal on 200", 200, "internal", slog.LevelError},
		{"unknown on 200", 200, "unknown", slog.LevelError},
		{"client code never lowers 5xx", 500, "invalid_argument", slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, requestLevel(tt.status, tt.code))
		})
	}
}

func TestRemoteIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = "10.0.0.5:54321"
	assert.Equal(t, "10.0.0.5", remoteIP(r), "falls back to RemoteAddr host")

	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	assert.Equal(t, "203.0.113.7", remoteIP(r), "uses first XFF hop when present")
}

// captureLogs swaps slog.Default for a JSON logger writing to a buffer.
func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return buf, func() { slog.SetDefault(prev) }
}

func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	require.NotEmpty(t, line, "expected one access-log line")
	require.NotContains(t, line, "\n", "expected exactly one line")
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &m))
	return m
}

func TestAccessLog_BasicLineFields(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/things?token=secret", http.NoBody)
	req.Header.Set("Authorization", "Bearer supersecret")
	req.RemoteAddr = "10.0.0.5:1234"
	h.ServeHTTP(httptest.NewRecorder(), req)

	m := decodeLine(t, buf)
	assert.Equal(t, "request", m["msg"])
	assert.Equal(t, "GET", m["method"])
	assert.Equal(t, "/v1/things", m["path"], "query string must be stripped")
	assert.Equal(t, float64(204), m["status"])
	assert.Equal(t, "10.0.0.5", m["remote_ip"])
	assert.Contains(t, m, "duration_ms")
	assert.NotContains(t, buf.String(), "secret", "no query/auth secrets in the line")
}

func TestAccessLog_CarrierEnrichment(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := reqctx.FromContext(r.Context())
		require.NotNil(t, info, "AccessLog must seed the carrier")
		info.Procedure = "/specgraph.v1.GraphService/AddEdge"
		info.Code = "invalid_argument"
		info.Identity = "apikey:k1"
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/rpc", http.NoBody))

	m := decodeLine(t, buf)
	assert.Equal(t, "/specgraph.v1.GraphService/AddEdge", m["procedure"])
	assert.Equal(t, "invalid_argument", m["code"])
	assert.Equal(t, "apikey:k1", m["identity"])
	assert.Equal(t, "WARN", m["level"], "invalid_argument escalates a 200 to WARN")
}

func TestAccessLog_PanicLogs500AndRepanics(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	h := AccessLog(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	assert.Panics(t, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", http.NoBody))
	}, "panic must propagate after logging")

	m := decodeLine(t, buf)
	assert.Equal(t, float64(500), m["status"])
	assert.Equal(t, "ERROR", m["level"])
}

func TestAccessLog_NoProjectKeyAddedByMiddleware(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()
	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	assert.NotContains(t, decodeLine(t, buf), "project")
}

func TestAccessLog_BytesAndImplicitStatus(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello ")) // 6 bytes, implicit 200
		_, _ = w.Write([]byte("world"))  // 5 bytes
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	m := decodeLine(t, buf)
	assert.Equal(t, float64(200), m["status"], "implicit 200 when a body is written without WriteHeader")
	assert.Equal(t, float64(11), m["bytes"], "bytes must accumulate across multiple Write calls")
}

// hijackableRecorder is an httptest recorder that also satisfies http.Hijacker,
// so we can assert AccessLog's wrapper preserves both Flusher and Hijacker
// (required for the MCP streamable/SSE endpoint).
type hijackableRecorder struct {
	*httptest.ResponseRecorder
}

func (hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) { return nil, nil, nil }

func TestAccessLog_PreservesFlusherAndHijacker(t *testing.T) {
	var okFlusher, okHijacker bool
	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, okFlusher = w.(http.Flusher)
		_, okHijacker = w.(http.Hijacker)
		w.WriteHeader(http.StatusOK)
	}))
	rw := hijackableRecorder{httptest.NewRecorder()}
	h.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	assert.True(t, okFlusher, "wrapped ResponseWriter must still satisfy http.Flusher")
	assert.True(t, okHijacker, "wrapped ResponseWriter must still satisfy http.Hijacker")
}
