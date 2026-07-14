---
phase: 05-ui-project-selector-and-refinements
reviewed: 2026-07-12T15:11:56Z
depth: standard
files_reviewed: 39
files_reviewed_list:
  - web/vite.config.ts
  - web/src/app.css
  - web/src/app.html
  - web/components.json
  - web/src/lib/utils.ts
  - web/src/lib/components/badge-variants.ts
  - web/src/lib/project.svelte.ts
  - web/src/lib/project.test.ts
  - web/src/lib/constitution-load.test.ts
  - internal/server/api_handler_test.go
  - web/src/routes/+layout.ts
  - web/src/routes/+layout.svelte
  - web/src/lib/components/ModeToggle.svelte
  - web/src/lib/components/AccordionSection.svelte
  - web/src/lib/components/TabBar.svelte
  - web/src/lib/components/FindingsSection.svelte
  - web/src/lib/components/SpecTable.svelte
  - web/src/lib/components/StatsBar.svelte
  - web/src/lib/components/FunnelBar.svelte
  - web/src/lib/components/SearchFilter.svelte
  - web/src/lib/components/MetadataBar.svelte
  - web/src/lib/components/ChangelogTimeline.svelte
  - web/src/lib/components/LoginModal.svelte
  - web/src/lib/components/RevealKeyModal.svelte
  - web/src/lib/components/DiffView.svelte
  - web/src/lib/components/VersionCompare.svelte
  - web/src/lib/components/Graph.svelte
  - web/src/lib/components/GraphMini.svelte
  - web/src/routes/+page.ts
  - web/src/routes/+page.svelte
  - web/src/routes/graph/+page.ts
  - web/src/routes/graph/+page.svelte
  - web/src/routes/spec/[...slug]/+page.ts
  - web/src/routes/spec/[...slug]/+page.svelte
  - web/src/routes/decision/[...slug]/+page.ts
  - web/src/routes/decision/[...slug]/+page.svelte
  - web/src/routes/keys/+page.svelte
  - web/src/routes/constitution/+page.ts
  - web/src/routes/constitution/+page.svelte
findings:
  critical: 0
  warning: 3
  info: 6
  total: 9
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-07-12T15:11:56Z
**Depth:** standard
**Files Reviewed:** 39
**Status:** issues_found

## Summary

Reviewed the SvelteKit 5 (runes) + shadcn-svelte project-selector/refinements migration:
universal `load()` seams, Svelte 5 runes usage, the `{@html}`/XSS surface, the
auth/CSRF flows, and the project-scoping header propagation. The core architecture
is sound and the phase's stated concerns are largely well-handled:

- **XSS:** Clean. No `{@html}`, `innerHTML`, `eval`, or `dangerouslySetInnerHTML`
  anywhere in `web/src`. Every dynamic value (spec content, constitution fields,
  diff hunks, graph labels, the one-time plaintext key) is rendered through
  Svelte's auto-escaping interpolation.
- **load() correctness:** The stream-a-promise + `{#await}` re-suspend pattern is
  applied consistently; errors are caught *inside* the streamed promises and
  surfaced as `loadError`/`notFound` sentinels, so a failed RPC renders an inline
  Retry card and never propagates to `+error.svelte`. `depends('app:project')` is
  registered on every scoped page and the layout `switchProject` calls
  `invalidateAll()`.
- **Project-scoping header:** `projectInterceptor` reads the reactive
  `project.current` (falling back to `'default'`) on every ConnectRPC, and the
  switch → persist → `invalidateAll()` → re-run-load path re-issues RPCs with the
  new `X-Specgraph-Project` header.
- **Auth/CSRF:** The double-submit `csrfInterceptor` echoes the non-HttpOnly cookie
  into `X-CSRF-Token` for all Connect mutations; the reveal-key modal holds the
  plaintext only in component-local state and clears it on close.

No BLOCKER-level defects were found. The issues below are robustness and
maintainability concerns, plus a dev-tooling gap that breaks `vite dev`.

## Warnings

### WR-01: Dashboard load is all-or-nothing — one non-critical RPC failure blanks the whole page

**File:** `web/src/routes/+page.ts:44-77`
**Issue:** `loadDashboardData()` fires all five RPCs inside a single `Promise.all`
and wraps the whole thing in one `try/catch`. Any single rejection —
`getReady`, `checkDrift`, or `listDecisions` — rejects the `Promise.all`, hits the
catch, and returns `emptyDashboard(loadError)`, so the user sees the *"Couldn't
load dashboard"* Retry card even when the primary `listSpecs`/`getFullGraph` calls
succeeded. This is inconsistent with the deliberate per-fetch resilience the same
phase applied in the detail loaders (`spec/[...slug]/+page.ts:55-78` and
`decision/[...slug]/+page.ts:48-58`), where secondary fetches are individually
`try/caught` with `[]` fallbacks so a secondary failure never loses the primary
entity. The dashboard's stat bar (drift/ready/decisions counts) is exactly the
kind of secondary data that should degrade gracefully, not blank the page.
**Fix:** Load the primary data (`listSpecs`, `getFullGraph`) as the critical path
and defensively default the secondary counts, e.g.:
```ts
const [specsRes, graphRes] = await Promise.all([
  specClient.listSpecs({}),
  graphClient.getFullGraph({}),
]);
// secondary — never fail the whole dashboard
const readyCount = await graphClient.getReady({})
  .then((r) => (r.ready ?? []).length).catch(() => 0);
const decisions = await decisionClient.listDecisions({})
  .then((r) => r.decisions ?? []).catch(() => []);
const reports = await lifecycleClient.checkDrift({ slug: '' })
  .then((r) => r.reports ?? []).catch(() => []);
```
Reserve `loadError` for a failure of the two critical calls only.

### WR-02: `loadProjects()` conflates a fetch failure with "zero projects"

**File:** `web/src/lib/project.svelte.ts:20-47`
**Issue:** The precedence/assignment logic lives entirely inside `if (resp.ok)`.
When `/api/projects` returns a non-OK status or the `fetch` throws, the function
falls through to the silent `catch`/end, sets `loaded = true`, and leaves
`available` at its initial `[]`. The layout keys its terminal empty state purely on
`project.available.length === 0` (`+layout.svelte:105`), so a *transient* projects
API failure on first load renders the permanent *"No projects found — create a
project with the CLI"* screen (hiding all nav and `{@render children()}`) with no
error signal or retry, indistinguishable from a genuinely empty account.
Additionally a stale saved `current` slug can survive in `localStorage` yet be
silently ignored because `available` is empty.
**Fix:** Track fetch failure distinctly from an empty result set and give the
layout a third state (error/retry) vs. the true zero-projects empty state:
```ts
let loadFailed = $state(false);
// ...
if (resp.ok) { /* existing precedence */ }
else { loadFailed = true; }
// in catch: loadFailed = true;
```
Then branch the layout on `loaded && loadFailed` → retry card, vs.
`available.length === 0` → the CLI empty state.

### WR-03: Vite dev proxy forwards RPCs but not the `/api/*` REST endpoints

**File:** `web/vite.config.ts:7-14`
**Issue:** The dev `server.proxy` only forwards `/specgraph.v1` (the ConnectRPC
prefix) to `http://localhost:8080`. All auth/bootstrap traffic goes to `/api/*`
via plain `fetch` — `/api/projects` (`project.svelte.ts:22`), `/api/auth/whoami`,
`/api/auth/login`, `/api/auth/logout` (`auth.svelte.ts:17,34,52`), and
`/api/auth/oidc/providers` (`oidc.svelte.ts:10`), plus the `/api/auth/oidc/<id>/start`
button href. Under `vite dev` these paths resolve against the SvelteKit dev server,
which has no such routes, so `checkAuth()`/`loadProjects()` fail and the app is
stuck on the login modal (or a false zero-projects state per WR-02). Since a dev
proxy for `/specgraph.v1` exists at all, the intended dev flow is clearly
vite → Go server on :8080, which makes the missing `/api` entry a real dev-loop
break.
**Fix:** Add the REST prefix to the proxy table:
```ts
server: {
  proxy: {
    '/specgraph.v1': { target: 'http://localhost:8080', changeOrigin: true },
    '/api':          { target: 'http://localhost:8080', changeOrigin: true },
  },
},
```

## Info

### IN-01: `$app/stores` `page` is deprecated in SvelteKit 2

**File:** `web/src/routes/+layout.svelte:3,58`
**Issue:** The layout imports `{ page }` from `$app/stores` and reads
`$page.url.pathname` inside `$derived(...)`. In current SvelteKit 2 the runes-native
replacement is `page` from `$app/state` (`import { page } from '$app/state'`;
`$derived(page.url.pathname)`), and `$app/stores` is on the deprecation path. Works
today, but mixing the legacy store auto-subscription with runes is the kind of thing
that breaks on a framework bump.
**Fix:** Migrate to `$app/state`:
`import { page } from '$app/state'` and `const pathname = $derived(page.url.pathname);`.

### IN-02: Keyless `{#each}` over dynamic lists

**File:** `web/src/lib/components/LoginModal.svelte:53`, `web/src/lib/components/FindingsSection.svelte:74,95`, `web/src/routes/+page.svelte:130,149`, `web/src/routes/spec/[...slug]/+page.svelte:369,372`
**Issue:** Several `{#each}` blocks over dynamic/derived collections omit a
`(key)` expression (`providers as p`, `grouped as group`, `d.decisions as dec`,
`priorityGroups(...) as group`, edge `items as item`). For lists that never reorder
this is harmless, but keyless each blocks re-use DOM by index and can mis-associate
component state if the list is ever reordered or filtered in place.
**Fix:** Add stable keys, e.g. `{#each providers as p (p.id)}`,
`{#each d.decisions as dec (dec.slug)}`, `{#each group.findings as finding (finding.summary)}`.

### IN-03: `GraphMini` keyboard handler only responds to Enter, not Space

**File:** `web/src/lib/components/GraphMini.svelte:19-21`
**Issue:** The card is given `role="link"` and `tabindex={0}` but `onkeydown` only
navigates on `e.key === 'Enter'`. Native links activate on Enter; for a `role="link"`
element that's acceptable, but the surrounding `role`/`tabindex`/`onclick` pattern
is really a button-like affordance where Space is commonly expected.
**Fix:** Either navigate on both keys (`(e.key === 'Enter' || e.key === ' ')`) or,
simpler and fully accessible, wrap the mini-graph in a real `<a href="/graph">`
instead of a click-handler card.

### IN-04: `listChanges({ limit: 0 })` relies on an undocumented "0 = all" magic value

**File:** `web/src/routes/spec/[...slug]/+page.svelte:43`
**Issue:** The changelog fetch passes `limit: 0` intending "return everything." This
depends entirely on the server interpreting `0` as unbounded rather than as a literal
zero-row limit; the meaning isn't visible at the call site.
**Fix:** Document the sentinel with a comment or use an explicit sentinel constant
(e.g. `const ALL_CHANGES = 0; // server: 0 = no limit`), and confirm the server
contract treats `0` as unlimited.

### IN-05: Changelog reset via `$effect` writing state is better expressed with `{#key}`

**File:** `web/src/routes/spec/[...slug]/+page.svelte:32-37`
**Issue:** The changelog cache is reset by an `$effect` that reads `data.detail` and
then writes three `$state` variables. It's correct (no read-write cycle) and the
intent is well-commented, but effect-driven state resets are an anti-pattern the
Svelte team explicitly steers away from; a keyed block scopes the reset declaratively
and can't accidentally grow into a loop.
**Fix:** Wrap the changelog section in `{#key data.detail} … {/key}` and make the
changelog vars plain `$state` inside it, dropping the reset `$effect`.

### IN-06: `notFound` sentinel is set by loaders but never read by the components

**File:** `web/src/routes/spec/[...slug]/+page.ts:27,39,52` (and `decision/[...slug]/+page.ts:22,34,45`)
**Issue:** Both detail loaders compute and return a `notFound` boolean, but the
components branch only on `d.loadError` then `!d.spec`/`!d.decision`. `notFound` is
effectively dead data (the empty state is reached via the null entity instead).
Harmless, but it's a field that documents an intent the UI doesn't actually consume,
which invites future confusion (e.g. a reader assuming NotFound gets a distinct 404
treatment).
**Fix:** Either drop `notFound` from the returned shape, or use it in the template to
render a distinct "not found in this project" vs. generic-empty state.

---

_Reviewed: 2026-07-12T15:11:56Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
