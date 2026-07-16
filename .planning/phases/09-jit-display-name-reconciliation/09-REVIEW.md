---
phase: 09-jit-display-name-reconciliation
reviewed: 2026-07-16T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - internal/auth/identitystore.go
  - internal/auth/identitystore_authn_integration_test.go
  - internal/auth/identitystore_jit_test.go
  - internal/auth/identitystore_test.go
  - internal/auth/introspection_test.go
  - internal/auth/loginsync.go
  - internal/auth/loginsync_internal_test.go
  - internal/auth/oauth2_provider_test.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 09: Code Review Report

**Reviewed:** 2026-07-16T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This is a re-verification pass over the prior review of this phase (AUTH-06, #994),
which extracts the `user.DisplayName == claims.Subject` staleness heuristic out of the
config-gated `applyLoginSync` path into a new `reconcileDisplayName` helper that
`materializeIdentity` invokes unconditionally on every successful OIDC resolve (bearer
JWT, introspection, and interactive `ResolveLogin`).

I re-traced both previously-flagged warnings against the current code and confirmed
neither has been fixed — nothing has changed since the prior pass. I also did a fresh
general read of all eight files (including a diff against `diff_base` to isolate exactly
what this phase touched) looking for anything new: the role/email pass-through
discipline holds up (`reconcileDisplayName` never touches `user.Role`/`user.Email`, and
its write always echoes them back unmodified — confirmed by
`TestResolveJWT_ReconciliationPreservesRoleAndEmail`), the no-op write guard is present
and correctly scoped, and the new test coverage exercises the happy path across all
three call sites (JWT, introspection, interactive login) plus a real-Postgres
integration test. I found no new Critical or Warning issues beyond what was already
flagged. Both prior warnings stand as-is.

## Warnings

### WR-01: `reconcileDisplayName`'s persist failure doesn't distinguish a concurrently soft-deleted user, unlike `applyLoginSync` (re-verified, still present)

**File:** `internal/auth/identitystore.go:577-587`

**Issue:** When the reconciliation write's `UpdateUserOnLogin` call fails, the code
unconditionally logs a warning and proceeds ("Best-effort: never deny a login over a
display-name write failure"):

```go
if newName, changed := reconcileDisplayName(user, claims); changed {
    if err := s.users.UpdateUserOnLogin(ctx, user.ID, newName, user.Email, user.Role); err != nil {
        // Best-effort: never deny a login over a display-name write failure.
        slog.LogAttrs(ctx, slog.LevelWarn, "auth: display-name reconciliation persist failed (proceeding)",
            slog.String("user_id", user.ID), slog.Any("error", err))
    } else {
        ...
    }
}
```

This treats every failure mode identically — including `storage.ErrUserNotFound`, which
`UpdateUserOnLogin`'s active-row guard (`deleted_at IS NULL`, confirmed in
`internal/storage/postgres/users.go:239-253` — it is the *only* documented sentinel this
call returns) returns specifically when the user was concurrently soft-deleted between
the `GetUserByID` load a few lines above and this write. The sibling `applyLoginSync`
treats this exact signal as security-relevant and fails closed:

```go
// loginsync.go:104-112
if errors.Is(err, storage.ErrUserNotFound) {
    slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync target user not active — denying login", ...)
    return nil, ErrUnauthenticated
}
```

Before this phase, a non-loginSync, non-interactive resolve (the common case: every
bearer-JWT/MCP/ConnectRPC request) built and returned the `Identity` immediately after
the `user.DeletedAt != nil` check with no further I/O in between — so there was
effectively no TOCTOU window on that path. This change inserts an unconditional DB
round-trip (the reconciliation `UPDATE`) between the soft-delete check and the returned
`Identity` on *every* login, widening that window for the entire authenticated-request
surface, not just interactive login. If a user is soft-deleted in that (admittedly
narrow) window, the request now completes as an authenticated identity for a
just-deleted user, rather than failing closed the way `applyLoginSync` explicitly guards
against for the interactive path.

No test exercises this path (confirmed again this pass — none of
`identitystore_test.go`, `identitystore_authn_integration_test.go`,
`identitystore_jit_test.go`, `introspection_test.go` reference `ErrUserNotFound` near the
reconciliation write; the new `TestResolveJWT_Reconciles*` / `TestIntrospection_Reconciles*`
tests all stub `updateUserOnLogin` to return `nil`), so this gap is also uncovered by the
new test suite added in this phase.

**Fix:** Mirror `applyLoginSync`'s classification:

```go
if newName, changed := reconcileDisplayName(user, claims); changed {
    if err := s.users.UpdateUserOnLogin(ctx, user.ID, newName, user.Email, user.Role); err != nil {
        if errors.Is(err, storage.ErrUserNotFound) {
            slog.LogAttrs(ctx, slog.LevelWarn, "auth: display-name reconciliation target user not active — denying login",
                slog.String("user_id", user.ID))
            return nil, ErrUnauthenticated
        }
        slog.LogAttrs(ctx, slog.LevelWarn, "auth: display-name reconciliation persist failed (proceeding)",
            slog.String("user_id", user.ID), slog.Any("error", err))
    } else {
        ...
    }
}
```

Add a unit test (stub `updateUserOnLogin` returning `storage.ErrUserNotFound`) asserting
`store.Resolve`/`ResolveLogin` return `ErrUnauthenticated`, mirroring the existing
`TestApplyLoginSync_UserNotFound_Denies`.

### WR-02: A denied login can still leave the display-name write persisted (re-verified, still present)

**File:** `internal/auth/identitystore.go:577-594`

**Issue:** `reconcileDisplayName`'s write and `applyLoginSync`'s write are two
independent, sequential, non-transactional `UpdateUserOnLogin` calls against the same
row within one `materializeIdentity` invocation. If the first (display-name) write
succeeds but the login is subsequently denied by `applyLoginSync` (allowlist miss, or a
demotion whose persist fails — both of which return an error from
`materializeIdentity` at `identitystore.go:591`), the overall auth attempt returns
`ErrUnauthenticated`/`ErrTransient` to the caller, yet the display-name change from the
first write is already committed. Previously, when this heuristic lived solely inside
`applyLoginSync`, a denied login (allowlist miss, pre-write) or a failed persist
(transactionally the *same* statement as the role/email change) meant a denial was
either a true no-op or an atomic single-statement failure — there was no way to get
"half a write." Now a denied interactive login can have a real, silent side effect on
the row. This is unlikely to be a privilege-escalation vector (only the display name
moves, and only toward a value the token holder is already asserting via a verified
claim), but it is a correctness/consistency regression an auditor or operator reasoning
about "denied login = no DB effect" would not expect.

**Fix:** Either (a) document this explicitly as an accepted tradeoff next to the
`applyLoginSync` doc comment (it currently documents the *absence* of display-name
computation there, but not the two-write split's atomicity implications), or (b) fold
both writes into a single `UpdateUserOnLogin` call in `materializeIdentity` by computing
the reconciled name first and passing it into `applyLoginSync` as an input rather than
persisting it separately.

## Info

### IN-01: Two independent `UpdateUserOnLogin` round-trips when both display name and role/email change (re-verified, still present, awareness only)

**File:** `internal/auth/identitystore.go:577-594`

**Issue:** When a login both triggers the display-name self-heal AND a role/email
change from `applyLoginSync`, two separate `UPDATE` statements are issued against the
same user row in the same request instead of one combined statement. This is called out
as an intentional design tradeoff in the code comments (`identitystore.go:571-576`,
`loginsync.go:85-89`), and is not a performance concern in scope for this review, but it
is the root cause of WR-02 above — worth a short pointer since a future reader fixing
WR-02 will likely want to also address this redundancy.

**Fix:** No action required beyond what's suggested in WR-02; noting for awareness since
a combined single-statement write would resolve both the redundancy and the
partial-write consistency gap at once.

---

_Reviewed: 2026-07-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
