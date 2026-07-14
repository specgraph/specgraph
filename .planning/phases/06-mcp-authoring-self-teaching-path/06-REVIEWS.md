---
phase: 6
reviewers: [cursor]
reviewed_at: 2026-07-14T14:06:58Z
review_pass: 2
plans_reviewed: [06-01-PLAN.md, 06-02-PLAN.md, 06-03-PLAN.md, 06-04-PLAN.md, 06-05-PLAN.md]
note: "Second review pass, run after the --reviews replan (commit b9f566c5) that incorporated pass-1 findings. Confirms all four pass-1 findings landed; surfaces new execution-fidelity concerns concentrated in 06-05."
---

# Cross-AI Plan Review — Phase 6 (Pass 2)

## Cursor Review

# Phase 6 Plan Review (Second Pass): MCP Authoring Self-Teaching Path

**Reviewed against:** live repo at `/Volumes/Code/github.com/specgraph`  
**Plans:** 06-01 through 06-05 (post-revision)  
**Requirement:** MCP-01 (#1002)  
**Method:** Plan claims verified against source (`internal/mcp`, `internal/render`, `internal/authoring`, `internal/constitution/load`, `e2e/api`, embedded skills)

---

## Prior Review Fix Verification

| # | Original finding | Incorporated? | Evidence in revised plans |
|---|------------------|---------------|---------------------------|
| 1 | **`exchanges` teaching gap** — post-spark stages require `ConversationExchanges` server-side (`internal/authoring/validate.go:43-44`, enforced at `internal/server/authoring_handler.go:136-138`) while MCP param remains protojson (`internal/mcp/tools_authoring.go:70-78`, `:135`) | **Yes** | 06-02 Task 1(c) exchanges/sequence gate; Task 2 explicit JSON examples; 06-04 `exchanges` param doc rewrite + malformed-JSON negative test; 06-05 canonical fixtures + pre-flight `It` for missing exchanges |
| 2 | **06-03 golden-test error** — `expectedProjectMatchLegacy` embeds `## Skills` verbatim (`internal/render/prime_test.go:53-55`, asserted at `:67-71`) | **Yes** | 06-03 `must_haves`, Task 1 `read_first`, action, and acceptance criteria all require updating `expectedProjectMatchLegacy` in lockstep with `writeSkills` |
| 3 | **06-01 nested-message coverage underspecified** | **Yes** | 06-01 `must_haves` truths, Task 1 behavior blocks, and acceptance criteria require full nested fixtures per stage plus `vertical_slice`/`layer_cake`/`steel_thread` enum cases |
| 4 | **`skills_test.go` omits `specgraph-constitution`** (`e2e/api/skills_test.go:106-121`) | **Yes (routed)** | Fix assigned to 06-05 Task 1 with explicit six→seven update; 06-02 documents routing rationale (e2e-only assertion belongs in e2e plan) |

All four prior findings are addressed in the revised plan text. The remaining work is execution fidelity and a few gaps the revision did not close.

---

## Overall Phase Assessment

The five-plan wave structure remains sound and the revision materially closes the first-pass holes. Root-cause analysis still matches the repo: constitution `update` and all four funnel `output` handlers use `protojson.Unmarshal` today (`internal/mcp/tools_core.go:111-112`, `internal/mcp/tools_authoring.go:178-179` and analogous lines for shape/specify/decompose), while `specgraph-constitution` still teaches CLI import (`internal/mcp/skills/embedded/specgraph-constitution/SKILL.md:24-26`, `129-135`) and prime empty-states still emit `specgraph constitution set` (`internal/render/prime.go:210`, `:326`; `internal/mcp/resources.go:40`).

The revised plans should deliver constitution MCP-only authoring and a full funnel path **if** the e2e gate handles shared-database state and the Ginkgo filter command is corrected. Two new concerns (e2e ordering/pollution, wrong `-run` incantation) could block or silently weaken the phase gate despite the prior fixes landing in plan text.

**Overall phase risk: MEDIUM** — architecture and revision quality are strong; execution hazards concentrate in 06-05.

---

## Plan 06-01: `internal/authoring/load` (TDD)

### Summary
Well-scoped foundation package mirroring `internal/constitution/load/load.go:22-33`. Correctly targets the verified gap (`internal/authoring/load/` does not exist). Revision adequately addresses nested-message and multi-token enum coverage that the first pass flagged.

### Strengths
- **Correct layering** — parser at MCP boundary, no proto regen; matches proven constitution pipeline.
- **Security-aligned enum handling** — UNSPECIFIED→error pattern matches `constitutionLayerFromString` at `internal/mcp/tools_core.go:29-34`.
- **Revision closes nested coverage gap** — explicit requirements for `approaches[].tradeoffs`, `decisions[]`, `verify_criteria[]`, `touches[]`, `slices[].depends_on` align with `proto/specgraph/v1/authoring.proto` message shapes.
- **Multi-token enum cases** — `vertical_slice`, `layer_cake`, `steel_thread` are now mandatory positive tests.

### Concerns
- **LOW — No `InterfaceSection` nested assertion called out separately.** Specify stage lists `interfaces (name/body)` in behavior but acceptance criteria emphasize `verify_criteria` and `touches` only; a parser that drops `interfaces` could slip through.
- **LOW — RED verify uses brittle `grep` on test output** — acceptable for automation but may false-positive on unrelated "FAIL" strings.

### Suggestions
- Add one positive fixture asserting `interfaces[].name` and `interfaces[].body` round-trip for Specify.
- Export thin `ToProto` helpers per stage (mirror constitution `ToProto`) to keep 06-04 handlers minimal.

### Risk Assessment
**LOW** — Isolated package, clear template, no runtime dependency until 06-04.

---

## Plan 06-02: MCP-First Skill Rewrite + Content Gate

### Summary
Strong content plan. Revision correctly treats `exchanges` as a first-class teaching surface while scoping wire format to JSON for this milestone. Parser-binding design preserves wave-1 parallelism.

### Strengths
- **Targets verified #1002 symptom** — constitution skill is CLI-first today (`SKILL.md:22-26`, `129-135`); rewrite to `constitution` `update` is correct.
- **Exchanges gate added** — Task 1(c) substring gate for `exchanges` + `sequence` closes the first-pass HIGH finding.
- **Clever wave-1 decoupling** — authoring parser binding via snake_case key guard avoids compile dependency on 06-01.
- **Constitution binding is executable today** — `internal/constitution/load.FromYAML` already exists and is production-used.
- **D-07 appendix pattern** — uniform `"Requires local CLI"` gate is concrete and testable.

### Concerns
- **MEDIUM — Wave-1 skills/handler skew persists.** Skills can land teaching friendly YAML before 06-04 wires handlers; agents hitting MCP mid-wave still get protojson failures (`tools_authoring.go:178-179`). Acceptable only if waves merge atomically.
- **LOW — Task 2 `read_first` calls specgraph-authoring "already MCP-first."** It routes via `author` / `author_start_stage` (`specgraph-authoring/SKILL.md:38-49`) but teaches neither friendly `output` YAML nor `exchanges`; the label is misleading for executors, not a functional gap.
- **LOW — Authoring parser binding is key-presence only until 06-05.** Semantically empty YAML with correct keys passes 06-02 gate; full round-trip deferred to e2e.

### Suggestions
- Add explicit acceptance criterion preserving `author_start_stage` as documented entry path (already in current skill; cheap regression guard).
- Note in plan metadata that 06-02 + 06-04 should land in the same merge window to avoid interim agent confusion.

### Risk Assessment
**MEDIUM** — Content work is thorough post-revision; sequencing hazard is the main residual risk.

---

## Plan 06-03: Prime Entry-Point Routing

### Summary
Minimal, correct scope. Revision fully fixes the first-pass golden-test error. Targets all three verified CLI hint sites.

### Strengths
- **Golden correction landed** — Task 1 explicitly requires updating `expectedProjectMatchLegacy` (`internal/render/prime_test.go:21-57`) when `writeSkills` changes.
- **SpecView gap closed** — new `TestRenderSpecMarkdown_EmptyConstitution` addresses duplicate CLI hint at `prime.go:326` (project hint at `:210` was the only tested surface today: `prime_test.go:59-65`).
- **D-10 discipline** — routing text only; skills carry depth.

### Concerns
- **MEDIUM — Stale `SkillsCount: 6` in golden fixture while repo serves 7 skills.** `fixtureProjectView()` sets `SkillsCount: 6` (`internal/render/prime_test.go:243`) and `expectedProjectMatchLegacy` says "6 skills" (`:53-55`), but embedded catalog has 7 skills (`internal/mcp/skills/embedded_test.go:14-22`; live composer sets `view.SkillsCount = len(metas)` at `internal/prime/composer.go:108`). Plan 03 updates golden for routing prose but does not mention correcting skill count. Touching the golden without fixing count leaves a pre-existing drift (or forces an incidental fix without guidance).
- **LOW — `constitutionEmptyResource` still has no dedicated unit test** (`internal/mcp/resources.go:36-41`); consistency relies on manual review and 06-05 smoke.

### Suggestions
- When updating `expectedProjectMatchLegacy`, also change `SkillsCount` fixture and literal from 6→7 to match `embedded_test.go`.
- Extract shared empty-state wording constant used by `writeProjectConstitution`, `writeSpecConstitution`, and `constitutionEmptyResource` to prevent drift.
- Add a small `resources_test.go` assertion for `constitutionEmptyResource` MCP wording.

### Risk Assessment
**LOW–MEDIUM** — Small code change; golden update is now correctly specified; skill-count drift is a secondary `task check` surprise.

---

## Plan 06-04: MCP Write-Input Handler Shim

### Summary
Core #1002 fix. Constitution path is low-risk reuse; funnel `output` path correctly depends on 06-01. Revision adds `exchanges` param documentation without expanding wire format scope.

### Strengths
- **Minimal constitution change** — replace `protojson.Unmarshal` at `tools_core.go:111-112` with `load.FromYAML` → `load.ToProto` (~10 lines).
- **Correct wave dependency** — `depends_on: [06-01]` is necessary.
- **Exchanges doc revision** — Task 2 explicitly rewrites `exchanges` `stringProp` with worked example; malformed-JSON negative test at MCP boundary.
- **Honest test boundary note** — plan correctly states mock backend cannot exercise `ValidateExchanges`; defers semantic rejection to 06-05 e2e.

### Concerns
- **MEDIUM — `exchanges` remains protojson on the wire** (`parseOptionalExchanges` at `tools_authoring.go:70-78`). Acceptable for MCP-01 given explicit teaching in 02/04/05, but still the second half of the author tool contract agents must learn.
- **MEDIUM — No protojson fallback.** Plan removes protojson entirely from `output`; fine for MCP-only goal, but any external MCP caller sending camelCase JSON (`tools_authoring_test.go:238` today) must migrate.
- **LOW — `constitution.get` still returns protojson** via `jsonResult` (`tools_core.go:102`). Fresh-init path is unaffected; get→edit→update round-trips still expose proto enums.
- **LOW — Pre-existing `referenceTypeToProto` silent UNSPECIFIED** (`internal/constitution/load/load.go:94-106`) — not introduced by plan but weakens "impossible to get wrong" for reference blocks.

### Suggestions
- Document `constitution.get` protojson read path as known v0.14.0 limitation in 06-02 constitution skill (agents authoring from template, not from get).
- Consider a second 06-05 pre-flight `It` for `exchanges` with missing `sequence` (server rejects at `internal/authoring/validate.go:74-75`) — plan only covers wholly absent exchanges.

### Risk Assessment
**MEDIUM** — Constitution path is proven; funnel `output` path is well-specified; `exchanges` JSON remains a teaching burden but is now planned end-to-end.

---

## Plan 06-05: MCP-Only Authoring E2E Gate

### Summary
Appropriate phase gate (D-08). Revision adds canonical `exchanges` fixtures, pre-flight rejection test, and `skills_test.go` fix. Harness reuse (`skillsMCPClient` at `e2e/api/skills_test.go:35-57`) is sound.

### Strengths
- **Real MCP server path** — in-process `mcp.NewServer` against e2e Postgres, not a stub.
- **Exchanges fixtures specified** — mirrors `e2e/api/helpers_test.go:117-119`, `137-139`, `156-158`.
- **Approve path verified** — MCP `handleApprove` sends slug only (`tools_authoring.go:296-303`); server ACCEPT path does not validate exchanges (`authoring_handler.go:453-485`; REJECT path does at `:488`).
- **Spark without exchanges correct** — server treats spark exchanges as optional (`authoring_handler.go:57-64`).
- **Pre-flight `It`** — missing `exchanges` on shape exercises real `ValidateExchanges` the mock unit tests cannot.

### Concerns
- **HIGH — Empty-state prime assertion likely flaky against shared e2e DB.** Suite clears once in `BeforeSuite` (`e2e/api/api_suite_test.go:47-48`), single project `e2e-test` (`e2e/testutil/server.go:39`). `constitution_test.go` creates a constitution (`:40+`) and does not clear afterward. If `MCPOnly` runs after constitution tests (alphabetical file order: `constitution_test.go` before `mcp_only_authoring_test.go`), step 1's empty-state hint assertion will fail. Plan says "fresh/empty constitution project" but does not require `BeforeAll(ClearAll)` or isolated project scope.
- **MEDIUM — Wrong Ginkgo filter command.** Plan verify uses `go test -tags e2e ./e2e/api/ -run MCPOnly`. Repo pattern for labels is `--ginkgo.label-filter` (see `e2e/api/auth_test.go:89` and `docs/plans/2026-03-18-auth-interceptor-plan.md:1343`). `-run MCPOnly` does not filter on `Label("MCPOnly")` unless the Describe *title* contains that substring; label-only filtering will not work as documented.
- **LOW — Step order within spec is good** (prime read before constitution `update` in the same `It`), but cross-spec pollution remains the issue.
- **LOW — Full suite regression** — acceptance criteria mention full `e2e/api` run; good, but not wired into plan `verify` block (only `-run MCPOnly`).

### Suggestions
- Add `BeforeAll(func() { Expect(serverInfo.Store.ClearAll(ctx)).To(Succeed()) })` to the `MCPOnly` Describe (or `Ordered` + `BeforeAll` ClearAll) so empty-state prime assertion is deterministic.
- Change verify to `go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly` (or name the Describe `"MCPOnly …"` *and* document which mechanism is used).
- Add optional pre-flight `It` for malformed `exchanges` JSON (missing `sequence`) to complement 06-04's syntax-only negative test.

### Risk Assessment
**MEDIUM–HIGH** — Right gate design post-revision, but DB isolation and filter command errors could block or neuter the gate.

---

## Cross-Cutting Findings

### Dependency ordering
```
Wave 1 (parallel): 06-01, 06-02, 06-03
Wave 2: 06-04 → depends on 06-01
Wave 3: 06-05 → depends on 06-02, 06-03, 06-04
```
Ordering is logically sound. Residual hazard: 06-02 landing before 06-04 during development.

### Phase goal coverage (post-revision)

| Success criterion | Status | Notes |
|-------------------|--------|-------|
| #1 Skills describe MCP round-trip | **Yes** | 06-02 + 06-04; `exchanges` explicitly included |
| #2 Full funnel MCP-only | **Yes, with e2e caveats** | 06-05; blocked if DB isolation / Ginkgo filter wrong |
| #3 Constitution MCP-only | **Yes** | 06-02 + 06-04 + 06-05 |
| #4 skills_get/search reference MCP path | **Yes** | 06-02 content gate |
| Prime reliability | **Yes** | 06-03 + 06-05 smoke (if DB empty) |

### Security
Typed structs, enum rejection, `errResult` sanitization — adequate. No new auth surface.

### Performance
Negligible — YAML parse at MCP boundary vs Postgres RPC.

### Scope creep
Appropriate for MCP-01. Seven-skill rewrite is required by D-05.

---

## Priority Fixes Before Execution

1. **06-05 — Add deterministic DB reset** (`ClearAll` in `MCPOnly` Describe `BeforeAll`) so empty-state prime assertion is reliable against `e2e/api` suite ordering.
2. **06-05 — Fix Ginkgo filter command** to `--ginkgo.label-filter=MCPOnly` (or document Describe-title-based `-run` matching explicitly).
3. **06-03 — When touching golden, fix skill count 6→7** in `fixtureProjectView` and `expectedProjectMatchLegacy` to match `embedded_test.go` (7 canonical skills).
4. **Optional — 06-05** add pre-flight `It` for `exchanges` missing `sequence` (semantic validation at `validate.go:74-75`).

---

## Final Risk Assessment

| Plan | Risk |
|------|------|
| 06-01 | LOW |
| 06-02 | MEDIUM |
| 06-03 | LOW–MEDIUM |
| 06-04 | MEDIUM |
| 06-05 | MEDIUM–HIGH |
| **Phase overall** | **MEDIUM** |

**Justification:** The revision successfully incorporates all four first-pass findings with concrete plan text and source-grounded mechanisms. Constitution MCP-only authoring and friendly funnel `output` are well-planned. Residual risk concentrates in 06-05: shared e2e database state can falsify the empty-state prime smoke, and the documented `-run MCPOnly` command does not match the repo's Ginkgo label-filter pattern. Addressing those two items before execution should move phase risk to **LOW–MEDIUM**.

---

## Consensus Summary

Single external reviewer (Cursor), second pass, run against the revised plans (commit `b9f566c5`) with the prior REVIEWS.md supplied as context. Cursor read the live repo and cited `file:line` evidence throughout.

**Headline:** All four pass-1 findings are confirmed correctly incorporated (exchanges teaching, 06-03 golden-test correction, 06-01 nested/enum coverage, skills_test.go skill-list fix). Overall phase risk **MEDIUM**; residual risk concentrates in the 06-05 e2e gate.

### Confirmed Fixed (pass-1 findings)
- **Exchanges teaching gap** → 06-02 (content gate + JSON examples), 06-04 (param-doc rewrite + malformed negative test), 06-05 (canonical fixtures + no-exchanges pre-flight). ✅
- **06-03 golden-test error** → 06-03 now updates `expectedProjectMatchLegacy` in lockstep with `writeSkills`. ✅
- **06-01 nested-message/enum coverage** → full nested fixtures + multi-token enum cases mandated. ✅
- **skills_test.go skill-list omission** → routed to 06-05 (six→seven). ✅

### New Agreed Concerns (highest priority — for a potential second `--reviews` pass)
1. **[HIGH] 06-05 empty-state prime assertion is likely flaky against the shared e2e DB.** The suite clears once in `BeforeSuite` (`e2e/api/api_suite_test.go:47-48`) against a single project `e2e-test` (`e2e/testutil/server.go:39`). `constitution_test.go` creates a constitution and never clears it; if `mcp_only_authoring_test.go` runs after it (alphabetical file order), the "empty-state constitution hint" prime assertion fails. The plan says "fresh/empty constitution project" but does not require a `BeforeAll(ClearAll)` or an isolated project scope. Fix: add `Ordered` + `BeforeAll(func(){ Expect(serverInfo.Store.ClearAll(ctx)).To(Succeed()) })` to the `MCPOnly` Describe, or use a uniquely-named project.
2. **[MEDIUM] 06-05 Ginkgo filter command is wrong.** The plan's verify uses `go test -tags e2e ./e2e/api/ -run MCPOnly`, but `-run` matches Go test / Ginkgo Describe *titles*, not `Label("MCPOnly")`. The repo's label pattern is `--ginkgo.label-filter=MCPOnly` (`e2e/api/auth_test.go:89`). As written, the phase-gate command may silently select nothing (green vacuous pass) or not filter as intended. Fix: use `--ginkgo.label-filter=MCPOnly`, or name the Describe so `-run` matches — and state which.
3. **[MEDIUM] 06-03 stale `SkillsCount: 6` vs 7 served skills.** `fixtureProjectView()` sets `SkillsCount: 6` and `expectedProjectMatchLegacy` says "6 skills" (`internal/render/prime_test.go:243,53-55`), but the embedded catalog serves 7 (`internal/mcp/skills/embedded_test.go:14-22`; composer sets `len(metas)` at `internal/prime/composer.go:108`). Since 06-03 already edits that golden, it should also correct 6→7 to avoid an incidental/surprise `task check` fix without guidance.

### Divergent Views / Lower Priority
- **[LOW] `constitution.get` still returns protojson** (`tools_core.go:102`) — fresh-init authoring unaffected; get→edit→update round-trips still expose proto enums. Reviewer suggests documenting as a known v0.14.0 limitation, not fixing this milestone.
- **[LOW] 06-01 `interfaces[].name/body` (Specify) not called out in acceptance criteria** — a parser dropping `interfaces` could slip through; add one positive fixture.
- **[LOW] Optional extra 06-05 pre-flight** for `exchanges` present-but-missing-`sequence` (`validate.go:74-75`) to complement 06-04's syntax-only negative test.
- **[MEDIUM, known/accepted] Wave-1 skills/handler skew** — 06-02 teaching can land before 06-04 wires handlers; acceptable only if the wave merges atomically. Reviewer suggests a merge-window note.

### Recommended Action
The three top concerns are all concrete and cheap, and two of them (#1 DB isolation, #2 Ginkgo filter) directly affect whether the D-08 phase gate actually gates. Worth a second `/gsd-plan-phase 6 --reviews` pass to fold them into 06-05 (and the 6→7 skill-count fix into 06-03) before execution. None are architectural — the plans remain fundamentally sound and MEDIUM risk overall.
