---
phase: 6
reviewers: [cursor]
reviewed_at: 2026-07-14T13:31:21Z
plans_reviewed: [06-01-PLAN.md, 06-02-PLAN.md, 06-03-PLAN.md, 06-04-PLAN.md, 06-05-PLAN.md]
---

# Cross-AI Plan Review — Phase 6

## Cursor Review

# Phase 6 Plan Review: MCP Authoring Self-Teaching Path

**Reviewed against:** live repo at `/Volumes/Code/github.com/specgraph`  
**Plans:** 06-01 through 06-05  
**Requirement:** MCP-01 (#1002)

---

## Overall Phase Assessment

The five-plan wave structure is sound: extract a reusable funnel YAML parser (06-01), rewrite teaching surfaces in parallel (06-02/06-03), wire handlers (06-04), then gate with MCP-only e2e (06-05). The plans correctly anchor on verified defects — `handleUpdate` and the four funnel handlers all `protojson.Unmarshal` agent input today (`internal/mcp/tools_core.go:111`, `internal/mcp/tools_authoring.go:179/207/240/273`), while skills and prime still route agents to CLI (`internal/mcp/skills/embedded/specgraph-constitution/SKILL.md:24-26`, `internal/render/prime.go:210`). Reusing `internal/constitution/load.FromYAML` for the constitution path is well-grounded (`internal/constitution/load/load.go:22-33`).

Two gaps threaten full-funnel MCP-only success despite fixing `output` YAML: **(1)** `exchanges` remain protojson with server-side required validation for shape/specify/decompose (`internal/authoring/validate.go:43-44`, `internal/server/authoring_handler.go:137-138`), and plans do not teach or friendly-format that param; **(2)** plan 03 misidentifies which golden test breaks when `writeSkills` changes (`internal/render/prime_test.go:53-55` embeds the Skills section in `expectedProjectMatchLegacy`). Constitution-only MCP authoring is well-covered; the full spark→approve path needs sharper exchanges guidance in 02/05.

**Overall phase risk: MEDIUM** — architecture is right, but incomplete input-surface coverage and one test-planning error could stall wave 1 or produce a false-negative e2e gate.

---

## Plan 06-01: `internal/authoring/load` (TDD)

### Summary
Creates the missing friendly-YAML→proto layer for Spark/Shape/Specify/Decompose, mirroring the constitution pipeline. Correctly scoped as a standalone package with enum-rejection tests before handler wiring. Verified: `internal/authoring/load/` does not exist today; proto shapes are in `proto/specgraph/v1/authoring.proto`.

### Strengths
- **Correct layering.** Parser at MCP boundary, no proto regen — matches research and existing `internal/constitution/load` pattern.
- **Security-aligned enum handling.** UNSPECIFIED→error mirrors `constitutionLayerFromString` behavior in `internal/mcp/tools_core.go:29-34`.
- **TDD contract is concrete.** Tests cover all four stages plus both enums (`scope_sniff`, `strategy`) with invalid cases.
- **TestContentProtoDrift awareness.** Snake_case yaml tags matching proto field names (`scope_in`, `chosen_approach`, etc.) align with `internal/authoring/drift_test.go:54-64`.

### Concerns
- **MEDIUM — Nested message mapping complexity understated.** `ShapeOutput` includes `repeated Approach` and `repeated DecisionInput` with nested fields (`proto/specgraph/v1/authoring.proto:119-153`). Plan lists them in behavior but doesn't call out `tradeoffs`, `verify_criteria.category`, or `touches.change_type` as explicit test cases; implementation bugs here would only surface in 04/05.
- **LOW — Enum mapper edge cases.** Proto has `steel_thread`, `layer_cake`, `vertical_slice` (`authoring.proto:49-59`). Plan tests `single_unit` and `bogus` but not multi-token enum values; mapper spec (`ReplaceAll("-","_")` + `ToUpper`) should be tested for `vertical_slice` and `steel_thread`.
- **LOW — No `ToYAML`/round-trip.** Acceptable for phase scope, but means get→edit→update for funnel stages stays protojson on read path.

### Suggestions
- Add table-driven cases for at least one nested `approaches` block and one `decisions` block in `load_test.go`.
- Add enum cases for `vertical_slice` and `steel_thread`, not only `single_unit`.
- Document expected yaml shape for `DecompositionSlice.depends_on` (repeated field) in plan task behavior block.

### Risk Assessment
**LOW** — Isolated package, clear template (`internal/constitution/load/load.go`), no cross-plan runtime dependency until 06-04.

---

## Plan 06-02: MCP-First Skill Rewrite + Content Gate

### Summary
Rewrites all seven embedded skills and adds `skill_mcp_reference_test.go` to gate MCP-first posture and parser binding. Directly addresses the verified root cause: constitution skill Step 1/4 are CLI-first (`specgraph constitution show` / `import` at `SKILL.md:24-26`, `129-136`) while friendly YAML schema is already correct (`SKILL.md:51-101`).

### Strengths
- **Clever wave-1 decoupling.** Parser-binding for authoring uses snake_case key presence checks without importing `internal/authoring/load`, so 06-02 can run parallel to 06-01 (`plan task 1 action (b)`).
- **Constitution binding is real.** `load.FromYAML` already exists and is the same path CLI import uses — binding test is executable today.
- **D-07 is operationalized.** Uniform "Requires local CLI" appendix with automated substring gate.
- **Covers criteria #1 and #4** with content-level tests, matching D-09 precedent.

### Concerns
- **HIGH — `exchanges` teaching gap for full funnel.** `author` tool param docs say exchanges are "required for shape/specify/decompose" (`internal/mcp/tools_authoring.go:135`) and server enforces `ValidateExchanges` with "at least one exchange required" (`internal/authoring/validate.go:43-44`). Plan 02 rewrites `specgraph-authoring` to teach friendly `output` YAML but does not require teaching a minimal valid `exchanges` JSON array (needs `role`, `content`, `sequence` per `authoring.proto:455-463`). An MCP-only agent fixing `output` can still fail shape+ with opaque protojson errors from `parseOptionalExchanges` (`tools_authoring.go:77-78`). This undermines success criterion #2.
- **MEDIUM — Wave-1 skills/handler skew.** Skills can land teaching friendly YAML before 06-04 wires handlers; any agent using MCP between plans gets worse failures. Acceptable if waves are merged atomically, risky if plans land independently.
- **LOW — `specgraph-authoring` references `author_start_stage`.** Tool exists (`internal/mcp/tools_authoring.go:29`) — good — but rewrite should preserve/verify this path explicitly; plan doesn't mention it in Task 2 acceptance criteria.
- **LOW — `e2e/api/skills_test.go` still omits `specgraph-constitution`.** Listed in research pitfall 6 (`skills_test.go:106-121`); no plan task fixes it.

### Suggestions
- In `specgraph-authoring` rewrite, add a **canonical minimal `exchanges` example** per stage (copy structure from `e2e/api/helpers_test.go:117-119`: `role`, `content`, `stage`, `sequence`). Gate it in `skill_mcp_reference_test.go` with a substring check for `exchanges` + `sequence`.
- Add explicit acceptance criterion: `specgraph-authoring` documents `author_start_stage` as an alternative entry path (already in current skill at line 43).
- Optionally extend `skills_test.go` "lists six skills" to include `specgraph-constitution` while touching skill bodies.

### Risk Assessment
**MEDIUM** — Content work is thorough, but incomplete round-trip teaching for required non-YAML params leaves a verified server requirement unaddressed.

---

## Plan 06-03: Prime Entry-Point Routing

### Summary
Updates prime render and constitution empty-resource hints from CLI (`specgraph constitution set`) to MCP skill/tool routing. Targets verified strings at `internal/render/prime.go:210,326` and `internal/mcp/resources.go:40`.

### Strengths
- **Correct defect identification.** Both ProjectView and SpecView empty-states share the same CLI hint (`prime.go:210` vs `326`) — plan catches both; research initially under-emphasized SpecView.
- **Minimal scope.** No composer/DB logic changes; aligns with D-10 and prime reliability finding.
- **Adds missing SpecView test.** `TestRenderSpecMarkdown_EmptyConstitution` fills a real gap (plan notes no existing spec empty-state test).

### Concerns
- **HIGH — Wrong golden-test impact analysis.** Plan Task 1 claims `TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` "will NOT break" when changing `writeSkills`, but `expectedProjectMatchLegacy` includes the Skills section verbatim (`internal/render/prime_test.go:53-55`). Adding a "start here" routing line to `writeSkills# Phase 6 Plan Review: MCP Authoring Self-Teaching Path

**Reviewed:** 2026-07-14  
**Scope:** Plans 06-01 through 06-05 against repo state and MCP-01 / ROADMAP success criteria  
**Method:** Plan claims verified against source (`internal/mcp`, `internal/render`, `internal/authoring`, `e2e/api`, embedded skills)

---

## Phase-Level Assessment

The five-plan wave structure is sound and maps cleanly to the research recommendation (friendly YAML at the MCP write boundary, skills rewrite, prime routing, handler shim, MCP-only e2e gate). The root-cause analysis is accurate: today’s defect is real protojson at the write boundary (`internal/mcp/tools_core.go:111-112`, `internal/mcp/tools_authoring.go:178-179`), while the constitution skill already teaches friendly YAML but routes writes through CLI import (`internal/mcp/skills/embedded/specgraph-constitution/SKILL.md:129-135`). Reusing `internal/constitution/load` for constitution updates is the right minimal fix.

The plans will close #1002 for constitution authoring and stage `output` payloads, but **the full funnel MCP-only path is not fully closed** unless `exchanges` (still protojson-only at `internal/mcp/tools_authoring.go:70-78`) is taught and exercised. Server-side validation requires at least one exchange for shape/specify/decompose (`internal/authoring/validate.go:43-44`, enforced in `internal/server/authoring_handler.go:136-138`). Plan 05 mentions exchanges in passing; plans 02 and 04 do not address them. That gap threatens success criterion #2.

**Overall phase risk: MEDIUM** — architecture and sequencing are strong; two concrete plan errors/gaps (golden-test claim, exchanges teaching) could block the e2e gate or leave agents stuck mid-funnel.

---

## Plan 06-01: `internal/authoring/load` (TDD)

### Summary
Well-scoped foundation plan. Correctly mirrors `internal/constitution/load/load.go:22-32`, targets the verified gap (no `internal/authoring/load` package exists), and uses TDD with enum-rejection tests aligned to D-04 and ASVS V5.

### Strengths
- **Correct dependency anchor for plan 04** — funnel handlers need a parser before the handler shim can land.
- **Enum handling matches existing patterns** — `constitutionLayerFromString` at `internal/mcp/tools_core.go:29-34` is the right template; proto enums at `proto/specgraph/v1/authoring.proto:49-81` confirm `scope_sniff` and `strategy` are the only friendly enum fields.
- **TestContentProtoDrift awareness** — snake_case yaml tags matching proto field names (`scope_in`, `chosen_approach`, etc. in `authoring.proto:119-143`) is correct given `internal/authoring/drift_test.go:54-64`.
- **Security posture is explicit** — fixed structs, UNSPECIFIED→error; consistent with research threat model.

### Concerns
| Severity | Issue |
|----------|-------|
| **MEDIUM** | Nested message mapping complexity (`Approach`, `DecisionInput`, `InterfaceSection`, `VerifyCriterion`, `FileTouch`, `DecompositionSlice` in `authoring.proto:111-190`) is underspecified in task behavior blocks. Task 1 lists shapes/decisions but not all nested types; implementer may ship incomplete parsers that pass minimal tests but fail real skill examples. |
| **LOW** | Enum mapper spec (`DECOMPOSITION_STRATEGY_` + `ToUpper`) must accept values like `vertical_slice` and `steel_thread` (proto lines 52-59). Tests cover `single_unit` and `bogus` but not all enum variants. |
| **LOW** | RED verify uses `grep` on test output — brittle but acceptable for plan automation. |

### Suggestions
- Add one table-driven fixture per stage that mirrors the YAML blocks plan 02 will embed in `specgraph-authoring` (full nested structures, not scalars only).
- Add positive tests for `vertical_slice`, `layer_cake`, `steel_thread` enum strings.
- Export a small `ToProto` helper per stage (mirror `constitution/load.ToProto`) so plan 04 handlers stay thin.

### Risk Assessment
**LOW** — Isolated package, clear template, no proto regen. Main risk is incomplete nested-field coverage.

---

## Plan 06-02: MCP-First Skills Rewrite + Content Gate

### Summary
Strong content plan and the best guard against regression. Parser-binding test design (constitution → `load.FromYAML`, authoring → snake_case key guard without importing plan 01) is clever and keeps wave-1 parallelism valid.

### Strengths
- **Targets the actual #1002 symptom** — `specgraph-constitution` is CLI-first today (`SKILL.md:22-26`, `129-135`); rewrite to `constitution` tool `update` is correct.
- **D-07 appendix pattern** is concrete and testable via `"Requires local CLI"` substring gate.
- **Constitution parser binding uses existing code** — `internal/constitution/load/load.go:22-32` is already production-used; no cross-plan compile dependency for that assertion.
- **Preserves friendly YAML schema** the load pipeline already accepts (`layer: "project"`, `type: "adr"` at `SKILL.md:52-100`).

### Concerns
| Severity | Issue |
|----------|-------|
| **HIGH** | **`exchanges` not in scope.** Shape/specify/decompose require `ConversationExchanges` server-side (`authoring_handler.go:136-138`). MCP param docs still say protojson (`tools_authoring.go:135`). Rewritten `specgraph-authoring` must teach a minimal valid `exchanges` JSON array (role/content/stage/sequence per `authoring.proto:455-463`), or plan 05 e2e will fail even after friendly `output` lands. Current skill has zero exchange guidance (`specgraph-authoring/SKILL.md:47-49`). |
| **MEDIUM** | **Wave-1 skills land before wave-2 handlers (06-04).** Agents hitting MCP between plans get friendly-YAML docs against protojson-only handlers. Acceptable if waves are merged atomically; risky if plans ship independently. |
| **LOW** | `e2e/api/skills_test.go:112-119` lists 6 of 7 skills (omits `specgraph-constitution`). Plans note this (pitfall 6) but no task fixes it — minor coverage gap. |
| **LOW** | Parser-binding for authoring is key-presence only until plan 05; a syntactically valid but semantically empty YAML block could pass the gate. |

### Suggestions
- Add explicit task content in Task 2: document minimal `exchanges` JSON for shape/specify/decompose, with a copy-paste example matching `e2e/api/helpers_test.go:117-119` field shape.
- Extend `skill_mcp_reference_test.go` to assert `specgraph-authoring` mentions `exchanges` for post-spark stages.
- Optionally add `depends_on: [06-04]` soft note in plan metadata for release ordering documentation (not a hard blocker for wave-1 test authoring).

### Risk Assessment
**MEDIUM** — Content rewrite is thorough for `output`/constitution, but incomplete for the other half of the author tool contract (`exchanges`).

---

## Plan 06-03: Prime Entry Point + Empty-State Routing

### Summary
Correctly identifies all three CLI-first empty-state sites (`internal/render/prime.go:210`, `326`; `internal/mcp/resources.go:40`) and the skills section that needs stronger routing (`prime.go:306-314`). SpecView empty-state test addition is a good catch — `writeSpecConstitution` duplicates the CLI hint at line 326 and has no test today (`prime_test.go:59-65` only covers ProjectView).

### Strengths
- **Accurate prime-failure diagnosis** — environmental DB failure vs. code bug; consistent with soft-empty constitution in composer (research claim holds).
- **Minimal D-10 scope** — routing text only, no duplicate teaching in prime.
- **SpecView coverage** — closes a real hole for “spark before constitution” flows.

### Concerns
| Severity | Issue |
|----------|-------|
| **HIGH** | **Incorrect claim about golden layout test.** Plan states `TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` “will NOT break” when `writeSkills` changes. That test is byte-for-byte against `expectedProjectMatchLegacy`, which **includes the Skills section verbatim** (`internal/render/prime_test.go:53-55`, asserted at `67-71`). Adding a “start here” routing line to `writeSkills` **will break this test** unless `expectedProjectMatchLegacy` is updated. Plan only mentions updating `TestRenderProjectMarkdown_EmptyConstitution`. |
| **LOW** | Task 2 done-text says “golden test updated” but no golden update task exists for the populated-layout test. |
| **LOW** | `constitutionEmptyResource` has no dedicated unit test; consistency relies on manual review. |

### Suggestions
- Add explicit task step: update `expectedProjectMatchLegacy` in `internal/render/prime_test.go:21-57` after `writeSkills` change.
- Add a small test for `constitutionEmptyResource` output (or assert via existing `internal/mcp` resource tests) matching project/spec empty-state wording.
- Keep the three surfaces (project prime, spec prime, constitution resource) on one shared constant or helper to prevent wording drift.

### Risk Assessment
**MEDIUM** — Small code change, but the wrong golden-test assumption will cause a surprise `task check` failure during implementation.

---

## Plan 06-04: MCP Write-Input Handler Shim

### Summary
The core #1002 fix. Correctly wires constitution through existing load pipeline and funnel stages through plan-01 parsers. Tool description rewrites align with criterion #4. Test migration scope (`tools_core_test.go:83-87` protojson today) is identified.

### Strengths
- **Minimal handler change for constitution** — replace `protojson.Unmarshal` at `tools_core.go:111-112` with `load.FromYAML` → `load.ToProto` is ~10 lines as research predicted.
- **Correct wave dependency** — `depends_on: [06-01]` is necessary.
- **Leaves RPC layer untouched** — no storage/proto change; matches architectural map.
- **Invalid-input tests** — aligns with `TestConstitutionTool_Update_InvalidJSON` pattern at `tools_core_test.go:93-106`.

### Concerns
| Severity | Issue |
|----------|-------|
| **HIGH** | **`exchanges` left protojson** (`parseOptionalExchanges` at `tools_authoring.go:70-78`). For MCP-only self-teaching, this is the remaining protojson surface on the critical path. Tool description still says “JSON array of ConversationExchange objects” (`tools_authoring.go:135`). Skills + descriptions should document the exact JSON shape (proto field names: `role`, `content`, `stage`, `sequence`). |
| **MEDIUM** | **No back-compat fallback.** Research recommended optional protojson fallback for belt-and-suspenders; plan removes protojson entirely. Fine for MCP-only goal, but all MCP tool callers must migrate (plan acknowledges Pitfall 1; `helpers_test.go:advanceStage` uses ConnectRPC not MCP tools — unaffected). |
| **MEDIUM** | **`constitution.get` still returns protojson** via `jsonResult` (`tools_core.go:102`). Less blocking for fresh-init (author from template), but get→edit→update round-trips still expose proto enums. Research nicety omitted. |
| **LOW** | `referenceType` invalid values map to UNSPECIFIED in `load/referenceTypeToProto` (`load.go:94-106`) without error — pre-existing; not introduced by plan but weakens “impossible to get wrong” for constitution references. |

### Suggestions
- In Task 2, rewrite `exchanges` param documentation with a minimal worked example (two-exchange probe/response pair with `sequence: 1` and `sequence: 2`, matching `helpers_test.go:117-119`).
- Consider accepting YAML for `exchanges` in a follow-up, or defer explicitly to Phase 8 (CONV-01) with a note in skills that exchanges remain JSON for v0.14.0.
- Add one negative test: `exchanges` with missing `sequence` returns `errResult` through the real handler path.
- Optionally emit friendly YAML from `constitution.get` (research enhancement) — out of scope is fine if documented as known limitation.

### Risk Assessment
**MEDIUM** — Constitution path is low-risk and well-proven. Funnel path is correct for `output` but incomplete for mandatory `exchanges`.

---

## Plan 06-05: MCP-Only Authoring E2E Gate

### Summary
Appropriate phase gate (D-08). Reuses the proven harness (`e2e/api/skills_test.go:35-57`) and correctly forbids ConnectRPC service clients in the spec itself. Dependencies `[06-02, 06-03, 06-04]` are correct.

### Strengths
- **Real MCP server path** — `skillsMCPClient` spins in-process `mcp.NewServer` against e2e Postgres; not a stub.
- **Prime smoke extends partial coverage** — `prime_cross_surface_test.go:156-162` proves prime works with data; plan adds empty-state assertion (gap today).
- **Constitution + full funnel in one spec** — matches MCP-01 end-to-end intent.
- **`mcpResourceText` reuse** — `prime_cross_surface_test.go:203-211` avoids duplication.

### Concerns
| Severity | Issue |
|----------|-------|
| **HIGH** | **E2E will require protojson `exchanges` on shape/specify/decompose** unless server validation is bypassed (it isn’t). Plan says “plus required exchanges where the funnel demands them” but does not specify the JSON payload. Implementer must derive from `helpers_test.go:117-159` — should be explicit in the plan to avoid gate flakiness. |
| **MEDIUM** | **Approve may need exchanges** — server validates exchanges on approve in some paths (`authoring_handler.go:488`). Plan only lists `approve` with slug; verify approve path doesn’t require exchanges (spark-only approve might be OK if prior stages recorded conversations). Trace before e2e authoring. |
| **MEDIUM** | **“Fresh/empty constitution project” setup unclear** — prime smoke needs a project with no constitution. Harness must use a clean project scope or assert substring resilient to populated state. |
| **LOW** | `-run MCPOnly` label dependency — Ginkgo `Describe` label must match exactly or gate command silently skips. |

### Suggestions
- Embed canonical minimal `exchanges` fixtures in the test file (copy structure from `helpers_test.go:117-119`, `137-139`, `156-158`) as string constants for MCP `CallTool` args.
- Add a pre-flight `It` that asserts `author` shape with friendly `output` but **no** `exchanges` fails — documents the contract the skill must teach.
- Document project isolation strategy (unique project name / fresh test DB) for empty-state prime assertion.
- After green, run full `go test -tags e2e ./e2e/api/...` not just `-run MCPOnly` to catch regressions in `prime_cross_surface_test.go` after golden/layout changes from plan 03.

### Risk Assessment
**MEDIUM-HIGH** — Right gate design, but underspecified on the mandatory `exchanges` input that current plans don’t fully self-teach.

---

## Cross-Cutting Findings

### Dependency ordering
```
Wave 1 (parallel): 06-01, 06-02, 06-03
Wave 2: 06-04 → depends on 06-01
Wave 3: 06-05 → depends on 06-02, 06-03, 06-04
```
Ordering is logically sound. The only sequencing hazard is **06-02 landing before 06-04** during development.

### Security
Plans adequately address V5 for new YAML parsers (typed structs, enum rejection). No new auth surface. Error sanitization via `errResult` is consistent with existing handlers.

### Performance
Negligible — YAML parse at MCP boundary is cheap relative to Postgres RPC. No concerns.

### Scope creep
Appropriate for MCP-01. Seven-skill rewrite is large but required by D-05. No obvious over-engineering (section-by-section tools correctly rejected in research).

### Phase goal coverage

| Success criterion | Plans cover? | Evidence / gap |
|-------------------|--------------|----------------|
| #1 Skills describe MCP round-trip | **Partial** | 06-02 covers `output`/constitution; **gap: `exchanges`** |
| #2 Full funnel MCP-only | **At risk** | 06-05 plans it; blocked by exchanges teaching + protojson param |
| #3 Constitution MCP-only | **Yes** | 06-04 + 06-02 + 06-05 |
| #4 skills_get/search reference MCP path | **Yes** | 06-02 content gate |
| Prime reliability | **Yes** | 06-03 + 06-05 smoke |

---

## Priority Fixes Before Execution

1. **Fix plan 06-03** — Acknowledge `TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` / `expectedProjectMatchLegacy` must be updated when `writeSkills` changes (`prime_test.go:53-71`).
2. **Extend plans 06-02, 06-04, 06-05** — Treat `exchanges` as a first-class self-teaching surface: skill docs, tool param examples, e2e fixtures (even if format stays protojson for this milestone).
3. **Strengthen plan 06-01** — Full nested-message test fixtures aligned with skill YAML examples.
4. **Optional** — Fix `skills_test.go` “lists six skills” omission of `specgraph-constitution` while touching skill tests.

---

## Final Risk Assessment

| Plan | Risk |
|------|------|
| 06-01 | LOW |
| 06-02 | MEDIUM |
| 06-03 | MEDIUM |
| 06-04 | MEDIUM |
| 06-05 | MEDIUM-HIGH |
| **Phase overall** | **MEDIUM** |

**Justification:** The plans are well-researched, correctly identify the protojson defect, reuse proven constitution load code, and install the right verification gates. They will likely ship constitution MCP-only authoring cleanly. The full-funnel MCP-only promise is at risk because mandatory `exchanges` remain protojson (`tools_authoring.go:70-78`, `validate.go:43-44`) without a teaching or testing plan commensurate with the `output`/constitution work, and plan 06-03 contains a factual error about the prime golden layout test that will break `task check` if unaddressed.

---

## Consensus Summary

Only one external reviewer (Cursor) was invoked, so "consensus" here reflects Cursor's source-grounded findings rather than agreement across multiple models. Cursor read the live repo and cited `file:line` evidence throughout; its two highest-severity findings are actionable and were not raised by the internal plan-checker.

### Agreed Strengths
- Wave structure and sequencing are sound: reusable funnel YAML parser (06-01) → parallel skills/prime rewrites (06-02/06-03) → handler shim (06-04) → MCP-only e2e gate (06-05).
- Root-cause anchoring is accurate and verified against source: real `protojson.Unmarshal` at the write boundary (`internal/mcp/tools_core.go:111`, `internal/mcp/tools_authoring.go:178-273`), CLI-first constitution skill (`SKILL.md:24-26,129-135`), and CLI hints in prime (`internal/render/prime.go:210,326`).
- Reuse of `internal/constitution/load.FromYAML` for the constitution path is the correct minimal fix (~10 lines, no proto regen).
- Security posture (typed structs, UNSPECIFIED→error enum rejection, `errResult` sanitization) is consistent with existing patterns.

### Agreed Concerns (highest priority)
1. **[HIGH] `exchanges` self-teaching gap threatens success criterion #2 (full funnel MCP-only).** Shape/specify/decompose require a `ConversationExchanges` payload server-side (`internal/authoring/validate.go:43-44`, enforced at `internal/server/authoring_handler.go:136-138`), and that param is still protojson-only (`internal/mcp/tools_authoring.go:70-78`, param docs at `:135`). The plans make the stage `output` friendly YAML but never teach or friendly-format `exchanges` (role/content/stage/sequence per `authoring.proto:455-463`). An MCP-only agent that fixes `output` can still hit opaque protojson errors on the first post-spark stage — the exact #1002 failure class, unclosed. Raised against 06-02 (teaching), 06-04 (handler/param docs), and 06-05 (e2e fixtures).
2. **[HIGH] 06-03 golden-test impact analysis is wrong and will surprise-fail `task check`.** The plan claims `TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` will NOT break when `writeSkills` changes, but `expectedProjectMatchLegacy` embeds the Skills section verbatim (`internal/render/prime_test.go:53-55`, asserted at `:67-71`). Adding a "start here" routing line to `writeSkills` WILL break the byte-for-byte golden unless `expectedProjectMatchLegacy` is updated. The plan only mentions updating `TestRenderProjectMarkdown_EmptyConstitution`.
3. **[MEDIUM] 06-01 nested-message mapping underspecified.** `ShapeOutput`/`DecomposeOutput` carry nested `repeated Approach`, `DecisionInput`, `VerifyCriterion`, `FileTouch`, `DecompositionSlice` (`authoring.proto:111-190`). Task behavior blocks list top-level shapes but don't force test coverage of nested fields or all enum variants (`vertical_slice`, `steel_thread`, `layer_cake`) — incomplete parsers could pass minimal tests yet fail real skill YAML in 06-04/06-05.
4. **[LOW] `e2e/api/skills_test.go` "lists six skills" omits `specgraph-constitution`** (`skills_test.go:112-119`); noted in research pitfall 6 but no plan task fixes it while skill tests are being touched.

### Divergent Views
None — single reviewer. Note that findings #1 and #2 are new relative to the internal plan-checker's pass (which returned 0 blockers / 3 now-resolved warnings), so they warrant independent verification before execution.

### Recommended Action
Findings #1 and #2 are concrete and cheap to fold in. Consider running `/gsd-plan-phase 6 --reviews` to incorporate them — specifically: (a) add `exchanges` as a first-class self-teaching surface in 06-02 skill docs + 06-04 param docs + 06-05 e2e fixtures (even if the wire format stays JSON for this milestone), and (b) correct 06-03 to update `expectedProjectMatchLegacy` when `writeSkills` changes.
