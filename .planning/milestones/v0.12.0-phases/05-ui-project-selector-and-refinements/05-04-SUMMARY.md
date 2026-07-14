---
phase: 05-ui-project-selector-and-refinements
plan: 04
subsystem: ui
tags: [svelte, shadcn-svelte, tailwind, slate-tokens, accordion, tabs, badge, card]

# Dependency graph
requires:
  - phase: 05-01
    provides: shadcn-svelte primitives (Accordion, Tabs, Badge, Card), Slate OKLCH tokens, badge-variants.ts categorical severity map
provides:
  - AccordionSection on shadcn Accordion + Badge (tokenized, contract unchanged)
  - TabBar on shadcn Tabs (line variant, --primary active accent)
  - FindingsSection on shadcn Card + categorical severity Badge (D-10 palette)
affects: [05-10, 05-11, 05-12, 05-13, constitution page, spec-detail page]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Structural component migration: keep Props/Snippet + Svelte 5 runes identical, delete scoped <style>, re-express markup on shadcn primitives with Slate token utilities"
    - "Categorical severity badges consume shared badge-variants.ts map (fixed light/dark pairs), never --primary"

key-files:
  created: []
  modified:
    - web/src/lib/components/AccordionSection.svelte
    - web/src/lib/components/TabBar.svelte
    - web/src/lib/components/FindingsSection.svelte

key-decisions:
  - "AccordionSection maps its expanded/toggled runes to a controlled single-item shadcn Accordion via value + onValueChange, preserving the exact external Props contract"
  - "TabBar uses the shadcn Tabs 'line' variant with data-active:text-primary/after:bg-primary to honor the UI-SPEC accent rule while keeping the underline aesthetic"
  - "FindingsSection reuses severityBadgeClass('warning') for the amber finding-count badge and a categorical green pair for the 'passed' badge; per-finding severity badges pull from the shared badge-variants severity map (D-10)"

patterns-established:
  - "Component migration invariant: no <style>, no 6-digit hex, unchanged prop/Snippet contract so call-sites need no edits"

requirements-completed: [D-12, D-13, D-10]

coverage:
  - id: D1
    description: "AccordionSection migrated to shadcn Accordion + Badge with Slate tokens; no scoped style or hex; Props/Snippet contract unchanged"
    requirement: "D-12"
    verification:
      - kind: other
        ref: "node style/hex/Accordion guard + pnpm build (Task 1 <verify>)"
        status: pass
    human_judgment: false
  - id: D2
    description: "TabBar migrated to shadcn Tabs with --primary active accent; tabs/active/onchange props preserved; no scoped style or hex"
    requirement: "D-13"
    verification:
      - kind: other
        ref: "node style/hex/Tabs guard + pnpm build (Task 2 <verify>)"
        status: pass
    human_judgment: false
  - id: D3
    description: "FindingsSection migrated to shadcn Card + categorical severity Badge from shared badge-variants map (not --primary), per the D-10 palette carve-out"
    requirement: "D-10"
    verification:
      - kind: other
        ref: "node style/hex/Badge guard + pnpm build (Task 3 <verify>)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Accordions, tabs, and findings render correctly in light and dark mode"
    verification: []
    human_judgment: true
    rationale: "Visual/theme correctness across light and dark requires human judgment; build guards prove structure but not rendered appearance"

# Metrics
duration: 6min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 04: Structural Component Migration Summary

**AccordionSection, TabBar, and FindingsSection migrated to shadcn-svelte primitives with Slate tokens — zero call-site churn, with FindingsSection severity badges honoring the D-10 categorical palette carve-out.**

## Performance

- **Duration:** ~6 min
- **Completed:** 2026-07-12
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- `AccordionSection` re-implemented on shadcn `Accordion` + `Badge`; internal expanded/toggled runes map to a controlled single-item accordion, preserving the exact `{ title, expanded, badge, children }` Props/Snippet contract.
- `TabBar` re-implemented on shadcn `Tabs` (line variant) with a `--primary` active accent; `{ tabs, active, onchange }` props and behavior preserved.
- `FindingsSection` re-implemented on shadcn `Card` with per-pass grouping intact; per-finding severity badges pull fixed light/dark class pairs from the shared `badge-variants.ts` severity map (not `--primary`), satisfying the D-10 categorical carve-out.
- All three files: scoped `<style>` blocks and every 6-digit hex removed, replaced with Slate token / categorical utility classes.

## Task Commits

1. **Task 1: Migrate AccordionSection** - `db463de8` (feat)
2. **Task 2: Migrate TabBar** - `69dc83e5` (feat)
3. **Task 3: Migrate FindingsSection** - `ff8ca03a` (feat)

## Files Created/Modified
- `web/src/lib/components/AccordionSection.svelte` - shadcn Accordion + Badge, tokenized
- `web/src/lib/components/TabBar.svelte` - shadcn Tabs line variant, --primary active accent
- `web/src/lib/components/FindingsSection.svelte` - shadcn Card + categorical severity Badge

## Decisions Made
- Mapped AccordionSection's existing `toggled`/`open` runes onto a controlled single-item shadcn Accordion (`value` + `onValueChange`) rather than replacing the state model, keeping the external contract byte-for-byte and letting the primitive supply its own chevron.
- Used the Tabs `line` variant to keep the original underline affordance while overriding the active indicator to `--primary` per the UI-SPEC accent rule.
- For FindingsSection status badges: amber "N findings" reuses the categorical `severityBadgeClass('warning')` pair; the "passed" state uses a categorical green light/dark pair; card left-border uses `border-l-amber-500` / `border-l-green-500` utilities — no hex, no `--primary` misuse for categorical data.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. `pnpm build` succeeded after each task; commits passed pre-commit hooks (lint/format) without `--no-verify`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- These structural components now render on tokenized shadcn primitives with unchanged contracts, so Wave 3 pages (05-10..05-13) can restyle their views without editing AccordionSection/TabBar/FindingsSection call-sites.
- Light/dark visual verification remains a human UAT item (coverage D4).

## Self-Check: PASSED
- `web/src/lib/components/AccordionSection.svelte` — FOUND
- `web/src/lib/components/TabBar.svelte` — FOUND
- `web/src/lib/components/FindingsSection.svelte` — FOUND
- Commit `db463de8` — FOUND
- Commit `69dc83e5` — FOUND
- Commit `ff8ca03a` — FOUND

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
