# ListChanges RPC Design

**Date:** 2026-03-29
**Bead:** spgr-fn5 (Expose ListChanges via ConnectRPC API)
**Status:** Draft

## Problem

The storage layer has `ChangeLogBackend.ListChanges(slug, filter)` but there's no RPC to expose it. Consumers (CLI, web UI, Gastown, impact notifications) need changelog access to display spec history, track versions, and trigger downstream workflows.

## Decision Summary

| Question | Decision |
|----------|----------|
| Proto service | SpecService (alongside GetSpec, ListSpecs, UpdateSpec) |
| CLI output | Both markdown renderer + `--json` |
| Filters | Full filter on RPC + CLI: checkpoints-only, since-version, limit |

## Proto Changes

Add to `proto/specgraph/v1/spec.proto`. The `FieldChange` message already exists (line 48).

**New message:**

```protobuf
message ChangeLogEntry {
  string id = 1;
  int32 version = 2;
  string stage = 3;
  string content_hash = 4;
  bool checkpoint = 5;
  string summary = 6;
  string reason = 7;
  repeated FieldChange changes = 8;
  google.protobuf.Timestamp date = 9;
}

message ListChangesRequest {
  string slug = 1;
  bool checkpoints_only = 2;
  int32 since_version = 3;
  int32 limit = 4;
}

message ListChangesResponse {
  repeated ChangeLogEntry entries = 1;
}
```

**New RPC in SpecService:**

```protobuf
rpc ListChanges(ListChangesRequest) returns (ListChangesResponse);
```

## Architecture

### Files

| File | Responsibility |
|------|---------------|
| `proto/specgraph/v1/spec.proto` | Add `ChangeLogEntry`, `ListChangesRequest/Response`, `ListChanges` RPC |
| `internal/server/spec_handler.go` | Add `ListChanges` method to existing spec handler |
| `internal/server/convert_changelog.go` | New. `changeLogEntryToProto` converter (domain → proto) |
| `internal/server/convert_changelog_test.go` | New. Converter unit tests |
| `internal/auth/permissions.go` | Add `SpecServiceListChangesProcedure: "spec:read"` |
| `internal/render/changelog.go` | New. Markdown renderer for changelog entries |
| `internal/render/changelog_test.go` | New. Renderer unit tests |
| `cmd/specgraph/changes.go` | New. `specgraph changes <slug>` CLI command |

### Handler

Add `ListChanges` to the existing spec handler (`internal/server/spec_handler.go`). The handler follows the standard scoper pattern:

1. Calls `scopeStore(ctx, h.scoper)` to get the project-scoped store
2. Extracts `slug` from request
3. Builds `storage.ChangeLogFilter` from request fields
4. Calls `store.ListChanges(ctx, slug, filter)`
5. Converts `[]*storage.ChangeLogEntry` → `[]*specgraphv1.ChangeLogEntry` via converter
6. Returns `ListChangesResponse`

Error mapping follows existing patterns:

- `storage.ErrSpecNotFound` → `connect.CodeNotFound`
- Other errors → `connect.CodeInternal`

### Converter

New file `internal/server/convert_changelog.go`:

- `changeLogEntryToProto(e *storage.ChangeLogEntry) *specgraphv1.ChangeLogEntry` — maps all fields including `time.Time` → `timestamppb.New()`
- `fieldChangesToProto(changes []storage.FieldChange) []*specgraphv1.FieldChange` — maps field change slices

The `FieldChange` proto message already exists at `spec.proto:48` with fields `field`, `old_value`, `new_value`.

### Permissions

Add to `rpcPermissions` map in `internal/auth/permissions.go`:

```go
specgraphv1connect.SpecServiceListChangesProcedure: "spec:read",
```

The existing `TestPermissionTable_Completeness` test will catch this if missing.

### CLI Command

New file `cmd/specgraph/changes.go`:

```text
specgraph changes <slug> [--checkpoints] [--since-version N] [--limit N] [--json]
```

Follows the pattern of `specgraph show`: calls RPC, renders markdown or prints JSON based on `--json` flag. Uses `printJSON` from `output.go` for JSON mode.

### Renderer

New file `internal/render/changelog.go`:

`RenderChanges(entries []*specgraphv1.ChangeLogEntry) string`

Output format (markdown):

```text
## v3 — shape (checkpoint)
**2026-03-28** | Hash: a1b2c3d4

Summary: Refined scope after feedback

| Field  | Old       | New            |
|--------|-----------|----------------|
| intent | Build X   | Build X (v2)   |
| stage  | spark     | shape          |

## v2 — spark
**2026-03-27** | Hash: e5f6g7h8
...
```

Empty entries → "No changelog entries found."

## Testing

| Test | File | Coverage |
|------|------|----------|
| Converter | `convert_changelog_test.go` | All fields mapped, empty changes, nil input |
| Renderer | `changelog_test.go` | Entries with changes, checkpoint flag, empty list |
| Handler | Existing `spec_handler_test.go` pattern | Mock store, slug validation, filter pass-through, not-found error |
| Permissions | `TestPermissionTable_Completeness` | Automatically catches missing RPC entries |

## Not Building

- No `GetChangeLogEntry` (single entry by ID) — not needed yet
- No `CompareVersions` — can be built from `ListChanges` client-side
- No streaming — changelog lists are small (bounded by spec lifetime)
