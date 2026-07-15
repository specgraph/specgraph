---
phase: 09-jit-display-name-reconciliation
plan: 02
subsystem: auth
tags: [oidc, oauth2, introspection, jit, identity, go, postgres, testcontainers]

requires:
  - phase: 09-jit-display-name-reconciliation
    plan: 01
    provides: "reconcileDisplayName helper and its unconditional call site in materializeIdentity's existing-user branch"
provides:
  - "resolveIntrospection populates OIDCClaims.Name via nameFromClaims(res.Raw), closing the third (opaque-token) resolution path for AUTH-06 reconciliation"
  - "jitResolve seeds a newly-provisioned user's DisplayName from claims.Name (falling back to claims.Subject), eliminating the stale-fallback window at first login (D-07)"
  - "Regression-locked confirmation that the non-OIDC oauth2 Exchange path already populates OIDCClaims.Name via displayNameFromUserinfo (D-05, no production change)"
  - "End-to-end Postgres proof that reconciliation fires unconditionally on both the interactive (ResolveLogin) and non-interactive bearer (Resolve) paths, with the pre-existing login-sync gate test unmodified"
affects: []

tech-stack:
  added: []
  patterns:
    - "Local seed-preference variable (seedName := claims.Subject; override if claims.Name != \"\") rather than an inline ternary-equivalent, mirroring the existing pattern for readability at the storage.User construction site."

key-files:
  created: []
  modified:
    - internal/auth/identitystore.go
    - internal/auth/introspection_test.go
    - internal/auth/identitystore_jit_test.go
    - internal/auth/oauth2_provider_test.go
    - internal/auth/identitystore_authn_integration_test.go

key-decisions:
  - "D-04: resolveIntrospection's OIDCClaims literal gains only `Name: nameFromClaims(res.Raw)` — a one-line fix, no new sanitization, since nameFromClaims already unmarshals defensively and this only lets an existing feature (09-01's reconciliation) run on a path it was silently skipping."
  - "D-07: jitResolve prefers claims.Name over claims.Subject when seeding DisplayName, via a local `seedName` variable; role derivation is untouched (still solely claims-mapping at JIT time, never influenced by name)."
  - "D-05 (RESEARCH-resolved, reconfirmed here): oauth2LoginProvider.Exchange already sets Name via displayNameFromUserinfo — no production change; added a dedicated named regression test to close what was previously only an implicit assertion inside a differently-scoped test."
  - "Integration test built without MCPResourceURI (introspection unit test) and via authnTestStore's shared JIT-enabled resolver (integration test) to keep each test focused on the specific reconciliation path under test."

requirements-completed: [AUTH-06]

coverage:
  - id: D1
    description: "An opaque-token (introspection) login populates claims.Name from the introspection payload and reconciles a stale display_name, with role/email passed through unchanged"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_ReconcilesStaleDisplayName"
        status: pass
    human_judgment: false
  - id: D2
    description: "First-time JIT provisioning seeds display_name from claims.Name when present, falling back to claims.Subject"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_jit_test.go#TestJIT_SeedsDisplayNameFromClaimsName"
        status: pass
    human_judgment: false
  - id: D3
    description: "The non-OIDC oauth2 Exchange path already populates OIDCClaims.Name (verified, no fix) and is now covered by an explicit regression test"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_Exchange_PopulatesName"
        status: pass
    human_judgment: false
  - id: D4
    description: "End-to-end against real Postgres, reconciliation fires on both interactive and non-interactive resolve paths, and the existing non-interactive login-sync gate test still passes unmodified"
    requirement: "AUTH-06"
    verification:
      - kind: integration
        ref: "internal/auth/identitystore_authn_integration_test.go#TestIdentityStore_DisplayNameReconciliation"
        status: pass
      - kind: integration
        ref: "internal/auth/identitystore_authn_integration_test.go#TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive"
        status: pass
    human_judgment: false

duration: 11min
completed: 2026-07-15
status: complete
---

# Phase 9 Plan 2: Introspection Claim Parity, JIT Seed Fix, and End-to-End Proof Summary

**Introspection logins now populate `claims.Name` (closing the third AUTH-06 resolution path), JIT provisioning seeds `display_name` from `claims.Name` at first login, and a real-Postgres integration test proves the whole feature reconciles unconditionally on both interactive and non-interactive resolves.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-07-15T18:20:23-04:00 (approx., first commit after 09-01)
- **Completed:** 2026-07-15T18:31:17-04:00
- **Tasks:** 3/3 completed
- **Files modified:** 5

## Accomplishments

- `resolveIntrospection` (`internal/auth/identitystore.go`) now sets `Name: nameFromClaims(res.Raw)` on the `OIDCClaims` it constructs — previously `claims.Name` was always empty on this path, so 09-01's unconditional reconciliation silently never fired for opaque-token/introspection logins (Pitfall 3, D-04).
- `jitResolve` (`internal/auth/identitystore.go`) now seeds a newly-provisioned user's `DisplayName` from `claims.Name` when non-empty, falling back to `claims.Subject` otherwise — eliminating the stale-fallback window for providers that supply a name at signup (D-07). Role derivation is unchanged (still solely from claims-mapping at JIT time).
- Confirmed (no code change) that the non-OIDC `oauth2LoginProvider.Exchange` path already populates `OIDCClaims.Name` via `displayNameFromUserinfo` (D-05 research finding), and added a dedicated `TestOAuth2Provider_Exchange_PopulatesName` regression test locking that guarantee in explicitly.
- Added `TestIntrospection_ReconcilesStaleDisplayName` proving a stale (`display_name == sub`) bound user self-heals to an introspected `"name"` claim, with role/email passed through unchanged.
- Added `TestJIT_SeedsDisplayNameFromClaimsName` (two sub-cases: Name-seed and Subject-fallback) proving the JIT seed preference.
- Added `TestIdentityStore_DisplayNameReconciliation`, a real-Postgres (testcontainers) integration test proving reconciliation fires on BOTH the interactive `ResolveLogin` path and the non-interactive bearer `Resolve` path, plus the SC4 no-regression direction (an operator-set `display_name` survives a resolve with no usable name claim). `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` was left unmodified and still passes.
- Verified `go test -short -race ./internal/auth/...`, `go test -tags integration -race ./internal/auth/...` (against real Postgres via testcontainers/Docker), and full `task check` (fmt:check → license:check → lint → build → unit tests, including the SvelteKit web build) all green.

## Task Commits

Each task was committed atomically:

1. **Task 1: Populate claims.Name on the introspection path and prove reconciliation fires there** - `ae86438c` (feat)
2. **Task 2: Seed jitResolve DisplayName from claims.Name (D-07) and add the JIT + oauth2 regression tests** - `5bdec238` (feat)
3. **Task 3: End-to-end integration test — reconciliation fires on both interactive and non-interactive resolves** - `9999b1f9` (test)

**Plan metadata:** (this commit) - `docs(09-02): complete introspection claim-parity, JIT seed fix, and end-to-end proof plan`

## Files Created/Modified

- `internal/auth/identitystore.go` - `resolveIntrospection` OIDCClaims literal gains `Name: nameFromClaims(res.Raw)`; `jitResolve` seed line replaced with a `seedName` preference (claims.Name, falling back to claims.Subject)
- `internal/auth/introspection_test.go` - New `TestIntrospection_ReconcilesStaleDisplayName`
- `internal/auth/identitystore_jit_test.go` - New `TestJIT_SeedsDisplayNameFromClaimsName` (Name-seed + Subject-fallback sub-tests)
- `internal/auth/oauth2_provider_test.go` - New `TestOAuth2Provider_Exchange_PopulatesName` (D-05 regression guard)
- `internal/auth/identitystore_authn_integration_test.go` - New `TestIdentityStore_DisplayNameReconciliation` (`//go:build integration`), three sub-tests: interactive reconciliation, non-interactive reconciliation, and SC4 no-usable-claim preservation

## Decisions Made

- Kept the introspection test WITHOUT `MCPResourceURI` (per the plan's explicit read-first guidance) so the aud/RFC 8707 audience check doesn't interfere with the reconciliation assertion being tested.
- Built a dedicated `usersBackendStub` for the introspection reconciliation test rather than reusing `introspectionBindingStub()`/`activeUser()`, since `activeUser` sets `DisplayName: "test-"+id`, which never equals the introspected `sub` and would never trigger the staleness heuristic.
- Used two separate JIT-created users (rather than resetting one user's `display_name` between sub-tests) for the interactive vs. non-interactive integration sub-tests, avoiding any inter-test ordering dependency while still proving both paths against the same real-Postgres harness (`authnTestStore`).
- For the integration test's interactive path, called `ResolveLogin` directly with hand-built `*auth.OIDCClaims` (matching the plan's explicit action text), rather than minting a JWT and calling `Resolve(auth.WithInteractiveLogin(ctx), token)` as the pre-existing login-sync gate test does — both are valid "interactive" invocations, but the plan named `ResolveLogin` specifically.

## Deviations from Plan

None — plan executed exactly as written. All three tasks matched their `<action>` specs; the integration test (Task 3) ran successfully against real Postgres via Docker/testcontainers in this environment, so no "written but not executed" flag is needed.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

AUTH-06 is now fully closed across all three resolution paths (JWT — 09-01; introspection — this plan; interactive callback — shared choke point from 09-01) and the JIT seed-time gap is closed (D-07). This was the last plan of Phase 9 and the last requirement of the v0.14.0 milestone's active scope (MCP-01 and CONV-01 already validated in Phases 6–8). No further phases are queued; `/gsd-new-milestone` is the next step per PROJECT.md's Operator Next Steps.

## Self-Check: PASSED

All modified files present on disk; all three task commits (`ae86438c`, `5bdec238`, `9999b1f9`) confirmed in `git log`.
