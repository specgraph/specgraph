---
phase: 09-jit-display-name-reconciliation
reviewed: 2026-07-16T18:34:39Z
depth: deep
files_reviewed: 12
files_reviewed_list:
  - internal/auth/identitystore.go
  - internal/auth/identitystore_authn_integration_test.go
  - internal/auth/identitystore_jit_test.go
  - internal/auth/identitystore_test.go
  - internal/auth/introspection_test.go
  - internal/auth/loginsync.go
  - internal/auth/loginsync_internal_test.go
  - internal/auth/oauth2_provider_test.go
  - internal/storage/users.go
  - internal/storage/postgres/users.go
  - internal/auth/usersbackend_stub_test.go
  - internal/server/usersbackend_stub_test.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 09: Code Review Report (deep re-review, iteration 3 — final)

**Reviewed:** 2026-07-16T18:34:39Z
**Depth:** deep
**Files Reviewed:** 12
**Status:** issues_found

## Summary

This is the third and final deep-review iteration for AUTH-06 (#994), cap reached. This
round verifies the CR-01 fix from the previous iteration, re-derives its correctness
independently rather than trusting the fixer's changelog, traces the full call graph for
`UpdateUserOnLogin` / `UpdateDisplayNameOnLogin` across the entire repository (not just the
files in scope), and adds the two storage files (`internal/storage/users.go`,
`internal/storage/postgres/users.go`) newly in scope this round. I ran `go build
./internal/...`, `go vet ./internal/auth/... ./internal/storage/...`, and `go test
./internal/auth/... ./internal/storage/...` — all green — and diffed the exact CR-01 commit
(`f28f8aa1`) directly rather than relying on its commit message.

**CR-01 verdict: the fix is correct and structurally sound. No BLOCKER findings this round.**

1. **Genuinely narrower write, confirmed at the SQL level.**
   `storage.UsersBackend.UpdateDisplayNameOnLogin(ctx, userID, displayName)` is implemented
   in `internal/storage/postgres/users.go:272-282` as `UPDATE users SET display_name = $1
   WHERE id = $2::uuid AND deleted_at IS NULL` — it has no `email`/`role` parameters and no
   code path can make it touch either column. This is not merely "documented narrower"; it
   is structurally narrower — there is nothing to pass in that would let it write those
   fields even if a caller wanted to.
2. **No remaining path back to `UpdateUserOnLogin` from the standalone branch.** A
   repo-wide grep for `UpdateUserOnLogin` call sites (across `internal/bootstrap`,
   `internal/server`, `internal/auth/usagetracker`, and all of `internal/auth`) confirms
   exactly one production caller remains: `applyLoginSync` in
   `internal/auth/loginsync.go:114`. `materializeIdentity`'s standalone reconciliation
   branch (`internal/auth/identitystore.go:605`) calls only `UpdateDisplayNameOnLogin`. The
   lost-update race identified in the prior round (a concurrent non-interactive resolve
   silently reverting a role revocation processed moments earlier by an interactive
   login-sync write) is eliminated by construction: the two write paths now touch disjoint
   column sets, so there is nothing left to race on `role`/`email`.
3. **Both `usersBackendStub` test doubles implement the new method consistently.**
   `internal/auth/usersbackend_stub_test.go:142-147` and
   `internal/server/usersbackend_stub_test.go:101-106` both add
   `UpdateDisplayNameOnLogin` following the exact same fail-loud-by-default pattern already
   used for `UpdateUserOnLogin` in each file (a nil func field returns
   `errUnexpectedCall`/`errUnexpected`). No divergence in default behavior between the two
   stubs. Both retain their compile-time `var _ storage.UsersBackend = (*...)(nil)`
   assertions, and the build is clean.
4. **IN-02's regression test is a real guard, not a placebo.**
   `TestMaterializeIdentity_StandaloneReconciliationNeverCallsUpdateUserOnLogin`
   (`internal/auth/identitystore_test.go:923-969`) wires both `updateDisplayNameOnLogin`
   and `updateUserOnLogin` hooks on the same stub instance and asserts call counts (1 vs.
   0). A future refactor that reintroduced the old 3-field call under the standalone branch
   — even under a different code path or helper name — would increment `userOnLoginCalls`
   and fail the `require.Equal(t, 0, userOnLoginCalls, ...)` assertion. This is not
   trivially defeated: the interface's method set is fixed, both stubs' unconfigured
   mutating methods fail loud by default, and the two hooks are independently observable in
   this test.

Two residual gaps remain — both coverage gaps, not behavioral defects in the shipped code —
plus one bookkeeping note about a file in scope with no phase-relevant content. Since this
is the final iteration in the deep-review/fix loop (cap reached), these are recorded below
as WARNING-level backlog items for a follow-up pass rather than blocking closure; neither
represents a functional defect in the CR-01 fix itself.

## Warnings

### WR-01: No integration test for the new `AuthStore.UpdateDisplayNameOnLogin` SQL method

**File:** `internal/storage/postgres/users.go:272-282` (implementation); no corresponding
test exists in `internal/storage/postgres/users_test.go`

**Issue:** The sibling method `UpdateUserOnLogin` has direct Postgres-backed coverage
(`TestAuthStore_UpdateUserOnLogin`, `internal/storage/postgres/users_test.go:301`),
exercising the success path, the `deleted_at IS NULL` guard (→ `ErrUserNotFound`), and a
second successful update. `UpdateDisplayNameOnLogin` — the new method that structurally
carries the entire CR-01 fix — has zero direct test coverage against a real database. It is
only exercised indirectly through mocked `usersBackendStub` hooks in the `internal/auth`
package tests, which prove the *auth* package calls it correctly but say nothing about
whether the SQL itself behaves as documented: that it truly leaves `email`/`role` untouched,
that the `deleted_at IS NULL` guard rejects soft-deleted rows as expected, and that
`RowsAffected() == 0` correctly maps to `ErrUserNotFound` against the real driver.
`internal/storage/postgres/users.go` and `internal/storage/users.go` were newly added to
this iteration's scope; this gap was not visible to the prior two rounds, which never had
these files in scope.

**Fix:** Add a `TestAuthStore_UpdateDisplayNameOnLogin` to
`internal/storage/postgres/users_test.go` mirroring `TestAuthStore_UpdateUserOnLogin`:
create a user, call `UpdateDisplayNameOnLogin`, read the row back and assert `email`/`role`
are unchanged while `display_name` updated; assert `ErrUserNotFound` on a missing or
soft-deleted user ID.

### WR-02: No full-pipeline test proving the combined write correctly threads a reconciled display name through when the login-sync gate fires

**File:** `internal/auth/identitystore.go:597-603` (the `materializeIdentity` →
`applyLoginSync` call site); closest existing coverage is
`TestApplyLoginSync_NameChangeFoldedIntoRoleWrite`
(`internal/auth/loginsync_internal_test.go:274-297`) and
`TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive`
(`internal/auth/identitystore_authn_integration_test.go:148-214`)

**Issue:** The task asked me to check whether removing
`TestResolveJWT_ReconciliationPreservesRoleAndEmail` opened a coverage gap for "the case
this test didn't cover" — the combined-write path when the login-sync gate DOES fire. It
did, partially:

- `TestApplyLoginSync_NameChangeFoldedIntoRoleWrite` calls `s.applyLoginSync(...)` directly
  (white-box, same package) with hand-constructed `newName`/`nameChanged` arguments. It
  proves `applyLoginSync` folds a *given* name change into the combined write correctly,
  but does not exercise `materializeIdentity`'s computation of those two values via
  `reconcileDisplayName`, nor their threading into the call at the real call site
  (`identitystore.go:599`).
- `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` (integration-tagged, real
  Postgres) exercises the full pipeline with `LoginSyncEnabled: true` end-to-end, but the
  seeded user's `DisplayName` is `"Carol"` — never equal to the OIDC subject — so
  `reconcileDisplayName` never fires and no name change is ever folded in; only the role
  change is asserted.
- `TestIdentityStore_DisplayNameReconciliation` (same integration file) exercises name
  reconciliation end-to-end via `ResolveLogin`/`Resolve`, but its `authnTestStore` helper
  (`identitystore_authn_integration_test.go:28-57`) never sets `LoginSyncEnabled: true`, so
  every sub-test there takes the *standalone* `UpdateDisplayNameOnLogin` branch, never
  `applyLoginSync`'s combined-write branch, even for the sub-test explicitly labeled
  "interactive resolve".

No test in the suite exercises the full call chain (`resolveJWT`/`ResolveLogin` →
`materializeIdentity` → `applyLoginSync`) with `LoginSyncEnabled: true`, an interactive
login, a role change taking effect, AND a genuinely stale display name (`== subject`) with a
`name` claim present, asserting that the single combined write persists all three fields
together. The wiring between `reconcileDisplayName`'s output and `applyLoginSync`'s
parameters is two lines of trivial pass-through code today, so the immediate risk is low —
but it is exactly the kind of code a future refactor (e.g., reordering the
`newName, nameChanged := reconcileDisplayName(...)` call relative to the login-sync gate, or
changing what gets passed) could silently break without any test catching it.

**Fix:** Add a black-box test — either an addition to
`identitystore_authn_integration_test.go`'s existing suite, or a new stub-based test in
`identitystore_test.go` alongside
`TestMaterializeIdentity_StandaloneReconciliationNeverCallsUpdateUserOnLogin` — that: seeds
a user with `DisplayName == subject` (stale/JIT-fallback), enables `LoginSyncEnabled` with a
claims mapping that changes the role, resolves via `ResolveLogin` (or `Resolve` +
`WithInteractiveLogin`) with a `name` claim present, and asserts the single
`UpdateUserOnLogin` call (or persisted row, for the integration variant) carries both the
new role AND the reconciled display name together.

## Info

### IN-01: `oauth2_provider_test.go` is in scope but unrelated to AUTH-06's reconciliation fix chain

**File:** `internal/auth/oauth2_provider_test.go`

**Issue:** Not a defect — recorded for the record since the file was included in this
iteration's explicit file list. It tests `oauth2LoginProvider.Exchange`/`AuthCodeURL`
(GitHub-style OAuth2 login, email verification fallback, name population) and has no code
path touching `reconcileDisplayName`, `applyLoginSync`, `UpdateUserOnLogin`, or
`UpdateDisplayNameOnLogin`. It was read in full and is correct as written; it simply does
not exercise anything specific to CR-01/WR-01/WR-02/IN-02.

**Fix:** None needed — informational only, confirming this file was read and considered
during the final pass.

---

_Reviewed: 2026-07-16T18:34:39Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
