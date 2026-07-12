---
phase: 05-ui-project-selector-and-refinements
plan: 13
subsystem: ui
tags: [sveltekit, svelte5, load-function, invalidateAll, constitution, badges, vitest, tailwind, shadcn]

requires:
  - phase: 05-03
    provides: layout-owned project breadcrumb + switchProject invalidateAll()
  - phase: 05-04
    provides: migrated AccordionSection component
  - phase: 05-01
    provides: badge-variants categorical layer palette
provides:
  - Constitution universal +page.ts load() (fetch out of component-body $effect)
  - Provenance-derived categorical layer badges that re-derive on project switch (D-10)
  - "No constitution for this project" empty state + Loading (Skeleton)/Error (Retry) states
  - Vitest-only load-seam unit test (depends('app:project') + data mapping + loadError)
affects: [constitution, project-switch, verify-work]

tech-stack:
  added: []
  patterns:
    - "Universal load() streams per-field promises off one RPC; a $derived Promise.all re-suspends {#await} to Skeleton on invalidateAll() (skeleton-on-switch, Pitfall 3/4)"
    - "Categorical layer badges via layerBadgeClass() fixed light/dark pairs (not --primary)"
    - "Vitest-only load-function seam test (no component harness) pins depends+mapping+loadError"

key-files:
  created:
    - web/src/routes/constitution/+page.ts
    - web/src/lib/constitution-load.test.ts
  modified:
    - web/src/routes/constitution/+page.svelte

key-decisions:
  - "load streams constitution/provenance/loadError as three top-level promise fields (off one RPC) so the component references data.provenance directly AND {#await} re-suspends to Skeleton on switch — reconciles Task 1's literal return shape with Task 2's data.provenance derivation + streamed skeleton-on-switch requirement"
  - "layerOf()/layerBadge snippet take provenance as a param (fed d.provenance inside {#await ... then d}) rather than closing over local $state — guarantees badges re-derive from reloaded provenance with no stale prior-project state (D-10/Pitfall 4)"

patterns-established:
  - "Constitution view fully tokenized: scoped <style> + all hex removed, Tailwind + shadcn Badge/Card/Skeleton/Button only"

requirements-completed: [D-10, D-01, D-02, D-09, D-13]

coverage:
  - id: D1
    description: "Constitution fetched in universal +page.ts load() that registers depends('app:project') (the invalidateAll() re-run mechanism) — D-01/D-02/D-09"
    requirement: "D-01"
    verification:
      - kind: unit
        ref: "web/src/lib/constitution-load.test.ts#registers depends('app:project') exactly once"
        status: pass
    human_judgment: false
  - id: D2
    description: "load maps the RPC response to { constitution, provenance } and the empty seam to { constitution: null, provenance: [] } (D-10)"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "web/src/lib/constitution-load.test.ts#maps a resolved response / maps an empty constitution"
        status: pass
    human_judgment: false
  - id: D3
    description: "RPC rejection is caught in load() and surfaced as loadError (no throw to +error.svelte) — T-05-15"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "web/src/lib/constitution-load.test.ts#returns a loadError when the RPC rejects"
        status: pass
    human_judgment: false
  - id: D4
    description: "Merged/Layer badge + four categorical layer badges re-derive correctly (correct light/dark hues) after a project switch (D-10)"
    requirement: "D-10"
    verification: []
    human_judgment: true
    rationale: "Visual badge re-derivation across a live switch needs the eye — the Svelte component-test harness stays out of appetite this phase (VALIDATION); the load seam is unit-covered (D1/D2) but the rendered badge hues/re-render are manual UAT."
  - id: D5
    description: "Switching to a project with no constitution shows the 'No constitution for this project' empty state (no stale badges/sections); Loading (Skeleton) and Error (Retry) states render per the State Matrix"
    requirement: "D-10"
    verification: []
    human_judgment: true
    rationale: "Visual/interaction states (skeleton-on-switch, empty-state copy, retry) require human visual confirmation; no component harness in appetite."

duration: 13 min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 13: Constitution View D-10 Polish Summary

**Constitution view load-ified into a universal `+page.ts` that streams `{ constitution, provenance, loadError }` off one RPC, so the four categorical layer badges + Merged/Layer badge re-derive from reloaded `provenance` and the "No constitution for this project" empty state renders correctly on every project switch — pinned by a Vitest-only load-seam test.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-07-12T10:52:00Z (approx)
- **Completed:** 2026-07-12T11:05:00Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 rewritten)

## Accomplishments
- Moved the `getConstitution` fetch out of the component-body `$effect` anti-pattern (RESEARCH Pitfall 4) into a universal `+page.ts` `load()` that `await parent()` + `depends('app:project')` — the mechanism that makes `invalidateAll()` re-run it on switch (D-01/D-02/D-09).
- Rewrote `constitution/+page.svelte` to consume streamed `data`, deriving the four categorical layer badges (User→blue, Org→amber, Project→green, Domain→violet fixed light/dark pairs) and the neutral Merged/Layer `Badge variant="secondary"` from the reloaded `provenance` — no local `$state`, so no stale prior-project badges linger (D-10 / T-05-05).
- Rendered the UI-SPEC State Matrix: Loading (Skeleton meta + rows via streamed `{#await}` skeleton-on-switch), Empty ("No constitution for this project" card), Error (inline Retry card reading `loadError`).
- Deleted the scoped `<style>` block and every hex color; the view is now Tailwind tokens + shadcn `Badge`/`Card`/`Skeleton`/`Button` only. Per-page breadcrumb stays removed (layout owns it, D-11).
- Added a Vitest-only load-seam unit test (no component harness) pinning `depends('app:project')` registration + response→`{constitution,provenance}` mapping + empty seam + `loadError` — 4 tests, all green.

## Task Commits

1. **Task 1: Constitution load() — constitution/+page.ts** - `7817998e` (feat)
2. **Task 2: Restyle constitution/+page.svelte — provenance-derived badges + empty state + skeleton** - `faf2a4ab` (feat)
3. **Task 3: Vitest load-seam unit test** - `9a648e51` (test)

_TDD note: Task 3's `load()` seam already existed (built in Task 1 of the same plan), so the test is a characterization/contract test that passes against the existing implementation rather than a RED→GREEN drive. No RED failure was expected or manufactured._

## Files Created/Modified
- `web/src/routes/constitution/+page.ts` (created) - Universal `PageLoad` streaming `{ constitution, provenance, loadError }` promise fields off one `getConstitution` RPC; `depends('app:project')`; errors caught → `loadError`.
- `web/src/routes/constitution/+page.svelte` (rewritten) - Consumes streamed `data` via a `$derived` `Promise.all`; provenance-derived categorical layer badges; empty/loading/error states; scoped `<style>`/hex removed.
- `web/src/lib/constitution-load.test.ts` (created) - Vitest-only load-seam test: `depends('app:project')` once, data mapping, empty seam, `loadError` on reject.

## Decisions Made
- **Streamed per-field promises, not a `{ detail }` wrapper.** Task 1's literal return shape (`{ constitution, provenance }`) and Task 2's requirement to derive from `data.provenance` conflicted with the sibling pages' `{ detail: promise }` wrapper (which would force `data.detail`, not `data.provenance`). Resolved by returning `constitution`/`provenance`/`loadError` as three top-level promises off the same RPC, combined in a stable `$derived(Promise.all(...))` so `{#await}` re-suspends to Skeleton on `invalidateAll()` while the component still references `data.provenance` directly. Task 1's `+page.ts` commit was amended (local, unpushed) to this shape before Task 2.
- **`layerOf()`/`layerBadge` take `provenance` as a param** (fed `d.provenance` inside `{#await ... then d}`) instead of closing over module state — structurally guarantees badges re-derive from reloaded provenance.

## Deviations from Plan

None affecting code — plan executed as written. Two verification-tooling notes (logged to `deferred-items.md`, not code changes):

1. **[Verify-command bug — not code] Task 2's `node -e` grep is broken by shell escaping.** The double-quoted `node -e "...!/\$props\(\)/..."` collapses `\$`→`$` under zsh/bash, turning the check into the `$`-anchor `/$props()/` that never matches — so the plan's command reports failure for ANY file. The `+page.svelte` genuinely satisfies every intended check; confirmed via the single-quoted equivalent (`$props()`, `data.provenance`, `Skeleton` present; no `<style>`/hex/`onMount`/`class="breadcrumb"`). Verify-command bug only; no impact on the delivered code.

## Issues Encountered
- **`constitution-load.test.ts` initially tripped the Task 3 `node -e /testing-library/` guard** because a code comment literally spelled out "@testing-library/svelte" when stating it was NOT used. Reworded the comment to "Svelte component harness"; the test uses only Vitest. Resolved before commit.

## Deferred Issues
- **Pre-existing `dprint fmt` drift in `.planning/intel/classifications/*.json` (139 files) blocks `task check` at `fmt:check`.** Wholly unrelated to plan 05-13 (which touches only three files under `web/`; `dprint check` finds no files to format for them). `task web:build`, `task build`, `pnpm build`, and `pnpm test` (20 tests, incl. 4 new) all pass. Logged in `deferred-items.md` under `## 05-13`. Remediation: `dprint fmt` on `.planning/`.

## Next Phase Readiness
- Plan 13 of 13 — Phase 05 execution complete. Recommend `/gsd-verify-work 05` for the D-10 manual UAT (switch between a project with a constitution and one without: badges/sections re-derive, empty state appears with no stale content, layer hues correct in light + dark).

## Self-Check: PASSED
- Files verified on disk: `web/src/routes/constitution/+page.ts`, `web/src/routes/constitution/+page.svelte`, `web/src/lib/constitution-load.test.ts` — all FOUND.
- Commits verified: `7817998e`, `faf2a4ab`, `9a648e51` — all FOUND.
- `pnpm build` ✓, `pnpm test` ✓ (20 passed), `task web:build` ✓, `task build` ✓.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
