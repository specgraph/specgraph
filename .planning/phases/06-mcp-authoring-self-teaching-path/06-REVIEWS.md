---
phase: 6
reviewers: [cursor]
reviewed_at: 2026-07-14T14:22:00Z
review_pass: 3
plans_reviewed: [06-01-PLAN.md, 06-02-PLAN.md, 06-03-PLAN.md, 06-04-PLAN.md, 06-05-PLAN.md]
note: "Third review pass (Cursor, --cursor). Source-grounded against the live repo. Confirms pass-1/pass-2 fixes landed and surfaces execution-sequencing risks concentrated in the skills-before-handlers merge window and the 06-03 resources_test.go update."
---

# Cross-AI Plan Review — Phase 6 (Pass 3)

## Cursor Review

# Phase 6: MCP Authoring Self-Teaching Path — Plan Review

## Summary

The five-plan wave structure is sound, the root-cause diagnosis matches the repo, and the chosen fix (friendly YAML at the MCP write boundary, reusing `internal/constitution/load` for constitution and a new `internal/authoring/load` for funnel stages) is the right scope for MCP-01. Pass-2 revisions incorporated real gaps (golden-test updates, e2e DB isolation, `exchanges` teaching, nested parser coverage). The plans should achieve the phase success criteria if wave 2 handler work ships in the same merge window as the wave 1 skill rewrite and if a few test files outside the declared `files_modified` lists are updated.

---

## Strengths

- **Root cause is verified in code.** Constitution `update` unmarshals raw protojson (`internal/mcp/tools_core.go:111-112`); all four funnel handlers do the same (`internal/mcp/tools_authoring.go:178-179`, `:207-208`, `:240-241`, `:273-274`). The served constitution skill still teaches CLI import (`internal/mcp/skills/embedded/specgraph-constitution/SKILL.md:22-26`, `:129-136`), while prime empty-states route to CLI (`internal/render/prime.go:210`, `:326`; `internal/mcp/resources.go:40`).

- **Constitution path reuses proven infrastructure.** `internal/constitution/load/load.go:22-33` already parses friendly YAML (`layer: project`, `type: adr`) into domain/proto — exactly what plan 06-04 wires into `handleUpdate`. No proto regen is required.

- **06-01 is well-scoped TDD.** Mirroring `internal/constitution/load` for `SparkOutput`/`ShapeOutput`/`SpecifyOutput`/`DecomposeOutput` is correct; proto field names are snake_case (`proto/specgraph/v1/authoring.proto:97-218`), so `TestContentProtoDrift` stays compatible. Required nested fixtures (`approaches[].tradeoffs`, `decisions[]`, `interfaces[]`, `slices[].depends_on`) and multi-token strategy enums (`vertical_slice`, `layer_cake`, `steel_thread` at `authoring.proto:49-59`) prevent shallow parsers from passing.

- **06-02 content gates are strong.** Parser-binding via `load.FromYAML` on the constitution YAML block, camelCase rejection on authoring examples, and `exchanges`/`sequence` substring checks close distinct #1002 failure classes. The merge-window note (skills land before handlers) is accurate and important.

- **06-03 correctly identifies brittle tests.** Byte-for-byte golden `expectedProjectMatchLegacy` (`internal/render/prime_test.go:21-57`, asserted at `:71`) and stale skill count (`fixtureProjectView().SkillsCount: 6` at `:243` vs seven embedded skills at `internal/mcp/skills/embedded_test.go:14-22` and live `composer.go:108`) are real `task check` traps; the plan addresses both.

- **06-04 test migration target is correct.** MCP tool tests hard-code protojson today (`internal/mcp/tools_core_test.go:83`, `internal/mcp/tools_authoring_test.go:238`, `:385`). RPC service-client tests in `e2e/api/helpers_test.go` are unaffected (they hit ConnectRPC directly, not MCP tools).

- **06-05 e2e design is rigorous.** `skillsMCPClient` (`e2e/api/skills_test.go:35-57`) is the right harness. `BeforeAll(ClearAll)` fixes a real flake: `api_suite_test.go:47-48` clears once, but `constitution_test.go:40-86` seeds a constitution that would falsify empty-state prime assertions in alphabetically earlier files. `ValidateExchanges` requires ≥1 exchange (`internal/authoring/validate.go:43-44`) and rejects missing `sequence` (`:74-75`); approve ACCEPT does not validate exchanges (`internal/server/authoring_handler.go:453-485` vs reject at `:488`). Pre-flight negative `It`s complement 06-04's mock-only malformed-JSON test.

- **Wave ordering is coherent.** 06-01 (parser) → 06-04 (handlers) → 06-05 (e2e gate); 06-02/06-03 parallel in wave 1 with no file conflicts.

---

## Concerns

| Severity | Finding | Evidence |
|----------|---------|----------|
| **HIGH** | **Skills/handlers split across merges will reproduce #1002.** After 06-02, skills teach friendly YAML `output`; handlers still protojson-unmarshal until 06-04. Any agent using live MCP between those merges hits the same failure. | `tools_authoring.go:178-179`; 06-02 merge-window note |
| **MEDIUM** | **06-03 omits `internal/mcp/resources_test.go` from `files_modified`.** Task 2 updates `constitutionEmptyResource` but tests still assert CLI text (`resources_test.go:170`, `:187`). `go test ./internal/mcp/` will fail until those expectations change. | `resources.go:40`; `resources_test.go:170` |
| **MEDIUM** | **Dual wire format for funnel stages (`output` YAML + `exchanges` JSON).** Server enforces exchanges on shape+ (`authoring_handler.go:137-138`); MCP `parseOptionalExchanges` only syntax-checks JSON (`tools_authoring.go:70-80`). Plans address via teaching/docs/e2e, but this remains a second failure surface for MCP-only agents. | `validate.go:43-44`; `tools_authoring.go:134-135` |
| **MEDIUM** | **06-02 authoring parser-binding is partial until 06-05.** Task 1(b) only checks snake_case keys, not `authoring/load` round-trip, because 06-02 is wave 1. A skill could pass the camelCase gate but teach invalid nested YAML until e2e. | 06-02 plan Task 1(b) |
| **LOW** | **`constitution` `get` still returns protojson.** Agents doing get→edit→update may still see enum names unless they author from the skill template. Research flagged optional friendly `get` output; not in any plan. | `tools_core.go:102` (`jsonResult` of proto) |
| **LOW** | **No protojson back-compat at MCP boundary.** Research suggested optional `{` fallback; 06-04 replaces entirely. Acceptable for MCP-01 scope, but any protojson MCP callers break. | `tools_core.go:111`; plan 06-04 |
| **LOW** | **06-02 Task 2 labels specgraph-authoring "already MCP-first" but it lacks friendly `output`/`exchanges` examples.** Skill routes via `author`/`author_start_stage` (`specgraph-authoring/SKILL.md:38-49`) without write-payload teaching — rewrite scope is larger than the label implies. | `specgraph-authoring/SKILL.md:47-49` |
| **LOW** | **`e2e/api/skills_test.go` "lists six skills" omits `specgraph-constitution`.** Routed to 06-05 (correct), but remains a latent e2e failure until then. | `skills_test.go:106-121` vs `embedded_test.go:17` |

---

## Suggestions

1. **Add `internal/mcp/resources_test.go` to 06-03 `files_modified` and acceptance criteria** — update `:170` and `:187` to assert MCP-first routing text, matching `prime_test.go` changes.

2. **Treat 06-02 + 06-04 as an atomic release** — single PR or stacked merge with no intermediate deploy. Document in phase STATE/ROADMAP, not only as a plan note.

3. **After 06-01 lands, extend 06-02 parser-binding** — import `internal/authoring/load` in `skill_mcp_reference_test.go` and round-trip the authoring YAML example (same pattern as constitution `FromYAML`). Removes the wave-1 blind spot without blocking 06-02 start.

4. **06-04: grep entire repo for MCP tool callers** — plans cite `tools_core_test.go` and `tools_authoring_test.go`; also audit any e2e or integration tests that invoke `constitution`/`author` MCP tools with protojson payloads (research assumption A2).

5. **06-05: use `strategy: single_unit` with minimal slices** — avoids `validateSteelThread` constraints (`authoring_handler.go:975-997`) that could make decompose fixtures unexpectedly strict.

6. **06-02 rewrite: preserve `author_start_stage` as a documented entry path** — tool exists (`tools_authoring.go:29`) and is referenced in the current skill (`specgraph-authoring/SKILL.md:43`); add an explicit acceptance criterion so the rewrite doesn't drop it.

7. **Optional follow-up (out of scope): friendly YAML on `constitution` `get`** — would complete get→edit→update in one format; not blocking fresh-init MCP-only authoring.

---

## Risk Assessment

**Overall: MEDIUM**

**Justification:** Technical approach is low-risk (handler-layer shim, no proto/storage changes, reuse of `constitution/load`). Test coverage is unusually thorough for a planning doc — TDD parser, content gates, MCP-only e2e with real `ValidateExchanges` paths, and prime smoke. Risk concentrates in **execution sequencing**: the skills-before-handlers merge window (HIGH if violated), the undocumented `resources_test.go` update (will block `task check` in 06-03), and the **dual-format funnel contract** (YAML `output` + JSON `exchanges`) which demands consistent teaching across skills, tool param docs, and e2e fixtures. With atomic 06-02/06-04 delivery and the `resources_test.go` fix, the plans should satisfy all four MCP-01 success criteria and the D-08 automated gate.

---

## Consensus Summary

Single reviewer (Cursor) this pass. Highest-priority items to fold into planning:

### Agreed Concerns

- **HIGH — skills-before-handlers merge window.** 06-02 (skills teach friendly YAML) and 06-04 (handlers parse it) must land atomically, or live MCP-only agents reproduce #1002 between merges.
- **MEDIUM — 06-03 `files_modified` gap.** `internal/mcp/resources_test.go` (`:170`, `:187`) still asserts CLI-routing text and will break `task check` unless added to the plan's scope.
- **MEDIUM — dual funnel wire format.** `output` YAML plus `exchanges` JSON is a second failure surface; teaching, tool-param docs, and e2e fixtures must stay consistent.

### Divergent Views

None (single reviewer).
