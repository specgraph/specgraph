---
phase: 06-mcp-authoring-self-teaching-path
fixed_at: 2026-07-14T17:03:57Z
review_path: .planning/phases/06-mcp-authoring-self-teaching-path/06-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 6: Code Review Fix Report

**Fixed at:** 2026-07-14T17:03:57Z
**Source review:** .planning/phases/06-mcp-authoring-self-teaching-path/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (2 Critical, 1 Warning — Info deferred)
- Fixed: 3
- Skipped: 0

All in-scope findings were fixed, verified per-package, and confirmed green by
a full `task check` run (exit 0: fmt → license → lint → build → unit tests).

## Fixed Issues

### CR-01: YAML parsers silently drop unrecognized keys — camelCase typos produce silent partial/empty writes

**Files modified:** `internal/authoring/load/load.go`, `internal/authoring/load/load_test.go`
**Commits:** `65bb6a2a`, `97e304a4`
**Applied fix:**
Added a `decodeStrict` helper that wraps `yaml.NewDecoder` with
`KnownFields(true)` (tolerating empty-document `io.EOF`), and switched all four
parsers (`SparkFromYAML`, `ShapeFromYAML`, `SpecifyFromYAML`,
`DecomposeFromYAML`) from `yaml.Unmarshal` to it. Unknown/camelCase keys
(`scopeIn`, `chosenApproach`, `verifyCriteria`, `changeType`, `dependsOn`, …)
now return an error instead of being silently dropped, honoring the tool
description and SKILL.md contract. `KnownFields` propagates into nested structs,
so camelCase nested keys (e.g. `changeType` under `touches`, `dependsOn` under
`slices`) are rejected too. Updated the package doc to describe strict decoding.
Added six negative tests asserting camelCase / unknown keys return an error at
top level and nested level; existing enum-rejection and malformed-input behavior
is preserved. Follow-up commit `97e304a4` wraps the decoder error
(`fmt.Errorf("decode yaml: %w", err)`) to satisfy the `wrapcheck` linter surfaced
by `task check`.

### CR-02: Stage prompt content still taught camelCase protojson — the composed MCP prompt drove agents into CR-01

**Files modified:** `internal/authoring/content/stage-spark.md`, `internal/authoring/content/stage-shape.md`, `internal/authoring/content/stage-specify.md`, `internal/authoring/content/stage-decompose.md`, `internal/authoring/drift_test.go`, `internal/authoring/testdata/golden/{spark,shape,specify,decompose}.md`
**Commit:** `c0f35b20`
**Applied fix:**
Rewrote the "Persistence Contract" sections in all four stage content files to
teach the friendly **snake_case YAML** `author`-tool `output` payload (matching
the parser tags in `load.go` and the `specgraph-authoring` SKILL.md), replacing
the old camelCase protojson JSON blocks. Each contract now names the `author`
tool with its stage `action`, includes the snake_case `output` YAML, and teaches
the explicit JSON `exchanges` contract (required for shape/specify/decompose,
optional for spark). Fixed lingering prose camelCase references (`successMust`/
`successShould` → `success_must`/`success_should` in stage-specify.md;
`dependsOn` → `depends_on` in the stage-decompose.md elicitation table).
Added `TestContentPersistenceContractSnakeCase` to `drift_test.go` — it fails if
any fenced example block in `content/stage-*.md` contains a banned camelCase
stage-field form, mirroring the `authoring-snake-case-guard` subtest in
`skill_mcp_reference_test.go` and closing the gap that `TestContentProtoDrift`
(which strips fenced blocks) left open. Allowlisted the cross-stage ShapeOutput
field names in `TestContentProtoDrift` since specify prose legitimately maps them.
Regenerated the four composer golden files (`-update`); all remained within the
stable/total token budgets.

### WR-01: `amend` target_stage guidance was internally inconsistent (`approve` vs `approved`, schema hint omitted the approved value)

**Files modified:** `internal/mcp/tools_authoring.go`, `internal/mcp/tools_authoring_test.go`
**Commit:** `9dbc695e`
**Applied fix:**
Confirmed the canonical token set against the proto enum
(`AUTHORING_STAGE_{SPARK,SHAPE,SPECIFY,DECOMPOSE,APPROVED}`). Updated the
`target_stage` schema hint to list the identical accepted set as the validation
error message (`spark, shape, specify, decompose, approved`). Added an `approve`
→ `approved` alias in `authoringStageFromString` so a caller who follows the
natural funnel verb `approve` is accepted rather than rejected. Added a test
asserting the `approve` alias maps to `AUTHORING_STAGE_APPROVED`. Schema hint,
accepted values, and error message are now internally consistent.

## Skipped Issues

None — all in-scope findings were fixed.

Note: Info findings IN-01 (JSON-injection comment) and IN-02 (DoS-bounding
doc claim) were out of scope for this `critical_warning` fix pass and were not
addressed.

---

_Fixed: 2026-07-14T17:03:57Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
