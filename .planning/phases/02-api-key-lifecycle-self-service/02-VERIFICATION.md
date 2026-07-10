---
phase: 02-api-key-lifecycle-self-service
verified: 2026-07-10T02:30:24Z
status: human_needed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  # initial verification — no prior VERIFICATION.md existed
requirements:
  - id: AUTH-02
    status: satisfied
  - id: AUTH-03
    status: satisfied
human_verification:
  - test: "Start the SpecGraph server + web dev build, log in via OIDC/spgr_ws_ session, open /keys, then create → rotate → revoke a key."
    expected: "The dashboard lists the caller's own keys; create/rotate open the reveal modal showing the plaintext exactly once; after closing the modal the secret is unrecoverable (no re-fetch); revoke removes the active key and the list refreshes."
    why_human: "Interactive browser behavior — one-time-reveal irrecoverability, clipboard copy, and visual eligibility messaging — cannot be asserted headlessly; requires a running server and human visual/functional confirmation. (Plan 02-08 D5 — intentionally deferred; automated vitest 8/8 + web build already pass.)"
  - test: "On the live /keys page, strip/blank the specgraph_csrf cookie (or the echoed X-CSRF-Token header) and attempt a create/rotate/revoke mutation."
    expected: "The mutation is rejected with HTTP 403 (invalid or missing CSRF token); a normal mutation with the cookie present succeeds."
    why_human: "Requires a running server + browser to exercise the double-submit cookie/header round-trip end-to-end. (csrfValidate unit tests already pass in isolation.)"
  - test: "Log in with a session whose Source is an api key (legacy SPECGRAPH_API_KEY-style session) and attempt a self-mint from the dashboard."
    expected: "The anti-key-chaining gate denies the mint and the panel renders a readable 'sign in to provision a key' message rather than a raw error."
    why_human: "Requires a running server to produce a Source==\"apikey\" identity and confirm the user-facing message rendering."
---

# Phase 2: API Key Lifecycle & Self-Service — Verification Report

**Phase Goal:** OIDC users can safely self-provision scoped MCP API keys, and a revoked app-role can no longer be exploited via an already-issued key.
**Verified:** 2026-07-10T02:30:24Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria — the contract)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | An authenticated OIDC user can create, list, rotate, and revoke their own role-capped, expiring MCP API key without borrowing an admin's bootstrap key | ✓ VERIFIED | `identity_handler.go` `CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey` derive owner from `auth.IdentityFromContext` (no wire `user_id`), reject `Source=="apikey"`, clamp expiry via `clampExpiry` (90d/180d). CLI `auth api-key create/list/rotate/revoke` self-variants authenticate with the stored `spgr_ws_` session (`client.go resolveSessionCredential`, ignores `SPECGRAPH_API_KEY`). Web `/keys` dashboard wired to the 4 self RPCs. `go test ./internal/server ./cmd/specgraph` pass; storage integration (`CreateAPIKeyForUser` quota-safe) passes. |
| 2 | A self-minted key's effective role is capped at the caller's own current role at mint/rotate time — no privilege-escalation "laundering" through a stale or elevated role | ✓ VERIFIED | `CreateMyAPIKey` floors at `auth.RoleMin(requestedOrInherit, id.EffectiveRole)`; `RotateMyAPIKey` RE-DERIVES the floor at the caller's LIVE `EffectiveRole` and never inherits the old key's ceiling (`identity_handler.go:493, :576`). Storage `RotateAPIKeyForUser` builds the new key entirely from explicit args, never inheriting stale `role_downgrade`/`expires_at` (`users.go:124-132`). `RoleMin` is fail-closed to `RoleReader` (`rolemin.go`). Cedar `apikey.self` verb + `IsBuiltinRole` guard. Unit tests pass. |
| 3 | When a user's app role is revoked or downgraded upstream, their standing API/MCP keys stop carrying the old privilege on forced re-sync (not only on next interactive login) | ✓ VERIFIED (behavioral) | `ResyncUserRole` writes `users.role` via `UpdateUserRole` (live floor) and, with `revoke_keys`, revokes every active standing key (`identity_handler.go:647-702`). **Integration tests EXECUTED and PASS:** `TestResync_LiveRoleClamp` (auth) proves a standing token resolves writer→reader after the role write; `TestResync_RPCSeam_LiveRoleClamp` (server) proves the same through the ResyncUserRole RPC seam. Operator CLI `auth user resync <id> --role <r> [--revoke-keys]` wired. |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Plan-Level Must-Have Truths (all 8 plans)

| Plan | Truth (abbrev.) | Status | Evidence |
|------|-----------------|--------|----------|
| 02-01 | 5 IdentityService RPCs + no `user_id` on self-request msgs + gen builds | ✓ VERIFIED | 5 procedure constants in `identity.connect.go`; `CreateMyAPIKeyRequest` has label/role_downgrade/expires_at only; `go build ./...` green |
| 02-02 | Owner-scoped get/rotate/revoke → uniform `ErrAPIKeyNotFound`; quota-safe `FOR UPDATE` mint → `ErrQuotaExceeded`; rotate never inherits stale ceiling | ✓ VERIFIED | `postgres/users.go:522-694` (`FOR UPDATE`, `ErrQuotaExceeded`); integration tests pass |
| 02-03 | Self-service key-policy config (90/180/quota10); double-submit CSRF middleware; cookie issued on whoami GET; legacy `APIKeyConfig` NOT extended | ✓ VERIFIED | `config.SelfServiceKeysConfig` defaults 90/180/10/30/5; `csrf.go` validate+issue; `auth_handler.go:48` `csrfIssue` on whoami |
| 02-04 | `RoleMin` fail-closed; Cedar `apikey.self` verb + base.cedar permit; action map + drift test | ✓ VERIFIED | `engine.go:162` knownVerbs has `self`; `base.cedar` permit; `actions.go:112-116`; `actions_test.go` |
| 02-05 | Owner-from-context, `Source=="apikey"` reject, `RoleMin` floor on create+rotate; hard-set List filter; expiry/rate/quota codes; CSRF mount; audit log w/o secret | ✓ VERIFIED | `identity_handler.go` handlers + `RegisterIdentityService` `mux.Handle(path, csrfValidate(h))`; `TestSelfMint_CSRFMountRejectsMissingToken` |
| 02-06 | ResyncUserRole live floor via UpdateUserRole; revoke_keys off-board; CLI resync; standing-key live-floor + RPC-seam integration tests | ✓ VERIFIED | Handler `:647-702`; both integration tests EXECUTED + PASS; `auth_user.go` resync cmd |
| 02-07 | `auth api-key` self vs `--user` admin split; session-preferring resolver ignoring `SPECGRAPH_API_KEY`; plaintext printed once | ✓ VERIFIED | `auth_apikey.go` self/admin split; `client.go resolveSessionCredential` + warn; `auth_apikey_test.go` pass |
| 02-08 | `/keys` dashboard over 4 self RPCs; one-time reveal modal; CSRF echo | ✓ VERIFIED (code) / ⚠ browser deferred | 4 substantive web files (298/128/107/73 LOC); `identityClient` + `csrfInterceptor`; vitest 10/10 pass; interactive browser flow → human verification |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/specgraph/v1/identity.proto` + `gen/` | 5 RPCs + messages | ✓ VERIFIED | 5 procedure constants present; `go build` green |
| `internal/server/identity_handler.go` | self-mint + resync handlers | ✓ VERIFIED | Owner-from-context, RoleMin floor, source-gate, audit lines; wired via `RegisterIdentityService` |
| `internal/server/csrf.go` | double-submit CSRF | ✓ VERIFIED | validate + issue; mounted on Connect handler; `csrfIssue` on whoami |
| `internal/config/global.go` | `SelfServiceKeysConfig` 90/180/10 | ✓ VERIFIED | struct + defaults; read by handler; served in `serve.go:203` |
| `internal/auth/rolemin.go` | `RoleMin` fail-closed | ✓ VERIFIED | Reader on unranked |
| `internal/auth/actions.go` + `policies/base.cedar` | `apikey.self` map + permit | ✓ VERIFIED | 4 self→apikey.self, resync→user.manage; knownVerbs `self` |
| `internal/storage/users.go` + `postgres/users.go` | owner-scoped + quota-safe | ✓ VERIFIED | `FOR UPDATE` lock, `ErrQuotaExceeded`, uniform NotFound |
| `cmd/specgraph/auth_apikey.go` / `auth_user.go` / `client.go` | CLI self-variants + resolver | ✓ VERIFIED | self/admin split; session resolver; resync cmd |
| `web/src/routes/keys/+page.svelte` + `keys.svelte.ts` + `RevealKeyModal.svelte` + `client.ts` | dashboard + reveal + CSRF | ✓ VERIFIED (code) | substantive, wired to 4 self RPCs; interactive verify deferred to human |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `serve.go:203` | `RegisterIdentityService` | passes `cfg.Auth.SelfServiceKeys` + interceptor into the live server | ✓ WIRED |
| `RegisterIdentityService` | `csrfValidate(h)` | `mux.Handle(path, csrfValidate(h))` mounts CSRF in front of Connect handler | ✓ WIRED |
| handler | `auth.RoleMin` | floored role_downgrade on create + rotate | ✓ WIRED |
| handler | `storage.*APIKeyForUser` | owner-scoped storage, handler-owned secret | ✓ WIRED |
| `auth_handler.go:48` | `csrfIssue` | cookie issued on `/api/auth/whoami` GET | ✓ WIRED |
| web `client.ts` `csrfInterceptor` | `X-CSRF-Token` | echoes `specgraph_csrf` cookie into header | ✓ WIRED |
| web `keys.svelte.ts` | 4 self RPCs | `identityClient.{list,create,rotate,revoke}MyAPIKeys` | ✓ WIRED |
| `actions.go` | Cedar engine | `apikey.self` / `user.manage` map + base.cedar permit + knownVerbs | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Standing key EffectiveRole drops after role write (SC#3, resolver) | `go test -tags integration -run TestResync_LiveRoleClamp ./internal/auth/` | ok 1.810s | ✓ PASS |
| Standing key clamps via ResyncUserRole RPC seam (SC#3, RPC) | `go test -tags integration -run TestResync_RPCSeam_LiveRoleClamp ./internal/server/` | ok 1.713s | ✓ PASS |
| Quota-safe/owner-scoped storage mutations | `go test -tags integration -run 'TestAuthStore_CreateAPIKeyForUser\|RotateAPIKeyForUser\|RevokeAPIKeyForUser' ./internal/storage/postgres/` | ok 2.092s | ✓ PASS |
| Server + auth + CLI unit suites | `go test ./internal/server ./internal/auth ./cmd/specgraph` | ok (cached) | ✓ PASS |
| Web dashboard unit tests | `pnpm test` (web) | 10 passed (2 files) | ✓ PASS |
| Whole tree compiles | `go build ./...` | BUILD_OK | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| AUTH-02 | 02-01, 02-04, 02-06 | Enforce app-role revocation on standing API/MCP keys, forcing re-sync | ✓ SATISFIED | `ResyncUserRole` seam (live floor + revoke_keys off-board); CLI `auth user resync`; both live-floor integration tests pass |
| AUTH-03 | 02-01…02-05, 02-07, 02-08 | Self-service / automatic MCP API-key provisioning for OIDC users | ✓ SATISFIED | 4 self RPCs, quota-safe owner-scoped storage, RoleMin floor, CSRF, config policy, CLI self-variants, web dashboard (unit-tested; interactive browser flow deferred to human) |

No orphaned requirements: REQUIREMENTS.md maps only AUTH-02, AUTH-03 to Phase 2; both are claimed across the plans' `requirements:` frontmatter.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | No `TBD`/`FIXME`/`XXX` debt markers in any phase-modified file | — | Clean |
| `web/.../+page.svelte` | 97,101 | `placeholder="ci-runner"` / `placeholder="reader"` | ℹ️ Info | Legitimate HTML input placeholder attributes (UI hints), NOT stub markers |

### Human Verification Required

Three items require a running server + browser (all supporting SC#1's web half; backend + unit coverage already verified). Plan 02-08 D5 was **intentionally deferred** to the human per phase context — its automated checks (vitest 8/8, web build) already pass and the code is committed.

1. **Live /keys dashboard flow** — create → rotate → revoke; confirm one-time reveal is unrecoverable after modal close and list refreshes.
2. **CSRF enforcement on live mutations** — strip the `specgraph_csrf` cookie / `X-CSRF-Token` header and confirm 403; normal mutation succeeds.
3. **Anti-key-chaining message** — a `Source=="apikey"` session self-mint is denied with a readable message.

### Gaps Summary

No gaps. All three ROADMAP success criteria are verified in the codebase, and every plan-level must-have truth, artifact, and key link is present, substantive, and wired. The strongest criterion (SC#3, the security-critical "revoked role can't survive on a standing key") was confirmed **behaviorally** by executing both live-floor integration tests against real Postgres — they pass. AUTH-02 and AUTH-03 are both satisfied.

The phase is not marked `passed` solely because Plan 02-08's interactive browser verification was intentionally deferred to a human (per phase context and the plan's own D5 human-verify checkpoint). This is a `human_needed` routing item, not a code gap — the dashboard code is committed and unit-tested.

---

_Verified: 2026-07-10T02:30:24Z_
_Verifier: the agent (gsd-verifier)_
