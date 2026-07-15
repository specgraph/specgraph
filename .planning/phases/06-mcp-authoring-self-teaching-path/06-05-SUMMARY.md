---
phase: 06-mcp-authoring-self-teaching-path
plan: 05
subsystem: testing
tags: [mcp, e2e, ginkgo, authoring, constitution, friendly-yaml, phase-gate]

# Dependency graph
requires:
  - phase: 06-mcp-authoring-self-teaching-path
    provides: MCP write surfaces accept friendly YAML (06-04)
  - phase: 06-mcp-authoring-self-teaching-path
    provides: skills teach friendly YAML + exchanges dual wire-format (06-02)
  - phase: 06-mcp-authoring-self-teaching-path
    provides: prime routes MCP-only agents with empty-state constitution hint (06-03)
provides:
  - "Automated MCP-client-only e2e gate proving an agent can author the constitution to persistence and walk a spec spark->approved from specgraph://prime alone, with no CLI/ConnectRPC service client constructed (D-08)"
  - "Regression coverage for the empty-state prime constitution routing hint on a fresh project"
  - "End-to-end verification of the real server ValidateExchanges rejection (missing exchanges; exchange missing sequence)"
affects: [mcp authoring self-teaching, phase gate, verify-work]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP-only e2e harness: project-scoped inner ConnectRPC transport wrapped by the real in-process MCP server; test bodies call only mcpCli.ReadResource/CallTool"
    - "Dedicated per-Describe project slug to prevent cross-suite constitution contamination"
    - "Canonical friendly-YAML output + JSON exchanges fixtures copied from the proven ConnectRPC fixtures (helpers_test.go) to keep the two verification paths in lockstep"

key-files:
  created:
    - e2e/api/mcp_only_authoring_test.go
  modified:
    - e2e/api/skills_test.go

key-decisions:
  - "Defined a dedicated project-scoped MCP harness (mcpProjectClient + mcpOnlyProject) instead of reusing skillsMCPClient, because skillsMCPClient uses http.DefaultClient with no X-Specgraph-Project header and scopeStore rejects the constitution/author write paths without it"
  - "Used a dedicated mcp-only-project slug so the constitution this test writes does not contaminate constitution_test.go's not-found assertion on the shared e2e-test project"

patterns-established:
  - "MCP-only e2e: never construct specgraphv1connect service clients; drive everything through mcpCli.ReadResource/CallTool (D-08 no-CLI simulation)"

requirements-completed: [MCP-01]

coverage:
  - id: D1
    description: "MCP-client-only run reads specgraph://prime, authors the constitution to persistence (friendly YAML update + get round-trip), and drives a spec spark->shape->specify->decompose->approve via friendly YAML output + JSON exchanges to approved, with NO CLI/ConnectRPC service client constructed"
    requirement: "MCP-01"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_authoring_test.go#MCP-only authoring authors the constitution and walks a spec spark->approved via MCP only"
        status: pass
    human_judgment: false
  - id: D2
    description: "specgraph://prime returns the empty-state constitution routing hint on a fresh project (order-independent via BeforeAll ClearAll)"
    requirement: "MCP-01"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_authoring_test.go#MCP-only authoring specgraph://prime returns the empty-state constitution hint on a fresh project"
        status: pass
    human_judgment: false
  - id: D3
    description: "Post-spark stages exercise the real server ValidateExchanges rejection: missing exchanges and an exchange missing sequence both IsError"
    requirement: "MCP-01"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_authoring_test.go#MCP-only authoring rejects a post-spark stage with valid output but no exchanges"
        status: pass
      - kind: e2e
        ref: "e2e/api/mcp_only_authoring_test.go#MCP-only authoring rejects a post-spark stage whose exchange is missing sequence"
        status: pass
    human_judgment: false
  - id: D4
    description: "skills_test.go skill-list assertion includes specgraph-constitution (seven skills) and its It description is updated"
    requirement: "MCP-01"
    verification:
      - kind: e2e
        ref: "e2e/api/skills_test.go#Skills via MCP lists seven skills via specgraph_skills_list"
        status: pass
    human_judgment: false

# Metrics
duration: 25 min
completed: 2026-07-14
status: complete
---

# Phase 06 Plan 05: MCP-only authoring e2e gate Summary

**An automated Ginkgo e2e (`Label("MCPOnly")`) drives specgraph://prime → specgraph_skills_get → constitution (friendly YAML) → author spark→shape→specify→decompose→approve entirely through the MCP client surface — no ConnectRPC service client constructed — and proves the constitution persists and the spec reaches approved.**

## Performance

- **Duration:** 25 min
- **Completed:** 2026-07-14
- **Tasks:** 1
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments
- Created `e2e/api/mcp_only_authoring_test.go`: an `Ordered`, `Label("MCPOnly")` Describe whose `BeforeAll` runs `serverInfo.Store.ClearAll(ctx)` for deterministic empty-state, and whose specs call ONLY `mcpCli.ReadResource`/`CallTool` (the D-08 no-CLI simulation).
- Empty-state prime smoke: `specgraph://prime` on a fresh project returns the plan-03 constitution routing hint (`No constitution configured` → `constitution` MCP tool / `specgraph-constitution` skill), order-independent.
- Full funnel walk: friendly-YAML constitution `update` persists and round-trips via `get`; spark (friendly YAML, creates the spec) → shape/specify/decompose (friendly YAML `output` + explicit canonical JSON `exchanges`) → approve (slug only); a follow-up `spec action:get` read tool asserts the approved stage (not the approve tool's echo).
- Two negative pre-flight specs exercise the real server `ValidateExchanges`: shape with no exchanges, and shape with an exchange missing `sequence` (defaults to 0 → rejected), both `IsError`.
- Fixed `skills_test.go` to assert `specgraph-constitution` in the skill list (seven skills) and renamed the `It` accordingly (review finding #4).

## Task Commits

1. **Task 1: MCP-only authoring e2e (prime smoke + constitution + full funnel) + skills fix** - `9c225e25` (test)

## Files Created/Modified
- `e2e/api/mcp_only_authoring_test.go` - MCP-only authoring e2e gate: project-scoped MCP harness, canonical friendly-YAML/exchanges fixtures, prime empty-state smoke, full funnel walk with spec-get assertion, two ValidateExchanges negative specs.
- `e2e/api/skills_test.go` - Skill-list assertion now includes `specgraph-constitution`; `It` description updated to seven skills.

## Decisions Made
- **Dedicated project-scoped MCP harness instead of reusing `skillsMCPClient`.** `skillsMCPClient` wires `http.DefaultClient` with no `X-Specgraph-Project` header; the constitution/author tools reach handlers that call `scopeStore`, which rejects requests lacking that header (`internal/server/project.go`). The new `mcpProjectClient` mirrors the harness but injects the header via `projectClientFor(mcpOnlyProject)`. Test bodies still speak only MCP — the inner `mcppkg.NewClient` is the MCP server's own backend wiring, not a surface the test calls (D-08 preserved).
- **Dedicated `mcp-only-project` slug** so the constitution this test writes cannot contaminate other suites (see Deviations).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Project-scoped MCP harness instead of reusing `skillsMCPClient`**
- **Found during:** Task 1
- **Issue:** The plan directed reusing `skillsMCPClient(ctx)`, but that harness uses `http.DefaultClient` with no project header. The constitution `update`/`get` and `author` tools hit `scopeStore`, which returns `CodeInvalidArgument` ("X-Specgraph-Project header required") without the header — so the funnel could never run through `skillsMCPClient`.
- **Fix:** Added a local `mcpProjectClient(ctx)` harness that mirrors `skillsMCPClient` but wires the project-scoped transport (`projectClientFor(mcpOnlyProject)`) into the MCP server's inner ConnectRPC client. Reused the existing `toolText` and `mcpResourceText` helpers verbatim. No `specgraphv1connect` service client is constructed in test bodies (D-08 intact).
- **Files modified:** e2e/api/mcp_only_authoring_test.go
- **Verification:** `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly` → 4/4 specs pass.
- **Committed in:** 9c225e25

**2. [Rule 1 - Bug] Dedicated project slug to avoid cross-suite constitution contamination**
- **Found during:** Task 1 (surfaced by the full `go test -tags e2e ./e2e/api/...` regression)
- **Issue:** An initial version wrote the constitution to the shared `e2eProject` (`e2e-test`). Under Ginkgo's random spec ordering, when the MCPOnly Describe ran before `constitution_test.go`, its "returns not-found when no constitution exists" spec found the seeded constitution and failed (1 failure in the full run).
- **Fix:** Introduced `const mcpOnlyProject = "mcp-only-project"` and pointed the harness at it — mirroring the per-Describe project convention already used across the suite (conversation-e2e, pipeline-project, prime-xsurf-project, …). The `BeforeAll` `ClearAll` is retained per the plan's acceptance criteria for order-independent empty-state.
- **Files modified:** e2e/api/mcp_only_authoring_test.go
- **Verification:** Full `go test -tags e2e ./e2e/api/...` green; re-run under `--ginkgo.seed` 1, 42, 99 all green (ordering-robust).
- **Committed in:** 9c225e25

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both were necessary for correctness — the harness change is the only way the MCP-only funnel can authenticate to a project, and the dedicated-slug fix removes cross-suite flakiness. The plan's intent (MCP-only D-08 gate, empty-state smoke, ValidateExchanges negatives, skills fix, label-filter convention) is fully realized. No scope creep.

## Issues Encountered
None beyond the two deviations above, which were resolved and verified.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The MCP-only self-teaching path is now locked behind an automated gate that cannot pass via the CLI. `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly` and the full `go test -tags e2e ./e2e/api/...` regression are green (across seeds), and `task check` passes (EXIT=0).
- Phase gate for MCP-01 is satisfied. Ready for `/gsd-verify-work`. The full `task pr-prep` pipeline (check → integration → e2e) should be run by the orchestrator/CI as the final phase gate.

## Self-Check: PASSED
- `e2e/api/mcp_only_authoring_test.go` — FOUND (created)
- `e2e/api/skills_test.go` — FOUND (modified)
- Commit `9c225e25` — FOUND
- `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly` — PASS (4/4 specs)
- `go test -tags e2e ./e2e/api/...` — PASS (seeds default/1/42/99)
- `task check` — PASS (EXIT=0)

---
*Phase: 06-mcp-authoring-self-teaching-path*
*Completed: 2026-07-14*
