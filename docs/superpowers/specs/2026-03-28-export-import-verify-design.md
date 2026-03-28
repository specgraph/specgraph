# Full Text Export/Import with Verification

**Bead:** spgr-m56
**Date:** 2026-03-28
**Status:** Approved

## Context

SpecGraph stores all project data in Memgraph with no way to back up, migrate,
or audit a project's state. A reliable export/import pipeline is needed for
disaster recovery, portability across instances, and point-in-time archival.
The design must support future upsert/merge semantics and alternative backends.

## Goals

1. Per-project export to a versioned, signed JSON document
2. Import into fresh or force-wiped projects with referential integrity validation
3. Round-trip verification (export → import → re-export → diff)
4. Streaming-friendly field ordering and incremental HMAC computation
5. Backend-agnostic domain logic exposed through ConnectRPC

## Non-Goals

- Server-wide export (trivially wraps per-project later)
- Upsert/merge import (V1 is fresh-only + `--force`; schema designed for future merge)
- Cross-version schema migration (only schema_version 1 exists)
- NDJSON or alternative formats (JSON V1; extensible later)
- Incremental/differential exports
- Scheduled/automated exports (cron can call the CLI)

---

## Architecture

### New Components

| Component | Purpose |
|-----------|---------|
| `proto/specgraph/v1/export.proto` | ExportService — `ExportProject`, `ImportProject`, `VerifyExport` RPCs |
| `internal/export/` | Domain logic — encoder, decoder, verifier using storage interfaces |
| `internal/server/export_handler.go` | ConnectRPC handler wrapping the domain layer |
| `cmd/specgraph/export.go` | CLI commands — `export`, `import`, `verify` |

### Data Flow

**Export:**

```text
CLI: specgraph export <slug> -o backup.json
  → ConnectRPC: ExportService.ExportProject(project_slug)
    → export.Engine.Export(ctx, slug)
      → storage: ListSpecs, GetSpec, ListEdges, GetConstitution, ...
      → serialize to ExportDocument (versioned JSON)
      → compute HMAC-SHA256 incrementally over data bytes
    ← returns ExportDocument bytes
  ← CLI writes to file
```

**Import:**

```text
CLI: specgraph import backup.json [--force] [--require-signature]
  → ConnectRPC: ExportService.ImportProject(data, force, require_signature)
    → export.Engine.Import(ctx, data, force, requireSig)
      → verify HMAC if signature present and key configured
      → validate schema version
      → validate referential integrity (all edges reference existing entities)
      → if !force && project exists with data → error
      → if force → delete project data first
      → create entities in dependency order
    ← returns ImportResult (counts, warnings)
```

**Verify:**

```text
CLI: specgraph verify backup.json
  → ConnectRPC: ExportService.VerifyExport(data)
    → export.Engine.Verify(ctx, data)
      → re-export current project state
      → deep-compare entity-by-entity against provided data
    ← returns VerifyResult (match bool, per-type mismatch details)
```

### Import Ordering

Entities are created in dependency order so referential integrity holds at
every step:

1. Project
2. Constitution
3. Specs (nodes only — no edges yet)
4. Decisions
5. Slices (reference parent specs)
6. Edges (all types — all referenced nodes now exist)
7. Findings (reference specs)
8. ChangeLogs (reference specs)
9. ConversationLogs (reference specs + stages)
10. SyncMappings (reference specs)
11. ExecutionEvents (reference specs)

Claims are **not exported** — they are ephemeral leases with expiry times that
become meaningless on restore. Active claims would immediately be stale.

### New Storage Interface Methods Required

The current storage interfaces only support per-spec reads for several entity
types. Export needs project-wide enumeration. New methods:

| Interface | New Method | Purpose |
|-----------|-----------|---------|
| `FindingsBackend` | `ListAllFindings(ctx) ([]*AnalyticalFinding, error)` | All findings across all specs |
| `ChangeLogBackend` | `ListAllChanges(ctx) ([]*ChangeLogEntry, error)` | All changelogs across all specs |
| `ConversationBackend` | `ListAllConversations(ctx) ([]*ConversationLogEntry, error)` | All conversation logs |
| `ProjectBackend` | `WipeProjectData(ctx) error` | Delete all project data for `--force` import |

The `ListSpecs` and `ListDecisions` methods already support full enumeration
(pass empty filters and `limit=0`). `ListSlices` requires a parent slug, so
export iterates per-spec using the existing method (no new interface needed).
`ListSyncMappings` already supports `("", "")` for all mappings.
`GetFullGraph` returns all user-facing edges. `GetExecutionEvents` requires a
slug, so export iterates per-spec (no new interface needed — pass `limit=0`
for all events).

`WipeProjectData` deletes all nodes with `BELONGS_TO` edges to the project
(specs, decisions, slices, findings, changelogs, conversations, sync mappings,
execution events, constitution) and all edges between them. The Project node
itself is preserved.

---

## Export Schema

### Versioned JSON Structure

Field ordering supports streaming — metadata first, then data in dependency
order, then the signature (which must come last because it covers `data`):

```json
{
  "schema_version": 1,
  "exported_at": "2026-03-28T00:00:00Z",
  "specgraph_version": "0.2.0",
  "project_slug": "my-project",
  "data": {
    "project": {},
    "constitution": {},
    "specs": [],
    "decisions": [],
    "slices": [],
    "edges": [],
    "findings": [],
    "changelogs": [],
    "conversations": [],
    "sync_mappings": [],
    "execution_events": []
  },
  "signature": {
    "algorithm": "hmac-sha256",
    "digest": "a1b2c3..."
  }
}
```

### Schema Version Contract

- `schema_version: 1` is the initial and only version
- Import refuses files with `schema_version > supported`
- New versions may add fields (backward-compatible) or restructure (breaking)
- The version number selects the deserializer

### Entity Keying (Future Merge Support)

All entities are keyed by stable identifiers to support future upsert:

- Specs: `slug`
- Decisions: `slug`
- Slices: `slug` (composite: `parent-slug/slice-id`)
- Edges: `from_slug` + `to_slug` + `type` (composite key)
- Findings: `spec_slug` + `pass_type` + `summary` (composite key)
- ChangeLogs: `spec_slug` + `version`
- ConversationLogs: `spec_slug` + `stage` + `date`
- SyncMappings: `spec_slug` + `adapter`

---

## Signature & Integrity

### HMAC-SHA256

- Computed over the canonical JSON encoding of the `data` field
- Key source (priority order):
  1. Dedicated `export.signing_key` in `config.yaml`
  2. First API key from `auth.api_keys` config
  3. No key → export is unsigned (signature field omitted)

### Signing Flow

Export computes HMAC incrementally as `data` bytes are written to the output
stream. This avoids buffering the entire serialized data in memory.

### Verification on Import

| Scenario | Behavior |
|----------|----------|
| Signature present, key configured | Verify HMAC. Reject on mismatch. |
| Signature present, no key configured | Warn: "cannot verify signature, no key configured". Proceed. |
| No signature, `--require-signature` | Reject: "unsigned export and --require-signature specified". |
| No signature, no flag | Proceed without verification. |

### Referential Integrity Validation

Before any writes, the importer validates:

- Every edge's `from_slug` and `to_slug` exist in `specs` or `decisions`
- Every slice's `parent_slug` exists in `specs`
- Every finding's `spec_slug` exists in `specs`
- Every changelog's `spec_slug` exists in `specs`
- Every conversation's `spec_slug` exists in `specs`
- Every sync mapping's `spec_slug` exists in `specs`
- Every execution event's `spec_slug` exists in `specs`

Validation fails fast with a clear error listing all broken references.

---

## Streaming & Large Exports

### Export Streaming

The encoder uses `json.Encoder` with streaming writes:

1. Write envelope fields (`schema_version`, `exported_at`, `specgraph_version`,
   `project_slug`)
2. Open `data` object — start feeding bytes to `hash.Hash` via `io.TeeWriter`
3. For each entity type: open array, write entities one at a time from storage,
   close array
4. Close `data` object — finalize HMAC
5. Write `signature` object with the computed digest (signature must be last in
   the JSON because it covers all of `data`)

Memory usage is proportional to the largest single entity, not the full graph.

### Import Buffering

V1 reads the full file into memory for HMAC verification before deserializing.
This works for exports up to ~100MB (tens of thousands of specs). Future: two-pass
approach (hash pass + deserialize pass) for larger exports.

---

## CLI Commands

```bash
# Export to file (defaults to stdout if no -o)
specgraph export <project-slug> -o backup.json

# Import (refuses if project exists with data)
specgraph import backup.json

# Import with force (wipes existing project data first)
specgraph import backup.json --force

# Import requiring valid signature
specgraph import backup.json --require-signature

# Verify export matches current server state
specgraph verify backup.json
```

---

## Proto Service

```protobuf
service ExportService {
  rpc ExportProject(ExportProjectRequest) returns (ExportProjectResponse);
  rpc ImportProject(ImportProjectRequest) returns (ImportProjectResponse);
  rpc VerifyExport(VerifyExportRequest) returns (VerifyExportResponse);
}

message ExportProjectRequest {
  string project_slug = 1;
}

message ExportProjectResponse {
  bytes data = 1;
}

message ImportProjectRequest {
  bytes data = 1;
  bool force = 2;
  bool require_signature = 3;
}

message ImportProjectResponse {
  ImportResult result = 1;
}

message ImportResult {
  int32 specs_created = 1;
  int32 decisions_created = 2;
  int32 slices_created = 3;
  int32 edges_created = 4;
  int32 findings_created = 5;
  int32 changelogs_created = 6;
  int32 conversations_created = 7;
  int32 sync_mappings_created = 8;
  int32 execution_events_created = 9;
  repeated string warnings = 10;
}

message VerifyExportRequest {
  bytes data = 1;
  // Optional — if empty, inferred from data.project_slug after parsing.
  string project_slug = 2;
}

message VerifyExportResponse {
  bool match = 1;
  repeated EntityDiff diffs = 2;
}

message EntityDiff {
  string entity_type = 1;
  int32 matched = 2;
  int32 missing = 3;
  int32 extra = 4;
  repeated string details = 5;
}
```

---

## Import Conflict Handling

### V1: Fresh-Only + Force

| Scenario | Behavior |
|----------|----------|
| Project doesn't exist | Create project, import all entities |
| Project exists, no data | Import into empty project |
| Project exists with data, no `--force` | Error: "project has existing data, use --force" |
| Project exists with data, `--force` | Delete all project data, then import |

### Future: Upsert/Merge

The schema's entity keying (slug-based) and deterministic composite keys for
edges/findings are designed so that a future `--merge` mode can:

- Match entities by key
- Update if changed (comparing content hashes or field-level diffs)
- Create if missing
- Optionally flag conflicts for manual resolution

This is explicitly out of scope for V1 but the schema does not prevent it.

---

## Testing Strategy

### Unit Tests (`internal/export/`)

- Encoder: mock backend → verify JSON structure, field ordering, signature
- Decoder: known JSON → verify parsed entities, referential integrity catches
  broken refs
- Schema version: reject future versions, accept current
- Signature: valid HMAC passes, tampered data fails, missing key warns

### Integration Tests (build tag `integration`)

- Round-trip: export from real Memgraph → import into empty project → re-export
  → byte-compare `data` sections
- Force import: create project with data → import `--force` → verify old data
  gone, new data present
- Conflict: import into existing project without `--force` → verify error
- Signature round-trip: export with key → import with key → passes; tamper →
  fails

### E2E Test (build tag `e2e`)

- Full CLI pipeline: `specgraph create` specs + edges → `specgraph export` →
  nuke project → `specgraph import` → `specgraph verify` → passes

---

## File Inventory

**New files:**

| File | Type |
|------|------|
| `proto/specgraph/v1/export.proto` | Proto service definition |
| `internal/export/engine.go` | Core export/import/verify logic |
| `internal/export/encoder.go` | Streaming JSON encoder with incremental HMAC |
| `internal/export/decoder.go` | JSON decoder with validation |
| `internal/export/schema.go` | Export document types and version handling |
| `internal/export/engine_test.go` | Unit tests |
| `internal/export/integration_test.go` | Integration tests (build tag) |
| `internal/server/export_handler.go` | ConnectRPC handler |
| `internal/server/export_handler_test.go` | Handler tests |
| `cmd/specgraph/export.go` | CLI commands (export, import, verify) |
| `cmd/specgraph/export_test.go` | CLI tests |
| `e2e/api/export_test.go` | E2E round-trip test |

**Modified files:**

| File | Changes |
|------|---------|
| `cmd/specgraph/serve.go` | Register ExportService handler |
| `internal/config/config.go` | Add `export.signing_key` config field |
| `internal/storage/findings.go` | Add `ListAllFindings` to interface |
| `internal/storage/changelog.go` | Add `ListAllChanges` to interface |
| `internal/storage/conversation.go` | Add `ListAllConversations` to interface |
| `internal/storage/project.go` | Add `WipeProjectData` to interface |
| `internal/storage/memgraph/findings.go` | Implement `ListAllFindings` |
| `internal/storage/memgraph/changelog.go` | Implement `ListAllChanges` |
| `internal/storage/memgraph/conversation.go` | Implement `ListAllConversations` |
| `internal/storage/memgraph/project.go` | Implement `WipeProjectData` |
