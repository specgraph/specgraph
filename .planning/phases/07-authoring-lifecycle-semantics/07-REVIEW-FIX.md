---
phase: 07-authoring-lifecycle-semantics
fixed_at: 2026-07-15T13:02:25Z
review_path: .planning/phases/07-authoring-lifecycle-semantics/07-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 07: Code Review Fix Report

**Fixed at:** 2026-07-15T13:02:25Z
**Source review:** .planning/phases/07-authoring-lifecycle-semantics/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (CR-01, WR-01, WR-02, IN-01, IN-02, IN-03)
- Fixed: 6
- Skipped: 0

**Quality gates:** `task check` → exit 0 (after clearing a stale golangci-lint
cache that was surfacing 70 phantom issues from a prior worktree; a fresh
`task lint` reports 0 issues). Integration suite (`-tags integration`,
Docker) and MCP-only e2e (`-tags e2e`) both pass. See "Verification" below.

## Fixed Issues

### CR-01: Amend → re-decompose silently discarded re-authored slice content and orphaned removed slices

**Files modified:** `internal/storage/postgres/authoring.go`, `internal/storage/postgres/slice.go`, `internal/storage/postgres/authoring_test.go`
**Commit:** 6b77eda6
**Applied fix:**
`StoreDecomposeOutput` now reconciles the incoming decomposition against the
previously-stored slice set instead of treating existing child Slice nodes as
immutable:
- Added `readStoredSliceSlugs` to read the parent's prior
  `decompose_output.SliceSlugs`.
- **Prune:** slices absent from the incoming set are deleted via the new
  `DeleteSlice`, which cascades every incident edge (BELONGS_TO, COMPOSES, and
  DEPENDS_ON in both directions) so no orphaned nodes/edges remain.
- **Update:** slices that already exist have their body overwritten via the new
  `UpdateSlice` (Intent/Verify/Touches/DependsOn), so a re-authored
  decomposition persists instead of being silently dropped.
- **Create:** genuinely new slices are created as before.
- **DEPENDS_ON reconciliation:** each incoming slice's outgoing DEPENDS_ON edges
  are cleared before recreation, so a re-authored slice whose dependency set
  changed does not retain stale edges.
All work stays inside the existing transaction. `UpdateSlice`/`DeleteSlice` were
added as concrete `*Store` methods (not added to the `SliceBackend` interface,
to avoid churn in three unrelated fakes; reconciliation is postgres-specific and
already manipulates the `edges` table directly, consistent with the existing
Pass-2 edge code). Cross-cutting invariants preserved: COMPOSES direction and
`content_hash_at_link` on DEPENDS_ON are untouched by the update path; the
parent content hash is recomputed by `storeJSONColumn` as before.

**Note (logic-heavy change — recommend human confirmation of semantics):**
`UpdateSlice` intentionally overwrites only the slice body and leaves
`status`/`assigned_to` intact (a re-author does not reset an in-progress
slice's claim). This matches the review's stated scope
("UPDATE existing slices' Intent/Verify/Touches/DependsOn"), but the desired
behaviour for re-decomposing a spec with *already-claimed* slices is a product
decision worth a human glance.

### WR-01: Abandon did not release the active claim / CLAIMED_BY edge

**Files modified:** `internal/storage/postgres/lifecycle.go`, `internal/storage/postgres/lifecycle_test.go`
**Commit:** e6c17616
**Applied fix:**
Extracted a shared `releaseActiveClaim(ctx, slug, op)` helper (gets the active
claim; deletes the `claims` row and the `CLAIMED_BY` edge; no-op when
unclaimed) and used it in both `LifecycleAmendSpec` (replacing its inline block)
and `LifecycleAbandonSpec` (new call inside the abandon transaction). This
restores the D-08 invariant for the terminal `abandoned` transition and ensures
amend and abandon cannot drift. Added `TestLifecycleAbandon_ReleasesClaim`
(claimed spec → claim + CLAIMED_BY edge gone; unclaimed spec → harmless no-op).
`RecordCompletion` was left as-is (it already deletes the claim using the
known completer `agent`; folding it into the helper was out of scope and would
have widened the diff into `execution.go`).

### WR-02: No test covered the amend → re-author round trip

**Files modified:** (covered by CR-01) `internal/storage/postgres/authoring_test.go`
**Commit:** 6b77eda6 (same as CR-01)
**Applied fix:**
Satisfied by CR-01's `TestStoreDecomposeOutput_ReconcilesOnReauthor`, which
walks the full round trip: decompose → move to an amend-eligible stage →
`LifecycleAmendSpec(re_entry_stage=decompose)` (asserts landing at
`PrecedingAuthStage(decompose)` = specify) → re-run `StoreDecomposeOutput` with
(a) changed slice bodies, (b) a removed slice, (c) a changed dependency, and
(d) a new slice — then asserts the persisted graph reflects the new
decomposition with no orphans. This is exactly the test the review said "would
have caught CR-01". The amend error paths (amend-on-done → `ErrSpecNotAmendable`,
re-entry validation) remain covered by the pre-existing
`TestLifecycleAmend_ReleasesClaim` and lifecycle handler tests.

### IN-01: Supersede-on-terminal error message misdirected the caller to amend

**Files modified:** `internal/storage/postgres/lifecycle.go`, `internal/storage/postgres/lifecycle_test.go`
**Commit:** f73d6a7e
**Applied fix:**
`LifecycleSupersedeSpec` now distinguishes a genuinely terminal old spec
(`superseded`/`abandoned`) from a merely non-done one: terminal old specs return
`ErrSpecTerminal` ("is in a terminal state"), while in-flight specs keep the
`ErrSpecNotDone` "use amend for in-flight specs" hint (which is correct advice
for them). Updated `SupersedeSpec_TerminalState` to assert `ErrSpecTerminal`.
Chose the "distinguish terminal" option over softening the message so the
helpful amend hint is preserved for the cases where amend genuinely applies.

### IN-02: `ErrSpecIneligibleForDrift` branch in `lifecycleError`

**Files modified:** `internal/server/lifecycle_handler.go`
**Commit:** 34a0b54c
**Applied fix (reviewer's second option — with a correction to the premise):**
The finding's premise that the branch is "effectively dead" is **incorrect**.
The reviewer traced only the `AcknowledgeDrift` path (which returns
`ErrSpecIneligibleStage`), but the branch is live via the **`CheckDrift` RPC**:
`CheckDrift` routes `driftChecker.Check()` errors through `lifecycleError`, and
`Check()` returns `ErrSpecIneligibleForDrift` at the top level when a specific
**non-done** spec is drift-checked (`internal/drift/drift.go` eligibility
guard). Removing the branch would regress a non-done drift-check from
`CodeFailedPrecondition` to an opaque `CodeInternal` **and** break the existing
`TestLifecycleError` case at `error_mapper_internal_test.go:168`. Therefore I
applied the review's *second* offered option — a comment pinning exactly where
the branch fires — rather than deleting it. (The separate `sanitizeDriftError`
branch in `drift.go:181` remains genuinely unreachable and already carries its
own note; it was not in scope for this finding.)

### IN-03: Concurrent mutually-superseding done specs could surface as CodeInternal (lock-order inversion)

**Files modified:** `internal/storage/postgres/lifecycle.go`, `internal/storage/postgres/lifecycle_test.go`
**Commit:** 9c5a9fc2
**Applied fix (concurrency — verified by reasoning + integration test):**
`LifecycleSupersedeSpec` now acquires row locks on **both** spec rows up front
via `SELECT 1 ... FOR UPDATE`, iterating in deterministic lexicographic slug
order (smaller slug first). Two concurrent mutually-superseding operations
(A→B and B→A) previously locked the old-then-new rows in opposite orders,
deadlocking; Postgres aborted one with SQLSTATE `40P01`, which no sentinel
matched, surfacing as an opaque `CodeInternal`. With a shared deterministic
order the two operations now agree on lock acquisition and serialize — the loser
proceeds to the version guard and fails with `ErrConcurrentModification`
(retryable `CodeAborted`). Preferred fixing the inversion at the source over
mapping `40P01` in `lifecycleError`, per the review. Added
`SupersedeSpec_ConcurrentMutual_NoDeadlock`, which runs both directions
concurrently and asserts neither surfaces `ErrInternalGuardFailure` nor a raw
deadlock, only recognized retryable/precondition sentinels.

## Verification

**`task check`:** exit 0. Note: the first run reported 70 lint issues that all
referenced a *different, non-existent* worktree path
(`../sv-07-reviewfix-SIJkSh/`) and none of the files I changed — stale
golangci-lint cache entries (including `nolintlint` "directive unused" errors
indicative of a version/cache mismatch) left by a prior run. After
`golangci-lint cache clean`, a fresh `task lint` reported **0 issues** and the
full `task check` passed exit 0.

**Integration tests** (`go test -tags integration ./internal/storage/postgres/...`, Docker):
- `TestStoreDecomposeOutput_ReconcilesOnReauthor` — PASS (CR-01 / WR-02)
- `TestLifecycleAbandon_ReleasesClaim` (Claimed + Unclaimed subtests) — PASS (WR-01)
- `TestLifecycle/SupersedeSpec_TerminalState` — PASS (IN-01)
- `TestLifecycle/SupersedeSpec_ConcurrentMutual_NoDeadlock` — PASS (IN-03)
- Full `TestLifecycle`, `TestStoreDecomposeOutput*`, `TestSlice*`,
  `TestCreateSlice`, amend/abandon suites — PASS (no regressions).

**E2E** (`go test -tags e2e ./e2e/api/...`, Docker): PASS — the phase's primary
MCP-only gate, exercised because CR-01 changes amend/decompose round-trip
behavior.

## Skipped Issues

None — all 6 in-scope findings were fixed.

---

_Fixed: 2026-07-15T13:02:25Z_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
