---
phase: 3
slug: external-identity-provider-integration
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-10
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `03-RESEARCH.md` §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (unit + `//go:build integration` testcontainers) + Ginkgo/Gomega (`//go:build e2e`) |
| **Config file** | `Taskfile.yml` (test task definitions); no separate test config |
| **Quick run command** | `task test` (excludes integration + e2e via build tags) |
| **Full suite command** | `task pr-prep` (check → `task test:integration` → `task test:e2e`; requires Docker) |
| **Estimated runtime** | ~30–60 s for `task test`; minutes for `pr-prep` (Docker/testcontainers) |

---

## Sampling Rate

- **After every task commit:** Run `task test` (fast, no Docker) — must stay green.
- **After every plan wave:** Run `task check` then `go test -tags integration ./internal/auth/ ./internal/server/`.
- **Before `/gsd-verify-work`:** `task pr-prep` (full check + integration + e2e) green.
- **Max feedback latency:** ~60 seconds for the per-commit signal.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|--------|
| 03-01-01 | 03-01 | 1 | AUTH-01, AUTH-05 | Fail-closed resolution preserved; `Identity.Issuer` + `ResolveLogin` seam added without changing `Resolve` dispatch (D-08) | unit | `task test` | ⬜ pending |
| 03-01-02 | 03-01 | 1 | AUTH-01 | Shared `materializeIdentity`; `Exchange`→`*OIDCClaims` reshape keeps OIDC path intact | unit | `go test ./internal/auth/ -run 'Resolve\|JIT\|ClaimsMapping\|LoginSync'` | ⬜ pending |
| 03-01-03 | 03-01 | 1 | AUTH-05 | Browser-callback session mint carries verified iss (OIDC) / synthetic issuer (oauth2) | integration | `go test -tags integration ./internal/server/ -run SessionIssuer` | ⬜ pending |
| 03-02-01 | 03-02 | 2 | AUTH-01 | Only verified primary email trusted (D-02); stable numeric subject | unit (httptest) | `go test ./internal/auth/ -run OAuth2Provider` | ⬜ pending |
| 03-02-02 | 03-02 | 2 | AUTH-01 | Missing userinfo URL is startup-fatal; kind-gate relaxed safely | unit | `go test ./internal/auth/ -run 'BuildLoginProviders\|ClaimsMapping\|OAuth2Provider'` | ⬜ pending |
| 03-03-01 | 03-03 | 2 | AUTH-04 | Unauth `/mcp/` → 401 + `WWW-Authenticate`; metadata lists all IdP issuers + canonical resource | unit (httptest) | `go test ./internal/server/ -run 'ProtectedResourceMetadata\|MCPChallenge'` | ⬜ pending |
| 03-03-02 | 03-03 | 2 | AUTH-04 | Canonical resource URI computed once, `https`+no-fragment startup-validated; challenge scoped to `/mcp/` only | build/unit | `task test && go build ./cmd/...` | ⬜ pending |
| 03-04-01 | 03-04 | 3 | AUTH-04 | Token bound only to `client_id` (not resource URI) rejected — RFC 8707 no-passthrough (D-05.3) | unit/integration | `go test ./internal/auth/ -run 'AudienceBinding\|Resolve'` | ⬜ pending |
| 03-04-02 | 03-04 | 3 | AUTH-04 | Opaque token → introspection → active+aud → identity; inactive → 401 (D-06) | unit (stub) | `go test ./internal/auth/ -run 'Introspection\|Resolve'` | ⬜ pending |

*All tasks are `tdd`-tagged (except 03-03-02) and co-create their Wave 0 test files listed below.*

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Requirement → behavior coverage (from research)

| Req | Behavior | Test Type | Command | File |
|-----|----------|-----------|---------|------|
| AUTH-01 | oauth2 provider: code → userinfo → `*OIDCClaims` (subject=id, verified-email fallback) | unit (httptest) | `go test ./internal/auth/ -run OAuth2Provider` | ❌ W0 |
| AUTH-01 | `BuildLoginProviders` constructs oauth2 kind; missing userinfo URL is startup-fatal | unit | `go test ./internal/auth/ -run BuildLoginProviders` | ⚠️ extend |
| AUTH-01 | userinfo → role via `claims_mapping` + default fallback | unit | `go test ./internal/auth/ -run ClaimsMapping` | ✅ add oauth2 case |
| AUTH-01 | JIT: binding miss = new user; 2nd provider binds distinct `(issuer,subject)` (D-03) | integration | `go test -tags integration ./internal/auth/ -run JIT` | ✅ extend |
| AUTH-04 | `/.well-known/oauth-protected-resource` lists all IdP issuers + canonical resource | unit (httptest) | `go test ./internal/server/ -run ProtectedResourceMetadata` | ❌ W0 |
| AUTH-04 | unauth `/mcp/` → 401 + `WWW-Authenticate: Bearer resource_metadata=...` | unit | `go test ./internal/server/ -run MCPChallenge` | ❌ W0 |
| AUTH-04 | wrong-audience token (client_id, not resource URI) → 401 | unit/integration | `go test ./internal/auth/ -run AudienceBinding` | ❌ W0 |
| AUTH-04 | opaque token → introspection → active/aud → identity; inactive → 401 | unit (stub) | `go test ./internal/auth/ -run Introspection` | ❌ W0 |
| AUTH-04 | `spgr_sk_` / `spgr_ws_` resolve unchanged with OAuth enabled (D-08) | unit/integration | `go test ./internal/auth/ -run Resolve` | ✅ add "OAuth enabled" variant |
| AUTH-05 | web login mints session with `issuer` = verified iss (OIDC) / synthetic (oauth2) | integration | `go test -tags integration ./internal/server/ -run SessionIssuer` | ❌ W0 |
| AUTH-05 | no backfill: existing empty-issuer sessions untouched (D-10) | assertion-by-absence | n/a | n/a |

---

## Wave 0 Requirements

- [ ] `internal/auth/oauth2_provider_test.go` — AUTH-01 userinfo/email/subject mapping (httptest stubs)
- [ ] `internal/auth/introspection_test.go` — AUTH-04 D-06 opaque-token introspection
- [ ] `internal/auth/identitystore_audience_test.go` — AUTH-04 D-05.3 audience binding
- [ ] `internal/server/mcp_metadata_test.go` — AUTH-04 metadata + `WWW-Authenticate` challenge
- [ ] Extend `internal/server/identity_integration_test.go` / `identitystore_integration_test.go` — AUTH-05 issuer population
- [ ] Framework install: none — Go test + testcontainers already present

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end GitHub browser login (real IdP redirect) | AUTH-01 | Requires live GitHub OAuth app + browser redirect | Configure a GitHub OAuth app, run `/api/auth/oidc/{github}/start`, complete consent, confirm `spgr_ws_` cookie + `oidc_bindings` row (issuer=synthetic, subject=numeric id) |
| Real MCP client OAuth 2.1 discovery handshake | AUTH-04 | Depends on a spec-compliant external MCP client | Point an MCP client with no token at `/mcp/`; confirm 401 + challenge, discovery of `authorization_servers`, then successful auth with a resource-bound token |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-10 (plan-time; `wave_0_complete` flips true once execution writes the Wave 0 test files)
