---
phase: 09-jit-display-name-reconciliation
fixed_at: 2026-07-16T16:48:51Z
review_path: .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
iteration: 2
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-07-16T16:48:51Z
**Source review:** .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 1 (WR-02 — re-flagged; `fix_scope: all`)
- Fixed: 1
- Skipped: 0

## Fixed Issues

### WR-02: Reconciliation and login-sync writes remain non-atomic (re-flagged — not fixed, only documented)

**Files modified:** `internal/auth/identitystore.go`, `internal/auth/loginsync.go`, `internal/auth/loginsync_internal_test.go`
**Commit:** `c2e821c2`
**Applied fix:** Iteration 1's documentation-only fix was correctly rejected by
the re-reviewer — comments don't change runtime behavior. This iteration
applies the review's option (b): the reconciled display name is now threaded
into `applyLoginSync` as two new parameters (`newName string, nameChanged
bool`) so that, when the login-sync gate fires (`loginSyncEnabled &&
interactive`), the display-name change is folded into `applyLoginSync`'s
single `UpdateUserOnLogin` call alongside the role/email decision, rather than
being persisted separately beforehand.

Specifically:
- In `materializeIdentity` (`identitystore.go`), `reconcileDisplayName` is
  still called unconditionally, before the login-sync gate — its
  *computation* is untouched (preserves D-01). What changed is the *write*:
  when the gate fires, `newName`/`nameChanged` are passed into
  `applyLoginSync` instead of being persisted immediately. When the gate does
  **not** fire (login-sync disabled, or a non-interactive resolve), there is
  no combined write to fold into, so the reconciliation write still happens
  directly in `materializeIdentity`, exactly as before (same
  `ErrUserNotFound` classification, same best-effort-on-other-errors
  fallback) — this is the "no combined write available" branch the review's
  guidance called out, and it keeps D-01 (reconciliation independent of the
  gate) intact for that case.
- In `applyLoginSync` (`loginsync.go`), the no-op-skip guard now also checks
  `nameChanged` (previously it only checked role/email deltas), and the
  persisted display-name value is `newName` when `nameChanged`, else
  unchanged. The single `UpdateUserOnLogin` call now covers all three fields
  together. Failure classification (allowlist miss / `ErrUserNotFound` /
  demotion-fails-closed / promotion-best-effort / metadata-only-best-effort)
  is unchanged and still gated on `changed` (role/email) — `nameChanged` only
  affects whether a write is attempted and what gets persisted on success,
  never the classification branch taken on failure. Because the write is a
  single atomic `UpdateUserOnLogin` call, every denial path (allowlist miss,
  or a persist failure classified as demotion/deny) now performs **no write
  at all** — the display-name change is never committed ahead of a denial.
  This directly closes the gap the review described: "denied login = no DB
  effect" now holds for the display-name field too, in the case both
  concerns are in play together.

Recorded in `internal/auth/loginsync_internal_test.go`:
- Updated all ten pre-existing `TestApplyLoginSync_*` call sites to pass the
  new `(user.DisplayName, false)` arguments, preserving every previously
  asserted behavior unchanged (verified: none of those tests exercise a
  gate-fired name reconciliation, so `nameChanged=false` reproduces the exact
  prior code path in each case).
- Added four new tests:
  - `TestApplyLoginSync_NameChangeFoldedIntoRoleWrite` — both a name change
    and a role change in play; asserts exactly one `UpdateUserOnLogin` call
    persists both.
  - `TestApplyLoginSync_NameChangeAlonePersists` — a name-only change (no
    role/email delta) now triggers the write via the updated no-op-skip
    guard, where previously this case bypassed `applyLoginSync` entirely.
  - `TestApplyLoginSync_AllowlistMiss_NameChangeNotPersisted` — the direct
    WR-02 regression guard: an allowlist-denied login with a pending name
    change never calls `UpdateUserOnLogin` at all.
  - `TestApplyLoginSync_DemotionPersistFailure_NameChangeNotPersisted` — the
    second WR-02 regression guard: a failed combined write (classified as a
    denied demotion) is attempted exactly once and leaves the in-memory
    user's `DisplayName` untouched — the name change is part of the same
    failed atomic write, not a separate commit.

No new gap was introduced for the "gate does not fire" branch — that path is
untouched code, just relocated to an `else if` arm, and is still covered by
the existing `TestResolveJWT_ReconciliationRunsWithoutLoginSync`,
`TestResolveJWT_ReconciliationPreservesRoleAndEmail`, and
`TestResolveJWT_ReconciliationUserNotFound_Denies` (none of which set
`LoginSyncEnabled: true`, so all three exercise the direct-write branch
exactly as before).

## Verification

- `go test -short -race ./internal/auth/...` — all pass, including the four
  new tests and all pre-existing reconciliation/login-sync tests
  (`TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive`,
  `TestResolveJWT_ReconciliationRunsWithoutLoginSync`,
  `TestResolveJWT_ReconciliationPreservesRoleAndEmail`,
  `TestResolveJWT_ReconciliationUserNotFound_Denies`, and all ten
  pre-existing `TestApplyLoginSync_*` cases).
- `task check` — full pipeline (fmt:check, license:check, lint, build, unit
  tests across the whole module) passes with no regressions.
- `go build ./internal/auth/...` clean.

## Skipped Issues

None — the single in-scope finding was fixed.

---

_Fixed: 2026-07-16T16:48:51Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
