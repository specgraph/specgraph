# Phase 3: External Identity Provider Integration - Research

**Researched:** 2026-07-09
**Domain:** OAuth 2.1 / OIDC delegated identity — a native generic OAuth2+userinfo login provider (GitHub), an MCP OAuth 2.1 resource-server protocol surface (RFC 9728 / RFC 8707 / RFC 7662), and session issuer audit population — all extending SpecGraph's existing Go (ConnectRPC, pgx v5) auth subsystem.
**Confidence:** HIGH (all code claims verified against live source this session with file:line; external protocol claims cited to the MCP 2025-06-18 spec and RFC 9728)

## Summary

This phase extends an already-mature identity subsystem. All three requirements plug into existing seams rather than building new subsystems; the research below is almost entirely about *where the seams are* and *how to extend them without breaking the OIDC/API-key/session paths that already ship*. No new third-party packages are required — `go-oidc/v3 v3.17.0`, `golang.org/x/oauth2 v0.36.0`, and `mark3labs/mcp-go v0.45.0` are already in `go.mod` and cover every need here.

**AUTH-01** is the largest slice: it is genuinely net-new provider work because `BuildLoginProviders` hard-rejects any `kind != "oidc"` (`internal/auth/loginprovider.go:117`) and the `LoginProvider.Exchange` contract returns a raw verified `id_token string` (`:29`) which GitHub cannot produce (no id_token, no discovery). The core design problem is reconciling the `Exchange` seam so an `oauth2` provider can materialize identity from a userinfo response while the OIDC provider keeps returning a JWT — and threading a normalized issuer/subject/email/claims assertion into the *same* binding-lookup / JIT / login-sync / claims-mapping machinery that `resolveJWT` already implements (`identitystore.go:418-485`).

**AUTH-04** is mostly protocol surface: token *validation* already exists (`resolveJWT` peeks issuer → routes to per-issuer verifier → validates → binding/JIT). The real gaps are (1) the RFC 9728 `/.well-known/oauth-protected-resource` metadata endpoint + the `WWW-Authenticate` 401 challenge (which `RequireAuth` does not currently emit — `internal/auth/middleware.go:20-29` writes a bare `401` with no challenge header), (2) reconciling audience validation from "audience == `cfg.Audience || cfg.ClientID`" (`oidc_verifier.go:50-54`) to "audience == this MCP server's canonical resource URI" per RFC 8707, and (3) an RFC 7662 introspection branch for opaque tokens in the `Resolve` dispatcher (`identitystore.go:167`). **AUTH-05** is population-only: the `web_sessions.issuer` column and the `CreateSession` INSERT already exist (`002_web_auth.sql:10`, `web_auth.go:31-38`); only the callback (`auth_oidc_handler.go:258-265`) fails to set `sess.Issuer`, and `Identity` has no `Issuer` field to carry it (`auth.go:19-27`) — so surfacing the verified issuer out of resolution is the one structural change.

**Primary recommendation:** Refactor the post-verification tail of `resolveJWT` (binding lookup → JIT → login-sync → Identity) into a shared `materializeIdentity(ctx, *OIDCClaims, interactive bool)` helper; have the OIDC and new `oauth2` login providers both produce `*OIDCClaims` (with a synthetic issuer for oauth2) that feeds it; add an `Issuer` field to `Identity` so AUTH-05 and future RP-logout can read it; and mount an RFC-9728 metadata handler plus a WWW-Authenticate-emitting wrapper on the `/mcp/` route without touching the API-key/session prefix dispatch (D-08).

## User Constraints (from CONTEXT.md)

### Locked Decisions

**AUTH-01 — Native generic OAuth2 + userinfo login provider (spgr-1rq9)**
- **D-01 (provider shape = generic `oauth2` kind):** Add a new provider `kind` (e.g. `"oauth2"`) alongside `"oidc"`, driven by config fields for the authorization URL, token URL, userinfo URL, and subject/email field selectors. **GitHub ships as documented config, not a hardcoded preset.** `BuildLoginProviders` must stop hard-rejecting non-`oidc` kinds and construct an `oauth2` `LoginProvider` that skips OIDC discovery/id_token verification and instead exchanges the code for an access token, then calls the userinfo endpoint.
- **D-02 (subject = stable provider id; email = verified primary):** The binding `subject` is the provider's stable numeric/opaque user id (GitHub `/user` `id`), configured via a subject field selector — survives username/login renames. Email is read from a configurable userinfo field; when the primary userinfo response omits it (GitHub private email), fetch the **primary verified** email via a secondary endpoint (`/user/emails`, requiring the `user:email` scope). Only a verified email is trusted; blank when unavailable.
- **D-03 (multi-provider linking = design-for now, link later):** GitHub + Google + Okta + EntraID must all be able to resolve to **one** user. The `oidc_bindings` schema already supports this — `UNIQUE (issuer, subject)` with an FK to `users(id)`. This phase ensures the `oauth2` provider stores bindings the same `(issuer, subject) → user_id` way, keeping current JIT behavior (binding miss = new user). Actual cross-provider *linking* is a deliberate fast-follow phase, not built here. **Do NOT introduce a 1:1 user↔subject assumption anywhere that would preclude later linking.**
- **D-04 (roles = userinfo-as-claims + default fallback):** Feed the userinfo JSON response into the **same per-provider `claims_mapping`** mechanism OIDC uses. When no mapping matches, fall back to the configured JIT default role. One role model for all provider kinds. Org/team-membership fetching is explicitly **not** in this phase.

**AUTH-04 — MCP OAuth 2.1 resource server (spgr-tmqm)** — grounded in MCP Authorization spec rev 2025-06-18. MCP server acts purely as an OAuth 2.1 resource server; the external IdP is the authorization server.
- **D-05 (implement the RS-side MUSTs):** (1) RFC 9728 Protected Resource Metadata at `/.well-known/oauth-protected-resource` with `authorization_servers` and this server's canonical resource URI. (2) `WWW-Authenticate: Bearer resource_metadata=...` challenge on unauthenticated MCP requests (RFC 9728 §5.1). (3) Audience/resource-bound validation (RFC 8707) — tokens MUST be validated as issued specifically for this MCP server (audience = server's canonical resource URI), **not** merely against the verifier's `client_id`. No token passthrough; reject tokens not audience-bound to this RS. (4) Error codes: **401** invalid/absent, **403** insufficient scope, **400** malformed.
- **D-06 (opaque tokens via RFC 7662 introspection):** In addition to local JWT validation, add a token introspection path so IdPs that issue opaque (non-JWT) access tokens work. Per-request introspection call + config, gated to introspection-capable providers.
- **D-07 (advertise ALL configured IdPs):** Protected-resource metadata lists **every** configured interactive/OAuth IdP issuer in `authorization_servers`; the MCP client selects one per RFC 9728 §7.6. No new server-side selection logic, no single-designated-AS config field.
- **D-08 (static credentials remain fully additive):** Existing `spgr_sk_` API keys and `spgr_ws_` sessions MUST keep working with zero behavior change. The prefix dispatcher (`Resolve`) keeps routing `spgr_sk_` → API key, `spgr_ws_` → session, bearer JWT → audience-checked `resolveJWT`, other opaque bearer tokens → introspection. All credential types accepted concurrently. The `WWW-Authenticate` 401 challenge is emitted for OAuth-client discovery **without** breaking key-based clients.

**AUTH-05 — Populate `web_sessions.issuer` (spgr-bbp2)**
- **D-09 (issuer value = verified `iss` for OIDC, provider-id for oauth2):** For OIDC providers, store the verified `iss` claim (canonical issuer URL). For `oauth2`/GitHub (no `iss`), store a stable synthetic issuer derived from the provider config (the configured provider ID or its issuer/base URL). One `issuer` column, meaningful for every provider type. Thread this value from the login flow into `CreateSession(sess.Issuer = ...)` at mint time.
- **D-10 (no backfill):** Do **not** backfill existing empty-issuer sessions. They age out within the 12h default session TTL; all new sessions carry the issuer.

### the agent's Discretion (planner/researcher)
- Exact config field names/shape for the `oauth2` provider kind (URL fields, subject/email selectors) and how the userinfo field-selector is expressed (D-01/D-02/D-04).
- Precise `LoginProvider` interface accommodation for the non-OIDC path (the current interface returns a raw id_token for `Exchange`; the oauth2 path returns userinfo instead — reconcile the seam without breaking the OIDC provider).
- The exact audience/resource-URI reconciliation in `resolveJWT` and how the canonical MCP resource URI is derived/configured (D-05.3).
- Introspection endpoint config + caching/rate-limiting for the opaque-token path (D-06).
- Rate-limiting and audit-log line shape for the new login and RS paths — follow existing per-IP/per-issuer limiter and slog conventions.

### Deferred Ideas (OUT OF SCOPE)
- **Cross-provider account linking flow** (verified-email auto-link or explicit "link account" UX). Data model ships this phase (D-03); linking flow is a fast-follow. Security-sensitive (account-takeover on unverified email).
- **GitHub org/team-membership → role mapping** (extra API calls + `read:org` scope). This phase does userinfo-field mapping + default fallback only (D-04).
- **RP-initiated logout** — AUTH-05 stores the issuer *so that* a future RP-logout can target the correct `end_session_endpoint`; the logout flow itself is out of scope.
- **SpecGraph as its own authorization server / DCR proxy** — explicitly rejected. SpecGraph stays a pure resource server.
- **Deprecating static API keys** — rejected in favor of fully additive coexistence (D-08).

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AUTH-01 | Native generic OAuth2 + userinfo login provider (GitHub-direct) (`spgr-1rq9`) | New `oauth2` `LoginProvider` impl + config fields (Standard Stack, Architecture Pattern 1); shared identity-materialization seam (Pattern 2); reuses `claims_mapping` (Pattern 4); `oidc_bindings` unchanged (Don't Hand-Roll) |
| AUTH-04 | MCP OAuth 2.1 resource server delegating auth to a real IdP (`spgr-tmqm`) | RFC 9728 metadata endpoint + WWW-Authenticate (Pattern 3, Code Examples); audience reconciliation in verifier (Pitfall 2); introspection branch in `Resolve` (Pattern 5); MCP-spec MUSTs table (State of the Art) |
| AUTH-05 | Populate `web_sessions.issuer` for audit / future RP-logout (`spgr-bbp2`) | `Identity.Issuer` field + callback threading (Pattern 6); schema already present; no backfill (D-10) |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| GitHub OAuth2 Authorization-Code + userinfo login (AUTH-01) | API / Backend (`internal/server` handlers + `internal/auth` providers) | Browser (redirect to GitHub, callback) | Token exchange + client secret + userinfo fetch MUST be server-side; the browser only carries the front-channel redirect. GitHub client secret must never reach the SvelteKit client. |
| Identity materialization (binding lookup, JIT, role mapping) | API / Backend (`internal/auth` resolver) | Database (`oidc_bindings`, `users`) | Single resolver owns all identity provisioning; storage owns persistence. Same tier as existing OIDC. |
| MCP RFC 9728 metadata + WWW-Authenticate 401 (AUTH-04) | API / Backend (HTTP mux in `cmd/specgraph/serve.go` + `internal/auth` middleware) | — | Protocol surface is HTTP-layer, mounted alongside `/mcp/`. The MCP client (external agent) consumes it; SpecGraph never renders it. |
| Access-token validation (JWT + introspection) (AUTH-04) | API / Backend (`internal/auth` resolver + verifier) | External IdP (JWKS / introspection endpoint) | Token validation is delegated-auth: SpecGraph validates locally (JWKS) or delegates to the IdP's introspection endpoint. |
| Session issuer persistence (AUTH-05) | Database (`web_sessions.issuer`) | API / Backend (callback sets `sess.Issuer`) | Column already exists; the backend just threads the verified issuer at mint time. |
| Provider list surfacing | Frontend Server / Browser (`web/src/lib/oidc.svelte.ts`, `LoginModal.svelte`) | API (`/api/auth/oidc/providers`) | GitHub appears in the existing provider list with zero id_token-specific assumptions in the UI (verified: UI only reads `id`/`display_name`). |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/coreos/go-oidc/v3` | v3.17.0 | OIDC discovery + JWKS-backed JWT verification (`oidc.NewProvider`, `provider.Verifier`, `provider.Endpoint()`) | Already the project's OIDC engine (`oidc_verifier.go`); the `oauth2` path *bypasses* it but the JWT-bearer/MCP path keeps using it. `[VERIFIED: go.mod]` |
| `golang.org/x/oauth2` | v0.36.0 | OAuth2 Authorization-Code flow, PKCE helpers (`oauth2.GenerateVerifier`, `oauth2.S256ChallengeFromVerifier`), `oauth2.Config.Exchange`, `Token.Extra`/`AccessToken` | Already used for the OIDC login flow (`loginprovider.go`, `auth_oidc_handler.go:146-147`); GitHub `oauth2` provider reuses the same `oauth2.Config`, only replacing id_token verification with a userinfo GET. `[VERIFIED: go.mod]` |
| `github.com/mark3labs/mcp-go` | v0.45.0 | MCP StreamableHTTP server (`server.NewStreamableHTTPServer`, `WithHTTPContextFunc`) | Already the MCP transport (`internal/mcp/server.go:157`). AUTH-04 mounts the metadata/challenge *around* this handler; the handler itself is unchanged. `[VERIFIED: go.mod]` |
| `github.com/jackc/pgx/v5` | (in tree) | Postgres driver for `AuthStore` (`CreateSession`, bindings) | Existing storage layer; AUTH-05 needs no new queries. `[VERIFIED: internal/storage/postgres/web_auth.go]` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/http` (stdlib) | — | Userinfo/`/user/emails` GET, introspection POST, RFC 9728 metadata handler, WWW-Authenticate wrapper | All new HTTP I/O. Reuse the existing `newHTTPClient` pattern (`serve.go:227`) with a bounded timeout for outbound calls. `[VERIFIED: cmd/specgraph/serve.go]` |
| `golang.org/x/time/rate` | (in tree) | Per-issuer / per-endpoint token buckets for introspection + JIT | Reuse the existing `rateLimiterFor` pattern (`identitystore.go:643-651`). `[VERIFIED: identitystore.go]` |
| `log/slog` (stdlib) | — | Structured audit logging (`slog.Bool("audit", true)` convention) | New login + RS paths follow the existing audit-line shape (`loginsync.go:127`). `[VERIFIED: loginsync.go]` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Config-driven `oauth2` provider (D-01) | Hardcoded GitHub preset | Rejected by D-01 — config keeps a second non-OIDC IdP cheap and avoids a preset registry. |
| Local JWKS validation (go-oidc) | Introspection for *all* tokens | Introspection is per-request network I/O; use it only for opaque tokens (D-06), keep JWKS for JWTs. |
| Adding `Issuer` to `Identity` struct | Returning issuer via a side channel / second lookup | The struct field is the minimal, race-free carrier; `resolveJWT` already has `claims.Issuer` in scope (`identitystore.go:443`). |

**Installation:** No `go get` required — all libraries are already direct dependencies in `go.mod`. `[VERIFIED: go.mod]`

**Version verification:** Confirmed via `grep` of `go.mod` this session: `go-oidc/v3 v3.17.0`, `golang.org/x/oauth2 v0.36.0`, `mark3labs/mcp-go v0.45.0`. `[VERIFIED: go.mod]`

## Package Legitimacy Audit

> **This phase installs NO new external packages.** Every library needed (`go-oidc/v3`, `x/oauth2`, `mark3labs/mcp-go`, `pgx/v5`, `x/time/rate`) is already a direct, in-use dependency in `go.mod`. GitHub's OAuth/userinfo endpoints (`https://github.com/login/oauth/authorize`, `.../access_token`, `https://api.github.com/user`, `/user/emails`) are **configuration values (URLs), not packages** — they ship as documented config per D-01, so there is no registry artifact to slopsquat.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| *(none — no new packages)* | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
AUTH-01 (GitHub oauth2 login) — front + back channel
───────────────────────────────────────────────────────
 Browser ──GET /api/auth/oidc/{github}/start──▶ handleStart (auth_oidc_handler.go:103)
   │                                             │ create oidc_login_flows row, set tx cookie
   │  302 → GitHub authorize (PKCE S256)         │ p.AuthCodeURL(state,nonce,challenge,redirectURI)
   ▼                                             ▼
 GitHub authorize ── user consents ── 302 → /api/auth/oidc/callback?code&state
                                               │ handleCallback (:167)
                                               ▼
   p.Exchange(code) ──[oauth2 provider]──▶ POST token endpoint → access_token
                     │                     GET /user (userinfo) ──▶ subject/email selectors
                     │                     GET /user/emails (if email private) ──▶ primary verified
                     ▼
        *OIDCClaims{Issuer=synthetic, Subject=<stable id>, Email, Raw=userinfo}
                     │
                     ▼
   materializeIdentity(claims, interactive=true)  ◀── SHARED with resolveJWT tail
     LookupOIDCBinding(issuer,subject) ─miss─▶ jitResolve (claims_mapping → role, default fallback)
                     │  hit ─▶ GetUserByID ─▶ (login-sync if enabled)
                     ▼
        Identity{UserID, Issuer, Subject, Role, ...}
                     │
                     ▼
   CreateSession{Issuer=claims.Issuer (AUTH-05)} ─▶ spgr_ws_ cookie ─▶ 302 /

AUTH-04 (MCP resource server) — token path
───────────────────────────────────────────────────────
 MCP client ──(no token)──▶ /mcp/ ─▶ [RequireAuth wrapper] ─▶ 401 + WWW-Authenticate: Bearer resource_metadata="…/.well-known/oauth-protected-resource"
 MCP client ──GET /.well-known/oauth-protected-resource──▶ {resource:<canonical URI>, authorization_servers:[all IdP issuers], scopes_supported}
 MCP client ──(OAuth flow against chosen IdP, resource=<canonical URI>)──▶ access token
 MCP client ──Bearer <token>──▶ /mcp/ ─▶ Resolve dispatch:
     JWT-shaped  ─▶ resolveJWT ─▶ per-issuer verifier ─▶ audience == canonical resource URI? ─▶ binding/JIT
     opaque      ─▶ resolveIntrospection ─▶ POST IdP /introspect ─▶ active? aud? ─▶ binding/JIT   [NEW, D-06]
     spgr_sk_    ─▶ resolveAPIKey   (unchanged, D-08)
     spgr_ws_    ─▶ resolveSession  (unchanged, D-08)
```

### Recommended Project Structure

```
internal/auth/
├── loginprovider.go        # extend: add oauth2LoginProvider + relax BuildLoginProviders kind gate
├── oauth2_provider.go      # NEW: oauth2LoginProvider (Exchange → userinfo → *OIDCClaims)
├── identitystore.go        # extract materializeIdentity(); add resolveIntrospection branch in Resolve
├── introspection.go        # NEW: RFC 7662 client + per-issuer config/cache (D-06)
├── oidc_verifier.go        # audience reconciliation: verify against canonical resource URI (D-05.3)
└── auth.go                 # add Identity.Issuer field (AUTH-05)
internal/server/
├── auth_oidc_handler.go    # thread sess.Issuer at CreateSession (AUTH-05); userinfo path is provider-side
└── mcp_metadata.go         # NEW: RFC 9728 handler + WWW-Authenticate challenge wrapper (AUTH-04)
internal/config/
└── global.go               # add oauth2/userinfo/introspection fields to OIDCProviderConfig + resource URI
cmd/specgraph/
└── serve.go                # mount metadata endpoint + swap /mcp/ auth wrapper to challenge-emitting one
```

### Pattern 1: `oauth2` LoginProvider (non-OIDC Authorization-Code + userinfo)

**What:** A second `LoginProvider` implementation that reuses `oauth2.Config` for the code exchange but replaces id_token verification with a userinfo GET.
**When to use:** GitHub and any future plain-OAuth2 IdP (D-01).
**Key seam constraint:** `BuildLoginProviders` currently aborts on `kind != "oidc"` (`loginprovider.go:117-119`) and the `oidc` provider's `Exchange` requires an `id_token` (`:61-64`). The `oauth2` provider has no id_token — see Pattern 2 for the return-type reconciliation.

```go
// Source: derived from existing oidcLoginProvider.oauth2Config (loginprovider.go:71-79)
// and auth_oidc_handler.go PKCE usage (:146-147). All AuthCodeURL/PKCE machinery is reused.
type oauth2LoginProvider struct {
    id, displayName            string
    authURL, tokenURL          string
    userinfoURL, emailsURL     string   // D-02: /user and /user/emails
    clientID, secret           string
    scopes                     []string // must include user:email for private-email fallback
    subjectField, emailField   string   // D-02 selectors, e.g. "id", "email"
    issuerID                   string   // D-09 synthetic issuer (provider ID or base URL)
    httpClient                 *http.Client
}
// AuthCodeURL mirrors oidc but WITHOUT oidc.Nonce (no id_token to bind a nonce to);
// state + PKCE S256 remain the CSRF/interception defenses.
```

### Pattern 2: Reconcile the `Exchange` seam via a shared identity assertion (DISCRETION — recommended)

**What:** `LoginProvider.Exchange` today returns `(idToken string, err error)` (`loginprovider.go:29`), and `handleCallback` passes that string straight into `resolver.Resolve(...)` (`auth_oidc_handler.go:215`) where `isJWTShaped` routes it to `resolveJWT`. GitHub has no JWT, so this contract cannot be satisfied by `oauth2`.
**Recommended reconciliation:** Change `Exchange` to return a normalized `*OIDCClaims` (the type already carries `Issuer`, `Subject`, `Email`, `Name`, `Raw` — `oidc_verifier.go:23-30`) instead of a raw string. The OIDC provider verifies the id_token and returns the parsed claims (it already parses them internally); the `oauth2` provider builds `*OIDCClaims` from userinfo with a synthetic `Issuer`. Then extract the *tail* of `resolveJWT` (everything after `verifier.Verify`, i.e. `identitystore.go:443-485`) into:

```go
// materializeIdentity is the shared binding-lookup → JIT → login-sync → Identity tail,
// called by BOTH resolveJWT (JWT bearer path) and the interactive login callback.
func (s *pgIdentityStore) materializeIdentity(ctx context.Context, claims *OIDCClaims, interactive bool) (*Identity, error)
```

**Why this is the low-risk seam:** it does NOT touch the `Resolver.Resolve(ctx, token string)` interface (`resolver.go:18`) that the interceptor, `RequireAuth`, API keys, sessions, and MCP bearer tokens depend on — so D-08 additivity is structurally preserved. Only the *interactive-login* entrypoint gets a new claims-based method. The bearer-JWT MCP path (AUTH-04) still enters through `Resolve` → `resolveJWT` unchanged.
**Anti-alternative:** Do NOT overload `Resolve` to accept userinfo blobs or synthesize a fake JWT — that pollutes the credential dispatcher and risks routing bugs (`isJWTShaped`, `identitystore.go:192`).

### Pattern 3: RFC 9728 metadata + WWW-Authenticate challenge (AUTH-04)

**What:** A `GET /.well-known/oauth-protected-resource` handler returning the metadata JSON, plus a middleware that replaces the bare `RequireAuth` 401 on `/mcp/` with a challenge-carrying 401.
**Where it mounts:** `serve.go:246-248` currently wraps `/mcp/` with `auth.RequireAuth(resolver)`. `RequireAuth` (`middleware.go:16-37`) writes `http.Error(w, {"error":"unauthenticated"}, 401)` with **no** `WWW-Authenticate` header. AUTH-04 needs either a new MCP-specific middleware or a variant of `RequireAuth` that sets the header before writing 401. Mount the metadata handler on the root `mux` (unauthenticated, like `/api/auth/oidc/providers`).

```go
// Source: RFC 9728 §5.1 challenge format + §3.2 metadata shape (CITED, see State of the Art)
w.Header().Set("WWW-Authenticate",
    `Bearer resource_metadata="`+baseURL+`/.well-known/oauth-protected-resource"`)
http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
```

### Pattern 4: Reuse `claims_mapping` for userinfo → role (D-04)

**What:** The `applyClaimsMapping(claims map[string]json.RawMessage, rules []config.ClaimMapping)` function (`identitystore.go:581-592`) and login-sync's `resolveLoginRole` (`loginsync.go:27-39`) operate on a `map[string]json.RawMessage`. GitHub's userinfo JSON deserializes into exactly this shape, so mapping userinfo fields (e.g. `login`) to roles works with **zero new mapping code** — the `oauth2` provider just populates `OIDCClaims.Raw` from the userinfo body. The JIT default-role fallback (`identitystore.go:532-540`) applies unchanged.

### Pattern 5: Introspection branch in the `Resolve` dispatcher (D-06)

**What:** `Resolve` (`identitystore.go:167-178`) routes: JWT-shaped → `resolveJWT`; `spgr_ws_` → `resolveSession`; else → `resolveAPIKey`. Add a branch so an opaque bearer that is neither `spgr_sk_`/`spgr_ws_` nor JWT-shaped attempts RFC 7662 introspection against introspection-capable providers, then materializes identity via the shared helper. **Ordering matters:** `spgr_sk_` and `spgr_ws_` prefix checks MUST run before introspection so static credentials never hit the network (D-08). The current `resolveAPIKey` fallthrough already parses the `spgr_sk_` prefix and rejects non-matching tokens with `ErrUnauthenticated` (`identitystore.go:306-310`) — introspection slots in *before* that final fallthrough, gated on config presence.

### Pattern 6: Thread `Issuer` for AUTH-05

**What:** Add `Issuer string` to `Identity` (`auth.go:19-27`). `resolveJWT` sets it from `claims.Issuer` (already in scope, `identitystore.go:443`); the oauth2 callback path sets the synthetic issuer. In `handleCallback`, set `sess.Issuer = id.Issuer` on the `storage.Session` literal (`auth_oidc_handler.go:258-260`), which `CreateSession` already persists (`web_auth.go:31-38`). **Do NOT** set it on the CLI-exchange path — `ExchangeCLICode` deliberately leaves issuer empty (`web_auth.go:228-231`); if CLI login for GitHub is in scope, thread it through `CreateCLICode`/`ExchangeCLICode` explicitly rather than assuming.

### Anti-Patterns to Avoid

- **Overloading `Resolve(ctx, token string)` for userinfo:** breaks the credential dispatcher contract and D-08. Use a separate claims-based interactive-login entrypoint (Pattern 2).
- **Trusting an unverified GitHub email:** D-02 requires the *primary verified* email; a userinfo `email` that isn't verified must be treated as blank. GitHub `/user.email` can be null (private) — the `/user/emails` fallback returns `{email, primary, verified}` entries; select `primary && verified`.
- **Binding on GitHub `login` (username):** usernames are renameable. D-02 mandates the numeric `id` as subject.
- **Emitting `WWW-Authenticate` on the API-key/ConnectRPC paths:** the challenge belongs only on the MCP resource route; ConnectRPC returns `CodeUnauthenticated` (`interceptor.go:114-117`) and must keep doing so for key-based clients (D-08).
- **Validating MCP token audience against `client_id`:** the current `NewOIDCVerifier` sets `oidc.Config{ClientID: cfg.Audience || cfg.ClientID}` (`oidc_verifier.go:50-54`). For MCP the audience MUST be the canonical resource URI (RFC 8707), which is generally *not* the OIDC client_id.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT signature/exp/aud validation | Custom JWS parser | `go-oidc/v3` `provider.Verifier` (already wired, `oidc_verifier.go:54`) | JWKS rotation, alg allow-listing, clock skew all handled. |
| PKCE verifier/challenge | Manual S256 | `oauth2.GenerateVerifier` / `oauth2.S256ChallengeFromVerifier` (already used, `auth_oidc_handler.go:146-147`) | Correct RFC 7636 encoding. |
| OAuth2 code exchange | Hand-built POST | `oauth2.Config.Exchange` (already used, `loginprovider.go:57`) | Handles form encoding, basic auth, error parsing. |
| RFC 9728 metadata document | Ad-hoc JSON | A typed struct serialized once; fields per §3.2 | Field names are normative (`resource`, `authorization_servers`); `resource` MUST equal the canonical URI or clients reject it (§3.3). |
| Binding/JIT/user creation | New provisioning code for oauth2 | Shared `materializeIdentity` + existing `JITCreateHuman` (`identitystore.go:554`) | The `oidc_bindings` `UNIQUE(issuer,subject)` model already supports N-providers-per-user (D-03); duplicating it risks a 1:1 assumption the spec forbids. |
| Per-issuer rate limiting | New limiter | Existing `rateLimiterFor` (`identitystore.go:643`) | Consistent burst/refill semantics. |
| Session issuer column | Schema migration | Column already exists (`002_web_auth.sql:10`) | AUTH-05 is population-only (D-10, no migration). |

**Key insight:** The only genuinely new *logic* in this phase is (a) userinfo fetch + field selection (AUTH-01), (b) the RFC 9728 metadata/challenge protocol surface (AUTH-04), and (c) RFC 7662 introspection (AUTH-04 D-06). Everything else is wiring existing, tested machinery to a new entrypoint.

## Runtime State Inventory

> This phase is additive/greenfield within the auth subsystem, not a rename/refactor. The one migration-adjacent concern is AUTH-05's issuer population, explicitly scoped to **no backfill** (D-10).

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `web_sessions.issuer` column exists; existing rows have `issuer=''` (DB default, `002_web_auth.sql:10`). New `oauth2` bindings land in `oidc_bindings` with a synthetic issuer string. | **No data migration** (D-10). New writes only. Verify no query treats `issuer=''` as an error. |
| Live service config | GitHub OAuth App must be registered out-of-band (client id/secret, callback URL `<base>/api/auth/oidc/callback`). Introspection-capable IdPs need an introspection endpoint + client credentials. | Operator config, not code. Document required GitHub App settings + scopes (`read:user`, `user:email`). |
| OS-registered state | None. | None — verified: no scheduler/daemon registrations touched. |
| Secrets/env vars | GitHub client secret via `client_secret_env` (existing pattern, `loginprovider.go:83-95`); introspection client secret similarly. No secret *renames*. | Add new provider entries to config; reuse `resolveClientSecret`. |
| Build artifacts | `gen/` proto code — **not touched** (this phase adds no RPCs; login/MCP-metadata are plain HTTP handlers, not ConnectRPC services). | None. If a future revision adds an RPC, run `task proto`. |

**Nothing found requiring migration or re-registration** beyond operator-side IdP/GitHub-App configuration.

## Common Pitfalls

### Pitfall 1: The OIDC `Exchange` contract cannot represent GitHub
**What goes wrong:** Implementers try to make `oauth2LoginProvider.Exchange` return a `string` to satisfy the current interface (`loginprovider.go:29`), then synthesize a fake JWT or stuff userinfo into a string — routing chaos in `isJWTShaped`/`Resolve`.
**Why it happens:** The interface was designed OIDC-only; `handleCallback` feeds the return value straight into `resolver.Resolve`.
**How to avoid:** Change the interface to return `*OIDCClaims` and route the callback through the shared `materializeIdentity` helper (Pattern 2). Keep `Resolve(token string)` untouched.
**Warning signs:** Any `strings.Count(token, ".")` logic near the login path; a "fake id_token" builder.

### Pitfall 2: MCP audience validated against `client_id` instead of the resource URI
**What goes wrong:** MCP tokens are accepted as long as `aud == client_id`, so a token minted for a *different* SpecGraph OAuth client (or another RS sharing the IdP) is honored — the RFC 8707 "access token privilege restriction" / confused-deputy failure the MCP spec explicitly forbids.
**Why it happens:** `NewOIDCVerifier` hardcodes `oidc.Config{ClientID: cfg.Audience || cfg.ClientID}` (`oidc_verifier.go:50-54`); reusing that verifier verbatim for MCP inherits the wrong audience.
**How to avoid:** For the MCP resource-server validation, verify the token's `aud` contains the server's **canonical resource URI** (D-05.3). This likely means a distinct verifier configuration (or an explicit post-verify `aud` check against the resource URI) for the MCP path — the interactive-login/OIDC-web verifier keeps `client_id` audience. Decide (Open Question 1) how the canonical URI is configured.
**Warning signs:** A single verifier instance shared between web-login id_token verification and MCP access-token verification.

### Pitfall 3: GitHub private email → blank binding email
**What goes wrong:** Users with private emails get `email=""`, breaking email-domain allowlist gating (`jitResolve` refuses empty email when an allowlist is set — `identitystore.go:498-509`), so GitHub logins are rejected even though intended.
**Why it happens:** `/user` returns `email: null` for private-email users; the `/user/emails` fallback (D-02) is easy to forget.
**How to avoid:** Always attempt `/user/emails` (requires `user:email` scope) when the primary email is absent; pick `primary && verified`. Document that an email-domain allowlist + GitHub private emails + missing `user:email` scope = locked-out users.
**Warning signs:** `insufficient_scope` from `/user/emails`; allowlist rejections with `domain=""` in logs.

### Pitfall 4: WWW-Authenticate leaks onto non-MCP 401s or breaks API-key clients
**What goes wrong:** Adding the challenge to the shared `RequireAuth` or the ConnectRPC interceptor makes every unauthenticated response advertise OAuth discovery, and can confuse `spgr_sk_`/`spgr_ws_` clients (D-08 violation).
**Why it happens:** `RequireAuth` is shared across `/api/*` and `/mcp/` (`serve.go:214-215, 246`).
**How to avoid:** Scope the challenge to the `/mcp/` route only (a dedicated wrapper), leaving `RequireAuth` and the ConnectRPC interceptor unchanged. Static-credential clients still get a plain 401 and keep working.
**Warning signs:** `WWW-Authenticate` appearing on `/api/auth/*` responses or ConnectRPC error frames.

### Pitfall 5: Introspection turned into an unbounded per-request network call
**What goes wrong:** Every opaque bearer triggers a synchronous POST to the IdP introspection endpoint with no cache/timeout, adding latency and a DoS amplification vector.
**Why it happens:** Naive D-06 implementation.
**How to avoid:** Bound the introspection HTTP client timeout, cache active results to the token's `exp` (short TTL), and rate-limit per issuer (reuse `rateLimiterFor`). Fail-closed to `ErrUnauthenticated` on introspection failure, `ErrTransient` on IdP outage — matching existing error discipline (`identitystore.go:316`).
**Warning signs:** No timeout on the introspection client; no cache; introspection attempted for `spgr_sk_`-prefixed tokens.

### Pitfall 6: ADR-004 transaction reach on `AuthStore`
**What goes wrong:** New multi-query write paths call `*Store.RunInTransaction`, which `AuthStore` **cannot reach** (it is a structurally separate store — `.planning/intel/decisions.md` ADR-004 note; `.planning/intel/constraints.md` CLI-OIDC entry).
**Why it happens:** Habit from the project-scoped store.
**How to avoid:** For any atomic multi-statement auth write, use a direct `pool.Begin`-based transaction like `ExchangeCLICode` (`web_auth.go:180-222`). Most of this phase's writes are single-statement (`CreateSession`, `JITCreateHuman` is already atomic internally) so this rarely bites — but the account-linking-adjacent code (if any oauth2 binding+user write is added) must follow the `AuthStore` tx pattern.
**Warning signs:** `RunInTransaction` referenced inside `internal/storage/postgres/*auth*` or `internal/auth`.

## Code Examples

### RFC 9728 Protected Resource Metadata response (AUTH-04, D-05.1)
```go
// Source: RFC 9728 §3.2 (metadata shape) + §2 (field semantics). CITED.
// resource MUST equal the canonical resource identifier or clients discard it (§3.3).
type protectedResourceMetadata struct {
    Resource               string   `json:"resource"`                          // REQUIRED: canonical MCP resource URI
    AuthorizationServers   []string `json:"authorization_servers"`             // D-07: ALL configured IdP issuers
    BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`// ["header"]
    ScopesSupported        []string `json:"scopes_supported,omitempty"`
}
// GET /.well-known/oauth-protected-resource → 200 application/json, Cache-Control: max-age=... (§7.10)
```

### WWW-Authenticate 401 challenge (AUTH-04, D-05.2)
```
// Source: RFC 9728 §5.1 (verbatim header format). CITED.
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"
```

### GitHub userinfo + verified-email fallback (AUTH-01, D-02)
```go
// Source: GitHub REST API (config-supplied URLs per D-01). Subject = numeric id; email via fallback.
// GET https://api.github.com/user           → { "id": 583231, "login": "octocat", "email": null }
// GET https://api.github.com/user/emails     → [ { "email":"a@x.com","primary":true,"verified":true }, ... ]
// subjectField="id" → "583231" (stringified, stable). emailField="email"; if empty, pick primary&&verified.
```

### Existing PKCE/state machinery the oauth2 provider reuses (AUTH-01)
```go
// Source: internal/server/auth_oidc_handler.go:146-147 (VERIFIED). Reused verbatim; only Exchange differs.
verifier := oauth2.GenerateVerifier()
challenge := oauth2.S256ChallengeFromVerifier(verifier)
// state + PKCE remain; nonce is OIDC-only and is dropped for the oauth2 (no id_token) provider.
```

## State of the Art

### MCP Authorization spec rev 2025-06-18 — concrete resource-server MUSTs (CITED)

| Requirement | Spec source | SpecGraph gap |
|-------------|-------------|---------------|
| RS MUST implement RFC 9728 Protected Resource Metadata; doc MUST include `authorization_servers` with ≥1 AS | MCP §"Authorization Server Location"; RFC 9728 §2 | New endpoint (Pattern 3) |
| Well-known path `/.well-known/oauth-protected-resource` (GET, 200, `application/json`) | RFC 9728 §3, §3.1, §3.2 | New handler |
| RS MUST emit `WWW-Authenticate: Bearer resource_metadata="…"` on 401 | MCP §"Authorization Server Location"; RFC 9728 §5.1 | `RequireAuth` emits no challenge (`middleware.go:20-29`) |
| RS MUST validate token audience = issued specifically for this RS (canonical resource URI) | MCP §"Token Handling", §"Access Token Privilege Restriction"; RFC 8707 §2 | Verifier checks `client_id`, not resource URI (`oidc_verifier.go:50-54`) |
| RS MUST NOT accept/transit tokens not intended for it; **no token passthrough** | MCP §"Access Token Privilege Restriction" | Ensure the MCP loopback client (`serve.go:229`) does not forward the *client's* token to upstream as-is (it re-forwards via `WithBearerToken` `serve.go:237` — verify this is the same-audience internal hop, not an upstream API) |
| Error model: 401 invalid/absent, 403 insufficient scope, 400 malformed | MCP §"Error Handling" | New in the MCP wrapper |
| `resource` value in metadata MUST equal the resource identifier used to fetch it | RFC 9728 §3.3 | Canonical URI must be configured consistently |
| Canonical resource URI: `https`, no fragment, trailing slash discouraged; examples `https://mcp.example.com/mcp` | MCP §"Canonical Server URI"; RFC 8707 §2 | Open Question 1 (derivation) |
| Bearer token in `Authorization` header only, never query string; sent on every request | MCP §"Token Requirements" | Already header-only (`serve.go:233`) |
| Multiple ASes allowed; client selects per RFC 9728 §7.6 | MCP §"Authorization Server Location"; RFC 9728 §7.6 | Matches D-07 (advertise all) — no server-side selection |

**Note on DCR/PKCE/token issuance:** All authorization-server responsibilities (authorize endpoint, token endpoint, PKCE enforcement, Dynamic Client Registration) are **out of scope** for SpecGraph — it is a pure resource server (deferred: "SpecGraph as its own AS"). The spec's `MUST implement PKCE` / `MUST use resource parameter` clauses are **client-side** obligations, not RS obligations.

### Existing auth evolution (from intel)
| Old Approach | Current Approach | When | Impact |
|--------------|------------------|------|--------|
| Static `rpcPermissions` table | Cedar policy engine (`auth.PolicyEngine`) | 2026-05-26 | New procedures/paths are authorized by Cedar; MCP metadata/challenge endpoints are unauthenticated (public), so no Cedar entry needed for discovery. |
| ID-token-in-cookie | Opaque `spgr_ws_` server-side sessions | 2026-06-12 | AUTH-05 issuer rides on the server-side session row, not a token claim. |
| `groups`-claim role mapping | App-roles + `sync_on_login` | 2026-06-15 | D-04 userinfo mapping plugs into the same `claims_mapping`/login-sync machinery. |

**Deprecated/outdated:** GitHub-via-OIDC-broker (the prior `oidc-interactive-ui-login-design` said "GitHub only via an OIDC broker") is **explicitly replaced** by this phase's native `oauth2` provider (D-01) — no broker.

## Assumptions Log

> Claims tagged `[ASSUMED]` — need user/operator confirmation before becoming locked plan decisions.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GitHub's OAuth/userinfo endpoints and shapes (`/user` returns numeric `id`, `email` may be null; `/user/emails` returns `{email,primary,verified}` requiring `user:email` scope) are as described. | Code Examples, Pitfall 3 | If GitHub's API shape differs, subject/email selection breaks. Low risk — stable public API — but not re-verified against GitHub docs live this session. |
| A2 | The canonical MCP resource URI should be `<server base URL>/mcp` (matching the mount path `serve.go:246`), no trailing slash. | Open Question 1 | If chosen inconsistently with what clients request as `resource=`, RFC 9728 §3.3 validation fails and audience checks reject valid tokens. Needs an explicit config decision. |
| A3 | The MCP loopback re-forward of the client's bearer token (`serve.go:237` `WithBearerToken`) is an internal same-audience hop to SpecGraph's own ConnectRPC, not an upstream third-party API — so it is not a forbidden "token passthrough." | State of the Art (passthrough row) | If it were an upstream API hop, it would violate the MCP no-passthrough MUST. Must be confirmed by reading the loopback client's target during planning. |
| A4 | Introspection (D-06) config is per-provider (issuer → introspection endpoint + credentials), reusing `OIDCProviderConfig`. | Pattern 5 | If a separate config shape is preferred, field layout changes. Discretion area per CONTEXT. |

**If this table is empty:** it is not — A1–A4 need confirmation during discuss/plan.

## Open Questions (RESOLVED)

> All three resolved during planning (Phase 3 plans 03-01/03-03/03-04). Resolutions annotated inline below.

1. **How is the canonical MCP resource URI derived/configured? (D-05.3)** — **RESOLVED (A2):** dedicated `auth.oidc.mcp_resource_uri` config, default `<base_url>/mcp`, startup-fatal `https` + no-fragment validation (`ValidateMCPResourceURI`, plan 03-01/03-03).
   - What we know: the MCP handler mounts at `/mcp/` (`serve.go:246`); the server has a base URL concept (`cfg.Auth.OIDC.BaseURL`, `RedirectURI` derivation `loginprovider.go:172-185`). The MCP spec wants `https`, no fragment, trailing-slash-discouraged, most-specific URI (e.g. `https://host/mcp`).
   - What's unclear: whether to reuse `auth.oidc.base_url`, add a dedicated `mcp.resource_uri` config field, or derive from the request (risky behind proxies). The `resource` in metadata MUST match what clients send as `resource=` and what tokens carry as `aud`.
   - Recommendation: add an explicit `mcp.resource_uri` (or `auth.mcp_resource`) config field, defaulting to `<base_url>/mcp`; validate it is `https` + no fragment at startup (startup-fatal, matching `BuildLoginProviders` discipline `loginprovider.go:103`).

2. **Does the MCP audience check need a separate verifier, or a post-verify `aud` assertion?** — **RESOLVED:** post-verify `aud`-contains-resource-URI assertion (not a second verifier); web-login `aud==client_id` left untouched (plan 03-04).
   - What we know: the current per-issuer verifier binds audience to `client_id` (`oidc_verifier.go:50-54`) and is shared via `s.verifiers` (`identitystore.go:40`).
   - What's unclear: whether the same IdP is used for both web-login (aud=client_id) and MCP (aud=resource URI). If so, one verifier cannot enforce both audiences.
   - Recommendation: verify signature/issuer/exp via the existing verifier, then do an explicit `aud` contains-resource-URI check for the MCP path (a small helper reading `claims.Raw["aud"]`). Keep web-login audience unchanged.

3. **Is GitHub login exposed to the CLI (`specgraph login`)?** — **RESOLVED:** No. GitHub login is NOT exposed to `specgraph login` this phase, and the CLI-exchange path keeps session issuer empty (user-accepted scope decision, bounded ≤1 session TTL per D-10). Threading issuer through `CreateCLICode`/`ExchangeCLICode` is deferred (plan 03-01 assumption).
   - What we know: the CLI broker path (`handleCLIExchange`, `ExchangeCLICode`) deliberately leaves session issuer empty (`web_auth.go:228-231`).
   - What's unclear: whether AUTH-01 GitHub login must work through `specgraph login` too, which would require threading the synthetic issuer through `CreateCLICode`/`ExchangeCLICode` for AUTH-05 completeness.
   - Recommendation: confirm scope; if CLI is in scope, add issuer threading to the CLI code path (small, but a distinct task).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain + `task` | Build/test all requirements | ✓ (project standard) | per Taskfile | — |
| Docker | Postgres integration + e2e tests (testcontainers `pgvector/pgvector:pg18`) | ✓ (per AGENTS.md `task pr-prep`) | — | Unit tests (`task test`) run without Docker; integration/e2e need it |
| GitHub OAuth App (client id/secret) | AUTH-01 manual/e2e verification | ✗ (operator must create) | — | Unit-test the userinfo mapping with a stubbed HTTP server; manual UAT needs a real GitHub App |
| External IdP with introspection endpoint | AUTH-04 D-06 opaque-token path | ✗ (operator config) | — | Unit/integration-test introspection against a stub introspection server |
| Reachable OIDC IdP for discovery | AUTH-04 JWT path (existing) | ✗ at unit-test time | — | Existing tests stub verifiers; `BuildLoginProviders` needs a live issuer only at server startup |

**Missing dependencies with no fallback:** none block *implementation*; GitHub App + live IdP are needed only for **manual UAT / e2e**, not for unit/integration coverage (which stubs HTTP endpoints).
**Missing dependencies with fallback:** GitHub App → stubbed `httptest` server for userinfo/emails; introspection IdP → stub introspection endpoint.

## Validation Architecture

> `workflow.nyquist_validation` is absent from `.planning/config.json` → treated as ENABLED.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (unit + `//go:build integration` testcontainers) + Ginkgo/Gomega (`//go:build e2e`) |
| Config file | `Taskfile.yml` (test task definitions); no separate test config |
| Quick run command | `task test` (excludes integration + e2e via build tags) |
| Full suite command | `task pr-prep` (check → `task test:integration` → `task test:e2e`; requires Docker) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | oauth2 provider exchanges code → userinfo → `*OIDCClaims` (subject=id, verified email fallback) | unit (httptest stub for `/user`,`/user/emails`) | `go test ./internal/auth/ -run OAuth2Provider` | ❌ Wave 0 |
| AUTH-01 | `BuildLoginProviders` constructs oauth2 kind; rejects missing userinfo URL (startup-fatal) | unit | `go test ./internal/auth/ -run BuildLoginProviders` | ⚠️ extend existing `loginprovider` tests |
| AUTH-01 | userinfo → role via `claims_mapping` + default fallback | unit | `go test ./internal/auth/ -run ClaimsMapping` | ✅ (`applyClaimsMapping` covered; add oauth2 case) |
| AUTH-01 | binding miss = new user; second provider binds to distinct `(issuer,subject)` (D-03) | integration (testcontainers) | `go test -tags integration ./internal/auth/ -run JIT` | ✅ extend `identitystore_jit_test.go` / `_integration_test.go` |
| AUTH-04 | `GET /.well-known/oauth-protected-resource` returns metadata with all IdP issuers + canonical resource | unit (httptest) | `go test ./internal/server/ -run ProtectedResourceMetadata` | ❌ Wave 0 |
| AUTH-04 | unauthenticated `/mcp/` → 401 with `WWW-Authenticate: Bearer resource_metadata=...` | unit | `go test ./internal/server/ -run MCPChallenge` | ❌ Wave 0 |
| AUTH-04 | token with wrong audience (client_id but not resource URI) → 401 | unit/integration | `go test ./internal/auth/ -run AudienceBinding` | ❌ Wave 0 |
| AUTH-04 | opaque token → introspection → active/aud → identity; inactive → 401 | unit (stub introspection) | `go test ./internal/auth/ -run Introspection` | ❌ Wave 0 |
| AUTH-04 | `spgr_sk_` / `spgr_ws_` still resolve unchanged with OAuth enabled (D-08) | unit/integration | `go test ./internal/auth/ -run Resolve` | ✅ (`identitystore_test.go`, `identitystore_session_test.go`) — add "OAuth enabled" variant |
| AUTH-05 | web login mints session with `issuer` = verified iss (OIDC) / synthetic (oauth2) | integration | `go test -tags integration ./internal/server/ -run SessionIssuer` | ❌ Wave 0 (extend `identitystore_integration_test.go`) |
| AUTH-05 | no backfill: existing empty-issuer sessions untouched (D-10) | (assertion by absence — no migration) | n/a | n/a |

### Sampling Rate
- **Per task commit:** `task test` (fast, no Docker) — must stay green.
- **Per wave merge:** `task check` (fmt/license/lint/build/unit) then `go test -tags integration ./internal/auth/ ./internal/server/`.
- **Phase gate:** `task pr-prep` (full check + integration + e2e) green before `/gsd-verify-work`.

### Observable validation signals (per success criterion)
1. **GitHub login works with existing session model:** a browser flow through `/api/auth/oidc/{github}/start` → GitHub → callback yields an `spgr_ws_` cookie and an authenticated `/api/auth/whoami`; the resulting `oidc_bindings` row has issuer=synthetic, subject=numeric id.
2. **MCP OAuth 2.1 RS flow:** an MCP client with no token gets a 401 + `WWW-Authenticate`; fetching `/.well-known/oauth-protected-resource` returns all configured issuers; a token audience-bound to the canonical resource URI authenticates, one bound only to `client_id` is rejected.
3. **Session issuer audit:** every newly minted `web_sessions` row has a non-empty `issuer` matching the authenticating provider (verified iss or synthetic); querying by issuer buckets sessions per provider.

### Wave 0 Gaps
- [ ] `internal/auth/oauth2_provider_test.go` — covers AUTH-01 userinfo/email/subject mapping (httptest stubs)
- [ ] `internal/auth/introspection_test.go` — covers AUTH-04 D-06
- [ ] `internal/auth/identitystore_audience_test.go` — covers AUTH-04 D-05.3 audience binding
- [ ] `internal/server/mcp_metadata_test.go` — covers AUTH-04 metadata + challenge
- [ ] Extend `internal/server/identity_integration_test.go` / `identitystore_integration_test.go` — AUTH-05 issuer population
- [ ] Framework install: none — Go test + testcontainers already present

## Security Domain

> `security_enforcement` absent from config → treated as ENABLED.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Delegated OAuth2/OIDC to external IdP; PKCE (`oauth2` helpers), state/nonce; no local password handling |
| V3 Session Management | yes | Existing opaque `spgr_ws_` server-side sessions (`SameSite=Lax`, HttpOnly, dynamic Secure — `auth_oidc_handler.go:270-280`); TTL-bounded; AUTH-05 adds issuer audit metadata |
| V4 Access Control | yes | Cedar policy engine (existing) governs RPCs; MCP discovery endpoints intentionally public; identity role via `claims_mapping` + fail-closed downgrade |
| V5 Input Validation | yes | Validate callback `state` (constant-time, `auth_oidc_handler.go:196`); validate/bound config URLs; reject malformed introspection responses; canonical resource URI must be `https`+no-fragment |
| V6 Cryptography | yes | Never hand-roll: JWKS/JWT via go-oidc; PKCE via x/oauth2; argon2id for API keys (existing). No new crypto. |
| V9 Communications | yes | All IdP/userinfo/introspection calls over HTTPS with cert validation; bounded timeouts |
| V13 API / Web Service | yes | RFC 9728 metadata + RFC 8707 audience binding are the specific API-security controls this phase delivers |

### Known Threat Patterns for delegated OAuth2/OIDC + MCP RS

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token audience confusion / confused deputy (token for another RS accepted) | Elevation of Privilege | RFC 8707 audience validation against canonical resource URI (D-05.3); reject non-bound tokens (Pitfall 2) |
| Token passthrough to upstream API | Elevation / Spoofing | MCP no-passthrough MUST; confirm loopback hop is internal same-audience (A3) |
| GitHub username-rename account hijack | Spoofing | Bind on stable numeric `id`, not `login` (D-02) |
| Unverified-email auto-linking (deferred, but guard now) | Spoofing | Trust only `primary && verified` email; do NOT auto-link across providers this phase (D-03 defers linking) |
| Authorization-code interception | Tampering | PKCE S256 + `state` (reused, `auth_oidc_handler.go:146,196`) |
| Open redirect on callback | Tampering | Existing strict callback/redirect validation (`RedirectURI`, `validateCLICallback`); metadata URLs are server-controlled |
| SSRF via attacker-controlled userinfo/introspection URLs | Information Disclosure | Config-supplied endpoints are operator-trusted + startup-validated; bound timeouts; do not derive endpoints from token/user input |
| Introspection endpoint DoS amplification | Denial of Service | Cache active results to token exp; per-issuer rate limit; bounded client timeout (Pitfall 5) |
| Static-credential regression when OAuth enabled | Denial of Service (lockout) | D-08 additivity: prefix dispatch order preserved; challenge scoped to `/mcp/` only (Pitfall 4) |

## Project Constraints (from AGENTS.md)

- **Taskfile-driven:** use `task build`, `task test`, `task check`, `task pr-prep` — do not invoke `go build`/`go test` ad hoc in place of the gates. Run `task check` before pushing (pre-push hook enforces).
- **`gen/` is committed & generated:** this phase adds **no** proto RPCs (HTTP handlers only); if that changes, edit `proto/` sources and run `task proto` — never edit `gen/` (PreToolUse-blocked).
- **License headers required** on all new `.go` files: `SPDX-License-Identifier: Apache-2.0` + copyright (`task license:add`).
- **DCO sign-off** on every commit (`Signed-off-by:` trailer); conventional-commit messages (cog).
- **ADR-004 / `AuthStore`:** multi-query auth writes use a direct `pool.Begin` transaction, NOT `*Store.RunInTransaction` (unreachable from `AuthStore`) — see Pitfall 6.
- **`revive` package comments:** any new package (e.g. if introspection lands in a new pkg) needs a `// Package …` doc comment.
- **jj-colocated repo:** use `jj --no-pager`, `-m` on describe/commit; never `git push` (use `jj bookmark set` + `jj git push`).
- **Fail-closed identity discipline:** not-found/revoked/expired → `ErrUnauthenticated`; backend failure → `ErrTransient` (`identitystore.go` throughout). New paths must preserve this.
- **Startup-fatal provider construction:** bad `oauth2`/introspection config (missing userinfo URL, non-https resource URI) = boot abort, matching `BuildLoginProviders`.

## Sources

### Primary (HIGH confidence — verified against live source this session)
- `internal/auth/loginprovider.go` — `LoginProvider` interface (:24-30), `Exchange` returns id_token (:29), `oidcLoginProvider.Exchange` (:55-69), `BuildLoginProviders` kind reject (:117-119), `resolveClientSecret` (:83-95), `RedirectURI` (:172-185)
- `internal/auth/identitystore.go` — `Resolve` dispatcher (:167-178), `isJWTShaped` (:192-196), prefixes (:199,:204), `resolveJWT` (:418-485), `peekIssuer` (:395), binding/JIT (:443-452,:496-567), `resolveSession` (:354-390), `applyClaimsMapping` (:581), `rateLimiterFor` (:643-651)
- `internal/auth/oidc_verifier.go` — `OIDCClaims` (:23-30), `NewOIDCVerifier` audience=client_id (:50-54), `Verify` (:79-114)
- `internal/auth/auth.go` — `Identity` struct, no Issuer field (:19-27)
- `internal/auth/resolver.go` — `Resolver` interface (:10-23)
- `internal/auth/middleware.go` — `RequireAuth`, bare 401 no challenge (:16-37)
- `internal/auth/interceptor.go` — `NewAuthInterceptor`, `extractBearerToken`, `mapAuthError` (:21-123)
- `internal/auth/context.go` — `WithBearerToken` (:33), `WithInteractiveLogin` (:83)
- `internal/auth/loginsync.go` — `resolveLoginRole`/`applyLoginSync` (:27-135), audit-log shape (:127)
- `internal/server/auth_oidc_handler.go` — `handleCallback` session mint without issuer (:258-265), PKCE (:146-147), `validateCLICallback` (:290-298)
- `internal/storage/postgres/web_auth.go` — `CreateSession` inserts issuer (:31-38), `ExchangeCLICode` direct tx + empty issuer (:180-231)
- `internal/storage/postgres/auth_migrations/001_initial.sql` — `oidc_bindings UNIQUE(issuer,subject)` FK user_id (:26-34)
- `internal/storage/postgres/auth_migrations/002_web_auth.sql` — `web_sessions.issuer` column (:10)
- `internal/config/global.go` — `OIDCConfig` (:206-224), `OIDCProviderConfig` with Kind "reserved for oauth2" (:286-298), `ClaimMapping` (:301-305)
- `cmd/specgraph/serve.go` — resolver wiring (:162-178), `/mcp/` mount + auth wrapper (:246-248), `WithHTTPContextFunc` Bearer extraction (:231-245)
- `internal/mcp/server.go` — `HTTPHandler`/StreamableHTTP (:156-159)
- `go.mod` — go-oidc/v3 v3.17.0, x/oauth2 v0.36.0, mark3labs/mcp-go v0.45.0

### Secondary (HIGH — authoritative external spec, fetched this session)
- MCP Authorization spec rev 2025-06-18 (`modelcontextprotocol.io/specification/2025-06-18/basic/authorization`) — RS role, RFC 9728 metadata + WWW-Authenticate MUST, RFC 8707 audience MUST, no-passthrough, 401/403/400 error model, canonical URI rules
- RFC 9728 (`datatracker.ietf.org/doc/html/rfc9728`) — metadata fields (§2), well-known path (§3), response shape (§3.2), validation (§3.3), WWW-Authenticate §5.1, authorization_servers §7.6

### Tertiary (project intel — LOCKED ADRs/constraints)
- `.planning/intel/decisions.md` — ADR-004 `AuthStore` tx-reach note
- `.planning/intel/constraints.md` — Identity/RBAC epic lineage (OIDC → Cedar → interactive login → app-roles/login-sync → self-service keys), CLI-OIDC `pool.Begin` tx pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libs verified present in `go.mod`; no new packages.
- Architecture / seams: HIGH — every seam claim cites live source file:line read this session.
- MCP/RFC protocol requirements: HIGH — cited to the governing 2025-06-18 spec + RFC 9728 fetched this session.
- Config field shapes / canonical URI derivation: MEDIUM — discretion areas with open questions (A2, Q1).
- GitHub API endpoint shapes: MEDIUM — stable public API but not re-verified against GitHub docs live (A1).

**Research date:** 2026-07-09
**Valid until:** ~2026-08-09 (30 days; stable subsystem). Re-verify go.mod versions and the MCP spec revision if the phase slips past a SpecGraph dependency bump or an MCP spec revision beyond 2025-06-18.

