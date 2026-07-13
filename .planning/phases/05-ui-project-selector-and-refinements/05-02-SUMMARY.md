---
phase: 05-ui-project-selector-and-refinements
plan: 02
subsystem: ui
tags: [svelte, vitest, runes, localstorage, project-selector, go, connectrpc]

# Dependency graph
requires:
  - phase: 05-01
    provides: shadcn-svelte + Tailwind v4 foundation (UI shell that will consume the selector)
provides:
  - Deterministic project-selection logic (case-insensitive sort + D-04 default precedence + D-06 stale fallback)
  - Vitest unit spec pinning the D-04/D-05/D-06 + zero-projects contract
  - Go test proving /api/projects excludes the internal '_server' slug
affects: [project-selector-ui, dropdown, project-switching]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Module-level rune reset in Vitest beforeEach for order-independent store tests"
    - "Fake ScopedBackend variant embedding stubBackend to assert handler-level filtering"

key-files:
  created:
    - web/src/lib/project.test.ts
  modified:
    - web/src/lib/project.svelte.ts
    - internal/server/api_handler_test.go

key-decisions:
  - "Client is the single source of truth for project ordering (D-05 sort applied client-side; server does not sort)"
  - "D-04 precedence: valid saved localStorage slug → project named 'default' → alphabetically-first → empty"
  - "D-06 stale fallback falls out of the tier-1 'in available' guard (no separate branch)"

patterns-established:
  - "Store tests reset module-level $state + localStorage shim in beforeEach for isolation"

requirements-completed: [D-04, D-05, D-06]

coverage:
  - id: D1
    description: "Project list sorted case-insensitively alphabetically (D-05)"
    requirement: "D-05"
    verification:
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-05: sorts the project list case-insensitively"
        status: pass
    human_judgment: false
  - id: D2
    description: "Default selection precedence: valid saved → 'default' → alpha-first (D-04)"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-04 tier 1: keeps a valid saved project present in the list"
        status: pass
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-04 tier 2: with no valid saved project, prefers a project named 'default'"
        status: pass
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-04 tier 3: with no saved and no default, picks the alphabetically-first project"
        status: pass
    human_judgment: false
  - id: D3
    description: "Stale saved project auto-falls-back per precedence (D-06)"
    requirement: "D-06"
    verification:
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-06: a saved project absent from the list falls back per precedence"
        status: pass
    human_judgment: false
  - id: D4
    description: "Zero available projects leaves project.current empty (D-07 seam)"
    verification:
      - kind: unit
        ref: "web/src/lib/project.test.ts#D-07 seam: an empty project list leaves current empty"
        status: pass
    human_judgment: false
  - id: D5
    description: "/api/projects excludes the internal '_server' slug (review MEDIUM fix)"
    verification:
      - kind: unit
        ref: "internal/server/api_handler_test.go#TestAPIHandler_ExcludesServerProject"
        status: pass
    human_judgment: false

# Metrics
duration: 5min
completed: 2026-07-12
status: complete
---

# Phase 5 Plan 2: Project Selection Logic (Sort + Default Precedence) Summary

**Deterministic `loadProjects()` — case-insensitive sort (D-05), D-04 three-tier default precedence, D-06 stale-fallback — pinned by six Vitest cases, plus a Go test proving `/api/projects` filters the internal `_server` slug.**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-07-12T09:42:00Z
- **Completed:** 2026-07-12T09:45:40Z
- **Tasks:** 3
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments
- `loadProjects()` now sorts `/api/projects` slugs case-insensitively before assigning `available` — client is the single source of truth for ordering (D-05).
- Applied the locked D-04 precedence: valid saved localStorage slug → project named `default` → alphabetically-first → empty; D-06 stale-fallback and the D-07 zero-projects seam fall out of the same guards.
- Added `web/src/lib/project.test.ts` — six order-independent Vitest cases with a `beforeEach` that resets the module-level rune + localStorage shim.
- Added `TestAPIHandler_ExcludesServerProject`, closing the review MEDIUM gap where the server-side `_server` exclusion filter was untested.

## Task Commits

Each task committed atomically:

1. **Task 1: Failing test — loadProjects sort + precedence + stale fallback (RED)** - `0213b648` (test)
2. **Task 2: Extend loadProjects with sort + D-04 precedence (GREEN)** - `ee16febd` (feat)
3. **Task 3: Go test — /api/projects excludes '_server'** - `04cf16e7` (test)

_No REFACTOR commit — the GREEN implementation was already minimal and clean._

## Files Created/Modified
- `web/src/lib/project.test.ts` - Six Vitest cases (D-04 x3, D-05, D-06, D-07 seam) with fetch/localStorage mocking and per-test module reset.
- `web/src/lib/project.svelte.ts` - `loadProjects()` body extended with case-insensitive sort + four-tier precedence; getter/setter, `STORAGE_KEY`, and state declarations unchanged.
- `internal/server/api_handler_test.go` - New `serverFilterScoper`/`serverFilterBackend` fakes + `TestAPIHandler_ExcludesServerProject` asserting the response is exactly `["alpha"]`.

## Decisions Made
- Ordering is enforced entirely client-side (D-05); the server never sorts. The Go test verifies only `_server` filtering (review Round-2 LOW #7), which complements — not duplicates — the client sort.
- D-06 is not a separate code branch: a stale saved slug simply fails the tier-1 "in available" guard and falls through to `default` / alpha-first.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None during planned work.

## TDD Gate Compliance

- **RED** (`0213b648`, `test`): `web/src/lib/project.test.ts` failed 4/6 cases before implementation (sort, default tier, alpha-first, stale-fallback) — confirmed genuine RED.
- **GREEN** (`ee16febd`, `feat`): all six cases pass after the `loadProjects` change.
- **REFACTOR**: none needed. Gate sequence satisfied.

## Deferred Issues

- **Pre-existing `dprint fmt` drift in `.planning/intel/classifications/*.json` (139 files)** causes `task check` to fail at `fmt:check`. Out of scope for 05-02 (no plan files affected); logged to `deferred-items.md`. The plan's own automated verifications (`pnpm -C web test` project spec, `go test -run TestAPIHandler_ExcludesServerProject`, `go build ./internal/server/`) all pass green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Deterministic, tested project-selection logic is ready for the selector UI to consume (dropdown source list + default resolution).
- No blockers introduced.

---
*Phase: 05-ui-project-selector-and-refinements*
*Completed: 2026-07-12*

## Self-Check: PASSED
- `web/src/lib/project.test.ts` — FOUND
- `web/src/lib/project.svelte.ts` — FOUND (modified)
- `internal/server/api_handler_test.go` — FOUND (modified)
- Commit `0213b648` (RED) — FOUND
- Commit `ee16febd` (GREEN) — FOUND
- Commit `04cf16e7` (Go test) — FOUND
