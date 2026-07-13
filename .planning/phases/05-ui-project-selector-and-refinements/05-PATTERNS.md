# Phase 5: UI Project Selector & Refinements - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 34 (15 components + 6 route pages/loads + 3 scaffolding + 4 build config + shadcn foundation set + 1 test)
**Analogs found:** 24 / 34 (10 net-new shadcn/Tailwind artifacts have no analog — nearest structural precedent noted)

> **Read this with 05-UI-SPEC.md open.** UI-SPEC pins the *target* (Slate tokens, shadcn primitive per component, per-view state matrix). This PATTERNS.md pins the *source* — the exact current-code excerpts each new/modified file should copy structure/behavior from while dropping the plain-`<style>` navy/blue palette.
>
> **Migration invariant (applies to every component/page below):** keep Svelte 5 runes (`$props`/`$state`/`$derived`/`$effect`), keep every existing prop name + behavior, delete the scoped `<style>` block, and replace hard-coded hex (`#1a1a2e`, `#2563eb`, `#64748b`, `#f8fafc`, `#e2e8f0`…) with shadcn token utility classes (`text-foreground`, `text-primary`, `text-muted-foreground`, `bg-muted`, `border-border`…). The categorical stage/layer badge hues are the ONE carve-out (see Shared Pattern: Categorical Badge Palette).

---

## File Classification

### Core scaffolding (EXTEND — do not rewrite)

| File | Role | Data Flow | Closest Analog | Match Quality |
|------|------|-----------|----------------|---------------|
| `web/src/lib/project.svelte.ts` | store (runes module) | transform | itself (extend in place) + `auth.svelte.ts` | self / exact |
| `web/src/routes/+layout.svelte` | layout/provider | request-response | itself (rebuild nav) | self |
| `web/src/routes/+layout.ts` | route config → load | request-response | itself + `+page.svelte` onMount bootstrap | self / role-match |
| `web/src/lib/api/client.ts` | service (transport) | request-response | unchanged (interceptor already correct) | no change |

### New `load()` files (move fetch out of `onMount`/`$effect`)

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `web/src/routes/+page.ts` | load | request-response | `web/src/routes/+page.svelte` `loadDashboard()` (L60-95) | exact (same RPCs) |
| `web/src/routes/graph/+page.ts` | load | request-response | `web/src/routes/graph/+page.svelte` `loadGraph()` (L15-26) | exact |
| `web/src/routes/constitution/+page.ts` | load | request-response | `web/src/routes/constitution/+page.svelte` `load()` (L12-24) | exact |
| `web/src/routes/spec/[...slug]/+page.ts` | load | request-response | `web/src/routes/spec/[...slug]/+page.svelte` `loadSpec()` (L33-69) | exact |
| `web/src/routes/decision/[...slug]/+page.ts` | load | request-response | `web/src/routes/decision/[...slug]/+page.svelte` `loadDecision()` (L26-45) | exact |

### Route pages (consume `data` prop + restyle to shadcn)

| Page | Role | Data Flow | Analog for state matrix | Match Quality |
|------|------|-----------|--------------------------|---------------|
| `web/src/routes/+page.svelte` | page | CRUD (read) | itself (current onMount/$state → `data` prop) | self |
| `web/src/routes/graph/+page.svelte` | page | read | itself | self |
| `web/src/routes/constitution/+page.svelte` | page | read | itself (D-10 polish — Pitfall 4) | self |
| `web/src/routes/spec/[...slug]/+page.svelte` | page | read | itself (has stale-guard `activeSlug` to remove) | self |
| `web/src/routes/decision/[...slug]/+page.svelte` | page | read | itself | self |
| `web/src/routes/keys/+page.svelte` | page | CRUD | restyle-only (data-scope OUT of scope, D-09) | self |

### Existing components → shadcn primitives (15, drop `<style>`)

| Component | Role | shadcn target (UI-SPEC) | Best excerpt analog | Match |
|-----------|------|-------------------------|---------------------|-------|
| `AccordionSection.svelte` | component | `Accordion` + `Badge` | self (L1-24 markup, L26-75 style to drop) | self |
| `TabBar.svelte` | component | `Tabs` | self (L1-20 markup) | self |
| `SpecTable.svelte` | component | `Table` + `Badge` | self (L55-81 markup, L137-151 stage badges) | self |
| `StatsBar.svelte` | component | grid of `Card` | self (L23-30 markup) | self |
| `FunnelBar.svelte` | component | `div` segments + `Badge` | self | self |
| `SearchFilter.svelte` | component | `Input` | self (L11-18) | self |
| `MetadataBar.svelte` | component | `Card` / desc list + `Badge` | self | self |
| `LoginModal.svelte` | component | `Dialog` + `Input` + `Button` | self (L38-64) | self |
| `RevealKeyModal.svelte` | component | `Dialog` + `AlertDialog` (revoke) | self | self |
| `DiffView.svelte` | component | `Card` + `font-mono` | self | self |
| `VersionCompare.svelte` | component | `Select`/`Tabs` + `Card` | self | self |
| `ChangelogTimeline.svelte` | component | timeline + `Separator` + `Badge` | self | self |
| `FindingsSection.svelte` | component | `Accordion`/`Card` + severity `Badge` | self | self |
| `Graph.svelte` | component (dagre) | keep dagre, wrap in `Card`, theme-token node colors | self | self |
| `GraphMini.svelte` | component | compact `Graph` | self | self |

### Net-new shadcn/Tailwind artifacts (NO analog — nearest precedent noted)

| File | Role | Data Flow | Nearest structural precedent | Match |
|------|------|-----------|------------------------------|-------|
| `web/src/app.css` | config | n/a | none (Tailwind v4 entry — use RESEARCH Code Examples verbatim) | none |
| `web/components.json` | config | n/a | none (shadcn init metadata — RESEARCH shape) | none |
| `web/src/lib/utils.ts` (`cn()`) | utility | transform | none (generated by `shadcn-svelte init`) | none |
| `web/src/lib/components/ui/*` | component (vendored) | n/a | none (generated by `shadcn-svelte add`) | none |
| `web/src/lib/components/ModeToggle.svelte` | component | event | small components (`SearchFilter`, `TabBar`) for runes+props shape; RESEARCH Pattern 4 for body | role-match |
| `web/src/lib/project.test.ts` | test | n/a | `web/src/lib/oidc.test.ts` (L1-14) + `keys.test.ts` | role-match |

---

## Pattern Assignments

### `web/src/lib/project.svelte.ts` (store — EXTEND for D-04/D-05/D-06)

**Analog:** itself. The existing "keep if valid, else pick first" seam (L26-31) is exactly where D-04 precedence + D-05 sort slot in. Keep the getter/setter shape (L10-18) unchanged — the setter's localStorage write is what D-03 relies on.

**Current setter + state (L1-18) — keep verbatim:**
```typescript
const STORAGE_KEY = 'specgraph-project';
let current = $state(
  typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) ?? '' : ''
);
let available = $state<string[]>([]);
let loaded = $state(false);

export const project = {
  get current() { return current; },
  set current(v: string) {
    current = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, v);
  },
  get available() { return available; },
  get loaded() { return loaded; },
};
```

**Current `loadProjects()` seam (L20-37) — the ONLY block to change:**
```typescript
export async function loadProjects(): Promise<void> {
  try {
    const resp = await fetch('/api/projects');
    if (resp.ok) {
      const data = await resp.json();
      available = data.projects ?? [];
      // Use saved project if it's still valid, otherwise pick the first
      if (current && available.includes(current)) {
        // keep it
      } else if (available.length > 0) {
        project.current = available[0]; // triggers localStorage save
      }
    }
  } catch { /* Fall back silently */ }
  loaded = true;
}
```

**Change to (per RESEARCH Pattern 3):** sort `available` case-insensitive, then apply D-04 3-tier precedence (valid last-used → `'default'` → alpha-first), else set `current = ''` for the D-07 zero-projects state. Do NOT touch the getter/setter or `STORAGE_KEY`.

---

### `web/src/routes/+page.ts` (NEW load — Dashboard)

**Analog:** `web/src/routes/+page.svelte` `loadDashboard()` (L60-95). Move the exact 5-way `Promise.all` verbatim into a `PageLoad`; return the data instead of assigning `$state`.

**Copy this RPC fan-out (from +page.svelte L62-88):**
```typescript
const [specsRes, readyRes, graphRes, decisionsRes, driftRes] = await Promise.all([
  specClient.listSpecs({}),
  graphClient.getReady({}),
  graphClient.getFullGraph({}),
  decisionClient.listDecisions({}),
  lifecycleClient.checkDrift({ slug: '' }),
]);
// return { specs: specsRes.specs ?? [], ready: readyRes.ready ?? [], nodes: graphRes.nodes ?? [],
//          edges: graphRes.edges ?? [], decisions: decisionsRes.decisions ?? [], reports: driftRes.reports ?? [] }
```

**load() skeleton (RESEARCH Pattern 1 — apply to all 5 load files):**
```typescript
import type { PageLoad } from './$types';
export const load: PageLoad = async ({ depends, parent }) => {
  await parent();          // wait for +layout.ts project default (D-04) — Pitfall 6
  depends('app:project');  // cheap insurance for targeted invalidate('app:project')
  try { /* Promise.all above */ return { /* data */ }; }
  catch (e) { return { loadError: e instanceof Error ? e.message : 'Failed to load' }; }
};
```
> **Error handling (RESEARCH L279):** catch INSIDE `load()` and return a `loadError` field — do NOT let it hit `+error.svelte`. UI-SPEC wants a per-view inline Retry card.

**Page then consumes `data` (replaces L25-35 `$state` block + L95 `onMount`):**
```svelte
<script lang="ts">
  let { data } = $props();                    // reactive; updates on invalidateAll()
  let totalSpecs = $derived(data.specs.length);
  // move existing $derived (recentSpecs L40-42, priorityGroups L44-49,
  // decisionSpecCounts L51-58) to read data.* instead of local $state
</script>
```
The current derived helpers (`recentSpecs`, `priorityGroups`, `decisionSpecCounts`, L40-58) port over unchanged except their source becomes `data.specs`/`data.edges`.

---

### `web/src/routes/graph/+page.ts` (NEW load — Graph)

**Analog:** `graph/+page.svelte` `loadGraph()` (L15-26).
```typescript
const resp = await graphClient.getFullGraph({});
// return { nodes: resp.nodes ?? [], edges: resp.edges ?? [] }
```
Page keeps `filterText = $state('')` (local UI state stays; only the fetch moves). Empty state guard `nodes.length === 0` (current L35) becomes the UI-SPEC "Nothing here yet" card.

---

### `web/src/routes/constitution/+page.ts` (NEW load — D-10 polish)

**Analog:** `constitution/+page.svelte` `load()` (L12-24). **This page currently fetches in a component-body `$effect(() => { load() })` (L26) — the exact anti-pattern (Pitfall 4) causing stale badges after switch.**
```typescript
const resp = await constitutionClient.getConstitution({});
// return { constitution: resp.constitution ?? null, provenance: resp.provenance ?? [] }
```
**Critical for D-10:** the page's `provenance` moves to `data.provenance`; `layerOf()` (L53-58) and the `layerBadge` snippet (L61-66) then read `$derived(data.provenance)` so badges re-derive for free on `invalidateAll()`. The `{:else}` "No constitution found for this project" (L239) becomes the UI-SPEC empty-state card.

---

### `web/src/routes/spec/[...slug]/+page.ts` (NEW load — Spec detail)

**Analog:** `spec/[...slug]/+page.svelte` `loadSpec()` (L33-69). This is the most complex load: a primary `getSpec` + three non-critical secondary fetches (edges, findings, slices) each individually try/caught.

**Delete the manual stale-guard (`activeSlug`, L31/39/44…) — SvelteKit's load re-run + param dependency replaces it.** Pass `params.slug` into load:
```typescript
export const load: PageLoad = async ({ params, parent, depends }) => {
  await parent(); depends('app:project');
  const s = params.slug;
  const specResp = await specClient.getSpec({ slug: s });   // primary — may throw → loadError
  // secondary fetches: keep the per-call try/catch → [] fallback pattern (L42-63)
};
```
Keep the direction-aware `groupedEdges` derivation (L146-168) and all label helpers (L98-138) in the component reading `data.*`. Changelog stays lazy-loaded on demand (L80-92) — it's user-triggered, not part of the switch refetch.

---

### `web/src/routes/decision/[...slug]/+page.ts` (NEW load — Decision detail)

**Analog:** `decision/[...slug]/+page.svelte` `loadDecision()` (L26-45).
```typescript
const resp = await decisionClient.getDecision({ slug: params.slug });   // primary
// linkedSpecs is non-critical: graphClient.listEdges then filter DECIDED_IN + toId===slug (L31-37)
```
Return `{ decision, linkedSpecs }`; keep `statusLabel()` (L16-24) in the component.

---

### `web/src/routes/+layout.ts` (EXTEND — move bootstrap into load)

**Analog:** current `+layout.svelte` `onMount` (L12-26) — that auth+project bootstrap moves HERE (Pitfall 6), so page loads see the resolved project before firing RPCs.

**Keep the SPA flags verbatim (current file, L3-4):**
```typescript
export const ssr = false;
export const prerender = false;
```
**Add `load: LayoutLoad`** that runs `await checkAuth()` then `if (auth.authenticated) await loadProjects()` (mirrors the current onMount body L13/22-24) and returns `{ authenticated, authError }`.

---

### `web/src/routes/+layout.svelte` (REBUILD nav on shadcn)

**Analog:** itself. The D-08 selector branch already exists (L44-52) — re-express with shadcn `Select`/label, and add the switch handler + `<ModeWatcher/>` + breadcrumb.

**Current selector branch to re-implement (L44-52) — preserves D-08 logic:**
```svelte
{#if project.available.length > 1}
  <select bind:value={project.current} class="project-picker">
    {#each project.available as slug}<option value={slug}>{slug}</option>{/each}
  </select>
{:else if project.current}
  <span class="project-name">{project.current}</span>
{/if}
```
**Replace with:** shadcn `Select` (multi) / muted-text label (single) / nothing (zero → main-area empty state), plus the switch handler (RESEARCH Pattern 2):
```svelte
import { invalidateAll } from '$app/navigation';
async function switchProject(slug: string) {
  project.current = slug;   // setter persists (D-03)
  await invalidateAll();    // re-runs +layout.ts + all +page.ts (D-01)
}
```
**Retire** the entire `<style>` block (L65-126) incl. navy `#1a1a2e` nav (L78) and `.project-picker` (L97-104). Nav links (L39-42) keep `class:active` logic but styled with tokens; active link uses `--primary` (UI-SPEC accent rule #2). Add `<ModeWatcher />` + `ModeToggle` in the nav's spacer zone.

---

### `web/src/lib/components/ModeToggle.svelte` (NEW — no direct analog)

**Nearest precedent:** `SearchFilter.svelte`/`TabBar.svelte` for the minimal runes+props component shape. **Body:** copy RESEARCH Pattern 4 verbatim (`toggleMode` from `mode-watcher`, sun/moon from `@lucide/svelte/icons/*`, shadcn `Button variant="outline" size="icon"`, `aria-label="Toggle theme"` per UI-SPEC copy contract).

---

### `web/src/lib/project.test.ts` (NEW test)

**Analog:** `web/src/lib/oidc.test.ts` (L1-14) — the Vitest structure to copy:
```typescript
import { describe, it, expect } from 'vitest';
import { authErrorMessage } from './oidc.svelte';
describe('authErrorMessage', () => {
  it('maps known reasons', () => { expect(authErrorMessage('denied')).toContain('cancelled'); });
});
```
`keys.test.ts` (larger, mocks `fetch`) is the analog for **mocking `/api/projects`**. Cover D-04 precedence (last-used/`default`/alpha-first), D-05 case-insensitive sort determinism, D-06 stale-fallback. Vitest is already a devDependency (`package.json` L21) — no new framework install for `.ts` unit tests.

---

## Component Migration Pattern (applies to all 15 — one worked example)

Every component follows the SAME transform. `AccordionSection` is the reference:

**Analog `AccordionSection.svelte` — KEEP the script/markup shape (L1-24):**
```svelte
<script lang="ts">
  interface Props { title: string; expanded?: boolean; badge?: string; children: import('svelte').Snippet; }
  let { title, expanded = false, badge = '', children }: Props = $props();
  let toggled = $state<boolean | null>(null);
  let open = $derived(toggled !== null ? toggled : expanded);
</script>
```
**DELETE the whole `<style>` block (L26-75)** — every hex there (`#e2e8f0`, `#1a1a2e`, `#2563eb`, `#94a3b8`, `#f1f5f9`) maps to a token. Re-express the markup on shadcn `Accordion.*` primitives with Tailwind utilities; the `badge` count becomes `<Badge>`. **Props + `children` snippet contract stay identical** so call-sites (`constitution/+page.svelte`, `spec/[...slug]/+page.svelte`) need no change.

**Same recipe per component:**
- `TabBar` (L1-20): keep `{ tabs, active, onchange }` props → `Tabs.List/Trigger`; drop `.tab.active` navy/blue (L47-51).
- `SpecTable` (L55-81): keep sort runes (L10-52) → `Table`; **stage badges (L145-151) are categorical — see Shared Pattern below.**
- `StatsBar` (L23-30): keep `cards` `$derived` (L13-20) → grid of `Card`, Display-size numbers; the per-card `color` hexes become the categorical stat accents.
- `SearchFilter` (L11-18): `<input>` → `Input`; drop focus-ring hex (L37-40) → `--ring`.
- `LoginModal` (L38-64): overlay+card → `Dialog`; button navy `#1a1a2e` (L99) → `Button` primary.
- `Graph`/`GraphMini`: **keep dagre layout logic untouched**; only recolor node/edge fills from theme tokens (must stay dark-mode legible) and wrap in `Card`.

---

## Shared Patterns

### Categorical Badge Palette (the ONE color carve-out — D-10, UI-SPEC)
**Sources:** stage badges in `SpecTable.svelte` (L145-151) & `spec/[...slug]/+page.svelte` (L494-503); layer badges in `constitution/+page.svelte` (L388-391); decision-status badges in `decision/[...slug]/+page.svelte` (L169-172).
**Apply to:** SpecTable, spec detail, constitution, decision detail, FindingsSection severity.
**Rule:** these are *categorical data encoding*, NOT the 60/30/10 accent budget. Do NOT map them to `--primary`. Replace the current light-only hex pairs with fixed light/dark Tailwind utility pairs via a `Badge` variant map. UI-SPEC pins the four constitution layer pairs, e.g.:
```
User → bg-blue-100 text-blue-800 / dark:bg-blue-950 dark:text-blue-300
Org → bg-amber-100 ... / Project → bg-green-100 ... / Domain → bg-violet-100 ...
```
Current constitution hexes to REPLACE (constitution L388-391):
```css
.layer-user { background: #dbeafe; color: #1e40af; }   /* → blue token pair */
.layer-org { background: #fef3c7; color: #92400e; }    /* → amber token pair */
.layer-project { background: #dcfce7; color: #166534; }/* → green token pair */
.layer-domain { background: #ede9fe; color: #5b21b6; } /* → violet token pair */
```
Merged/Layer badge → neutral `Badge variant="secondary"`.

### Project header propagation (UNCHANGED — the switch seam)
**Source:** `web/src/lib/api/client.ts` `projectInterceptor` (L15-18):
```typescript
const projectInterceptor: Interceptor = (next) => async (req) => {
  req.header.set('X-Specgraph-Project', project.current || 'default');
  return next(req);
};
```
**Apply to:** nothing to change — this file is already correct. Every `load()` RPC re-issues through this transport, so once `project.current` is set and `invalidateAll()` fires, the new header rides along automatically. The `'default'` fallback (L16) is why D-04's `'default'` tier aligns.

### Auth-error → login modal (UNCHANGED)
**Source:** `client.ts` `authErrorInterceptor` (L50-59) → `onUnauthenticated()`; layout gate (`+layout.svelte` L35-36). Unauthenticated `load()` RPCs still surface the login modal — no new handling needed.

### Per-view state matrix (Loading / Empty / Error)
**Source contract:** UI-SPEC "Per-View State Matrix". Loading = shadcn `Skeleton` (NOT the current `<p class="status">Loading...</p>` seen in every page, e.g. +page.svelte L101, graph L32, constitution L73). Error = inline Retry `Card` with `Button variant="outline"` reading the load's returned `loadError`. **Do NOT bind skeletons to `$navigating`** (Pitfall 3 — `invalidateAll()` doesn't set it); use streamed `{#await}` promises from `load()` or a module-level `switching` flag.

### Server-side sort alternative (D-05 — agent's discretion)
**Source:** `internal/server/api_handler.go` (L20-39). If the planner opts for server-side sort instead of client-side (RESEARCH recommends client-side so one function owns order+fallback), the seam is the `slugs` slice build at L32-37 — sort before `json.NewEncoder(w).Encode`. RESEARCH recommends AGAINST this (D-04 fallback lives client-side anyway).

---

## Build Config Integration

| File | Change | Analog / precedent |
|------|--------|--------------------|
| `web/vite.config.ts` | add `tailwindcss()` plugin BEFORE `sveltekit()`; keep the `/specgraph.v1` proxy (current L6-12) | current file L1-13 + RESEARCH Code Example |
| `web/svelte.config.js` | likely UNCHANGED — `adapter-static` fallback `index.html` (L6-8) stays; confirm compat | current file L1-12 |
| `web/embed.go` | UNCHANGED — `//go:embed all:build` (L12) glob already captures new Tailwind CSS asset filenames; no Go edit | current file L9-13 |
| `web/package.json` | add deps (tailwindcss, @tailwindcss/vite, bits-ui, mode-watcher, @lucide/svelte, tailwind-variants/merge, clsx, tw-animate-css) — mind pnpm `minimumReleaseAge` (RESEARCH Pitfall 7) | current file L12-28 |
| `web/src/app.html` | add blocking FOUC-guard `<script>` in `<head>` reading mode-watcher's localStorage key before `%sveltekit.head%` (L7) | current file L1-12 + RESEARCH Pitfall 5 |
| `web/src/app.css` | NEW — Tailwind v4 entry + `@custom-variant dark` + Slate `:root`/`.dark` OKLCH tokens; import in `+layout.svelte` | none — RESEARCH Code Example verbatim |
| `web/components.json` | NEW — shadcn metadata (`baseColor: slate`, aliases) | none — let `init` generate, then verify |
| `web/src/lib/utils.ts` | NEW — generated `cn()` (clsx + tailwind-merge) | none — generated |

---

## No Analog Found

Files with no in-repo precedent — planner uses RESEARCH.md Code Examples / shadcn-svelte CLI output instead:

| File | Role | Why no analog |
|------|------|---------------|
| `web/src/app.css` | Tailwind v4 entry + Slate tokens | First Tailwind file in repo; use RESEARCH verbatim OKLCH block |
| `web/components.json` | shadcn config | Generated by `shadcn-svelte init` |
| `web/src/lib/utils.ts` (`cn()`) | class-merge util | Generated by init |
| `web/src/lib/components/ui/*` | vendored shadcn primitives | Generated by `shadcn-svelte add`; treat as vendored (don't hand-edit — wrap/compose) |

`ModeToggle.svelte` and `project.test.ts` have partial analogs (component shape / test harness) but net-new bodies — see Pattern Assignments.

---

## Metadata

**Analog search scope:** `web/src/lib/`, `web/src/routes/`, `web/src/lib/components/`, `web/src/lib/api/`, `web/*` build config, `internal/server/api_handler.go`.
**Files scanned:** 22 read in full (all 5 route pages, +layout.svelte/.ts, project/auth/client, 6 representative components, 4 build config, app.html, api_handler.go, 1 test) + directory globs of the remaining 9 components (structure confirmed, same migration recipe applies).
**Pattern extraction date:** 2026-07-10
