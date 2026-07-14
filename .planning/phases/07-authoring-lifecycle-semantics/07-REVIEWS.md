---
phase: 7
reviewers: [cursor]
reviewed_at: 2026-07-14T00:00:00Z
plans_reviewed: [07-01-PLAN.md, 07-02-PLAN.md, 07-03-PLAN.md, 07-04-PLAN.md, 07-05-PLAN.md]
reviewer_models:
  cursor: composer-2.5  # default model hit usage limit; fell back to Composer per Cursor's prompt
---

# Cross-AI Plan Review — Phase 7

## Cursor Review

# Phase 7: Authoring Lifecycle Semantics — Cross-AI Plan Review

## 1. Summary

The plans correctly diagnose Phase 7 as a **consolidation refactor**: the lifecycle path (`LifecycleService` → `LifecycleAmendSpec`/`LifecycleSupersedeSpec`) already implements the desired semantics, while the MCP `author` tool still calls the broken authoring path (`AuthoringService` → `AmendSpec`/`SupersedeSpec`). Verified in source: `internal/mcp/tools_authoring.go:357-381` calls `t.client.Authoring.Amend/Supersede`, whereas `cmd/specgraph/lifecycle.go:50-82` already uses `TransitionAmend`/`TransitionSupersede`. Wave ordering (proto reason → storage claim-release + MCP reroute in parallel → proto/Go retirement → skills + e2e) is sound, and the Plan 07-04 two-step compile-safe deletion strategy matches Go interface mechanics (`internal/server/authoring_handler.go:33`). The main gaps are **re_entry_stage validation** (D-03 says `spark|shape|specify|decompose` only, but handler/storage accept `approved` and execution stages) and **ambiguous e2e sequencing** in Plan 07-05 (in-flight amend vs. drive-to-done on the same spec).

---

## 2. Strengths

- **Divergent-path diagnosis is accurate and well-evidenced.**
  - Broken amend rejects `approved`: `internal/storage/postgres/authoring.go:368-369` returns `storage.ErrSpecAlreadyApproved`.
  - Broken amend lands **at** target stage: `internal/storage/postgres/authoring.go:382-384` sets `stage = targetStage` (not one-before).
  - Broken supersede has **no done guard**: `internal/storage/postgres/authoring.go:306-317` updates any existing spec to `superseded`.
  - Correct lifecycle amend allows in-flight only: `internal/storage/postgres/lifecycle.go:64-65` (`stage IN ('approved', 'in_progress', 'review')`).
  - Correct lifecycle supersede is done-only: `internal/storage/postgres/lifecycle.go:123-125` (`oldCheck.Stage != done → ErrSpecNotDone`).
  - Land-one-before math exists: `internal/storage/postgres/lifecycle.go:47-52` + `internal/storage/spec_domain.go:92-97`.

- **MCP reroute is low-risk plumbing.** `Client.Lifecycle` is already wired at `internal/mcp/client.go:25,43`; Plan 07-03 is a call-site swap, not new infrastructure.

- **Plan 07-04 compile-safety ordering is correct.** `AuthoringHandler` uses `var _ specgraphv1connect.AuthoringServiceHandler = (*AuthoringHandler)(nil)` with **no** `Unimplemented` embed (`internal/server/authoring_handler.go:33`). Removing RPC lines from proto first drops methods from generated interfaces while extra Go methods remain compilable — matches Go's "extra methods allowed" rule. Deleting messages before Go impls would orphan `specv1.AmendRequest` references in `internal/server/authoring_handler.go:551-616`.

- **Claim-release design (07-02) copies a proven pattern.** `RecordCompletion` deletes claim row then `CLAIMED_BY` edge at `internal/storage/postgres/execution.go:162-177`; `GetActiveClaim` returns `(nil, nil)` when unclaimed at `:337-338`; `claims` PK is `(project_slug, spec_slug)` at `internal/storage/postgres/migrations/001_initial_schema.sql:145` (at most one claim per spec).

- **Supersede `reason` threading (07-01) needs no migration.** `ChangeLogEntry.Reason` exists at `internal/storage/changelog.go:20`; `TransitionSupersedeRequest` currently lacks `reason` at `proto/specgraph/v1/lifecycle.proto:114-117` (field 3 is free). Default changelog text today is hardcoded at `internal/storage/postgres/lifecycle.go:217`.

- **Existing e2e/CLI coverage reduces regression risk.** ConnectRPC lifecycle tests already assert land-at-spark for `re_entry_stage=shape` (`e2e/api/lifecycle_test.go:69-70`, `e2e/api/lifecycle_pipeline_test.go:57-67`) and reject amend-on-done / supersede-on-non-done (`e2e/api/lifecycle_test.go:667-694`).

- **Blast-radius inventory is thorough.** Grep confirms production callers of `Authoring.Amend/Supersede` are only `internal/mcp/tools_authoring.go:357,377` plus handler impls and tests — safe to retire after 07-03 reroute.

---

## 3. Concerns

### HIGH — `re_entry_stage` allowlist mismatch (Plans 07-03, 07-05; D-03)

**Claim:** D-03 / Plan 07-03 require `re_entry_stage ∈ {spark, shape, specify, decompose}` and reject `approved`/`done`/terminal.

**Source reality:** Handler validation uses `IsValid()` + `ExcludesReEntry()` only (`internal/server/lifecycle_handler.go:57-64`). `ExcludesReEntry()` rejects only `done`, `superseded`, `abandoned` (`internal/storage/spec_domain.go:29-35`; tested at `internal/storage/spec_domain_test.go:25-35` where **`approved` is explicitly non-excluding**). Storage repeats the same check at `internal/storage/postgres/lifecycle.go:43-44`.

**Effect:** `re_entry_stage=approved` (or `in_progress`/`review`) would be **accepted** by the handler if the MCP tool passes values through as Plan 07-03 instructs ("do NOT re-validate stage names in the tool", Pitfall 5). That contradicts D-03, proto doc comment at `proto/specgraph/v1/lifecycle.proto:104-105`, and the error text at `internal/server/lifecycle_handler.go:290` ("one of: spark, shape, specify, decompose") which is only returned for `ErrReEntryStageRequired`, not invalid values.

**Severity:** HIGH for MCP-only agents — wrong re-entry targets could land at unexpected stages (e.g. `approved` → `decompose` via `PrecedingAuthStage`).

---

### MEDIUM — Plan 07-05 e2e sequencing is internally contradictory

**Behavior block** (`07-05-PLAN.md`) requires:
1. Drive a spec to **done** via author + claim + report (`tools_execution.go:79-103` claim, `:361-362` completion).
2. **Amend an in-flight spec** with re-entry and re-author.
3. Supersede a **done** spec.
4. Negative: amend on done; supersede on non-done.

Steps 1 and 2 cannot apply to the **same** spec without an intermediate state change. The action text does not specify a second slug for the in-flight amend path. Phase title "done→amend→re-author→supersede" also conflicts with amend-on-done rejection (step 4).

**Severity:** MEDIUM — implementer may produce a flaky or logically impossible single-spec flow.

---

### MEDIUM — Plan 07-03 internal contradiction on validation strategy

Task 1 action requires empty `re_entry_stage` messages listing `spark|shape|specify|decompose`, but also says pass strings through without re-validating ("Pitfall 5"). Threat register T-07-04 claims handler `ExcludesReEntry` covers invalid values — **verified false** for `approved` (see HIGH concern above).

**Severity:** MEDIUM — plan text will confuse implementers; behavior depends on unstated tool-side allowlist.

---

### MEDIUM — Stale claim gap is real and unimplemented (Plan 07-02)

`LifecycleAmendSpec` (`internal/storage/postgres/lifecycle.go:55-107`) performs stage update + changelog but **no claim release**. A spec amended from `in_progress`/`review` while claimed would retain its lease until Plan 07-02 lands. This matches #900's execution-state concern and Plan 07-02's T-07-01 (high severity) — the plan is right to prioritize it, but it is not yet in code.

**Severity:** MEDIUM (pre-implementation); becomes HIGH if 07-03 ships before 07-02.

---

### LOW — Supersede `reason` loses length validation (Plan 07-01)

Broken path requires and bounds `reason` at `internal/server/authoring_handler.go:610-614`. `TransitionSupersede` (`internal/server/lifecycle_handler.go:79-97`) validates slugs only — no `validateRequiredField` or max-length check on reason. Plan 07-01 explicitly adds no validation. Threat T-07-04's "length bounds inherited from handler" is **not supported** for supersede.

**Severity:** LOW (audit note only; parameterized SQL prevents injection).

---

### LOW — Artificial wave dependency 07-02 → 07-01

Claim-release in 07-02 does not depend on supersede `reason` (07-01). Dependency only serializes work; 07-02 could run parallel to 07-01. Not harmful, but slows the critical D-08 fix relative to 07-03 MCP reroute.

**Severity:** LOW.

---

### LOW — Skills gap confirmed (Plan 07-05)

No `re_entry_stage`, `amend`, or `supersede` teaching in `internal/mcp/skills/embedded/` today (grep empty). MCP tool docs still expose `target_stage` / `superseded_by` at `internal/mcp/tools_authoring.go:168-169`. Plan 07-05 correctly targets this gap.

**Severity:** LOW (planned work, not a plan error).

---

## 4. Suggestions

1. **Resolve `re_entry_stage` validation in one layer (before or with 07-03).**
   - **Option A (preferred for MCP):** Tool-side allowlist in `handleAmend` rejecting anything outside `{spark, shape, specify, decompose}` with `errResult`, matching D-03 and empty-field messaging.
   - **Option B:** Extend `ExcludesReEntry()` or add `IsValidReEntryStage()` in `internal/storage/spec_domain.go` and call it from `lifecycle_handler.go:57-64` and `lifecycle.go:43-44` so CLI + MCP + storage agree.
   - Update Plan 07-03 Pitfall 5 / threat T-07-04 to match whichever option is chosen.

2. **Clarify Plan 07-05 e2e fixture model.** Recommend explicit two-slug layout, e.g.:
   - `lifecycle-amend-spec`: approved → amend/re-entry/re-author (LIFE-02).
   - `lifecycle-done-spec` + `lifecycle-replacement`: approved → claim/report → done → supersede (LIFE-01); negative amend-on-done on the done slug; negative supersede-on-non-done on a third in-flight slug.
   - Align phase title with actual order (in-flight amend **before** done, or rename).

3. **Serialize 07-02 before 07-03** (or merge claim-release into 07-03's wave) so MCP reroute to lifecycle does not expose in-flight amend with lingering claims.

4. **Plan 07-01:** Add optional `validateMaxLen` on supersede `reason` in `TransitionSupersede` for parity with amend (`lifecycle_handler.go:51-52` pattern) — or document intentional relaxation.

5. **Plan 07-04 Task 1:** Add a grep acceptance check that no non-test code calls `AuthoringServiceClient.Amend/Supersede` after 07-03 (should be only dead handler bodies).

6. **Plan 07-05:** Cross-check report tool args — `slug` + `agent` at `internal/mcp/tools_execution.go:343-344` (not `spec_slug` on report; claim uses `spec_slug` at `:55`).

---

## 5. Risk Assessment

**Overall risk: MEDIUM**

**Justification:** The consolidation strategy is well-grounded — the lifecycle implementation and CLI path already match LIFE-01/LIFE-02, and the research-backed deletion ordering for Plan 07-04 is compile-safe. Risk is elevated by (1) the **`re_entry_stage` validation gap** between D-03 and actual handler/storage behavior, which could let MCP agents use invalid re-entry values if 07-03 follows "pass through" literally; (2) **claim-release not yet in `LifecycleAmendSpec`**, which is security-adjacent if MCP reroute lands before 07-02; and (3) **underspecified e2e fixture sequencing** in Plan 07-05. None of these invalidate the phase architecture; they need explicit resolution in implementation tasks rather than discovery during execution.

| Plan   | Risk   | Notes |
|--------|--------|-------|
| 07-01  | LOW    | Straightforward proto + signature ripple; `ChangeLogEntry.Reason` exists |
| 07-02  | MEDIUM | Correct pattern; should precede or ship with MCP reroute |
| 07-03  | MEDIUM-HIGH | Core fix; blocked on re_entry validation decision |
| 07-04  | LOW-MEDIUM | Two-boundary deletion verified sound; large blast radius but enumerated |
| 07-05  | MEDIUM | Skills straightforward; e2e needs multi-spec clarity |

---

## Consensus Summary

Single reviewer (Cursor / composer-2.5), source-grounded against the live working tree. No cross-reviewer consensus to synthesize, but the highest-signal findings the planner should act on via `/gsd-plan-phase 7 --reviews`:

### Agreed Strengths
- Consolidation diagnosis is accurate and evidence-backed: the lifecycle path is already correct; the MCP `author` tool is the broken twin (verified at `tools_authoring.go:357-381` vs `cmd/specgraph/lifecycle.go:50-82`).
- Plan 07-04's two-boundary compile-safe deletion ordering is verified sound against the `AuthoringHandler` interface assertion (`authoring_handler.go:33`, no `Unimplemented` embed).
- Claim-release (07-02) and supersede `reason` (07-01) copy proven patterns and need no schema migration.

### Agreed Concerns (priority order)
1. **HIGH — `re_entry_stage` allowlist gap.** The handler/storage validation (`ExcludesReEntry()`) only rejects done/superseded/abandoned, so `approved`/`in_progress`/`review` would be accepted as re-entry targets — contradicting D-03. If Plan 07-03's "pass through, don't re-validate" is followed literally, an MCP agent can pass an invalid re-entry stage and land at an unexpected stage. Needs a single-layer fix (tool-side allowlist in `handleAmend`, or a shared `IsValidReEntryStage()` used by handler + storage) before/with 07-03.
2. **MEDIUM — Plan 07-05 e2e fixture sequencing is underspecified/contradictory** (single spec can't be both driven-to-done and amended-in-flight; no second slug named). Recommend an explicit multi-slug fixture layout.
3. **MEDIUM — ordering risk:** if 07-03 (MCP reroute) ships before 07-02 (claim-release), in-flight amend via MCP exposes the lingering-claim gap. Serialize 07-02 before 07-03 or ship together.
4. **LOW — supersede `reason` has no length/required validation** on the lifecycle path (parity gap vs the retired authoring path); the 07-03 threat register's "length bounds inherited" claim is unsupported for supersede.

### Divergent Views
None — single reviewer.
