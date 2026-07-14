---
phase: 07-authoring-lifecycle-semantics
plan: 05
subsystem: testing
tags: [lifecycle, amend, supersede, re-entry, mcp, e2e, ginkgo, skills]

# Dependency graph
requires:
  - phase: 07-authoring-lifecycle-semantics
    provides: 07-03 rerouted the MCP author amend/supersede handlers onto LifecycleService (re_entry_stage/new_slug params + next-step hint); 07-04 retired the divergent authoring-level path leaving a single source of truth
provides:
  - specgraph-authoring SKILL.md teaches amend/supersede/re-entry with the land-one-before model (redo shape -> land at spark -> author action=shape) and the spark same-stage no-op caveat
  - specgraph-troubleshooting SKILL.md maps a post-amend "invalid stage transition" to the land-one-before rule
  - e2e/api/mcp_only_lifecycle_test.go — MCP-only Ginkgo suite proving done->amend->re-author->supersede + both rejection cases through author/claim/report tools
affects: [mcp-authoring, lifecycle, phase-8]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP-only e2e harness per Describe: dedicated project slug + in-process mcppkg.NewServer over a project-scoped ConnectRPC transport, asserting only via ReadResource/CallTool (no specgraphv1connect client)"
    - "Per-scenario distinct spec slugs so no single spec is used contradictorily (driven-to-done vs amended-in-flight)"

key-files:
  created:
    - e2e/api/mcp_only_lifecycle_test.go
  modified:
    - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
    - internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md

key-decisions:
  - "e2e suite reuses the friendly-YAML/exchanges fixtures and harness shape from mcp_only_authoring_test.go but defines its own mcpLifecycleClient bound to a dedicated project (mcp-only-lifecycle-project) for DB isolation."
  - "Amend-landing assertion uses substring `spark` (protojson stage field) + next-step-hint substring `action=shape` rather than message equality — honors AGENTS.md (assert codes/substrings, never sanitized message strings)."
  - "supersede-on-non-done negative test creates a valid non-terminal replacement so the ONLY failure reason is the non-done precondition (the done-check runs before the replacement-check in LifecycleSupersedeSpec)."

patterns-established:
  - "MCP-only lifecycle verification: drive to done via claim(spec_slug+agent) + report completion(slug+agent), noting the deliberate per-tool arg-name asymmetry."

requirements-completed: [LIFE-01, LIFE-02]

coverage:
  - id: D1
    description: "specgraph-authoring + specgraph-troubleshooting SKILL.md teach amend/supersede/re-entry with the land-one-before model (redo shape -> land at spark -> author action=shape), canonical shape happy-path, and the degenerate spark same-stage no-op caveat"
    requirement: "LIFE-02"
    verification:
      - kind: other
        ref: "task skills:validate (7 packages OK) + rg re_entry_stage/new_slug present, no stale target_stage/superseded_by"
        status: pass
    human_judgment: false
  - id: D2
    description: "MCP-only e2e: an in-flight (approved) spec is amended with re_entry_stage=shape, lands one stage before (spark), the next-step hint names shape, and re-authoring shape succeeds (no #899 no-op)"
    requirement: "LIFE-02"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_lifecycle_test.go#amends an in-flight spec back and re-authors the landed stage (LIFE-02)"
        status: pass
    human_judgment: false
  - id: D3
    description: "MCP-only e2e: a spec is driven to done via author+claim+report and superseded with a non-terminal replacement (LIFE-01)"
    requirement: "LIFE-01"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_lifecycle_test.go#supersedes a done spec with a replacement (LIFE-01)"
        status: pass
    human_judgment: false
  - id: D4
    description: "MCP-only e2e: amend-on-done and supersede-on-non-done are both rejected (res.IsError true)"
    requirement: "LIFE-01"
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_lifecycle_test.go#rejects amend on a done spec (LIFE-01); #rejects supersede on a non-done in-flight spec (LIFE-01)"
        status: pass
    human_judgment: false

# Metrics
duration: 16min
completed: 2026-07-14
status: complete
---

# Phase 7 Plan 5: MCP-First Amend/Supersede/Re-entry Teaching + MCP-only e2e Summary

**Completed the Phase 6 self-teaching story by teaching amend/supersede/re-entry (with the land-one-before model) in the `specgraph-authoring` and `specgraph-troubleshooting` skills, and added the phase's primary verification gate — `e2e/api/mcp_only_lifecycle_test.go`, a fully MCP-driven Ginkgo suite that (across distinct per-scenario specs) amends an in-flight spec + re-authors the landed stage, drives a spec to done + supersedes it, and asserts amend-on-done and supersede-on-non-done are both rejected — all through the author/claim/report MCP tools.**

## Performance

- **Duration:** ~16 min
- **Started:** 2026-07-14T22:02:00Z
- **Completed:** 2026-07-14T22:17:46Z
- **Tasks:** 2
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments
- Added an "Amending and superseding a spec" section to `specgraph-authoring/SKILL.md`: a precondition table (amend while in-flight; supersede only on done), the land-one-before model taught verbatim in the user's framing (`re_entry_stage: shape` → lands at `spark` → `author action=shape` succeeds), the canonical `shape` happy-path example, and an explicit caveat that `re_entry_stage: spark` is API-allowed but a degenerate same-stage no-op.
- Added a troubleshooting entry mapping a post-amend "invalid stage transition" to the land-one-before rule (run `author action=<re_entry_stage>`, not the stage the spec landed at).
- Created `e2e/api/mcp_only_lifecycle_test.go` — a dedicated MCP-only Ginkgo suite (its own `mcpLifecycleClient` harness + `mcp-only-lifecycle-project` for DB isolation) that exercises the full amend/supersede/re-entry surface purely through the `author`, `claim`, and `report` MCP tools with distinct per-scenario slugs.
- Verified the consolidated lifecycle semantics end-to-end: all 4 new specs pass under Docker; `task check` (incl. `skills:validate`) green.

## Task Commits

Each task was committed atomically:

1. **Task 1: Teach amend/supersede/re-entry in the MCP-first skills** — `50c98bfb` (docs)
2. **Task 2: MCP-only e2e — done→amend→re-author→supersede with rejection cases** — `3a78ae94` (test)

**Plan metadata:** _(this SUMMARY commit)_ (docs)

## Files Created/Modified
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` — Added the amend/supersede/re-entry section (precondition table, land-one-before model, shape happy path, spark no-op caveat).
- `internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md` — Added the post-amend "invalid stage transition" → land-one-before entry.
- `e2e/api/mcp_only_lifecycle_test.go` — New MCP-only Ginkgo suite (4 specs) for done→amend→re-author→supersede + both rejection cases.

## Decisions Made
- **Dedicated MCP harness + project for isolation** — reused the fixture consts (`mcpOnlySparkYAML`, etc.) and the harness shape from `mcp_only_authoring_test.go`, but defined a local `mcpLifecycleClient` bound to `mcp-only-lifecycle-project` so specs never collide with the other MCP-only suite. No `ClearAll` needed (distinct project + distinct slugs).
- **Assertion style** — amend-landing proven via substring `spark` (the protojson `stage` field) and the next-step hint via `action=shape`; rejections via `res.IsError`. Never asserts on sanitized error message strings (AGENTS.md).
- **Self-contained negative supersede** — the supersede-on-non-done case creates a valid non-terminal replacement so the sole failure cause is the non-done precondition (done-check precedes the new-spec check in `LifecycleSupersedeSpec`).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## TDD Gate Compliance
- **Task 2 (tdd="true"):** The behavior under test (MCP amend/supersede reroute + land-one-before re-entry) was already delivered by the prior waves — 07-03 (reroute to `LifecycleService`, `re_entry_stage`/`new_slug` params, next-step hint) and 07-04 (retirement of the divergent path). This plan's Task 2 is a **verification gate**, not new behavior, so there was no separate production-code GREEN step to write: the newly-authored e2e passed on first run against the already-consolidated implementation (4/4 specs). Per the fail-fast investigation rule, the "unexpected pass" was confirmed to be exactly this — the consolidated lifecycle path is correct — not a mis-scoped test. Committed as a single `test(07-05)` commit accordingly; no RED→GREEN split applies to an end-to-end assertion of pre-built, cross-wave behavior.

## Verification Results
- `task skills:validate` — 7 packages OK (both edited skills pass the schema, kebab-case name, summary extension).
- `rg` token checks — `re_entry_stage` and `new_slug` present in `specgraph-authoring/SKILL.md`; no stale `target_stage`/`superseded_by` tokens in either skill.
- `go vet -tags e2e ./e2e/api/` — exit 0 (the suite compiles; `go build` reports "no non-test Go files" as expected for a test-only package).
- `task test:e2e:api`-equivalent (`go test -tags e2e ./e2e/api/ -run TestAPI -ginkgo.focus 'MCP-only lifecycle'`, Docker `pgvector/pgvector:pg18`) — **4 Passed | 0 Failed** in 10.0s.
- `task check` (fmt:check → license:check → lint → build → unit tests → skills:validate) — exit 0.
- **Docker availability:** available locally (Colima, Docker 29.5.2); the MCP-only e2e gate ran successfully.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 7 (Authoring Lifecycle Semantics) is functionally complete: the MCP surface routes to the single correct lifecycle path, the divergent path is retired, the skills teach amend/supersede/re-entry, and the MCP-only e2e proves the consolidated semantics + both rejection paths. The phase gate `task pr-prep` (full integration + e2e under Docker) can now be run before `/gsd-verify-work`.

## Self-Check: PASSED
- `e2e/api/mcp_only_lifecycle_test.go` verified present on disk.
- Commits `50c98bfb`, `3a78ae94` verified in git log.
- `task check` exits 0; MCP-only lifecycle e2e suite passes 4/4 under Docker.

---
*Phase: 07-authoring-lifecycle-semantics*
*Completed: 2026-07-14*
