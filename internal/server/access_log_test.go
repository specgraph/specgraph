// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
