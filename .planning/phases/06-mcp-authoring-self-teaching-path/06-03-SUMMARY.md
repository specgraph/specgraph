---
phase: 06-mcp-authoring-self-teaching-path
plan: 03
subsystem: mcp
tags: [prime, render, mcp, skills, constitution, authoring-routing]

requires:
  - phase: 06-mcp-authoring-self-teaching-path
    provides: embedded skills catalog served via specgraph_skills_* MCP tools
provides:
  - "specgraph://prime routes an MCP-only agent to the authoring skills as the explicit next step"
  - "Fresh-init constitution empty-states (project prime, spec prime, MCP resource) route to the constitution MCP tool / specgraph-constitution skill instead of the CLI (D-10)"
  - "Shared render.ConstitutionEmptyHint const preventing drift across the three fresh-init surfaces"
affects: [mcp authoring self-teaching, prime, constitution]

tech-stack:
  added: []
  patterns:
    - "Single shared exported const for cross-surface user-facing routing text (prevents copy-paste drift)"

key-files:
  created: []
  modified:
    - internal/render/prime.go
    - internal/render/prime_test.go
    - internal/mcp/resources.go
    - internal/mcp/resources_test.go

key-decisions:
  - "Empty-state routing text lives in one exported const render.ConstitutionEmptyHint, referenced by all three fresh-init surfaces (internal/mcp already imports internal/render), so the surfaces cannot drift via copy-paste"
  - "writeSkills routing sentence names specgraph_skills_list AND specgraph_skills_get (not only get), staying consistent with prime already surfacing list/search/get"
  - "Corrected the stale served-skill count 6->7 to match the live embedded catalog (7 skills)"

patterns-established:
  - "Shared const for user-facing routing strings: define once, reference at every call site"

requirements-completed: [MCP-01]

coverage:
  - id: D1
    description: "writeSkills emits a start-here authoring routing sentence naming specgraph_skills_list and specgraph_skills_get and the specgraph-constitution/specgraph-authoring skills"
    requirement: "MCP-01"
    verification:
      - kind: unit
        ref: "internal/render/prime_test.go#TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout"
        status: pass
    human_judgment: false
  - id: D2
    description: "Project + Spec prime constitution empty-states route to the constitution MCP tool / specgraph-constitution skill via shared ConstitutionEmptyHint, no specgraph constitution set CLI hint (D-10)"
    requirement: "MCP-01"
    verification:
      - kind: unit
        ref: "internal/render/prime_test.go#TestRenderProjectMarkdown_EmptyConstitution"
        status: pass
      - kind: unit
        ref: "internal/render/prime_test.go#TestRenderSpecMarkdown_EmptyConstitution"
        status: pass
    human_judgment: false
  - id: D3
    description: "MCP constitution empty-resource (specgraph://constitution) routes to the MCP tool/skill via shared render.ConstitutionEmptyHint, no CLI hint"
    requirement: "MCP-01"
    verification:
      - kind: unit
        ref: "internal/mcp/resources_test.go#TestConstitutionResource_NotFoundRendersHint,TestConstitutionResource_SlugRequiredRendersHint"
        status: pass
      - kind: integration
        ref: "task check"
        status: pass
    human_judgment: false

duration: 5 min
completed: 2026-07-14
status: complete
---

# Phase 06 Plan 03: MCP-first Prime & Constitution Routing Summary

**`specgraph://prime` and the constitution empty-states now route a fresh-init MCP-only agent to the authoring skills (`specgraph_skills_list`/`get`) and the `constitution` MCP tool instead of a CLI it does not have (D-10), via a single shared `render.ConstitutionEmptyHint` const.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-07-14T16:00:05Z
- **Completed:** 2026-07-14T16:04:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added exported `const render.ConstitutionEmptyHint` — the one routing string shared by all three fresh-init constitution surfaces, so they cannot drift via copy-paste.
- `writeProjectConstitution` and `writeSpecConstitution` empty-states now use the shared MCP-first hint; no `specgraph constitution set` CLI text remains on either prime surface (D-10).
- `writeSkills` now emits a "start here" routing sentence naming `specgraph_skills_list` and `specgraph_skills_get` plus the `specgraph-constitution`/`specgraph-authoring` skills.
- `constitutionEmptyResource` (`specgraph://constitution`) references the same shared const, keeping the MCP resource consistent with the prime surfaces.
- Corrected the stale served-skill count 6→7 in the golden and fixture to match the live embedded catalog (7 skills).

## Task Commits

Each task was committed atomically:

1. **Task 1: Prime render routing + empty-state MCP hint (project + spec) (+ tests)** - `33422dd6` (feat)
2. **Task 2: MCP constitution empty-resource routing** - `caffc263` (feat)

## Files Created/Modified
- `internal/render/prime.go` - Added `ConstitutionEmptyHint` const; project + spec empty-states reference it; `writeSkills` routing sentence.
- `internal/render/prime_test.go` - Updated `expectedProjectMatchLegacy` golden (routing line + 6→7 count), `fixtureProjectView().SkillsCount` 6→7, `TestRenderProjectMarkdown_EmptyConstitution` MCP-first wording, added `TestRenderSpecMarkdown_EmptyConstitution`.
- `internal/mcp/resources.go` - `constitutionEmptyResource` references `render.ConstitutionEmptyHint`.
- `internal/mcp/resources_test.go` - Updated `TestConstitutionResource_NotFoundRendersHint` and `TestConstitutionResource_SlugRequiredRendersHint` to assert MCP-first routing text and `NotContains` the old CLI string.

## Decisions Made
- Empty-state routing text is defined once as an exported const and referenced at all three call sites (`internal/mcp` already imports `internal/render`), the REQUIRED shared-const approach from the pass-2 review finding — inline literals would let the surfaces drift.
- The `writeSkills` routing sentence names `specgraph_skills_list` (not only `get`) to stay consistent with prime already surfacing list/search/get (Cursor review 06-03 suggestion #2).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. `go test ./internal/render/ ./internal/mcp/` and `task check` all green (EXIT=0).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 06-03 complete. The fresh-init MCP-only path no longer surfaces any `specgraph constitution` CLI hint. Plan 05 adds the empty-state prime smoke assertion referenced in the objective.

## Self-Check: PASSED
- `internal/render/prime.go` — FOUND (modified, contains `ConstitutionEmptyHint`)
- `internal/mcp/resources.go` — FOUND (references `render.ConstitutionEmptyHint`)
- Commit `33422dd6` — FOUND
- Commit `caffc263` — FOUND
- `go test ./internal/render/ ./internal/mcp/` — PASS
- `task check` — PASS (EXIT=0)

---
*Phase: 06-mcp-authoring-self-teaching-path*
*Completed: 2026-07-14*
