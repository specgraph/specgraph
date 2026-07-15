---
phase: 05-ui-project-selector-and-refinements
plan: 06
subsystem: ui
tags: [svelte, shadcn, tailwind, tokens, slate, badge, card, separator, input]

requires:
  - phase: 05-01
    provides: shadcn primitives (Input, Card, Badge, Separator), Slate tokens, badge-variants palette
provides:
  - SearchFilter migrated to shadcn Input with --ring focus token
  - MetadataBar migrated to shadcn Card + description list + neutral Badge
  - ChangelogTimeline migrated to token timeline with vertical Separator spine + shared stageBadgeClass Badges
affects: [05-wave-3, spec-detail, dashboard]

tech-stack:
  added: []
  patterns:
    - "Data-display components render on shadcn primitives + Slate tokens; no scoped <style>, no 6-digit hex"
    - "Categorical stage chips reuse shared badge-variants stageBadgeClass (D-10 carve-out)"
    - "Focus rings use --ring token (UI-SPEC accent rule #3)"

key-files:
  created: []
  modified:
    - web/src/lib/components/SearchFilter.svelte
    - web/src/lib/components/MetadataBar.svelte
    - web/src/lib/components/ChangelogTimeline.svelte

key-decisions:
  - "ChangelogTimeline stage badges reuse the shared stageBadgeClass palette instead of the old 11-stage inline hex map; unmapped stages (review/amended/superseded/abandoned) fall back to the neutral pair — consistent with SpecTable/FunnelBar"
  - "Vertical shadcn Separator used as the timeline spine (token --border) rather than a scoped ::before pseudo-element"
  - "MetadataBar provenance rendered as neutral Badge variant=secondary (not a categorical D-10 color)"

patterns-established:
  - "Timeline spine via <Separator orientation=vertical> positioned absolutely inside a relative padded container"

requirements-completed: [D-12, D-13]

coverage:
  - id: D1
    description: "SearchFilter re-implemented on shadcn Input; no <style>, no hex; focus ring via --ring; props (value/placeholder/onchange) preserved"
    requirement: "D-12"
    verification:
      - kind: unit
        ref: "node text-check: no <style>, no 6-digit hex, contains Input (SearchFilter.svelte)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build"
        status: pass
    human_judgment: false
  - id: D2
    description: "MetadataBar re-implemented on shadcn Card/Content + description list with neutral Badge; token muted-foreground labels; props preserved"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "node text-check: no <style>, no 6-digit hex, contains Card/Badge (MetadataBar.svelte)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build"
        status: pass
    human_judgment: false
  - id: D3
    description: "ChangelogTimeline re-implemented as token timeline with vertical Separator spine + shared stageBadgeClass/checkpoint Badges; expand toggle + props preserved"
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "node text-check: no <style>, no 6-digit hex, contains Separator/Badge (ChangelogTimeline.svelte)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build"
        status: pass
    human_judgment: false
  - id: D4
    description: "SearchFilter, MetadataBar, and ChangelogTimeline render correctly in both light and dark themes (visual/UX correctness)"
    verification: []
    human_judgment: true
    rationale: "Theme/contrast fidelity and timeline visual polish require human visual inspection; no automated assertion covers rendered appearance across both themes"

duration: 12 min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 06: Data-Display Component Migration Summary

**SearchFilter, MetadataBar, and ChangelogTimeline migrated to shadcn primitives (Input / Card / Badge / Separator) on Slate tokens — zero scoped `<style>`, zero hardcoded hex, contracts preserved.**

## Performance

- **Duration:** 12 min
- **Completed:** 2026-07-12
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- SearchFilter rebuilt on shadcn `Input`; focus ring now driven by the `--ring` token (UI-SPEC accent rule #3) instead of a hardcoded blue box-shadow.
- MetadataBar rebuilt as a shadcn `Card` + description list with `text-muted-foreground` labels and a neutral `Badge` for provenance; monospace hash retained via `font-mono`.
- ChangelogTimeline rebuilt as a token-styled timeline: vertical shadcn `Separator` spine, `--primary`/`--border` markers, and stage chips via the shared `stageBadgeClass` (badge-variants) plus a `Badge` checkpoint chip. The 11-entry inline hex palette was removed.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate SearchFilter to shadcn Input** - `da4fc592` (feat)
2. **Task 2: Migrate MetadataBar to Card/description list + Badge** - `5957c4ac` (feat)
3. **Task 3: Migrate ChangelogTimeline to token timeline + Separator + Badge** - `d4708b6d` (feat)

## Files Created/Modified
- `web/src/lib/components/SearchFilter.svelte` - shadcn Input, token focus ring, `<style>` removed
- `web/src/lib/components/MetadataBar.svelte` - shadcn Card + description list + Badge, `<style>` removed
- `web/src/lib/components/ChangelogTimeline.svelte` - token timeline + vertical Separator spine + shared stage Badges, `<style>` + inline hex palette removed

## Decisions Made
- **Shared stage palette over the old inline map:** ChangelogTimeline now consumes `stageBadgeClass` from `badge-variants.ts`, matching SpecTable/FunnelBar. Stages not present in the shared map (`review`, `amended`, `superseded`, `abandoned`) fall back to the neutral Slate pair rather than reintroducing bespoke hex colors — a deliberate consolidation onto the D-10 categorical palette.
- **Separator as timeline spine:** the vertical line uses `<Separator orientation="vertical">` (token `--border`) instead of a scoped `::before`, so the timeline stays theme-reactive with no scoped CSS.
- **Provenance as neutral badge:** provenance is metadata, not a D-10 categorical encoding, so it uses `Badge variant="secondary"` rather than a colored palette entry.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three data-display components are contract-stable and token-based, unblocking the Wave 3 restyle without further interface changes.
- `pnpm -C web build` passes. Manual UAT (D4) — visual rendering in both light/dark themes — remains for end-of-phase verification.

## Self-Check

- Files modified verified present on disk (3/3).
- Task commits verified in git log (`da4fc592`, `5957c4ac`, `d4708b6d`).
- `pnpm -C web build` succeeds; all three per-file text-checks pass.

## Self-Check: PASSED

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
