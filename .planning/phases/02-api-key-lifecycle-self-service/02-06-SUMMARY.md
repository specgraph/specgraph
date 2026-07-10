---
phase: 02-api-key-lifecycle-self-service
plan: 06
subsystem: auth
tags: [api-keys, rbac, resync, off-board, connectrpc, cobra, go, postgres]

requires:
  - phase: 02-01
    provides: generated IdentityService ResyncUserRole procedure + ResyncUserRoleRequest/Response (with revoke_keys)
  - phase: 02-04
    provides: ResyncUserRole → user.manage (admin-only) procedure→action map entry
  - phase: 02-05
    provides: IdentityHandler self-service handlers + identityError mapping + audit-line pattern (this plan appends to the same file)
provides:
  - "ResyncUserRole handler — the single reusable D-01/D-04 server-side seam: writes users.role via the existing UpdateUserRole path (live floor for standing keys) and hard-revokes active keys when revoke_keys is set (D-02)"
  - "auth user resync <user-id> --role <r> [--revoke-keys] CLI subcommand"
  - "Integration proof of SC#3 standing-key live floor at both the storage/resolver primitive and the ResyncUserRole RPC seam"
affects: [gastown-execution, future automation resync driver, web keys dashboard]

tech-stack:
  added: []
  patterns:
    - "Reusable operator seam: ResyncUserRole reuses UpdateUserRole + ListAPIKeys + RevokeAPIKey — no new storage method, no schema change (D-03), directly callable by a future automation driver with a derived role (D-01)"
    - "Explicit operator-supplied role input (never IdP-derived this phase, D-05) keeps the entrypoint automation-ready"
    - "Structured audit line on forced demotion/off-board (T-02-22): target/role/revoke_keys/keys_revoked, never token material"
    - "In-package server integration test constructs &IdentityHandler{...} directly to drive the RPC method against real Postgres"

key-files:
  created:
    - internal/server/identity_resync_test.go
    - cmd/specgraph/auth_user_test.go
    - internal/auth/resync_integration_test.go
    - internal/server/identity_resync_integration_test.go
  modified:
    - internal/server/identity_handler.go
    - cmd/specgraph/auth_user.go

key-decisions:
  - "ResyncUserRole reuses the existing UpdateUserRole write path (D-01/D-04) — one reusable seam, no schema change (D-03), no new storage method"
  - "The re-sync role is an explicit operator input (--role), never IdP-derived this phase (D-05), so a future automation driver reuses the same entrypoint with a derived role"
  - "revoke_keys filters on IsActive before revoking so the convergence (soft) path never fires a spurious revoke; RevokeAPIKey is idempotent for already-revoked keys"
  - "The RPC-seam live-floor is proven at integration depth in-package (package server) so the handler can be constructed directly against a real AuthStore, driving the demotion through the RPC — not only authStore.UpdateUserRole (cursor #3)"

patterns-established:
  - "Operator-gated write reusing an existing mutation seam + optional hard off-board, with a redacted audit line"

requirements-completed: [AUTH-02]

coverage:
  - id: D1
    description: "ResyncUserRole writes the operator-supplied id+role via the existing UpdateUserRole path (live floor for all standing keys) and returns the updated user; unknown user → CodeNotFound, empty id/role → CodeInvalidArgument"
    requirement: "AUTH-02"
    verification:
      - kind: unit
        ref: "internal/server/identity_resync_test.go#TestResyncUserRole_WritesRole"
        status: pass
      - kind: unit
        ref: "internal/server/identity_resync_test.go#TestResyncUserRole_UnknownUser"
        status: pass
      - kind: unit
        ref: "internal/server/identity_resync_test.go#TestResyncUserRole_RequiresIDAndRole"
        status: pass
    human_judgment: false
  - id: D2
    description: "revoke_keys=true hard-revokes all active standing keys (D-02 off-board); revoke_keys=false revokes none (D-03 convergence)"
    requirement: "AUTH-02"
    verification:
      - kind: unit
        ref: "internal/server/identity_resync_test.go#TestResyncUserRole_RevokeKeysTrue"
        status: pass
      - kind: unit
        ref: "internal/server/identity_resync_test.go#TestResyncUserRole_RevokeKeysFalse"
        status: pass
    human_judgment: false
  - id: D3
    description: "auth user resync <user-id> --role <r> [--revoke-keys] calls the ResyncUserRole RPC with parsed id/role/flag; --json emits the response via printJSON, default prints a human summary; registered under the auth user group"
    requirement: "AUTH-02"
    verification:
      - kind: unit
        ref: "cmd/specgraph/auth_user_test.go#TestAuthUserResync_ForwardsIDRoleAndRevoke"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_user_test.go#TestAuthUserResync_NoRevokeKeepsKeys"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_user_test.go#TestAuthUserResync_JSONEmitsResponse"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_user_test.go#TestAuthUserResync_Registered"
        status: pass
    human_judgment: false
  - id: D4
    description: "SC#3 storage/resolver primitive: a standing key's EffectiveRole drops writer→reader after UpdateUserRole, on the SAME token's next resolve — no re-mint, no re-login (cursor #5)"
    requirement: "AUTH-02"
    verification:
      - kind: integration
        ref: "internal/auth/resync_integration_test.go#TestResync_LiveRoleClamp"
        status: pass
    human_judgment: false
  - id: D5
    description: "SC#3 operator RPC seam: driving the demotion through the ResyncUserRole RPC handler (not authStore.UpdateUserRole directly) clamps the same standing token writer→reader on its next resolve (cursor #3)"
    requirement: "AUTH-02"
    verification:
      - kind: integration
        ref: "internal/server/identity_resync_integration_test.go#TestResync_RPCSeam_LiveRoleClamp"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-10
status: complete
---

# Phase 02 Plan 06: AUTH-02 Operator-Driven Forced Role Re-Sync Summary

**`ResyncUserRole` handler — the single reusable D-01/D-04 seam that writes a user's role via the existing `UpdateUserRole` path (instantly clamping every standing key on its next request via the live-role read) and hard-revokes active keys when `revoke_keys` is set — plus the `auth user resync` CLI and integration proof of the standing-key live floor at both the storage primitive and the RPC seam.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-07-10T00:56:41Z
- **Completed:** 2026-07-10T01:04:27Z
- **Tasks:** 3 (all `tdd="true"`)
- **Files modified:** 2 (+4 created)

## Accomplishments

- **`ResyncUserRole` handler** replaces the `CodeUnimplemented` stub in `internal/server/identity_handler.go`. It writes `users.role` through the existing `UpdateUserRole` path — the live floor every standing key clamps to on its next request via `resolveAPIKey`'s per-request DB read (SC#3) — with input guards (`CodeInvalidArgument` on empty id/role) and `identityError` mapping (unknown user → `CodeNotFound`). When `revoke_keys` is set it lists the user's keys and hard-revokes every active one (D-02 off-board; `IsActive`-filtered so the soft path never fires a spurious revoke). The role stays an explicit operator input (D-01 seam, no IdP derivation D-05); no new storage method, no schema change (D-03).
- **Redacted audit line** (`user.resync`) logs only server-derived fields — target, applied role, revoke intent, and count of keys revoked — never token material (T-02-22).
- **`auth user resync <user-id> --role <r> [--revoke-keys]` CLI** mirrors `authUserSetRoleCmd`: required `--role`, `--revoke-keys` bool, `--json` via `printJSON` else a human summary; registered under the `auth user` group.
- **Integration proof of SC#3 at two depths** (both `//go:build integration`, modeled on `mint_integration_test.go`): `TestResync_LiveRoleClamp` isolates the storage+resolver primitive (writer→reader on the same token after `UpdateUserRole`), and `TestResync_RPCSeam_LiveRoleClamp` (in-package `server`) drives the demotion through the `ResyncUserRole` **RPC** and proves the same clamp — closing cursor #5 and cursor #3.

## Task Commits

1. **Task 1 (RED): failing tests for ResyncUserRole** — `523d4524` (test)
2. **Task 1 (GREEN): ResyncUserRole handler** — `99550edb` (feat)
3. **Task 2: `auth user resync` CLI subcommand** — `ebf2d2e1` (feat; includes the errcheck lint fix — see Deviations)
4. **Task 3: integration proof (storage primitive + RPC seam)** — `6945f52d` (test)

**Plan metadata:** _(docs commit — see below)_

_Note: Tasks 1–3 were `tdd="true"`. Task 1 followed a clean RED→GREEN commit split (the pre-existing stub let the RED test compile and fail with `Unimplemented`). Task 2's test could not compile before the command symbols existed, so its RED was verified by the build failure (undefined `authUserResyncCmd`/flag vars) and the implementation landed as one `feat` commit — the standard Go TDD unit. Task 3 is test-only (integration proof of already-shipped behavior)._

## Files Created/Modified

- `internal/server/identity_handler.go` — replaced the `ResyncUserRole` stub with the real admin-gated handler (reuses `UpdateUserRole` + `ListAPIKeys` + `RevokeAPIKey`, redacted audit line)
- `internal/server/identity_resync_test.go` (new) — unit coverage: role write, revoke_keys true/false, unknown user → NotFound, empty id/role → InvalidArgument (assertions on connect codes)
- `cmd/specgraph/auth_user.go` — added `authUserResyncCmd` + `--role`/`--revoke-keys` flags + registration
- `cmd/specgraph/auth_user_test.go` (new) — CLI coverage via stub identity server: id/role/flag forwarding, no-revoke default, `--json`, client-error surfacing, registration
- `internal/auth/resync_integration_test.go` (new) — `TestResync_LiveRoleClamp` (storage/resolver primitive)
- `internal/server/identity_resync_integration_test.go` (new) — `TestResync_RPCSeam_LiveRoleClamp` (in-package, RPC seam)

## Decisions Made

- **Reuse over new seam:** `ResyncUserRole` writes through the existing `UpdateUserRole` path rather than adding a storage method or schema (D-01/D-03/D-04). The live-role read at `resolveAPIKey` does the propagation; no new machinery needed.
- **Explicit operator role input** (`--role`), never IdP-derived this phase (D-05), so the same entrypoint is directly callable by a future automation driver with a derived role.
- **`IsActive`-filtered revoke** so the convergence (soft) path provably revokes nothing; already-revoked keys are idempotent no-ops on the hard path.
- **RPC-seam integration test in-package** (`package server`) so the handler is constructed directly against a real `AuthStore` and the demotion is driven through the RPC — proving the operator seam, not only the storage primitive (cursor #3).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `errcheck` on `MarkFlagRequired`**
- **Found during:** Task 2 (`task lint:go` in the verification gate)
- **Issue:** `_ = authUserResyncCmd.MarkFlagRequired("role")` tripped `errcheck` (the repo config checks blank assignments).
- **Fix:** Switched to the repo-wide convention `cobra.CheckErr(authUserResyncCmd.MarkFlagRequired("role"))` (used in claim.go, lifecycle.go, decision.go, etc.).
- **Files modified:** `cmd/specgraph/auth_user.go`
- **Verification:** `task lint:go` → 0 issues; `go test ./cmd/specgraph/ -run TestAuthUserResync` green.
- **Committed in:** `ebf2d2e1` (folded into the Task 2 CLI commit)

---

**Total deviations:** 1 auto-fixed (1 blocking lint). **Impact:** aligns the new command with the established `cobra.CheckErr` convention; no scope change.

## Issues Encountered

- `task check` fails at `fmt:check` on **pre-existing** dprint drift in unrelated `.planning/intel/**/*.json` files (none touched by this plan) — the same out-of-scope drift documented in the 02-05 summary. All Go gates pass: `go build ./...`, `task lint:go` (0 issues), and the unit + both integration suites are green.
- Pre-existing `golangci-lint` findings under the `integration` build tag (`identitystore_integration_test.go` gosec, `usersbackend_stub_test.go` nolintlint, `identity_integration_test.go` revive, `identitystore_authn_integration_test.go` unparam) are unrelated to this plan and left untouched (scope boundary). My four new files are lint-clean.

## Deferred Issues

- None deferred for lack of Docker: Docker was available, so **both** integration tests (`TestResync_LiveRoleClamp`, `TestResync_RPCSeam_LiveRoleClamp`) ran and passed against real Postgres. `TestResync_RevokeKeys` (end-to-end resync-with-revoke) remains at the `task pr-prep` phase gate by plan design — the revoke path is unit-proven (Task 1) and its storage behavior is covered by Plan 02's suite.

## Next Phase Readiness

- AUTH-02 is complete: an operator forces a role demotion (and optional hard off-board) that reaches standing keys immediately via the live-role read, through the reusable D-01 seam. SC#3 is proven at unit and integration depth.
- The `ResyncUserRole` seam is ready for a future automation/resync driver (explicit role input, no schema coupling).
- Ready for 02-07.

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-10*

## Self-Check: PASSED

- `internal/server/identity_resync_test.go` — FOUND
- `cmd/specgraph/auth_user_test.go` — FOUND
- `internal/auth/resync_integration_test.go` — FOUND
- `internal/server/identity_resync_integration_test.go` — FOUND
- `ResyncUserRole` real handler present; `CodeUnimplemented` stub GONE
- Commits `523d4524`, `99550edb`, `ebf2d2e1`, `6945f52d` — all FOUND
- `go build ./...` — OK; `go test ./internal/server/ ./cmd/specgraph/` — PASS
- `go test -tags integration` (both resync tests) — PASS (Docker); `task lint:go` — 0 issues
