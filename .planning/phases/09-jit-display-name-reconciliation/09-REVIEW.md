---
phase: 09-jit-display-name-reconciliation
reviewed: 2026-07-16T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - internal/auth/identitystore.go
  - internal/auth/identitystore_authn_integration_test.go
  - internal/auth/identitystore_jit_test.go
  - internal/auth/identitystore_test.go
  - internal/auth/introspection_test.go
  - internal/auth/loginsync.go
  - internal/auth/loginsync_internal_test.go
  - internal/auth/oauth2_provider_test.go
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 09: Code Review Report (re-review, iteration 3 — final)

**Reviewed:** 2026-07-16T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** clean

## Summary

Previous iteration left one open finding: **WR-02** — the reconciliation
write in `materializeIdentity` and `applyLoginSync`'s role/email write were
two independent, sequential, non-transactional `UpdateUserOnLogin` calls
against the same row, so a denied/failed login could still leave a
display-name change committed. Commit `c2e821c2` ("fold display-name
reconciliation into single login-sync write") closes this in code, not just
in comments. I re-verified against the actual diff (`git show c2e821c2`),
the current source, and a full test run, plus did a fresh general pass over
all 8 files.

**1. WR-02 closure — verified in code, not just comments.**
`applyLoginSync` (`internal/auth/loginsync.go:80-155`) now takes
`(newName string, nameChanged bool)` as explicit parameters. Walking the
control flow:
- The email-allowlist check (step 1) returns `ErrUnauthenticated` *before*
  any write is attempted — a denied login on allowlist grounds performs zero
  writes, so a pending name change is never persisted ahead of the denial.
  Confirmed by `TestApplyLoginSync_AllowlistMiss_NameChangeNotPersisted`.
- The no-op-skip guard was correctly widened to
  `newEmail == user.Email && newRole == user.Role && !nameChanged` (previously
  two conditions) — a name-only change no longer short-circuits and now
  drives the single write. Confirmed by
  `TestApplyLoginSync_NameChangeAlonePersists` (name-only change persists,
  exactly one call) and `TestApplyLoginSync_NoOpSkipsWrite` (true no-op still
  skips entirely, `nameChanged=false`).
- The persisted name (`persistName`) folds into the *same*
  `UpdateUserOnLogin` call that decides role/email
  (`internal/auth/loginsync.go:108-114`), so success or failure is atomic
  across all three fields for one SQL statement. On a demotion-persist
  failure (`changed && !isPromotion`), the function returns
  `nil, ErrTransient` **without** mutating the in-memory `user` struct and
  without the write having partially applied — confirmed by
  `TestApplyLoginSync_DemotionPersistFailure_NameChangeNotPersisted`, which
  explicitly asserts `user.DisplayName` is still the pre-login value ("sub-1")
  after the failed combined write, and that the backend was called exactly
  once.
- `TestApplyLoginSync_NameChangeFoldedIntoRoleWrite` proves the promotion +
  name-change case: exactly one `UpdateUserOnLogin` call carries both the new
  role and the new name together.

There is no longer any reachable path where a denied or failed login leaves
a partial display-name write committed — the single-write invariant now
holds for all three fields (name, role, email) whenever the login-sync gate
fires.

**2. D-01 preservation — verified.** `materializeIdentity`
(`internal/auth/identitystore.go:543-624`) still computes
`reconcileDisplayName` unconditionally, before the `s.loginSyncEnabled &&
interactive` gate. When the gate does *not* fire (login-sync disabled, or a
non-interactive resolve), the pre-existing `else if nameChanged` branch
performs its own independent `UpdateUserOnLogin` call and its own
`ErrUserNotFound` fail-closed handling — unchanged from before this commit.
This is exercised end-to-end by `TestResolveJWT_ReconciliationRunsWithoutLoginSync`
(non-interactive, `LoginSyncEnabled` omitted/false),
`TestIdentityStore_DisplayNameReconciliation` (both the interactive
`ResolveLogin` path and the non-interactive `Resolve` path, against real
Postgres), and `TestResolveJWT_ReconciliationUserNotFound_Denies`.

**3. Regression-guard tests — verified they still exercise the intended
behavior, not just that they compile.**
- `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` seeds the DB user
  with `DisplayName: "Carol"`, which is never equal to the token's `sub`
  (`"oidc-subject-loginsync"`), so `reconcileDisplayName` always returns
  `nameChanged=false` in this test — the new parameters are inert and the
  test's actual assertion (role sync gated on `interactive`) is unaffected by
  the refactor.
- `TestResolveJWT_ReconciliationRunsWithoutLoginSync`,
  `TestResolveJWT_ReconciliationPreservesRoleAndEmail`, and
  `TestResolveJWT_ReconciliationUserNotFound_Denies` all exercise the `else if
  nameChanged` branch in `materializeIdentity`, which this commit left
  structurally the same (only relocated below the now-unconditional
  `reconcileDisplayName` call, which was already unconditional before this
  commit). All three pass under `go test ./internal/auth/...`.

**4. No new bugs from parameter threading.** The single production call site
(`internal/auth/identitystore.go:588`) passes `newName, nameChanged` straight
through from the one upstream `reconcileDisplayName` call — no duplicate
computation, no stale value reuse, no re-derivation. All 10 pre-existing
white-box call sites in `loginsync_internal_test.go` were updated to pass
`(user.DisplayName, false)`, correctly preserving their original
no-name-change semantics (verified each one individually — none accidentally
passed a mismatched `newName`/`nameChanged` pair). The 4 new regression tests
correctly pass a changed name with `nameChanged=true` where the scenario
calls for it. I also checked: `isPromotion`'s classification is computed
solely from `resolveLoginRole`'s `changed` output, independent of
`nameChanged`, so a pending name-only reconciliation cannot flip a
metadata-only failure into a fail-closed demotion-deny or vice versa (matches
the updated docstring's claim). Log statements referencing `user.Role` /
`user.DisplayName` as "old" values are all emitted before the corresponding
mutation in the success path, so there's no stale-vs-fresh logging bug.

`go build ./internal/auth/...` and `go test ./internal/auth/...` both pass
clean.

A general pass over all 8 files — including the two unrelated to this fix
(`introspection_test.go`, `oauth2_provider_test.go`) and the integration test
file (`identitystore_authn_integration_test.go`) — found no additional bugs,
security gaps, or quality regressions. Dispatch routing (`Resolve` →
API-key/session/JWT/introspection), the JIT rate limiter, the email-domain
allowlist, claims-mapping's JIT-only scoping, and the role-rank/clamp helpers
are all unchanged from the prior (already-reviewed) iterations and remain
correct.

No findings to report. This closes the WR-02 review/fix loop.

---

_Reviewed: 2026-07-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
