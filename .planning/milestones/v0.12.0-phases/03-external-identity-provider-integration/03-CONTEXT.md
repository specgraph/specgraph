# Phase 3: External Identity Provider Integration - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Three identity requirements extending the already-shipped OIDC/Authn/Cedar/login-sync
foundation, so SpecGraph authenticates users and MCP clients against real external IdPs
with enough audit metadata for session audit and future RP-initiated logout:

- **AUTH-01 (`spgr-1rq9`):** a native, generic OAuth2 + userinfo login provider (GitHub as
  the driving use case), so a user can log in via GitHub-direct — no Entra/Okta broker — using
  the same session model as existing OIDC providers.
- **AUTH-04 (`spgr-tmqm`):** make SpecGraph's MCP server a spec-correct OAuth 2.1 **resource
  server**, validating access tokens issued by the configured external IdP (delegated auth)
  rather than only SpecGraph-issued API keys.
- **AUTH-05 (`spgr-bbp2`):** populate `web_sessions.issuer` at session creation so operators
  can audit login-provider usage per session and a future RP-initiated logout can target the
  correct issuer.

This is HOW-to-implement clarification on fixed roadmap scope. **No dedicated design docs
exist for any of the three requirements** (`spgr-1rq9`/`spgr-tmqm`/`spgr-bbp2` are named as
forward-looking follow-ons across the identity docs but never designed) — so the decisions
below are net-new design, like AUTH-02 was in Phase 2.

**Key scouting findings (important for downstream):**
- **AUTH-01 is net-new provider work.** The current `LoginProvider` abstraction
  (`internal/auth/loginprovider.go`) is OIDC-only: `BuildLoginProviders` hard-rejects any
  `kind != "oidc"` and requires `.well-known` discovery + id_token verification. GitHub is
  **not** OIDC (plain OAuth2 Authorization Code + a `/user` userinfo endpoint, no id_token),
  so a second, non-OIDC provider path is required.
- **AUTH-04's token validation largely exists.** `Resolve` already routes bearer JWTs to
  `resolveJWT` (issuer-peek → per-issuer verifier routing → JWT validation → JIT). AUTH-04 is
  mostly the OAuth 2.1 **protocol surface** (RFC 9728 metadata + `WWW-Authenticate` challenge)
  plus reconciling audience validation to the MCP server's canonical resource URI.
- **AUTH-05 is population-only.** The `web_sessions.issuer` column **already exists** and
  `CreateSession` already writes `sess.Issuer` — but the login callback creates sessions
  without setting it. No schema change; thread the issuer through at mint time.

</domain>

<decisions>
## Implementation Decisions

### AUTH-01 — Native generic OAuth2 + userinfo login provider (spgr-1rq9)

- **D-01 (provider shape = generic `oauth2` kind):** Add a new provider `kind` (e.g.
  `"oauth2"`) alongside `"oidc"`, driven by config fields for the authorization URL, token URL,
  userinfo URL, and subject/email field selectors. **GitHub ships as documented config, not a
  hardcoded preset** — this matches the roadmap wording "native generic OAuth2 + userinfo login
  provider (GitHub-direct)" and keeps a second non-OIDC IdP cheap later. `BuildLoginProviders`
  must stop hard-rejecting non-`oidc` kinds and construct an `oauth2` `LoginProvider`
  implementation that skips OIDC discovery/id_token verification and instead exchanges the code
  for an access token, then calls the userinfo endpoint.
- **D-02 (subject = stable provider id; email = verified primary):** The binding `subject` is
  the provider's **stable numeric/opaque user id** (GitHub `/user` `id`), configured via a
  subject field selector — survives username/login renames. Email is read from a configurable
  userinfo field; when the primary userinfo response omits it (GitHub private email), fetch the
  **primary verified** email via a secondary endpoint (`/user/emails`, requiring the
  `user:email` scope). Only a verified email is trusted; blank when unavailable.
- **D-03 (multi-provider linking = design-for now, link later):** The intent is that GitHub +
  Google + Okta + EntraID can all resolve to **one** user. The `oidc_bindings` schema already
  supports this — `UNIQUE (issuer, subject)` with an FK to `users(id)` means one `user_id` can
  own many bindings. **This phase ensures the `oauth2` provider stores bindings the same
  `(issuer, subject) → user_id` way, so N-providers-per-user is fully possible at the data
  layer, and keeps the current JIT behavior (binding miss = new user).** Actual cross-provider
  *linking* — auto-link on verified email, or an explicit "link account" flow — is a deliberate
  **fast-follow phase**, not built here. This mirrors the AUTH-02 "ship the path, leave the
  seam ready" pattern. **Do NOT introduce a 1:1 user↔subject assumption anywhere that would
  preclude later linking.**
- **D-04 (roles = userinfo-as-claims + default fallback):** Feed the userinfo JSON response
  into the **same per-provider `claims_mapping`** mechanism OIDC uses, so operators can map
  userinfo fields (e.g. `login`, or org/team membership if later fetched) to roles exactly like
  OIDC claims. When no mapping matches, fall back to the configured JIT default role. One role
  model for all provider kinds. Org/team-membership fetching (extra API calls + `read:org`
  scope) is explicitly **not** in this phase — a later enhancement.

### AUTH-04 — MCP OAuth 2.1 resource server (spgr-tmqm)

Grounded in the **current MCP Authorization spec (revision 2025-06-18)** — see canonical refs.
The MCP server acts purely as an **OAuth 2.1 resource server**; the external IdP is the
authorization server (auth endpoints, token issuance, PKCE, DCR all delegated to it).

- **D-05 (implement the RS-side MUSTs):**
  1. **RFC 9728 Protected Resource Metadata** — serve `/.well-known/oauth-protected-resource`
     with an `authorization_servers` field and this server's canonical resource URI.
  2. **`WWW-Authenticate` on 401** — emit the `Bearer resource_metadata=...` challenge on
     unauthenticated MCP requests (RFC 9728 §5.1) so standard MCP clients auto-discover the AS.
  3. **Audience/resource-bound validation (RFC 8707)** — tokens MUST be validated as issued
     specifically for this MCP server (audience = the server's canonical resource URI), not
     merely against the OIDC verifier's `client_id`. This is the one real gap vs today's
     `resolveJWT` (which checks audience against `cfg.Audience || cfg.ClientID`) and the core
     new validation work. No token passthrough; reject tokens not audience-bound to this RS.
  4. Error codes: **401** invalid/absent token, **403** insufficient scope, **400** malformed.
- **D-06 (also support opaque tokens via RFC 7662 introspection):** In addition to local JWT
  validation, add a **token introspection** path so IdPs that issue **opaque (non-JWT)** access
  tokens also work. Per-request introspection call + config, gated to the introspection-capable
  providers.
- **D-07 (advertise ALL configured IdPs):** The protected-resource metadata lists **every**
  configured interactive/OAuth IdP issuer in `authorization_servers`; the MCP client selects
  one per RFC 9728 §7.6 (mirrors how web login already offers a provider list). No new
  server-side selection logic, no single-designated-AS config field.
- **D-08 (static credentials remain fully additive):** Existing `spgr_sk_` API keys and
  `spgr_ws_` sessions **MUST keep working with zero behavior change** whether or not OAuth is
  enabled. The existing prefix dispatcher (`Resolve`) keeps routing `spgr_sk_` → API key,
  `spgr_ws_` → session, bearer JWT → audience-checked `resolveJWT`, and other opaque bearer
  tokens → introspection. All credential types are accepted concurrently. The
  `WWW-Authenticate` 401 challenge is emitted for OAuth-client discovery **without** breaking
  key-based clients.

### AUTH-05 — Populate `web_sessions.issuer` (spgr-bbp2)

- **D-09 (issuer value = verified `iss` for OIDC, provider-id for oauth2):** For OIDC providers,
  store the **verified `iss` claim** (canonical issuer URL — supports future RP-logout discovery
  via that issuer's `end_session_endpoint`). For `oauth2`/GitHub (no `iss`), store a **stable
  synthetic issuer derived from the provider config** — the configured provider ID (or its
  issuer/base URL). One `issuer` column, meaningful for every provider type; future RP-logout
  can branch on OIDC-vs-oauth2. Thread this value from the login flow / verified identity into
  the existing `CreateSession(sess.Issuer = ...)` at mint time (callback currently leaves it
  empty).
- **D-10 (no backfill):** Do **not** backfill existing empty-issuer sessions. They age out
  naturally within the 12h default session TTL; all new sessions carry the issuer. The audit
  gap is bounded to ≤1 session lifetime and there is zero data-migration risk.

### the agent's Discretion (planner/researcher)
- Exact config field names/shape for the `oauth2` provider kind (URL fields, subject/email
  selectors) and how the userinfo field-selector is expressed (D-01/D-02/D-04).
- Precise `LoginProvider` interface accommodation for the non-OIDC path (the current interface
  returns a raw id_token for `Exchange`; the oauth2 path returns userinfo instead — reconcile
  the seam without breaking the OIDC provider).
- The exact audience/resource-URI reconciliation in `resolveJWT` and how the canonical MCP
  resource URI is derived/configured (D-05.3).
- Introspection endpoint config + caching/rate-limiting for the opaque-token path (D-06).
- Rate-limiting and audit-log line shape for the new login and RS paths — follow existing
  per-IP/per-issuer limiter and slog conventions.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### AUTH-04 — MCP OAuth 2.1 resource server (external specs)
- `https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization` — **the
  governing MCP Authorization spec (rev 2025-06-18).** Defines the RS role, RFC 9728 metadata +
  `WWW-Authenticate` requirement, mandatory audience/resource validation (RFC 8707), no token
  passthrough, and the 401/403/400 error model. MUST read before planning AUTH-04.
- RFC 9728 (OAuth 2.0 Protected Resource Metadata), RFC 8707 (Resource Indicators), RFC 7662
  (Token Introspection), RFC 8414 (AS Metadata), draft-ietf-oauth-v2-1 — the underlying specs
  the MCP doc composes.

### Identity foundation this phase extends (project docs)
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-design.md` — app-roles + `claims_mapping` +
  `sync_on_login` behavior. Basis for D-04 (userinfo-as-claims role mapping). MUST read for
  AUTH-01 role handling.
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-implementation-plan.md` — companion impl plan.
- `docs/superpowers/specs/2026-06-12-oidc-interactive-ui-login-design.md` — the interactive web
  login flow (start/callback, tx cookie, session mint) the GitHub provider plugs into.
- `docs/plans/2026-06-15-cli-oidc-login-design.md` — the `specgraph login` CLI broker (loopback
  redirect + `/api/auth/cli/exchange`); relevant if GitHub login is exposed to the CLI.
- `.planning/intel/decisions.md`, `.planning/intel/constraints.md` — locked ADRs + Identity/RBAC
  lineage. Note ADR-004 caveat: `AuthStore` is NOT reachable by `*Store.RunInTransaction`.

### Key existing code (grounding)
- `internal/auth/loginprovider.go` — `LoginProvider` interface (`:24`), `oidcLoginProvider`
  (`:32`), `BuildLoginProviders` (`:107`, the `kind != "oidc"` reject at `:117` is the AUTH-01
  extension point), `RedirectURI` (`:172`), `defaultScopes` (`:98`).
- `internal/auth/oidc_verifier.go` — `OIDCVerifier`, `OIDCClaims`, `Verify`/`VerifyWithNonce`,
  `Endpoint()`, `Issuer()` — the OIDC-only validation the oauth2 path bypasses.
- `internal/auth/identitystore.go` — `Resolve` prefix dispatcher (`:167`), `resolveJWT`
  (`:418`, issuer-peek/verifier-routing/binding-lookup/JIT), `resolveSession` (`:354`),
  `resolveAPIKey` (`:306`), `jitResolve` (`:487`, binding-miss = new user — the D-03 seam),
  `LookupOIDCBinding` call (`:443`), per-issuer verifiers/`jitClaimsMapping` (`:40`,`:45`).
- `internal/server/auth_oidc_handler.go` — `handleStart`/`handleCallback` (login flow, session
  mint at `:258`-`:266` where `Issuer` must be threaded for AUTH-05), `OIDCLoginConfig`.
- `internal/storage/postgres/web_auth.go` — `CreateSession` (`:21`, already inserts `issuer`;
  AUTH-05 just needs the callers to set `sess.Issuer`), `LookupSessionByHash`.
- `internal/storage/postgres/auth_migrations/001_initial.sql` — `oidc_bindings` DDL (`:26`,
  `UNIQUE (issuer, subject)`, FK `user_id` → `users(id)`; the N-bindings-per-user model for
  D-03), `web_sessions` schema (issuer column already present).
- `internal/config/global.go` — `OIDCConfig` (`:206`), `OIDCProviderConfig` (kind/interactive/
  scopes/issuer/audience/client_secret_env) — where the `oauth2` kind + userinfo/introspection
  config fields land.
- MCP server wiring (interceptor / ConnectRPC handlers under `internal/server/`) — where the
  RFC 9728 metadata endpoint + `WWW-Authenticate` 401 challenge mount.
- `web/src/lib/oidc.svelte.ts`, `web/src/lib/components/LoginModal.svelte` — provider list UI
  the GitHub provider surfaces in (no id_token-specific assumptions).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Interactive login flow** (`auth_oidc_handler.go`): `handleStart`/`handleCallback`, tx
  cookie, PKCE (`oauth2.GenerateVerifier`/`S256ChallengeFromVerifier`), state/nonce, session
  mint — the GitHub `oauth2` provider reuses this machinery; only `Exchange`/identity
  materialization differs (userinfo instead of id_token verification).
- **Prefix dispatcher** (`Resolve`, `identitystore.go:167`): already multiplexes API key /
  session / JWT by token prefix — the natural place to add the opaque-token introspection branch
  while keeping static keys fully additive (D-08).
- **`resolveJWT` bearer path**: existing external-IdP JWT validation; AUTH-04 extends it with
  audience/resource-URI binding rather than rebuilding validation.
- **Per-provider `claims_mapping`** (`jitClaimsMapping`, keyed by issuer): reused to map
  userinfo fields → roles for GitHub (D-04).
- **`oidc_bindings` many-to-one schema**: already supports N-providers-per-user (D-03) — no
  schema change needed for the linking seam.

### Established Patterns
- **"Ship the path, leave the seam ready"** (AUTH-02 D-01): applied to multi-provider linking
  (D-03) — build the data model that permits linking; defer the linking flow itself.
- **Fail-closed identity resolution**: not-found/revoked/expired → `ErrUnauthenticated`; other
  backend errors → `ErrTransient`. New paths must preserve this discipline.
- **Startup-fatal provider construction**: `BuildLoginProviders` treats any provider error as a
  boot abort — the `oauth2` kind follows suit (missing userinfo URL, bad config = fatal).
- **ADR-004**: multi-query writes use `RunInTransaction`; `AuthStore` uses its own explicit tx
  path (not reachable by `*Store.RunInTransaction`).

### Integration Points
- New `oauth2` `LoginProvider` impl in `internal/auth/loginprovider.go` + config in
  `internal/config/global.go`.
- AUTH-05: set `sess.Issuer` in `handleCallback` before `CreateSession`
  (`auth_oidc_handler.go:258`).
- AUTH-04: RFC 9728 metadata endpoint + `WWW-Authenticate` 401 in the MCP/HTTP mux; audience
  reconciliation in `resolveJWT`; introspection branch in `Resolve`.
- Web/CLI: GitHub provider appears in the existing provider list (`/api/auth/oidc/providers`,
  LoginModal); CLI login broker if GitHub is exposed to `specgraph login`.

</code_context>

<specifics>
## Specific Ideas

- User's north-star for AUTH-01: multiple auth providers (GitHub + Google + Okta + EntraID) all
  mapping to the **same user**. Captured as D-03 — the data model must not preclude it, even
  though the linking flow itself is deferred.
- User explicitly wanted AUTH-04 grounded in **what MCP servers should do *today* per current
  specs/RFCs** — hence the 2025-06-18 MCP Authorization spec drives D-05/D-06/D-07.
- User: static (API-key) tokens must keep working even when OIDC/OAuth is enabled (D-08).

</specifics>

<deferred>
## Deferred Ideas

- **Cross-provider account linking flow** — the actual mechanism to attach a second provider to
  an existing user (verified-email auto-link, or explicit dashboard "link account" UX). The data
  model ships this phase (D-03); the linking flow is a fast-follow phase. Security-sensitive
  (account-takeover risk on unverified email) — design carefully when picked up.
- **GitHub org/team-membership → role mapping** — fetching org/team membership (extra API calls +
  `read:org` scope) to drive richer role assignment. This phase does userinfo-field mapping +
  default fallback only (D-04).
- **RP-initiated logout** — AUTH-05 stores the issuer *so that* a future RP-logout can target
  the correct `end_session_endpoint`; the logout flow itself is out of scope here.
- **SpecGraph as its own authorization server / DCR proxy** — explicitly rejected; contradicts
  "delegating auth to a real IdP." SpecGraph stays a pure resource server.
- **Deprecating static API keys** — considered (mark-as-legacy option); rejected in favor of
  fully additive, no-signal coexistence (D-08).

</deferred>

---

*Phase: 3-External Identity Provider Integration*
*Context gathered: 2026-07-10*
