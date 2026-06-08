# Non-fatal Postgres Startup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `server.docker: false` and the external Postgres is unreachable at boot, the `specgraph serve` process stays alive (main port returns HTTP 503, `/readyz` returns 503) and self-heals when Postgres becomes reachable, instead of exiting non-zero.

**Architecture:** Refactor `cmd/specgraph/serve.go` so the PG-touching prefix (`postgres.New → Subscribe → NewAuth → bootstrap.Ensure`) becomes a re-runnable "connect unit" with partial-pool cleanup, the rest of the wiring runs once after a successful connect, and the main `http.Server` serves through an atomically-swappable handler that starts as a blanket-503 and is replaced with the real handler on connect. A `readinessPinger` lets the existing `/readyz` probe report the live PG state. A background connector retries with classified backoff (bad credentials: 5×1s then fatal; everything else: 1s→1m, forever, loud).

**Tech Stack:** Go, ConnectRPC, pgx v5 (`pgconn.PgError` for SQLSTATE classification), `sync/atomic`, cobra, slog, testify, testcontainers (integration).

**Spec:** `docs/superpowers/specs/2026-06-08-non-fatal-postgres-startup-design.md`

---

## File Structure

New files (keep `serve.go` focused):

- `cmd/specgraph/readiness.go` — `readinessPinger` (probes.Pinger over an atomic inner pinger), `atomicHandler` (swappable `http.Handler`), `notReadyHandler` (blanket 503).
- `cmd/specgraph/readiness_test.go` — unit tests for the above.
- `cmd/specgraph/connector.go` — `connectResult`, `classifyConnectError`, `connectStore` (retry unit + cleanup), `runConnector` + `firstConnect` (backoff/classification), `ctxSleep`, `backoffPolicy`.
- `cmd/specgraph/connector_test.go` — unit tests for classification + connector loop.
- `cmd/specgraph/serve_degraded_integration_test.go` — `//go:build integration` end-to-end degraded→recovery, credential-fatal.

Modified files:

- `cmd/specgraph/serve.go` — extract `buildAppDeps` (PG-independent) and `buildAppHandler` (PG-dependent wiring), add `startMainServer`, rewrite `runServe` control flow.

All new `.go` files need the license header:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

---

## Task 1: readinessPinger

The `/readyz` probe takes a `probes.Pinger`. We need one whose backing store is nil at boot (reports not-ready) and is set later by the connector. Holding the inner pinger as an interface behind an atomic pointer makes "delegates after set" unit-testable with a stub.

**Files:**

- Create: `cmd/specgraph/readiness.go`
- Test: `cmd/specgraph/readiness_test.go`

- [ ] **Step 1: Write the failing test**

```go
// cmd/specgraph/readiness_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadinessPinger_NotReadyBeforeSet(t *testing.T) {
	p := newReadinessPinger()
	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreNotReady)
}

func TestReadinessPinger_DelegatesAfterSet(t *testing.T) {
	p := newReadinessPinger()

	p.set(&stubPinger{}) // stubPinger defined in serve_probes_test.go (same package)
	assert.NoError(t, p.Ping(context.Background()))

	want := errors.New("db down")
	p.set(&stubPinger{err: want})
	assert.ErrorIs(t, p.Ping(context.Background()), want)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/specgraph/ -run TestReadinessPinger -v`
Expected: FAIL — `undefined: newReadinessPinger`, `undefined: errStoreNotReady`.

- [ ] **Step 3: Write minimal implementation**

```go
// cmd/specgraph/readiness.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/specgraph/specgraph/internal/server/probes"
)

// errStoreNotReady is returned by readinessPinger.Ping before a live store has
// been installed, so /readyz reports 503 during the degraded window.
var errStoreNotReady = errors.New("storage not ready: postgres connection not yet established")

// readinessPinger adapts a not-yet-available backing pinger (the *postgres.Store)
// to probes.Pinger. The inner pinger lives behind an atomic pointer so the
// background connector can publish the live store without locking the probe
// goroutine. The double-pointer (atomic.Pointer to a struct wrapping the
// interface) is required because atomic.Pointer needs a concrete element type.
type readinessPinger struct {
	inner atomic.Pointer[livePinger]
}

type livePinger struct{ p probes.Pinger }

func newReadinessPinger() *readinessPinger { return &readinessPinger{} }

// set publishes the live backing pinger. *postgres.Store satisfies probes.Pinger.
func (rp *readinessPinger) set(p probes.Pinger) { rp.inner.Store(&livePinger{p: p}) }

// Ping reports errStoreNotReady until a backing pinger is set, then delegates.
func (rp *readinessPinger) Ping(ctx context.Context) error {
	lp := rp.inner.Load()
	if lp == nil || lp.p == nil {
		return errStoreNotReady
	}
	return lp.p.Ping(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/specgraph/ -run TestReadinessPinger -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/readiness.go cmd/specgraph/readiness_test.go
git commit -s -m "feat(serve): add readinessPinger for deferred postgres readiness"
```

---

## Task 2: atomicHandler + notReadyHandler

The main `http.Server.Handler` must be a fixed shim that loads the current handler from an atomic pointer each request (reassigning `srv.Handler` from another goroutine is a data race). The default handler is a blanket 503.

**Files:**

- Modify: `cmd/specgraph/readiness.go`
- Test: `cmd/specgraph/readiness_test.go`

- [ ] **Step 1: Write the failing test**

```go
// append to cmd/specgraph/readiness_test.go

func TestNotReadyHandler_Returns503(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)

	notReadyHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
	assert.Contains(t, rec.Body.String(), "storage not ready")
}

func TestAtomicHandler_SwapsBehaviour(t *testing.T) {
	a := newAtomicHandler(notReadyHandler())

	rec1 := httptest.NewRecorder()
	a.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec1.Code)

	a.set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec2 := httptest.NewRecorder()
	a.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTeapot, rec2.Code)
}
```

Add these imports to the test file's import block: `"net/http"`, `"net/http/httptest"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/specgraph/ -run 'TestNotReadyHandler|TestAtomicHandler' -v`
Expected: FAIL — `undefined: newAtomicHandler`, `undefined: notReadyHandler`.

- [ ] **Step 3: Write minimal implementation**

```go
// append to cmd/specgraph/readiness.go

// (add to the import block: "io", "net/http")

// atomicHandler is a fixed http.Handler whose delegate is swapped atomically.
// srv.Handler is set to the atomicHandler once and never reassigned; only the
// internal pointer changes, so concurrent swaps never race the http.Server's
// per-request Handler read.
type atomicHandler struct {
	h atomic.Pointer[http.Handler]
}

func newAtomicHandler(initial http.Handler) *atomicHandler {
	a := &atomicHandler{}
	a.set(initial)
	return a
}

func (a *atomicHandler) set(h http.Handler) { a.h.Store(&h) }

func (a *atomicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*a.h.Load()).ServeHTTP(w, r)
}

// notReadyHandler responds 503 to every request while Postgres is unavailable.
func notReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "storage not ready\n")
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/specgraph/ -run 'TestNotReadyHandler|TestAtomicHandler' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/readiness.go cmd/specgraph/readiness_test.go
git commit -s -m "feat(serve): add swappable atomic handler and blanket-503 handler"
```

---

## Task 3: classifyConnectError

Classify a connect error as credential (pgx SQLSTATE class `28xxx`) vs. other. `postgres.New` wraps the `pool.Ping` error with `%w` (`postgres.go:81`), so `errors.As` recovers the `*pgconn.PgError`.

**Files:**

- Create: `cmd/specgraph/connector.go`
- Test: `cmd/specgraph/connector_test.go`

- [ ] **Step 1: Write the failing test**

```go
// cmd/specgraph/connector_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestClassifyConnectError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want errClass
	}{
		{"invalid_password 28P01", &pgconn.PgError{Code: "28P01"}, classCredential},
		{"invalid_authorization 28000", &pgconn.PgError{Code: "28000"}, classCredential},
		{"wrapped credential", fmt.Errorf("postgres: verify connectivity: %w", &pgconn.PgError{Code: "28P01"}), classCredential},
		{"connection refused 08006", &pgconn.PgError{Code: "08006"}, classOther},
		{"plain error", errors.New("dial tcp: connection refused"), classOther},
		{"nil-ish wrapped non-pg", fmt.Errorf("boom: %w", errors.New("x")), classOther},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyConnectError(tc.err))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/specgraph/ -run TestClassifyConnectError -v`
Expected: FAIL — `undefined: classifyConnectError`, `undefined: errClass`.

- [ ] **Step 3: Write minimal implementation**

```go
// cmd/specgraph/connector.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// errClass distinguishes connect failures that warrant different retry policies.
type errClass int

const (
	// classOther covers transient/operational failures (connection refused,
	// dial timeout, migration failure). Retried forever with backoff.
	classOther errClass = iota
	// classCredential covers PostgreSQL SQLSTATE class 28 (invalid auth).
	// Misconfiguration: retried a bounded number of times, then fatal.
	classCredential
)

// classifyConnectError inspects err for a pgx *pgconn.PgError with a SQLSTATE
// in the 28xxx class (28P01 invalid_password, 28000 invalid_authorization).
func classifyConnectError(err error) errClass {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "28") {
		return classCredential
	}
	return classOther
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/specgraph/ -run TestClassifyConnectError -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/connector.go cmd/specgraph/connector_test.go
git commit -s -m "feat(serve): classify postgres connect errors (credential vs other)"
```

---

## Task 4: connectStore retry unit with partial-pool cleanup

The retry unit. Runs the pool-touching prefix; if `postgres.New` succeeds but a later step fails, the pool is closed before returning so retries never orphan a pool. This is verified by the integration test (Task 9); here we just add the function and confirm the package builds.

**Files:**

- Modify: `cmd/specgraph/connector.go`

- [ ] **Step 1: Add the implementation**

```go
// append to cmd/specgraph/connector.go

// (add to the import block: "context",
//  "github.com/specgraph/specgraph/internal/bootstrap",
//  "github.com/specgraph/specgraph/internal/notify",
//  "github.com/specgraph/specgraph/internal/storage/postgres")

// connectResult holds the live stores produced by a successful connect unit.
type connectResult struct {
	store     *postgres.Store
	authStore *postgres.AuthStore
	bootstrap bootstrap.Result
}

// connectStore runs the pool-touching prefix once:
// postgres.New → Subscribe → NewAuth → bootstrap.Ensure. bootstrap.Ensure is
// deliberately last so the one-time admin token is minted only when nothing
// fallible runs after it within an attempt.
//
// Partial-failure cleanup: if postgres.New succeeds but a later step fails, the
// created pool (and borrowing auth store) is closed before returning, so
// repeated retries never leak a pgxpool.Pool. postgres.New self-cleans its own
// internal failures (postgres.go:80,85,92); this covers the steps after it.
func connectStore(ctx context.Context, connURL string) (connectResult, error) {
	s, err := postgres.New(ctx, connURL, postgres.WithProject("_server"))
	if err != nil {
		return connectResult{}, err
	}

	var authStore *postgres.AuthStore
	ok := false
	defer func() {
		if ok {
			return
		}
		// Teardown order mirrors shutdown: auth (borrows pool) before store
		// (owns pool).
		if authStore != nil {
			_ = authStore.Close(ctx)
		}
		_ = s.Close(ctx)
	}()

	s.Subscribe(notify.NewImpactLogger())

	authStore, err = postgres.NewAuth(ctx, s.Pool())
	if err != nil {
		return connectResult{}, err
	}

	res, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	if err != nil {
		return connectResult{}, err
	}

	ok = true
	return connectResult{store: s, authStore: authStore, bootstrap: res}, nil
}
```

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./cmd/specgraph/`
Expected: builds cleanly (no exit code, no output).

- [ ] **Step 3: Run existing package tests to confirm nothing broke**

Run: `go test ./cmd/specgraph/ -run 'TestClassifyConnectError|TestReadinessPinger|TestAtomicHandler|TestNotReadyHandler' -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/connector.go
git commit -s -m "feat(serve): add connectStore retry unit with partial-pool cleanup"
```

---

## Task 5: runConnector + firstConnect + ctxSleep (backoff loop)

The classified retry driver. `firstConnect` is the synchronous boot phase (success / bounded credential retry / signal degrade). `runConnector` is the background driver (credential 5×1s→fatal; other 1s→1m forever). Timing and sleep are injected so tests run instantly.

**Files:**

- Modify: `cmd/specgraph/connector.go`
- Test: `cmd/specgraph/connector_test.go`

- [ ] **Step 1: Write the failing test**

```go
// append to cmd/specgraph/connector_test.go
// (add to the import block: "context", "time")

func testPolicy() backoffPolicy {
	// Tiny durations; sleep is stubbed to no-op in tests anyway.
	return backoffPolicy{otherBase: time.Millisecond, otherMax: 4 * time.Millisecond, credInterval: time.Millisecond, credMaxRetry: 5}
}

func noSleep(_ context.Context, _ time.Duration) error { return nil }

func TestRunConnector_SucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	attempt := func(context.Context) (connectResult, error) {
		calls++
		if calls < 3 {
			return connectResult{}, errors.New("connection refused")
		}
		return connectResult{}, nil // success (zero stores; we only assert no error)
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRunConnector_CredentialExhaustionIsFatal(t *testing.T) {
	calls := 0
	attempt := func(context.Context) (connectResult, error) {
		calls++
		return connectResult{}, &pgconn.PgError{Code: "28P01"}
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials rejected")
	assert.Equal(t, 5, calls)
}

func TestRunConnector_CredentialCounterResetsOnOther(t *testing.T) {
	// 4 credential, then 1 other (reset), then 4 credential, then success.
	seq := []error{
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		errors.New("refused"),
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		nil,
	}
	i := 0
	attempt := func(context.Context) (connectResult, error) {
		err := seq[i]
		i++
		return connectResult{}, err
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	assert.NoError(t, err)
}

func TestRunConnector_ContextCancellationStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempt := func(context.Context) (connectResult, error) {
		return connectResult{}, errors.New("refused")
	}
	_, err := runConnector(ctx, attempt, testPolicy(), noSleep)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestFirstConnect_DegradesOnOtherError(t *testing.T) {
	t.Skip("firstConnect uses the real connectStore; covered by integration Task 9")
}
```

Add `"github.com/stretchr/testify/require"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/specgraph/ -run 'TestRunConnector' -v`
Expected: FAIL — `undefined: runConnector`, `undefined: backoffPolicy`.

- [ ] **Step 3: Write minimal implementation**

```go
// append to cmd/specgraph/connector.go
// (add to the import block: "fmt", "log/slog", "time")

// backoffPolicy parametrises the connect retry loop.
type backoffPolicy struct {
	otherBase    time.Duration // first backoff for "other" errors (1s)
	otherMax     time.Duration // cap for "other" backoff (1m)
	credInterval time.Duration // fixed interval between credential retries (1s)
	credMaxRetry int           // max consecutive credential attempts before fatal (5)
}

func defaultBackoffPolicy() backoffPolicy {
	return backoffPolicy{
		otherBase:    time.Second,
		otherMax:     time.Minute,
		credInterval: time.Second,
		credMaxRetry: 5,
	}
}

type attemptFunc func(context.Context) (connectResult, error)

// ctxSleep sleeps for d or returns early if ctx is cancelled.
func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// runConnector retries attempt until it succeeds, ctx is cancelled, or
// consecutive credential failures are exhausted (returns a fatal error).
// Credential errors retry at a fixed interval up to credMaxRetry; a
// non-credential attempt resets the counter. All other errors retry forever
// with exponential backoff capped at otherMax, logged loudly.
func runConnector(ctx context.Context, attempt attemptFunc, pol backoffPolicy, sleep func(context.Context, time.Duration) error) (connectResult, error) {
	credCount := 0
	backoff := pol.otherBase
	for {
		res, err := attempt(ctx)
		if err == nil {
			return res, nil
		}
		if ctx.Err() != nil {
			return connectResult{}, ctx.Err()
		}
		switch classifyConnectError(err) {
		case classCredential:
			credCount++
			slog.LogAttrs(ctx, slog.LevelError, "postgres credential failure",
				slog.Int("attempt", credCount), slog.Int("max", pol.credMaxRetry), slog.Any("error", err))
			if credCount >= pol.credMaxRetry {
				return connectResult{}, fmt.Errorf("postgres credentials rejected after %d attempts: %w", credCount, err)
			}
			if serr := sleep(ctx, pol.credInterval); serr != nil {
				return connectResult{}, serr
			}
		default:
			credCount = 0
			slog.LogAttrs(ctx, slog.LevelWarn, "postgres unavailable, retrying",
				slog.Duration("backoff", backoff), slog.Any("error", err))
			if serr := sleep(ctx, backoff); serr != nil {
				return connectResult{}, serr
			}
			backoff *= 2
			if backoff > pol.otherMax {
				backoff = pol.otherMax
			}
		}
	}
}

// firstConnect performs the synchronous boot phase against the real
// connectStore. Returns (res, false, nil) on success; (zero, true, nil) when
// the first failure is non-credential (caller goes degraded); (zero, false,
// err) when credentials are exhausted or ctx ends.
func firstConnect(ctx context.Context, connURL string, pol backoffPolicy) (connectResult, bool, error) {
	credCount := 0
	for {
		res, err := connectStore(ctx, connURL)
		if err == nil {
			return res, false, nil
		}
		if ctx.Err() != nil {
			return connectResult{}, false, ctx.Err()
		}
		if classifyConnectError(err) != classCredential {
			return connectResult{}, true, nil
		}
		credCount++
		slog.LogAttrs(ctx, slog.LevelError, "postgres credential failure",
			slog.Int("attempt", credCount), slog.Int("max", pol.credMaxRetry), slog.Any("error", err))
		if credCount >= pol.credMaxRetry {
			return connectResult{}, false, fmt.Errorf("postgres credentials rejected after %d attempts: %w", credCount, err)
		}
		if serr := ctxSleep(ctx, pol.credInterval); serr != nil {
			return connectResult{}, false, serr
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/specgraph/ -run 'TestRunConnector|TestClassifyConnectError' -v`
Expected: PASS (the `TestFirstConnect_DegradesOnOtherError` case is skipped).

- [ ] **Step 5: Commit**

```bash
git add cmd/specgraph/connector.go cmd/specgraph/connector_test.go
git commit -s -m "feat(serve): add classified connect backoff loop (runConnector/firstConnect)"
```

---

## Task 6: Extract PG-independent deps into buildAppDeps

Move the PG-independent construction (OIDC verifiers, Cedar engine/authorizer, known roles, embedded skills, web FS, CORS origin) out of the inline boot path into a helper that runs **before** connecting and is fatal on failure. This does not change behavior yet — `runServe` still calls everything inline; we only introduce the helper and the `appDeps` struct, then have `runServe` use it.

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Add the appDeps type and buildAppDeps helper**

Add near the top of `serve.go` (after the imports), a struct and constructor that lift the existing code at `serve.go:177-196` (verifiers), `:220` (knownRoles), `:238-248` (Cedar engine + authorizer), `:263-269` (skills), `:326-340` (web FS + CORS flag):

```go
// appDeps holds everything constructed before (and independent of) the
// Postgres connection. Build failures here are fatal at startup, exactly as
// before this refactor: a broken binary, bad OIDC config, or bad CORS flag
// should fail fast on every boot regardless of database availability.
type appDeps struct {
	verifiers  []*auth.OIDCVerifier
	authorizer *auth.CedarAuthorizer
	knownRoles []auth.Role
	skillsSrc  *skills.Source
	webFS      fs.FS
	corsOrigin string
}

func buildAppDeps(ctx context.Context, cfg *config.GlobalConfig, cmd *cobra.Command) (appDeps, error) {
	verifiers := make([]*auth.OIDCVerifier, 0, len(cfg.Auth.OIDC.Providers))
	for _, pc := range cfg.Auth.OIDC.Providers {
		issuerCtx, issuerCancel := context.WithTimeout(ctx, 10*time.Second)
		v, oidcErr := auth.NewOIDCVerifier(issuerCtx, pc)
		issuerCancel()
		if oidcErr != nil {
			return appDeps{}, fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
		}
		verifiers = append(verifiers, v)
		slog.LogAttrs(context.Background(), slog.LevelInfo, "auth: OIDC provider configured",
			slog.String("id", pc.ID), slog.String("issuer", pc.Issuer))
	}

	knownRoles := auth.KnownRolesFrom(cfg.Auth.Roles)

	policySources := []auth.PolicySource{auth.NewEmbeddedPolicySource()}
	for _, dir := range cfg.Auth.Policies.ExtraDirs {
		policySources = append(policySources, auth.NewDirectoryPolicySource(dir))
	}
	engine, err := auth.NewCedarEngine(ctx, policySources, auth.ActionNames())
	if err != nil {
		return appDeps{}, fmt.Errorf("policy engine: %w", err)
	}
	authorizer := auth.NewCedarAuthorizer(engine)

	skillsSrc, skillsErr := skills.NewEmbedded()
	if skillsErr != nil {
		return appDeps{}, fmt.Errorf("load embedded skills: %w", skillsErr)
	}

	webFS, err := fs.Sub(web.Build, "build")
	if err != nil {
		return appDeps{}, fmt.Errorf("embedded web FS: %w", err)
	}

	corsOrigin, err := cmd.Flags().GetString("cors-origin")
	if err != nil {
		return appDeps{}, fmt.Errorf("cors-origin flag: %w", err)
	}

	return appDeps{
		verifiers:  verifiers,
		authorizer: authorizer,
		knownRoles: knownRoles,
		skillsSrc:  skillsSrc,
		webFS:      webFS,
		corsOrigin: corsOrigin,
	}, nil
}
```

> **Note on types:** confirm the concrete types while implementing —
> `skills.NewEmbedded()` returns `(*skills.Source, error)` per `serve.go:266`;
> `auth.NewCedarAuthorizer` returns `*auth.CedarAuthorizer` per `serve.go:248`;
> `auth.KnownRolesFrom` returns `[]auth.Role` per `serve.go:220`. If any differ,
> match the real return types in the struct fields.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/specgraph/`
Expected: builds cleanly. (`go build` does not reject an as-yet-unused package-level function; `buildAppDeps` is wired into `runServe` in Task 8.)

> **DO NOT COMMIT YET.** `golangci-lint`'s `unused` linter rejects an unexported
> function that is never called, and the pre-commit hook runs it. `serve.go` is
> committed **once** at the end of Task 8, after `buildAppDeps`/`buildAppHandler`
> are wired into `runServe`. Leave the changes on disk and proceed to Task 7.
> (Tasks 6, 7, and 8 are one atomic `serve.go` change split for readability.)

---

## Task 7: Extract PG-dependent wiring into buildAppHandler

Move the wiring that needs the live stores (`serve.go:198-215` usagetracker, `:222-236` identity store/resolver, `:250-261` interceptor/opts, `:271-330` mux + Register* + MCP + static, `:332` handler assembly) into a helper taking the connected stores and `appDeps`. It returns the top-level `http.Handler`, the `resolver` (needed for the no-auth warn), and a `cleanup` closure that drains the usagetracker.

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Add buildAppHandler**

```go
// appHandler is the result of wiring the application once a live connection
// exists.
type appHandler struct {
	handler  http.Handler
	resolver *auth.IdentityStore // for the post-listen no-auth warn
	cleanup  func()              // drains the usagetracker; call after server drain
}

func buildAppHandler(ctx context.Context, cfg *config.GlobalConfig, deps appDeps, res connectResult) (appHandler, error) {
	store := res.store // *postgres.Store satisfies the backendStore interface used below

	// usagetracker for async TouchLastUsed.
	tracker := usagetracker.NewManager(res.authStore, usagetracker.Config{
		BufferSize:    256,
		FlushInterval: 5 * time.Second,
	})
	cleanup := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if trackerErr := tracker.Close(shutdownCtx); trackerErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "usagetracker close", slog.Any("error", trackerErr))
		}
		if dropped := tracker.Dropped(); dropped > 0 {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "usagetracker dropped touches over session lifetime",
				slog.Uint64("count", dropped),
				slog.String("hint", "increase auth usagetracker BufferSize or decrease FlushInterval"))
		}
	}

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:                   res.authStore,
		Verifiers:               deps.verifiers,
		Tracker:                 tracker,
		KnownRoles:              deps.knownRoles,
		JITEnabled:              cfg.Auth.OIDC.JITCreate.Enabled,
		JITDefaultRole:          auth.Role(cfg.Auth.OIDC.JITCreate.DefaultRole),
		JITClaimsMapping:        buildClaimsMappingByIssuer(cfg.Auth.OIDC.Providers),
		JITRateBurstPerHour:     cfg.Auth.OIDC.JITCreate.RateLimitPerHour,
		JITEmailDomainAllowlist: cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist,
	})
	if err != nil {
		cleanup() // drain the tracker goroutine we just started
		return appHandler{}, fmt.Errorf("identity store: %w", err)
	}

	interceptor := auth.NewAuthInterceptor(resolver, deps.authorizer)
	maxBytes := connect.WithReadMaxBytes(4 << 20)
	opts := connect.WithInterceptors(interceptor)

	mux := server.NewMux(store, opts, maxBytes)
	server.RegisterHealthService(mux, opts, maxBytes)
	server.RegisterDecisionService(mux, store, opts, maxBytes)
	server.RegisterGraphService(mux, store, opts, maxBytes)
	server.RegisterClaimService(mux, store, opts, maxBytes)
	server.RegisterConstitutionService(mux, store, opts, maxBytes)
	server.RegisterAuthoringService(mux, store, opts, maxBytes)
	server.RegisterAnalyticalPassService(mux, store, ".specgraph/templates", opts, maxBytes)
	server.RegisterExecutionService(mux, store, deps.skillsSrc, opts, maxBytes)
	server.RegisterSliceService(mux, store, opts, maxBytes)
	server.RegisterIdentityService(mux, res.authStore, opts, maxBytes)
	server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
	driftEngine := drift.NewEngine(store, nil)
	lintEngine := linter.NewEngine(store, nil)
	server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts, maxBytes)

	syncHandler := server.RegisterSyncService(mux, store, opts, maxBytes)
	runner := syncpkg.NewExecRunner()
	syncHandler.RegisterAdapter(syncpkg.NewBeadsAdapter(runner))
	syncHandler.RegisterAdapter(syncpkg.NewGitHubAdapter(runner, ""))

	server.RegisterAPIHandlers(mux, store, auth.RequireAuth(resolver))
	server.RegisterAuthHandlers(mux, resolver, auth.RequireAuth(resolver))

	mcpClient := mcppkg.NewClient(newHTTPClient(""), selfBaseURL(cfg.Server.Listen))
	mcpSrv := mcppkg.NewServer(mcpClient)
	mcpHTTPHandler := mcpSrv.HTTPHandler(
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			if v := r.Header.Get("Authorization"); v != "" {
				scheme, token, ok := strings.Cut(v, " ")
				token = strings.TrimSpace(token)
				if ok && strings.EqualFold(scheme, "Bearer") && token != "" {
					ctx = auth.WithBearerToken(ctx, token)
				}
			}
			if slug := r.Header.Get("X-Specgraph-Project"); slug != "" {
				ctx = auth.WithProject(ctx, slug)
			}
			return ctx
		}),
	)
	mux.Handle("/mcp/", mcpHeaderLogger(auth.RequireAuth(resolver)(
		http.StripPrefix("/mcp", mcpHTTPHandler),
	)))

	mux.Handle("/", server.StaticHandler(deps.webFS))

	handler := server.SecurityHeaders(server.ProjectMiddleware(mux))
	if deps.corsOrigin != "" {
		handler = server.CORSMiddleware(deps.corsOrigin, handler)
	}

	return appHandler{handler: handler, resolver: resolver, cleanup: cleanup}, nil
}
```

> **Note:** `auth.NewIdentityStore` returns `*auth.IdentityStore` per
> `serve.go:223`. Confirm the concrete type for the `appHandler.resolver` field
> while implementing; adjust if the real type differs.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/specgraph/`
Expected: builds cleanly. Both helpers now exist; `runServe` still uses the old inline path (replaced in Task 8). Leave the old inline code intact for now.

> **DO NOT COMMIT YET** — same reason as Task 6. `serve.go` is committed once at
> the end of Task 8.

---

## Task 8: Rewrite runServe control flow

Add `startMainServer` and replace the body of `runServe` (from the store-creation block through `ListenAndServe`, i.e. `serve.go:114-402`) with the deferred-connect orchestration. This is the final step of the consolidated `serve.go` refactor: it wires in `buildAppDeps` and `buildAppHandler` (added in Tasks 6-7) and **commits all three changes in a single commit**, so the `unused` linter never sees a not-yet-wired helper.

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Add startMainServer helper**

```go
// startMainServer runs srv.Serve in a goroutine and returns a channel that
// fires once if Serve returns a non-ErrServerClosed error, so the caller can
// trigger shutdown (a dead main listener is unrecoverable).
func startMainServer(srv *http.Server, ln net.Listener) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		serveErr := srv.Serve(ln)
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		errCh <- serveErr
	}()
	return errCh
}
```

- [ ] **Step 2: Replace the runServe body**

Replace everything from the `// Create backend store.` block (`serve.go:114`) through the final `return nil` of `runServe` (`serve.go:402`) with:

```go
	connURL := cfg.Server.Postgres.URL
	if connURL == "" {
		return fmt.Errorf("postgres backend requires a connection URL (set server.postgres.url in config or use --pg-url)")
	}

	// PG-independent construction is fatal up front (unchanged behavior).
	deps, err := buildAppDeps(ctx, cfg, cmd)
	if err != nil {
		return err
	}

	probesCfg, err := validateServerConfig(cfg)
	if err != nil {
		return err
	}

	pol := defaultBackoffPolicy()

	// Decide the boot path BEFORE starting any server or shutdown goroutine, so
	// a fatal docker:true / credential failure starts nothing (unchanged).
	var (
		res      connectResult
		haveConn bool
	)
	if cfg.Server.Docker {
		r, connErr := connectStore(ctx, connURL)
		if connErr != nil {
			return fmt.Errorf("connect to postgres: %w", connErr)
		}
		res, haveConn = r, true
	} else {
		r, degrade, connErr := firstConnect(ctx, connURL, pol)
		if connErr != nil {
			return connErr // credential-exhausted fatal, or ctx cancelled
		}
		if !degrade {
			res, haveConn = r, true
		} else {
			slog.LogAttrs(context.Background(), slog.LevelWarn,
				"postgres unavailable at startup; serving 503 until it connects",
				slog.String("listen", cfg.Server.Listen))
			if probesCfg.Listen == "" {
				slog.LogAttrs(context.Background(), slog.LevelWarn,
					"readiness is not observable: set server.probes.listen to expose /readyz")
			}
		}
	}

	// Shared activation: wires the app once a live connection exists, swaps in
	// the real handler, publishes the store to the probe pinger, prints the
	// bootstrap banner, and starts the sweeper. Returns the post-drain cleanup.
	pinger := newReadinessPinger()
	handlerRef := newAtomicHandler(notReadyHandler())
	addr := cfg.Server.Listen

	sweeperCtx, stopSweeper := context.WithCancel(ctx)
	var (
		cleanupMu sync.Mutex
		cleanup   func() // store/auth/tracker teardown; nil until activation
	)

	activate := func(r connectResult) error {
		ah, buildErr := buildAppHandler(ctx, cfg, deps, r)
		if buildErr != nil {
			return buildErr
		}
		// Print the one-time bootstrap banner immediately, before exposing the
		// real handler, so the token can never be stranded by a later failure.
		if r.bootstrap.Created {
			fmt.Fprintf(os.Stderr,
				"\n========================================================================\n"+
					"SpecGraph created a bootstrap admin on first start.\n"+
					"  API key: %s\n"+
					"  Server:  http://%s\n"+
					"Copy this key into your CLI credentials now. Rotate it after you\n"+
					"configure OIDC. This key is shown ONCE and will not be displayed again.\n"+
					"========================================================================\n\n",
				r.bootstrap.Token, addr)
		}

		pinger.set(r.store)
		handlerRef.set(ah.handler)

		if hasAuth, hasAuthErr := ah.resolver.HasAuth(ctx); hasAuthErr != nil {
			slog.LogAttrs(context.Background(), slog.LevelWarn, "auth: HasAuth check failed at startup", slog.Any("error", hasAuthErr))
		} else if !hasAuth {
			warnIfNoAuthOnPublicListen(cfg.Server.Listen, false)
		}

		server.StartSweeper(sweeperCtx, r.store, 60*time.Second)

		cleanupMu.Lock()
		cleanup = func() {
			// Post-drain teardown order (sweeper already stopped):
			// tracker → authStore → store.
			ah.cleanup()
			if closeErr := r.authStore.Close(context.Background()); closeErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "auth store close", slog.Any("error", closeErr))
			}
			if closeErr := r.store.Close(context.Background()); closeErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "close store", slog.Any("error", closeErr))
			}
		}
		cleanupMu.Unlock()
		slog.LogAttrs(context.Background(), slog.LevelInfo, "storage connected; serving")
		return nil
	}

	// Happy path: wire and swap in the real handler BEFORE opening the port,
	// so clients never observe a 503 window. activate also sets the pinger's
	// store, so the probe listener's first probe is healthy (no spurious WARN).
	// If activation fails here, no listener has been opened yet — clean return.
	if haveConn {
		if actErr := activate(res); actErr != nil {
			stopSweeper()
			return actErr
		}
	}

	// Open the ports. On the happy path handlerRef already holds the real
	// handler; in the degraded window it holds the blanket-503 handler.
	mainLn, err := net.Listen("tcp", addr)
	if err != nil {
		stopSweeper()
		return fmt.Errorf("main listener bind %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:           handlerRef,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	probeSrv, probeErrCh, err := startProbeListener(ctx, pinger, probesCfg)
	if err != nil {
		stopSweeper()
		_ = mainLn.Close()
		return err
	}
	mainErrCh := startMainServer(srv, mainLn)

	// fatalErr is published by the shutdown goroutine; read after <-done.
	var fatalErr error
	wiringFatalCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
		case e := <-probeErrCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "probe listener died; triggering shutdown", slog.Any("error", e))
			cancel()
		case e := <-mainErrCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "main listener died; triggering shutdown", slog.Any("error", e))
			fatalErr = e
			cancel()
		case e := <-wiringFatalCh:
			slog.LogAttrs(context.Background(), slog.LevelError, "fatal wiring error; triggering shutdown", slog.Any("error", e))
			fatalErr = e
			cancel()
		}
		slog.LogAttrs(context.Background(), slog.LevelInfo, "server shutting down")
		stopSweeper()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			mainCtx, c := context.WithTimeout(context.Background(), 15*time.Second)
			defer c()
			if shErr := srv.Shutdown(mainCtx); shErr != nil {
				slog.LogAttrs(context.Background(), slog.LevelWarn, "main server shutdown", slog.Any("error", shErr))
			}
		}()
		if probeSrv != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				probeCtx, c := context.WithTimeout(context.Background(), probeShutdownTimeout)
				defer c()
				if shErr := probeSrv.Shutdown(probeCtx); shErr != nil {
					slog.LogAttrs(context.Background(), slog.LevelWarn, "probe server shutdown", slog.Any("error", shErr))
				}
			}()
		}
		wg.Wait()

		// Post-drain store/auth/tracker teardown (nil until activation).
		cleanupMu.Lock()
		cl := cleanup
		cleanupMu.Unlock()
		if cl != nil {
			cl()
		}
	}()

	slog.LogAttrs(context.Background(), slog.LevelInfo, "server listening", slog.String("addr", "http://"+addr))
	slog.LogAttrs(context.Background(), slog.LevelInfo, "MCP endpoint available", slog.String("path", "/mcp/"))

	// Degraded path: retry in the background; activate on first success. The
	// ports are already open (serving 503) before this goroutine starts.
	if !haveConn {
		go func() {
			r, connErr := runConnector(ctx, func(c context.Context) (connectResult, error) {
				return connectStore(c, connURL)
			}, pol, ctxSleep)
			if connErr != nil {
				if ctx.Err() == nil { // genuine fatal (credential exhaustion), not shutdown
					wiringFatalCh <- connErr
				}
				return
			}
			if actErr := activate(r); actErr != nil {
				wiringFatalCh <- actErr
			}
		}()
	}

	<-done
	return fatalErr
```

> **Removals:** delete the old inline blocks now replaced — the
> `type backendStore interface{...}` + `var store backendStore` declaration, the
> old `postgres.New` call, the old `store.Close`/`stopSweeper` defers, the old
> `authStore`/`bootstrap.Ensure`/`tracker`/`resolver`/`interceptor`/`mux`/MCP/
> static/handler/`http.Server`/`startProbeListener`/shutdown-goroutine/
> `ListenAndServe` code (`serve.go:114-402`). Keep `selfBaseURL`,
> `isLoopbackAddr`, `warnIfNoAuthOnPublicListen`, `validateServerConfig`,
> `startProbeListener`, `mcpHeaderLogger`, `buildClaimsMappingByIssuer`.

- [ ] **Step 3: Fix imports**

`net` is now used directly in `serve.go` (already imported). Remove any imports that became unused after the extraction (the compiler will list them). Run:

Run: `go build ./cmd/specgraph/`
Expected: builds cleanly after removing unused imports.

- [ ] **Step 4: Run the full package unit tests**

Run: `go test ./cmd/specgraph/`
Expected: PASS (existing `serve_test.go`, `serve_mcp_test.go`, `serve_probes_test.go` still green).

- [ ] **Step 5: Run lint**

Run: `task lint`
Expected: no new findings (watch for `revive` package-comment, `wrapcheck`, `govet -shadow`).

- [ ] **Step 6: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): non-fatal postgres startup with deferred connect (docker:false)"
```

---

## Task 9: Integration tests (degraded→recovery, credential-fatal)

End-to-end tests behind the `integration` build tag, using a real Postgres testcontainer. They exercise the full `runServe` path by invoking `serveCmd` in a goroutine and asserting HTTP behavior.

**Files:**

- Create: `cmd/specgraph/serve_degraded_integration_test.go`

> **Why a TCP proxy:** the shared testcontainer helper
> (`internal/storage/postgres/postgrestest.ConnString`) maps Postgres to a
> **random** host port, but the degraded→recovery test needs the connection
> address to be known *before* Postgres is reachable. We point the server at a
> reserved-then-released local port (dead → degraded), then start an in-test TCP
> proxy from that port to the real container, making PG "appear" at the known
> address so the connector self-heals. The credential-fatal test mangles the
> shared conn string's password.
>
> **Verify while implementing:** env var names map to the koanf loader
> (`SPECGRAPH_SERVER_POSTGRES_URL`, `_SERVER_LISTEN`, `_SERVER_DOCKER`,
> `_SERVER_PROBES_LISTEN/_INTERVAL/_TIMEOUT` — all confirmed in
> `internal/config/global_test.go`). `cfgFile` is the package global in
> `main.go` consulted by `loadGlobalCfg`.

- [ ] **Step 1: Write the integration tests**

```go
// cmd/specgraph/serve_degraded_integration_test.go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reservePort binds :0, records the address, releases it, and returns the
// host:port. There is an inherent TOCTOU window before something rebinds it;
// acceptable for a local integration test.
func reservePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// tcpProxy forwards each connection on listenAddr to dialAddr until ctx ends.
func tcpProxy(t *testing.T, ctx context.Context, listenAddr, dialAddr string) {
	t.Helper()
	ln, err := net.Listen("tcp", listenAddr)
	require.NoError(t, err)
	go func() { <-ctx.Done(); _ = ln.Close() }()
	go func() {
		for {
			client, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				up, derr := net.Dial("tcp", dialAddr)
				if derr != nil {
					return
				}
				defer up.Close()
				go func() { _, _ = io.Copy(up, c) }()
				_, _ = io.Copy(c, up)
			}(client)
		}
	}()
}

// newServeCmd builds a *cobra.Command carrying the serve flag set and ctx, so
// runServe can be invoked directly. Values come from env (see configureServeEnv);
// flags default empty so config.WithFlags lets env win.
func newServeCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{Use: "serve", RunE: runServe}
	cmd.Flags().String("cors-origin", "", "")
	cmd.Flags().String("pg-url", "", "")
	cmd.Flags().String("listen", "", "")
	cmd.Flags().String("log-level", "", "")
	cmd.Flags().String("log-format", "", "")
	cmd.Flags().String("log-output", "", "")
	cmd.SetContext(ctx)
	return cmd
}

// configureServeEnv points loadGlobalCfg at an empty temp config and forces
// docker:false + the given addresses via SPECGRAPH_* env (t.Setenv restores).
func configureServeEnv(t *testing.T, pgURL, mainAddr, probeAddr string) {
	t.Helper()
	empty := t.TempDir() + "/config.yaml"
	require.NoError(t, os.WriteFile(empty, []byte("{}\n"), 0o600))
	cfgFile = empty
	t.Cleanup(func() { cfgFile = "" })

	t.Setenv("SPECGRAPH_SERVER_DOCKER", "false")
	t.Setenv("SPECGRAPH_SERVER_POSTGRES_URL", pgURL)
	t.Setenv("SPECGRAPH_SERVER_LISTEN", mainAddr)
	t.Setenv("SPECGRAPH_SERVER_PROBES_LISTEN", probeAddr)
	t.Setenv("SPECGRAPH_SERVER_PROBES_INTERVAL", "100ms")
	t.Setenv("SPECGRAPH_SERVER_PROBES_TIMEOUT", "100ms")
}

func httpStatus(url string) (int, bool) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url) //nolint:noctx // short-timeout test probe
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	return resp.StatusCode, true
}

func TestServe_DegradedThenRecovers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realCS, err := postgrestest.ConnString(ctx)
	require.NoError(t, err)
	// realCS == "postgres://test:test@HOST:PORT/testdb"
	realHostPort := realCS[strings.LastIndex(realCS, "@")+1 : strings.LastIndex(realCS, "/")]

	frontAddr := reservePort(t) // server connects here: dead until the proxy starts
	mainAddr := reservePort(t)
	probeAddr := reservePort(t)
	frontCS := "postgres://test:test@" + frontAddr + "/testdb"

	configureServeEnv(t, frontCS, mainAddr, probeAddr)
	cmd := newServeCmd(ctx)

	serveErr := make(chan error, 1)
	go func() { serveErr <- runServe(cmd, nil) }()

	// Degraded: main port 503, /readyz 503.
	require.Eventually(t, func() bool {
		code, ok := httpStatus("http://" + mainAddr + "/")
		return ok && code == http.StatusServiceUnavailable
	}, 10*time.Second, 100*time.Millisecond, "main port must serve 503 while PG is down")
	code, ok := httpStatus("http://" + probeAddr + "/readyz")
	require.True(t, ok)
	assert.Equal(t, http.StatusServiceUnavailable, code)

	// Bring PG up at the front address → server self-heals.
	tcpProxy(t, ctx, frontAddr, realHostPort)
	require.Eventually(t, func() bool {
		c, ok := httpStatus("http://" + probeAddr + "/readyz")
		return ok && c == http.StatusOK
	}, 40*time.Second, 250*time.Millisecond, "/readyz must flip to 200 once PG is reachable")

	select {
	case e := <-serveErr:
		t.Fatalf("server exited prematurely: %v", e)
	default:
	}

	cancel()
	select {
	case <-serveErr:
	case <-time.After(20 * time.Second):
		t.Fatal("server did not shut down after cancel")
	}
}

func TestServe_CredentialFailureIsFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realCS, err := postgrestest.ConnString(ctx)
	require.NoError(t, err)
	badCS := strings.Replace(realCS, "test:test@", "test:wrongpassword@", 1)

	configureServeEnv(t, badCS, reservePort(t), reservePort(t))
	cmd := newServeCmd(ctx)

	done := make(chan error, 1)
	go func() { done <- runServe(cmd, nil) }()

	select {
	case e := <-done:
		require.Error(t, e)
		assert.Contains(t, e.Error(), "credentials rejected")
	case <-time.After(15 * time.Second):
		t.Fatal("expected credential failure to be fatal within ~5 retries")
	}
}
```

> **Note:** if `config.WithFlags` does not let env override an unset flag in this
> codebase, set `--pg-url`/`--listen` on the command instead of (or in addition
> to) the env vars. Confirm against `internal/config` loader precedence while
> implementing; the assertions above are the contract.

- [ ] **Step 2: Run the integration tests**

Run: `go test -tags integration ./cmd/specgraph/ -run 'TestServe_DegradedThenRecovers|TestServe_CredentialFailureIsFatal' -v`
Expected: PASS (requires Docker).

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/serve_degraded_integration_test.go
git commit -s -m "test(serve): integration coverage for non-fatal postgres startup"
```

---

## Task 10: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Unit + lint gate**

Run: `task check`
Expected: PASS (fmt:check → license:check → lint → build → unit tests).

- [ ] **Step 2: Integration + e2e gate (requires Docker)**

Run: `task pr-prep`
Expected: PASS (check → test:integration → test:e2e).

- [ ] **Step 3: Manual smoke (optional, requires a spare port)**

Run a server pointed at a dead PG with `server.docker: false` and a probe port, confirm it stays up and `/readyz` returns 503, then start PG and confirm `/readyz` flips to 200. Document the exact commands you used in the PR description.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit -s -m "chore(serve): address verification findings"
```

---

## Self-Review Notes (author)

- **Spec coverage:** behavior contract (Tasks 6-9), error policy/classification (Tasks 3, 5), connect unit + cleanup (Task 4), swappable handler + 503 (Task 2), readiness pinger (Task 1), banner-before-fallible-wiring + post-drain teardown order (Task 8), no-degraded-window happy path / docker:true unchanged (Task 8), testing (Tasks 1-3, 5, 9). All spec sections map to a task.
- **Type consistency:** `connectResult`, `appDeps`, `appHandler`, `errClass`, `backoffPolicy`, `attemptFunc` are defined once and reused. Confirm the three concrete-type notes (Tasks 6-7) against the real signatures during implementation.
- **Known follow-up:** `connectStore`'s partial-pool cleanup is correct-by-construction (deferred close) and asserted only indirectly via the degraded→recovery integration test. If a no-DB unit test for the cleanup path is desired later, introduce function-pointer seams for `postgres.New`/`NewAuth`/`bootstrap.Ensure`; out of scope here (YAGNI).
