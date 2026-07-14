---
phase: 06-mcp-authoring-self-teaching-path
plan: 01
subsystem: authoring
tags: [yaml, proto, authoring-funnel, mcp, go]

requires:
  - phase: 05-authoring-funnel
    provides: authoring stage proto outputs (SparkOutput/ShapeOutput/SpecifyOutput/DecomposeOutput)
provides:
  - internal/authoring/load package parsing friendly snake_case YAML into the four funnel stage protos
  - scopeSniffFromString / decompositionStrategyFromString enum mappers with reject-on-unknown semantics
affects: [mcp-author-handlers, plan-06-04, plan-06-05]

tech-stack:
  added: []
  patterns:
    - "Friendly-YAML → stage proto transform layer mirroring internal/constitution/load"
    - "Fixed typed friendly-input structs (never map[string]any) to bound input shape"
    - "Enum mapper returns UNSPECIFIED on unknown; parser converts UNSPECIFIED to a returned error"

key-files:
  created:
    - internal/authoring/load/load.go
    - internal/authoring/load/load_test.go
  modified: []

key-decisions:
  - "Reused the constitution/load FromYAML/ToProto shape and the tools_core.go enum-mapper style (_value map lookup)"
  - "Enum mappers only run when the friendly field is non-empty; empty enum leaves the proto zero value without error (minimal e2e payloads parse cleanly)"

patterns-established:
  - "Stage parsers unmarshal into typed structs then map field-by-field into the stage proto — no proto regen"
  - "Invalid enum values are rejected with an error, never silently written as UNSPECIFIED (T-06-01)"

requirements-completed: [MCP-01]

coverage:
  - id: D1
    description: "Friendly snake_case YAML for each funnel stage parses into the correct stage proto with nested repeated messages round-tripping every field"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestShapeFromYAML_FullNested"
        status: pass
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestSpecifyFromYAML_FullNested"
        status: pass
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestDecomposeFromYAML_FullNested"
        status: pass
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestSparkFromYAML_Full"
        status: pass
    human_judgment: false
  - id: D2
    description: "Invalid enum values (scope_sniff, strategy) are rejected with an error; multi-token strategy values map correctly"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestSparkFromYAML_InvalidScopeSniff"
        status: pass
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestDecomposeFromYAML_InvalidStrategy"
        status: pass
      - kind: unit
        ref: "internal/authoring/load/load_test.go#TestDecomposeFromYAML_MultiTokenStrategies"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-14
status: complete
---

# Phase 06 Plan 01: Friendly Funnel YAML Parser Summary

**New `internal/authoring/load` package parses friendly snake_case YAML for spark/shape/specify/decompose into their stage protos, with enum mappers that reject unknown values instead of silent-writing UNSPECIFIED.**

## Performance

- **Duration:** ~12 min
- **Tasks:** 2 (TDD RED + GREEN)
- **Files created:** 2

## Accomplishments
- Created the reusable friendly-YAML → stage-proto transform layer the MCP `author.<stage>` handlers (plan 06-04) will call, mirroring the constitution/load pipeline so "the server is the schema" holds without any proto change.
- Full nested-field round-trip coverage: `approaches[].tradeoffs`, all `decisions[]` fields, `interfaces[].name/body`, `verify_criteria[]`, `touches[]`, and `slices[].depends_on`.
- Enum correctness fix (#1002 / D-04): `scopeSniffFromString` and `decompositionStrategyFromString` map friendly lowercase → proto enum; unknown values become a returned error, never a silent UNSPECIFIED write. Multi-token strategies (vertical_slice/layer_cake/steel_thread) map correctly.
- Minimal/required-fields-only fixtures per stage (mirroring the 06-05 e2e payloads) parse cleanly with no spurious defaults.

## Task Commits

1. **Task 1: RED — failing parser + enum-rejection tests** - `025ebd5e` (test)
2. **Task 2: GREEN — implement the friendly funnel load package** - `7f489536` (feat)

_No REFACTOR commit — the GREEN implementation was already clean; the only post-GREEN change was a lint-driven local variable rename (`strat` → `strategy`) folded into the feat commit before it landed._

## Files Created/Modified
- `internal/authoring/load/load.go` - Four `*FromYAML` parsers, two enum mappers, and typed friendly-input structs with snake_case yaml tags.
- `internal/authoring/load/load_test.go` - Table-driven tests: full nested fixtures, minimal fixtures, all scope_sniff values, all four strategies, invalid-enum rejection, and malformed-YAML cases per stage.

## Decisions Made
- Reused the `internal/constitution/load` FromYAML shape and the `tools_core.go` `_value`-map enum-mapper style for consistency.
- Enum mapping only runs when the friendly field is non-empty, so minimal e2e payloads that omit an enum still parse (proto keeps its zero value) — invalid *non-empty* values still error.

## Deviations from Plan

None - plan executed exactly as written. (The `misspell` linter flagged the local variable `strat`; renamed to `strategy` before the GREEN commit landed. Not a behavioral deviation.)

## Issues Encountered
- `golangci-lint` `misspell` rejected the identifier `strat` (read as a misspelling of `start`). Resolved by renaming to `strategy`; `task check` then passed fully.

## TDD Gate Compliance
- RED gate: `025ebd5e` (`test(06-01)`) — test written and confirmed failing (`build failed: no non-test Go files`).
- GREEN gate: `7f489536` (`feat(06-01)`) — implementation lands, `go test ./internal/authoring/load/` and `task check` green.
- REFACTOR gate: not required.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Parser package is ready for plan 06-04 to wire into the MCP `author` handlers.
- No blockers.

## Self-Check: PASSED
- `internal/authoring/load/load.go` — FOUND
- `internal/authoring/load/load_test.go` — FOUND
- Commit `025ebd5e` (RED) — FOUND
- Commit `7f489536` (GREEN) — FOUND

---
*Phase: 06-mcp-authoring-self-teaching-path*
*Completed: 2026-07-14*
