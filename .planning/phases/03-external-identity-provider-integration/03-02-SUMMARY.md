---
phase: 03-external-identity-provider-integration
plan: 02
subsystem: auth
tags: [oauth2, github, userinfo, oidc, identity, claims-mapping, pkce, login]

# Dependency graph
requires:
  - phase: 03-external-identity-provider-integration
    provides: "Plan 01 seam — LoginProvider.Exchange returning *OIDCClaims, materializeIdentity/ResolveLogin, config.ProviderIssuer canonical-issuer helper, OIDCProviderConfig oauth2 fields"
provides:
  - "oauth2LoginProvider — native generic OAuth2 + userinfo login provider (GitHub as driving config), Exchange → userinfo → *OIDCClaims"
  - "BuildLoginProviders kind==\"oauth2\" construction branch with startup-fatal endpoint/selector/email-scope validation"
  - "buildOIDCProvider/buildOAuth2Provider split (kind gate relaxed from hard oidc-only reject)"
  - "issuerID stamped from config.ProviderIssuer(pc) so claims.Issuer, the (issuer,subject) binding, and the claims-mapping key cannot diverge (review HIGH #1)"
affects: [03-03-rfc9728-metadata, 03-04-introspection-audience, cross-provider-account-linking]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pattern 1 (RESEARCH): oauth2 LoginProvider reuses oauth2.Config + PKCE S256 for the front channel/code exchange but replaces id_token verification with a userinfo GET"
    - "Verified-email fallback: a null/private userinfo email fetches the primary&&verified entry from the emails endpoint; unverified/absent → blank (never trust an unverified email, D-02)"
    - "Stable-subject binding: numeric userinfo id stringified as Subject, never a renameable username (D-02)"
    - "Single canonical issuer (config.ProviderIssuer) stamped onto claims.Issuer so runtime claims-mapping lookup and startup key align (HIGH #1)"
    - "Startup-fatal config validation for oauth2 providers mirrors the existing oidc missing-secret/bad-audience discipline"

key-files:
  created:
    - internal/auth/oauth2_provider.go
    - internal/auth/oauth2_provider_test.go
  modified:
    - internal/auth/loginprovider.go
    - internal/auth/loginprovider_test.go

key-decisions:
  - "RED+GREEN folded per task into one buildable commit (pre-commit/pre-push gates require a compiling tree; matches Plan 01 discipline)"
  - "buildOIDCProvider extracted alongside buildOAuth2Provider so the switch-on-kind keeps each provider path self-contained; oidc behavior byte-identical"
  - "hasEmailScope uses substring \"email\" match so both GitHub \"user:email\" and a plain \"email\" scope satisfy the emails_url precondition"
  - "displayName selector reads userinfo \"name\" then \"login\"; emailField defaults to \"email\" when unset"
  - "issuerID never re-derived in the provider — always populated by BuildLoginProviders from config.ProviderIssuer(pc) (HIGH #1)"

patterns-established:
  - "oauth2 provider outbound calls (userinfo + emails) go through one bounded getJSON helper that fails closed on transport error, non-2xx, or malformed body"

requirements-completed: [AUTH-01]

# Coverage metadata
coverage:
  - id: D1
    description: "oauth2LoginProvider.Exchange swaps code → access token → userinfo GET and materializes *OIDCClaims with a stringified stable subject, Issuer == config.ProviderIssuer, and userinfo Raw for claims_mapping"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_Exchange_VerifiedEmailFallback"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_Exchange_MissingSubjectFatal"
        status: pass
    human_judgment: false
  - id: D2
    description: "Verified-email fallback (D-02): a null/private userinfo email fetches the primary&&verified entry; an unverified/absent verified-primary yields blank Email"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_Exchange_VerifiedEmailFallback"
        status: pass
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_Exchange_UnverifiedEmailBlank"
        status: pass
    human_judgment: false
  - id: D3
    description: "AuthCodeURL carries state + PKCE S256 but no OIDC nonce (no id_token to bind)"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/oauth2_provider_test.go#TestOAuth2Provider_AuthCodeURL"
        status: pass
    human_judgment: false
  - id: D4
    description: "BuildLoginProviders constructs an oauth2 provider for kind==\"oauth2\"; missing auth/token/userinfo URL or subject_field is startup-fatal; emails_url without an email scope is startup-fatal; unsupported kinds still fatal; oidc path unchanged"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_OAuth2_Constructed"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_OAuth2_MissingUserinfoURL"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_OAuth2_MissingSubjectField"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_OAuth2_EmailsURLWithoutEmailScope"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_UnsupportedKind"
        status: pass
    human_judgment: false
  - id: D5
    description: "issuerID is stamped from config.ProviderIssuer(pc); userinfo Raw drives a claims_mapping role keyed by that same ProviderIssuer value — runtime-key alignment (review HIGH #1, D-04 reuse, no mapping code change)"
    requirement: "AUTH-01"
    verification:
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestBuildLoginProviders_OAuth2_IssuerFromProviderIssuer"
        status: pass
      - kind: unit
        ref: "internal/auth/loginprovider_test.go#TestOAuth2ClaimsMapping_KeyedByProviderIssuer"
        status: pass
    human_judgment: false
  - id: D6
    description: "End-to-end native GitHub OAuth2 login: browser redirect → GitHub consent → callback yields an spgr_ws_ session cookie and an oidc_bindings row (issuer=synthetic, subject=numeric id)"
    requirement: "AUTH-01"
    verification: []
    human_judgment: true
    rationale: "Requires a registered GitHub OAuth App + live browser consent (user_setup); no unit/integration test asserts the full front-channel flow this plan — the provider is unit-covered with httptest stubs, but the real IdP round-trip is manual UAT."

# Metrics
duration: ~40 min
completed: 2026-07-10
status: complete
---

# Phase 3 Plan 02: Native OAuth2 + Userinfo Login Provider Summary

**A native generic `oauth2LoginProvider` (GitHub as driving config) that reuses `oauth2.Config` + PKCE for the front channel but swaps id_token verification for a userinfo GET — materializing `*OIDCClaims` with a stable stringified subject, a verified-email fallback, and a `config.ProviderIssuer`-derived issuer that flows through the exact same binding/JIT/claims-mapping machinery OIDC uses.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-07-10T12:33Z (approx)
- **Completed:** 2026-07-10T13:12Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- New `oauth2LoginProvider` (`oauth2_provider.go`): `AuthCodeURL` mirrors the oidc authorize URL but drops the nonce (state + PKCE S256 remain the defenses); `Exchange` swaps the code for an access token, GETs the userinfo endpoint, stringifies the stable id selector into `Subject`, and returns `*OIDCClaims{Issuer, Subject, Email, Name, Raw}`.
- Verified-email fallback (D-02): a null/private userinfo email triggers a secondary GET to the emails endpoint and selects the `primary && verified` entry; an unverified/absent verified-primary leaves `Email` blank — an unverified address is never trusted.
- Relaxed `BuildLoginProviders`: the bare `kind != "oidc"` reject is replaced with a switch; `buildOIDCProvider` (unchanged behavior) and `buildOAuth2Provider` (new) are the two branches. Missing `auth_url`/`token_url`/`userinfo_url`/`subject_field` is startup-fatal; `emails_url` set without an email scope is startup-fatal.
- `issuerID` is stamped from `config.ProviderIssuer(pc)` — the single canonical issuer shared by the runtime `claims.Issuer` lookup and (Plan 03) the startup claims-mapping key — so a GitHub `claims_mapping` role reliably resolves instead of silently falling through to the default role (review HIGH #1).
- Userinfo `Raw` feeds the existing `applyClaimsMapping` → role mechanism with the JIT default-role fallback, with **zero mapping code changes** (D-04); proven by a `ClaimsMapping` test keyed by the `ProviderIssuer` value.

## Task Commits

Each task was committed atomically (RED+GREEN folded per task for a buildable tree):

1. **Task 1: implement oauth2LoginProvider (Exchange → userinfo → *OIDCClaims)** - `7a34f8fa` (feat)
2. **Task 2: wire oauth2 into BuildLoginProviders with startup-fatal validation** - `19b9e01d` (feat)

**Plan metadata:** _(this SUMMARY commit)_

## Files Created/Modified
- `internal/auth/oauth2_provider.go` - `oauth2LoginProvider` struct + `ID`/`DisplayName`/`AuthCodeURL`/`Exchange`; `fetchUserinfo`/`fetchPrimaryVerifiedEmail`/`getJSON`/`selectStringField`/`displayNameFromUserinfo` helpers
- `internal/auth/oauth2_provider_test.go` - httptest-stubbed token/userinfo/emails coverage: verified-email fallback, unverified-blank, no-nonce AuthCodeURL, missing-subject fail-closed
- `internal/auth/loginprovider.go` - `BuildLoginProviders` switch on kind; extracted `buildOIDCProvider`; new `buildOAuth2Provider` + `hasEmailScope`; `net/http` import
- `internal/auth/loginprovider_test.go` - oauth2 construction, startup-fatal (missing userinfo/subject, emails-without-email-scope), unsupported-kind, `issuerID == ProviderIssuer`, and `ProviderIssuer`-keyed ClaimsMapping cases

## Decisions Made
- **RED+GREEN folded per task into one buildable commit.** The repo's pre-commit (golangci-lint + build) and pre-push (`task check`) gates require a compiling tree; a standalone RED commit referencing an undefined symbol would be rejected. Each of the 2 commits builds green and passes unit tests — matching Plan 01's established discipline.
- **`buildOIDCProvider` extracted** alongside `buildOAuth2Provider` so the switch keeps each provider path self-contained. The oidc branch is behavior-identical (audience check, secret, bounded discovery, verifier, scope/display defaulting) — only relocated.
- **`hasEmailScope` uses a substring `"email"` match** so both GitHub's `user:email` and a plain `email` scope satisfy the `emails_url` precondition.
- **`issuerID` is never re-derived in the provider** — always `config.ProviderIssuer(pc)` from `BuildLoginProviders` (HIGH #1).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Lint fixes folded into the Task 2 commit for a green pre-commit gate**
- **Found during:** Task 2 (`golangci-lint run ./internal/auth/`)
- **Issue:** The initial oauth2 implementation tripped four linters: `errcheck` with `check-blank: true` flagged an `email, _ =` blank assign and the `resp.Body.Close()` defer; `gocritic` wanted `http.NoBody` over a `nil` request body; `gosec` G101 false-positived on the test config struct's `ClientSecretEnv` field.
- **Fix:** Replaced the blank assign with an `if v, selErr := selectStringField(...); selErr == nil` guard; added `//nolint:errcheck // best-effort close on read path` on the body-close defer (matching `cmd/specgraph/login.go` convention); switched the GET to `http.NoBody`; added `//nolint:gosec // G101: ClientSecretEnv is an env var name, not a credential` (matching the existing `loginprovider_test.go` convention).
- **Files modified:** internal/auth/oauth2_provider.go, internal/auth/loginprovider_test.go
- **Verification:** `golangci-lint run ./internal/auth/` → 0 issues; `go test ./internal/auth/` green.
- **Committed in:** 19b9e01d (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 lint/correctness). **Impact on plan:** No scope change; all planned artifacts delivered exactly. The lint guard on the blank email assign is also a minor correctness improvement (explicit error handling over a discarded error).

## Issues Encountered
None — all planned work completed. `task check`'s `fmt:check` surfaces the same 139 pre-existing dprint drifts in unrelated `.planning/intel/classifications/*.json` (plus one untracked `.planning/research/.cache/*.json`) that Plan 01 already logged to `deferred-items.md`; none are Go source and none are files this plan touched. All Go source is gofmt-clean and `golangci-lint` reports 0 issues on `internal/auth`. `task test` (build + fast unit suite) is fully green.

## User Setup Required
**External service required for manual UAT only.** Unit tests stub the endpoints via httptest and need no credentials. For the end-to-end GitHub login flow (D6, `human_judgment: true`):
- Register a GitHub OAuth App (callback `<base>/api/auth/oidc/callback`, scopes `read:user`, `user:email`).
- Set `SPECGRAPH_GITHUB_CLIENT_SECRET` (referenced via the provider's `client_secret_env`).
- Verify: browser flow through the provider start → consent → callback yields an `spgr_ws_` cookie and an `oidc_bindings` row (issuer=synthetic, subject=numeric id).

## Next Phase Readiness
- The oauth2 provider is ready for Plan 03 (RFC 9728 metadata): Plan 03 Task 2 must re-key `buildClaimsMappingByIssuer` (serve.go:773-780) by `config.ProviderIssuer(pc)` to match this plan's runtime `claims.Issuer` — the serve.go edit is deferred to Plan 03 to keep serve.go single-owner within Wave 2. Both sites call the SAME `ProviderIssuer` helper, so they cannot diverge; this plan's `ClaimsMapping` test proves the runtime side.
- `task test` green (build + fast unit tests). No blockers.
- Deferred (out of scope, per RESEARCH): cross-provider account-linking flow (data model ships via D-03; linking UX is a fast-follow) and GitHub org/team-membership → role mapping.

---
*Phase: 03-external-identity-provider-integration*
*Completed: 2026-07-10*

## Self-Check: PASSED
- Created files `internal/auth/oauth2_provider.go` and `internal/auth/oauth2_provider_test.go` exist on disk.
- Commits `7a34f8fa` and `19b9e01d` present in git log with DCO sign-off.
- All task acceptance-criteria greps pass; `go build ./...`, `go test ./internal/auth/`, and full `task test` green; `golangci-lint run ./internal/auth/` reports 0 issues.
