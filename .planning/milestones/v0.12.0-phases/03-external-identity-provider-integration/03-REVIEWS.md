---
phase: 3
reviewers: [cursor]
reviewed_at: 2026-07-10T04:35:53Z
plans_reviewed: [03-01-PLAN.md, 03-02-PLAN.md, 03-03-PLAN.md, 03-04-PLAN.md]
---

# Cross-AI Plan Review — Phase 3

## Cursor Review

# Phase 3 External Identity Provider Integration — Cross-AI Plan Review

**Review method:** Claims verified against live source in `internal/auth/`, `internal/server/`, `internal/config/global.go`, `internal/storage/postgres/`, and `cmd/specgraph/serve.go`. Line citations below reference the current working tree.

---

## Overall Phase Assessment

### Summary

The four-plan wave structure is sound: Plan 01 establishes the identity-materialization seam and AUTH-05; Plans 02–03 parallelize AUTH-01 and AUTH-04 protocol surface; Plan 04 completes token validation. Research grounding is largely accurate — the OIDC-only login path, empty session issuer, bare MCP 401, and existing JWT resolution machinery are all confirmed in code. The biggest risks are **issuer identity consistency** across oauth2 bindings, claims_mapping, and RFC 9728 metadata; **global MCP audience enforcement** in `resolveJWT` affecting non-MCP JWT callers; **Resolve dispatch ordering** for introspection vs `spgr_sk_`; and **https-only resource URI validation** conflicting with the existing `http://` loopback base URL.

### Strengths

- **Correct diagnosis of the interactive-login seam.** `LoginProvider.Exchange` returns a raw `id_token` string (`internal/auth/loginprovider.go:28-29`, `:55-68`), `handleCallback` feeds it to `resolver.Resolve` (`internal/server/auth_oidc_handler.go:208-215`), and `Resolve` routes JWT-shaped tokens to `resolveJWT` (`internal/auth/identitystore.go:171-172`). `Exchange` already calls `VerifyWithNonce` internally (`loginprovider.go:65-67`) then re-verifies in `resolveJWT` (`identitystore.go:427-428`) — redundant work Plan 01 correctly eliminates via `ResolveLogin`.
- **AUTH-05 is truly population-only.** `web_sessions.issuer` exists (`internal/storage/postgres/auth_migrations/002_web_auth.sql:10`), `CreateSession` inserts it (`internal/storage/postgres/web_auth.go:31-38`), but the web callback omits `Issuer` on the session literal (`internal/server/auth_oidc_handler.go:258-260`). `ExchangeCLICode` also deliberately leaves issuer empty (`web_auth.go:206-230`).
- **D-08 additivity is structurally protected.** `Resolver.Resolve(ctx, token string)` is the sole credential dispatcher (`internal/auth/resolver.go:18`, `identitystore.go:167-178`). `spgr_sk_` is excluded from JWT routing via `isJWTShaped` (`identitystore.go:192-195`). Plan 01's `ResolveLogin` side-entry preserves this contract.
- **Reuse targets are real.** `OIDCClaims` already carries `Issuer`, `Subject`, `Email`, `Raw` (`internal/auth/oidc_verifier.go:23-30`). `applyClaimsMapping` operates on `map[string]json.RawMessage` (`identitystore.go:581-592`). `oidc_bindings` has `UNIQUE (issuer, subject)` (`auth_migrations/001_initial.sql:26-34`).
- **MCP protocol gap is accurately scoped.** `RequireAuth` writes a bare 401 with no `WWW-Authenticate` (`internal/auth/middleware.go:20-28`). `/mcp/` is wrapped with `auth.RequireAuth(resolver)` (`cmd/specgraph/serve.go:246-248`).
- **Threat models and test naming are concrete** — each plan ties acceptance to `-run` filters and existing integration harness (`internal/server/identity_integration_test.go` exists for extension).

### Concerns (cross-cutting)

| Severity | Issue | Evidence |
|----------|-------|----------|
| **HIGH** | **oauth2 synthetic issuer vs `claims_mapping` / metadata issuer mismatch** | Runtime mapping lookup uses `claims.Issuer` (`identitystore.go:536`, `loginsync.go:81`). Startup mapping is keyed by `pc.Issuer` only (`cmd/specgraph/serve.go:773-780`). Plan 03-02 derives a *synthetic* issuer from provider ID/base URL (D-09), but neither Plan 02 nor Plan 03 updates `buildClaimsMappingByIssuer` or requires `issuer:` in oauth2 config to equal the synthetic value. GitHub `claims_mapping` and `authorization_servers` can silently fail. |
| **HIGH** | **MCP audience check in `resolveJWT` is path-unscoped** | Plan 03-04 adds `mcpResourceURI` audience assertion inside `resolveJWT` when set. `resolveJWT` is invoked for *all* JWT-shaped tokens via `Resolve` (`identitystore.go:171-172`), which is shared by ConnectRPC interceptor (`internal/auth/interceptor.go:59-67`) and `/mcp/` (`serve.go:246`). Once MCP RS is configured, any JWT bearer on ConnectRPC would also require resource-URI `aud` — not just MCP. Plan text says "MCP-path" but mechanism is global. |
| **HIGH** | **Introspection dispatch must explicitly exclude `spgr_sk_`** | Current `Resolve` has no `spgr_sk_` prefix branch — API keys reach `resolveAPIKey` only as the final fallthrough (`identitystore.go:177`). Plan 03-04 inserts introspection before that fallthrough. Behavior text says `spgr_sk_` never introspects, but the action must gate on `!strings.HasPrefix(token, apiKeyPrefix)`; omission would send API keys to the IdP when introspection is configured. |
| **MEDIUM** | **Plan 03-03 vs 03-04 URI hoist ordering** | Plan 03-03 computes canonical URI "before the `/mcp/` mount" (~`serve.go:246`). Plan 03-04 requires hoisting above `NewIdentityStore` (~`serve.go:162`). Executing 03-03 first may compute the URI twice or leave 03-04 to refactor; not fatal but creates merge friction. |
| **MEDIUM** | **`ValidateMCPResourceURI` https requirement vs dev `http://` base** | Plan requires https + no fragment at startup. `selfBaseURL` returns `http://127.0.0.1:…` (`serve.go:643-653`). Default `<base_url>/mcp` without `auth.oidc.base_url` set to https will fail validation, blocking local MCP OAuth RS unless operators set an explicit https `mcp_resource_uri`. |
| **MEDIUM** | **Success Criterion #3 vs CLI issuer gap** | Plan 03-01 documents CLI sessions leaving issuer empty (`web_auth.go:228-230`). Phase success criterion says "every web session record stores issuer." Accepted deferral is reasonable but should be explicit in phase verification checklist. |
| **LOW** | **Double nonce storage for oauth2** | `handleStart` always generates and stores a nonce (`auth_oidc_handler.go:141-151`) even though Plan 03-02's oauth2 `AuthCodeURL` omits `oidc.Nonce`. Harmless but wastes a DB column value; no security issue. |
| **LOW** | **`OIDCProviderConfig.Kind` already reserved** | `global.go:288` documents `kind` reserved for `"oauth2"` — Plan 01 config surface aligns with existing schema comment. |

### Suggestions (cross-cutting)

- Add a single **`ProviderIssuer(pc)` helper** used by: oauth2 `OIDCClaims.Issuer`, `buildClaimsMappingByIssuer`, RFC 9728 `authorization_servers`, and session audit — eliminating the synthetic-vs-config issuer split.
- Scope MCP audience assertion via **request context** (e.g. only when token arrives on `/mcp/`) or a dedicated `ResolveMCP` path, rather than conditioning all `resolveJWT` calls on `mcpResourceURI != ""`.
- In Plan 03-04 Task 2, make the `spgr_sk_` prefix exclusion **explicit in code and tests** before the introspection branch, not only in prose.
- Hoist canonical resource URI computation to **one site above `NewIdentityStore`** in Plan 03-03 Task 2 (not only in 03-04), so metadata and audience config share the same variable from the first mount.
- Document **dev/prod URI policy**: either allow `http://` resource URIs on loopback, or require explicit `mcp_resource_uri` / `base_url` for local MCP OAuth testing.

### Risk Assessment

**MEDIUM-HIGH.** Architecture and seam selection are well-grounded in verified code. Execution risk concentrates in issuer consistency (oauth2), global vs path-scoped audience enforcement, and introspection dispatch ordering — any of which could cause silent role-mapping failure, lock out API keys, or break non-MCP JWT callers once MCP RS is enabled.

---

## Plan 03-01 — Identity Materialization Seam + AUTH-05

### Summary

Well-designed foundation plan. The `Exchange → *OIDCClaims` + `materializeIdentity` + `ResolveLogin` seam directly addresses verified pain points: redundant id_token verification, missing `Identity.Issuer`, and empty `web_sessions.issuer` on browser login.

### Strengths

- **Verified current-state claims:**
  - No `Identity.Issuer` (`internal/auth/auth.go:19-27`)
  - No `ResolveLogin` on `Resolver` (`internal/auth/resolver.go:10-23`)
  - Callback uses `Resolve`, not claims path (`auth_oidc_handler.go:214-215`)
  - Session mint omits `Issuer` (`auth_oidc_handler.go:258-260` vs `web_auth.go:31-32`)
- **Correct extraction boundaries.** Post-verify tail starts at binding lookup (`identitystore.go:443-484`), including login-sync gate (`identitystore.go:469-474`) and JIT path (`identitystore.go:448-452`).
- **`InteractiveLoginFromContext` is the sole interactive gate** (`internal/auth/context.go:73-91`) — plan's single derivation point matches security comment at `context.go:79-82`.
- **ADR-004 respected** — no new multi-query auth writes; extraction is refactor-only.

### Concerns

| Severity | Issue |
|----------|-------|
| **MEDIUM** | `jitResolve` currently reads `InteractiveLoginFromContext(ctx)` directly (`identitystore.go:521`). Plan says "thread `interactive` into jitResolve" but leaves implementation ambiguous (param vs context). Either works; inconsistency could cause rate-limit bypass bugs if both paths diverge. |
| **LOW** | `fakeResolver` in `interceptor_test.go:22-31` lacks `ResolveLogin` today — plan correctly requires adding it. Also check `middleware_test.go` if any type implements `Resolver` directly. |
| **LOW** | Integration test `SessionIssuer` is net-new (no existing match in `identity_integration_test.go`) — plan appropriately scopes it `//go:build integration`. |

### Suggestions

- In Task 2, specify that `jitResolve` takes an explicit `interactive bool` parameter and **remove** the context read inside `jitResolve` to avoid dual sources of truth.
- Add a unit test asserting `oidcLoginProvider.Exchange` returns the same claims as `VerifyWithNonce` (no second network/JWKS round-trip through `Resolve`).

### Risk Assessment

**LOW-MEDIUM.** Refactor of well-tested code with clear before/after boundaries. Main risk is subtle behavior drift in login-sync or JIT gating during extraction.

---

## Plan 03-02 — AUTH-01 Native OAuth2 + Userinfo Provider

### Summary

Correctly identifies AUTH-01 as net-new provider work. `BuildLoginProviders` hard-rejects non-`oidc` kinds today (`loginprovider.go:117-118`). GitHub cannot satisfy the id_token contract (`loginprovider.go:61-63`). Plan 02 appropriately depends on Plan 01's `*OIDCClaims` return type.

### Strengths

- **Verified blockers:** `kind != "oidc"` fatal at `loginprovider.go:117-118`; `Exchange` contract requires id_token (`loginprovider.go:28-29`).
- **PKCE machinery already exists** in `handleStart` (`auth_oidc_handler.go:146-147`, `:164`) — oauth2 provider can reuse without `oidc.Nonce`.
- **`resolveClientSecret` pattern** is established (`loginprovider.go:83-95`) for outbound credential handling.
- **D-02/D-03 alignment** with `oidc_bindings UNIQUE (issuer, subject)` (`001_initial.sql:33`) — no schema change needed.

### Concerns

| Severity | Issue |
|----------|-------|
| **HIGH** | **`claims_mapping` issuer key mismatch** (see cross-cutting). `buildClaimsMappingByIssuer` keys by `pc.Issuer` (`serve.go:776-777`), not synthetic issuerID. Plan 03-02 Task 2 does not update this helper or mandate `issuer:` config for oauth2 providers. D-04 "zero new mapping code" holds only if issuer strings align. |
| **MEDIUM** | **RFC 9728 `authorization_servers` will advertise `pc.Issuer`** (Plan 03-03), while bindings use synthetic issuer (Plan 03-02 D-09). MCP clients may discover an AS URL that doesn't match binding issuer or mapping keys. |
| **MEDIUM** | **GitHub scopes not specified in plan tasks.** `user:email` scope required for `/user/emails` fallback (D-02) must appear in operator config/docs; plan's `user_setup` mentions it but Task 2 doesn't validate scopes for oauth2 providers. |
| **LOW** | **Numeric subject stringification** — GitHub `id` is JSON number; plan says stringify. Must ensure `LookupOIDCBinding` receives consistent string form on repeat logins. |

### Suggestions

- Add Task 2 sub-step: **update `buildClaimsMappingByIssuer`** to key oauth2 providers by the same `issuerID` helper used in `oauth2LoginProvider`, OR require `issuer:` in oauth2 YAML and validate it equals `issuerID` at startup.
- Add oauth2 startup validation: if `EmailsURL` is set, require `user:email` (or equivalent) in `scopes`.
- Document example GitHub provider config showing `issuer: "github"` (or chosen synthetic) matching `claims_mapping` keys.

### Risk Assessment

**MEDIUM.** Provider implementation is straightforward; issuer consistency across binding, mapping, metadata, and session audit is the main failure mode.

---

## Plan 03-03 — AUTH-04 Protocol Surface (RFC 9728 + WWW-Authenticate)

### Summary

Accurately targets the verified gap: bare 401 on `/mcp/` (`middleware.go:27`) with no discovery metadata. Scoping challenge to `/mcp/` only preserves D-08 for ConnectRPC (`interceptor.go:114-117`) and `/api/*` (`serve.go:214-215`).

### Strengths

- **Verified mounting point:** `/mcp/` at `serve.go:246-248` uses `auth.RequireAuth(resolver)`.
- **Public endpoint pattern exists:** `handleProviders` at `auth_oidc_handler.go:90-101` is unauthenticated — good template for metadata handler.
- **Token passthrough analysis is correct:** loopback `mcpClient` targets `selfBaseURL(cfg.Server.Listen)` (`serve.go:229`, `:643-653`) — internal ConnectRPC hop, not upstream passthrough.
- **D-05.4 scope clarification** (401 only, no OAuth scope insufficiency) matches Cedar post-auth model.

### Concerns

| Severity | Issue |
|----------|-------|
| **MEDIUM** | **https validation vs default URI derivation** — default `<base_url>/mcp` from `selfBaseURL` yields `http://` (`serve.go:653`), conflicting with `ValidateMCPResourceURI` https requirement. Local dev may need explicit config. |
| **MEDIUM** | **URI computation site** — Plan computes URI before `/mcp/` mount (~`:246`) but Plan 03-04 needs it above `NewIdentityStore` (~`:162`). Execute hoist in 03-03 to avoid 03-04 rework. |
| **MEDIUM** | **`authorization_servers` for oauth2** — Plan says collect `pc.Issuer` for interactive providers (`03-03` Task 2). For oauth2, `Issuer` may be empty/unused while synthetic issuer differs (Plan 02). Metadata could list wrong AS identifiers. |
| **LOW** | **`RequireAuthWithChallenge` duplicates `authenticate()` logic** — acceptable for D-08 isolation, but drift risk vs `middleware.go:16-37`. Consider shared internal helper with optional challenge header. |

### Suggestions

- Compute canonical resource URI **once** above `NewIdentityStore` in Task 2; pass to both metadata registration and (later) `IdentityStoreConfig.MCPResourceURI`.
- Use shared `ProviderIssuer(pc)` for `authorization_servers` list, not raw `pc.Issuer`.
- Add startup test: with `BaseURL: "https://specgraph.example"` and empty `MCPResourceURI`, default resolves to `https://specgraph.example/mcp` and passes validation.

### Risk Assessment

**LOW-MEDIUM.** Protocol surface is isolated and testable. Issuer advertisement and https default URI are the main integration pitfalls.

---

## Plan 03-04 — AUTH-04 Token Validation (Audience + Introspection)

### Summary

Correctly identifies existing JWT validation (`resolveJWT` at `identitystore.go:416-428`, `aud == client_id` at `oidc_verifier.go:50-54`) and the missing introspection branch. Post-verify audience assertion (OQ2) preserves web-login verifier semantics — sound choice.

### Strengths

- **Verified JWT path:** issuer peek → verifier route → `Verify` → binding/JIT (`identitystore.go:419-452`).
- **Verified audience config:** `NewOIDCVerifier` sets `ClientID: audience` where `audience = cfg.Audience || cfg.ClientID` (`oidc_verifier.go:50-54`).
- **Verified dispatch order baseline:** JWT → session prefix → API key (`identitystore.go:171-177`).
- **`rateLimiterFor` reuse** is appropriate (`identitystore.go:643+`).
- **Introspection gated on config presence** — additive when no introspection URLs configured.

### Concerns

| Severity | Issue |
|----------|-------|
| **HIGH** | **Global MCP audience check** — adding `mcpResourceURI` check inside `resolveJWT` affects all JWT resolution, not only `/mcp/`. ConnectRPC accepts JWT bearer via same `authenticate()` → `Resolve` path (`interceptor.go:59-67`). |
| **HIGH** | **`spgr_sk_` prefix exclusion not explicit in Resolve** — must add `strings.HasPrefix(token, apiKeyPrefix)` guard before introspection; current code relies on fallthrough to `resolveAPIKey` (`identitystore.go:177`). |
| **MEDIUM** | **Introspector selection for multi-IdP** — plan says "pick configured introspector(s)" without issuer routing for opaque tokens (no peek). May need trial-all or config default; undefined behavior if multiple introspection endpoints configured. |
| **MEDIUM** | **JIT on introspection path** — `materializeIdentity(..., interactive=false)` means introspected tokens hit JIT rate limiter (`identitystore.go:521-527`). Probably correct for MCP, but differs from interactive login bypass. |
| **LOW** | **Plan references `identitystore.go:469-484 materializeIdentity call shape`** — `materializeIdentity` doesn't exist yet (Plan 01). Acceptable as forward reference across wave dependency. |

### Suggestions

- Gate audience assertion on **MCP request context** (set in `/mcp/` `WithHTTPContextFunc` at `serve.go:232-244`) rather than global `mcpResourceURI != ""` in `resolveJWT`.
- Add explicit Resolve dispatch pseudocode in Task 2:
  ```
  if isJWTShaped → resolveJWT
  if spgr_ws_ → resolveSession
  if spgr_sk_ → resolveAPIKey   // BEFORE introspection
  if introspectors configured → resolveIntrospection
  else → resolveAPIKey (reject)
  ```
- For multi-IdP introspection, require per-provider config flag or iterate introspectors until `active:true` with matching `aud` (document semantics).

### Risk Assessment

**HIGH.** Audience scoping and introspection dispatch ordering can cause production auth regressions (API keys, ConnectRPC JWT users) if implemented literally without prefix guards and path scoping.

---

## Phase Goal Coverage

| Success Criterion | Plans | Verdict |
|-------------------|-------|---------|
| GitHub OAuth2 login, same session model | 01 + 02 | **Achievable** — blocked only by issuer consistency for bindings/mapping |
| MCP OAuth 2.1 RS with external IdP tokens | 03 + 04 | **Achievable with fixes** — audience scoping and introspection ordering need tightening |
| `web_sessions.issuer` populated | 01 | **Achievable** for browser callback; CLI gap documented |

---

## Recommended Execution Order Adjustments

1. **Plan 01** — execute as written; add explicit `jitResolve(interactive bool)` parameter.
2. **Plan 02** — add `ProviderIssuer` helper + update `buildClaimsMappingByIssuer` in same PR.
3. **Plan 03** — hoist resource URI above `NewIdentityStore`; use `ProviderIssuer` for metadata.
4. **Plan 04** — path-scope audience check; explicit `spgr_sk_` guard before introspection; clarify multi-IdP introspector selection.

**Overall phase risk: MEDIUM-HIGH** — strong architectural planning grounded in verified code, with three HIGH-severity integration gaps (issuer consistency, global audience check, introspection dispatch) that should be resolved in plan text before execution.

---

## Consensus Summary

Only one reviewer was invoked (`--cursor`), so there is no multi-reviewer consensus. Cursor performed a source-grounded review, verifying plan claims against the live working tree with `file:line` citations. Its verdict: **architecture and seam selection are well-grounded and correct; execution risk concentrates in three HIGH-severity integration gaps that should be tightened in plan text before execution.**

### Agreed Strengths
_(single reviewer)_ Cursor confirmed against source:
- The identity-materialization seam correctly eliminates redundant id_token verification (`loginprovider.go:65-67` re-verified in `identitystore.go:427-428`).
- AUTH-05 is truly population-only (`web_sessions.issuer` exists at `002_web_auth.sql:10`; callback omits it at `auth_oidc_handler.go:258-260`).
- D-08 additivity is structurally protected by the single `Resolve` dispatcher + `isJWTShaped` guard.
- MCP protocol gap (bare 401, no `WWW-Authenticate`) is accurately scoped.

### Agreed Concerns (highest priority — single reviewer, HIGH severity)
1. **oauth2 synthetic issuer vs `claims_mapping`/metadata issuer mismatch (HIGH).** Runtime mapping lookup uses `claims.Issuer` (`identitystore.go:536`); startup mapping is keyed by `pc.Issuer` (`serve.go:773-780`). Plan 03-02's synthetic issuer (D-09) is not reconciled with `buildClaimsMappingByIssuer` or the RFC 9728 `authorization_servers` list — GitHub role mapping and AS advertisement can silently break. Suggested: a shared `ProviderIssuer(pc)` helper used by bindings, mapping, metadata, and session audit.
2. **MCP audience check in `resolveJWT` is path-unscoped (HIGH).** `resolveJWT` runs for ALL JWT-shaped tokens via `Resolve` (shared by the ConnectRPC interceptor and `/mcp/`). Conditioning on `mcpResourceURI != ""` would force resource-URI `aud` on non-MCP ConnectRPC JWT callers too. Suggested: scope the assertion to the `/mcp/` request context (`serve.go:232-244`) or a dedicated `ResolveMCP` path.
3. **Introspection dispatch must explicitly exclude `spgr_sk_` (HIGH).** Current `Resolve` reaches `resolveAPIKey` only as the final fallthrough (`identitystore.go:177`). Inserting introspection before it without a `strings.HasPrefix(token, apiKeyPrefix)` guard would send API keys to the external IdP. Make the prefix exclusion explicit in code + tests.

Additional MEDIUM items worth folding in: `ValidateMCPResourceURI` https-only requirement conflicts with the dev `http://` loopback base URL (`serve.go:643-653`) — local MCP OAuth RS needs an explicit `mcp_resource_uri`; and the 03-03→03-04 canonical-URI hoist ordering (already partially addressed in plan text — Cursor independently confirmed it).

### Divergent Views
None — single reviewer.

### Note on already-addressed items
Cursor independently re-flagged the CLI-issuer gap (Success Criterion #3) and the canonical-URI hoist ordering. Both were already dispositioned during planning: the CLI-issuer gap is a user-accepted, bounded deferral (per D-09/D-10), and the hoist instruction was added to 03-04 Task 1. The three HIGH findings above are NEW and not yet reflected in the plans.
