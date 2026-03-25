# Show Stage Detail Design Spec

**Bead:** spgr-0dg — `specgraph show`: compact markdown output missing stage detail
**Date:** 2026-03-24

## Problem

`specgraph show` renders basic metadata (slug, intent, stage, priority, version) but not the authoring stage outputs (Spark, Shape, Specify, Decompose). Users must fall back to `--json` and manually reconstruct readable output. The render package should expand these sections for both CLI and web UI consumption.

## Current State

Authoring outputs are stored as JSON properties on Spec graph nodes (`spark_output`, `shape_output`, `specify_output`, `decompose_output`). The `GetSpec` Cypher query already retrieves them. However:

1. The `GetSpec` query in `memgraph.go` **does not return** the stage output columns — they must be added to the RETURN clause (positions 14-17). `recordToSpec()` currently only parses 14 columns
2. The proto `Spec` message has **no fields** for stage outputs
3. `specToProto()` in `convert.go` has **no conversion** for them
4. `render.Spec()` has **no rendering** for them

## Design

Extend the existing GetSpec → proto → render pipeline. No new RPCs, commands, or services.

### Proto: Add stage output fields to Spec message

In `proto/specgraph/v1/spec.proto`, add 4 optional fields to the `Spec` message:

```protobuf
SparkOutput spark_output = 17;
ShapeOutput shape_output = 18;
SpecifyOutput specify_output = 19;
DecomposeOutput decompose_output = 20;
```

These message types already exist in `authoring.proto`. Import `authoring.proto` in `spec.proto` (already imported for `ConversationLog`).

### Domain: Add stage output fields to storage.Spec

In `internal/storage/spec_domain.go`, add pointer fields:

```go
SparkOutput     *SparkOutput
ShapeOutput     *ShapeOutput
SpecifyOutput   *SpecifyOutput
DecomposeOutput *DecomposeOutput
```

### Storage: Extend GetSpec query and parse stage outputs in recordToSpec()

In `internal/storage/memgraph/memgraph.go`:

1. **Extend the `GetSpec` query** (line 268) to add `s.spark_output, s.shape_output, s.specify_output, s.decompose_output` to the RETURN clause (positions 14-17). Also extend `BatchGetSpecs` and `ListSpecs` queries for consistency.

2. **Extend `recordToSpecOffset()`** to parse the four JSON strings at the new positions. No existing unmarshal functions exist — use `json.Unmarshal` to deserialize each non-empty string into the corresponding domain type (`storage.SparkOutput`, `storage.ShapeOutput`, etc.). The domain types already have `json` struct tags. If the JSON string is empty, leave the pointer nil.

Create unmarshal helper functions in `authoring.go` (e.g., `unmarshalSparkOutput(raw string) (*storage.SparkOutput, error)`) for reuse.

### Server: Extend specToProto()

In `internal/server/convert.go`, add domain-to-proto conversion functions for each stage output: `sparkOutputToProto`, `shapeOutputToProto`, `specifyOutputToProto`, `decomposeOutputToProto`. These are the **reverse** of the existing `*ToDomain` functions in `authoring_handler.go` (lines 644-789). No existing domain-to-proto mappers exist — all four must be written from scratch.

### Render: New authoring.go with stage section renderers

Create `internal/render/authoring.go` with functions accepting proto types and returning markdown strings:

- `SparkSection(o *specv1.SparkOutput) string` — seed (blockquote), signal (blockquote), scope sniff, kill test, questions (bulleted list)
- `ShapeSection(o *specv1.ShapeOutput) string` — scope in/out (bulleted lists), approaches (H3 each, chosen marked), risks (bulleted), success criteria (must/should/won't sections), decisions (bulleted)
- `SpecifySection(o *specv1.SpecifyOutput) string` — interfaces (H3 per section with contract/notes), verify criteria (table: description, strategy, automated), invariants (bulleted), file touches (table: path, reason, create/modify)
- `DecomposeSection(o *specv1.DecomposeOutput) string` — strategy (blockquote), slices (H3 each with intent, acceptance criteria, dependencies, estimated complexity)

Each function returns empty string for nil input. Sections only appear in `render.Spec()` when the corresponding output is non-nil (i.e., the spec has passed through that stage).

### Render: Update render.Spec() to include stage sections

After the existing metadata table and notes section, append:

```go
b.WriteString(SparkSection(s.SparkOutput))
b.WriteString(ShapeSection(s.ShapeOutput))
b.WriteString(SpecifySection(s.SpecifyOutput))
b.WriteString(DecomposeSection(s.DecomposeOutput))
b.WriteString(ConversationLogList(s.ConversationLogs))
```

### Output Format

```markdown
# my-feature-spec

> Build a widget factory

| Field | Value |
|-------|-------|
| Stage | shape |
| Priority | p1 |
| Complexity | medium |
| Version | 3 |
| Lifecycle | task |

## Notes

Some context about this spec...

## Spark

> **Seed:** Build a widget factory for production use
>
> **Signal:** High demand from enterprise customers

**Scope Sniff:** greenfield
**Kill Test:** No existing widget system to migrate from

**Questions:**

- What throughput is expected?
- Which widget types are in scope?

## Shape

**Scope In:**

- API layer
- Storage backend
- CLI commands

**Scope Out:**

- Web UI
- Authentication

### Approaches

#### 1. Modular plugin architecture (chosen)

Plugin-based design with hot-reloading for widget types...

#### 2. Monolithic service

Single service handling all widget operations...

### Risks

- Performance under high concurrent load
- Plugin compatibility across versions

### Success Criteria

**Must:**

- Create, read, update, delete widgets via API
- Sub-100ms p99 latency

**Should:**

- Support batch operations
- Provide CLI autocomplete

**Won't:**

- Real-time collaboration
- Widget marketplace

### Decisions

- Use protobuf for wire format
- PostgreSQL for persistence

## Specify

...

## Decompose

...

## Conversations

...
```

## Testing

- **Unit tests:** Render functions for each stage output (nil handling, field rendering, nested types)
- **Integration tests:** GetSpec returns stage outputs correctly (create spec, store spark output, verify GetSpec populates it)
- **No E2E changes needed** — existing `specgraph show` command just renders more data

## Files Changed

| File | Action | What |
|------|--------|------|
| `proto/specgraph/v1/spec.proto` | Modify | Add 4 stage output fields (17-20) |
| `gen/specgraph/v1/spec.pb.go` | Regenerate | Proto codegen |
| `internal/storage/spec_domain.go` | Modify | Add 4 pointer fields to Spec struct |
| `internal/storage/memgraph/memgraph.go` | Modify | Add stage output columns to GetSpec/BatchGetSpecs/ListSpecs queries; parse JSON in recordToSpec() |
| `internal/storage/memgraph/authoring.go` | Modify | Add unmarshal helper functions for stage outputs |
| `internal/server/convert.go` | Modify | Add stage output conversion in specToProto() |
| `internal/render/authoring.go` | Create | Stage section renderers |
| `internal/render/authoring_test.go` | Create | Unit tests for renderers |
| `internal/render/spec.go` | Modify | Call stage section renderers |
