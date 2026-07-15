---
phase: 9
slug: jit-display-name-reconciliation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-15
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `github.com/stretchr/testify/require` |
| **Config file** | none — plain `go test`, driven by Taskfile.yml |
| **Quick run command** | `go test -short -race ./internal/auth/... -run '<TestPattern>'` |
| **Full suite command** | `task check` (unit suite + fmt/lint/build gates); `task pr-prep` additionally runs `task test:integration` (requires Docker) |
| **Estimated runtime** | ~10-20 seconds (quick, per-package unit); ~2-4 minutes (`task check` full); +Docker startup for integration |

---

## Sampling Rate

- **After every task commit:** Run `go test -short -race ./internal/auth/... -run <touched-area-pattern>`
- **After every plan wave:** Run `task check` (full unit suite + lint/build/fmt gates)
- **Before `/gsd-verify-work`:** `task check` full green, plus `task test:integration` (requires Docker) for the two integration tests below
- **Max feedback latency:** ~20 seconds (quick unit run)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | AUTH-06 (SC1) | V4 (role must not regress) | Existing-user JWT login with stale (`DisplayName == Subject`) fallback + `name` claim present → `display_name` updated | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconcilesStaleDisplayName` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | AUTH-06 (SC2) | — | Same as SC1 via `resolveIntrospection` (opaque token) — proves D-04 fix | unit | `go test -short -race ./internal/auth/... -run TestIntrospection_ReconcilesStaleDisplayName` | ❌ W0 | ⬜ pending |
| 09-01-03 | 01 | 1 | AUTH-06 (SC3) | — | Non-interactive login with `LoginSyncEnabled: false` still reconciles display_name (full decoupling, D-01) | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconciliationRunsWithoutLoginSync` | ❌ W0 | ⬜ pending |
| 09-01-04 | 01 | 1 | AUTH-06 (SC4) | — | Operator-set `DisplayName` (≠ Subject) with no usable claim → unchanged; stale-fallback user with no usable claim → unchanged (no-op, no regression) | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim` | ❌ W0 | ⬜ pending |
| 09-01-05 | 01 | 1 | D-06 | V4 (no role/email mutation) | Reconciliation write passes through unchanged `email`/`role` | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconciliationPreservesRoleAndEmail` | ❌ W0 | ⬜ pending |
| 09-01-06 | 01 | 1 | D-07 | — | `jitResolve` seeds `DisplayName` from `claims.Name` when present, falls back to `Subject` | unit | `go test -short -race ./internal/auth/... -run TestJIT_SeedsDisplayNameFromClaimsName` | ❌ W0 | ⬜ pending |
| 09-01-07 | 01 | 1 | D-05 (regression guard) | — | `oauth2LoginProvider.Exchange` returns non-empty `Name` when userinfo has a `name` field (already-correct behavior, newly guarded) | unit | `go test -short -race ./internal/auth/... -run TestOAuth2Provider_Exchange_PopulatesName` | ❌ W0 | ⬜ pending |
| 09-01-08 | 01 | 1 | Pitfall 1 regression | V4 Access Control | Non-interactive resolve with `LoginSyncEnabled: true` still does NOT persist a role change (existing test, must keep passing unmodified) | integration | `go test -tags integration -race ./internal/auth/... -run TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` | ✅ exists | ⬜ pending |
| 09-01-09 | 01 | 1 | End-to-end reconciliation | V2 Authentication (adjacent) | Real Postgres: stale-fallback user reconciles on both interactive and non-interactive resolve | integration | `go test -tags integration -race ./internal/auth/... -run TestIdentityStore_DisplayNameReconciliation` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/auth/identitystore_test.go` — add `TestResolveJWT_ReconcilesStaleDisplayName`, `TestResolveJWT_ReconciliationRunsWithoutLoginSync`, `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim`, `TestResolveJWT_ReconciliationPreservesRoleAndEmail` (AUTH-06 SC1, SC3, SC4, D-06)
- [ ] `internal/auth/introspection_test.go` — add `TestIntrospection_ReconcilesStaleDisplayName` (AUTH-06 SC2 / D-04)
- [ ] `internal/auth/identitystore_jit_test.go` — add `TestJIT_SeedsDisplayNameFromClaimsName` (D-07)
- [ ] `internal/auth/oauth2_provider_test.go` — add `TestOAuth2Provider_Exchange_PopulatesName` (optional regression guard for D-05's already-correct finding)
- [ ] `internal/auth/loginsync_internal_test.go` — UPDATE `TestApplyLoginSync_PromotesAndRefreshesMetadata` and `TestApplyLoginSync_PreservesOperatorRename` to drop/restructure their display-name assertions once the display-name block is extracted out of `applyLoginSync` (expected test change, not a regression)
- [ ] `internal/auth/identitystore_authn_integration_test.go` — add `TestIdentityStore_DisplayNameReconciliation` (end-to-end, both interactive and non-interactive, real Postgres)
- No framework install needed — `testify` and existing fixtures (`usersBackendStub`, `loginSyncFakeBackend`, `activeUser`, `newOIDCTestIssuer`/`mintToken`) cover every shape of test this phase needs. **Note:** `activeUser` sets `DisplayName: "test-" + id`, which will NOT trigger the staleness heuristic as-is — any new test exercising reconciliation must construct a `storage.User` with `DisplayName` explicitly equal to the token's `sub` claim.

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
