---
phase: 06-mcp-authoring-self-teaching-path
plan: 02
subsystem: mcp
tags: [skills, mcp, authoring, constitution, content-drift-test, self-teaching]

# Dependency graph
requires:
  - phase: 06-01
    provides: internal/authoring/load friendly-YAML parser (round-tripped end-to-end by 06-05, not imported here)
provides:
  - 7 embedded SKILL.md canonicals rewritten MCP-first (lead with MCP tools/resources, gated CLI appendix)
  - specgraph-constitution + specgraph-authoring teach the friendly-YAML write payload (constitution update, author per-stage output + exchanges)
  - skill_mcp_reference_test.go — content-level + parser-binding gate against MCP-first regression
affects: [06-04, 06-05, mcp-authoring, self-teaching-path]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Content-level assertion over embedded SKILL.md bodies (TestContentProtoDrift-style) guards prose posture"
    - "Parser-binding test: taught constitution YAML round-trips through load.FromYAML; authoring output guarded against camelCase typos"
    - "Uniform gated 'Requires local CLI (source/CLI users only — MCP-only agents skip this)' appendix + CLI-ordering guard"

key-files:
  created:
    - internal/mcp/skills/skill_mcp_reference_test.go
  modified:
    - internal/mcp/skills/embedded/specgraph-constitution/SKILL.md
    - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
    - internal/mcp/skills/embedded/specgraph-graph-query/SKILL.md
    - internal/mcp/skills/embedded/specgraph-analytical-passes/SKILL.md
    - internal/mcp/skills/embedded/specgraph-drift/SKILL.md
    - internal/mcp/skills/embedded/specgraph-conventions/SKILL.md
    - internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md

key-decisions:
  - "Scoped the camelCase field-name guard to extracted ```yaml blocks so prose anti-examples (scopeIn/chosenApproach as 'do not use') don't false-fail the gate."
  - "CLI-ordering guard scans the body AFTER front-matter — the description field legitimately names CLI trigger phrases (e.g. 'running specgraph drift from the CLI')."
  - "MCP-reference regex fixed to `specgraph_` OR `specgraph://` (unambiguous), and each of the 5 aligned skills was given at least one such reference rather than matching bare tool words."
  - "exchanges stays a JSON array this milestone (taught + tested, not converted to YAML) per plan scope; output is friendly snake_case YAML."

patterns-established:
  - "Every served SKILL.md leads with MCP tools/resources; CLI is a single trailing gated appendix an MCP-only agent skips."
  - "Skill YAML examples are bound to the parsers that ingest them, so a field-name typo fails `task check` instead of reproducing #1002."

requirements-completed: [MCP-01]

coverage:
  - id: D1
    description: "7 embedded SKILL.md canonicals rewritten MCP-first with a uniform gated CLI appendix"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/skills/skill_mcp_reference_test.go#TestSkillMCPReference"
        status: pass
      - kind: unit
        ref: "task skills:validate (7 packages, all OK)"
        status: pass
    human_judgment: false
  - id: D2
    description: "specgraph-constitution teaches constitution tool get/update with the inline friendly-YAML write payload; block round-trips through load.FromYAML to the project layer"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/skills/skill_mcp_reference_test.go#TestSkillMCPReference/MCPReference/constitution-parse-binding"
        status: pass
    human_judgment: false
  - id: D3
    description: "specgraph-authoring teaches snake_case friendly-YAML output per stage and the mandatory exchanges JSON array (required shape/specify/decompose, optional spark); guarded against camelCase typos"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "internal/mcp/skills/skill_mcp_reference_test.go#TestSkillMCPReference/MCPReference/authoring-snake-case-guard"
        status: pass
      - kind: unit
        ref: "internal/mcp/skills/skill_mcp_reference_test.go#TestSkillMCPReference/MCPReference/authoring-exchanges-gate"
        status: pass
    human_judgment: false
  - id: D4
    description: "Content-level assertion + full unit suite gate the MCP-first posture against regression"
    requirement: MCP-01
    verification:
      - kind: unit
        ref: "task check (fmt/license/lint/build/unit — PASSED)"
        status: pass
    human_judgment: false

# Metrics
duration: 8min
completed: 2026-07-14
status: complete
---

# Phase 6 Plan 02: MCP-First Skill Rewrite Summary

**All 7 embedded SpecGraph skills rewritten MCP-first — constitution/authoring now teach the friendly-YAML write payload inline — guarded by a content-level + parser-binding test that fails on a field-name typo instead of reproducing #1002.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-07-14T15:48:42Z
- **Completed:** 2026-07-14T15:56:57Z
- **Tasks:** 3
- **Files modified:** 8 (1 created, 7 rewritten)

## Accomplishments

- Added `skill_mcp_reference_test.go` (`TestSkillMCPReference`): enumerates all 7 skills and asserts MCP-tool/resource reference, the gated "Requires local CLI" appendix, a CLI-appendix ordering guard, and preserved front-matter — plus two cross-plan data-contract gates: the constitution write block round-trips through `internal/constitution/load.FromYAML` to `ConstitutionLayerProject`, and the authoring `output` block is guarded against the `chosenApproach` vs `chosen_approach` camelCase typo class (#1002).
- Rewrote **specgraph-constitution** MCP-first: `constitution` tool `get`/`update` lead, the friendly-YAML schema block is the inline `update` payload (not a file to `import`), and `constitution show/import/emit` are demoted to the gated appendix.
- Rewrote **specgraph-authoring**: friendly snake_case `output` per stage (spark/shape/specify/decompose), the mandatory `exchanges` JSON array (required for shape/specify/decompose, explicitly OPTIONAL for spark, not needed for approve), and a preserved `author_start_stage` entry path.
- Aligned the remaining 5 skills (graph-query, analytical-passes, drift, conventions, troubleshooting) MCP-first with the uniform gated appendix; troubleshooting reframed on the `health` tool + `specgraph://` resources.

## Task Commits

1. **Task 1: Content-reference + parser-binding assertion test (RED)** — `f6c11637` (test)
2. **Task 2: Rewrite the two critical-path skills MCP-first** — `31143b65` (feat)
3. **Task 3: Align the remaining 5 skills + uniform gated appendix** — `f7360d30` (feat)

_Task 1 wrote the failing gate; the test's camelCase/CLI-ordering scoping was refined during Tasks 2–3 as the skills turned it GREEN (see Deviations)._

## Files Created/Modified

- `internal/mcp/skills/skill_mcp_reference_test.go` — content-level + parser-binding gate (created)
- `internal/mcp/skills/embedded/specgraph-constitution/SKILL.md` — MCP `constitution` get/update, inline friendly-YAML write payload, gated CLI appendix
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` — friendly-YAML `output` per stage + `exchanges` JSON contract, `author_start_stage` preserved, gated appendix
- `internal/mcp/skills/embedded/specgraph-graph-query/SKILL.md` — gated appendix
- `internal/mcp/skills/embedded/specgraph-analytical-passes/SKILL.md` — gated appendix
- `internal/mcp/skills/embedded/specgraph-drift/SKILL.md` — `drift` tool acknowledge/detect; CLI form gated
- `internal/mcp/skills/embedded/specgraph-conventions/SKILL.md` — MCP resource/tool reference + gated appendix
- `internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md` — reframed on `health` tool + resources; CLI diagnostics gated

All edits are on the canonical `internal/mcp/skills/embedded/` paths (the repo-root `skills/` is a reverse-symlink; `skills_symlink_test.go` and `task check` confirm no divergence — Pitfall 4 avoided).

## Decisions Made

- **camelCase guard scoped to ```yaml blocks, not whole body.** The plan's "extract the yaml output block" intent conflicts with keeping prose anti-examples ("don't use `scopeIn`…"). The guard concatenates all extracted `output` blocks and checks those, so the teaching anti-examples remain while the actual payload is still guarded.
- **CLI-ordering guard scans body after front-matter.** The skill `description` fields legitimately name CLI trigger phrases (e.g. specgraph-drift's "running `specgraph drift` from the CLI"). Stripping front-matter before the ordering check preserves those triggers without weakening the body-level gate.
- **MCP-reference regex is `specgraph_` OR `specgraph://`.** Chosen over matching bare tool words (`edge`, `spec`) which would be fragile; each aligned skill was given a concrete resource/tool reference.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] camelCase guard false-failed on prose anti-examples**
- **Found during:** Task 2 (authoring rewrite)
- **Issue:** The Task-1 guard scanned the whole authoring body for camelCase keys; the rewritten skill's "Don't camelCase (`scopeIn`, `chosenApproach`, `verifyCriteria`)" teaching line tripped it.
- **Fix:** Restricted the snake_case/camelCase guard to the extracted ```yaml `output` blocks (via `allYAMLBlocks`), matching the plan's "extract the yaml output block" intent.
- **Files modified:** internal/mcp/skills/skill_mcp_reference_test.go
- **Verification:** `authoring-snake-case-guard` subtest passes; prose anti-examples retained.
- **Committed in:** 31143b65 (Task 2 commit)

**2. [Rule 1 - Bug] CLI-ordering guard false-failed on front-matter description**
- **Found during:** Task 3 (drift alignment)
- **Issue:** specgraph-drift's front-matter `description` names "running `specgraph drift` from the CLI" as an activation trigger; the whole-body ordering guard flagged it as a CLI command before the appendix.
- **Fix:** Added `stripFrontmatter` and ran the CLI-ordering guard over the body after the closing `---`, so front-matter trigger phrases are ignored while body prose is still gated.
- **Files modified:** internal/mcp/skills/skill_mcp_reference_test.go
- **Verification:** all 7 skills pass `TestSkillMCPReference`; `task check` green.
- **Committed in:** f7360d30 (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (2 test-scoping bugs, Rule 1). Both were test-precision corrections that tightened the gate to the plan's stated intent (scan the taught YAML / body, not prose / front-matter). No production-skill scope change.
**Impact on plan:** None to scope. The gate is stricter-but-correct; all planned acceptance criteria met.

## Issues Encountered

None — all three verifications (`go test ./internal/mcp/skills/`, `task skills:validate`, `task check`) pass.

## Merge-Window / Atomic-Release Constraint (carry-forward)

Per the plan's ATOMIC-RELEASE INVARIANT (HIGH): this plan rewrites skills to teach friendly-YAML `output`/`exchanges`, but the handlers that accept friendly YAML land in **06-04**. 06-02 and 06-04 MUST land in the SAME merge window (single PR or stacked merge, no intermediate deploy) — otherwise a live MCP-only agent between the two merges reproduces #1002 against the still-protojson handlers (`tools_authoring.go:178-179`). The friendly-YAML `output`/`exchanges` values the skills teach (e.g. `strategy: vertical_slice`, `scope_sniff: small`) are educated friendly forms that 06-04's parser must accept.

## Next Phase Readiness

- MCP-first skill content is in place and regression-gated for the phase.
- **06-04** must wire the friendly-YAML `constitution update` / `author output` handlers and land in the same merge window (see above).
- **06-05** owns the MCP-only e2e that round-trips the taught authoring YAML through `internal/authoring/load` and corrects the e2e six→seven skill-list assertion (`e2e/api/skills_test.go`).

## Self-Check: PASSED

- `internal/mcp/skills/skill_mcp_reference_test.go` — FOUND
- All 7 modified `SKILL.md` canonicals — FOUND
- Commits `f6c11637`, `31143b65`, `f7360d30` — FOUND in git log
- `go test ./internal/mcp/skills/` — ok
- `task skills:validate` — 7 packages, all OK
- `task check` — PASSED

---
*Phase: 06-mcp-authoring-self-teaching-path*
*Completed: 2026-07-14*
