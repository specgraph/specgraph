# Phase 5: UI Project Selector & Refinements - Research

**Researched:** 2026-07-10
**Domain:** SvelteKit 2 SPA (Svelte 5 runes, `adapter-static`, Vite 8) + shadcn-svelte / Tailwind v4 migration + reactive project switching via `load()` / `invalidateAll()`
**Confidence:** HIGH (stack + install verified against live shadcn-svelte docs and npm registry on 2026-07-10; existing code read directly)

## Summary

This phase does two things at once: (a) a **full visual migration** of the `web/` SvelteKit app to shadcn-svelte on **Tailwind CSS v4** (the current shadcn-svelte line — there is no more PostCSS/`tailwind.config.js`), and (b) a **data-flow refactor** moving all five project-scoped pages from `onMount`/`$effect` fetches into SvelteKit universal `load()` functions so a project switch can call `invalidateAll()` and re-issue every RPC with the new `X-Specgraph-Project` header. The scaffolding for project state, the `X-Specgraph-Project` interceptor, and `/api/projects` all already exist and should be *extended*, not rewritten. [VERIFIED: files under web/]

The stack is bleeding-edge but internally consistent: Svelte 5.56, SvelteKit 2.63, Vite 8.0, `@sveltejs/adapter-static` 3.0, TypeScript 6, **pnpm** (workspace with a `minimumReleaseAge` supply-chain policy). shadcn-svelte 1.4.1 targets Svelte 5 + Tailwind v4 and is compatible with this stack. The single biggest integration risk is **not shadcn itself but the interactive CLIs** (`sv add tailwindcss`, `shadcn-svelte init`) which block on prompts — automated execution must drive them with flags or fall back to writing the config files directly. [VERIFIED: shadcn-svelte.com/docs, npm registry]

The second-biggest risk is subtle SvelteKit load ordering: `load()` functions run *before* the layout's `onMount` auth/project bootstrap, and `invalidateAll()` does **not** populate the `$navigating` store — so the UI-SPEC's "return to skeleton on switch" contract needs streamed load promises (`{#await}`) or a manual switching flag, not `$navigating`. Both are addressed below.

**Primary recommendation:** Wave the work exactly as CONTEXT suggests (foundation+theming → shared components → selector/switch refactor + states → constitution polish). In Wave 1, drive `sv add tailwindcss` and `shadcn-svelte init` with explicit flags (or write `components.json` + `app.css` + `cn()` by hand), then overwrite the token block with the verified **Slate** OKLCH values from the theming docs. Move project bootstrap into `+layout.ts` `load()`, have each `+page.ts` `await parent()`, and switch via `project.current = x; await invalidateAll()`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Reactive Project Switching**
- **D-01:** On active-project change, views must refresh. Mechanism: migrate each project-scoped page's data-fetch out of `onMount`/component-body `$effect` into SvelteKit `load()` functions, then call `invalidateAll()` on project change so all `load()` re-run and re-fetch with the new `X-Specgraph-Project` header. (Rejected: per-page reactive `$effect` on `project.current`; rejected: redirect-to-dashboard on switch.)
- **D-02:** Because `web/` ships as a static embedded build (`web/embed.go`, `@sveltejs/adapter-static`), these are **client-side universal `load()`** functions (`+page.ts`), not server `load()`. Planner confirms `ssr`/`csr` settings are compatible.
- **D-03:** The dropdown's `project.current` setter already writes localStorage; the switch handler updates state, then triggers `invalidateAll()`.

**Default Project Selection**
- **D-04:** Default precedence when user has not chosen: (1) last-used from localStorage if still valid → (2) a project literally named `default` if present → (3) alphabetically-first available project. Aligns with existing `'default'` fallback in the header.
- **D-05:** Project list sorted alphabetically (case-insensitive) for dropdown order AND the "first available" fallback. Planner picks client-side (`loadProjects()`) or server-side (`/api/projects`).
- **D-06:** A saved localStorage project that no longer exists auto-falls-back per D-04 precedence.

**Empty / Single-Project States**
- **D-07:** Zero projects → explicit empty state in main content ("No projects found — create one via the CLI/authoring flow"); no picker in nav.
- **D-08:** Exactly one project → static project-name label (no dropdown). More than one → dropdown.

**Scope of Refinements**
- **D-09:** In scope for switch-refactor: Dashboard (`/`), Graph (`/graph`), Constitution (`/constitution`), Spec detail (`/spec/[slug]`), Decision detail (`/decision/[slug]`). Keys page (`/keys`) EXCLUDED (user-scoped).
- **D-10:** Constitution view gets extra polish — "No constitution found" state and Merged/Layer badges behave correctly across switches.
- **D-11:** Small active-project indicator on pages (project name in breadcrumb/heading), visible beyond the nav picker.

**shadcn-svelte Migration (full)**
- **D-12:** Full migration to shadcn-svelte this phase — install stack (Tailwind CSS + PostCSS*, `bits-ui`, `tailwind-variants`/`tailwind-merge`, `clsx`, `lucide-svelte`* icons, `components.json`, `$lib/components/ui/`) AND convert every component/page. Components to migrate: `AccordionSection`, `TabBar`, `SpecTable`, `StatsBar`, `FunnelBar`, `SearchFilter`, `MetadataBar`, `LoginModal`, `RevealKeyModal`, `DiffView`, `VersionCompare`, `ChangelogTimeline`, `FindingsSection`, `Graph`, `GraphMini`. (*See "State of the Art" — PostCSS is gone in Tailwind v4, and the icon package is now `@lucide/svelte`. These are the only material drifts between the decision text and the current tooling.)
- **D-13:** Adopt shadcn's default aesthetic — intentional redesign. Replace navy `#1a1a2e` nav, `#2563eb` accents, layer/badge colors with shadcn theme tokens. Planner picks base color (UI-SPEC pins **Slate**).
- **D-14:** Light + dark mode with a toggle (`mode-watcher`), using shadcn's built-in theming.

### the agent's Discretion
- Where to sort the project list (client vs server) — D-05.
- shadcn base color choice and exact component structure conventions — D-12/D-13 (UI-SPEC has already chosen **Slate**, `style: default`, aliases).
- Which components migrate first / wave ordering for shadcn work — planner's call.

### Deferred Ideas (OUT OF SCOPE)
- `/keys` page project-scoping (user-scoped, not project-scoped) — D-09. *Note: `/keys` still migrates to shadcn visually per UI-SPEC; only its project-data-scoping is out of scope.*
- UI project CRUD (create/delete/rename projects from the web UI).
- Backend project-model changes beyond what `/api/projects` already exposes.
- REQUIREMENTS v2 items UI-01 (spec syntax highlighting, `spgr-2pk`) and UI-02 (`SpecView.blockers` reshape, `spgr-to31`) — candidates to fold in later, NOT committed to this phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

Requirements were deferred to discuss; the D-01..D-14 decisions are the effective scope contract. Mapping each to the research support that enables it:

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01 | Switch refreshes views via `load()` + `invalidateAll()` | Architecture Pattern 1 (load-ification), Pattern 2 (switch handler); Pitfall 3 (`$navigating` not set on invalidateAll) |
| D-02 | Client-side universal `load()` for static SPA | `+layout.ts` already sets `ssr=false,prerender=false`; universal `load()` runs client-side — verified compatible. Pattern 1. |
| D-03 | Setter writes localStorage, then `invalidateAll()` | `project.svelte.ts` setter already persists; Pattern 2 |
| D-04 | Default precedence (localStorage → `default` → alpha-first) | `loadProjects()` extension in Pattern 3; existing "keep if valid, else pick[0]" is the seam |
| D-05 | Case-insensitive alpha sort | Pattern 3 (sort in `loadProjects()` recommended — client-side, single source of truth); server-side alt noted |
| D-06 | Stale saved project auto-falls-back | Pattern 3 covers the precedence + fallback in one function |
| D-07 | Zero-projects empty state | UI-SPEC State Matrix; `project.available.length === 0` guard in layout |
| D-08 | Single vs multi selector | Existing layout already branches on `available.length > 1`; re-implement with shadcn `Select` |
| D-09 | Five in-scope views, Keys excluded | Component/Page inventory below |
| D-10 | Constitution polish across switches | Pattern 1 + Pitfall 4 (stale badge data); provenance re-derivation |
| D-11 | Active-project indicator | shadcn `Breadcrumb`; reads `project.current` reactively |
| D-12 | Full shadcn migration | Standard Stack, Install, Component Migration Map |
| D-13 | shadcn default aesthetic (Slate) | Verified Slate OKLCH token block (Code Examples) |
| D-14 | Light/dark toggle via mode-watcher | Pattern 4 (dark mode) + Pitfall 5 (SPA FOUC) |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Project selection UI (dropdown/label/empty) | Browser / Client (Svelte component) | — | Pure presentation over `project.svelte.ts` state |
| Default-project resolution (D-04) | Browser / Client (`project.svelte.ts` + `+layout.ts` load) | — | Reads localStorage + `/api/projects`; localStorage is client-only |
| Project list sort (D-05) | Browser / Client (`loadProjects()`) | API (`/api/projects`) optional | Client sort keeps one deterministic source; server sort is an alt |
| Per-view data fetch (D-01) | Browser / Client (`+page.ts` universal `load()`) | API (ConnectRPC) | SPA has no frontend server; `load()` runs in browser, calls ConnectRPC |
| `X-Specgraph-Project` propagation | Browser / Client (`projectInterceptor`) | API (reads header) | Already implemented; header is the single seam to backend |
| Re-fetch on switch (`invalidateAll`) | Browser / Client (SvelteKit runtime) | — | Client-side navigation/invalidation only |
| Theme (light/dark) | Browser / Client (`mode-watcher`, CSS vars) | — | No SSR; class toggled on `<html>` client-side |
| Project list source of truth | API / Backend (`/api/projects` → `storage.Scoper`) | Database | Already exists; excludes `_server` |
| Static asset serving | CDN / Static (`web/embed.go` via Go binary) | — | Vite build embedded into binary; no Node server at runtime |

**Key correctness note:** every capability here is **client tier except the project list source and RPC data**. There is no SSR/frontend-server tier — do not introduce `+page.server.ts` or server `load()` (would break `adapter-static`). [VERIFIED: web/svelte.config.js, web/src/routes/+layout.ts]

## Standard Stack

### Core
| Library | Version (verified 2026-07-10) | Purpose | Why Standard |
|---------|--------|---------|--------------|
| `shadcn-svelte` (CLI, dev) | 1.4.1 | Scaffolds `components.json`, `cn()`, CSS tokens; `add` copies component source into `$lib/components/ui` | Canonical shadcn port for Svelte; Tailwind-v4 native [VERIFIED: npm] |
| `tailwindcss` | 4.3.2 | Utility CSS engine (v4 — CSS-first, no `tailwind.config.js` needed) | shadcn-svelte 1.x requires Tailwind v4 [VERIFIED: npm + shadcn docs] |
| `@tailwindcss/vite` | 4.3.2 | Tailwind v4 Vite plugin (replaces PostCSS pipeline) | Official v4 integration for Vite/SvelteKit [VERIFIED: npm + shadcn docs] |
| `bits-ui` | 2.18.1 | Svelte 5 headless primitive layer under shadcn-svelte | Every interactive shadcn-svelte component depends on it [VERIFIED: npm] |
| `mode-watcher` | 1.1.0 | Light/dark mode manager; `<ModeWatcher/>`, `toggleMode()`, `setMode()`, `resetMode()` | The shadcn-svelte-recommended dark-mode tool (D-14) [VERIFIED: npm + shadcn docs] |
| `@lucide/svelte` | 1.24.0 | Icon set (`import Sun from "@lucide/svelte/icons/sun"`) | **Current** package the shadcn-svelte docs import from [VERIFIED: npm + shadcn dark-mode docs] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `tailwind-variants` | 3.2.2 | Variant API used by generated `buttonVariants`, `badgeVariants` | Auto-added by `shadcn-svelte init`; used by generated components [VERIFIED: npm] |
| `tailwind-merge` | 3.6.0 | Dedupe/merge conflicting Tailwind classes inside `cn()` | Part of generated `$lib/utils.ts` `cn()` [VERIFIED: npm] |
| `clsx` | 2.1.1 | Conditional className joining inside `cn()` | Part of generated `cn()` [VERIFIED: npm] |
| `tw-animate-css` | 1.4.0 | Tailwind-v4 animation utilities (replaces `tailwindcss-animate`) | Added to `app.css` by init for accordion/dialog transitions [VERIFIED: npm] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `invalidateAll()` on every switch | `depends('app:project')` in each load + `invalidate('app:project')` | Surgical — only project-scoped loads re-run (skips whoami). More wiring but avoids redundant auth re-checks. CONTEXT locked `invalidateAll()`; `depends`/`invalidate` documented as the tradeoff (see Pattern 2). |
| Client-side sort (D-05) | Server-side sort in `/api/projects` | Server sort centralizes ordering but D-04's "alphabetically-first fallback" logic lives client-side anyway → recommend client sort so one function owns both order + fallback. |
| `mode-watcher` | Hand-rolled `document.documentElement.classList` + `matchMedia` | mode-watcher already handles system-pref, persistence, and cross-tab sync; hand-rolling re-invents FOUC handling. Use mode-watcher (D-14). |
| shadcn `Select` for switcher | Native `<select>` (current) or `DropdownMenu` | `Select` matches UI-SPEC and keyboard/focus a11y; keep it. |

**Installation (recommended, non-interactive-safe — pnpm):**
```bash
# From web/ . Step 1: add Tailwind v4 to the existing SvelteKit app.
# `sv add` is interactive; pass the adder inline to avoid prompts:
pnpm dlx sv add tailwindcss --no-install=false
# (If the prompt still blocks in an automated shell, fall back to manual:
#  pnpm add -D tailwindcss @tailwindcss/vite  and wire the plugin by hand — see Pitfall 1.)

# Step 2: init shadcn-svelte non-interactively. NOTE: --base-color flag choices are
# neutral|stone|zinc|mauve|olive|mist|taupe (NO 'slate'); scaffold with a valid value,
# then overwrite the token block with the Slate OKLCH set (Code Examples).
pnpm dlx shadcn-svelte@latest init \
  --base-color neutral \
  --css src/app.css \
  --lib-alias '$lib' \
  --components-alias '$lib/components' \
  --utils-alias '$lib/utils' \
  --hooks-alias '$lib/hooks' \
  --ui-alias '$lib/components/ui'

# Step 3: dark mode + icons
pnpm add mode-watcher @lucide/svelte

# Step 4: add the component set (‑y to skip confirms, ‑o to overwrite)
pnpm dlx shadcn-svelte@latest add -y button card dialog alert-dialog dropdown-menu \
  select tabs table accordion badge input separator skeleton tooltip sonner breadcrumb
```
[CITED: shadcn-svelte.com/docs/installation/vite, /docs/cli, /docs/dark-mode/svelte]

**Version verification:** all versions above confirmed via `npm view <pkg> version` on 2026-07-10 (see Package Legitimacy Audit).

## Package Legitimacy Audit

All packages are well-established shadcn-svelte / Tailwind ecosystem dependencies pulled from official shadcn-svelte docs (authoritative source) and confirmed on the npm registry. None discovered via unverified WebSearch.

| Package | Registry | Latest (2026-07-10) | Last modified | Ecosystem role | Verdict | Disposition |
|---------|----------|---------------------|---------------|----------------|---------|-------------|
| `shadcn-svelte` | npm | 1.4.1 | 2026-07-09 | Official CLI (huntabyte) | OK | Approved |
| `tailwindcss` | npm | 4.3.2 | 2026-07-09 | Core, tens of millions/wk | OK | Approved |
| `@tailwindcss/vite` | npm | 4.3.2 | 2026-07-09 | Official Tailwind Vite plugin | OK | Approved |
| `bits-ui` | npm | 2.18.1 | 2026-05-03 | shadcn-svelte primitive layer | OK | Approved |
| `mode-watcher` | npm | 1.1.0 | 2025-06-28 | shadcn-svelte dark-mode (huntabyte) | OK | Approved |
| `@lucide/svelte` | npm | 1.24.0 | 2026-07-09 | Official lucide Svelte icons | OK | Approved |
| `tailwind-variants` | npm | 3.2.2 | 2025-11-22 | Variant API | OK | Approved |
| `tailwind-merge` | npm | 3.6.0 | 2026-07-06 | class merge | OK | Approved |
| `clsx` | npm | 2.1.1 | 2025-06-27 | className util, ubiquitous | OK | Approved |
| `tw-animate-css` | npm | 1.4.0 | 2026-02-28 | v4 animate utilities | OK | Approved |
| `lucide-svelte` (legacy) | npm | 1.0.1 | 2026-05-15 | **Superseded** by `@lucide/svelte` | SUS (legacy/renamed) | Do NOT use — see State of the Art |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged:** `lucide-svelte` — legacy name still resolves on npm but the current shadcn-svelte docs import from **`@lucide/svelte`**. Use `@lucide/svelte`. The UI-SPEC's mention of `lucide-svelte` is stale (Registry Safety section) — not blocking, just use the new package.

> **pnpm supply-chain gate:** `web/pnpm-workspace.yaml` enforces `minimumReleaseAge` (a package too new is rejected by `--frozen-lockfile`, as already seen with `svelte@5.56.2`). Because several of these packages published within the last few days (tailwindcss 4.3.2, shadcn-svelte 1.4.1, @lucide/svelte 1.24.0 all 2026-07-09), a `pnpm install --frozen-lockfile` in CI **may fail with `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION`**. Planner: pin to slightly older patch versions that clear the age cutoff, OR add targeted `minimumReleaseAgeExclude` entries (same mechanism already used for svelte). This is a real Wave-1 landmine. [VERIFIED: web/pnpm-workspace.yaml]

## Architecture Patterns

### System Architecture Diagram

```
                            ┌─────────────────────────────────────────┐
  User picks project        │  SvelteKit SPA (adapter-static, ssr=off) │
  (shadcn Select) ─────────►│                                          │
                            │  project.svelte.ts  (current/available)  │
                            │      │ setter writes localStorage         │
                            │      ▼                                    │
                            │  invalidateAll()  ──re-runs──►            │
                            │                                          │
  page navigation ─────────►│  +layout.ts load()                       │
                            │    • checkAuth (whoami)                   │
                            │    • loadProjects() → default resolution  │
                            │    • sort (D-05), fallback (D-04/06)      │
                            │      │ (pages await parent())             │
                            │      ▼                                    │
                            │  +page.ts load()  (per in-scope view)     │
                            │    calls ConnectRPC client                │
                            │      │                                    │
                            │      ▼                                    │
                            │  projectInterceptor                       │
                            │    sets X-Specgraph-Project: current      │
                            └──────────────┬───────────────────────────┘
                                           │  HTTP (Connect, POST)
                                           ▼
                            ┌──────────────────────────────────────────┐
                            │  Go server (internal/server)              │
                            │   ConnectRPC handlers ── read header ──►   │
                            │   storage.Scoper.Scoped(project)          │
                            │   /api/projects → ListProjects (excl _server)│
                            └──────────────────────────────────────────┘
                                           │
                                           ▼  Postgres (pgx)
```
Trace the switch use-case: Select change → `project.current = slug` (localStorage write) → `invalidateAll()` → `+layout.ts` + every `+page.ts` `load()` re-run → RPCs re-issued with new header → handlers scope to new project → data returns → pages re-render (skeleton → data/empty/error).

### Recommended Project Structure (post-migration)
```
web/src/
├── app.css                     # NEW: @import "tailwindcss"; @custom-variant dark; Slate token block
├── app.html                    # add FOUC-guard inline script (Pitfall 5)
├── lib/
│   ├── utils.ts                # NEW: generated cn() (clsx + tailwind-merge)
│   ├── components/
│   │   ├── ui/                 # NEW: shadcn-svelte generated primitives (button, card, select, …)
│   │   ├── ModeToggle.svelte   # NEW: theme toggle (D-14)
│   │   └── *.svelte            # existing components re-implemented on ui/ primitives
│   ├── project.svelte.ts       # EXTEND: D-04 precedence + D-05 sort
│   └── api/client.ts           # unchanged (interceptor already correct)
└── routes/
    ├── +layout.ts              # EXTEND: move auth+project bootstrap here (load)
    ├── +layout.svelte          # rebuild nav on shadcn; <ModeWatcher/>, Select, breadcrumb
    ├── +page.ts                # NEW: dashboard load()
    ├── +page.svelte            # consume data prop; skeleton/empty/error
    ├── graph/+page.ts          # NEW
    ├── constitution/+page.ts   # NEW (D-10 polish)
    ├── spec/[...slug]/+page.ts # NEW
    └── decision/[...slug]/+page.ts # NEW
```

### Pattern 1: "Load-ify" a project-scoped page (D-01/D-02)
**What:** Replace `onMount`/`$effect` in-component fetch + `$state` with a universal `+page.ts` `load()` returning data the page consumes via `data` prop. Universal load runs client-side (ssr already off), so ConnectRPC works.
**When to use:** All five in-scope views.
**Example (dashboard — mirrors current `loadDashboard`):**
```typescript
// web/src/routes/+page.ts  — Source: SvelteKit load() convention + existing +page.svelte logic
import type { PageLoad } from './$types';
import { specClient, graphClient, decisionClient, lifecycleClient } from '$lib/api/client';

export const load: PageLoad = async ({ depends, parent }) => {
  await parent();               // ensure +layout.ts resolved project default first (D-04)
  depends('app:project');       // opt into targeted invalidate('app:project') if chosen over invalidateAll
  const [specsRes, readyRes, graphRes, decisionsRes, driftRes] = await Promise.all([
    specClient.listSpecs({}),
    graphClient.getReady({}),
    graphClient.getFullGraph({}),
    decisionClient.listDecisions({}),
    lifecycleClient.checkDrift({ slug: '' }),
  ]);
  return {
    specs: specsRes.specs ?? [],
    ready: readyRes.ready ?? [],
    nodes: graphRes.nodes ?? [],
    edges: graphRes.edges ?? [],
    decisions: decisionsRes.decisions ?? [],
    reports: driftRes.reports ?? [],
  };
};
```
```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  let { data } = $props();               // reactive; updates on invalidateAll()
  let totalSpecs = $derived(data.specs.length);
  // …derive stageCounts, recentSpecs, etc. from data.* with $derived
</script>
```
Errors thrown in `load()` are caught by SvelteKit and rendered via a `+error.svelte` boundary — but the UI-SPEC wants a per-view inline Retry card, so **catch inside `load()` and return an `error` field** (or return a `data.loadError` sentinel) and branch in the component, rather than letting it hit `+error.svelte`.

### Pattern 2: Switch handler (D-03) and invalidate strategy
**What:** The Select `onValueChange` sets `project.current` (persists) then re-runs loads.
```svelte
<!-- in +layout.svelte -->
<script lang="ts">
  import { invalidateAll } from '$app/navigation';
  import { project } from '$lib/project.svelte';
  async function switchProject(slug: string) {
    project.current = slug;          // setter already writes localStorage (D-03)
    await invalidateAll();           // re-runs +layout.ts + all +page.ts load() (D-01)
  }
</script>
```
**Tradeoff (documented per D-01):** `invalidateAll()` re-runs *every* load including `+layout.ts` (which re-checks auth/whoami). To skip that, have loads call `depends('app:project')` and switch via `invalidate('app:project')` instead — only opted-in loads re-run. CONTEXT locked `invalidateAll()`; keep it unless the extra whoami round-trip proves costly. Either way, `depends('app:project')` in each `+page.ts` is cheap insurance.

### Pattern 3: Extend `project.svelte.ts` for D-04/D-05/D-06 (don't rewrite)
**What:** Fold the precedence + sort into the existing `loadProjects()`.
```typescript
// web/src/lib/project.svelte.ts (extended) — Source: existing file + D-04/D-05 rules
export async function loadProjects(): Promise<void> {
  try {
    const resp = await fetch('/api/projects');
    if (resp.ok) {
      const data = await resp.json();
      // D-05: deterministic case-insensitive sort (client-side, single source of truth)
      available = (data.projects ?? []).slice().sort(
        (a: string, b: string) => a.toLowerCase().localeCompare(b.toLowerCase())
      );
      // D-04 precedence: (1) valid last-used → (2) 'default' → (3) alpha-first
      if (current && available.includes(current)) {
        /* keep valid last-used */
      } else if (available.includes('default')) {
        project.current = 'default';
      } else if (available.length > 0) {
        project.current = available[0];    // already alpha-first after sort
      } else {
        project.current = '';              // D-07 zero-projects → no selection
      }
    }
  } catch { /* silent fallback */ }
  loaded = true;
}
```
This preserves the existing "keep if valid, else pick" seam (D-06 falls out for free) and keeps `set current` writing localStorage.

### Pattern 4: Dark mode (D-14) for a static SPA
**What:** shadcn-svelte + `mode-watcher`. Tailwind v4 has no `darkMode: "class"` config option — `init` writes `@custom-variant dark (&:is(.dark *));` into `app.css`; `mode-watcher` toggles the `.dark` class on `<html>`.
```svelte
<!-- +layout.svelte -->
<script lang="ts">
  import '../app.css';
  import { ModeWatcher } from 'mode-watcher';
</script>
<ModeWatcher />
<!-- nav … --> {@render children()}
```
```svelte
<!-- ModeToggle.svelte — Source: shadcn-svelte.com/docs/dark-mode/svelte -->
<script lang="ts">
  import SunIcon from '@lucide/svelte/icons/sun';
  import MoonIcon from '@lucide/svelte/icons/moon';
  import { toggleMode } from 'mode-watcher';
  import { Button } from '$lib/components/ui/button/index.js';
</script>
<Button onclick={toggleMode} variant="outline" size="icon" aria-label="Toggle theme">
  <SunIcon class="h-[1.2rem] w-[1.2rem] scale-100 rotate-0 dark:scale-0 dark:-rotate-90" />
  <MoonIcon class="absolute h-[1.2rem] w-[1.2rem] scale-0 rotate-90 dark:scale-100 dark:rotate-0" />
</Button>
```

### Anti-Patterns to Avoid
- **`+page.server.ts` / server `load()`** — breaks `adapter-static`. Only universal `+page.ts` loads. (D-02)
- **Keeping `onMount` fetch + `$state` alongside `load()`** — double-fetches and defeats `invalidateAll()`. Migrate fully.
- **Relying on `$navigating` to show switch skeletons** — `invalidateAll()` does not populate it (Pitfall 3).
- **`darkMode: "class"` in a JS config** — Tailwind v4 uses the `@custom-variant` directive in CSS; no `tailwind.config.js` is created.
- **Reintroducing per-component scoped `<style>` navy/blue palette** — D-13 retires it; use tokens.
- **Manually editing anything under `$lib/components/ui/` heavily** — treat generated shadcn source as vendored; wrap/compose instead so future `add -o` re-sync is clean.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Re-fetch views on project switch | Custom `$effect` watching `project.current` in every page | `load()` + `invalidateAll()` (D-01) | Centralized, cancels stale requests, one code path; the rejected `$effect` approach scatters logic and races |
| Light/dark theme mgmt | `matchMedia` + `classList` + localStorage glue | `mode-watcher` | System-pref, persistence, cross-tab, and no-flash already solved |
| className conflict resolution | String concat of Tailwind classes | `cn()` (clsx + tailwind-merge) | Correctly de-dupes conflicting utilities; generated for you |
| Accessible Select/Dialog/Dropdown/Tabs | Custom ARIA + keyboard handling | `bits-ui` via shadcn components | Focus traps, roving tabindex, ESC handling are error-prone |
| Loading placeholders | Ad-hoc spinners | shadcn `Skeleton` | Matches UI-SPEC State Matrix; consistent |
| Toast on switch (optional) | Custom toast | `sonner` (shadcn) | UI-SPEC lists it as optional switch feedback |

**Key insight:** Nearly everything this phase needs (switching, theming, a11y primitives) has a first-class solution in the SvelteKit + shadcn-svelte stack. The *only* genuinely bespoke work is composing existing behavior (project precedence, provenance badges, dagre graph) on top of these primitives.

## Runtime State Inventory

> This is a UI migration, not a rename/data migration. There is one client-side stored key.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data (client) | `localStorage['specgraph-project']` (project selection). `mode-watcher` will ADD its own key(s) (default `mode-watcher-mode`). | None for existing key — reused as-is. New theme key is created by mode-watcher; ensure FOUC guard script (Pitfall 5) reads the same key mode-watcher writes. |
| Live service config | None — `/api/projects` reads from Postgres via `storage.Scoper`; no external UI-held config. | None ("None — verified: api_handler.go reads storage only") |
| OS-registered state | None (browser app). | None |
| Secrets/env vars | None renamed. CSRF (`specgraph_csrf`) + `X-Specgraph-Project` unchanged. | None |
| Build artifacts | `web/build/` (embedded via `web/embed.go` `//go:embed all:build`); `web/.svelte-kit/`. Tailwind adds generated CSS into the Vite build. | Rebuild via `task web:build` each wave; `web/embed.go` `all:build` glob already captures new asset filenames — no Go change needed. Verify `build/` non-empty before `task build`. |

**Nothing found** in Live-service, OS-registered, or Secrets categories — verified by reading `api_handler.go`, `client.ts`, and the auth module.

## Common Pitfalls

### Pitfall 1: Interactive CLIs block automated execution
**What goes wrong:** `pnpm dlx sv add tailwindcss` and `shadcn-svelte init`/`add` prompt interactively; an agent-run shell hangs or aborts.
**Why:** These CLIs default to TUI prompts.
**How to avoid:** Use flags — `shadcn-svelte init --base-color … --css … --*-alias …`, `add -y -o`. For `sv add`, if it still blocks, fall back to manual: `pnpm add -D tailwindcss @tailwindcss/vite`, add `tailwindcss()` to `vite.config.ts` plugins, create `src/app.css` with `@import "tailwindcss";`, import it in `+layout.svelte`. Then run `shadcn-svelte init` (which is flag-drivable). 
**Warning signs:** A wave "hangs" with no output; CI timeout on the install step.

### Pitfall 2: `--base-color slate` is not a valid CLI flag value
**What goes wrong:** `shadcn-svelte init --base-color slate` errors — the flag enum is `neutral|stone|zinc|mauve|olive|mist|taupe` (no `slate`/`gray`), even though the interactive prompt and the theming reference still list Slate.
**Why:** The CLI flag validator and the theming docs diverged in shadcn-svelte 1.x.
**How to avoid:** Scaffold with a valid value (e.g. `neutral`), then **overwrite the `:root`/`.dark` token block in `app.css` with the verified Slate OKLCH set** (Code Examples). `components.json`'s `baseColor` is just metadata for future `add`; the real colors live in `app.css`. UI-SPEC's Slate pin is satisfied by the token block, not the flag.
**Warning signs:** CLI exits with "invalid choice"; or a neutral (pure-gray) palette that doesn't match the intended cool-slate look.

### Pitfall 3: `invalidateAll()` does not set `$navigating` — switch skeletons won't show
**What goes wrong:** UI-SPEC requires each view to return to its skeleton state during a switch, but binding skeletons to `$navigating` shows nothing because `invalidateAll()`/`invalidate()` re-run loads *without* a navigation.
**Why:** `$navigating` only populates for actual route navigations, not invalidations.
**How to avoid:** Either (a) return **streamed promises** from `load()` (don't `await` the RPC in load; return the promise) and use `{#await data.specs}` blocks that show `Skeleton` while pending and re-suspend on invalidate; or (b) set a module-level `switching = $state(true)` before `invalidateAll()` and clear it after, gating skeletons on it. Option (a) is the idiomatic SvelteKit answer and gives per-widget skeletons.
**Warning signs:** Stale previous-project data stays visible during the switch (violates UI-SPEC's "no stale data after skeleton clears").

### Pitfall 4: Constitution badges show stale layer/provenance after switch (D-10)
**What goes wrong:** Merged/Layer badge and the four layer badges keep old-project values after switching because they're derived from a `$state` `provenance` populated in a now-bypassed `$effect`.
**Why:** The current constitution page fetches in a component-body `$effect(() => { load() })` and stores `provenance` in `$state`.
**How to avoid:** Move the fetch to `constitution/+page.ts` `load()`, return `{ constitution, provenance }`, and derive badges with `$derived(data.provenance)`. On `invalidateAll()` the load re-runs and `data` (thus every `$derived`) updates — badges re-render for free. Preserve the categorical badge palette per UI-SPEC (fixed blue/amber/green/violet, not theme accent).
**Warning signs:** After switching to a project with no constitution, old badges/sections linger instead of the "No constitution for this project" empty state.

### Pitfall 5: FOUC / theme flash in a static SPA (no SSR)
**What goes wrong:** Because `ssr=false` + `adapter-static` serve an empty `index.html` shell, the `.dark` class isn't applied until JS hydrates → a light flash before dark resolves.
**Why:** `mode-watcher`'s no-flash relies on head injection that, in pure SPA, runs only after hydration; `%sveltekit.head%` is empty at build time (prerender off).
**How to avoid:** Add a tiny **blocking inline script** in `web/src/app.html` `<head>` that reads the same localStorage key mode-watcher uses and sets `document.documentElement.classList` before first paint:
```html
<script>
  try {
    var m = localStorage.getItem('mode-watcher-mode') || 'system';
    var d = m === 'dark' || (m === 'system' &&
      matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.classList.toggle('dark', d);
  } catch (e) {}
</script>
```
Verify the exact localStorage key mode-watcher@1.1.0 writes (default `mode-watcher-mode`) and match it. [ASSUMED: default key name — confirm against installed mode-watcher before relying on it]
**Warning signs:** White flash on reload in dark mode.

### Pitfall 6: `load()` runs before layout `onMount` auth/project bootstrap
**What goes wrong:** Universal `load()` fires RPCs before the current `+layout.svelte` `onMount` runs `checkAuth()` + `loadProjects()`, so the first RPC may carry `X-Specgraph-Project: default` (empty `project.current` → interceptor fallback) instead of the resolved default.
**Why:** SSvelteKit runs load functions before components mount; auth/project bootstrap currently lives in `onMount`.
**How to avoid:** Move `checkAuth()` + `loadProjects()` (default resolution) into `+layout.ts` `load()`, and have each `+page.ts` `await parent()` so the project default is resolved before page RPCs issue. Keep the auth-gate UI in `+layout.svelte` but source its state from layout `load()` data. (The session cookie is already present, so unauthenticated RPCs are still caught by `authErrorInterceptor` → login modal.)
**Warning signs:** First paint after login shows the wrong (or `default`) project's data until a manual refresh.

### Pitfall 7: pnpm `minimumReleaseAge` rejects just-published deps
Covered in Package Legitimacy Audit — several deps published 2026-07-09; `--frozen-lockfile` may fail. Pin older patches or add `minimumReleaseAgeExclude` entries.

## Code Examples

### app.css — Tailwind v4 entry + dark variant + Slate tokens (verified)
```css
/* web/src/app.css — Source: shadcn-svelte.com/docs/theming (Slate) + Tailwind v4 conventions */
@import "tailwindcss";
@import "tw-animate-css";

@custom-variant dark (&:is(.dark *));

:root {
  --radius: 0.625rem;
  --background: oklch(1 0 0);
  --foreground: oklch(0.129 0.042 264.695);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.129 0.042 264.695);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.129 0.042 264.695);
  --primary: oklch(0.208 0.042 265.755);
  --primary-foreground: oklch(0.984 0.003 247.858);
  --secondary: oklch(0.968 0.007 247.896);
  --secondary-foreground: oklch(0.208 0.042 265.755);
  --muted: oklch(0.968 0.007 247.896);
  --muted-foreground: oklch(0.554 0.046 257.417);
  --accent: oklch(0.968 0.007 247.896);
  --accent-foreground: oklch(0.208 0.042 265.755);
  --destructive: oklch(0.577 0.245 27.325);
  --border: oklch(0.929 0.013 255.508);
  --input: oklch(0.929 0.013 255.508);
  --ring: oklch(0.704 0.04 256.788);
}
.dark {
  --background: oklch(0.129 0.042 264.695);
  --foreground: oklch(0.984 0.003 247.858);
  --card: oklch(0.208 0.042 265.755);
  --card-foreground: oklch(0.984 0.003 247.858);
  --popover: oklch(0.208 0.042 265.755);
  --popover-foreground: oklch(0.984 0.003 247.858);
  --primary: oklch(0.929 0.013 255.508);
  --primary-foreground: oklch(0.208 0.042 265.755);
  --secondary: oklch(0.279 0.041 260.031);
  --secondary-foreground: oklch(0.984 0.003 247.858);
  --muted: oklch(0.279 0.041 260.031);
  --muted-foreground: oklch(0.704 0.04 256.788);
  --accent: oklch(0.279 0.041 260.031);
  --accent-foreground: oklch(0.984 0.003 247.858);
  --destructive: oklch(0.704 0.191 22.216);
  --border: oklch(1 0 0 / 10%);
  --input: oklch(1 0 0 / 15%);
  --ring: oklch(0.551 0.027 264.364);
}
/* @theme inline { --color-background: var(--background); … }  ← init generates the full map */
```
> The `@theme inline { … }` block that maps `--color-*` → the vars above is generated by `shadcn-svelte init`; keep it. The block above shows the Slate values to paste over whatever base color you scaffolded with. [CITED: shadcn-svelte.com/docs/theming]

### vite.config.ts — add Tailwind v4 plugin (keep existing proxy)
```typescript
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],   // tailwindcss() before sveltekit()
  server: { proxy: { '/specgraph.v1': { target: 'http://localhost:8080', changeOrigin: true } } },
});
```
[CITED: shadcn-svelte.com/docs/installation/vite]

### components.json (target shape per UI-SPEC)
```json
{
  "$schema": "https://shadcn-svelte.com/schema.json",
  "style": "default",
  "tailwind": { "css": "src/app.css", "baseColor": "slate" },
  "aliases": {
    "components": "$lib/components",
    "utils": "$lib/utils",
    "ui": "$lib/components/ui",
    "hooks": "$lib/hooks",
    "lib": "$lib"
  },
  "typescript": true,
  "registry": "https://shadcn-svelte.com/registry"
}
```
> `baseColor: "slate"` here is metadata; ensure the Slate token block above is what actually lands in `app.css`. Confirm exact `components.json` schema against `shadcn-svelte.com/docs/components-json` when writing. [ASSUMED: exact field names — verify against components-json docs during Wave 1]

## Component Inventory & Migration Map (D-12)

Verified list of files to migrate (all confirmed present under `web/src/lib/components/`):
`AccordionSection, TabBar, SpecTable, StatsBar, FunnelBar, SearchFilter, MetadataBar, LoginModal, RevealKeyModal, DiffView, VersionCompare, ChangelogTimeline, FindingsSection, Graph, GraphMini`. Mapping to shadcn primitives is fully specified in **05-UI-SPEC.md → "Component Inventory & Migration Map"** — follow it verbatim. Pages to load-ify + restyle: `/`, `/graph`, `/constitution`, `/spec/[...slug]`, `/decision/[...slug]` (+ `/keys` restyle-only, no data-scope change). [VERIFIED: glob of web/src/lib/components + web/src/routes]

## State of the Art

| Old Approach (in decision text / UI-SPEC) | Current Approach (2026-07-10) | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Tailwind CSS + **PostCSS** + `tailwind.config.js` (D-12) | Tailwind **v4**: `@tailwindcss/vite` plugin, CSS-first `@import "tailwindcss"`, no PostCSS config, no JS config | Tailwind v4 (2025) | No `postcss.config`/`tailwind.config.js` to create; theming is CSS-var only |
| `darkMode: "class"` in config (UI-SPEC) | `@custom-variant dark (&:is(.dark *));` directive in `app.css` | Tailwind v4 | mode-watcher's `.dark` on `<html>` still works; the config key is gone |
| `lucide-svelte` (D-12, UI-SPEC) | **`@lucide/svelte`** (`@lucide/svelte/icons/<name>`) | lucide Svelte repackage (2025/26) | Install `@lucide/svelte`; legacy `lucide-svelte` still resolves but is superseded |
| `tailwindcss-animate` | `tw-animate-css` | Tailwind v4 | init adds `tw-animate-css`; don't add the old one |
| Component fetch in `onMount`/`$effect` + `$state` | Universal `+page.ts` `load()` + `data` prop | This phase (D-01) | Enables `invalidateAll()` re-fetch |

**Deprecated/outdated:**
- Any `tailwind.config.{js,ts}` or `postcss.config.js` guidance — not used in Tailwind v4 for this setup.
- `lucide-svelte` import path — use `@lucide/svelte/icons/<name>`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `mode-watcher@1.1.0` default localStorage key is `mode-watcher-mode` | Pitfall 5, Runtime State | FOUC guard reads wrong key → flash persists. Verify against installed package before shipping the inline script. |
| A2 | `components.json` field shape (`tailwind.css`, `tailwind.baseColor`, `aliases.*`, `registry`) as shown | Code Examples | Init may write a slightly different schema; prefer letting `init` generate it, then edit. Verify at /docs/components-json. |
| A3 | `sv add tailwindcss` can be driven non-interactively / falls back cleanly to manual `pnpm add -D tailwindcss @tailwindcss/vite` | Install, Pitfall 1 | If the adder can't be scripted, use the manual path (well-documented). Low risk. |
| A4 | `--base-color` enum excludes `slate` (choices: neutral/stone/zinc/mauve/olive/mist/taupe) as printed in the live CLI docs | Pitfall 2 | If a newer CLI restores `slate`, the workaround is still valid (harmless). |
| A5 | Streamed-promise `{#await}` re-suspends on `invalidateAll()` to show switch skeletons | Pitfall 3 | If not, use the manual `switching` flag fallback (also given). Low risk — this is documented SvelteKit streaming behavior. |
| A6 | Just-published deps may trip pnpm `minimumReleaseAge` | Package Audit, Pitfall 7 | If the policy window is short enough that all clear, no action needed — but the mechanism is confirmed in the repo. |

## Open Questions (RESOLVED)

1. **`invalidateAll()` vs targeted `invalidate('app:project')`**
   - Known: CONTEXT locked `invalidateAll()`; `depends('app:project')` enables the surgical alt.
   - RESOLVED: ship `invalidateAll()` per D-01 but add `depends('app:project')` to loads so switching to `invalidate('app:project')` is a one-line change if the `+layout.ts` (auth/whoami) re-run overhead proves unacceptable. Encoded in plans 05-03/05-10/05-11/05-13.

2. **Skeleton-on-switch mechanism (streamed load vs switching flag)**
   - Known: `$navigating` won't fire on invalidate.
   - RESOLVED: use streamed promises + `{#await}` for the UI-SPEC's per-view skeleton fidelity; fall back to a page-level `switching` flag if streaming complicates the graph/dagre view. Wave-3 page plans implement this.

3. **Base color reconciliation (Slate token block vs CLI enum)**
   - Known: Slate tokens verified and available; CLI flag doesn't accept `slate`.
   - RESOLVED: scaffold with `--base-color neutral`, then paste the verified Slate OKLCH block (Pitfall 2); `components.json.baseColor` is cosmetic metadata only. Encoded in plan 05-01.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| pnpm | web build / installs | ✓ (lockfile + workspace present) | per repo | — |
| Node (Vite 8 needs Node ≥ 20.19/22.12) | Vite build | ✓ (repo already builds `web/`) | ≥20.19 assumed | verify `node -v` in Wave 0 |
| Go toolchain | `task build`, embed | ✓ | repo-pinned | — |
| Network to npm + shadcn-svelte registry | init/add fetches component source | ✓ (assumed CI/dev online) | — | `shadcn-svelte add` needs registry access; vendor components if offline |
| Docker | NOT needed for this phase | n/a (UI only; no Postgres integration) | — | `task check` excludes Postgres tests |

**Missing dependencies with no fallback:** none identified — the phase is buildable with the existing toolchain.
**Missing dependencies with fallback:** offline install of shadcn components (fallback: manual vendoring) — low likelihood.

> Note: `task web:build` runs `pnpm install` then `pnpm build`; `task build` depends on `web:build` and embeds `web/build/`. Each wave MUST end green through `task web:build` → `task build` → `task check` (fmt/license/lint/build/unit; no Docker). License headers required on any new `.go`; new TS/Svelte files don't need SPDX but check repo conventions.

## Validation Architecture

> `nyquist_validation` key is absent from `.planning/config.json` → treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 3.0 (web); Go `testing` (backend `/api/projects`) |
| Config file | none dedicated for web (`vitest run` via `package.json` `test` script; uses Vite config) |
| Quick run command | `pnpm -C web test` (Vitest) |
| Full suite command | `task check` (Go fmt/lint/build/unit) + `pnpm -C web test` |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| D-04/D-05/D-06 | default precedence + sort + stale fallback in `loadProjects()` | unit | `pnpm -C web test` (add `project.test.ts`) | ❌ Wave 0 — new test |
| D-05 | case-insensitive sort deterministic | unit | same, mock `/api/projects` | ❌ Wave 0 |
| D-01/D-02 | `load()` returns data; switch re-fetches | integration (component) | Vitest + `@testing-library/svelte` (not yet a dep) | ❌ Wave 0 — needs test-lib decision |
| D-07/D-08 | zero/one/many selector states | component | same | ❌ Wave 0 |
| D-05 (server alt) | `/api/projects` excludes `_server` | unit (Go) | `task test` (`internal/server`) | ⚠️ verify existing coverage |
| D-14 | theme toggle flips `.dark` | manual / component | manual UAT (visual) | manual-only justified (visual) |
| build | app still builds static + embeds | smoke | `task web:build && task build` | ✅ existing pipeline |

### Sampling Rate
- **Per task commit:** `pnpm -C web test` (+ `pnpm -C web build` for migrated pages).
- **Per wave merge:** `task web:build && task build && task check`.
- **Phase gate:** full suite green before `/gsd-verify-work`; visual dark-mode + switch UAT.

### Wave 0 Gaps
- [ ] `web/src/lib/project.test.ts` — covers D-04/D-05/D-06 (mock `fetch('/api/projects')`, assert precedence + sort).
- [ ] Decide/introduce a Svelte component test lib (`@testing-library/svelte` + `@vitest/browser` or jsdom) for D-01/D-07/D-08 — currently only plain `.ts` unit tests exist (`oidc.test.ts`, `keys.test.ts`). If component testing is out of appetite, mark D-07/D-08/D-14 manual-UAT explicitly.
- [ ] Confirm existing Go coverage for `/api/projects` (`_server` exclusion); add if missing.
- [ ] Verify Node version satisfies Vite 8 (`node -v` ≥ 20.19 or ≥ 22.12).

*(No framework install needed for `.ts` unit tests — Vitest already present.)*

## Security Domain

> `security_enforcement` not set to false → included. This is a client-side UI phase with no new auth/crypto surface.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing session cookie + whoami; not modified |
| V3 Session Management | no (unchanged) | Existing `specgraph_csrf` double-submit + session cookie |
| V4 Access Control | partial | `X-Specgraph-Project` header selects tenant scope; server enforces via `storage.Scoper` — **client cannot bypass server scoping** (header is advisory; server authorizes). No change. |
| V5 Input Validation | yes (light) | Project slug comes from `/api/projects` (server-provided allow-list), not free user input → low injection surface. Don't echo unsanitized slugs into `innerHTML`; Svelte auto-escapes text bindings. |
| V6 Cryptography | no | none introduced |

### Known Threat Patterns for SvelteKit SPA + shadcn
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via project name / spec content rendered raw | Tampering / Info-disclosure | Use Svelte `{text}` bindings (auto-escaped); avoid `{@html …}` for server data. DiffView/markdown content: ensure no `{@html}` of untrusted content (UI-01 syntax highlighting is deferred — keep it out). |
| Supply-chain (malicious/typosquat dep, slopsquat) | Tampering | Package Legitimacy Audit above; all deps from official shadcn-svelte docs; pnpm `minimumReleaseAge` policy is an existing guard — keep it, add targeted excludes only. Use `@lucide/svelte`, not lookalikes. |
| CSRF on cookie-authed mutations | Spoofing | Unchanged — `csrfInterceptor` already echoes `specgraph_csrf`. This phase adds no new mutations (keys revoke path already covered). |
| Tenant data leakage across switch (stale render) | Info-disclosure | Pitfall 3/4 — ensure no previous-project data remains after switch; skeleton-clear must show new-project data only. This is a UX *and* a mild info-hygiene control. |

## Sources

### Primary (HIGH confidence)
- shadcn-svelte.com/docs/installation/sveltekit & /installation/vite — Tailwind v4 add flow, path aliases, init CLI [fetched 2026-07-10]
- shadcn-svelte.com/docs/dark-mode/svelte — `mode-watcher`, `@lucide/svelte`, ModeToggle [fetched 2026-07-10]
- shadcn-svelte.com/docs/cli — `init`/`add` flags (incl. `--base-color` enum) [fetched 2026-07-10]
- shadcn-svelte.com/docs/theming — full Slate/Neutral/Zinc OKLCH token blocks, `@theme inline`, adding colors [fetched 2026-07-10]
- npm registry (`npm view … version`) — all package versions + last-modified dates [2026-07-10]
- Repo source: `web/src/lib/project.svelte.ts`, `+layout.svelte`, `+layout.ts`, `lib/api/client.ts`, `auth.svelte.ts`, the 5 route pages, `svelte.config.js`, `vite.config.ts`, `embed.go`, `package.json`, `pnpm-workspace.yaml`, `tsconfig.json`, `internal/server/api_handler.go`, `Taskfile.yml` (web targets) [read 2026-07-10]

### Secondary (MEDIUM confidence)
- SvelteKit `load()` / `invalidateAll()` / `depends()` / `await parent()` / streamed-promise semantics — training knowledge cross-checked against the SPA config in-repo (`ssr=false`). Recommend a quick confirm against svelte.dev/docs during Wave 1 for the `$navigating`-not-set-on-invalidate detail.

### Tertiary (LOW confidence)
- `mode-watcher` exact localStorage key name (A1) — verify against installed package.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions + install verified against official docs and npm same-day.
- Architecture (load-ify + invalidate): HIGH for the pattern; MEDIUM on the exact skeleton-on-switch mechanism (two documented options given).
- Theming/dark mode: HIGH — token block and mode-watcher usage from official docs; SPA FOUC guard is the one ASSUMED detail (key name).
- Pitfalls: HIGH — grounded in the actual repo (interactive CLI, pnpm age policy, load-before-onMount ordering) and live CLI docs (base-color enum).

**Research date:** 2026-07-10
**Valid until:** ~2026-08-09 (30 days) — but shadcn-svelte/Tailwind/lucide are fast-moving (several deps published the day before research); re-verify versions and the `--base-color` enum at Wave 1 start (treat as ~7-day freshness for exact versions).
