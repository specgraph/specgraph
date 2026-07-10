---
phase: 5
round: 2
reviewers: [cursor]
reviewed_at: 2026-07-10T22:19:51Z
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md, 05-04-PLAN.md, 05-05-PLAN.md, 05-06-PLAN.md, 05-07-PLAN.md, 05-08-PLAN.md, 05-09-PLAN.md, 05-10-PLAN.md, 05-11-PLAN.md, 05-12-PLAN.md, 05-13-PLAN.md]
supersedes: "round 1 (commit b9f8dbec)"
---

# Cross-AI Plan Review — Phase 5 (Round 2, post-revision)

> Round 2 re-review of the plans after the `--reviews` revision pass addressed Round 1.
> Round 1 findings are preserved in git history (commit `b9f8dbec`).

## Cursor Review

# Phase 5 Plan Review (Round 2)

## Summary

The revisions are **sound and materially address Round 1**. The plans still match the repo (static SPA, `bind:value` switch bug, `onMount`/`$effect` fetches, constitution stale-badge anti-pattern, `_server` filter in Go). Round 1 fixes are encoded as concrete tasks: `app.css` import in 05-01, Go `_server` test in 05-02, explicit Wave 3 scope boundary for D-01, layout-owned breadcrumb with Wave 3 removals, default skeleton strategy, and documented manual-UAT appetite. Remaining risk is **execution-order UX** (transient double breadcrumbs mid-wave) and **still-heavy manual verification** for the phase’s core outcomes—not architectural flaws.

---

## Resolution Check

| Round-1 finding | Verdict | Evidence |
|-----------------|---------|----------|
| **HIGH — Wave 2 runs before Tailwind is active** | **RESOLVED** | Current tree has no Tailwind deps (`web/package.json:12-28`), no `app.css`, and no stylesheet import in `web/src/routes/+layout.svelte:1-6`. Revised **05-01** adds `web/src/app.css` in Task 2, wires `@tailwindcss/vite` in Task 1, and adds `import '../app.css'` in Task 3 *before* Wave 2 migrations (05-04..09 still `depends_on: ["05-01"]` only, which is now sufficient). |
| **HIGH — Project switch not end-to-end until Wave 3** | **RESOLVED** (planning) | Bug still real today: picker binds `project.current` without invalidation (`web/src/routes/+layout.svelte:44-49`); pages fetch once via `onMount`/`$effect` (`web/src/routes/+page.svelte:95`, `graph/+page.svelte:13`, `constitution/+page.svelte:26`, `decision/[...slug]/+page.svelte:45`). **05-03** explicitly disclaims end-to-end D-01; **05-10/11/13** add `+page.ts` + `depends('app:project')`. |
| **MEDIUM — D-11 breadcrumb duplication** | **PARTIAL** | **05-03** declares layout as single owner; **05-11/13** require removing per-page `<nav class="breadcrumb">` with automated checks (spec at `web/src/routes/spec/[...slug]/+page.svelte:171-173`, decision at `:48-50`, constitution at `:68-70`). Keys correctly excluded (`web/src/routes/keys/+page.svelte:75-77`; 05-12). **Gap:** 05-03 lands in Wave 2 while removals are Wave 3 → **transient double breadcrumbs** on constitution/spec/decision between waves. **05-03** does not specify `{View}` derivation from `$page.url` (pathname vs slug for detail routes). |
| **MEDIUM — Pitfall 3 skeleton strategy implicit** | **RESOLVED** | RESEARCH Open Questions #2 locked streamed `{#await}` as default; **05-10** Task 1 mandates it; graph may use `switching` flag only (`05-10` Task 2). **05-PATTERNS.md** forbids `$navigating` for switch skeletons. |
| **MEDIUM — `/api/projects` `_server` exclusion untested** | **RESOLVED** | Filter exists at `internal/server/api_handler.go:33-36`. **05-02** Task 3 adds `TestAPIHandler_ExcludesServerProject` using existing `apiTestResolver` / `RegisterAPIHandlers` pattern (`internal/server/api_handler_test.go:17-111`). `storage.Project.Slug` field matches plan (`internal/storage/project.go:12-14`). |
| **MEDIUM — Core switch behavior manual-UAT only** | **PARTIAL** (explicit tradeoff) | **05-VALIDATION.md** records component-test harness **out of appetite**; `nyquist_compliant: false` retained. Added automation: `project.test.ts` (05-02) + Go test + structural `+page.ts` string verifies—not end-to-end switch refetch. `web/package.json` still has only Vitest, no `@testing-library/svelte` (`:12-21`). |

---

## Strengths

- **Root-cause diagnosis still accurate.** `projectInterceptor` threads header from `project.current` (`web/src/lib/api/client.ts:15-17`); picker updates state without reload (`+layout.svelte:44-49`); constitution `$effect(() => { load(); })` has no project dependency (`constitution/+page.svelte:12-26`)—exactly Pitfall 4 / D-10 target.
- **Static-SPA constraints preserved.** `ssr = false` / `prerender = false` (`web/src/routes/+layout.ts:3-4`); `adapter-static` (`web/svelte.config.js:6-8`); `//go:embed all:build` (`web/embed.go:12`)—no server `load()` proposed.
- **Round 1 suggestions incorporated systematically.** `onValueChange` → `switchProject` → `invalidateAll()` (05-03); `depends('app:project')` escape hatch (05-10/11/13); TDD for D-04/05/06 (05-02); graph dark-mode pairs called out (05-09).
- **Go test design is correct.** Handler scopes via `_server` then filters slugs (`api_handler.go:21-36`); new test should use a **separate** backend variant (not mutating `fakeProjectBackend` that returns `nil` at `api_handler_test.go:30-31`).
- **Keys fence remains tight.** User-scoped `onMount` fetch (`keys/+page.svelte:17-18`); 05-12 forbids `+page.ts` / `invalidateAll()`.

---

## Concerns

| Severity | Issue | Evidence / mechanism |
|----------|-------|-------------------|
| **MEDIUM** | **Transient double breadcrumbs during execution.** 05-03 adds layout `{project} / {View}` in Wave 2; per-page breadcrumbs removed only in Wave 3 (05-11/13). Constitution/spec/decision still have page breadcrumbs today (`constitution/+page.svelte:68-70`, `spec/[...slug]/+page.svelte:171-173`, `decision/[...slug]/+page.svelte:48-50`). | Implementers finishing Wave 2 before Wave 3 ship duplicated indicators despite Round 1 fix intent. |
| **MEDIUM** | **`{View}` label mapping underspecified for dynamic routes.** UI-SPEC shows `{project} / {View}` (`05-UI-SPEC.md:130`) but 05-03 does not define View for `/spec/[slug]` or `/decision/[slug]` (static "Spec" vs slug). Removing page breadcrumbs drops current nav trail (`Dashboard / Graph / {slug}` at spec/decision pages). | D-11 satisfied for *project visibility*; wayfinding regression vs today unless page titles compensate. |
| **MEDIUM** | **Manual UAT remains the gate for D-01/D-10/D-11.** Documented in VALIDATION.md; no integration test for `invalidateAll()` → page reload (Round 1 suggestion #8 not adopted). | Structural `node -e` checks won't catch stale data after switch. |
| **LOW** | **ROADMAP still tags 05-03 with D-01** while plan disclaims end-to-end switch until Wave 3 (`.planning/ROADMAP.md:151` vs `05-03-PLAN.md` scope boundary). | Could mislead wave sign-off. |
| **LOW** | **05-01 Task 2 verify runs `pnpm build` before layout imports `app.css`.** Task 3 adds the import; within-plan order is safe, but Task 2 acceptance "Tailwind active" is weaker until Task 3. | Unreferenced `app.css` may not ship in bundle until import lands. |
| **LOW** | **mode-watcher localStorage key marked `[ASSUMED]`** in 05-03 Task 1. | FOUC guard fails silently if key wrong. |
| **LOW** | **05-02 Task 3 wording ties Go test to "D-05 server-side slug contract."** D-05 is client sort; server only filters `_server`. | Harmless intent; wording slip only. |

---

## Suggestions

1. **Add a Wave 2 completion note to ROADMAP / 05-03:** "D-01 not accepted until Wave 3 merges" so 05-03’s D-01 tag doesn’t imply false confidence.
2. **Specify `{View}` mapping in 05-03 or PATTERNS** (e.g. pathname → Dashboard/Graph/Constitution/Spec/Decision; detail pages use static view name, slug stays in `<h1>`).
3. **Order Wave 3 breadcrumb removals early** within the wave (05-11/13 before or with 05-03 sign-off), or add a 05-03 follow-up task to strip page breadcrumbs immediately when layout breadcrumb lands.
4. **Document `project.test.ts` module-reset pattern** (reset `project.current` / `localStorage` between cases) since `project.svelte.ts` uses module-level `$state` (`project.svelte.ts:4-8`).
5. **Confirm mode-watcher key in 05-03 Task 1** before FOUC script ships (plan already flags this—make it a hard verify).
6. **Optional post-Wave-3:** one Vitest test mocking transport + calling `invalidateAll()` (Round 1 #8)—even a single test would anchor D-01 better than manual UAT alone.

---

## Risk Assessment

**Overall: LOW–MEDIUM**

**Justification:** Round 1 architectural risks are addressed in plan text with repo-grounded mechanisms. The switch/load/`invalidateAll()` design is coherent against current code. Risk is now primarily **execution sequencing** (double breadcrumbs mid-wave, Wave 2 “switch works” misread from ROADMAP tags) and **verification depth** (manual UAT for the phase’s defining behaviors). No fundamental design flaws; revisions are correct enough to execute with the clarifications above.

---

## Consensus Summary

Single reviewer (Cursor, `cursor-agent` 2026.07.09), source-grounded against the real
`web/` and `internal/server/` files. **Overall verdict: LOW–MEDIUM risk — revisions are
sound and correct enough to execute.** Both Round-1 HIGH findings are RESOLVED; the two
MEDIUM items that remain are execution-sequencing and verification-depth, not design flaws.

### Round-1 Resolution Scorecard
- **[HIGH] Tailwind before Wave 2** → **RESOLVED** (05-01 imports `app.css` in Wave 1; Wave 2 `depends_on: [05-01]` now sufficient).
- **[HIGH] Switch not e2e until Wave 3** → **RESOLVED** (05-03 disclaims e2e D-01; 05-10/11/13 add `+page.ts` + `depends('app:project')`).
- **[MED] Breadcrumb duplication** → **PARTIAL** (owner = layout; per-page removals in Wave 3 → transient double breadcrumbs mid-wave; `{View}` mapping underspecified).
- **[MED] Skeleton strategy** → **RESOLVED** (streamed `{#await}` default; `switching` flag graph-only).
- **[MED] `_server` untested** → **RESOLVED** (05-02 Go test using existing `apiTestResolver` pattern).
- **[MED] Manual-UAT only** → **PARTIAL** (explicit accepted tradeoff; `nyquist_compliant: false`; no e2e switch test).

### Remaining Concerns (for optional Round-3 replan or execution-time attention)
- **[MED] Transient double breadcrumbs mid-wave** — 05-03 adds layout breadcrumb (Wave 2) but per-page removals are Wave 3. Fix: order 05-11/13 breadcrumb strips early in Wave 3, or strip immediately when the layout breadcrumb lands.
- **[MED] `{View}` label mapping underspecified** for `/spec/[slug]` & `/decision/[slug]` — removing page breadcrumbs risks a wayfinding regression unless `{View}` derivation (pathname → static view name, slug stays in `<h1>`) is specified in 05-03/PATTERNS.
- **[MED] Manual UAT remains the gate for D-01/D-10/D-11** — accepted, but one Vitest test mocking transport + `invalidateAll()` would anchor D-01.
- **[LOW]** ROADMAP still tags 05-03 with D-01 while the plan disclaims e2e switch until Wave 3 — add a "D-01 not accepted until Wave 3" note.
- **[LOW]** `project.test.ts` should document a module-reset pattern (module-level `$state` in `project.svelte.ts`).
- **[LOW]** Confirm the `mode-watcher` localStorage key (currently `[ASSUMED]`) before the FOUC script ships.
- **[LOW]** 05-02 Task 3 wording ties the Go `_server` test to "D-05 server-side slug contract" — D-05 is client sort; wording slip only.

### Divergent Views
_(none — single reviewer)_

### Disposition
These are refinements, not blockers. They can be folded in via `/gsd-plan-phase 5 --reviews`
(cheap plan edits: breadcrumb-strip ordering, `{View}` mapping, ROADMAP note) or simply
attended to during execution. The plans are executable as-is.
