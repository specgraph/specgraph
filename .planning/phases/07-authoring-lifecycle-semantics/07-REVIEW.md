---
phase: 07-authoring-lifecycle-semantics
reviewed: 2026-07-15T00:00:00Z
depth: standard
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
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-07-15
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Phase 7 corrected amend/supersede lifecycle semantics, added claim release on amend,
rerouted the MCP `author` tool to `LifecycleService`, and deleted the divergent
`AuthoringService.Amend/Supersede` path. The core state-machine and storage code is
solid on the dimensions the phase targeted:

- **SQL is fully parameterized.** No injection surface in `postgres/lifecycle.go`. The
  one dynamic column name in `postgres/authoring.go` (`storeJSONColumn`) is
  allowlist-gated *and* char-validated.
- **Transactions are correct (ADR-004).** Every inner query in `LifecycleAmendSpec`,
  `LifecycleSupersedeSpec`, `LifecycleAbandonSpec`, and `LifecycleAcknowledgeDrift`
  threads `txCtx`, not `ctx`. Version guards + `ErrConcurrentModification` are present
  on all mutating UPDATEs.
- **Claim-release atomicity is correct.** The amend claim + `CLAIMED_BY` edge deletion
  (lifecycle.go:88–105) runs inside the same transaction and matches the canonical
  `UnclaimSpec` pattern (claim.go:161–169) byte-for-byte.
- **Error sanitization holds.** `lifecycleError`/`stageError` map sentinels to connect
  codes and fall through to `CodeInternal` with a generic message; the sanitization
  tests assert on codes, not strings, and cover every Lifecycle RPC.
- **Proto hygiene is correct.** Removed authoring response fields use `reserved` for
  both number and name; no field-number collisions in `lifecycle.proto`; the deleted
  Amend/Supersede RPCs/messages are cleanly gone.
- **Allowlist logic is right.** `IsValidReEntryStage` is an explicit 4-value switch
  (deliberately *not* a range over `authoringStages`, which would readmit `approved`
  and reintroduce the review bug). `PrecedingAuthStage` land-one-before is correct for
  shape/specify/decompose.

No blockers found. The findings below are concentrated on the `re_entry_stage=spark`
degenerate case (where the tool actively emits a hint that fails), a missing
required-field check, and a few latent/robustness gaps.

## Warnings

### WR-01: MCP `author` amend hint tells the agent to run a command that errors for `re_entry_stage=spark`

**File:** `internal/mcp/tools_authoring.go:375-381`
**Issue:** `handleAmend` unconditionally appends:
`"Next step: run author action=<re_entry_stage> …"`. For `re_entry_stage=spark`, the
spec lands at `spark` (PrecedingAuthStage(spark)==spark), and the suggested follow-up
`author action=spark` routes to `AuthoringService.Spark` → `CreateSpec` →
`storage.ErrSpecAlreadyExists` → `CodeAlreadyExists` (authoring_handler.go:79,106). So
the tool instructs the agent to run a call that is guaranteed to fail. This directly
contradicts the `specgraph-authoring` skill's own caveat that spark re-entry must never
be presented as the happy path. The amend itself succeeds; only the emitted next-step
guidance is wrong.
**Fix:** Suppress or rewrite the hint when the landing stage equals the target
(i.e. `re_entry_stage == "spark"`). For example:
```go
if reEntryStage == "spark" {
    res.Content = append(res.Content, Content{Type: "text", Text:
        fmt.Sprintf("Spec %q is now at spark. There is no forward re-author command for spark; edit the seed via the normal flow.", slug)})
    return res, nil
}
hint := fmt.Sprintf("Next step: run author action=%s for spec %q to re-author the %s stage.",
    reEntryStage, slug, reEntryStage)
res.Content = append(res.Content, Content{Type: "text", Text: hint})
```

### WR-02: `AcknowledgeDrift` handler does not enforce the proto-required `note`

**File:** `internal/server/lifecycle_handler.go:192-194`
**Issue:** `lifecycle.proto` documents `note` as **Required** ("Acknowledgment note
explaining why drift is accepted"), and the CLI marks `--note` required
(cmd/specgraph/lifecycle.go:330). The RPC handler only checks the *maximum* length —
it never rejects an empty note. A direct RPC caller (any non-CLI client) can persist a
drift acknowledgment with no rationale, silently weakening the audit trail that the
whole ack-changelog mechanism exists to provide. The store writes `Reason: note` with
an empty string (postgres/lifecycle.go:407).
**Fix:** Add a required-field check alongside the length check, mirroring the
`reason`/`slug` validation used elsewhere:
```go
if err := validateRequiredField("note", msg.Note); err != nil {
    return nil, connect.NewError(connect.CodeInvalidArgument, err)
}
if len(msg.Note) > maxFieldLen {
    return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("note exceeds maximum of %d characters", maxFieldLen))
}
```

## Info

### IN-01: Skill doc mislabels the spark re-entry failure mode as a "no-op"

**File:** `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md:224-227`
**Issue:** The caveat states that re-running `author action=spark` on a spec already at
spark "is a same-stage no-op. It is API-allowed but degenerate." This is factually
wrong: `Spark` always calls `CreateSpec`, which returns `ErrSpecAlreadyExists`
(`CodeAlreadyExists`) — an error, not a no-op. Combined with WR-01, an agent that trusts
either the tool hint or this doc will hit a hard error.
**Fix:** Reword to: "re-running `author action=spark` on an existing spec returns
ALREADY_EXISTS — there is no spark re-author path; never present it as a next step."

### IN-02: Concurrent supersede race mislabels *which* spec is terminal

**File:** `internal/storage/postgres/lifecycle.go:194-201`, `421-439`; `internal/server/lifecycle_handler.go:289-291`
**Issue:** In the non-concurrent path, a terminal replacement is correctly reported via
`ErrNewSpecTerminal` (lifecycle.go:157). But if the new spec transitions to
superseded/abandoned *between* the pre-read and the guarded UPDATE, `RowsAffected()==0`
falls into `preconditionError`, whose first check returns the generic
`ErrSpecTerminal`. `lifecycleError` then formats that against `msg.Slug` (the **old**
slug), so the client sees `spec "old-slug" is in a terminal state` when the actually-
terminal spec is the new one. Rare (concurrent-only) and not a correctness/data issue,
but a misleading diagnostic.
**Fix:** In the new-spec `preconditionError` closure, detect terminal state on `current`
and return `storage.ErrNewSpecTerminal` before the generic terminal check fires, or pass
a `newSlug`-aware message.

### IN-03: Amend does not clear stale downstream stage outputs

**File:** `internal/storage/postgres/lifecycle.go:38-133`
**Issue:** `LifecycleAmendSpec` sets `stage = landingStage` and recomputes the content
hash, but leaves `shape_output` / `specify_output` / `decompose_output` untouched. The
inline comment only justifies leaving *slices* intact. After e.g. `re_entry_stage=shape`
(landing `spark`), if the agent stops after re-authoring shape, the spec sits at `shape`
while still carrying stale `specify_output`/`decompose_output` JSON that contribute to
the recomputed content hash and to any downstream reader. This is partly intentional
(slices are deliberately preserved), but the non-slice outputs are undocumented and can
present a spec whose stored contract no longer matches its stage.
**Fix (or document):** Either null the stage outputs at or beyond `landingStage` during
amend, or extend the D-08 comment to state explicitly that all pre-existing stage
outputs are intentionally retained and will be overwritten only when their stage is
re-authored.

### IN-04: `ValidateTransition` still permits arbitrary backward transitions, bypassing the new amend guards

**File:** `internal/storage/stage_validation.go:67-70`
**Issue:** After Phase 7, backward movement through the funnel is supposed to happen
*only* via `LifecycleAmendSpec`, which enforces the re-entry allowlist, amend-eligibility,
and claim release. Yet `ValidateTransition` still returns `nil` for any `toIdx < fromIdx`
within the authoring range, so `TransitionStage(review, spark)` (or similar) would be
accepted and would skip claim release and re-entry validation entirely. No in-scope
caller does this today (the authoring handlers only transition forward), so this is
latent rather than an active bug — but it is a now-inconsistent escape hatch around the
very semantics this phase introduced.
**Fix:** Consider restricting `TransitionStage`/`ValidateTransition` to forward-only
transitions and routing all backward movement through the amend path, or add a comment
documenting why the permissive backward branch must remain.

---

_Reviewed: 2026-07-15_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
