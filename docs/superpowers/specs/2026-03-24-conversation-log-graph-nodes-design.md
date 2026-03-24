# ConversationLog Graph Nodes for Authoring Audit Trail

**Status:** Approved
**Date:** 2026-03-24
**Bead:** spgr-9mz
**Blocks:** spgr-0dg (CLI show detail), spgr-zn1 (web UI spec detail)

## Problem

The authoring skills (spark, shape, specify, decompose, approve) collect user inputs during conversational probes but only persist the final synthesized outputs. The user's answers, corrections, and judgment calls are lost when the session ends. This makes it impossible to:

1. Audit why a spec looks the way it does — what alternatives were considered, what tradeoffs drove the decisions.
2. Onboard new team members — understanding a spec's intent requires knowing the reasoning behind scope, approach, and interface choices.
3. Power richer `specgraph show` and web UI detail views — both spgr-0dg and spgr-zn1 need this data to render a complete spec story.

The existing ChangeLog nodes capture *what* changed (field-level deltas), but not *why* the user made those choices.

## Decision

Introduce `ConversationLog` as a first-class graph node linked to specs via `AUTHORED_VIA` edges. Each authoring stage completion creates a ConversationLog node containing probe-and-response exchange pairs. ConversationLog nodes link to their corresponding ChangeLog nodes via `EXPLAINS` edges and to each other via `CONTINUES` edges, forming a traversable narrative chain.

### Capture mechanism

Skills bundle conversation exchanges into the stage output JSON. A standalone `specgraph conversation record` CLI command also exists so hooks, scripts, or external tools can feed the same write path.

### Significance filter

Probe-and-response granularity: each skill probe and the user's substantive response are captured. Meta-conversation ("does that make sense?", "yes go on") is excluded. This maps naturally to each skill's elicitation structure — Shape has ~5 probes, Specify has ~4, etc. — producing a predictable, bounded number of exchanges per stage.

### Storage format

Semi-structured: an array of exchange objects with freeform text content but structured metadata (role, stage, sequence number). An optional `decision_point` flag marks exchanges where the user chose between alternatives.

## Design

### Data Model

**New graph node — `ConversationLog`:**

| Property | Type | Description |
|---|---|---|
| `id` | string | `cvl` prefixed ULID (via `newID("cvl")` → `cvl-<ULID>`) |
| `stage` | string | Authoring stage that produced this log |
| `version` | int32 | Spec version at capture time (always current version, set server-side) |
| `is_amend` | bool | True if this was an amend re-entry |
| `exchanges_json` | string | JSON-serialized `[]ConversationExchange` |
| `exchange_count` | int32 | Number of exchanges (avoids deserializing to count) |
| `date` | string | Timestamp (sortableRFC3339Nano format, matches ChangeLog convention) |

**Exchange structure (inside `exchanges_json`):**

```json
[
  {
    "role": "probe",
    "content": "What should be in scope for this feature?",
    "stage": "shape",
    "sequence": 1
  },
  {
    "role": "response",
    "content": "API endpoints and storage layer, not the CLI",
    "stage": "shape",
    "sequence": 1,
    "decision_point": true
  }
]
```

**New proto messages:**

```protobuf
message ConversationExchange {
  string role = 1;           // "probe" or "response"
  string content = 2;
  string stage = 3;
  int32 sequence = 4;        // pairs probes with responses
  bool decision_point = 5;   // true if user made a judgment call
}

message ConversationLog {
  string id = 1;
  string stage = 2;
  int32 version = 3;
  bool is_amend = 4;
  repeated ConversationExchange exchanges = 5;
  int32 exchange_count = 6;
  google.protobuf.Timestamp date = 7;
}
```

**New edge types:**

| Edge | Direction | Description |
|---|---|---|
| `AUTHORED_VIA` | `(:Spec)->(:ConversationLog)` | Links spec to its first conversation log |
| `CONTINUES` | `(:ConversationLog)->(:ConversationLog)` | Stage-to-stage narrative chain |
| `EXPLAINS` | `(:ConversationLog)->(:ChangeLog)` | Links conversation to the change it produced |

All three are internal-only — not added to the proto `EdgeType` enum, not exposed via `AddEdge`/`RemoveEdge` RPCs. Created automatically by the storage layer. Must be added to the internal edge exclusion list in `internal/storage/memgraph/graph.go` (alongside `BELONGS_TO`, `HAS_CHANGE`, `HAS_FINDING`).

**Graph shape for a spec that completed spark through decompose:**

```text
Spec ──AUTHORED_VIA──> ConvLog(spark) ──CONTINUES──> ConvLog(shape) ──CONTINUES──> ConvLog(specify) ──CONTINUES──> ConvLog(decompose)
                           |                             |                              |                              |
                        EXPLAINS                      EXPLAINS                       EXPLAINS                       EXPLAINS
                           v                             v                              v                              v
                      ChangeLog(spark)            ChangeLog(shape)              ChangeLog(specify)             ChangeLog(decompose)
```

**Amend re-entries** extend the `CONTINUES` chain. If a spec is amended back to Shape after Specify, a new ConversationLog(shape, is_amend=true) is appended to the tail with `EXPLAINS` pointing to the amend's ChangeLog.

### RPC Interface

Extend `AuthoringService`:

```protobuf
rpc RecordConversation(RecordConversationRequest) returns (RecordConversationResponse);
rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);

message RecordConversationRequest {
  string slug = 1;
  string stage = 2;
  repeated ConversationExchange exchanges = 3;
  bool is_amend = 4;
}

message RecordConversationResponse {
  ConversationLog conversation_log = 1;
}

message ListConversationsRequest {
  string slug = 1;
  string stage = 2;  // optional filter
}

message ListConversationsResponse {
  repeated ConversationLog conversation_logs = 1;
}
```

**Spec message addition:**

```protobuf
// In Spec message (field 15 is content_hash, field 13 is reserved)
repeated ConversationLog conversation_logs = 16;
```

`GetSpec` returns conversation data inline, serving both CLI show (spgr-0dg) and web UI detail (spgr-zn1).

**Domain struct addition:** The `Spec` struct in `internal/storage/spec_domain.go` gets a new `ConversationLogs []ConversationLogEntry` field. The memgraph `GetSpec` implementation populates this via a separate query (same pattern as how edges are fetched alongside the spec node).

### Storage Interface

New interface in `internal/storage/`:

```go
type ConversationBackend interface {
    RecordConversation(ctx context.Context, slug string, entry ConversationLogEntry) (ConversationLogEntry, error)
    ListConversations(ctx context.Context, slug string, stage string) ([]ConversationLogEntry, error)
}
```

Domain types in `internal/storage/`:

```go
type ConversationLogEntry struct {
    ID            string
    Stage         SpecStage
    Version       int32
    IsAmend       bool
    Exchanges     []ConversationExchange
    ExchangeCount int32
    Date          time.Time
}

type ConversationExchange struct {
    Role          string  // "probe" or "response"
    Content       string
    Stage         string
    Sequence      int32
    DecisionPoint bool
}
```

### Write Path

`RecordConversation` in `memgraph/conversation.go`:

1. Verify spec exists via `GetSpec`, capture current version. The ConversationLog's `version` is always set server-side to the spec's current version at record time (not caller-supplied). This avoids stale-version bugs and simplifies the RPC contract.
2. Find the most recent ChangeLog for this stage+version, sorted by `date` descending, first result (target for `EXPLAINS` edge). If no matching ChangeLog exists (e.g., the stage store failed or hasn't run yet), the `EXPLAINS` edge is omitted — the ConversationLog is still created and linked via `AUTHORED_VIA`/`CONTINUES`. This makes conversation capture resilient to partial failures.
3. Find the current `CONTINUES` chain tail — the ConversationLog with no outgoing `CONTINUES` edge, scoped to the spec via `AUTHORED_VIA` traversal.
4. Create ConversationLog node with `newID("cvl")`.
5. Create edges:
   - `AUTHORED_VIA` from Spec (only if this is the first ConversationLog for this spec — check via `OPTIONAL MATCH`)
   - `EXPLAINS` to the target ChangeLog (if one was found in step 2)
   - `CONTINUES` from the previous chain tail (if one exists)
6. All steps wrapped in `RunInTransaction` per ADR-004. All queries use project-scoped `BELONGS_TO` pattern.

`RecordConversation` is called *after* the stage store (e.g., `StoreShapeOutput`), not inside it. The stage store creates the ChangeLog; then `RecordConversation` links to it. This keeps existing stage write paths unchanged.

### Read Path

**GetSpec augmentation:**

The existing `GetSpec` in memgraph adds a query to fetch ConversationLog nodes in chain order:

```cypher
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
      -[:AUTHORED_VIA]->(first:ConversationLog)
OPTIONAL MATCH path = (first)-[:CONTINUES*0..10]->(log)
RETURN log ORDER BY length(path)
```

The `*0..10` upper bound provides a safe traversal limit (a spec going through all 5 stages plus 5 amends). Returns the full chain in narrative order. Populated into the `Spec` proto response's `conversation_logs` field.

**ListConversations:**

```cypher
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
      -[:AUTHORED_VIA]->(first:ConversationLog)
OPTIONAL MATCH path = (first)-[:CONTINUES*0..10]->(log)
WHERE ($stage = '' OR log.stage = $stage)
RETURN log ORDER BY length(path)
```

### CLI Commands

```bash
specgraph conversation record <slug> --stage shape --json-file /tmp/conv.json
specgraph conversation list <slug> [--stage shape]
```

The `record` command reads the JSON file (same pattern as `specgraph shape --json-file`), calls `RecordConversation` RPC. The `list` command calls `ListConversations` and renders via the render package.

### Render Package

New `internal/render/conversation.go`:

Accepts proto `ConversationLog` messages and returns markdown. Format:

```markdown
### Authoring Conversation (shape, v3)

**[1] Scope** (decision)
> **Probe:** What should be in scope?
> **User:** API endpoints and storage, not CLI — we want to keep CLI changes in a follow-up.

**[2] Approaches**
> **Probe:** What approaches did you consider?
> **User:** Event-driven vs polling. Chose event-driven for latency.
```

Used by both `specgraph show` (spgr-0dg) and the web UI (spgr-zn1). The web UI consumes the proto `ConversationLog` directly for its own rendering.

### Skill Integration

Each authoring skill (spark, shape, specify, decompose, approve) gets a new instruction block at the end of its stage completion flow. After the skill calls the existing stage command, it writes conversation exchanges and calls `conversation record`.

**Instruction addition (example for Shape):**

> After calling `specgraph shape <slug> --json-file ...`, record the authoring conversation:
>
> 1. For each probe you asked and the user's substantive response, create an exchange pair.
> 2. Mark exchanges where the user chose between alternatives as `decision_point: true`.
> 3. Skip meta-conversation ("does that make sense?", "yes, continue").
> 4. Write to JSON and call: `specgraph conversation record <slug> --stage shape --json-file /tmp/conv-shape.json`

**Approve stage:** Records the approval decision and any conditions as a single exchange — probe = "review checklist", response = approval rationale.

### Interaction with ChangeLog Nodes

ConversationLog and ChangeLog are independent siblings hanging off the Spec, connected via `EXPLAINS` edges. This means:

- ChangeLog continues to be created by the storage layer during mutations (unchanged).
- ConversationLog is created after the stage store completes, linking to the ChangeLog that was just created.
- Both node types share `stage` and `version` fields for correlation.
- The `EXPLAINS` edge provides a direct traversal path: "show me the conversation that led to this specific change."

## Indexes

```cypher
CREATE INDEX ON :ConversationLog(id);
CREATE INDEX ON :ConversationLog(date);
```

Stage-based filtering uses the `AUTHORED_VIA` traversal from Spec, so a `stage` index is not needed for V1. If cross-spec conversation queries are added later, an index on `ConversationLog(stage)` can be introduced.

## Testing

- **Unit tests:** ConversationExchange JSON serialization, render output formatting.
- **Integration tests (memgraph):** Full write/read cycle — `RecordConversation` creates node + edges, `ListConversations` returns in chain order, `GetSpec` includes conversation data. Verify `EXPLAINS` points to correct ChangeLog. Verify `CONTINUES` chain ordering across stages and amends.
- **Integration edge cases:**
  - Recording a conversation when no matching ChangeLog exists for the stage+version — ConversationLog is created without `EXPLAINS` edge.
  - Recording a conversation for a nonexistent spec — returns `ErrSpecNotFound`.
  - Concurrent `RecordConversation` calls for the same spec — the `CONTINUES` chain tail detection must be inside the transaction to avoid race conditions on tail pointer.
  - Multiple ChangeLogs for the same stage+version — `EXPLAINS` links to the most recent by `date`.
- **E2E test:** Stage command -> conversation record -> `specgraph show` verifies conversation appears in output.

## Migration & Backward Compatibility

- Purely additive — existing specs have no ConversationLog nodes. `GetSpec` returns empty `conversation_logs`, `specgraph show` renders no conversation section.
- No data migration needed.
- Skills that haven't been updated simply don't call `conversation record` — no breakage.

## Out of Scope (V1)

- Retroactive conversation capture for existing specs.
- Conversation editing or deletion commands.
- Cross-spec conversation queries ("show me all scope decisions") — possible via Cypher but no dedicated CLI command.
- Hook-based fallback capture — the standalone CLI command exists so this can be added later.
- Conversation data in content hash computation — ConversationLog is metadata, not substantive spec content.
