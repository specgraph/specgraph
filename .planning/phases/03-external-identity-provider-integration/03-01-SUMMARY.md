---
phase: 03-external-identity-provider-integration
plan: 01
subsystem: auth
tags: [oidc, oauth2, identity, session, mcp, rfc8707, jwt]

# Dependency graph
requires:
  - phase: 02-authoring-cli-integration
    provides: OIDC login flow, IdentityStore resolver, claims_mapping, login-sync, web_sessions schema
provides:
  - Identity.Issuer field (verified iss for OIDC / synthetic id for oauth2, D-09)
  - Resolver.ResolveLogin(ctx, *OIDCClaims) claims-based interactive-login entrypoint
  - pgIdentityStore.materializeIdentity shared binding→JIT→login-sync→Identity tail
  - LoginProvider.Exchange returns *OIDCClaims (was raw id_token string)
  - config.ProviderIssuer canonical-issuer helper (single source of truth, review HIGH #1)
  - config OIDCProviderConfig oauth2 fields + OIDCConfig.MCPResourceURI + ValidateMCPResourceURI
  - auth.WithMCPRequest / MCPRequestFromContext per-request marker (review HIGH #2)
  - AUTH-05 session issuer population in the browser login callback
affects: [03-02-oauth2-provider, 03-03-rfc9728-metadata, 03-04-introspection-audience]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pattern 2 (RESEARCH): claims-based interactive-login entrypoint (ResolveLogin) keeps Resolve(ctx, token string) structurally untouched, preserving D-08 additivity"
    - "Shared materializeIdentity seam: resolveJWT and ResolveLogin both flow through one binding→JIT→login-sync tail"
    - "Single canonical-issuer helper (config.ProviderIssuer) so synthetic oauth2 issuer cannot diverge across consumers"
    - "Per-request context markers (WithMCPRequest) mirror WithInteractiveLogin for path-scoped audience checks"

key-files:
  created:
    - internal/config/oidc_issuer_test.go
  modified:
    - internal/auth/auth.go
    - internal/auth/resolver.go
    - internal/auth/context.go
    - internal/auth/identitystore.go
    - internal/auth/loginprovider.go
    - internal/config/global.go
    - internal/server/auth_oidc_handler.go
    - internal/server/identity_integration_test.go

key-decisions:
  - "materializeIdentity/ResolveLogin (planned as Task 2 identitystore work) landed in the Task 1 commit to keep every commit buildable after the interface change"
  - "handleCallback's switch to ResolveLogin(claims) landed with the Task 2 Exchange-signature change (buildability); Issuer threading stayed in Task 3"
  - "jitResolve takes an explicit interactive bool; InteractiveLoginFromContext is derived once in resolveJWT (single source of truth)"
  - "CLI-exchange path left with empty issuer (OQ3 accepted scope, bounded ≤1 session TTL)"

patterns-established:
  - "Interface method + production impl folded into one commit when a broken intermediate build would otherwise result"

requirements-completed: [AUTH-01, AUTH-05]

# Coverage metadata
coverage:
  - id: D1
    description: "Identity.Issuer, Resolver.ResolveLogin, WithMCPRequest marker, and the phase config surface (oauth2 fields, MCPResourceURI + ValidateMCPResourceURI, ProviderIssuer) exist and the tree builds"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/config/oidc_issuer_test.go#TestProviderIssuer"
        status: pass
      - kind: unit
        ref: "internal/config/oidc_issuer_test.go#TestValidateMCPResourceURI"
        status: pass
      - kind: unit
        ref: "internal/auth/context_test.go#TestMCPRequestContext"
        status: pass
    human_judgment: false
  - id: D2
    description: "resolveJWT and ResolveLogin share materializeIdentity; JWT-path behavior preserved; Exchange returns *OIDCClaims; D-08 credential paths unchanged"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_jit_test.go#TestResolveLogin_ThreadsIssuerOnBindingHit"
        status: pass
      - kind: unit
        ref: "internal/auth/identitystore_jit_test.go#TestResolveLogin_ThreadsIssuerOnJIT"
        status: pass
      - kind: unit
        ref: "go test ./internal/auth/ -run 'Resolve|JIT|LoginSync'"
        status: pass
    human_judgment: false
  - id: D3
    description: "Browser login callback resolves via claims and persists web_sessions.issuer (AUTH-05, D-09); no backfill (D-10); CLI path unchanged"
    requirement: "AUTH-05"
    verification:
      - kind: unit
        ref: "internal/server/auth_oidc_handler_test.go#TestOIDCCallback_HappyPath"
        status: pass
      - kind: integration
        ref: "internal/server/identity_integration_test.go#TestIntegration_SessionIssuer"
        status: pass
    human_judgment: false

# Metrics
duration: ~30 min
completed: 2026-07-10
status: complete
---

# Phase 3 Plan 01: Identity-Materialization Seam + Session Issuer Summary

**Claims-based `ResolveLogin` interactive-login entrypoint sharing a `materializeIdentity` seam with the JWT path, `LoginProvider.Exchange` now returning verified `*OIDCClaims`, and browser sessions persisting the authenticating issuer (AUTH-05) — with the full phase config contract (oauth2 fields, canonical `ProviderIssuer`, MCP resource-URI validation) defined up front.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-07-10T08:53Z (approx)
- **Completed:** 2026-07-10T09:00Z
- **Tasks:** 3
- **Files modified:** 8 (+1 created)

## Accomplishments
- Added `Identity.Issuer` and the `Resolver.ResolveLogin(ctx, *OIDCClaims)` interface method; extracted the post-verification tail of `resolveJWT` into a shared `materializeIdentity(ctx, claims, interactive)` so JWT-bearer and interactive-login paths stay behavior-identical (Pattern 2 — `Resolve(ctx, token string)` left structurally untouched, preserving D-08).
- Reshaped `LoginProvider.Exchange` to return verified `*OIDCClaims` instead of a raw id_token string; `handleCallback` now calls `ResolveLogin(WithInteractiveLogin(ctx), claims)`.
- Threaded the verified/synthetic issuer through to the browser session mint (`Issuer: id.Issuer` on the `storage.Session` literal) — AUTH-05 / D-09. CLI-exchange path deliberately unchanged (OQ3).
- Defined the whole phase config surface in `global.go`: oauth2 provider fields (`AuthURL`/`TokenURL`/`UserinfoURL`/`EmailsURL`/`SubjectField`/`EmailField`/`IntrospectionURL`), `OIDCConfig.MCPResourceURI` + `ValidateMCPResourceURI`, and the canonical `ProviderIssuer` helper (review HIGH #1).
- Added `auth.WithMCPRequest`/`MCPRequestFromContext` per-request marker for the future path-scoped MCP audience check (review HIGH #2), unconsumed this plan.

## Task Commits

1. **Task 1: Identity.Issuer + ResolveLogin seam + phase config surface** - `5f4ec7c0` (feat)
2. **Task 2: Exchange returns *OIDCClaims; callback resolves via ResolveLogin** - `b3bbe6c8` (feat)
3. **Task 3: thread session issuer into the login callback mint (AUTH-05)** - `99cc5248` (feat)

_Note: the interface+impl coupling meant `materializeIdentity`/`ResolveLogin` (planned Task 2 identitystore work) landed in commit 1, and the handler's `ResolveLogin` switch landed in commit 2 — see Deviations._

## Files Created/Modified
- `internal/auth/auth.go` - `Identity.Issuer` field
- `internal/auth/resolver.go` - `ResolveLogin` on the Resolver interface
- `internal/auth/context.go` - `WithMCPRequest` / `MCPRequestFromContext`
- `internal/auth/identitystore.go` - `materializeIdentity` extraction, `ResolveLogin` impl, `jitResolve(interactive bool)`, Issuer stamping
- `internal/auth/loginprovider.go` - `Exchange` → `*OIDCClaims`
- `internal/config/global.go` - oauth2 fields, `MCPResourceURI`, `ValidateMCPResourceURI`, `ProviderIssuer`
- `internal/config/oidc_issuer_test.go` - `ProviderIssuer` + `ValidateMCPResourceURI` tests
- `internal/server/auth_oidc_handler.go` - callback uses `ResolveLogin(claims)` + sets `Issuer: id.Issuer`
- `internal/server/identity_integration_test.go` - `TestIntegration_SessionIssuer` (issuer persist + no-backfill)
- test-only: mock `Resolver`s across `internal/auth`, `internal/server`, `cmd/specgraph`, `e2e/api` gained `ResolveLogin`; `fakeProvider.Exchange` updated to claims.

## Decisions Made
- Folded the tightly-coupled interface+implementation into buildable commits (see Deviations). Each of the 3 commits builds green and passes unit tests.
- `jitResolve` now takes an explicit `interactive bool`; `InteractiveLoginFromContext` is read exactly once (in `resolveJWT`) — single source of truth (review 03-01 MEDIUM).
- CLI-exchange path keeps issuer empty (OQ3 accepted scope; bounded ≤1 session TTL, consistent with D-10).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Cross-task compile coupling required resequencing buildable commits**
- **Found during:** Task 1 → Task 2 → Task 3
- **Issue:** The plan split the interface addition (Task 1) from its production implementation (Task 2, `identitystore.go`) and the `Exchange` signature change (Task 2) from its caller update (Task 3, `handleCallback`). Committing strictly per the planned file lists would produce non-building intermediate commits (`pgIdentityStore` failing to satisfy `Resolver`; `handleCallback` calling the old `Exchange` shape).
- **Fix:** Kept 3 task-aligned commits but shifted the minimum work needed for buildability: `materializeIdentity`/`ResolveLogin`/`jitResolve(interactive)` landed in commit 1; the `handleCallback` switch to `ResolveLogin(claims)` landed in commit 2 (with the `Exchange` change); `Issuer: id.Issuer` threading + integration test stayed in commit 3. Behavior at each commit is correct and tests pass.
- **Files modified:** internal/auth/identitystore.go (commit 1), internal/server/auth_oidc_handler.go (commit 2)
- **Verification:** `go build ./...` + `go test ./internal/auth/ ./internal/server/` green after each commit.
- **Committed in:** 5f4ec7c0, b3bbe6c8

**2. [Rule 2 - Missing Critical] All mock Resolvers updated for the new interface method**
- **Found during:** Task 1
- **Issue:** Adding `ResolveLogin` to the `Resolver` interface broke compilation of every mock resolver across the codebase (`internal/auth`, `internal/server`, `cmd/specgraph`, `e2e/api`), not just the plan-named `fakeResolver` in `interceptor_test.go`.
- **Fix:** Added a `ResolveLogin` method to each mock resolver (`stubResolver`, `apiTestResolver`, `noAuthResolver`, `mockResolver`, `staticResolver`, and both `fakeResolver`s).
- **Files modified:** internal/auth/{interceptor_test.go, identity_carrier_test.go}, internal/server/{api_handler_test.go, auth_handler_test.go, auth_oidc_handler_test.go}, cmd/specgraph/serve_mcp_test.go, e2e/api/auth_test.go
- **Verification:** `go build ./...` + full unit suite green.
- **Committed in:** 5f4ec7c0

---

**Total deviations:** 2 auto-fixed (1 blocking-resequence, 1 missing-critical interface fanout)
**Impact on plan:** No scope change; all planned artifacts delivered. Commit boundaries shifted only to preserve buildable history. No behavioral drift from the plan's intent.

## Issues Encountered
None — all planned work completed. `task check` surfaced 139 pre-existing dprint-formatting drifts in unrelated `.planning/intel/classifications/*.json`; logged to `deferred-items.md` and left untouched per the scope boundary. All Go source is gofmt-clean and `golangci-lint` reports 0 issues on the affected packages.

## User Setup Required
None - no external service configuration required this plan (GitHub OAuth App / live IdP are only needed for later manual UAT of AUTH-01, not for this seam).

## Next Phase Readiness
- The seam is ready for Plan 02 (oauth2 provider consuming `materializeIdentity` + `ProviderIssuer`), Plan 03 (RFC 9728 metadata mounting `MCPResourceURI`/`WithMCPRequest`), and Plan 04 (introspection + audience check reading `MCPRequestFromContext`).
- `task test` green (build + fast unit tests). Integration `TestIntegration_SessionIssuer` passes on Docker.
- No blockers.

---
*Phase: 03-external-identity-provider-integration*
*Completed: 2026-07-10*

## Self-Check: PASSED
- Created file `internal/config/oidc_issuer_test.go` exists on disk.
- Commits `5f4ec7c0`, `b3bbe6c8`, `99cc5248` present in git log.
- All task acceptance-criteria greps pass; `go build ./...`, full unit suite, and `golangci-lint` (auth/config/server/cmd) green; integration `SessionIssuer` passes on Docker.
