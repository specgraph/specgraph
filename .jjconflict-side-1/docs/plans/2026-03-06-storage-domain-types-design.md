# Storage Domain Types & Decision Promotion Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove proto type coupling from storage interfaces and promote ShapeOutput decisions to first-class graph nodes before Slice 4 adds more storage interfaces.

**Beads:** spgr-q5h (domain types), spgr-0zd (decision promotion)

---

## Problem

### Proto Coupling in Backend Interface

The core `Backend` interface (`internal/storage/storage.go`) returns `*specv1.Spec` directly. `DecisionBackend` similarly returns `*specv1.Decision` and accepts `specv1.DecisionStatus`. This couples 13 storage files to generated proto types.

`AuthoringBackend` already uses domain types (`storage.SparkOutput`, `storage.ShapeOutput`, etc.) — this refactor extends that pattern to the remaining interfaces.

### ShapeOutput Decisions as Strings

`ShapeOutput.Decisions` is `[]string`, stored as a JSON blob inside the Spec node. Per ADR-003, decisions should be first-class Decision graph nodes with `DECIDED_IN` edges. The `DecisionBackend` already supports full CRUD — the gap is that Shape stage doesn't create Decision nodes.

---

## Design

### Part 1: Domain Spec and Decision Types

Create domain types in `internal/storage/` that mirror the proto messages but belong to the storage layer:

```go
// storage/spec_domain.go
type Spec struct {
    ID         string
    Slug       string
    Intent     string
    Stage      string
    Priority   string
    Complexity string
    Version    int32
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

```go
// storage/decision.go (extend existing file)
type DecisionStatus string

const (
    DecisionStatusActive     DecisionStatus = "active"
    DecisionStatusSuperseded DecisionStatus = "superseded"
    DecisionStatusDeprecated DecisionStatus = "deprecated"
)

type Decision struct {
    ID           string
    Slug         string
    Title        string
    Status       DecisionStatus
    Decision     string
    Rationale    string
    SupersededBy string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Converter functions** live in `internal/server/` (handler layer), converting between `storage.Spec` and `specv1.Spec` at the RPC boundary. This is the same pattern AuthoringHandler already uses for authoring domain types.

### Part 2: Swap Interfaces

Update `Backend` to return `*storage.Spec`:

```go
type Backend interface {
    CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*Spec, error)
    GetSpec(ctx context.Context, slug string) (*Spec, error)
    ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*Spec, error)
    UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity *string) (*Spec, error)
    Close(ctx context.Context) error
}
```

Update `DecisionBackend` to return `*storage.Decision`:

```go
type DecisionBackend interface {
    CreateDecision(ctx context.Context, slug, title, decision, rationale string) (*Decision, error)
    GetDecision(ctx context.Context, slug string) (*Decision, error)
    ListDecisions(ctx context.Context, status DecisionStatus, limit int) ([]*Decision, error)
    UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus, decision, rationale, supersededBy *string) (*Decision, error)
}
```

### Part 3: Decision Promotion

**Proto change** — add `DecisionInput` to `authoring.proto`:

```protobuf
message DecisionInput {
    string slug = 1;
    string title = 2;
    string decision = 3;
    string rationale = 4;
}
```

Change `ShapeOutput.decisions` from `repeated string` to `repeated DecisionInput`.

**Domain type change** — update `storage.ShapeOutput`:

```go
type DecisionInput struct {
    Slug      string `json:"slug"`
    Title     string `json:"title"`
    Decision  string `json:"decision"`
    Rationale string `json:"rationale"`
}

type ShapeOutput struct {
    // ... existing fields ...
    Decisions []DecisionInput `json:"decisions,omitempty"`
}
```

**Storage logic** — in `StoreShapeOutput`, after storing the JSON blob:

1. Iterate `output.Decisions`
2. For each, call `CreateDecision(ctx, input.Slug, input.Title, input.Decision, input.Rationale)`
3. Add a `DECIDED_IN` edge from the Decision node to the Spec node
4. If a Decision with that slug already exists, skip creation (idempotent)

This requires `StoreShapeOutput` to have access to `DecisionBackend`. The Memgraph `Store` already implements both `AuthoringBackend` and `DecisionBackend`, so this is just using `s.CreateDecision()` internally.

---

## Migration

No data migration needed:

- Memgraph stores node properties as raw values, not proto bytes
- `specFromNode` already reads from `node.Props` — just change the return type
- Proto change is breaking but pre-1.0 with no external consumers
- Existing decision strings in stored ShapeOutput JSON will be ignored (old format)

## Testing

- Unit tests for `storage.Spec` <-> `specv1.Spec` converter functions
- Update Memgraph integration tests: assert `*storage.Spec` / `*storage.Decision` returns
- Update handler tests: verify proto conversion at RPC boundary
- Decision promotion: test `StoreShapeOutput` with decisions creates Decision nodes + `DECIDED_IN` edges
- E2E: shape a spec with structured decisions, then `decision list` shows them

## Task Order

1. Add domain types (`storage.Spec`, `storage.Decision`, `storage.DecisionStatus`) — pure addition
2. Add converter functions in handler layer — pure addition
3. Swap `Backend` interface to domain types, update Memgraph impl
4. Swap `DecisionBackend` interface to domain types, update Memgraph impl
5. Update all handlers to use converters
6. Proto: add `DecisionInput`, change `ShapeOutput.decisions` field type
7. Update `storage.ShapeOutput.Decisions` domain type
8. Implement decision promotion in `StoreShapeOutput`
9. Update handler for structured decisions (proto <-> domain conversion)
10. Update tests throughout (integration + E2E)
11. Final verification: `task test`, `task lint`, `buf lint`
