---
phase: 04-verification-integration-reliability
plan: 02
subsystem: docs
tags: [drift, documentation, connectrpc, mcp, lifecycle]

requires:
  - phase: 04-01
    provides: LifecycleService.CheckDrift/AcknowledgeDrift + MCP drift tool (already shipped, verified)
provides:
  - "site/docs/concepts/drift.md documents drift as queryable across CLI, API, and MCP (SC#1 gap closed)"
affects: [drift, documentation, DRFT-01]

tech-stack:
  added: []
  patterns:
    - "MkDocs concept doc mirrors CLI Usage section idiom for a sibling API/MCP access note"

key-files:
  created: []
  modified:
    - site/docs/concepts/drift.md

key-decisions:
  - "Confirm-and-lightly-extend only (D-04): added one sibling ## section, no rewrite, no cli-reference.md regen"
  - "Preserved the existing interfaces/verify 'Planned' stub admonition intact (D-03)"

patterns-established:
  - "Doc note names the same server-side content-hash detection reachable identically from CLI, API, and MCP"

requirements-completed: [DRFT-01]

coverage:
  - id: D1
    description: "site/docs/concepts/drift.md gains an ## Accessing Drift via API / MCP section (between CLI Usage and Worked Example) naming LifecycleService.CheckDrift, LifecycleService.AcknowledgeDrift, and the MCP drift tool (check/acknowledge actions), stating all three surfaces share identical content-hash detection on DEPENDS_ON edges"
    requirement: "DRFT-01"
    verification:
      - kind: other
        ref: 'rg -q "^## Accessing Drift via API / MCP" && rg -q "LifecycleService.CheckDrift" && rg -q "LifecycleService.AcknowledgeDrift" && rg -q "drift.*tool" && rg -q ''info \"Planned\"'' site/docs/concepts/drift.md'
        status: pass
      - kind: other
        ref: "task lint:markdown — no issues reported for site/docs/concepts/drift.md"
        status: pass
    human_judgment: true
    rationale: "04-VALIDATION.md flags 'API/MCP access documentation reads correctly' as a human judgment — prose accuracy/readability against the live LifecycleService RPCs and MCP drift tool warrants a human sign-off, even though structural/naming criteria auto-pass."

duration: 2 min
completed: 2026-07-10
status: complete
---

# Phase 4 Plan 02: Accessing Drift via API / MCP Documentation Summary

**Added a concise `## Accessing Drift via API / MCP` section to the drift concept doc naming `LifecycleService.CheckDrift`/`AcknowledgeDrift` (ConnectRPC) and the MCP `drift` tool, closing DRFT-01's SC#1 "CLI/API/MCP" documentation gap.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-07-10T16:14:42Z
- **Completed:** 2026-07-10T16:16:09Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Documented drift status as queryable across all three surfaces (CLI, API, MCP), satisfying SC#1's CLI/API/MCP naming (D-04)
- New `## Accessing Drift via API / MCP` section placed between `## CLI Usage` and `## Worked Example`, mirroring the surrounding MkDocs idiom
- Symbol names verified accurate against live code before writing (`internal/server/lifecycle_handler.go` `CheckDrift`/`AcknowledgeDrift`; `internal/mcp/tools_lifecycle.go` `drift` tool `check`/`acknowledge` actions)
- Existing `!!! info "Planned"` note about the `interfaces`/`verify` stub scopes left fully intact (D-03)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add "Accessing Drift via API / MCP" section to drift.md** — `b6a21e73` (docs)

## Files Created/Modified

- `site/docs/concepts/drift.md` — added a 16-line sibling `## Accessing Drift via API / MCP` section documenting the API (ConnectRPC `LifecycleService.CheckDrift`/`AcknowledgeDrift`) and MCP (`drift` tool, `check`/`acknowledge` actions) surfaces, noting all three surfaces share identical content-hash detection on `DEPENDS_ON` edges

## Decisions Made

- **Confirm-and-lightly-extend only (D-04):** added a single sibling `##` section rather than rewriting the doc; no `cli-reference.md` regeneration.
- **Preserved the "Planned" stub note (D-03):** the `--scope interfaces|verify` admonition remains verbatim.
- **Accuracy-first:** verified `slug`-empty semantics (all eligible specs), `upstream_slug` vs `all: true` acknowledge semantics, and MCP action names directly against source before writing.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. `task lint:markdown` reports pre-existing issues only in unrelated `.planning/` planning artifacts (out of scope per the scope boundary); `site/docs/concepts/drift.md` itself has zero lint issues.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- DRFT-01 SC#1 (documented, queryable interface across CLI/API/MCP) is now closed in docs; 04-01 already verified SC#2 with real-DB tests.
- Phase 4 (DRFT-01 only, after INTG-01 descope per D-05) plan work complete — ready for verification.

---
*Phase: 04-verification-integration-reliability*
*Completed: 2026-07-10*
