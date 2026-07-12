---
phase: 05-ui-project-selector-and-refinements
plan: 05
subsystem: ui
tags: [svelte, shadcn, tailwind, slate-tokens, badge, data-display]

requires:
  - phase: 05-ui-project-selector-and-refinements
    provides: "badge-variants.ts categorical stage/priority/status/severity map, shadcn ui/ primitives (Table, Card, Badge), Slate token theme (05-01)"
provides:
  - "SpecTable on shadcn Table with categorical stage Badge + accent slug links"
  - "StatsBar as a shadcn Card grid with Display-size (text-3xl) numbers"
  - "FunnelBar on token-utility segments (--primary opacity) + categorical Badge counts"
affects: [dashboard, wave-3-dashboard-loadification]

tech-stack:
  added: []
  patterns:
    - "Data-display components consume shadcn ui/ primitives + Slate tokens; no scoped <style>, no hex"
    - "Categorical stage encodings use the shared stageBadgeClass map (D-10 carve-out), not theme accent"
    - "Table/decision slug hyperlinks are the reserved --primary accent use (UI-SPEC accent rule #5)"

key-files:
  created: []
  modified:
    - web/src/lib/components/SpecTable.svelte
    - web/src/lib/components/StatsBar.svelte
    - web/src/lib/components/FunnelBar.svelte

key-decisions:
  - "Priority column rendered as a neutral Badge variant=secondary (badge-variants.ts has no priority palette; keeps it inside the token budget)"
  - "FunnelBar segment fills use --primary at descending opacity steps (muted/accent token rule); stage identity/colour is carried by the categorical stageBadgeClass legend Badges rather than the fill"
  - "$lib/colors.ts left intact — still consumed by Graph.svelte; only FunnelBar's dependency on it was removed"

patterns-established:
  - "Categorical-vs-accent split: stage badges use badge-variants map, slug links use --primary, everything else muted/secondary"
  - "Funnel/segment charts encode magnitude via accent opacity and category via Badge chips"

requirements-completed: [D-12, D-13, D-10]

coverage:
  - id: D1
    description: "SpecTable migrated to shadcn Table with categorical stage Badge, secondary priority Badge, accent slug links; sort runes/props preserved"
    requirement: "D-12"
    verification:
      - kind: automated_ui
        ref: "node guard (no <style>, no 6-digit hex, contains Table) + pnpm build"
        status: pass
    human_judgment: true
    rationale: "Table sorting interaction and both-theme visual correctness need a human to click through (plan verification: manual UAT of table sorting)"
  - id: D2
    description: "StatsBar migrated to a grid of shadcn Cards with Display-size (text-3xl) numbers and gap-6 spacing; cards $derived preserved"
    requirement: "D-13"
    verification:
      - kind: automated_ui
        ref: "node guard (no <style>, no hex, contains Card) + pnpm build"
        status: pass
    human_judgment: true
    rationale: "Card layout/typography adequacy across light+dark themes is a visual judgment (plan: manual UAT of stat cards)"
  - id: D3
    description: "FunnelBar migrated to token-utility div segments (--primary opacity) + categorical Badge counts; props/runes preserved"
    requirement: "D-10"
    verification:
      - kind: automated_ui
        ref: "node guard (no <style>, no hex) + pnpm build"
        status: pass
    human_judgment: true
    rationale: "Funnel segment readability and colour distinction in both themes needs human visual sign-off (plan: manual UAT of funnel render)"

duration: 4min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 05: Data-Display Component Migration Summary

**SpecTable, StatsBar, and FunnelBar re-built on shadcn Table/Card/Badge primitives + Slate tokens — categorical stage badges via the shared badge-variants map, accent-reserved slug links, and Display-size stat numbers — with all sort runes and props preserved.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-12T14:04:53Z
- **Completed:** 2026-07-12T14:08:31Z
- **Tasks:** 3
- **Files modified:** 3 (+ deferred-items.md log)

## Accomplishments
- SpecTable now renders on shadcn `Table` with categorical stage `Badge` (stageBadgeClass), a neutral secondary priority `Badge`, and `--primary` accent slug hyperlinks — sort `$state`/`$derived` runes and all props unchanged.
- StatsBar rebuilt as a `gap-6` grid of shadcn `Card`s; stat values use the Display role (`text-3xl` weight 600) per UI-SPEC Typography; per-card hex accents dropped for muted-foreground token labels.
- FunnelBar rebuilt from token-utility `div` segments (`--primary` at descending opacity) with categorical `stageBadgeClass` legend `Badge`s for counts; its `$lib/colors` hex dependency removed.
- No scoped `<style>` blocks and no 6-digit hex remain in any of the three components.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate SpecTable to shadcn Table + categorical stage Badge** — `8283e1ac` (feat)
2. **Task 2: Migrate StatsBar to a grid of shadcn Cards** — `afbc3be8` (feat)
3. **Task 3: Migrate FunnelBar to token div-segments + Badge counts** — `1b198098` (feat)

**Plan metadata:** _(docs commit — see below)_

## Files Created/Modified
- `web/src/lib/components/SpecTable.svelte` — shadcn Table + categorical stage/secondary priority Badge, accent slug links
- `web/src/lib/components/StatsBar.svelte` — grid of shadcn Card, Display-size numbers
- `web/src/lib/components/FunnelBar.svelte` — token `--primary` opacity segments + categorical Badge counts
- `.planning/phases/05-ui-project-selector-and-refinements/deferred-items.md` — logged pre-existing dprint drift

## Decisions Made
- **Priority as neutral `Badge variant="secondary"`.** `badge-variants.ts` defines no priority palette; the plan's "stage/priority categorical" intent is satisfied for stage via `stageBadgeClass`, while priority uses a token-budget-safe neutral chip rather than inventing a new categorical hue set.
- **FunnelBar fills via accent opacity, colour identity via Badge chips.** Honors the UI-SPEC "segments use muted/accent tokens" rule literally (`bg-primary/75…/20`) while keeping stages distinguishable through the categorical `stageBadgeClass` legend (D-10).
- **Kept `$lib/colors.ts`.** Still consumed by `Graph.svelte`; only FunnelBar's import was removed. No orphan.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- **`task check` fails at `fmt:check`** on 139 pre-existing unformatted `.planning/intel/classifications/*.json` (138) + `.planning/research/.cache` (1) files. These are untouched by this plan (my three commits modify only the Svelte files), `dprint` has no Svelte plugin, and `task web:build` / `task build` / `pnpm build` / `pnpm test` (16 tests) all pass. Out of scope per the scope boundary; logged to `deferred-items.md` (same drift already recorded for 05-02/05-03). Remediation: `dprint fmt` on `.planning/intel/`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Three data-display components migrated with unchanged contracts, ready for the Wave 3 dashboard load-ification restyle without prop changes.
- Manual UAT recommended at end-of-phase: table sorting, stat card layout, and funnel render in both light and dark themes.

## Self-Check: PASSED

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
