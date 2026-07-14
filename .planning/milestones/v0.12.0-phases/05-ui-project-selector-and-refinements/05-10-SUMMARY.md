---
phase: 05-ui-project-selector-and-refinements
plan: 10
subsystem: ui
tags: [sveltekit, svelte5, load, invalidateAll, shadcn, skeleton, project-switch]

requires:
  - phase: 05-03
    provides: "+layout.ts project bootstrap + invalidateAll switch seam (await parent resolves project default)"
  - phase: 05-05
    provides: "shadcn-migrated StatsBar, FunnelBar, SpecTable, TabBar, GraphMini"
  - phase: 05-09
    provides: "shadcn-migrated Graph component (dagre render, token colors)"
provides:
  - "Dashboard (/) fetches via universal +page.ts load(), re-fetches on project switch with new X-Specgraph-Project header"
  - "Graph (/graph) fetches via universal graph/+page.ts load(), re-fetches on switch"
  - "Both views render Loading (Skeleton) / Empty / Error (inline Retry) states per the UI-SPEC State Matrix"
  - "Streamed-promise {#await} pattern that re-suspends to skeleton on invalidateAll() (Pitfall 3), leaving no stale previous-project data"
affects: [05-11, 05-12, 05-13, constitution-load, spec-detail-load, decision-detail-load]

tech-stack:
  added: []
  patterns:
    - "Universal +page.ts load(): await parent() → depends('app:project') → return streamed promise (unawaited)"
    - "Errors caught INSIDE the streamed promise as a loadError sentinel — never reaches +error.svelte"
    - "{#await data.x}skeleton{:then d}error/empty/data{/await} for switch-refetch skeletons (not $navigating, not a switching flag)"

key-files:
  created:
    - web/src/routes/+page.ts
    - web/src/routes/graph/+page.ts
  modified:
    - web/src/routes/+page.svelte
    - web/src/routes/graph/+page.svelte

key-decisions:
  - "Dashboard AND graph both use the idiomatic streamed-promise {#await} skeleton strategy; the module-level switching-flag fallback allowed for graph was not needed because the {#await :then} block hands dagre the fully-resolved node/edge set."
  - "loadError is a caught sentinel returned from the streamed promise (resolves, never rejects) so it satisfies both the streamed-skeleton requirement AND the catch-inside-load requirement without conflict."

patterns-established:
  - "Load-ification streamed-promise pattern: reusable for the remaining Wave 3 views (constitution, spec/decision detail)"
  - "Inline Retry Card (Card + Button variant=outline calling invalidateAll()) as the per-view error boundary"

requirements-completed: [D-01, D-02, D-09, D-13, D-10]

coverage:
  - id: D1
    description: "Dashboard fetches via +page.ts load() (await parent, depends, 5-way Promise.all), returns data or caught loadError"
    requirement: "D-01"
    verification:
      - kind: unit
        ref: "web/src/routes/+page.ts structural check (await parent() + loadError) + pnpm build"
        status: pass
    human_judgment: false
  - id: D2
    description: "Graph fetches via graph/+page.ts load() (await parent, depends, getFullGraph), returns data or caught loadError"
    requirement: "D-02"
    verification:
      - kind: unit
        ref: "web/src/routes/graph/+page.ts structural check (await parent() + getFullGraph) + pnpm build"
        status: pass
    human_judgment: false
  - id: D3
    description: "Both views render Loading (Skeleton) / Empty / Error states with UI-SPEC copy; +page.svelte consumes data via $props(), no onMount/$state fetch"
    requirement: "D-09"
    verification:
      - kind: unit
        ref: "structural check (no onMount, has $props + Skeleton) + pnpm build + pnpm test (16 tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "End-to-end project switch returns each view to skeleton then new-project data with no stale data; active-project breadcrumb updates (D-13/D-10 visual contract)"
    requirement: "D-13"
    verification: []
    human_judgment: true
    rationale: "Per plan Verification Appetite (review MEDIUM #6): the visual switch-refetch behavior is accepted via manual UAT this phase; a Svelte component-test harness is out of appetite. The load() seam is covered structurally + by build; the visual re-fetch is human-confirmed."

duration: 12 min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 10: Dashboard & Graph load-ification Summary

**Dashboard (/) and Graph (/graph) migrated from onMount/$effect fetches to universal `+page.ts` `load()` functions with streamed-promise `{#await}` Skeleton/Empty/Error states, so a project switch's `invalidateAll()` re-fetches with the new `X-Specgraph-Project` header and leaves no stale data.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-12T14:33:00Z (approx)
- **Completed:** 2026-07-12T14:45:00Z (approx)
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 rewritten)

## Accomplishments
- Dashboard fetches its 5-way `Promise.all` (specs/ready/fullGraph/decisions/checkDrift) in `web/src/routes/+page.ts`, streamed so the page re-suspends to skeletons on switch.
- Graph fetches `getFullGraph` in `web/src/routes/graph/+page.ts`, streamed identically; dagre receives the fully-resolved node/edge set in the `{:then}` block.
- Both `+page.svelte` files consume `data` via `$props()`, removed their `onMount`/`$state` data fetches, and render Loading (shadcn `Skeleton`) / Empty ("Nothing here yet") / Error (inline Retry `Card` + `Button variant="outline"` → `invalidateAll()`) per the UI-SPEC State Matrix.
- Switch skeletons driven by streamed promises + `{#await}` — NOT bound to `$navigating` (Pitfall 3) and no stale previous-project data retained (T-05-05).
- Errors caught inside the streamed promise as a `loadError` sentinel, so they render the per-view inline Retry card instead of hitting `+error.svelte` (T-05-15, RESEARCH L279).

## Task Commits

Each task was committed atomically:

1. **Task 1: Dashboard load() — +page.ts + restyle +page.svelte** - `2109770f` (feat)
2. **Task 2: Graph load() — graph/+page.ts + restyle graph/+page.svelte** - `b9cfe0e5` (feat)

**Plan metadata:** _(this commit)_ (docs: complete plan)

## Files Created/Modified
- `web/src/routes/+page.ts` - Dashboard universal `PageLoad`: `await parent()`, `depends('app:project')`, streamed 5-way `Promise.all`, caught `loadError` sentinel.
- `web/src/routes/+page.svelte` - Consumes `data` via `$props()`; `{#await}` skeleton stat cards + rows / "Nothing here yet" empty / inline Retry error; ports recentSpecs/priorityGroups/decisionSpecCounts helpers to read `data.*`.
- `web/src/routes/graph/+page.ts` - Graph universal `PageLoad`: `await parent()`, `depends('app:project')`, streamed `getFullGraph`, caught `loadError`.
- `web/src/routes/graph/+page.svelte` - Consumes `data` via `$props()`; keeps `filterText` local `$state`; `{#await}` Skeleton `Card` / empty / inline Retry.

## Decisions Made
- Used the streamed-promise `{#await}` strategy for BOTH views (the graph-only `switching`-flag fallback the plan explicitly permitted was unnecessary — the `{:then}` block already hands dagre the complete, resolved set).
- `loadError` is a resolved sentinel field on the streamed promise (the helper never rejects), which reconciles the two otherwise-conflicting plan requirements: "stream the promise (don't await)" and "catch inside load() → loadError".

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `task check` fails at `fmt:check` on ~139 pre-existing `.planning/intel/classifications/*.json` files (multi-line array reformatting). These are **out of scope** — untouched by this plan and unrelated to the web frontend. `dprint` does not govern the four `web/src/routes/` `.svelte`/`.ts` files changed here (confirmed: `dprint check` finds no matching web files), and `task web:build`, `pnpm build`, and `pnpm test` (16 tests) all pass. Logged as a repo-wide pre-existing formatting drift, not a regression from this plan.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The streamed-promise load-ification pattern is established and ready to reuse for the remaining Wave 3 views (05-11 constitution, 05-12/05-13 spec & decision detail).
- Manual UAT still owns the end-to-end visual switch-refetch confirmation (D-13/D-10 visual contract), per the phase Verification Appetite.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
