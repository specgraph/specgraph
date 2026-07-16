---
phase: 9
slug: jit-display-name-reconciliation
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-16
---

# Phase 9 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| IdP → SpecGraph identity resolution (bearer JWT) | OIDC `name`/`preferred_username` claims from an external, semi-trusted IdP now cross into persisted user profile state (`display_name`) on every login, not only at JIT creation | `display_name` (cosmetic profile field, not an identity/authz key) |
| IdP → SpecGraph identity resolution (opaque-token introspection) | Introspection response claims (now including a `name` field) cross into persisted `display_name` | `display_name` |
| IdP → SpecGraph JIT provisioning | Claim values seed a brand-new user's `display_name` at first login | `display_name` |
| Client → API (bearer token) | An untrusted bearer JWT drives the resolve path that triggers reconciliation | Bearer token → resolved `Identity` |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-01 | Tampering | `reconcileDisplayName` write from an IdP `name` claim (`materializeIdentity`) | low | accept | `display_name` has been settable from untrusted IdP claims since JIT creation and at prior `applyLoginSync` time — this phase only increases write frequency, not the trust boundary. `nameFromClaims` already unmarshals defensively (empty/type-mismatch falls through, no panic — ASVS V5). UI rendering/escaping is unchanged and out of scope (CONTEXT.md `<specifics>`). No new sanitization added; documented non-regression. | closed |
| T-09-02 | Elevation of Privilege | Reconciliation write path passing role into `UpdateUserOnLogin` (`internal/auth/identitystore.go`, `materializeIdentity`) | high | mitigate | The reconciliation call site passes `user.Role` and `user.Email` UNCHANGED (D-06) — never a claims-derived role. Only the still-gated `applyLoginSync` computes new role/email. **Verified**: `TestResolveJWT_ReconciliationPreservesRoleAndEmail` — PASS (re-run 2026-07-16 against current code, post code-review fixes). ASVS V4 access-control non-regression. | closed |
| T-09-03 | Tampering | `resolveIntrospection` now feeds `claims.Name` into display-name reconciliation | low | accept | Same boundary as T-09-01: `display_name` was already IdP-controlled at bind/JIT time; adding `Name` to the introspection claims only lets an existing feature run on this path. `nameFromClaims` unmarshals defensively (ASVS V5). No new sanitization; documented non-regression. | closed |
| T-09-04 | Elevation of Privilege | `jitResolve` seed + introspection reconcile write | high | mitigate | The JIT seed sets only `DisplayName`; role is still derived solely from claims-mapping at JIT time (unchanged). The introspection reconcile write passes `user.Role`/`user.Email` through unchanged (D-06). **Verified**: `TestIntrospection_ReconcilesStaleDisplayName` — PASS, `TestJIT_SeedsDisplayNameFromClaimsName` — PASS (both re-run 2026-07-16 against current code), and `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` confirmed unmodified via `git diff` across all three code-review fix commits (ASVS V4 access-control non-regression). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

No package-manager installs occurred in this phase (Package Legitimacy Audit: N/A per `09-RESEARCH.md`) — no supply-chain checkpoint required.

**Post-plan-time addendum (code review fixes, not in the original PLAN.md threat_model):** The code-review cycle after execution surfaced and closed two additional non-STRIDE-classified robustness issues, tracked in `09-REVIEW.md`/`09-REVIEW-FIX.md` rather than as new threat-register entries since neither is an elevation-of-privilege or tampering vector on its own:
- A concurrently-soft-deleted-user TOCTOU gap on the reconciliation write (WR-01) — fixed to fail closed (`ErrUnauthenticated`) on `storage.ErrUserNotFound`, mirroring `applyLoginSync`'s existing pattern.
- A non-atomic two-write split that could leave a display-name change persisted even when `applyLoginSync` later denies the login (WR-02) — fixed by folding both writes into a single `UpdateUserOnLogin` call when the login-sync gate fires.

Both are relevant to defense-in-depth and are noted here for completeness; they do not change the ASVS V4 access-control disposition above (role/email were never affected by either gap).

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-09-01 | T-09-01 | `display_name` has been an IdP-controlled, cosmetic (non-identity-key) field since the JIT provisioning path was introduced in an earlier milestone; this phase increases write *frequency* only, not the trust boundary or blast radius. UI escaping is an existing, separate concern out of this phase's scope. | Claude (gsd-secure-phase, retroactive per PLAN.md-authored disposition) | 2026-07-16 |
| AR-09-02 | T-09-03 | Same rationale as AR-09-01 — the introspection path now reaches the same already-accepted `display_name` write surface, not a new one. | Claude (gsd-secure-phase, retroactive per PLAN.md-authored disposition) | 2026-07-16 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-16 | 4 | 4 | 0 | Claude (gsd-secure-phase, State B — built from PLAN.md `<threat_model>` + SUMMARY.md; `register_authored_at_plan_time: true`, ASVS L1 short-circuit — mitigation tests independently re-run against current code rather than trusted from PLAN.md text) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-16
