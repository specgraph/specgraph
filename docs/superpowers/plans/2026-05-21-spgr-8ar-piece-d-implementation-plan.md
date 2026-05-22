# spgr-8ar Piece D — Retire Single-Layer Compat Method

> **For agentic workers:** Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove `Store.GetConstitution` (the single-layer "backward compatibility" method marked `// Deprecated` in Piece A) from the storage interface, the Postgres implementation, and all test mocks. Add a CI grep guard against regrowth.

**Architecture:** Pure deletion + a lint-time grep guard. No schema change, no behavior change for callers that already migrated to `GetMergedConstitution`.

## Pre-condition audit

Production callers of `Store.GetConstitution` (the storage interface method, not the RPC) after Piece A merged:

```text
$ rg 'func.*GetConstitution\(.*context\.Context.*\) \(\*storage\.Constitution' internal/
internal/server/sync_handler_test.go        # syncTestBackend mock (test-only)
internal/storage/postgres/constitution.go   # *Store impl
```

And the matching receiver-less variant for other mocks (`func (stubBackend) GetConstitution(...)`):

```text
internal/server/test_scoper_test.go         # stubBackend mock
internal/server/constitution_handler_test.go # mockConstitutionBackend mock
internal/server/error_sanitize_test.go      # errorBackend mock
```

Plus tests of the deprecated method itself:

- `internal/storage/postgres/constitution_test.go:62` — `TestGetConstitution_NotFound`
- `internal/storage/postgres/constitution_test.go:71` — `TestGetConstitution_RoundTrip`

No production code outside test mocks references the method. Safe to delete.

## File Structure

### Files modified

| Path | Change |
|---|---|
| `internal/storage/constitution.go` | Remove `GetConstitution` from `ConstitutionBackend` interface |
| `internal/storage/postgres/constitution.go` | Remove `*Store.GetConstitution` implementation |
| `internal/storage/postgres/constitution_test.go` | Remove `TestGetConstitution_NotFound`, `TestGetConstitution_RoundTrip` |
| `internal/server/test_scoper_test.go` | Remove `stubBackend.GetConstitution` stub |
| `internal/server/constitution_handler_test.go` | Remove `mockConstitutionBackend.GetConstitution` |
| `internal/server/error_sanitize_test.go` | Remove `errorBackend.GetConstitution` |
| `internal/server/sync_handler_test.go` | Remove `syncTestBackend.GetConstitution` (including the `//nolint:staticcheck` line) |
| `Taskfile.yml` or `task lint:*` | Add CI grep guard against `*Store.GetConstitution` reintroduction |

## Tasks

### Task 1: Delete the interface method, impl, and mocks (one commit)

The deletion is mechanical and tightly coupled — interface signature ↔ all implementations must move together or the build breaks. Doing it as a single commit keeps the diff atomic.

- [ ] Remove the deprecated method from `ConstitutionBackend` interface
- [ ] Remove `*Store.GetConstitution` from Postgres impl
- [ ] Remove the matching method from each of the four test mocks
- [ ] Remove the two `TestGetConstitution_*` tests (superseded by `TestGetMergedConstitution_*` + `TestGetConstitutionLayer_*`)
- [ ] Run `go build ./...` — clean
- [ ] Run `go test ./internal/...` — all pass
- [ ] Commit as `refactor(storage): remove deprecated GetConstitution method (spgr-8ar piece D)`

### Task 2: Add CI grep guard

Adds a small shell pipeline that fails the build if `*Store.GetConstitution` or `storage.GetConstitution` reappears in production code. Wire into `task check` so it runs on every PR.

- [ ] Add a new `lint:constitution-callers` task to `Taskfile.yml` that runs:

  ```bash
  if rg -n 'Store\.GetConstitution\b' --type go -g '!*_test.go' -g '!internal/storage/postgres/constitution_test.go' .; then
    echo "ERROR: Store.GetConstitution is removed; use GetMergedConstitution" >&2
    exit 1
  fi
  ```

- [ ] Add to the `lint` task's dependencies so `task check` runs it
- [ ] Manually verify the guard triggers by transiently reintroducing a usage, then revert
- [ ] Commit as `chore(lint): guard against Store.GetConstitution reintroduction`

### Task 3: Quality gates + PR

- [ ] `task check` green
- [ ] `task test:integration` green
- [ ] Direct API e2e green
- [ ] Push bookmark + open PR
- [ ] Update bd + push

## Self-Review

- No first-party callers of `Store.GetConstitution` after Piece A merged (verified by audit)
- All four mocks updated atomically with the interface change (or build breaks)
- Two deprecated-method tests removed; their behavior is fully covered by sibling tests on `GetMergedConstitution` / `GetConstitutionLayer`
- CI guard prevents accidental reintroduction
- DCO trailer uses `4678+seanb4t@users.noreply.github.com`

## Plan complete

2 implementation commits + plan + quality gates = ~3-4 commits total.
