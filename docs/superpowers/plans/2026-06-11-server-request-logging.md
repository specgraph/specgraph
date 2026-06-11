# Per-request Access Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit one idiomatic access-log line per request (HTTP fields + RPC procedure/code/identity, outcome-based level), with probe-endpoint logging gated behind config.

**Architecture:** An edge HTTP middleware `AccessLog` (innermost, around the mux) emits the single line; an outermost Connect interceptor and the auth layer enrich a shared mutable `*reqctx.RequestInfo` carrier passed through `context`. `project`/`trace_id` come from the existing telemetry-enriched slog handler (the middleware must not re-add them). Probe-listener logging is wrapped only when `server.probes.log_requests` is true; the master switch is `log.requests` (default true).

**Tech Stack:** Go, `log/slog`, `connectrpc.com/connect` v1.19.1, `github.com/felixge/httpsnoop` v1.0.4 (already in `go.sum`, indirect).

**Design spec:** `docs/superpowers/specs/2026-06-11-server-request-logging-design.md`

**Key constraint (import cycle):** `internal/server` imports `internal/auth`, so the carrier CANNOT live in `internal/server` (auth could not import it back). It lives in a new leaf package `internal/reqctx` that imports only `context`; both `internal/server` and `internal/auth` import `reqctx`.

---

## File structure

- `internal/reqctx/reqctx.go` (new) — the `RequestInfo` carrier + context helpers. Leaf package.
- `internal/reqctx/reqctx_test.go` (new) — carrier tests.
- `internal/server/access_log.go` (new) — `AccessLog` middleware, status recorder, `remoteIP`, `requestLevel`.
- `internal/server/access_log_test.go` (new) — middleware tests.
- `internal/server/access_log_interceptor.go` (new) — `AccessLogInterceptor` + `connectCode`.
- `internal/server/access_log_interceptor_test.go` (new) — interceptor tests.
- `internal/auth/interceptor.go` (modify) — write `info.Identity` on the unary path.
- `internal/auth/middleware.go` (modify) — write `info.Identity` in `RequireAuth`.
- `internal/auth/identity_carrier_test.go` (new) — identity-write tests for both paths.
- `internal/config/global.go` (modify) — `LogConfig.Requests` (default true) + `ProbesConfig.LogRequests`.
- `internal/config/loader_internal_test.go` (modify) — config default/override tests.
- `cmd/specgraph/serve.go` (modify) — prepend interceptor; wrap main chain; pass probe flag.
- `cmd/specgraph/serve_probes_test.go` (modify) — probe access-log test.
- `go.mod` (modify via `go mod tidy`) — httpsnoop indirect → direct.

---

## Task 1: `reqctx` carrier package

**Files:**

- Create: `internal/reqctx/reqctx.go`
- Test: `internal/reqctx/reqctx_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package reqctx_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromContext_AbsentReturnsNil(t *testing.T) {
	assert.Nil(t, reqctx.FromContext(context.Background()),
		"no carrier seeded → FromContext must return nil so writers can guard")
}

func TestNewContext_RoundTripAndPointerMutationVisible(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())
	require.NotNil(t, info)

	// A downstream layer looks the carrier up and mutates it through the pointer.
	got := reqctx.FromContext(ctx)
	require.Same(t, info, got, "FromContext must return the same pointer NewContext seeded")
	got.Procedure = "/svc/Method"
	got.Code = "ok"
	got.Identity = "apikey:abc"

	// The seeder's original pointer observes the mutation (no re-injection needed).
	assert.Equal(t, "/svc/Method", info.Procedure)
	assert.Equal(t, "ok", info.Code)
	assert.Equal(t, "apikey:abc", info.Identity)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/reqctx/ -run TestFromContext -v`
Expected: FAIL — `package github.com/specgraph/specgraph/internal/reqctx is not in std` / undefined `reqctx`.

- [ ] **Step 3: Write minimal implementation**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package reqctx carries request-scoped log fields that are produced by
// different layers (HTTP edge middleware, Connect interceptor, auth) and
// consumed by the access-log middleware. It is a leaf package (imports only
// context) so both internal/server and internal/auth can use it without an
// import cycle.
package reqctx

import "context"

// RequestInfo holds fields the access-log line cannot otherwise obtain at the
// HTTP edge: the RPC procedure, the connect outcome code, and the
// authenticated principal. Fields are exported because separate packages set
// them. A pointer is seeded into the context by NewContext; inner layers
// mutate it in place, and the seeder reads it after the handler returns.
type RequestInfo struct {
	Procedure string // RPC procedure, e.g. /specgraph.v1.GraphService/AddEdge
	Code      string // connect code string: "ok", "invalid_argument", ...
	Identity  string // authenticated principal (Identity.Subject), or ""
}

type ctxKey struct{}

// NewContext returns a context carrying a fresh *RequestInfo plus that pointer.
func NewContext(ctx context.Context) (context.Context, *RequestInfo) {
	info := &RequestInfo{}
	return context.WithValue(ctx, ctxKey{}, info), info
}

// FromContext returns the *RequestInfo carried by ctx, or nil if none.
func FromContext(ctx context.Context) *RequestInfo {
	info, _ := ctx.Value(ctxKey{}).(*RequestInfo)
	return info
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/reqctx/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/reqctx/
git commit -s -m "feat(reqctx): request-scoped log-field carrier package"
```

---

## Task 2: Config fields (`log.requests`, `server.probes.log_requests`)

**Files:**

- Modify: `internal/config/global.go` (LogConfig ~73-80, ProbesConfig ~150-154, globalDefaults ~384-403)
- Test: `internal/config/loader_internal_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/config/loader_internal_test.go`)

```go
func TestGlobalDefaults_LogRequestsDefaultsTrue(t *testing.T) {
	assert.True(t, globalDefaults().Log.Requests,
		"access logging must be on by default")
	assert.False(t, globalDefaults().Server.Probes.LogRequests,
		"probe-request logging must be off by default")
}

func TestLoadGlobal_LogRequestsExplicitFalseOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("log:\n  requests: false\n"), 0o600))

	cfg, err := LoadGlobalExplicit(path)
	require.NoError(t, err)
	assert.False(t, cfg.Log.Requests, "explicit log.requests:false must override the true default")
}
```

Ensure the test file imports `os`, `path/filepath`, `github.com/stretchr/testify/assert`, `github.com/stretchr/testify/require` (add any missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run 'TestGlobalDefaults_LogRequests|TestLoadGlobal_LogRequestsExplicit' -v`
Expected: FAIL — `cfg.Log.Requests undefined` / `Probes.LogRequests undefined`.

- [ ] **Step 3: Write minimal implementation**

In `internal/config/global.go`, add the field to `LogConfig` (after `Output`):

```go
	// Requests enables one access-log line per request on the main listener.
	Requests bool `yaml:"requests" koanf:"requests"`
```

Add the field to `ProbesConfig` (after `Timeout`):

```go
	// LogRequests enables access logging on the probe listener. Off by default
	// so kubelet's frequent /livez,/readyz polls don't flood the logs.
	LogRequests bool `yaml:"log_requests,omitempty" koanf:"log_requests"`
```

In `globalDefaults()`, set `Requests: true` in the `Log` literal:

```go
		Log: LogConfig{
			Level:    "info",
			Format:   "json",
			Output:   "stdout",
			Requests: true,
		},
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run 'TestGlobalDefaults_LogRequests|TestLoadGlobal_LogRequestsExplicit' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/global.go internal/config/loader_internal_test.go
git commit -s -m "feat(config): add log.requests + server.probes.log_requests"
```

---

## Task 3: Level mapping + status recorder + remote_ip helper

**Files:**

- Create: `internal/server/access_log.go`
- Test: `internal/server/access_log_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run 'TestRequestLevel|TestRemoteIP' -v`
Expected: FAIL — `undefined: requestLevel` / `undefined: remoteIP`.

- [ ] **Step 3: Write minimal implementation** (create `internal/server/access_log.go`)

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run 'TestRequestLevel|TestRemoteIP' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/access_log.go internal/server/access_log_test.go
git commit -s -m "feat(server): access-log level mapping + remote_ip helper"
```

---

## Task 4: `AccessLog` middleware

**Files:**

- Modify: `internal/server/access_log.go`
- Modify: `internal/server/access_log_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/server/access_log_test.go`)

Add imports: `bytes`, `context`, `encoding/json`, `strings`, `github.com/felixge/httpsnoop` is NOT needed in the test, `github.com/specgraph/specgraph/internal/reqctx`, `github.com/stretchr/testify/require`.

```go
// captureLogs swaps slog.Default for a JSON logger writing to a buffer and
// returns the buffer + a restore func.
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

	// Inner handler simulates the interceptor/auth filling the carrier.
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
	// AccessLog must NOT emit project itself (the enrich handler owns it).
	assert.NotContains(t, decodeLine(t, buf), "project")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestAccessLog -v`
Expected: FAIL — `undefined: AccessLog`.

- [ ] **Step 3: Write minimal implementation** (append to `internal/server/access_log.go`)

Add imports to the file: `time`, `github.com/felixge/httpsnoop`, `github.com/specgraph/specgraph/internal/reqctx`.

```go
// AccessLog logs one line per request. It seeds a reqctx.RequestInfo carrier
// (filled by the Connect interceptor and the auth layer), captures status and
// byte count via httpsnoop.Wrap (which preserves Flusher/Hijacker for the MCP
// streamable endpoint), and emits the line in a defer so a panic still logs.
//
// The line is emitted with the post-next r.Context() so the telemetry-enriched
// slog handler can add trace_id/span_id/project. AccessLog deliberately does
// NOT add those itself (that would duplicate the enriched keys).
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestAccessLog -v`
Expected: PASS (all four).

- [ ] **Step 5: Commit**

```bash
git add internal/server/access_log.go internal/server/access_log_test.go
git commit -s -m "feat(server): AccessLog edge middleware (one line per request)"
```

---

## Task 5: `AccessLogInterceptor` + `connectCode`

**Files:**

- Create: `internal/server/access_log_interceptor.go`
- Test: `internal/server/access_log_interceptor_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectCode(t *testing.T) {
	assert.Equal(t, "ok", connectCode(nil))
	assert.Equal(t, "unknown", connectCode(errors.New("plain")))
	assert.Equal(t, "invalid_argument",
		connectCode(connect.NewError(connect.CodeInvalidArgument, errors.New("x"))))
}

// fakeRequest is a minimal connect.AnyRequest for Spec().Procedure.
func TestAccessLogInterceptor_FillsCarrier(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())

	ic := AccessLogInterceptor()
	var called bool
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&struct{}{}), nil
	})
	// Wrap and invoke with a request carrying a Spec.
	req := connect.NewRequest(&struct{}{})
	req.Spec().Procedure = "/svc/M" // set below via helper if needed
	_, err := ic.WrapUnary(next)(ctx, req)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", info.Code)
}

func TestAccessLogInterceptor_CapturesErrorCodeFromInner(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())
	ic := AccessLogInterceptor()
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	})
	_, err := ic.WrapUnary(next)(ctx, connect.NewRequest(&struct{}{}))
	require.Error(t, err)
	assert.Equal(t, "permission_denied", info.Code,
		"outermost interceptor must observe an inner reject-without-next code")
}

func TestAccessLogInterceptor_NoCarrierIsNoop(t *testing.T) {
	ic := AccessLogInterceptor()
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	// context.Background() has no carrier; must not panic.
	_, err := ic.WrapUnary(next)(context.Background(), connect.NewRequest(&struct{}{}))
	require.NoError(t, err)
}
```

> NOTE for the implementer: `connect.NewRequest(&struct{}{}).Spec().Procedure` is empty in a unit test (Spec is populated by the framework). The `FillsCarrier` test asserts `Code`, which is reliable; if you want to assert `Procedure`, drive it through a real handler/client instead. Keep the `Code` assertions as the contract.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run 'TestConnectCode|TestAccessLogInterceptor' -v`
Expected: FAIL — `undefined: connectCode` / `undefined: AccessLogInterceptor`.

- [ ] **Step 3: Write minimal implementation** (create `internal/server/access_log_interceptor.go`)

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/reqctx"
)

// connectCode returns the access-log code string for an RPC outcome.
// connect.CodeOf(nil) is Unknown, so nil is special-cased to "ok" first; a
// plain (non-connect) handler error yields "unknown" → ERROR, the desired
// unexpected-failure signal.
func connectCode(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}

// AccessLogInterceptor enriches the reqctx carrier with the RPC procedure and
// outcome code. It never logs — the edge AccessLog middleware emits the line.
// It MUST be registered OUTERMOST (first in the interceptor list) so it runs
// even when an inner interceptor (auth) rejects without calling next, and so
// it observes the final outcome code.
func AccessLogInterceptor() connect.Interceptor {
	return interceptorFunc{}
}

type interceptorFunc struct{}

func (interceptorFunc) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if info := reqctx.FromContext(ctx); info != nil {
			info.Procedure = req.Spec().Procedure
			info.Code = connectCode(err)
		}
		return resp, err
	}
}

func (interceptorFunc) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // server process only; no client enrichment
}

func (interceptorFunc) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if info := reqctx.FromContext(ctx); info != nil {
			info.Procedure = conn.Spec().Procedure
			info.Code = connectCode(err)
		}
		return err
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run 'TestConnectCode|TestAccessLogInterceptor' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/access_log_interceptor.go internal/server/access_log_interceptor_test.go
git commit -s -m "feat(server): AccessLogInterceptor enriches carrier with procedure+code"
```

---

## Task 6: Identity capture in the auth layer

**Files:**

- Modify: `internal/auth/interceptor.go:27-46`
- Modify: `internal/auth/middleware.go:17-29`
- Test: `internal/auth/identity_carrier_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResolver resolves any token to a fixed identity.
type stubResolver struct{ id *Identity }

func (s stubResolver) Resolve(context.Context, string) (*Identity, error) { return s.id, nil }

func TestRequireAuth_WritesIdentityToCarrier(t *testing.T) {
	res := stubResolver{id: &Identity{Subject: "apikey:k1", Role: RoleAdmin}}
	ctx, info := reqctx.NewContext(context.Background())

	var sawIdentity string
	h := RequireAuth(res)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if info := reqctx.FromContext(r.Context()); info != nil {
			sawIdentity = info.Identity
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", http.NoBody).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "apikey:k1", sawIdentity, "RequireAuth must write identity into the carrier")
	assert.Equal(t, "apikey:k1", info.Identity)
}
```

> NOTE: adjust the `Identity` literal fields (`Role`, etc.) to whatever the auth package requires to be non-zero for `RequireAuth` to proceed; the assertion is on `info.Identity`. If `RoleAdmin` is not a valid identifier, use any valid `Role` constant from the package.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestRequireAuth_WritesIdentityToCarrier -v`
Expected: FAIL — `info.Identity` is empty (no write yet).

- [ ] **Step 3: Write minimal implementation**

In `internal/auth/middleware.go`, add the import `"github.com/specgraph/specgraph/internal/reqctx"` and write the identity after successful authenticate (replace the final `next.ServeHTTP(...)` line, ~29):

```go
			if info := reqctx.FromContext(r.Context()); info != nil {
				info.Identity = id.Subject
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
```

In `internal/auth/interceptor.go`, add the import `"github.com/specgraph/specgraph/internal/reqctx"` and write the identity right after `id` is resolved (after line 30, before `authorizer.Authorize`), so even an authorization denial still logs the principal:

```go
			if info := reqctx.FromContext(ctx); info != nil {
				info.Identity = id.Subject
			}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -run TestRequireAuth_WritesIdentityToCarrier -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/interceptor.go internal/auth/identity_carrier_test.go
git commit -s -m "feat(auth): record authenticated subject in request-log carrier"
```

---

## Task 7: Wire into the main listener

**Files:**

- Modify: `cmd/specgraph/serve.go:173-182` (interceptor list), `:233-239` (handler chain)

- [ ] **Step 1: Prepend the interceptor**

In `cmd/specgraph/serve.go`, change the interceptor assembly so `AccessLogInterceptor` is FIRST (outermost). Replace lines ~173-181:

```go
	interceptors := []connect.Interceptor{server.AccessLogInterceptor()} // outermost: always runs, sees final code
	otelIC, otelErr := telemetry.ServerInterceptor(telState.enabled)
	if otelErr != nil {
		return appHandler{}, fmt.Errorf("otel interceptor: %w", otelErr)
	}
	if otelIC != nil {
		interceptors = append(interceptors, otelIC)
	}
	interceptors = append(interceptors, interceptor) // existing auth interceptor
```

- [ ] **Step 2: Wrap the mux with AccessLog when enabled**

Replace the handler-chain line (~233):

```go
	var rootHandler http.Handler = mux
	if cfg.Log.Requests {
		rootHandler = server.AccessLog(mux)
	}
	handler := server.SecurityHeaders(server.ProjectMiddleware(rootHandler))
```

(Leave the subsequent `telemetry.WrapHTTPHandler` and `CORSMiddleware` wrapping unchanged — they stay outer, so AccessLog's context has the span + project.)

- [ ] **Step 3: Build**

Run: `go build ./cmd/specgraph/`
Expected: no output (success).

- [ ] **Step 4: Run the package tests**

Run: `go test ./cmd/specgraph/ ./internal/server/ ./internal/auth/`
Expected: PASS (web/build placeholder must exist; if `pattern all:build` error, run `mkdir -p web/build && printf '<!doctype html>' > web/build/index.html`).

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): wire access logging (outermost interceptor + edge middleware)"
```

---

## Task 8: Gate + wire probe-listener logging

**Files:**

- Modify: `cmd/specgraph/serve.go:688` (`startProbeListener`)
- Test: `cmd/specgraph/serve_probes_test.go`

- [ ] **Step 1: Write the failing test** (append to `cmd/specgraph/serve_probes_test.go`)

Add imports if missing: `bytes`, `encoding/json`, `log/slog`, `strings`.

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/specgraph/ -run TestStartProbeListener_Logs -v`
Expected: FAIL — `cfg.LogRequests` field exists (Task 2) but `startProbeListener` does not wrap with AccessLog, so no `/livez` line.

- [ ] **Step 3: Implement the wrap** (in `startProbeListener`, where the `http.Server` Handler is set, ~at the `Handler: handler.Mux()` assignment)

Replace the handler assignment so the mux is wrapped when enabled:

```go
	var probeHandler http.Handler = handler.Mux()
	if cfg.LogRequests {
		probeHandler = server.AccessLog(probeHandler)
	}
	srv := &http.Server{
		Addr:              ln.Addr().String(),
		Handler:           probeHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
```

Ensure `cmd/specgraph/serve.go` imports `github.com/specgraph/specgraph/internal/server` (it already does) and `net/http` (already does).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/specgraph/ -run TestStartProbeListener -v`
Expected: PASS (both new tests + the existing probe tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/serve.go cmd/specgraph/serve_probes_test.go
git commit -s -m "feat(serve): gate probe-listener access logging on server.probes.log_requests"
```

---

## Task 9: Tidy module + full gate

**Files:**

- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Promote httpsnoop to a direct dependency**

Run: `go mod tidy`
Expected: `github.com/felixge/httpsnoop` loses its `// indirect` comment in `go.mod`.

- [ ] **Step 2: Run the full unit suite**

Run: `go test ./...`
Expected: PASS (exit 0). If a `web/build` embed error appears, create the placeholder (`mkdir -p web/build && printf '<!doctype html>' > web/build/index.html`) and re-run.

- [ ] **Step 3: Race + lint the touched packages**

Run: `go test -race ./internal/server/ ./internal/auth/ ./internal/reqctx/ ./cmd/specgraph/`
Run: `golangci-lint run ./internal/server/... ./internal/auth/... ./internal/reqctx/... ./cmd/specgraph/...`
Expected: PASS / `0 issues`.

- [ ] **Step 4: Quality gate**

Run: `task check`
Expected: PASS (fmt, license headers, lint, build, version-stamp guard, unit tests). Run `task license:add` if the license-header check flags the new files.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -s -m "build: promote felixge/httpsnoop to a direct dependency"
```

---

## Self-review notes (verified against the spec)

- **Coverage:** carrier (Task 1) ↔ spec §1; config (Task 2) ↔ spec §Config; level/remote_ip (Task 3) ↔ spec §7/§2; `AccessLog` incl. panic + post-`next` ctx + no-`project` (Task 4) ↔ spec §2/§Field-ownership; interceptor outermost + `connectCode` nil/unknown (Task 5) ↔ spec §3/§7; identity writes unary + RequireAuth (Task 6) ↔ spec §4; serve wiring prepend + chain placement (Task 7) ↔ spec §3/§5; probe gating (Task 8) ↔ spec §6; httpsnoop direct dep (Task 9) ↔ spec §Files.
- **Type consistency:** `reqctx.RequestInfo{Procedure,Code,Identity}`, `reqctx.NewContext`/`FromContext`, `server.AccessLog`, `server.AccessLogInterceptor`, `connectCode`, `requestLevel`, `remoteIP`, `statusRecorder`, `LogConfig.Requests`, `ProbesConfig.LogRequests` are used identically across tasks.
- **Known doc-level caveats carried from the spec (not bugs):** streaming RPCs carry no `identity` (unary auth interceptor); panic lines carry empty `code` (status drives ERROR); telemetry-off lines omit `project`/`trace_id`; a panic also yields net/http's secondary plain `ErrorLog` line.
