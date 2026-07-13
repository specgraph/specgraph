---
phase: 05-ui-project-selector-and-refinements
verified: 2026-07-12T17:00:58Z
status: human_needed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 2/3
  gaps_closed:
    - "The full web UI is migrated to shadcn-svelte (Tailwind v4, Slate theme) with light/dark mode (D-12/D-13/D-14) — spec & decision detail pages migrated off the light-only #1a1a2e/#2563eb palette onto shadcn theme tokens (plan 05-14)."
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "In the running UI, toggle dark mode and open a Spec detail (/spec/[slug]) and a Decision detail (/decision/[slug]) page (post-migration)."
    expected: "All body content (headings, meta labels, body text, tables, blockquotes, approaches, lifecycle banners, conversation roles, spec chips, decision-status badge) is readable with proper contrast in BOTH light and dark mode."
    why_human: "Dark-mode contrast/readability is a perceptual judgment. Grep confirms the old light-only hex is gone and shadcn theme tokens (text-foreground/text-muted-foreground/bg-muted/border-border) are wired, but the on-screen contrast of the freshly-migrated markup needs a human eye."
  - test: "D-10 constitution switch UAT: switch between a project WITH a constitution and one WITHOUT."
    expected: "Merged/Layer badges and sections re-derive with no stale prior-project content; the 'No constitution found for this project' empty state appears; layer hues correct in light + dark."
    why_human: "Real-time project-switch re-derivation and badge hues are visual/runtime behavior; the loader wiring is verified in code but the on-screen result needs UAT."
  - test: "Project-switch skeleton-on-switch across dashboard/graph/spec/decision/constitution with >1 project available."
    expected: "Selecting a different project re-suspends each view to its Skeleton, then renders the new project's data with the correct X-Specgraph-Project scoping; no stale content."
    why_human: "invalidateAll() + streamed {#await} re-suspend is verified in code, but the live re-fetch/skeleton transition is runtime behavior best confirmed against a real server with multiple projects."
---

# Phase 5: UI Project Selector & Refinements — Verification Report

**Phase Goal:** The web UI lets users select which project they're viewing (with a sensible default) and surfaces project-specific views/refinements (constitution, etc.) instead of assuming a single implicit project.
**Verified:** 2026-07-12T17:00:58Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure (plan 05-14)

## Goal Achievement

The prior pass (`gaps_found`, 2/3) had a single code-level gap: truth #3 — the Spec detail
and Decision detail pages still carried light-only navy `#1a1a2e` / blue `#2563eb` plain-CSS
`<style>` blocks with no `.dark` override, so their body content was unreadable in dark mode.

Gap-closure plan **05-14** migrated both pages off the old palette. This re-verification
confirms the gap is **closed at the code level**: the old-palette hex is gone (0 matches in
each file), both `<style>` blocks are deleted, and body content is now styled with shadcn
theme-token Tailwind utilities that flip under `.dark`. The categorical carve-outs
(`sliceStatusBadge()` slice chips, `statusBadgeClass()` decision badge) are preserved
untouched, and no `{@html}`/`innerHTML`/`eval` was introduced. `pnpm build` exits 0 and
`pnpm test` passes 20/20.

All three must-haves now pass at the code level (**3/3**). The only remaining item is the
**dark-mode visual-contrast UAT** — a perceptual confirmation that the migrated markup
actually renders readable in both themes, which cannot be asserted in code. It is carried
forward (with two pre-existing runtime UATs) as `human_verification`, so the status is
`human_needed`, not `passed`. Per the framework, this is "automated checks passed, awaiting
human verification" — it does **not** fail the phase.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A user can pick the active project with a sensible default (last-used → `default` → alpha-first), and switching re-fetches every project-scoped view (D-01..D-08) | ✓ VERIFIED | `project.svelte.ts` `loadProjects()` implements case-insensitive sort (D-05, L29) + D-04 precedence + stale fallback (D-06), proven by `project.test.ts` (6/6 pass). `+layout.svelte` `switchProject()` → `invalidateAll()` (D-01/D-03, L34/L44); `Select` for >1 project, static label for 1 (D-08). `client.ts` `projectInterceptor` re-issues `X-Specgraph-Project` from `project.current` on every RPC (L15-16). No regression from 05-14. |
| 2 | Project-scoped views reflect the selected project with correct empty/error states + active-project indicator; constitution badges re-derive across switches (D-09/D-10/D-11) | ✓ VERIFIED | All 5 scoped views carry `depends('app:project')` loads (dashboard, graph, spec, decision, constitution) — keys excluded (D-09). Constitution badges derive from reloaded `data.provenance` with no local `$state` → no stale badges on switch (D-10). Active-project breadcrumb owned by layout (D-11). No regression from 05-14. |
| 3 | The full web UI is migrated to shadcn-svelte (Tailwind v4, Slate theme) with light/dark mode (D-12/D-13/D-14) | ✓ VERIFIED | **Gap closed by 05-14.** `grep -c '#1a1a2e\|#2563eb'` = 0 on both `spec/[...slug]/+page.svelte` and `decision/[...slug]/+page.svelte`; both `<style>` blocks deleted (0 matches). Body content now on shadcn tokens (spec: `text-foreground`×10, `text-muted-foreground`×17, `bg-muted`×7, `border-border`×4; decision: `text-foreground`×6, `text-muted-foreground`×5). Categorical carve-outs preserved (`sliceStatusBadge()` 4 semantic chip hex unchanged; `statusBadgeClass()` badge unchanged). `pnpm build` exit 0, `pnpm test` 20/20. Dark-mode visual contrast → human_verification (perceptual, not code-assertable). |

**Score:** 3/3 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/src/routes/spec/[...slug]/+page.svelte` | shadcn migration incl. dark mode (D-12/D-13) | ✓ VERIFIED | 0 old-palette hex; 0 `<style>` blocks; token utilities present; `sliceStatusBadge()` categorical chips preserved (L77-82); no `{@html}`. |
| `web/src/routes/decision/[...slug]/+page.svelte` | shadcn migration incl. dark mode (D-12/D-13) | ✓ VERIFIED | 0 old-palette hex; 0 `<style>` blocks; token utilities present; `statusBadgeClass()` badge preserved (L8/L61); no `{@html}`. |
| `web/src/lib/project.svelte.ts` | D-04/05/06 precedence + sort + fallback | ✓ VERIFIED | loadProjects sorts case-insensitive + 4-tier precedence; wired via client.ts interceptor + layout. |
| `web/src/lib/project.test.ts` | Unit tests for D-04/05/06 + zero-projects | ✓ VERIFIED | 6 tests pass. |
| `web/src/routes/+layout.svelte` / `+layout.ts` | Selector, switch→invalidateAll, breadcrumb, dark mode, empty state | ✓ VERIFIED | Select+label (D-08), switchProject invalidateAll (D-01/D-03), ModeWatcher (D-14). |
| `web/src/lib/api/client.ts` | X-Specgraph-Project interceptor | ✓ VERIFIED | projectInterceptor reads reactive `project.current`, fallback `'default'`. |
| `web/src/routes/{+page,graph,constitution,spec,decision}/+page.ts` | load()-based project-scoped fetch | ✓ VERIFIED | All 5 register `depends('app:project')`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `switchProject` | every `+page.ts` load() | `invalidateAll()` re-runs loads | ✓ WIRED | +layout.svelte:44 invalidateAll; each load registers `depends('app:project')`. |
| `project.current` | backend | `X-Specgraph-Project` header | ✓ WIRED | projectInterceptor sets header from reactive current on each RPC. |
| `data.provenance` | constitution badges | derive from reloaded provenance | ✓ WIRED | No local $state — re-derives on switch (D-10). |
| spec/decision detail markup | `.dark` theme | shadcn theme-token utilities | ✓ WIRED | Body content on text-foreground/text-muted-foreground/bg-muted/border-border — flips under `.dark` (05-14). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Web unit tests | `pnpm test` | 20/20 pass (project 6, oidc 2, constitution-load 4, keys 8) | ✓ PASS |
| Production build (static adapter) | `pnpm build` | built in 2.84s, adapter-static wrote `build/`, exit 0 | ✓ PASS |
| Old-palette removed (spec) | `grep -c '#1a1a2e\|#2563eb' spec/[...slug]/+page.svelte` | 0 | ✓ PASS |
| Old-palette removed (decision) | `grep -c '#1a1a2e\|#2563eb' decision/[...slug]/+page.svelte` | 0 | ✓ PASS |
| Categorical carve-outs preserved | `sliceStatusBadge()` (4 semantic chip hex) + `statusBadgeClass()` unchanged | preserved | ✓ PASS |
| XSS-clean | `grep -Ec '@html|innerHTML|eval\(' ` on both files | 0 | ✓ PASS |

### Requirements Coverage (D-01..D-14 scope contract)

| Decision | Description | Status | Evidence |
|----------|-------------|--------|----------|
| D-01 | Switch → invalidateAll() re-fetch | ✓ | switchProject + depends('app:project') on all 5 loads |
| D-02 | Client universal load() (ssr=false) | ✓ | +layout.ts ssr=false; +page.ts await parent() |
| D-03 | Setter persists localStorage then invalidate | ✓ | project setter + switchProject |
| D-04 | Default precedence last-used→default→alpha | ✓ | loadProjects + tests |
| D-05 | Case-insensitive alpha sort | ✓ | loadProjects sort (L29) + test |
| D-06 | Stale saved project auto-fallback | ✓ | "in available" guard + test |
| D-07 | Zero-projects empty state | ✓ | layout empty state; current='' seam |
| D-08 | 1 project → label, >1 → dropdown | ✓ | layout Select vs span |
| D-09 | Scoped views load-ified; Keys excluded | ✓ | 5 loads present; keys has no load |
| D-10 | Constitution polish: badges/empty re-derive | ✓ | provenance-derived badges, empty state, Skeleton |
| D-11 | Active-project indicator on pages | ✓ | layout breadcrumb |
| D-12 | Full component + page migration | ✓ | 16 components + all 6 pages migrated (spec/decision detail closed by 05-14) |
| D-13 | Adopt shadcn palette (replace #1a1a2e/#2563eb) | ✓ | old palette removed from spec/decision detail (0 matches); tokens in use |
| D-14 | Light + dark mode with toggle | ✓ (code) | Toggle + tokens work on all pages incl. spec/decision detail; dark-mode visual contrast → human_verification |

All 14 decisions (D-01..D-14) accounted for against 05-CONTEXT.md and the 05-14 SUMMARY. No orphaned or unclaimed decisions.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | Prior light-only-hex warnings (spec L417-635, decision L102-165) | ✓ RESOLVED | `<style>` blocks removed by 05-14; 0 old-palette hex remain |

No debt markers (TODO/FIXME/XXX) in phase-authored `web/src`. No `{@html}`/`innerHTML`/`eval` anywhere (XSS clean). The 4 residual hex in the spec page are the `sliceStatusBadge()` categorical slice-status chip colors (open/claimed/done) — legitimate semantic data-viz colors, explicitly out of the D-13 gap scope.

### Human Verification Required

1. **Dark-mode contrast on migrated detail pages** — Toggle dark mode and open a Spec detail (`/spec/[slug]`) and a Decision detail (`/decision/[slug]`) page.
   - Expected: All body content (headings, meta labels, body text, tables, blockquotes, approaches, lifecycle banners, conversation roles, spec chips, decision-status badge) is readable with proper contrast in BOTH light and dark mode.
   - Why human: Perceptual contrast judgment; code confirms old hex removed and tokens wired, but on-screen readability needs a human eye.

2. **D-10 constitution switch UAT** — Switch between a project WITH a constitution and one WITHOUT.
   - Expected: Merged/Layer badges + sections re-derive, no stale content; empty state appears; layer hues correct in light + dark.
   - Why human: Real-time re-derivation + badge hues are visual/runtime behavior.

3. **Project-switch skeleton-on-switch** — Across dashboard/graph/spec/decision/constitution with >1 project.
   - Expected: Each view re-suspends to Skeleton then renders new project's data with correct scoping; no stale content.
   - Why human: Live re-fetch/skeleton transition best confirmed against a real multi-project server.

### Gaps Summary

No code-level gaps remain. The sole prior gap (truth #3 dark-mode migration of the two
detail pages) is closed by plan 05-14: old palette removed, `<style>` blocks deleted, shadcn
theme tokens wired, categorical carve-outs preserved, build + 20/20 tests green. Status is
`human_needed` solely because dark-mode visual contrast (plus two pre-existing runtime UATs)
requires human confirmation — the phase is not failed.

---

_Verified: 2026-07-12T17:00:58Z_
_Verifier: the agent (gsd-verifier)_
