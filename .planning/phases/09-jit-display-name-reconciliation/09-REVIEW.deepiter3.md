---
phase: 09-jit-display-name-reconciliation
reviewed: 2026-07-16T00:00:00Z
depth: deep
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
  critical: 1
  warning: 0
  info: 1
  total: 2
status: issues_found
---

# Phase 09: Code Review Report (deep re-review, iteration 3 — final)

**Reviewed:** 2026-07-16T00:00:00Z
**Depth:** deep
**Files Reviewed:** 8
**Status:** issues_found

> **Note on the `reviewed`/`Reviewed` timestamp:** `2026-07-16T00:00:00Z` is a nominal
> placeholder date stamped by the reviewing subagent, not a precise wall-clock capture —
> it is identical across all of this phase's review iterations and does not reflect true
> chronological ordering between review passes. The authoritative ordering is the git
> commit history: this iteration verifies the fix committed as `f28f8aa1`/`99a333a7`
> (CR-01/IN-02), which lands after `09-REVIEW-FIX.deepiter2.md`'s `16a833fe` in `git log`.

## Summary

This is the third and final deep-review iteration for the display-name
reconciliation phase (AUTH-06, #994). The prior deep pass found WR-01, WR-02,
and IN-01; a fixer addressed all three. This round verifies each fix and
performs a fresh full pass.

**WR-01 (`reconcileDisplayName` false-positive `changed=true` when
`claims.Name == claims.Subject`) — verified correctly closed.** The new
guard (`claims.Name != user.DisplayName`, `identitystore.go:535`) is only
reachable once the outer `user.DisplayName == claims.Subject` condition
already holds, so it cannot suppress a legitimate reconciliation: an
operator-chosen display name that differs from the subject never enters
this branch at all, with or without the added clause. I traced the
specific hypothesized regression (an operator renaming a user to a value
that happens to equal the *current* `claims.Name`) and confirmed it
cannot occur — that scenario requires `user.DisplayName != claims.Subject`,
which already short-circuits the whole function before the new guard is
even evaluated. `TestResolveJWT_ReconciliationNoOpWhenClaimNameEqualsSubject`
correctly exercises the fixed case. No new edge case introduced.

**IN-01 (fail-closed regression coverage across all three entry points) —
verified correct.** `TestResolveJWT_ReconciliationUserNotFound_Denies`,
`TestResolveLogin_ReconciliationUserNotFound_Denies`, and
`TestIntrospection_ReconciliationUserNotFound_Denies` were each read in
full. Each stubs `UpdateUserOnLogin` to return `storage.ErrUserNotFound`,
drives the matching entry point (`Resolve` w/ JWT, `ResolveLogin`,
`Resolve` w/ opaque token + introspector), and asserts
`ErrUnauthenticated`. All three genuinely reach and exercise the
standalone-reconciliation branch in `materializeIdentity` (not
`applyLoginSync`'s already-covered classification path) — confirmed by
constructing each user/binding so `DisplayName == subject` at load time,
which is the precondition for `nameChanged=true` to hold.

**WR-02 (documentation-only resolution of the missing optimistic-
concurrency guard on `UpdateUserOnLogin`) — NOT acceptable; reopened as
CR-01 below.** The rationale written into the code ("the only two
callers... both derive every field from the same claims-vs-DB-row
comparison, so two racing writes... converge on the same eventual value")
does not survive independent verification: the two callers derive
`role`/`email` through materially different logic, which means a race
between them is not "benign convergent" — it can silently revert a role
change made moments earlier by an interactive login-sync (a security-
relevant regression, not a rounding/formatting nondeterminism). See CR-01
for the concrete trace and a proportionate fix that doesn't require the
full version/CAS machinery the fixer correctly judged as overkill last
round — it only needs to stop the standalone write from touching
`role`/`email` at all.

A fresh full pass across all 8 files (dispatch order in `Resolve`,
`jitResolve`'s gate ordering, `applyLoginSync`'s classification table,
`clampedRole`/`isPromotion`, the introspection multi-provider trial logic,
and all test helper wiring) found no other correctness, security, or
quality defects beyond CR-01 and the one INFO item below. This should be
treated as the final iteration of this deep-review/fix loop.

## Critical Issues

### CR-01: WR-02's "benign convergent write" rationale is false — a concurrent standalone reconciliation write can silently revert a role change

**File:** `internal/auth/identitystore.go:591-618` (standalone reconciliation write, `materializeIdentity`)
**File:** `internal/auth/loginsync.go:80-155` (`applyLoginSync`)
**File:** `internal/storage/postgres/users.go:239-265` (`UpdateUserOnLogin`, carrying the accepted-tradeoff comment)

**Issue:**

The comment accepting the missing optimistic-concurrency guard (added by
the WR-02 fix, mirrored on both `UpdateUserOnLogin` and
`reconcileDisplayName`) reasons:

> "the only two callers (applyLoginSync and materializeIdentity's
> standalone reconciliation write) both derive every field from the same
> claims-vs-DB-row comparison, so two racing writes for the same user
> converge on the same eventual value (last-writer-wins is benign here,
> not a lost-update)."

This is factually wrong for `role` and `email`. The two call sites do not
derive those fields the same way:

- `applyLoginSync` (`loginsync.go:94`, via `resolveLoginRole`)
  **freshly re-derives** the role from the issuer's current
  `claims_mapping` plus the currently-authenticated token — it can
  legitimately compute a *different* role than what's stored (a
  promotion or a demotion).
- The standalone reconciliation write in `materializeIdentity`
  (`identitystore.go:599`) issues
  `s.users.UpdateUserOnLogin(ctx, user.ID, newName, user.Email, user.Role)`
  — `user.Email`/`user.Role` here are nothing more than the values this
  particular request happened to read via `GetUserByID` a few lines
  earlier. It is a blind passthrough of a point-in-time read, not a
  "comparison" of any kind.

Because `UpdateUserOnLogin` is one unconditional 3-column `UPDATE` with
no version guard, these two call sites racing on the same user is a real
lost-update on a security-relevant field, not a harmless convergence:

1. User U is `role=admin`. `display_name` is still `== sub` (the
   JIT-fallback value, not yet reconciled — a window this very phase
   introduces via commit `d9b2bb93`, "reconcile stale display_name
   unconditionally on OIDC login").
2. An operator revokes U's admin access. This is processed via an
   **interactive** login (`LoginSyncEnabled && interactive`, so
   `applyLoginSync` runs): it reads `role=admin`, computes
   `newRole=reader`, and is about to persist
   `UpdateUserOnLogin(id, name, email, "reader")`.
3. Concurrently, a **non-interactive** bearer-JWT request for the same
   user (any live client session, MCP tool call, or ConnectRPC call
   using a cached token — `interactive=false` here since it isn't a
   login flow) reaches `materializeIdentity`'s `else if nameChanged`
   branch. It reads the user **before** step 2's write commits
   (`role=admin` still), computes `nameChanged=true` (the fallback name
   is still stale relative to the token's `name` claim), and issues its
   own `UpdateUserOnLogin(id, name, email, "admin")`.
4. If this second write commits **after** step 2's, the demotion is
   silently undone. There is no error, no audit log (the standalone
   branch has no concept of role change — from its perspective `role`
   never changed, since it copied the value it read), and no signal to
   the operator that the revocation didn't stick. A revoked admin
   regains admin.

The same comment states the tradeoff should be revisited "the moment a
distinct write path... starts calling this method" — but that condition
is already true today: `applyLoginSync` and the standalone reconciliation
write are two call sites with divergent field-derivation semantics, not
one code path invoked twice with identical logic. The "only two callers"
premise (verified accurate — confirmed via grep, exactly `loginsync.go:114`
and `identitystore.go:599` in production code) does not make the race
benign; only identical derivation logic would, and that isn't what exists.
This should be reopened rather than accepted as documentation.

**Fix:** A full CAS/version column (the fixer's previously-rejected,
disproportionate fix) is not required. The narrower, proportionate fix is
to stop the standalone reconciliation write from touching `role`/`email`
at all, since it never legitimately changes them:

```go
// internal/storage/users.go — add a narrower method used ONLY by the
// standalone reconciliation path, so it structurally cannot race-clobber
// role/email.
UpdateDisplayNameOnLogin(ctx context.Context, userID, displayName string) error
```

```go
// internal/storage/postgres/users.go
func (s *AuthStore) UpdateDisplayNameOnLogin(ctx context.Context, userID, displayName string) error {
	const q = `UPDATE users SET display_name = $1 WHERE id = $2::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, displayName, userID)
	if err != nil {
		return fmt.Errorf("update display name on login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}
```

```go
// internal/auth/identitystore.go — materializeIdentity's standalone
// branch: call the narrower method instead of UpdateUserOnLogin.
} else if nameChanged {
	if err := s.users.UpdateDisplayNameOnLogin(ctx, user.ID, newName); err != nil {
		...
	}
}
```

This removes the race by construction — the standalone path no longer
writes `role`/`email` at all, so it has nothing left to clobber — without
introducing version columns, CAS loops, or transactions. It keeps the fix
proportionate (narrowing the write's blast radius to the one field this
path is actually responsible for) rather than accepting a security-
relevant lost-update as a documented tradeoff.

## Info

### IN-02: No contract test guards against CR-01 recurring

**File:** `internal/auth/identitystore_test.go`, `internal/auth/loginsync_internal_test.go`

**Issue:** The CR-01 race has no test coverage (reasonably — a true
concurrent-write race isn't easily expressed as a deterministic unit test
against a synchronously-executing stub backend). Once CR-01 is fixed via
the narrower `UpdateDisplayNameOnLogin` method, the *contract* that
prevents recurrence can still be asserted without needing real
concurrency: the standalone reconciliation branch should provably never
invoke a role/email-carrying persistence call.

**Fix:** After applying CR-01's fix, add a test asserting the standalone
reconciliation branch calls only a display-name-only stub hook (e.g. a
new `updateDisplayNameOnLogin` field on `usersBackendStub`) and that the
stub's `updateUserOnLogin` (3-field) hook is never invoked from that
branch. This turns "the standalone path cannot touch role/email" from an
implicit code-review invariant into an explicit, enforced one.

---

_Reviewed: 2026-07-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
