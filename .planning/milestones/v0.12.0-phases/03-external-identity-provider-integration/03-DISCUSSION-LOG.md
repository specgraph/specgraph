# Phase 3: External Identity Provider Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 3-External Identity Provider Integration
**Areas discussed:** GitHub provider shape (AUTH-01), GitHub role mapping / JIT (AUTH-01), MCP OAuth 2.1 RS scope (AUTH-04), Issuer semantics + backfill (AUTH-05), API-key + bearer coexistence (AUTH-04)

---

## GitHub provider shape (AUTH-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Generic oauth2 kind | New config-driven kind (authorize/token/userinfo URLs + field mappings); GitHub as documented config | ✓ |
| GitHub-specific preset | kind=="github" with URLs/endpoints/extraction hardcoded | |
| Generic + github preset sugar | Generic kind plus a github preset filling defaults | |

**User's choice:** Generic oauth2 kind
**Notes:** Matches roadmap wording "native generic OAuth2 + userinfo login provider (GitHub-direct)"; keeps a second non-OIDC IdP cheap.

### Subject + email extraction (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Numeric id subject + verified email fetch | Subject = stable numeric id; email = verified primary via /user/emails + user:email | ✓ |
| id subject, single userinfo call only | No secondary email call; blank email when private | |
| Username as subject | Human-readable but breaks on rename | |

**User's choice:** Numeric-id subject + verified email fetch — **and added:** "let's plan on supporting multiple auth provider mapping (github + google + okta + entraid → same user)".
**Notes:** Scouting confirmed `oidc_bindings` is already `UNIQUE (issuer, subject)` with FK to `users(id)` — N bindings per user is a data-layer given. Framed the real decision as the *linking* step below.

### Multi-provider linking scope

| Option | Description | Selected |
|--------|-------------|----------|
| Design-for now, link later | Store bindings the same way (N-per-user possible); keep JIT binding-miss = new user; linking flow is a fast-follow | ✓ |
| Verified-email auto-link now | JIT matches existing user by verified email and attaches binding | |
| Explicit link-account UX now | Logged-in user links a second provider from the dashboard | |

**User's choice:** Design-for now, link later
**Notes:** Mirrors AUTH-02's "ship the path, leave the seam ready" pattern.

---

## GitHub role mapping / JIT (AUTH-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Userinfo-as-claims + default fallback | Feed userinfo JSON into the existing per-provider claims_mapping; fall back to JIT default role | ✓ |
| Default role only for oauth2 | GitHub logins always get the JIT default role | |
| Org/team membership mapping now | Fetch org/team membership (read:org) and map teams→roles | |

**User's choice:** Userinfo-as-claims + default fallback
**Notes:** One role model for all provider kinds; org/team fetch deferred.

---

## MCP OAuth 2.1 RS scope (AUTH-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Metadata + challenge layer, reuse resolveJWT | RFC 9728 metadata + WWW-Authenticate + reuse existing validation | |
| Also add token introspection | Above plus RFC 7662 for opaque tokens | |
| SpecGraph as AS/DCR proxy | SpecGraph issues/registers tokens | |

**User's choice (initial):** "do the research on what MCP servers should be doing _today_ based on current specs/rfcs."
**Research performed:** Fetched the MCP Authorization spec rev 2025-06-18. RS-side MUSTs: RFC 9728 protected-resource-metadata + WWW-Authenticate 401, OAuth 2.1 §5.2 validation, **RFC 8707 audience/resource binding** (reject tokens not issued for this server), no token passthrough, 401/403/400 errors. AS/DCR delegated to the external IdP. Introspection (RFC 7662) only needed for opaque tokens.

**User's choice (after research):** "2 + allow for static tokens to still work, even if OIDC/OAuth enabled" — i.e. **full RS MUSTs + introspection for opaque tokens + additive static-key coexistence**.
**Notes:** The one code gap: `resolveJWT` currently checks audience against client_id, but the spec requires audience = the MCP server's canonical resource URI.

### Multi-AS metadata (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Advertise all configured IdPs | List every configured IdP issuer in authorization_servers; client selects per RFC 9728 §7.6 | ✓ |
| Single designated AS | One "MCP authorization server" config field | |

**User's choice:** Advertise all configured IdPs

---

## API-key + bearer coexistence (AUTH-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Additive, all credential types concurrent | spgr_sk_/spgr_ws_/JWT/opaque all accepted; zero behavior change to keys | ✓ |
| Additive but mark keys legacy | Same, but keys emit a deprecation signal | |

**User's choice:** Additive, all credential types concurrent
**Notes:** Prompted directly by the user's requirement that static tokens keep working when OAuth is enabled.

---

## Issuer semantics + backfill (AUTH-05)

| Option | Description | Selected |
|--------|-------------|----------|
| Verified iss for OIDC, provider-id for oauth2 | OIDC → verified iss claim; oauth2/GitHub → stable synthetic issuer (provider id) | ✓ |
| Always store provider ID | Uniform but loses canonical issuer URL RP-logout needs | |

**User's choice:** Verified iss for OIDC, provider-id for oauth2

### Backfill (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| No backfill, natural expiry | Empty-issuer sessions age out within 12h TTL; new sessions carry issuer | ✓ |
| Best-effort backfill existing rows | Derive issuer for existing rows from binding lookup | |

**User's choice:** No backfill, natural expiry

---

## the agent's Discretion

- Exact `oauth2` config field names/shape (URL fields, subject/email selectors, userinfo field-selector expression).
- `LoginProvider` interface reconciliation for the non-OIDC path (Exchange returns userinfo vs raw id_token).
- Audience/resource-URI reconciliation in `resolveJWT` and how the canonical MCP resource URI is derived/configured.
- Introspection endpoint config + caching/rate-limiting for the opaque-token path.
- Rate-limiting and audit-log line shape for the new login and RS paths.

## Deferred Ideas

- Cross-provider account linking flow (verified-email auto-link or explicit link-account UX) — data model ships this phase, flow is a fast-follow.
- GitHub org/team-membership → role mapping (read:org scope, extra API calls).
- RP-initiated logout flow (AUTH-05 stores the issuer to enable it later).
- SpecGraph as its own authorization server / DCR proxy — rejected (contradicts delegating to a real IdP).
- Deprecating static API keys — rejected in favor of fully additive coexistence.
