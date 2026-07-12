---
gsd_state_version: 1.0
milestone: v0.12.0
milestone_name: milestone
current_phase: 05
current_phase_name: ui-project-selector-and-refinements
status: executing
stopped_at: Completed 05-03-PLAN.md
last_updated: "2026-07-12T13:57:49.545Z"
last_activity: 2026-07-12
last_activity_desc: Phase 05 execution started
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 28
  completed_plans: 18
  percent: 64
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-10)

**Core value:** Specs stay live and queryable as a graph — with locked architectural decisions,
drift detection, and a durable storage/query layer — so both humans and agent-based execution
engines can trust the spec graph as ground truth instead of static, decaying markdown.

**Current focus:** Phase 05 — ui-project-selector-and-refinements

## Current Position

Phase: 05 (ui-project-selector-and-refinements) — EXECUTING
Plan: 4 of 13
Status: Ready to execute
Last activity: 2026-07-12 — Phase 05 execution started

Progress: [███████████████░░░░░] 75%

## Performance Metrics

**Velocity:**

- Total plans completed: 15
- Average duration: - min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 1 | - | - |
| 02 | 8 | - | - |
| 03 | 4 | - | - |
| 4 | 2 | - | - |

**Recent Trend:**

- Last 5 plans: none yet
- Trend: N/A (no plans executed yet)

*Updated after each plan completion*
| Phase 01 P01 | 7min | 2 tasks | 2 files |
| Phase 02 P01 | 4min | 2 tasks | 5 files |
| Phase 02 P02 | 7 min | 3 tasks | 6 files |
| Phase 02 P03 | 4 min | 2 tasks | 5 files |
| Phase 02 P04 | 2min | 3 tasks | 6 files |
| Phase 02 P05 | 20 min | 3 tasks | 5 files |
| Phase 02 P06 | 8 min | 3 tasks | 6 files |
| Phase 02-api-key-lifecycle-self-service P07 | 9min | 2 tasks | 5 files |
| Phase 02 P08 | 35min | 2 tasks | 6 files |
| Phase 03 P01 | 30 min | 3 tasks | 9 files |
| Phase 03 P02 | 40 min | 2 tasks | 4 files |
| Phase 03 P03 | 15 min | 2 tasks | 4 files |
| Phase 03 P04 | 25 min | 2 tasks | 7 files |
| Phase 04-verification-integration-reliability P01 | 4min | 3 tasks | 2 files |
| Phase 04 P02 | 2 min | 1 tasks | 1 files |
| Phase 05 P01 | ~10 min | 3 tasks | 115 files |
| Phase 05 P02 | 5 min | 3 tasks | 3 files |
| Phase 05-ui-project-selector-and-refinements P03 | 7 min | 3 tasks | 7 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table. Full architectural history (14 ADRs,
63 constraint entries, 98 context entries from the 177-doc corpus ingest) lives in
`.planning/intel/{decisions,constraints,context,SYNTHESIS}.md`.

- Roadmap: Auth cluster split across two phases (Phase 2: API-key lifecycle; Phase 3: external
  IdP integration) rather than one, since AUTH-04's OAuth 2.1 resource-server direction is
  described in intel as the eventual complement/successor to AUTH-03's self-service model —
  sequencing Phase 3 after Phase 2 avoids double-touching the identity subsystem.

- Roadmap: DRFT-01 and INTG-01 merged into one "Verification & Integration Reliability" phase
  rather than two single-requirement phases — both are small, isolated reliability items with
  no other natural cluster-mate.

- [Phase 01]: CFG-02: pinned golangci-lint via a single Taskfile.yml var + go install (matching CI's existing method), with a silent leaf task exposing it to CI via command substitution — Closes local (brew, unpinned) vs CI (pinned) version drift structurally — one declaration, one install method for both sides
- [Phase 02]: RotateAPIKeyForUser takes an explicit newKey (handler-owned secret + floored downgrade + capped expiry), never inheriting the old key ceiling
- [Phase 02]: Quota-safe self-mint serializes a user's mints via a parent users-row FOR UPDATE lock (not count(*) FOR UPDATE); count+insert inside the tx
- [Phase 02]: Self-service key policy in dedicated SelfServiceKeysConfig (auth.self_service_keys), not deprecated APIKeyConfig — Keeps new AUTH-03 policy off the storage-owned legacy struct; CSRF validator exempts Bearer callers, enforces constant-time double-submit on cookie-authed self-key POSTs
- [Phase 02]: RegisterIdentityService takes a non-variadic SelfServiceKeysConfig param so self-service key policy is threaded, not swallowed by the opts variadic — Adding it as another HandlerOption would lose the config
- [Phase 02]: Web CSRF via double-submit interceptor echoing the specgraph_csrf cookie into X-CSRF-Token; enforcement scoping is the server's job
- [Phase 02]: One-time minted plaintext returned to the caller and held only in the page's transient reveal state, never in the keys.svelte.ts module state
- [Phase 03]: oauth2 provider issuerID stamped from config.ProviderIssuer(pc) so claims.Issuer, the (issuer,subject) binding, and the claims-mapping key cannot diverge (review HIGH #1)
- [Phase 03]: Verified-email fallback: null/private userinfo email fetches primary&&verified from the emails endpoint; unverified/absent leaves Email blank (D-02)
- [Phase 03]: RFC 8707 resource-URI audience check is additive and path-scoped — fires only when MCPRequestFromContext(ctx) AND mcpResourceURI set, so ConnectRPC/web-login aud==client_id semantics are untouched (OQ2, review HIGH #2, D-05.3)
- [Phase 03]: Resolve dispatch is explicit — spgr_sk_/spgr_ws_ prefix guards run BEFORE the introspection branch so static API-key/session secrets are never POSTed to an external IdP (review HIGH #3, D-08)
- [Phase 03]: Introspection fail-closed algebra — decisive inactive/wrong-aud → ErrUnauthenticated, all-non-decisive (5xx/timeout/rate-limited) → ErrTransient; bounded by client timeout + active-result cache + per-issuer rate limiter (D-06)
- [Phase 05]: Manual-fallback shadcn install: init 1.4.1 blocks on an interactive preset prompt, so components.json/app.css/utils.ts authored by hand + primitives via 'shadcn-svelte add -y -o --no-deps'
- [Phase 05]: Slate delivered via the verified OKLCH token block in app.css (CLI base-color enum has no 'slate'); components.json baseColor:slate is cosmetic metadata
- [Phase 05-ui-project-selector-and-refinements]: D-01 is mechanism-only after 05-03: the layout invalidates on switch but project-scoped pages still fetch via onMount/$effect until Wave 3 (05-10..13) adds +page.ts loads. Layout is single owner of the active-project breadcrumb. — Prevents Wave 3 from reintroducing per-page breadcrumbs or claiming end-to-end switch re-fetch prematurely.

### Pending Todos

None yet.

### Blockers/Concerns

- AUTH-03 (`spgr-g7st`, Phase 2) is already in progress per beads status, prior to any GSD plan
  existing. When planning Phase 2, check current repo state / open PRs first to avoid re-doing
  work already underway. (REL-01's equivalent Phase 1 concern is resolved — Phase 1 is complete.)

- DRFT-01 and INTG-01 (Phase 4) are not covered by the 177-doc intel corpus (both are newer/
  smaller items) — plan-phase should scope these from current code, not from `.planning/intel/`.

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none — see REQUIREMENTS.md v2 Requirements for the full deferred backlog from the beads migration)* | | | |

## Session Continuity

Last session: 2026-07-12T13:57:41.925Z
Stopped at: Completed 05-03-PLAN.md
Resume file: None
