# Interactive OIDC login for the web UI

**Date:** 2026-06-12
**Status:** Design revised after two adversarial-review rounds, pending spec review

## Problem

SpecGraph's server already **verifies** OIDC ID tokens. The verifier
(`internal/auth/oidc_verifier.go`) discovers a provider via
`.well-known/openid-configuration`, validates a presented JWT's
signature/audience/expiry, and the identity resolver
(`internal/auth/identitystore.go`) routes JWT-shaped tokens through it, with
just-in-time (JIT) user provisioning. Per-provider config lives under
`auth.oidc.providers` (`internal/config/global.go:237`).

But there is **no interactive login flow**. The web dashboard
(`web/`, a SvelteKit app embedded into the binary) authenticates **only via
API key**: `LoginModal.svelte` shows an "Enter your API key" box, posts it to
`POST /api/auth/login` (`internal/server/auth_handler.go:52`), which resolves
the value and stores it in an HttpOnly `specgraph_session` cookie that the auth
middleware re-resolves per request (`cookieToAuthHeader`, `auth_handler.go:105`).

To use OIDC today, a user must obtain a JWT out-of-band and either present it as
a bearer token (CLI/API) or paste it into the modal's key field. There is no
"Sign in with &lt;provider&gt;" button, no authorize redirect, and no callback.

We want the dashboard to offer a real **"Sign in with &lt;provider&gt;"** button
that runs the standard OAuth2 Authorization Code flow and lands the user
authenticated.

## Review history (what changed and why)

**Round 1** found the original "store the IdP ID token in the cookie" approach
breaks for the design's own headline case: Entra ID tokens **with a `groups`
claim** routinely exceed the ~4 KB cookie limit (silently dropped → login loop).
It also caught a non-existent `${env:}` secret mechanism, a backward-compat
regression, a silently-skipped nonce check, and a JIT-disabled login loop. The
fix pulled the **server-side session** forward.

**Round 2** verified those fixes are sound but caught: a self-contradictory
"non-fatal discovery" claim (unbuildable against the immutable verifier map), an
**ephemeral tx-cookie key that breaks multi-replica login**, **interactive login
silently consuming the JIT rate-limit budget**, and an unspecified
transient-vs-unauthenticated path in session resolution that would log users out
on a DB blip. This revision resolves all of them (see "Resolved review
findings").

## Goals

- A **"Sign in with &lt;provider&gt;"** button per configured interactive
  provider, alongside the existing API-key field.
- A server-side **Authorization Code flow with PKCE** that, on success, issues a
  **SpecGraph-issued opaque server-side session** (small id in the cookie;
  session row in storage), decoupled from IdP token lifetime.
- **Entra ID** and **Okta** native; **GitHub** documented via an OIDC broker
  (not OIDC for sign-in — see below), native generic-OAuth2 deferred.
- Existing **token-only OIDC** configs keep working unchanged, no secret
  required.
- Multi-replica / rolling-deploy safe.

## Non-goals

- No silent refresh of IdP tokens (server session has its own TTL; re-login on
  expiry; no IdP refresh tokens stored).
- No native GitHub-direct login now (deferred follow-up).
- No CLI browser-login flow. Web UI only.
- No change to JIT policy semantics, claim mapping, the role model, or the
  bearer-token verification path (other than the interactive-login limiter
  bypass, §6).

## Why GitHub is not native OIDC

Confirmed against GitHub's OAuth docs: GitHub's web flow issues an **opaque
access token** (`gho_...`) and **no `id_token`**; there is no user-facing
`.well-known/openid-configuration`. Identity comes from
`GET https://api.github.com/user`. Our flow verifies an ID token, so GitHub
needs a separate OAuth2+userinfo path (the deferred follow-up). The supported
path **today** is to federate GitHub through an OIDC broker (Entra External ID,
Auth0, Okta, Keycloak, Dex) and point SpecGraph at the broker's issuer.

## Behavior contract

| Scenario | Outcome |
|----------|---------|
| No interactive providers configured | Modal shows API-key field only (today's behavior, unchanged). |
| 1+ interactive providers configured | Button per provider **above** the API-key field. |
| Click "Sign in with X" | Full-page nav to `/api/auth/oidc/{id}/start` → 302 to IdP authorize. |
| IdP success, identity resolves | callback exchanges code, verifies ID token (+nonce), resolves identity, mints a server session, sets the session cookie, 302 to `/`. |
| IdP success but identity unauthorized (JIT off / not allowlisted) | 302 to `/?auth_error=unauthorized` — **no cookie seated**. |
| Backend/DB error during resolve or mint | 302 to `/?auth_error=temporary`. |
| Server session later expires / revoked | Next request 401 → modal; user re-clicks the button. |
| Session lookup hits a DB outage | Request gets **503** (ErrTransient), not 401 — user stays logged in. |
| IdP returns `error` / user cancels | 302 to `/?auth_error=denied`. |
| flow-state mismatch / tx cookie missing/expired | 302 to `/?auth_error=state` / `=expired`. |
| token exchange / verification fails | 302 to `/?auth_error=exchange`. |
| Provider removed from config between `start` and `callback` | 302 to `/?auth_error=exchange` (logged). |
| `start`/`callback` per-IP rate limit exceeded | **429** with `Retry-After`. |
| Unknown `{provider}` in path (at `start`) | 404. |
| Provider misconfigured (bad kind / interactive without secret / bad audience) | **Fatal at startup**. |
| Provider discovery (IdP) unreachable at startup | **Fatal at startup** (see "Startup behavior"). |

## Architecture

A server-side OAuth2 Authorization Code + PKCE flow added as HTTP endpoints in
`internal/server/`, login-provider construction at startup, a new
**web-auth store** (sessions + login-flow state) in the storage layer, a new
**session resolution path** in the resolver, and small frontend additions. The
cookies hold only opaque ids; all sensitive/bulky data lives server-side.

### 1. Provider config: interactive opt-in + secret resolution (fixes B1, B2, M2)

`OIDCProviderConfig` (`internal/config/global.go:237`) gains:

```yaml
auth:
  oidc:
    base_url: "https://specgraph.example.com"   # NEW (optional); see redirect URI
    session_ttl: 12h                              # NEW (optional); default 12h
    providers:
      - id: entra
        kind: oidc                 # NEW; only "oidc" now (default "oidc")
        interactive: true          # NEW; opt-in to the login flow
        display_name: "Microsoft Entra"   # NEW; button label, defaults to id
        issuer: https://login.microsoftonline.com/<tenant>/v2.0
        client_id: <app-id>
        client_secret_env: SPECGRAPH_OIDC_ENTRA_SECRET  # NEW; preferred
        client_secret: ""          # NEW; plaintext fallback (dev only), discouraged
        # audience omitted -> defaults to client_id (required for interactive)
        scopes: [openid, profile, email]   # NEW; default [openid, email, profile]
        claims_mapping:
          - { claim: groups, value: <group-oid>, role: admin }
```

- **`interactive`** (default `false`) is the opt-in. Only interactive providers
  require a secret, get an `oauth2.Config`, are built into the login-provider
  set, and render a button. `interactive: false` providers behave exactly as
  today: a bearer-JWT verifier, **no secret required** (fixes the B2 regression).
- **Secret resolution (fixes B1):** `auth.oidc.providers` is a slice, so the
  `SPECGRAPH_*` env mapper cannot reach a per-provider field
  (`global.go:411,429-435`), and there is no `${env:...}` interpolation in the
  loader. We add **`client_secret_env`**: the name of an env var read at
  **provider-build time** in `serve.go` (no koanf changes). `client_secret`
  (plaintext) remains a dev-only fallback. An interactive provider MUST resolve a
  non-empty secret from exactly one of the two; an unset named var is a **fatal**
  startup error. Secret never logged. (Verified no code dumps the full config;
  `writeGlobal` only persists `globalDefaults()`, never a loaded secret —
  `global.go:329,438`.)
- **`kind`** defaults to `oidc`; unknown → fatal. Reserves space for a future
  `oauth2` kind (Seam below).
- **`audience` (M2):** for interactive providers, validate at startup that
  `audience` is empty or equals `client_id` (an interactive `id_token` always has
  `aud=client_id`). Custom-audience bearer flows use a separate non-interactive
  entry. Documented.
- **`scopes`** default `[openid, email, profile]`; `openid` forced.
- **`session_ttl`** default 12h; absolute server-session lifetime.
- Additive and backward compatible; the deprecated `auth.oidc_providers`
  migration (`applyPostLoad`, `global.go:285`) is untouched.

### 2. Login-provider abstraction (seam for future native GitHub)

```go
type loginProvider interface {
    ID() string
    DisplayName() string
    AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string
    Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (idToken string, err error)
}
```

- The **`oidc`** impl wraps an `*oauth2.Config` (endpoints from go-oidc discovery
  via `provider.Endpoint()`) and a verifier. `Exchange` does the confidential-
  client token exchange (`client_secret` + PKCE `code_verifier`), pulls
  `id_token` from the `*oauth2.Token` extra, verifies it **with nonce** (§3), and
  returns the raw ID token for the callback to resolve (§7).
- A future **`oauth2`** kind (userinfo-based, GitHub) implements the same
  interface — **deferred follow-up.**
- Built once at startup for interactive providers only.

### 3. Nonce-checked verification (fixes B3)

`OIDCVerifier.Verify` passes no `oidc.Nonce(...)` and `OIDCClaims` has no nonce
field (`oidc_verifier.go:48,78`), so "wrapping it" would silently skip nonce.
Add `OIDCVerifier.VerifyWithNonce(ctx, rawToken, expectedNonce)` (passes
`oidc.Nonce(expectedNonce)` / constant-time compares `idToken.Nonce`) and add a
`Nonce` field to `OIDCClaims`. The bearer path keeps the existing `Verify`
(unchanged — we don't mint bearer tokens). Test: wrong/missing nonce fails.

### 4. Web-auth store (sessions + login-flow state)

One new goose migration
`internal/storage/postgres/auth_migrations/002_web_auth.sql` with **two** tables.
Both run under the dedicated `goose_db_version_auth` table (`auth.go:106`),
isolated from project migrations; `gen_random_uuid()`/pgcrypto already enabled
(`001_initial.sql:6`).

```sql
-- +goose Up
CREATE TABLE web_sessions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash    bytea NOT NULL UNIQUE,         -- SHA-256 of the opaque session token
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer        text NOT NULL DEFAULT '',      -- audit / future RP-logout
    oidc_subject  text NOT NULL DEFAULT '',      -- audit
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);
CREATE INDEX web_sessions_user   ON web_sessions(user_id);
CREATE INDEX web_sessions_expiry ON web_sessions(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE oidc_login_flows (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),  -- the opaque flow id (in the tx cookie)
    state         text NOT NULL,                -- CSRF token, constant-time compared
    nonce         text NOT NULL,
    code_verifier text NOT NULL,                -- PKCE
    provider_id   text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL          -- now + ~5m (short TTL bounds table growth)
);
CREATE INDEX oidc_login_flows_expiry ON oidc_login_flows(expires_at);

-- +goose Down
DROP INDEX IF EXISTS oidc_login_flows_expiry;
DROP TABLE IF EXISTS oidc_login_flows;
DROP INDEX IF EXISTS web_sessions_expiry;
DROP INDEX IF EXISTS web_sessions_user;
DROP TABLE IF EXISTS web_sessions;
```

New `storage.WebAuthStore` interface (separate from `UsersBackend`, so existing
`UsersBackend` fakes/stubs are unaffected), implemented by `*postgres.AuthStore`
(which already owns the identity tables, `connector.go`):

```go
type WebAuthStore interface {
    // sessions
    CreateSession(ctx, *Session) (*Session, error)
    LookupSessionByHash(ctx, tokenHash []byte) (*Session, error) // ErrSessionNotFound on miss
    RevokeSession(ctx, tokenHash []byte) error                   // idempotent
    DeleteExpiredSessions(ctx) (int64, error)
    // login-flow state (server-side; survives multi-replica — fixes MA-1)
    CreateLoginFlow(ctx, *LoginFlow) (flowID string, err error)
    ConsumeLoginFlow(ctx, flowID string) (*LoginFlow, error)     // atomic read+delete; ErrLoginFlowNotFound on miss/expired
    DeleteExpiredLoginFlows(ctx) (int64, error)
}
```

- Session token: **32 random bytes**, shown to the browser as
  `spgr_ws_<base64url>`; only its **SHA-256 hash** is stored (high-entropy →
  SHA-256 sufficient; argon2 is only for low-entropy pasteable secrets like API
  keys). Lookup by hash. There is **no `last_used_at`** column and no idle
  timeout — absolute `session_ttl` only — so resolution stays read-only (no
  per-request write).
- `ConsumeLoginFlow` is a single `DELETE ... RETURNING` (atomic, single-use),
  rejecting expired rows.
- **Plaintext-at-rest (deliberate, MINOR-3):** unlike `web_sessions` (which
  stores only a SHA-256 hash), `oidc_login_flows` stores `state`/`nonce`/
  `code_verifier` in cleartext. This is safe: a DB-read attacker still cannot
  complete a hijacked handshake without the authorization `code`, which the IdP
  delivers only to the victim's browser and which never touches the DB. Rows are
  short-lived (~5m) and single-use.
- A periodic sweeper (see §7 wiring) GCs both expired sessions and flows on a
  tight interval (e.g. 1m), bounding `oidc_login_flows` together with the per-IP
  rate limit on `start` (§7) and the short TTL.

### 5. Resolver: session resolution path (fixes B4 cleanly + MA-3)

Add a branch to `pgIdentityStore.Resolve` (`identitystore.go:150`):

```text
if isJWTShaped(token)                     -> resolveJWT      (bearer / OIDC JWT, unchanged)
if strings.HasPrefix(token, "spgr_ws_")   -> resolveSession  (NEW)
else                                       -> resolveAPIKey   (unchanged)
```

A `spgr_ws_<base64url>` token has zero dots (base64url contains no `.`), so it is
never JWT-shaped (`isJWTShaped`, `identitystore.go:171`); the new prefix branch
must precede the api-key fallback (a `spgr_ws_` token fails `parseAPIKey`).

`resolveSession` mirrors `resolveAPIKey`'s error discipline (**MA-3**):
SHA-256 the token, `LookupSessionByHash`, then:

- `ErrSessionNotFound` / `revoked_at != nil` / `expires_at <= now` / soft-deleted
  user → **`ErrUnauthenticated`**;
- **any other** lookup/`GetUserByID` error → **`fmt.Errorf("%w: %w",
  ErrTransient, err)`** (so a Postgres outage yields 503, not a 401 that logs
  everyone out).
Returns an `Identity{Source:"oidc", Subject:"oidc:"+OIDCSubject,
EffectiveRole:user.Role}`.

The resolver gains a `WebAuth storage.WebAuthStore` field on
`IdentityStoreConfig`. **Nil-guard (MI-3):** if `WebAuth == nil`, a `spgr_ws_`
token resolves to `ErrUnauthenticated` (never a nil panic), so existing
resolver constructions/tests that don't wire it stay safe.

Resolver construction ordering is fine: the resolver is built in
`buildAppHandler`, which runs only **after** a live DB connection exists
(degraded boot serves a blanket 503 until then), so `resolveSession` is never
called without a store.

### 6. Interactive login bypasses the JIT rate limiter (fixes MA-2)

The callback resolves identity via `resolver.Resolve(idToken)` → `resolveJWT` →
(on binding miss, JIT) `jitResolve`, which consumes a per-issuer rate-limit
token (`identitystore.go:444`, default 100/hr) shared with the bearer path. That
limiter exists to bound **unsolicited bearer-JWT** JIT; an interactive login is
user-driven and already IdP+PKCE+nonce authenticated, so the abuse vector is
weak and an onboarding burst shouldn't lock users out.

The callback marks the context via a new `auth.WithInteractiveLogin(ctx)`;
`jitResolve` checks it and **skips the rate-limit gate** for that path. The
**email-domain allowlist and claims-mapping role derivation still apply** — only
the rate counter is bypassed. Bearer-path JIT is unchanged. Test: an interactive
JIT create does not decrement the issuer bucket; allowlist still rejects
out-of-domain.

**Residual risk (MINOR-1), documented:** bypassing the limiter removes the only
backpressure on JIT user creation from the interactive path, so an actor
controlling many valid IdP accounts in an allowed domain (or any account when no
allowlist is set) could mass-create `users`/`web_sessions` rows. This is weaker
than an anonymous flood — each create costs a full IdP auth + code + nonce
round-trip and is fully attributable — and the mitigating control is the
**email-domain allowlist**, which the guide recommends configuring for any
internet-facing deployment that enables JIT. The marker is an unexported context
key set **only** in the callback, so no bearer request can forge the bypass
(both bearer entry points resolve on the unmarked request context —
`interceptor.go`, `middleware.go`).

### 7. New endpoints (`internal/server/auth_oidc_handler.go`)

Registered with bare `mux.HandleFunc` (no `RequireAuth`) next to the existing
auth handlers (`serve.go:206`). Go ServeMux pattern matching routes
`/api/auth/oidc/...` ahead of the `/` SPA handler; `ProjectMiddleware` passes
through (reads only `X-Specgraph-Project`). The handlers receive the
`[]loginProvider`, the `resolver`, and the `WebAuthStore`.

**Per-IP rate limit (fixes MAJOR-1).** `start` and `callback` are public and
`start` performs an unauthenticated DB write (`CreateLoginFlow`); with no
existing rate-limiting middleware in the server, this is a write-amplification /
table-growth DoS vector. Both endpoints are wrapped in a per-client-IP
token-bucket limiter (a `sync.Map` of `golang.org/x/time/rate.Limiter` keyed by
IP — the same library the JIT limiter uses), e.g. ~10/min burst 20 per IP;
over-limit → **429** with `Retry-After`. Client IP is taken from
`X-Forwarded-For`/`X-Real-IP` **only when** a trusted-proxy setting is enabled
(reusing the same proxy-trust posture as the `X-Forwarded-Proto` `Secure`
decision), else `RemoteAddr`; the docs state that behind a proxy the trusted-
header config is required or all clients share one bucket. This per-IP bound,
plus the ~5m TTL and the 1m sweeper (§4), bounds `oidc_login_flows`.

1. **`GET /api/auth/oidc/providers`** — public, **no DB access** (reads the
   in-memory provider list). Returns
   `{"providers":[{"id","display_name"}]}` for **interactive** providers; empty
   when none.

2. **`GET /api/auth/oidc/{provider}/start`** (per-IP rate limited)
   - Look up interactive provider; unknown → 404.
   - Generate `state` (32 random bytes, base64url), `nonce`, PKCE
     `code_verifier` + `S256` `code_challenge`.
   - `CreateLoginFlow{state,nonce,code_verifier,provider_id, expires_at:+5m}` →
     **opaque flow id**; set `specgraph_oidc_tx` cookie to that id (HttpOnly,
     `SameSite=Lax`, `Secure` dynamic, `Path=/api/auth/oidc`, `Max-Age` ~300s).
     No signing key needed — the cookie carries only an unguessable id; the
     secret material lives server-side (**fixes MA-1; multi-replica safe**).
   - 302 to `provider.AuthCodeURL(...)`.

3. **`GET /api/auth/oidc/callback`** (per-IP rate limited)
   - Read `specgraph_oidc_tx`; missing → `auth_error=expired`. Always emit a
     `Max-Age=-1` delete for `specgraph_oidc_tx` (`Path=/api/auth/oidc`) on this
     response, success or failure (MINOR-4 hygiene; the flow id is inert once
     consumed).
   - `ConsumeLoginFlow(flowID)` (atomic single-use); miss/expired →
     `auth_error=expired`.
   - IdP `error` param → `auth_error=denied`.
   - Constant-time compare the IdP `state` against the flow's `state` → mismatch
     `auth_error=state`.
   - Resolve the provider from `flow.ProviderID`; **if it is no longer a
     configured interactive provider** (config edited mid-flow, MINOR-2) →
     `auth_error=exchange` (logged).
   - `provider.Exchange(code, flow.CodeVerifier, flow.Nonce, redirectURI)` → raw
     ID token; failure → `auth_error=exchange`.
   - **Resolve identity (fixes B4):**
     `resolver.Resolve(auth.WithInteractiveLogin(ctx), idToken)` → `resolveJWT`
     (binding lookup or JIT at login time, limiter bypassed per §6). The nonce
     was already checked in `Exchange`; `resolveJWT` re-verifies
     signature/aud/expiry via the existing `Verify` — which requires the
     interactive provider's issuer to be in the resolver verifier map, so
     `serve.go` builds a verifier for **every** provider (interactive or not) and
     passes them all to `NewIdentityStore`, exactly as today; `interactive` only
     gates the additional `oauth2.Config`/login-provider build. On
     `ErrUnauthenticated` → `auth_error=unauthorized` (no cookie). On
     `ErrTransient` → `auth_error=temporary`.
   - **Mint session:** `CreateSession` (user id, issuer, subject,
     `expires_at = now + session_ttl`); `establishSession` sets
     `specgraph_session` to the opaque `spgr_ws_...` token.
   - 302 to `/`.
   - All failure reasons are generic tokens; no internal text leaks; full errors
     logged server-side with provider id.

**Sweeper wiring (MI-1):** the existing `StartSweeper` is hardcoded to
`ClaimSweeper.ReleaseExpiredClaims` (`sweeper.go:13`), so add a small parallel
`StartWebAuthSweeper` (calling `DeleteExpiredSessions` + `DeleteExpiredLoginFlows`
on an interval), started in `activate` after the connection is live and stopped
on graceful shutdown.

### 8. Cookies (fixes M1, M3, M4 via server-side state)

- **`specgraph_session`** carries the small opaque `spgr_ws_...` id (4 KB issue
  moot). Switch `sessionCookie()` (`auth_handler.go:135`) from `SameSiteStrict`
  to **`SameSiteLax`** (M3): redirect-based login lands via a cross-site
  top-level GET, and Lax is the standard CSRF-safe choice. **Invariant this
  depends on (MI-8):** no GET endpoint mutates state — all state-changing calls
  are POST/Connect-RPC (Connect-RPC is POST; `interceptor.go`), on which Lax
  withholds the cookie cross-site. This relaxation applies to the legacy API-key
  session cookie too, which is acceptable under the same invariant. Stated as an
  explicit design constraint to preserve.
- **Session-fixation / cookie-pinning invariant (MINOR-5):** `establishSession`
  MUST reuse `sessionCookie()`'s exact name (`specgraph_session`) and `Path=/`,
  so a callback's `Set-Cookie` deterministically overwrites any pre-existing
  session cookie (stale API key or old session) and `handleLogout`'s clear
  (also `Path=/`) always deletes it. A divergent name/path would leave a stale
  cookie that "wins" — disallowed.
- **`specgraph_oidc_tx`** carries only the opaque flow id (no secret, no HMAC
  key); HttpOnly, `SameSite=Lax`, `Secure` dynamic, `Path=/api/auth/oidc`,
  ~300s; explicitly deleted by the callback (§7).
- **Concurrent logins (M5):** the fixed-name tx cookie means a second `start`
  (another tab) overwrites the flow id; the first tab's callback then gets
  `auth_error=expired` — an accepted, clearly-surfaced limitation (not silent).
  Completed server sessions are independent.

### 9. Logout (fixes M6; MI-2)

`RegisterAuthHandlers`/`handleLogout` (`auth_handler.go:84`) gain access to the
`WebAuthStore` (signature change, noted in Files touched). On logout: if the
`specgraph_session` cookie value is `spgr_ws_`-prefixed, `RevokeSession` its hash
(guarded so a legacy API-key cookie value is never hashed/looked-up); then clear
the cookie. **v1 limitations (MI-6), documented:** re-login mints a new session
and leaves prior rows valid until TTL/sweep; logout revokes only the current
session (no "log out everywhere"); RP-initiated IdP logout
(`end_session_endpoint`) is out of scope, so local logout leaves the IdP SSO
session intact (shared-machine caveat).

### 10. Frontend (`web/`)

- `auth.svelte.ts`: `fetchProviders()` → `GET /api/auth/oidc/providers` on modal
  mount.
- `LoginModal.svelte`: a "Sign in with {display_name}" button per provider
  **above** the API-key field (kept verbatim); buttons are full-page navigations
  to `/api/auth/oidc/{id}/start` (not `fetch`). Empty list → today's modal.
  (`SecurityHeaders` sets no CSP, so navigations/inline errors are unaffected.)
- `+layout.svelte`: read `?auth_error=<reason>` on load, map to a friendly
  message, show inline, strip via `history.replaceState`.

## Startup behavior for IdP discovery (decision on M7 / BL-1)

Login-provider/verifier construction (discovery) stays **fatal at startup**, as
the existing verifier construction is today (`serve.go:79-83`). The round-1 idea
of "non-fatal discovery with background retry" was **rejected**: it is
unbuildable against the resolver's immutable, lock-free verifier map
(`identitystore.go:38,91-100,357`), where a missing issuer returns
`ErrUnauthenticated` (a hard 401), not a retryable `ErrTransient`
(`identitystore.go:358`). The non-fatal-Postgres precedent does not transfer —
Postgres has a reconnect seam; the verifier map has none. Making discovery
resilient would require a concurrency-safe verifier registry with a
"configured-but-initializing" state — a separate, larger change, explicitly
**out of scope** here and recorded as a possible future follow-up. Deterministic
config errors (unknown `kind`, interactive without a resolvable secret, bad
`audience`) are likewise fatal.

## Seam — native generic OAuth2 + userinfo (GitHub-direct), deferred

The `kind` field + `loginProvider` interface reserve space for an `oauth2`
implementation (authorize + token + `GET userinfo`, identity from userinfo).
It pairs naturally with the server-side session. **Deferred to a follow-up.**

## Security

- **state**: 32 random bytes, single-use (atomic `ConsumeLoginFlow`),
  constant-time compare.
- **nonce**: sent in authorize, validated via `VerifyWithNonce` (§3).
- **PKCE `S256`**: per-attempt `code_verifier`, defense-in-depth atop the
  confidential-client secret.
- **login-flow state** lives server-side; the tx cookie is just an unguessable
  opaque id (no signing key; multi-replica safe).
- **session cookie**: opaque id only, HttpOnly, `SameSite=Lax`, `Secure`
  dynamic; server session is revocable and absolute-TTL-bounded.
- **session token at rest**: SHA-256 hash only; raw token never logged.
- **client secret**: from `client_secret_env` (preferred) or dev plaintext
  fallback; never logged.
- **open-redirect**: callback only 302s to fixed same-origin `/` or
  `/?auth_error=...`.
- **CSRF**: relies on `SameSite=Lax` + the "no GET mutates state" invariant (§8).
- **unauthenticated-endpoint backpressure (MAJOR-1)**: per-IP rate limit on
  `start`/`callback` + ~5m flow TTL + 1m sweeper bound the unauthenticated DB
  write. `/providers` is in-memory (no DB).
- **login-flow plaintext-at-rest**: deliberate (MINOR-3) — useless to a DB-read
  attacker without the browser-delivered authorization `code`.

## Example docs (grounded)

New guide `site/docs/guides/oidc-login.md`, copy-pasteable real config:

- **Entra ID** — app registration; redirect URI
  `https://<host>/api/auth/oidc/callback`; issuer
  `https://login.microsoftonline.com/<tenant>/v2.0` (discovery confirmed: scopes
  `openid profile email offline_access`, RS256 id_token). **Use a tenant-specific
  issuer**, not `common`/`organizations` — the resolver routes by exact `iss`
  match (`identitystore.go:357`) and `common` yields a templated issuer that
  won't match a token's concrete `iss` (M-a). Document that the app registration
  must **emit the `groups` claim** (and beware "groups overage") for `groups`→
  role mapping (M-b). Secret via `client_secret_env`. Note the **multi-replica**
  requirement is satisfied automatically (server-side flow state) — no sticky
  sessions needed.
- **Okta** — OIDC "Web" app; issuer `https://<org>.okta.com/oauth2/default` (or a
  custom/org authorization server); redirect URI; scopes; `groups` mapping.
- **GitHub (via OIDC broker)** — explicit callout that GitHub-direct is not OIDC;
  worked example via a broker; "native GitHub is a deferred follow-up" pointer.
- Update `site/docs/architecture.md` auth section (currently token-only) to
  describe the interactive flow + server session, and link the guide.

## Testing

### Unit

- **Config validation:** unknown `kind` → error; `interactive` without a
  resolvable secret → error; `interactive` with `audience != client_id` → error;
  non-interactive secret-less OIDC config loads fine (**B2 regression**);
  `scopes`/`display_name`/`session_ttl` defaults; `openid` forced.
- **Secret resolution (B1):** `client_secret_env` reads the var; unset → fatal;
  plaintext fallback works; secret never in logs.
- **VerifyWithNonce (B3):** correct nonce passes; wrong/missing fails; `Nonce`
  populated.
- **state/nonce/PKCE:** high-entropy; `S256` challenge matches; constant-time
  compare.
- **redirect URI (M-d):** derived from request; `base_url` override wins.
- **resolveSession (B4/MA-3):** valid hash → identity; missing/revoked/expired →
  `ErrUnauthenticated`; soft-deleted → `ErrUnauthenticated`; **DB error →
  `ErrTransient`**; `WebAuth==nil` → `ErrUnauthenticated` (no panic); prefix
  routes ahead of api-key path.
- **interactive limiter bypass (MA-2):** interactive JIT does not decrement the
  issuer bucket; allowlist still rejects out-of-domain; bearer-path JIT still
  limited.
- **establishSession:** cookie attributes (HttpOnly, `SameSite=Lax`, `Secure`
  dynamic).
- **providers endpoint:** only interactive; empty when none; `display_name`
  default.
- **auth_error mapping:** each reason → friendly message; no internal text.
- **per-IP rate limit (MAJOR-1):** `start`/`callback` over-limit → 429 with
  `Retry-After`; client-IP extraction honors trusted-proxy headers only when
  configured.

### Flow (httptest, behind `loginProvider` + a fake OIDC issuer)

- `start`: 302 with correct `client_id`/`redirect_uri`/`scope`/`state`/
  `code_challenge`/`nonce`; creates a login-flow row; sets tx cookie with the
  flow id; unknown provider → 404.
- `callback` happy path: stubbed token endpoint + verifier → resolves identity,
  creates a `web_sessions` row, sets `spgr_ws_` cookie, 302 `/`; subsequent
  request with that cookie resolves via `resolveSession`; the login-flow row is
  consumed (single-use — replay → `auth_error=expired`); the tx cookie is
  deleted on the callback response (MINOR-4).
- `callback` failures: missing tx → `expired`; consumed/expired flow → `expired`;
  tampered state → `state`; IdP `error` → `denied`; exchange/verify failure →
  `exchange`; **provider removed mid-flow → `exchange` (MINOR-2)**; **JIT-disabled
  binding miss → `unauthorized`, no cookie (B4)**; resolve `ErrTransient` →
  `temporary`.
- **logout (M6):** revokes the session row; the token no longer resolves; a
  legacy API-key cookie value is not hashed/looked-up.

### Integration (testcontainers, `//go:build integration`)

- Session + login-flow CRUD, atomic `ConsumeLoginFlow` single-use, expiry GC,
  cascade delete on user purge; `resolveSession` end-to-end.

### Regression

- Existing API-key login, `/whoami`, and the bearer-JWT path unchanged; a
  secret-less token-only OIDC config still verifies bearer JWTs.

## Files touched (anticipated)

- `internal/config/global.go` — `OIDCProviderConfig` fields (`kind`,
  `interactive`, `display_name`, `client_secret`, `client_secret_env`, `scopes`),
  `OIDCConfig.base_url` + `session_ttl`, validation.
- `internal/auth/oidc_verifier.go` — `VerifyWithNonce` + `OIDCClaims.Nonce`.
- `internal/auth/` — `loginProvider` interface + `oidc` impl; `resolveSession` +
  `Resolve` dispatch + `WebAuth` field + nil-guard; `WithInteractiveLogin` ctx
  marker + `jitResolve` bypass.
- `internal/storage/` — `WebAuthStore` interface, `Session` + `LoginFlow` domain
  types, `ErrSessionNotFound` + `ErrLoginFlowNotFound`.
- `internal/storage/postgres/` — `002_web_auth.sql` migration + impl on
  `AuthStore`; web-auth sweeper.
- `internal/server/auth_oidc_handler.go` — providers/start/callback + per-IP
  rate limiter; `auth_handler.go` — `sessionCookie()` SameSite=Lax, logout
  revocation + signature change, `establishSession`.
- `cmd/specgraph/serve.go` — build interactive login providers (secret from env,
  fatal discovery), build every-provider verifier set (as today), wire
  `WebAuthStore` into the resolver + handlers, start the web-auth sweeper.
- `web/src/lib/auth.svelte.ts`, `web/src/lib/components/LoginModal.svelte`,
  `web/src/routes/+layout.svelte`.
- `site/docs/guides/oidc-login.md` (new), `site/docs/architecture.md` (update).
- Unit + httptest + integration tests; follow-up issue for native generic-OAuth2.

## Resolved review findings

### Round 1

- **B1** secret resolution didn't exist → `client_secret_env` at build time;
  fatal when unresolved.
- **B2** backward-compat vs validation → `interactive` opt-in; token-only configs
  need no secret.
- **B3** nonce silently unverified → `VerifyWithNonce` + `OIDCClaims.Nonce`.
- **B4** JIT-disabled login loop → callback resolves identity before seating.
- **M1** 4 KB cookie → server-side session; opaque id in cookie.
- **M2** audience collision → interactive `audience` must be empty/`client_id`,
  validated.
- **M3** SameSite=Strict → session cookie `Lax` (+ stated invariant).
- **M4** unauthenticated tx cookie → server-side flow state; cookie is an opaque
  id.
- **M5** concurrent logins → accepted/surfaced (`auth_error=expired`),
  documented.
- **M6** logout/RP-logout → logout revokes the server session; RP-logout out of
  scope, documented.
- **M-a/M-b/M-d** → documented in the guide.
- **M-c** `x/oauth2` indirect→direct → `go mod tidy` promotes; v0.36 has PKCE
  helpers.

### Round 2

- **BL-1** non-fatal-discovery contradiction → reverted to fatal-at-startup,
  rationale documented; resilient registry recorded as out-of-scope follow-up.
- **MA-1** ephemeral tx-cookie key broke multi-replica → server-side
  `oidc_login_flows` table; cookie carries only an opaque flow id.
- **MA-2** interactive login spent the JIT budget → `WithInteractiveLogin`
  bypasses the rate limiter (allowlist + claim mapping still enforced).
- **MA-3** session transient path unspecified → `resolveSession` mirrors
  `resolveAPIKey` (DB error → `ErrTransient`, not 401).
- **MI-1** sweeper not reusable → new `StartWebAuthSweeper`.
- **MI-2** logout store access + API-key guard → signature change +
  `spgr_ws_`-only revoke.
- **MI-3** nil `WebAuth` → guarded to `ErrUnauthenticated`.
- **MI-4** missing goose Down → added.
- **MI-5** dead `last_used_at` / no idle TTL → column dropped; absolute TTL only.
- **MI-6** session accumulation / no global logout → documented v1 limitation.
- **MI-7** double verification → accepted (JWKS cached; negligible), noted.
- **MI-8** Strict→Lax global relaxation → documented "no GET mutates state"
  invariant.

### Round 3

- **MAJOR-1** unauthenticated DB-write DoS on `start` (introduced by MA-1) →
  per-IP token-bucket rate limit on `start`/`callback` (429 + `Retry-After`),
  flow TTL shortened to ~5m, sweeper on a ~1m interval; `/providers` is
  in-memory.
- **MINOR-1** residual interactive JIT-creation DoS → documented; email-domain
  allowlist named as the mitigating control; bypass marker is unexported and
  callback-only (no forgery).
- **MINOR-2** provider removed mid-flow → callback maps to `auth_error=exchange`
  (logged); contract row + test added.
- **MINOR-3** flow secrets plaintext-at-rest → documented as deliberate (useless
  without the browser-delivered `code`).
- **MINOR-4** tx cookie not cleared on callback → callback emits a `Max-Age=-1`
  delete.
- **MINOR-5** session-fixation implicit → stated `establishSession` name/path
  invariant so a callback `Set-Cookie` always overwrites a stale cookie.

## Open questions

None outstanding.
