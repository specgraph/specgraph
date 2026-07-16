---
phase: 09-jit-display-name-reconciliation
fixed_at: 2026-07-16T16:28:20Z
review_path: .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-07-16T16:28:20Z
**Source review:** .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (WR-01, WR-02, IN-01 — `fix_scope: all`)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### WR-01: `reconcileDisplayName`'s persist failure doesn't distinguish a concurrently soft-deleted user, unlike `applyLoginSync`

**Files modified:** `internal/auth/identitystore.go`, `internal/auth/identitystore_test.go`
**Commit:** `2a864b69`
**Applied fix:** Added an `errors.Is(err, storage.ErrUserNotFound)` branch in the
reconciliation write inside `materializeIdentity`, mirroring `applyLoginSync`'s
classification: on a concurrently soft-deleted user (the active-row guard
`deleted_at IS NULL` matching nothing), log a warning and return
`ErrUnauthenticated` instead of proceeding as a best-effort write failure.
Added `TestResolveJWT_ReconciliationUserNotFound_Denies`, mirroring the
existing `TestApplyLoginSync_UserNotFound_Denies`, asserting `store.Resolve`
returns `ErrUnauthenticated` when the reconciliation write's
`UpdateUserOnLogin` stub returns `storage.ErrUserNotFound`. Both the new test
and the full `internal/auth` suite pass under `-race`.

### WR-02: A denied login can still leave the display-name write persisted

**Files modified:** `internal/auth/identitystore.go`, `internal/auth/loginsync.go`
**Commit:** `f80dcf90`
**Applied fix:** Chose option (a) from the review's fix guidance — documented the
non-atomicity as an accepted tradeoff rather than attempting the more invasive
single-write refactor (option (b)), since combining the two writes would
require re-threading the reconciled display name into `applyLoginSync` as an
input and I could not verify with high confidence that no existing test or
caller relies on the current two-write ordering/behavior. Added an explicit
comment block at the reconciliation call site in `materializeIdentity`
(`identitystore.go`) explaining that the reconciliation write and
`applyLoginSync`'s write are independent, non-transactional
`UpdateUserOnLogin` calls, and that a subsequent denial by `applyLoginSync`
does not roll back an already-committed display-name change. Added a
corresponding pointer comment at `applyLoginSync`'s doc comment
(`loginsync.go`) noting the same non-atomicity from that side.

### IN-01: Two independent `UpdateUserOnLogin` round-trips when both display name and role/email change

**Files modified:** `internal/auth/identitystore.go` (covered by the WR-02 commit)
**Commit:** `f80dcf90`
**Applied fix:** No separate action was required — this finding is explicitly
tied to WR-02 in the review ("root cause of WR-02... worth a short pointer").
The WR-02 documentation fix's closing note ("Folding both writes into a
single `UpdateUserOnLogin` call would close this gap... but requires
threading the reconciled name into `applyLoginSync` as an input") directly
addresses IN-01's awareness point for a future reader.

## Verification

- `go test -short -race ./internal/auth/...` — all pass (including the new
  `TestResolveJWT_ReconciliationUserNotFound_Denies` and the pre-existing
  `TestApplyLoginSync_UserNotFound_Denies` it mirrors).
- `task check` — full pipeline (fmt:check, license:check, lint, build, unit
  tests) passes with no regressions.
- `gofmt -l` and `go vet ./internal/auth/...` clean on all modified files.

## Skipped Issues

None — all in-scope findings were fixed.

---

_Fixed: 2026-07-16T16:28:20Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
