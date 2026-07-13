---
phase: 05-ui-project-selector-and-refinements
plan: 01
subsystem: ui
tags: [shadcn-svelte, tailwindcss-v4, sveltekit, svelte5, bits-ui, mode-watcher, slate, design-system]

# Dependency graph
requires:
  - phase: 04-verification-integration-reliability
    provides: existing web/ SvelteKit SPA (adapter-static, embedded via web/embed.go)
provides:
  - Tailwind v4 wired into the web/ Vite build (@tailwindcss/vite before sveltekit)
  - shadcn-svelte components.json + generated cn() utility
  - 16 vendored shadcn primitives under $lib/components/ui/
  - Slate OKLCH :root/.dark token block + @custom-variant dark + @theme inline map in app.css
  - shared categorical badge-variant maps (layer/stage/status/severity)
  - app.css imported in root +layout.svelte (Tailwind live app-wide from Wave 1)
affects: [05-03, 05-04, 05-05, 05-06, 05-07, 05-08, 05-09, 05-10, 05-11, 05-12, 05-13]

# Tech tracking
tech-stack:
  added:
    - tailwindcss@4.3.2
    - "@tailwindcss/vite@4.3.2"
    - bits-ui@2.18.1
    - mode-watcher@1.1.0
    - "@lucide/svelte@1.24.0"
    - tailwind-variants@3.2.2
    - tailwind-merge@3.6.0
    - clsx@2.1.1
    - tw-animate-css@1.4.0
    - svelte-sonner@1.1.1
    - "@internationalized/date@3.12.2"
    - "shadcn-svelte@1.4.1 (CLI, via dlx)"
  patterns:
    - "Tailwind v4 CSS-first (no tailwind.config.js / postcss.config.js)"
    - "Vendored shadcn primitives under $lib/components/ui/ — treat as generated, wrap/compose not hand-edit"
    - "Categorical badge palette as fixed light/dark Tailwind class pairs (NOT theme --primary)"

key-files:
  created:
    - web/components.json
    - web/src/app.css
    - web/src/lib/utils.ts
    - web/src/lib/components/badge-variants.ts
    - web/src/lib/components/ui/ (16 primitives)
  modified:
    - web/package.json
    - web/pnpm-lock.yaml
    - web/vite.config.ts
    - web/src/routes/+layout.svelte

key-decisions:
  - "Used the sanctioned manual-fallback path (RESEARCH Pitfall 1): shadcn-svelte init blocked on an interactive preset prompt in 1.4.1, so components.json / app.css / utils.ts were authored by hand and primitives added via `shadcn-svelte add -y -o --no-deps`."
  - "Scaffold base-color enum has no `slate` (Pitfall 2 confirmed via --help); app.css carries the verified Slate OKLCH block and components.json baseColor:slate is cosmetic metadata."
  - "Used @lucide/svelte, not the superseded lucide-svelte (RESEARCH Package Audit)."
  - "No pnpm-workspace.yaml minimumReleaseAgeExclude edits were needed — the supply-chain policy passed for all pinned versions under --frozen-lockfile (no ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION)."

patterns-established:
  - "Tailwind v4 CSS-first entry (app.css) is the single theming source; imported once in root +layout.svelte."
  - "badge-variants.ts is the single source for categorical data-encoding hues consumed by later waves."

requirements-completed: [D-12, D-13, D-14]

coverage:
  - id: D1
    description: "Tailwind v4 installed and wired into the Vite build; web app builds as a static bundle with Tailwind active."
    requirement: "D-12"
    verification:
      - kind: integration
        ref: "cd web && pnpm install --frozen-lockfile"
        status: pass
      - kind: integration
        ref: "cd web && pnpm build (static build/ non-empty)"
        status: pass
      - kind: integration
        ref: "task web:build && task build (bundle embeds into Go binary)"
        status: pass
    human_judgment: false
  - id: D2
    description: "16 shadcn-svelte primitives vendored under $lib/components/ui/ and import cleanly."
    requirement: "D-12"
    verification:
      - kind: integration
        ref: "ls web/src/lib/components/ui | wc -l == 16; pnpm build resolves imports"
        status: pass
    human_judgment: false
  - id: D3
    description: "Slate OKLCH token block present for :root and .dark with @custom-variant dark; @theme inline map generated."
    requirement: "D-13"
    verification:
      - kind: integration
        ref: "node check: app.css contains oklch(0.129 0.042 264.695) and @custom-variant dark"
        status: pass
    human_judgment: false
  - id: D4
    description: "Shared categorical badge-variant maps (layer/stage/status/severity) exist as fixed light/dark class pairs, none mapped to --primary."
    requirement: "D-14"
    verification:
      - kind: integration
        ref: "grep: only comment mentions --primary; no badge class value uses primary; pnpm build type-checks the file"
        status: pass
    human_judgment: false
  - id: D5
    description: "Tailwind is live app-wide from Wave 1 via import '../app.css' in root +layout.svelte (review HIGH fix)."
    requirement: "D-13"
    verification:
      - kind: integration
        ref: "node check: +layout.svelte contains import '../app.css'; pnpm build"
        status: pass
    human_judgment: false

# Metrics
duration: ~10 min
completed: 2026-07-12
status: complete
---

# Phase 5 Plan 01: shadcn-svelte + Tailwind v4 Foundation Summary

**Tailwind v4 (CSS-first) wired into the web/ Vite build with 16 vendored shadcn-svelte primitives, a verified Slate OKLCH token block (light+dark), a shared categorical badge-variant map, and app.css imported app-wide so every later wave has live styling and primitives to consume.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-07-12T13:29Z
- **Completed:** 2026-07-12T13:39Z
- **Tasks:** 3
- **Files modified:** 115 (4 hand-authored + 16 primitive dirs + deps/lockfile)

## Accomplishments
- Installed the RESEARCH-pinned Tailwind v4 + shadcn ecosystem (tailwindcss, @tailwindcss/vite, bits-ui, mode-watcher, @lucide/svelte, tailwind-variants, tailwind-merge, clsx, tw-animate-css) under the pnpm `--frozen-lockfile` supply-chain policy; wired `tailwindcss()` before `sveltekit()` while preserving the `/specgraph.v1` proxy.
- Authored `components.json`, `cn()` (`utils.ts`), and the full Slate `app.css` (Tailwind v4 entry, `tw-animate-css`, `@custom-variant dark`, verified Slate `:root`/`.dark` OKLCH tokens, `@theme inline` map, base layer); vendored 16 shadcn primitives via `shadcn-svelte add`.
- Created `badge-variants.ts` with fixed light/dark categorical class pairs for constitution layers (User/Org/Project/Domain), spec stages, decision statuses, and finding severities — none mapped to `--primary` (D-10 palette carve-out).
- Imported `../app.css` in the root `+layout.svelte` so Tailwind v4 is active app-wide from Wave 1 (review HIGH fix) without rebuilding the nav (deferred to 05-03).

## Task Commits

1. **Task 1: Install deps + wire Vite plugin** - `27c98e8a` (feat)
2. **Task 2: shadcn init + Slate tokens + primitives + badge-variants** - `bc88f4e3` (feat)
3. **Task 3: import app.css in root +layout.svelte** - `809f8a34` (feat)

## Files Created/Modified
- `web/components.json` - shadcn-svelte metadata (style default, baseColor slate, UI-SPEC aliases)
- `web/src/app.css` - Tailwind v4 entry + Slate OKLCH tokens + @custom-variant dark + @theme inline map + base layer
- `web/src/lib/utils.ts` - generated `cn()` (clsx + tailwind-merge)
- `web/src/lib/components/badge-variants.ts` - categorical layer/stage/status/severity class-pair maps
- `web/src/lib/components/ui/*` - 16 vendored shadcn primitives (button…breadcrumb)
- `web/vite.config.ts` - `@tailwindcss/vite` plugin before `sveltekit()`; proxy preserved
- `web/src/routes/+layout.svelte` - one-line `import '../app.css';`
- `web/package.json`, `web/pnpm-lock.yaml` - dependency set + transitive deps (svelte-sonner, @internationalized/date)

## Decisions Made
- **Manual-fallback install path (RESEARCH Pitfall 1):** `shadcn-svelte init` in 1.4.1 blocks on an interactive "preset" prompt even with `--base-color/--css/--*-alias` flags. Rather than fight the TUI, `components.json`/`app.css`/`utils.ts` were written by hand and primitives added via the flag-drivable `shadcn-svelte add -y -o --no-deps`. Result is byte-equivalent to init output.
- **Slate via token block, not flag (Pitfall 2 confirmed):** `--help` shows the base-color enum is `neutral|stone|zinc|mauve|olive|mist|taupe` (no `slate`). The verified Slate OKLCH block lands in `app.css`; `components.json.baseColor: slate` is cosmetic metadata.
- **`@lucide/svelte`, not `lucide-svelte`** per the RESEARCH Package Legitimacy Audit (legacy name is SUS/superseded).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] shadcn-svelte init required a pre-existing CSS file, then blocked on an interactive preset prompt**
- **Found during:** Task 2
- **Issue:** `init --css src/app.css` errored ("global CSS file does not exist"); after creating a stub `app.css`, init still blocked on an interactive "which preset?" TUI prompt (Pitfall 1) with no non-interactive flag to bypass it.
- **Fix:** Took the sanctioned manual-fallback path — authored `components.json`, `app.css` (full Slate block), and `utils.ts` by hand, then vendored the 16 primitives with `shadcn-svelte add -y -o --no-deps` (which reads `components.json` and does not prompt).
- **Files modified:** web/components.json, web/src/app.css, web/src/lib/utils.ts
- **Verification:** `pnpm build` succeeds; acceptance checks (marker token, @custom-variant, 16 primitives) pass.
- **Committed in:** bc88f4e3 (Task 2 commit)

**2. [Rule 3 - Blocking] `--no-deps` on `shadcn-svelte add` skipped two required transitive deps**
- **Found during:** Task 2
- **Issue:** `add --no-deps` reported it installed components "without" `svelte-sonner` and `@internationalized/date` (needed by sonner and select/date primitives) — build would fail to resolve them.
- **Fix:** `pnpm add svelte-sonner@^1.1.0 @internationalized/date@^3.12.0`.
- **Files modified:** web/package.json, web/pnpm-lock.yaml
- **Verification:** `pnpm build` resolves all primitive imports and completes.
- **Committed in:** bc88f4e3 (Task 2 commit)

**Note (no-op, not a deviation):** The plan lists `web/pnpm-workspace.yaml` as a modified file for targeted `minimumReleaseAgeExclude` entries. None were needed — every pinned version cleared the supply-chain policy under `--frozen-lockfile` (no `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION`), so the file was intentionally left unchanged. Acceptance criterion (frozen-lockfile install succeeds) is met.

---

**Total deviations:** 2 auto-fixed (both Rule 3 - blocking). **Impact:** Both were tooling-path corrections required to complete Task 2; the delivered artifacts match the plan's intent exactly. No scope creep.

## Issues Encountered

- **`task check` fails at `fmt:check` and `lint:markdown` on pre-existing `.planning/` files (out of scope).** `dprint` flags 139 `.planning/intel` + `.planning/research` JSON files and `rumdl` flags ~950 markdown-lint issues across 85 `.planning/` docs — all pre-existing, none authored by this plan (verified: zero overlap with this plan's changed files, which are all under `web/`). Logged to `.planning/phases/05-ui-project-selector-and-refinements/deferred-items.md` per the SCOPE BOUNDARY rule. The substantive gates all pass independently: `license:check` ✓, `lint:go` (0 issues) ✓, `task build` (Go binary embeds the static bundle) ✓, `task test` (Go unit) ✓, `pnpm -C web test` (10/10) ✓.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Foundation complete: Tailwind v4 active app-wide, 16 primitives vendored, Slate tokens live, `badge-variants` map available.
- Ready for Wave 2 (05-03 layout rebuild; 05-04..05-09 component migrations) and Wave 3 page load-ification — all depend only on this plan's primitives/tokens.
- 05-03 will retire the legacy scoped `<style>` in `+layout.svelte` and KEEP the `import '../app.css'` added here.

## Self-Check: PASSED

All created files exist on disk (components.json, app.css, utils.ts, badge-variants.ts, ui/ primitives, vite.config.ts, +layout.svelte) and all three task commits (`27c98e8a`, `bc88f4e3`, `809f8a34`) are present in git history.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*
