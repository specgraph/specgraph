---
phase: 09-jit-display-name-reconciliation
plan: 01
subsystem: auth
tags: [oidc, jwt, identity, go, login-sync]

requires:
  - phase: 03-coordination-export
    provides: "materializeIdentity / applyLoginSync gate (loginSyncEnabled && interactive), OIDCClaims.Name via nameFromClaims"
provides:
  - "reconcileDisplayName helper implementing the D-03 staleness heuristic (display_name == subject → self-heal from a usable name claim)"
  - "Unconditional reconciliation call site in materializeIdentity's existing-user branch, decoupled from LoginSyncEnabled and interactive"
  - "applyLoginSync narrowed to role/email/allowlist only — no longer computes or writes display_name"
affects: ["09-02 (introspection path, jitResolve seed, oauth2 regression test, integration test)"]

tech-stack:
  added: []
  patterns:
    - "Unconditional side-effect precedent (mirrors resolveAPIKey's s.tracker.Touch): a helper that runs regardless of gates, guarded internally by its own `changed` boolean, with a best-effort (log-and-proceed) failure mode so it never denies a login."

key-files:
  created: []
  modified:
    - internal/auth/identitystore.go
    - internal/auth/identitystore_test.go
    - internal/auth/loginsync.go
    - internal/auth/loginsync_internal_test.go

key-decisions:
  - "reconcileDisplayName does not accept, thread, or read the `interactive` flag (D-01) — reconciliation is unconditional by construction, not by omission of a check."
  - "The write is guarded by the `changed` boolean owned by the caller, since UpdateUserOnLogin is not SQL-level no-op-safe (Pitfall 4)."
  - "In-memory user.DisplayName is updated BEFORE the loginSyncEnabled && interactive gate runs, so applyLoginSync sees the already-reconciled value and never re-derives it (Pitfall 2, no double-write)."
  - "A persist failure on reconciliation is logged at WARN and the login proceeds — display-name is lower-sensitivity than role, so it is never a login-denial reason."

patterns-established:
  - "Best-effort, gate-independent identity-metadata reconciliation: compute + guard-by-changed + write + best-effort-log-on-failure, positioned between the soft-delete check and any interactive-only sync gate."

requirements-completed: [AUTH-06]

coverage:
  - id: D1
    description: "A stale (display_name == subject) existing OIDC user is reconciled to a usable name/preferred_username claim on JWT login, with role/email passed through unchanged"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_test.go#TestResolveJWT_ReconcilesStaleDisplayName"
        status: pass
      - kind: unit
        ref: "internal/auth/identitystore_test.go#TestResolveJWT_ReconciliationPreservesRoleAndEmail"
        status: pass
    human_judgment: false
  - id: D2
    description: "Reconciliation runs unconditionally — decoupled from both LoginSyncEnabled and the interactive/non-interactive distinction"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_test.go#TestResolveJWT_ReconciliationRunsWithoutLoginSync"
        status: pass
    human_judgment: false
  - id: D3
    description: "An operator-set display_name, or a login with no usable name claim, is left unchanged with no UpdateUserOnLogin write"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_test.go#TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim"
        status: pass
    human_judgment: false
  - id: D4
    description: "applyLoginSync no longer computes or writes display_name; role/email/allowlist behavior is byte-for-byte unchanged"
    requirement: "AUTH-06"
    verification:
      - kind: unit
        ref: "internal/auth/loginsync_internal_test.go#TestApplyLoginSync_PromotesAndRefreshesMetadata"
        status: pass
      - kind: unit
        ref: "internal/auth/loginsync_internal_test.go#TestApplyLoginSync_PreservesOperatorRename"
        status: pass
      - kind: unit
        ref: "go test -short -race ./internal/auth/..."
        status: pass
    human_judgment: false

duration: 16min
completed: 2026-07-15
status: complete
---

# Phase 9 Plan 1: Unconditional Display-Name Reconciliation Summary

**Stale JIT-fallback `display_name` (equal to the OIDC subject) now self-heals to a usable `name`/`preferred_username` claim on every JWT login, decoupled from both `LoginSyncEnabled` and the interactive/non-interactive gate, via a new `reconcileDisplayName` helper — with the redundant computation removed from `applyLoginSync`.**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-07-15T18:09:00-04:00 (approx.)
- **Completed:** 2026-07-15T18:12:39-04:00
- **Tasks:** 2/2 completed
- **Files modified:** 4

## Accomplishments

- Added `reconcileDisplayName(user *storage.User, claims *OIDCClaims) (newName string, changed bool)` in `internal/auth/identitystore.go`, implementing the D-03 staleness heuristic (`user.DisplayName == claims.Subject && claims.Name != ""`) with no dependency on the `interactive` flag.
- Wired an unconditional call site into `materializeIdentity`'s existing-user branch, positioned after the soft-delete check and before the `loginSyncEnabled && interactive` gate — guarded by `changed`, writing via `UpdateUserOnLogin` with `user.Email`/`user.Role` passed through unchanged, best-effort on persist failure (never denies login), and updating `user.DisplayName` in memory before the gate runs.
- Added four new JWT-path unit tests covering reconciliation, gate-decoupling, preservation (operator-set name and no-usable-claim), and role/email non-leakage.
- Removed the now-redundant display-name computation from `applyLoginSync` (local `newDisplay` derivation, the no-op comparison term, and the success mutation), narrowing it to role/email/allowlist only; `user.DisplayName` passes through unchanged at the `UpdateUserOnLogin` call site.
- Updated the two affected white-box tests (`TestApplyLoginSync_PromotesAndRefreshesMetadata`, `TestApplyLoginSync_PreservesOperatorRename`) to assert the pass-through value, per the explicitly planned Pitfall-2 diff.
- Verified `task check` (fmt:check → license:check → lint → build → full unit suite, including web build) is green.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add reconcileDisplayName helper and wire it unconditionally into materializeIdentity** - `d9b2bb93` (feat)
2. **Task 2: Remove the display-name block from applyLoginSync and update its two white-box tests** - `25c9fd65` (refactor)

**Plan metadata:** (this commit) - `docs(09-01): complete unconditional display-name reconciliation plan`

## Files Created/Modified

- `internal/auth/identitystore.go` - New `reconcileDisplayName` helper + unconditional call site in `materializeIdentity`
- `internal/auth/identitystore_test.go` - Four new JWT-path tests (`TestResolveJWT_ReconcilesStaleDisplayName`, `TestResolveJWT_ReconciliationRunsWithoutLoginSync`, `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim`, `TestResolveJWT_ReconciliationPreservesRoleAndEmail`)
- `internal/auth/loginsync.go` - `applyLoginSync` narrowed to role/email/allowlist; display-name pass-through
- `internal/auth/loginsync_internal_test.go` - Two updated white-box tests reflecting the pass-through behavior

## Decisions Made

- Placed `reconcileDisplayName` immediately before `materializeIdentity` in `identitystore.go` for locality with its single call site, rather than alongside `applyLoginSync` in `loginsync.go` — it is conceptually part of the unconditional resolve path, not the gated sync path.
- Did not add `slog.Bool("audit", true)` to the reconciliation log records (matching the plan's explicit guidance that display-name is lower-sensitivity than role, which does carry the audit flag in `applyLoginSync`).
- Test users for the four new JWT-path tests are built via a literal `storage.User{}` (not the `activeUser` helper, which sets `DisplayName: "test-"+id` and would never trigger the staleness heuristic), per the plan's explicit read-first guidance.

## Deviations from Plan

None — plan executed exactly as written. Both tasks matched their `<action>` specs; the two white-box test updates in Task 2 were explicitly pre-planned (RESEARCH.md Pitfall 2), not an unplanned regression.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 09-02 (introspection path, `jitResolve` seed-time fix, oauth2 regression test, and the Postgres integration test) depends on this plan and can proceed: `reconcileDisplayName` is a stable, reusable choke point that 09-02's introspection call site will also invoke, and the JWT/interactive-callback paths covered here are fully green (`go test -short -race ./internal/auth/...` and `task check` both pass).

## Self-Check: PASSED

All modified files present on disk; both task commits (`d9b2bb93`, `25c9fd65`) and the summary commit (`fb2725ca`) confirmed in `git log`.
