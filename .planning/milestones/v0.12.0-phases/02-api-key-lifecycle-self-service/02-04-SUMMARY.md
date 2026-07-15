---
phase: 02-api-key-lifecycle-self-service
plan: 04
subsystem: auth
tags: [cedar, authorization, rbac, api-keys, go]

requires:
  - phase: 02-01
    provides: generated IdentityService self-service + ResyncUserRole procedure constants (CreateMyAPIKey, ListMyAPIKeys, RotateMyAPIKey, RevokeMyAPIKey, ResyncUserRole)
provides:
  - "exported auth.RoleMin(a, b Role) Role — fail-closed role-floor primitive"
  - "apikey.self Cedar verb: \"self\" in knownVerbs + apikey.self permit in base.cedar"
  - "procedure→action map: four self procedures → apikey.self, ResyncUserRole → user.manage"
  - "self-only-on-apikey.* drift test"
affects: [02-05, 02-06, self-mint handlers, ResyncUserRole handler]

tech-stack:
  added: []
  patterns:
    - "Boot-invariant grouping: knownVerbs + base.cedar + actions.go land in one commit or NewCedarEngine fails"
    - "Fail-closed role comparison reusing the single roleRank ordering (no second comparator)"
    - "Broad Cedar permit + handler-enforced restriction where Cedar cannot see Source/floor"

key-files:
  created:
    - internal/auth/rolemin.go
    - internal/auth/rolemin_test.go
  modified:
    - internal/auth/engine.go
    - internal/auth/policies/base.cedar
    - internal/auth/actions.go
    - internal/auth/actions_test.go

key-decisions:
  - "RoleMin reuses the existing roleRank map rather than introducing a second comparator (RESEARCH 'Don't Hand-Roll')"
  - "base.cedar apikey.self permit admits any authenticated role; the handler (Plan 05) enforces the Source reject + RoleMin floor Cedar cannot see"
  - "The self-verb drift test iterates the exported ActionNames() (procedureActions is unexported to the external test package) — equivalent distinct-action-value coverage"

patterns-established:
  - "Boot invariant: verb registry (knownVerbs), policy (base.cedar), and action map (actions.go) must agree in a single commit"
  - "Fail-closed authz primitive: unranked/empty input collapses to RoleReader, never the fuller role"

requirements-completed: [AUTH-03, AUTH-02]

coverage:
  - id: D1
    description: "Exported fail-closed auth.RoleMin returning the less-privileged role, RoleReader when either side is unranked/empty"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/auth/rolemin_test.go#TestRoleMin"
        status: pass
    human_judgment: false
  - id: D2
    description: "apikey.self verb registered so the Cedar engine boots (\"self\" in knownVerbs + apikey.self permit in base.cedar)"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "internal/auth/engine_test.go#TestNewCedarEngine_LoadsPolicies"
        status: pass
      - kind: unit
        ref: "go build ./... (engine constructs with real ActionNames + base.cedar)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Procedure→action map: four self procedures → apikey.self, ResyncUserRole → user.manage; mirror lists + self-only-on-apikey drift test"
    requirement: "AUTH-02"
    verification:
      - kind: unit
        ref: "internal/auth/actions_test.go#TestActionForProcedure_Identity"
        status: pass
      - kind: unit
        ref: "internal/auth/actions_test.go#TestActionNames_SelfVerbConfinedToAPIKey"
        status: pass
      - kind: unit
        ref: "internal/auth/actions_test.go#TestActionNames_AllParseToKnownVerb"
        status: pass
    human_judgment: false

duration: 2min
completed: 2026-07-09
status: complete
---

# Phase 02 Plan 04: apikey.self Verb + RoleMin Floor Summary

**Exported fail-closed `auth.RoleMin` plus the `apikey.self` Cedar verb (knownVerbs + base.cedar permit + five procedure→action map entries) landed with the boot invariant intact, all gated by mirror and self-only drift tests.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-07-09T16:53:53Z
- **Completed:** 2026-07-09T16:56:21Z
- **Tasks:** 3
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- Exported `auth.RoleMin(a, b Role) Role` — fail-closed to `RoleReader` when either role is unranked/empty, reusing the existing `roleRank` ordering (the role-floor primitive the Plan 05 self-mint create/rotate handlers consume).
- Registered the `apikey.self` authorization verb across the three boot-coupled sites in one atomic commit: `"self"` in `knownVerbs` (engine.go), an `apikey.self` permit for any authenticated principal (base.cedar), and the action map entries (actions.go).
- Mapped the four self procedures to `apikey.self` (AUTH-03) and `ResyncUserRole` to the admin-only `user.manage` verb (AUTH-02).
- Added a self-only-on-`apikey.*` drift test plus updates to both previously hard-coded mirror lists (the verb allow-list and the Identity procedure→action mirror).

## Task Commits

1. **Task 1: Exported auth.RoleMin (fail-closed) + table test** — `a50adf6b` (feat)
2. **Tasks 2 & 3: Register apikey.self verb + self-service action map (boot invariant)** — `54747aa8` (feat)

_Tasks 2 and 3 were committed together because the boot invariant requires knownVerbs, base.cedar, and actions.go to agree in a single commit — an `apikey.self` action entry without the `"self"` verb in `knownVerbs` makes `NewCedarEngine` fail at construction._

## Files Created/Modified
- `internal/auth/rolemin.go` - Exported fail-closed `RoleMin`, reusing `roleRank`
- `internal/auth/rolemin_test.go` - `TestRoleMin` table test (ranked min both orders + fail-closed cases)
- `internal/auth/engine.go` - Added `"self"` to `knownVerbs`
- `internal/auth/policies/base.cedar` - Added `apikey.self` permit for any authenticated role
- `internal/auth/actions.go` - Five procedure→action entries (four self → `apikey.self`, ResyncUserRole → `user.manage`)
- `internal/auth/actions_test.go` - Verb allow-list + Identity mirror updates, new self-only drift test

## Decisions Made
- `RoleMin` reuses `roleRank` rather than duplicating an ordering table (RESEARCH "Don't Hand-Roll").
- The `apikey.self` Cedar permit is intentionally broad (any authenticated role); the source reject and `RoleMin` role floor are enforced by the handler in Plan 05 because Cedar only sees `principal.role/id/email`.
- The self-verb drift test iterates the exported `ActionNames()` (the external `auth_test` package cannot reach the unexported `procedureActions`); this covers the identical distinct-action-value set the plan's "iterate procedureActions" wording intended.

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

Task 1 was marked `tdd="true"`. The RED→GREEN discipline was followed in execution: `rolemin_test.go` was written first and `go test ./internal/auth/ -run TestRoleMin` was run, failing with `undefined: auth.RoleMin` (RED); `rolemin.go` was then added and the test passed (GREEN). A separate `test(...)` RED commit was NOT created because Go cannot compile a `_test.go` file referencing an undefined symbol, and the `lint-go` pre-commit hook (`golangci-lint`, typecheck) rejects a non-compiling commit — and sequential mode forbids `--no-verify`. The test and implementation were therefore committed together as the standard Go TDD unit (`a50adf6b`). This is a commit-granularity note only; the red-green cycle itself was honored.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The auth foundation for Plans 05 and 06 is in place: `auth.RoleMin` (role floor), the `apikey.self` verb (boots cleanly), and admin-gated `ResyncUserRole`.
- Plan 05 must implement the handler-side enforcement the broad Cedar permit defers: reject `Source == "apikey"` and floor the minted role via `RoleMin`.

## Self-Check: PASSED

- `internal/auth/rolemin.go` — FOUND
- `internal/auth/rolemin_test.go` — FOUND
- Commit `a50adf6b` — FOUND
- Commit `54747aa8` — FOUND
- `go build ./...` — OK; `go test ./internal/auth/...` — PASS

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-09*
