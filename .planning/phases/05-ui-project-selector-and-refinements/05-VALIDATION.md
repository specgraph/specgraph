---
phase: 5
slug: ui-project-selector-and-refinements
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-10
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `05-RESEARCH.md` § Validation Architecture. `nyquist_validation`
> is absent from `.planning/config.json` → treated as enabled.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 3.0 (web, already present); Go `testing` (backend `/api/projects`) |
| **Config file** | none dedicated for web — `vitest run` via `web/package.json` `test` script (uses Vite config) |
| **Quick run command** | `pnpm -C web test` |
| **Full suite command** | `task check` (Go fmt/lint/build/unit) + `pnpm -C web test` |
| **Estimated runtime** | ~30–60s (web unit); `task check` ~1–3 min (no Docker/Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `pnpm -C web test` (add `pnpm -C web build` for migrated pages as a smoke gate).
- **After every plan wave:** Run `task web:build && task build && task check`.
- **Before `/gsd-verify-work`:** Full suite must be green + visual dark-mode and project-switch UAT.
- **Max feedback latency:** 30s (inner loop = `pnpm -C web test` / `svelte-check`; reserve the full Vite production build for the per-wave gate).

---

## Per-Task Verification Map

| Requirement (D-#) | Behavior | Wave | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------------------|----------|------|------------|-----------------|-----------|-------------------|-------------|--------|
| D-04 / D-05 / D-06 | default-selection precedence + case-insensitive sort + stale-project fallback in `loadProjects()` | 1 | T-INFO (tenant scope) | slug comes from server allow-list `/api/projects`; no free-text injection | unit | `pnpm -C web test` (`project.test.ts`) | ❌ W0 (new test) | ⬜ pending |
| D-05 | case-insensitive sort deterministic regardless of API order | 1 | — | N/A | unit | `pnpm -C web test` (mock `/api/projects`) | ❌ W0 | ⬜ pending |
| D-01 / D-02 / D-03 | `+page.ts load()` returns data; `invalidateAll()` on switch re-fetches with new `X-Specgraph-Project` | 3 | T-INFO (stale-render leak) | no previous-project data remains after switch (skeleton clears then shows new data) | unit (load-seam) + manual UAT + structural | `pnpm -C web test` (`constitution-load.test.ts` — `depends('app:project')` + mapping + `loadError`) + `pnpm build` (load seam) + manual switch UAT (component harness out of appetite, review R2 #3) | ✅ W3 (05-13 Task 3) + manual + build gate | ⬜ pending |
| D-07 / D-08 | zero / one / many project selector states | 2 | — | N/A | manual UAT | manual (component lib out of appetite) | manual-only | ⬜ pending |
| D-10 | constitution empty state + Merged/Layer badges correct across switch | 3 | T-INFO (stale badges) | badges re-derive from new project's data | unit (load-seam) + manual UAT | `pnpm -C web test` (`constitution-load.test.ts`: mapping + `loadError` + `depends`) for the load seam; visual badge re-derivation via manual UAT | ✅ W3 (05-13 Task 3) / manual | ⬜ pending |
| D-11 | active-project indicator reflects current project on pages | 2/3 | — | N/A | component / manual | manual UAT (visual) | manual-only | ⬜ pending |
| D-05 (server alt) | `/api/projects` excludes `_server` | 1 | T-AC (V4) | server scopes tenant via `storage.Scoper` | unit (Go) | `go test ./internal/server/ -run TestAPIHandler_ExcludesServerProject` | ✅ W0 (added in 05-02 Task 3) | ⬜ pending |
| D-12 / D-13 / D-14 | full shadcn migration builds; theme toggle flips `.dark`; Slate tokens applied | 1/2 | T-SUPPLY (deps) | deps from official shadcn-svelte; pnpm `minimumReleaseAge` guard kept | smoke + manual | `task web:build && task build`; visual UAT for `.dark` | ✅ pipeline / manual | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/src/lib/project.test.ts` — unit stubs for D-04/D-05/D-06 (mock `fetch('/api/projects')`, assert precedence + case-insensitive sort + stale fallback). **Created in plan 05-02.**
- [ ] **Component-test lib decision — RESOLVED (review MEDIUM #6 / Round-2 #3): full component harness OUT of appetite this phase.** A Svelte component-test harness (`@testing-library/svelte` + jsdom or `@vitest/browser`) is NOT adopted; adding one is a follow-up, not a Phase-5 blocker. **However, a cheap Vitest-ONLY load-seam unit test WAS added (05-13 Task 3, `constitution-load.test.ts`)** — it needs no new dep (same category as `oidc.test.ts`/`keys.test.ts`) and anchors the constitution `load()` contract: `depends('app:project')` registration (the D-01/D-02 switch mechanism), response→`{constitution,provenance}` mapping, and `loadError` on RPC throw. Consequently the manual-UAT surface for D-01/D-10 narrows to the VISUAL re-render/badge check; the load+dependency mechanism is now automated. D-07/D-08/D-11 remain manual-UAT (visual selector/breadcrumb states). Automated coverage this phase = `project.test.ts` (D-04/05/06) + `constitution-load.test.ts` (D-01/D-02/D-10 load seam) + the Go `_server` test + `pnpm build`/`task check` smoke gates. Rationale for NOT adopting the full harness: introducing a jsdom/browser harness exceeds this phase's appetite; a plain load-function unit test does not.
- [ ] `internal/server/api_handler_test.go` — `TestAPIHandler_ExcludesServerProject`: fake backend returns `{_server, alpha}` → `/api/projects` response is `["alpha"]`. **Added in plan 05-02 Task 3** (closes the previously-untested `_server` exclusion FILTER — this is the handler's `_server` filtering, NOT D-05 which is the client-side sort; review Round-2 LOW #7).
- [ ] `web/src/lib/constitution-load.test.ts` — Vitest-only load-seam unit test (constitution `load()`: `depends('app:project')` + response mapping + `loadError`). **Added in plan 05-13 Task 3** (review Round-2 MEDIUM #3 — anchors the D-01/D-02/D-10 load mechanism without a component harness).
- [ ] Verify Node satisfies Vite 8 (`node -v` ≥ 20.19 or ≥ 22.12).

*(No framework install needed for `.ts` unit tests — Vitest already present.)*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dark/light theme toggle flips `.dark`; no FOUC on reload | D-14 | Visual/perceptual; FOUC guard timing not unit-observable | Toggle in nav; reload; confirm no flash of wrong theme; confirm persisted across reload |
| Project switch visibly re-renders all project-scoped views with new data | D-01, D-10 | End-to-end VISUAL across 5 pages; full component harness not adopted (load-seam mechanism now unit-tested in `constitution-load.test.ts`) | Switch project in nav; verify Dashboard/Graph/Constitution/Spec/Decision all show the new project's data, no stale content |
| Selector states: 0 projects (empty state, no picker), 1 (static label), >1 (dropdown) | D-07, D-08 | Requires seeded project fixtures; visual | Exercise each project-count scenario; confirm correct nav rendering + main-content empty state |
| Active-project indicator visible on pages | D-11 | Visual placement/legibility | Confirm project name appears in breadcrumb/heading on each scoped page |
| shadcn visual redesign (Slate palette, categorical badges keep fixed light/dark pairs) | D-12, D-13 | Aesthetic/contrast judgment | Visual review of all 15 migrated components in both themes; verify badges are NOT `--primary` |

*Component-testable items (D-01/D-07/D-08/D-10) move from manual to automated if the Wave-0 component test lib is adopted.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify (inner loop = `pnpm -C web test`)
- [ ] Wave 0 covers all MISSING references (`project.test.ts` in 05-02; component-lib decision recorded)
- [ ] No watch-mode flags (use `vitest run` / `pnpm -C web test`, not `vitest --watch`)
- [ ] Feedback latency < 30s (inner loop); full `pnpm build` reserved for per-wave gate
- [ ] `nyquist_compliant: true` set in frontmatter once Wave 0 complete

**Approval:** pending
