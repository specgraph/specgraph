---
phase: 05-ui-project-selector-and-refinements
plan: 09
subsystem: ui
tags: [svelte, shadcn, dagre, theme-tokens, dark-mode, graph, svg]

# Dependency graph
requires:
  - phase: 05-01
    provides: shadcn Card primitive + theme CSS variables in app.css (light/dark tokens)
provides:
  - Graph.svelte reframed on shadcn Card with theme-token node/edge/label colors (dark-mode legible)
  - GraphMini.svelte reframed on shadcn Card with theme-token title color
affects: [graph page, dashboard graph preview, any future graph visualization work]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SVG fill/stroke driven by theme CSS variables via inline `style=\"fill: var(--token)\"` (SVG presentation attributes do not resolve var(); style does)"
    - "Categorical encoding (stage/edge/slice-status colors) kept as mid-tone hex — the ONE color carve-out; structural fills (card/popover/muted/foreground/border) come from tokens"

key-files:
  created: []
  modified:
    - web/src/lib/components/Graph.svelte
    - web/src/lib/components/GraphMini.svelte

key-decisions:
  - "SVG node/edge/label fills use inline `style=\"fill: var(--token)\"` because SVG presentation attributes (fill=/stroke=) do not resolve CSS var()."
  - "Slice node fills switched from light-only hex tints to `var(--muted)` with categorical strokes, so status stays distinguishable AND dark-mode legible."
  - "dagre layout/derivation code left completely untouched — only framing (Card) and color values changed."

patterns-established:
  - "Theme-token SVG coloring: structural surfaces (rect fills, tooltip) → var(--card)/var(--popover)/var(--muted)/var(--border); text → var(--foreground)/var(--card-foreground)/var(--muted-foreground); categorical data (stage/edge/slice status) stays as hex."

requirements-completed: [D-12, D-13]

coverage:
  - id: D1
    description: "Graph.svelte wrapped in shadcn Card; dagre layout logic unchanged; node/edge/label/tooltip colors derive from theme CSS variables; no legacy navy/blue hex."
    requirement: "D-12"
    verification:
      - kind: unit
        ref: "node -e regex guard (no #1a1a2e/#2563eb; requires Card + --foreground/--primary/--muted/--border)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build / task web:build"
        status: pass
    human_judgment: true
    rationale: "Dark-mode contrast legibility (node fills vs Card bg, label readability, edge/arrow visibility, legend distinguishability in BOTH themes) is a visual judgment no automated check asserts — see plan verification graph-contrast checklist."
  - id: D2
    description: "GraphMini.svelte reframed on shadcn Card with theme-token title color; compact graph render unchanged; no legacy navy hex."
    requirement: "D-13"
    verification:
      - kind: unit
        ref: "node -e regex guard (no #1a1a2e/#2563eb; requires --foreground/--primary/--muted/--border)"
        status: pass
      - kind: integration
        ref: "pnpm -C web build"
        status: pass
    human_judgment: true
    rationale: "Same dark-mode contrast/legibility visual judgment as D1, applied to the compact dashboard preview."

# Metrics
duration: 8min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 09: Graph & GraphMini shadcn Reframe Summary

**Graph and GraphMini migrated to shadcn Card framing with theme-token SVG colors (dark-mode legible), dagre layout logic left untouched.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-07-12T14:22:00Z
- **Completed:** 2026-07-12T14:30:42Z
- **Tasks:** 2
- **Files modified:** 2 (+ 1 deferred-items log)

## Accomplishments
- `Graph.svelte`: dagre SVG canvas wrapped in `Card.Root`; node rects, decision diamonds, slice pills, hover tooltip, arrowhead marker, and all label text now resolve fills from theme CSS variables (`--card`, `--popover`, `--muted`, `--foreground`, `--muted-foreground`, `--border`) via inline SVG `style` (presentation attributes cannot resolve `var()`).
- Retired the legacy navy/blue palette: `#1a1a2e` label text → `var(--foreground)`, white rect fills → `var(--card)`, light-only slice tint hexes → `var(--muted)` with categorical strokes preserved.
- `GraphMini.svelte`: bespoke `.mini-wrapper` div replaced with a clickable `Card.Root` (cursor/hover-shadow utilities, role=link, keyboard handler preserved); title color from `var(--foreground)`.
- Categorical color encoding (stage colors, edge-type styles, slice-status strokes) kept as mid-tone hex per the phase's "ONE color carve-out" rule — legible in both themes.

## Task Commits

1. **Task 1: Reframe Graph on a Card with theme-token colors** - `5b03dfaa` (feat)
2. **Task 2: Apply the same reframe to GraphMini** - `8559d18f` (feat)

**Deferred log:** `24ac035c` (docs: log pre-existing dprint drift)

## Files Created/Modified
- `web/src/lib/components/Graph.svelte` - dagre intact; Card frame; node/edge/label/tooltip fills from theme tokens; arrowhead + slice recolored.
- `web/src/lib/components/GraphMini.svelte` - Card wrapper replaces bespoke div; title uses `--foreground`.

## Decisions Made
- SVG coloring uses inline `style="fill: var(--token)"` rather than `fill=` attributes, because SVG presentation attributes do not resolve CSS `var()` — only the CSS `style`/stylesheet path does.
- Slice-node fills moved off light-only tint hexes to `var(--muted)` + categorical stroke/text, keeping status distinguishable while adapting to dark mode.
- No change to any dagre construction, layout, pan/zoom, filter, or routing logic.

## Deviations from Plan

None - plan executed exactly as written. Structural fills were driven from theme tokens and categorical data colors retained, matching the plan's review-suggestion #8 guidance (define legible values for previously light-only hardcoded fills).

## Issues Encountered
- `task check` fails at `fmt:check` due to 139 pre-existing unformatted `.planning/intel/classifications/*.json` files — a committed repo-state drift unrelated to this plan (web-only Svelte changes; dprint has no Svelte plugin). Logged to `deferred-items.md` under `## 05-09`. Plan-scoped verification (`pnpm -C web build`, `task web:build`, per-task regex guards) all pass.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Graph/GraphMini visually consistent with the migrated shadcn shell; both build cleanly.
- Recommended manual UAT (from plan `<verification>`): graph contrast checklist in BOTH light and dark mode — node fills vs Card bg, label readability, edge/arrow visibility, legend/highlight distinguishability. Deliverables D1/D2 are marked `human_judgment: true` for this reason.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*

## Self-Check: PASSED
- `web/src/lib/components/Graph.svelte` — FOUND (modified, builds)
- `web/src/lib/components/GraphMini.svelte` — FOUND (modified, builds)
- Commit `5b03dfaa` — FOUND
- Commit `8559d18f` — FOUND
- Commit `24ac035c` — FOUND
