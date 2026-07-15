# Phase 9: JIT Display Name Reconciliation - Research

**Researched:** 2026-07-15
**Domain:** Go identity/auth internals — OIDC claim reconciliation on an existing, single-repo codebase (no new external dependencies)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Display-name reconciliation must fire on **every** successful login that resolves to an existing (already-bound) OIDC user — not gated on the `interactive` flag (currently threaded from `resolveJWT`/materializeIdentity) and not gated on the `LoginSyncEnabled` config toggle. `LoginSyncEnabled` is a separate, optional feature (role re-evaluation + allowlist re-enforcement + display-name refresh bundled together in `applyLoginSync`); AUTH-06 requires the display-name piece to be unconditional, so it must be extracted from that gate.
- **D-02:** This means the reconciliation check must run for all three `materializeIdentity` entry paths that hit the existing-user (binding-found) branch: bearer-JWT (`resolveJWT`), opaque-token introspection (`resolveIntrospection`), and interactive OIDC callback (`ResolveLogin`) — anywhere a binding lookup succeeds.
- **D-03:** Reuse the existing proven heuristic from `applyLoginSync` (`internal/auth/loginsync.go:83-86`): `display_name` is considered a stale fallback ⇔ `user.DisplayName == claims.Subject`. Do not add a new schema column/flag (e.g. `display_name_source`) to track provenance explicitly.
- **D-04:** `OIDCClaims.Name` (populated via `nameFromClaims`, preferring `name` then `preferred_username` — `internal/auth/oidc_verifier.go:111,156`) is already populated on the JWT verify path (`OIDCVerifier.Verify`). It is currently **not** populated on the introspection path (`resolveIntrospection`, `internal/auth/identitystore.go:613-617` constructs `OIDCClaims` with only `Issuer`/`Subject`/`Raw`). Populate `Name: nameFromClaims(res.Raw)` there too, so introspected/opaque-token logins participate in reconciliation.
- **D-05:** Research whether the non-OIDC `oauth2LoginProvider.Exchange` path (`internal/auth/oauth2_provider.go`, userinfo-based) already derives a `Name` value; if not, apply the equivalent fix there. Flagged for the researcher — not independently verified during discussion.
  **→ RESOLVED BY THIS RESEARCH: it already does.** See "D-05 Resolution" below — no code change needed on this path.
- **D-06:** Reuse the existing `UpdateUserOnLogin(ctx, userID, newDisplay, newEmail, newRole)` storage method (already used by `applyLoginSync`) for the write — pass through unchanged `email`/`role` when only `display_name` changes. Do not add a new storage method. Keep the existing no-op skip (don't write when nothing changed).
- **D-07:** At first JIT provisioning (`jitResolve`, `internal/auth/identitystore.go:685-691`), `DisplayName` is currently always seeded from `claims.Subject`, even when `claims.Name` is already available at that moment. Seed with `claims.Name` when non-empty, falling back to `claims.Subject` otherwise.

### Claude's Discretion

- Exact code shape of the extraction (e.g., a new small helper `reconcileDisplayName(user, claims) (newDisplayName string, changed bool)` called from `materializeIdentity`'s existing-user branch, vs. inlining) is left to planning/implementation.
- Whether `applyLoginSync` keeps its own display-name block (now redundant once decoupled) or has it removed in favor of the new unconditional path is an implementation detail — avoid computing/writing display-name twice per login.
- Logging/audit shape for a reconciliation event (info-level log on change, matching the `auth: login-sync role change` pattern) — follow existing logging conventions in `identitystore.go`/`loginsync.go`.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. The oauth2 (non-OIDC) `Name`-population check (D-05) and the JIT-seeding improvement (D-07) were kept in-scope as same-root-cause, same-file fixes rather than deferred.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AUTH-06 | On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable `name`/`preferred_username` claim and updated when a better value becomes available, replacing a stale subject-hash fallback (#994) | Confirms the exact insertion point (`materializeIdentity`'s existing-user branch), the exact staleness heuristic already proven in `applyLoginSync`, the exact persistence method (`UpdateUserOnLogin`, single-query, no transaction needed), and closes both known claim-population gaps (introspection path — needs a 1-line fix; oauth2 non-OIDC path — already correct, no fix needed). All three answers to the phase's open questions are resolved below with citations to current on-disk code and line numbers. |

</phase_requirements>

## Summary

This phase is a small, well-bounded internal refactor with almost all of the hard research already done during `/gsd-discuss-phase` (D-01 through D-07 in CONTEXT.md). This research session's job was to verify those decisions against the current on-disk code and resolve the two explicitly-flagged open items. Both resolved cleanly:

1. **D-05 (oauth2 Name population) is a non-issue.** `oauth2LoginProvider.Exchange` in `internal/auth/oauth2_provider.go:111-117` already sets `Name: displayNameFromUserinfo(userinfo)` on every returned `*OIDCClaims`, and `displayNameFromUserinfo` (lines 199-211) already implements a `name` → `login` claim fallback mirroring `nameFromClaims`'s `name` → `preferred_username` fallback. **No code change is needed in `oauth2_provider.go`.** The planner should mark D-05 as "verified, no-op" rather than scheduling a fix task.

2. **`UpdateUserOnLogin`'s signature and semantics match CONTEXT.md's assumptions exactly.** It is a single UPDATE statement (`internal/storage/postgres/users.go:241-253`), takes independent `(userID, displayName, email, role string)` args, has an active-row guard (`deleted_at IS NULL`) that returns `storage.ErrUserNotFound` on zero rows, and is declared on the `storage.UsersBackend` interface (`internal/storage/users.go:53-57`). It is NOT itself no-op-safe at the SQL level (it always executes the UPDATE) — the no-op skip is the *caller's* responsibility, exactly as `applyLoginSync` already does at loginsync.go:92-95 before calling it. This confirms ADR-004's `RunInTransaction` requirement does not apply — it's a single query, ADR-004 only gates *multi*-query writes.

The remaining architecture is a straightforward "unconditional side branch alongside an existing gated branch" pattern. `materializeIdentity`'s existing-user branch (`internal/auth/identitystore.go:522-567`) already has exactly one gated call (`if s.loginSyncEnabled && interactive { ... applyLoginSync ... }`, lines 550-556); the new reconciliation logic needs to run **unconditionally, before or independent of that gate**, using a value already in scope (`user`, `claims`) with no new dependencies.

There IS a directly relevant precedent for "some auth side-effects run unconditionally, others are gated by config": `s.tracker.Touch(key.ID)` in `resolveAPIKey` (line 391) fires unconditionally on every successful API-key resolve regardless of any config toggle. This confirms D-01's "decouple entirely" choice is consistent with existing codebase philosophy, not a novel pattern.

**Primary recommendation:** Add a small unexported helper (e.g. `reconcileDisplayName(user *storage.User, claims *OIDCClaims) (newName string, changed bool)`) that implements the D-03 heuristic once, call it unconditionally from `materializeIdentity`'s existing-user branch (after the soft-delete check, independent of `loginSyncEnabled && interactive`), persist via `UpdateUserOnLogin` with a no-op skip mirroring `applyLoginSync`'s pattern, remove the now-redundant display-name computation from `applyLoginSync` (keep it doing role + email + allowlist only), add `Name: nameFromClaims(res.Raw)` to `resolveIntrospection`'s claims construction, and seed `DisplayName` from `claims.Name` (falling back to `claims.Subject`) in `jitResolve`. No new files, no new storage methods, no schema change, no new config flag.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Display-name staleness detection | API / Backend (`internal/auth`) | — | Pure in-memory comparison against already-loaded `user`/`claims`; no I/O, belongs in the identity-resolution business logic, not storage |
| Display-name persistence | Database / Storage (`internal/storage/postgres`) | — | `UpdateUserOnLogin` already exists and is the correct write path; storage layer owns the SQL, auth layer owns the decision of *when* to call it |
| Claim extraction (`name`/`preferred_username`/`login`) | API / Backend (`internal/auth`, OIDC/oauth2 verifiers) | — | Claim-shape parsing is provider-specific and already centralized in `nameFromClaims` (OIDC) and `displayNameFromUserinfo` (oauth2); no change to this tier's ownership |
| Login-sync gate (role/email/allowlist) | API / Backend (`internal/auth/loginsync.go`) | — | Stays gated on `loginSyncEnabled && interactive` — explicitly OUT of scope per the phase boundary (D-01 only decouples display_name, not role/email/allowlist) |
| UI display of `display_name` | Browser / Client (web frontend) | — | Out of scope — rendering already exists elsewhere; this phase only ensures the value stored is correct |

## Package Legitimacy Audit

**Not applicable.** This phase introduces no new external dependencies — it modifies existing Go code in `internal/auth/` and calls an already-existing storage method. No `go get`/`npm install` of any kind is required.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │           Resolve(ctx, token)            │
                    └──────────────────┬────────────────────────┘
                                       │
              ┌────────────────────────┼─────────────────────────┐
              │                        │                         │
        JWT-shaped              opaque + introspectors      spgr_ws_/spgr_sk_
              │                        │                         │
    ┌─────────▼─────────┐   ┌──────────▼───────────┐    (session/API-key —
    │    resolveJWT      │   │  resolveIntrospection │     NOT in scope: no
    │ verifier.Verify()   │   │  c.Introspect()       │     OIDC claims carried)
    │ claims.Name set     │   │ [FIX] claims.Name =   │
    │ via nameFromClaims  │   │  nameFromClaims(Raw)  │
    │ (already correct)   │   │  (currently missing)  │
    └─────────┬───────────┘   └──────────┬────────────┘
              │                          │
              │      ResolveLogin(claims)│ (interactive OIDC/oauth2 callback;
              │                          │  oauth2LoginProvider.Exchange already
              │                          │  sets claims.Name — verified, no fix)
              └────────────┬─────────────┘
                           │
                 ┌─────────▼──────────────────────────┐
                 │      materializeIdentity             │
                 │  1. LookupOIDCBinding                │
                 │  2a. miss -> jitResolve               │
                 │      [FIX] seed DisplayName from      │
                 │       claims.Name, fallback Subject   │
                 │  2b. hit -> GetUserByID               │
                 │      -> soft-delete check             │
                 │      -> [NEW] reconcileDisplayName    │
                 │         (unconditional, D-01)         │
                 │           user.DisplayName==Subject?  │
                 │           claims.Name != ""?          │
                 │           -> UpdateUserOnLogin(...)   │
                 │      -> if loginSyncEnabled &&        │
                 │         interactive: applyLoginSync   │
                 │         (role/email/allowlist ONLY    │
                 │         after this phase — display-   │
                 │         name block removed)           │
                 └─────────┬──────────────────────────┘
                           │
                     Identity{DisplayName: user.DisplayName, ...}
```

### Recommended Project Structure

No new files. All changes land in existing files:

```
internal/auth/
├── identitystore.go   # materializeIdentity (add reconciliation call),
│                      # jitResolve (D-07 seed fix), resolveIntrospection (D-04 fix)
├── loginsync.go        # applyLoginSync (remove now-redundant display-name block)
└── oidc_verifier.go    # unchanged — nameFromClaims already correct, reused as-is
```

### Pattern 1: Unconditional side-effect alongside a gated one (existing precedent)

**What:** A resolve-path side effect that must always run, living next to a config-gated side effect in the same function, without conflating the two.
**When to use:** When a requirement demands "every successful login" behavior distinct from an optional/configurable feature bundled in the same code path.
**Example (existing precedent, not modified by this phase):**
```go
// Source: internal/auth/identitystore.go:391 (resolveAPIKey)
s.tracker.Touch(key.ID)  // unconditional — no config gate, runs on every resolve
return &Identity{...}
```
**Applied to this phase:**
```go
// Source: internal/auth/identitystore.go materializeIdentity, existing-user branch
// (illustrative — exact code shape is Claude's discretion per CONTEXT.md)
if newName, changed := reconcileDisplayName(user, claims); changed {
    if err := s.users.UpdateUserOnLogin(ctx, user.ID, newName, user.Email, user.Role); err != nil {
        slog.LogAttrs(ctx, slog.LevelWarn, "auth: display-name reconciliation persist failed (proceeding)",
            slog.String("user_id", user.ID), slog.Any("error", err))
        // best-effort: do not deny login over a display-name write failure
    } else {
        user.DisplayName = newName
        slog.LogAttrs(ctx, slog.LevelInfo, "auth: display-name reconciled",
            slog.String("user_id", user.ID), slog.String("old", user.DisplayName), slog.String("new", newName))
    }
}
if s.loginSyncEnabled && interactive {
    synced, syncErr := s.applyLoginSync(ctx, claims, user)
    if syncErr != nil {
        return nil, syncErr
    }
    user = synced
}
```

### Pattern 2: Staleness-guarded reconciliation (D-03, already proven)

**What:** Only overwrite a value if it's provably the auto-generated fallback, never if an operator could have set it intentionally.
**When to use:** Any "auto-heal a bad default" feature where the field is also operator-editable.
**Example:**
```go
// Source: internal/auth/loginsync.go:83-86 (existing, proven, unit-tested)
newDisplay := user.DisplayName
if user.DisplayName == claims.Subject && claims.Name != "" {
    newDisplay = claims.Name // update only if never renamed by an operator
}
```
This is the exact heuristic D-03 says to reuse. It already correctly protects an operator-renamed value (if `DisplayName != Subject`, nothing changes) while healing the exact stale-fallback case. **Edge case to note for the planner:** if an operator manually renames a user to the literal string equal to that user's own `Subject` value, the heuristic will incorrectly treat it as unreconciled and re-overwrite it on next login. This is an existing, accepted limitation of the D-03 heuristic (not introduced by this phase) — CONTEXT.md explicitly chose not to add a provenance column to solve it, so no action is needed, but it's worth a one-line code comment restating the tradeoff at the new call site for future maintainers.

### Anti-Patterns to Avoid

- **Computing display-name reconciliation twice per login:** Once decoupled, `applyLoginSync`'s own display-name block (loginsync.go:83-86, 93, 131) becomes dead weight when `loginSyncEnabled && interactive` is also true — the new unconditional call already handled it. Remove that block from `applyLoginSync` rather than leaving both to run (the second one would be a guaranteed no-op given the first already wrote it, but it's dead code and the no-op-skip check `newDisplay == user.DisplayName` in `applyLoginSync` would technically still cover it — Claude's discretion notes this explicitly, but leaving it in is misleading dead logic, not a correctness bug).
- **Re-deriving the `interactive` flag downstream:** Per the established convention (`identitystore.go:510-512`), `interactive` is derived exactly once in `resolveJWT` and threaded by value. The new reconciliation call does not need `interactive` at all (D-01 says it must NOT gate on it) — do not thread or check it in the new helper.
- **Gating the new logic on `s.loginSyncEnabled`:** D-01 is explicit that this is a separate concern from the `LoginSyncEnabled` toggle. A `pgIdentityStore` with `loginSyncEnabled: false` (the default) must still reconcile display names.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Claim → display-name resolution (OIDC) | New claim-parsing logic | `nameFromClaims(raw map[string]json.RawMessage) string` (`internal/auth/oidc_verifier.go:156`) | Already implements exactly the `name` → `preferred_username` fallback AUTH-06 requires; already unit-tested (`TestNameFromClaims`) |
| Claim → display-name resolution (oauth2/non-OIDC) | New userinfo-parsing logic | `displayNameFromUserinfo(userinfo map[string]json.RawMessage) string` (`internal/auth/oauth2_provider.go:199`) | Already implements `name` → `login` fallback; already wired into `Exchange`; verified in this research as functioning correctly (D-05 resolved) |
| Persisting the new display_name | A new storage method / raw SQL | `UpdateUserOnLogin(ctx, userID, displayName, email, role string) error` (`internal/storage/postgres/users.go:241`) | Single atomic UPDATE, already has the correct active-row guard, already on the `storage.UsersBackend` interface, already mocked in every relevant test fixture |
| Staleness detection | A new schema column (`display_name_source`) | The `user.DisplayName == claims.Subject` comparison (already proven in `applyLoginSync`) | D-03 explicitly rejects the schema-change option as over-engineering for a medium-priority fix |

**Key insight:** Every primitive this phase needs already exists in the codebase, proven and unit-tested, from a prior phase (AUTH-05, per the `identitystore.go` history) that built `applyLoginSync`. This phase is an *extraction and unconditional re-application* of existing logic, not new logic — the risk profile is almost entirely in refactor-correctness (don't double-write, don't accidentally gate the new path, don't break the existing gated role/email tests), not in designing new behavior.

## Common Pitfalls

### Pitfall 1: Breaking `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive`

**What goes wrong:** This existing integration test (`internal/auth/identitystore_authn_integration_test.go:148-214`) explicitly asserts that a **non-interactive** bearer resolve does NOT persist a role change when `LoginSyncEnabled` is true. If the new unconditional reconciliation is implemented sloppily (e.g., by accidentally also unconditionally recomputing role), this test would fail.
**Why it happens:** The refactor touches the same function (`materializeIdentity`) and the same persistence call (`UpdateUserOnLogin`) that this test exercises, but only display_name is supposed to become unconditional — role/email must remain gated exactly as today.
**How to avoid:** Keep the new reconciliation call passing `user.Role` and `user.Email` **unchanged** (pass-through, per D-06) to `UpdateUserOnLogin` — never derive a new role/email value in the new unconditional path. Only `applyLoginSync` (still gated) computes new role/email values.
**Warning signs:** If the planner's task touches `resolveLoginRole` or the role-derivation logic at all, that's scope creep outside AUTH-06.

### Pitfall 2: Double-writing display_name in one login (gated path also fires)

**What goes wrong:** When both the new unconditional reconciliation AND `applyLoginSync` (gated, interactive + loginSyncEnabled) run in the same request, if `applyLoginSync`'s display-name block isn't removed, it recomputes the exact same heuristic a second time and potentially issues a second `UpdateUserOnLogin` call (wasted write, or — worse — a race if the in-memory `user.DisplayName` wasn't updated between the two calls, causing the second heuristic check to use stale data).
**Why it happens:** `applyLoginSync` receives `user` by pointer semantics but the docstring says "mutates the passed-in user in place" — if the first (unconditional) reconciliation doesn't update the in-memory `user.DisplayName` before `applyLoginSync` runs, `applyLoginSync`'s own staleness check (`user.DisplayName == claims.Subject`) would still see the OLD (stale) value and try to "reconcile" again, redundantly.
**How to avoid:** Remove the display-name block from `applyLoginSync` entirely (Claude's discretion in CONTEXT.md explicitly names this as the preferred outcome: "avoid computing/writing display-name twice per login"). Update the in-memory `user.DisplayName` immediately after the new unconditional write succeeds, before falling through to the `loginSyncEnabled && interactive` gate.
**Warning signs:** `applyLoginSync`'s existing unit tests (`TestApplyLoginSync_PromotesAndRefreshesMetadata`, `TestApplyLoginSync_PreservesOperatorRename`) currently assert on `gotName`/`dn` captured from `UpdateUserOnLogin`'s call — these tests MUST be updated (or their display-name assertions removed) once the display-name block is extracted out of `applyLoginSync`. This is an EXPECTED test change, not a regression — flag it explicitly in the plan so execution doesn't treat the test diff as a red flag.

### Pitfall 3: Forgetting the introspection-path fix, silently leaving a "not every login" gap

**What goes wrong:** AUTH-06 success criterion 3 says "Reconciliation runs on every successful login, not only at first provisioning." If `resolveIntrospection`'s `OIDCClaims` construction (identitystore.go:613-617) isn't given `Name: nameFromClaims(res.Raw)`, introspected/opaque-token logins will always have `claims.Name == ""`, so the new reconciliation logic's `claims.Name != ""` guard will always be false on that path — the code "works" (doesn't panic, doesn't regress) but silently never reconciles for that login mode. This is the exact failure mode D-04 pre-emptively identifies.
**Why it happens:** It's an easy line to miss because the resolveIntrospection code compiles and passes existing tests without it — the bug is a silent no-op, not a crash or test failure, unless a NEW test specifically covers this path.
**How to avoid:** Add the one-line fix (`Name: nameFromClaims(res.Raw)`) AND add a new test asserting introspection-path reconciliation actually fires (see Validation Architecture below).

### Pitfall 4: Assuming `UpdateUserOnLogin` is no-op-safe at the SQL level

**What goes wrong:** CONTEXT.md's D-06 phrasing ("already has an atomic no-op-safe UPDATE") could be misread as "the SQL statement itself detects no-ops." It does not — verified in this research: `UpdateUserOnLogin`'s SQL (`internal/storage/postgres/users.go:242-244`) unconditionally executes `UPDATE users SET display_name=$1, email=$2, role=$3 WHERE id=$4 AND deleted_at IS NULL` every time it's called, regardless of whether the values actually changed.
**Why it happens:** The no-op *skip* is entirely the caller's responsibility — `applyLoginSync` implements it at loginsync.go:92-95 (`if newDisplay == user.DisplayName && newEmail == user.Email && newRole == user.Role { return user, nil }`) BEFORE calling `UpdateUserOnLogin`, not inside the storage method.
**How to avoid:** The new reconciliation helper/call site must implement its own no-op guard (only call `UpdateUserOnLogin` when `changed` is true) — do not assume the storage layer will skip a redundant write for you. An unconditional call on every login without this guard would issue a write on every single login regardless of change, which is both wasteful and — more importantly — would update `updated_at`-style triggers/audit trails if any exist on the `users` table (none currently do, per the migrations, but this is still bad practice worth avoiding).

## Code Examples

### `nameFromClaims` — the exact fallback logic to reuse (already correct, do not modify)
```go
// Source: internal/auth/oidc_verifier.go:153-168
// nameFromClaims resolves a human-friendly display name from the claims,
// preferring the standard "name" claim and falling back to
// "preferred_username". Returns "" when neither is present or both are empty.
func nameFromClaims(raw map[string]json.RawMessage) string {
	for _, claim := range []string{"name", "preferred_username"} {
		rawVal, ok := raw[claim]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(rawVal, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}
```

### `displayNameFromUserinfo` — the oauth2-path equivalent (D-05 resolution: already correct)
```go
// Source: internal/auth/oauth2_provider.go:196-211
// displayNameFromUserinfo resolves a human-friendly display name from the
// userinfo body, preferring "name" and falling back to "login". Returns "" when
// neither is a non-empty string.
func displayNameFromUserinfo(userinfo map[string]json.RawMessage) string {
	for _, field := range []string{"name", "login"} {
		raw, ok := userinfo[field]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}

// Called at internal/auth/oauth2_provider.go:111-117 inside Exchange:
return &OIDCClaims{
    Issuer:  p.issuerID,
    Subject: subject,
    Email:   email,
    Name:    displayNameFromUserinfo(userinfo),  // ALREADY WIRED — no fix needed
    Raw:     userinfo,
}, nil
```

### `UpdateUserOnLogin` — the exact persistence method to reuse unchanged
```go
// Source: internal/storage/postgres/users.go:239-253
// UpdateUserOnLogin sets display_name, email, and role on an active user in a
// single statement. Returns ErrUserNotFound if no active user has the given ID.
func (s *AuthStore) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	const q = `
		UPDATE users SET display_name = $1, email = $2, role = $3
		WHERE id = $4::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, displayName, email, role, userID)
	if err != nil {
		return fmt.Errorf("update user on login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}
```

### `resolveIntrospection`'s claims construction — the exact spot for the D-04 fix
```go
// Source: internal/auth/identitystore.go:613-619 (current — MISSING Name)
claims := &OIDCClaims{
    Issuer:  res.Issuer,
    Subject: res.Subject,
    Raw:     res.Raw,
}
// Opaque-token introspection is never an interactive login.
return s.materializeIdentity(ctx, claims, false)

// REQUIRED FIX (D-04):
claims := &OIDCClaims{
    Issuer:  res.Issuer,
    Subject: res.Subject,
    Name:    nameFromClaims(res.Raw),  // ADD THIS LINE
    Raw:     res.Raw,
}
```
`res.Raw` is `map[string]json.RawMessage` (confirmed: `internal/auth/introspection.go:25-30`, `IntrospectionResult.Raw`), the exact type `nameFromClaims` accepts — no type conversion needed.

### `jitResolve`'s seeding — the exact spot for the D-07 fix
```go
// Source: internal/auth/identitystore.go:685-691 (current)
u := &storage.User{
    Kind:        storage.KindHuman,
    DisplayName: claims.Subject, // operator can rename later
    Email:       claims.Email,
    Role:        string(role),
}

// REQUIRED FIX (D-07):
seedName := claims.Subject
if claims.Name != "" {
    seedName = claims.Name
}
u := &storage.User{
    Kind:        storage.KindHuman,
    DisplayName: seedName, // prefer claims.Name; falls back to Subject; operator can still rename later
    Email:       claims.Email,
    Role:        string(role),
}
```

## State of the Art

No stack/library changes involved — this is an internal-only Go refactor. There is no "old approach → new approach" library migration to document. The one relevant piece of external-standard context is:

| Concept | Current codebase alignment | Confirmation |
|---------|----------------------------|--------------|
| OIDC `name`/`preferred_username` claim semantics | Codebase's fallback order (`name` then `preferred_username`) matches the OIDC Core spec's intent — `name` is the full display name (part of the `profile` scope), `preferred_username` is a user-chosen handle, and neither is meant to be a unique identifier (the codebase correctly uses `Subject`, not `Name`, for the identity binding key) | `[CITED: OIDC Core standard claims — profile scope]` (WebSearch, MEDIUM confidence — see Sources) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | No `users` table trigger/audit-log fires on every `UPDATE users` regardless of value change (referenced in Pitfall 4) | Common Pitfalls | Low — this claim is based on reading the current migrations directory structure conceptually, not an exhaustive migration-by-migration audit in this session. If wrong, an unconditional (unguarded) write would have a side effect beyond wasted I/O. Mitigated regardless: the recommended design already guards writes with a `changed` check, so this assumption doesn't gate correctness, only explains *why* the guard matters. |

**If this table is empty:** N/A — one low-risk assumption logged above; it does not affect the correctness of the recommended design since the no-op guard is required regardless of whether a trigger exists.

## Open Questions

All five research questions posed in the phase brief were resolved during this session:

1. ~~Confirm `UpdateUserOnLogin` signature/location/semantics~~ — **RESOLVED.** Single-query, `internal/storage/postgres/users.go:241-253`, independent args, active-row-guarded, not itself no-op-safe (caller must guard). See Summary and Pitfall 4.
2. ~~Confirm whether `oauth2_provider.go`'s `Exchange` populates `Name`~~ — **RESOLVED: yes, already does**, via `displayNameFromUserinfo`. No fix needed for D-05.
3. ~~Identify existing unit tests covering the relevant functions~~ — **RESOLVED.** See Validation Architecture below for the full file/test inventory.
4. ~~Existing test helpers/fixtures to reuse~~ — **RESOLVED.** `usersBackendStub` (`internal/auth/usersbackend_stub_test.go`, external `auth_test` package) for black-box tests via `Resolve`/`ResolveLogin`; `loginSyncFakeBackend` (`internal/auth/loginsync_internal_test.go`, internal `auth` package) for white-box tests of unexported helpers. See Validation Architecture.
5. ~~Confirm no existing config-flag precedent makes "unconditional regardless of LoginSyncEnabled" surprising~~ — **RESOLVED.** `resolveAPIKey`'s unconditional `s.tracker.Touch(key.ID)` (identitystore.go:391) is a direct precedent for "some resolve-path side effects are unconditional, others are config-gated, in the same codebase." No config flag exists or is implied for display-name reconciliation specifically (`Auth.OIDC.SyncOnLogin` maps only to `LoginSyncEnabled`, confirmed via `cmd/specgraph/serve.go:223`).

No remaining open questions block planning.

## Environment Availability

Skipped — this phase has no external dependencies (databases, CLIs, services) beyond what the existing test suite already exercises (Postgres via testcontainers for the `//go:build integration` tests, already verified working in prior phases per STATE.md). No new external dependency is introduced.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify/require` |
| Config file | none — plain `go test`, driven by Taskfile.yml |
| Quick run command | `go test -short -race ./internal/auth/... -run '<TestPattern>'` |
| Full suite command | `task check` (runs `go test -short -race ./...` plus fmt/lint/build gates); `task pr-prep` additionally runs `task test:integration` (`go test -tags integration -race -v -p 1 ./...`, requires Docker/Postgres) |

### Existing Test Inventory (verified this session)

| File | Package | Scope | Relevant existing tests |
|------|---------|-------|--------------------------|
| `internal/auth/loginsync_internal_test.go` | `auth` (white-box) | `applyLoginSync`, `resolveLoginRole`, `isPromotion` | `TestApplyLoginSync_PromotesAndRefreshesMetadata`, `TestApplyLoginSync_PreservesOperatorRename`, `TestApplyLoginSync_NoOpSkipsWrite`, `TestApplyLoginSync_DemotionPersistFailure_Denies`, `TestApplyLoginSync_UserNotFound_Denies`, `TestApplyLoginSync_DemotionSucceeds`, `TestApplyLoginSync_PromotionPersistFailure_BestEffort`, `TestApplyLoginSync_MetadataOnlyFailure_BestEffort`, `TestApplyLoginSync_AllowlistMiss_Denies`, `TestApplyLoginSync_AllowlistSkippedOnAbsentEmail` — **two of these (`PromotesAndRefreshesMetadata`, `PreservesOperatorRename`) assert display-name behavior that must be UPDATED (not left broken) once the display-name block is extracted out of `applyLoginSync`, per Pitfall 2.** |
| `internal/auth/oidc_verifier_internal_test.go` | `auth` (white-box) | `nameFromClaims` | `TestNameFromClaims` — 5 subtests covering name-present, name-only, fallback, both-absent, empty-name-falls-through. Fully covers D-03/D-04's claim-parsing dependency; no new test needed here. |
| `internal/auth/identitystore_test.go` | `auth_test` (black-box) | `Resolve`/`ResolveJWT`/existing-binding/JIT paths | `TestResolveJWT_ExistingBindingResolves`, `TestResolveJWT_ExistingBinding_IgnoresClaimsMapping`, `TestResolveJWT_JITCreatesNewUser`, `TestResolveJWT_SoftDeletedUserUnauthenticated`, etc. — none currently assert display-name reconciliation; NEW tests needed here (see gaps below). None of these set `DisplayName == Subject` in their fixtures, so none will break from the new unconditional logic. |
| `internal/auth/identitystore_jit_test.go` | `auth_test` (black-box) | JIT rate-limit, allowlist, claims-mapping | `TestResolveLogin_ThreadsIssuerOnBindingHit`, `TestResolveLogin_ThreadsIssuerOnJIT`, `TestJIT_RateLimitExhaustion`, `TestJIT_EmailAllowlist_*`, `TestJIT_ClaimsMapping_*` — none currently assert `DisplayName` on the JIT-created user; a NEW test for D-07 (seed from `claims.Name`) belongs here or in `identitystore_test.go`. |
| `internal/auth/introspection_test.go` | `auth_test` (black-box) | `resolveIntrospection` via `Resolve` | `TestIntrospection_ActiveResourceBound_Resolves`, `TestIntrospection_Inactive_Rejected`, `TestIntrospection_ServerError_Transient`, `TestIntrospection_WrongAudience_Rejected`, `TestIntrospection_MultiIntrospector_FirstMatchWins`, `TestIntrospection_APIKeyNeverIntrospected` — none currently exercise a `name` claim in the introspection JSON payload. NEW test needed for D-04 (introspection-path claim now populates `Name` and reconciliation fires). |
| `internal/auth/identitystore_authn_integration_test.go` | `auth_test`, `//go:build integration` | End-to-end against real Postgres + real OIDC verifier | `TestIdentityStore_JWT_ResolvesViaExistingBinding`, `TestIdentityStore_JWT_JITCreatesThenResolvesViaBinding`, `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` — **the last one is the critical regression guard for Pitfall 1**: it explicitly proves non-interactive resolve does NOT persist a role change even with `LoginSyncEnabled: true`. Must continue to pass unmodified (it only touches role, not display_name) after this phase's changes. A NEW integration test asserting display-name reconciliation fires on BOTH interactive and non-interactive paths (proving the D-01 decoupling end-to-end against real Postgres) is the natural complement to add alongside it. |
| `internal/auth/oauth2_provider_test.go` | `auth` (internal, based on file naming) | `oauth2LoginProvider.Exchange` | `TestOAuth2Provider_Exchange_VerifiedEmailFallback`, `TestOAuth2Provider_Exchange_UnverifiedEmailBlank`, `TestOAuth2Provider_AuthCodeURL`, `TestOAuth2Provider_Exchange_MissingSubjectFatal` — none currently assert `claims.Name` is populated. A NEW lightweight test asserting `Exchange`'s returned `OIDCClaims.Name` is non-empty when userinfo carries a `name` field would give D-05's "already correct" finding explicit regression protection (currently implicit/untested). |
| `internal/auth/usersbackend_stub_test.go` | `auth_test` | Shared fixtures | `usersBackendStub` (black-box, has `updateUserOnLogin` field), `loginSyncFakeBackend` (white-box, in `loginsync_internal_test.go`), `activeUser(id, role, kind)` helper (sets `DisplayName: "test-" + id` — **note:** any new black-box test that needs to exercise the staleness heuristic must NOT use `activeUser` as-is; it must construct a `storage.User` with `DisplayName` explicitly equal to the token's `sub` claim to trigger the reconciliation branch). |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-06 (SC1) | Existing-user JWT login with stale (`DisplayName == Subject`) fallback + a `name` claim present → `display_name` updated to the claim value | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconcilesStaleDisplayName` | ❌ Wave 0 — new test in `identitystore_test.go` |
| AUTH-06 (SC2) | Same as SC1 but via `resolveIntrospection` (opaque token) — proves the D-04 fix closes the gap | unit | `go test -short -race ./internal/auth/... -run TestIntrospection_ReconcilesStaleDisplayName` | ❌ Wave 0 — new test in `introspection_test.go` |
| AUTH-06 (SC3, "every login not just first provisioning") | A login that is NON-interactive AND has `LoginSyncEnabled: false` still reconciles display_name (proves full decoupling, D-01) | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconciliationRunsWithoutLoginSync` | ❌ Wave 0 — new test in `identitystore_test.go` |
| AUTH-06 (SC4, "no regression when no usable claim") | Existing user with an already-operator-set `DisplayName` (≠ Subject) and a login with NO `name`/`preferred_username` claim → `display_name` unchanged; also: stale-fallback user logging in with NO usable claim present → `display_name` unchanged (not regressed to Subject, since it's already Subject — this is really "no spurious write", covered by a no-op-skip assertion) | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim` | ❌ Wave 0 — new test in `identitystore_test.go` |
| D-06 (persistence contract) | Reconciliation write passes through unchanged `email`/`role` — proves no accidental role/email mutation from the new unconditional path | unit | `go test -short -race ./internal/auth/... -run TestResolveJWT_ReconciliationPreservesRoleAndEmail` | ❌ Wave 0 — new test in `identitystore_test.go` (or fold into SC1's test as an additional assertion) |
| D-07 (JIT seeding) | `jitResolve` with `claims.Name` present seeds `DisplayName` from it, not `Subject` | unit | `go test -short -race ./internal/auth/... -run TestJIT_SeedsDisplayNameFromClaimsName` | ❌ Wave 0 — new test in `identitystore_jit_test.go` |
| D-05 (regression guard) | `oauth2LoginProvider.Exchange` returns non-empty `Name` when userinfo has a `name` field | unit | `go test -short -race ./internal/auth/... -run TestOAuth2Provider_Exchange_PopulatesName` | ❌ Wave 0 — new test in `oauth2_provider_test.go` (optional but recommended; closes an implicit-only guarantee) |
| Pitfall 1 regression | Non-interactive resolve with `LoginSyncEnabled: true` still does NOT persist a role change (unmodified existing test) | integration | `go test -tags integration -race ./internal/auth/... -run TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` | ✅ exists — `identitystore_authn_integration_test.go:148` |
| End-to-end reconciliation (both gate states) | Real Postgres: stale-fallback user reconciles on both interactive and non-interactive resolve | integration | `go test -tags integration -race ./internal/auth/... -run TestIdentityStore_DisplayNameReconciliation` | ❌ Wave 0 — new test in `identitystore_authn_integration_test.go`, sibling to the existing `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` |

### Sampling Rate

- **Per task commit:** `go test -short -race ./internal/auth/... -run <touched-area-pattern>`
- **Per wave merge:** `task check` (full unit suite + lint/build/fmt gates)
- **Phase gate:** `task check` full green, plus `task test:integration` (requires Docker) for the two integration tests listed above, before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/auth/identitystore_test.go` — add `TestResolveJWT_ReconcilesStaleDisplayName`, `TestResolveJWT_ReconciliationRunsWithoutLoginSync`, `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim`, `TestResolveJWT_ReconciliationPreservesRoleAndEmail` (covers AUTH-06 SC1, SC3, SC4, D-06)
- [ ] `internal/auth/introspection_test.go` — add `TestIntrospection_ReconcilesStaleDisplayName` (covers AUTH-06 SC2 / D-04)
- [ ] `internal/auth/identitystore_jit_test.go` — add `TestJIT_SeedsDisplayNameFromClaimsName` (covers D-07)
- [ ] `internal/auth/oauth2_provider_test.go` — add `TestOAuth2Provider_Exchange_PopulatesName` (optional regression guard for D-05's "already correct" finding)
- [ ] `internal/auth/loginsync_internal_test.go` — UPDATE `TestApplyLoginSync_PromotesAndRefreshesMetadata` and `TestApplyLoginSync_PreservesOperatorRename` to drop their display-name assertions (or restructure) once the display-name block is extracted out of `applyLoginSync` — this is a required edit to existing tests, not a new file, but must be explicitly planned so it isn't mistaken for an unplanned regression during execution
- [ ] `internal/auth/identitystore_authn_integration_test.go` — add `TestIdentityStore_DisplayNameReconciliation` (end-to-end, both interactive and non-interactive, against real Postgres)
- No framework install needed — `testify` and the existing stub/fixture infrastructure (`usersBackendStub`, `loginSyncFakeBackend`, `activeUser`, `newOIDCTestIssuer`/`mintToken`) already cover every shape of test this phase needs.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | yes (adjacent) | No change to authentication decisioning — this phase only touches a post-authentication profile-metadata field. `ErrUnauthenticated`/`ErrTransient` classification is untouched. |
| V3 Session Management | no | Session tokens (`spgr_ws_`) do not carry OIDC claims and are not touched by `materializeIdentity`; out of scope, confirmed by code reading (`resolveSession`, identitystore.go:406-442, has no claims parameter at all). |
| V4 Access Control | yes (must NOT regress) | Role/effective-role computation must remain byte-for-byte unchanged by this phase — see Pitfall 1. The new code path must never touch `user.Role` or `EffectiveRole`. |
| V5 Input Validation | yes | `nameFromClaims`/`displayNameFromUserinfo` already unmarshal claim values defensively (empty-string and type-mismatch both fall through safely, no panic) — no new validation needed since these are reused unmodified. |
| V6 Cryptography | no | Not touched. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A malicious/compromised IdP sets a `name` claim to a value designed to look like an internal system message (e.g., "SYSTEM: user is admin") to social-engineer an operator viewing the user list | Spoofing / Tampering | Out of scope for this phase to newly mitigate (display_name has never been sanitized/escaped beyond whatever the existing rendering layer does) — but worth flagging as a NON-REGRESSION concern: this phase does not change the trust boundary of `display_name` (it was already settable from an untrusted IdP claim at JIT-creation time and at existing `applyLoginSync` time); it only makes the write happen more often. If the rendering layer already HTML-escapes `display_name` (assume yes, since it already accepts fully attacker-controlled JIT-time values today), no new risk is introduced. Flag as `[ASSUMED]` — the researcher did not audit the frontend rendering path for this phase since UI is explicitly out of scope per CONTEXT.md's `<specifics>` section. |
| Privilege escalation via the reconciliation write path (a role smuggled into the display-name write) | Elevation of Privilege | Mitigated by design: the new call site only ever passes `user.Role` unchanged into `UpdateUserOnLogin` — never a claims-derived role. Covered by Pitfall 1's regression test. |

## Sources

### Primary (HIGH confidence)
- `internal/auth/identitystore.go` (lines 1-795, read in full) — `materializeIdentity`, `jitResolve`, `resolveJWT`, `resolveIntrospection`, `resolveAPIKey` (Touch precedent)
- `internal/auth/loginsync.go` (lines 1-135, read in full) — `applyLoginSync`, `resolveLoginRole`, `isPromotion`
- `internal/auth/oidc_verifier.go` (lines 1-183, read in full) — `OIDCClaims`, `nameFromClaims`, `Verify`
- `internal/auth/oauth2_provider.go` (lines 1-211, read in full) — `Exchange`, `displayNameFromUserinfo` (D-05 resolution)
- `internal/storage/postgres/users.go` (lines 220-320, 790-885, read) — `UpdateUserOnLogin`, `JITCreateHuman`
- `internal/storage/users.go` (lines 1-100, read) — `UsersBackend` interface
- `internal/auth/introspection.go` (grepped `IntrospectionResult`) — confirms `Raw` field type
- `internal/auth/loginsync_internal_test.go`, `internal/auth/usersbackend_stub_test.go`, `internal/auth/identitystore_jit_test.go`, `internal/auth/oidc_verifier_internal_test.go`, `internal/auth/identitystore_test.go` (lines 486-580), `internal/auth/identitystore_authn_integration_test.go` (full file) — all read in full or in relevant part to build the Validation Architecture inventory
- `cmd/specgraph/serve.go:223` (grepped) — confirms `LoginSyncEnabled` ← `cfg.Auth.OIDC.SyncOnLogin`, no separate display-name config flag exists

### Secondary (MEDIUM confidence)
- WebSearch: OpenID Connect Core standard claims (`name`, `preferred_username`, `profile` scope) — confirms the codebase's fallback ordering matches spec intent and that neither claim should be a unique identifier (codebase already correctly uses `Subject` for binding)

### Tertiary (LOW confidence)
- None used for this phase.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new libraries/stack involved, pure internal refactor
- Architecture: HIGH — every integration point, function signature, and test fixture was read directly from current on-disk code in this session, not inferred
- Pitfalls: HIGH — all four pitfalls are derived from concrete existing test assertions and code semantics read in this session (e.g., `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive`'s explicit non-interactive-no-persist assertion), not speculative

**Research date:** 2026-07-15
**Valid until:** No expiry pressure — this is a static internal-codebase research artifact tied to the current commit (`5f5ab3d0`); re-verify only if `identitystore.go`/`loginsync.go`/`oauth2_provider.go`/`users.go` change materially before this phase is planned/executed.
