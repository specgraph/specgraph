---
phase: 05-ui-project-selector-and-refinements
plan: 14
subsystem: ui
tags: [svelte, sveltekit, tailwind, shadcn, dark-mode, theme-tokens]

# Dependency graph
requires:
  - phase: 05-ui-project-selector-and-refinements
    provides: "shadcn theme-token migration of dashboard/graph/keys/constitution pages and Card/Badge/Skeleton/Retry shells, plus the badge-variants.ts categorical class-pair convention"
provides:
  - "Spec detail page (/spec/[slug]) body content migrated off the light-only navy/blue plain-CSS palette onto shadcn theme-token utilities that flip under the .dark class"
  - "Decision detail page (/decision/[slug]) body content migrated onto the same shadcn theme-token utilities"
  - "Both remaining project-scoped detail views now render readable in both light and dark mode from one markup source"
affects: [ui, dark-mode, theme]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Body content styled with Tailwind utilities on shadcn theme tokens (text-foreground / text-muted-foreground / bg-muted / bg-card / border-border) instead of per-page <style> blocks"
    - "Categorical/semantic accents (lifecycle banners, conversation roles, decision markers, spec chips) use explicit light/dark Tailwind class pairs per the badge-variants.ts convention"

key-files:
  created: []
  modified:
    - "web/src/routes/spec/[...slug]/+page.svelte"
    - "web/src/routes/decision/[...slug]/+page.svelte"

key-decisions:
  - "Removed both pages' <style> blocks entirely — every styled selector was expressible as a Tailwind token utility on the markup (mirroring the already-migrated constitution page, which carries zero <style>)."
  - "Lifecycle banners, conversation roles, decision markers, and decision spec chips are semantic/categorical accents, so each got an explicit light/dark Tailwind class pair (not a single theme token) to stay readable in both themes."
  - "Replaced the hand-rolled .load-changelog-btn with the already-imported shadcn Button (variant=outline size=sm), dropping the var(--accent-color) fallback."

patterns-established:
  - "Detail-page body content follows the constitution page token idiom: token utilities on markup, no page-local <style> block."

requirements-completed: [D-12, D-13, D-14]

coverage:
  - id: D1
    description: "Spec detail page (/spec/[slug]) body content migrated off the old light-only navy/blue palette onto shadcn theme-token utilities; old-palette hex removed; categorical slice-status chips preserved."
    requirement: "D-14"
    verification:
      - kind: other
        ref: "grep -Eoc '#1a1a2e|#2563eb|#374151|#475569|#64748b|#f8fafc|#eff6ff|#dbeafe|#fef3c7' web/src/routes/spec/[...slug]/+page.svelte == 0"
        status: pass
      - kind: other
        ref: "presence of text-foreground / text-muted-foreground / bg-muted / border-border in web/src/routes/spec/[...slug]/+page.svelte"
        status: pass
      - kind: e2e
        ref: "cd web && pnpm build (adapter-static writes build/)"
        status: pass
      - kind: unit
        ref: "cd web && pnpm test (20/20)"
        status: pass
    human_judgment: true
    rationale: "Dark-mode visual contrast (headings, meta labels, tables, blockquotes, approaches, lifecycle banners readable in BOTH themes) is a perceptual judgment the automated greps/build/test cannot assert — carried forward as the plan's human_verification note."
  - id: D2
    description: "Decision detail page (/decision/[slug]) body content migrated onto shadcn theme-token utilities; old-palette hex removed; categorical decision-status badge preserved."
    requirement: "D-14"
    verification:
      - kind: other
        ref: "grep -Eoc '#1a1a2e|#2563eb|#374151|#64748b|#eff6ff|#dbeafe' web/src/routes/decision/[...slug]/+page.svelte == 0"
        status: pass
      - kind: other
        ref: "presence of text-foreground / text-muted-foreground in web/src/routes/decision/[...slug]/+page.svelte"
        status: pass
      - kind: e2e
        ref: "cd web && pnpm build"
        status: pass
      - kind: unit
        ref: "cd web && pnpm test (20/20)"
        status: pass
    human_judgment: true
    rationale: "Dark-mode visual contrast (title, section headings, meta labels, body text, spec chips readable in BOTH themes) is a perceptual judgment no automated check asserts — carried forward as the plan's human_verification note."

# Metrics
duration: 4min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 14: Dark-mode token migration of Spec & Decision detail pages Summary

**Migrated the Spec (`/spec/[slug]`) and Decision (`/decision/[slug]`) detail pages' body content off their light-only navy `#1a1a2e` / blue `#2563eb` plain-CSS `<style>` blocks onto shadcn theme-token Tailwind utilities, closing the last dark-mode readability gap from 05-VERIFICATION.md.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-12T16:52:11Z
- **Completed:** 2026-07-12T16:56:58Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Spec detail page body content (h1/h3/h4 headings, meta labels, notes, blockquotes, approaches, interfaces/tables/code, slice cards, conversation logs, lifecycle banners, changelog trigger) rendered with shadcn theme-token utilities that flip under `.dark`.
- Decision detail page body content (title, meta table, section headings, body prose, spec chips) rendered with the same token idiom.
- Both pages' page-local `<style>` blocks removed entirely (spec: −287 CSS lines, decision: −83), matching the constitution page's zero-`<style>` reference.
- Categorical data-viz colors preserved untouched: `sliceStatusBadge()` slice chips and the `statusBadgeClass()` decision-status badge.

## Task Commits

Each task was committed atomically (DCO signed-off):

1. **Task 1: Migrate Spec detail page body content** — `5024351e` (fix)
2. **Task 2: Migrate Decision detail page body content** — `951b583e` (fix)

## Files Created/Modified
- `web/src/routes/spec/[...slug]/+page.svelte` — body content moved from the plain-CSS `<style>` block to Tailwind token utilities on the markup; old light-only palette hex removed; `<style>` block deleted.
- `web/src/routes/decision/[...slug]/+page.svelte` — same migration; `<style>` block deleted.

## Decisions Made
- Removed both `<style>` blocks entirely — every selector mapped cleanly to a token utility, so no residual page-local CSS was needed.
- Semantic/categorical accents (lifecycle banners, conversation probe/response roles, decision markers, decision spec chips) got explicit light/dark Tailwind class pairs (the `badge-variants.ts` convention) rather than a single theme token, so they stay readable in both themes without hardcoded light-only hex.
- Swapped the hand-rolled `.load-changelog-btn` for the already-imported shadcn `Button` (`variant="outline" size="sm"`), dropping the `var(--accent-color, #6366f1)` fallback.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- `requirements mark-complete D-12 D-13 D-14` reported these IDs as not found in REQUIREMENTS.md. D-12/D-13/D-14 are UI-SPEC deliverable IDs (referenced in 05-VERIFICATION.md / the plan frontmatter), not REQUIREMENTS.md requirement rows, so there is nothing to check off there. No impact on the code change; recorded in the SUMMARY `requirements-completed` field per the plan frontmatter.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The single verified gap from 05-VERIFICATION.md (truth #3, D-12/D-13/D-14) is closed in source. Ready for `/gsd-verify-work` — the remaining item is the human UAT confirmation of dark-mode contrast on both detail pages (carried in the coverage block as `human_judgment: true`).

## Self-Check: PASSED
- `web/src/routes/spec/[...slug]/+page.svelte` — exists, forbidden hex count 0, tokens present, slice chips preserved.
- `web/src/routes/decision/[...slug]/+page.svelte` — exists, forbidden hex count 0, tokens present, status badge preserved.
- Commit `5024351e` — found in git log.
- Commit `951b583e` — found in git log.
- `cd web && pnpm build` exits 0; `cd web && pnpm test` passes 20/20.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
