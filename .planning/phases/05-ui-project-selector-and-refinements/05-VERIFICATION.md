---
phase: 05-ui-project-selector-and-refinements
verified: 2026-07-12T15:18:14Z
status: gaps_found
score: 2/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "The full web UI is migrated to shadcn-svelte (Tailwind v4, Slate theme) with light/dark mode (D-12/D-13/D-14)"
    status: partial
    reason: >
      Spec detail (/spec/[slug]) and Decision detail (/decision/[slug]) retain
      large plain-CSS <style> blocks using the OLD navy #1a1a2e / blue #2563eb
      palette that D-13 explicitly said to replace with shadcn theme tokens.
      These are hardcoded LIGHT-ONLY hex values with no .dark override, so the
      body content of both pages (headings, body text, labels, chips, tables)
      renders low-contrast/near-invisible in dark mode — a D-14 violation on two
      of the five project-scoped views. The shells (Card/Badge/Skeleton/Retry
      states) and all other pages (dashboard, graph, keys, constitution) ARE
      fully migrated to tokens. The 05-11-SUMMARY claims requirements-completed
      includes D-13 for these pages; the code contradicts that claim.
    artifacts:
      - path: "web/src/routes/spec/[...slug]/+page.svelte"
        issue: "~25 hardcoded hex colors in the <style> block (L417-635): #1a1a2e headings, #374151/#475569 body text, #f8fafc/#eff6ff/#dbeafe/#fef3c7 light-only backgrounds. No dark-mode handling — body content unreadable in dark mode."
      - path: "web/src/routes/decision/[...slug]/+page.svelte"
        issue: "<style> block (L102-165): #1a1a2e headings, #374151 body text, #64748b labels, #eff6ff/#dbeafe chip backgrounds, #2563eb chip text. Light-only, no .dark override — body content unreadable in dark mode."
    missing:
      - "Convert the spec/[...slug]/+page.svelte body <style> block to Tailwind utility classes on shadcn tokens (text-foreground, text-muted-foreground, bg-muted, bg-card, border-border, etc.) so it responds to light/dark mode."
      - "Convert the decision/[...slug]/+page.svelte body <style> block the same way."
      - "Remove the remaining #1a1a2e / #2563eb old-palette hex from both pages (D-13)."
      - "(Semantic data-viz hex in Graph.svelte stage/edge colors and the spec slice-status chips are legitimate categorical colors and are NOT part of this gap — they are palette-neutral by design.)"
human_verification:
  - test: "In the running UI, toggle dark mode and open a Spec detail and a Decision detail page (post-fix)."
    expected: "All body content (headings, body text, labels, chips) is readable with proper contrast in BOTH light and dark mode."
    why_human: "Dark-mode contrast/readability is a visual judgment; grep confirms the current hardcoded hex breaks it but the post-fix result needs a human eye."
  - test: "D-10 constitution switch UAT: switch between a project WITH a constitution and one WITHOUT."
    expected: "Merged/Layer badges and sections re-derive with no stale prior-project content; the 'No constitution found for this project' empty state appears; layer hues correct in light + dark."
    why_human: "Real-time project-switch re-derivation and badge hues are visual/runtime behavior; the loader wiring is verified in code but the on-screen result needs UAT (flagged by 05-13-SUMMARY)."
  - test: "Project-switch skeleton-on-switch across dashboard/graph/spec/decision/constitution with >1 project available."
    expected: "Selecting a different project re-suspends each view to its Skeleton, then renders the new project's data with the correct X-Specgraph-Project scoping; no stale content."
    why_human: "invalidateAll() + streamed {#await} re-suspend is verified in code, but the live re-fetch/skeleton transition is runtime behavior best confirmed against a real server with multiple projects."
---

# Phase 5: UI Project Selector & Refinements — Verification Report

**Phase Goal:** The web UI lets users select which project they're viewing (with a sensible default) and surfaces project-specific views/refinements (constitution, etc.) instead of assuming a single implicit project.
**Verified:** 2026-07-12T15:18:14Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

The phase's **primary** goal — a project selector with sensible-default resolution and
project-scoped views that re-fetch on switch — is **fully achieved and well-engineered**.
The shadcn/dark-mode migration (secondary deliverable, SC3) is **complete except for the
body content of the two detail pages**, which still carry the pre-migration navy/blue
plain-CSS palette and are broken in dark mode.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A user can pick the active project with a sensible default (last-used → `default` → alpha-first), and switching re-fetches every project-scoped view (D-01..D-08) | ✓ VERIFIED | `project.svelte.ts` `loadProjects()` implements case-insensitive sort (D-05) + exact D-04 precedence + stale fallback (D-06) + zero-projects empty (D-07), proven by `project.test.ts` (6/6 pass). `+layout.svelte` `switchProject()` sets `project.current` then `invalidateAll()` (D-01/D-03); Select shown for >1 project, static label for 1 (D-08). `+layout.ts` is client universal load, `ssr=false` (D-02). `projectInterceptor` re-issues `X-Specgraph-Project` from `project.current` on every RPC (client.ts:15-18). |
| 2 | Project-scoped views reflect the selected project with correct empty/error states + active-project indicator; constitution badges re-derive across switches (D-09/D-10/D-11) | ✓ VERIFIED | All 5 scoped views have universal `+page.ts` `load()` with `await parent()` + `depends('app:project')` (dashboard, graph, spec, decision, constitution). Keys correctly excluded from load-refactor (D-09). Constitution badges derive from reloaded `data.provenance` with NO local `$state` → cannot show stale badges on switch (D-10); empty state + Skeleton present. Active-project breadcrumb owned by layout (D-11), suppressed on `/keys` and zero-projects. Detail pages route NotFound→empty card, other errors→inline Retry (never `+error.svelte`). |
| 3 | The full web UI is migrated to shadcn-svelte (Tailwind v4, Slate theme) with light/dark mode (D-12/D-13/D-14) | ✗ FAILED (partial) | Stack fully installed (mode-watcher 1.1.0, bits-ui 2.18.1, tailwind-merge, tailwind-variants, clsx, @lucide/svelte, tailwindcss 4.3.2); 16 components + `components/ui/` migrated; `ModeWatcher`+`ModeToggle`/`toggleMode` wired (D-14); dashboard/graph/keys/constitution have zero `<style>` blocks. **BUT** spec detail & decision detail retain plain-CSS `<style>` blocks with the OLD `#1a1a2e`/`#2563eb` palette (D-13 said replace) using light-only hex with no `.dark` handling → body content unreadable in dark mode (D-14). |

**Score:** 2/3 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/src/lib/project.svelte.ts` | D-04/05/06 precedence + sort + fallback | ✓ VERIFIED | loadProjects sorts case-insensitive, applies 4-tier precedence; 47 lines, substantive, wired via client.ts interceptor + layout. |
| `web/src/lib/project.test.ts` | Unit tests for D-04/05/06 + zero-projects | ✓ VERIFIED | 6 tests, all pass (order-independent with beforeEach reset). |
| `web/src/routes/+layout.svelte` / `+layout.ts` | Selector, switch→invalidateAll, breadcrumb, dark mode, empty state | ✓ VERIFIED | Select+label (D-08), switchProject invalidateAll (D-01/D-03), breadcrumb (D-11), ModeWatcher (D-14), zero-projects empty (D-07). |
| `web/src/lib/api/client.ts` | X-Specgraph-Project interceptor | ✓ VERIFIED | projectInterceptor reads reactive `project.current`, fallback `'default'`. |
| `web/src/routes/{+page,graph,constitution}/+page.ts` | load()-based project-scoped fetch | ✓ VERIFIED | All 3 stream promise + depends('app:project') + await parent(). |
| `web/src/routes/{spec,decision}/[...slug]/+page.ts` | load() keyed on params.slug | ✓ VERIFIED | Both load-ified, per-fetch resilience on secondary fetches. |
| `web/src/routes/spec/[...slug]/+page.svelte` | shadcn migration incl. dark mode (D-12/D-13) | ⚠️ PARTIAL | Shell/states on shadcn tokens; body `<style>` block retains old light-only palette (dark-mode broken). |
| `web/src/routes/decision/[...slug]/+page.svelte` | shadcn migration incl. dark mode (D-12/D-13) | ⚠️ PARTIAL | Same — body `<style>` block retains old light-only palette. |
| `internal/server/api_handler_test.go` | `_server` exclusion test | ✓ VERIFIED | `TestAPIHandler_ExcludesServerProject` passes. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `switchProject` | every `+page.ts` load() | `invalidateAll()` re-runs loads | ✓ WIRED | +layout.svelte:44 invalidateAll; each load registers `depends('app:project')`. |
| `project.current` | backend | `X-Specgraph-Project` header | ✓ WIRED | projectInterceptor sets header from reactive current on each RPC. |
| `loadProjects()` sort+precedence | `project.current` setter | localStorage `specgraph-project` | ✓ WIRED | setter writes localStorage (project.svelte.ts:12-15). |
| `data.provenance` | constitution badges | `layerBadge`/`layerOf` derive from reloaded provenance | ✓ WIRED | No local $state — re-derives on switch (D-10). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Web unit tests | `pnpm test` | 20/20 pass (project 6, constitution-load 4, keys 8, oidc 2) | ✓ PASS |
| Production build (static adapter) | `pnpm build` | built in 3.37s, adapter-static wrote `build/` | ✓ PASS |
| `_server` exclusion | `go test ./internal/server/ -run TestAPIHandler_ExcludesServerProject` | ok | ✓ PASS |
| Dark-mode readability of detail-page body | static analysis: hardcoded `#1a1a2e`/`#374151` in `<style>`, no `.dark` override | body text near-invisible in dark mode | ✗ FAIL |

### Requirements Coverage (D-01..D-14 scope contract)

| Decision | Description | Status | Evidence |
|----------|-------------|--------|----------|
| D-01 | Switch → invalidateAll() re-fetch | ✓ | switchProject + depends('app:project') on all loads |
| D-02 | Client universal load() (ssr=false) | ✓ | +layout.ts ssr=false; +page.ts await parent() |
| D-03 | Setter persists localStorage then invalidate | ✓ | project setter + switchProject |
| D-04 | Default precedence last-used→default→alpha | ✓ | loadProjects + tests |
| D-05 | Case-insensitive alpha sort | ✓ | loadProjects sort + test |
| D-06 | Stale saved project auto-fallback | ✓ | tier-1 "in available" guard + test |
| D-07 | Zero-projects empty state | ✓ | layout empty state; current='' seam |
| D-08 | 1 project → label, >1 → dropdown | ✓ | layout Select vs span |
| D-09 | Scoped views load-ified; Keys excluded | ✓ | 5 loads present; keys has no load |
| D-10 | Constitution polish: badges/empty re-derive | ✓ | provenance-derived badges, empty state, Skeleton |
| D-11 | Active-project indicator on pages | ✓ | layout breadcrumb |
| D-12 | Full component + page migration | ⚠️ PARTIAL | 16 components + 4/6 pages fully migrated; spec/decision detail body content not converted |
| D-13 | Adopt shadcn palette (replace #1a1a2e/#2563eb) | ✗ | old palette still present in spec/decision detail `<style>` |
| D-14 | Light + dark mode with toggle | ⚠️ PARTIAL | Toggle + tokens work everywhere EXCEPT spec/decision detail body (dark-mode broken) |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `spec/[...slug]/+page.svelte` | 417-635 | Hardcoded light-only hex palette in `<style>` | ⚠️ Warning | Dark-mode readability defect; D-13 palette not replaced |
| `decision/[...slug]/+page.svelte` | 102-165 | Hardcoded light-only hex palette in `<style>` | ⚠️ Warning | Dark-mode readability defect; D-13 palette not replaced |

No debt markers (TODO/FIXME/XXX) in phase-authored `web/src`. No `{@html}`/`innerHTML`/`eval` anywhere (XSS clean, confirmed by review). "not yet implemented" match is in generated `gen/*.ts` (not phase-authored).

### Advisory Findings (from 05-REVIEW.md — 0 critical / 3 warning / 6 info)

These are robustness/maintainability notes, not goal blockers, but worth folding into the gap-closure pass:

- **WR-01** — Dashboard `load()` uses one `Promise.all` + one `try/catch`, so any single non-critical RPC failure (`getReady`/`checkDrift`/`listDecisions`) blanks the whole page with the Retry card, inconsistent with the per-fetch resilience the detail loaders use. Advisory.
- **WR-02** — `loadProjects()` conflates a `/api/projects` fetch failure with "zero projects"; a transient API failure renders the permanent "No projects found" empty state with no retry. Advisory but a real edge.
- **WR-03** — `vite.config.ts` dev proxy forwards `/specgraph.v1` but not `/api/*`, breaking `vite dev` auth/bootstrap. Dev-tooling only (production is same-origin embedded build). Advisory.
- IN-01..IN-06 — `$app/stores` deprecation, keyless `{#each}`, GraphMini Space-key, `limit:0` magic value, `$effect` reset vs `{#key}`, unread `notFound` sentinel. Informational.

### Gaps Summary

The project-selector and reactive-switching architecture (SC1, SC2 — the heart of the
phase goal) is **fully delivered and verified**: deterministic default resolution,
`invalidateAll()`-driven re-fetch, load()-based data flow for all scoped views, correct
empty/error/not-found states, a constitution view whose badges re-derive on switch, an
active-project breadcrumb, and a working dark-mode toggle. Unit tests (20/20) and the
production build pass.

The one gap is in SC3 (the shadcn/dark-mode migration): **the body content of the Spec
detail and Decision detail pages was never converted off the pre-migration plain-CSS
navy/blue palette.** Those two `<style>` blocks use ~30 hardcoded light-only hex values
(`#1a1a2e` headings, `#374151` body, light backgrounds) with no `.dark` handling, so in
dark mode — an explicit phase deliverable (D-14) — the detail-page body renders
near-invisible. The 05-11-SUMMARY listed D-13 as completed for these pages; the code does
not support that claim. This is a self-contained, mechanical fix (swap the `<style>` block
for Tailwind token utilities, as the other four pages already do) and does not undermine
the primary goal.

---

_Verified: 2026-07-12T15:18:14Z_
_Verifier: the agent (gsd-verifier)_
