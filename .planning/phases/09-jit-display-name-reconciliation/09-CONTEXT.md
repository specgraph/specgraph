# Phase 9: JIT Display Name Reconciliation - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable name claim (`name`, falling back to `preferred_username`), replacing a stale subject-hash fallback. Reconciliation runs on every successful login (not only first provisioning), and never regresses a `display_name` back to a subject-hash value when no usable claim is present. Scope is limited to `display_name` reconciliation — role sync, email sync, and allowlist enforcement (the existing `LoginSyncEnabled` feature) are untouched.

</domain>

<decisions>
## Implementation Decisions

This phase ran in `--auto` mode (no interactive discussion). All gray areas below were auto-resolved to their recommended option; each is logged for audit.

### Trigger scope — decouple from `interactive` and `LoginSyncEnabled`
- **D-01:** Display-name reconciliation must fire on **every** successful login that resolves to an existing (already-bound) OIDC user — not gated on the `interactive` flag (currently threaded from `resolveJWT`/materializeIdentity) and not gated on the `LoginSyncEnabled` config toggle. `LoginSyncEnabled` is a separate, optional feature (role re-evaluation + allowlist re-enforcement + display-name refresh bundled together in `applyLoginSync`); AUTH-06 requires the display-name piece to be unconditional, so it must be extracted from that gate.
  `[auto] Trigger scope — Q: "Should display_name reconciliation run on every successful login regardless of the interactive flag and LoginSyncEnabled toggle?" → Selected: "Yes, decouple entirely" (recommended default — matches literal roadmap/requirement wording "each successful login" / "every successful login, not only at first provisioning")`
- **D-02:** This means the reconciliation check must run for all three `materializeIdentity` entry paths that hit the existing-user (binding-found) branch: bearer-JWT (`resolveJWT`), opaque-token introspection (`resolveIntrospection`), and interactive OIDC callback (`ResolveLogin`) — anywhere a binding lookup succeeds.

### Staleness detection heuristic
- **D-03:** Reuse the existing proven heuristic from `applyLoginSync` (`internal/auth/loginsync.go:83-86`): `display_name` is considered a stale fallback ⇔ `user.DisplayName == claims.Subject`. Do not add a new schema column/flag (e.g. `display_name_source`) to track provenance explicitly.
  `[auto] Staleness heuristic — Q: "Reuse the exact-match-against-subject heuristic, or add an explicit provenance column?" → Selected: "Reuse existing heuristic, no schema change" (recommended default — avoids over-engineering a medium-priority fix; the existing comparison already correctly protects operator-renamed values)`

### Claim population parity across resolution paths
- **D-04:** `OIDCClaims.Name` (populated via `nameFromClaims`, preferring `name` then `preferred_username` — `internal/auth/oidc_verifier.go:111,156`) is already populated on the JWT verify path (`OIDCVerifier.Verify`). It is currently **not** populated on the introspection path (`resolveIntrospection`, `internal/auth/identitystore.go:613-617` constructs `OIDCClaims` with only `Issuer`/`Subject`/`Raw`). Populate `Name: nameFromClaims(res.Raw)` there too, so introspected/opaque-token logins participate in reconciliation.
  `[auto] Claim parity — Q: "Should the introspection path also populate OIDCClaims.Name for consistency?" → Selected: "Yes, add nameFromClaims(res.Raw) to the introspection claims construction" (recommended default — closes an otherwise-silent gap in the "every successful login" guarantee)`
- **D-05:** Research whether the non-OIDC `oauth2LoginProvider.Exchange` path (`internal/auth/oauth2_provider.go`, userinfo-based) already derives a `Name` value; if not, apply the equivalent fix there. Flagged for the researcher — not independently verified during this discussion (codegraph explore budget did not cover `oauth2_provider.go`'s `Exchange` body).

### Persistence path
- **D-06:** Reuse the existing `UpdateUserOnLogin(ctx, userID, newDisplay, newEmail, newRole)` storage method (already used by `applyLoginSync`) for the write — pass through unchanged `email`/`role` when only `display_name` changes. Do not add a new storage method. Keep the existing no-op skip (don't write when nothing changed).
  `[auto] Persistence — Q: "New dedicated storage method, or reuse UpdateUserOnLogin?" → Selected: "Reuse UpdateUserOnLogin" (recommended default — the method already accepts all three fields and already has an atomic no-op-safe UPDATE)`

### JIT-time seeding (adjacent improvement, same root cause)
- **D-07:** At first JIT provisioning (`jitResolve`, `internal/auth/identitystore.go:685-691`), `DisplayName` is currently always seeded from `claims.Subject`, even when `claims.Name` is already available at that moment (comment: "operator can rename later"). Seed with `claims.Name` when non-empty, falling back to `claims.Subject` otherwise — this eliminates the stale-fallback window entirely for providers that supply a name/preferred_username claim at signup, directly serving AUTH-06's intent with a same-file, low-risk change.
  `[auto] JIT seeding — Q: "Should jitResolve also seed DisplayName from claims.Name when available, instead of always using claims.Subject?" → Selected: "Yes" (recommended default — same root cause, same file, reduces occurrences of the exact bug being fixed)`

### Claude's Discretion
- Exact code shape of the extraction (e.g., a new small helper `reconcileDisplayName(user, claims) (newDisplayName string, changed bool)` called from `materializeIdentity`'s existing-user branch, vs. inlining) is left to planning/implementation.
- Whether `applyLoginSync` keeps its own display-name block (now redundant once decoupled) or has it removed in favor of the new unconditional path is an implementation detail — avoid computing/writing display-name twice per login.
- Logging/audit shape for a reconciliation event (info-level log on change, matching the `auth: login-sync role change` pattern) — follow existing logging conventions in `identitystore.go`/`loginsync.go`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirement source
- `.planning/REQUIREMENTS.md` (AUTH-06, line 27) — full requirement text and GitHub issue #994 reference
- `.planning/ROADMAP.md` (Phase 9 section, lines 129-141) — goal + success criteria

### Core implementation files (read before planning)
- `internal/auth/identitystore.go` — `materializeIdentity` (line 522), `jitResolve` (line 637), `resolveJWT` (line 470), `resolveIntrospection` (line 582); the `interactive` flag is derived once in `resolveJWT` and threaded by value — do not re-derive from context downstream
- `internal/auth/loginsync.go` — `applyLoginSync` (line 67), the existing display-name reconciliation logic to generalize (lines 83-86)
- `internal/auth/oidc_verifier.go` — `OIDCClaims` struct (line 23, has `Name` field already), `nameFromClaims` (line 156), `Verify` (line 79)
- `internal/auth/oauth2_provider.go` — `Exchange` method; unverified whether `Name` is populated here (D-05) — researcher must check
- `internal/storage/postgres/users.go` — `JITCreateHuman` (line 824), and the `UpdateUserOnLogin` method used by `applyLoginSync` (not yet located in this discussion — researcher must confirm signature/location)

### Project constraints (from PROJECT.md)
- `.planning/PROJECT.md` §Constraints — pgx v5 native driver; `RunInTransaction` required for multi-query writes (ADR-004) — the display-name update is a single-query `UPDATE`, so this constraint likely does not apply, but confirm during research
- `.planning/PROJECT.md` §Constraints — DCO `Signed-off-by:` required on all commits

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `nameFromClaims(raw map[string]json.RawMessage) string` (`internal/auth/oidc_verifier.go:156`) — already implements the exact "name, falling back to preferred_username" claim-resolution logic the requirement asks for. No new claim-parsing code needed.
- `UpdateUserOnLogin(ctx, userID, newDisplay, newEmail, newRole)` — existing atomic, no-op-safe persistence method already wired into `applyLoginSync`.
- The `user.DisplayName == claims.Subject` staleness check already exists and is unit-tested (`internal/auth/loginsync_internal_test.go`).

### Established Patterns
- Error/log discipline in `identitystore.go`/`loginsync.go`: `slog.LogAttrs` with structured fields (`slog.String`, `slog.Bool("audit", true)` for security-relevant changes), never bare `log.Printf`.
- The `interactive` flag is derived exactly once (in `resolveJWT`) and threaded by value through `materializeIdentity` → `jitResolve`/`applyLoginSync` — no downstream helper re-reads `InteractiveLoginFromContext`. Any new reconciliation helper should follow the same value-threading discipline (though D-01 means it should NOT gate on `interactive` at all).
- `materializeIdentity`'s existing-user branch (binding found) is the single choke point for all three resolution paths (JWT, introspection, interactive callback) — the natural insertion point for unconditional reconciliation.

### Integration Points
- New/generalized reconciliation logic slots into `materializeIdentity` (`internal/auth/identitystore.go:522-566`), likely replacing or wrapping the display-name portion of the existing `if s.loginSyncEnabled && interactive { ... applyLoginSync ... }` block so it runs unconditionally while role/email sync stays gated as-is.
- `jitResolve` (`internal/auth/identitystore.go:637-711`) is the secondary integration point for D-07 (seed-time fix).

</code_context>

<specifics>
## Specific Ideas

No particular UI/UX preferences — this is a backend-only identity/auth correctness fix with no user-facing surface beyond the eventual `display_name` value shown in the UI (already rendered elsewhere; out of scope to change rendering).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. The oauth2 (non-OIDC) `Name`-population check (D-05) and the JIT-seeding improvement (D-07) were kept in-scope as same-root-cause, same-file fixes rather than deferred, since both directly serve AUTH-06's stated intent without introducing new capabilities.

</deferred>

---

*Phase: 9-JIT Display Name Reconciliation*
*Context gathered: 2026-07-15*
