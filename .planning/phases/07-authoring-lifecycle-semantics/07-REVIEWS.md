---
phase: 7
reviewers: [cursor]
reviewed_at: 2026-07-14T00:00:00Z
review_pass: 2
plans_reviewed: [07-01-PLAN.md, 07-02-PLAN.md, 07-03-PLAN.md, 07-04-PLAN.md, 07-05-PLAN.md]
reviewer_models:
  cursor: composer-2.5
---

# Cross-AI Plan Review — Phase 7 (Second Pass)

> Second review pass after the plans were revised via `/gsd-plan-phase 7 --reviews` to incorporate the first Cursor review. The first-pass HIGH finding (`re_entry_stage` allowlist gap) is confirmed resolved; this pass surfaces one new MEDIUM implementation footgun plus a test-coverage gap.

## Cursor Review

# Phase 7 Authoring Lifecycle Semantics — Second-Pass Plan Review

## 1. Summary

The revised five-plan wave (07-01 → 07-02 → 07-03 → 07-04 → 07-05) is **substantially sound** and correctly grounded in the repo. Source verification confirms the core diagnosis: the lifecycle path already implements correct amend/supersede semantics, while the MCP `author` tool still calls the divergent authoring path. The prior review's **`IsValidReEntryStage()` fix is correctly specified** in Plan 07-03 (handler + storage, shared helper, no tool-side allowlist duplication), and it **does not break** `PrecedingAuthStage` land-one-before behavior. Wave linearization and 07-04's two-step proto-then-Go deletion strategy are **verified correct** against Go interface mechanics and current call sites.

**Overall risk: MEDIUM** — down from the first pass because the re-entry allowlist gap is now addressed in-plan; remaining risk is mostly implementation footguns and one edge-case gap (`spark` re-entry).

---

## 2. Strengths

### Root-cause diagnosis is accurate and evidenced

| Path | Evidence | Mechanism |
|------|----------|-----------|
| **Broken MCP path** | `internal/mcp/tools_authoring.go:357-381` | `handleAmend` / `handleSupersede` call `client.Authoring.Amend` / `Supersede` with `target_stage` / `superseded_by` |
| **Correct CLI/lifecycle path** | `cmd/specgraph/lifecycle.go:50-54`, `internal/server/lifecycle_handler.go:67` | CLI uses `TransitionAmend` → `LifecycleAmendSpec` |
| **`client.Lifecycle` already wired** | `internal/mcp/client.go:25`, `:43` | MCP reroute is a call-site swap, not new infrastructure |

### Correct lifecycle semantics already exist in storage

- **Land-one-before re-entry:** `internal/storage/postgres/lifecycle.go:47-52` computes `landingStage := targetStage.PrecedingAuthStage()`; integration test at `internal/storage/postgres/lifecycle_test.go:32-33` asserts `re_entry "shape"` → lands at `spark`.
- **Amend in-flight only:** `internal/storage/postgres/lifecycle.go:61-65` UPDATE guard restricts to `approved|in_progress|review`; eligibility helper at `internal/storage/spec_domain.go:77-85`.
- **Supersede done-only:** `internal/storage/postgres/lifecycle.go:123-124` returns `ErrSpecNotDone` when old spec is not `done`.

### Broken authoring path semantics match the plan's "inverted" table

- **Amend rejects `approved`:** `internal/storage/postgres/authoring.go:368-369` returns `ErrSpecAlreadyApproved`.
- **Supersede has no done guard:** `internal/storage/postgres/authoring.go:314-318` UPDATEs any existing spec to `superseded` without stage precondition (contrast `lifecycle.go:123-124`).
- **Amend lands AT target (not one-before):** `internal/storage/postgres/authoring.go:382-384` sets `stage = targetStage` directly (no `PrecedingAuthStage`).

### `IsValidReEntryStage()` fix is correctly placed and consistent

**Current bug (confirmed):** Handler at `internal/server/lifecycle_handler.go:57-64` uses `IsValid()` + `ExcludesReEntry()`. `ExcludesReEntry()` at `internal/storage/spec_domain.go:29-35` rejects only `done|superseded|abandoned`. Tests at `internal/storage/spec_domain_test.go:25-35` explicitly assert `approved`, `in_progress`, and `review` are **non-excluding**. Storage repeats the weak check at `internal/storage/postgres/lifecycle.go:43-44`.

**Plan 07-03 fix (correct):**
- Single helper on `SpecStage` in `spec_domain.go`
- Handler replaces `IsValid()+ExcludesReEntry()` with `!IsValidReEntryStage()` → `CodeInvalidArgument` for both CLI and MCP (`lifecycle_handler.go` is the single RPC gate)
- Storage defense-in-depth uses the same helper → `ErrInvalidReEntryStage`, mapped to `CodeInvalidArgument` at `internal/server/lifecycle_handler.go:314-315` (tested at `internal/server/lifecycle_handler_test.go:1163-1178`, `internal/server/error_mapper_internal_test.go:173`)

**No double-rejection of valid stages:** Handler rejects invalid values before storage; for `spark|shape|specify|decompose`, both layers accept. **Land-one-before preserved:** `PrecedingAuthStage()` at `internal/storage/spec_domain.go:92-97` is unchanged; plan only tightens the *input* allowlist, not landing math.

### Claim-release plan (07-02) matches existing patterns

- **DELETE pattern to copy:** `internal/storage/postgres/execution.go:162-177` — claims row by `(project_slug, spec_slug, agent)`, then `CLAIMED_BY` edge with `from_slug=spec`, `to_slug=agent`.
- **`GetActiveClaim`:** `internal/storage/postgres/execution.go:326-338` returns `nil` when unclaimed (no error).
- **Placement inside amend tx:** Plan inserts claim release after successful stage UPDATE, before `recomputeContentHash` (`lifecycle.go:71-83`) — correct transaction boundary per ADR-004.

### Wave linearization is sound

| Wave | Dependency rationale | Source evidence |
|------|---------------------|-----------------|
| 07-01 → 07-02 | Both touch `lifecycle.go`; 07-02 supersede-reason integration tests need `LifecycleSupersedeSpec(..., reason)` signature from 07-01 | Current signature at `internal/storage/lifecycle.go:114`, impl `lifecycle.go:113`; no `reason` field in proto `TransitionSupersedeRequest` (`proto/specgraph/v1/lifecycle.proto:114-117`) |
| 07-02 → 07-03 | MCP reroute must not ship before claim release on in-flight amend | No claim release in `LifecycleAmendSpec` today (`lifecycle.go:55-107`) |
| 07-03 → 07-04 | Only prod callers of `Authoring.Amend/Supersede` are MCP tool (`tools_authoring.go:357,377`) — must reroute first | `rg` confirms single non-test production usage |
| 07-04 → 07-05 | Skills/e2e teach `re_entry_stage`/`new_slug` params that 07-03 introduces | Skills have no amend content today; e2e file `mcp_only_lifecycle_test.go` does not exist (only `mcp_only_authoring_test.go`) |

### 07-04 two-boundary compile-safe deletion holds

- `internal/server/authoring_handler.go:33` — `var _ AuthoringServiceHandler = (*AuthoringHandler)(nil)` with **no** `Unimplemented` embed.
- Removing RPC lines from proto first drops methods from generated interfaces while Go handler methods remain as legal extras — valid Go semantics.
- Task 1 verify (`! rg Authoring.Amend` in non-test `internal/`) is safe **because 07-03 runs first** and removes the only prod callers.

### 07-05 e2e design is well-grounded

- Claim tool uses `spec_slug`: `internal/mcp/tools_execution.go:55`, `:80-82`
- Report completion uses `slug`: `internal/mcp/tools_execution.go:343`, `:369-371`
- Distinct slugs per scenario avoids contradictory state — appropriate given amend and supersede preconditions differ.

---

## 3. Concerns

### HIGH — None remaining from first pass

The first-pass **HIGH** finding (`re_entry_stage` allowlist accepting `approved|in_progress|review`) is **resolved in Plan 07-03 Task 1** with verified-accurate root-cause citations and a correct two-layer enforcement design.

---

### MEDIUM — Plan 07-03 ambiguously suggests `authoringStages` for the allowlist

**Severity:** MEDIUM (implementation footgun)

Plan 07-03 Task 1 action says: *"explicit switch, or membership over `authoringStages`"*.

**Problem:** `authoringStages` at `internal/storage/stage_validation.go:10-16` includes **`SpecStageApproved`** as its fifth element. Using that slice directly would make `IsValidReEntryStage()` return true for `approved`, reintroducing the bug.

**Evidence:**

```
var authoringStages = []SpecStage{
	SpecStageSpark,
	SpecStageShape,
	SpecStageSpecify,
	SpecStageDecompose,
	SpecStageApproved,
}
```
(`internal/storage/stage_validation.go:10-16`)

**Recommendation:** Strike "membership over `authoringStages`" from the plan; require an explicit four-value switch or a dedicated `reEntryStages` slice of exactly `{spark, shape, specify, decompose}`.

---

### MEDIUM — Integration tests do not yet cover `approved|in_progress|review` as invalid re-entry (post-fix gap in 07-02)

**Severity:** MEDIUM (test coverage gap, not a design flaw)

`internal/storage/postgres/lifecycle_test.go:166-181` (`AmendSpec_InvalidReEntryStage`) only rejects `done|superseded|abandoned`. Plan 07-02 Task 2 extends supersede-reason and claim-release tests but does **not** explicitly add storage integration cases for `approved|in_progress|review` after the allowlist fix (07-03 adds handler unit tests only).

**Risk:** Storage could regress if someone reverts only the storage guard. Defense-in-depth is weaker without integration coverage.

**Recommendation:** Add one integration sub-case in 07-02 or 07-03 asserting `LifecycleAmendSpec(..., "approved")` → `ErrInvalidReEntryStage`.

---

### LOW — `re_entry_stage=spark` may still produce a same-stage no-op on re-author

**Severity:** LOW (edge case; LIFE-02 examples use `shape`)

`PrecedingAuthStage(spark)` returns `spark` (`spec_domain.go:92-95`, `idx <= 0`). After amend with `re_entry_stage=spark`, the spec lands at `spark`. Plan 07-03's next-step hint tells the agent to run `author action=spark`, but `ValidateTransition` rejects same-to-same (`internal/storage/stage_validation.go:51-52`).

Proto and CLI already allow `spark` as re-entry (`proto/specgraph/v1/lifecycle.proto:104-105`, `cmd/specgraph/lifecycle.go:310`). This is pre-existing, not introduced by the revision, but 07-05 e2e should use `shape` (as planned) and skills should not imply `spark` re-entry is the primary happy path.

---

### LOW — Handler validation has redundant `ReEntryStage != ""` branch

**Severity:** LOW (cosmetic)

`internal/server/lifecycle_handler.go:54-56` already rejects empty `re_entry_stage`; lines `57-65` wrap validation in `if msg.ReEntryStage != ""` unnecessarily. Plan 07-03 should simplify to a single path when editing this block.

---

### LOW — `ExcludesReEntry()` name/doc remains misleading after fix

**Severity:** LOW (maintainability)

`ExcludesReEntry()` doc at `spec_domain.go:27-28` says stages that "cannot be used as a re-entry target," but `approved|in_progress|review` return false. Plan correctly adds `IsValidReEntryStage()` without updating `ExcludesReEntry` semantics. Consider a doc comment clarifying `ExcludesReEntry` is terminal-only exclusion, not the D-03 allowlist.

---

## 4. Suggestions

1. **07-03 Task 1:** Require explicit four-value allowlist only; remove `authoringStages` membership option (see MEDIUM concern).
2. **07-02 or 07-03:** Add storage integration assertion that `approved`, `in_progress`, and `review` are rejected as `re_entry_stage` with `ErrInvalidReEntryStage`.
3. **07-03 Task 1 handler test:** Plan already requires `re_entry_stage=approved` AND `done` → `CodeInvalidArgument`. Add `in_progress` or `review` as a third case for completeness (mirrors `spec_domain_test.go:25-35`).
4. **07-05 skills:** Teach land-one-before using `shape` as the canonical example (already planned); note that `spark` re-entry is allowed by the API but re-running `spark` on a spec already at `spark` is a no-op.
5. **07-01:** Proto analog reference is accurate — `TransitionAbandonRequest.reason = 2` at `proto/specgraph/v1/lifecycle.proto:124-127`; `TransitionSupersedeRequest` has fields 1–2 only today (`:114-117`), so `reason = 3` is free.
6. **07-04 Task 1:** Confirm `mockAuthoringService` assignment in `internal/mcp/testhelpers_test.go` — extra methods after interface shrink are fine (same mechanism as `AuthoringHandler`).

---

## 5. Risk Assessment

**Overall: MEDIUM**

| Area | Level | Rationale |
|------|-------|-----------|
| Consolidation strategy | **LOW** | Single correct path exists; MCP is the only prod caller of broken path |
| `IsValidReEntryStage()` fix | **LOW** | Correctly specified; shared helper; does not alter `PrecedingAuthStage` |
| Wave ordering | **LOW** | File overlap and semantic dependencies verified |
| 07-04 deletion | **LOW** | Two-step proto-RPC-first strategy matches `authoring_handler.go:33` |
| Claim release (07-02) | **LOW** | Pattern copied from `execution.go:162-177`; conditional on `GetActiveClaim` |
| Implementation execution | **MEDIUM** | `authoringStages` footgun; integration test gap for new rejections |
| Phase verification (07-05) | **LOW** | E2e template exists (`mcp_only_authoring_test.go`); claim/report arg names verified |

The revised plans **adequately address the first-pass HIGH finding**. Execute with attention to the `authoringStages` trap and extend integration tests for the newly rejected execution stages. No structural replanning is needed.

---

## Consensus Summary

Single reviewer (Cursor / composer-2.5), source-grounded second pass. Overall risk downgraded to **MEDIUM** (from the first pass) — the HIGH `re_entry_stage` gap is resolved. Two actionable items remain for a `/gsd-plan-phase 7 --reviews` pass:

### Agreed Strengths
- First-pass HIGH finding resolved: `IsValidReEntryStage()` correctly placed at handler + storage, shared helper, land-one-before (`PrecedingAuthStage`) preserved.
- Wave linearization, 07-04 two-boundary deletion, and 07-02 claim-release all re-verified against source.

### Agreed Concerns (priority order)
1. **MEDIUM — `authoringStages` footgun (new).** Plan 07-03 Task 1 offers "membership over `authoringStages`" as an allowlist option, but that slice (`stage_validation.go:10-16`) includes `SpecStageApproved` — using it would reintroduce the exact first-pass bug. Fix: require an explicit four-value switch or a dedicated `reEntryStages` slice of `{spark,shape,specify,decompose}` only, and strike the `authoringStages` option.
2. **MEDIUM — integration-test gap.** The `approved|in_progress|review` re-entry rejections are only covered by handler unit tests (07-03); add a storage integration assertion (`LifecycleAmendSpec(..., "approved")` → `ErrInvalidReEntryStage`) in 07-02/07-03 for defense-in-depth.
3. **LOW ×3** — `spark` re-entry same-stage no-op edge (pre-existing; keep `shape` as the canonical e2e/skill example), redundant `ReEntryStage != ""` handler branch, and a misleading `ExcludesReEntry()` doc comment.

### Divergent Views
None — single reviewer.
