# ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths

- **Status:** Accepted
- **Date:** 2026-03-19
- **Bead:** spgr-1s1

## Context

The codebase commented that SpecGraph is "single-writer," but the ConnectRPC
server handles concurrent goroutines per request. Multi-query write paths
(spec mutation + hash recomputation + ChangeLog creation) could leave partial
state if a failure or concurrent modification occurred between queries — for
example, a spec mutation committed but its ChangeLog entry never created.

## Decision

Wrap all multi-query write paths in `RunInTransaction` (existing mechanism
in `tx.go`) for atomic rollback. Keep version guards
(`WHERE s.version = $expected_version`) for conflict detection. First writer
wins; second receives `ErrConcurrentModification` (mapped to `CodeAborted`,
retryable by the client).

## Consequences

- All new multi-query write paths must use `RunInTransaction`.
- Validation that doesn't hit the database stays outside the transaction to
  reduce lock time.
- Version guards remain the primary conflict detection mechanism.
- No distributed locking needed for single-server deployment.
- Callers handle `CodeAborted` with retry if desired.
- Nested `RunInTransaction` calls reuse the outer transaction (safe for
  operations like `StoreDecomposeOutput` that call `CreateSpec` internally).
