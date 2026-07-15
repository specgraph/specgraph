---
phase: 05-ui-project-selector-and-refinements
plan: 11
subsystem: ui
tags: [sveltekit, svelte5, load-function, connect-rpc, skeleton, project-switch]

requires:
  - phase: 05-ui-project-selector-and-refinements
    provides: "Wave 2 migrated spec/decision detail components (05-03), AccordionSection (05-04), MetadataBar (05-06), DiffView/VersionCompare (05-08), badge-variants (05-01)"
provides:
  - "Spec detail (/spec/[slug]) universal +page.ts load() keyed on params.slug"
  - "Decision detail (/decision/[slug]) universal +page.ts load() keyed on params.slug"
  - "Skeleton/Empty(not-found)/Error(Retry) states for both detail views via streamed {#await}"
  - "Removal of manual activeSlug stale-guard (load-driven data replaces it)"
affects: [ui-project-selector, spec-detail, decision-detail, project-switch-reactivity]

tech-stack:
  added: []
  patterns:
    - "Universal +page.ts load() streams a promise (not awaited) so {#await} re-suspends to Skeleton on invalidateAll()"
    - "await parent() + depends('app:project') before RPCs (Pitfall 6) for project-switch re-run"
    - "ConnectError + Code.NotFound distinguishes empty (not-found) from error (Retry) states"
    - "Non-critical secondary fetches individually try/caught with [] fallback"

key-files:
  created:
    - web/src/routes/spec/[...slug]/+page.ts
    - web/src/routes/decision/[...slug]/+page.ts
  modified:
    - web/src/routes/spec/[...slug]/+page.svelte
    - web/src/routes/decision/[...slug]/+page.svelte

key-decisions:
  - "NotFound RPC (Code.NotFound) routes to the empty 'not found in this project' card; all other errors route to the inline Retry card (T-05-15) — never +error.svelte."
  - "Changelog stays lazy-loaded (user-triggered); its cache resets keyed on the load promise reference so a switch clears stale changelog (T-05-05)."
  - "groupedEdges converted from top-level $derived to a plain groupEdges(edges, slug) function so it composes with resolved load data inside {#await}."

patterns-established:
  - "Detail-view load-ification: streamed detail promise + {#await} skeleton/empty/error, ConnectError NotFound → empty sentinel"

requirements-completed: [D-01, D-02, D-09, D-13, D-10]

coverage:
  - id: D1
    description: "Spec detail fetches in universal +page.ts load() keyed on params.slug, re-running on switch and slug change (D-01/D-02/D-09)"
    requirement: "D-01"
    verification:
      - kind: automated_ui
        ref: "web/src/routes/spec/[...slug]/+page.ts (PageLoad: await parent + depends('app:project') + getSpec keyed on params.slug)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build && pnpm -C web test"
        status: pass
    human_judgment: false
  - id: D2
    description: "Decision detail fetches in universal +page.ts load() keyed on params.slug; fixes pre-existing slug-nav gap (D-01/D-02/D-09)"
    requirement: "D-02"
    verification:
      - kind: automated_ui
        ref: "web/src/routes/decision/[...slug]/+page.ts (PageLoad: await parent + depends('app:project') + getDecision keyed on params.slug)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build && pnpm -C web test"
        status: pass
    human_judgment: false
  - id: D3
    description: "Manual activeSlug stale-guard removed; SvelteKit load re-run + params.slug dependency replaces it"
    requirement: "D-09"
    verification:
      - kind: automated_ui
        ref: "regex guard: +page.svelte contains no /activeSlug/, no /onMount/"
        status: pass
    human_judgment: false
  - id: D4
    description: "Each view renders Loading (Skeleton), Empty (not-found), Error (Retry) states with streamed {#await} skeleton-on-switch"
    requirement: "D-13"
    verification:
      - kind: automated_ui
        ref: "web/src/routes/{spec,decision}/[...slug]/+page.svelte ({#await} Skeleton / not-found Card / Retry Card)"
        status: pass
    human_judgment: true
    rationale: "Visual correctness of skeleton-on-switch and not-found copy needs human UAT against a real project switch (plan Manual UAT)."
  - id: D5
    description: "Categorical status/stage badges render via shared badge-variants map; direction-aware edge grouping preserved"
    requirement: "D-10"
    verification:
      - kind: automated_ui
        ref: "stageBadgeClass in spec/+page.svelte, statusBadgeClass in decision/+page.svelte; groupEdges preserves direction-aware labels"
        status: pass
    human_judgment: false
  - id: D6
    description: "Per-page breadcrumb stays removed (layout owns it, D-11); orphaned scoped .breadcrumb styles deleted"
    requirement: "D-13"
    verification:
      - kind: automated_ui
        ref: "regex guard: neither +page.svelte contains /class=\"breadcrumb\"/; .breadcrumb CSS block removed"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 11: Spec & Decision Detail Load-ification Summary

**Spec and Decision detail views now fetch in universal `+page.ts` `load()` keyed on `params.slug` with streamed `{#await}` Skeleton/Empty/Error states, replacing the manual `activeSlug` stale-guard and fixing the decision slug-nav gap.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-07-12T10:29:00Z (approx)
- **Completed:** 2026-07-12T14:44:13Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 rewritten)

## Accomplishments
- Spec detail (`/spec/[slug]`) load-ified: `getSpec` + secondary edges/findings/slices fetches moved into `+page.ts`, streamed so `{#await}` re-suspends to Skeleton on every project switch (`invalidateAll()`) and on slug change.
- Decision detail (`/decision/[slug]`) load-ified: `getDecision` + `linkedSpecs` moved into `+page.ts`; keying on `params.slug` also fixes the pre-existing bug where the old `onMount` fetch never re-ran on in-project slug change.
- Manual `activeSlug` stale-guard deleted (T-05-05) — load-driven data can never retain a prior-project/prior-slug spec.
- All three UI-SPEC states rendered per view: Skeleton (title + metadata + rows), Empty ("Spec/Decision not found in this project." card), Error (inline Retry card reading `loadError`). `NotFound` RPC routes to Empty; all other errors route to Retry (T-05-15).
- Categorical badges migrated to the shared `badge-variants` map (`stageBadgeClass`, `statusBadgeClass`); direction-aware `groupedEdges` and all label helpers preserved.

## Task Commits

Each task was committed atomically:

1. **Task 1: Spec detail load()** - `a5eca848` (feat)
2. **Task 2: Decision detail load()** - `18dbf926` (feat)

**Plan metadata:** _(this commit)_ (docs: complete plan)

## Files Created/Modified
- `web/src/routes/spec/[...slug]/+page.ts` - Spec detail universal `load()`: `await parent()` + `depends('app:project')` + primary `getSpec` (NotFound → notFound sentinel, other error → loadError) + three try/caught secondary fetches; streams `detail` promise.
- `web/src/routes/spec/[...slug]/+page.svelte` - Consumes `data.detail` via `$props()`; `{#await}` Skeleton/Empty/Error; removed `activeSlug`, `$effect` fetch, `onMount`, and orphaned `.breadcrumb` styles; `groupEdges()` function + `stageBadgeClass`; changelog stays lazy with cache reset keyed on load promise.
- `web/src/routes/decision/[...slug]/+page.ts` - Decision detail universal `load()`: `getDecision` + `linkedSpecs` filter (`DECIDED_IN` + `toId===slug`); NotFound/loadError sentinels; streams `detail` promise.
- `web/src/routes/decision/[...slug]/+page.svelte` - Consumes `data.detail` via `$props()`; `{#await}` Skeleton/Empty/Error; removed `onMount` fetch and orphaned `.breadcrumb` styles; kept `statusLabel`; `statusBadgeClass`.

## Decisions Made
- **NotFound vs error split:** used `ConnectError` + `Code.NotFound` in `load()` to distinguish the empty "not found in this project" state from a genuine load failure that gets the Retry card. This preserves the UI-SPEC State Matrix distinction while keeping errors off `+error.svelte` (T-05-15).
- **Changelog cache reset:** rather than reintroduce a slug-keyed guard, the changelog lazy-load cache resets via `$effect` tracking the `data.detail` promise reference — a fresh promise on every slug change and every `invalidateAll()`, so stale changelog clears on both (T-05-05).
- **groupedEdges as function:** converted the top-level `$derived` to a `groupEdges(edges, slug)` function so it composes with the resolved load data inside the `{#await}` `{:then}` block (keyed on the resolved spec's own slug).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `task check` fails at `fmt:check` due to 139 pre-existing unformatted `.planning/**/*.json` intel artifacts — **out of scope** and unrelated to this UI task. Verified 0 flagged files are outside `.planning/`/`.beads/`, and `dprint` has no Svelte/TS plugin so none of the four touched files are affected. Logged to `deferred-items.md` (05-11 entry). `task web:build`, `task build`, `pnpm build`, and `pnpm test` all pass.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Both detail views complete the Wave 3 reactive-switching + not-found handling on the Wave 2 migrated components. Ready for remaining Phase 05 plans / verification.
- Manual UAT recommended (plan verification): switching to a project without the spec/decision shows the not-found empty state; switch returns view to skeleton then new data; no stale content.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*

## Self-Check: PASSED

All 4 created/modified source files exist on disk; all 3 commits (a5eca848, 18dbf926, 70e8f8ce) present in git log.
