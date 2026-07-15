---
phase: 06-mcp-authoring-self-teaching-path
plan: 04
subsystem: mcp
tags: [mcp, authoring, constitution, yaml, protojson, self-teaching, go]

# Dependency graph
requires:
  - phase: 06-01
    provides: internal/authoring/load friendly-YAML parsers (SparkFromYAML/ShapeFromYAML/SpecifyFromYAML/DecomposeFromYAML) + reject-on-unknown enum mappers
  - phase: 06-02
    provides: MCP-first skills teaching the friendly-YAML output shape + JSON exchanges contract these handlers must accept
provides:
  - constitution MCP tool `update` accepts friendly YAML via internal/constitution/load (no protojson)
  - author MCP tool spark/shape/specify/decompose accept friendly snake_case YAML via internal/authoring/load (no protojson)
  - agent-facing tool Description + output/exchanges param docs rewritten to teach the friendly-YAML/JSON shapes
  - sanitized error paths on all five write handlers (no raw parser internals leaked, T-06-03)
affects: [06-05, mcp-authoring, self-teaching-path]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP write-boundary handlers map friendly YAML → typed proto via the load packages before the ConnectRPC call (server stays the schema)"
    - "Sanitized errResult at the MCP boundary — parser internals never surfaced to the agent (T-06-03), with //nolint:nilerr documenting the intentional error swallow"
    - "Explicit-layer guard: reject an empty-layer constitution rather than persisting a defaulted value"

key-files:
  created: []
  modified:
    - internal/mcp/tools_core.go
    - internal/mcp/tools_core_test.go
    - internal/mcp/tools_authoring.go
    - internal/mcp/tools_authoring_test.go

key-decisions:
  - "Replaced protojson entirely at the constitution/author write boundary (no dual-accept fallback) — the friendly-YAML shape is THE authoring format for MCP-01"
  - "exchanges stays a JSON array this milestone (documented + tested, not converted to YAML), matching 06-02 skills and 06-05 e2e"
  - "Added //nolint:nilerr with a T-06-03 reason on sanitized error returns — the swallow is intentional, the linter false-positives because err is no longer surfaced"

patterns-established:
  - "Every MCP write handler that ingests agent YAML returns a concise sanitized errResult; the load package owns reject-on-unknown enum/layer validation"
  - "Tool Description + param docs are a teaching surface — they document the exact friendly-YAML/JSON shapes the skills teach"

requirements-completed: [MCP-01]

coverage:
  - id: D1
    description: "constitution tool update accepts friendly YAML (layer: project) via constitution/load and persists as CONSTITUTION_LAYER_PROJECT — no protojson"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_core_test.go#TestConstitutionTool_Update_RoundTrip"
        status: pass
    human_judgment: false
  - id: D2
    description: "constitution update rejects invalid layer and empty-layer input with a sanitized errResult (T-06-01 tampering / T-06-03 no-leak / explicit-layer guard)"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_core_test.go#TestConstitutionTool_Update_InvalidInput"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_core_test.go#TestConstitutionTool_Update_EmptyLayer"
        status: pass
    human_judgment: false
  - id: D3
    description: "author spark/shape/specify/decompose accept friendly snake_case YAML via authoring/load and persist via the funnel RPCs — no protojson; strategy: vertical_slice persists as DECOMPOSITION_STRATEGY_VERTICAL_SLICE"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Spark"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Shape"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Specify"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Decompose"
        status: pass
    human_judgment: false
  - id: D4
    description: "Invalid enums (scope_sniff, strategy) return a sanitized errResult; malformed exchanges JSON is rejected at the MCP boundary via parseOptionalExchanges (T-06-01 / T-06-03)"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Spark_InvalidScopeSniff"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Decompose_InvalidStrategy"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Shape_MalformedExchanges"
        status: pass
    human_judgment: false
  - id: D5
    description: "Agent-facing tool Description + output/exchanges param docs teach the friendly-YAML/JSON shapes (no 'Full constitution JSON' / 'JSON string' wording)"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "task check (fmt/license/lint/build/unit — PASSED)"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-14
status: complete
---

# Phase 6 Plan 04: MCP Write-Boundary Friendly-YAML Summary

**Both MCP write surfaces — `constitution update` and the `author` spark/shape/specify/decompose funnel — now accept the friendly YAML the skills teach (via internal/constitution/load and internal/authoring/load), reject invalid enums/layers with sanitized errors, and no longer require protojson — closing the #1002 defect at the live MCP boundary.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-07-14T15:55:00Z
- **Completed:** 2026-07-14T16:15:19Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- **Task 1 — constitution:** `handleUpdate` replaced `protojson.Unmarshal` with `constload.FromYAML` + `constload.ToProto`; added an explicit-layer guard (FromYAML permits an empty layer, but a project bootstrap needs a real one → sanitized reject); rewrote the tool `Description` and the `constitution` param doc to teach the friendly-YAML shape (`layer: project`, `name:`, tech/principles/constraints/antipatterns/references `type: adr`). Removed the `protojson` import from `tools_core.go`.
- **Task 2 — author funnel:** each of `handleSpark`/`handleShape`/`handleSpecify`/`handleDecompose` replaced `protojson.Unmarshal` with the matching `authload.<Stage>FromYAML`; rewrote the `author` `Description` + `output` param doc (per-stage snake_case fields) and the `exchanges` param doc (names role/content/stage/sequence with a minimal two-element JSON example, states exchanges are required for shape/specify/decompose). Left `parseOptionalExchanges`, `handleRecord`, `handleStore`, posture, amend/supersede, and the analytical-pass paths behaviorally unchanged.
- **Sanitized errors (T-06-03):** all five write handlers now return concise no-internals messages (e.g. "invalid spark output (expected friendly snake_case YAML)") instead of leaking raw parser text; `//nolint:nilerr` with a T-06-03 reason documents the intentional error swallow.
- **Tests migrated (Pitfall 1):** the MCP tool tests for constitution + the four stages now submit friendly YAML; added invalid-enum tampering assertions (`scope_sniff: gigantic`, `strategy: sideways_slice`), an empty-layer case, and a malformed-`exchanges` MCP-boundary negative test. RPC service-client tests were left proto-typed and untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Constitution update — friendly YAML pipeline + description + tests** — `a489d97b` (feat)
2. **Task 2: Funnel stages — friendly YAML pipeline + description + tests** — `0af9d577` (feat)

## Files Created/Modified

- `internal/mcp/tools_core.go` — `handleUpdate` on `constitution/load`; explicit-layer guard; rewritten Description/param doc; `protojson` import removed.
- `internal/mcp/tools_core_test.go` — update tests migrated to friendly YAML; invalid-input + empty-layer cases added.
- `internal/mcp/tools_authoring.go` — four stage handlers on `authoring/load`; rewritten Description + `output`/`exchanges` param docs; sanitized error paths.
- `internal/mcp/tools_authoring_test.go` — stage tests migrated to friendly YAML; invalid-enum + malformed-exchanges negative tests.

## Repo-Wide protojson Audit (acceptance criterion)

`rg -n "protojson" --glob '*.go'` re-run at execution time confirms the constitution/author **write path** is protojson-free. Remaining `protojson.Unmarshal` callers are all legitimate retentions (unchanged, out of scope for MCP-01):

- `internal/mcp/tools_authoring.go` lines 78 / 437 / 544 — the `exchanges` (parseOptionalExchanges, handleRecord) and `findings` (handleStore) JSON-array isolation paths.
- `internal/mcp/tools_spec.go:58-61,232` — the `spec` tool's optional `*_output` create params (documented OUT-OF-SCOPE deferral).
- `cmd/specgraph/*`, `internal/render/prime.go`, `internal/mcp/resources.go`, `internal/mcp/helpers.go` — CLI / `jsonResult` output helpers, not MCP-tool write input.
- `internal/mcp/tools_core.go` + `internal/authoring/load/load.go` matches are comment/description text only (no import, no call).

No new MCP-tool protojson caller on the constitution/author write path appeared; nothing to migrate beyond the two test files handled here.

## Decisions Made

- **No protojson back-compat at the write boundary** (D-01/D-02/D-03): friendly YAML fully replaces protojson; a dual-accept path would keep the confusing surface alive and dilute the skill teaching. The only in-repo protojson MCP-tool write callers were the two migrated test files, so nothing in-repo breaks.
- **`exchanges` stays a JSON array** this milestone — documented and tested (minimal two-element example in the param doc), not converted to YAML, matching the 06-02 skills and the 06-05 e2e fixtures (the single documented dual-wire-format contract).
- **`//nolint:nilerr` on sanitized returns** — the `nilerr` linter false-positives when the checked `err` is no longer surfaced in the return (the pre-existing protojson code referenced `err` via `%v`, so it never fired). The nolint carries a T-06-03 reason; the swallow is intentional.

## Deviations from Plan

None — plan executed exactly as written.

The only judgment call within scope: the plan named the invalid-input cases as "invalid-enum per enum". Shape and specify have no enum to tamper, so their negative tests assert a type-mismatch YAML rejection (`scope_in: not-a-list` / `invariants: not-a-list`) while spark and decompose use true invalid-enum cases (`scope_sniff: gigantic`, `strategy: sideways_slice`) — the T-06-01 tampering assertion applies where an enum exists.

## Issues Encountered

- **`nilerr` lint failure** on the five sanitized error returns (the parser `err` is intentionally not surfaced). Resolved by adding `//nolint:nilerr // raw parser error intentionally not surfaced (T-06-03)` to each — documents intent and keeps the sanitized message. `task check` then passed fully.

## Merge-Window / Atomic-Release Constraint (carry-forward)

Per the ATOMIC-RELEASE INVARIANT (HIGH): this plan (wave 2) and 06-02 (wave 1) MUST ship in the SAME merge window (single PR or stacked merge, no intermediate deploy). 06-02 rewrote the skills to teach friendly YAML; this plan wired the handlers that accept it. Shipping either alone leaves a window where a live MCP-only agent reproduces #1002. Both are now on the phase branch together.

## Next Phase Readiness

- The MCP write boundary accepts exactly the friendly YAML the 06-02 skills teach; **06-05** owns the MCP-only e2e that round-trips the taught authoring YAML through the real ConnectRPC handler (including the missing-`sequence` server rejection that the mock-based unit tests cannot exercise) and corrects the six→seven skill-list assertion.
- No proto regen was performed; no blockers.

## Self-Check: PASSED

- `internal/mcp/tools_core.go` — FOUND (protojson import removed; `constload.FromYAML` present)
- `internal/mcp/tools_authoring.go` — FOUND (`authload.*FromYAML` in all four stage handlers)
- `internal/mcp/tools_core_test.go` / `internal/mcp/tools_authoring_test.go` — FOUND (friendly-YAML fixtures)
- Commit `a489d97b` (Task 1) — FOUND in git log
- Commit `0af9d577` (Task 2) — FOUND in git log
- `go test ./internal/mcp/` — ok
- `task check` — PASSED

---
*Phase: 06-mcp-authoring-self-teaching-path*
*Completed: 2026-07-14*
