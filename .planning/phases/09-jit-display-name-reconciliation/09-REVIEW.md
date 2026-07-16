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
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 09: Code Review Report (deep re-review)

**Reviewed:** 2026-07-16T00:00:00Z
**Depth:** deep
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This is a deep, cross-file re-review of AUTH-06 (#994) after two standard-depth
rounds already fixed WR-01 (fail-closed on concurrent soft-delete) and WR-02
(folding the display-name write into `applyLoginSync`'s single
`UpdateUserOnLogin` call). I traced every caller of `applyLoginSync` (1
production call site, `identitystore.go:588`; 14 test call sites in
`loginsync_internal_test.go`, all matching the current 5-arg signature) and
the single caller of `reconcileDisplayName` (`identitystore.go:586`),
confirmed the full call chain from all three entry points (`resolveJWT`,
`resolveIntrospection`, `ResolveLogin`) through `materializeIdentity` to the
two production `UpdateUserOnLogin` call sites (`identitystore.go:594` and
`loginsync.go:114` — no others exist anywhere in the codebase, confirmed via
grep), and checked for dead imports/helpers left behind by the extraction. No
unused imports, no dead code, no signature-drift bugs, and no path where
role/email is derived from claims outside the gated `applyLoginSync` were
found — the two previously-fixed issues stayed fixed and the refactor is
internally consistent across all 8 files.

The deep pass did surface one real logic defect in the reconciliation
heuristic itself (not a call-chain/wiring bug, so a standard per-file pass
reading each function in isolation could plausibly miss it):
`reconcileDisplayName` treats "a usable name claim is present" as sufficient
to report `changed=true`, without checking whether the computed value
actually differs from what's already stored. For any IdP where the `name`
claim happens to equal `sub` (not exotic — some enterprise IdPs seed `name`
from an employee/subject ID, and `jitResolve` will happily seed
`DisplayName` from such a claim at creation), this fires on every login
forever instead of self-healing once, because after the first write
`user.DisplayName` is still `== claims.Subject` on every subsequent load. It
also amplifies a pre-existing, low-severity concurrency characteristic:
because reconciliation now runs unconditionally (not gated by
`LoginSyncEnabled`/interactive), the read-then-write window against
`UpdateUserOnLogin` (which has no optimistic-concurrency/version guard,
unlike other multi-writer paths per project convention) is now open on every
successful authentication rather than only on interactive logins.

## Warnings

### WR-01: `reconcileDisplayName` reports `changed=true` even when the new value is identical to the old one

**File:** `internal/auth/identitystore.go:529-534`

**Issue:** The staleness heuristic only checks `user.DisplayName ==
claims.Subject && claims.Name != ""`; it never checks whether `claims.Name`
actually differs from `user.DisplayName`. For an IdP whose `name` claim
happens to equal its `sub` claim (plausible for enterprise IdPs that seed
`name` from an employee/subject ID), the JIT-seeded `DisplayName` equals both
`claims.Subject` **and** `claims.Name` at creation time (`jitResolve` seeds
`DisplayName` from `claims.Name` when present, per D-07,
`identitystore.go:748-751`). On every subsequent login,
`reconcileDisplayName` sees `user.DisplayName == claims.Subject` still true
(it can never become anything other than that value for such a user) and
reports `changed=true`, triggering an `UpdateUserOnLogin` write on **every
single login** for that user — forever — instead of self-healing once, as
the function's own doc comment claims ("If a usable name claim is now
present, self-heal it"). This is not a security issue, but it:
- defeats `applyLoginSync`'s no-op-skip optimization (`newEmail == user.Email
  && newRole == user.Role && !nameChanged`, `loginsync.go:104`) on every
  login for affected users, forcing the combined write path every time;
- emits a misleading `"auth: display-name reconciled"` info log with `old ==
  new` on every login (`identitystore.go:609-610`, `loginsync.go:147-150`),
  polluting audit trails with false "change" events;
- performs an unnecessary DB write on every authenticated request for such
  users through the unconditional standalone path
  (`identitystore.go:593-613`) when login-sync is off or the resolve is
  non-interactive.

No existing test exercises this: every test fixture's `DisplayName`
("test-u1", "sub-1", etc. — see `activeUser` in
`usersbackend_stub_test.go:186-191`) is deliberately either always-equal or
always-different from the token's `sub`/`Name` pair by construction, so the
"no functional change but the heuristic still fires" case (`claims.Name ==
claims.Subject == user.DisplayName`) is untested.

**Fix:**
```go
func reconcileDisplayName(user *storage.User, claims *OIDCClaims) (newName string, changed bool) {
	if user.DisplayName == claims.Subject && claims.Name != "" && claims.Name != user.DisplayName {
		return claims.Name, true
	}
	return user.DisplayName, false
}
```
Add a regression test with `claims.Name == claims.Subject == user.DisplayName` asserting `UpdateUserOnLogin` is NOT called.

### WR-02: Unconditional reconciliation widens the `UpdateUserOnLogin` TOCTOU window; the write has no optimistic-concurrency guard

**File:** `internal/auth/identitystore.go:586-613`, `internal/storage/postgres/users.go:241-253`

**Issue:** Prior to this phase, the display-name write only happened inside
the `LoginSyncEnabled && interactive` gate (a narrow window: interactive
logins with login-sync explicitly turned on). After this phase,
`reconcileDisplayName` is computed and — when it fires — written on *every*
successful `materializeIdentity` call: bearer JWTs, introspected opaque
tokens, and interactive logins alike, regardless of `LoginSyncEnabled`. The
read (`GetUserByID`) and the write (`UpdateUserOnLogin`) are two separate
round-trips with no version/CAS guard between them:
```sql
UPDATE users SET display_name = $1, email = $2, role = $3
WHERE id = $4::uuid AND deleted_at IS NULL
```
This statement guards only `deleted_at IS NULL` (correctly, per WR-01's prior
fix), not a row version — unlike the project's documented convention for
multi-writer paths (CLAUDE.md: "Version guards in WHERE clauses detect
conflicts... First writer wins; second fails fast"). Two concurrent requests
for the same user (e.g., two browser tabs, or a bearer-JWT request racing an
interactive login) now race on this write far more often than before, and
whichever request's claims-derived reconciliation lands last silently wins
with no conflict signal. I checked `internal/storage/postgres/users.go` for
any other RPC that mutates an existing user row (`UpdateUserRole`,
`UpdateUserOnLogin`, `SoftDeleteUser`, `PurgeUser` are the only ones) — there
is currently no separate admin "rename display name" endpoint, so today's
practical blast radius is limited to two reconciliation writes racing each
other (benign, same eventual value) rather than clobbering an unrelated
operator edit. But the widened window is a direct, provable consequence of
this phase's "run unconditionally" change, and will become a real lost-update
risk the moment any future profile-edit RPC is added.

**Fix:** Either document this as an accepted tradeoff near
`reconcileDisplayName`/`UpdateUserOnLogin` (a one-line comment noting the
missing version guard and why it's acceptable today), or add an
optimistic-concurrency check to `UpdateUserOnLogin` consistent with the rest
of the write paths described in CLAUDE.md, e.g.:
```sql
UPDATE users SET display_name = $1, email = $2, role = $3, version = version + 1
WHERE id = $4::uuid AND deleted_at IS NULL AND version = $5
```

## Info

### IN-01: No test covers the `ErrUserNotFound` fail-closed branch of the standalone reconciliation write outside the `resolveJWT` path

**File:** `internal/auth/identitystore_test.go:901` (only existing coverage),
`internal/auth/introspection_test.go`, `internal/auth/identitystore_jit_test.go`

**Issue:** `TestResolveJWT_ReconciliationUserNotFound_Denies` proves the
standalone reconciliation branch (`materializeIdentity`'s `else if
nameChanged` path, `identitystore.go:593-607`) fails closed when
`UpdateUserOnLogin` returns `storage.ErrUserNotFound` (concurrent
soft-delete). Because this branch is shared code reached from all three
entry points (`resolveJWT`, `resolveIntrospection`, `ResolveLogin`), the risk
of an actual regression is low, but there is no equivalent test proving the
same fail-closed behavior when reached via `resolveIntrospection` (opaque
token, whose `OIDCClaims` is hand-built with no `Email` — see
`resolveIntrospection`, `identitystore.go:670-675`) or `ResolveLogin`
(interactive, login-sync disabled). A future refactor that special-cases one
entry point's error handling could silently regress the other two without
any test catching it.

**Fix:** Add `TestIntrospection_ReconciliationUserNotFound_Denies` (mirroring
`TestIntrospection_ReconcilesStaleDisplayName`) and
`TestResolveLogin_ReconciliationUserNotFound_Denies`, each asserting
`ErrUnauthenticated` when the standalone write returns
`storage.ErrUserNotFound`.

---

_Reviewed: 2026-07-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
