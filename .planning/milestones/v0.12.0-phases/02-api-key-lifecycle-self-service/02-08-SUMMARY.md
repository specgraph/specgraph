---
phase: 02-api-key-lifecycle-self-service
plan: 08
subsystem: ui
tags: [sveltekit, svelte5, connectrpc, csrf, api-keys, vitest, web]

requires:
  - phase: 02-01
    provides: generated IdentityService TypeScript client (identity_pb.ts) — the four self RPCs
  - phase: 02-03
    provides: specgraph_csrf cookie issuance on the whoami GET + csrfValidate double-submit middleware
  - phase: 02-05
    provides: server-side self-mint/rotate/revoke handlers + csrfValidate mount + anti-key-chaining gate
provides:
  - "identityClient exported from web/src/lib/api/client.ts"
  - "csrfInterceptor — double-submit echo of specgraph_csrf into X-CSRF-Token on Connect calls"
  - "keys.svelte.ts — Svelte 5 $state key list + list/create/rotate/revoke over the self RPCs"
  - "/keys dashboard route with create/rotate/revoke controls and loading/error states"
  - "RevealKeyModal — one-time plaintext reveal with copy + secret-manager guidance"
  - "Keys nav entry in the dashboard layout"
affects: [phase-03-external-idp, web-dashboard]

tech-stack:
  added: []
  patterns:
    - "Double-submit CSRF on the web client: interceptor reads the non-HttpOnly specgraph_csrf cookie and echoes it as X-CSRF-Token"
    - "One-time-secret UX: minted plaintext returned to the caller and rendered once in a modal; never stored in module $state or re-fetched"
    - "Partial vi.mock (importOriginal + vi.hoisted) to keep the real interceptor while stubbing the RPC client"

key-files:
  created:
    - web/src/lib/keys.svelte.ts
    - web/src/lib/keys.test.ts
    - web/src/routes/keys/+page.svelte
    - web/src/lib/components/RevealKeyModal.svelte
  modified:
    - web/src/lib/api/client.ts
    - web/src/routes/+layout.svelte

key-decisions:
  - "csrfInterceptor sets X-CSRF-Token whenever the specgraph_csrf cookie is present (Connect unary RPCs are all POSTs); server-side csrfValidate scopes enforcement to the four self-key procedures"
  - "The minted plaintext is returned from createKey/rotateKey to the caller and held only in the page's transient `revealed` $state while the modal is open — never in keys.svelte.ts state"
  - "PermissionDenied (anti key-chaining, Source==apikey) is mapped in the state module to a readable OIDC-required message rather than surfacing the raw code"
  - "Task 2 (UI) implemented without unit tests — no component-testing lib is configured and UI is not unit-testable per tdd.md; verified via production build"

patterns-established:
  - "Web self-service panel consumes the same server RPCs that enforce owner-from-context/floor/source gates — no client-trusted authorization"

requirements-completed: [AUTH-03]

coverage:
  - id: D1
    description: "identityClient exported and wired into the shared Connect transport"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "web/src/lib/keys.test.ts#keys state module (RPC calls resolve through the mocked identityClient)"
        status: pass
      - kind: integration
        ref: "pnpm -C web run build (exit 0)"
        status: pass
    human_judgment: false
  - id: D2
    description: "csrfInterceptor echoes the specgraph_csrf cookie into X-CSRF-Token and omits it when absent"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "web/src/lib/keys.test.ts#csrfInterceptor echoes the specgraph_csrf cookie value into X-CSRF-Token"
        status: pass
      - kind: unit
        ref: "web/src/lib/keys.test.ts#csrfInterceptor sets no X-CSRF-Token header when the cookie is absent"
        status: pass
    human_judgment: false
  - id: D3
    description: "keys.svelte.ts list/create/rotate/revoke over the four self RPCs; create/rotate surface the plaintext once and never persist it"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "web/src/lib/keys.test.ts#createKey surfaces the plaintext exactly once and never persists it in state"
        status: pass
      - kind: unit
        ref: "web/src/lib/keys.test.ts#listKeys populates the reactive list / rotateKey returns the freshly minted plaintext / revokeKey calls the RPC and refreshes the list"
        status: pass
    human_judgment: false
  - id: D4
    description: "PermissionDenied (anti key-chaining) maps to a readable OIDC-required message, not a raw code"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "web/src/lib/keys.test.ts#maps a PermissionDenied (anti key-chaining) failure to a readable message"
        status: pass
    human_judgment: false
  - id: D5
    description: "Live /keys panel: interactive list/create/rotate/revoke, one-time reveal (shown once, unrecoverable after close), CSRF enforcement on mutations, and the eligibility note behaving against a running server"
    verification:
      - kind: manual_procedural
        ref: "02-08-PLAN.md Task 3 checkpoint steps 1–6 (start server + web dev build, log in, create/rotate/revoke, strip CSRF cookie, legacy-apikey-session denial)"
        status: unknown
    human_judgment: true
    rationale: "Interactive browser behavior — one-time-reveal irrecoverability, clipboard copy, CSRF-strip rejection, and visual eligibility messaging — cannot be asserted headlessly; requires a running server and human visual/functional confirmation."

duration: 35min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 08: MCP Keys Dashboard Panel Summary

**A net-new SvelteKit "MCP Keys" panel — `identityClient` + a double-submit CSRF interceptor, a Svelte 5 `keys.svelte.ts` state module over the four self RPCs, a `/keys` route with list/create/rotate/revoke, and a one-time reveal modal — delivering SC#1's web half with the plaintext shown exactly once and never re-fetched.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-07-09T21:20:00Z (approx, incl. exploration)
- **Completed:** 2026-07-09T21:25:17Z
- **Tasks:** 2 implementation tasks (+ 1 human-verify checkpoint pending)
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- Exported `identityClient` on the shared Connect transport and added `csrfInterceptor`, which reads the non-HttpOnly `specgraph_csrf` cookie and echoes it into the `X-CSRF-Token` header (D-09 double-submit) — the token the Plan 05 `csrfValidate` mount validates.
- Built `keys.svelte.ts`: Svelte 5 `$state` key list + loading/error, and async `listKeys/createKey/rotateKey/revokeKey` over `identityClient.{listMyAPIKeys,createMyAPIKey,rotateMyAPIKey,revokeMyAPIKey}`. Create/rotate return the one-time plaintext to the caller; the module never stores or re-fetches it.
- Created the `/keys` route (list table + create form with label/role-downgrade/expiry, rotate/revoke actions, loading/error states) and wired create/rotate to open `RevealKeyModal` with the returned plaintext.
- `RevealKeyModal` shows the minted secret once with a copy-to-clipboard button and a secret-manager / `SPECGRAPH_API_KEY` storage instruction; closing clears the transient `revealed` state (no re-fetch path exists).
- Added a "Keys" nav entry to the dashboard layout and an eligibility note: self-mint requires an OIDC/`spgr_ws_` session; a `Source=="apikey"` session denied by the anti-key-chaining gate renders a readable message (cursor #4a).
- 8 Vitest cases (CSRF echo present/absent, list populates, plaintext surfaced once and not persisted, create sends label/role-downgrade, rotate returns new plaintext, revoke refreshes, PermissionDenied→readable message) — all green; production build green.

## Task Commits

1. **Task 1 (RED): failing keys/CSRF tests** - `760fc370` (test)
2. **Task 1 (GREEN): identityClient + CSRF interceptor + keys state module** - `cea2a4e0` (feat)
3. **Task 2: /keys panel + RevealKeyModal + nav entry** - `206e7d7f` (feat)

**Plan metadata:** _(this docs commit)_

_Task 3 is a human-verify checkpoint — pending (see below)._

## Files Created/Modified

- `web/src/lib/api/client.ts` - Added `identityClient`, `csrfInterceptor`, `readCsrfToken`, `CSRF_COOKIE`/`CSRF_HEADER`; registered the interceptor on the transport
- `web/src/lib/keys.svelte.ts` (new) - `$state` key list + list/create/rotate/revoke over the self RPCs; `friendlyError` maps PermissionDenied
- `web/src/lib/keys.test.ts` (new) - Vitest coverage for the interceptor + state module
- `web/src/routes/keys/+page.svelte` (new) - MCP Keys panel UI
- `web/src/lib/components/RevealKeyModal.svelte` (new) - One-time reveal modal
- `web/src/routes/+layout.svelte` - Keys nav entry

## Decisions Made

- The CSRF interceptor sets `X-CSRF-Token` whenever the cookie is present (all Connect unary RPCs are POSTs); scoping enforcement to the self-key procedures is the server's job (`csrfValidate`), so echoing on reads is harmless and keeps the client simple.
- The one-time plaintext is returned up to the page and held only in the page's transient `revealed` `$state` while the modal is open — deliberately never in `keys.svelte.ts` state (T-02-28).
- Task 2 (UI) was implemented without unit tests: no component-testing library is configured and the surface is visual/interactive (per `tdd.md`, skip TDD for UI). It is covered by the production build plus the Task 3 human-verify checkpoint.

## Deviations from Plan

None - plan executed exactly as written. (Task 2 carried `tdd="true"`, but its own `<verify>` is a production build and no component-test harness exists; the RPC/CSRF logic it depends on is fully unit-tested in Task 1. Documented above, not a scope change.)

## TDD Gate Compliance

- Task 1: clean RED (`760fc370` test) → GREEN (`cea2a4e0` feat) sequence; the RED run failed on the missing `keys.svelte`/`csrfInterceptor` imports, GREEN passed all 8 cases.
- Task 2: UI-only, verified by build (no RED/GREEN gate applicable — no unit-testable behavior beyond the Task-1 logic it consumes).

## Issues Encountered

- Initial `vi.mock` factory referenced top-level stub variables and hit Vitest's hoisting guard ("Cannot access 'listMyAPIKeys' before initialization"); resolved by declaring the stubs via `vi.hoisted(() => ({...}))`. Verified during the RED run before implementation.

## Known Stubs

None — the panel is wired to the live self RPCs; no placeholder/mock data paths remain.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SC#1's web half is delivered: a session-authenticated user can self-provision/list/rotate/revoke keys from the dashboard with one-time reveal and CSRF-protected mutations.
- **Pending:** Task 3 human-verify checkpoint (interactive `/keys` flows against a running server) — see the returned checkpoint for exact steps. Automated checks (`pnpm -C web test`, `pnpm -C web run build`) are green.

## Self-Check: PASSED

- Files verified on disk: `web/src/lib/api/client.ts`, `web/src/lib/keys.svelte.ts`, `web/src/lib/keys.test.ts`, `web/src/routes/keys/+page.svelte`, `web/src/lib/components/RevealKeyModal.svelte`, `web/src/routes/+layout.svelte` — all FOUND.
- Commits verified in `git log`: `760fc370` (test), `cea2a4e0` (feat), `206e7d7f` (feat) — all FOUND.
- `pnpm -C web test` → 10 passed (2 files); `pnpm -C web test -- --run keys.test.ts` → exit 0; `pnpm -C web run build` → exit 0 (keys page built).

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*
