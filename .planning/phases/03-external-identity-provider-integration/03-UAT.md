---
status: testing
phase: 03-external-identity-provider-integration
source: [03-VERIFICATION.md]
started: 2026-07-10T00:00:00Z
updated: 2026-07-10T00:00:00Z
---

## Current Test

number: 1
name: Native GitHub OAuth2 + userinfo browser login
expected: |
  A spgr_ws_ session cookie is issued and an oidc_bindings row exists with
  issuer=oauth2:<id> (synthetic) and subject=the GitHub numeric id; the
  web_sessions row carries that same issuer.
awaiting: user response

## Tests

### 1. Native GitHub OAuth2 + userinfo browser login
expected: Register a GitHub OAuth App (callback <base>/api/auth/oidc/callback, scopes read:user + user:email), set SPECGRAPH_GITHUB_CLIENT_SECRET, and log in via the browser through provider start → GitHub consent → callback. A spgr_ws_ session cookie is issued and an oidc_bindings row exists with issuer=oauth2:<id> (synthetic) and subject=the GitHub numeric id; the web_sessions row carries that same issuer.
result: [pending]

### 2. MCP OAuth 2.1 resource-server flow against a live external IdP
expected: Deploy behind https (or set an explicit https mcp_resource_uri), point a standard MCP client with no token at /mcp/, then have it fetch /.well-known/oauth-protected-resource and obtain a resource-bound access token from the configured external IdP. The tokenless /mcp/ request returns 401 + WWW-Authenticate: Bearer resource_metadata="…"; the well-known doc lists all configured issuers; a token whose aud contains the canonical resource URI authenticates while a client_id-only token is rejected; an opaque token validates via the IdP introspection endpoint.
result: [pending]

### 3. Docker-gated session-issuer integration test
expected: Run `go test -tags integration ./internal/server/ -run SessionIssuer`. TestIntegration_SessionIssuer passes — a session minted via the interactive path persists a non-empty web_sessions.issuer matching the authenticating provider, and any pre-existing empty-issuer row is left untouched (no backfill, D-10).
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
