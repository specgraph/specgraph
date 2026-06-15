# CLI OIDC Login Design

**Date:** 2026-06-15
**Status:** Proposed (revised after three adversarial-review passes; pass 3 verdict: ready to implement)
**Part of:** Identity, RBAC & Audit epic. Companion designs: Identity Authn, Identity Storage, Bootstrap & UX.

## Problem

OIDC login exists for the web dashboard but not the CLI. The browser flow
(`internal/server/auth_oidc_handler.go`) ends by minting an opaque `spgr_ws_`
session token, seating it in an **HttpOnly** cookie, and redirecting to `/`. A
CLI cannot read that cookie, so today the only way to authenticate the CLI is a
long-lived API key (`SPECGRAPH_API_KEY` env var or an entry in the credentials
file, resolved in `cmd/specgraph/client.go`).

We want a `gh auth login`-style experience: run `specgraph login`, complete the
existing OIDC flow in the browser, and have the CLI capture a token and store it
in the credentials file (`internal/credentials/`) — whose header already reads
*"Managed by `specgraph login`"*.

A `spgr_ws_` session token already resolves as a valid CLI bearer token (the
resolver accepts it at `internal/auth/identitystore.go`), so the only missing
piece is a server-side affordance to hand the token back to the CLI instead of
seating a cookie.

## Scope

- A top-level `specgraph login` command that performs a **browser-based OIDC
  login via a loopback redirect** against the configured server and writes the
  resulting session token to the credentials file.
- A top-level `specgraph logout` command that revokes the session server-side
  and removes the local credentials entry.
- The minimal server-side changes to broker the flow to a CLI loopback listener.

**Out of scope (v1):**

- **Any remote/headless login path.** SSH / remote-dev / container shells cannot
  use the loopback redirect (the browser runs on a different host). v1 detects
  this and hard-errors with guidance to use `specgraph auth api-key create`. A
  paste-the-code fallback was considered and **rejected** — it is phishable
  (login-CSRF / account takeover, see "Why no paste/manual code" below). A proper
  RFC 8628 device flow is the correct future answer and is deferred.
- Device authorization grant (RFC 8628), refresh-token handling (re-run
  `specgraph login` when the session expires), a separate native/public IdP
  client registration, and any change to the web dashboard flow.

### Why no paste/manual code

A "server displays a one-time code, user pastes it into the CLI" fallback does
**not** match loopback security, even with PKCE. PKCE proves possession of the
verifier but never binds the *authenticating user* to the CLI process. An
attacker can run `specgraph login` on their own machine (keeping verifier `V`,
challenge `S256(V)`), phish the victim with the resulting authorize URL, let the
victim authenticate as themselves (server mints a code bound to the *victim's*
identity and displays it to the victim), then social-engineer the victim into
returning the code and exchange `{code, V}` for a session **for the victim's
account, delivered to the attacker**. The loopback path is immune because the
code is delivered only to an attacker-uncontrolled `127.0.0.1` listener. v1 does
not ship a manual-code path.

## Threat model summary

The IdP only ever sees the **server** callback (`auth.RedirectURI` →
`/api/auth/oidc/callback`, `loginprovider.go:172`); it never validates the
loopback URL. The server itself performs the final 302 to the CLI's
`cli_callback`. Therefore the server-side loopback validation is the **sole**
gate preventing the server from being weaponized as an open redirect that
discloses a bearer-capable artifact. Two independent secrets protect the result,
and crucially the code is only ever delivered to an attacker-uncontrolled
loopback listener:

- **`cli_state`** — CSRF protection on the loopback leg (CLI-verified, constant
  time).
- **`cli_verifier` (PKCE)** — proof-of-possession on the exchange leg
  (server-verified, constant time). The one-time `cli_code` is useless without
  the verifier, which never leaves the CLI process.

These are distinct from, and additional to, the unchanged IdP-leg
`state`/`nonce`/PKCE.

## Architectural shape

Server-brokered loopback with a one-time code **bound by PKCE**. The CLI never
talks to the IdP directly; the server keeps its existing relationship with the
IdP (client secret, PKCE, nonce) and only learns to deliver the result to a
loopback URL when the flow originated from the CLI.

```text
specgraph login
  │ 1. resolveBaseURL() → server + project ; enforce loopback-or-https (see Security)
  │    refuse if SSH/headless/no-browser (see Remote sessions) → point to api-key
  │ 2. GET <server>/api/auth/oidc/providers → pick provider (--provider or prompt)
  │ 3. gen cli_state (CSRF) + cli_verifier (PKCE) ; cli_challenge = S256(cli_verifier)
  │    start loopback listener bound to literal 127.0.0.1:<rand>/callback
  │ 4. open browser → <server>/api/auth/oidc/{provider}/start
  │        ?cli_callback=http://127.0.0.1:<port>/callback
  │        &cli_state=<state>&cli_challenge=<challenge>
  ▼
server handleStart  – validate cli_callback (strict loopback, see Security);
  │                    require cli_challenge ; reject if cli_login_enabled=false (403);
  │                    persist cli_callback + cli_state + cli_challenge on the
  │                    LoginFlow row ; run normal IdP redirect
  ▼ (IdP auth — unchanged: PKCE + nonce, client secret stays server-side)
server handleCallback – code exchange + identity resolve (unchanged) … then:
  │   if the flow carries a cli_callback → DO NOT mint a session / set a cookie.
  │   Instead: gen one-time cli_code (32-byte random), store
  │     {code_hash → user_id, oidc_subject, cli_challenge, TTL 60s},
  │   redirect 302 to <validated cli_callback> with cli_state + code (via url.Values)
  ▼
CLI loopback handler – validate Host header is 127.0.0.1:<port> ; constant-time
  │   compare cli_state ; then POST {code, cli_verifier} → /api/auth/cli/exchange
  ▼
server handleExchange – AuthStore.ExchangeCLICode (single transaction):
  │   SELECT … FOR UPDATE the code (single-use); verify S256(cli_verifier) ==
  │   stored cli_challenge (constant time); INSERT spgr_ws_ session; DELETE code.
  │   Any error rolls back → code stays retryable within TTL. Return {token, expires_at}.
  ▼
CLI – credentials.Upsert(server, {token, label}) + Save(); render browser success page;
      print "Logged in as <subject> on <server>" ; shut down the loopback listener
```

The web path is unchanged: a flow with no `cli_callback` mints the session and
sets the cookie exactly as today.

### Remote sessions (SSH / headless)

Before opening anything, the CLI checks for an environment where the loopback
redirect cannot work: `SSH_CONNECTION`/`SSH_TTY` set, no usable browser, or
`--no-browser` on a non-loopback server. In those cases it does **not** start a
doomed loopback listener; it exits with a clear message: *"Browser-based login
isn't available over SSH/headless sessions. Create an API key instead:
`specgraph auth api-key create`."*

## Resolution / validation semantics

- **`cli_callback` validation (server, `handleStart`).** Parse with `url.Parse`
  and require *all* of: scheme exactly `http`; `u.Hostname()` exactly the string
  `127.0.0.1` or `::1` (literal IPs only — `url.Hostname()` strips the brackets
  from `[::1]`, so the compare is against `::1`; **`localhost` is rejected** to
  avoid resolver/DNS rebinding per RFC 8252 §8.3); no userinfo (`u.User == nil`);
  path exactly `/callback`; empty `RawQuery` and empty `Fragment`. Anything else
  → 400. The 302 `Location` is then built by re-parsing the validated callback
  and setting `cli_state`/`code` via `url.Values` — never by string
  concatenation. The **port is intentionally unvalidated**: the redirect targets
  a port on the *requesting browser's own* `127.0.0.1`, so even a hypothetical
  validation slip discloses the code only to the user's own loopback, and the
  code is useless without the PKCE verifier.
- **`cli_challenge` is required** whenever `cli_callback` is present. A CLI flow
  without a challenge is rejected — there is no unbound-code path.
- **`cli_state`** is an opaque high-entropy CSRF token round-tripped on the
  loopback leg, compared in constant time by the CLI; empty/short values are
  rejected.
- **One-time `cli_code`** is a 32-byte random token (same generator as the web
  session/state tokens, `randToken` at `auth_oidc_handler.go:236`), single-use,
  60s TTL, stored hashed. It maps to the **resolved identity** (`user_id`,
  `oidc_subject`) plus the `cli_challenge` — never to a token, so nothing
  bearer-capable is persisted at rest.
- **Exchange (server)** verifies `S256(cli_verifier) == stored cli_challenge`
  using `crypto/subtle.ConstantTimeCompare` before minting. The `spgr_ws_`
  session is minted only here; only its SHA-256 hash is stored, exactly as in the
  web callback.
- A flow that resolves identity but is never exchanged (user closes the browser)
  leaves only a short-lived code row that expires and is swept; no session is
  created.

## Storage

`internal/storage/` domain types and the postgres implementation
(`internal/storage/postgres/`), behind the existing `WebAuthStore` interface,
implemented by `*postgres.AuthStore`.

- Extend `LoginFlow` with `CLICallback`, `CLIState`, and `CLIChallenge` columns,
  declared **`NOT NULL DEFAULT ''`** (not nullable). The domain `LoginFlow`
  fields stay `string` (`web_auth_domain.go`), and `ConsumeLoginFlow` scans into
  them (`web_auth.go:116`); pgx v5 cannot scan SQL `NULL` into a Go `string`, so
  nullable columns would regress **every** web login. `NOT NULL DEFAULT ''`
  matches the "web flow leaves them empty" intent and keeps the scan working.
  `CreateLoginFlow`'s INSERT and `ConsumeLoginFlow`'s `RETURNING`/`Scan` are both
  extended for the three columns.
- New goose migration: `cli_login_codes(code_hash bytea pk, user_id, oidc_subject,
  cli_challenge text, expires_at, created_at)`. Codes stored hashed; single-use
  via the exchange transaction.
- New `WebAuthStore` methods. To match the existing session convention
  (`CreateSession` takes a pre-hashed `TokenHash` — `web_auth.go:20`), the
  **caller hashes**; the raw code never crosses the storage boundary. The PKCE
  comparison is passed as a **precomputed challenge string** (the server computes
  `S256(cli_verifier)` and passes the result), not a behavioral closure — this
  keeps the storage boundary to domain values only (per the constitution) and
  keeps S256 in the server layer:
  - `CreateCLICode(ctx, codeHash []byte, userID, subject, challenge string, expiresAt time.Time) error`
  - `ExchangeCLICode(ctx, codeHash []byte, sess *Session, gotChallenge string) (*Session, error)`
    — returns the minted session (incl. `OIDCSubject`); see Atomic exchange.
  - `DeleteExpiredCLICodes(ctx) (int64, error)` — for the existing sweep loop.

### Atomic exchange (ADR-004)

`AuthStore` does **not** participate in `*Store.RunInTransaction` —
`RunInTransaction` is defined on `*Store` (`tx.go:42`) and threads a `pgx.Tx`
through context, but `AuthStore` runs statements directly against its own pool
(`web_auth.go:37…`). The two are distinct objects (`serve.go:142` vs `:218`), so
a `*Store` transaction cannot make `AuthStore` calls atomic.

Therefore the consume-then-mint write path is encapsulated in a single
`AuthStore.ExchangeCLICode` method that opens **its own** `pool.Begin`
transaction (`AuthStore` holds a `*pgxpool.Pool`, `auth.go:48`) and, in one
round-trip-bounded unit, runs all statements **directly on the `pgx.Tx`** — it
must NOT call the existing `CreateSession` (that method runs on `s.pool`, outside
the new tx, and would silently defeat atomicity). `AuthStore` has no
tx-from-context routing like `*Store`, so the SQL is inlined on the tx handle:

1. `SELECT user_id, oidc_subject, cli_challenge FROM cli_login_codes
   WHERE code_hash = $1 AND expires_at > now() FOR UPDATE`
   → `ErrCLICodeNotFound` if absent/expired (also enforces single-use under the
   row lock; READ COMMITTED makes a concurrent exchange block then re-read the
   DELETEd row).
2. `subtle.ConstantTimeCompare([]byte(gotChallenge), []byte(storedChallenge))` —
   the `gotChallenge` argument is `S256(cli_verifier)` precomputed by the handler,
   so no crypto runs in storage. A mismatch rolls back (the code is **not**
   burned — it stays retryable for the rest of its TTL; see brute-force note).
3. `INSERT INTO web_sessions (… SELECT … WHERE EXISTS user not soft-deleted)
   RETURNING …` → `ErrUserNotFound` if the user was deleted mid-flow.
4. `DELETE FROM cli_login_codes WHERE code_hash = $1`.
5. Commit. The returned `*Session` carries `OIDCSubject` for the handler's
   response.

Any error rolls back the whole unit: a transient `INSERT` failure leaves the
code un-consumed and retryable within its TTL; a terminal `ErrUserNotFound`
propagates so the handler can return a terminal status (see Error handling).

**Brute-force barrier.** Because a wrong verifier does not consume the code, the
verifier's only protections are its entropy (256-bit, `oauth2.GenerateVerifier`),
the 60s code TTL, and the per-IP rate limiter. That margin is ample; burning the
code on mismatch would be marginally stricter but is not required.

## Server changes

`internal/server/auth_oidc_handler.go`:

- `handleStart`: read optional `cli_callback`, `cli_state`, `cli_challenge` query
  params; apply the strict validation above; if `cli_login_enabled=false` and a
  `cli_callback` is present, **fail fast with 403 `cli_login_disabled`** (do NOT
  silently degrade to the web flow); otherwise persist all three on the
  `LoginFlow`.
- `handleCallback`: after identity resolution, branch on the flow's
  `cli_callback`. The loopback branch **re-runs the strict `cli_callback`
  validation** (defense-in-depth — never trust the stored value into a redirect),
  mints a one-time code, and 302s to the validated loopback with `cli_state` +
  `code` built via `url.Values`; the web branch (no `cli_callback`) is untouched.
- New `POST /api/auth/cli/exchange`: body `{code, cli_verifier}`. Hashes the code,
  precomputes `S256(cli_verifier)`, calls `AuthStore.ExchangeCLICode`, and returns
  JSON `{token, expires_at, oidc_subject}` — `oidc_subject` (carried on the code
  row and the returned session) lets the CLI write the `oidc:<subject>`
  credentials label and print "Logged in as <subject>" without an extra round
  trip. Public endpoint, per-IP rate limited via the existing
  `NewIPRateLimiterForOIDC` limiter. When `cli_login_enabled=false` the route is
  not registered (404). Response carries `Cache-Control: no-store`.

`internal/server/auth_handler.go`:

- Extend `handleLogout` to also revoke when the token arrives via an
  `Authorization: Bearer` header — but only when it carries the `spgr_ws_` prefix
  (mirror the cookie path's guard at `auth_handler.go:92`), so an API key sent as
  bearer is never hashed and looked up as a session. `RevokeSession` is already
  idempotent for expired/revoked tokens (`web_auth.go:70`).

`cmd/specgraph/serve.go`: wire the new `/api/auth/cli/exchange` route into the
existing `RegisterOIDCLoginHandlers` config (conditional on `cli_login_enabled`).

### Config gate

Add `auth.oidc.cli_login_enabled` as a plain `bool` defaulted to `true` in
`globalDefaults()` (the same pattern as `Log.Requests`; koanf merges only
file-present keys over defaults, so an absent key keeps `true` and an explicit
`false` wins — no `*bool` ambiguity needed). It is wired onto `OIDCLoginConfig`
and read in `handleStart` / route registration. When `false`: `handleStart`
rejects CLI flows with 403, the exchange route is not registered, and the CLI
surfaces "CLI login is disabled on this server; use `specgraph auth api-key
create`." (`RegisterOIDCLoginHandlers` already no-ops when zero providers exist,
so a static `true` default is harmless with no providers.)

## CLI changes

`cmd/specgraph/`:

- New `login.go` — top-level `specgraph login`:
  - Flags: `--provider` (skip the picker), `--server` (override resolved base
    URL), `--no-browser` (print the loopback authorize URL instead of opening a
    browser; still loopback-bound, for local-but-no-default-browser cases).
  - **Remote/headless guard:** if `SSH_CONNECTION`/`SSH_TTY` is set or no usable
    browser is found (and not a local `--no-browser`), hard-error to api-key
    guidance (see "Remote sessions").
  - **HTTPS guard:** refuse to run the exchange (and to write creds) when the
    server base URL is non-loopback and not `https://`. "Loopback" here uses the
    same **strict literal-IP** check as `cli_callback` (127.0.0.1 / ::1), shared
    as a small helper — **not** the existing `isLoopbackAddr` (`serve.go:655`),
    which accepts `localhost` and all of `127.0.0.0/8` and would silently weaken
    the guard.
  - Generates `cli_state` + PKCE `cli_verifier`/`cli_challenge`, starts a loopback
    listener bound to literal `127.0.0.1`, opens the browser, validates the
    callback (`Host` header + constant-time `cli_state`), exchanges
    `{code, cli_verifier}`, writes the token via `credentials.Upsert` + `Save`,
    then shuts down the listener (and on a hard timeout).
  - **Non-TTY + multiple providers + no `--provider`:** hard-error with guidance
    rather than blocking on an unanswerable prompt.
  - **Credentials write:** label set to `oidc:<subject>` (subject from the
    exchange response). To decide whether to warn about overwriting, read the
    existing entry via `credentials.Load` + `TokenFor` directly — **not**
    `resolveAPIKey` (`client.go:102`), which masks the file entry behind
    `SPECGRAPH_API_KEY` and would misfire the warning. If the existing entry is
    **not** a session token (e.g. a hand-placed `spgr_sk_` API key), warn before
    overwriting.
  - On success, prints "Logged in as <subject> on <server>" using the subject
    from the exchange response (no extra `Whoami` round trip required).
- New `logout.go` — top-level `specgraph logout`:
  - Reads the stored token. If it is a `spgr_ws_` session, POSTs it as a bearer
    to `/api/auth/logout` to revoke server-side. Then removes the credentials
    entry for the server (preserving other servers). If the stored token is an
    API key, only the local entry is removed (with a warning) and no revoke is
    attempted.
- New `internal/browser` package — cross-platform opener (`open` on darwin,
  `xdg-open` on linux, `rundll32 url.dll,FileProtocolHandler` on windows) with a
  clean fallback to printing the URL.

## Concurrency

Concurrent `specgraph login` invocations share the browser's single cookie jar,
and the existing `tx` cookie has a fixed name scoped to `/api/auth/oidc`
(`auth_oidc_handler.go:24,213`); a second `handleStart` would overwrite the
first's cookie. v1 treats CLI login as **one-at-a-time**: the CLI listener
enforces a hard timeout (default 3 min) and exits with a clear "login timed out,
please retry" rather than hanging. A per-flow `tx` cookie name (suffixed with the
flow id) is noted as the robust follow-up but is out of scope for v1.

## Error handling

- `cli_callback`/`cli_challenge` validation failure → server 400; CLI surfaces a
  clear message.
- `cli_login_enabled=false` → `handleStart` 403 `cli_login_disabled`; CLI prints
  api-key guidance (no hang).
- `cli_state` mismatch or bad `Host` header on the loopback leg → CLI aborts
  without writing creds.
- Expired/replayed `cli_code` or PKCE mismatch → exchange returns 400; CLI
  reports "login expired, please retry".
- User soft-deleted between callback and exchange (`ErrUserNotFound` from the
  mint) → **terminal** 403 `account_unavailable` with a distinct message; the CLI
  does **not** retry. (The un-consumed code simply expires.)
- Transient backend failure during exchange → 503; the transaction rolled back,
  so the code is NOT consumed and the CLI may retry within the TTL.
- Browser cannot be opened on a local session → fall back to printing the URL.
- Remote/SSH/headless → hard-error to api-key guidance before opening anything.
- Non-loopback server over plain `http://` → CLI refuses with the HTTPS-guard
  message before opening anything.
- Server has no interactive OIDC providers (or CLI login disabled) → CLI exits
  with a message pointing to `specgraph auth api-key create`.

## Testing

- **Server unit:** `cli_callback` validation table (literal v4 `127.0.0.1` and v6
  `::1` pass; `localhost`, `127.0.0.1.evil.com`, `user@127.0.0.1`, `https`, wrong
  path, non-empty query/fragment, missing challenge all rejected); CLI-loopback
  branch in `handleCallback` builds the redirect via `url.Values`;
  `cli_login_enabled=false` → 403 and no exchange route; `ExchangeCLICode`
  happy/expired/replayed/PKCE-mismatch/`ErrUserNotFound` paths; rollback leaves
  the code retryable on transient mint failure; logout-via-bearer revocation only
  for `spgr_ws_` prefixes.
- **CLI unit:** loopback handler `cli_state` + `Host` validation; PKCE
  verifier/challenge generation; credentials write + label + overwrite warning;
  HTTPS guard; SSH/headless → hard-error; non-TTY multi-provider hard-error;
  listener timeout; logout removes the entry and revokes only sessions.
- **Storage integration (postgres/testcontainers):** `ExchangeCLICode` atomicity
  — concurrent exchange of the same code yields exactly one session (FOR UPDATE);
  expired code rejected; `ErrUserNotFound` rolls back and leaves no session;
  `LoginFlow` CLI fields round-trip; **a web-flow login still succeeds with the
  new `NOT NULL DEFAULT ''` columns** (regression guard against the nullable-scan
  trap); sweep deletes expired codes.
- **Regression:** assert the access log records only `r.URL.Path`, never the
  query string (so `cli_code`/`cli_state` never hit server logs), and the
  outbound 302 `Location` to the loopback is not logged by any middleware.
- **Wiring:** `serve.go` registers the exchange route iff enabled and it is
  reachable.

## Trade-offs

- **No remote/headless login in v1.** SSH/remote users use API keys; a secure
  remote path (RFC 8628 device flow) is deferred. This is a deliberate cut to
  avoid shipping a phishable paste flow.
- Session lifetime is bounded by `auth.oidc.session_ttl`; there is no refresh, so
  the CLI re-authenticates with `specgraph login` when the token expires. Matches
  the web session's posture.
- The server brokers the flow, so a separate public/native IdP client is not
  required and the client secret never reaches the CLI. The cost is one new
  endpoint, one branch, one small table, and a dedicated transactional
  `ExchangeCLICode` method.
- One-login-at-a-time (concurrency) is an accepted v1 limitation with a hard
  timeout.

## Appendix: adversarial review disposition

Two review passes (2026-06-15) raised twelve findings. Disposition:

**Pass 1**

- **#1 No proof-of-possession (Critical)** — fixed: CLI↔server PKCE bound to the
  code, verified at exchange.
- **#2 Loopback validation under-specified (Critical)** — fixed: strict
  `url.Parse` validation, literal IPs only, `localhost` rejected, scheme/path
  pinned, no userinfo, query/fragment rejected; plus `cli_login_enabled` gate.
- **#3 SSH/headless breaks (High)** — addressed by **cutting** remote login from
  v1 with a clear hard-error (paste fallback rejected, see #C1).
- **#4 No transaction on consume+mint (High)** — fixed: dedicated transactional
  `AuthStore.ExchangeCLICode` (see "Atomic exchange").
- **#5 Hashing inconsistency (Medium)** — fixed: caller hashes; store takes
  `codeHash []byte`.
- **#6 Concurrent-login cookie collision (Medium)** — accepted v1 limitation:
  one-at-a-time with a hard timeout; per-flow cookie noted as follow-up.
- **#7 No HTTPS enforcement (Medium)** — fixed: CLI HTTPS guard (literal-IP
  loopback definition).
- **#8 Loopback listener hardening (Medium)** — fixed: literal `127.0.0.1` bind,
  `Host` check, constant-time `cli_state`, shutdown after first hit / timeout.
- **#9 Rate-limit NAT + code entropy (Low)** — code entropy stated (32-byte
  random); shared per-IP limiter accepted as-is for v1.
- **#10 Non-TTY multi-provider (Low)** — fixed: hard-error with guidance.

**Pass 2**

- **#C1 Paste flow phishable / account takeover (Critical)** — fixed by
  **removing** the paste path entirely; v1 is loopback-only (see "Why no
  paste/manual code").
- **#H2 `RunInTransaction` not implemented on `AuthStore` (High)** — fixed:
  `ExchangeCLICode` opens its own `pool.Begin` transaction; the prior
  cross-`Store` plan is dropped (see "Atomic exchange").
- **#M3 Soft-deleted user mid-flow (Medium)** — fixed: terminal 403
  `account_unavailable`, no retry.
- **#M4 `[::1]` vs `::1` (Medium)** — fixed: compare `u.Hostname()` to `::1`
  (brackets stripped); added to the test table.
- **#M5 `cli_login_enabled=false` silent degradation (Medium)** — fixed:
  fail-fast 403 and unregistered exchange route, not a degraded web flow.
- **#M6 Paste page cache/referrer (Medium)** — moot (paste removed); the exchange
  response carries `Cache-Control: no-store`.
- **#L7 Query/fragment on `cli_callback` (Low)** — fixed: rejected; redirect built
  via `url.Values`.
- **#L8 Constant-time PKCE compare (Low)** — fixed: `subtle.ConstantTimeCompare`.
- **#L9 Logout bearer prefix guard (Low)** — fixed: revoke only `spgr_ws_`
  bearers.
- **#L10 HTTPS-guard `localhost` inconsistency (Low)** — fixed: literal-IP
  definition shared with `cli_callback`.
- **#L11 Credentials overwrite/label (Low)** — fixed: label `oidc:<subject>`;
  warn before overwriting a non-session entry.

**Pass 3** (verdict: ready for writing-plans, no security blocker)

- **#N1 Nullable CLI columns would regress every web login (Medium)** — fixed:
  columns are `NOT NULL DEFAULT ''` so pgx scans into the `string` domain fields;
  added a web-login regression test.
- **#4 Subject not in exchange response (Medium)** — fixed: exchange returns
  `oidc_subject`, so the `oidc:<subject>` label and success line need no extra
  round trip; resolves the prior `{token, expires_at}` inconsistency.
- **`ExchangeCLICode` must inline SQL, not call `CreateSession` (caveat)** — made
  explicit: `CreateSession` runs on the pool, outside the tx, and would defeat
  atomicity; the exchange runs all statements on the `pgx.Tx`.
- **#N2 `verify` closure crossed the storage boundary (Low)** — fixed: pass a
  precomputed `gotChallenge string`; storage does only a constant-time compare.
- **#N3 Code not burned on PKCE mismatch (Low)** — documented: brute-force
  resistance rests on verifier entropy + 60s TTL + rate limiter.
- **#N4 Re-validate `cli_callback` at callback time (Low)** — added as
  defense-in-depth before building the redirect.
- **Strict-loopback helper, not `isLoopbackAddr` (Low)** — called out: the HTTPS
  guard shares the literal 127.0.0.1/::1 check, not `serve.go`'s permissive one.
- **`cli_login_enabled` default (Low)** — pinned: plain `bool` defaulted `true`
  in `globalDefaults()`, mirroring `Log.Requests`; no `*bool`.
