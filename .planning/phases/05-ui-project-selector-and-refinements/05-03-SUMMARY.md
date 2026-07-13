---
phase: 05-ui-project-selector-and-refinements
plan: 03
subsystem: ui
tags: [sveltekit, shadcn-svelte, mode-watcher, dark-mode, bits-ui, invalidateAll, layout-load]

requires:
  - phase: 05-01
    provides: shadcn-svelte primitives (Button, Select, Breadcrumb, Separator), app.css Slate tokens, app.css import in layout
  - phase: 05-02
    provides: project.svelte.ts loadProjects() with D-04 precedence + D-05 sort, /api/projects
provides:
  - "+layout.ts LayoutLoad that resolves auth + project default before page RPCs fire (D-02)"
  - "shadcn app shell: token-based nav, project Select/label/empty-state branches (D-07/D-08)"
  - "project-switch seam: onValueChange -> switchProject -> project.current + invalidateAll() (D-03; D-01 mechanism)"
  - "layout-owned active-project Breadcrumb {project} / {View} (D-11), single owner"
  - "dark mode: ModeWatcher + ModeToggle + FOUC-guard inline script (D-14)"
  - "legacy per-page breadcrumb markup stripped from constitution/spec/decision (no transient double-render)"
affects: [05-10, 05-11, 05-12, 05-13]

tech-stack:
  added: [mode-watcher@1.1.0, "@lucide/svelte (sun/moon/chevron icons)"]
  patterns:
    - "Universal LayoutLoad bootstrap (checkAuth + loadProjects) so pages await parent() for a resolved project"
    - "Explicit onValueChange -> invalidateAll() switch (never bind:value alone)"
    - "Static SPA FOUC guard: blocking inline script reads mode-watcher-mode key pre-paint"
    - "Layout is single owner of the active-project breadcrumb; {View} from a pathname label map"

key-files:
  created:
    - web/src/lib/components/ModeToggle.svelte
  modified:
    - web/src/routes/+layout.ts
    - web/src/routes/+layout.svelte
    - web/src/app.html
    - web/src/routes/constitution/+page.svelte
    - web/src/routes/spec/[...slug]/+page.svelte
    - web/src/routes/decision/[...slug]/+page.svelte

key-decisions:
  - "Gate the shell on data.authenticated from load(); handleLoginSuccess calls invalidateAll() so load re-runs and the gate flips reactively (idiomatic SvelteKit, avoids the removed ready flag)."
  - "Confirmed mode-watcher@1.1.0 storage key is 'mode-watcher-mode' by inspecting dist/storage-keys.svelte.js (box(\"mode-watcher-mode\")) before hard-coding it in the FOUC guard."
  - "D-01 is 'mechanism introduced' here, NOT accepted — pages still fetch via onMount/$effect until Wave 3 load-ifies them (05-10..13)."

patterns-established:
  - "LayoutLoad auth/project bootstrap: page loads await parent() before issuing project-scoped RPCs (Pitfall 6 fix)"
  - "shadcn Select switch handler: onValueChange -> setter (persist) -> invalidateAll() re-runs every load()"
  - "Single-owner breadcrumb with static {View} map; detail slugs stay in page <h1>"

requirements-completed: [D-01, D-02, D-03, D-07, D-08, D-11, D-13, D-14]

coverage:
  - id: D1
    description: "ModeToggle theme button (shadcn icon Button, aria-label=Toggle theme, toggleMode) + pre-paint FOUC guard reading the confirmed mode-watcher-mode key (D-14)"
    requirement: D-14
    verification:
      - kind: other
        ref: "cd web && grep -rq mode-watcher-mode node_modules/mode-watcher/ && node static-assert (app.html has classList guard before %sveltekit.head%, ModeToggle has toggleMode + aria-label) && pnpm build"
        status: pass
    human_judgment: true
    rationale: "Structure is automated-verified, but 'no theme flash on reload' is a visual property only a human can confirm (plan verification appetite: D-14 accepted via manual UAT)."
  - id: D2
    description: "+layout.ts LayoutLoad awaits checkAuth() then loadProjects() when authenticated, keeps ssr=false/prerender=false, no server load — resolves project default before page RPCs (D-02)"
    requirement: D-02
    verification:
      - kind: other
        ref: "cd web && node static-assert(+layout.ts has ssr=false, prerender=false, export const load, loadProjects) && test ! -f +layout.server.ts && pnpm build"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-08 selector branches: shadcn Select (many, aria-label=Select project, alpha-sorted options), muted text label (one), no selector (zero); D-07 zero-projects main-area empty state ('No projects found')"
    requirement: D-08
    verification:
      - kind: other
        ref: "cd web && pnpm build (branch markup compiles); visual states per 05-UI-SPEC"
        status: pass
    human_judgment: true
    rationale: "Multi/single/zero selector rendering + zero-state copy are visual states accepted via manual UAT (no component-test harness in appetite this phase)."
  - id: D4
    description: "Project-switch seam: Select onValueChange -> switchProject(slug) sets project.current (localStorage persist) then await invalidateAll() so +layout.ts and every +page.ts re-run with the new X-Specgraph-Project header (D-03; D-01 mechanism only)"
    requirement: D-03
    verification:
      - kind: other
        ref: "cd web && grep invalidateAll in +layout.svelte && pnpm build && pnpm test (16 passed)"
        status: pass
    human_judgment: true
    rationale: "Mechanism is structurally verified; end-to-end re-fetch on switch (D-01) is NOT accepted here — pages load-ify in Wave 3. Full switch behavior needs manual UAT after Wave 3."
  - id: D5
    description: "Layout-owned active-project Breadcrumb {project} / {View} with {View} from a $page.url.pathname label map (Dashboard/Graph/Constitution/Spec/Decision), suppressed on /keys, updates on switch; legacy per-page breadcrumb markup stripped from constitution/spec/decision (D-11)"
    requirement: D-11
    verification:
      - kind: other
        ref: "cd web && static-assert (no class=\"breadcrumb\" in constitution/spec/decision pages; +layout.svelte references pathname) && pnpm build"
        status: pass
    human_judgment: true
    rationale: "Breadcrumb rendering + reactive update-on-switch + no-transient-double-render are visual behaviors accepted via manual UAT."
  - id: D6
    description: "Retire navy palette (#1a1a2e, .project-picker) — nav rebuilt on Slate tokens; active link uses --primary (D-13)"
    requirement: D-13
    verification:
      - kind: other
        ref: "cd web && static-assert (no #1a1a2e in +layout.svelte) && pnpm build"
        status: pass
    human_judgment: false

duration: 7 min
completed: 2026-07-12
status: complete
---

# Phase 05 Plan 03: App Shell Rebuild on shadcn Summary

**shadcn app shell with LayoutLoad auth/project bootstrap, invalidate-on-switch project Select, single-owner active-project breadcrumb, and mode-watcher dark mode with a pre-paint FOUC guard.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-12T13:49:20Z
- **Completed:** 2026-07-12T13:55:59Z
- **Tasks:** 3
- **Files modified:** 7 (1 created, 6 modified)

## Accomplishments

- Moved auth + project bootstrap into `+layout.ts` `load()` so page loads can `await parent()` and see the resolved project before firing RPCs (D-02, fixes Pitfall 6).
- Rebuilt `+layout.svelte` on Slate tokens: retired the navy `#1a1a2e` `<style>` block and `.project-picker`; nav active link uses `--primary`.
- Implemented the D-08 selector: shadcn `Select` (many, `aria-label="Select project"`, alpha-sorted), muted text label (one), no selector (zero) + D-07 zero-projects main-area empty state.
- Wired the switch seam via explicit `onValueChange` → `switchProject` → `project.current` setter (localStorage) → `await invalidateAll()` (D-03; D-01 mechanism).
- Added the layout-owned active-project `Breadcrumb` `{project} / {View}` with `{View}` from a `$page.url.pathname` label map, suppressed on `/keys` (D-11); stripped the legacy per-page breadcrumb markup from constitution/spec/decision atomically (no transient double-render).
- Activated dark mode: `ModeWatcher` + `ModeToggle` (confirmed `mode-watcher-mode` key) + blocking FOUC-guard inline script in `app.html` (D-14).

## Task Commits

1. **Task 1: ModeToggle component + FOUC-guard script (D-14)** — `f1f3963d` (feat)
2. **Task 2: Move auth + project bootstrap into +layout.ts load() (D-02)** — `e8289a67` (feat)
3. **Task 3: Rebuild +layout.svelte shell — nav, selector, switch, breadcrumb, ModeWatcher** — `37640755` (feat)

**Plan metadata:** committed with SUMMARY + STATE + ROADMAP + REQUIREMENTS.

## Files Created/Modified

- `web/src/lib/components/ModeToggle.svelte` — shadcn icon Button calling `toggleMode`, sun/moon lucide icons, `aria-label="Toggle theme"` (created).
- `web/src/routes/+layout.ts` — `LayoutLoad` bootstrap: `checkAuth()` then `loadProjects()`; keeps `ssr=false`/`prerender=false`; returns `{ authenticated }`.
- `web/src/routes/+layout.svelte` — shadcn nav on tokens, `ModeWatcher`/`ModeToggle`, D-08 Select/label/empty selector, `switchProject`+`invalidateAll()`, layout-owned `{project} / {View}` breadcrumb, D-07 zero-state; retired navy `<style>`.
- `web/src/app.html` — blocking inline FOUC-guard script (reads `mode-watcher-mode`, toggles `.dark` pre-paint) before `%sveltekit.head%`.
- `web/src/routes/constitution/+page.svelte`, `web/src/routes/spec/[...slug]/+page.svelte`, `web/src/routes/decision/[...slug]/+page.svelte` — legacy `<nav class="breadcrumb">` markup removed (markup-only; fetch logic and `<style>` untouched; orphaned `.breadcrumb` CSS harmless, cleaned in Wave 3).

## Decisions Made

- **Auth gate on `data.authenticated` + `invalidateAll()` on login success:** removed the old `ready` flag; the shell gates on the load-provided `data.authenticated`, and `handleLoginSuccess` calls `invalidateAll()` so `+layout.ts` re-runs (fresh whoami via the just-set session cookie) and the gate flips reactively. Idiomatic SvelteKit and fully consumes the load data.
- **Confirmed FOUC key before shipping:** inspected `node_modules/mode-watcher/dist/storage-keys.svelte.js` → `box("mode-watcher-mode")` and `dist/mode.js` `setInitialMode` default; hard-coded `mode-watcher-mode` (RESEARCH A1 was `[ASSUMED]`; now verified).
- **D-01 is mechanism-only here:** the layout invalidates but project-scoped pages still fetch via `onMount`/`$effect` until Wave 3 (05-10..13) adds `+page.ts` loads. End-to-end switch re-fetch is NOT claimed at this sign-off.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 1 verify command needed a trailing slash to traverse the pnpm symlink**
- **Found during:** Task 1 (ModeToggle + FOUC guard verification)
- **Issue:** The plan's automated verify runs `grep -rq "mode-watcher-mode" node_modules/mode-watcher`. Under pnpm, `node_modules/mode-watcher` is a symlink into `.pnpm/...`; macOS/BSD `grep -r` (and `-R`) does not traverse the symlinked directory given without a trailing slash, so it returned exit 1 even though the key exists.
- **Fix:** Ran the equivalent confirmation with a trailing slash — `grep -rq "mode-watcher-mode" node_modules/mode-watcher/` (exit 0) — and independently confirmed the key by reading `dist/storage-keys.svelte.js` (`box("mode-watcher-mode")`). The acceptance criterion (key CONFIRMED against the installed package) is satisfied; only the verify command form changed, not the implementation.
- **Files modified:** None (verification-command correction only).
- **Verification:** `grep -rq "mode-watcher-mode" node_modules/mode-watcher/` → exit 0; `pnpm build` succeeds; `app.html` uses the confirmed key.
- **Committed in:** `f1f3963d` (Task 1 commit — implementation unchanged).

---

**Total deviations:** 1 auto-fixed (1 blocking — verification-command portability, no implementation change).
**Impact on plan:** Negligible. The FOUC key is confirmed and correct; only the shape of the grep in the verify command was adjusted for pnpm symlinks on macOS. No scope creep.

## Issues Encountered

- **`task check` fails at `fmt:check` and `lint:markdown` on pre-existing `.planning/` artifact drift** (139 dprint-unformatted `.planning/intel/classifications/*.json`; 966 rumdl markdown issues across 87 `.planning/` files). These are pre-existing, tracked, and entirely unrelated to this frontend plan — all 05-03 web files pass `task fmt:check`. Out of scope per the scope boundary; logged in `deferred-items.md` (05-02 already recorded the dprint drift). The meaningful gates for this plan all pass: `pnpm build`, `pnpm test` (16 passed), `task web:build`, `task build` (Go binary embeds the web bundle), and `task lint:go` (0 issues).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The shell seam is ready: `+layout.ts` resolves the project default before page RPCs, and `switchProject` invalidates all loads. Wave 3 (05-10..13) can now add `+page.ts` loads that `await parent()` and re-fetch on switch to complete D-01/D-10 acceptance.
- The layout is the single owner of the active-project breadcrumb; Wave 3 pages must NOT reintroduce a per-page project breadcrumb (constitution/spec/decision breadcrumb markup already removed; Keys keeps its own user-scoped breadcrumb in 05-12).
- Orphaned `.breadcrumb` CSS rules remain in the three page `<style>` blocks (harmless); they are cleaned when 05-11/05-13 fully restyle those pages.

## Self-Check: PASSED

- All 7 created/modified files verified present on disk.
- All 3 task commits verified in git log: `f1f3963d`, `e8289a67`, `37640755`.
- Plan verification re-run: `pnpm build` ✓, `pnpm test` (16 passed) ✓, `task web:build` ✓, `task build` ✓, `task lint:go` (0 issues) ✓. `task check` blocked only by pre-existing, out-of-scope `.planning/` doc-format drift (deferred).

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
