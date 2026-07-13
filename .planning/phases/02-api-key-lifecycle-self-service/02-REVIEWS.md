---
phase: 2
review_round: 2
reviewers: [cursor]
reviewed_at: 2026-07-09T15:50:50Z
plans_reviewed: [02-01-PLAN.md, 02-02-PLAN.md, 02-03-PLAN.md, 02-04-PLAN.md, 02-05-PLAN.md, 02-06-PLAN.md, 02-07-PLAN.md, 02-08-PLAN.md]
supersedes: "round 1 (cursor) — findings incorporated via /gsd-plan-phase 2 --reviews, commit c5ac551b"
---

# Cross-AI Plan Review — Phase 2 (Round 2, post-incorporation convergence check)

> Reviewer: **cursor-agent** CLI (independent session; `--opencode` skipped because the
> orchestrator IS OpenCode). This is a SECOND pass verifying that round-1 findings were
> correctly incorporated by the `--reviews` replan, plus new-issue detection. Single
> external reviewer.

---

## Cursor Review

# Phase 2 Plan Review (Round 2 — Post-Incorporation)

## Summary

Round 1’s actionable findings are **substantively addressed in the eight revised execution plans** (`02-01` through `02-08`) and in `02-VALIDATION.md`. The split between Plan 03 (CSRF issue + unit tests + whoami bootstrap) and Plan 05 Task 3 (actual `csrfValidate` mount on the Connect handler) is sound and matches the real wiring surface at `internal/server/identity_handler.go:29-35`. **No implementation has landed** — `identity.proto` still ends at `UnbindOIDC` (`proto/specgraph/v1/identity.proto:32`), there is no `internal/server/csrf.go`, no `storage.ErrQuotaExceeded`, and no self RPC symbols in `gen/`.

The plans remain well grounded against brownfield mechanisms (`resolveAPIKey` live-role clamp at `internal/auth/identitystore.go:324-346`, `ListAPIKeys` empty-`user_id` leak at `internal/server/identity_handler.go:319-324`, handler-owned secret generation at `identity_handler.go:236-253`). Round 1 is **not fully converged in supporting docs**: `02-RESEARCH.md` and parts of `02-PATTERNS.md` still describe Connect web auth via `cookieToAuthHeader`, which the live tree does not use. **Recommendation: proceed with execution after syncing those docs; overall risk is MEDIUM (down from round 1’s blockers, but credential-minting scope keeps stakes high).**

---

## Round-1 Fix Verification

| Round-1 finding | Status | Evidence |
|-----------------|--------|----------|
| **HIGH — CSRF middleware never wired / token never issued** | **RESOLVED (in plans)** | Plan 03 Task 2 wires issuance onto REST whoami (`internal/server/auth_handler.go:45-46`); dashboard already calls it via `web/src/lib/auth.svelte.ts:17`, gated by `+layout.svelte:12-13` before children render. Plan 05 Task 3 mounts `csrfValidate(h)` in `RegisterIdentityService` (`identity_handler.go:29-35`). Validated by `02-VALIDATION.md` task `2-05-03` (`TestSelfMint_CSRFMount`). No `csrf.go` exists yet — expected pre-implementation. |
| **HIGH — Web session auth path mis-documented (`cookieToAuthHeader` vs Connect interceptor)** | **PARTIAL** | **Plans fixed:** `02-03-PLAN.md` key_links, `02-05-PLAN.md` Task 3, `02-08-PLAN.md` must_haves cite `internal/auth/interceptor.go:57-81` (`authenticate` → `sessionCookieValue` reading `specgraph_session`). **Live source confirms:** Connect uses `sessionCookieValue` (`interceptor.go:70-81`); `cookieToAuthHeader` wraps only REST whoami (`auth_handler.go:45-46`, `:133-147`). **Still stale:** `02-RESEARCH.md:204-222` and `02-PATTERNS.md:343-344` still claim web mutations flow through `cookieToAuthHeader`. |
| **MEDIUM — Storage signature mismatch (plaintext from storage vs handler-owned PHC)** | **RESOLVED (in plans)** | Plan 02 Task 1 explicitly aligns on handler-owned secret generation, matching admin `CreateAPIKey` (`identity_handler.go:236-253`) and `AuthStore.CreateAPIKey` accepting caller `PHCHash` (`internal/storage/postgres/users.go:357-367`). `CreateAPIKeyForUser` returns `(*APIKey, error)` only. |
| **MEDIUM — `identityError` lacks `ErrQuotaExceeded` → `CodeResourceExhausted`** | **RESOLVED (in plans)** | Plan 02 adds sentinel; Plan 05 Task 3 extends `identityError` switch (`identity_handler.go:61-75`). Current switch has no quota case; `internal/storage/errors.go` has no `ErrQuotaExceeded` yet. |
| **MEDIUM — AUTH-02 live-floor lacks integration proof** | **RESOLVED (in plans)** | Plan 06 Task 3 schedules `TestResync_LiveRoleClamp` (`internal/auth/resync_integration_test.go`, `//go:build integration`), modeled on `mint_integration_test.go:25-73`. `02-VALIDATION.md:59-71` no longer defers this to `task pr-prep` alone. Mechanism verified in source: `resolveAPIKey` reloads `users.role` every request (`identitystore.go:324-346`). |
| **MEDIUM — CSRF scope for `ListMyAPIKeys` (also POST) unspecified** | **RESOLVED (in plans)** | Plan 03 Task 2 explicitly includes `/ListMyAPIKeys` in the enforced procedure set (cursor #6). Connect procedure path shape confirmed: `/specgraph.v1.IdentityService/ListAPIKeys` (`gen/specgraph/v1/specgraphv1connect/identity.connect.go:67-69`). |
| **LOW — `identity.proto` service comment drift; mandatory self expiry; revoke WHERE semantics** | **RESOLVED (in plans)** | Plan 01 Task 1 updates service comment (`identity.proto:12-15` still wrong in tree). Plan 07 Task 2 documents mandatory 90d/180d self expiry vs optional admin mint (`auth_apikey.go:28-39`). Plan 02 Task 2 distinguishes owner-scoped revoke from admin `RevokeAPIKey` (`postgres/users.go:406-415`, no `RowsAffected` check). |

---

## Strengths

- **Round-1 feedback is traceable in plan tasks**, not just REVIEWS.md — CSRF mount, quota mapping, storage contract, integration test, and `ListMyAPIKeys` CSRF scope each have acceptance criteria and validation rows (`02-VALIDATION.md:54-59`).
- **Security invariants match live code.** Empty `user_id` on admin `ListAPIKeys` lists all keys (`identity_handler.go:319-324`). Standing keys pick up DB role changes via per-request `GetUserByID` + `clampedRole` (`identitystore.go:324-346`). Cedar boot coupling is real: `knownVerbs` lacks `"self"` (`engine.go:155`) and `TestActionNames_AllParseToKnownVerb` hard-codes four verbs (`actions_test.go:33-41`).
- **CSRF bootstrap path is viable.** Whoami is a safe GET (`auth_handler.go:45-46`); layout awaits `checkAuth()` before rendering (`+layout.svelte:12-26`), so `specgraph_csrf` can exist before `/keys` mutations (Plan 08).
- **Handler-owned secret generation is the established pattern** (`identity_handler.go:236-253`, `:286-300`; storage never returns plaintext).
- **Wave ordering remains sound:** proto → storage/config → auth verbs → handlers → CLI/web/AUTH-02; Plan 05 correctly grows `RegisterIdentityService` and threads config through `cmd/specgraph/serve.go:203` (`buildAppHandler` has `cfg *config.GlobalConfig` at `:141`).
- **`mint_integration_test.go` is a proven integration template** for `TestResync_LiveRoleClamp` (`internal/auth/mint_integration_test.go:25-73`).
- **No schema migration required** — `api_keys` already has `user_id`, `role_downgrade`, `expires_at`, `revoked_at` (`auth_migrations/001_initial.sql:36-50`).

---

## Concerns

### HIGH

*(None new — round-1 HIGH items are addressed in plan text; risk shifts to faithful execution.)*

### MEDIUM

- **Supporting research docs still contradict corrected plans on web auth.** `02-RESEARCH.md:204` and the architecture diagram at `:218-222` still route web mutations through `cookieToAuthHeader`; Connect actually authenticates via `interceptor.go:57-81`. Implementers reading RESEARCH/PATTERNS before PATTERNS’ plan cross-refs could mount CSRF on the wrong mental model.
- **Structured audit logging for self-mint is specified in research but not gated in Plan 05.** `02-RESEARCH.md:150-151` and validation row V7 require audit lines on create/rotate/revoke; Plan 05 threat model mentions untrusted `label` in logs (`T-02-19`) but has no acceptance criterion for structured audit emission. Plan 06 threat model covers resync audit only (`02-06-PLAN.md:T-02-22`).
- **`TestResync_LiveRoleClamp` proves storage+resolver, not the `ResyncUserRole` RPC seam.** Plan 06 Task 3 calls `authStore.UpdateUserRole` directly — correct for isolating SC#3 propagation, but SC#3’s operator path still depends on Plan 06 Task 1 unit tests + manual validation (`02-VALIDATION.md:95`).

### LOW

- **Dashboard users who authenticated via legacy API-key paste cannot self-mint from web.** `handleLogin` stores the raw key in `specgraph_session` (`auth_handler.go:80-81`); resolution yields `Source: "apikey"` (`identitystore.go:347`); Plan 05 correctly rejects that for anti-chaining — but UX is undocumented.
- **`serve.go:203` citation is slightly imprecise.** `maxBytes` is `connect.WithReadMaxBytes` (`serve.go:181`), passed as a variadic `HandlerOption` alongside `opts` — not a separate parameter. Harmless for implementers, but line-number references should note variadic bundling.
- **Plan 03 threat model typo:** artifacts table in `02-01-PLAN.md` once referenced Plan 03 for `ResyncUserRole` → `user.manage`; that mapping belongs in Plan 04 (`02-04-PLAN.md:21`).

---

## Suggestions

1. **Sync `02-RESEARCH.md` and `02-PATTERNS.md` web-auth sections** to cite `internal/auth/interceptor.go:57-81` for Connect/dashboard RPCs; reserve `cookieToAuthHeader` for REST `/api/auth/whoami` only (`auth_handler.go:45-46`).
2. **Add an explicit Plan 05 acceptance criterion** for structured audit log lines on self create/rotate/revoke (actor = self identity, never log plaintext or unescaped `label`), matching `02-RESEARCH.md` V7.
3. **Optional end-to-end complement:** after Plan 06 lands, add one handler-level or CLI test that calls `ResyncUserRole` then resolves a standing key — closes the gap between unit stubs and the operator seam.
4. **Document web self-mint eligibility** in Plan 08 or CLI help: OIDC/`spgr_ws_` session required; legacy API-key dashboard login is intentionally blocked by `Source=="apikey"` gate.
5. **Keep `go build ./...` in Plan 05 verify steps** — `serve.go:203` is not exercised by `go test ./internal/server/` alone (plans already note this; worth enforcing in CI mindset).

---

## Risk Assessment

**Overall risk: MEDIUM**

**Justification:** Round-1 **blockers are resolved in plan text** — CSRF has a credible issue point (whoami GET), mount point (`RegisterIdentityService`), and automated mount test (`2-05-03`); storage contract, quota mapping, and AUTH-02 integration proof are scheduled. Security design quality remains **high** and matches verified mechanisms in the tree.

Risk stays **medium** (not low) because: (1) **no code has landed yet** for a credential-minting phase; (2) **research doc drift** on the Connect auth path could cause implementation mistakes despite corrected plans; (3) **audit logging** is underspecified relative to the canonical design; (4) any bug in owner scoping, `RoleMin` floor, or CSRF mount has high blast radius.

| Success criterion | Plans cover it? | Confidence |
|-------------------|-----------------|------------|
| **SC#1** — Self-provision without bootstrap key | Yes (RPCs + CLI session resolver + web panel + CSRF) | **Medium–High** (was Medium; CSRF wiring now explicit) |
| **SC#2** — Role capped at mint/rotate, no laundering | Yes (`RoleMin`, explicit rotate args, adversarial tests) | **High** |
| **SC#3** — Forced re-sync reaches standing keys | Yes (`ResyncUserRole` + `UpdateUserRole` + live `resolveAPIKey`) | **Medium–High** (integration test scheduled; RPC seam mostly unit-tested) |

**Recommendation:** Proceed with execution. Before marking the phase complete, run `task pr-prep` (Docker integration + e2e), reconcile `02-RESEARCH.md` with the interceptor-based Connect auth path, and treat Plan 05 CSRF mount + `TestResync_LiveRoleClamp` as hard gates alongside the Plan 08 human-verify checkpoint.

---

## Consensus Summary

Single reviewer (cursor-agent), source-grounded (findings cited to `file:line`). This round
verified convergence of the round-1 incorporation.

### Round-1 Convergence Verdict
- **6 of 7 findings RESOLVED in plan text**, verified against live source (CSRF mount at the real `RegisterIdentityService`/`identity_handler.go:29-35` shape; cookie issuance on the whoami GET that `web/src/lib/auth.svelte.ts:17` already calls; storage-signature alignment; `ErrQuotaExceeded → CodeResourceExhausted`; `TestResync_LiveRoleClamp` scheduled; `ListMyAPIKeys` CSRF scope stated; proto/CLI/revoke LOW fixes).
- **1 finding PARTIAL:** the web-auth-path correction landed in the PLANs (03/05/08 cite `interceptor.go:57-81`) but the supporting **`02-RESEARCH.md:204-222` and `02-PATTERNS.md:343-344` still describe Connect web mutations via `cookieToAuthHeader`** — stale mental model for an implementer who reads research before the plan cross-refs.

### Agreed Concerns (actionable for a follow-up `/gsd-plan-phase 2 --reviews` or a doc sync)
- **[MEDIUM] Doc drift:** sync `02-RESEARCH.md` + `02-PATTERNS.md` web-auth sections to `interceptor.go:57-81` (Connect/`specgraph_session`); reserve `cookieToAuthHeader` for REST `/api/auth/whoami` only.
- **[MEDIUM] Audit logging underspecified in Plan 05:** RESEARCH V7 requires structured audit lines on self create/rotate/revoke, but Plan 05 has no acceptance criterion for structured audit emission (only the untrusted-`label` threat row T-02-19). Add an explicit criterion (actor = self identity; never log plaintext/unescaped label).
- **[MEDIUM] `TestResync_LiveRoleClamp` proves storage+resolver, not the `ResyncUserRole` RPC seam** — Plan 06 Task 3 calls `authStore.UpdateUserRole` directly. Optional: add one handler/CLI test that calls `ResyncUserRole` then resolves a standing key to cover the operator path end-to-end.
- **[LOW]** Document web self-mint eligibility (legacy API-key `specgraph_session` login is intentionally blocked by the `Source=="apikey"` gate); note `serve.go:203` `maxBytes` is a variadic `HandlerOption` bundled with `opts`, not a separate param; fix the artifacts-table mapping (`ResyncUserRole → user.manage` belongs in Plan 04, not Plan 03).

### New HIGH concerns
None — round-1 HIGH blockers are resolved in plan text; residual risk is faithful execution.

### Divergent Views
None — single reviewer.

### Overall
Cursor round-2 verdict: **MEDIUM risk (down from round 1). Proceed with execution.** SC#2 HIGH confidence; SC#1 and SC#3 Medium–High (CSRF wiring now explicit; AUTH-02 integration test scheduled). Before phase completion: run `task pr-prep` (Docker integration + e2e), reconcile the RESEARCH/PATTERNS web-auth drift, and treat the Plan 05 CSRF mount + `TestResync_LiveRoleClamp` + Plan 08 human-verify as hard gates.
