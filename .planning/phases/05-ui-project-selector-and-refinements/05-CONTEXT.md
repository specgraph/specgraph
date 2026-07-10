# Phase 5: UI Project Selector & Refinements - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

The SpecGraph web UI (SvelteKit, `web/`) lets users pick the active project with a
sensible default, and every project-scoped view reflects the selected project
correctly — including when the user switches projects mid-session. This phase also
performs a **full migration of the web UI to shadcn-svelte** (Tailwind + shadcn
component system) as the design-system foundation for these refinements.

**Reframing note:** The project-selector *scaffolding already exists* — reactive
project state (`web/src/lib/project.svelte.ts`), a nav dropdown
(`web/src/routes/+layout.svelte`), localStorage persistence, and the
`X-Specgraph-Project` request header (`web/src/lib/api/client.ts`). The real work is
(a) making project *switching* actually refresh views, (b) hardening default/empty
states, and (c) rebuilding the UI on shadcn.

**In scope:** project switching that reloads data; default-selection rules; empty /
single-project states; active-project indicator on pages; full shadcn-svelte
migration (all components + pages); light + dark mode.

**Out of scope:** the `/keys` page project-scoping (it is user-scoped, not
project-scoped); any new project CRUD (create/delete projects) from the UI; backend
project-model changes beyond what `/api/projects` already exposes.
</domain>

<decisions>
## Implementation Decisions

### Reactive Project Switching
- **D-01:** When the active project changes, views must refresh to show the new
  project's data. Chosen mechanism: **migrate each project-scoped page's data-fetch
  out of `onMount`/component-body `$effect` into SvelteKit `load()` functions, then
  call `invalidateAll()` on project change** so all `load()` functions re-run and
  re-fetch with the new `X-Specgraph-Project` header. (Rejected: per-page reactive
  `$effect` tracking `project.current`; rejected: redirect-to-dashboard on switch.)
- **D-02:** Because `web/` ships as a static embedded build (`web/embed.go`,
  `@sveltejs/adapter-static`), these are **client-side universal `load()`**
  functions (`+page.ts`), not server `load()`. Planner should confirm `ssr`/`csr`
  settings are compatible.
- **D-03:** The dropdown's `project.current` setter already writes localStorage;
  the switch handler should update state, then trigger `invalidateAll()`.

### Default Project Selection
- **D-04:** Default-selection precedence when the user has not explicitly chosen:
  **(1) last-used from localStorage if still valid → (2) a project literally named
  `default` if present → (3) the alphabetically-first available project.** This
  aligns with the existing `'default'` fallback in the `X-Specgraph-Project` header.
- **D-05:** The project list is **sorted alphabetically (case-insensitive)** for both
  the dropdown order and the "first available" fallback, so ordering is deterministic
  regardless of API/DB return order. Planner picks whether to sort client-side
  (`loadProjects()`) or server-side (`/api/projects`).
- **D-06:** A saved localStorage project that no longer exists auto-falls-back per the
  D-04 precedence (existing `loadProjects()` already does the "keep if valid, else
  pick" check).

### Empty / Single-Project States
- **D-07:** **Zero projects** → show an explicit empty state in the main content area
  (e.g. "No projects found — create one via the CLI/authoring flow") instead of
  rendering a broken/empty dashboard. No picker in the nav.
- **D-08:** **Exactly one project** → keep the static project-name label (no dropdown).
  **More than one** → dropdown. (Preserves current behavior.)

### Scope of Refinements
- **D-09:** Project-scoped views IN scope for the switch-refactor: **Dashboard (`/`),
  Graph (`/graph`), Constitution (`/constitution`), Spec detail (`/spec/[slug]`),
  Decision detail (`/decision/[slug]`)**. The **Keys page (`/keys`) is excluded** —
  it is user-scoped, not project-scoped.
- **D-10:** **Constitution view gets extra polish** — its "No constitution found for
  this project" state and Merged/Layer badges should behave correctly across project
  switches.
- **D-11:** Add a small **active-project indicator on pages** (e.g. project name in
  the breadcrumb/heading) so the current project is always visible, not only in the
  nav picker.

### shadcn-svelte Migration (full)
- **D-12:** **Full migration of the web UI to shadcn-svelte in this phase** — install
  the stack (Tailwind CSS + PostCSS, `bits-ui`, `tailwind-variants`/`tailwind-merge`,
  `clsx`, `lucide-svelte` icons, `components.json`, `$lib/components/ui/`) **and**
  convert every existing component and page to shadcn primitives. Existing components
  to migrate include: `AccordionSection`, `TabBar`, `SpecTable`, `StatsBar`,
  `FunnelBar`, `SearchFilter`, `MetadataBar`, `LoginModal`, `RevealKeyModal`,
  `DiffView`, `VersionCompare`, `ChangelogTimeline`, `FindingsSection`, `Graph`,
  `GraphMini`. (User chose the full in-phase migration over a foundation-only /
  incremental split — accept the larger scope.)
- **D-13:** **Adopt shadcn's default aesthetic** — this is an intentional visible
  redesign, not just a structural swap. The current plain-CSS palette (navy
  `#1a1a2e` nav, blue `#2563eb` accents, layer/badge colors) is replaced by shadcn's
  default theme tokens. Planner: pick a reasonable shadcn base color.
- **D-14:** **Light + dark mode with a toggle** (e.g. `mode-watcher`), using shadcn's
  built-in theming.

### the agent's Discretion
- Where to sort the project list (client vs server) — D-05.
- shadcn base color choice and exact component structure conventions — D-12/D-13.
- Which components migrate first / wave ordering for the shadcn work — planner's call.

**Scope caution for planner:** D-12 (full shadcn migration) + D-14 (dark mode) make
this a large phase that dwarfs the original selector work. Strongly consider splitting
into waves: (1) shadcn foundation + theming, (2) migrate shared components, (3)
selector/switch refactor + empty states + active-project indicator, (4) constitution
polish. Each wave should leave the app buildable (`task build`, `web` vite build).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing project-selector scaffolding (the code being refined)
- `web/src/lib/project.svelte.ts` — reactive project state, localStorage
  persistence, `/api/projects` fetch, current default-selection logic.
- `web/src/routes/+layout.svelte` — nav dropdown / single-project label, auth gate,
  `loadProjects()` call, "Loading projects..." transition.
- `web/src/lib/api/client.ts` — `projectInterceptor` sets `X-Specgraph-Project`
  header (fallback `'default'`); ConnectRPC transport + all typed clients.
- `internal/server/api_handler.go` — `/api/projects` HTTP endpoint (returns project
  slugs, excludes `_server`) via `storage.Scoper` / `ListProjects`.

### Project-scoped views to migrate (load() + shadcn)
- `web/src/routes/+page.svelte` — Dashboard (`onMount(loadDashboard)`).
- `web/src/routes/graph/+page.svelte` — Graph.
- `web/src/routes/constitution/+page.svelte` — Constitution (extra polish; empty
  state at "No constitution found for this project").
- `web/src/routes/spec/[...slug]/+page.svelte` — Spec detail.
- `web/src/routes/decision/[...slug]/+page.svelte` — Decision detail.
- `web/src/routes/keys/+page.svelte` — Keys (EXCLUDED from project-scope refactor).

### Design system
- https://shadcn-svelte.com — canonical shadcn-svelte docs (install, `components.json`,
  Tailwind setup, theming/dark mode, component list). Researcher: confirm current
  install steps for Svelte 5 + SvelteKit `adapter-static` + Vite 8.
- `web/svelte.config.js`, `web/vite.config.ts`, `web/embed.go` — build config the
  Tailwind/shadcn setup must integrate with (static adapter → Go embed).

### Project conventions
- `AGENTS.md` (repo root) — web shim / harness notes, `task build` behavior.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `project.svelte.ts` already provides reactive `current`/`available`/`loaded`,
  localStorage persistence, and `loadProjects()` — extend it rather than rewrite;
  add the D-04 precedence + D-05 sort here.
- `projectInterceptor` in `client.ts` already threads the project into every RPC via
  `X-Specgraph-Project` — switching just needs the views to re-fetch (D-01).
- `AccordionSection`, `TabBar`, `SpecTable`, `StatsBar` etc. are the components that
  will be re-implemented on shadcn (D-12).

### Established Patterns
- Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) throughout — shadcn-svelte
  is Svelte-5 compatible; keep runes.
- Pages currently fetch in `onMount` (dashboard) or component-body `$effect`
  (constitution) — the anti-pattern D-01 fixes; both must move to `load()`.
- Plain scoped `<style>` blocks per component today (no Tailwind) — being replaced by
  Tailwind utility classes + shadcn tokens (D-13).
- Static build embedded into the Go binary (`web/embed.go` + `adapter-static`) — the
  Tailwind/shadcn build must still produce a static bundle.

### Integration Points
- `/api/projects` (Go, `internal/server/api_handler.go`) feeds the dropdown — a
  server-side sort (D-05) would live here.
- `X-Specgraph-Project` header is the single point where project selection reaches the
  backend; the switch mechanism must ensure re-issued requests carry the new value.
</code_context>

<specifics>
## Specific Ideas

- User explicitly wants shadcn adopted "fully" — the complete migration, default
  shadcn look, in this phase (not deferred, not foundation-only).
- Dark mode with a toggle is explicitly wanted.
- Active-project indicator visible on pages (not just the nav picker).
</specifics>

<deferred>
## Deferred Ideas

- `/keys` page project-scoping — intentionally excluded (user-scoped, not
  project-scoped) — D-09.
- UI project CRUD (create/delete/rename projects from the web UI) — not raised as in
  scope; belongs in a future phase if desired.
- Related backlog UI items already tracked in REQUIREMENTS.md v2: **UI-01** (spec
  syntax highlighting / code-block rendering, `spgr-2pk`) and **UI-02** (reconsider
  `SpecView.blockers` shape, `spgr-to31`) — candidates to fold into the shadcn
  redesign later, but not committed to this phase.

</deferred>

---

*Phase: 5-ui-project-selector-and-refinements*
*Context gathered: 2026-07-10*
