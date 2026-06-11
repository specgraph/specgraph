// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// requestLevel picks the access-log level as the more severe of the HTTP
// status level and the connect-code level. gRPC/gRPC-Web errors arrive as
// HTTP 200 with the code in a trailer, so the code dimension is load-bearing.
func requestLevel(status int, code string) slog.Level {
	level := slog.LevelInfo
	switch {
	case status >= 500:
		level = slog.LevelError
	case status >= 400:
		level = slog.LevelWarn
	}
	switch code {
	case "", "ok":
		// status governs
	case "internal", "unknown", "unavailable", "data_loss", "deadline_exceeded":
		if level < slog.LevelError {
			level = slog.LevelError
		}
	default:
		if level < slog.LevelWarn {
			level = slog.LevelWarn
		}
	}
	return level
}

// remoteIP returns the first X-Forwarded-For hop when present (NOTE:
// client-spoofable unless a trusted proxy overwrites it — informational only),
// else the host part of RemoteAddr.
func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// statusRecorder captures the response status and byte count for the access
// log. status defaults to 200 (handlers that write a body without an explicit
// WriteHeader implicitly send 200).
type statusRecorder struct {
	status int
	bytes  int
}
