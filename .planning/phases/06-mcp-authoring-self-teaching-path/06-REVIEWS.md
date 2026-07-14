---
phase: 6
reviewers: [cursor]
reviewed_at: 2026-07-14T14:43:33Z
plans_reviewed: [06-01-PLAN.md,06-02-PLAN.md,06-03-PLAN.md,06-04-PLAN.md,06-05-PLAN.md]
---

# Cross-AI Plan Review — Phase 6

## Cursor Review

# Phase 6 Plan Review — MCP Authoring Self-Teaching Path

Verified against the live repo at commit time. Key defect sites, test fixtures, and skill canonicals match what the plans describe.

---

## Overall Assessment

These five plans form a coherent, evidence-grounded fix for #1002: they correctly identify the protojson write boundary as the root cause (`internal/mcp/tools_core.go:111`, `internal/mcp/tools_authoring.go:179-207`), reuse the existing constitution friendly-YAML pipeline (`internal/constitution/load/load.go:19-32`), add a parallel funnel parser (new `internal/authoring/load`), rewrite skills/prime routing, and gate the phase with an MCP-client-only e2e. Wave ordering is sound; the atomic 06-02+06-04 release constraint is necessary and now documented in `.planning/ROADMAP.md`. Residual risk is operational (merge sequencing) and the dual wire-format contract (`output` = YAML, `exchanges` = JSON), not architectural.

**Overall risk: MEDIUM** — low technical risk on the handler shim; medium execution risk if wave-1 skills ship before wave-2 handlers, or if the YAML/JSON split is taught inconsistently across skills, param docs, and e2e fixtures.

---

## 06-01-PLAN — `internal/authoring/load` (TDD)

### Summary
Well-scoped foundation plan. Correctly mirrors `internal/constitution/load` for the four funnel stage protos, with TDD coverage for nested messages and enum rejection. Package does not exist today (confirmed); proto field names in `authoring.proto` align with the planned snake_case tags (`scope_in`, `chosen_approach`, `verify_criteria`, `depends_on`, etc.).

### Strengths
- **Accurate defect model:** Current MCP handlers use `protojson.Unmarshal` on raw `output` (`tools_authoring.go:178-179`, `207`, `240`, `273`); a typed YAML parser is the right fix.
- **Enum mapping matches proto:** `ScopeSniff` and `DecompositionStrategy` values (`authoring.proto:49-60`, `70-81`) support the planned `medium` / `vertical_slice` / `steel_thread` mappers.
- **Security posture is concrete:** Fixed structs + UNSPECIFIED→error mirrors `constitutionLayerFromString` (`tools_core.go:29-34`) and `ValidateLayer` (`internal/config/config.go:161-164`).
- **Nested-field test requirements are justified:** Shape `approaches[].tradeoffs`, Specify `interfaces[]`, Decompose `slices[].depends_on` are all real proto fields (`authoring.proto:127-143`, `156-161`, `207-217`) easy to drop in a naive mapper.

### Concerns
- **MEDIUM — `task check` in a parser-only plan may surface unrelated failures.** Task 2 runs full `task check`; acceptable but slows iteration if other packages are red.
- **LOW — No round-trip / `ToYAML` helper.** Acceptable for MCP-01 (write-only), but agents doing get→edit→update on funnel stages have no symmetric read format (constitution `get` is also protojson — see 06-04).

### Suggestions
- Add one table-driven case for **empty/minimal YAML** per stage (only required fields) alongside the fully-populated nested fixtures — e2e will use minimal payloads (`e2e/api/helpers_test.go:150-154` uses a single slice for decompose).
- Consider exporting enum mappers only if 06-04 tests need them directly; otherwise keep them unexported per constitution/load style.

### Risk Assessment
**LOW** — Isolated new package, no proto changes, clear precedent in `internal/constitution/load/load_test.go:17-35`.

---

## 06-02-PLAN — MCP-first skill rewrite + content gate

### Summary
Addresses the visible #1002 symptom: `specgraph-constitution/SKILL.md` is CLI-first (`specgraph constitution show` / `import` at lines 24-36, 129-141) and `specgraph-authoring/SKILL.md` routes via MCP prompts but never teaches friendly `output` or `exchanges` (lines 47-49 only mention `author` action, no payload examples). The content-reference test and parser-binding gate are the right regression harness.

### Strengths
- **Root cause confirmed in skills:** Constitution skill Step 4 is `specgraph constitution import` (`embedded/specgraph-constitution/SKILL.md:129-136`); authoring skill lacks write-format teaching (`embedded/specgraph-authoring/SKILL.md:36-49`).
- **`exchanges` gap correctly identified:** Server enforces ≥1 exchange for shape/specify/decompose (`authoring_handler.go:137-138`, `228`, `347`; `validate.go:43-44`) while MCP tool only documents JSON generically (`tools_authoring.go:135`).
- **Constitution parser-binding is viable now:** `load.FromYAML` already accepts `layer: project` (`load_test.go:17-29`) and `type: adr` references (`config_test.go:166`).
- **Atomic-release invariant is real:** Skills teaching friendly YAML before handler changes would reproduce failures at `tools_authoring.go:178-179`; ROADMAP now records the constraint.
- **Pragmatic wave-1 compromise:** Snake_case key guard without importing `internal/authoring/load` preserves parallel wave-1 execution; full round-trip deferred to 06-05 is reasonable.

### Concerns
- **HIGH — Merge-window dependency on 06-04 (mitigated in docs, not in git).** If 06-02 merges alone, live agents following rewritten skills still hit protojson handlers. Plans document this; enforcement is process-only.
- **MEDIUM — Wave-1 blind spot for authoring YAML structure.** CamelCase guard catches `chosenApproach` vs `chosen_approach` (current tests use camelCase: `tools_authoring_test.go:238`) but not wrong nesting or missing `approaches` until 06-05. Acceptable only if no partial release occurs.
- **MEDIUM — Dual wire-format complexity.** `output` = YAML, `exchanges` = JSON array (`tools_authoring.go:76-77` uses protojson wrapper). Skills, 06-04 param docs, and 06-05 fixtures must stay aligned or agents fix one failure and hit the next.
- **LOW — `exchanges` teaching uses proto field names in JSON.** Consistent with `parseOptionalExchanges` but unlike friendly YAML for `output`; document prominently in skills to avoid agents YAML-wrapping exchanges.

### Suggestions
- In `skill_mcp_reference_test.go`, name the test `TestSkillMCPReference` (or subtests containing `MCPReference`) so `-run MCPReference` in Task 2 verify matches reliably.
- When extracting the constitution YAML block, target the schema block with `layer: "project"` (`embedded/specgraph-constitution/SKILL.md:51-53`) — not an example with invalid placeholder structure.
- Add a substring guard that **CLI primary paths appear only after** the "Requires local CLI" header (not just that the header exists).

### Risk Assessment
**MEDIUM** — Content-only changes are low-risk technically; merge sequencing and dual-format teaching are the main hazards.

---

## 06-03-PLAN — Prime entry-point routing

### Summary
Minimal, accurate plan for D-10. All three CLI misroutes exist today: project empty-state (`prime.go:210`), spec empty-state (`prime.go:326`), and constitution MCP resource (`resources.go:40`). Stale skill count in tests (`prime_test.go:55`, `243` says 6; live catalog has 7 in `embedded_test.go:14-22`; composer sets `len(metas)` at `composer.go:108`) is correctly flagged.

### Strengths
- **Claims verified line-by-line:** `TestRenderProjectMarkdown_EmptyConstitution` asserts CLI text (`prime_test.go:63`); `resources_test.go:170,187` assert `specgraph constitution set`.
- **Golden test trap identified:** `expectedProjectMatchLegacy` embeds `## Skills` verbatim (`prime_test.go:53-55`); `writeSkills` changes require golden update — plan explicitly requires this.
- **SpecView gap closed:** Spark-first agents hit `writeSpecConstitution` empty branch (`prime.go:325-326`), not just project prime.
- **No false prime bug fix:** DB-backed sections fail with sanitized `internal error` while embedded skills succeed — environmental, not composer logic.

### Concerns
- **LOW — Routing line only renders when `SkillsCount > 0`:** `writeSkills` returns early on `count <= 0` (`prime.go:307-308`). Normal path has 7 embedded skills; failure mode is skills list error (composer errors at `composer.go:104-107`), not silent omission on happy path.

### Suggestions
- Keep empty-state hint wording **identical** across `writeProjectConstitution`, `writeSpecConstitution`, and `constitutionEmptyResource` — plan says this; worth enforcing in review.
- Include `specgraph_skills_list` in the routing sentence (not only `get`) since prime already mentions list/search/get (`prime.go:311-313`).

### Risk Assessment
**LOW** — Small string changes with explicit test updates; main failure mode is missing golden/`resources_test` sync (now in scope).

---

## 06-04-PLAN — MCP write-input handler shim

### Summary
Correctly targets the two defect sites and reuses existing pipelines. Depends on 06-01 only (appropriate). Repo-wide audit claim holds: e2e has no MCP `CallTool` for `author`/`constitution` with protojson; only unit tests do (`tools_core_test.go:83`, `tools_authoring_test.go:238,385`).

### Strengths
- **Precise handler mapping:** `handleUpdate` protojson (`tools_core.go:110-112`) → `load.FromYAML` + `load.ToProto` (`load.go:19-37`) is a proven path.
- **Get/update asymmetry acknowledged:** `handleGet` returns protojson via `jsonResult` (`tools_core.go:102`, `helpers.go:80-82`); deferring friendly `get` is consistent with MCP-01 write-focused criteria, though it limits edit workflows.
- **Mock vs real validation split is accurate:** `mockAuthoringService` (`testhelpers_test.go:309-367`) does not run `ValidateExchanges`; semantic exchange rejection belongs in 06-05 e2e — correctly assigned.
- **Malformed exchanges test at MCP boundary:** `parseOptionalExchanges` (`tools_authoring.go:70-79`) is the right place for syntax errors; sequence invariants are in `validate.go:73-78`.

### Concerns
- **HIGH — Same atomic-release dependency as 06-02** (paired mitigation).
- **MEDIUM — `constitution` `get` still returns protojson enum names.** An agent following get→modify→update without the skill template may still fail on `CONSTITUTION_LAYER_PROJECT`. Plan defers this; acceptable for fresh-init MCP-only path using skill YAML, but not a complete round-trip story.
- **LOW — No protojson back-compat at write boundary.** Intentional per D-01; repo audit supports it today. External MCP callers outside the repo are out of scope.
- **LOW — Error messages currently leak parser internals** (`tools_core.go:112`: `invalid constitution JSON: %v`). Plan requires sanitization — implement consistently across all four stage handlers.

### Suggestions
- Re-run `rg protojson internal/mcp` at execution time (plan requires this) and include `tools_spec.go` / `tools_authoring.go` conversation paths that still legitimately use protojson for exchanges/findings.
- In `handleUpdate`, validate layer after `FromYAML` if layer is empty — `load.FromYAML` allows empty layer (`load_test.go:43-48`); MCP update may need explicit layer required for project bootstrap.
- Align tool `Description` wording with skill examples verbatim (same `layer: project` sample).

### Risk Assessment
**LOW–MEDIUM** — Small, localized handler changes; medium if atomic release with 06-02 is violated.

---

## 06-05-PLAN — MCP-only e2e gate

### Summary
Strong phase gate aligned with D-08. Reuses proven harness (`skills_test.go:skillsMCPClient`), addresses real flake risk from shared DB + `constitution_test.go` seeding (`api_suite_test.go:47-48`, `constitution_test.go:40-41`), and exercises server paths mocks cannot cover.

### Strengths
- **Harness exists and is correct:** In-process MCP server pattern in `skills_test.go` avoids ConnectRPC clients — satisfies "no CLI" simulation.
- **Ordered + `BeforeAll(ClearAll)` fixes real ordering bug:** Alphabetical `constitution_test.go` before `mcp_only_authoring_test.go` would falsify empty-state prime without per-describe reset.
- **Pre-flight negative tests target real server code:** Missing exchanges → `ValidateExchanges` (`validate.go:43-44`); missing `sequence` → `validate.go:74-75`; approve ACCEPT path does not require exchanges (`authoring_handler.go:487-489` only on REJECT).
- **`strategy: single_unit` choice is evidence-based:** Avoids `validateSteelThread` constraints (`authoring_handler.go:975-997`) that `steel_thread` would trigger.
- **Skill-count fix routed correctly:** `skills_test.go:106-121` omits `specgraph-constitution` while `embedded_test.go:17` includes it.
- **Label-filter convention correct:** Repo uses `Label("auth")` (`auth_test.go:89`), not `-run` on Describe titles.

### Concerns
- **MEDIUM — E2e fixture minimality vs server field validation.** Shape e2e needs `chosen_approach` and enough fields to pass handler validations (`authoring_handler.go:141-161`); plan should ensure fixtures include at least one `approaches[]` entry matching `chosen_approach` (helpers use this pattern at `helpers_test.go:114-115`).
- **LOW — `ClearAll` in `BeforeAll` may slow the Ordered spec** if many `It` blocks share one DB — acceptable for gate test.
- **LOW — Prime smoke asserts routing substring from 06-03** — coupling is intentional but requires stable hint text.

### Suggestions
- Copy shape/specify/decompose **minimal but valid** fixtures from `helpers_test.go:111-158` into canonical constants — already referenced; reduces fixture drift.
- After approve, assert spec stage via MCP `graph_query` or a read tool if available — not only tool result text — to strengthen criterion #2 proof.
- Run full `go test -tags e2e ./e2e/api/...` after skill-count change to ensure no other tests assume six skills.

### Risk Assessment
**LOW** — E2e adds confidence without architectural change; main risk is under-specified YAML fixtures causing flaky failures.

---

## Cross-Cutting Findings

| Severity | Finding | Evidence |
|----------|---------|----------|
| **HIGH** | 06-02 and 06-04 must ship atomically | `tools_authoring.go:178-179`; skills would teach YAML handlers don't parse |
| **MEDIUM** | Dual wire-format (`output` YAML + `exchanges` JSON) must be taught consistently | `tools_authoring.go:134-135`, `validate.go:43-44` |
| **MEDIUM** | Constitution `get` remains protojson while `update` becomes YAML | `tools_core.go:102` vs planned `handleUpdate` change |
| **LOW** | Stale "6 skills" assertions in unit + e2e tests | `prime_test.go:55,243`; `skills_test.go:106` vs `embedded_test.go:14-22` |
| **LOW** | `TestContentProtoDrift` scans authoring content, not SKILL.md | `drift_test.go:15-24` — new `skill_mcp_reference_test.go` correctly fills that gap for skills |

---

## Phase Goal Coverage (MCP-01 / four success criteria)

| Criterion | Plans | Verdict |
|-----------|-------|---------|
| #1 Skills describe MCP round-trip | 06-02, 06-04 param docs | Covered if atomic release + exchanges taught |
| #2 Full funnel MCP-only | 06-02, 06-04, 06-05 | Covered by e2e gate |
| #3 Constitution approved MCP-only | 06-02, 06-04, 06-05 | Covered |
| #4 skills_get/search reference MCP path | 06-02 content gate | Covered |
| Prime reliability / empty-state routing | 06-03, 06-05 smoke | Covered |

---

## Final Risk Assessment

**MEDIUM overall.**

**Justification:** The technical approach is sound — handler-layer shims, no proto migration, reuse of `constitution/load`, and a real MCP-only e2e harness. Evidence in the repo supports nearly every plan claim. Risk concentrates in **release sequencing** (06-02 before 06-04 reproduces #1002), **teaching consistency** across YAML `output` and JSON `exchanges`, and **test hygiene** (golden prime layout, `resources_test.go`, skill-count drift). With atomic 06-02+06-04 delivery and the documented dual-format contract, these plans should satisfy MCP-01 and the D-08 automated gate.

---

## Consensus Summary

Only one reviewer (Cursor) was invoked for this run, so there is no cross-reviewer consensus to synthesize. Cursor verified every plan claim against the live repo (`file:line` evidence throughout) and returned an overall **MEDIUM** risk verdict driven by execution/process risk, not architecture.

### Top Concerns (single-reviewer, evidence-grounded)

- **HIGH** — 06-02 (skills teach friendly YAML) and 06-04 (handlers parse it) must ship in one merge window; otherwise a live MCP-only agent hits the still-protojson handlers (`internal/mcp/tools_authoring.go:178-179`) and reproduces #1002. Documented in ROADMAP but process-enforced only.
- **MEDIUM** — Dual wire-format contract (`output` = friendly YAML, `exchanges` = JSON array) must be taught consistently across skills, 06-04 param docs, and 06-05 fixtures, or agents fix one failure and hit the next.
- **MEDIUM** — Constitution `get` stays protojson while `update` becomes YAML, so get→edit→update round-trips remain broken for agents not using the skill template (acceptable for the fresh-init write-only MCP-01 path, but not a full round-trip story).
- **LOW** — Stale "6 skills" assertions in unit + e2e tests (`prime_test.go:55,243`, `skills_test.go:106` vs `embedded_test.go:14-22`); 06-03/06-05 fold the fix in.

### Verdict

Technical approach is sound (handler-layer shims, no proto migration, reuse of `internal/constitution/load`, real MCP-only e2e gate). With atomic 06-02+06-04 delivery and a consistently-taught dual-format contract, the plans should satisfy MCP-01 and the D-08 automated gate.
