---
phase: 03-external-identity-provider-integration
plan: 03
subsystem: auth
tags: [mcp, oauth2, rfc9728, rfc8707, www-authenticate, resource-server, oidc]

# Dependency graph
requires:
  - phase: 03-external-identity-provider-integration
    provides: "Plan 01 seam — OIDCConfig.MCPResourceURI + ValidateMCPResourceURI, config.ProviderIssuer canonical-issuer helper, auth.WithMCPRequest per-request marker, exported token-resolution path"
  - phase: 03-external-identity-provider-integration
    provides: "Plan 02 — oauth2 provider stamping claims.Issuer from config.ProviderIssuer (runtime side of the HIGH #1 key alignment)"
provides:
  - "RFC 9728 Protected Resource Metadata endpoint (GET /.well-known/oauth-protected-resource) — public, advertises canonical resource URI + authorization_servers"
  - "server.RequireAuthWithChallenge — /mcp/-scoped auth wrapper emitting WWW-Authenticate: Bearer resource_metadata=... on 401 and marking auth.WithMCPRequest before authenticate"
  - "auth.Authenticate — exported bearer/session token-resolution path (reused by the challenge wrapper; RequireAuth untouched)"
  - "serve.go canonical-URI hoist + mcpRSEnabled dev/prod policy, ProviderIssuer-keyed authorization_servers + buildClaimsMappingByIssuer, conditional RS enable"
affects: [03-04-introspection-audience]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pattern 3 (RESEARCH): OAuth 2.1 resource-server discovery surface — RFC 9728 metadata + WWW-Authenticate challenge, path-scoped to /mcp/ so static-credential /api/* clients keep a bare 401 (D-08)"
    - "Single canonical resource URI hoisted once above NewIdentityStore and reused by metadata mount + Plan 04 audience config (no double derivation)"
    - "Exported auth.Authenticate wraps the unexported authenticate so the server-package challenge wrapper reuses the exact token path instead of duplicating extraction (RequireAuth in middleware.go untouched)"
    - "config.ProviderIssuer drives BOTH authorization_servers advertisement AND the startup claims-mapping key, so the advertised AS equals the binding issuer (review HIGH #1)"
    - "Dev/prod URI enablement policy: explicit non-https mcp_resource_uri is startup-fatal; a defaulted http loopback URI disables RS with a warn rather than aborting local dev"

key-files:
  created:
    - internal/server/mcp_metadata.go
    - internal/server/mcp_metadata_test.go
  modified:
    - internal/auth/interceptor.go
    - cmd/specgraph/serve.go

key-decisions:
  - "Exported auth.Authenticate (new, in interceptor.go) instead of duplicating bearer/cookie extraction in the server package — keeps the bare-401 and challenge-401 paths byte-identical; middleware.go RequireAuth left untouched (deviation Rule 3)"
  - "RED+GREEN folded into one buildable commit per task (pre-commit/pre-push gates require a compiling tree; matches Plan 01/02 discipline; config tdd_mode is false so no gate enforcement)"
  - "Challenge string built with fmt.Sprintf(\"Bearer resource_metadata=%q\", url) so the RFC-required double-quoting is exact"
  - "authorization_servers de-duplicates on config.ProviderIssuer and only advertises Interactive providers (D-07)"
  - "The WithMCPRequest-marked context is threaded into the downstream WithIdentity so the /mcp/ marker survives to Plan 04's resolveJWT"

patterns-established:
  - "Public discovery endpoints (metadata) mount like /api/auth/oidc/providers — no RequireAuth; only issuer URLs already known to clients are exposed"

requirements-completed: [AUTH-04]

# Coverage metadata
coverage:
  - id: D1
    description: "GET /.well-known/oauth-protected-resource returns RFC 9728 metadata JSON whose resource == the canonical MCP resource URI and authorization_servers == config.ProviderIssuer of every configured interactive/OAuth IdP; bearer_methods_supported=[header]; non-GET → 405; public/unauthenticated"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/server/mcp_metadata_test.go#TestProtectedResourceMetadata"
        status: pass
      - kind: unit
        ref: "internal/server/mcp_metadata_test.go#TestProtectedResourceMetadata_NonGET"
        status: pass
    human_judgment: false
  - id: D2
    description: "An unauthenticated /mcp/ request returns 401 with WWW-Authenticate: Bearer resource_metadata=\"<...>/.well-known/oauth-protected-resource\"; the wrapper marks auth.WithMCPRequest before authenticating (HIGH #2) and the success path sets identity; ErrTransient → 503 with no challenge; scoped to /mcp/ only (D-08)"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/server/mcp_metadata_test.go#TestMCPChallenge_Unauthenticated"
        status: pass
      - kind: unit
        ref: "internal/server/mcp_metadata_test.go#TestMCPChallenge_Authenticated"
        status: pass
      - kind: unit
        ref: "internal/server/mcp_metadata_test.go#TestMCPChallenge_Transient"
        status: pass
    human_judgment: false
  - id: D3
    description: "buildClaimsMappingByIssuer (serve.go) keys the startup claims-mapping by config.ProviderIssuer(pc), matching the runtime claims.Issuer so a synthetic-issuer oauth2 provider's claims_mapping resolves a role (review HIGH #1, D-04)"
    requirement: "AUTH-04"
    verification:
      - kind: other
        ref: "grep -n 'out[config.ProviderIssuer(pc)]' cmd/specgraph/serve.go (no raw pc.Issuer keying remains)"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestOAuth2ClaimsMapping_KeyedByProviderIssuer (Plan 02 — proves the runtime side of the shared ProviderIssuer key)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Canonical resource URI hoisted ONCE above NewIdentityStore and reused by the metadata mount + Plan 04 audience config; explicit non-https/fragment mcp_resource_uri is startup-fatal; a defaulted http loopback URI disables RS (warn, no abort) — dev/prod URI policy"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/config/oidc_issuer_test.go#TestValidateMCPResourceURI (Plan 01 — https+no-fragment enforcement the boot policy gates on)"
        status: pass
      - kind: other
        ref: "go build ./cmd/... + grep: single mcpResourceURI/mcpRSEnabled derivation above serve.go NewIdentityStore, no second <base_url>/mcp near the /mcp/ mount"
        status: pass
    human_judgment: false
  - id: D5
    description: "End-to-end MCP discovery handshake against a live https deployment: a real MCP client with no token receives 401 + WWW-Authenticate challenge and, fetching the well-known doc, sees all configured issuers"
    verification: []
    human_judgment: true
    rationale: "Requires a live https-terminated deployment (RS is disabled on the http loopback default) and a real MCP client performing the discovery round-trip; no unit/integration test asserts the full external handshake this plan — the handler + wrapper are unit-covered with httptest, but the client-side auto-discovery is manual UAT."

# Metrics
duration: ~15 min
completed: 2026-07-10
status: complete
---

# Phase 3 Plan 03: RFC 9728 Protected-Resource Metadata + WWW-Authenticate Challenge Summary

**The AUTH-04 OAuth 2.1 resource-server discovery surface: a public RFC 9728 metadata endpoint advertising the canonical resource URI + `config.ProviderIssuer` authorization servers, and a `/mcp/`-scoped `RequireAuthWithChallenge` wrapper that emits `WWW-Authenticate: Bearer resource_metadata=…` on a 401 and marks `auth.WithMCPRequest` — with the canonical URI hoisted once, a dev/prod enablement policy, and `buildClaimsMappingByIssuer` re-keyed by `ProviderIssuer` so the advertised AS equals the binding issuer.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-07-10T09:15Z (approx)
- **Completed:** 2026-07-10T09:22Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- New `internal/server/mcp_metadata.go`: `protectedResourceMetadata` (RFC 9728 §3.2 JSON) + `RegisterProtectedResourceMetadata` mounting the public `GET /.well-known/oauth-protected-resource` (405 on non-GET, `bearer_methods_supported=[header]`, `Cache-Control` max-age).
- `RequireAuthWithChallenge(resolver, metadataURL)`: mirrors `auth.RequireAuth`'s authenticate/error mapping but (a) marks the context with `auth.WithMCPRequest` **before** authenticating so Plan 04's resource-URI audience check is path-scoped to `/mcp/` (review HIGH #2), and (b) sets `WWW-Authenticate: Bearer resource_metadata="<url>"` alongside the 401 (D-05.2). `ErrTransient` → 503 with no challenge; the challenge + marker never appear on `RequireAuth`/ConnectRPC (D-08).
- Exported `auth.Authenticate` wrapping the unexported `authenticate` so the server-package wrapper reuses the exact bearer/session token path — `internal/auth/middleware.go` `RequireAuth` is byte-for-byte unchanged.
- `serve.go`: hoisted the canonical `mcpResourceURI` + `mcpRSEnabled` computation ONCE above `NewIdentityStore` (reused verbatim by Plan 04); explicit `mcp_resource_uri` is startup-fatal unless https+no-fragment, a defaulted http loopback URI disables RS with a warn (dev/prod policy); advertises `authorization_servers` via new `buildMCPAuthorizationServers` (de-duped `ProviderIssuer` of each interactive provider); conditionally swaps `/mcp/` to `RequireAuthWithChallenge` when enabled, keeping `auth.RequireAuth` otherwise.
- Re-keyed `buildClaimsMappingByIssuer` by `config.ProviderIssuer(pc)` (was raw `pc.Issuer`) so the startup map key equals the runtime `claims.Issuer` for oauth2 providers (review HIGH #1, D-04) — the serve.go edit Plan 02 deferred here (single owner within Wave 2). Commented the loopback `mcpClient` hop as internal same-audience, not a passthrough (A3).

## Task Commits

Each task committed atomically (RED+GREEN folded per task for a buildable tree; DCO-signed):

1. **Task 1: RFC 9728 metadata handler + WWW-Authenticate challenge wrapper** - `ed823257` (feat)
2. **Task 2: mount metadata + swap the /mcp/ auth wrapper in serve.go** - `c7d107bd` (feat)

**Plan metadata:** _(this SUMMARY commit)_

## Files Created/Modified
- `internal/server/mcp_metadata.go` - `protectedResourceMetadata` struct, `RegisterProtectedResourceMetadata`, `RequireAuthWithChallenge`, `protectedResourceMetadataPath`
- `internal/server/mcp_metadata_test.go` - metadata (resource==canonical, issuers, 405) + challenge (401+header, success sets identity+MCP marker, transient→503) coverage
- `internal/auth/interceptor.go` - exported `Authenticate` wrapper over the unexported `authenticate` (middleware.go untouched)
- `cmd/specgraph/serve.go` - canonical-URI hoist + `mcpRSEnabled` policy, `buildClaimsMappingByIssuer` ProviderIssuer re-key, `buildMCPAuthorizationServers`, conditional metadata mount + `/mcp/` wrapper swap, A3 comment

## Decisions Made
- **Exported `auth.Authenticate`** rather than re-implementing bearer/cookie extraction in the server package. The challenge wrapper lives in `internal/server` (per plan) but needs the same token path as `RequireAuth`; the extraction helpers are unexported. A thin exported wrapper keeps the two 401 paths identical and leaves `middleware.go` untouched (acceptance criterion met). Tracked as deviation Rule 3.
- **RED+GREEN folded per task** into one buildable commit — the repo's pre-commit (golangci-lint + build) and pre-push (`task check`) gates reject a non-compiling RED-only commit; `tdd_mode` is `false` in config so no gate enforcement. Task 1's genuine RED (test written first, `go test` compile-failed) was confirmed before implementing.
- **`authorization_servers`** advertises only `Interactive` providers, de-duplicated on `ProviderIssuer` (D-07); both it and the claims-mapping key call the SAME helper so they cannot diverge (HIGH #1).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Exported `auth.Authenticate` to give the server-package wrapper the exact token path**
- **Found during:** Task 1 (RequireAuthWithChallenge implementation)
- **Issue:** `RequireAuthWithChallenge` must "duplicate RequireAuth's authenticate/error mapping", but `authenticate`, `extractBearerToken`, and `sessionCookieValue` are all unexported in `internal/auth`, and the wrapper lives in `internal/server`. Re-implementing bearer + `specgraph_session` cookie extraction in the server package would fork the credential-extraction logic and risk divergence between the bare-401 and challenge-401 paths.
- **Fix:** Added a one-line exported `Authenticate(ctx, resolver, headers)` in `internal/auth/interceptor.go` that calls the existing unexported `authenticate`. The wrapper reuses it; `internal/auth/middleware.go` `RequireAuth` is unchanged (`git diff --stat` empty), satisfying the acceptance criterion.
- **Files modified:** internal/auth/interceptor.go (not in the plan's files_modified list — additive export only)
- **Verification:** `go build ./...`, `go test ./internal/server/ -run 'ProtectedResourceMetadata|MCPChallenge'`, and `golangci-lint run ./internal/auth/ ./internal/server/` all green; `git diff --stat internal/auth/middleware.go` shows no change.
- **Committed in:** ed823257 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — additive export to avoid duplicating the credential path). **Impact on plan:** No scope change; all planned artifacts delivered. The export is the minimal seam that keeps `RequireAuth` untouched while the /mcp/ wrapper authenticates identically.

## Issues Encountered
None — all planned work completed. `task check` still aborts at `fmt:check` (dprint) and `lint:markdown` on the same pre-existing `.planning/intel/classifications/*.json` (138 files) and `.planning/phases/**/*.md` drift that Plans 01 and 02 already logged to `deferred-items.md`; none are Go source and none are files this plan touched. All Go gates are green independently: `gofmt -l` clean on the four touched files, `task lint:go` reports 0 issues repo-wide, `task license:check` passes, `task build` passes, and `task test` (build + fast unit suite) exits 0.

## User Setup Required
None — no external service configuration required for this plan. The MCP OAuth resource server is disabled on the http loopback default; enabling the discovery surface for real MCP clients requires an https-terminated deployment (or an explicit https `mcp_resource_uri`), which is the D5 manual-UAT precondition, not a code setup step.

## Next Phase Readiness
- Plan 04 (introspection + audience check) consumes the hoisted `mcpResourceURI`/`mcpRSEnabled` pair verbatim for `NewIdentityStore`'s audience config, and reads `auth.MCPRequestFromContext` (set by this plan's `/mcp/` wrapper) to path-scope the resource-URI audience assertion. Both the advertised `authorization_servers` and the claims-mapping key now agree with the binding issuer via `config.ProviderIssuer`.
- `task test` green; no blockers.
- Deferred (out of scope): the repo-wide `.planning/` dprint + markdown-lint drift (already logged); end-to-end MCP client discovery UAT (D5, `human_judgment: true`).

---
*Phase: 03-external-identity-provider-integration*
*Completed: 2026-07-10*

## Self-Check: PASSED
- Created files `internal/server/mcp_metadata.go` and `internal/server/mcp_metadata_test.go` exist on disk.
- Commits `ed823257` and `c7d107bd` present in git log with DCO sign-off.
- All task acceptance-criteria greps pass; `go build ./cmd/...`, `task test`, `task lint:go`, and `task license:check` green; `RequireAuth` (middleware.go) unchanged.
