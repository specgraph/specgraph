# CLI OIDC Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level `specgraph login` (and `specgraph logout`) that performs a browser-based OIDC login via a loopback redirect and stores the resulting `spgr_ws_` session token in the CLI credentials file.

**Architecture:** Server-brokered loopback with a PKCE-bound one-time code. The CLI opens the browser to the existing `/api/auth/oidc/{provider}/start` with extra `cli_callback`/`cli_state`/`cli_challenge` params; the server runs its normal IdP flow, then (for CLI flows) redirects a one-time code to the CLI's `127.0.0.1` listener instead of setting a cookie. The CLI exchanges `{code, cli_verifier}` at a new `POST /api/auth/cli/exchange`, which mints the session atomically inside a single `AuthStore` transaction. v1 is loopback-only; SSH/headless hard-errors to API keys.

**Tech Stack:** Go, ConnectRPC + plain `net/http` handlers, pgx v5 / goose migrations (`internal/storage/postgres`), cobra CLI, `golang.org/x/oauth2` (PKCE helpers), testcontainers for postgres integration tests.

**Spec:** `docs/plans/2026-06-15-cli-oidc-login-design.md`

**Repo conventions (read before starting):**

- This is a **jj-colocated** repo. Commit with `jj commit -m "<msg>"` (never `git push`). Every commit needs a `Signed-off-by:` trailer — append it inside the `-m` body as shown in each Commit step.
- All `.go` files need the SPDX header (first two lines). New Go packages need a `// Package <name> ...` doc comment or `revive` fails.
- `gen/` is generated; this plan touches **no** proto, so no `task proto` runs.
- Run `task check` before the final commit of each task batch (fmt → license → lint → build → unit tests). Postgres integration tests (`//go:build integration`) run via `task test:integration` (requires Docker) — only Task 3 adds those.

---

## File Structure

**New files:**

- `internal/storage/postgres/auth_migrations/003_cli_login.sql` — migration: 3 columns on `oidc_login_flows`, new `cli_login_codes` table.
- `internal/auth/loopback.go` — `IsLiteralLoopbackHost(host string) bool`, shared strict literal-loopback check (server validation + CLI HTTPS guard).
- `internal/auth/loopback_test.go` — unit tests for the host check.
- `internal/browser/browser.go` — cross-platform `Open(url string) error`.
- `internal/browser/browser_test.go` — opener command-resolution test.
- `cmd/specgraph/login.go` — `specgraph login`.
- `cmd/specgraph/login_test.go` — login unit tests.
- `cmd/specgraph/logout.go` — `specgraph logout`.
- `cmd/specgraph/logout_test.go` — logout unit tests.

**Modified files:**

- `internal/storage/errors.go` — add `ErrCLICodeNotFound`, `ErrCLIChallengeMismatch`.
- `internal/storage/web_auth_domain.go` — add CLI fields to `LoginFlow`.
- `internal/storage/web_auth.go` — extend `WebAuthStore` interface (3 new methods).
- `internal/storage/postgres/web_auth.go` — extend `CreateLoginFlow`/`ConsumeLoginFlow`; add `CreateCLICode`/`ExchangeCLICode`/`DeleteExpiredCLICodes`.
- `internal/storage/postgres/web_auth_test.go` — integration tests for the new storage paths.
- `internal/config/global.go` — add `OIDCConfig.CLILoginEnabled bool`; default `true` in `globalDefaults()`.
- `internal/server/auth_oidc_handler.go` — `OIDCLoginConfig.CLILoginEnabled`; handler field; `handleStart` CLI params; `handleCallback` CLI branch; `handleExchange` + route.
- `internal/server/auth_oidc_handler_test.go` — handler unit tests for the CLI paths.
- `internal/server/auth_handler.go` — `handleLogout` accepts `spgr_ws_` bearer.
- `internal/server/auth_handler_test.go` — logout-via-bearer test.
- `cmd/specgraph/serve.go` — pass `CLILoginEnabled` into `OIDCLoginConfig`.

---

## Task 1: Storage domain — errors and `LoginFlow` CLI fields

**Files:**

- Modify: `internal/storage/errors.go`
- Modify: `internal/storage/web_auth_domain.go`

- [ ] **Step 1: Add the two sentinel errors**

In `internal/storage/errors.go`, after the existing `ErrLoginFlowNotFound` declaration (around line 166), add:

```go
// ErrCLICodeNotFound is returned when a CLI one-time login code does not exist
// or has expired.
var ErrCLICodeNotFound = errors.New("cli login code not found")

// ErrCLIChallengeMismatch is returned when the PKCE verifier presented at the
// CLI exchange does not match the challenge stored with the code.
var ErrCLIChallengeMismatch = errors.New("cli login challenge mismatch")
```

- [ ] **Step 2: Add CLI fields to the `LoginFlow` domain struct**

In `internal/storage/web_auth_domain.go`, extend the `LoginFlow` struct (after the `ProviderID` field):

```go
type LoginFlow struct {
	ID           string // opaque flow id (tx cookie value)
	State        string // CSRF token, constant-time compared at callback
	Nonce        string
	CodeVerifier string // PKCE
	ProviderID   string
	// CLI-login fields. Empty for the web flow; set when the flow originated
	// from `specgraph login`. CLICallback is a validated loopback URL,
	// CLIState a CSRF token round-tripped to the CLI, CLIChallenge the PKCE
	// S256 challenge bound to the one-time code.
	CLICallback  string
	CLIState     string
	CLIChallenge string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
```

- [ ] **Step 3: Build**

Run: `go build ./internal/storage/...`
Expected: success (no callers reference the new fields yet).

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(storage): CLI login domain errors and LoginFlow fields

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 2: Migration — `oidc_login_flows` columns + `cli_login_codes` table

**Files:**

- Create: `internal/storage/postgres/auth_migrations/003_cli_login.sql`

- [ ] **Step 1: Write the migration**

Create `internal/storage/postgres/auth_migrations/003_cli_login.sql`:

```sql
-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

ALTER TABLE oidc_login_flows
    ADD COLUMN cli_callback  text NOT NULL DEFAULT '' CHECK (length(cli_callback) <= 512),
    ADD COLUMN cli_state     text NOT NULL DEFAULT '' CHECK (length(cli_state) <= 512),
    ADD COLUMN cli_challenge text NOT NULL DEFAULT '' CHECK (length(cli_challenge) <= 256);

CREATE TABLE cli_login_codes (
    code_hash     bytea PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    oidc_subject  text NOT NULL DEFAULT '' CHECK (length(oidc_subject) <= 255),
    cli_challenge text NOT NULL CHECK (length(cli_challenge) <= 256),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL
);

CREATE INDEX cli_login_codes_expiry ON cli_login_codes(expires_at);

-- +goose Down

DROP INDEX IF EXISTS cli_login_codes_expiry;
DROP TABLE IF EXISTS cli_login_codes;
ALTER TABLE oidc_login_flows
    DROP COLUMN IF EXISTS cli_challenge,
    DROP COLUMN IF EXISTS cli_state,
    DROP COLUMN IF EXISTS cli_callback;
```

- [ ] **Step 2: Verify license header lint passes**

Run: `task license:check`
Expected: PASS (the `-- SPDX` header satisfies addlicense for `.sql`).

- [ ] **Step 3: Commit**

```bash
jj commit -m "feat(storage): migration for CLI login codes and flow columns

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 3: Storage methods — extend flow methods + add CLI-code methods

**Files:**

- Modify: `internal/storage/web_auth.go` (interface)
- Modify: `internal/storage/postgres/web_auth.go` (implementation)
- Test: `internal/storage/postgres/web_auth_test.go` (integration, `//go:build integration`)

- [ ] **Step 1: Extend the `WebAuthStore` interface**

In `internal/storage/web_auth.go`, add to the `// --- login-flow state ---` section (after `DeleteExpiredLoginFlows`):

```go
	// --- CLI one-time login codes ---

	// CreateCLICode inserts a one-time CLI login code. codeHash is the SHA-256
	// of the opaque code; the raw code never reaches storage.
	CreateCLICode(ctx context.Context, codeHash []byte, userID, subject, challenge string, expiresAt time.Time) error

	// ExchangeCLICode atomically consumes an unexpired code and mints a session
	// in one transaction. gotChallenge is S256(verifier) precomputed by the
	// caller; it is constant-time compared against the stored challenge.
	// sess must carry TokenHash and ExpiresAt; UserID/OIDCSubject/ID/CreatedAt
	// are filled from the consumed code and the inserted row.
	// Returns ErrCLICodeNotFound (unknown/expired), ErrCLIChallengeMismatch
	// (PKCE mismatch), or ErrUserNotFound (user soft-deleted mid-flow).
	ExchangeCLICode(ctx context.Context, codeHash []byte, sess *Session, gotChallenge string) (*Session, error)

	// DeleteExpiredCLICodes removes expired code rows; returns count.
	DeleteExpiredCLICodes(ctx context.Context) (int64, error)
```

Add `"time"` to the imports if not already present (it is not — add it).

- [ ] **Step 2: Extend `CreateLoginFlow` and `ConsumeLoginFlow` for the CLI columns**

In `internal/storage/postgres/web_auth.go`, replace the `CreateLoginFlow` query and args:

```go
	const q = `
		INSERT INTO oidc_login_flows (state, nonce, code_verifier, provider_id, cli_callback, cli_state, cli_challenge, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`
	var id string
	if err := s.pool.QueryRow(ctx, q, f.State, f.Nonce, f.CodeVerifier, f.ProviderID, f.CLICallback, f.CLIState, f.CLIChallenge, f.ExpiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("create login flow: %w", err)
	}
	return id, nil
```

And replace the `ConsumeLoginFlow` query + scan:

```go
	const q = `
		DELETE FROM oidc_login_flows
		WHERE id = $1::uuid AND expires_at > $2
		RETURNING id, state, nonce, code_verifier, provider_id, cli_callback, cli_state, cli_challenge, created_at, expires_at`
	row := s.pool.QueryRow(ctx, q, flowID, s.now())
	var f storage.LoginFlow
	err := row.Scan(&f.ID, &f.State, &f.Nonce, &f.CodeVerifier, &f.ProviderID, &f.CLICallback, &f.CLIState, &f.CLIChallenge, &f.CreatedAt, &f.ExpiresAt)
```

(Leave the rest of `ConsumeLoginFlow`'s error handling unchanged.)

- [ ] **Step 3: Add the three new methods**

Append to `internal/storage/postgres/web_auth.go`:

```go
// CreateCLICode inserts a one-time CLI login code (code stored hashed).
func (s *AuthStore) CreateCLICode(ctx context.Context, codeHash []byte, userID, subject, challenge string, expiresAt time.Time) error {
	if len(codeHash) == 0 {
		return errors.New("CreateCLICode: codeHash required")
	}
	if userID == "" || challenge == "" {
		return errors.New("CreateCLICode: userID and challenge required")
	}
	if expiresAt.IsZero() {
		return errors.New("CreateCLICode: expiresAt required")
	}
	const q = `
		INSERT INTO cli_login_codes (code_hash, user_id, oidc_subject, cli_challenge, expires_at)
		SELECT $1, $2::uuid, $3, $4, $5
		WHERE EXISTS (SELECT 1 FROM users WHERE id = $2::uuid AND deleted_at IS NULL)`
	tag, err := s.pool.Exec(ctx, q, codeHash, userID, subject, challenge, expiresAt)
	if err != nil {
		return fmt.Errorf("create cli code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("create cli code: %w", storage.ErrUserNotFound)
	}
	return nil
}

// ExchangeCLICode consumes a code and mints a session atomically. All
// statements run on the transaction handle; it MUST NOT call CreateSession
// (which runs on the pool and would break atomicity).
func (s *AuthStore) ExchangeCLICode(ctx context.Context, codeHash []byte, sess *storage.Session, gotChallenge string) (*storage.Session, error) {
	if len(sess.TokenHash) == 0 || sess.ExpiresAt.IsZero() {
		return nil, errors.New("ExchangeCLICode: sess.TokenHash and ExpiresAt required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("exchange cli code: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() //nolint:errcheck // no-op after a successful Commit

	var userID, subject, storedChallenge string
	selErr := tx.QueryRow(ctx, `
		SELECT user_id::text, oidc_subject, cli_challenge
		FROM cli_login_codes
		WHERE code_hash = $1 AND expires_at > $2
		FOR UPDATE`, codeHash, s.now()).Scan(&userID, &subject, &storedChallenge)
	if errors.Is(selErr, pgx.ErrNoRows) {
		return nil, storage.ErrCLICodeNotFound
	}
	if selErr != nil {
		return nil, fmt.Errorf("exchange cli code: select: %w", selErr)
	}

	if subtle.ConstantTimeCompare([]byte(gotChallenge), []byte(storedChallenge)) != 1 {
		return nil, storage.ErrCLIChallengeMismatch // rollback via defer; code NOT consumed
	}

	var id string
	var createdAt time.Time
	insErr := tx.QueryRow(ctx, `
		INSERT INTO web_sessions (token_hash, user_id, oidc_subject, expires_at)
		SELECT $1, $2::uuid, $3, $4
		WHERE EXISTS (SELECT 1 FROM users WHERE id = $2::uuid AND deleted_at IS NULL)
		RETURNING id, created_at`, sess.TokenHash, userID, subject, sess.ExpiresAt).Scan(&id, &createdAt)
	if errors.Is(insErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("exchange cli code: %w", storage.ErrUserNotFound)
	}
	if insErr != nil {
		return nil, fmt.Errorf("exchange cli code: insert: %w", insErr)
	}

	if _, delErr := tx.Exec(ctx, `DELETE FROM cli_login_codes WHERE code_hash = $1`, codeHash); delErr != nil {
		return nil, fmt.Errorf("exchange cli code: delete: %w", delErr)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("exchange cli code: commit: %w", err)
	}

	sess.ID = id
	sess.UserID = userID
	sess.OIDCSubject = subject
	sess.CreatedAt = createdAt
	return sess, nil
}

// DeleteExpiredCLICodes removes expired code rows.
func (s *AuthStore) DeleteExpiredCLICodes(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM cli_login_codes WHERE expires_at <= $1`, s.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired cli codes: %w", err)
	}
	return tag.RowsAffected(), nil
}
```

Add `"crypto/subtle"` to the imports of `internal/storage/postgres/web_auth.go`.

- [ ] **Step 4: Write the failing integration tests**

Append to `internal/storage/postgres/web_auth_test.go` (this file already has `//go:build integration` and a helper that seeds a user and returns an `*AuthStore`; mirror the existing tests' setup — inspect the top of the file for the exact helper name, e.g. `newTestAuthStore(t)` returning the store and a seeded `userID`):

```go
func TestExchangeCLICode_RoundTrip(t *testing.T) {
	t.Parallel()
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()

	codeHash := sha256.Sum256([]byte("rawcode"))
	challenge := "CHALLENGE"
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "oidc:subj", challenge, time.Now().Add(time.Minute)))

	tokenHash := sha256.Sum256([]byte("spgr_ws_token"))
	sess, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour),
	}, challenge)
	require.NoError(t, err)
	require.Equal(t, userID, sess.UserID)
	require.Equal(t, "oidc:subj", sess.OIDCSubject)

	// Single-use: second exchange fails.
	_, err = auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour),
	}, challenge)
	require.ErrorIs(t, err, storage.ErrCLICodeNotFound)
}

func TestExchangeCLICode_ChallengeMismatchLeavesCode(t *testing.T) {
	t.Parallel()
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()

	codeHash := sha256.Sum256([]byte("rawcode2"))
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "", "GOOD", time.Now().Add(time.Minute)))

	tokenHash := sha256.Sum256([]byte("tok"))
	_, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "BAD")
	require.ErrorIs(t, err, storage.ErrCLIChallengeMismatch)

	// Code is NOT consumed; a correct verifier still works.
	sess, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "GOOD")
	require.NoError(t, err)
	require.Equal(t, userID, sess.UserID)
}

func TestExchangeCLICode_Expired(t *testing.T) {
	t.Parallel()
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()
	codeHash := sha256.Sum256([]byte("rawcode3"))
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "", "C", time.Now().Add(-time.Second)))
	_, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: codeHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "C")
	require.ErrorIs(t, err, storage.ErrCLICodeNotFound)
}

func TestLoginFlow_CLIFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	auth, _ := newAuthStoreWithUser(t)
	ctx := context.Background()
	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "s", Nonce: "n", CodeVerifier: "v", ProviderID: "p",
		CLICallback: "http://127.0.0.1:5000/callback", CLIState: "cs", CLIChallenge: "cc",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	got, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:5000/callback", got.CLICallback)
	require.Equal(t, "cs", got.CLIState)
	require.Equal(t, "cc", got.CLIChallenge)
}

// TestLoginFlow_WebStillWorks guards against the nullable-scan regression:
// the new NOT NULL DEFAULT '' columns must let a web-only flow round-trip.
func TestLoginFlow_WebStillWorks(t *testing.T) {
	t.Parallel()
	auth, _ := newAuthStoreWithUser(t)
	ctx := context.Background()
	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "s", Nonce: "n", CodeVerifier: "v", ProviderID: "p",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	got, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "", got.CLICallback)
}
```

If the existing test file uses a different seed helper than `newAuthStoreWithUser`, reuse that one verbatim (check the top of `web_auth_test.go` — e.g. the helper used by `TestWebAuth_CreateSession`). Add `"crypto/sha256"` to the test imports if absent.

- [ ] **Step 5: Run the integration tests (requires Docker)**

Run: `task test:integration -- -run 'TestExchangeCLICode|TestLoginFlow' ./internal/storage/postgres/`
Expected: PASS for all five tests. (If `task test:integration` does not accept `-run`, run `go test -tags integration -run 'TestExchangeCLICode|TestLoginFlow' ./internal/storage/postgres/`.)

- [ ] **Step 6: Build the whole module**

Run: `go build ./...`
Expected: success.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(storage): CLI login code create/exchange and flow columns

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 4: Shared strict literal-loopback host check

**Files:**

- Create: `internal/auth/loopback.go`
- Test: `internal/auth/loopback_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/loopback_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestIsLiteralLoopbackHost(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"127.0.0.1":           true,
		"::1":                 true,
		"localhost":           false,
		"127.0.0.1.evil.com":  false,
		"127.0.0.1.":          false,
		"0.0.0.0":             false,
		"2130706433":          false,
		"::ffff:127.0.0.1":    false,
		"":                    false,
		"127.0.0.2":           false,
	}
	for host, want := range cases {
		if got := auth.IsLiteralLoopbackHost(host); got != want {
			t.Errorf("IsLiteralLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/auth/ -run TestIsLiteralLoopbackHost`
Expected: FAIL (undefined: `auth.IsLiteralLoopbackHost`).

- [ ] **Step 3: Implement**

Create `internal/auth/loopback.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// IsLiteralLoopbackHost reports whether host is exactly the IPv4 or IPv6
// loopback literal. It deliberately rejects "localhost" (resolver/DNS-rebinding
// risk, RFC 8252 §8.3) and any non-canonical form. Callers pass the result of
// net/url's url.Hostname(), which strips the brackets from "[::1]" → "::1".
func IsLiteralLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/auth/ -run TestIsLiteralLoopbackHost`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(auth): strict literal-loopback host check

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 5: Server handlers — CLI start params, callback branch, exchange endpoint

**Files:**

- Modify: `internal/server/auth_oidc_handler.go`
- Test: `internal/server/auth_oidc_handler_test.go`

- [ ] **Step 1: Add `CLILoginEnabled` to config + handler and a `cli_callback` validator**

In `internal/server/auth_oidc_handler.go`, add `CLILoginEnabled bool` to `OIDCLoginConfig` (after `Limiter`) and to `oidcLoginHandler` (after `flowTTL`). In `RegisterOIDCLoginHandlers`, set `cliLoginEnabled: cfg.CLILoginEnabled` on the handler, and register the exchange route only when enabled:

```go
	mux.HandleFunc("/api/auth/oidc/providers", h.handleProviders)
	mux.Handle("/api/auth/oidc/callback", limit(http.HandlerFunc(h.handleCallback)))
	mux.Handle("/api/auth/oidc/{provider}/start", limit(http.HandlerFunc(h.handleStart)))
	if cfg.CLILoginEnabled {
		mux.Handle("/api/auth/cli/exchange", limit(http.HandlerFunc(h.handleCLIExchange)))
	}
```

Add this validator function (and the imports `net/url`, `strings`, `crypto/sha256`, `encoding/json`, `time` — most are already present; add `net/url` and `strings`):

```go
// validateCLICallback enforces a strict loopback redirect target. Returns the
// parsed URL (query/fragment-free, path "/callback") or false.
func validateCLICallback(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Path != "/callback" ||
		!auth.IsLiteralLoopbackHost(u.Hostname()) {
		return nil, false
	}
	return u, true
}
```

- [ ] **Step 2: Read CLI params in `handleStart`**

In `handleStart`, after resolving the provider `p` and before generating `state`, add:

```go
	cliCallback := r.URL.Query().Get("cli_callback")
	cliState := r.URL.Query().Get("cli_state")
	cliChallenge := r.URL.Query().Get("cli_challenge")
	if cliCallback != "" {
		if !h.cliLoginEnabled {
			writeJSONError(w, http.StatusForbidden, "cli_login_disabled")
			return
		}
		if _, ok := validateCLICallback(cliCallback); !ok {
			writeJSONError(w, http.StatusBadRequest, "invalid cli_callback")
			return
		}
		if cliChallenge == "" || cliState == "" {
			writeJSONError(w, http.StatusBadRequest, "cli_challenge and cli_state required")
			return
		}
	}
```

Then persist them on the flow — extend the `CreateLoginFlow` call's `&storage.LoginFlow{...}` literal to include:

```go
		CLICallback: cliCallback, CLIState: cliState, CLIChallenge: cliChallenge,
```

- [ ] **Step 3: Branch in `handleCallback`**

In `handleCallback`, replace the tail (from `// Mint the server session.` through the final `http.Redirect(w, r, "/", http.StatusFound)`) with:

```go
	// CLI flow: deliver a one-time code to the validated loopback, no cookie.
	if flow.CLICallback != "" {
		cb, ok := validateCLICallback(flow.CLICallback) // re-validate; never trust stored value into a redirect
		if !ok {
			fail("exchange")
			return
		}
		code, err := randToken()
		if err != nil {
			fail("temporary")
			return
		}
		sum := sha256.Sum256([]byte(code))
		if err := h.webAuth.CreateCLICode(r.Context(), sum[:], id.UserID, subjectOnly(id.Subject), flow.CLIChallenge, time.Now().Add(60*time.Second)); err != nil {
			if errors.Is(err, storage.ErrUserNotFound) {
				fail("unauthorized")
				return
			}
			fail("temporary")
			return
		}
		q := url.Values{"cli_state": {flow.CLIState}, "code": {code}}
		cb.RawQuery = q.Encode()
		http.Redirect(w, r, cb.String(), http.StatusFound) //nolint:gosec // G710: target validated to literal loopback via validateCLICallback
		return
	}

	// Web flow: mint the server session (unchanged).
	token, err := randSessionToken()
	if err != nil {
		fail("temporary")
		return
	}
	sum := sha256.Sum256([]byte(token))
	if _, err := h.webAuth.CreateSession(r.Context(), &storage.Session{
		TokenHash: sum[:], UserID: id.UserID, OIDCSubject: subjectOnly(id.Subject),
		ExpiresAt: time.Now().Add(h.sessionTTL),
	}); err != nil {
		slog.LogAttrs(r.Context(), slog.LevelError, "oidc: create session", slog.Any("error", err))
		fail("temporary")
		return
	}
	establishSession(w, r, token)
	http.Redirect(w, r, "/", http.StatusFound)
```

- [ ] **Step 4: Add the exchange handler**

Append to `internal/server/auth_oidc_handler.go`:

```go
type cliExchangeRequest struct {
	Code        string `json:"code"`
	CLIVerifier string `json:"cli_verifier"`
}

type cliExchangeResponse struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	OIDCSubject string    `json:"oidc_subject"`
}

func (h *oidcLoginHandler) handleCLIExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req cliExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.CLIVerifier == "" {
		writeJSONError(w, http.StatusBadRequest, "missing code or verifier")
		return
	}
	token, err := randSessionToken()
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		return
	}
	tokenHash := sha256.Sum256([]byte(token))
	codeHash := sha256.Sum256([]byte(req.Code))
	gotChallenge := oauth2.S256ChallengeFromVerifier(req.CLIVerifier)
	expiresAt := time.Now().Add(h.sessionTTL)

	sess, err := h.webAuth.ExchangeCLICode(r.Context(), codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: expiresAt,
	}, gotChallenge)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrCLICodeNotFound), errors.Is(err, storage.ErrCLIChallengeMismatch):
			writeJSONError(w, http.StatusBadRequest, "invalid or expired code")
		case errors.Is(err, storage.ErrUserNotFound):
			writeJSONError(w, http.StatusForbidden, "account_unavailable")
		default:
			slog.LogAttrs(r.Context(), slog.LevelError, "oidc: cli exchange", slog.Any("error", err))
			writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(cliExchangeResponse{ //nolint:errcheck // best-effort write
		Token: token, ExpiresAt: sess.ExpiresAt, OIDCSubject: sess.OIDCSubject,
	})
}
```

Ensure imports include `golang.org/x/oauth2` (already imported) and `github.com/specgraph/specgraph/internal/auth` (already imported).

- [ ] **Step 5: Write handler unit tests**

The existing `auth_oidc_handler_test.go` has a `fakeWA` implementing `storage.WebAuthStore` and a `newTestOIDCMux` helper. Add `CreateCLICode`/`ExchangeCLICode`/`DeleteExpiredCLICodes` methods to `fakeWA` (record args / return configurable values), then add:

```go
func TestValidateCLICallback(t *testing.T) {
	t.Parallel()
	ok := []string{"http://127.0.0.1:5000/callback", "http://[::1]:5000/callback"}
	for _, s := range ok {
		if _, valid := validateCLICallback(s); !valid {
			t.Errorf("want valid: %s", s)
		}
	}
	bad := []string{
		"https://127.0.0.1:5000/callback", "http://localhost:5000/callback",
		"http://127.0.0.1.evil.com/callback", "http://user@127.0.0.1/callback",
		"http://127.0.0.1/other", "http://127.0.0.1/callback?x=1", "http://127.0.0.1/callback#y",
	}
	for _, s := range bad {
		if _, valid := validateCLICallback(s); valid {
			t.Errorf("want invalid: %s", s)
		}
	}
}

func TestHandleStart_CLIDisabled(t *testing.T) {
	t.Parallel()
	// Build a mux with CLILoginEnabled=false (extend newTestOIDCMux or inline
	// RegisterOIDCLoginHandlers with the flag false) and one provider "entra".
	mux := newTestOIDCMuxCLI(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/entra/start?cli_callback=http://127.0.0.1:5000/callback&cli_state=s&cli_challenge=c", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHandleCLIExchange_Success(t *testing.T) {
	t.Parallel()
	wa := &fakeWA{exchangeSubject: "subj"} // fakeWA.ExchangeCLICode returns the passed sess with OIDCSubject set
	mux := newTestOIDCMuxWith(t, true, wa)
	body := `{"code":"abc","cli_verifier":"verifier"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/cli/exchange", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp cliExchangeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Token, "spgr_ws_") || resp.OIDCSubject != "subj" {
		t.Fatalf("bad response: %+v", resp)
	}
}

func TestHandleCLIExchange_BadCode(t *testing.T) {
	t.Parallel()
	wa := &fakeWA{exchangeErr: storage.ErrCLICodeNotFound}
	mux := newTestOIDCMuxWith(t, true, wa)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/cli/exchange", strings.NewReader(`{"code":"x","cli_verifier":"y"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

Add the small test helpers `newTestOIDCMuxCLI(t, enabled)` and `newTestOIDCMuxWith(t, enabled, wa)` next to the existing `newTestOIDCMux`, passing `CLILoginEnabled: enabled` into `RegisterOIDCLoginHandlers`. Give `fakeWA` the fields `exchangeSubject string` and `exchangeErr error`; its `ExchangeCLICode` returns `exchangeErr` if set, else sets `sess.OIDCSubject = exchangeSubject` and returns `sess, nil`.

- [ ] **Step 6: Run the handler tests**

Run: `go test ./internal/server/ -run 'TestValidateCLICallback|TestHandleStart_CLIDisabled|TestHandleCLIExchange'`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat(server): CLI loopback login start/callback/exchange handlers

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 6: Logout-via-bearer

**Files:**

- Modify: `internal/server/auth_handler.go`
- Test: `internal/server/auth_handler_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/server/auth_handler_test.go`, add (the file already has a `logoutFakeWA` recording `RevokeSession` calls):

```go
func TestHandleLogout_BearerSession(t *testing.T) {
	t.Parallel()
	wa := &logoutFakeWA{}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, stubResolver{}, wa, func(h http.Handler) http.Handler { return h })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer spgr_ws_abc")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	want := sha256.Sum256([]byte("spgr_ws_abc"))
	if !wa.revokedWith(want[:]) {
		t.Fatal("expected RevokeSession for the bearer session token")
	}
}

func TestHandleLogout_BearerAPIKeyIgnored(t *testing.T) {
	t.Parallel()
	wa := &logoutFakeWA{}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, stubResolver{}, wa, func(h http.Handler) http.Handler { return h })
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_key")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if wa.revokeCalled {
		t.Fatal("RevokeSession must NOT be called for a non-spgr_ws_ bearer")
	}
}
```

If `logoutFakeWA` lacks `revokedWith`/`revokeCalled`, add them (record the hash passed to `RevokeSession`). Reuse the existing `stubResolver` in that test file; if none exists, pass the package's existing fake resolver.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/server/ -run TestHandleLogout_Bearer`
Expected: FAIL (bearer path not handled).

- [ ] **Step 3: Implement**

In `internal/server/auth_handler.go`, replace the body of `handleLogout` with:

```go
func handleLogout(w http.ResponseWriter, r *http.Request, webAuth storage.WebAuthStore) {
	if webAuth != nil {
		if tok := bearerSessionToken(r); tok != "" {
			sum := sha256.Sum256([]byte(tok))
			if revErr := webAuth.RevokeSession(r.Context(), sum[:]); revErr != nil {
				slog.LogAttrs(r.Context(), slog.LevelWarn, "logout: revoke session", slog.Any("error", revErr))
			}
		}
	}
	c := sessionCookie("", r) //nolint:gosec // G124: sessionCookie sets HttpOnly/SameSite/dynamic Secure
	c.MaxAge = -1
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}

// bearerSessionToken returns a spgr_ws_ session token from the cookie or an
// Authorization: Bearer header. Non-session values (e.g. API keys) yield "".
func bearerSessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && strings.HasPrefix(c.Value, "spgr_ws_") {
		return c.Value
	}
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, prefix) {
		if tok := strings.TrimSpace(h[len(prefix):]); strings.HasPrefix(tok, "spgr_ws_") {
			return tok
		}
	}
	return ""
}
```

(`strings` and `crypto/sha256` are already imported in this file.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/server/ -run TestHandleLogout`
Expected: PASS (including the pre-existing cookie logout test).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(server): logout accepts spgr_ws_ bearer token

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 7: Config flag + serve wiring

**Files:**

- Modify: `internal/config/global.go`
- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Add the config field**

In `internal/config/global.go`, add to `OIDCConfig` (after `SessionTTL`):

```go
	// CLILoginEnabled gates the `specgraph login` broker (loopback redirect +
	// /api/auth/cli/exchange). Defaults to true (set in globalDefaults).
	CLILoginEnabled bool `yaml:"cli_login_enabled" koanf:"cli_login_enabled"`
```

- [ ] **Step 2: Default it to true**

In `globalDefaults()`, add an `Auth` section to the returned struct literal (it currently has none) so the default merges like `Log.Requests`:

```go
		Auth: AuthConfig{
			OIDC: OIDCConfig{CLILoginEnabled: true},
		},
```

- [ ] **Step 3: Wire it into serve.go**

In `cmd/specgraph/serve.go`, add to the `server.OIDCLoginConfig{...}` literal (around line 215):

```go
		CLILoginEnabled: cfg.Auth.OIDC.CLILoginEnabled,
```

- [ ] **Step 4: Add a default-value test**

In the config package's existing loader test file (e.g. `internal/config/loader_internal_test.go` or `global_test.go` — match where `Log.Requests` default is asserted), add:

```go
func TestDefault_CLILoginEnabled(t *testing.T) {
	t.Parallel()
	cfg := globalDefaults()
	if !cfg.Auth.OIDC.CLILoginEnabled {
		t.Fatal("CLILoginEnabled should default to true")
	}
}
```

(If asserting via the public load path instead, load a minimal config with no `auth:` block and assert `cfg.Auth.OIDC.CLILoginEnabled == true`, then load one with `auth.oidc.cli_login_enabled: false` and assert it is `false`.)

- [ ] **Step 5: Build + test**

Run: `go build ./... && go test ./internal/config/ -run TestDefault_CLILoginEnabled`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(config): cli_login_enabled flag, default true

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 8: Cross-platform browser opener

**Files:**

- Create: `internal/browser/browser.go`
- Test: `internal/browser/browser_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/browser/browser_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package browser

import "testing"

func TestOpenCommand(t *testing.T) {
	t.Parallel()
	name, args := openCommand("darwin", "https://example.com")
	if name != "open" || len(args) != 1 || args[0] != "https://example.com" {
		t.Fatalf("darwin: got %s %v", name, args)
	}
	name, _ = openCommand("linux", "https://example.com")
	if name != "xdg-open" {
		t.Fatalf("linux: got %s", name)
	}
	name, args = openCommand("windows", "https://example.com")
	if name != "rundll32" || args[0] != "url.dll,FileProtocolHandler" {
		t.Fatalf("windows: got %s %v", name, args)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/browser/`
Expected: FAIL (package/function missing).

- [ ] **Step 3: Implement**

Create `internal/browser/browser.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package browser opens a URL in the user's default browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openCommand returns the platform command + args to open url.
func openCommand(goos, url string) (string, []string) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		return "open", []string{url}
	default:
		return "xdg-open", []string{url}
	}
}

// Open launches the default browser at url. It returns an error if the platform
// opener cannot be started; callers should fall back to printing the URL.
func Open(url string) error {
	name, args := openCommand(runtime.GOOS, url)
	if err := exec.Command(name, args...).Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/browser/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(browser): cross-platform URL opener

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 9: `specgraph login`

**Files:**

- Create: `cmd/specgraph/login.go`
- Test: `cmd/specgraph/login_test.go`

- [ ] **Step 1: Implement the command**

Create `cmd/specgraph/login.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/browser"
	"github.com/specgraph/specgraph/internal/credentials"
	"github.com/specgraph/specgraph/internal/xdg"
)

var (
	loginProviderFlag string
	loginServerFlag   string
	loginNoBrowser    bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in via OIDC in the browser and store a session token",
	RunE:  runLogin,
}

func runLogin(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()

	serverURL := loginServerFlag
	if serverURL == "" {
		resolved, _, err := resolveBaseURL()
		if err != nil {
			return fmt.Errorf("resolve server URL: %w", err)
		}
		serverURL = resolved
	}
	serverURL = strings.TrimRight(serverURL, "/")

	if err := guardHTTPS(serverURL); err != nil {
		return err
	}
	if err := guardRemote(); err != nil {
		return err
	}

	provider, err := pickProvider(cmd.Context(), serverURL, loginProviderFlag)
	if err != nil {
		return err
	}

	// PKCE + CSRF secrets (CLI↔server leg).
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	cliState := oauth2.GenerateVerifier() // reuse as a high-entropy opaque token

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start loopback listener: %w", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	cbURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	authURL := fmt.Sprintf("%s/api/auth/oidc/%s/start?%s", serverURL, url.PathEscape(provider),
		url.Values{"cli_callback": {cbURL}, "cli_state": {cliState}, "cli_challenge": {challenge}}.Encode())

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: loopbackHandler(port, cliState, codeCh, errCh), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	if loginNoBrowser {
		fmt.Fprintf(w, "Open this URL to log in:\n\n  %s\n\n", authURL) //nolint:errcheck
	} else if err := browser.Open(authURL); err != nil {
		fmt.Fprintf(w, "Could not open a browser. Open this URL to log in:\n\n  %s\n\n", authURL) //nolint:errcheck
	}
	fmt.Fprintln(w, "Waiting for the browser to complete login…") //nolint:errcheck

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return err
	case <-time.After(3 * time.Minute):
		return errors.New("login timed out, please retry")
	case <-cmd.Context().Done():
		return cmd.Context().Err()
	}

	token, subject, err := exchangeCode(cmd.Context(), serverURL, code, verifier)
	if err != nil {
		return err
	}

	if err := writeCredentials(w, serverURL, token, subject); err != nil {
		return err
	}
	fmt.Fprintf(w, "Logged in as %s on %s\n", subject, serverURL) //nolint:errcheck
	return nil
}

// loopbackHandler validates the Host header + cli_state and forwards the code.
func loopbackHandler(port int, cliState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	wantHost := fmt.Sprintf("127.0.0.1:%d", port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != wantHost {
			http.Error(w, "bad host", http.StatusBadRequest)
			return
		}
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("cli_state")), []byte(cliState)) != 1 {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- errors.New("loopback state mismatch — aborting")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("loopback callback missing code")
			return
		}
		fmt.Fprintln(w, "Login complete. You can close this tab and return to the terminal.") //nolint:errcheck
		codeCh <- code
	})
}

func guardHTTPS(serverURL string) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("parse server URL: %w", err)
	}
	if u.Scheme == "https" || auth.IsLiteralLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("refusing to log in over plain http to a non-loopback server (%s); use https", serverURL)
}

func guardRemote() error {
	if loginNoBrowser {
		return nil
	}
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return errors.New("browser-based login isn't available over SSH/headless sessions; create an API key instead: specgraph auth api-key create")
	}
	return nil
}

type providersResponse struct {
	Providers []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"providers"`
}

func pickProvider(ctx context.Context, serverURL, want string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/api/auth/oidc/providers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("list providers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var pr providersResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", fmt.Errorf("decode providers: %w", err)
	}
	if len(pr.Providers) == 0 {
		return "", errors.New("server has no interactive OIDC providers; use specgraph auth api-key create")
	}
	if want != "" {
		for _, p := range pr.Providers {
			if p.ID == want {
				return p.ID, nil
			}
		}
		return "", fmt.Errorf("provider %q not found on server", want)
	}
	if len(pr.Providers) == 1 {
		return pr.Providers[0].ID, nil
	}
	return "", errors.New("multiple OIDC providers configured; pass --provider <id>")
}

func exchangeCode(ctx context.Context, serverURL, code, verifier string) (token, subject string, err error) {
	body, _ := json.Marshal(map[string]string{"code": code, "cli_verifier": verifier})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/auth/cli/exchange", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("exchange code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			return "", "", errors.New("login expired, please retry")
		}
		return "", "", fmt.Errorf("exchange failed: server returned %d", resp.StatusCode)
	}
	var er struct {
		Token       string `json:"token"`
		OIDCSubject string `json:"oidc_subject"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return "", "", fmt.Errorf("decode exchange response: %w", err)
	}
	return er.Token, er.OIDCSubject, nil
}

func writeCredentials(w interface{ Write([]byte) (int, error) }, serverURL, token, subject string) error {
	credPath := xdg.CredentialsFile()
	f, err := credentials.Load(credPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	if existing := f.TokenFor(serverURL); existing != "" && !strings.HasPrefix(existing, "spgr_ws_") {
		fmt.Fprintf(w, "warning: replacing a non-session credential for %s\n", serverURL) //nolint:errcheck
	}
	f.Upsert(serverURL, credentials.ServerCreds{Token: token, Label: "oidc:" + subject})
	if err := f.Save(credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}

func init() {
	loginCmd.Flags().StringVar(&loginProviderFlag, "provider", "", "OIDC provider id (skips the picker)")
	loginCmd.Flags().StringVar(&loginServerFlag, "server", "", "server base URL (overrides resolved)")
	loginCmd.Flags().BoolVar(&loginNoBrowser, "no-browser", false, "print the URL instead of opening a browser")
	rootCmd.AddCommand(loginCmd)
}
```

- [ ] **Step 2: Write unit tests for the pure helpers**

Create `cmd/specgraph/login_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuardHTTPS(t *testing.T) {
	t.Parallel()
	if err := guardHTTPS("http://127.0.0.1:9090"); err != nil {
		t.Errorf("loopback http should pass: %v", err)
	}
	if err := guardHTTPS("https://api.example.com"); err != nil {
		t.Errorf("https should pass: %v", err)
	}
	if err := guardHTTPS("http://api.example.com"); err == nil {
		t.Error("remote http should fail")
	}
}

func TestLoopbackHandler_StateMismatch(t *testing.T) {
	t.Parallel()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := loopbackHandler(5000, "secret", codeCh, errCh)
	req := httptest.NewRequest(http.MethodGet, "/callback?cli_state=wrong&code=abc", nil)
	req.Host = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	select {
	case <-errCh:
	default:
		t.Fatal("expected an error on the channel")
	}
}

func TestLoopbackHandler_Success(t *testing.T) {
	t.Parallel()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	h := loopbackHandler(5000, "secret", codeCh, errCh)
	req := httptest.NewRequest(http.MethodGet, "/callback?cli_state=secret&code=thecode", nil)
	req.Host = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := <-codeCh; got != "thecode" {
		t.Fatalf("code = %q", got)
	}
}

func TestPickProvider(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"providers":[{"id":"entra","display_name":"Entra"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()
	id, err := pickProvider(t.Context(), srv.URL, "")
	if err != nil || id != "entra" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

func TestExchangeCode_BadRequest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	_, _, err := exchangeCode(t.Context(), srv.URL, "c", "v")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("want expired error, got %v", err)
	}
}
```

(If the Go toolchain version predates `t.Context()`, use `context.Background()` and add the import.)

- [ ] **Step 3: Run the tests**

Run: `go test ./cmd/specgraph/ -run 'TestGuardHTTPS|TestLoopbackHandler|TestPickProvider|TestExchangeCode'`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(cli): specgraph login (browser OIDC loopback)

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 10: `specgraph logout`

**Files:**

- Create: `cmd/specgraph/logout.go`
- Test: `cmd/specgraph/logout_test.go`

- [ ] **Step 1: Implement the command**

Create `cmd/specgraph/logout.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/specgraph/specgraph/internal/credentials"
	"github.com/specgraph/specgraph/internal/xdg"
)

var logoutServerFlag string

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke the stored session and remove local credentials",
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	serverURL := logoutServerFlag
	if serverURL == "" {
		resolved, _, err := resolveBaseURL()
		if err != nil {
			return fmt.Errorf("resolve server URL: %w", err)
		}
		serverURL = resolved
	}
	serverURL = strings.TrimRight(serverURL, "/")

	credPath := xdg.CredentialsFile()
	f, err := credentials.Load(credPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	token := f.TokenFor(serverURL)
	if token == "" {
		fmt.Fprintf(w, "No stored credential for %s\n", serverURL) //nolint:errcheck
		return nil
	}

	if strings.HasPrefix(token, "spgr_ws_") {
		if err := revokeSession(cmd.Context(), serverURL, token); err != nil {
			fmt.Fprintf(w, "warning: server revoke failed: %v\n", err) //nolint:errcheck
		}
	} else {
		fmt.Fprintf(w, "warning: removing a non-session credential (API key) for %s\n", serverURL) //nolint:errcheck
	}

	delete(f.Servers, strings.TrimRight(serverURL, "/"))
	if err := f.Save(credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Fprintf(w, "Logged out of %s\n", serverURL) //nolint:errcheck
	return nil
}

func revokeSession(ctx context.Context, serverURL, token string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func init() {
	logoutCmd.Flags().StringVar(&logoutServerFlag, "server", "", "server base URL (overrides resolved)")
	rootCmd.AddCommand(logoutCmd)
}
```

> Note: `credentials.File.Servers` is an exported map, so `delete(f.Servers, key)` is valid. The key normalization (`TrimRight` of trailing slashes) mirrors `credentials.normalize`; since `serverURL` is already trimmed above, the delete key matches the `Upsert` key written by login.

- [ ] **Step 2: Write the test**

Create `cmd/specgraph/logout_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRevokeSession(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if err := revokeSession(t.Context(), srv.URL, "spgr_ws_abc"); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer spgr_ws_abc" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

func TestRevokeSession_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := revokeSession(t.Context(), srv.URL, "spgr_ws_abc"); err == nil {
		t.Fatal("expected error on 500")
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./cmd/specgraph/ -run TestRevokeSession`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(cli): specgraph logout

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Task 11: Full gate + docs touch-up

**Files:**

- Modify: `cmd/specgraph/docs_test.go` (only if it asserts a fixed command list — add `login`/`logout`)

- [ ] **Step 1: Run the full check gate**

Run: `task check`
Expected: PASS (fmt, license, lint incl. `revive` package comments + `wrapcheck`, build, unit tests). Fix any lint findings inline (common: `wrapcheck` wants `fmt.Errorf("...: %w", err)` wrapping on returned errors — already applied in the code above; `errcheck` is silenced with `//nolint` on best-effort writes).

- [ ] **Step 2: Run the postgres integration suite (Docker)**

Run: `task test:integration`
Expected: PASS (includes Task 3's `cli_login_codes` tests).

- [ ] **Step 3: Manual smoke (optional, requires a configured interactive provider)**

Run a local server with one interactive OIDC provider, then:

```bash
specgraph login --provider <id>
specgraph auth whoami      # shows the OIDC identity
specgraph logout
```

Expected: browser opens, login completes, `whoami` resolves via the `spgr_ws_` session, `logout` revokes and clears the entry.

- [ ] **Step 4: Final commit**

```bash
jj commit -m "test: full gate for CLI OIDC login

Signed-off-by: Sean Brandt <seanb4t@users.noreply.github.com>"
```

---

## Self-Review notes (already reconciled against the spec)

- **PKCE binding:** CLI generates `verifier`/`challenge` (Task 9); server stores `challenge` with the code (Task 5) and verifies `S256(verifier)` at exchange (Task 5 → Task 3 `ExchangeCLICode`). ✓
- **Strict loopback validation:** `validateCLICallback` (Task 5) uses `IsLiteralLoopbackHost` (Task 4); re-validated at callback. ✓
- **Atomic exchange:** `ExchangeCLICode` runs all statements on one `pgx.Tx`, never calls `CreateSession` (Task 3). ✓
- **Soft-deleted mid-flow:** `INSERT … WHERE EXISTS (not deleted)` → `ErrUserNotFound` → terminal 403 (Tasks 3, 5). ✓
- **Config gate:** `cli_login_enabled` default true, fail-fast 403, unregistered exchange route (Tasks 5, 7). ✓
- **HTTPS guard + creds label + overwrite warning:** `guardHTTPS`, `oidc:<subject>` label via exchange response, `credentials.Load` (not `resolveAPIKey`) for the warning (Task 9). ✓
- **Logout-via-bearer with prefix guard:** `bearerSessionToken` (Task 6); CLI logout only revokes `spgr_ws_` (Task 10). ✓
- **Remote/headless hard-error; non-TTY multi-provider hard-error:** `guardRemote`, `pickProvider` (Task 9). ✓
- **Method/type name consistency:** `ExchangeCLICode`, `CreateCLICode`, `IsLiteralLoopbackHost`, `validateCLICallback`, `handleCLIExchange`, `cliExchangeResponse`, `bearerSessionToken` are used identically across tasks. ✓
