---
phase: 09-jit-display-name-reconciliation
verified: 2026-07-15T22:55:38Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 9: JIT Display-Name Reconciliation Verification Report

**Phase Goal:** On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable name claim, replacing a stale subject-hash fallback.
**Verified:** 2026-07-15T22:55:38Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | On login, a JIT user's `display_name` is updated when a usable `name`/`preferred_username` claim becomes available. | VERIFIED | `reconcileDisplayName` (internal/auth/identitystore.go:529-534) implements the staleness heuristic and is called unconditionally in `materializeIdentity` (line 577). `TestResolveJWT_ReconcilesStaleDisplayName` and `TestIntrospection_ReconcilesStaleDisplayName` both re-run fresh in this session and PASS, proving the state transition (stale → reconciled) on both the JWT and introspection paths. |
| 2 | A stale subject-hash fallback `display_name` is replaced once a usable claim appears. | VERIFIED | Same heuristic (`user.DisplayName == claims.Subject && claims.Name != ""`) — the D-03 subject-hash detection is exactly the described fallback signal. `TestResolveJWT_ReconciliationPreservesRoleAndEmail` and the integration sub-test `interactive_resolve_(ResolveLogin)_reconciles_a_stale_display_name` confirm the DB row is actually updated (re-run against real Postgres in this session, PASS). |
| 3 | Reconciliation runs on every successful login, not only at first provisioning. | VERIFIED | All three resolution entry points converge on the single choke point `materializeIdentity`: `ResolveLogin` (line 229, `interactive=true`), `resolveJWT`/`Resolve` (line 513, interactive derived from context), and `resolveIntrospection` (line 658, `interactive=false`). `reconcileDisplayName`'s call site (line 577) sits ABOVE the `s.loginSyncEnabled && interactive` gate (line 588) and does not read the `interactive` flag at all. `TestResolveJWT_ReconciliationRunsWithoutLoginSync` (LoginSyncEnabled=false, non-interactive `store.Resolve`) and the fresh integration run of `TestIdentityStore_DisplayNameReconciliation` (both `ResolveLogin` and non-interactive `Resolve` sub-tests) PASS in this session against real Postgres. |
| 4 | When no usable claim is present, the existing `display_name` is preserved (no regression back to a subject-hash value). | VERIFIED | `reconcileDisplayName` returns `(user.DisplayName, false)` when `claims.Name == ""` or the stored name isn't stale. `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim` (both sub-cases: operator-set name with claim present, and stale-but-no-claim) and the integration sub-test `no_usable_name_claim_leaves_an_operator-set_display_name_unchanged_(SC4)` PASS fresh in this session; the stub's `updateUserOnLogin` capture is wired to `t.Fatal` on any spurious write, so a false-positive "unchanged" claim would fail loudly. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/auth/identitystore.go` — `reconcileDisplayName` | New unexported helper implementing D-03 heuristic | VERIFIED | Present at lines 516-534; does not accept/read `interactive` or call `InteractiveLoginFromContext` (confirmed by direct read). |
| `internal/auth/identitystore.go` — `materializeIdentity` call site | Unconditional call before the login-sync gate | VERIFIED | Lines 571-587, positioned after soft-delete check (line 563-570) and before `if s.loginSyncEnabled && interactive` (line 588). Guarded by `changed`; passes `user.Email`/`user.Role` unchanged. |
| `internal/auth/identitystore.go` — `resolveIntrospection` | `Name: nameFromClaims(res.Raw)` added | VERIFIED | Line 654, `Name: nameFromClaims(res.Raw)` present in the `OIDCClaims` literal. |
| `internal/auth/identitystore.go` — `jitResolve` | Seeds `DisplayName` from `claims.Name`, falling back to `claims.Subject` | VERIFIED | Lines 729-732 (`seedName := claims.Subject; if claims.Name != "" { seedName = claims.Name }`), used at line 735. Role derivation (lines 712-722) untouched. |
| `internal/auth/loginsync.go` — `applyLoginSync` | Display-name block removed; role/email/allowlist only | VERIFIED | Full file read; no display-name computation remains — `user.DisplayName` passed through unchanged at line 103; no-op skip (line 98) compares only email/role; success mutation (line 136-137) sets only Email/Role. |
| `internal/auth/identitystore_test.go` — 4 new JWT-path tests | `TestResolveJWT_ReconcilesStaleDisplayName`, `TestResolveJWT_ReconciliationRunsWithoutLoginSync`, `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim`, `TestResolveJWT_ReconciliationPreservesRoleAndEmail` | VERIFIED | All 4 present (confirmed via grep) and PASS in a fresh, non-cached run in this session. |
| `internal/auth/loginsync_internal_test.go` — updated tests | `TestApplyLoginSync_PromotesAndRefreshesMetadata`, `TestApplyLoginSync_PreservesOperatorRename` | VERIFIED | Both PASS in fresh run; asserted pass-through semantics per plan. |
| `internal/auth/introspection_test.go` — `TestIntrospection_ReconcilesStaleDisplayName` | New introspection-path reconciliation test | VERIFIED | Present and PASSES fresh. |
| `internal/auth/identitystore_jit_test.go` — `TestJIT_SeedsDisplayNameFromClaimsName` | JIT seed test (Name-seed + Subject-fallback) | VERIFIED | Present with 2 sub-tests, both PASS fresh. |
| `internal/auth/oauth2_provider_test.go` — `TestOAuth2Provider_Exchange_PopulatesName` | D-05 regression guard | VERIFIED | Present and PASSES fresh. |
| `internal/auth/identitystore_authn_integration_test.go` — `TestIdentityStore_DisplayNameReconciliation` | Real-Postgres, both interactive/non-interactive paths + SC4 | VERIFIED | Present (git diff against pre-phase commit 7a1338a6 shows pure 88-line append, no modification to the existing gate test). Re-ran live against real Postgres (testcontainers) in this session — all 3 sub-tests PASS. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `materializeIdentity` existing-user branch | `reconcileDisplayName` | direct call, guarded by `changed` | WIRED | Confirmed by source read; commit `d9b2bb93`. |
| `reconcileDisplayName` (changed=true) | `s.users.UpdateUserOnLogin` | direct call passing `user.Email`/`user.Role` unchanged | WIRED | Confirmed; role/email non-leakage also proven by `TestResolveJWT_ReconciliationPreservesRoleAndEmail`. |
| `resolveIntrospection` OIDCClaims construction | `nameFromClaims(res.Raw)` | direct field assignment | WIRED | Line 654; confirmed by fresh test run of `TestIntrospection_ReconcilesStaleDisplayName`. |
| `jitResolve` seed | `JITCreateHuman` | `storage.User{DisplayName: seedName, ...}` passed to `s.users.JITCreateHuman` | WIRED | Confirmed by source read and `TestJIT_SeedsDisplayNameFromClaimsName`. |
| `ResolveLogin` / `resolveJWT` / `resolveIntrospection` | `materializeIdentity` (shared choke point) | direct calls at lines 229, 513, 658 | WIRED | All three paths converge; confirmed by grep + source read — this is the structural basis for SC3 ("every successful login"). |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| AUTH-06 | 09-01-PLAN.md, 09-02-PLAN.md | On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable `name`/`preferred_username` claim and updated when a better value becomes available, replacing a stale subject-hash fallback (#994) | SATISFIED | Both plans declare `requirements: [AUTH-06]`. REQUIREMENTS.md traceability table (line 53) maps AUTH-06 → Phase 9. No orphaned requirements found for this phase — AUTH-06 is the sole requirement mapped, and both plans account for it. |

### Anti-Patterns Found

None. Scanned all 8 files touched by this phase (`identitystore.go`, `identitystore_authn_integration_test.go`, `identitystore_jit_test.go`, `identitystore_test.go`, `introspection_test.go`, `loginsync.go`, `loginsync_internal_test.go`, `oauth2_provider_test.go`) for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` — zero matches.

### Code Review Findings (09-REVIEW.md) — Impact Assessment

09-REVIEW.md recorded 2 WARNING-level findings (WR-01, WR-02), both stemming from the same root cause: the new reconciliation write and `applyLoginSync`'s write are two independent, non-transactional `UpdateUserOnLogin` calls in one login flow.

- **WR-01** (a concurrently soft-deleted user's reconciliation-write failure is treated identically to any other transient failure, rather than fail-closed like `applyLoginSync` does for the same signal): this is a narrow TOCTOU race on an *unrelated* trust boundary (soft-delete enforcement), not a display-name-reconciliation correctness issue. It does not affect any of the 4 phase success criteria — none of SC1-4 concern soft-delete handling. Assessed as a legitimate but non-blocking hardening opportunity for a future phase.
- **WR-02** (a denied login can still leave the display-name write persisted, since the two writes aren't atomic): this is a consistency/auditability nuance ("denied login = no DB effect" no longer strictly holds), not a violation of any stated success criterion — the write only ever moves `display_name` toward a value the token holder is already asserting via a verified claim, and no SC promises atomicity with the login-sync gate.

Both findings are code-quality/defense-in-depth observations on edge cases outside the phase's 4 stated success criteria, not gaps in goal achievement. They do not block phase closure but are worth carrying forward as backlog items (the reviewer's suggested fixes are documented in 09-REVIEW.md).

### Human Verification Required

None. All observable truths are behavior-dependent (state-transition / preservation invariants) and each is covered by a passing automated test that was re-executed fresh in this verification session — including a live run of the real-Postgres integration test (`TestIdentityStore_DisplayNameReconciliation`, 3 sub-tests) and the pre-existing gate-regression test (`TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive`), both against a live testcontainers Postgres instance, not merely re-read from SUMMARY.md.

### Gaps Summary

No gaps. All 4 roadmap success criteria are independently confirmed against the codebase (not merely SUMMARY.md claims): source inspection of `reconcileDisplayName`, `materializeIdentity`, `resolveIntrospection`, `jitResolve`, and `applyLoginSync`; fresh (non-cached) unit test runs; a live integration-test run against real Postgres via testcontainers; a live `task check` run (fmt:check → license:check → lint → build → unit tests, all green); a `git diff` confirming the pre-existing gate-regression test is untouched; and an anti-pattern scan of all 8 touched files (zero debt markers). The 2 WARNING findings from 09-REVIEW.md are scoped to edge cases outside the phase's stated success criteria and do not block goal achievement.

---

_Verified: 2026-07-15T22:55:38Z_
_Verifier: Claude (gsd-verifier)_
