# Non-fatal Postgres at startup (`docker: false`)

**Date:** 2026-06-08
**Status:** Design approved (revised after adversarial review), pending spec review

## Problem

When `server.docker: false`, SpecGraph connects to an **external** Postgres
at `cfg.Server.Postgres.URL`. If that database is unreachable at boot, startup
is **hard-fatal**: `postgres.New` eagerly pings the DB, runs migrations, and
inserts a project row (`internal/storage/postgres/postgres.go:74-94`). Any
failure returns an error that propagates through `runServe`
(`cmd/specgraph/serve.go:127-129`) to `main.go`, which calls `os.Exit(1)`.

The HTTP API and the `/readyz` probe listener never start, so there is no way
for the process to come up in a degraded/not-ready state and surface "Postgres
down" — the existing probe-based readiness only covers outages that occur
*after* a successful boot.

We want the server to **start anyway**, report **not-ready**, and **recover
without a restart** once Postgres becomes reachable — without papering over a
genuine misconfiguration (bad credentials).

## Goals

- `server.docker: false` + an unreachable external PG at boot is **not fatal**;
  the process stays alive and self-heals when PG returns, with **no restart**.
- The server reports **not-ready** while PG is down (HTTP `503` on the main
  port, `/readyz` `503`) and **ready** once connected.
- A genuine misconfiguration (bad credentials) still **fails loudly** after a
  bounded number of retries, rather than masking the error forever.

## Non-goals

- Changing the `docker: true` path. When SpecGraph starts its own container, an
  unreachable PG after `compose up --wait` remains a hard error (fast-fail).
- Re-degrading to blanket `503` if PG dies *after* the server became ready (see
  "Runtime PG loss" below) — that stays as today's per-RPC errors + `/readyz`
  `503`.
- Per-RPC degraded behavior, a DB-aware `Health` RPC, auth-interceptor changes,
  a delegating store wrapper, or a new storage sentinel error. The blanket `503`
  - `/readyz` cover the degraded signal, so none of these are needed.

## Behavior contract

| Mode | First connect result | Outcome |
|------|----------------------|---------|
| `docker: true` | success | Normal boot (unchanged). |
| `docker: true` | failure | **Fatal** — return error, `os.Exit(1)` (unchanged). |
| `docker: false` | success | Normal boot, **no degraded window** (real handler from the start). |
| `docker: false` | transient/other failure | **Non-fatal degraded boot** (below). |
| `docker: false` | credential failure ×5 | **Fatal** — loud exit (see error policy). |

### Degraded boot (`docker: false`, first connect failed, non-credential)

Both listeners open:

- **Probe port** (`server.probes.listen`, if configured): `/livez` → `200`
  always; `/readyz` → `503` until connected, then `200`.
- **Main API/MCP port** (`server.listen`): open, serving a **blanket `503`**
  ("storage not ready", with a `Retry-After` header) for *every* request until
  PG connects.

A background connector retries the **connect unit** (§3) with backoff. On the
first success it prints the one-time bootstrap-admin banner if a backstop admin
was created, runs the post-connect wiring once, swaps the real handler into the
main server atomically, sets the probe pinger's store, and starts the claim
sweeper. `/readyz` flips to `200` on the next probe. **No restart is required.**

If `server.probes.listen` is empty (probes off), the process still stays alive
(the core requirement); a single WARN line is logged noting that readiness is
not observable over a probe port in this configuration.

## Error policy (the connect retry loop)

Each attempt runs only the **connect unit** — the pool-touching prefix
(`postgres.New → Subscribe → NewAuth → bootstrap.Ensure`, Architecture §3). The
non-DB wiring runs **once** after a successful connect and is never retried (see
"Failure surfaces" below). Connect-unit failures are classified:

- **Credential/auth errors** — a pgx `*pgconn.PgError` with SQLSTATE in the
  `28xxx` class (`28P01` invalid_password, `28000` invalid_authorization),
  surfaced by `postgres.New` at `pool.Ping` time and wrapped with `%w` so
  `errors.As` recovers it. These indicate misconfiguration, not a transient
  outage:
  - Retry at a **fixed 1s interval, at most 5 consecutive attempts**, then
    **fail loudly and fatally** (return the error → `os.Exit(1)`), even under
    `docker: false`.
  - A non-credential attempt resets the consecutive-credential counter.
- **All other errors** — connection-refused, dial timeout, `Ping` failure,
  migration failure, `ensureProjectRow` failure, `NewAuth` (auth-migration)
  failure, `bootstrap.Ensure` failure:
  - Retry **forever**, exponential backoff starting at 1s and **capped at 1m**.
  - Logged **very loudly** (sustained `WARN`/`ERROR`, never demoted to `DEBUG`),
    so a stuck server (e.g. a permanently broken migration) is always visible in
    logs. The server stays alive at `503` and self-heals if the condition clears.

### Failure surfaces that are NOT retried (fatal)

These are **not** in the connect unit and must fail fast/loud rather than loop:

- **PG-independent construction** built up front — OIDC verifier, Cedar engine,
  embedded skills, web FS, CORS flag. Fatal at startup exactly as today.
- **Post-connect non-DB wiring** — `usagetracker.NewManager`,
  `auth.NewIdentityStore` (pure deterministic config validation: duplicate
  issuer, JIT role/claims typos), `NewAuthInterceptor`, `NewMux` +
  `Register*`, MCP, static. These consume only references to the now-live
  store/authStore; they do no DB I/O and their failures are deterministic
  config/build errors. They run **once** after a successful connect and are
  **fatal** — on the happy path via `return err`; in the degraded path the
  background goroutine signals the main path via the server error channel to
  exit non-zero. They are never folded into the infinite retry loop.

## Architecture

A refactor of `cmd/specgraph/serve.go` plus small new helpers. No changes to
handlers, interceptors, the storage layer, the auth interceptor, or proto.

### 1. Swappable main handler

The main `http.Server.Handler` is a **fixed shim** that, per request, loads the
current handler from an `atomic.Pointer[http.Handler]` and delegates to it.
`srv.Handler` itself is never reassigned (assigning it from another goroutine
would be a data race); only the atomic pointer is swapped.

- **Default** (degraded path only) = a blanket-503 handler responding
  `503 Service Unavailable`, body `storage not ready`, with a `Retry-After`
  header, to every request.
- **After connect** = the fully-wired handler
  (`SecurityHeaders(ProjectMiddleware(mux))`, plus optional CORS).

On the happy path the atomic pointer holds the real handler *before*
`ListenAndServe` starts, so there is no `503` window.

### 2. Readiness-aware probe pinger

`startProbeListener` takes a `probes.Pinger`. Introduce `readinessPinger`:

- Holds an `atomic.Pointer[postgres.Store]`.
- `Ping(ctx)`: returns a "not ready" error while the pointer is nil; otherwise
  delegates to the live store's `Ping`.

To avoid spurious "readiness probe failed / recovered" log noise on healthy
boots (`probes.go` runs its first probe immediately), the pinger's store is
**set before the probe listener starts on the happy path**. In the degraded
path the store is legitimately nil, so the first-attempt WARN and the later
"recovered" INFO are accurate.

### 3. Connect unit (the retry unit) and post-connect wiring

Split the PG-dependent work into two helpers so retries touch only what must be
retried:

**`connectStore(ctx, connURL) (*postgres.Store, *AuthStore, bootstrap.Result, error)`**
— the retry unit. In order:

1. `postgres.New(ctx, connURL, WithProject("_server"))` — ping + migrations +
   project row. Returns the concrete `*postgres.Store` (needed for `s.Pool()`).
2. `store.Subscribe(notify.NewImpactLogger())`.
3. `NewAuth(ctx, s.Pool())` — runs auth migrations.
4. `bootstrap.Ensure(ctx, authStore, …)` — returns `Created`/`Token` for the
   banner. This is the **last** step in the unit, so nothing fallible runs after
   the token is minted within an attempt.

**Partial-failure cleanup (required):** if step 1 succeeds but step 3 or 4 fails,
`connectStore` MUST `Close` the pool it created before returning the error —
otherwise each retry orphans a `pgxpool.Pool` (and its connections/fds),
eventually exhausting the database. Implement as a deferred rollback inside the
helper, disarmed only on full success. (`postgres.New` already self-cleans its
own internal failures at `postgres.go:80,85,92`; this covers the steps *after*
`New` returns.) No `usagetracker` goroutine is started inside the unit, so there
is nothing else to leak.

**Post-connect wiring** runs **once**, after `connectStore` succeeds, and is
fatal on error (see "Failure surfaces"): build `usagetracker.NewManager`,
`NewIdentityStore`, `NewAuthInterceptor`, `NewMux` + all `Register*Service` +
API/auth handlers + MCP endpoint + static handler, then assemble the top-level
`http.Handler`. These hold references to the live store/authStore and do no DB
I/O, so they need run only once. The bootstrap banner is printed here
**immediately**, before any of this fallible wiring, whenever `Created==true`,
so the one-time token can never be stranded by a later failure.

Re-running `connectStore` on each retry is safe: goose migrations,
`bootstrap.Ensure`, and `ensureProjectRow` are all idempotent
(`ON CONFLICT` / version-guarded), and the partial pool is closed between
attempts. The connect unit corresponds to today's `serve.go:127-175`; the
post-connect wiring corresponds to `serve.go:177-340`.

The helpers thread the **concrete** `*postgres.Store` (for `s.Pool()` and the
probe pinger) and `*AuthStore`, while the mux continues to consume the narrower
`backendStore` interface.

### 4. Boot sequence (revised `runServe`)

1. Load config, init logging, signal-cancellable `ctx` (unchanged).
2. `docker: true` only: ensure + `compose up` the container; `defer compose
   stop` (unchanged).
3. Validate `connURL` non-empty (unchanged).
4. Build PG-**independent** deps (OIDC verifiers, Cedar engine/authorizer,
   skills source, web FS, CORS) — **fatal on failure**, as today.
5. **Decide the path with a synchronous first phase** (no server or shutdown
   goroutine has started yet, so a failure here starts nothing):
   - Call `connectStore`. On a **credential** error, retry at 1s up to 5
     consecutive attempts (still synchronous). On **success** → happy path
     below. On the 5th credential failure → `return err` (fatal). On an
     **other** error → degraded path below.
6. **Happy path** (`connectStore` succeeded): print the bootstrap banner if
   `Created`; run the post-connect wiring (§3, fatal on error); set the atomic
   handler to the real handler and the pinger's store; start the probe listener
   (pinger already has the store → no spurious WARN) and the main server (real
   handler from the start); start the claim sweeper; install store/auth/tracker
   closers and the shutdown goroutine. **No degraded window.** For
   `docker: true` this is the only non-fatal outcome and matches today exactly.
7. **Degraded path** (`docker: false`, other-class failure): set the atomic
   handler to blanket-503; start the probe listener with the nil-store pinger;
   start the main server (serving 503); install the shutdown goroutine (store
   closers added later, on connect). Spawn the **background connector
   goroutine** that loops `connectStore` under the §Error-policy. On its first
   success it prints the banner (if `Created`), runs the post-connect wiring (a
   fatal error here is sent to the server error channel → exit non-zero), swaps
   in the real handler, sets the pinger store, registers store closers, and
   starts the sweeper.
8. Block on shutdown signal / server error channels; graceful drain of both
   servers (existing logic, `serve.go:360-393`).

For `docker: true` + connect failure, step 5 returns before step 6/7 — **no
probe server, no main server, and no shutdown goroutine are ever started**, so
the path is byte-for-byte today's behavior (no extra "server shutting down" log
line).

### 5. Shutdown ordering and nil-safety

The shutdown goroutine is installed in step 6/7 (never before a server starts).
Because the store may be nil during the degraded window before first connect:

- the shutdown path **nil-checks** the store before `Close` and the sweeper
  before stopping;
- store/auth/tracker closers are **registered on successful connect** (happy
  path inline; degraded path inside the connector goroutine), into a small
  shutdown registry the teardown consults — not via top-of-function `defer`s
  that assume a live store.

Teardown ordering preserves today's LIFO contract (`serve.go:133-141`,
`auth.go:34-36`): registered closers run **after** the main server has finished
draining (`srv.Shutdown` completes / `ListenAndServe` returns), in the order
**sweeper-stop → tracker.Close → authStore.Close → store.Close**, each
nil-guarded. Closing the store before the drain completes would race in-flight
RPCs against a closed pool and is explicitly disallowed.

Shutdown during the degraded retry loop (no store yet) drains the running
servers and exits cleanly (exit 0). The background connector respects `ctx`
cancellation and returns promptly.

## Runtime PG loss (post-ready, out of scope to change)

Once the real handler is swapped in, it stays swapped in. If PG later dies,
`/readyz` flips to `503` (the pinger sees it) but the main port keeps serving
the real handler, whose RPCs return their natural per-RPC errors — identical to
today's behavior. The boot-time blanket-503 is **not** re-applied at runtime;
this asymmetry (boot-down = blanket 503; runtime-down = per-RPC errors) is
intentional and documented here so it isn't mistaken for a bug.

## Testing

### Unit

- `readinessPinger`: `Ping` returns not-ready before the store is set; delegates
  after.
- blanket-503 handler: returns `503` with the expected body and `Retry-After`
  for arbitrary requests; the atomic shim delegates to whichever handler is
  currently stored.
- error classification: a `*pgconn.PgError{Code:"28P01"}` is treated as
  credential class; a dial error / generic error / migration error is treated as
  "other". The 5×1s-then-fatal credential rule and the 1s→1m-cap infinite
  backoff for "other" are exercised via an injectable connect func.
- backoff loop honors `ctx` cancellation (returns promptly, no store).
- partial-failure cleanup: a `connectStore` where `postgres.New` succeeds but a
  later step fails closes the pool before returning (assert via a fake that the
  created pool's `Close` is called on every error path — no leak across N
  retries).

### Integration (testcontainers, `//go:build integration`)

- **Degraded → recovery:** reserve an ephemeral TCP port, **release** it, and
  point `connURL` at that fixed host:port (now connection-refused → "other"
  class). Boot `docker: false`; assert the main port returns `503`, `/readyz`
  `503`, `/livez` `200`. Then start a Postgres testcontainer bound to that
  **exact fixed host port** (testcontainers fixed-host-port mapping, not the
  default random port, since `connURL` is fixed before boot). Within the backoff
  window assert an RPC succeeds and `/readyz` `200`, and that the process never
  exited/restarted. (Accept the inherent reserve→release TOCTOU window.)
- **Credential fatal:** boot `docker: false` against a live PG with a wrong
  password; assert the process exits non-zero after ~5 attempts with a loud
  error (not a silent 503).
- **No token loss:** force a post-connect wiring failure on the first ready
  attempt (e.g. injected `NewIdentityStore` error) and assert the bootstrap
  banner/token was already printed before the fatal exit.

### Regression

- `docker: true` with an unreachable PG still returns an error and exits
  non-zero (no main/probe server started).
- `docker: false` with PG already up boots with **no** degraded window (real
  handler from the start; `/readyz` `200`; no spurious probe WARN).

## Files touched (anticipated)

- `cmd/specgraph/serve.go` — boot-sequence refactor, atomic-handler shim,
  blanket-503 handler, `readinessPinger`, error classifier + backoff connector,
  `connectStore` retry unit (with partial-pool cleanup) + once-after-connect
  post-connect wiring, nil-safe shutdown registry.
- New unit tests alongside `serve.go` (e.g. `serve_test.go`).
- New integration test gated by `//go:build integration`
  (degraded→recovery, credential-fatal, no-token-loss cases).

No proto, storage-interface, handler, auth-interceptor, or config-schema
changes.

## Resolved review findings

### Round 1

- Happy-path "no degraded window" is preserved by **branching at the first
  connect result** (real handler before `ListenAndServe`), rather than serving
  503 unconditionally.
- Errors are **classified** (credential vs. other) with distinct policies, so
  bad credentials fail loudly (5×1s → fatal) and never retry forever silently.
- `docker: true` ordering is genuinely unchanged (it returns before any server
  or shutdown goroutine starts on connect failure).
- The atomic-handler indirection shim is named explicitly (no `srv.Handler`
  reassignment race).
- Probe pinger store is set before the probe listener on the happy path, killing
  spurious "probe failed/recovered" logs.
- Runtime PG-loss asymmetry is documented as intentional/out-of-scope.

### Round 2

- **Resource leak:** the retry unit is shrunk to the pool-touching prefix only,
  and `connectStore` **closes the partial pool on every post-`New` error path**,
  so persistent post-`New` failures no longer orphan a pool per retry. No
  `usagetracker` goroutine is created inside the unit.
- **Bootstrap token loss:** `bootstrap.Ensure` is the **last** step of the
  connect unit, and the banner is printed **immediately** on a successful
  connect, before any fallible post-connect wiring — the one-time token can
  never be stranded.
- **Deterministic config errors retried forever:** `NewIdentityStore` and the
  rest of the non-DB wiring are moved **out of the retry loop**; they run once
  after connect and are **fatal** (happy path returns; degraded path signals the
  server error channel to exit).
- **`docker: true` extra teardown log:** the shutdown goroutine is now installed
  only after a server starts, so the fatal `docker:true` path starts nothing.
- **Shutdown ordering:** explicit post-drain order (sweeper → tracker → auth →
  store), nil-guarded, replacing the old LIFO `defer` chain.
- **Integration test phrasing:** restated as reserve→release→refused-boot→bind
  PG to the fixed host port; added the no-token-loss case.

## Open questions

None outstanding.
