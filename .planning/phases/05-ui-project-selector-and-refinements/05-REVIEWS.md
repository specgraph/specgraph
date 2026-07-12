---
phase: 5
round: 3
reviewers: [cursor]
reviewed_at: 2026-07-12T13:27:15Z
plans_reviewed: [05-01-PLAN.md, 05-02-PLAN.md, 05-03-PLAN.md, 05-04-PLAN.md, 05-05-PLAN.md, 05-06-PLAN.md, 05-07-PLAN.md, 05-08-PLAN.md, 05-09-PLAN.md, 05-10-PLAN.md, 05-11-PLAN.md, 05-12-PLAN.md, 05-13-PLAN.md]
verdict: "LOW risk — execute now"
supersedes: "round 2 (commit 186093f9), round 1 (commit b9f8dbec)"
---

# Cross-AI Plan Review — Phase 5 (Round 3, post-revision-2)

> Round 3 re-review after the second `--reviews` pass. Rounds 1 & 2 preserved in git
> history (commits `b9f8dbec`, `186093f9`). **Disposition: execute now.**

## Cursor Review

# Phase 5 Plan Review (Round 3)

## Summary

**The plans are execution-ready.** Round-2 revisions landed in the plan artifacts: the transient double-breadcrumb problem is closed by an atomic strip in 05-03, `{View}` derivation is specified, the Vitest load-seam test is added to 05-13, and the LOW hygiene items (module reset, mode-watcher key verification, Go-test wording) are addressed. Source verification against the current tree confirms the plans still describe real bugs and the right fix mechanisms. No new architectural blockers surfaced from the Round-2 edits.

**Recommendation: execute-now** (optional one-line ROADMAP caveat during execution; not a replan gate).

---

## Round-2 Resolution Check

| Round-2 item | Verdict | Evidence |
|--------------|---------|----------|
| **MEDIUM #1 — Transient double breadcrumbs** | **RESOLVED** | `05-03-PLAN.md` Task 3 strips `<nav class="breadcrumb">` from constitution/spec/decision **in the same task** that adds the layout breadcrumb; automated verify greps all three pages for `class="breadcrumb"` (`05-03` L133–134). `05-11`/`05-13` re-verify removal and forbid re-introduction (e.g. `05-11` L20, L67). Current markup still present at `constitution/+page.svelte:68`, `spec/[...slug]/+page.svelte:171`, `decision/[...slug]/+page.svelte:48`. |
| **MEDIUM #2 — `{View}` label mapping underspecified** | **RESOLVED** | `05-03` Task 3 defines a static pathname map: `/` → Dashboard, `/graph` → Graph, `/constitution` → Constitution, `/spec/...` → Spec, `/decision/...` → Decision; slugs stay in page `<h1>` (spec L180, decision L57). Suppressed on `/keys` (D-09). Matches `05-UI-SPEC.md` D-11 contract (L130). Not duplicated in `05-PATTERNS.md`, but the Round-2 suggestion was “05-03 **or** PATTERNS” — 05-03 is sufficient. |
| **MEDIUM #3 — Manual UAT only for switch/load** | **PARTIAL** (improved, accepted) | Full component harness remains out of appetite (`05-VALIDATION.md`). **05-13** Task 3 adds `constitution-load.test.ts` — Vitest-only, four behaviors (`depends('app:project')`, mapping, empty, `loadError`), no new deps. Narrows manual surface to visual badge re-derivation (D-10). End-to-end `invalidateAll()` → page re-render still manual. |
| **LOW #4 — ROADMAP D-01 tag misleading** | **PARTIAL** | Caveat added in `05-03-PLAN.md` L49 (“mechanism introduced”, not satisfied until Wave 3). **`.planning/ROADMAP.md` L151** still tags 05-03 with D-01 without inline caveat — cosmetic, not a design flaw. |
| **LOW #5 — `project.test.ts` module reset** | **RESOLVED** | `05-02` Task 1 mandates `beforeEach` clearing module-level `$state` and `localStorage` shim (L73–80). Matches real leak risk in `web/src/lib/project.svelte.ts:4-8`. |
| **LOW #6 — mode-watcher localStorage key assumed** | **RESOLVED** | `05-03` Task 1: HARD PREREQUISITE to grep `node_modules/mode-watcher` before writing FOUC script; automated verify checks `mode-watcher-mode` in `app.html` (L81–88). Current `web/src/app.html:1-12` has no FOUC script yet. |
| **LOW #7 — 05-02 Go test wording (D-05 vs `_server`)** | **RESOLVED** | `05-02` Task 3 action explicitly separates client D-05 sort from server `_server` filter (L118). |

### Round-1 carryover (still valid)

| Round-1 finding | Verdict | Evidence |
|-----------------|---------|----------|
| **HIGH — Tailwind before Wave 2** | **RESOLVED** | No Tailwind deps (`web/package.json:12-28`), no `app.css`, no stylesheet import (`+layout.svelte:1-6`). **05-01** Task 3 adds `import '../app.css'` in Wave 1 before 05-04..09. |
| **HIGH — Switch not e2e until Wave 3** | **RESOLVED** (planning) | Bug real today: `bind:value` on picker (`+layout.svelte:44-49`), `onMount`/`$effect` fetches (`+page.svelte:95`, `graph/+page.svelte:13`, `constitution/+page.svelte:26`, `decision/[...slug]/+page.svelte:45`). **05-03** disclaims e2e D-01; **05-10/11/13** add `+page.ts` + `depends('app:project')`. |
| **MEDIUM — Skeleton strategy** | **RESOLVED** | Streamed `{#await}` default in **05-10**; graph `switching` flag optional; `05-PATTERNS.md` forbids `$navigating` (L337). |
| **MEDIUM — `_server` exclusion untested** | **RESOLVED** | Filter at `internal/server/api_handler.go:33-36`; `storage.Project.Slug` at `project.go:12-14`. **05-02** Task 3 adds `TestAPIHandler_ExcludesServerProject` using existing `apiTestResolver` pattern (`api_handler_test.go:17-45`). Test not present yet. |
| **MEDIUM — Manual-UAT gate** | **PARTIAL** (accepted) | Documented tradeoff; `nyquist_compliant: false` retained. |

---

## Strengths

- **Round-2 fixes are concrete, not hand-wavy.** The atomic breadcrumb strip has an automated gate; `{View}` mapping is spelled out in the task action; the constitution load-seam test has explicit behaviors and verify commands.
- **Cross-wave file overlap is acknowledged and safe.** `05-03` limits edits to constitution/spec/decision to markup-only breadcrumb removal; fetch logic and `<style>` blocks stay until 05-11/05-13. No same-wave collision — 05-04..09 do not touch those route files.
- **Root-cause diagnosis still matches the tree.** `projectInterceptor` threads header from `project.current` (`client.ts:15-17`); picker updates state without invalidation (`+layout.svelte:44-49`); constitution `$effect(() => { load(); })` has no project dependency (`constitution/+page.svelte:26`) — exactly Pitfall 4 / D-10.
- **Static-SPA constraints preserved.** `ssr = false` / `prerender = false` (`+layout.ts:3-4`); `adapter-static` (`svelte.config.js:6-8`); `//go:embed all:build` (`web/embed.go:12`).
- **05-02 TDD contract is tight.** Six tests pin D-04 three-tier precedence (including `'default'`), D-05 sort, D-06 stale fallback, D-07 zero-projects — all against current `loadProjects()` gap (`project.svelte.ts:27-30` picks `available[0]`, no sort, no `default` tier).

---

## Concerns

| Severity | Issue | Evidence / mechanism |
|----------|-------|-------------------|
| **LOW** | **ROADMAP.md still tags 05-03 with D-01 without inline caveat.** | Plan-level disclaimer exists (`05-03-PLAN.md:49`); ROADMAP L151 unchanged. Could mislead wave sign-off if someone reads ROADMAP only. |
| **LOW** | **`constitution-load.test.ts` is the first route-`load` Vitest import.** | Prior tests are lib-only (`oidc.test.ts`, `keys.test.ts`). Plan design is sound (`vi.mock` client, hand-rolled load event), but if `+page.ts` resolution fails under Vitest, extract load to `$lib/` — execution-time fallback, not a replan blocker. |
| **LOW** | **Manual UAT remains the gate for visual D-01/D-07/D-08/D-11 and badge re-derivation (D-10).** | Explicit accepted tradeoff in `05-VALIDATION.md`. Load mechanism partially automated via `constitution-load.test.ts`. |
| **LOW** | **Spec/decision lose in-page `Dashboard / Graph / {slug}` trail after breadcrumb strip.** | Intentional per D-11 redesign; main nav links remain (`+layout.svelte:39-41`); slug/title stays in `<h1>`. Accepted UX tradeoff from Round 2. |
| **LOW** | **Orphaned `.breadcrumb` CSS on stripped pages until Wave 3.** | `05-03` explicitly allows this; 05-11/13 delete `<style>` on full restyle. Harmless. |

**No HIGH or blocking MEDIUM concerns.** The Round-2 edit that has 05-03 touch three route files does **not** collide with Wave 2 component migrations or strip markup those pages depend on for rendering.

---

## Suggestions

1. **During 05-03 execution**, add a one-line note to `.planning/ROADMAP.md` under the 05-03 entry: “D-01 mechanism only — not accepted until Wave 3” (optional; plan already has it).
2. **If `constitution-load.test.ts` import fails**, move load body to `web/src/lib/constitution-load.ts` and re-export from `+page.ts` — keep the test file path stable.
3. **Post-Wave-3 optional:** one Vitest test asserting `switchProject` calls `invalidateAll()` (mock `$app/navigation`) — would further anchor D-01 without a component harness.

---

## Risk Assessment

**Overall: LOW**

**Justification:** Both Round-1 HIGH findings remain resolved in plan text. All three Round-2 MEDIUM items are resolved or materially improved (atomic breadcrumb, `{View}` map, load-seam test). Residual risk is execution/verification depth (manual UAT for visual switch behavior) and one cosmetic ROADMAP note — not architectural.

### Disposition

**Execute now.** Do not run another revision pass unless execution discovers a concrete failure (e.g. Vitest cannot import `+page.ts` load without extraction). The twice-revised plans are coherent, repo-grounded, and wave-ordered correctly.

---

## Consensus Summary

Single reviewer (Cursor, `cursor-agent` 2026.07.09), source-grounded. **Verdict: LOW risk —
execute now.** Across three rounds the plans converged: both Round-1 HIGH findings and all
three Round-2 MEDIUM findings are RESOLVED or materially improved. No HIGH or blocking-MEDIUM
concerns remain; residuals are cosmetic/verification-depth LOW items already accepted as tradeoffs.

### Three-Round Convergence
- **Round 1** → MEDIUM risk; 2 HIGH + 4 MED findings.
- **Round 2** → LOW–MEDIUM; both HIGH RESOLVED, 3 MED refinements raised.
- **Round 3** → **LOW; execute now.** All Round-2 MED resolved/improved.

### Remaining (LOW, non-blocking)
- ROADMAP L151 still tags 05-03 with D-01 without an inline caveat (the plan itself carries the caveat) — cosmetic; optionally add a one-line ROADMAP note during 05-03 execution.
- `constitution-load.test.ts` is the first route-`load` Vitest import — sound design; if `+page.ts` resolution fails under Vitest, extract the load body to `$lib/constitution-load.ts` (execution-time fallback, not a replan blocker).
- Manual UAT remains the gate for visual D-01/D-07/D-08/D-11 + D-10 badge re-derivation — explicit accepted tradeoff (`nyquist_compliant: false`).
- Spec/decision lose the in-page `Dashboard / Graph / {slug}` trail after the breadcrumb strip — intentional per D-11; main nav + `<h1>` slug remain.
- Orphaned `.breadcrumb` CSS on stripped pages until Wave 3 `<style>` retirement — harmless dead rule.

### Divergent Views
_(none — single reviewer)_

### Disposition
**Execute now.** Do not run another revision pass unless execution discovers a concrete failure
(e.g. Vitest cannot import `+page.ts` load without extraction). The twice-revised plans are
coherent, repo-grounded, and correctly wave-ordered. The remaining LOW items are best handled
at execution time, not via another replan.
