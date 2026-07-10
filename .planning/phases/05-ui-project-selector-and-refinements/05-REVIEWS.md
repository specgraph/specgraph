---
phase: 5
reviewers: [cursor]
reviewed_at: 2026-07-10T21:46:58Z
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md, 05-04-PLAN.md, 05-05-PLAN.md, 05-06-PLAN.md, 05-07-PLAN.md, 05-08-PLAN.md, 05-09-PLAN.md, 05-10-PLAN.md, 05-11-PLAN.md, 05-12-PLAN.md, 05-13-PLAN.md]
---

# Cross-AI Plan Review — Phase 5

## Cursor Review

# Phase 5 Plan Review: UI Project Selector & Refinements

## Summary

The plans are well grounded in the actual codebase: they correctly identify the root failure mode (project picker updates `project.current` but views never re-fetch), document the static-SPA constraints (`ssr = false`, `adapter-static`, `embed.go`), and propose a coherent fix (`+layout.ts` bootstrap → universal `load()` → `invalidateAll()` on switch). Wave ordering and pitfall callouts (Pitfalls 3–6, constitution stale badges, FOUC, shadcn CLI flags) show strong research quality. The main risks are **Wave 2 dependency ordering** (Tailwind styles before `app.css` is imported), **a functional gap between Wave 2 shell and Wave 3 page loads**, **duplicate breadcrumbs for D-11**, and **heavy reliance on manual UAT** for the phase’s core success criteria.

---

## Strengths

- **Accurate diagnosis of the switch bug.** `+layout.svelte` binds the picker to `project.current` but never invalidates loads:

```44:49:web/src/routes/+layout.svelte
    {#if project.available.length > 1}
      <select bind:value={project.current} class="project-picker">
        {#each project.available as slug}
          <option value={slug}>{slug}</option>
        {/each}
```

  Meanwhile every project-scoped page fetches once via `onMount` or a mount-only `$effect` (e.g. `+page.svelte:95`, `graph/+page.svelte:13`, `constitution/+page.svelte:26`). The `load()` + `invalidateAll()` approach in 05-03/05-10–13 directly addresses this.

- **Correct static-SPA assumptions.** `+layout.ts` already sets `ssr = false` and `prerender = false` (`web/src/routes/+layout.ts:3-4`), matching the plan’s universal `+page.ts` loads and `embed.go`’s `//go:embed all:build` (`web/embed.go:12`). No server `load()` is proposed.

- **Existing seams reused, not rewritten.** Plans extend `project.svelte.ts`, keep `projectInterceptor` (`client.ts:15-17`), and align D-04 with the `'default'` fallback already in the interceptor. `/api/projects` already excludes `_server` (`api_handler.go:33-36`).

- **Pitfall 4 (constitution badges) is evidenced in code.** Constitution stores `provenance` in `$state` and loads via `$effect(() => { load(); })` (`constitution/+page.svelte:7-26,53-58`) with no dependency on `project.current`—exactly the stale-badge scenario 05-13 targets.

- **D-09 Keys fence is explicit.** Keys uses `onMount` + user-scoped `keys.svelte.ts` (`keys/+page.svelte:17-18`); 05-12 forbids `+page.ts`, `invalidateAll()`, and project breadcrumb—appropriate for user-scoped data.

- **Sensible wave structure.** Foundation (05-01/02) → shell + component migration (05-03–09) → load-ification (05-10–13) keeps `task web:build` viable at wave boundaries. TDD for D-04/D-05/D-06 in 05-02 matches the real gap in `loadProjects()` (`project.svelte.ts:26-30`: unsorted `available[0]`, no `'default'` tier).

- **Supply-chain awareness.** Plans reference `pnpm-workspace.yaml` `minimumReleaseAgeExclude` (`web/pnpm-workspace.yaml:9-9`), Pitfall 2 (`--base-color` has no `slate`), and `@lucide/svelte` vs deprecated `lucide-svelte`.

- **Threat model is mostly honest.** T-05-04 correctly treats `X-Specgraph-Project` as client-selected; server scopes via `ProjectMiddleware` + `scopeStore` (`project.go:32-62`). Stale-render threats map to real UI bugs.

---

## Concerns

| Severity | Issue | Evidence / mechanism |
|----------|-------|-------------------|
| **HIGH** | **Wave 2 component plans can run before Tailwind is active in the app.** 05-04–09 depend only on `05-01`; `import '../app.css'` is deferred to `05-03`. Migrated components will use Tailwind utilities while pages still use scoped CSS—build passes, UI is unstyled until 05-03 lands. | `05-04` `depends_on: ["05-01"]`; 05-03 adds `import '../app.css'` in layout. No `app.css` exists today; `package.json` has no Tailwind deps (`web/package.json:12-28`). |
| **HIGH** | **Project switch is not end-to-end until Wave 3 completes.** After 05-03, `invalidateAll()` re-runs layout load but project pages still use `onMount`/`$effect`—switching will refresh shell state but **not** dashboard/graph/constitution/detail data. | All five scoped routes lack `+page.ts` today; load-ification is 05-10–13 only. |
| **MEDIUM** | **D-11 breadcrumb likely duplicates page breadcrumbs.** Layout will add `{project} / {View}` (05-03); spec, constitution, decision, and keys pages already render their own `<nav class="breadcrumb">` (`constitution/+page.svelte:68-70`, `spec/[...slug]/+page.svelte:171`, etc.). 05-10–13 do not say to remove them. Keys correctly excludes project breadcrumb (05-12); others do not. | Double breadcrumb on four project-scoped views unless consolidated. |
| **MEDIUM** | **Core switch behavior is manual-UAT only.** Validation map marks D-01/D-07/D-08/D-10/D-11 as component/manual; no `@testing-library/svelte`, no jsdom harness (`web/package.json` only has `vitest`). `nyquist_compliant: false` in embedded VALIDATION.md. | Automated verifies are mostly `node -e` string checks + `pnpm build`; they won’t catch stale data after switch. |
| **MEDIUM** | **`/api/projects` `_server` exclusion untested.** Research flags “verify existing coverage”; `api_handler_test.go` only tests auth (401/200), not slug filtering. | `api_handler.go:34` filters `_server`; no test asserts response slugs. |
| **MEDIUM** | **Pitfall 3 (switch skeletons) left implicit in Wave 3.** Plans say “streamed `{#await}` OR `switching` flag” but don’t pick one; wrong choice leaves stale data visible during switch (UI-SPEC violation). | RESEARCH Pitfall 3; 05-10 tasks list both options without a default. |
| **MEDIUM** | **`invalidateAll()` re-runs `checkAuth()` + `loadProjects()` every switch.** 05-03 moves bootstrap to `+layout.ts` load; each switch triggers whoami + `/api/projects` again. Acceptable but adds latency; `invalidate('app:project')` alternative is documented but CONTEXT locked `invalidateAll()`. | `auth.svelte.ts:15-30`, Pattern 2 tradeoff in RESEARCH. |
| **LOW** | **Decision detail doesn’t reload on slug change today; plan fixes it but current gap is worse than spec.** Spec uses `$effect` on slug (`spec/[...slug]/+page.svelte:72-77`); decision uses `onMount(() => loadDecision(slug))` only (`decision/[...slug]/+page.svelte:45`)—slug nav within project is already broken. | 05-11 correctly moves to `+page.ts` keyed on `params.slug`. |
| **LOW** | **D-12 CONTEXT still mentions “PostCSS”.** Plans/RESEARCH correctly use Tailwind v4 CSS-first (no `postcss.config.js`). Minor upstream doc drift, not plan error. | `05-CONTEXT.md` D-12 vs RESEARCH Pitfall 1. |
| **LOW** | **Security claim “server authorizes via Scoper” is slightly overstated.** `ProjectMiddleware` only injects the header into context (`project.go:35-38`); enforcement is per-handler via `scopeStore`. Client can request any valid kebab-case slug; access control depends on `scoper.Scoped` implementation, not header validation alone. | Acceptable for trusted UI; worth noting for multi-tenant hardening. |

---

## Suggestions

1. **Import `app.css` in 05-01 (one line) or make 05-04–09 depend on 05-03.** Minimal fix: add `import '../app.css'` to `+layout.svelte` at end of 05-01 Task 2 so Tailwind is live before any component migration. Otherwise Wave 2 parallel execution produces visually broken intermediate states.

2. **Consolidate breadcrumbs in Wave 3 page plans.** Either (a) layout owns `{project} / {View}` and page plans must delete per-page `<nav class="breadcrumb">`, or (b) pages own full breadcrumb including project segment and 05-03 should not add a layout-level duplicate. Pick one pattern in 05-UI-SPEC and reference it in 05-10–13 task actions.

3. **Pick a default skeleton strategy for Pitfall 3.** Recommend streamed promises from `load()` (idiomatic SvelteKit) and document it in 05-PATTERNS so implementers don’t mix approaches across pages.

4. **Add Go test for `_server` exclusion** in `api_handler_test.go` (Wave 0 item): fake backend returns `{_server, alpha}` → response `["alpha"]`. Low effort, closes validation gap.

5. **Gate Wave 2 “switch works” demos until Wave 3.** Mark intermediate success as “build green + shadcn visual” only; defer D-01 acceptance until 05-10–13 merge to avoid false confidence.

6. **05-03 Select wiring:** use explicit `onValueChange` → `switchProject` rather than `bind:value` alone, so every user change guarantees `invalidateAll()` (current `bind:value` is the bug pattern).

7. **Graph dark-mode legibility:** `Graph.svelte` hardcodes many light-theme hex fills (`Graph.svelte:17-48`). 05-09 says “theme tokens” but dagre/SVG may need explicit light/dark pairs or CSS variables—call out a manual UAT checklist item for graph contrast in both themes.

8. **Consider one integration test** after Wave 3: mock `fetch` + Connect transport, flip `project.current`, call `invalidateAll()`, assert dashboard `load()` re-invoked. Even a single test would anchor D-01 better than string-matching verifies.

---

## Risk Assessment

**Overall: MEDIUM**

**Justification:** Architecture and file-level claims match the repo closely; the phased approach is executable and build gates (`task web:build` → `task build` → `task check`) are realistic. Risk is elevated because (1) Wave 2 dependency ordering can ship broken styling and a shell that “switches” without refreshing data until Wave 3, (2) the phase’s primary user-visible outcomes (switch refresh, constitution badges, dark mode, empty states) depend heavily on manual UAT and weak automated verifies, and (3) D-11 breadcrumb consolidation is underspecified. None of these are fundamental design flaws—they are ordering, test-coverage, and UX-consistency gaps that are fixable in plan edits before execution.

With the suggested dependency and breadcrumb clarifications, the plans should achieve the phase goal: selectable project with correct defaults, views that re-fetch on switch, shadcn-Slate redesign, and constitution polish across switches.

---

## Consensus Summary

Only one reviewer (Cursor, `cursor-agent` 2026.07.09) was invoked for this phase, so
there is no cross-reviewer consensus to triangulate. The findings below are Cursor's
source-grounded assessment (it verified plan claims against the actual `web/` and
`internal/server/` files). Overall verdict: **MEDIUM risk — no fundamental design flaws;
ordering, test-coverage, and UX-consistency gaps fixable via plan edits before execution.**

### Agreed Strengths
_(single reviewer — strengths as cited by Cursor)_
- Accurate diagnosis of the switch bug (picker updates `project.current` but views never re-fetch); `load()` + `invalidateAll()` directly fixes it.
- Correct static-SPA assumptions (`ssr = false`, `prerender = false`, `embed.go` `all:build`); no server `load()`.
- Existing seams reused not rewritten (`project.svelte.ts`, `projectInterceptor`, `/api/projects` already excludes `_server`).
- D-09 Keys fence explicit; supply-chain awareness (`minimumReleaseAgeExclude`, `--base-color` no `slate`, `@lucide/svelte`); mostly-honest threat model.

### Agreed Concerns (highest priority for `--reviews` replan)
- **HIGH — Wave 2 can run before Tailwind is active.** 05-04..05-09 depend only on 05-01, but `import '../app.css'` is deferred to 05-03 → migrated components use Tailwind utilities while the app has no active stylesheet (build passes, UI unstyled until 05-03). Fix: import `app.css` in 05-01, or make 05-04..09 depend on 05-03.
- **HIGH — Project switch is not end-to-end until Wave 3.** After 05-03, `invalidateAll()` refreshes shell state but pages still use `onMount`/`$effect` until load-ification (05-10..13). Mark intermediate "switch works" as false confidence; defer D-01 acceptance to Wave 3.
- **MEDIUM — D-11 breadcrumb likely duplicates existing per-page breadcrumbs** (constitution/spec/decision). 05-10..13 don't remove them. Pick one owner (layout vs page) in UI-SPEC and reference it.
- **MEDIUM — Core switch behavior is manual-UAT only.** No component test harness (`@testing-library/svelte`/jsdom); `nyquist_compliant: false`. Add at least one integration test anchoring D-01.
- **MEDIUM — `/api/projects` `_server` exclusion untested** — add a Go test (Wave 0).
- **MEDIUM — Pitfall 3 skeleton strategy left implicit** (streamed `{#await}` vs `switching` flag) — pick a default (recommend streamed) and record in PATTERNS.
- **MEDIUM — `invalidateAll()` re-runs `checkAuth()` + `loadProjects()` every switch** — acceptable latency cost; `invalidate('app:project')` escape hatch already wired.

### Divergent Views
_(none — single reviewer)_
