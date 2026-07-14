---
phase: 06-mcp-authoring-self-teaching-path
reviewed: 2026-07-14T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - e2e/api/mcp_only_authoring_test.go
  - e2e/api/skills_test.go
  - internal/authoring/load/load.go
  - internal/authoring/load/load_test.go
  - internal/mcp/resources.go
  - internal/mcp/resources_test.go
  - internal/mcp/skills/embedded/specgraph-analytical-passes/SKILL.md
  - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
  - internal/mcp/skills/embedded/specgraph-constitution/SKILL.md
  - internal/mcp/skills/embedded/specgraph-conventions/SKILL.md
  - internal/mcp/skills/embedded/specgraph-drift/SKILL.md
  - internal/mcp/skills/embedded/specgraph-graph-query/SKILL.md
  - internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md
  - internal/mcp/skills/skill_mcp_reference_test.go
  - internal/mcp/tools_authoring.go
  - internal/mcp/tools_authoring_test.go
  - internal/mcp/tools_core.go
  - internal/mcp/tools_core_test.go
  - internal/render/prime.go
  - internal/render/prime_test.go
findings:
  critical: 2
  warning: 1
  info: 2
  total: 5
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-07-14
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

The phase replaces the raw-protojson MCP write boundary with a friendly
snake_case YAML pipeline (`internal/authoring/load`), rewrites the seven
embedded SKILL.md canonicals MCP-first, routes `specgraph://prime` through the
render layer, and adds an MCP-only e2e gate. The mechanical parts are solid:
enum mappers correctly reject unknown values with an error instead of
silent-writing `*_UNSPECIFIED`, handler errors are sanitized (`//nolint:nilerr`
with T-06-03 rationale), the render layer clones proto messages defensively
before clearing provenance, and the e2e harness genuinely speaks only MCP
(`ReadResource`/`CallTool`) — the inner `mcppkg.NewClient` is the server's own
ConnectRPC wiring, never a surface the test body drives directly. The
`skill_mcp_reference_test.go` gate is a strong regression guard for the SKILL.md
files.

However, the review surfaced a **silent-data-loss path that reproduces the very
#1002 defect this phase claims to fix**, through two coupled defects: the YAML
parsers drop unrecognized keys silently (no `KnownFields`), and the stage prompt
content files this phase modified still teach the *old camelCase protojson*
shape. An agent following the composed `shape`/`specify`/`decompose` MCP prompt
is instructed to emit camelCase, which the new parser silently discards,
persisting an empty/partial stage output with a success result. The standalone
SKILL.md files were fixed; the composed-prompt content was not.

## Critical Issues

### CR-01: YAML parsers silently drop unrecognized keys — camelCase typos produce silent partial/empty writes

**File:** `internal/authoring/load/load.go:124,147,181,213`
**Issue:**
All four parsers use `yaml.Unmarshal(data, &in)`, which does **not** error on
unknown/unmatched keys. yaml.v3 only rejects unknown fields when you use a
`*yaml.Decoder` with `KnownFields(true)`. As written, any key that does not
match a `yaml:"snake_case"` tag — e.g. the camelCase form `chosenApproach`,
`scopeIn`, `verifyCriteria` — is **silently ignored**, leaving the corresponding
proto field at its zero value.

This directly contradicts the code's own documented contract:
- The `author` tool description (`tools_authoring.go:148`): *"camelCase (scopeIn,
  chosenApproach) is rejected."*
- `specgraph-authoring/SKILL.md:142-144`: *"Do NOT camelCase them (`scopeIn`,
  `chosenApproach`, `verifyCriteria` will be rejected)."*

Neither is true — the keys are dropped, not rejected. Worse, the server does not
backstop this: `internal/server/authoring_handler.go:140-161` validates only
*maximum* slice sizes and element counts for shape output; there is **no
presence/required-field check** on `scope_in`, `chosen_approach`, etc. So a
mistyped key yields an empty `ShapeOutput` that persists successfully with no
error surfaced to the agent — silent data loss, which is the #1002 failure mode
(agent hands the tool a shape it "can't accept") re-expressed as a silent
success instead of a hard failure.

**Fix:** Parse with a strict decoder so unrecognized keys become a returned
error (which the handlers already sanitize into "expected friendly snake_case
YAML"):

```go
func SparkFromYAML(data []byte) (*specv1.SparkOutput, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var in sparkYAML
	if err := dec.Decode(&in); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse spark yaml: %w", err)
	}
	// ... rest unchanged
}
```

Apply the same to `ShapeFromYAML`, `SpecifyFromYAML`, `DecomposeFromYAML` (and
the nested structs decode strictly too, since `KnownFields` propagates). Add a
test asserting `chosenApproach:` / `scopeIn:` inputs now return an error, to lock
the doc claim to real behavior.

### CR-02: Stage prompt content (modified this phase) still teaches camelCase protojson — the composed MCP prompt drives agents into CR-01

**File:** `internal/authoring/content/stage-shape.md:140-165`, `stage-specify.md:120-143`, `stage-spark.md:72`, `stage-decompose.md:44,105`
**Issue:**
These stage content files are embedded via `//go:embed` and composed into the
`shape`/`specify`/`decompose`/`spark` **MCP prompt responses**
(`internal/authoring/composer.go`, per AGENTS.md). All four were modified in this
phase's diff, yet their "Persistence Contract" sections still present the stage
output as **camelCase protojson**:

```json
{
  "scopeIn": ["item 1", "item 2"],
  "chosenApproach": "approach-a",
  "successMust": ["criterion 1"],
  ...
}
```
(`stage-shape.md:142-155`; similarly `verifyCriteria`/`changeType` in
`stage-specify.md`, `scopeSniff`/`killTest` in `stage-spark.md`, `dependsOn` in
`stage-decompose.md`.)

The `author` tool now accepts **only friendly snake_case YAML** for `output`
(`tools_authoring.go:140-148`), and there is no snake_case / `author`-tool
guidance anywhere in the stage content files (grep confirms zero references to
`snake_case`, `scope_in`, `chosen_approach`, or the `author` tool in
`content/stage-*.md`). An agent that invokes the primary MCP prompt path is
therefore taught to synthesize camelCase, feeds it to `author`, and — via CR-01
— gets those keys silently dropped. This is the exact "skill teaches a shape the
tool cannot accept" defect (#1002) that the phase set out to eliminate; it was
fixed only in the standalone `SKILL.md` canonicals, not in the composed prompt
content that agents actually receive first.

Note: `TestContentProtoDrift` (`internal/authoring/drift_test.go`) only checks
backticked snake_case tokens, so these JSON-block camelCase names evade the
existing drift gate.

**Fix:** Rewrite the "Persistence Contract" blocks in all four
`content/stage-*.md` files to teach the friendly snake_case YAML `author`-tool
`output` payload (matching the parser tags and the specgraph-authoring SKILL.md),
and reference the `author` tool + `exchanges` contract. Then extend a drift/gate
test to fail if a `content/stage-*.md` fenced block contains any of the camelCase
stage-field forms, mirroring the `authoring-snake-case-guard` subtest in
`skill_mcp_reference_test.go`.

## Warnings

### WR-01: `amend` target_stage guidance is internally inconsistent (`approve` vs `approved`, and schema hint omits the approved value)

**File:** `internal/mcp/tools_authoring.go:162,348`
**Issue:**
The `target_stage` schema hint (line 162) advertises *"spark, shape, specify,
decompose"* — omitting the approved terminal stage — while the validation error
message (line 348) advertises *"spark, shape, specify, decompose, approved"*.
`authoringStageFromString` maps only `AUTHORING_STAGE_APPROVED`, so a caller who
follows the natural verb `approve` (matching the funnel action name) gets
rejected, while `approved` is required. The schema hint and the error string
disagree, and neither matches the action verb the rest of the tool uses.
**Fix:** Make the schema hint and the error message list the identical accepted
set, and accept both `approve` and `approved` (or document the exact accepted
token unambiguously in the schema hint):

```go
"target_stage": stringProp("Target stage for amend: spark, shape, specify, decompose, approved"),
```

## Info

### IN-01: "prevent JSON injection" comment is misleading — the string concatenation *is* the injection surface

**File:** `internal/mcp/tools_authoring.go:76-78,435-437` (and `tools_authoring.go:542-544`)
**Issue:**
`parseOptionalExchanges`, `handleRecord`, and `handleStore` build the protojson
document by raw string concatenation (`[]byte(`{"exchanges":`+raw+`}`)`) and
comment it as *"Parse ... in isolation to prevent JSON injection."* The
concatenation is the opposite of isolation — a caller can inject sibling fields
(e.g. `raw = `[],"slug":"x"``). It is *contained* only incidentally: the code
reads back just `.Exchanges`, and protojson errors on genuinely unknown fields.
No exploit results, but the comment overstates the safety property and could mask
a real regression if a future edit consumes more of the parsed wrapper.
**Fix:** Unmarshal `raw` directly into the leaf type instead of splicing it into
a wrapper string, e.g. decode into `[]json.RawMessage` / a
`[]*specv1.ConversationExchange` via a small typed wrapper unmarshalled from
`raw` alone, and correct the comment to describe what is actually guaranteed.

### IN-02: package-doc DoS-bounding claim is stronger than the implementation

**File:** `internal/authoring/load/load.go:8-11`
**Issue:**
The package comment states input is decoded into fixed typed structs to *"bound
the input shape"* for DoS. Typed structs bound *shape*, but there is no explicit
cap on input length or nesting depth; the parsers rely on the caller and on
yaml.v3's built-in alias-expansion protection. This is acceptable given the MCP
transport typically bounds request size, but the doc implies a stronger local
guarantee than the code provides.
**Fix:** Either soften the comment to note the reliance on upstream size limits +
yaml.v3 alias protection, or add an explicit `len(data)` guard before decoding.

---

## Structural Findings (fallow)

No `<structural_findings>` block was provided with this review; none to report.

---

_Reviewed: 2026-07-14_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
