# Impact Notification Service Design

**Date:** 2026-03-30
**Bead:** spgr-w6o (Impact notification service for spec changes)
**Status:** Draft

## Problem

When a spec materially changes (changelog entry created), there's no mechanism to identify and notify impacted downstream specs. Consumers (drift engine, web UI, Gastown) need to know when upstream changes affect their specs.

## Decision Summary

| Question | Decision |
|----------|----------|
| Scope | Event emission + impact analysis only (no webhooks/sync adapters) |
| Trigger point | Post-transaction hook in `RunInTransaction` |
| Subscriber execution | Synchronous with interface designed for async upgrade |
| First subscriber | Log-only (proves pipeline, no side effects) |

## Subscriber Interface

Defined in `internal/storage/change_event.go`:

```go
// ChangeEvent is emitted after a changelog entry is persisted.
// Mirrors key ChangeLogEntry fields. FieldChange deltas and Date are
// omitted for simplicity — add them if a future subscriber needs them.
type ChangeEvent struct {
    Slug        string
    Version     int32
    Stage       SpecStage // storage.SpecStage (named string type)
    ContentHash string
    Checkpoint  bool
    Summary     string
    Reason      string
}

// ChangeSubscriber receives notifications after spec changes are committed.
type ChangeSubscriber interface {
    OnSpecChanged(ctx context.Context, event ChangeEvent)
}

// Subscribable is implemented by storage backends that support change notifications.
// Part of the storage interface, not backend-specific.
type Subscribable interface {
    Subscribe(ChangeSubscriber)
}
```

This interface works for both synchronous and asynchronous dispatch. When upgrading to channel-based async later, only the dispatch mechanism changes — subscriber implementations remain the same.

## Architecture

### Post-Transaction Hook

`RunInTransaction` in `internal/storage/memgraph/tx.go` currently executes a callback and commits. The changes:

1. `createChangeLog` stashes a `ChangeEvent` into a transaction-scoped slice via the context during the transaction.
2. After successful commit, `RunInTransaction` retrieves stashed events from the context and fires them to registered subscribers.
3. Failed transactions produce no events (correct: the changelog wasn't persisted).

This keeps all 10+ `createChangeLog` call sites untouched — they already run inside `RunInTransaction`.

### Event Stashing

Events are stashed via a context key during the transaction. The stashed value is a `*[]ChangeEvent` (pointer to slice) so that appends from nested `createChangeLog` calls are visible to the outermost `RunInTransaction` that drains them.

Helper functions:

- `initChangeEvents(ctx context.Context) context.Context` — creates a `*[]ChangeEvent` and stores it in context. Called once at the start of `RunInTransaction`.
- `stashChangeEvent(ctx context.Context, event ChangeEvent)` — appends event to the pointer-to-slice in context. No-op if context has no event slice (non-transactional path).
- `drainChangeEvents(ctx context.Context) []ChangeEvent` — returns stashed events (does not clear — context is about to be discarded).

Using a pointer-to-slice in context avoids race conditions (each transaction has its own context) and correctly propagates events from nested transactions — `RunInTransaction` short-circuits on nested calls (`txFromContext` check), but the inner `createChangeLog` calls share the same `*[]ChangeEvent` through the parent context.

### Subscriber Registration

Subscribers are stored on the root `Store` (pre-`Scoped()`). `Store.Scoped()` creates project-scoped stores that share the driver but are separate struct instances. The current `Store` struct has no shared config — `Scoped()` copies `driver`, `nowFunc`, and sets `project`.

**Concrete change required:** Add a `*shared` field to `Store` containing the subscribers slice. Initialize `shared` in `New()`. Copy the pointer in `Scoped()` so scoped stores see the same subscribers. The subscriber dispatch in `RunInTransaction` uses `s.shared.subscribers`.

`Subscribe(ChangeSubscriber)` is called once at startup on the root store before any writes — no concurrent mutation. The `Subscribable` interface is part of `internal/storage/` and composed into `Scoper`, so `serve.go` calls `store.Subscribe(sub)` without knowing the backend.

The subscriber dispatch loop passes the scoped `Store` (i.e., `s` in `RunInTransaction`) as context for `GetImpact`, not the root store. This ensures project-scoped graph queries.

Each backend implements dispatch internally (memgraph uses post-commit hooks in `RunInTransaction`), but the `Subscribe` contract is shared.

### Impact Logger Subscriber

The first subscriber lives in `internal/notify/impact.go`:

1. Receives `ChangeEvent` via `OnSpecChanged`
2. Calls `store.GetImpact(ctx, event.Slug)` for reverse-dependency traversal
3. Logs: `slog.Info("spec change impact", "slug", event.Slug, "version", event.Version, "impacted_count", len(refs), "impacted", slugList)`
4. If `GetImpact` fails, logs warning and continues (non-fatal)

The `ImpactLogger` is stateless — it extracts the scoped `GraphBackend` from the dispatch context via `storage.GraphBackendFromContext(ctx)`. The dispatch loop in `dispatchChangeEvents` injects the scoped store via `storage.WithGraphBackend(ctx, s)` where `s` is the scoped store that ran the transaction. This ensures `GetImpact` queries the correct project graph without the subscriber needing a store reference at construction time.

Subscribers MUST NOT block for extended periods since execution is synchronous post-commit.

## Files

| File | Responsibility |
|------|---------------|
| `internal/storage/change_event.go` | New. `ChangeEvent` type, `ChangeSubscriber` interface, `Subscribable` interface, context stash helpers |
| `internal/storage/scoper.go` | Modify. Compose `Subscribable` into `Scoper` |
| `internal/storage/memgraph/tx.go` | Modify. Post-commit subscriber dispatch, drain stashed events |
| `internal/storage/memgraph/changelog.go` | Modify. `createChangeLog` stashes `ChangeEvent` into context |
| `internal/storage/memgraph/memgraph.go` | Modify. Add `Subscribe(ChangeSubscriber)` method, subscribers slice |
| `internal/notify/impact.go` | New. `ImpactLogger` subscriber — GetImpact + slog |
| `internal/notify/impact_test.go` | New. Unit test with mock GraphBackend |
| `cmd/specgraph/serve.go` | Modify. Wire `ImpactLogger` subscriber at startup |

## Error Handling

- Subscriber panics are recovered per-subscriber with `defer/recover` in the dispatch loop — one panicking subscriber doesn't skip subsequent subscribers.
- `GetImpact` failures in `ImpactLogger` are logged as warnings, not propagated.
- Event stashing failures (shouldn't happen with context approach) are silently ignored — missing a notification is acceptable; blocking a write is not.

## Testing

| Test | File | Coverage |
|------|------|----------|
| ImpactLogger | `notify/impact_test.go` | Mock GraphBackend returning canned NodeRefs. Verify log output via slog handler. |
| Event stashing | `storage/change_event_test.go` | Stash/drain round-trip, empty drain, multiple events |
| Post-commit hook | `memgraph/tx_test.go` (extend) | Register subscriber, run transaction with changelog, verify subscriber called |
| No-fire on rollback | `memgraph/tx_test.go` (extend) | Transaction fails, verify subscriber NOT called |

## Not Building

- No webhooks — separate delivery mechanism, separate bead
- No sync adapter triggers — depends on webhook/delivery infrastructure
- No event persistence (ChangeEvent graph nodes) — log-only for now
- No CLI command — internal infrastructure, no user-facing surface
