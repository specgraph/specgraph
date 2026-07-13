---
phase: 03-external-identity-provider-integration
plan: 04
subsystem: auth
tags: [oidc, oauth2, rfc8707, rfc7662, introspection, audience, mcp, jwt]

# Dependency graph
requires:
  - phase: 03-external-identity-provider-integration
    provides: "Plan 01 seam — materializeIdentity, OIDCConfig.MCPResourceURI, auth.WithMCPRequest/MCPRequestFromContext marker, config.ProviderIssuer, OIDCProviderConfig.IntrospectionURL"
  - phase: 03-external-identity-provider-integration
    provides: "Plan 03 — the SINGLE hoisted mcpResourceURI/mcpRSEnabled above NewIdentityStore, and the /mcp/ challenge wrapper that sets auth.WithMCPRequest"
provides:
  - "RFC 8707 resource-URI audience assertion on the bearer-JWT path, path-scoped to /mcp/ via MCPRequestFromContext (D-05.3)"
  - "auth.audienceContains helper (string|[]string aud claim)"
  - "RFC 7662 introspection: auth.Introspector (bounded client + active-result cache), IntrospectionResult, BuildIntrospectors, and resolveIntrospection multi-IdP trial selection (D-06)"
  - "Explicit spgr_sk_ apiKeyPrefix dispatch guard in Resolve before the introspection branch (HIGH #3, D-08)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Post-verify additive audience assertion (no second verifier) — web-login aud==client_id semantics untouched; the RFC 8707 resource-URI check reads already-verified claims.Raw and fires ONLY on the WithMCPRequest-marked /mcp/ path (OQ2, HIGH #2)"
    - "Explicit prefix dispatch: spgr_ws_/spgr_sk_ guards run BEFORE introspection so static credentials never reach an external IdP (HIGH #3, D-08)"
    - "Introspection fail-closed error algebra: decisive inactive/wrong-aud -> ErrUnauthenticated; all-non-decisive (5xx/timeout/rate-limited) -> ErrTransient"
    - "Bounded introspection: per-request client timeout + active-result cache to min(exp, TTL cap) + per-issuer rate limiter (reuses rateLimiterFor buckets)"

key-files:
  created:
    - internal/auth/introspection.go
    - internal/auth/introspection_test.go
    - internal/auth/identitystore_audience_test.go
  modified:
    - internal/auth/oidc_verifier.go
    - internal/auth/identitystore.go
    - internal/auth/usersbackend_stub_test.go
    - cmd/specgraph/serve.go

key-decisions:
  - "Exported the introspection types (Introspector, IntrospectionResult, NewIntrospector) — the plan named them lowercase, but serve.go (package main) must construct them and revive forbids an exported method returning an unexported type"
  - "Added an exported BuildIntrospectors(providers) helper so client-secret resolution stays centralized in the auth package (reuses the unexported resolveClientSecret), mirroring BuildLoginProviders"
  - "Introspect return contract encodes decisiveness: (active,nil) | (inactive,nil for 2xx active==false or 4xx) | (nil,err for 5xx/network/decode) so the resolver can distinguish ErrUnauthenticated from ErrTransient"
  - "A 4xx from the introspection endpoint is treated as a decisive inactive (fail-closed), a 5xx as non-decisive/retryable"

patterns-established:
  - "Trust-boundary outbound client (introspection) uses operator-config endpoint (never token-derived), bounded timeout, and per-issuer rate limiting — the SSRF/DoS mitigation shape for future IdP round-trips"

requirements-completed: [AUTH-04]

# Coverage metadata
coverage:
  - id: D1
    description: "An MCP-path bearer JWT (WithMCPRequest) is accepted only when its aud contains the canonical MCP resource URI; a client_id-only token is rejected (RFC 8707, D-05.3)"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_audience_test.go#TestAudienceBinding_MCPMarked_ResourceBoundAccepted"
        status: pass
      - kind: unit
        ref: "internal/auth/identitystore_audience_test.go#TestAudienceBinding_MCPMarked_ClientIDOnlyRejected"
        status: pass
    human_judgment: false
  - id: D2
    description: "A non-MCP ConnectRPC JWT (aud=client_id, no WithMCPRequest) still resolves when MCP RS is enabled — the resource-URI assertion is path-scoped (review HIGH #2, D-08); with no resource URI configured the check is additive"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/auth/identitystore_audience_test.go#TestAudienceBinding_NonMCPMarked_ClientIDOnlyResolves"
        status: pass
      - kind: unit
        ref: "internal/auth/identitystore_audience_test.go#TestAudienceBinding_EmptyConfig_Additive"
        status: pass
    human_judgment: false
  - id: D3
    description: "Opaque (non-JWT) bearers validate via bounded RFC 7662 introspection: active+resource-aud -> identity, inactive/wrong-aud -> ErrUnauthenticated, IdP 5xx -> ErrTransient, multi-introspector first-match wins (D-06)"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_ActiveResourceBound_Resolves"
        status: pass
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_Inactive_Rejected"
        status: pass
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_ServerError_Transient"
        status: pass
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_WrongAudience_Rejected"
        status: pass
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_MultiIntrospector_FirstMatchWins"
        status: pass
    human_judgment: false
  - id: D4
    description: "An spgr_sk_ API key is routed to resolveAPIKey by the explicit prefix guard BEFORE the introspection branch, so it is never POSTed to the external IdP even when introspectors are configured (review HIGH #3, D-08, T-03-04-03/04)"
    requirement: "AUTH-04"
    verification:
      - kind: unit
        ref: "internal/auth/introspection_test.go#TestIntrospection_APIKeyNeverIntrospected"
        status: pass
    human_judgment: false
  - id: D5
    description: "End-to-end delegated-auth against a live external IdP: a real MCP client presenting an IdP-issued resource-bound access token (and an opaque token introspected against a live RFC 7662 endpoint) authenticates through the /mcp/ boundary"
    verification: []
    human_judgment: true
    rationale: "Requires an https-terminated deployment with a real external IdP issuing RFC 8707 resource-audience tokens and a live introspection endpoint; unit coverage uses httptest stubs for the IdP, so the live cross-service handshake is manual UAT."

# Metrics
duration: ~25 min
completed: 2026-07-10
status: complete
---

# Phase 3 Plan 04: RFC 8707 Audience Binding + RFC 7662 Introspection Summary

**The AUTH-04 token-validation half: an additive RFC 8707 resource-URI audience assertion on the bearer-JWT path (path-scoped to `/mcp/` via `WithMCPRequest`, leaving ConnectRPC and web-login aud semantics untouched) and a bounded RFC 7662 introspection path for opaque tokens (multi-IdP trial selection, audience-checked, fail-closed), fronted by an explicit `spgr_sk_` dispatch guard so API keys never reach the external IdP.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-10
- **Tasks:** 2
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- `audienceContains` helper (string|[]string aud, RFC 7519 §4.1.3) + `mcpResourceURI` field on `pgIdentityStore`. `resolveJWT` now enforces `aud`-contains-resource-URI (D-05.3) — but ONLY when `s.mcpResourceURI != ""` AND `MCPRequestFromContext(ctx)`, so ConnectRPC JWT callers (aud=client_id) sharing `Resolve` are never regressed (review HIGH #2). `NewOIDCVerifier`'s `aud==client_id` config is unchanged (OQ2: additive post-verify check).
- New `internal/auth/introspection.go`: `Introspector` (bounded `http.Client` timeout + active-result cache to `min(exp, 60s)` + per-issuer identity), `IntrospectionResult`, `NewIntrospector`, `BuildIntrospectors`, and `Introspect` performing the RFC 7662 form-POST with HTTP Basic RS credentials.
- `Resolve` dispatch made EXPLICIT: `isJWTShaped → resolveJWT`; `spgr_ws_ → resolveSession`; **`spgr_sk_ → resolveAPIKey` (new explicit guard)**; `len(introspectors) > 0 → resolveIntrospection`; else `resolveAPIKey` reject. The `spgr_sk_` guard runs BEFORE introspection so an API-key secret is never sent to a third-party IdP (review HIGH #3, D-08).
- `resolveIntrospection`: multi-IdP trial selection (config order), first `active==true` + resource-aud wins → `materializeIdentity(ctx, claims, false)`; per-issuer rate limit via the reused `rateLimiterFor` buckets; decisive inactive/wrong-aud → `ErrUnauthenticated`, all-non-decisive → `ErrTransient` (fail-closed).
- `serve.go` threads the single hoisted `mcpResourceURI` (only when `mcpRSEnabled`) as `MCPResourceURI` and `BuildIntrospectors(cfg.Auth.OIDC.Providers)` as `Introspectors` into `NewIdentityStore` — reusing Plan 03's hoist, no second derivation.

## Task Commits

Each task committed atomically (RED test written first, folded with GREEN into one buildable, DCO-signed commit — matches Plan 01/03 discipline; `tdd_mode` is false so no gate enforcement):

1. **Task 1: MCP resource-URI audience assertion on the bearer-JWT path (D-05.3)** — `673a4707` (feat)
2. **Task 2: RFC 7662 introspection path for opaque tokens (D-06)** — `a7b1557a` (feat)

**Plan metadata:** _(this SUMMARY commit)_

## Files Created/Modified
- `internal/auth/oidc_verifier.go` — `audienceContains(raw, want)` helper (string/[]string aud)
- `internal/auth/identitystore.go` — `mcpResourceURI`/`introspectors` fields + `IdentityStoreConfig.MCPResourceURI`/`Introspectors`; resolveJWT resource-URI check; explicit `spgr_sk_` dispatch guard; `resolveIntrospection`
- `internal/auth/introspection.go` — `Introspector`, `IntrospectionResult`, `NewIntrospector`, `BuildIntrospectors`, `Introspect` (RFC 7662, bounded, cached)
- `internal/auth/identitystore_audience_test.go` — AudienceBinding cases (accept, reject, HIGH #2 non-MCP regression, empty-config additive)
- `internal/auth/introspection_test.go` — active/inactive/5xx/wrong-aud/multi-first-match + spgr_sk_-never-introspected (counter zero)
- `internal/auth/usersbackend_stub_test.go` — dropped a now-unused `//nolint:unparam` on `stubAPIKeyToken`
- `cmd/specgraph/serve.go` — threads `MCPResourceURI` (hoisted) + `Introspectors` into `NewIdentityStore`

## Decisions Made
- **Exported the introspection types** (`Introspector`, `IntrospectionResult`, `NewIntrospector`). The plan's artifact registry named them lowercase, but `serve.go` (package `main`) must construct them, and `revive`'s `unexported-return` forbids an exported `Introspect` method returning an unexported type. Exporting is the minimal fix and matches the registry's alternate `Introspector` naming.
- **Added `BuildIntrospectors`** (exported) so RS client-secret resolution stays in the auth package via the existing unexported `resolveClientSecret` (mirrors `BuildLoginProviders`), rather than duplicating env-var lookup in `serve.go`.
- **Decisiveness encoded in `Introspect`'s return**: 2xx `active==false` and 4xx → decisive inactive (fail-closed); 5xx/network/decode failure → non-decisive error → `ErrTransient` only if no introspector answered decisively.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Exported the introspection types to satisfy cross-package construction + revive**
- **Found during:** Task 2
- **Issue:** The plan named the type `introspector` / result `introspectionResult` (unexported), but `serve.go` (package `main`) must build them, and `revive` rejects an exported `Introspect` method returning an unexported `*introspectionResult`.
- **Fix:** Exported `Introspector`, `IntrospectionResult`, `NewIntrospector`; added exported `BuildIntrospectors`.
- **Files modified:** internal/auth/introspection.go, cmd/specgraph/serve.go
- **Verification:** `go build ./...`, `golangci-lint run ./internal/auth/ ./cmd/specgraph/` (0 issues).
- **Committed in:** a7b1557a

**2. [Rule 3 - Blocking] Restructured introspection JSON decode + gosec/errcheck justifications**
- **Found during:** Task 2
- **Issue:** Per-field `_ = json.Unmarshal(...)` tripped errcheck (`check-blank`); the outbound POST tripped gosec G704 (SSRF taint); `resp.Body.Close` tripped errcheck.
- **Fix:** Read the body once, decode into a `Raw` map AND a typed struct (no unchecked unmarshals); added `//nolint:gosec // G704 ... operator config (T-03-04-06)` on the request/Do calls and `//nolint:errcheck // best-effort close` on the deferred close (matching `oauth2_provider.go`'s existing pattern).
- **Files modified:** internal/auth/introspection.go
- **Verification:** `golangci-lint run ./internal/auth/` → 0 issues.
- **Committed in:** a7b1557a

**3. [Rule 3 - Blocking] Removed a now-unused `//nolint:unparam` directive**
- **Found during:** Task 2
- **Issue:** Adding an `stubAPIKeyToken("prefix12")` caller (varying the previously-constant prefix) made the `//nolint:unparam` directive on `stubAPIKeyToken` unused, which `nolintlint` flags as an error.
- **Fix:** Removed the directive.
- **Files modified:** internal/auth/usersbackend_stub_test.go
- **Verification:** `golangci-lint run ./internal/auth/` → 0 issues.
- **Committed in:** a7b1557a

---

**Total deviations:** 3 auto-fixed (all Rule 3 blocking — cross-package export, linter conformance). **Impact on plan:** No scope change; every planned artifact delivered. The only public-API divergence from the plan text is exporting the introspection types (the plan's own artifact registry lists `Introspector`), required for construction from `serve.go`.

## Issues Encountered
None affecting delivery. `task check`'s `fmt:check` still reports the same pre-existing repo-wide `.planning/` dprint/markdown drift and two untouched Go files (`cmd/specgraph/doctor_server.go`, `cmd/specgraph/spec.go`) with pre-existing gofmt drift — none are files this plan touched (scope boundary). All Go gates are green on the touched files: `gofmt -l` clean, `golangci-lint run ./internal/auth/ ./cmd/specgraph/` reports 0 issues, `task license:check` passes, `task test` exits 0 (auth/server/cmd all `ok`).

## User Setup Required
None — no external service configuration required for this plan's code. Exercising the delegated-auth path against a real IdP (RFC 8707 resource-bound tokens + a live RFC 7662 introspection endpoint) is the D5 manual-UAT precondition, gated on an https deployment, not a code setup step.

## Next Phase Readiness
- AUTH-04's token-validation half is complete: MCP-path JWTs are resource-audience-bound (D-05.3), opaque tokens validate via bounded introspection (D-06), and static-credential dispatch is preserved and hardened (D-08). Phase 03 (external-identity-provider-integration) plans 01–04 are all complete.
- `task test` green; no blockers. Suggested next: `/gsd-verify-work 03` for the human-judgment D5 cross-IdP handshake, then advance to the next phase.
- Deferred (out of scope): pre-existing repo-wide `.planning/` dprint + markdown-lint drift and the two untouched `cmd/specgraph` gofmt drifts.

---
*Phase: 03-external-identity-provider-integration*
*Completed: 2026-07-10*

## Self-Check: PASSED
- Created files `internal/auth/introspection.go`, `internal/auth/introspection_test.go`, `internal/auth/identitystore_audience_test.go` exist on disk.
- Commits `673a4707` and `a7b1557a` present in git log with DCO sign-off.
- All task acceptance-criteria greps pass; `go build ./...`, `task test`, `golangci-lint` (auth/cmd), and `task license:check` green; `go test ./internal/auth/ -run 'AudienceBinding|Introspection|Resolve'` → all pass (43 tests).
