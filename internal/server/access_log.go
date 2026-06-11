// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/specgraph/specgraph/internal/reqctx"
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

// AccessLog logs one line per request. It seeds a reqctx.RequestInfo carrier
// (filled by the Connect interceptor and the auth layer), captures status and
// byte count via httpsnoop.Wrap (which preserves Flusher/Hijacker for the MCP
// streamable endpoint), and emits the line in a defer so a panic still logs.
//
// The line is emitted with the post-next r.Context() so the telemetry-enriched
// slog handler can add trace_id/span_id/project. AccessLog deliberately does
// NOT add those itself (that would duplicate the enriched keys). Consequently,
// when telemetry is disabled there is no enrich handler, so access lines omit
// project/trace_id/span_id; identity still appears (it is carrier-sourced).
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, info := reqctx.NewContext(r.Context())
		r = r.WithContext(ctx)

		rec := &statusRecorder{status: http.StatusOK}
		ww := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) { rec.status = code; next(code) }
			},
			Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) {
					n, err := next(b)
					rec.bytes += n
					return n, err
				}
			},
		})

		defer func() {
			rec2 := recover()
			status := rec.status
			if rec2 != nil {
				status = http.StatusInternalServerError
			}
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.Int("bytes", rec.bytes),
				slog.String("remote_ip", remoteIP(r)),
			}
			if info.Procedure != "" {
				attrs = append(attrs, slog.String("procedure", info.Procedure))
			}
			if info.Code != "" {
				attrs = append(attrs, slog.String("code", info.Code))
			}
			if info.Identity != "" {
				attrs = append(attrs, slog.String("identity", info.Identity))
			}
			slog.LogAttrs(r.Context(), requestLevel(status, info.Code), "request", attrs...)
			if rec2 != nil {
				panic(rec2)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}
