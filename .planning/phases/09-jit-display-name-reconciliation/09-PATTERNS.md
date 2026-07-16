# Phase 9: JIT Display Name Reconciliation - Pattern Map

**Mapped:** 2026-07-15
**Files analyzed:** 8 (all modifications, no new files)
**Analogs found:** 8 / 8 (all analogs found in-repo; several are "self" — existing code in the same file being extended/generalized)

**Note on analog sourcing:** This phase is an *extraction and unconditional re-application* of existing, already-proven logic (per RESEARCH.md). RESEARCH.md's `## Code Examples` section already contains exact current-code excerpts and proposed diffs read directly from disk this session — those are used verbatim below as the primary source, per the orchestrator's instruction, rather than re-reading files already quoted.

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|----------------|------|-----------|-----------------|----------------|
| `internal/auth/identitystore.go` (`materializeIdentity`, existing-user branch) | service (identity resolution business logic) | request-response with CRUD side-effect | `internal/auth/identitystore.go:391` (`resolveAPIKey`'s unconditional `s.tracker.Touch(key.ID)`) | exact — same function family, same "unconditional side effect next to gated one" shape |
| `internal/auth/identitystore.go` (`jitResolve`, D-07 seed fix) | service (JIT provisioning) | CRUD (create) | `internal/auth/identitystore.go:685-691` (jitResolve itself, current seeding line) | exact — self-modification, one-line change in an existing struct literal |
| `internal/auth/identitystore.go` (`resolveIntrospection`, D-04 claim-parity fix) | service (claims construction) | request-response | `internal/auth/oidc_verifier.go` `Verify` (JWT path, already populates `Name` via `nameFromClaims`) | exact — parity fix, mirrors sibling resolution path already in the same package |
| `internal/auth/loginsync.go` (`applyLoginSync`, remove display-name block) | service (login-sync business logic) | CRUD | `internal/auth/loginsync.go:83-95` (existing display-name block being extracted) | exact — self, extraction/removal not addition |
| `internal/auth/identitystore_test.go` (new tests) | test | unit / request-response | `TestResolveJWT_JITCreatesNewUser`, `TestResolveJWT_ExistingBindingResolves` (same file) | exact — same file, same black-box `Resolve`/JWT test shape |
| `internal/auth/introspection_test.go` (new test) | test | unit / request-response | `TestIntrospection_ActiveResourceBound_Resolves` (same file) | exact — same file, same black-box introspection test shape |
| `internal/auth/identitystore_jit_test.go` (new test) | test | unit / CRUD | `TestResolveLogin_ThreadsIssuerOnJIT`, `TestJIT_ClaimsMapping_*` (same file) | exact — same file, same JIT-provisioning test shape |
| `internal/auth/oauth2_provider_test.go` (new test, optional) | test | unit / request-response | `TestOAuth2Provider_Exchange_VerifiedEmailFallback` (same file) | exact — same file, same `Exchange` test shape |
| `internal/auth/loginsync_internal_test.go` (update existing tests) | test | unit / white-box | `TestApplyLoginSync_PromotesAndRefreshesMetadata`, `TestApplyLoginSync_PreservesOperatorRename` (same file, being updated in place) | exact — self-modification |
| `internal/auth/identitystore_authn_integration_test.go` (new integration test) | test | integration / end-to-end | `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` (same file, lines 148-214) | exact — same file, same real-Postgres integration harness |

## Pattern Assignments

### `internal/auth/identitystore.go` — `materializeIdentity` existing-user branch (service, request-response + CRUD side-effect)

**Analog:** `resolveAPIKey` (same file, line 391) — precedent for "unconditional side-effect alongside a gated one" in the exact same package/function family.

**Precedent excerpt (unconditional side-effect, unmodified by this phase):**
```go
// Source: internal/auth/identitystore.go:391 (resolveAPIKey)
s.tracker.Touch(key.ID)  // unconditional — no config gate, runs on every resolve
return &Identity{...}
```

**New pattern to apply** (illustrative shape from RESEARCH.md; exact helper name/signature is Claude's discretion per CONTEXT.md):
```go
// Insertion point: materializeIdentity's existing-user branch, after the
// soft-delete check, BEFORE the `if s.loginSyncEnabled && interactive` gate.
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

**Staleness heuristic to reuse (D-03), copy verbatim from the analog:**
```go
// Source: internal/auth/loginsync.go:83-86 (existing, proven, unit-tested)
newDisplay := user.DisplayName
if user.DisplayName == claims.Subject && claims.Name != "" {
    newDisplay = claims.Name // update only if never renamed by an operator
}
```

**Persistence call to reuse unchanged (D-06):**
```go
// Source: internal/storage/postgres/users.go:239-253
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
Note: not itself no-op-safe — caller (the new reconciliation call) MUST guard with its own `changed` check before calling, mirroring `applyLoginSync`'s existing no-op skip at `loginsync.go:92-95`.

**Error/log discipline pattern (apply to new logging):**
Use `slog.LogAttrs` with structured fields, matching the existing convention in `identitystore.go`/`loginsync.go` — never bare `log.Printf`. Use `slog.String`, and `slog.Bool("audit", true)` for security-relevant changes (role changes use this; display-name changes are lower-sensitivity but should still follow the structured-field shape).

---

### `internal/auth/identitystore.go` — `jitResolve` D-07 seed fix (service, CRUD create)

**Analog:** self (existing struct literal at the fix site).

**Current code (before):**
```go
// Source: internal/auth/identitystore.go:685-691
u := &storage.User{
    Kind:        storage.KindHuman,
    DisplayName: claims.Subject, // operator can rename later
    Email:       claims.Email,
    Role:        string(role),
}
```

**Required fix (D-07), exact shape to apply:**
```go
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

---

### `internal/auth/identitystore.go` — `resolveIntrospection` D-04 claim-parity fix (service, request-response)

**Analog:** the JWT path's `OIDCVerifier.Verify` (`internal/auth/oidc_verifier.go`), which already populates `Name` via `nameFromClaims` — this fix brings `resolveIntrospection` to parity.

**Current code (before, missing `Name`):**
```go
// Source: internal/auth/identitystore.go:613-619
claims := &OIDCClaims{
    Issuer:  res.Issuer,
    Subject: res.Subject,
    Raw:     res.Raw,
}
// Opaque-token introspection is never an interactive login.
return s.materializeIdentity(ctx, claims, false)
```

**Required fix (D-04):**
```go
claims := &OIDCClaims{
    Issuer:  res.Issuer,
    Subject: res.Subject,
    Name:    nameFromClaims(res.Raw),  // ADD THIS LINE
    Raw:     res.Raw,
}
```
`res.Raw` is `map[string]json.RawMessage` (`internal/auth/introspection.go`, `IntrospectionResult.Raw`) — exact type `nameFromClaims` accepts, no conversion needed.

**Reused claim-parsing function (do not modify, copy nowhere — just call):**
```go
// Source: internal/auth/oidc_verifier.go:153-168
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

**D-05 note (no fix needed, for completeness / regression-test context):**
```go
// Source: internal/auth/oauth2_provider.go:196-211 (already correct, unmodified)
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

---

### `internal/auth/loginsync.go` — `applyLoginSync` display-name block removal (service, CRUD)

**Analog:** self — the block being removed (lines 83-86, 93, 131 per RESEARCH.md).

**Pattern:** Once the new unconditional reconciliation runs first in `materializeIdentity` and updates `user.DisplayName` in place, `applyLoginSync`'s own display-name computation becomes dead/misleading logic. Remove the block; `applyLoginSync` should thereafter compute/persist role + email + allowlist only, still calling `UpdateUserOnLogin` but passing through the (already-current) `user.DisplayName` unchanged.

**Existing no-op-skip pattern to preserve for the remaining role/email fields (copy shape, not content):**
```go
// Source: internal/auth/loginsync.go:92-95 (existing, keep this shape for role/email)
if newDisplay == user.DisplayName && newEmail == user.Email && newRole == user.Role {
    return user, nil
}
```
After the display-name block is removed, this no-op check should compare only the fields `applyLoginSync` still computes (email/role), using `user.DisplayName` (already reconciled upstream) as the pass-through value into `UpdateUserOnLogin`.

---

### Test files — pattern assignments

#### `internal/auth/identitystore_test.go` (test, unit/request-response)

**Analog:** `TestResolveJWT_JITCreatesNewUser`, `TestResolveJWT_ExistingBindingResolves` (same file) — black-box tests via `Resolve`, using `usersBackendStub` fixtures.

**New tests to add** (per RESEARCH.md Validation Architecture / Wave 0 Gaps):
- `TestResolveJWT_ReconcilesStaleDisplayName` — existing-user JWT login, `DisplayName == Subject`, `claims.Name` present → expect `display_name` updated.
- `TestResolveJWT_ReconciliationRunsWithoutLoginSync` — non-interactive AND `LoginSyncEnabled: false` → display_name still reconciles (proves D-01 decoupling).
- `TestResolveJWT_PreservesDisplayNameWhenNoUsableClaim` — operator-set `DisplayName` (≠ Subject) with no `name`/`preferred_username` claim → unchanged, no write.
- `TestResolveJWT_ReconciliationPreservesRoleAndEmail` — asserts `UpdateUserOnLogin` call passes through `user.Role`/`user.Email` unchanged (D-06/Pitfall 1 guard).

**Fixture caveat to carry into the new tests (from RESEARCH.md):**
```go
// Source: internal/auth/usersbackend_stub_test.go — activeUser helper
// activeUser(id, role, kind) sets DisplayName: "test-" + id
// New tests exercising the staleness heuristic must NOT use activeUser as-is;
// construct a storage.User with DisplayName explicitly equal to the token's
// sub claim to trigger the reconciliation branch.
```

#### `internal/auth/introspection_test.go` (test, unit/request-response)

**Analog:** `TestIntrospection_ActiveResourceBound_Resolves` (same file).

**New test:** `TestIntrospection_ReconcilesStaleDisplayName` — introspection payload carries a `name` claim, bound user has stale `DisplayName == Subject` → expect reconciliation fires (covers AUTH-06 SC2 / D-04 fix).

#### `internal/auth/identitystore_jit_test.go` (test, unit/CRUD)

**Analog:** `TestResolveLogin_ThreadsIssuerOnJIT`, `TestJIT_ClaimsMapping_*` (same file).

**New test:** `TestJIT_SeedsDisplayNameFromClaimsName` — `claims.Name` present at first JIT provisioning → new user's `DisplayName` seeded from claim, not `Subject` (covers D-07).

#### `internal/auth/oauth2_provider_test.go` (test, unit/request-response, optional per RESEARCH.md)

**Analog:** `TestOAuth2Provider_Exchange_VerifiedEmailFallback` (same file).

**New test (optional regression guard):** `TestOAuth2Provider_Exchange_PopulatesName` — userinfo has a `name` field → returned `OIDCClaims.Name` non-empty (regression-protects D-05's "already correct, no fix" finding, which is currently only implicitly tested).

#### `internal/auth/loginsync_internal_test.go` (test, unit/white-box — UPDATE existing tests, not new)

**Analog:** self.

**Required updates:** `TestApplyLoginSync_PromotesAndRefreshesMetadata` and `TestApplyLoginSync_PreservesOperatorRename` currently assert display-name behavior via captured `gotName`/`dn` values from `UpdateUserOnLogin`. Once the display-name block is extracted out of `applyLoginSync`, these assertions must be updated (dropped or restructured to assert display-name is passed through unchanged) — this is an EXPECTED test change per Pitfall 2, not a regression.

#### `internal/auth/identitystore_authn_integration_test.go` (test, integration/end-to-end)

**Analog:** `TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` (same file, lines 148-214) — the critical regression guard for Pitfall 1; must continue to pass unmodified (only touches role, not display_name).

**New test:** `TestIdentityStore_DisplayNameReconciliation` — real Postgres, asserts display-name reconciliation fires on BOTH interactive and non-interactive resolve paths, proving the D-01 decoupling end-to-end. Sibling to the existing gate test, same harness/fixtures.

## Shared Patterns

### Structured logging discipline
**Source:** `internal/auth/identitystore.go` / `internal/auth/loginsync.go` (existing convention, no single line range — pervasive)
**Apply to:** All new logging in the reconciliation call site
```go
slog.LogAttrs(ctx, slog.LevelInfo, "auth: display-name reconciled",
    slog.String("user_id", user.ID), slog.String("old", user.DisplayName), slog.String("new", newName))
```
Never use bare `log.Printf`. Use `slog.Bool("audit", true)` only for security-relevant changes (role); display-name changes follow the same structured-field shape but do not need the audit flag unless the planner decides otherwise.

### No-op write guard before `UpdateUserOnLogin`
**Source:** `internal/auth/loginsync.go:92-95` (existing, proven)
**Apply to:** The new unconditional reconciliation call site — `UpdateUserOnLogin` is NOT itself no-op-safe at the SQL level (per RESEARCH.md Pitfall 4); the caller must gate the call on a `changed` boolean.

### Value-threading discipline for `interactive`
**Source:** `internal/auth/identitystore.go:510-512` (existing convention)
**Apply to:** The new reconciliation helper must NOT accept, thread, or re-derive `interactive` — D-01 requires the new logic run regardless of it. Do not call `InteractiveLoginFromContext` from the new helper.

### Claim-to-display-name resolution (do not duplicate, only call)
**Source:** `internal/auth/oidc_verifier.go:156` (`nameFromClaims`) and `internal/auth/oauth2_provider.go:199` (`displayNameFromUserinfo`)
**Apply to:** `resolveIntrospection` (call `nameFromClaims`), `jitResolve` (consume `claims.Name`, already populated upstream) — no new claim-parsing code anywhere in this phase.

## No Analog Found

None. Every modified file has either a strong in-file precedent (self-modification of existing, well-understood logic) or a clear cross-file analog (`resolveAPIKey`'s unconditional-Touch precedent; `nameFromClaims`/`displayNameFromUserinfo` claim-resolution precedent). This phase introduces no new architectural shape — it is a pure extraction/generalization of existing, proven patterns per RESEARCH.md's "Key insight."

## Metadata

**Analog search scope:** `internal/auth/` (all files touched by this phase), `internal/storage/postgres/users.go`, `internal/storage/users.go` (interface)
**Files scanned:** 8 modified files + their existing analog code, all previously read verbatim during `/gsd-research-phase` (cited with line numbers in RESEARCH.md `## Code Examples` and `## Sources`)
**Pattern extraction date:** 2026-07-15
**Primary source:** RESEARCH.md `## Code Examples`, `## Architecture Patterns`, and `## Validation Architecture` sections (verbatim current-code excerpts and exact fix diffs, read from disk this research session at commit `5f5ab3d0`) — no re-reads performed in this pattern-mapping pass, per orchestrator instruction.
