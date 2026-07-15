---
phase: 07-authoring-lifecycle-semantics
fixed_at: 2026-07-15T00:00:00Z
review_path: .planning/phases/07-authoring-lifecycle-semantics/07-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 7: Code Review Fix Report

**Fixed at:** 2026-07-15
**Source review:** .planning/phases/07-authoring-lifecycle-semantics/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (fix_scope: all — WR-01, WR-02, IN-01, IN-02, IN-03, IN-04)
- Fixed: 6
- Skipped: 0
- `task check`: **green (exit 0)** after all fixes — fmt:check → license:check → lint → build → unit tests → skills:validate.

## Fixed Issues

### WR-01: MCP `author` amend hint tells the agent to run a command that errors for `re_entry_stage=spark`

**Files modified:** `internal/mcp/tools_authoring.go`, `internal/mcp/tools_authoring_test.go`
**Commit:** 17b3423c
**Applied fix:** `handleAmend` now branches on `reEntryStage == "spark"`. When the spec
lands at `spark` (PrecedingAuthStage(spark)==spark), it emits a terminal-stage hint
("There is no forward re-author command for spark; edit the seed via the normal flow.")
instead of the failing `Next step: run author action=spark …` guidance that would route
to CreateSpec → ALREADY_EXISTS. All non-spark re-entry stages keep the original
next-step hint. Added `TestAuthorTool_Amend_SparkReEntry` asserting the emitted content
does NOT contain `action=spark` and does surface a spark terminal-stage explanation.

### WR-02: `AcknowledgeDrift` handler does not enforce the proto-required `note`

**Files modified:** `internal/server/lifecycle_handler.go`, `internal/server/lifecycle_handler_test.go`, `internal/server/error_sanitize_test.go`
**Commit:** c7ef6920
**Applied fix:** Replaced the length-only check with
`validateRequiredField("note", msg.Note)` (the same helper used for the required
`reason` field), which rejects both empty and over-length notes with
`connect.CodeInvalidArgument` before any store write. Added
`TestLifecycleHandler_AcknowledgeDrift_MissingNote` (asserts CodeInvalidArgument and
that the store is never reached for an empty note). Updated the existing
`error_sanitize_test.go` AcknowledgeDrift subtest to pass a non-empty `Note` so it still
reaches the errorBackend and exercises CodeInternal sanitization as intended. Test
assertions use connect error codes, not message strings, per project convention.

### IN-01: Skill doc mislabels the spark re-entry failure mode as a "no-op"

**Files modified:** `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md`
**Commit:** 847979ee
**Applied fix:** Reworded the caveat from "…is a same-stage no-op. It is API-allowed but
degenerate — never present it as the happy path." to "Re-running `author action=spark` on
an existing spec returns `ALREADY_EXISTS` — there is no spark re-author path. Never
present it as a next step." Now consistent with WR-01's tool behavior and the actual
`CreateSpec` → ALREADY_EXISTS semantics. `task skills:validate` passes.

### IN-02: Concurrent supersede race mislabels *which* spec is terminal

**Files modified:** `internal/storage/postgres/lifecycle.go`
**Commit:** 793c5666
**Status:** fixed — **requires human verification** (rare concurrent-only path; not
covered by a deterministic unit test; postgres integration test is Docker-gated and did
not run in `task check`).
**Applied fix:** In `LifecycleSupersedeSpec`, the new-spec `RowsAffected()==0` branch no
longer delegates to the shared `preconditionError` (whose generic terminal check returns
`ErrSpecTerminal`, which `lifecycleError` then formats against the OLD slug). It now
classifies the new-spec guard failure locally: not-found → `ErrNewSpecNotFound`, version
mismatch → `ErrConcurrentModification`, terminal → `ErrNewSpecTerminal`, else
`ErrInternalGuardFailure`. This surfaces the replacement spec as the terminal one instead
of misattributing terminality to the spec being superseded. The change is isolated to the
supersede path — it deliberately does NOT reorder the shared `preconditionError` (which
would have changed the amend/abandon diagnostics), so there is no blast radius to the
amend or abandon flows. Build + `go vet` pass; full `task check` green.

### IN-03: Amend does not clear stale downstream stage outputs

**Files modified:** `internal/storage/postgres/lifecycle.go`
**Commit:** 639cf793
**Applied fix:** Documented (rather than changed behavior). Extended the D-08 comment in
`LifecycleAmendSpec` to state explicitly that shape/specify/decompose stage outputs (and
the slices derived from decompose output) are **intentionally retained** on amend and
overwritten only when their stage is re-authored — so the agent re-enters authoring with
prior work as the starting point, and readers must treat `stage` (not the presence of a
downstream output blob) as the authoritative contract boundary.
**Why documented rather than nulled:** Nulling downstream outputs would change the
recomputed content hash and the re-author starting state, a behavior change beyond the
finding's scope that risks destabilizing the passing amend/e2e suite. The finding
explicitly offered documentation as an acceptable resolution.

### IN-04: `ValidateTransition` still permits arbitrary backward transitions

**Files modified:** `internal/storage/stage_validation.go`
**Commit:** 4c9e8069
**Applied fix:** Documented (rather than tightened). Added a doc-comment note and an
inline note explaining that `ValidateTransition` is a low-level *structural* validator
shared by the general `TransitionStage` path (including export/import restore), and that
Phase-7 amend semantics (re-entry allowlist, amend-eligibility, claim release) are
enforced by `LifecycleAmendSpec`, not by this validator. The permissive backward branch is
intentionally retained.
**Why documented rather than tightened:** (1) The existing `stage_validation_test.go`
suite explicitly asserts backward (amend) transitions such as `shape→spark`,
`approved→decompose`, and `approved→spark` are valid; forward-only would flip those and
destabilize the passing suite. (2) `internal/export/engine.go` uses `TransitionStage` to
reconstruct persisted stages during restore. (3) No in-scope caller performs a backward
`TransitionStage` (authoring handlers only transition forward; the `done→terminal`
restore transition already fails structural validation and is handled as a warning), so
the escape hatch is latent, not an active bug. The finding explicitly offered "add a
comment documenting why the permissive backward branch must remain" as an acceptable fix.

## Notes

- **Docker-gated tests:** The IN-02 change touches postgres code whose behavior is
  exercised by `//go:build integration` tests (Docker/testcontainers), which are NOT part
  of `task check`. `task check` compiled and unit-tested the package (green), but the
  concurrent-supersede race diagnostic itself is best confirmed via
  `task test:integration` with Docker available, or by manual review. Flagged as
  "requires human verification" above.
- **No proto changes** were required for any finding (WR-02 was enforced handler-side, as
  the project guidance anticipated). `gen/` was not touched.

---

_Fixed: 2026-07-15_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
