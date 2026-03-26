# Idempotent Push: FindOrCreate for Sync Adapters

**Date**: 2026-03-26
**Status**: Approved
**Bead**: spgr-ylq

## Problem

When `adapter.Push` succeeds but `store.CreateSyncMapping` fails, an orphaned external item is created (a GitHub issue or bead with no corresponding sync mapping in the graph). On the next sync run, `GetSyncMapping` returns `ErrSyncMappingNotFound`, so the handler calls `Push` again — creating a duplicate external item.

The current retry-once logic in `sync_handler.go` mitigates transient store failures but does not prevent duplicates when the store is persistently unavailable or when the process crashes between Push and CreateSyncMapping.

## Decision

Add a `FindOrCreate` method to the `Adapter` interface. Each adapter searches for an existing external item by title convention before creating a new one. The sync handler calls `FindOrCreate` instead of `Push`.

## Adapter Interface

```go
type Adapter interface {
    Name() storage.SyncAdapterType
    Available(ctx context.Context) error
    Push(ctx context.Context, spec *storage.Spec) (externalID string, err error)
    FindOrCreate(ctx context.Context, spec *storage.Spec) (externalID string, created bool, err error)
    Pull(ctx context.Context, externalID string) (status string, err error)
}
```

`Push` remains on the interface for backward compatibility. `FindOrCreate` is the primary method the handler uses. Each adapter's `FindOrCreate` calls `Push` internally when no existing item is found.

## Adapter Implementations

### GitHubAdapter.FindOrCreate

1. Search: `gh issue list --search "in:title [spec] <slug>" --repo <repo> --json number,url --limit 1`
2. If result found: return existing issue URL, `created=false`
3. If no result: call `Push(ctx, spec)`, return new URL, `created=true`

### BeadsAdapter.FindOrCreate

1. Search: `bd search "[spec] <slug>" --json --limit 1`
2. If result found: return existing bead ID, `created=false`
3. If no result: call `Push(ctx, spec)`, return new ID, `created=true`

## Handler Change

In `syncWithAdapter` (sync_handler.go:145), replace:

```go
externalID, pushErr := adapter.Push(ctx, spec)
```

With:

```go
externalID, created, pushErr := adapter.FindOrCreate(ctx, spec)
```

The `created` bool informs logging and the result message. When `created=false`, the handler still calls `CreateSyncMapping` to heal the orphan. The existing `ErrSyncMappingExists` handling covers the race where the mapping already exists.

The retry-once logic on `CreateSyncMapping` failure remains unchanged.

## Scope

- **In scope**: FindOrCreate on both adapters, handler wiring, tests
- **Out of scope**: Updating existing external items when spec state changes. Create-only; updates are a separate concern.

## File Changes

### Modify

- `internal/sync/adapter.go` — add `FindOrCreate` to `Adapter` interface
- `internal/sync/github.go` — implement `GitHubAdapter.FindOrCreate`
- `internal/sync/beads.go` — implement `BeadsAdapter.FindOrCreate`
- `internal/server/sync_handler.go` — call `FindOrCreate` instead of `Push`

### Test Files

- `internal/sync/github_test.go` — tests for FindOrCreate (found, not-found, search-error)
- `internal/sync/beads_test.go` — tests for FindOrCreate (found, not-found, search-error)
- `internal/server/sync_handler_test.go` — update mock adapter, test FindOrCreate integration (orphan healing, created vs found paths)
