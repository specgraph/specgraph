---
phase: 07-authoring-lifecycle-semantics
reviewed: 2026-07-15T12:21:12Z
depth: deep
files_reviewed: 28
files_reviewed_list:
  - cmd/specgraph/lifecycle.go
  - e2e/api/mcp_only_lifecycle_test.go
  - internal/auth/actions.go
  - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
  - internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md
  - internal/mcp/testhelpers_test.go
  - internal/mcp/tools_authoring_test.go
  - internal/mcp/tools_authoring.go
  - internal/server/authoring_handler_test.go
  - internal/server/authoring_handler.go
  - internal/server/error_sanitize_test.go
  - internal/server/lifecycle_handler_test.go
  - internal/server/lifecycle_handler.go
  - internal/server/test_scoper_test.go
  - internal/storage/authoring.go
  - internal/storage/lifecycle.go
  - internal/storage/postgres/authoring_test.go
  - internal/storage/postgres/authoring.go
  - internal/storage/postgres/changelog_test.go
  - internal/storage/postgres/graph_ready_provenance_test.go
  - internal/storage/postgres/lifecycle_test.go
  - internal/storage/postgres/lifecycle.go
  - internal/storage/spec_domain_test.go
  - internal/storage/spec_domain.go
  - internal/storage/stage_validation_test.go
  - internal/storage/stage_validation.go
  - proto/specgraph/v1/authoring.proto
  - proto/specgraph/v1/lifecycle.proto
findings:
  critical: 1
  warning: 2
  info: 3
  total: 6
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-07-15T12:21:12Z
**Depth:** deep
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Deep, cross-layer review of the Phase 7 amend/supersede/abandon/ack-drift semantics.
I traced every lifecycle mutation end-to-end (CLI → ConnectRPC handler → storage
interface → postgres impl, plus the MCP `author` tool → `LifecycleService`) and
audited transaction boundaries, guard duplication, the error taxonomy, and the
(stage, operation) transition matrix.

**Prior-pass fixes verified as held.** WR-01 (spark-landing hint in
`tools_authoring.go:385-389` emits a terminal-stage hint rather than a guaranteed
ALREADY_EXISTS re-author), IN-02 (concurrent-supersede new-spec guard classifies
locally at `lifecycle.go:204-228` instead of misnaming the old slug), IN-03
(amend retains stage-output blobs) and IN-04 (`ValidateTransition` intentionally
permissive on backward moves) are all intact. Guard duplication between handler
and storage is genuine defense-in-depth, not divergence: the re-entry allowlist
(`IsValidReEntryStage`), amend-eligibility, terminal-stage and version guards are
enforced at both layers and cannot disagree (both reject the same sets;
`IsValidReEntryStage` deliberately excludes `approved`). Sentinel→connect-code
mapping in `lifecycleError` is consistent and tested for every branch in
`error_mapper_internal_test.go`.

The review nonetheless surfaced defects that a per-file standard pass missed
because they only appear when you follow the amend *re-author* flow past the
landing stage, and when you compare the three "spec is no longer executable"
mutations against each other:

- **CR-01 (BLOCKER):** the primary amend re-author flow (`re_entry_stage=decompose`,
  and any amend that walks back through decompose) re-runs `StoreDecomposeOutput`,
  which treats pre-existing child Slice nodes as immutable — silently discarding
  re-authored slice content and/or orphaning removed slices. No test exercises
  amend→re-author.
- **WR-01:** `LifecycleAbandonSpec` does not release the active claim / CLAIMED_BY
  edge, breaking the exact invariant (D-08) that amend and `RecordCompletion` both
  enforce, and leaving a dangling graph edge to a terminal spec.

## Critical Issues

### CR-01: Amend → re-decompose silently discards re-authored slice content and orphans removed slices

**File:** `internal/storage/postgres/authoring.go:213-233` (call chain: `internal/mcp/tools_authoring.go:handleAmend` / `cmd/specgraph/lifecycle.go:runAmend` → `LifecycleService.TransitionAmend` → `LifecycleAmendSpec` → author action=decompose → `AuthoringHandler.Decompose` → `store.StoreDecomposeOutput`)

**Issue:**
Phase 7 makes `re_entry_stage=decompose` a first-class amend target: amend lands
the spec one stage *before* decompose (`PrecedingAuthStage(decompose) == specify`,
`spec_domain.go:116-122`), so the very next author action is `decompose`
(specify→decompose). At that point the spec **already carries the child Slice
nodes** created by the original decompose (IN-03 intentionally retains
`decompose_output` and, per its own note, "the slices derived from decompose
output"). Re-running `StoreDecomposeOutput` does **not** reconcile them:

- `authoring.go:214-231` — for each incoming slice it calls `GetSlice(childSlug)`;
  if the slice already exists (the amend case) it takes the `else`/skip branch and
  **never updates** `Intent`, `Verify`, `Touches`, or `DependsOn`. Re-authored
  slice bodies are silently dropped — the agent's corrected decomposition is
  accepted by the RPC (returns 200, echoes the input) but the graph keeps the old
  slice content. This is silent data loss on the core re-author path.
- If the re-authored decomposition uses **different** slice IDs, the old Slice
  nodes (with their `BELONGS_TO` / `COMPOSES` / inter-slice `DEPENDS_ON` edges,
  `authoring.go:205-253`) are left in the graph, no longer referenced by the
  parent's overwritten `decompose_output.SliceSlugs` (`authoring.go:255-262`).
  Orphaned nodes/edges corrupt downstream graph queries (impact, critical-path,
  ready-set) that traverse `COMPOSES`/`DEPENDS_ON`.

This diverges from the sibling re-author stages: `StoreShapeOutput` /
`StoreSpecifyOutput` overwrite their JSON blob via `storeJSONColumn`
(`authoring.go:126-173`), so re-authoring shape/specify works; only decompose,
because it materializes child nodes under a "idempotent for retries" assumption
(`authoring.go:213`), breaks under amend. That assumption was safe pre-Phase-7
(decompose ran once) but is now reachable as a re-author.

The IN-03 rationale explicitly scopes itself to *retaining* prior work as a
starting point; it does not cover reconciling child nodes on re-author, so this
is a genuinely new gap, not the documented one. It is untested: `lifecycle_test.go`
and `authoring_test.go` verify amend landing + claim release but never re-author
through decompose.

**Fix:**
Make `StoreDecomposeOutput` reconcile against the existing decomposition when the
parent is being re-authored. Concretely, inside the transaction:

```go
// Before Pass 1, load the parent's currently-stored slice set.
existing, _ := s.readStoredSliceSlugs(txCtx, slug) // from decompose_output.SliceSlugs
incoming := make(map[string]bool, len(output.Slices))
for _, sl := range output.Slices {
    incoming[fmt.Sprintf("%s/%s", slug, sl.ID)] = true
}
// Delete slices (and their edges) that are no longer part of the decomposition.
for _, old := range existing {
    if !incoming[old] {
        if err := sliceBackend.DeleteSlice(txCtx, old); err != nil { // cascade BELONGS_TO/COMPOSES/DEPENDS_ON
            return fmt.Errorf("postgres: prune stale slice %q: %w", old, err)
        }
    }
}
// In Pass 1, when GetSlice succeeds, UPDATE the slice body instead of skipping:
if getErr == nil {
    if err := sliceBackend.UpdateSlice(txCtx, sliceDomain); err != nil {
        return fmt.Errorf("postgres: update slice %q: %w", childSlug, err)
    }
}
```

Add an integration test that amends a decomposed spec, re-authors decompose with
(a) changed slice intents and (b) a removed slice, then asserts the slice bodies
updated and the removed slice + its edges are gone. If reconciliation is judged
out of scope, at minimum fail fast: reject `re_entry_stage=decompose` (and
re-decompose while child slices exist) with a clear error rather than silently
losing data.

## Warnings

### WR-01: Abandon does not release the active claim / CLAIMED_BY edge (invariant asymmetry with amend + complete)

**File:** `internal/storage/postgres/lifecycle.go:302-360` (compare `LifecycleAmendSpec` claim release at `lifecycle.go:98-115` and `RecordCompletion` at `execution.go:161-177`)

**Issue:**
`LifecycleAmendSpec` explicitly releases the claim on the grounds that "a spec
returning to authoring is no longer executable, so its active lease must not
linger" (D-08, `lifecycle.go:83-115`), deleting both the `claims` row and the
`CLAIMED_BY` edge inside the transaction. `RecordCompletion` does the same on the
done transition (`execution.go:161-177`). `LifecycleAbandonSpec` transitions a
spec to the **terminal** `abandoned` state — strictly *more* "no longer
executable" than amend — yet performs **no** claim release. An abandoned spec that
was claimed keeps a live `claims` row and a dangling `CLAIMED_BY` edge pointing at
a terminal node.

Reachability is confirmed across the call graph: `ClaimSpec` has no stage guard
(`claim.go:21-123` claims any existing spec), and abandon is permitted from
`approved`/`in_progress`/`review` (`lifecycle.go:309`, `terminalStages` only
blocks superseded/abandoned) — the same claimable stages the amend test itself
uses (`lifecycle_test.go:697-703`). Consequences: the agent's active-claim view
and graph reachability queries surface a terminal spec; the `CLAIMED_BY` edge
violates the "leases only on executable specs" invariant the phase established.
It self-heals only when the 15-minute lease expires and `ReleaseExpiredClaims`
runs (`execution.go:277-299`), so it is not data loss — hence Warning, not
Blocker — but it is an incorrect, untested state (`lifecycle_test.go` has a
dedicated `TestLifecycleAmend_ReleasesClaim` but no abandon-with-claim analogue).

**Fix:**
Mirror the amend claim-release block inside `LifecycleAbandonSpec`'s transaction
(ideally extract a shared `releaseClaim(txCtx, slug)` helper used by amend,
abandon, and complete so the three cannot drift again):

```go
claim, claimErr := s.GetActiveClaim(txCtx, slug)
if claimErr != nil {
    return fmt.Errorf("postgres: abandon spec: get active claim: %w", claimErr)
}
if claim != nil {
    if _, err := s.exec(txCtx, `DELETE FROM claims WHERE project_slug=$1 AND spec_slug=$2 AND agent=$3`,
        s.project, slug, claim.Agent); err != nil { return fmt.Errorf("postgres: abandon spec: delete claim: %w", err) }
    if _, err := s.exec(txCtx, `DELETE FROM edges WHERE project_slug=$1 AND from_slug=$2 AND to_slug=$3 AND edge_type='CLAIMED_BY'`,
        s.project, slug, claim.Agent); err != nil { return fmt.Errorf("postgres: abandon spec: delete CLAIMED_BY edge: %w", err) }
}
```

Add an integration test: claim an in_progress spec, abandon it, assert
`GetActiveClaim` is nil and `countClaimedByEdges` is 0.

### WR-02: No test covers the amend → re-author round trip or its error paths

**File:** `internal/storage/postgres/lifecycle_test.go:20-166`, `internal/server/lifecycle_handler_test.go`, `e2e/api/mcp_only_lifecycle_test.go`

**Issue:**
Cross-referencing the transition matrix against the suites, the amend tests stop
at the landing stage: they assert the spec lands at `PrecedingAuthStage(target)`
and that the claim is released, but **no** test then runs the forward
`shape`/`specify`/`decompose` re-author command that the whole feature exists to
enable. This is precisely the region where CR-01 lives (re-decompose) and where
IN-03's "retained outputs as starting point" claim is exercised. Also uncovered:
`re_entry_stage=spark` landing behavior end-to-end through the MCP hint
(`tools_authoring.go:385-389`) at the storage level, and the interaction of amend
with pre-existing downstream `DEPENDS_ON` drift beyond the single
`TestLifecycle_AmendRefreshesEdgeHash` hash check.

**Fix:**
Add an integration test that: amends an approved/decomposed spec to
`re_entry_stage=shape` and `=decompose`, walks the funnel forward, and asserts the
re-authored outputs (shape blob overwrite, decompose slice reconciliation) are
what actually persist. This test would have caught CR-01.

## Info

### IN-01: Supersede-on-terminal error message misdirects the caller to amend

**File:** `internal/storage/postgres/lifecycle.go:157-159`, `internal/server/lifecycle_handler.go:282-284`

**Issue:**
When the old spec is `superseded`/`abandoned` (terminal), supersede returns
`ErrSpecNotDone`, which the handler renders as "must be in done stage; use amend
for in-flight specs". A terminal spec is not "in-flight", and amend also rejects
terminal specs (`ErrSpecTerminal`), so the remediation hint sends the user to an
operation guaranteed to fail. Confirmed by `SupersedeSpec_TerminalState`
(`lifecycle_test.go:276-293`) which asserts `ErrSpecNotDone` for an abandoned old
spec.

**Fix:** Either return `ErrSpecTerminal` when `oldCheck.Stage` is fully terminal
(distinct from a merely non-done stage), or soften the message to "must be in done
stage" without the misleading amend suggestion.

### IN-02: `ErrSpecIneligibleForDrift` branch in `lifecycleError` is effectively dead

**File:** `internal/server/lifecycle_handler.go:297-299`

**Issue:**
`LifecycleAcknowledgeDrift` returns `ErrSpecIneligibleStage` (`lifecycle.go:389`),
not `ErrSpecIneligibleForDrift`; the latter is produced only in `drift.go` whose
own comment (`drift.go:177`) notes it is "currently unreachable". Both are mapped
to the same `CodeFailedPrecondition`, so no behavioral bug — but the
`ErrSpecIneligibleForDrift` branch in the lifecycle handler is dead code carrying
a distinct user-facing string that will never be emitted from this path.

**Fix:** Drop the unreachable branch, or add a comment pinning where (if anywhere)
it is expected to fire, to prevent future readers assuming it is live.

### IN-03: Concurrent mutually-superseding done specs can surface as CodeInternal instead of retryable CodeAborted

**File:** `internal/storage/postgres/lifecycle.go:174-228`

**Issue:**
`LifecycleSupersedeSpec` locks the old spec's row (UPDATE) then the new spec's row.
Two concurrent supersedes with swapped old/new (A→B and B→A, both done) acquire the
two row locks in opposite order — a classic lock-order inversion. Postgres detects
the deadlock and aborts one transaction with a deadlock error, which is not matched
by any sentinel in `lifecycleError` and therefore surfaces as `CodeInternal`
("internal error") rather than the retryable `CodeAborted` used for the
version-guard `ErrConcurrentModification` path (`lifecycle_handler.go:306-308`).
Exotic (requires two simultaneous mutually-replacing done specs) and safe
(Postgres preserves consistency), hence Info.

**Fix:** Acquire the two row locks in a deterministic order (e.g. sort
`{oldSlug,newSlug}` and lock the lexicographically-smaller row first), or detect
the pgx deadlock SQLSTATE `40P01` in `lifecycleError` and map it to `CodeAborted`
so callers retry rather than see an opaque internal error.

---

_Reviewed: 2026-07-15T12:21:12Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: deep_
