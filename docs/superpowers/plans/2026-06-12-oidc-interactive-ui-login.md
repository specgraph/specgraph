# Interactive OIDC Login for the Web UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Sign in with <provider>" Authorization Code + PKCE flow to the SpecGraph web UI that issues a server-side opaque session, reusing the existing cookie/resolver path.

**Architecture:** New endpoints (`/api/auth/oidc/{providers,start,callback}`) drive a server-side OAuth2 code flow. Handshake state (`state`/`nonce`/`code_verifier`) and the issued session both live in Postgres (new `oidc_login_flows` + `web_sessions` tables behind a `WebAuthStore`). The session cookie carries only an opaque `spgr_ws_` id, resolved per-request by a new `resolveSession` path in the identity resolver. Interactive logins reuse the existing `resolveJWT`/JIT machinery (limiter bypassed via a context marker).

**Tech Stack:** Go 1.26, pgx v5 + goose migrations, `coreos/go-oidc/v3`, `golang.org/x/oauth2`, `golang.org/x/time/rate`, ConnectRPC, SvelteKit (Svelte 5 runes) embedded via `//go:embed`.

**Design spec:** `docs/superpowers/specs/2026-06-12-oidc-interactive-ui-login-design.md`

---

## Conventions in this codebase (read before starting)

- **Postgres access:** positional `row.Scan(&...)`, **no** `RowToStructByName`, **no** `db:` struct tags. Not-found → `errors.Is(err, pgx.ErrNoRows)` mapped to a `storage.Err*NotFound` sentinel. Mutation timestamps use the injectable `s.now()`; insert timestamps use SQL `DEFAULT now()` returned via `RETURNING`.
- **Auth store:** `*postgres.AuthStore` (`internal/storage/postgres/auth.go`) borrows `*Store`'s pool, owns the identity tables, runs `auth_migrations/*.sql` under the `goose_db_version_auth` table.
- **Resolver errors:** `auth.ErrUnauthenticated` (→ HTTP 401 / Connect `CodeUnauthenticated`) and `auth.ErrTransient` (→ HTTP 503 / `CodeUnavailable`). DB errors MUST wrap `ErrTransient`: `fmt.Errorf("%w: %w", ErrTransient, err)`.
- **Context keys:** unexported `struct{}` key types with `WithX`/`XFromContext` helpers (`internal/auth/context.go`).
- **Tests:** Go unit tests with testify (`require`/`assert`). Integration tests are `//go:build integration` + `package <pkg>_test`, using `postgrestest.SharedPool(t, ctx)`.
- **Run a single Go test:** `go test ./internal/<pkg>/ -run TestName -v`
- **Run integration tests:** `go test -tags integration ./internal/storage/postgres/ -run TestName -v` (requires Docker).
- **Build:** `task build` (regenerates proto + web). For Go-only: `go build ./...`.
- **Commit style:** Conventional Commits, DCO sign-off required. Use `git commit -s -m "feat(scope): ..."`.

## File structure (what each new/modified file is responsible for)

**New files:**

- `internal/storage/web_auth_domain.go` — `Session` + `LoginFlow` domain structs.
- `internal/storage/web_auth.go` — `WebAuthStore` interface.
- `internal/storage/postgres/auth_migrations/002_web_auth.sql` — `web_sessions` + `oidc_login_flows` tables.
- `internal/storage/postgres/web_auth.go` — `WebAuthStore` impl on `*AuthStore`.
- `internal/storage/postgres/web_auth_test.go` — integration tests for the store.
- `internal/auth/loginprovider.go` — `loginProvider` interface + `oidc` impl + `BuildLoginProviders`.
- `internal/auth/loginprovider_test.go` — unit tests for build/validate/exchange.
- `internal/server/auth_oidc_handler.go` — `/providers`, `/start`, `/callback` handlers + per-IP rate limiter.
- `internal/server/auth_oidc_handler_test.go` — httptest flow tests.
- `web/src/lib/oidc.svelte.ts` — OIDC provider-list fetch + `auth_error` message map.
- `site/docs/guides/oidc-login.md` — operator guide (Entra, Okta, GitHub-via-broker).

**Modified files:**

- `internal/config/global.go` — extend `OIDCProviderConfig` + `OIDCConfig`; add `ServerSection.TrustedProxy`.
- `internal/auth/oidc_verifier.go` — `OIDCClaims.Nonce` + `VerifyWithNonce`.
- `internal/auth/context.go` — `WithInteractiveLogin` / `InteractiveLoginFromContext`.
- `internal/auth/identitystore.go` — `WebAuth` field, `resolveSession`, `Resolve` dispatch, `jitResolve` limiter bypass.
- `internal/storage/errors.go` — `ErrSessionNotFound`, `ErrLoginFlowNotFound`.
- `internal/server/auth_handler.go` — `sessionCookie()` → `SameSite=Lax`, `establishSession`, logout revocation, `RegisterAuthHandlers` signature.
- `cmd/specgraph/serve.go` — build login providers, wire `WebAuthStore`, register handlers, start web-auth sweeper.
- `web/src/lib/components/LoginModal.svelte` — provider buttons.
- `web/src/routes/+layout.svelte` — surface `?auth_error=`.
- `site/docs/architecture.md` — describe interactive flow.

**Task order:** storage types/errors → migration → store impl + tests → verifier nonce → context marker → resolver session path → login providers → rate limiter → handlers → auth_handler changes → serve wiring → frontend → docs.

---

### Task 1: Extend config structs (provider fields + base_url/session_ttl + trusted_proxy)

**Files:**

- Modify: `internal/config/global.go` (`OIDCProviderConfig` ~237-244, `OIDCConfig` ~195-200, `ServerSection`)
- Test: `internal/config/global_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/global_test.go`:

```go
func TestLoadGlobal_OIDCInteractiveFields(t *testing.T) {
	yamlBody := []byte(`
auth:
  oidc:
    base_url: https://specgraph.example.com
    session_ttl: 8h
    providers:
      - id: entra
        kind: oidc
        interactive: true
        display_name: Microsoft Entra
        issuer: https://login.microsoftonline.com/tenant/v2.0
        client_id: app-id
        client_secret_env: SPECGRAPH_OIDC_ENTRA_SECRET
        scopes: [openid, profile, email]
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, yamlBody, 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.Equal(t, "https://specgraph.example.com", cfg.Auth.OIDC.BaseURL)
	require.Equal(t, 8*time.Hour, cfg.Auth.OIDC.SessionTTL)
	require.Len(t, cfg.Auth.OIDC.Providers, 1)
	p := cfg.Auth.OIDC.Providers[0]
	require.Equal(t, "oidc", p.Kind)
	require.True(t, p.Interactive)
	require.Equal(t, "Microsoft Entra", p.DisplayName)
	require.Equal(t, "SPECGRAPH_OIDC_ENTRA_SECRET", p.ClientSecretEnv)
	require.Equal(t, []string{"openid", "profile", "email"}, p.Scopes)
}
```

- [ ] **Step 2: Run it to confirm it fails to compile**

Run: `go test ./internal/config/ -run TestLoadGlobal_OIDCInteractiveFields -v`
Expected: compile error — unknown fields `BaseURL`, `SessionTTL`, `Kind`, `Interactive`, etc.

- [ ] **Step 3: Extend the structs**

In `internal/config/global.go`, replace `OIDCConfig` (lines ~195-200) with:

```go
// OIDCConfig wraps the OIDC provider list and JIT settings under a
// nested auth.oidc key. Replaces the flat AuthConfig.OIDCProviders.
type OIDCConfig struct {
	Providers []OIDCProviderConfig `yaml:"providers" koanf:"providers"`
	JITCreate JITCreateConfig      `yaml:"jit_create" koanf:"jit_create"`
	// BaseURL overrides the request-derived origin used to build the OIDC
	// redirect_uri. Required behind a proxy that rewrites Host. Empty = derive
	// from the request.
	BaseURL string `yaml:"base_url" koanf:"base_url"`
	// SessionTTL is the absolute lifetime of a web session minted by the
	// interactive login flow. Zero = default (12h, applied in applyPostLoad).
	SessionTTL time.Duration `yaml:"session_ttl" koanf:"session_ttl"`
}
```

Replace `OIDCProviderConfig` (lines ~237-244) with:

```go
// OIDCProviderConfig defines a single OIDC identity provider.
type OIDCProviderConfig struct {
	ID            string         `yaml:"id" koanf:"id"`
	Kind          string         `yaml:"kind" koanf:"kind"`               // "oidc" (default); reserved for "oauth2"
	Interactive   bool           `yaml:"interactive" koanf:"interactive"` // opt-in to the UI login flow
	DisplayName   string         `yaml:"display_name" koanf:"display_name"`
	Issuer        string         `yaml:"issuer" koanf:"issuer"`
	ClientID      string         `yaml:"client_id" koanf:"client_id"`
	ClientSecret  string         `yaml:"client_secret" koanf:"client_secret"`         // dev-only plaintext fallback
	ClientSecretEnv string       `yaml:"client_secret_env" koanf:"client_secret_env"` // preferred: env var name
	Audience      string         `yaml:"audience" koanf:"audience"`
	Scopes        []string       `yaml:"scopes" koanf:"scopes"`
	ClaimsMapping []ClaimMapping `yaml:"claims_mapping" koanf:"claims_mapping"`
}
```

Add a `TrustedProxy` field to `ServerSection` (find the struct; add alongside existing fields):

```go
	// TrustedProxy, when true, lets request-IP extraction trust
	// X-Forwarded-For/X-Real-Ip (e.g. behind a load balancer) for the OIDC
	// per-IP rate limiter. Default false → use RemoteAddr.
	TrustedProxy bool `yaml:"trusted_proxy" koanf:"trusted_proxy"`
```

Confirm `time` is imported in `global.go` (it is — `time.Duration` is used elsewhere). `decoderConf` already wires `StringToTimeDurationHookFunc`, so `session_ttl: 8h` decodes correctly.

- [ ] **Step 4: Run the test to confirm it passes**

Run: `go test ./internal/config/ -run TestLoadGlobal_OIDCInteractiveFields -v`
Expected: PASS

- [ ] **Step 5: Add session_ttl default in applyPostLoad**

In `applyPostLoad` (lines ~282-296), before the closing brace add:

```go
	if cfg.Auth.OIDC.SessionTTL <= 0 {
		cfg.Auth.OIDC.SessionTTL = 12 * time.Hour
	}
```

Add a quick assertion to the test from Step 1 is unnecessary; instead add a tiny test:

```go
func TestLoadGlobal_SessionTTLDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  backend: memory\n"), 0o600))
	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.Equal(t, 12*time.Hour, cfg.Auth.OIDC.SessionTTL)
}
```

Run: `go test ./internal/config/ -run 'TestLoadGlobal_(OIDCInteractiveFields|SessionTTLDefault)' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/global.go internal/config/global_test.go
git commit -s -m "feat(config): add interactive OIDC provider fields, base_url, session_ttl, trusted_proxy"
```

---

### Task 2: Add nonce-checked verification to OIDCVerifier

**Files:**

- Modify: `internal/auth/oidc_verifier.go`
- Test: `internal/auth/oidc_verifier_test.go` (create if absent)

- [ ] **Step 1: Write the failing test**

Create/append `internal/auth/oidc_verifier_test.go`. We test the nonce-comparison logic directly via a small helper so we don't need a live IdP:

```go
package auth

import "testing"

func TestNonceMatches(t *testing.T) {
	if !nonceMatches("abc", "abc") {
		t.Fatal("equal nonces should match")
	}
	if nonceMatches("abc", "xyz") {
		t.Fatal("different nonces must not match")
	}
	if nonceMatches("", "abc") {
		t.Fatal("empty expected nonce must not match")
	}
}
```

- [ ] **Step 2: Run it to confirm failure**

Run: `go test ./internal/auth/ -run TestNonceMatches -v`
Expected: FAIL — `nonceMatches` undefined.

- [ ] **Step 3: Implement nonce field, helper, and VerifyWithNonce**

In `internal/auth/oidc_verifier.go`:

Add `"crypto/subtle"` to the imports.

Add a `Nonce` field to `OIDCClaims`:

```go
type OIDCClaims struct {
	Issuer  string
	Subject string
	Email   string
	Nonce   string
	Raw     map[string]json.RawMessage
}
```

In `Verify`, after constructing `c`, set `c.Nonce = idToken.Nonce` (add the line right after `Raw: raw,` block, i.e. after the struct literal):

```go
	c.Nonce = idToken.Nonce
```

Add at the end of the file:

```go
// nonceMatches reports whether got equals want in constant time. An empty
// want is never a match (a login flow always sets a non-empty nonce).
func nonceMatches(got, want string) bool {
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// VerifyWithNonce validates the token like Verify and additionally requires
// the id_token's nonce claim to equal expectedNonce. Used by the interactive
// login callback; the bearer-token path uses Verify (no nonce).
func (v *OIDCVerifier) VerifyWithNonce(ctx context.Context, rawToken, expectedNonce string) (*OIDCClaims, error) {
	claims, err := v.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	if !nonceMatches(claims.Nonce, expectedNonce) {
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: OIDC nonce mismatch", slog.String("provider", v.providerID))
		return nil, fmt.Errorf("oidc verify: nonce mismatch")
	}
	return claims, nil
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/auth/ -run TestNonceMatches -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/oidc_verifier.go internal/auth/oidc_verifier_test.go
git commit -s -m "feat(auth): add VerifyWithNonce and Nonce claim to OIDCVerifier"
```

---

### Task 3: Storage sentinel errors + domain types

**Files:**

- Modify: `internal/storage/errors.go`
- Create: `internal/storage/web_auth_domain.go`

- [ ] **Step 1: Add sentinel errors**

In `internal/storage/errors.go`, in the `// --- Identity errors ---` section, add:

```go
// ErrSessionNotFound is returned when a web session does not exist (or is expired/revoked at lookup).
var ErrSessionNotFound = errors.New("web session not found")

// ErrLoginFlowNotFound is returned when an OIDC login-flow row does not exist or has expired.
var ErrLoginFlowNotFound = errors.New("oidc login flow not found")
```

- [ ] **Step 2: Create the domain types**

Create `internal/storage/web_auth_domain.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import "time"

// Session is a SpecGraph-issued opaque web session minted by the interactive
// OIDC login flow. The raw token is never stored — only its SHA-256 hash.
type Session struct {
	ID          string
	TokenHash   []byte // SHA-256 of the opaque session token
	UserID      string
	Issuer      string // OIDC issuer (audit)
	OIDCSubject string // audit
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time // nil = active
}

// IsActive reports whether the session is neither revoked nor expired as of now.
func (s *Session) IsActive(now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	return s.ExpiresAt.After(now)
}

// LoginFlow is server-side OAuth2 handshake state for one interactive login
// attempt. The opaque flow id (ID) is carried in the short-lived tx cookie;
// state/nonce/code_verifier never leave the server.
type LoginFlow struct {
	ID           string // opaque flow id (tx cookie value)
	State        string // CSRF token, constant-time compared at callback
	Nonce        string
	CodeVerifier string // PKCE
	ProviderID   string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/storage/`
Expected: success (no test yet — types are exercised in Task 5).

- [ ] **Step 4: Commit**

```bash
git add internal/storage/errors.go internal/storage/web_auth_domain.go
git commit -s -m "feat(storage): add Session/LoginFlow domain types and sentinel errors"
```

---

### Task 4: WebAuthStore interface

**Files:**

- Create: `internal/storage/web_auth.go`

- [ ] **Step 1: Create the interface**

Create `internal/storage/web_auth.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import "context"

// WebAuthStore persists interactive-login web sessions and short-lived OAuth2
// login-flow handshake state. Kept separate from UsersBackend so existing
// UsersBackend implementations/fakes are unaffected. Implemented by
// *postgres.AuthStore.
type WebAuthStore interface {
	// --- sessions ---

	// CreateSession inserts a new session row. s.ExpiresAt must be set.
	CreateSession(ctx context.Context, s *Session) (*Session, error)

	// LookupSessionByHash returns the session with the given token hash.
	// Returns ErrSessionNotFound on miss. Does NOT filter expired/revoked —
	// the caller (resolver) gates on Session.IsActive.
	LookupSessionByHash(ctx context.Context, tokenHash []byte) (*Session, error)

	// RevokeSession marks the session revoked by token hash. Idempotent.
	RevokeSession(ctx context.Context, tokenHash []byte) error

	// DeleteExpiredSessions removes expired/long-revoked rows; returns count.
	DeleteExpiredSessions(ctx context.Context) (int64, error)

	// --- login-flow state ---

	// CreateLoginFlow inserts a flow row and returns its opaque id (the PK).
	CreateLoginFlow(ctx context.Context, f *LoginFlow) (flowID string, err error)

	// ConsumeLoginFlow atomically deletes and returns the flow for flowID.
	// Returns ErrLoginFlowNotFound if the id is unknown or already expired.
	ConsumeLoginFlow(ctx context.Context, flowID string) (*LoginFlow, error)

	// DeleteExpiredLoginFlows removes expired flow rows; returns count.
	DeleteExpiredLoginFlows(ctx context.Context) (int64, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/storage/`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/web_auth.go
git commit -s -m "feat(storage): add WebAuthStore interface"
```

---

### Task 5: Migration — web_sessions + oidc_login_flows

**Files:**

- Create: `internal/storage/postgres/auth_migrations/002_web_auth.sql`

- [ ] **Step 1: Create the migration**

Create `internal/storage/postgres/auth_migrations/002_web_auth.sql`:

```sql
-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE TABLE web_sessions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash    bytea NOT NULL UNIQUE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer        text NOT NULL DEFAULT '' CHECK (length(issuer) <= 512),
    oidc_subject  text NOT NULL DEFAULT '' CHECK (length(oidc_subject) <= 255),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);

CREATE INDEX web_sessions_user   ON web_sessions(user_id);
CREATE INDEX web_sessions_expiry ON web_sessions(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE oidc_login_flows (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state         text NOT NULL CHECK (length(state) <= 512),
    nonce         text NOT NULL CHECK (length(nonce) <= 512),
    code_verifier text NOT NULL CHECK (length(code_verifier) <= 256),
    provider_id   text NOT NULL CHECK (length(provider_id) <= 128),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL
);

CREATE INDEX oidc_login_flows_expiry ON oidc_login_flows(expires_at);

-- +goose Down

DROP INDEX IF EXISTS oidc_login_flows_expiry;
DROP TABLE IF EXISTS oidc_login_flows;
DROP INDEX IF EXISTS web_sessions_expiry;
DROP INDEX IF EXISTS web_sessions_user;
DROP TABLE IF EXISTS web_sessions;
```

- [ ] **Step 2: Verify the migration applies (integration)**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestAuthStore_NewAuth_RunsMigrations -v`
Expected: PASS — the existing migration test brings the schema up and now also runs 002 (no assertions on the new tables yet; Task 6 adds them). If Docker is unavailable, skip and rely on Task 6's tests in CI.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/auth_migrations/002_web_auth.sql
git commit -s -m "feat(storage): add web_sessions and oidc_login_flows migration"
```

---

### Task 6: Postgres WebAuthStore implementation

**Files:**

- Create: `internal/storage/postgres/web_auth.go`
- Modify: `internal/storage/postgres/auth.go` (add compile-time assertion)

- [ ] **Step 1: Add the compile-time assertion**

In `internal/storage/postgres/auth.go`, just below the existing `var _ storage.UsersBackend = (*AuthStore)(nil)` (line ~27), add:

```go
// Compile-time assertion that *AuthStore implements WebAuthStore.
var _ storage.WebAuthStore = (*AuthStore)(nil)
```

- [ ] **Step 2: Implement the methods**

Create `internal/storage/postgres/web_auth.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// CreateSession inserts a web session row, returning the generated id and
// created_at.
func (s *AuthStore) CreateSession(ctx context.Context, sess *storage.Session) (*storage.Session, error) {
	if len(sess.TokenHash) == 0 {
		return nil, errors.New("CreateSession: TokenHash required")
	}
	if sess.UserID == "" {
		return nil, errors.New("CreateSession: UserID required")
	}
	if sess.ExpiresAt.IsZero() {
		return nil, errors.New("CreateSession: ExpiresAt required")
	}
	const q = `
		INSERT INTO web_sessions (token_hash, user_id, issuer, oidc_subject, expires_at)
		SELECT $1, $2::uuid, $3, $4, $5
		WHERE EXISTS (SELECT 1 FROM users WHERE id = $2::uuid AND deleted_at IS NULL)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, sess.TokenHash, sess.UserID, sess.Issuer, sess.OIDCSubject, sess.ExpiresAt).
		Scan(&id, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("create session: %w", storage.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	sess.ID = id
	sess.CreatedAt = createdAt
	return sess, nil
}

// LookupSessionByHash returns the session for the given token hash.
func (s *AuthStore) LookupSessionByHash(ctx context.Context, tokenHash []byte) (*storage.Session, error) {
	const q = `
		SELECT id, token_hash, user_id, issuer, oidc_subject, created_at, expires_at, revoked_at
		FROM web_sessions
		WHERE token_hash = $1`
	row := s.pool.QueryRow(ctx, q, tokenHash)
	var sess storage.Session
	err := row.Scan(&sess.ID, &sess.TokenHash, &sess.UserID, &sess.Issuer,
		&sess.OIDCSubject, &sess.CreatedAt, &sess.ExpiresAt, &sess.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return &sess, nil
}

// RevokeSession marks the session revoked by token hash. Idempotent.
func (s *AuthStore) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE web_sessions SET revoked_at = $1
		WHERE token_hash = $2 AND revoked_at IS NULL`, s.now(), tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired rows. Revoked rows are left to expire
// naturally so a revoked id never silently reappears as "not found vs revoked".
func (s *AuthStore) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM web_sessions WHERE expires_at <= $1`, s.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CreateLoginFlow inserts a flow row and returns its opaque id.
func (s *AuthStore) CreateLoginFlow(ctx context.Context, f *storage.LoginFlow) (string, error) {
	if f.State == "" || f.Nonce == "" || f.CodeVerifier == "" || f.ProviderID == "" {
		return "", errors.New("CreateLoginFlow: state, nonce, code_verifier, provider_id required")
	}
	if f.ExpiresAt.IsZero() {
		return "", errors.New("CreateLoginFlow: ExpiresAt required")
	}
	const q = `
		INSERT INTO oidc_login_flows (state, nonce, code_verifier, provider_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	var id string
	if err := s.pool.QueryRow(ctx, q, f.State, f.Nonce, f.CodeVerifier, f.ProviderID, f.ExpiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("create login flow: %w", err)
	}
	return id, nil
}

// ConsumeLoginFlow atomically deletes and returns the (unexpired) flow.
func (s *AuthStore) ConsumeLoginFlow(ctx context.Context, flowID string) (*storage.LoginFlow, error) {
	// DELETE ... RETURNING is atomic single-use; the expiry guard rejects
	// stale rows even before the sweeper runs. An invalid UUID text errors at
	// the cast — treat as not-found.
	const q = `
		DELETE FROM oidc_login_flows
		WHERE id = $1::uuid AND expires_at > $2
		RETURNING id, state, nonce, code_verifier, provider_id, created_at, expires_at`
	row := s.pool.QueryRow(ctx, q, flowID, s.now())
	var f storage.LoginFlow
	err := row.Scan(&f.ID, &f.State, &f.Nonce, &f.CodeVerifier, &f.ProviderID, &f.CreatedAt, &f.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrLoginFlowNotFound
	}
	if err != nil {
		// A malformed flowID (not a uuid) yields a cast error; map to not-found
		// so the handler returns auth_error=expired rather than 500.
		return nil, storage.ErrLoginFlowNotFound
	}
	return &f, nil
}

// DeleteExpiredLoginFlows removes expired flow rows.
func (s *AuthStore) DeleteExpiredLoginFlows(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM oidc_login_flows WHERE expires_at <= $1`, s.now())
	if err != nil {
		return 0, fmt.Errorf("delete expired login flows: %w", err)
	}
	return tag.RowsAffected(), nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/storage/postgres/`
Expected: success — the `var _ storage.WebAuthStore` assertion confirms the method set.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/web_auth.go internal/storage/postgres/auth.go
git commit -s -m "feat(storage/postgres): implement WebAuthStore on AuthStore"
```

---

### Task 7: WebAuthStore integration tests

**Files:**

- Create: `internal/storage/postgres/web_auth_test.go`

- [ ] **Step 1: Write the tests**

Create `internal/storage/postgres/web_auth_test.go`. Uses the existing helpers `authTestSetup`, `sharedTestPool`, `truncateAuthTables` from `auth_helpers_test.go`. We seed a user via direct SQL.

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

func seedUser(t *testing.T, ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgxRow
}) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&id))
	return id
}

func TestWebAuth_SessionLifecycle(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&userID))

	hash := []byte("0123456789abcdef0123456789abcdef") // 32 bytes stand-in
	sess, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: hash, UserID: userID, Issuer: "iss", OIDCSubject: "sub",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NotEmpty(t, sess.ID)

	got, err := auth.LookupSessionByHash(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, userID, got.UserID)
	require.True(t, got.IsActive(time.Now()))

	require.NoError(t, auth.RevokeSession(ctx, hash))
	got, err = auth.LookupSessionByHash(ctx, hash)
	require.NoError(t, err)
	require.False(t, got.IsActive(time.Now()))

	_, err = auth.LookupSessionByHash(ctx, []byte("nope"))
	require.ErrorIs(t, err, storage.ErrSessionNotFound)
}

func TestWebAuth_CreateSessionUnknownUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	_, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: []byte("x"), UserID: "00000000-0000-0000-0000-000000000000",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}

func TestWebAuth_DeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&userID))

	_, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: []byte("expired-hash"), UserID: userID,
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	n, err := auth.DeleteExpiredSessions(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1))
}

func TestWebAuth_LoginFlowConsumeSingleUse(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "st", Nonce: "no", CodeVerifier: "cv", ProviderID: "entra",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	f, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "st", f.State)
	require.Equal(t, "entra", f.ProviderID)

	// Replay → gone.
	_, err = auth.ConsumeLoginFlow(ctx, id)
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)

	// Bad uuid → not-found, not error.
	_, err = auth.ConsumeLoginFlow(ctx, "not-a-uuid")
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)
}

func TestWebAuth_LoginFlowExpired(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "st", Nonce: "no", CodeVerifier: "cv", ProviderID: "entra",
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	_, err = auth.ConsumeLoginFlow(ctx, id)
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)
}
```

Note: delete the unused `seedUser`/`pgxRow` helper stub above — it was illustrative; each test inlines the seed insert. (If you keep a shared helper, give it the real `*pgxpool.Pool` type and a concrete return.)

- [ ] **Step 2: Run the tests**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestWebAuth -v`
Expected: PASS (requires Docker).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/web_auth_test.go
git commit -s -m "test(storage/postgres): integration tests for WebAuthStore"
```

---

### Task 8: Context marker for interactive login

**Files:**

- Modify: `internal/auth/context.go`
- Test: `internal/auth/context_test.go` (create/append)

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/context_test.go` (create if absent, `package auth`):

```go
package auth

import (
	"context"
	"testing"
)

func TestInteractiveLoginContext(t *testing.T) {
	ctx := context.Background()
	if InteractiveLoginFromContext(ctx) {
		t.Fatal("plain context must not be marked interactive")
	}
	ctx = WithInteractiveLogin(ctx)
	if !InteractiveLoginFromContext(ctx) {
		t.Fatal("marked context must report interactive")
	}
}
```

- [ ] **Step 2: Run it to confirm failure**

Run: `go test ./internal/auth/ -run TestInteractiveLoginContext -v`
Expected: FAIL — undefined `WithInteractiveLogin` / `InteractiveLoginFromContext`.

- [ ] **Step 3: Implement the helpers**

Append to `internal/auth/context.go`:

```go
type interactiveLoginKey struct{}

// WithInteractiveLogin marks the context as originating from the interactive
// OIDC login callback. Consumed by jitResolve to bypass the per-issuer JIT
// rate limiter (the limiter targets unsolicited bearer-JWT JIT; an interactive
// login is user-driven and IdP+PKCE+nonce authenticated). Set ONLY by the
// callback handler — bearer entry points never set it.
func WithInteractiveLogin(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactiveLoginKey{}, true)
}

// InteractiveLoginFromContext reports whether the context was marked by
// WithInteractiveLogin.
func InteractiveLoginFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(interactiveLoginKey{}).(bool)
	return v
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/auth/ -run TestInteractiveLoginContext -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/context.go internal/auth/context_test.go
git commit -s -m "feat(auth): add WithInteractiveLogin context marker"
```

---

### Task 9: Resolver — session resolution path, dispatch, JIT limiter bypass

**Files:**

- Modify: `internal/auth/identitystore.go`
- Test: `internal/auth/identitystore_test.go` (append; or wherever resolver tests live)

This task adds: (a) a `WebAuth` field on `IdentityStoreConfig` + `pgIdentityStore`; (b) `resolveSession`; (c) a `spgr_ws_` branch in `Resolve`; (d) a limiter bypass in `jitResolve` keyed off the context marker.

- [ ] **Step 1: Write failing tests**

Append to the resolver test file. We need a `WebAuthStore` fake; define a minimal one inline. We also reuse whatever `UsersBackend` fake already exists in the package's tests (the explore report noted `auth/usersbackend_stub_test.go`). The test below assumes a constructor pattern matching existing resolver tests — adapt the `UsersBackend` stub variable name to the existing one (`stubUsers`/`fakeUsers`).

```go
// fakeWebAuth is a minimal in-memory WebAuthStore for resolver tests.
type fakeWebAuth struct {
	sessions map[string]*storage.Session // keyed by hex(token_hash)
	err      error                       // forced lookup error (transient test)
}

func (f *fakeWebAuth) CreateSession(_ context.Context, s *storage.Session) (*storage.Session, error) {
	if f.sessions == nil {
		f.sessions = map[string]*storage.Session{}
	}
	s.ID = "sess-" + s.UserID
	f.sessions[string(s.TokenHash)] = s
	return s, nil
}
func (f *fakeWebAuth) LookupSessionByHash(_ context.Context, h []byte) (*storage.Session, error) {
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.sessions[string(h)]
	if !ok {
		return nil, storage.ErrSessionNotFound
	}
	return s, nil
}
func (f *fakeWebAuth) RevokeSession(_ context.Context, h []byte) error { delete(f.sessions, string(h)); return nil }
func (f *fakeWebAuth) DeleteExpiredSessions(context.Context) (int64, error) { return 0, nil }
func (f *fakeWebAuth) CreateLoginFlow(context.Context, *storage.LoginFlow) (string, error) { return "id", nil }
func (f *fakeWebAuth) ConsumeLoginFlow(context.Context, string) (*storage.LoginFlow, error) { return nil, storage.ErrLoginFlowNotFound }
func (f *fakeWebAuth) DeleteExpiredLoginFlows(context.Context) (int64, error) { return 0, nil }
```

```go
func TestResolveSession(t *testing.T) {
	// Build a resolver wired with a fake UsersBackend that returns an active
	// user for ID "u1", plus a fakeWebAuth. Use the same NewIdentityStore
	// construction the other resolver tests in this file use; set Now to a
	// fixed clock if the existing tests do.
	wa := &fakeWebAuth{}
	users := newStubUsers() // existing helper; must return an active user for GetUserByID("u1")
	users.addActiveUser("u1", "reader")

	r, err := NewIdentityStore(IdentityStoreConfig{
		Users:   users,
		Tracker: noopTracker{}, // existing test tracker
		WebAuth: wa,
	})
	require.NoError(t, err)

	// Seed an active session whose token is "spgr_ws_TOKEN".
	token := "spgr_ws_TOKEN"
	sum := sha256.Sum256([]byte(token))
	wa.sessions = map[string]*storage.Session{
		string(sum[:]): {TokenHash: sum[:], UserID: "u1", OIDCSubject: "sub-1", ExpiresAt: time.Now().Add(time.Hour)},
	}

	id, err := r.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "oidc:sub-1", id.Subject)
	require.Equal(t, "oidc", id.Source)
}

func TestResolveSession_Errors(t *testing.T) {
	users := newStubUsers()
	users.addActiveUser("u1", "reader")

	// Missing → ErrUnauthenticated.
	r, _ := NewIdentityStore(IdentityStoreConfig{Users: users, Tracker: noopTracker{}, WebAuth: &fakeWebAuth{}})
	_, err := r.Resolve(context.Background(), "spgr_ws_unknown")
	require.ErrorIs(t, err, ErrUnauthenticated)

	// DB error → ErrTransient.
	r2, _ := NewIdentityStore(IdentityStoreConfig{Users: users, Tracker: noopTracker{}, WebAuth: &fakeWebAuth{err: errors.New("db down")}})
	_, err = r2.Resolve(context.Background(), "spgr_ws_anything")
	require.ErrorIs(t, err, ErrTransient)

	// Nil WebAuth → ErrUnauthenticated (no panic).
	r3, _ := NewIdentityStore(IdentityStoreConfig{Users: users, Tracker: noopTracker{}})
	_, err = r3.Resolve(context.Background(), "spgr_ws_anything")
	require.ErrorIs(t, err, ErrUnauthenticated)
}
```

Ensure the test file imports `crypto/sha256`, `errors`, `time`, `context`, and `storage`.

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/auth/ -run 'TestResolveSession' -v`
Expected: FAIL — `WebAuth` field undefined; `resolveSession` not wired.

- [ ] **Step 3: Add the WebAuth field**

In `internal/auth/identitystore.go`, add to `IdentityStoreConfig` (after `Users`):

```go
	// WebAuth persists/looks up interactive-login web sessions. Optional: when
	// nil, spgr_ws_-prefixed tokens resolve to ErrUnauthenticated.
	WebAuth storage.WebAuthStore
```

Add to the `pgIdentityStore` struct (after `users`):

```go
	webAuth storage.WebAuthStore
```

In `NewIdentityStore`, set it in the returned struct literal (after `users: cfg.Users,`):

```go
		webAuth:            cfg.WebAuth,
```

- [ ] **Step 4: Add the dispatch branch + sessionTokenPrefix const + resolveSession**

In `Resolve` (lines ~150-158), insert the session branch BEFORE the api-key fallback:

```go
func (s *pgIdentityStore) Resolve(ctx context.Context, token string) (*Identity, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	if isJWTShaped(token) {
		return s.resolveJWT(ctx, token)
	}
	if strings.HasPrefix(token, sessionTokenPrefix) {
		return s.resolveSession(ctx, token)
	}
	return s.resolveAPIKey(ctx, token)
}
```

Add a const near `apiKeyPrefix` (lines ~175-179):

```go
const sessionTokenPrefix = "spgr_ws_" //nolint:gosec // G101: token-format prefix, not a credential
```

Add `"crypto/sha256"` to the imports. Add the method (place after `resolveAPIKey`):

```go
// resolveSession resolves an opaque web-session token (spgr_ws_...). Mirrors
// resolveAPIKey's error discipline: not-found/revoked/expired/soft-deleted →
// ErrUnauthenticated; any other backend error → ErrTransient.
func (s *pgIdentityStore) resolveSession(ctx context.Context, token string) (*Identity, error) {
	if s.webAuth == nil {
		return nil, ErrUnauthenticated
	}
	sum := sha256.Sum256([]byte(token))
	sess, err := s.webAuth.LookupSessionByHash(ctx, sum[:])
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if !sess.IsActive(s.now()) {
		return nil, ErrUnauthenticated
	}
	user, err := s.users.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: session resolved to soft-deleted user",
			slog.String("user_id", user.ID), slog.String("subject", sess.OIDCSubject))
		return nil, ErrUnauthenticated
	}
	return &Identity{
		UserID:        user.ID,
		Subject:       "oidc:" + sess.OIDCSubject,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: Role(user.Role),
		Source:        "oidc",
	}, nil
}
```

- [ ] **Step 5: Add the JIT limiter bypass**

In `jitResolve` (lines ~423-449), wrap the rate-limit gate so the interactive context skips it. Replace the limiter block:

```go
	// (2) Per-issuer rate-limit gate — bounds eligible creation attempts.
	// Skipped for interactive logins (user-driven, IdP+PKCE+nonce verified);
	// the limiter targets unsolicited bearer-JWT JIT. The email-domain
	// allowlist (step 1, above) still applies.
	if !InteractiveLoginFromContext(ctx) {
		limiter := s.rateLimiterFor(claims.Issuer)
		if !limiter.Allow() {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: JIT rate-limit exceeded",
				slog.String("issuer", claims.Issuer), slog.String("subject", claims.Subject))
			return nil, ErrUnauthenticated
		}
	}
```

- [ ] **Step 6: Run resolver tests**

Run: `go test ./internal/auth/ -run 'TestResolveSession' -v`
Expected: PASS

Add a bypass test:

```go
func TestJITResolve_InteractiveBypassesLimiter(t *testing.T) {
	// Construct a resolver with JITRateBurstPerHour=1 and JIT enabled, a
	// claims mapping/allowlist permissive, and a fake UsersBackend whose
	// LookupOIDCBinding misses and JITCreateHuman succeeds. Call the internal
	// jitResolve twice with WithInteractiveLogin(ctx): both succeed (no
	// limiter decrement). Then call once on a plain context after exhausting
	// the bucket to assert the bearer path still limits.
	// (Adapt to existing jitResolve test helpers in this file.)
}
```

Implement it against the existing jit test scaffolding in the file; run:
Run: `go test ./internal/auth/ -run TestJITResolve_InteractiveBypassesLimiter -v`
Expected: PASS

- [ ] **Step 7: Run the whole auth package**

Run: `go test ./internal/auth/ -v`
Expected: PASS (no regressions in existing resolver tests).

- [ ] **Step 8: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): add resolveSession path and interactive JIT limiter bypass"
```

---

### Task 10: Login-provider abstraction, oidc impl, and BuildLoginProviders

**Files:**

- Create: `internal/auth/loginprovider.go`
- Test: `internal/auth/loginprovider_test.go`

This builds the `oauth2.Config`-backed provider plus the startup builder that validates config and resolves the secret from env. `golang.org/x/oauth2` is already an indirect dep (`go.mod`); it becomes direct here.

- [ ] **Step 1: Write failing tests (validation + secret resolution + AuthCodeURL)**

Create `internal/auth/loginprovider_test.go`:

```go
package auth

import (
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
)

func TestBuildLoginProviders_SkipsNonInteractive(t *testing.T) {
	provs, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "verify-only", Issuer: "https://x", ClientID: "c"}, // interactive=false
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(provs) != 0 {
		t.Fatalf("non-interactive provider must not yield a login provider, got %d", len(provs))
	}
}

func TestBuildLoginProviders_MissingSecret(t *testing.T) {
	_, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "entra", Interactive: true, Issuer: "https://x", ClientID: "c"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("expected missing-secret error, got %v", err)
	}
}

func TestBuildLoginProviders_AudienceMismatch(t *testing.T) {
	t.Setenv("TEST_SECRET", "shh")
	_, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "entra", Interactive: true, Issuer: "https://x", ClientID: "c",
			Audience: "different", ClientSecretEnv: "TEST_SECRET"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "audience") {
		t.Fatalf("expected audience error, got %v", err)
	}
}

func TestResolveClientSecret(t *testing.T) {
	t.Setenv("MY_SECRET", "value")
	got, err := resolveClientSecret(config.OIDCProviderConfig{ClientSecretEnv: "MY_SECRET"})
	if err != nil || got != "value" {
		t.Fatalf("env resolution: got %q err %v", got, err)
	}
	got, err = resolveClientSecret(config.OIDCProviderConfig{ClientSecret: "plain"})
	if err != nil || got != "plain" {
		t.Fatalf("plaintext fallback: got %q err %v", got, err)
	}
	if _, err := resolveClientSecret(config.OIDCProviderConfig{ClientSecretEnv: "UNSET_VAR_XYZ"}); err == nil {
		t.Fatal("unset env var must error")
	}
}

func TestOIDCLoginProvider_AuthCodeURL(t *testing.T) {
	p := &oidcLoginProvider{
		id:          "entra",
		displayName: "Entra",
		authURL:     "https://idp/authorize",
		clientID:    "client-1",
		scopes:      []string{"openid", "email"},
	}
	got := p.AuthCodeURL("STATE", "NONCE", "CHALLENGE", "https://app/api/auth/oidc/callback")
	for _, want := range []string{
		"https://idp/authorize?", "client_id=client-1", "state=STATE",
		"nonce=NONCE", "code_challenge=CHALLENGE", "code_challenge_method=S256",
		"scope=openid+email", "response_type=code",
		"redirect_uri=https%3A%2F%2Fapp%2Fapi%2Fauth%2Foidc%2Fcallback",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AuthCodeURL missing %q in %q", want, got)
		}
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/auth/ -run 'TestBuildLoginProviders|TestResolveClientSecret|TestOIDCLoginProvider' -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement loginprovider.go**

Create `internal/auth/loginprovider.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/config"
)

// LoginProvider drives the interactive OAuth2 Authorization Code flow for one
// provider. The oidc implementation verifies the returned id_token (incl.
// nonce) and returns the raw token for the resolver to materialize identity.
type LoginProvider interface {
	ID() string
	DisplayName() string
	AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string
	// Exchange swaps the authorization code for a nonce-verified raw id_token.
	Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (idToken string, err error)
}

type oidcLoginProvider struct {
	id          string
	displayName string
	authURL     string
	tokenURL    string
	clientID    string
	secret      string
	scopes      []string
	verifier    *OIDCVerifier
}

func (p *oidcLoginProvider) ID() string          { return p.id }
func (p *oidcLoginProvider) DisplayName() string { return p.displayName }

func (p *oidcLoginProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string {
	cfg := p.oauth2Config(redirectURI)
	return cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *oidcLoginProvider) Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (string, error) {
	cfg := p.oauth2Config(redirectURI)
	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return "", fmt.Errorf("oidc exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return "", fmt.Errorf("oidc exchange: no id_token in response")
	}
	if _, err := p.verifier.VerifyWithNonce(ctx, rawID, nonce); err != nil {
		return "", err
	}
	return rawID, nil
}

func (p *oidcLoginProvider) oauth2Config(redirectURI string) oauth2.Config {
	return oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.secret,
		Endpoint:     oauth2.Endpoint{AuthURL: p.authURL, TokenURL: p.tokenURL},
		RedirectURL:  redirectURI,
		Scopes:       p.scopes,
	}
}

// resolveClientSecret returns the provider's client secret from
// ClientSecretEnv (preferred) or the plaintext ClientSecret fallback.
func resolveClientSecret(pc config.OIDCProviderConfig) (string, error) {
	if pc.ClientSecretEnv != "" {
		v := os.Getenv(pc.ClientSecretEnv)
		if v == "" {
			return "", fmt.Errorf("client secret env var %q is unset", pc.ClientSecretEnv)
		}
		return v, nil
	}
	if pc.ClientSecret != "" {
		return pc.ClientSecret, nil
	}
	return "", fmt.Errorf("no client secret (set client_secret_env or client_secret)")
}

// defaultScopes is applied when a provider configures none.
var defaultScopes = []string{"openid", "email", "profile"}

// BuildLoginProviders discovers and constructs a LoginProvider for each
// interactive OIDC provider. Non-interactive providers are skipped. All
// failures (unknown kind, missing secret, bad audience, discovery failure)
// are fatal — the caller treats a non-nil error as a startup abort.
//
// discover is the discovery function (oidc.NewProvider); pass nil to use the
// default. The extra param exists for tests that need to avoid a network call.
func BuildLoginProviders(ctx context.Context, providers []config.OIDCProviderConfig, _ string) ([]LoginProvider, error) {
	var out []LoginProvider
	for _, pc := range providers {
		if !pc.Interactive {
			continue
		}
		kind := pc.Kind
		if kind == "" {
			kind = "oidc"
		}
		if kind != "oidc" {
			return nil, fmt.Errorf("OIDC provider %s: unsupported kind %q (only \"oidc\")", pc.ID, kind)
		}
		if pc.Audience != "" && pc.Audience != pc.ClientID {
			return nil, fmt.Errorf("OIDC provider %s: interactive audience must be empty or equal client_id", pc.ID)
		}
		secret, err := resolveClientSecret(pc)
		if err != nil {
			return nil, fmt.Errorf("OIDC provider %s: %w", pc.ID, err)
		}
		// Discover endpoints + build a nonce-capable verifier.
		dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		provider, err := oidc.NewProvider(dctx, pc.Issuer)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("OIDC provider %s discovery: %w", pc.ID, err)
		}
		verifier, err := NewOIDCVerifier(ctx, pc)
		if err != nil {
			return nil, fmt.Errorf("OIDC provider %s verifier: %w", pc.ID, err)
		}
		scopes := pc.Scopes
		if len(scopes) == 0 {
			scopes = defaultScopes
		}
		scopes = ensureOpenID(scopes)
		display := pc.DisplayName
		if display == "" {
			display = pc.ID
		}
		ep := provider.Endpoint()
		out = append(out, &oidcLoginProvider{
			id:          pc.ID,
			displayName: display,
			authURL:     ep.AuthURL,
			tokenURL:    ep.TokenURL,
			clientID:    pc.ClientID,
			secret:      secret,
			scopes:      scopes,
			verifier:    verifier,
		})
	}
	return out, nil
}

// ensureOpenID guarantees the openid scope is present.
func ensureOpenID(scopes []string) []string {
	for _, s := range scopes {
		if s == "openid" {
			return scopes
		}
	}
	return append([]string{"openid"}, scopes...)
}

// RedirectURI builds the OIDC callback URL from the request, honoring an
// explicit base override (auth.oidc.base_url) when set.
func RedirectURI(baseURL string, tls bool, host, fwdProto, fwdHost string) string {
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/") + "/api/auth/oidc/callback"
	}
	scheme := "http"
	if tls || fwdProto == "https" {
		scheme = "https"
	}
	h := host
	if fwdHost != "" {
		h = fwdHost
	}
	u := url.URL{Scheme: scheme, Host: h, Path: "/api/auth/oidc/callback"}
	return u.String()
}
```

Note: the `TestBuildLoginProviders_*` tests that reach discovery (`MissingSecret`, `AudienceMismatch`) intentionally fail BEFORE `oidc.NewProvider` (secret/audience checks precede discovery), so they need no network. `SkipsNonInteractive` returns before any check. Keep these validation checks ordered before discovery exactly as written.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/auth/ -run 'TestBuildLoginProviders|TestResolveClientSecret|TestOIDCLoginProvider' -v`
Expected: PASS

- [ ] **Step 5: Add a RedirectURI unit test**

```go
func TestRedirectURI(t *testing.T) {
	// base_url override wins
	if got := RedirectURI("https://app.example.com", false, "ignored", "", ""); got != "https://app.example.com/api/auth/oidc/callback" {
		t.Fatalf("override: %q", got)
	}
	// derived from request, https via X-Forwarded-Proto + X-Forwarded-Host
	if got := RedirectURI("", false, "internal:9090", "https", "app.example.com"); got != "https://app.example.com/api/auth/oidc/callback" {
		t.Fatalf("derived: %q", got)
	}
	// plain http from Host
	if got := RedirectURI("", false, "localhost:9090", "", ""); got != "http://localhost:9090/api/auth/oidc/callback" {
		t.Fatalf("http: %q", got)
	}
}
```

Run: `go test ./internal/auth/ -run TestRedirectURI -v`
Expected: PASS

- [ ] **Step 6: Tidy modules and commit**

```bash
go mod tidy
git add internal/auth/loginprovider.go internal/auth/loginprovider_test.go go.mod go.sum
git commit -s -m "feat(auth): add LoginProvider, oidc impl, BuildLoginProviders, RedirectURI"
```

---

### Task 11: Per-IP rate limiter middleware

**Files:**

- Create: `internal/server/ratelimit.go`
- Test: `internal/server/ratelimit_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/ratelimit_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerIPRateLimiter(t *testing.T) {
	rl := newIPRateLimiter(1, 1, false) // 1/sec, burst 1
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatal("429 must set Retry-After")
	}

	// Different IP gets its own bucket.
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.RemoteAddr = "10.0.0.2:1234"
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req2)
	if rec3.Code != http.StatusOK {
		t.Fatalf("new IP should pass, got %d", rec3.Code)
	}
}

func TestClientIP_TrustedProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.9:5555"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.9")

	if ip := clientIP(req, false); ip != "10.0.0.9" {
		t.Fatalf("untrusted: want RemoteAddr host, got %q", ip)
	}
	if ip := clientIP(req, true); ip != "203.0.113.7" {
		t.Fatalf("trusted: want leftmost XFF, got %q", ip)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

Run: `go test ./internal/server/ -run 'TestPerIPRateLimiter|TestClientIP' -v`
Expected: FAIL — undefined `newIPRateLimiter`, `clientIP`.

- [ ] **Step 3: Implement the limiter**

Create `internal/server/ratelimit.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// ipRateLimiter is a per-client-IP token-bucket limiter. Buckets are created
// lazily and kept in memory for the process lifetime (bounded by distinct IPs;
// acceptable for the public OIDC start/callback endpoints). Used to bound the
// unauthenticated DB writes performed by /api/auth/oidc/start.
type ipRateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*rate.Limiter
	r            rate.Limit
	burst        int
	trustedProxy bool
}

// newIPRateLimiter returns a limiter allowing r events/sec with the given
// burst, per client IP. trustedProxy selects X-Forwarded-For extraction.
func newIPRateLimiter(perSec float64, burst int, trustedProxy bool) *ipRateLimiter {
	return &ipRateLimiter{
		buckets:      map[string]*rate.Limiter{},
		r:            rate.Limit(perSec),
		burst:        burst,
		trustedProxy: trustedProxy,
	}
}

func (l *ipRateLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.buckets[ip]
	if !ok {
		lim = rate.NewLimiter(l.r, l.burst)
		l.buckets[ip] = lim
	}
	return lim
}

// wrap returns middleware that rejects over-limit requests with 429.
func (l *ipRateLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, l.trustedProxy)
		if !l.limiterFor(ip).Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the client IP. When trustedProxy is true it uses the
// leftmost X-Forwarded-For entry (then X-Real-Ip); otherwise the TCP peer.
func clientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, ok := strings.Cut(xff, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(xff)
		}
		if xr := r.Header.Get("X-Real-Ip"); xr != "" {
			return strings.TrimSpace(xr)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/server/ -run 'TestPerIPRateLimiter|TestClientIP' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/server/ratelimit.go internal/server/ratelimit_test.go
git commit -s -m "feat(server): add per-IP rate limiter middleware"
```

---

### Task 12: OIDC handlers (providers / start / callback)

**Files:**

- Create: `internal/server/auth_oidc_handler.go`
- Test: `internal/server/auth_oidc_handler_test.go`

The handler reuses `sessionCookie` and `writeJSONError` from `auth_handler.go` (same package). It needs: the `[]auth.LoginProvider`, the `auth.Resolver`, the `storage.WebAuthStore`, the config `base_url`, the `session_ttl`, and a rate limiter.

- [ ] **Step 1: Implement the handler**

Create `internal/server/auth_oidc_handler.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

const txCookieName = "specgraph_oidc_tx"

// OIDCLoginConfig parametrizes the interactive login handlers.
type OIDCLoginConfig struct {
	Providers  []auth.LoginProvider
	Resolver   auth.Resolver
	WebAuth    storage.WebAuthStore
	BaseURL    string
	SessionTTL time.Duration
	FlowTTL    time.Duration // default 5m
	Limiter    *ipRateLimiter
}

type oidcLoginHandler struct {
	byID       map[string]auth.LoginProvider
	resolver   auth.Resolver
	webAuth    storage.WebAuthStore
	baseURL    string
	sessionTTL time.Duration
	flowTTL    time.Duration
}

// RegisterOIDCLoginHandlers wires /api/auth/oidc/{providers,start,callback}.
// Endpoints are public (no RequireAuth); start/callback are per-IP rate
// limited. No-op when no interactive providers are configured.
func RegisterOIDCLoginHandlers(mux *http.ServeMux, cfg OIDCLoginConfig) {
	if len(cfg.Providers) == 0 {
		return
	}
	flowTTL := cfg.FlowTTL
	if flowTTL <= 0 {
		flowTTL = 5 * time.Minute
	}
	h := &oidcLoginHandler{
		byID:       map[string]auth.LoginProvider{},
		resolver:   cfg.Resolver,
		webAuth:    cfg.WebAuth,
		baseURL:    cfg.BaseURL,
		sessionTTL: cfg.SessionTTL,
		flowTTL:    flowTTL,
	}
	for _, p := range cfg.Providers {
		h.byID[p.ID()] = p
	}

	limit := cfg.Limiter.wrap
	mux.HandleFunc("/api/auth/oidc/providers", h.handleProviders)
	mux.Handle("/api/auth/oidc/callback", limit(http.HandlerFunc(h.handleCallback)))
	// {provider}/start — Go 1.22+ wildcard pattern.
	mux.Handle("/api/auth/oidc/{provider}/start", limit(http.HandlerFunc(h.handleStart)))
}

type providerInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (h *oidcLoginHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	infos := make([]providerInfo, 0, len(h.byID))
	for _, p := range h.byID {
		infos = append(infos, providerInfo{ID: p.ID(), DisplayName: p.DisplayName()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"providers": infos}) //nolint:errcheck // best-effort write
}

func (h *oidcLoginHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := h.byID[r.PathValue("provider")]
	if !ok {
		writeJSONError(w, http.StatusNotFound, "unknown provider")
		return
	}
	state, err := randToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	nonce, err := randToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	flowID, err := h.webAuth.CreateLoginFlow(r.Context(), &storage.LoginFlow{
		State: state, Nonce: nonce, CodeVerifier: verifier, ProviderID: p.ID(),
		ExpiresAt: time.Now().Add(h.flowTTL),
	})
	if err != nil {
		slog.LogAttrs(r.Context(), slog.LevelError, "oidc: create login flow", slog.Any("error", err))
		writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		return
	}
	http.SetCookie(w, h.txCookie(flowID, r))

	redirectURI := auth.RedirectURI(h.baseURL, r.TLS != nil, r.Host,
		r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
	http.Redirect(w, r, p.AuthCodeURL(state, nonce, challenge, redirectURI), http.StatusFound)
}

func (h *oidcLoginHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Always delete the tx cookie on the response.
	http.SetCookie(w, h.deleteTxCookie(r))

	fail := func(reason string) { http.Redirect(w, r, "/?auth_error="+reason, http.StatusFound) }

	c, err := r.Cookie(txCookieName)
	if err != nil || c.Value == "" {
		fail("expired")
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		fail("denied")
		return
	}
	flow, err := h.webAuth.ConsumeLoginFlow(r.Context(), c.Value)
	if err != nil {
		fail("expired")
		return
	}
	if r.URL.Query().Get("state") != flow.State {
		fail("state")
		return
	}
	p, ok := h.byID[flow.ProviderID]
	if !ok {
		slog.LogAttrs(r.Context(), slog.LevelWarn, "oidc: provider removed mid-flow", slog.String("provider", flow.ProviderID))
		fail("exchange")
		return
	}
	redirectURI := auth.RedirectURI(h.baseURL, r.TLS != nil, r.Host,
		r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
	idToken, err := p.Exchange(r.Context(), r.URL.Query().Get("code"), flow.CodeVerifier, flow.Nonce, redirectURI)
	if err != nil {
		slog.LogAttrs(r.Context(), slog.LevelWarn, "oidc: exchange failed", slog.String("provider", p.ID()), slog.Any("error", err))
		fail("exchange")
		return
	}
	// Resolve identity (triggers binding lookup / JIT), limiter bypassed.
	id, err := h.resolver.Resolve(auth.WithInteractiveLogin(r.Context()), idToken)
	if err != nil {
		if errors.Is(err, auth.ErrTransient) {
			fail("temporary")
			return
		}
		fail("unauthorized")
		return
	}
	// Mint the server session.
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
}

func (h *oidcLoginHandler) txCookie(value string, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     txCookieName,
		Value:    value,
		Path:     "/api/auth/oidc",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		MaxAge:   300,
	}
}

func (h *oidcLoginHandler) deleteTxCookie(r *http.Request) *http.Cookie {
	c := h.txCookie("", r)
	c.MaxAge = -1
	return c
}

// subjectOnly strips the "oidc:" prefix the resolver adds to Identity.Subject.
func subjectOnly(subject string) string {
	const p = "oidc:"
	if len(subject) > len(p) && subject[:len(p)] == p {
		return subject[len(p):]
	}
	return subject
}

func randToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// randSessionToken returns the opaque spgr_ws_ session token.
func randSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "spgr_ws_" + base64.RawURLEncoding.EncodeToString(b), nil
}

var _ = context.Background // keep context import if unused after edits
```

Remove the trailing `var _ = context.Background` line if `context` ends up used (it is used via `r.Context()` returning `context.Context` only implicitly; if the import is unused, drop the import instead). Verify with `go build`.

- [ ] **Step 2: Write the flow tests**

Create `internal/server/auth_oidc_handler_test.go`. Use a fake `LoginProvider`, a fake `WebAuthStore`, and a fake `Resolver`.

```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

type fakeProvider struct{ id string; exchangeErr error; idToken string }

func (f *fakeProvider) ID() string          { return f.id }
func (f *fakeProvider) DisplayName() string { return "Fake" }
func (f *fakeProvider) AuthCodeURL(state, nonce, challenge, redirect string) string {
	return "https://idp/authorize?state=" + state
}
func (f *fakeProvider) Exchange(_ context.Context, _, _, _, _ string) (string, error) {
	return f.idToken, f.exchangeErr
}

type fakeWA struct {
	flows    map[string]*storage.LoginFlow
	sessions map[string]*storage.Session
	createErr error
}

func (f *fakeWA) CreateSession(_ context.Context, s *storage.Session) (*storage.Session, error) {
	if f.createErr != nil { return nil, f.createErr }
	if f.sessions == nil { f.sessions = map[string]*storage.Session{} }
	s.ID = "s1"; f.sessions[string(s.TokenHash)] = s; return s, nil
}
func (f *fakeWA) LookupSessionByHash(context.Context, []byte) (*storage.Session, error) { return nil, storage.ErrSessionNotFound }
func (f *fakeWA) RevokeSession(context.Context, []byte) error { return nil }
func (f *fakeWA) DeleteExpiredSessions(context.Context) (int64, error) { return 0, nil }
func (f *fakeWA) CreateLoginFlow(_ context.Context, fl *storage.LoginFlow) (string, error) {
	if f.flows == nil { f.flows = map[string]*storage.LoginFlow{} }
	fl.ID = "flow-1"; f.flows[fl.ID] = fl; return fl.ID, nil
}
func (f *fakeWA) ConsumeLoginFlow(_ context.Context, id string) (*storage.LoginFlow, error) {
	fl, ok := f.flows[id]; if !ok { return nil, storage.ErrLoginFlowNotFound }
	delete(f.flows, id); return fl, nil
}
func (f *fakeWA) DeleteExpiredLoginFlows(context.Context) (int64, error) { return 0, nil }

type fakeResolver struct{ id *auth.Identity; err error }
func (f *fakeResolver) Resolve(context.Context, string) (*auth.Identity, error) { return f.id, f.err }
func (f *fakeResolver) HasAuth(context.Context) (bool, error) { return true, nil }

func newTestMux(provs []auth.LoginProvider, wa storage.WebAuthStore, res auth.Resolver) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterOIDCLoginHandlers(mux, OIDCLoginConfig{
		Providers: provs, Resolver: res, WebAuth: wa,
		SessionTTL: time.Hour, FlowTTL: time.Minute,
		Limiter: newIPRateLimiter(1000, 1000, false),
	})
	return mux
}

func TestStart_RedirectsAndSetsCookie(t *testing.T) {
	wa := &fakeWA{}
	mux := newTestMux([]auth.LoginProvider{&fakeProvider{id: "entra"}}, wa, &fakeResolver{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/entra/start", nil))
	if rec.Code != http.StatusFound { t.Fatalf("want 302, got %d", rec.Code) }
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "https://idp/authorize") {
		t.Fatalf("bad location %q", loc)
	}
	if len(wa.flows) != 1 { t.Fatalf("want 1 flow, got %d", len(wa.flows)) }
	if !strings.Contains(rec.Header().Get("Set-Cookie"), txCookieName) {
		t.Fatal("missing tx cookie")
	}
}

func TestStart_UnknownProvider404(t *testing.T) {
	mux := newTestMux([]auth.LoginProvider{&fakeProvider{id: "entra"}}, &fakeWA{}, &fakeResolver{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/nope/start", nil))
	if rec.Code != http.StatusNotFound { t.Fatalf("want 404, got %d", rec.Code) }
}

func TestCallback_HappyPath(t *testing.T) {
	wa := &fakeWA{}
	wa.CreateLoginFlow(context.Background(), &storage.LoginFlow{State: "S", Nonce: "N", CodeVerifier: "V", ProviderID: "entra", ExpiresAt: time.Now().Add(time.Minute)})
	res := &fakeResolver{id: &auth.Identity{UserID: "u1", Subject: "oidc:sub-1"}}
	mux := newTestMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("want 302 to /, got %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if len(wa.sessions) != 1 { t.Fatalf("want 1 session, got %d", len(wa.sessions)) }
	sc := rec.Result().Cookies()
	var hasSession bool
	for _, c := range sc {
		if c.Name == "specgraph_session" && strings.HasPrefix(c.Value, "spgr_ws_") { hasSession = true }
	}
	if !hasSession { t.Fatal("missing spgr_ws_ session cookie") }
}

func TestCallback_Failures(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*fakeWA, *fakeResolver, *fakeProvider)
		cookie bool
		query  string
		reason string
	}{
		{"missing-tx", func(*fakeWA, *fakeResolver, *fakeProvider) {}, false, "state=S", "expired"},
		{"unknown-flow", func(*fakeWA, *fakeResolver, *fakeProvider) {}, true, "state=S", "expired"},
		{"idp-error", func(w *fakeWA, _ *fakeResolver, _ *fakeProvider) {
			w.CreateLoginFlow(context.Background(), &storage.LoginFlow{State: "S", Nonce: "N", CodeVerifier: "V", ProviderID: "entra", ExpiresAt: time.Now().Add(time.Minute)})
		}, true, "error=access_denied", "denied"},
		{"state-mismatch", func(w *fakeWA, _ *fakeResolver, _ *fakeProvider) {
			w.CreateLoginFlow(context.Background(), &storage.LoginFlow{State: "S", Nonce: "N", CodeVerifier: "V", ProviderID: "entra", ExpiresAt: time.Now().Add(time.Minute)})
		}, true, "state=WRONG&code=x", "state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wa := &fakeWA{}
			res := &fakeResolver{id: &auth.Identity{UserID: "u1", Subject: "oidc:sub"}}
			prov := &fakeProvider{id: "entra", idToken: "tok"}
			tc.setup(wa, res, prov)
			mux := newTestMux([]auth.LoginProvider{prov}, wa, res)

			req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?"+tc.query, nil)
			if tc.cookie { req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"}) }
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			loc := rec.Header().Get("Location")
			if !strings.Contains(loc, "auth_error="+tc.reason) {
				t.Fatalf("want auth_error=%s, got %q", tc.reason, loc)
			}
		})
	}
}

func TestCallback_Unauthorized(t *testing.T) {
	wa := &fakeWA{}
	wa.CreateLoginFlow(context.Background(), &storage.LoginFlow{State: "S", Nonce: "N", CodeVerifier: "V", ProviderID: "entra", ExpiresAt: time.Now().Add(time.Minute)})
	res := &fakeResolver{err: auth.ErrUnauthenticated}
	mux := newTestMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=x", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Header().Get("Location"), "auth_error=unauthorized") {
		t.Fatalf("want unauthorized, got %q", rec.Header().Get("Location"))
	}
	if len(wa.sessions) != 0 { t.Fatal("no session must be minted on unauthorized") }
}
```

This test depends on `establishSession` and `auth.Resolver`/`auth.Identity` shapes. `establishSession` is added in Task 13 — if you run Task 12 tests before Task 13, the package won't compile. **Do Task 13 Step 3 (add `establishSession`) before running these tests.** Alternatively reorder: implement `establishSession` first. The plan assumes Task 13's `establishSession` exists when these run.

- [ ] **Step 3: Run the flow tests (after Task 13's establishSession exists)**

Run: `go test ./internal/server/ -run 'TestStart_|TestCallback_' -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/server/auth_oidc_handler.go internal/server/auth_oidc_handler_test.go
git commit -s -m "feat(server): add interactive OIDC login handlers"
```

---

### Task 13: auth_handler.go — SameSite=Lax, establishSession, logout revocation, signature

**Files:**

- Modify: `internal/server/auth_handler.go`
- Test: `internal/server/auth_handler_test.go`

- [ ] **Step 1: Switch sessionCookie to SameSite=Lax**

In `internal/server/auth_handler.go`, in `sessionCookie()` (lines ~126-138), change:

```go
		SameSite: http.SameSiteStrictMode,
```

to:

```go
		SameSite: http.SameSiteLaxMode,
```

Update the doc comment above the function to note: `SameSite=Lax so the cookie is sent on the post-IdP redirect top-level navigation; safe because no GET endpoint mutates state.`

- [ ] **Step 2: Add establishSession**

Add to `internal/server/auth_handler.go` (near `sessionCookie`):

```go
// establishSession sets the session cookie to the given token value. It is the
// single seam that seats a web session (OIDC flow uses it). MUST reuse
// sessionCookie()'s exact name and Path so a callback's Set-Cookie
// deterministically overwrites any pre-existing session cookie and logout's
// clear always deletes it.
func establishSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, sessionCookie(token, r))
}
```

- [ ] **Step 3: Change RegisterAuthHandlers signature + revoke on logout**

Replace the `RegisterAuthHandlers` signature and the logout registration. New signature adds a `webAuth storage.WebAuthStore` (may be nil):

```go
// RegisterAuthHandlers registers login, logout, and whoami endpoints.
// authMW is applied to protected routes (whoami). It must not be nil.
// resolver validates credentials at login. webAuth (may be nil) is used to
// revoke the server session on logout.
func RegisterAuthHandlers(mux *http.ServeMux, resolver auth.Resolver, webAuth storage.WebAuthStore, authMW func(http.Handler) http.Handler) {
	if authMW == nil {
		panic("RegisterAuthHandlers: authMW must not be nil")
	}

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogin(w, r, resolver)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogout(w, r, webAuth)
	})

	mux.Handle("/api/auth/whoami", authMW(cookieToAuthHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleWhoami(w, r)
	}))))
}
```

Replace `handleLogout` (lines ~83-89) with a version that revokes a `spgr_ws_` session:

```go
// handleLogout revokes the server session (if the cookie holds a spgr_ws_
// token) and clears the session cookie. A legacy API-key cookie value is
// never hashed/looked-up.
func handleLogout(w http.ResponseWriter, r *http.Request, webAuth storage.WebAuthStore) {
	if c, err := r.Cookie(sessionCookieName); err == nil && webAuth != nil &&
		strings.HasPrefix(c.Value, "spgr_ws_") {
		sum := sha256.Sum256([]byte(c.Value))
		if revErr := webAuth.RevokeSession(r.Context(), sum[:]); revErr != nil {
			slog.LogAttrs(r.Context(), slog.LevelWarn, "logout: revoke session", slog.Any("error", revErr))
		}
	}
	c := sessionCookie("", r) //nolint:gosec // G124: sessionCookie sets HttpOnly/SameSite/dynamic Secure
	c.MaxAge = -1
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}
```

Add imports to `auth_handler.go`: `"crypto/sha256"`, `"log/slog"`, and `"github.com/specgraph/specgraph/internal/storage"`. (`strings` is already imported.)

- [ ] **Step 4: Update the existing RegisterAuthHandlers test caller**

In `internal/server/auth_handler_test.go`, find the `RegisterAuthHandlers(...)` call (the explore report noted one at `auth_handler_test.go:48`) and add a `nil` webAuth argument:

```go
RegisterAuthHandlers(mux, resolver, nil, auth.RequireAuth(resolver))
```

Add a logout-revocation test:

```go
func TestLogout_RevokesSession(t *testing.T) {
	revoked := false
	wa := &logoutFakeWA{onRevoke: func() { revoked = true }}
	mux := http.NewServeMux()
	// resolver can be a stub that returns ErrUnauthenticated; logout doesn't resolve.
	RegisterAuthHandlers(mux, stubResolver{}, wa, func(h http.Handler) http.Handler { return h })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "specgraph_session", Value: "spgr_ws_abc"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent { t.Fatalf("want 204, got %d", rec.Code) }
	if !revoked { t.Fatal("expected RevokeSession to be called") }
}
```

Define a minimal `logoutFakeWA` (implement the `WebAuthStore` methods; only `RevokeSession` matters — others can return zero values) and a `stubResolver`. (If a resolver stub already exists in this test file, reuse it.)

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -run 'TestLogout|TestStart_|TestCallback_' -v`
Expected: PASS (this also makes Task 12's tests compile + pass now that `establishSession` exists).

- [ ] **Step 6: Commit**

```bash
git add internal/server/auth_handler.go internal/server/auth_handler_test.go
git commit -s -m "feat(server): SameSite=Lax session cookie, establishSession, logout session revocation"
```

---

### Task 14: Wire everything in serve.go (build providers, register handlers, sweeper)

**Files:**

- Modify: `cmd/specgraph/serve.go`
- Modify: `internal/server/sweeper.go` (add web-auth sweeper)

- [ ] **Step 1: Add the web-auth sweeper**

In `internal/server/sweeper.go`, add:

```go
// WebAuthSweeper releases expired web sessions and login flows.
type WebAuthSweeper interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	DeleteExpiredLoginFlows(ctx context.Context) (int64, error)
}

// StartWebAuthSweeper launches a goroutine that periodically GCs expired web
// sessions and OIDC login flows. Stops when ctx is cancelled.
func StartWebAuthSweeper(ctx context.Context, store WebAuthSweeper, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := store.DeleteExpiredSessions(ctx); err != nil {
					slog.LogAttrs(ctx, slog.LevelError, "sweep expired sessions", slog.Any("error", err))
				} else if n > 0 {
					slog.LogAttrs(ctx, slog.LevelDebug, "swept expired sessions", slog.Int64("count", n))
				}
				if n, err := store.DeleteExpiredLoginFlows(ctx); err != nil {
					slog.LogAttrs(ctx, slog.LevelError, "sweep expired login flows", slog.Any("error", err))
				} else if n > 0 {
					slog.LogAttrs(ctx, slog.LevelDebug, "swept expired login flows", slog.Int64("count", n))
				}
			}
		}
	}()
}
```

- [ ] **Step 2: Build login providers in buildAppDeps (fatal on error)**

In `cmd/specgraph/serve.go`, add a field to `appDeps`:

```go
	loginProviders []auth.LoginProvider
```

In `buildAppDeps`, after the existing verifier loop (after line ~87), add:

```go
	loginProviders, lpErr := auth.BuildLoginProviders(ctx, cfg.Auth.OIDC.Providers, cfg.Auth.OIDC.BaseURL)
	if lpErr != nil {
		return appDeps{}, fmt.Errorf("OIDC login providers: %w", lpErr)
	}
```

Add `loginProviders: loginProviders,` to the returned `appDeps{...}` literal.

- [ ] **Step 3: Wire WebAuthStore into the resolver**

In `buildAppHandler`, in the `auth.NewIdentityStore(auth.IdentityStoreConfig{...})` literal (lines ~155-165), add:

```go
		WebAuth:                 res.authStore,
```

(`res.authStore` is `*postgres.AuthStore`, which now implements `storage.WebAuthStore`.)

- [ ] **Step 4: Register the OIDC handlers + update RegisterAuthHandlers call**

In `buildAppHandler`, replace the existing `server.RegisterAuthHandlers(...)` line (line ~206) with:

```go
	server.RegisterAuthHandlers(mux, resolver, res.authStore, auth.RequireAuth(resolver))
	server.RegisterOIDCLoginHandlers(mux, server.OIDCLoginConfig{
		Providers:  deps.loginProviders,
		Resolver:   resolver,
		WebAuth:    res.authStore,
		BaseURL:    cfg.Auth.OIDC.BaseURL,
		SessionTTL: cfg.Auth.OIDC.SessionTTL,
		FlowTTL:    5 * time.Minute,
		Limiter:    server.NewIPRateLimiterForOIDC(cfg.Server.TrustedProxy),
	})
```

Add an exported constructor to `internal/server/ratelimit.go` so serve.go doesn't reach into unexported internals:

```go
// NewIPRateLimiterForOIDC returns the rate limiter used for the public OIDC
// start/callback endpoints: 10 requests/min, burst 20, per client IP.
func NewIPRateLimiterForOIDC(trustedProxy bool) *ipRateLimiter {
	return newIPRateLimiter(10.0/60.0, 20, trustedProxy)
}
```

- [ ] **Step 5: Start the web-auth sweeper alongside the claim sweeper**

In `serve.go` `activate`, find the existing `server.StartSweeper(sweeperCtx, r.store, 60*time.Second)` call (line ~446) and add immediately after it:

```go
		server.StartWebAuthSweeper(sweeperCtx, r.authStore, 60*time.Second)
```

- [ ] **Step 6: Build and run the full suite**

Run: `go build ./...`
Expected: success.

Run: `go test ./internal/server/ ./internal/auth/ ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/specgraph/serve.go internal/server/sweeper.go internal/server/ratelimit.go
git commit -s -m "feat(server): wire interactive OIDC login providers, handlers, and session sweeper"
```

---

### Task 15: Frontend — provider buttons + auth_error surfacing

**Files:**

- Create: `web/src/lib/oidc.svelte.ts`
- Modify: `web/src/lib/components/LoginModal.svelte`
- Modify: `web/src/routes/+layout.svelte`
- Modify: `web/package.json` (add vitest for the pure-logic test)
- Create: `web/src/lib/oidc.test.ts`

- [ ] **Step 1: Create the OIDC client module**

Create `web/src/lib/oidc.svelte.ts`:

```ts
export interface OidcProvider {
  id: string;
  displayName: string;
}

// fetchProviders returns the configured interactive OIDC providers (empty on
// error or when none are configured).
export async function fetchProviders(): Promise<OidcProvider[]> {
  try {
    const resp = await fetch('/api/auth/oidc/providers');
    if (!resp.ok) return [];
    const data = await resp.json();
    return (data.providers ?? []).map((p: { id: string; display_name: string }) => ({
      id: p.id,
      displayName: p.display_name,
    }));
  } catch {
    return [];
  }
}

// authErrorMessage maps a backend auth_error reason token to a friendly message.
export function authErrorMessage(reason: string | null): string {
  switch (reason) {
    case 'denied': return 'Sign-in was cancelled or denied by the provider.';
    case 'unauthorized': return 'Your account is not permitted to sign in. Contact an administrator.';
    case 'expired': return 'The sign-in attempt expired. Please try again.';
    case 'state': return 'The sign-in could not be verified. Please try again.';
    case 'exchange': return 'Sign-in failed during authentication. Please try again.';
    case 'temporary': return 'The server is temporarily unavailable. Please try again shortly.';
    default: return 'Sign-in failed. Please try again.';
  }
}
```

- [ ] **Step 2: Add a vitest unit test for the pure logic**

Add vitest to `web/package.json` devDependencies and a `test` script:

```json
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vitest run"
  },
```

Add to `devDependencies`: `"vitest": "^3.0.0"`. Then create `web/src/lib/oidc.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { authErrorMessage } from './oidc.svelte';

describe('authErrorMessage', () => {
  it('maps known reasons', () => {
    expect(authErrorMessage('denied')).toContain('cancelled');
    expect(authErrorMessage('unauthorized')).toContain('not permitted');
    expect(authErrorMessage('temporary')).toContain('temporarily');
  });
  it('falls back for unknown reasons', () => {
    expect(authErrorMessage('weird')).toBe('Sign-in failed. Please try again.');
    expect(authErrorMessage(null)).toBe('Sign-in failed. Please try again.');
  });
});
```

- [ ] **Step 3: Run the frontend test**

Run (in `web/`): `pnpm install && pnpm test`
Expected: PASS (2 tests).

- [ ] **Step 4: Add provider buttons to LoginModal**

Edit `web/src/lib/components/LoginModal.svelte`. Replace the `<script>` block and the markup above the API-key input. New `<script>`:

```svelte
<script lang="ts">
  import { login } from '$lib/auth.svelte';
  import { fetchProviders, authErrorMessage, type OidcProvider } from '$lib/oidc.svelte';
  import { onMount } from 'svelte';

  let key = $state('');
  let error = $state('');
  let loading = $state(false);
  let providers = $state<OidcProvider[]>([]);

  let { onSuccess, authError = null } = $props<{ onSuccess: () => Promise<void>; authError?: string | null }>();

  onMount(async () => {
    providers = await fetchProviders();
    if (authError) error = authErrorMessage(authError);
  });

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = '';
    loading = true;
    try {
      const ok = await login(key);
      if (ok) {
        await onSuccess();
      } else {
        error = 'Invalid API key. Check your key and try again.';
        key = '';
      }
    } catch {
      error = 'Connection error. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>
```

In the markup, insert provider buttons and a divider above the `<p>Enter your API key...</p>` line, inside `<form class="login-card">` after `<h2>SpecGraph</h2>`:

```svelte
    {#if providers.length > 0}
      <div class="providers">
        {#each providers as p}
          <a class="oidc-btn" href={`/api/auth/oidc/${p.id}/start`}>Sign in with {p.displayName}</a>
        {/each}
      </div>
      <div class="divider"><span>or</span></div>
    {/if}
```

Add styles inside the `<style>` block:

```svelte
  .providers { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 0.75rem; }
  .oidc-btn {
    display: block; text-align: center; text-decoration: none;
    padding: 0.5rem; border: 1px solid #1a1a2e; border-radius: 4px;
    color: #1a1a2e; font-size: 0.9rem;
  }
  .oidc-btn:hover { background: #f1f5f9; }
  .divider { display: flex; align-items: center; text-align: center; color: #94a3b8; font-size: 0.8rem; margin: 0.25rem 0 0.75rem; }
  .divider::before, .divider::after { content: ''; flex: 1; border-bottom: 1px solid #e2e8f0; }
  .divider span { padding: 0 0.5rem; }
```

- [ ] **Step 5: Surface auth_error in +layout.svelte**

Edit `web/src/routes/+layout.svelte`. In the `<script>` block, after the `let ready` line, add:

```js
  let authError = $state(null);
```

In `onMount`, after `await checkAuth();`, add (read + strip the query param):

```js
    const params = new URLSearchParams(window.location.search);
    const ae = params.get('auth_error');
    if (ae) {
      authError = ae;
      params.delete('auth_error');
      const qs = params.toString();
      history.replaceState({}, '', window.location.pathname + (qs ? '?' + qs : ''));
    }
```

Pass it to the modal — change:

```svelte
  <LoginModal onSuccess={handleLoginSuccess} />
```

to:

```svelte
  <LoginModal onSuccess={handleLoginSuccess} {authError} />
```

- [ ] **Step 6: Build the web bundle**

Run (in `web/`): `pnpm build`
Expected: build succeeds, output in `web/build/`.

- [ ] **Step 7: Manual smoke (documented, not automated)**

With a configured interactive provider and `task build && ./specgraph serve`, open the dashboard: the modal shows a "Sign in with <provider>" button above the API-key field; visiting `/?auth_error=denied` shows the friendly message and strips the param. Record the result in the PR description.

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/oidc.svelte.ts web/src/lib/oidc.test.ts web/src/lib/components/LoginModal.svelte web/src/routes/+layout.svelte web/package.json web/pnpm-lock.yaml
git commit -s -m "feat(web): OIDC provider buttons and auth_error surfacing in login modal"
```

---

### Task 16: Documentation — operator guide + architecture update

**Files:**

- Create: `site/docs/guides/oidc-login.md`
- Modify: `site/docs/architecture.md`

- [ ] **Step 1: Write the guide**

Create `site/docs/guides/oidc-login.md` covering, with copy-pasteable YAML:

- **Overview:** interactive login mints a server-side session (`auth.oidc.session_ttl`); the session cookie is opaque; multi-replica safe (no sticky sessions needed because handshake state is server-side).
- **Common config:** redirect URI `https://<host>/api/auth/oidc/callback`; `auth.oidc.base_url` is **required behind any proxy that rewrites Host**; set `server.trusted_proxy: true` when behind a trusted load balancer so the per-IP rate limit keys on the real client IP; secret via `client_secret_env` (never commit plaintext).
- **Entra ID:** tenant-specific issuer `https://login.microsoftonline.com/<tenant>/v2.0` (NOT `common`/`organizations` — the resolver routes by exact `iss`); app registration must emit the `groups` claim for `groups`→role mapping; beware groups overage. Full provider block with `interactive: true`, `client_secret_env`, `claims_mapping`.
- **Okta:** issuer `https://<org>.okta.com/oauth2/default`; OIDC "Web" app; `groups` claim mapping.
- **GitHub via OIDC broker:** explicit callout that GitHub-direct is not OIDC (opaque token, no `id_token`); federate through a broker (Entra External ID / Auth0 / Keycloak / Dex) and point `issuer` at the broker. Note native GitHub is a deferred follow-up.
- **Security notes:** interactive logins bypass the JIT rate limiter, so configure `auth.oidc.jit_create.email_domain_allowlist` for internet-facing deployments; logout revokes the server session but not the IdP SSO session.

Use the verbatim Entra block from the design spec (`docs/superpowers/specs/2026-06-12-oidc-interactive-ui-login-design.md` §1) as the canonical example.

- [ ] **Step 2: Update architecture.md**

In `site/docs/architecture.md`, find the Authentication section (~lines 176-192, currently describes OIDC as token-only) and add a paragraph describing the interactive browser login flow + server-side session, with a link to `guides/oidc-login.md`.

- [ ] **Step 3: Lint the docs**

Run: `task lint` (runs markdown lint among others)
Expected: PASS (fix any rumdl/markdownlint issues in the new file).

- [ ] **Step 4: Commit**

```bash
git add site/docs/guides/oidc-login.md site/docs/architecture.md
git commit -s -m "docs: add interactive OIDC login guide and update architecture"
```

---

### Task 17: File the deferred follow-up + final verification

**Files:** none (issue tracker + full gate)

- [ ] **Step 1: File the native-GitHub follow-up issue**

Create a beads issue (per AGENTS.md, use `bd`) for "native generic OAuth2 + userinfo provider (GitHub-direct)": a new `kind: oauth2` LoginProvider doing authorize + token + `GET userinfo`, building Identity from userinfo, pairing with the server session. Reference this plan and the design spec.

```bash
bd create "Native generic OAuth2 + userinfo login provider (GitHub-direct)" --type feature
```

- [ ] **Step 2: Run the full quality gate**

Run: `task check`
Expected: fmt → license → lint → build → unit tests all PASS.

- [ ] **Step 3: Run integration tests (Docker)**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestWebAuth -v`
Expected: PASS.

- [ ] **Step 4: Commit any fixups, then push per the session-completion protocol**

```bash
git add -A
git commit -s -m "chore: oidc interactive login fixups" # only if needed
```

---

## Self-review checklist (completed by plan author)

- **Spec coverage:** config fields (T1) ✓; nonce verify B3 (T2); errors+domain (T3); WebAuthStore iface (T4); migration (T5); store impl (T6) + integration (T7); interactive ctx marker MA-2 (T8); resolveSession + dispatch + JIT bypass B4/MA-3 (T9); login providers + secret resolution B1/B2/M2 + RedirectURI (T10); per-IP rate limit MAJOR-1 (T11); providers/start/callback + tx cookie + provider-removed MINOR-2 + tx-cookie clear MINOR-4 (T12); SameSite=Lax M3 + establishSession MINOR-5 + logout revoke M6 + signature MINOR-2 (T13); serve wiring + sweeper MI-1 (T14); frontend buttons + auth_error (T15); docs incl. Entra `common`/groups/proxy MINOR-1/M-a/M-b/M-d (T16); follow-up + gates (T17). ✓
- **Type consistency:** `LoginProvider` (exported) used consistently T10/T12/T14; `storage.WebAuthStore` T4/T6/T9/T13/T14; `establishSession` defined T13, used T12; `OIDCLoginConfig`/`RegisterOIDCLoginHandlers` T12/T14; `NewIPRateLimiterForOIDC` T11/T14; `auth.RedirectURI` T10/T12. ✓
- **Cross-task dependency flagged:** T12's handler tests need `establishSession` from T13 — noted in T12 Step 2/3 and T13 Step 5.
- **Placeholder scan:** the only intentionally-abstract step is T9 Step 6's `TestJITResolve_InteractiveBypassesLimiter` body and T16's prose-described doc content; both reference the concrete existing scaffolding/spec to fill from. No TBD/TODO in code steps.

## Execution handoff

See the offer in the chat after this plan.
