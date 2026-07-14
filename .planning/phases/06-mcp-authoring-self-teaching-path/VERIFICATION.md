---
phase: 06-mcp-authoring-self-teaching-path
verified: 2026-07-14T16:36:13Z
status: human_needed
score: 2/4 must-haves verified
behavior_unverified: 2 # criteria 2 & 3 — full MCP-only funnel-to-approved flow is present, wired, and unit-proven at every boundary; its end-to-end behavioral proof (the MCPOnly e2e) requires Docker and was not executed in this environment
overrides_applied: 0
behavior_unverified_items:
  - truth: "Starting from only specgraph://prime in a fresh init-only project, an agent can discover and complete every authoring stage (Spark → Shape → Specify → Decompose → Approve) without any CLI/YAML knowledge."
    test: "Run the MCP-only e2e under Docker: `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly` (or `task pr-prep`)."
    expected: "The MCPOnly Describe passes: prime empty-state hint present, constitution persists via friendly YAML, spec walks spark→shape→specify→decompose→approve through mcpCli.CallTool only, spec get reflects `approved`."
    why_human: "This is a full multi-stage state-transition flow against a live Postgres-backed server. Unit tests prove each handler boundary and the e2e file is genuine and compiles, but the end-to-end persistence-to-approved transition can only be exercised with Docker (testcontainers), which is unavailable in this verification environment."
  - truth: "The constitution reaches an approved/completed state via MCP tool calls alone (no shell/CLI fallback required)."
    test: "Same MCPOnly e2e run under Docker; confirm `constitution action:get` round-trips the written constitution and `spec action:get` shows `approved`, with no specgraphv1connect service client constructed in the test body."
    expected: "Constitution round-trips (name/layer) and the spec reaches `approved` using only MCP ReadResource/CallTool."
    why_human: "Behavioral persistence + approval transition against the real ConnectRPC handler + DB; requires Docker to run."
human_verification:
  - test: "Run `task pr-prep` (or `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly`) with Docker running."
    expected: "The 4 MCPOnly specs in e2e/api/mcp_only_authoring_test.go pass, confirming the full MCP-only funnel-to-approved flow and the empty-state prime hint."
    why_human: "Live e2e requires Docker/testcontainers, unavailable in this verification run. All static + unit-level evidence is green."
---

# Phase 6: MCP Authoring Self-Teaching Path — Verification Report

**Phase Goal:** An agent in a fresh MCP-only project (`specgraph init` — `.mcp.json` + managed files, no source, no local CLI) can author the project constitution to completion using only `specgraph://prime` and the MCP-served skills, with no out-of-band CLI/YAML knowledge.
**Requirement:** MCP-01 (#1002)
**Verified:** 2026-07-14T16:36:13Z
**Status:** human_needed (code fully delivered; only the Docker-gated e2e remains to be executed by CI)
**Re-verification:** No — initial verification

## Goal Achievement

The #1002 defect fix is real and verified in code, not just claimed. Both MCP write surfaces now accept the friendly snake_case YAML the served skills teach, reject unknown enums/layers with sanitized errors instead of silent-writing, and no longer require protojson. The served skills lead with the MCP `author`/`constitution` tool path and demote the CLI to a gated appendix. `specgraph://prime` and every fresh-init empty-state route an MCP-only agent to the skills/tools, never to a CLI it cannot run. The MCP-only e2e gate exists, is genuine (constructs no service client), and compiles.

The two criteria that assert a full runtime funnel-to-approved flow (2 and 3) are present, wired, and unit-proven at each boundary, but their end-to-end behavioral proof — the `Label("MCPOnly")` e2e — requires Docker and could not be executed here. They are routed to CI/human verification rather than failed.

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | MCP-served authoring skills describe the full MCP `author`-tool round-trip (per-stage calls + inputs/outputs), not just CLI equivalents | ✓ VERIFIED | `specgraph-authoring/SKILL.md` leads "The write payload (this is the #1002 fix)" — per-stage friendly-YAML `output` (spark/shape/specify/decompose) + JSON `exchanges`, `author_start_stage` entry, CLI as a single gated "Requires local CLI … MCP-only agents skip this" appendix (L222). Guarded by `TestSkillMCPReference` (11 subtests PASS, run count=1). |
| 2 | From only `specgraph://prime` in a fresh init-only project, an agent can discover and complete every stage without CLI/YAML knowledge | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Routing present: `prime.go` `ConstitutionEmptyHint` (L27) + `writeSkills` "Start here: `specgraph_skills_list` … `specgraph_skills_get name=specgraph-constitution`" (L328). Each handler unit-proven. Full-flow proof is `e2e/api/mcp_only_authoring_test.go` (spark→approve via `mcpCli.CallTool` only) — genuine + compiles (`go vet -tags e2e` EXIT=0) but needs Docker to run. |
| 3 | The constitution reaches an approved/completed state via MCP tool calls alone (no CLI fallback) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `handleUpdate` → `constload.FromYAML`/`ToProto` → `UpdateConstitution`, no protojson (`tools_core.go:121-138`); funnel handlers persist via funnel RPCs. Unit tests green. End-to-end approved-state transition proven only by the Docker-gated MCPOnly e2e (asserts `spec action:get` → `approved`, L258). |
| 4 | `specgraph_skills_get`/`_search` return authoring guidance referencing the MCP tool path, verified against embedded canonicals | ✓ VERIFIED | Skills served from `internal/mcp/skills/embedded/*`; `specgraph-constitution` teaches "Step 4: Write It Over MCP" via `constitution action:update` with friendly YAML, `import` demoted to gated appendix (L199). `TestSkillMCPReference/.../constitution-parse-binding` + `authoring-snake-case-guard` + `authoring-exchanges-gate` PASS — the taught YAML round-trips the real parsers. |

**Score:** 2/4 truths verified (criteria 1, 4); 2 present, behavior-unverified (criteria 2, 3 — pending Docker e2e).

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/authoring/load/load.go` | Friendly-YAML → 4 stage protos, reject-on-unknown enums | ✓ VERIFIED | 234 lines; `SparkFromYAML`/`ShapeFromYAML`/`SpecifyFromYAML`/`DecomposeFromYAML` + `scopeSniffFromString`/`decompositionStrategyFromString`; invalid enum → returned error (L136, L220), never silent UNSPECIFIED. |
| `internal/mcp/tools_core.go` | `constitution update` on friendly YAML, no protojson | ✓ VERIFIED | `handleUpdate` uses `constload.FromYAML` (L121), explicit-layer guard (L129), sanitized error (L124); `protojson` import removed (only comment/desc text remains). |
| `internal/mcp/tools_authoring.go` | 4 stage handlers on friendly YAML, no protojson | ✓ VERIFIED | `handleSpark/Shape/Specify/Decompose` call `authload.*FromYAML` (L202/231/265/299), sanitized errors, `exchanges` JSON via `parseOptionalExchanges`. |
| `internal/mcp/skills/embedded/*/SKILL.md` (7) | MCP-first, gated CLI appendix | ✓ VERIFIED | All 7 pass `TestSkillMCPReference`; constitution + authoring teach the friendly-YAML write payload inline. |
| `internal/render/prime.go` / `internal/mcp/resources.go` | Shared MCP-first empty-state routing | ✓ VERIFIED | Single `ConstitutionEmptyHint` const referenced by project + spec prime surfaces and `constitutionEmptyResource`; no `specgraph constitution set` CLI text. |
| `e2e/api/mcp_only_authoring_test.go` | MCP-only funnel e2e, no service client | ✓ VERIFIED (exists/compiles) | Test bodies call only `mcpCli.ReadResource`/`CallTool`; no `specgraphv1connect.*ServiceClient` constructed; `go vet -tags e2e` EXIT=0. Runtime pass requires Docker. |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| `tools_core.go handleUpdate` | `internal/constitution/load` | `constload.FromYAML` → `ToProto` → `UpdateConstitution` (L121-133) | ✓ WIRED |
| `tools_authoring.go` handlers | `internal/authoring/load` | `authload.<Stage>FromYAML` → funnel RPC ×4 (L202-317) | ✓ WIRED |
| `prime.go` / `resources.go` | routing text | shared `render.ConstitutionEmptyHint` const, 3 call sites | ✓ WIRED |
| e2e test body | MCP surface only | `mcpProjectClient` → `mcpCli.CallTool`/`ReadResource` | ✓ WIRED (no CLI/service client) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Whole module builds | `go build ./...` | EXIT=0 | ✓ PASS |
| MCP/render/authoring unit suites | `go test -count=1 ./internal/mcp/... ./internal/authoring/load/...` | all `ok` | ✓ PASS |
| Author-handler friendly-YAML + invalid-enum tests | `go test -run 'TestAuthorTool' -v` | Spark/Shape/Specify/Decompose + InvalidScopeSniff/InvalidYAML PASS | ✓ PASS |
| Skill MCP-first + parser-binding gate | `go test -run TestSkillMCPReference -v` | 11 subtests PASS | ✓ PASS |
| MCP-only funnel-to-approved flow | `go test -tags e2e … --ginkgo.label-filter=MCPOnly` | not run — Docker/testcontainers unavailable | ? SKIP (→ human/CI) |
| e2e compiles under e2e tag | `go vet -tags e2e ./e2e/api/` | EXIT=0 | ✓ PASS |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| MCP-01 (#1002) | MCP-only agent authors the full funnel via friendly snake_case YAML; served skills/prime teach the MCP path (not the CLI import path) | ✓ SATISFIED (code) / CI-gated (runtime) | Write boundary protojson-free and friendly-YAML-accepting (unit-verified); skills/prime MCP-first (test-gated); MCP-only e2e gate exists and compiles (runtime pass needs Docker). |

### Anti-Patterns Found

None. Scan of all phase-modified files for `TODO/FIXME/XXX/PLACEHOLDER/not implemented/coming soon` returned no matches (the only `#1002` references are intentional fix annotations). Sanitized-error `//nolint:nilerr` on write handlers is documented with a T-06-03 reason (intentional, not debt).

### Human Verification Required

**1. Run the Docker-gated MCP-only e2e (closes criteria 2 & 3 behaviorally)**
- **Test:** With Docker running, `task pr-prep` — or `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly`.
- **Expected:** All 4 MCPOnly specs pass: (a) `specgraph://prime` returns the empty-state constitution hint on a fresh project; (b) constitution persists + round-trips and the spec walks spark→approve to `approved` via MCP only; (c)+(d) post-spark stages missing `exchanges` / missing `sequence` are rejected by the real server `ValidateExchanges`.
- **Why human:** Live ConnectRPC + Postgres (testcontainers) required; not runnable in this verification environment. All static and unit-level evidence is green and the e2e is genuine and compiles.

### Gaps Summary

No gaps. Every artifact exists, is substantive, and is wired; no stubs, no unwired links, no blocker anti-patterns. The #1002 root cause is closed at the live MCP write boundary and in the served teaching surfaces, confirmed by inspection of the actual code plus green build and unit suites. The only item preventing a `passed` verdict is that criteria 2 and 3 assert a full runtime funnel-to-approved flow whose end-to-end proof (the `MCPOnly` e2e) requires Docker and was not executed here — routed to CI/human verification, not failed. Once the MCPOnly e2e is run green under `task pr-prep`/CI, the phase is fully `passed`.

---

_Verified: 2026-07-14T16:36:13Z_
_Verifier: the agent (gsd-verifier)_
