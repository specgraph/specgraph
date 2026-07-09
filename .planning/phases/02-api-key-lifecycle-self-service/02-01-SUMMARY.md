---
phase: 02-api-key-lifecycle-self-service
plan: 01
subsystem: api
tags: [protobuf, connectrpc, identity, api-keys, buf, grpc]

requires:
  - phase: 01-foundation
    provides: IdentityService proto + generated Go/TS surface, APIKey/User messages
provides:
  - CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey RPCs (AUTH-03, self-scoped)
  - ResyncUserRole RPC with revoke_keys hard off-board flag (AUTH-02, admin-only)
  - Self-mint request messages with NO user_id (owner derived from context)
  - Regenerated gen/ (Go stubs, ConnectRPC procedure constants, TypeScript client)
affects: [02-02, 02-03, 02-04, 02-05, auth-actions-map, server-handlers, cli, web]

tech-stack:
  added: []
  patterns:
    - "Self-scoped RPC request messages omit any owner/target field so a caller cannot name another owner at the wire level (owner derived from context)"
    - "Compile-stub handler methods return CodeUnimplemented to keep the tree building while real logic lands downstream"

key-files:
  created: []
  modified:
    - proto/specgraph/v1/identity.proto
    - gen/specgraph/v1/identity.pb.go
    - gen/specgraph/v1/specgraphv1connect/identity.connect.go
    - web/src/lib/api/gen/specgraph/v1/identity_pb.ts
    - internal/server/identity_handler.go

key-decisions:
  - "Self-mint request messages (CreateMyAPIKey/Rotate/Revoke/List) carry no user_id — owner is context-derived (T-02-01 mitigation)"
  - "ResyncUserRole is a distinct admin-only RPC (not self-scoped), with revoke_keys bool driving the hard off-board (D-02)"
  - "Corrected IdentityService doc comment to distinguish self-scoped *MyAPIKey RPCs from admin-only management RPCs (cursor #7)"
  - "Added CodeUnimplemented compile stubs for the five new handler methods so the tree builds; real handlers are downstream (Rule 3 blocker fix)"

patterns-established:
  - "Wire-level least authority: self RPCs express no target field, making cross-owner requests unrepresentable"

requirements-completed: [AUTH-02, AUTH-03]

coverage:
  - id: D1
    description: "Five IdentityService RPCs (CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey + ResyncUserRole) and their request/response messages added to identity.proto; self-mint requests carry no user_id; ResyncUserRoleRequest carries revoke_keys."
    requirement: "AUTH-03"
    verification:
      - kind: other
        ref: "grep -c 'rpc CreateMyAPIKey\\|rpc ListMyAPIKeys\\|rpc RotateMyAPIKey\\|rpc RevokeMyAPIKey\\|rpc ResyncUserRole' proto/specgraph/v1/identity.proto == 5"
        status: pass
      - kind: other
        ref: "no user_id field in any *MyAPIKeyRequest message body; ResyncUserRoleRequest has 'bool revoke_keys'"
        status: pass
    human_judgment: false
  - id: D2
    description: "gen/ regenerated deterministically via task proto (byte-stable on re-run); five ConnectRPC procedure constants present; Go tree builds."
    requirement: "AUTH-02"
    verification:
      - kind: integration
        ref: "task proto && git diff --quiet -- gen/ (byte-stable) && task build (exit 0)"
        status: pass
      - kind: other
        ref: "IdentityServiceCreateMyAPIKeyProcedure/ListMyAPIKeysProcedure/RotateMyAPIKeyProcedure/RevokeMyAPIKeyProcedure/ResyncUserRoleProcedure exist in gen/specgraph/v1/specgraphv1connect/identity.connect.go"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 01: IdentityService Self-Service + Resync RPC Contract Summary

**Added five IdentityService RPCs — four self-scoped `*MyAPIKey` RPCs (AUTH-03) plus the admin-only `ResyncUserRole` with a `revoke_keys` off-board flag (AUTH-02) — and regenerated the committed Go/ConnectRPC/TypeScript surface deterministically.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-09T16:28:17Z
- **Completed:** 2026-07-09T16:31:55Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Declared the five net-new RPCs and their eight request/response messages in `identity.proto`, reusing the existing `APIKey`/`User` types.
- Self-mint request messages (`CreateMyAPIKeyRequest`, `RotateMyAPIKeyRequest`, `RevokeMyAPIKeyRequest`, `ListMyAPIKeysRequest`) carry no `user_id`/target field — a caller cannot name another owner at the wire level (T-02-01).
- `ResyncUserRoleRequest` carries the `bool revoke_keys` hard off-board flag (D-02); `ResyncUserRole` documented as admin-only, distinct from the self RPCs.
- Regenerated `gen/` (Go stubs, ConnectRPC procedure constants, TS client) via `task proto`; verified byte-stable on re-run and that all five procedure constants exist.
- Tree builds green (`task build` exit 0).

## Task Commits

1. **Task 1: Add five RPCs + messages to identity.proto** - `327a090e` (feat)
2. **Task 2: Regenerate gen/ via task proto and build** - `b506fab2` (feat)

**Plan metadata:** committed separately with this SUMMARY (docs)

## Files Created/Modified
- `proto/specgraph/v1/identity.proto` - Five new RPCs + request/response messages; corrected service doc comment
- `gen/specgraph/v1/identity.pb.go` - Regenerated Go message types
- `gen/specgraph/v1/specgraphv1connect/identity.connect.go` - Regenerated ConnectRPC client/handler interfaces + procedure constants
- `web/src/lib/api/gen/specgraph/v1/identity_pb.ts` - Regenerated TypeScript client
- `internal/server/identity_handler.go` - Added five `CodeUnimplemented` compile stubs (real logic downstream)

## Decisions Made
- Self-mint request messages express no owner/target field (least-authority at the wire level).
- `ResyncUserRole` kept as a distinct admin-only RPC rather than folding into `UpdateUserRole`, so the `revoke_keys` off-board semantics stay explicit.
- Corrected the `IdentityService` doc comment to stop claiming all non-Whoami RPCs require admin.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added CodeUnimplemented compile stubs to IdentityHandler**
- **Found during:** Task 2 (regenerate gen/ and build)
- **Issue:** Adding the five RPCs expanded the generated `IdentityServiceHandler` interface, so the existing `IdentityHandler` (with its compile-time `var _ ... = (*IdentityHandler)(nil)` assertion) no longer satisfied the interface — `task build` failed with "missing method CreateMyAPIKey". Task 2's own acceptance criteria requires `task build` to exit 0.
- **Fix:** Appended five stub methods (`CreateMyAPIKey`, `ListMyAPIKeys`, `RotateMyAPIKey`, `RevokeMyAPIKey`, `ResyncUserRole`) returning `connect.NewError(connect.CodeUnimplemented, ...)`, each documented as a compile stub to be replaced by downstream handler plans.
- **Files modified:** `internal/server/identity_handler.go`
- **Verification:** `task build` exits 0; stubs return `CodeUnimplemented` (no functional/security surface).
- **Committed in:** `b506fab2` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking).
**Impact on plan:** Necessary to satisfy Task 2's `task build` acceptance criterion. No scope creep — stubs are inert (CodeUnimplemented) and explicitly deferred to downstream handler plans (Plan 04/05 per threat model). See Known Stubs below.

## Known Stubs

The five new handler methods in `internal/server/identity_handler.go` are compile stubs returning `CodeUnimplemented`. They are intentional and required only so the tree builds against the expanded generated interface. Real logic (owner-from-context, quota/rate-limit, hard off-board) is delivered by downstream plans in this phase (per the phase artifacts inventory / threat model: server self-handlers + `ResyncUserRole` handler). This plan's scope is proto + gen only.

## Issues Encountered
- Plan `files_modified` listed the ConnectRPC output path as `gen/specgraph/v1/identityv1connect/identity.connect.go`; the repo's buf config actually emits to `gen/specgraph/v1/specgraphv1connect/identity.connect.go`. Verified the five procedure constants in the actual path. No action needed beyond noting the corrected path.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Generated Go/ConnectRPC/TypeScript symbols for all five RPCs are available and byte-stable; downstream plans (auth actions-map, storage, server handlers, CLI, web) can import them and proceed in parallel.
- `task check`'s `fmt:check` reports 139 pre-existing unformatted `.planning/intel/**/*.json` files unrelated to this plan (out of scope; not touched or committed here).

## Self-Check: PASSED

- Files verified on disk: proto, gen (Go pb + connect), web TS, handler — all FOUND.
- Commits verified: `327a090e` (Task 1), `b506fab2` (Task 2) present in `git log`.
- `task build` exit 0; `task proto` byte-stable on re-run; five procedure constants present.

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*
