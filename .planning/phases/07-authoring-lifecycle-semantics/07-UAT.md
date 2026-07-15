---
status: complete
phase: 07-authoring-lifecycle-semantics
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md, 07-03-SUMMARY.md, 07-04-SUMMARY.md, 07-05-SUMMARY.md]
started: 2026-07-15T13:20:00Z
updated: 2026-07-15T13:42:23.589Z
---

## Current Test

[testing complete]

## Tests

### 1. Supersede reason proto field
expected: TransitionSupersedeRequest carries an optional `reason` field; generated code compiles.
result: pass
source: automated
coverage_id: D1-07-01

### 2. Handler threads supersede reason
expected: Handler passes request Reason into LifecycleSupersedeSpec; store records it on the superseded spec changelog with default fallback.
result: pass
source: automated
coverage_id: D2-07-01

### 3. CLI supersede --reason flag
expected: `specgraph supersede --reason <text>` wires the reason into the request (visible in `supersede --help`).
result: pass
source: automated
coverage_id: D4-07-01

### 4. Supersede changelog reason precedence (caller reason vs default note)
expected: Supplied reason recorded verbatim on the superseded spec's changelog; empty reason falls back to "Superseded by <new-slug>".
result: pass
coverage_id: D3-07-01

### 5. Amend releases active claim + CLAIMED_BY edge
expected: Amending an in_progress/review spec deletes its active claim row and CLAIMED_BY edge inside the amend transaction (D-08 stale-lease fix, #900).
result: pass
source: automated
coverage_id: D1-07-02

### 6. Amend unclaimed spec is a harmless no-op
expected: Amending an unclaimed (approved) spec produces no error and no residual claim/edge.
result: pass
source: automated
coverage_id: D2-07-02

### 7. Amend lands one stage before re-entry target
expected: Amend lands the spec at PrecedingAuthStage(target) so the next stage command is a valid transition.
result: pass
source: automated
coverage_id: D3-07-02

### 8. Supersede changelog reason (supplied vs default)
expected: Supersede changelog Reason reflects the supplied reason when non-empty, and the default "Superseded by <new>" note when empty.
result: pass
source: automated
coverage_id: D4-07-02

### 9. Supersede on non-done returns ErrSpecNotDone
expected: Supersede on a non-done spec returns storage.ErrSpecNotDone (→ rejected).
result: pass
source: automated
coverage_id: D5-07-02

### 10. IsValidReEntryStage allowlist
expected: IsValidReEntryStage() is true only for spark|shape|specify|decompose; approved/in_progress/review/done/terminal/unknown are false.
result: pass
source: automated
coverage_id: D1-07-03

### 11. TransitionAmend rejects non-authoring re-entry
expected: TransitionAmend rejects re_entry_stage in {approved, in_progress, review, done} with CodeInvalidArgument (asserted by code) for both CLI and MCP.
result: pass
source: automated
coverage_id: D2-07-03

### 12. Storage guard rejects invalid re-entry (defense-in-depth)
expected: LifecycleAmendSpec storage guard rejects approved/in_progress/review (+ done/superseded/abandoned) re-entry via ErrInvalidReEntryStage.
result: pass
source: automated
coverage_id: D3-07-03

### 13. MCP author amend routes to LifecycleService with hint
expected: MCP `author` amend routes to Lifecycle.TransitionAmend with slug/reason/re_entry_stage, requires reason, and emits a next-step hint referencing re_entry_stage.
result: pass
source: automated
coverage_id: D4-07-03

### 14. MCP author supersede routes to LifecycleService
expected: MCP `author` supersede routes to Lifecycle.TransitionSupersede with slug/new_slug/reason; new_slug required, reason optional.
result: pass
source: automated
coverage_id: D5-07-03

### 15. Tool surfaces sanitized handler error
expected: Tool surfaces the sanitized handler connect error (res.IsError) on sentinel failures rather than a tool-side re-implementation.
result: pass
source: automated
coverage_id: D6-07-03

### 16. Broken AuthoringService amend/supersede path retired
expected: AuthoringService exposes no Amend/Supersede RPCs, messages, handlers, Store methods, AmendResult, or ValidateAmendTransition — single source of truth.
result: pass
source: automated
coverage_id: D1-07-04

### 17. TransitionStage preserved, Lifecycle path untouched
expected: TransitionStage preserved and funnel + approve RPCs unaffected; the LifecycleService amend/supersede path is untouched.
result: pass
source: automated
coverage_id: D2-07-04

### 18. Skills teach amend/supersede/re-entry
expected: specgraph-authoring + specgraph-troubleshooting SKILL.md teach amend/supersede/re-entry with the land-one-before model; skills validate; no stale tokens.
result: pass
source: automated
coverage_id: D1-07-05

### 19. MCP-only e2e: amend + re-author (LIFE-02)
expected: An in-flight spec is amended with re_entry_stage=shape, lands at spark, the hint names shape, and re-authoring shape succeeds (no #899 no-op).
result: pass
source: automated
coverage_id: D2-07-05

### 20. MCP-only e2e: supersede done spec (LIFE-01)
expected: A spec driven to done via author+claim+report is superseded with a non-terminal replacement.
result: pass
source: automated
coverage_id: D3-07-05

### 21. MCP-only e2e: amend-on-done + supersede-on-non-done rejected (LIFE-01)
expected: amend-on-done and supersede-on-non-done are both rejected (res.IsError true).
result: pass
source: automated
coverage_id: D4-07-05

## Summary

total: 21
passed: 21
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
