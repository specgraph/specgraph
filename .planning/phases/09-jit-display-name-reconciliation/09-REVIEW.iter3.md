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
  warning: 1
  info: 0
  total: 1
status: issues_found
---

# Phase 09: Code Review Report (Re-review)

**Reviewed:** 2026-07-16T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Re-reviewed the fixer's two follow-up commits against the prior review's
WR-01, WR-02, and IN-01 findings, plus a fresh standard-depth pass over all
8 files.

**WR-01 is correctly resolved.** Read via `git show 2a864b69`:
`materializeIdentity` in `internal/auth/identitystore.go` now mirrors
`applyLoginSync`'s classification on the reconciliation write's
`UpdateUserOnLogin` failure — `errors.Is(err, storage.ErrUserNotFound)` logs
and returns `ErrUnauthenticated` instead of falling through to the
best-effort "proceed anyway" branch, closing the TOCTOU window a
concurrently-soft-deleted user could otherwise ride through. The new test
`TestResolveJWT_ReconciliationUserNotFound_Denies`
(`identitystore_test.go:901-936`) stubs `updateUserOnLogin` to return
`storage.ErrUserNotFound` against a user whose stored display name equals the
token subject (triggering the write) and asserts `store.Resolve` returns
`auth.ErrUnauthenticated` — this is exactly the previously-uncovered gap, now
covered. I confirmed the fix doesn't regress the "other errors are
best-effort" branch by re-reading `TestResolveJWT_ReconciliationPreservesRoleAndEmail`
and `TestResolveJWT_ReconcilesStaleDisplayName`, both of which still exercise
a nil-error `updateUserOnLogin` and still pass under the new branch. **This
finding is closed.**

**WR-02 is NOT resolved — the follow-up commit only added comments.**
`git show f80dcf90` is a pure documentation diff: 14 lines of comment added
at the reconciliation call site in `identitystore.go` and 7 lines of comment
added at `applyLoginSync`'s doc comment in `loginsync.go`; zero lines of
executable code changed. The two sequential, non-transactional
`UpdateUserOnLogin` calls against the same user row still exist exactly as
before, and the failure mode WR-02 described is unchanged: if the
reconciliation write (display-name self-heal) succeeds and `applyLoginSync`
subsequently denies the login (allowlist miss, or a demotion whose persist
fails), the display-name mutation is already committed even though the
overall `Resolve`/`ResolveLogin` call returns an error to the caller.
Documenting a known tradeoff is good practice, but a comment does not change
runtime behavior — "denied login = no DB effect" still does not hold for
this field. Restated below as an open Warning; see that section for why this
should not be treated as resolved just because it's now written down.

**IN-01 is adequately addressed.** It was an informational note whose
purpose was to record the root cause of WR-02 (two independent writes
instead of one) for a future fixer's benefit. The added comments in both
files now state this explicitly and cross-reference each other, which is a
sufficient disposition for an Info-level "awareness" item. It is folded into
the WR-02 discussion below rather than re-listed separately, since its
substance doesn't change WR-02's disposition.

The fresh general pass over all 8 files (dispatch routing in `Resolve`,
`resolveAPIKey`/`resolveSession`/`resolveJWT`/`resolveIntrospection`,
`jitResolve`, the JIT rate limiter, the email-domain allowlist, claims-mapping
scoping to JIT-creation-only, and role-rank/clamp helpers) did not surface
additional Critical or Warning-level defects. Test coverage across the
JIT/login-sync/introspection/OAuth2 interaction is thorough — each test
asserts a specific, observable behavior (who gets called, with what
arguments, what the resulting `Identity`/persisted row looks like) rather
than only checking "no error."

## Warnings

### WR-02: Reconciliation and login-sync writes remain non-atomic (re-flagged — not fixed, only documented)

**File:** `internal/auth/identitystore.go:578-611` (reconciliation write in
`materializeIdentity`), `internal/auth/loginsync.go:91-134` (`applyLoginSync`'s
write)

**Issue:** The follow-up commit (`f80dcf90`) added explanatory comments at
both call sites but made no code change (confirmed via `git show f80dcf90`
— diff touches only comment lines). The display-name reconciliation write
and `applyLoginSync`'s role/email write remain two independent, sequential
`UpdateUserOnLogin` calls against the same row within one
`materializeIdentity` invocation, with no transaction wrapping them.
Concretely: a bearer JWT (or interactive login) presenting a stale-fallback
display name (`user.DisplayName == claims.Subject`) plus a usable `name`
claim, but whose email domain has since fallen out of the allowlist (or
whose role-demotion persist fails), will have its display name updated and
committed to Postgres even though `Resolve`/`ResolveLogin` ultimately returns
`ErrUnauthenticated`/`ErrTransient` to the caller. "Login denied ⇒ no DB
effect" no longer holds for this one field, and nothing in the code prevents
this from recurring on every subsequent denied attempt for the same user
(each one re-triggers `reconcileDisplayName` since the stored name is now the
claim-derived value the first time, but a *different* denied attempt with a
different display-adjacent claim, or a user whose name keeps rotating, would
keep committing). This is a genuine, if narrow, DB-consistency gap that an
operator or auditor reasoning about "denied login = no observable effect"
would not expect.

**Why "documented" is not "fixed":** a WARNING finding tracks incorrect or
risky *behavior*; adding a comment that accurately describes the risky
behavior does not change what the program does at runtime. The commit
message itself frames this as "documented as an accepted tradeoff" rather
than "resolved" — that framing is honest about what changed, and this review
holds the same line: the finding stays open until either the tradeoff is
formally ratified by the project (e.g., an ADR or an explicit
product/security sign-off distinct from an inline code comment, since the
comment alone is not discoverable by anyone auditing behavior rather than
reading source) or the code is changed to close the gap.

**Fix:** Either (a) escalate the acceptance out of source comments into a
tracked decision artifact (ADR / phase decision log) so the tradeoff is
visible to someone auditing behavior without reading this exact function, or
(b) close the gap in code by threading the reconciled name into
`applyLoginSync` as an input and performing a single `UpdateUserOnLogin` call
once both the reconciliation and login-sync decisions are known, deferring
any write until after the allowlist/demotion gate has already decided to
allow the login:

```go
// sketch: compute reconciled name + login-sync outcome first, write once
newName, nameChanged := reconcileDisplayName(user, claims)
if s.loginSyncEnabled && interactive {
    // applyLoginSync takes (newName, nameChanged) and performs at most ONE
    // UpdateUserOnLogin call combining name + role + email; on denial it
    // performs NO write at all.
    synced, syncErr := s.applyLoginSync(ctx, claims, user, newName, nameChanged)
    if syncErr != nil {
        return nil, syncErr // guaranteed: no partial write occurred
    }
    user = synced
} else if nameChanged {
    // existing single-field write, unchanged, only reached when login-sync
    // isn't gating the outcome
    ...
}
```

Recommend (b) given the fix is well-scoped (it was already identified and
described in the existing comments) and removes both the consistency gap and
the redundant round-trip noted by the original IN-01.

---

_Reviewed: 2026-07-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
