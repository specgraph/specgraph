---
phase: 6
reviewers: [cursor]
reviewed_at: 2026-07-14T15:10:18Z
plans_reviewed: [06-01-PLAN.md,06-02-PLAN.md,06-03-PLAN.md,06-04-PLAN.md,06-05-PLAN.md]
review_pass: 2 (post-revision)
---

# Cross-AI Plan Review — Phase 6 (Pass 2, post-revision)

## Cursor Review

# Phase 6 Plan Review — MCP Authoring Self-Teaching Path (Post-Revision)

Verified against the live repo. These plans were revised after a prior Cursor review; this pass checks whether those revisions are sound and whether new gaps were introduced.

---

## Overall Assessment

The revised plans remain a coherent, evidence-grounded fix for #1002. Prior-review findings are largely incorporated well: atomic 06-02+06-04 release constraint (now `must_have` + `.planning/ROADMAP.md:61`), `resources_test.go` lockstep updates (`internal/mcp/resources_test.go:170,187`), minimal per-stage fixtures in 06-01, dual wire-format contract (`output` YAML / `exchanges` JSON), deterministic e2e DB isolation, explicit-layer guard, error sanitization, and `spec action:get` post-approve assertion. Root-cause claims still match the code: protojson at the MCP write boundary (`internal/mcp/tools_core.go:111-112`, `internal/mcp/tools_authoring.go:178-179`), CLI-first skills (`internal/mcp/skills/embedded/specgraph-constitution/SKILL.md:24-25,129-136`), and reusable constitution parsing (`internal/constitution/load/load.go:19-32`).

The revisions are sound. No new architectural gaps were introduced; residual issues are mostly imprecise citations, optional hardening, and process-only enforcement of the atomic release.

**Overall risk: MEDIUM** — low technical risk on handler shims; medium execution risk if 06-02 ships without 06-04, or if the YAML/JSON split is taught inconsistently.

---

## Revision Soundness (Prior Review → Current Plans)

| Prior finding | Revision status | Verdict |
|---------------|-----------------|--------|
| Atomic 06-02 + 06-04 merge | Elevated to HIGH `must_have`; recorded in ROADMAP | Sound — still process-enforced only |
| `resources_test.go` drift on empty-state change | Explicitly in 06-03 scope + acceptance criteria | Sound |
| Golden `expectedProjectMatchLegacy` + skill count 6→7 | Required in 06-03 Task 1 | Sound — evidence: `prime_test.go:55,243` vs `embedded_test.go:14-22` |
| `exchanges` teaching gap | 06-02 gate + 06-04 param docs + 06-05 pre-flight `It`s | Sound — server requires ≥1 for shape/specify/decompose (`authoring_handler.go:137-138`; `validate.go:43-44`) |
| Minimal e2e fixtures / `single_unit` decompose | 06-01 minimal cases + 06-05 `strategy: single_unit` | Sound — avoids `validateSteelThread` (`authoring_handler.go:975-997`) |
| `TestSkillMCPReference` naming, CLI-after-appendix guard | In 06-02 Task 1 | Sound |
| Empty-layer `handleUpdate` guard | In 06-04 Task 1 (`load_test.go:43-48` allows empty layer) | Sound |
| Error message sanitization | In 06-04 (replaces `tools_core.go:112` leak) | Sound |
| `skills_test.go` seven-skill fix routed to 06-05 | Documented routing decision | Sound — e2e-only assertion (`skills_test.go:106-121`) |
| Authoring YAML round-trip deferred from 06-02 | Explicit `AUTHORING PARSER-BINDING SCOPE DECISION` | Sound tradeoff for wave-1 parallelism |

---

## 06-01-PLAN — `internal/authoring/load` (TDD)

### Summary
Foundation plan is well-scoped. `internal/authoring/load` does not exist yet; proto field names (`authoring.proto:97-217`) align with planned snake_case tags. TDD coverage for nested messages, multi-token strategy enums, invalid-enum rejection, and minimal fixtures (post-revision) matches what 06-05 will submit.

### Strengths
- Mirrors proven `internal/constitution/load` pattern (`load.go:4-8,19-32`).
- Enum values in proto support mappers (`authoring.proto:49-60`, `70-81`).
- Post-revision minimal-fixture cases close the gap between unit parser tests and e2e payloads (`helpers_test.go:150-154`).

### Concerns
- **LOW** — `task check` in Task 2 may surface unrelated failures; acceptable for phase gate consistency.
- **LOW** — No friendly read format for funnel stages (write-only); consistent with deferred constitution `get` protojson (`tools_core.go:102`).

### Suggestions
- Keep enum mappers unexported unless 06-04 tests need them directly.
- Assert `kill_test` and `questions` round-trip in at least one spark fixture (proto fields at `authoring.proto:103-107`).

### Risk Assessment
**LOW**

---

## 06-02-PLAN — MCP-first skill rewrite + content gate

### Summary
Correctly targets the visible #1002 symptom. Constitution skill is CLI-first today; authoring skill routes MCP but does not teach `output` or `exchanges` (`specgraph-authoring/SKILL.md:47-49`). Post-revision content gate, parser-binding, exchanges/sequence substring checks, and CLI-ordering guard are appropriate.

### Strengths
- Constitution `load.FromYAML` binding is viable (`load_test.go:17-29`; `config_test.go:166` for `type: adr`).
- `author_start_stage` preservation matches live tool (`tools_authoring.go:29`).
- Atomic-release invariant correctly tied to `tools_authoring.go:178-179`.
- Scope decision not to import `internal/authoring/load` in wave 1 is explicit and defensible.

### Concerns
- **HIGH** — Atomic release with 06-04 is documented but not mechanically enforced (unchanged from prior review).
- **MEDIUM** — Dual wire-format (`output` YAML, `exchanges` JSON via `parseOptionalExchanges` at `tools_authoring.go:70-79`) must stay aligned across skills, 06-04 param docs, and 06-05 fixtures.
- **LOW** — Wave-1 snake_case guard does not validate nested structure; full round-trip waits until 06-05 (accepted).

### Suggestions
- In skills, state explicitly that **spark exchanges are optional** (server only validates if present: `authoring_handler.go:57-64`) while shape/specify/decompose require them.
- Promote dual-format contract to a single fenced example block per post-spark stage in `specgraph-authoring`.

### Risk Assessment
**MEDIUM**

---

## 06-03-PLAN — Prime entry-point routing

### Summary
Minimal, accurate D-10 plan. All three CLI misroutes verified: `internal/render/prime.go:210,326`, `internal/mcp/resources.go:40`. Post-revision inclusion of `resources_test.go` and skill-count correction prevents `task check` surprise failures.

### Strengths
- Golden trap correctly identified (`prime_test.go:53-55` embeds `## Skills` verbatim).
- SpecView empty-state gap closed (`prime.go:325-326`).
- `composer.go:108` sets live `SkillsCount = len(metas)` (7), matching `embedded_test.go:14-22`.

### Concerns
- **LOW** — Acceptance criteria say empty-state wording should be "ideally a shared const" but do not require it; drift risk across three surfaces if executor uses copy-paste strings.
- **LOW** — `writeSkills` no-ops when `count <= 0` (`prime.go:307-308`); not an issue on the happy path (7 embedded skills).

### Suggestions
- **Require** a shared `const constitutionEmptyHint` used by `writeProjectConstitution`, `writeSpecConstitution`, and `constitutionEmptyResource` — do not leave as "ideally."
- Post-revision routing sentence naming `specgraph_skills_list` is correct (`prime.go:311-313` already mentions list/search/get).

### Risk Assessment
**LOW**

---

## 06-04-PLAN — MCP write-input handler shim

### Summary
Correctly targets both defect sites. Reuses `constitution/load`; consumes 06-01 funnel parsers. Post-revision explicit-layer guard, error sanitization, malformed-`exchanges` negative test, dual wire-format contract, and protojson audit requirement are all sound.

### Strengths
- Handler change is small and proven (`handleUpdate` at `tools_core.go:110-112` → `load.FromYAML` + `load.ToProto`).
- Mock vs real validation split is accurate (`mockAuthoringService` does not run `ValidateExchanges`; semantic rejection belongs in 06-05).
- Deferred friendly `get` and no protojson write back-compat are explicitly documented with rationale.

### Concerns
- **HIGH** — Paired atomic-release dependency on 06-02 (mitigated in docs only).
- **MEDIUM** — Constitution `get` still returns protojson (`tools_core.go:102`); get→edit→update round-trips remain broken for agents not using skill templates (acceptable for fresh-init MCP-01 write path).
- **LOW** — MCP layer does not pre-reject missing `exchanges` for shape/specify/decompose (`parseOptionalExchanges` returns `nil,nil` at `tools_authoring.go:72-73`); server rejects later. Teaching + 06-05 e2e cover this; optional MCP-level early check would improve error clarity.

### Suggestions
- Re-run `rg protojson internal/mcp` at execution (plan requires); expect legitimate retention in `tools_spec.go:58-61`, exchanges/findings paths, and `jsonResult` helpers.
- Align sanitized error strings with skill wording ("invalid constitution input" / "invalid shape output").

### Risk Assessment
**LOW–MEDIUM**

---

## 06-05-PLAN — MCP-only e2e gate

### Summary
Strong D-08 gate. Post-revision additions (`BeforeAll(ClearAll)`, two pre-flight exchange `It`s, `spec action:get` assertion, canonical fixtures, `Label("MCPOnly")`, seven-skill fix) address prior review findings well. Harness pattern in `skills_test.go` is the right "no ConnectRPC client" simulation.

### Strengths
- Flake root cause is real: `api_suite_test.go:47-48` clears once; `constitution_test.go:40+` seeds constitution alphabetically before `mcp_only_authoring_test.go`.
- Pre-flight tests target real server code (`validate.go:43-44`, `74-75`); approve ACCEPT path does not require exchanges (`authoring_handler.go:487-489` is REJECT-only).
- `spec action:get` read surface exists (`tools_spec.go:39-46`).
- Spark without exchanges is valid (`authoring_handler.go:57-64`).

### Concerns
- **LOW — Incorrect line citation (new in revision):** Plan cites `authoring_handler.go:141-161` for `chosen_approach` ↔ `approaches[]` matching, but those lines validate string-slice sizes and approach count limits, not name equality. No server rule enforces that `chosen_approach` matches an `approaches[].name`; the fixture requirement still mirrors good practice from `helpers_test.go:114-115` but the cited mechanism is wrong.
- **LOW** — Prime smoke couples to 06-03 hint substring stability.
- **LOW** — `ClearAll` per Ordered Describe adds DB reset cost; acceptable for a gate spec.

### Suggestions
- Fix the citation to `helpers_test.go:114-115` (fixture convention) rather than implying server validation at `141-161`.
- Consider a third pre-flight `It` for spark with **invalid** exchanges (optional) to exercise conditional spark validation (`authoring_handler.go:60-63`).

### Risk Assessment
**LOW**

---

## Cross-Cutting Findings

| Severity | Finding | Evidence |
|----------|---------|----------|
| **HIGH** | 06-02 and 06-04 must ship atomically | `tools_authoring.go:178-179`; ROADMAP records constraint |
| **MEDIUM** | Dual wire-format must be taught consistently | `tools_authoring.go:134-135`; `validate.go:43-44` |
| **MEDIUM** | Constitution `get` stays protojson while `update` becomes YAML | `tools_core.go:102` vs planned `handleUpdate` |
| **LOW** | `spec` MCP tool still documents protojson stage outputs | `tools_spec.go:58-61` — out of MCP-01 scope but adjacent confusion surface |
| **LOW** | Stale "6 skills" in tests | `prime_test.go:55,243`; `skills_test.go:106` vs `embedded_test.go:17` |
| **LOW** | 06-05 cites wrong lines for chosen_approach validation | `authoring_handler.go:141-161` is slice/approach count limits, not name match |

---

## Phase Goal Coverage (MCP-01)

| Criterion | Plans | Verdict |
|-----------|-------|---------|
| #1 Skills describe MCP round-trip | 06-02, 06-04 param docs | Covered if atomic release + exchanges taught |
| #2 Full funnel MCP-only | 06-02, 06-04, 06-05 | Covered by e2e gate |
| #3 Constitution via MCP only | 06-02, 06-04, 06-05 | Covered |
| #4 skills_get/search reference MCP path | 06-02 content gate | Covered |
| Prime reliability / empty-state routing | 06-03, 06-05 smoke | Covered |

---

## Final Risk Assessment

**MEDIUM overall.**

**Justification:** Revisions from the prior Cursor review are sound and close most identified gaps without introducing new architectural problems. The technical approach remains correct: handler-layer friendly-YAML shims, no proto migration, reuse of `internal/constitution/load`, parallel `internal/authoring/load`, MCP-first skills/prime routing, and a real MCP-client-only e2e gate. Remaining risk is concentrated in **release sequencing** (06-02 before 06-04 reproduces #1002), **dual-format teaching consistency**, and **minor plan imprecision** (06-05 line citation). With atomic 06-02+06-04 delivery and `task pr-prep` green before merge, these plans should satisfy MCP-01 and D-08.

---

## Consensus Summary

Single reviewer (Cursor), second pass against the revised plans. Cursor verified the plan claims against the live repo (`file:line` evidence throughout) and confirmed the prior-review revisions are **sound with no new architectural gaps**. Overall risk: **MEDIUM**, driven by release sequencing, not architecture.

### Confirmed Resolved (prior-review findings)

- Atomic 06-02+06-04 coupling elevated to a HIGH `must_have` in both plans + ROADMAP note.
- `resources_test.go` / golden `prime_test.go` drift + skill-count 6→7 now in 06-03 scope.
- `exchanges` teaching gap closed (06-02 gate + 06-04 param docs + 06-05 pre-flight tests).
- Minimal e2e fixtures (06-01), explicit-layer guard + error sanitization (06-04), `spec action:get` post-approve assertion (06-05).

### Remaining Concerns (unchanged risk profile)

- **HIGH** — 06-02↔06-04 atomic release is process-enforced only (no automated gate). Do not merge 06-02 to a live deploy without 06-04 in the same window (`internal/mcp/tools_authoring.go:178-179`).
- **MEDIUM** — Dual wire-format (`output`=YAML, `exchanges`=JSON) must stay aligned across skills, 06-04 param docs, and 06-05 fixtures.
- **MEDIUM** — Constitution `get` stays protojson while `update` becomes YAML (`tools_core.go:102`); get→edit→update round-trip gap remains (accepted for the write-only MCP-01 path).

### New Minor Findings (this pass)

- **LOW (06-05)** — Wrong line citation: plan cites `authoring_handler.go:141-161` for `chosen_approach`↔`approaches[]` name-matching, but those lines validate slice sizes / approach-count limits, not name equality. No server rule enforces the name match. Fix the citation to `helpers_test.go:114-115` (fixture convention) and drop the implied server-validation claim.
- **LOW (06-03)** — Acceptance criteria say the empty-state hint should be "ideally a shared const" but don't require it. Recommend **requiring** a shared `const constitutionEmptyHint` across `writeProjectConstitution`/`writeSpecConstitution`/`constitutionEmptyResource` to prevent copy-paste drift.
- **LOW (06-02)** — Skills should state explicitly that **spark exchanges are optional** (server only validates when present: `authoring_handler.go:57-64`) while shape/specify/decompose require ≥1.
- **LOW (06-04)** — Optional: add an MCP-level early check for missing `exchanges` on shape/specify/decompose (`parseOptionalExchanges` returns `nil,nil` at `tools_authoring.go:72-73`; server rejects later) for clearer errors.
- **LOW (cross-cutting)** — `spec` MCP tool still documents protojson stage outputs (`tools_spec.go:58-61`) — out of MCP-01 scope but an adjacent confusion surface.

### Verdict

Revisions are sound; no new architectural problems. With atomic 06-02+06-04 delivery and `task pr-prep` green before merge, the plans should satisfy MCP-01 and the D-08 gate. The new LOW findings are optional polish — the 06-05 citation fix is the only concrete correctness nit worth applying.
