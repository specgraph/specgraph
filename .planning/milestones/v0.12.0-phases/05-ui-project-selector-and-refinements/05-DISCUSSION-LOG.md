# Phase 5: UI Project Selector & Refinements - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 5-ui-project-selector-and-refinements
**Areas discussed:** Reactive project switching, Default project selection, Empty / single-project states, Scope of refinements

---

## Reactive Project Switching

| Option | Description | Selected |
|--------|-------------|----------|
| Reactive $effect per page | Wrap each page's data-load in a $effect that reads project.current | |
| invalidateAll() / reload on switch | Blow away and re-fetch everything on dropdown change | ✓ |
| Redirect to dashboard on switch | goto('/') and load fresh there | |

**User's choice:** invalidateAll() / reload on switch.

Follow-up — implementation given onMount fetches today:

| Option | Description | Selected |
|--------|-------------|----------|
| Hard location.reload() | Simplest; full browser reload | |
| Move fetches to load() + invalidateAll() | Migrate data-loading into SvelteKit load() functions | ✓ |
| You decide | Planner picks cleanest | |

**User's choice:** Move fetches to SvelteKit `load()` + `invalidateAll()`.
**Notes:** Repo ships a static embedded build (`web/embed.go` + adapter-static), so these are client-side universal `load()` (`+page.ts`).

---

## Default Project Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Last-used, then 'default' slug, then first | localStorage → 'default' named project → first available | ✓ |
| Last-used, then alphabetical-first | localStorage → alphabetical, no special 'default' | |
| Keep current (last-used, else API-first) | Formalize today's behavior | |

**User's choice:** Last-used → 'default' slug → first available.

Follow-up — ordering:

| Option | Description | Selected |
|--------|-------------|----------|
| Sort alphabetically | Deterministic dropdown + fallback | ✓ |
| Keep API order | Whatever ListProjects returns | |
| You decide | Planner chooses where to sort | |

**User's choice:** Sort alphabetically (case-insensitive) for dropdown and fallback.

---

## Empty / Single-Project States

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit 'no projects' empty state | Dedicated empty state when 0 projects | ✓ |
| Leave as-is | Let each view show its own empty/error | |
| You decide | Planner adds if cheap | |

**User's choice:** Explicit "no projects" empty state.

Follow-up — single project:

| Option | Description | Selected |
|--------|-------------|----------|
| Static label for single project | No dropdown when exactly 1 project | ✓ |
| Always show dropdown | Uniform placement even with 1 | |
| You decide | | |

**User's choice:** Static name label for a single project.

---

## Scope of Refinements

**Views in scope (multi-select):** Dashboard + Graph + Constitution; Spec & Decision detail pages; Constitution view polish; Exclude Keys page. → All project-scoped views in scope; `/keys` excluded.

**Extras question:**

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it tight — no extras | Just switch-refactor + defaults + empty states + constitution polish | |
| Add active-project indicator on pages | Show project name in breadcrumb/heading | ✓ |
| Something else | User describes | ✓ |

**User's choice:** Active-project indicator on pages **+ "let's adopt shadcn fully."**

**shadcn scope question:**

| Option | Description | Selected |
|--------|-------------|----------|
| Foundation now, full migration = own phase | Install stack + build new UI on shadcn, migrate rest later | |
| Full shadcn migration in this phase | Install stack AND convert every component/page | ✓ |
| Defer shadcn entirely | Build with existing plain CSS | |

**User's choice:** Full shadcn-svelte migration in this phase.
**Notes:** Web app currently uses plain scoped CSS — no Tailwind/shadcn/bits-ui. Facilitator flagged this as a large design-system migration larger than the selector work; user accepted the larger scope.

**shadcn theme question:**

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve current palette/look | Structural swap, keep brand colors | |
| Adopt shadcn default aesthetic | Visible redesign to shadcn default theme | ✓ |
| You decide | | |

**User's choice:** Adopt shadcn's default aesthetic (visible redesign).

**Dark mode question:**

| Option | Description | Selected |
|--------|-------------|----------|
| Light + dark with toggle | Ship both with a mode toggle | ✓ |
| Light only | shadcn default light theme, no toggle | |
| You decide | | |

**User's choice:** Light + dark with a toggle.

---

## the agent's Discretion

- Where to sort the project list (client vs server) — D-05.
- shadcn base color choice and component-structure conventions — D-12/D-13.
- Wave ordering / which components migrate first for the shadcn work.

## Deferred Ideas

- `/keys` page project-scoping — intentionally excluded (user-scoped).
- UI project CRUD from the web UI — not in scope; future phase.
- REQUIREMENTS.md v2 UI items UI-01 (spec syntax highlighting, `spgr-2pk`) and UI-02 (`SpecView.blockers` shape, `spgr-to31`) — candidates for the redesign later, not committed here.
