---
phase: 07-authoring-lifecycle-semantics
verified: 2026-07-14T22:26:19Z
status: passed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  # No prior VERIFICATION.md — initial verification
gaps: []
---

# Phase 7: Authoring Lifecycle Semantics Verification Report

**Phase Goal:** amend and supersede match natural spec lifecycle semantics, and amend re-entry lets the target stage be re-authored immediately.
**Verified:** 2026-07-14T22:26:19Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1 | A user can `amend` a spec while in flight (`>= approved` and `< done`) and it returns to authoring (SC1, LIFE-01) | ✓ VERIFIED | `LifecycleAmendSpec` (postgres/lifecycle.go:62-66) UPDATE guard `stage IN ('approved','in_progress','review')`; lands at `PrecedingAuthStage(target)` (authoring stage). Integration `TestLifecycle/AmendSpec_HappyPath` + `/AmendSpec_AllEligibleStages` **PASS** (run first-hand under Docker). |
| 2 | `supersede` is permitted only on a `done` spec and rejected for in-flight specs (SC2, LIFE-01) | ✓ VERIFIED | `LifecycleSupersedeSpec` (lifecycle.go:147-149) `if oldCheck.Stage != done → ErrSpecNotDone`, plus SQL version guard `AND stage = 'done'`. Integration `TestLifecycle/SupersedeSpec_NotDone` asserts `ErrSpecNotDone` **PASS**; e2e "rejects supersede on a non-done in-flight spec" **PASS**. |
| 3 | After `amend --re-entry <stage>`, the user can immediately re-author that stage without an `invalid stage transition` no-op (SC3, LIFE-02) | ✓ VERIFIED | Land-one-before: `landingStage := targetStage.PrecedingAuthStage()` (lifecycle.go:52). e2e "amends an in-flight spec back and re-authors the landed stage" amends `re_entry_stage=shape`, asserts landing at `spark` + hint `action=shape`, then **re-runs `author action=shape` successfully** — the exact #899 no-op path — **PASS** (run first-hand). |
| 4 | Re-entry lands the spec one stage before the target so the subsequent stage command is a valid transition (SC4, LIFE-02) | ✓ VERIFIED | `PrecedingAuthStage(shape)==spark` via ordered `authoringStages` (stage_validation.go:10-16). Integration `TestLifecycleAmend_ReleasesClaim` asserts `amended.Stage == SpecStage("shape").PrecedingAuthStage()` and `== SpecStage("specify").PrecedingAuthStage()` **PASS**. |
| 5 | The `IsValidReEntryStage` allowlist (spark\|shape\|specify\|decompose) is enforced in the `TransitionAmend` handler AND storage, rejecting approved/in_progress/review/done with `CodeInvalidArgument` — for both CLI and MCP (D-03) | ✓ VERIFIED | Explicit four-value switch (spec_domain.go:53-60, NOT reusing `authoringStages` which includes `approved`); handler single-path check (lifecycle_handler.go:57-59); storage guard `!targetStage.IsValidReEntryStage()` (lifecycle.go:43-45). Integration `TestLifecycle/AmendSpec_InvalidReEntryStage` **PASS**. |
| 6 | Amending an in-flight spec releases its active claim + CLAIMED_BY edge in the same transaction; unclaimed amend is a harmless no-op; slices left intact (D-08, #900) | ✓ VERIFIED | Conditional `GetActiveClaim(txCtx)` guard + two scoped DELETEs inside the amend tx (lifecycle.go:88-105); no `slices` touch. Integration `TestLifecycleAmend_ReleasesClaim/ClaimedSpec_ReleasesClaimAndEdge` + `/UnclaimedSpec_NoErrorNoClaim` **PASS**. |
| 7 | The MCP `author` amend/supersede actions route to `LifecycleService`; the divergent broken authoring path is fully retired (D-01/D-02) | ✓ VERIFIED | `handleAmend`→`client.Lifecycle.TransitionAmend`, `handleSupersede`→`client.Lifecycle.TransitionSupersede` (tools_authoring.go:363,393) with `re_entry_stage`/`new_slug` params + next-step hint. Absence greps: no `rpc Amend/Supersede`, no `AmendRequest/SupersedeRequest` in gen, no `AuthoringHandler.Amend/Supersede`, no `Store.AmendSpec/SupersedeSpec`, no `AmendResult/AuthoringSpecLifecycle/ValidateAmendTransition`; `TransitionStage` preserved. |

**Score:** 7/7 truths verified (0 present-behavior-unverified). All four behavior-dependent state-transition/invariant truths (SC1–SC4) carry passing behavioral tests run first-hand under Docker.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/storage/spec_domain.go` | `IsValidReEntryStage` allowlist + `PrecedingAuthStage` | ✓ VERIFIED | Explicit switch; documented as distinct from terminal-only `ExcludesReEntry`. |
| `internal/storage/postgres/lifecycle.go` | amend claim-release, land-one-before, supersede done-only + reason | ✓ VERIFIED | All present and wired; guarded via `IsValidReEntryStage`. |
| `internal/server/lifecycle_handler.go` | `TransitionAmend` single-path re-entry validation; `TransitionSupersede` threads reason | ✓ VERIFIED | Rejects approved/in_progress/review/done with `CodeInvalidArgument`. |
| `internal/mcp/tools_authoring.go` | rerouted handlers, renamed params, next-step hint, `authoringStageFromString` deleted | ✓ VERIFIED | `re_entry_stage`/`new_slug` present; no `target_stage`/`superseded_by`; helper removed. |
| `cmd/specgraph/lifecycle.go` | `amend --re-entry` (required) + `--reason` (required); `supersede --with` (required) + `--reason` (optional) | ✓ VERIFIED | Flags registered at lines 312-320. |
| `proto/specgraph/v1/authoring.proto` | Amend/Supersede RPCs + 4 messages removed | ✓ VERIFIED | Absence greps clean; gen regenerated. |
| `e2e/api/mcp_only_lifecycle_test.go` | MCP-only done→amend→re-author→supersede + 2 rejection cases | ✓ VERIFIED | 4 It blocks, distinct per-scenario slugs; suite **PASS** under Docker. |
| `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` | amend/supersede/re-entry land-one-before teaching | ✓ VERIFIED | Precondition table + land-one-before + spark no-op caveat present. |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| CLI `amend --re-entry` / MCP `author action=amend` | `TransitionAmend` handler | `ReEntryStage` field + `IsValidReEntryStage` gate | ✓ WIRED |
| `TransitionAmend` handler | `LifecycleAmendSpec` storage | `store.LifecycleAmendSpec(ctx, slug, reason, reEntryStage)` | ✓ WIRED |
| `LifecycleAmendSpec` | `PrecedingAuthStage` landing + claim release | in-tx UPDATE + conditional DELETEs | ✓ WIRED |
| `TransitionSupersede` handler | `LifecycleSupersedeSpec` (done-only + reason) | `store.LifecycleSupersedeSpec(...reason)` | ✓ WIRED |
| MCP `author` amend/supersede | `client.Lifecycle.Transition{Amend,Supersede}` | single-source-of-truth reroute | ✓ WIRED |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Amend land-one-before + claim release | `go test -tags integration -run TestLifecycleAmend_ReleasesClaim ./internal/storage/postgres/` | PASS (ClaimedSpec + Unclaimed subtests) | ✓ PASS |
| Amend in-flight eligibility, done-only supersede, allowlist, reason threading | `go test -tags integration -run 'TestLifecycle$' ./internal/storage/postgres/` | PASS (AmendSpec_*, SupersedeSpec_NotDone, _InvalidReEntryStage, _ReasonThreaded) | ✓ PASS |
| Full MCP-only done→amend→re-author→supersede + 2 rejections | `go test -tags e2e -run TestAPI ./e2e/api/ -args -ginkgo.focus 'MCP-only lifecycle'` | ok (7.3s, exit 0) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| ----------- | ------------ | ----------- | ------ | -------- |
| LIFE-01 (#900) | 07-01, 07-02, 07-03, 07-04, 07-05 | amend in-flight → authoring; supersede done-only | ✓ SATISFIED | Truths 1, 2, 6, 7 verified; integration + e2e PASS |
| LIFE-02 (#899) | 07-02, 07-03, 07-05 | amend `--re-entry` → immediate re-author, no no-op | ✓ SATISFIED | Truths 3, 4, 5 verified; e2e re-authors landed stage PASS |

Both requirement IDs declared across plans are mapped to Phase 7 in REQUIREMENTS.md (lines 18-19, 50-51). No orphaned requirements.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX` debt markers, no `TODO`/`HACK`/`PLACEHOLDER`, no stub returns, and no stray references to the retired `Authoring.Amend/Supersede` path or the deleted `authoringStageFromString` in production code.

### Human Verification Required

None. All four behavior-dependent success criteria (state-transition and land-one-before invariants) are exercised by passing behavioral tests that were run first-hand under Docker during this verification.

### Gaps Summary

No gaps. All 7 must-haves verified, all 8 artifacts present/substantive/wired, all key links connected, both requirements satisfied, no anti-patterns. The four ROADMAP success criteria are directly proven by first-hand integration + e2e runs:
- SC1 (amend in-flight → authoring): storage guard + `AmendSpec_AllEligibleStages` PASS.
- SC2 (supersede done-only): `SupersedeSpec_NotDone` → `ErrSpecNotDone` PASS + e2e rejection.
- SC3 (immediate re-author, no #899 no-op): e2e re-runs `author action=shape` after amend, PASS.
- SC4 (land-one-before valid transition): `PrecedingAuthStage` landing asserted in `TestLifecycleAmend_ReleasesClaim` PASS.

The divergent broken authoring path is fully retired, leaving `LifecycleService` as the single source of truth so the inverted semantics cannot re-diverge.

---

_Verified: 2026-07-14T22:26:19Z_
_Verifier: the agent (gsd-verifier)_
