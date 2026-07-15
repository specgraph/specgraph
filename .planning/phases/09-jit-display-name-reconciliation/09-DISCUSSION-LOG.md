# Phase 9: JIT Display Name Reconciliation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 9-JIT Display Name Reconciliation
**Areas discussed:** Trigger scope, Staleness detection heuristic, Claim population parity, Persistence path, JIT-time seeding

**Mode:** `--auto` — all areas auto-resolved to the recommended option, no interactive prompts. Single-pass discussion (auto-mode pass cap).

---

## Trigger scope — decouple from `interactive` and `LoginSyncEnabled`?

| Option | Description | Selected |
|--------|-------------|----------|
| Decouple entirely | Reconcile display_name on every successful login regardless of `interactive` flag or `LoginSyncEnabled` toggle | ✓ |
| Keep gated on `LoginSyncEnabled` only | Reconciliation only runs when the operator has login-sync enabled | |
| Keep gated on both `interactive` and `LoginSyncEnabled` (status quo) | No change — current `applyLoginSync` behavior | |

**Selected:** Decouple entirely.
**Notes:** The roadmap goal and success criteria use "each successful login" / "every successful login, not only at first provisioning" — unambiguous that this must not depend on an optional feature toggle or on whether the login was interactive.

---

## Staleness detection heuristic

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse existing heuristic | `user.DisplayName == claims.Subject` (already implemented in `applyLoginSync`) | ✓ |
| Add explicit provenance column | New `display_name_source` field on `users` to track jit-fallback vs. operator-set | |

**Selected:** Reuse existing heuristic, no schema change.
**Notes:** Existing comparison already correctly protects operator-renamed display names ("update only if never renamed by an operator"). A new schema column is unwarranted complexity for a medium-priority fix.

---

## Claim population parity across resolution paths

| Option | Description | Selected |
|--------|-------------|----------|
| Add `Name` population to introspection path | `resolveIntrospection` currently builds `OIDCClaims` without a `Name` field; add `nameFromClaims(res.Raw)` | ✓ |
| Leave introspection path unchanged | Introspected/opaque-token logins never reconcile display_name | |

**Selected:** Add `Name` population to introspection path.
**Notes:** Without this, "every successful login" silently excludes opaque-token/introspection-resolved users. The non-OIDC oauth2 provider path (`oauth2_provider.go`) was flagged for the researcher to verify separately — not confirmed during this discussion.

---

## Persistence path

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `UpdateUserOnLogin` | Existing atomic, no-op-safe method already used by `applyLoginSync` | ✓ |
| New dedicated method | A narrower `UpdateDisplayName` storage method | |

**Selected:** Reuse `UpdateUserOnLogin`.
**Notes:** Already accepts display_name/email/role and has the no-op-safe UPDATE semantics needed; a narrower method would duplicate that logic for no benefit.

---

## JIT-time seeding (adjacent improvement)

| Option | Description | Selected |
|--------|-------------|----------|
| Seed from `claims.Name` when available, fallback to `claims.Subject` | Reduces occurrence of the stale-fallback case at the source | ✓ |
| Leave `jitResolve` unchanged (always seed from `claims.Subject`) | Rely solely on post-hoc reconciliation | |

**Selected:** Seed from `claims.Name` when available.
**Notes:** Same file, same root cause as AUTH-06; a low-risk, same-commit improvement that directly reduces how often the fallback condition arises in the first place.

---

## Claude's Discretion

- Exact code shape of the extraction (dedicated helper vs. inline change in `materializeIdentity`).
- Whether `applyLoginSync`'s own display-name block is removed (now redundant) or left as a no-op once the unconditional path exists — avoid double computation/writes.
- Logging/audit shape for a reconciliation event, following existing `identitystore.go`/`loginsync.go` conventions.

## Deferred Ideas

None — discussion stayed within phase scope.
