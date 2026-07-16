---
phase: 09-jit-display-name-reconciliation
fixed_at: 2026-07-16T18:00:00Z
review_path: .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 09: Code Review Fix Report

**Fixed at:** 2026-07-16T18:00:00Z
**Source review:** .planning/phases/09-jit-display-name-reconciliation/09-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (deep re-review — new WR-01, new WR-02, IN-01; the earlier
  WR-01/WR-02 from prior standard-depth rounds were already fixed and are not
  in this REVIEW.md)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### WR-01: `reconcileDisplayName` reports `changed=true` even when the new value is identical to the old one

**Files modified:** `internal/auth/identitystore.go`, `internal/auth/identitystore_test.go`
**Commit:** `8fea3297`
**Applied fix:** Added an explicit `claims.Name != user.DisplayName` check to
the staleness heuristic in `reconcileDisplayName`, so it now only reports
`changed=true` when the computed new name actually differs from what's
stored. This closes the "IdP where `name == sub`" infinite-reconciliation
case: previously, for a user whose `DisplayName` equals both `claims.Subject`
and `claims.Name` (the JIT-fallback case for such an IdP), the heuristic
fired `changed=true` on every login forever, producing a redundant
`UpdateUserOnLogin` write and a misleading `"auth: display-name reconciled"`
log with `old == new`. Added
`TestResolveJWT_ReconciliationNoOpWhenClaimNameEqualsSubject`, asserting
`UpdateUserOnLogin` is NOT called when `claims.Name == claims.Subject ==
user.DisplayName`.

### WR-02: Unconditional reconciliation widens the `UpdateUserOnLogin` TOCTOU window; the write has no optimistic-concurrency guard

> ⚠️ **SUPERSEDED (2026-07-16):** This point-in-time audit-trail snapshot recorded a
> documentation-only resolution below. A later deep-review iteration correctly rejected
> this resolution as invalid — the "both derive every written field from the same
> claims-vs-DB-row comparison" premise is false for `role` (`applyLoginSync` freshly
> re-derives it from claims-mapping; the standalone write blindly passes through a
> point-in-time read), making the race a real lost-update, not a benign convergence. See
> **CR-01** in `09-REVIEW.md`, fixed in commit `f28f8aa1` (added a narrower
> `UpdateDisplayNameOnLogin` method so the standalone path can no longer touch
> `role`/`email` at all). This snapshot is preserved for history; do not treat the
> "accepted tradeoff" conclusion below as current.

**Files modified:** `internal/auth/identitystore.go`, `internal/storage/postgres/users.go`
**Commit:** `16a833fe`
**Applied fix:** Evaluated the review's two suggested options (comment-only
tradeoff note vs. adding a `version` column + CAS check to
`UpdateUserOnLogin`). Chose the documentation-only tradeoff rather than the
schema/concurrency-control change, judging the schema change disproportionate
for the current blast radius: `UpdateUserOnLogin` has exactly two callers
today (`applyLoginSync` and `materializeIdentity`'s standalone reconciliation
write), and both derive every written field from the same
claims-vs-DB-row comparison, so two racing writes converge on the same
eventual value rather than clobbering an unrelated edit — a low-likelihood,
benign race, not a lost-update risk today. This is consistent with the
project's existing convention (ADR-004 / CLAUDE.md) that `RunInTransaction`
governs multi-query writes, and with the two already-landed review rounds
that judged the single-query `UpdateUserOnLogin` with a `deleted_at IS NULL`
guard sufficient for this phase.

Added an explicit doc comment on `UpdateUserOnLogin`
(`internal/storage/postgres/users.go`) documenting the missing version guard,
why it's accepted today, and the trigger condition for revisiting (the
moment a distinct write path — e.g. an admin profile-edit RPC — starts
calling this method). Added a cross-reference comment on
`reconcileDisplayName` (`internal/auth/identitystore.go`) pointing to the
tradeoff. No schema, behavior, or test change was required by this fix;
existing tests continue to pass unmodified.

### IN-01: No test covers the `ErrUserNotFound` fail-closed branch of the standalone reconciliation write outside the `resolveJWT` path

**Files modified:** `internal/auth/identitystore_test.go`, `internal/auth/introspection_test.go`
**Commit:** `8c02d797`
**Applied fix:** Added `TestIntrospection_ReconciliationUserNotFound_Denies`
(mirroring the existing `TestIntrospection_ReconcilesStaleDisplayName`
fixture pattern) and `TestResolveLogin_ReconciliationUserNotFound_Denies`
(interactive `ResolveLogin` entry point with `LoginSyncEnabled` left false so
the test exercises the standalone reconciliation write branch rather than
`applyLoginSync`'s gated one). Both assert `auth.ErrUnauthenticated` when the
standalone `UpdateUserOnLogin` call returns `storage.ErrUserNotFound`,
matching the existing `TestResolveJWT_ReconciliationUserNotFound_Denies`
coverage for the third (bearer-JWT) entry point. All three entry points
(`resolveJWT`, `resolveIntrospection`, `ResolveLogin`) now have equivalent
regression coverage for this shared fail-closed branch.

## Verification

- `go test -short -race ./internal/auth/...` — pass, including the 3 new
  tests (`TestResolveJWT_ReconciliationNoOpWhenClaimNameEqualsSubject`,
  `TestIntrospection_ReconciliationUserNotFound_Denies`,
  `TestResolveLogin_ReconciliationUserNotFound_Denies`) and all pre-existing
  tests unmodified/passing.
- `task check` — full pipeline (fmt:check, license:check, lint, build, unit
  tests across the whole module, plus the site/web build) passes with no
  regressions. No stale-worktree lint artifacts encountered.

## Skipped Issues

None — all findings in scope were fixed.

---

_Fixed: 2026-07-16T18:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
