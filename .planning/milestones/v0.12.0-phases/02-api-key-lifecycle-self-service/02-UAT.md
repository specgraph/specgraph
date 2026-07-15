---
status: complete
phase: 02-api-key-lifecycle-self-service
source: [02-VERIFICATION.md]
started: 2026-07-10T02:35:00Z
updated: 2026-07-10T02:40:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Self-provision flow — create/rotate/revoke + one-time reveal
expected: Start the SpecGraph server (`task build && ./specgraph serve`) + web dev build (`pnpm -C web dev`), log in via OIDC/`spgr_ws_` session, open `/keys`, then create → rotate → revoke a key. The dashboard lists the caller's own keys; create/rotate open the reveal modal showing the plaintext exactly once; after closing the modal the secret is unrecoverable (no re-fetch); revoke removes the active key and the list refreshes.
result: pass

### 2. CSRF double-submit enforcement
expected: On the live `/keys` page, strip/blank the `specgraph_csrf` cookie (or the echoed `X-CSRF-Token` header) and attempt a create/rotate/revoke mutation. The mutation is rejected with HTTP 403 (invalid or missing CSRF token); a normal mutation with the cookie present succeeds.
result: pass

### 3. Anti-key-chaining message on a Source=="apikey" session
expected: Log in with a session whose Source is an API key (legacy `SPECGRAPH_API_KEY`-style session) and attempt a self-mint from the dashboard. The anti-key-chaining gate denies the mint and the panel renders a readable "sign in to provision a key" message rather than a raw error.
result: pass

## Summary

total: 3
passed: 3
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
