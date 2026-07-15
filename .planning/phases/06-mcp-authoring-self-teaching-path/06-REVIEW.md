---
phase: 06-mcp-authoring-self-teaching-path
reviewed: 2026-07-14T17:08:31Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - e2e/api/mcp_only_authoring_test.go
  - e2e/api/skills_test.go
  - internal/authoring/load/load.go
  - internal/authoring/load/load_test.go
  - internal/authoring/content/stage-spark.md
  - internal/authoring/content/stage-shape.md
  - internal/authoring/content/stage-specify.md
  - internal/authoring/content/stage-decompose.md
  - internal/authoring/drift_test.go
  - internal/mcp/resources.go
  - internal/mcp/resources_test.go
  - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
  - internal/mcp/skills/embedded/specgraph-constitution/SKILL.md
  - internal/mcp/skills/skill_mcp_reference_test.go
  - internal/mcp/tools_authoring.go
  - internal/mcp/tools_authoring_test.go
  - internal/mcp/tools_core.go
  - internal/mcp/tools_core_test.go
  - internal/render/prime.go
  - internal/render/prime_test.go
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 06: Code Review Report

**Reviewed:** 2026-07-14T17:08:31Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** clean

## Summary

Iteration 2 re-review of the three fixes applied by the previous `--fix --auto`
pass. All three prior findings (CR-01, CR-02, WR-01) are **confirmed resolved**,
and the fixes introduced no new correctness, security, or regression defects.
The full test surface for the touched packages passes:

- `go test ./internal/authoring/... ./internal/render/... ./internal/mcp/...` → **all green**
- Strict-decoder negative tests, drift guards, and the amend-alias test all exercise the fixed behavior.

Two low-severity Info observations remain (DRY duplication and a benign
multi-document YAML edge). Neither blocks; both are pre-existing patterns rather
than regressions from the fix pass.

### Prior-finding verification

**CR-01 — strict YAML decoder (RESOLVED).**
`internal/authoring/load/load.go` now defines `decodeStrict` (load.go:31-38)
using `yaml.NewDecoder(...)` + `dec.KnownFields(true)`, with the decode error
wrapped for wrapcheck (`fmt.Errorf("decode yaml: %w", err)`) and each parser
re-wrapping (`parse spark yaml: %w`, etc.). All four parsers route through it:
`SparkFromYAML` (142), `ShapeFromYAML` (165), `SpecifyFromYAML` (199),
`DecomposeFromYAML` (231). An empty document (`io.EOF`) is correctly treated as
an empty value, not an error, so genuinely-optional fields still work
(`TestSparkFromYAML_Minimal`, `TestShapeFromYAML_Minimal`,
`TestDecomposeFromYAML_Minimal`, `TestSpecifyFromYAML_Minimal` all pass). Negative
tests for unknown/camelCase keys exist at both top level
(`TestSparkFromYAML_RejectsCamelCase`, `TestSparkFromYAML_RejectsUnknownKey`,
`TestShapeFromYAML_RejectsCamelCase`, `TestSpecifyFromYAML_RejectsCamelCase`,
`TestDecomposeFromYAML_RejectsCamelCase`) and nested
(`TestSpecifyFromYAML_RejectsNestedCamelCase` — confirms KnownFields propagates
into nested structs). No over-strictness regression: the friendly structs bound
the surface intentionally, and every field the docs teach is present as a yaml
tag.

**CR-02 — stage prompts teach snake_case matching load.go (RESOLVED).**
The four `internal/authoring/content/stage-*.md` Persistence Contract sections
now teach friendly snake_case YAML `author` payloads plus the JSON `exchanges`
contract. Field-name cross-check against load.go yaml tags is **exact** for all
four stages (spark: `seed/signal/questions/scope_sniff/kill_test`; shape:
`scope_in/scope_out/approaches{name,description,tradeoffs}/chosen_approach/risks/
success_must/success_should/success_wont/decisions{slug,title,decision,rationale}`;
specify: `interfaces{name,body}/verify_criteria{category,description}/invariants/
touches{path,purpose,change_type}`; decompose: `strategy/slices{id,intent,verify,
touches,depends_on}`). No residual camelCase protojson *guidance* remains — the
only camelCase mentions are explicit "do NOT camelCase" warnings. The `exchanges`
`role` field is a proto `string` (`proto/.../authoring.proto:456 // "probe" or
"response"`), so the taught `"role":"probe"` value is valid. Two drift guards lock
this in: `TestContentProtoDrift` and `TestContentPersistenceContractSnakeCase`
(drift_test.go), plus the SKILL.md guard in `skill_mcp_reference_test.go`.

**WR-01 — amend `target_stage` internal consistency (RESOLVED).**
Schema hint (tools_authoring.go:168 "spark, shape, specify, decompose,
approved"), the `approve`→`approved` alias in `authoringStageFromString`
(112-114), and the error message (354, identical value list) are now mutually
consistent. `TestAuthoringStageFromString` explicitly asserts both `approved`
and the `approve` alias resolve to `AUTHORING_STAGE_APPROVED`.

## Info

### IN-01: `conversation.handleRecord` re-implements exchange parsing instead of reusing `parseOptionalExchanges`

**File:** `internal/mcp/tools_authoring.go:437-445` (vs. helper at 71-82)
**Issue:** `handleRecord` inlines the same `protojson.Unmarshal([]byte(`{"exchanges":`+raw+`}`), ...)`
wrapper-parse pattern that `parseOptionalExchanges` already encapsulates. The two
copies can drift (e.g., if the injection-isolation approach changes, one may be
updated and the other missed). This is a maintainability nit, not a bug — both
copies consume only `.Exchanges`, so the "JSON injection" of sibling fields is
harmless in both.
**Fix:** Extract a shared required-vs-optional parser, e.g. reuse
`parseOptionalExchanges` and layer a not-empty check in `handleRecord`:
```go
exchanges, exErr := parseOptionalExchanges(params)
if exErr != nil {
    return exErr, nil
}
if len(exchanges) == 0 {
    return errResult("exchanges is required for record (JSON array of ConversationExchange)"), nil
}
```

### IN-02: `decodeStrict` silently ignores trailing YAML documents in a multi-document stream

**File:** `internal/authoring/load/load.go:31-38`
**Issue:** `yaml.Decoder.Decode` reads only the first document. A payload like
`seed: a\n---\nseed: b` decodes only the first doc; the second is dropped without
error. This matches the pre-fix `yaml.Unmarshal` behavior (also first-doc-only),
so it is **not a regression** and the single-payload authoring contract makes it
unlikely in practice. Flagged only for completeness given the stated goal of
"reject rather than silently drop."
**Fix (optional):** After a successful decode, attempt a second `Decode` and
error if it does not return `io.EOF`:
```go
if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
    return fmt.Errorf("decode yaml: %w", err)
}
if err := dec.Decode(&struct{}{}); err == nil {
    return errors.New("decode yaml: unexpected extra document")
}
```

---

_Reviewed: 2026-07-14T17:08:31Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
