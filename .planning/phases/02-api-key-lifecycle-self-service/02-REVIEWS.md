---
phase: 2
reviewers: [cursor]
reviewed_at: 2026-07-09T15:24:22Z
plans_reviewed: [02-01-PLAN.md, 02-02-PLAN.md, 02-03-PLAN.md, 02-04-PLAN.md, 02-05-PLAN.md, 02-06-PLAN.md, 02-07-PLAN.md, 02-08-PLAN.md]
---

# Cross-AI Plan Review — Phase 2

> Reviewer independence note: this review was produced by the **cursor-agent** CLI
> (a separate model/session), invoked from an OpenCode orchestration session. The
> requested `--opencode` reviewer was skipped for independence (the orchestrator IS
> OpenCode); `--cursor` was substituted as a genuinely independent reviewer. Single
> external reviewer this run — the Consensus section below reflects one grounded
> review, not a multi-reviewer vote.

---

## Cursor Review

# Phase 2 Plan Review: API Key Lifecycle & Self-Service

## Summary

The eight-plan set is unusually well grounded in the brownfield codebase: research line references match current sources (with only minor drift), security invariants are correctly derived from real mechanisms (`resolveAPIKey` live-role clamp, `ListAPIKeys` empty-`user_id` leak, `actions_test.go` dual hard-coded verb lists, `SPECGRAPH_API_KEY` precedence), and wave ordering (proto → auth → storage/config → handlers → CLI/web) is sound. No implementation has landed yet — `identity.proto` still ends at `UnbindOIDC` and none of the five new RPCs exist. The plans should achieve SC#1–SC#3 if executed faithfully, but two integration gaps — **CSRF middleware mounting/token bootstrap** and **web Connect auth path documentation** — need explicit tasks before the web surface can work end-to-end.

---

## Strengths

- **Research grounding is accurate.** The drift ledger matches the tree: `resolveAPIKey` reads live `users.role` every request and clamps at mint time via `clampedRole` (`internal/auth/identitystore.go:306-348`), `UpdateUserRole` already writes `users.role` (`internal/server/identity_handler.go:155-175`, `internal/storage/postgres/users.go:225-236`), and standing keys pick up demotions on the next call without new schema.

- **Critical enumeration pitfall is real and called out.** `ListAPIKeys` passes `msg.GetUserId()` straight through; empty `user_id` lists all keys (`internal/server/identity_handler.go:319-324`). Plan 05’s hard-set-from-context requirement for `ListMyAPIKeys` is essential.

- **Boot invariant for Cedar `self` verb is correctly identified.** `knownVerbs` today is only `read|write|delete|manage` (`internal/auth/engine.go:155-176`); `TestActionNames_AllParseToKnownVerb` hard-codes the same four verbs (`internal/auth/actions_test.go:33-41`); `TestActionForProcedure_Identity` mirrors the procedure map (`internal/auth/actions_test.go:50-70`). Plan 04’s single-commit requirement for `knownVerbs` + `base.cedar` + `actions.go` is correct — `base.cedar` currently has no `self` permit (`internal/auth/policies/base.cedar:13-48`).

- **Role laundering fix targets the right field.** `Identity` exposes both `Role` and `EffectiveRole`; apikey callers get `EffectiveRole = clampedRole(Role(user.Role), Role(key.RoleDowngrade))` (`internal/auth/identitystore.go:340-348`). Flooring at `caller.EffectiveRole` via exported `RoleMin` (reusing `roleRank` at `:256-277`) closes the downgraded-apikey-caller hole the design describes.

- **Anti-key-chaining gate is implementable.** `Source` is only `"apikey"` or `"oidc"` (`internal/auth/auth.go:19-27`); apikey resolution sets `Source: "apikey"` (`internal/auth/identitystore.go:347`), sessions/JWT paths set `Source: "oidc"` (`:388`, `:483`). Rejecting `Source == "apikey"` on self-mint is coherent.

- **Finding D (CLI precedence) is verified and mandatory.** `resolveAPIKey` prefers `SPECGRAPH_API_KEY` over stored credentials (`cmd/specgraph/client.go:102-111`); login stores `spgr_ws_` sessions (`cmd/specgraph/login.go:257-260`). Without Plan 07’s session-preferring path, self-mint would authenticate as the bootstrap key and hit the source gate.

- **Storage/quota design respects ADR-004 reality.** `AuthStore` uses `s.pool` directly (`internal/storage/postgres/users.go:361-426`); `RunInTransaction` lives on `*Store` (`internal/storage/postgres/tx.go:35-42`). Explicit `pool.BeginTx` + `FOR UPDATE` on `users` is the right pattern; invalid `count(*) FOR UPDATE` is correctly avoided.

- **No migration needed — schema supports the design.** `api_keys` already has `user_id`, `role_downgrade`, `expires_at`, `revoked_at` and partial index `api_keys_active` (`internal/storage/postgres/auth_migrations/001_initial.sql:36-50`).

- **AUTH-02 is appropriately thin.** Forced re-sync reuses `UpdateUserRole`; propagation is automatic via the live read in `resolveAPIKey`. Plan 06’s `ResyncUserRole` + optional `--revoke-keys` is the right seam without over-building IdP fetch logic.

- **Test map and adversarial cases are first-class.** Source gate, laundering floor, list scoping, quota TOCTOU integration, CSRF, and CLI precedence all have named tests in the validation architecture — appropriate for a credential-minting phase.

---

## Concerns

### HIGH

- **CSRF middleware has no wiring or issuance task.** Plan 03 creates `internal/server/csrf.go` and unit tests but does not modify `cmd/specgraph/serve.go` or `RegisterIdentityService` to mount the middleware. Plan 08 assumes a `specgraph_csrf` cookie exists and the interceptor echoes it, but nothing in Plans 03, 05, or 08 defines *when* the cookie is first issued (e.g. on GET `/keys`, layout load, or a bootstrap endpoint). Without that, human-verify Task 3 in Plan 08 will fail on first mutation.

- **Web session auth path is mis-documented in research/plans.** Research cites `cookieToAuthHeader` (`internal/server/auth_handler.go:133-147`) for web RPC auth, but Connect RPCs authenticate via `authenticate()` → `sessionCookieValue()` reading `specgraph_session` from request headers (`internal/auth/interceptor.go:57-67`, `:70-81`). `cookieToAuthHeader` only wraps `/api/auth/whoami` (`internal/server/auth_handler.go:45-46`). CSRF middleware must wrap the **Connect IdentityService handler path**, not the REST whoami path.

### MEDIUM

- **Plan 02 / Plan 05 storage interface mismatch.** Plan 02 Task 1 declares `CreateAPIKeyForUser(...) (*APIKey, secret string, err error)` and similar for rotate, but Plan 05 generates secrets in the handler (`auth.GenerateAPIKeySecret`) and passes `PHCHash` into storage — matching the existing admin `CreateAPIKey` pattern (`internal/server/identity_handler.go:236-253`). Implementers need one contract; returning plaintext from storage would diverge from established style.

- **`identityError` has no `ErrQuotaExceeded` mapping yet.** Current switch handles `ErrAPIKeyNotFound`, `ErrUserNotFound`, etc. (`internal/server/identity_handler.go:61-75`) but not quota exhaustion. Plan 05 expects `CodeResourceExhausted` for quota/rate-limit; that case must be added explicitly or quota errors surface as `CodeInternal`.

- **AUTH-02 “live floor” lacks integration proof.** Plan 06 tests use fake `UsersBackend` stubs; the validation map lists `TestResync_LiveRoleClamp` as integration (`02-RESEARCH.md` validation table) but Plan 06 only specifies unit tests. The `resolveAPIKey` mechanism is real (`identitystore.go:324-346`), but nothing in the plans proves standing-key effective role changes after `UpdateUserRole` without an integration or e2e test.

- **Connect RPC list is also POST — CSRF scope ambiguous.** Plan 03 limits CSRF to “mutating POST requests to the self-key routes.” In Connect, `ListMyAPIKeys` is also POST. Cross-site key-metadata reads are lower risk than minting (no plaintext in list response per `APIKey` proto comment, `identity.proto:54-56`), but the CSRF middleware design should state whether `ListMyAPIKeys` is included for defense-in-depth.

- **`identity.proto` service comment will become wrong.** Lines 12–15 state all RPCs except `Whoami` require admin; self RPCs break that. Plan 01 does not mention updating the comment — easy to miss and misleading for future readers.

### LOW

- **Admin vs self revoke semantics differ by design but need care.** Admin `RevokeAPIKey` storage has no `RowsAffected` check (`internal/storage/postgres/users.go:408-415`). Self `RevokeAPIKeyForUser` intentionally omits `revoked_at IS NULL` for idempotent re-revoke (Plan 02) while using `RowsAffected()==0 → NotFound` for foreign keys — correct, but implementers must not copy the admin WHERE clause verbatim.

- **Mandatory expiry is a behavior change from admin mint.** Admin `CreateAPIKey` allows unset `expires_at` (`identity.proto:130`, handler `identity_handler.go:248-251`). Self-mint mandating 90d default / 180d max (D-08) is intentional but should be noted in CLI help when self path lands (`auth_apikey.go` today documents optional expiry via `parseExpiresAt` at `:28-39`).

- **Parallel Wave 1 plans (01/02/03) are safe** — no file conflicts — but Plan 04 (Wave 2) only depends on 01, so auth-layer work could start as soon as proto lands; current sequencing is conservative, not wrong.

- **Research “minor drift” on `principalEntity` is confirmed** (`engine.go:212` vs cited `:218`) — negligible.

---

## Suggestions

1. **Add an explicit CSRF integration task** (Plan 03 or 05): mount CSRF validate middleware on the IdentityService Connect handler in `serve.go` (near `server.RegisterIdentityService(mux, res.authStore, opts, maxBytes)` at `cmd/specgraph/serve.go:203`), and issue the `specgraph_csrf` cookie on a safe GET (layout, `/keys`, or middleware that sets cookie when absent).

2. **Correct web auth documentation** to cite `internal/auth/interceptor.go:57-67` as the session-cookie path for Connect, not `cookieToAuthHeader`.

3. **Align `CreateAPIKeyForUser` / `RotateAPIKeyForUser` signatures** with handler-owned secret generation: storage accepts `PHCHash`, returns `*APIKey` only (mirror `CreateAPIKey` at `postgres/users.go:361-404`).

4. **Extend `identityError`** with `errors.Is(err, storage.ErrQuotaExceeded) → connect.CodeResourceExhausted` (and rate-limit similarly) before Plan 05 ships.

5. **Add one integration test** for AUTH-02: write role via `UpdateUserRole`, resolve a standing key through `pgIdentityStore.resolveAPIKey`, assert `EffectiveRole` reflects the new floor — closes the gap between Plan 06 unit tests and SC#3.

6. **Update `identity.proto` service comment** in Plan 01 when adding self RPCs (lines 12–15).

7. **Clarify CSRF coverage** for `ListMyAPIKeys` POST in Plan 03 threat model.

8. **Plan 01 only:** after proto regen, grep confirms no `user_id` inside `CreateMyAPIKeyRequest` — good gate; also verify generated web TS client is imported only from Plan 08 paths to avoid unused-import churn.

---

## Risk Assessment

**Overall risk: MEDIUM**

**Justification:** Security design quality is **high** — the plans correctly identify real vulnerabilities in the current tree (list leak, laundering via `EffectiveRole` vs `Role`, env-key precedence, Cedar boot coupling, quota TOCTOU) and propose mechanisms that match existing patterns (`clampedRole`, `AuthStore` txs, emit-once responses). Implementation scope is **large but bounded** (no new deps, no migration).

Risk is **medium** rather than low because: (1) CSRF is specified as middleware code but not wired or bootstrapped — a common failure mode for cookie-authenticated POST RPCs; (2) web auth assumptions in research point at the wrong middleware layer; (3) AUTH-02’s core value proposition (standing keys clamp immediately) is asserted from code reading but not fully test-gated at integration depth; (4) this phase mints credentials — any gap in owner scoping, floor, or source gate has high blast radius.

**Phase success criteria outlook:**

| Criterion | Plans cover it? | Confidence |
|-----------|-----------------|------------|
| SC#1 — Self-provision without bootstrap key | Yes (RPCs + CLI session resolver + web panel) | Medium (blocked on CSRF wiring + web client) |
| SC#2 — Role capped at mint/rotate, no laundering | Yes (`RoleMin` + storage explicit rotate args + tests) | High |
| SC#3 — Forced re-sync reaches standing keys | Yes (reuse `UpdateUserRole` + live `resolveAPIKey`) | Medium–High (mechanism verified in code; integration test gap) |

**Recommendation:** Proceed with execution after adding CSRF mount/issuance tasks and fixing the web-auth/ storage-signature inconsistencies above. Run `task pr-prep` (integration + e2e) before marking the phase complete — quota TOCTOU and owner-scoped SQL especially need Docker-backed tests per the plans’ own validation architecture.

---

## Consensus Summary

Only one external reviewer (cursor-agent) ran this pass, so there is no cross-reviewer
vote. Cursor performed a source-grounded review against the live tree and its findings
are cited with `file:line` evidence, so they are weighted as high-value.

### Agreed Strengths
_(single reviewer — strengths cursor validated against source)_
- Research/drift grounding is accurate: `resolveAPIKey` live-role clamp (`internal/auth/identitystore.go:306-348`), `UpdateUserRole` writes `users.role`, no new schema/migration needed (`auth_migrations/001_initial.sql:36-50`).
- Security invariants derive from real mechanisms: `ListAPIKeys` empty-`user_id` enumeration leak (`identity_handler.go:319-324`), Cedar `self`-verb boot invariant with the dual hard-coded lists in `actions_test.go:33-41` / `:50-70`, `RoleMin` laundering floor on `EffectiveRole`, `Source=="apikey"` anti-key-chaining gate, `SPECGRAPH_API_KEY` precedence (Finding D, `client.go:102-111`), and the ADR-004 `AuthStore`-tx + `FOR UPDATE` quota pattern.
- Wave ordering (proto → auth → storage/config → handlers → CLI/web) is sound.

### Agreed Concerns (highest priority — actionable via `/gsd-plan-phase 2 --reviews`)
- **[HIGH] CSRF middleware is defined but never wired or bootstrapped.** Plan 03 creates `internal/server/csrf.go` + unit tests but no task mounts the middleware in `cmd/specgraph/serve.go` (near `RegisterIdentityService` at `serve.go:203`), and nothing defines *when* the `specgraph_csrf` cookie is first issued. Plan 08's human-verify mutation will fail on first POST. → add an explicit CSRF mount + issuance task.
- **[HIGH] Web session auth path mis-documented.** Connect RPCs authenticate via `authenticate()` → `sessionCookieValue()` reading `specgraph_session` (`internal/auth/interceptor.go:57-81`), NOT `cookieToAuthHeader` (which only wraps REST `/api/auth/whoami`, `auth_handler.go:45-46`). CSRF must wrap the **Connect IdentityService handler path**. → correct research/plan wording and target the right layer.
- **[MEDIUM] Storage-signature mismatch:** Plan 02 declares `CreateAPIKeyForUser`/`RotateAPIKeyForUser` returning a plaintext secret, but Plan 05 (matching the existing admin `CreateAPIKey`) generates the secret in the handler and passes `PHCHash` to storage. → pick one contract (handler-owned secret, storage returns `*APIKey`).
- **[MEDIUM] `identityError` lacks an `ErrQuotaExceeded` → `CodeResourceExhausted` mapping** (`identity_handler.go:61-75`); Plan 05 expects it or quota errors surface as `CodeInternal`. → add the mapping before Plan 05 ships.
- **[MEDIUM] AUTH-02 live-floor (SC#3) has no integration proof** — Plan 06 uses fake `UsersBackend` stubs; the research validation map lists `TestResync_LiveRoleClamp` as integration but no plan schedules it. → add one integration test (write role via `UpdateUserRole`, resolve a standing key, assert new `EffectiveRole`).
- **[MEDIUM] CSRF scope for `ListMyAPIKeys`** (also POST in Connect) is unspecified. → state inclusion/exclusion in Plan 03 threat model.
- **[LOW]** Update the `identity.proto` service comment (`:12-15`) that claims all non-`Whoami` RPCs require admin; note the self-mint mandatory-expiry behavior change vs admin mint; don't copy the admin `RevokeAPIKey` WHERE clause verbatim into the owner-scoped variant.

### Divergent Views
None — single reviewer.

### Overall
Cursor's verdict: **MEDIUM risk. Proceed after adding the CSRF mount/issuance task and fixing the web-auth/storage-signature inconsistencies.** SC#2 (no laundering) is HIGH confidence; SC#1 and SC#3 are Medium pending the CSRF wiring and an AUTH-02 integration test. Run `task pr-prep` (Docker integration + e2e) before marking the phase complete.
