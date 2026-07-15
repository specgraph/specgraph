---
phase: 07
slug: authoring-lifecycle-semantics
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on (high) severity
threats_open: 0
asvs_level: 1
created: 2026-07-15
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register built from artifacts (State B): all five 07-0N-PLAN.md `<threat_model>`
> blocks were authored at plan time (`register_authored_at_plan_time: true`), so
> this audit verifies mitigations rather than scanning for new threats. ASVS L1
> grep-depth verification (short-circuit: `threats_open:0` + plan-time register + L1).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CLI/MCP client → LifecycleService handler | Untrusted `slug`, `new_slug`, `reason`, `re_entry_stage` cross here; project scope enforced first by `scopeStore(ctx, h.scoper)` (X-Specgraph-Project). | spec slugs, free-text audit reason |
| MCP agent → `author` tool → LifecycleService | Rerouted amend/supersede pass agent input through to the same handler+storage gates; no tool-side bypass. | action/slug/reason/re_entry_stage/new_slug |
| Amend / Abandon transaction → claims + edges tables | The lease granting execution rights is deleted here; a lingering lease would let an ephemeral worker keep executing a spec that returned to authoring or terminated. | claim rows, `CLAIMED_BY` edges |
| Handler → postgres store | Parameterized pgx only; `reason` written to changelog as a free-text audit note. | reason audit note |
| Client → AuthoringService | Retired `Amend`/`Supersede` RPCs removed from the wire surface; remaining funnel/approve RPCs unchanged and still scope-enforced. | n/a (removed) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01 | Elevation of Privilege / Tampering | Stale claim/lease after amend (`LifecycleAmendSpec`) | high | mitigate | D-08: `releaseActiveClaim(txCtx, slug, ...)` deletes the `claims` row + `CLAIMED_BY` edge inside the amend transaction (`lifecycle.go:98`, `RunInTransaction` at `:55`). Same helper is also called by `LifecycleAbandonSpec` (`:353`), closing the deep-review WR-01 abandon-path asymmetry. Verified by `TestLifecycleAmend_ReleasesClaim` + `TestLifecycleAbandon_ReleasesClaim`. | closed |
| T-07-02 | Elevation of Privilege | `LifecycleService.TransitionSupersede` (authz) | low | mitigate | `scopeStore(ctx, h.scoper)` runs first in every lifecycle handler method (`lifecycle_handler.go:43,74,109,135,173,232`); the reason addition adds no new authz surface. | closed |
| T-07-03 | Information Disclosure | Error surface (handler / MCP tool / e2e) | low | mitigate | `lifecycleError` maps storage sentinels → connect codes via `errors.Is` with a generic `CodeInternal` fallthrough (`lifecycle_handler.go:274+`); MCP tool surfaces the sanitized `connectErrResult`; tests assert connect codes / `res.IsError`, never message-string equality. | closed |
| T-07-04 | Tampering | `re_entry_stage` input validation | low | mitigate | Shared `SpecStage.IsValidReEntryStage()` allowlist (spark\|shape\|specify\|decompose) enforced at BOTH handler (`lifecycle_handler.go:57`) and storage (`lifecycle.go:43`) — defense-in-depth. approved/in_progress/review/done rejected with `CodeInvalidArgument`. Replaces the prior `IsValid()`+`ExcludesReEntry()` check that wrongly accepted in-flight stages. | closed |
| T-07-05 | Tampering | Reroute bypassing an existing precondition | low | mitigate | Rerouting to `TransitionAmend`/`TransitionSupersede` inherits the amend-eligible / done-only preconditions enforced in storage; the tool re-validates nothing that would diverge (handler↔storage guard duplication confirmed non-divergent by the deep review). | closed |
| T-07-06 | Tampering | Slices retained on amend re-decompose | low | accept | Slices are the re-authored decompose output, not stale state; intentionally not deleted on amend. Hardened post-audit by the CR-01 fix, which makes `StoreDecomposeOutput` reconcile (update/create/prune) child slices on re-decompose instead of silently skipping. See Accepted Risks Log. | closed |
| T-07-07 | Tampering | Divergent second amend/supersede implementation | medium | mitigate | Deleting the broken `AuthoringService.Amend/Supersede` path removes the possibility of the inverted (approved-rejecting, any-state-supersede, land-at-target) semantics being reachable again. Verified by absence greps + green build (07-04). | closed |
| T-07-08 | Denial of Service | Removing an in-use RPC | low | mitigate | Plan 07-03 rerouted the only callers first; blast-radius greps confirmed zero remaining callers before deletion, so no live consumer breaks. | closed |
| T-07-09 | Tampering | MCP agent reproduces the #899 no-op / misuses amend | low | mitigate | Skills teach the land-one-before model + constrained params; the MCP-only e2e proves the correct sequence and both rejection paths (amend-on-done, supersede-on-non-done), so regressions are caught. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-04 (reason field) | Supersede `reason` is optional and intentionally not length-bounded on the lifecycle path. It is an internal changelog audit note stored via parameterized pgx (no injection risk), not a wire-exposed identifier, and supersede is a done-only state transition with no execution effect. | plan-time (07-01) | 2026-07-15 |
| AR-07-02 | T-07-06 | Slices retained (not deleted) on amend, by design — they are the re-authored decompose output, not stale state. Residual data-integrity concern on re-decompose was subsequently eliminated by the CR-01 fix (slice reconciliation). | plan-time (07-02) | 2026-07-15 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-15 | 9 | 9 | 0 | gsd secure-phase (L1 grep verification) |

**Cross-reference:** The deep code review (`07-REVIEW.md`) and its fixes (`07-REVIEW-FIX.md`) independently surfaced and closed two security-relevant items during this phase: **CR-01** (amend→re-decompose data integrity — now reconciled) and **WR-01** (abandon-path stale-lease asymmetry with T-07-01 — now releases the claim/edge). Both are reflected as closed above.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-15
