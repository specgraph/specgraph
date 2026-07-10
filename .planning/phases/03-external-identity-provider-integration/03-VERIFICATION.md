---
phase: 03-external-identity-provider-integration
verified: 2026-07-10T00:00:00Z
status: human_needed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Register a GitHub OAuth App (callback <base>/api/auth/oidc/callback, scopes read:user + user:email), set SPECGRAPH_GITHUB_CLIENT_SECRET, and log in via the browser through the provider start → GitHub consent → callback."
    expected: "A spgr_ws_ session cookie is issued and an oidc_bindings row exists with issuer=oauth2:<id> (synthetic) and subject=the GitHub numeric id; the web_sessions row carries that same issuer."
    why_human: "The full front-channel OAuth2 round-trip requires a registered GitHub OAuth App + live browser consent against the real GitHub IdP. Unit tests stub /user and /user/emails via httptest, so the real IdP handshake is not exercised by any automated test (SUMMARY 03-02 coverage D6, human_judgment: true)."
  - test: "Deploy behind https (or set an explicit https mcp_resource_uri), point a standard MCP client with no token at /mcp/, then have it fetch /.well-known/oauth-protected-resource and obtain a resource-bound access token from the configured external IdP."
    expected: "The tokenless /mcp/ request returns 401 + WWW-Authenticate: Bearer resource_metadata=\"…\"; the well-known doc lists all configured issuers; a token whose aud contains the canonical resource URI authenticates while a client_id-only token is rejected; an opaque token validates via the IdP introspection endpoint."
    why_human: "Requires an https-terminated deployment (MCP RS is intentionally disabled on the http loopback default) and a real MCP client + live external IdP issuing RFC 8707 resource-bound tokens and a live RFC 7662 introspection endpoint. Unit tests use httptest stubs for the IdP, so the live cross-service handshake is manual UAT (SUMMARY 03-03 D5 / 03-04 D5, human_judgment: true)."
  - test: "Run the Docker-gated integration test: go test -tags integration ./internal/server/ -run SessionIssuer"
    expected: "TestIntegration_SessionIssuer passes — a session minted via the interactive path persists a non-empty web_sessions.issuer matching the authenticating provider, and any pre-existing empty-issuer row is left untouched (no backfill, D-10)."
    why_human: "Postgres integration tests require Docker (testcontainers), which was not available in this verification session. The test exists at internal/server/identity_integration_test.go:169 and the full unit suite (go test ./...) passes green; only the DB-round-trip assertion is Docker-gated."
---

# Phase 3: External Identity Provider Integration — Verification Report

**Phase Goal:** SpecGraph authenticates users and MCP clients against real external identity providers, with enough audit metadata to support session audit and future RP-initiated logout.
**Verified:** 2026-07-10
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | A user can log in via a native GitHub OAuth2 + userinfo flow (no Entra/Okta broker), using the same session model as existing OIDC providers | ✓ VERIFIED (code) — live flow → human | `oauth2LoginProvider` (oauth2_provider.go, 211 lines): `Exchange` swaps code→access token→userinfo GET, stringifies the stable subject, verified-email fallback via `/user/emails`, returns `*OIDCClaims{Issuer=config.ProviderIssuer(pc)}`. `BuildLoginProviders` constructs it for `kind=="oauth2"` (loginprovider.go:130 `buildOAuth2Provider`), flowing through the SAME `materializeIdentity` → oidc_bindings → session model as OIDC. 12 behavioral tests pass (OAuth2Provider*, BuildLoginProviders_OAuth2*, OAuth2ClaimsMapping). Live GitHub round-trip = human UAT (item 1). |
| 2 | An MCP client can authenticate via a standard OAuth 2.1 resource-server flow, token validation delegated to the external IdP rather than a SpecGraph API key | ✓ VERIFIED (code) — live flow → human | RFC 9728 metadata endpoint (`RegisterProtectedResourceMetadata`, mcp_metadata.go:39) + `/mcp/`-scoped `RequireAuthWithChallenge` emitting `WWW-Authenticate: Bearer resource_metadata=…` (mcp_metadata.go:81). RFC 8707 audience binding in `resolveJWT` gated on `MCPRequestFromContext` (identitystore.go:505). RFC 7662 `resolveIntrospection` multi-IdP trial (identitystore.go:582). Explicit `spgr_sk_` guard before introspection (identitystore.go:213). All wired in serve.go (metadata mount + wrapper swap, dev/prod https policy). 10 behavioral tests pass (AudienceBinding*, Introspection*, MCPChallenge*, ProtectedResourceMetadata*). Live MCP client + real IdP = human UAT (item 2). |
| 3 | Every web session record stores which issuer authenticated it (per-session login-provider audit + future RP-initiated logout targeting) | ✓ VERIFIED | `Identity.Issuer` added (auth.go:32). `materializeIdentity` stamps `Issuer: claims.Issuer` on every return (identitystore.go:565,693,709). `handleCallback` resolves via `ResolveLogin(claims)` and sets `Issuer: id.Issuer` on the session literal before `CreateSession` (auth_oidc_handler.go:215,260). `CreateSession` already persists the issuer column. Unit tests TestResolveLogin_ThreadsIssuerOnBindingHit/OnJIT pass; integration test TestIntegration_SessionIssuer exists (Docker-gated, item 3). CLI-exchange path intentionally leaves issuer empty (accepted bounded deferral per D-10/OQ3, ≤1 session TTL). |

**Score:** 3/3 truths verified (code-level). 3 live-external-service / Docker-gated UAT items routed to human verification.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/auth/auth.go` | Identity.Issuer field | ✓ VERIFIED | Field present (:32) with D-09 doc comment |
| `internal/auth/resolver.go` | ResolveLogin on Resolver interface | ✓ VERIFIED | Interface method present (:28), documented interactive-login entrypoint |
| `internal/auth/identitystore.go` | materializeIdentity + ResolveLogin + explicit dispatch + audience check + resolveIntrospection | ✓ VERIFIED | All present & substantive; Resolve dispatch explicit (spgr_ws_/spgr_sk_ guards before introspection) |
| `internal/auth/context.go` | WithMCPRequest / MCPRequestFromContext | ✓ VERIFIED | Both present (:105,:111); TestMCPRequestContext passes |
| `internal/auth/oauth2_provider.go` | oauth2LoginProvider | ✓ VERIFIED | 211 lines; Exchange→userinfo→*OIDCClaims, verified-email fallback |
| `internal/auth/introspection.go` | RFC 7662 client + BuildIntrospectors | ✓ VERIFIED | 224 lines; Introspector, Introspect, NewIntrospector, BuildIntrospectors |
| `internal/auth/oidc_verifier.go` | audienceContains helper | ✓ VERIFIED | string/[]string aud handling, fail-closed (:124) |
| `internal/server/mcp_metadata.go` | RFC 9728 handler + challenge wrapper | ✓ VERIFIED | 107 lines; both functions present, WithMCPRequest marked before authenticate |
| `internal/config/global.go` | oauth2 fields + MCPResourceURI + ValidateMCPResourceURI + ProviderIssuer | ✓ VERIFIED | All present (:311-321,:231,:333,:347) |
| `internal/server/auth_oidc_handler.go` | callback uses ResolveLogin + sets session Issuer | ✓ VERIFIED | :215 ResolveLogin, :260 Issuer: id.Issuer |
| `cmd/specgraph/serve.go` | canonical-URI hoist, ProviderIssuer wiring, conditional RS enable, introspectors | ✓ VERIFIED | Hoisted once above NewIdentityStore; buildClaimsMappingByIssuer + buildMCPAuthorizationServers use ProviderIssuer |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| handleCallback | CreateSession | Exchange→*OIDCClaims → ResolveLogin → materializeIdentity → CreateSession(Issuer: id.Issuer) | ✓ WIRED (auth_oidc_handler.go:208-262) |
| resolveJWT | materializeIdentity | shared tail; JWT + login paths behavior-identical; single interactive derivation | ✓ WIRED (identitystore.go:513) |
| BuildLoginProviders (oauth2) | oidc_bindings | buildOAuth2Provider → issuerID=ProviderIssuer(pc) → materializeIdentity → (issuer,subject) binding | ✓ WIRED |
| serve.go mux | RegisterProtectedResourceMetadata + RequireAuthWithChallenge | conditional on mcpRSEnabled; /mcp/ only, /api/* keeps RequireAuth | ✓ WIRED (serve.go:306-315) |
| ProviderIssuer | authorization_servers + buildClaimsMappingByIssuer + provider claims.Issuer + session issuer | single canonical helper (review HIGH #1) | ✓ WIRED (serve.go:850,867; loginprovider.go:235) |
| Resolve dispatch | spgr_sk_ never introspected | explicit HasPrefix(apiKeyPrefix) guard BEFORE introspection branch | ✓ WIRED (identitystore.go:213 before :218) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Package build | `go build ./...` | exit 0 | ✓ PASS |
| Full unit suite | `go test ./...` | all packages ok, exit 0 | ✓ PASS |
| Auth phase behaviors | `go test ./internal/auth/ -run 'AudienceBinding\|Introspection\|OAuth2Provider\|BuildLoginProviders\|ClaimsMapping\|Resolve\|LoginSync\|JIT' -v` | 80+ tests, all PASS | ✓ PASS |
| MCP-marked JWT accepted only when aud contains resource URI | TestAudienceBinding_MCPMarked_ResourceBoundAccepted/ClientIDOnlyRejected | PASS | ✓ PASS |
| Non-MCP ConnectRPC JWT (client_id aud) still resolves | TestAudienceBinding_NonMCPMarked_ClientIDOnlyResolves | PASS | ✓ PASS |
| spgr_sk_ never POSTed to introspection endpoint | TestIntrospection_APIKeyNeverIntrospected (stub counter zero) | PASS | ✓ PASS |
| Introspection fail-closed algebra (inactive→401, 5xx→transient, wrong-aud→401) | TestIntrospection_Inactive_Rejected/ServerError_Transient/WrongAudience_Rejected | PASS | ✓ PASS |
| Issuer threaded on binding-hit and JIT paths | TestResolveLogin_ThreadsIssuerOnBindingHit/OnJIT | PASS | ✓ PASS |
| SessionIssuer DB persistence | `go test -tags integration ./internal/server/ -run SessionIssuer` | not run (Docker unavailable) | ? SKIP → human item 3 |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| AUTH-01 | 03-01, 03-02 | Native generic OAuth2 + userinfo login provider (GitHub-direct) | ✓ SATISFIED | oauth2LoginProvider built + wired through shared session model; unit-covered |
| AUTH-04 | 03-03, 03-04 | MCP OAuth 2.1 resource server delegating auth to a real IdP | ✓ SATISFIED | RFC 9728 metadata + WWW-Authenticate + RFC 8707 audience + RFC 7662 introspection; unit-covered |
| AUTH-05 | 03-01 | Populate web_sessions.issuer for audit / future RP-logout | ✓ SATISFIED | Issuer threaded through callback→CreateSession; unit + Docker-gated integration test |

All three requirement IDs (AUTH-01, AUTH-04, AUTH-05) are declared in plan frontmatter AND mapped to Phase 3 in REQUIREMENTS.md (lines 111,114,115). No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | No TBD/FIXME/XXX/PLACEHOLDER/stub markers in any phase-modified file | — | None |

The only deferred item (deferred-items.md) is pre-existing dprint formatting drift in unrelated `.planning/intel/classifications/*.json` — not Go source, not related to the phase goal; an accepted scope boundary, not a gap.

### Human Verification Required

3 items — all inherent live-external-service or Docker-gated verification (see frontmatter `human_verification` for full detail):

1. **Live GitHub OAuth2 browser login** — register a GitHub OAuth App, log in end-to-end, confirm spgr_ws_ cookie + oidc_bindings row (synthetic issuer, numeric subject). The mechanism is fully unit-covered with httptest stubs; only the real-IdP round-trip is manual.
2. **Live MCP client OAuth 2.1 handshake** — https deployment, standard MCP client discovers the AS via the well-known doc, obtains a resource-bound token from the external IdP, authenticates through /mcp/. Unit-covered with stubs; the live cross-service handshake is manual.
3. **Docker-gated SessionIssuer integration test** — `go test -tags integration ./internal/server/ -run SessionIssuer` (test exists at identity_integration_test.go:169; not run this session because Docker was unavailable).

### Gaps Summary

No blocking gaps. All three ROADMAP success criteria are achieved at the code level: the native OAuth2/userinfo login provider, the MCP OAuth 2.1 resource-server surface (RFC 9728 + 8707 + 7662), and web-session issuer population are all implemented, substantive, wired end-to-end through the shared identity-materialization seam, and covered by a passing unit/behavioral suite (80+ auth tests green, full `go test ./...` green). The static-credential invariants (spgr_sk_/spgr_ws_ additive, D-08) and the review-HIGH mitigations (single ProviderIssuer canonical helper #1, path-scoped MCP audience check #2, explicit API-key dispatch guard #3) are all present and test-verified.

Status is `human_needed` (not `passed`) solely because the phase goal — authenticating against **real external** identity providers — has three verification items that are inherently manual: two live external-service round-trips (real GitHub OAuth App; real MCP client + live IdP token/introspection) and one Docker-gated integration test. These were correctly pre-flagged as `human_judgment: true` in the plan SUMMARYs (03-02 D6, 03-03 D5, 03-04 D5). No code deficiency was found.

---

_Verified: 2026-07-10_
_Verifier: the agent (gsd-verifier)_
