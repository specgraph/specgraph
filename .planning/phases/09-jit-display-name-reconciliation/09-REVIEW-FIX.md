---
phase: 09-jit-display-name-reconciliation
fixed_at: 2026-07-16T18:24:41Z
review_path: .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
iteration: 2
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-07-16T18:24:41Z
**Source review:** .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 2 (deep re-review iteration 3 — CR-01, reopened from the
  prior round's WR-02 "accepted tradeoff" resolution; IN-02)
- Fixed: 2
- Skipped: 0

## Fixed Issues

### CR-01: WR-02's "benign convergent write" rationale is false — a concurrent standalone reconciliation write can silently revert a role change

**Files modified:** `internal/storage/users.go`, `internal/storage/postgres/users.go`, `internal/auth/identitystore.go`, `internal/auth/identitystore_test.go`, `internal/auth/introspection_test.go`, `internal/auth/usersbackend_stub_test.go`, `internal/server/usersbackend_stub_test.go`
**Commit:** `f28f8aa1`
**Applied fix:** Added a narrower `UpdateDisplayNameOnLogin(ctx, userID,
displayName string) error` method to `storage.UsersBackend` and implemented
it in `internal/storage/postgres/users.go` (a single-column `UPDATE ...
SET display_name = $1 WHERE id = $2::uuid AND deleted_at IS NULL`, mirroring
the existing `deleted_at IS NULL` guard pattern). Switched
`materializeIdentity`'s standalone reconciliation branch (`else if
nameChanged` — the one that fires when the `loginSyncEnabled && interactive`
gate does NOT) to call this narrower method instead of the 3-field
`UpdateUserOnLogin`. `applyLoginSync`'s own combined write (when the gate
DOES fire) is unchanged — it still legitimately writes role/email/name
together via `UpdateUserOnLogin`.

This removes the race by construction: with the standalone path no longer
touching role or email at all, there is nothing left for it to clobber, so
a concurrent interactive login-sync demotion (or any other role/email
change) can never be silently reverted by a racing non-interactive
reconciliation write.

Updated both `usersBackendStub` test doubles (`internal/auth` and
`internal/server`) to implement the new interface method. Updated tests
exercising the standalone branch (`TestResolveJWT_ReconcilesStaleDisplayName`,
`TestResolveJWT_ReconciliationRunsWithoutLoginSync`,
`TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim`,
`TestResolveJWT_ReconciliationNoOpWhenClaimNameEqualsSubject`,
`TestResolveJWT_ReconciliationUserNotFound_Denies`,
`TestResolveLogin_ReconciliationUserNotFound_Denies`,
`TestIntrospection_ReconcilesStaleDisplayName`,
`TestIntrospection_ReconciliationUserNotFound_Denies`) to assert on
`UpdateDisplayNameOnLogin` instead of `UpdateUserOnLogin`. Where a test
previously asserted role/email pass-through via the write's call
arguments, it now asserts against the fields of the returned `Identity`
instead, since the narrower write has no role/email parameters to inspect.

Removed `TestResolveJWT_ReconciliationPreservesRoleAndEmail`: its premise —
asserting that a 3-arg `UpdateUserOnLogin` call made by the standalone
branch passes role/email through unchanged — is now structurally
impossible to express, since that branch no longer calls `UpdateUserOnLogin`
at all. The invariant it protected is now enforced at the type level and is
additionally covered by the new IN-02 contract test (see below); a pointer
comment was left in its place explaining the removal.

Removed the now-inaccurate "accepted tradeoff" comments (added by the
previous WR-02 fix) on both `UpdateUserOnLogin`
(`internal/storage/postgres/users.go`) and `reconcileDisplayName`
(`internal/auth/identitystore.go`), replacing them with comments describing
the actual current state: `UpdateUserOnLogin` now has exactly one caller
(`applyLoginSync`), and the standalone path's write is structurally
incapable of racing it on role/email.

### IN-02: No contract test guards against CR-01 recurring

**Files modified:** `internal/auth/identitystore_test.go`
**Commit:** `99a333a7`
**Applied fix:** Added
`TestMaterializeIdentity_StandaloneReconciliationNeverCallsUpdateUserOnLogin`,
which drives a non-interactive bearer-JWT resolve through the standalone
reconciliation branch (login-sync disabled) and asserts the stub's
`updateDisplayNameOnLogin` hook is called exactly once while its
`updateUserOnLogin` (3-field) hook is called zero times. This turns "the
standalone path cannot touch role/email" from an implicit code-review
observation into an explicit, enforced regression test that will fail
loudly if a future change reintroduces a call to `UpdateUserOnLogin` from
that branch.

## Verification

- `go test -short -race ./internal/auth/...` — pass, including the new
  `TestMaterializeIdentity_StandaloneReconciliationNeverCallsUpdateUserOnLogin`
  and all pre-existing tests (updated for the new method) passing.
- `go test -short -race ./internal/storage/...` — pass.
- `golangci-lint run` scoped to `internal/storage/...`, `internal/auth/...`,
  `internal/server/...` — 0 issues.
- `task check` — full pipeline (fmt:check, license:check, lint, build, unit
  tests across the whole module, plus the site/web build) passes with no
  regressions. No stale-worktree lint artifacts encountered.

## Skipped Issues

None — both findings in scope were fixed.

---

_Fixed: 2026-07-16T18:24:41Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
