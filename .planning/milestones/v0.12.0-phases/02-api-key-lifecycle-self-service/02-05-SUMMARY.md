---
phase: 02-api-key-lifecycle-self-service
plan: 05
subsystem: auth
tags: [api-keys, self-service, rbac, csrf, connectrpc, go]

requires:
  - phase: 02-01
    provides: generated IdentityService self-service procedures (CreateMyAPIKey, ListMyAPIKeys, RotateMyAPIKey, RevokeMyAPIKey) + request/response messages
  - phase: 02-02
    provides: owner-scoped storage methods (GetAPIKeyForUser, RevokeAPIKeyForUser, RotateAPIKeyForUser, quota-safe CreateAPIKeyForUser) + storage.ErrQuotaExceeded sentinel
  - phase: 02-03
    provides: SelfServiceKeysConfig (90d/180d caps, quota, rate-limit) + csrfValidate double-submit middleware
  - phase: 02-04
    provides: exported auth.RoleMin fail-closed role floor + apikey.self Cedar verb + procedure→action map
provides:
  - "Four AUTH-03 self-service handlers on IdentityHandler: CreateMyAPIKey, ListMyAPIKeys, RotateMyAPIKey, RevokeMyAPIKey"
  - "selfMintLimiter — per-identity token-bucket rate limiter (sync.Map, reused JIT pattern)"
  - "clampExpiry — resolves/caps self-mint expiry to DefaultTTLDays / MaxTTLDays"
  - "csrfValidate mounted on the Connect IdentityService handler in RegisterIdentityService"
  - "identityError maps storage.ErrQuotaExceeded → connect.CodeResourceExhausted"
affects: [02-06, ResyncUserRole handler, CLI self-variants, web keys dashboard]

tech-stack:
  added: []
  patterns:
    - "Owner-from-context: self handlers derive the owner from auth.IdentityFromContext, never a request field"
    - "Re-derive on rotate: role ceiling + expiry are re-floored/re-clamped from the caller's live effective role, never inherited from the old key"
    - "Handler-owned secret generation: auth.GenerateAPIKeySecret in the handler; storage returns *APIKey only (never the plaintext)"
    - "Structured audit line with server-derived fields only — plaintext secret and raw user label never logged (log-injection / PII guard)"
    - "Middleware mounted in front of the Connect handler: csrfValidate(h) wraps NewIdentityServiceHandler before mux.Handle"

key-files:
  created:
    - internal/server/identity_selfkeys_test.go
  modified:
    - internal/server/identity_handler.go
    - cmd/specgraph/serve.go
    - internal/server/identity_handler_test.go
    - internal/server/identity_integration_test.go

key-decisions:
  - "RegisterIdentityService grew a non-variadic SelfServiceKeysConfig parameter (not another HandlerOption) so config is actually threaded, not lost in the opts variadic"
  - "Rotate re-floors the role at the caller's live EffectiveRole and re-clamps expiry to DefaultTTLDays — a stale higher ceiling or long window on the old key can never be inherited"
  - "The audit line logs only server-derived fields (actor, key_id, action); the raw user-supplied label is treated as untrusted and omitted entirely (RESEARCH §6 / T-02-33)"
  - "csrfValidate is mounted in RegisterIdentityService itself (not serve.go) so every mount of the IdentityService — including tests — inherits the CSRF gate"

patterns-established:
  - "Owner-scoped self RPC: identity from context → owner-scoped storage call → foreign/missing surfaces as CodeNotFound, never a cross-user leak"
  - "Two anti-abuse gates on mint/rotate: Source==apikey reject (anti key-chaining) + per-identity rate limit before argon2id compute"

requirements-completed: [AUTH-03]

coverage:
  - id: D1
    description: "CreateMyAPIKey mints an owner-from-context key with the role floored at the caller's live EffectiveRole, expiry clamped to the 90d/180d caps, per-identity rate limited, quota-safe, secret emitted once"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestCreateMyAPIKey_FloorsAtEffectiveRole"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestSelfMint_ExpiryCap"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestSelfMint_RateLimited"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestCreateMyAPIKey_QuotaExceededMapsResourceExhausted"
        status: pass
    human_judgment: false
  - id: D2
    description: "RotateMyAPIKey re-floors the role at the caller's live EffectiveRole and re-clamps expiry — never inheriting the old key's stale ceiling/window; anti key-chaining reject applies to create and rotate"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestRotateMyAPIKey_FloorsAtEffectiveRole"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestSelfMint_RejectsApikeySource"
        status: pass
    human_judgment: false
  - id: D3
    description: "ListMyAPIKeys hard-sets the storage filter UserID from context (no cross-user leak); RevokeMyAPIKey is owner-scoped, foreign/missing → CodeNotFound"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestListMyAPIKeys_ScopedToCaller"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestRevokeMyAPIKey_ForeignKeyNotFound"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every successful self create/rotate/revoke emits a structured audit line (actor/key_id/action) that never contains the plaintext secret or the raw user-supplied label"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestSelfMint_AuditLogged"
        status: pass
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestRevokeMyAPIKey_AuditLogged"
        status: pass
    human_judgment: false
  - id: D5
    description: "csrfValidate is mounted on the Connect IdentityService handler: a cookie-authed self-key POST without a valid X-CSRF-Token is rejected 403; a Bearer request is exempt"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/identity_selfkeys_test.go#TestSelfMint_CSRFMountRejectsMissingToken"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 05: AUTH-03 Self-Service API Key Handlers Summary

**Four self-service API key handlers (create/list/rotate/revoke) enforcing owner-from-context, the RoleMin role floor on create AND rotate, expiry clamp, per-identity rate limit, quota-safe mint, redacted audit logging, and a mounted double-submit CSRF gate.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-09
- **Tasks:** 3 (all `tdd="true"`)
- **Files modified:** 4 (+1 created)

## Accomplishments

- **CreateMyAPIKey + RotateMyAPIKey** — owner from `auth.IdentityFromContext`; `Source=="apikey"` rejected with `CodePermissionDenied` (anti key-chaining); role floored at `auth.RoleMin(requestedOrInherit, caller.EffectiveRole)` on create AND rotate (SC#2); expiry clamped to 90d default / 180d max; per-identity `selfMintLimiter` → `CodeResourceExhausted`; handler-owned secret via `auth.GenerateAPIKeySecret`, plaintext emitted exactly once (SC#1).
- **ListMyAPIKeys + RevokeMyAPIKey** — `ListAPIKeysFilter.UserID` hard-set from context (no cross-user leak); revoke is owner-scoped, foreign/missing → `CodeNotFound`, re-revoke idempotent.
- **CSRF mount + quota mapping** — `csrfValidate` wrapped around the Connect IdentityService handler in `RegisterIdentityService` (Bearer/CLI exempt); `identityError` maps `storage.ErrQuotaExceeded` → `CodeResourceExhausted`.
- **Redacted audit trail** — structured `apikey.self.{create,rotate,revoke}` lines carry only server-derived fields (actor, key_id, action); the plaintext secret and raw user label are never logged.
- `SelfServiceKeysConfig` threaded through `cmd/specgraph/serve.go`; all test call sites updated so `go build ./...` and `go test ./internal/server/` stay green.

## Task Commits

1. **Tasks 1–3: four self-service handlers + CSRF mount + quota mapping + adversarial tests** — `a17c933a` (feat)

**Plan metadata:** _(docs commit — see below)_

_Note: This plan resumed a cancelled mid-flight executor that had left uncommitted partial work (CreateMyAPIKey + RotateMyAPIKey implemented; ListMyAPIKeys + RevokeMyAPIKey still stubs). The pre-existing work was reconciled and completed. Because the RegisterIdentityService signature change ripples through the two `_test.go` call sites and the shared `testSelfServiceKeysConfig` helper lives in the new test file, the production code and its tests are compile-coupled — splitting them would leave a commit where `go test` fails to compile — so they landed as one atomic, green commit rather than per-task commits._

## Files Created/Modified

- `internal/server/identity_handler.go` — four self handlers, `selfMintLimiter`, `clampExpiry`, `SelfServiceKeysConfig` field + constructor param, `csrfValidate` mount, `ErrQuotaExceeded` → `CodeResourceExhausted` mapping
- `internal/server/identity_selfkeys_test.go` (new) — adversarial coverage: role floor (create+rotate), owner scoping, expiry cap, rate limit, CSRF-mount 403, quota mapping, audit-log redaction
- `cmd/specgraph/serve.go` — thread `cfg.Auth.SelfServiceKeys` into `RegisterIdentityService`
- `internal/server/identity_handler_test.go`, `internal/server/identity_integration_test.go` — updated `RegisterIdentityService` call sites for the new config parameter

## Decisions Made

- `RegisterIdentityService` grew a **non-variadic** `SelfServiceKeysConfig` parameter rather than another `HandlerOption` — otherwise the config would be swallowed by the `opts ...` variadic and never threaded.
- Rotate **re-derives** the role ceiling and expiry from the caller's live effective role and the policy default; it never inherits the old key's stale ceiling or window (closes T-02-14 on rotate).
- The audit line omits the raw user-supplied `label` entirely (untrusted → log-injection / PII vector), logging only server-derived fields.
- `csrfValidate` is mounted inside `RegisterIdentityService` (not `serve.go`) so every mount — including tests — inherits the gate uniformly.

## Deviations from Plan

None — plan executed as written. The three tasks were completed; the only structural note is the atomic (rather than per-task) commit, forced by the compile-coupling of the signature change, the shared test helper, and the pre-existing uncommitted work from the cancelled executor (documented above, not a scope change).

## Issues Encountered

- One `gosec` G124 finding on a test-only `http.Cookie` double (`identity_selfkeys_test.go`) — suppressed with a scoped `//nolint:gosec` and rationale, consistent with the project's documented gosec-in-tests convention. `task lint:go` → 0 issues.
- `task check` `fmt:check` reports pre-existing dprint drift in unrelated `.planning/intel/**/*.json` files (not touched by this plan) — out of scope, left untouched.

## TDD Gate Compliance

Plan tasks are `tdd="true"`. This plan resumed a cancelled executor that had already written implementation and tests together for Task 1, so a clean RED→GREEN commit sequence could not be reconstructed retroactively. All behavior is nonetheless proven by adversarial tests that pass (`go test ./internal/server/ -count=1` green); the tests fail against the prior stub implementations, confirming they assert real behavior.

## Next Phase Readiness

- AUTH-03 (SC#1 self-provision without an admin key; SC#2 role capped at caller's current role, no laundering) is met at the server boundary with adversarial coverage.
- `ResyncUserRole` remains a `CodeUnimplemented` stub, correctly deferred to plan 02-06 (AUTH-02 hard off-board).
- Ready for 02-06.

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*

## Self-Check: PASSED

- `internal/server/identity_selfkeys_test.go` — FOUND
- `internal/server/identity_handler.go` — FOUND (four handlers, no CodeUnimplemented among them)
- Commit `a17c933a` — FOUND in git log
- `go build ./...` — green
- `go test ./internal/server/ -count=1` — green
- `task lint:go` — 0 issues
