---
phase: 02-api-key-lifecycle-self-service
plan: 03
subsystem: api
tags: [csrf, config, koanf, connectrpc, security, api-keys, go]

# Dependency graph
requires:
  - phase: 02-api-key-lifecycle-self-service
    provides: existing auth handlers (auth_handler.go), Connect IdentityService, config loader (global.go)
provides:
  - SelfServiceKeysConfig struct (default 90d / max 180d / quota 10 / rate-limit) defaulted in globalDefaults()
  - csrfValidate double-submit CSRF validator middleware for cookie-authed self-key POSTs
  - issueCSRFCookie / csrfIssue issuer wired onto the /api/auth/whoami GET
affects: [plan-05-self-mint-handlers, plan-08-web-keys-panel]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Double-submit CSRF: crypto/rand non-HttpOnly cookie + echoed X-CSRF-Token header, crypto/subtle.ConstantTimeCompare"
    - "Dedicated config struct for new policy (SelfServiceKeysConfig) instead of extending deprecated APIKeyConfig"

key-files:
  created:
    - internal/server/csrf.go
    - internal/server/csrf_test.go
  modified:
    - internal/config/global.go
    - internal/config/loader_internal_test.go
    - internal/server/auth_handler.go

key-decisions:
  - "New SelfServiceKeysConfig on AuthConfig (auth.self_service_keys); deprecated APIKeyConfig untouched"
  - "CSRF validator exempts Bearer-authenticated (CLI/MCP) requests — not CSRF-able"
  - "ListMyAPIKeys included in the enforced procedure set for defense-in-depth (cursor #6)"
  - "CSRF cookie issued unconditionally on whoami GET when absent; existing cookie never rotated"

patterns-established:
  - "CSRF middleware shape modeled on cookieToAuthHeader http.Handler decorator"
  - "Cookie attributes (SameSite=Lax, dynamic Secure) mirror sessionCookie but non-HttpOnly for double-submit echo"

requirements-completed: [AUTH-03]

coverage:
  - id: D1
    description: "Self-service key-policy config struct (default 90d / max 180d / quota 10 / rate-limit) defaulted in globalDefaults(); deprecated APIKeyConfig untouched"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/config/loader_internal_test.go#TestGlobalDefaults_SelfServiceKeys"
        status: pass
      - kind: unit
        ref: "go build ./internal/config/"
        status: pass
    human_judgment: false
  - id: D2
    description: "Double-submit CSRF validator enforces constant-time token on cookie-authed self-key POSTs, exempts Bearer callers"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/csrf_test.go#TestCSRFValidate_MatchingTokenPasses,MissingHeaderRejected,MismatchedTokenRejected,BearerRequestExempt"
        status: pass
    human_judgment: false
  - id: D3
    description: "CSRF cookie issuer (non-HttpOnly, SameSite=Lax) wired onto the /api/auth/whoami GET to bootstrap the token before first mutation"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/server/csrf_test.go#TestCSRFIssue_SetsNonHTTPOnlyCookieWhenAbsent,DoesNotOverwriteExistingCookie"
        status: pass
    human_judgment: false

# Metrics
duration: 4min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 03: CSRF + Self-Service Key-Policy Config Summary

**Double-submit CSRF middleware (crypto/rand non-HttpOnly cookie + constant-time X-CSRF-Token compare) guarding cookie-authed self-key POSTs, plus a dedicated SelfServiceKeysConfig (90d/180d/quota-10/rate-limit) — both handler-independent, ready for Plan 05.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-09T16:45:43Z
- **Completed:** 2026-07-09T16:49:45Z
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments
- New `SelfServiceKeysConfig` struct on `AuthConfig` (`auth.self_service_keys`) with yaml+koanf tags, defaulted in `globalDefaults()` to DefaultTTLDays=90, MaxTTLDays=180, Quota=10, RateLimitPerHour=30, RateLimitBurst=5 (D-08). Deprecated `APIKeyConfig` left untouched.
- New `internal/server/csrf.go`: `csrfValidate` enforces a double-submit CSRF token (constant-time compare) only on cookie-authenticated POSTs to the four self-key Connect procedures; Bearer-authenticated CLI/MCP requests are exempt.
- `issueCSRFCookie`/`csrfIssue` generate a `crypto/rand` token in a non-HttpOnly, SameSite=Lax cookie and wire issuance onto the safe `/api/auth/whoami` GET so the token exists before the first mutation.
- Full unit coverage: pass / missing-header-403 / mismatch-403 / bearer-exempt / unprotected-path-pass / non-HttpOnly issuance / no-overwrite-existing.

## Task Commits

Each task was committed atomically:

1. **Task 1: Self-service key-policy config struct + defaults** - `3792eda6` (feat)
2. **Task 2: Double-submit CSRF middleware (validate + issue) + whoami issuance + test** - `be18e244` (feat)

**Plan metadata:** _(this docs commit)_

## Files Created/Modified
- `internal/config/global.go` - Added `SelfServiceKeysConfig` struct, `AuthConfig.SelfServiceKeys` field, and defaults in `globalDefaults()`
- `internal/config/loader_internal_test.go` - Added `TestGlobalDefaults_SelfServiceKeys` guarding the D-08 defaults
- `internal/server/csrf.go` - New double-submit CSRF validator + issuer middleware
- `internal/server/csrf_test.go` - New CSRF unit tests (validator + issuer)
- `internal/server/auth_handler.go` - Wrapped the whoami GET with `csrfIssue` for token bootstrap

## Decisions Made
- Placed the new policy on `AuthConfig` as `auth.self_service_keys` (scalar-only fields → env-settable) rather than nesting under OIDC; keeps it a peer of `oidc` and away from the deprecated `APIKeyConfig`.
- Validator enforces only when method=POST AND path∈self-key-procedures AND no Bearer token — the narrowest surface that still covers browser (cookie) mutations.
- `ListMyAPIKeys` (a POST in Connect) included in the enforced set for defense-in-depth (cursor #6).
- Issuer fails closed on a `crypto/rand` error (skips setting a predictable token); the validator already rejects when the cookie is absent, so no CSRF weakening results.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added a config defaults guard test**
- **Found during:** Task 1
- **Issue:** The plan's Task-1 verify runs `-run TestGlobalDefaults` but no test asserted the new self-service defaults, leaving the D-08 values (90/180/10) unguarded against regression.
- **Fix:** Added `TestGlobalDefaults_SelfServiceKeys` to `loader_internal_test.go` asserting DefaultTTLDays=90, MaxTTLDays=180, Quota=10, and positive rate-limit fields.
- **Files modified:** internal/config/loader_internal_test.go
- **Verification:** `go test ./internal/config/ -run TestGlobalDefaults` passes.
- **Committed in:** 3792eda6 (Task 1 commit)

**2. [Rule 3 - Blocking] gosec G124 on test request cookies**
- **Found during:** Task 2
- **Issue:** `golangci-lint` (gosec G124) flagged `req.AddCookie(&http.Cookie{...})` in the test file (client-side request cookies don't carry Secure/HttpOnly), which would block the pre-commit hook.
- **Fix:** Added `//nolint:gosec // G124: test request cookie` on each `AddCookie` line, matching the existing convention in `auth_handler_test.go`/`auth_oidc_handler_test.go`.
- **Files modified:** internal/server/csrf_test.go
- **Verification:** `golangci-lint run ./internal/server/...` → 0 issues.
- **Committed in:** be18e244 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 missing-critical test, 1 blocking lint). **Impact:** Both necessary — one guards the config contract, one clears the commit gate. No scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Config policy and CSRF protection now exist independently of the handlers.
- **Plan 05** must mount `csrfValidate` on the Connect IdentityService handler (`RegisterIdentityService` / `serve.go`) and read `SelfServiceKeysConfig` for expiry-cap/quota/rate-limit — the validator mount is a Plan 05 acceptance criterion; without it the CSRF mitigation is aspirational.
- **Plan 08** (web client) must echo the `specgraph_csrf` cookie as the `X-CSRF-Token` header on self-key mutations.

## Self-Check: PASSED

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*
