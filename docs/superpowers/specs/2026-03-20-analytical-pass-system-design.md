# Analytical Pass System Design

**Date:** 2026-03-20
**Bead:** spgr-5pq (constitution_check — infrastructure + first template)
**Milestone:** 0.2.0
**Status:** Implemented

## Problem

The analytical pass infrastructure exists (pass registry, scheduling, posture-aware auto-run) but `runAnalyticalPasses` returns placeholder strings. Five separate finding types and five separate `Store*` methods create unnecessary surface area. The existing `CheckViolation` does hardcoded substring matching against forbidden languages, which misunderstands the constitution — it's a fluid set of intentions and judgment calls, not a mechanical rulebook.

## Decision Summary

| Question | Decision |
|----------|----------|
| Content selection | Progressive: stage determines what spec content is available, uniform rule logic |
| LLM involvement | All LLM — constitution is too fluid for mechanical checks |
| Who runs the LLM | Agent-driven with centralized prompts; server-side provider deferred |
| Prompt structure | Single markdown persona prompt per pass with tool manifest as TOC |
| RPC design | Dedicated `RunAnalyticalPass` RPC, separate from stage transitions |
| Finding types | Unified `AnalyticalFinding` type replaces five separate types |
| Old CheckViolation | Remove entirely — new pass subsumes it |
| Data delivery | LLM uses tools to fetch constitution/spec content on demand, not bundled in prompt |

## Architecture

```text
+----------------+     RunAnalyticalPass(slug, pass)     +----------------+
|                | ------------------------------------> |                |
|   SpecGraph    |  returns: prompt + tools + message    |     Agent      |
|   Server       | <------------------------------------ |  (Claude Code  |
|                |                                       |   / Gastown)   |
|                |     StoreFindings(slug, findings)     |                |
|                | <------------------------------------ |                |
+----------------+                                       +-------+--------+
                                                                 |
                                                                 | LLM call
                                                                 v
                                                         +----------------+
                                                         | LLM Provider   |
                                                         | (Anthropic)    |
                                                         +----------------+
```

**SpecGraph's responsibilities:**

- Provide prompt template (markdown persona with task instructions and information map)
- Provide tool manifest (CLI commands the agent exposes to the LLM)
- Tell the agent which passes to run (pass registry, already exists)
- Store findings returned by the agent
- Serve spec/constitution content when the LLM calls tools

**Agent's responsibilities:**

- Call `RunAnalyticalPass` for each pass the stage requires
- Set up LLM session: system prompt from template, tools from manifest
- Parse structured findings from LLM response
- Store findings via `StoreFindings` RPC
- Automatic execution via hook/subagent on stage transitions (deferred, not 0.2.0 server scope)

## Service Ownership

New RPCs live in a new `AnalyticalPassService` (new proto service, new handler file `analytical_pass_handler.go`). The analytical pass system spans authoring and constitution concerns but is neither — it's its own domain. This follows the existing one-handler-per-service pattern (`constitution_handler.go`, `authoring_handler.go`, `graph_handler.go`).

## Proto Changes

### New Service and Messages

```protobuf
service AnalyticalPassService {
  rpc RunAnalyticalPass(RunAnalyticalPassRequest) returns (RunAnalyticalPassResponse);
  rpc StoreFindings(StoreFindingsRequest) returns (StoreFindingsResponse);
  rpc ListFindings(ListFindingsRequest) returns (ListFindingsResponse);
}

message RunAnalyticalPassRequest {
  string slug = 1;
  string pass_name = 2;
}

message RunAnalyticalPassResponse {
  string pass_name = 1;
  string prompt_template = 2;         // markdown persona (system prompt)
  repeated ToolReference tools = 3;   // CLI commands/RPCs to expose as tools
  string initial_message = 4;         // user-turn message to kick off evaluation
  repeated string offered_passes = 5; // passes available but not auto-running
  string stage = 6;                   // spec's current stage (informational)
}

message ToolReference {
  string name = 1;
  string command = 2;
  string description = 3;
}

message AnalyticalFinding {
  string id = 1;             // output-only; generated server-side (ULID)
  string pass_type = 2;
  FindingSeverity severity = 3;
  string summary = 4;
  string detail = 5;
  string constraint = 6;    // what rule/principle was evaluated
  string resolution = 7;    // suggested remediation
  int32 version = 8;        // spec version when finding was produced
}

message StoreFindingsRequest {
  string slug = 1;
  string pass_type = 2;
  repeated AnalyticalFindingInput findings = 3;
}

// Input variant without id — server generates IDs on storage (ULID, matching ChangeLog pattern).
message AnalyticalFindingInput {
  FindingSeverity severity = 1;
  string summary = 2;
  string detail = 3;
  string constraint = 4;
  string resolution = 5;
}

message StoreFindingsResponse {
  repeated string ids = 1;   // generated finding IDs
}

message ListFindingsRequest {
  string slug = 1;
  string pass_type = 2;  // optional: filter by pass type
}

message ListFindingsResponse {
  repeated AnalyticalFinding findings = 1;
}
```

### Removed Messages

- `CheckViolationRequest`, `CheckViolationResponse`
- `CheckViolation` RPC from `ConstitutionService`
- `ConstitutionViolation`, `RedTeamFinding`, `PeripheralVisionItem`, `ConsistencyIssue`, `SimplicityFinding`
- `constitution_violations` fields from `SparkResponse`, `ShapeResponse`, `SpecifyResponse`, `DecomposeResponse`
- Per-pass finding fields from stage responses (peripheral_vision, red_team_findings, etc.)

Clean break — no `reserved` needed. Pre-1.0, no external consumers.

## Domain Types

### New

```go
// PassType is a typed string for analytical pass identifiers in the storage layer.
type PassType string

const (
    PassTypeConstitutionCheck PassType = "constitution_check"
    PassTypePeripheralVision  PassType = "peripheral_vision"
    PassTypeRedTeam           PassType = "red_team"
    PassTypeConsistencyCheck  PassType = "consistency_check"
    PassTypeSimplicityCheck   PassType = "simplicity_check"
)

type AnalyticalFinding struct {
    ID         string
    PassType   PassType
    Severity   FindingSeverity
    Summary    string
    Detail     string
    Constraint string
    Resolution string
    Version    int32
    CreatedAt  time.Time
}
```

### Removed

- `ConstitutionViolation` struct (storage/authoring.go)
- `RedTeamFinding` struct
- `PeripheralVisionItem` struct
- `ConsistencyIssue` struct
- `SimplicityFinding` struct

## Graph Model

```text
(Spec)-[:HAS_FINDING]->(Finding {pass_type, severity, summary, detail, ...})
```

- Single directed edge, traversable both ways via Cypher
- Internal edge type (like `HAS_CHANGE`) — not exposed via `AddEdge`/`RemoveEdge`
- `HAS_FINDING` added to `ListEdges` exclusion list (same treatment as `HAS_CHANGE` and `BELONGS_TO`)
- `StoreFindings` replaces all findings for a given (slug, pass_type) pair — each run is a full refresh
- Stores spec `version` at time of analysis for staleness detection
- `Finding` is an internal-only node type (like `ChangeLog`) — no `NodeLabel` enum entry needed
- `StoreFindings` MUST use `RunInTransaction` (ADR-004) — delete-then-create must be atomic

### Migration

None. Clean break — pre-1.0, no external consumers. Old JSON properties on Spec nodes are removed without migration.

## Storage Interface Changes

### New

```go
type FindingsWriter interface {
    StoreFindings(ctx context.Context, slug string, passType PassType, findings []AnalyticalFinding) error
}

type FindingsReader interface {
    ListFindings(ctx context.Context, slug string, passType PassType) ([]AnalyticalFinding, error)
}
```

### Removed

- `PassWriter` interface (all five `Store*` methods except `StoreSafetyFlags`)
- `ConstitutionBackend.CheckViolation` method

### SafetyFlags

`SafetyFlag` and `StoreSafetyFlags` are **retained**. Safety flags are produced by deterministic in-process checks (`authoring.RunSafetyNet`), not LLM analytical passes. `StoreSafetyFlags` moves from `PassWriter` to `StageWriter` (it runs inline during stage transitions, which is `StageWriter`'s domain).

### Interface Composition

`FindingsBackend` (combining `FindingsWriter` + `FindingsReader`) is a **peer** of `AuthoringBackend` in `ScopedBackend` composition — not embedded inside it. Findings are decoupled from stage transitions.

## Prompt Template Design

The prompt template is a Markdown document with three responsibilities:

1. **Persona** — who the LLM is and how it should think
2. **Task instructions** — what to evaluate and how
3. **Information map** — what tools are available and what they provide (references the tool manifest)

Example structure for `constitution_check`:

```markdown
# Constitution Compliance Reviewer

## Who You Are

You are a constitution compliance analyst for SpecGraph...

## Your Task

Evaluate spec `{slug}` against the project constitution.
Identify any violations, conflicts, or tensions.

## Available Information

Use these tools to gather the information you need:

| Tool | Description |
|------|-------------|
| show_spec | Read the spec's current content and stage outputs |
| show_constitution | Read the full project constitution |
| list_deps | List specs this one depends on |
| show_dep | Read a specific dependency's content |

## Evaluation Framework

- Check tech stack alignment
- Check principle adherence (consider exceptions)
- Check constraint compliance
- Identify antipattern matches
- Verify process requirements for this stage

## Output Format

Return findings as structured JSON array...

## Severity Guidelines

- critical: direct violation of an explicit constraint
- warning: tension with a principle or borderline antipattern
- note: worth flagging but not blocking
```

Templates are stored as embedded resources in the SpecGraph binary, returned via the `RunAnalyticalPass` response. The `{slug}` placeholder in the initial message is interpolated server-side.

## Handler Changes

- Remove `runAnalyticalPasses` function entirely
- Remove constitution violation fields from stage response construction
- Stage RPCs (Spark, Shape, Specify, Decompose) no longer produce or return analytical findings
- New `AnalyticalPassHandler` with methods: `RunAnalyticalPass`, `StoreFindings`, `ListFindings`

### Validation in `RunAnalyticalPass`

- Spec must exist (return `CodeNotFound` if not)
- Pass name must be a known `PassType` (return `CodeInvalidArgument` if not)
- Stage validation: warn but allow. The response includes the spec's current stage so the agent knows context, but out-of-band pass execution is permitted (e.g., running `red_team` early at the agent's discretion).

## Client Impact

Stage responses no longer include inline finding fields. Clean break — agents use `ListFindings` instead.

## ToolReference Scope

`ToolReference` is intentionally minimal for 0.2.0 (name, command, description). Server-side LLM execution would need richer tool definitions (parameter schemas, return types). Acceptable for now since only Claude Code is a consumer and it already knows the CLI command signatures.

## Testing Strategy

**Deterministic (unit/integration):**

- `RunAnalyticalPass` assembles correct tool manifest for each pass
- `RunAnalyticalPass` returns valid markdown prompt template
- `StoreFindings` persists findings with `HAS_FINDING` edges
- `StoreFindings` replaces (not appends) findings per (slug, pass_type)
- `ListFindings` filters by pass type correctly
- `HAS_FINDING` edge traversal works both directions
- Pass registry returns correct passes per stage/posture (existing tests)
- Old `CheckViolation` codepath is removed (compilation verifies)

**Not tested deterministically:**

- LLM output quality (agent responsibility, depends on prompt template)
- Finding severity correctness for a given constitution+spec pair

## Beads Impact

- `spgr-5pq` — this bead: full infrastructure + constitution_check prompt template
- `spgr-bmd` (simplicity_check) — depends on spgr-5pq: write prompt template + wire it
- `spgr-ikx` (consistency_check) — depends on spgr-5pq: write prompt template + wire it
- `spgr-ney` (red_team) — depends on spgr-5pq: write prompt template + wire it
- `spgr-wjz` (peripheral_vision) — depends on spgr-5pq: write prompt template + wire it

## Deferred

- Prompt templates for the other four passes
- Claude Code plugin hook/agent for automatic pass execution on stage transitions
- Server-side LLM provider option (enterprise mode)
- Cross-spec content inclusion in tool responses for consistency_check
