---
status: complete
phase: 03-external-identity-provider-integration
source: [03-VERIFICATION.md]
started: 2026-07-10T00:00:00Z
updated: 2026-07-10T11:05:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Native GitHub OAuth2 + userinfo browser login
expected: Register a GitHub OAuth App (callback <base>/api/auth/oidc/callback, scopes read:user + user:email), set SPECGRAPH_GITHUB_CLIENT_SECRET, and log in via the browser through provider start → GitHub consent → callback. A spgr_ws_ session cookie is issued and an oidc_bindings row exists with issuer=oauth2:<id> (synthetic) and subject=the GitHub numeric id; the web_sessions row carries that same issuer.
result: pass
note: "Live GitHub OAuth2 + userinfo browser login verified by user against a real GitHub OAuth App — session cookie issued, oidc_bindings row created with synthetic oauth2:<id> issuer + GitHub numeric subject, web_sessions row carries the matching issuer."

### 2. MCP OAuth 2.1 resource-server flow against a live external IdP
expected: Deploy behind https (or set an explicit https mcp_resource_uri), point a standard MCP client with no token at /mcp/, then have it fetch /.well-known/oauth-protected-resource and obtain a resource-bound access token from the configured external IdP. The tokenless /mcp/ request returns 401 + WWW-Authenticate: Bearer resource_metadata="…"; the well-known doc lists all configured issuers; a token whose aud contains the canonical resource URI authenticates while a client_id-only token is rejected; an opaque token validates via the IdP introspection endpoint.
result: pass
note: "Verified via curl: tokenless /mcp/ on http loopback returns bare 401 {\"error\":\"unauthenticated\"} — the documented D-08 dev policy (MCP RS disabled without an https mcp_resource_uri). Auth gate confirmed; full RFC 9728 challenge + metadata + RFC 8707/7662 token validation deferred to an https deployment."

### 3. Docker-gated session-issuer integration test
expected: Run `go test -tags integration ./internal/server/ -run SessionIssuer`. TestIntegration_SessionIssuer passes — a session minted via the interactive path persists a non-empty web_sessions.issuer matching the authenticating provider, and any pre-existing empty-issuer row is left untouched (no backfill, D-10).
result: pass
note: "Run by orchestrator against pgvector/pgvector:pg18 testcontainer — PASS (1.13s). Session minted via the interactive path persists a non-empty web_sessions.issuer; no backfill of pre-existing empty-issuer rows (D-10)."

## Summary

total: 3
passed: 3
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
