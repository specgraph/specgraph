# Deferred Items — Phase 02

Out-of-scope pre-existing issues discovered during execution. NOT fixed (scope boundary — unrelated to the current plan's changes).

## Pre-existing lint issues (integration build tag) — discovered 02-02

Surfaced only under `golangci-lint run --build-tags integration ./internal/storage/postgres/`. All in pre-existing test files, none in files this plan touched:

- `internal/storage/postgres/migration_007_test.go:89` — govet shadow: `err` shadows declaration at line 53
- `internal/storage/postgres/postgres_test.go:94` — nolintlint: unused `//nolint:gosec` directive
- `internal/storage/postgres/execution_test.go:433` — prealloc: preallocate `principleIDs`
- `internal/storage/postgres/auth_helpers_test.go:25` — revive: context-as-argument (ctx not first param in `sharedTestPool`)

These do not fail `task check` (which does not pass `--build-tags integration` to the go lint step for the postgres package's integration-tagged files) and are unrelated to plan 02-02.
