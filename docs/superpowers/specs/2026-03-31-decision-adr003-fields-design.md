# Design: Extend Decision Domain Type with ADR-003 Fields

**Bead:** spgr-bk8
**Date:** 2026-03-31
**Status:** Approved

## Problem

The Decision struct is missing fields from ADR-003: question, rejected alternatives, confidence, tags, scope, origin_spec, origin_stage. The ChangeLog system also does not support Decision nodes. These gaps prevent ADR export, cross-spec referencing, and authoring funnel integration from working as designed.

## Design Decisions

1. **ChangeLog support for Decisions** — generalize `createChangeLog` to accept a node label parameter rather than hardcoding `Spec`. Decisions get the same `HAS_CHANGE` edges, version guards, and field-level deltas as specs. Decisions gain a `Version` field.
2. **Structured rejected alternatives** — `[]RejectedAlternative{Option, Reason}` stored as JSON property on Memgraph nodes (same pattern as `changes_json` on ChangeLog).
3. **Typed enums for confidence and scope** — `DecisionConfidence` (high/medium/low) and `DecisionScope` (project/team/org) with `IsValid()` methods, matching existing enum patterns.
4. **Tags as string slice** — stored as JSON array property on Memgraph nodes.

## Domain Types

### New Types (`internal/storage/decision.go`)

```go
type DecisionConfidence string

const (
    DecisionConfidenceHigh   DecisionConfidence = "high"
    DecisionConfidenceMedium DecisionConfidence = "medium"
    DecisionConfidenceLow    DecisionConfidence = "low"
)

type DecisionScope string

const (
    DecisionScopeProject DecisionScope = "project"
    DecisionScopeTeam    DecisionScope = "team"
    DecisionScopeOrg     DecisionScope = "org"
)

type RejectedAlternative struct {
    Option string
    Reason string
}
```

### Extended Decision Struct

New fields added to existing struct:

| Field | Type | Description |
|-------|------|-------------|
| Question | string | The question being decided |
| RejectedAlternatives | []RejectedAlternative | Options considered but not chosen |
| Confidence | DecisionConfidence | Confidence level (high/medium/low) |
| Tags | []string | Categorization tags |
| Scope | DecisionScope | How broadly the decision applies |
| OriginSpec | string | Slug of the spec that originated this decision |
| OriginStage | string | Authoring stage when decision was made |
| Version | int | Monotonic version counter for ChangeLog version guards |

### DecisionBackend Interface

Updated signatures:

```go
CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
    rejectedAlts []RejectedAlternative, confidence DecisionConfidence,
    tags []string, scope DecisionScope, originSpec, originStage string) (*Decision, error)

UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus,
    body, rationale, supersededBy, question *string,
    rejectedAlts *[]RejectedAlternative, confidence *DecisionConfidence,
    tags *[]string, scope *DecisionScope, originSpec, originStage *string) (*Decision, error)
```

Version is initialized to 1 on creation and incremented automatically by UpdateDecision. Not exposed as an update parameter.

## Proto Schema

### New Enums and Messages (`decision.proto`)

```protobuf
enum DecisionConfidence {
  DECISION_CONFIDENCE_UNSPECIFIED = 0;
  DECISION_CONFIDENCE_HIGH = 1;
  DECISION_CONFIDENCE_MEDIUM = 2;
  DECISION_CONFIDENCE_LOW = 3;
}

enum DecisionScope {
  DECISION_SCOPE_UNSPECIFIED = 0;
  DECISION_SCOPE_PROJECT = 1;
  DECISION_SCOPE_TEAM = 2;
  DECISION_SCOPE_ORG = 3;
}

message RejectedAlternative {
  string option = 1;
  string reason = 2;
}
```

### Extended Decision Message

New fields at field numbers 11+, wire-compatible with existing clients:

| Field Number | Name | Type |
|-------------|------|------|
| 11 | question | string |
| 12 | rejected_alternatives | repeated RejectedAlternative |
| 13 | confidence | DecisionConfidence |
| 14 | tags | repeated string |
| 15 | scope | DecisionScope |
| 16 | origin_spec | string |
| 17 | origin_stage | string |
| 18 | version | int32 |

### Request Messages

`CreateDecisionRequest` gains optional fields for question, rejected_alternatives, confidence, tags, scope, origin_spec, origin_stage. `UpdateDecisionRequest` gains optional versions of each new field.

## Storage (Memgraph)

### Node Properties

Decision nodes gain properties: `question`, `rejected_alternatives_json` (JSON string), `confidence`, `tags_json` (JSON string array), `scope`, `origin_spec`, `origin_stage`, `version`.

JSON serialization for `rejected_alternatives_json` and `tags_json` follows the same pattern as `changes_json` on ChangeLog nodes.

### ChangeLog Generalization

`createChangeLog` currently hardcodes `MATCH ... (s:Spec {slug: $slug})`. Generalize to accept a node label:

```go
func (s *Store) createChangeLog(ctx context.Context, label, slug string, ...) error
```

The Cypher MATCH clause uses `fmt.Sprintf` to inject the label (safe — label is always a hardcoded string `"Spec"` or `"Decision"`, never user input):

```cypher
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(n:%s {slug: $slug})
WHERE n.version = $expected_version
CREATE (n)-[:HAS_CHANGE]->(cl:ChangeLog { ... })
```

Same generalization for `ListChanges` and the ChangeLog index query. The ChangeLog node itself is unchanged — it already stores version, summary, field deltas, and content_hash. Callers pass `"Spec"` or `"Decision"`.

`UpdateDecision` calls `createChangeLog` with field-level deltas (comparing old vs new values) inside `RunInTransaction`, same pattern as `UpdateSpec`.

### Content Hash

`contenthash.Decision()` expands:

```go
func Decision(title, status, body, rationale, question, confidence, scope string,
    tags []string, rejectedAlts []RejectedAlternative) string
```

Tags sorted alphabetically (case-sensitive, matching Go's default `sort.Strings`). Rejected alternatives sorted alphabetically by Option field. Both are joined into deterministic strings before hashing.

## Handler & Converter

### Handler (`decision_handler.go`)

`CreateDecision` and `UpdateDecision` handlers pass new fields through. No new RPCs.

### Converter (`convert_decision.go`)

`decisionToProto` and `decisionFromProto` grow to map new fields. New enum maps: `decisionConfidenceToProtoMap`, `decisionConfidenceFromProtoMap`, `decisionScopeToProtoMap`, `decisionScopeFromProtoMap`. `RejectedAlternative` domain ↔ proto mapping.

## CLI

`decision create` gains flags: `--question`, `--confidence`, `--tags` (comma-separated), `--scope`, `--origin-spec`, `--origin-stage`. Rejected alternatives via repeatable `--rejected "Option:Reason"` flag. If option or reason contains a colon, use the first colon as delimiter (reason gets the rest). All new flags are optional — existing usage unchanged.

## Render

`render.Decision()` expands to display new fields in markdown output. Question as heading, rejected alternatives as table, tags inline, confidence/scope as metadata. Empty fields omitted.

## Backward Compatibility

Existing Decision nodes in Memgraph lack the new properties. On read, missing properties default to zero values: empty string for `question`/`origin_spec`/`origin_stage`/`confidence`/`scope`, empty JSON array `"[]"` for `rejected_alternatives_json`/`tags_json`, `0` for `version`. The `recordToDecision` parser already handles missing properties via `recordStringOptional` — new fields use the same pattern.

No migration script needed. Existing decisions are valid with zero-value defaults. First `UpdateDecision` call on a legacy node will populate the new fields and set `version` to 1 if absent.

`rejected_alternatives_json` format: `[{"option":"Redis","reason":"Adds ops complexity"}]`

`tags_json` format: `["auth","storage","security"]`

## Out of Scope

- ListDecisions filtering by tags/scope/confidence (follow-up; clients filter in-process for now)
- Authoring funnel skill integration (separate concern)
- Edge types DECIDED_IN/REFERENCES (already exist)
- Decision supersession workflow changes

## Testing

- Unit tests for new enum validation, content hash determinism, JSON round-trip
- Handler tests for create/update with new fields
- Integration tests for Memgraph storage of new properties and ChangeLog on decisions
- Render tests for new field display
