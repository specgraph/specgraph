---
phase: 02-api-key-lifecycle-self-service
plan: 02
subsystem: storage
tags: [api-keys, postgres, pgx, authz, quota, toctou, ownership, auth-03]

requires:
  - phase: 02-api-key-lifecycle-self-service
    provides: admin API-key CRUD (CreateAPIKey/RevokeAPIKey/RotateAPIKey) + AuthStore + api_keys schema
provides:
  - Owner-scoped storage methods GetAPIKeyForUser, RevokeAPIKeyForUser, RotateAPIKeyForUser (explicit newKey)
  - Quota-safe CreateAPIKeyForUser (parent-row FOR UPDATE lock) + CountActiveAPIKeys
  - storage.ErrQuotaExceeded sentinel
affects: [server self handlers (Plan 05), CLI self-variants, config self-service key-policy]

tech-stack:
  added: []
  patterns:
    - "Owner-scoped uniform NotFound: AND user_id in WHERE + RowsAffected()==0 -> ErrAPIKeyNotFound (enumeration-hardening)"
    - "Quota-safe mint: bespoke pool.BeginTx + SELECT 1 FROM users FOR UPDATE serializes a user's mints; count+insert inside the tx (never count(*) FOR UPDATE)"
    - "Explicit-arg rotate: new key built solely from caller newKey (PHCHash/RoleDowngrade/ExpiresAt), never inheriting the old ceiling"

key-files:
  created:
    - internal/storage/postgres/users_selfkeys_test.go
  modified:
    - internal/storage/users.go
    - internal/storage/errors.go
    - internal/storage/postgres/users.go
    - internal/auth/usersbackend_stub_test.go
    - internal/server/usersbackend_stub_test.go

key-decisions:
  - "RotateAPIKeyForUser takes an explicit newKey *APIKey (handler-owned secret + floored downgrade + capped expiry) rather than inherit-on-nil, aligning with cursor review #3 and the admin CreateAPIKey/RotateAPIKey contract"
  - "Owner-scoped revoke drops the admin path's revoked_at IS NULL guard (via coalesce) so self-revoking an already-revoked key is an idempotent success (Finding F4), while a foreign/missing key is uniform NotFound"
  - "CreateAPIKeyForUser locks the parent users row FOR UPDATE (Postgres forbids a row-lock on an aggregate) to close the quota TOCTOU race; verified by a 20-goroutine concurrency test"

patterns-established:
  - "Parent-row FOR UPDATE serialization for per-user quota enforcement at the storage boundary"
  - "Uniform ErrAPIKeyNotFound for all owner-scoped miss cases (not-yours == missing == revoked)"

requirements-completed: [AUTH-03]

coverage:
  - id: D1
    description: "Owner-scoped get/revoke/rotate of a key that is not the caller's returns storage.ErrAPIKeyNotFound (indistinguishable from missing); self-revoke of your own already-revoked key is idempotent"
    requirement: "AUTH-03"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_GetAPIKeyForUser_ForeignKeyNotFound"
        status: pass
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_RevokeAPIKeyForUser_ForeignKeyNotFound"
        status: pass
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_RotateAPIKeyForUser_ForeignKeyNotFound"
        status: pass
    human_judgment: false
  - id: D2
    description: "Quota-safe self-mint serializes a single user's concurrent mints via a parent users-row FOR UPDATE lock and rejects over-quota with ErrQuotaExceeded; active count never exceeds quota under concurrency"
    requirement: "AUTH-03"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_SelfMintQuota"
        status: pass
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_SelfMintQuota_Concurrency"
        status: pass
    human_judgment: false
  - id: D3
    description: "RotateAPIKeyForUser persists the explicit floored roleDowngrade + capped expiresAt from the caller's newKey, never the old key's stale higher ceiling"
    requirement: "AUTH-03"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/users_selfkeys_test.go#TestAuthStore_RotateAPIKeyForUser_ExplicitArgs"
        status: pass
    human_judgment: false
  - id: D4
    description: "UsersBackend interface widened with five owner-scoped/quota methods + ErrQuotaExceeded sentinel; both compile-gate stubs satisfy the widened interface (build stays green)"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./internal/storage/... ./internal/auth/... ./internal/server/..."
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 02: Owner-scoped + quota-safe self-service API-key storage Summary

**Owner-scoped `*AuthStore` methods (uniform `ErrAPIKeyNotFound`), a quota-safe self-mint that serializes a user's concurrent mints via a parent users-row `FOR UPDATE` lock, and an explicit-arg rotate that never re-pins a stale ceiling — plus the `ErrQuotaExceeded` sentinel and both compile-gate stubs.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-09T16:35:10Z
- **Completed:** 2026-07-09T16:42:48Z
- **Tasks:** 3
- **Files modified:** 5 (1 created)

## Accomplishments

- Widened `storage.UsersBackend` with `GetAPIKeyForUser`, `RevokeAPIKeyForUser`, `RotateAPIKeyForUser` (explicit `newKey`), quota-safe `CreateAPIKeyForUser`, and `CountActiveAPIKeys`; added `storage.ErrQuotaExceeded`.
- Implemented all five on `*AuthStore` using `s.pool` directly (ADR-004 — bespoke `pool.BeginTx`, never `RunInTransaction`): every owner-scoped query is `AND user_id`-scoped with a `RowsAffected()==0 → ErrAPIKeyNotFound` guard, and the quota mint takes a parent users-row `FOR UPDATE` lock before counting + inserting.
- Added `internal/storage/postgres/users_selfkeys_test.go` (integration): ownership NotFound on get/revoke/rotate, idempotent self-revoke, quota enforcement, a 20-goroutine TOCTOU concurrency test, and explicit-arg rotate — all 6 pass against testcontainers Postgres.
- Kept the build green throughout by updating both compile-gate stubs (`internal/auth`, `internal/server`).

## Task Commits

1. **Task 1: Extend UsersBackend interface + sentinel + both stubs** - `d6718c7d` (feat)
2. **Task 2: Implement owner-scoped + quota-safe methods on *AuthStore** - `98d9a698` (feat)
3. **Task 3: Integration tests — ownership NotFound + quota TOCTOU** - `26a8ed2f` (test)

## Files Created/Modified

- `internal/storage/users.go` - Added five owner-scoped/quota methods to the `UsersBackend` interface with mirrored doc-comment style.
- `internal/storage/errors.go` - Added `ErrQuotaExceeded` sentinel.
- `internal/storage/postgres/users.go` - Implemented the five methods on `*AuthStore`; shared `activeKeyCountQuery`; savepoint-guarded prefix-collision retry inside both self-service transactions.
- `internal/storage/postgres/users_selfkeys_test.go` - New `//go:build integration` suite (6 tests).
- `internal/auth/usersbackend_stub_test.go` - Stub methods added (guard-style, return sentinels).
- `internal/server/usersbackend_stub_test.go` - Stub func-fields + dispatch methods added.

## Decisions Made

- **Explicit-arg rotate over inherit-on-nil:** `RotateAPIKeyForUser` takes an explicit `newKey *APIKey`; storage builds the new row solely from `newKey.PHCHash`/`RoleDowngrade`/`ExpiresAt` (handler-owned secret, already floored/capped) and never inherits the old key's ceiling. This aligns with cursor review #3 and the existing admin CRUD contract (storage returns `*APIKey`, never a plaintext secret).
- **Idempotent self-revoke via `coalesce`:** The owner-scoped revoke uses `revoked_at = coalesce(revoked_at, $now)` scoped by `user_id`, dropping the admin path's `revoked_at IS NULL` guard so re-revoking your own key is a no-op success (Finding F4); `RowsAffected()==0` (foreign/missing) is uniform NotFound.
- **Parent-row lock for quota TOCTOU:** `SELECT id FROM users WHERE id=$ AND deleted_at IS NULL FOR UPDATE` serializes a user's mints (Postgres forbids a row-lock on an aggregate, so `count(*) FOR UPDATE` is not used); the count + insert then run inside the serialized window.

## Deviations from Plan

None - plan executed exactly as written.

The plan's Task 1 acceptance ("`go build ./...` exits 0" after Task 1) is coupled to Task 2 by the production compile-time assertion `var _ storage.UsersBackend = (*AuthStore)(nil)` in `auth.go` — the interface only builds green once `*AuthStore` implements it. This was handled by implementing Task 2 on disk before committing so the working tree (which the pre-commit hooks validate) stayed green at every commit; the three tasks were still committed atomically by file set. This is a sequencing note, not a scope deviation.

## Issues Encountered

- The local Docker/colima VM entered a filesystem I/O-error state (`mkdir /tmp/containerd-mount…: input/output error`) that blocked testcontainers from starting Postgres — a plain `docker run alpine` and `colima ssh` also failed, confirming it was VM corruption, not test code. Resolved by `colima restart`; the integration suite then passed all 6 tests (22.4s including image pull).

## Deferred Issues

Four pre-existing lint findings surface only under `golangci-lint run --build-tags integration ./internal/storage/postgres/`, all in test files this plan did not touch (`migration_007_test.go`, `postgres_test.go`, `execution_test.go`, `auth_helpers_test.go`). Out of scope (unrelated to 02-02) — logged in `.planning/phases/02-api-key-lifecycle-self-service/deferred-items.md`. My new `users_selfkeys_test.go` is lint-clean.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The widened `UsersBackend` is available to the server handlers: Plan 05 can wire the four self handlers (`CreateMyAPIKey`, `ListMyAPIKeys`, `RotateMyAPIKey`, `RevokeMyAPIKey`) to these owner-scoped/quota-safe methods, mapping `ErrAPIKeyNotFound → CodeNotFound` and `ErrQuotaExceeded → CodeResourceExhausted`.
- No new goose migration introduced — existing `api_keys` columns suffice.

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*
