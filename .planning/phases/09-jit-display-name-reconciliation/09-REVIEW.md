---
phase: 09-jit-display-name-reconciliation
reviewed: 2026-07-15T22:46:14Z
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

**Reviewed:** 2026-07-15T22:46:14Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This phase extracts the `user.DisplayName == claims.Subject` staleness heuristic
out of `applyLoginSync` (still gated on `loginSyncEnabled && interactive`) into a
new unconditional `reconcileDisplayName` helper invoked from
`materializeIdentity` on every successful OIDC resolve (bearer JWT,
introspection, and interactive login).

Verified against the four specific risk areas called out for this review:

1. **Role/email derivation stays gated.** `reconcileDisplayName` only computes
   and writes `DisplayName`; the `UpdateUserOnLogin` call it makes passes
   `user.Email`/`user.Role` straight through unmodified
   (`identitystore.go:578`). `applyLoginSync` still owns all role/email/
   allowlist derivation and remains gated on `s.loginSyncEnabled && interactive`
   (`identitystore.go:588`). No role/email logic leaked outside the gate.
2. **No double-fire of display-name semantics.** `applyLoginSync` no longer
   computes `newDisplay` at all — it passes `user.DisplayName` through
   unchanged (`loginsync.go:88-99`), and its own no-op check
   (`newEmail == user.Email && newRole == user.Role`) correctly ignores
   display name, so it never re-derives or overwrites the value
   `reconcileDisplayName` just set. See Warning WR-02 below for a related,
   narrower consistency gap this two-write split introduces.
3. **No-op write guard is present at the one new call site.** The write is
   correctly gated by `if newName, changed := reconcileDisplayName(user, claims); changed`
   (`identitystore.go:577`) — `UpdateUserOnLogin` is only invoked when the
   heuristic actually fires, consistent with `applyLoginSync`'s existing
   step-3 no-op skip.
4. **`TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive` is unmodified.**
   Confirmed via `git diff` against `diff_base` — the diff to
   `identitystore_authn_integration_test.go` is a pure append of a new
   `TestIdentityStore_DisplayNameReconciliation` test block after the existing
   test; the gate-regression test's body is untouched.

Two warnings below relate to error-handling asymmetry between the new
unconditional write and the sibling `applyLoginSync` write it now runs
alongside, both stemming from the fact that a single login can now issue two
independent, non-transactional `UpdateUserOnLogin` calls instead of one.

## Warnings

### WR-01: `reconcileDisplayName`'s persist failure doesn't distinguish a concurrently soft-deleted user, unlike `applyLoginSync`

**File:** `internal/auth/identitystore.go:577-587`

**Issue:** When the reconciliation write's `UpdateUserOnLogin` call fails, the
code unconditionally logs a warning and proceeds ("Best-effort: never deny a
login over a display-name write failure"):

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

This treats every failure mode identically — including
`storage.ErrUserNotFound`, which `UpdateUserOnLogin`'s active-row guard
(`deleted_at IS NULL`) returns specifically when the user was concurrently
soft-deleted between the `GetUserByID` load a few lines above and this write.
The sibling `applyLoginSync` treats this exact signal as security-relevant and
fails closed:

```go
// loginsync.go:104-112
if errors.Is(err, storage.ErrUserNotFound) {
    slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync target user not active — denying login", ...)
    return nil, ErrUnauthenticated
}
```

Before this phase, a non-loginSync, non-interactive resolve (the common case:
every bearer-JWT/MCP/ConnectRPC request) built and returned the `Identity`
immediately after the `user.DeletedAt != nil` check with no further I/O in
between — so there was effectively no TOCTOU window on that path. This change
inserts an unconditional DB round-trip (the reconciliation `UPDATE`) between
the soft-delete check and the returned `Identity` on *every* login, widening
that window for the entire authenticated-request surface, not just
interactive login. If a user is soft-deleted in that (admittedly narrow)
window, the request now completes as an authenticated identity for a
just-deleted user, rather than failing closed the way `applyLoginSync`
explicitly guards against for the interactive path.

No test exercises this path (searched `identitystore_test.go`,
`identitystore_authn_integration_test.go`, `identitystore_jit_test.go`,
`introspection_test.go` for `ErrUserNotFound` near the reconciliation code —
none found), so this gap is also uncovered.

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

### WR-02: A denied login can still leave the display-name write persisted

**File:** `internal/auth/identitystore.go:577-594`

**Issue:** `reconcileDisplayName`'s write and `applyLoginSync`'s write are now
two independent, sequential, non-transactional `UpdateUserOnLogin` calls
against the same row within one `materializeIdentity` invocation. If the
first (display-name) write succeeds but the login is subsequently denied by
`applyLoginSync` (allowlist miss, or a demotion whose persist fails — both of
which return an error from `materializeIdentity`), the overall auth attempt
returns `ErrUnauthenticated`/`ErrTransient` to the caller, yet the
display-name change from the first write is already committed. Previously,
when this heuristic lived solely inside `applyLoginSync`, a denied login
(allowlist miss, pre-write) or a failed persist (transactionally the *same*
statement as the role/email change) meant a denial was either a true no-op or
an atomic single-statement failure — there was no way to get "half a write."
Now a denied interactive login can have a real, silent side effect on the
row. This is unlikely to be a privilege-escalation vector (only the display
name moves, and only toward a value the token holder is already asserting
via a verified claim), but it is a correctness/consistency regression an
auditor or operator reasoning about "denied login = no DB effect" would not
expect.

**Fix:** Either (a) document this explicitly as an accepted tradeoff next to
the `applyLoginSync` doc comment (it currently documents the *absence* of
display-name computation there, but not the two-write split's atomicity
implications), or (b) fold both writes into a single `UpdateUserOnLogin` call
in `materializeIdentity` by computing the reconciled name first and passing
it into `applyLoginSync` as an input rather than persisting it separately.

## Info

### IN-01: Two independent `UpdateUserOnLogin` round-trips when both display name and role/email change

**File:** `internal/auth/identitystore.go:577-594`

**Issue:** When a login both triggers the display-name self-heal AND a
role/email change from `applyLoginSync`, two separate `UPDATE` statements are
issued against the same user row in the same request instead of one combined
statement. This is called out as an intentional design tradeoff in the code
comments (`identitystore.go:571-576`, `loginsync.go:85-89`), and is not a
performance concern in scope for this review, but it is the root cause of
WR-02 above — worth a short pointer since a future reader fixing WR-02 will
likely want to also address this redundancy.

**Fix:** No action required beyond what's suggested in WR-02; noting for
awareness since a combined single-statement write would resolve both the
redundancy and the partial-write consistency gap at once.

---

_Reviewed: 2026-07-15T22:46:14Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
