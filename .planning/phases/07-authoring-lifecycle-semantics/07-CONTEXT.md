# Phase 7: Authoring Lifecycle Semantics - Context

**Gathered:** 2026-07-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Make `amend` and `supersede` match natural spec lifecycle semantics **on the MCP authoring surface**, and make amend re-entry land the spec so the target stage can be re-authored immediately.

**Requirements (fixed by ROADMAP.md — this discussion clarifies HOW, not WHAT):**
- **LIFE-01 (#900):** amend works while a spec is in flight (`>= approved && < done`, i.e. approved/in_progress/review) and returns it to authoring; supersede is permitted **only** on a `done` spec.
- **LIFE-02 (#899):** after `amend` with re-entry `<stage>`, the user (or MCP agent) can immediately run that stage without hitting an `invalid stage transition` no-op.

**Root cause (the key discovery this phase turns on):** there are **two parallel amend/supersede implementations with divergent semantics.**

| Path | Amend eligibility | Supersede eligibility | Re-entry landing | State |
|------|-------------------|-----------------------|------------------|-------|
| **CLI** → `LifecycleService.TransitionAmend/TransitionSupersede` → `LifecycleAmendSpec`/`LifecycleSupersedeSpec` | in-flight (approved/in_progress/review) ✓ | done-only ✓ | lands one-before-target ✓ | **Already correct** (fixed by PRs #889, #892) |
| **MCP `author` tool** → `AuthoringService.Amend/Supersede` → `AmendSpec`/`SupersedeSpec` | rejects `approved`, old semantics ✗ | any non-terminal ✗ | lands AT target (the #899 no-op) ✗ | **Still broken** |

The CLI, the `LifecycleAmendSpec` storage path, and `site/docs/concepts/lifecycle.md` already describe the correct behavior. The **MCP `author` tool** — the authoring path Phase 6 made canonical for MCP-only agents — still runs the old, inverted logic. Phase 7 brings the MCP surface into line by routing it to the already-correct lifecycle path and retiring the divergent implementation.

**In scope:** rerouting the MCP `author` amend/supersede actions to the LifecycleService; retiring the divergent `AuthoringService.Amend/Supersede` RPCs + `AmendSpec`/`SupersedeSpec` storage; MCP tool param reconciliation (`re_entry_stage`, `new_slug`, next-step hint); adding an optional `reason` to the supersede lifecycle path (+ CLI `--reason`); releasing the active claim when amending an in-flight spec; skills teaching amend/supersede; MCP-only e2e + storage integration verification.

**Out of scope (own phases):** conversation-recording enforcement (Phase 8), JIT display-name reconciliation (Phase 9). The `LifecycleService`/CLI amend/supersede semantics are already correct and are only touched where noted (add supersede `reason`, claim release).

</domain>

<decisions>
## Implementation Decisions

### Consolidation strategy (LIFE-01 core)
- **D-01:** **Route the MCP `author` tool's amend/supersede handlers to `LifecycleService`.** Change `handleAmend`/`handleSupersede` (`internal/mcp/tools_authoring.go`) to call `client.Lifecycle.TransitionAmend` / `client.Lifecycle.TransitionSupersede` instead of `client.Authoring.Amend/Supersede`. Both surfaces (CLI + MCP) then share the single already-correct lifecycle implementation; the inverted semantics cannot re-diverge.
- **D-02:** **Fully retire the now-unused broken path.** After rerouting there are zero callers of `AuthoringService.Amend/Supersede`. Mark those two RPCs (and `AmendRequest`/`AmendResponse`/`SupersedeRequest`/`SupersedeResponse` if unused elsewhere) as `reserved` in `proto/specgraph/v1/authoring.proto`, run `task proto`, and delete the `AuthoringHandler.Amend/Supersede` handlers, the `AmendSpec`/`SupersedeSpec` storage methods, and `storage.AmendResult`. The CLI is untouched — it already talks only to `LifecycleService`.
  - Follow proto-removal gotcha: `reserved` for both field/RPC number and name; update all callers; verify `go build ./...`.

### MCP amend param semantics (LIFE-02 core)
- **D-03:** **Rename the tool param `target_stage` → `re_entry_stage`** to match `TransitionAmendRequest`. It is **required**, valid values `spark|shape|specify|decompose` (reject `approved`/`done`/terminal). Semantics = "the stage you want to redo"; storage lands the spec **one stage before** (`PrecedingAuthStage`) so the authoring command for that stage is a valid transition (fixes the #899 no-op). Rewrite the tool `Description` + `re_entry_stage` param doc to teach the land-one-before model so an MCP agent cannot reproduce the no-op.
- **D-04:** **`reason` is required on the MCP amend action** (TransitionAmend mandates it).
- **D-05:** **The amend tool result echoes the next stage command to run** (e.g. "Amended; spec landed at spark, re-entering shape — call `author action=shape` next"), mirroring the funnel stages' `NextPrompts` pattern. The MCP handler can compose this from the requested `re_entry_stage` — no proto change required (`TransitionAmendResponse` already returns the `Spec`).

### Supersede signature (LIFE-01)
- **D-06:** **Add an optional `reason` field to `TransitionSupersedeRequest`** and thread it through `LifecycleSupersedeSpec` into the supersede changelog entry, so both CLI and MCP can record why a done spec was replaced (preserves the audit note the old MCP path had). Requires a proto change + `task proto`.
- **D-07:** **Rename the MCP supersede param `superseded_by` → `new_slug`** (matches `TransitionSupersedeRequest`, consistent with the `re_entry_stage` rename) and **add an optional `--reason` flag to the CLI `supersede` command**. Full CLI/MCP parity. Done-only precondition + valid non-terminal replacement are already enforced by `LifecycleSupersedeSpec` (`ErrSpecNotDone`, `ErrNewSpecNotFound`, `ErrNewSpecTerminal`).

### Execution-state on amend
- **D-08:** **Amending an in_progress/review spec releases its active claim** (delete the claim row + `CLAIMED_BY` edge) inside the same amend transaction. The spec is no longer executable once it returns to authoring, so the lease must not linger (closes the stale-claim gap #900 flagged). **Slices are left intact** — they are the decompose output being re-authored, not discarded. Research/planning must confirm the exact claim + `CLAIMED_BY` edge mechanics against the existing claim code.

### Skills & verification
- **D-09:** **Add amend/supersede/re-entry teaching to the MCP-first skills** (`specgraph-authoring` and/or `specgraph-troubleshooting`) so an MCP-only agent can send an in-flight spec back and re-author it — completing the Phase 6 self-teaching story. Include the land-one-before re-entry model. (Skills currently teach nothing about amend/supersede.)
- **D-10:** **Verification gate = MCP-only e2e + storage integration.** An MCP-only Ginkgo e2e (`e2e/api/`, Phase 6 style) drives a spec to done, amends an in-flight spec back, re-authors the landed stage, and supersedes a done spec — all through the `author` MCP tool — asserting correct stage landings and correct rejections (amend on done, supersede on non-done). Plus storage integration tests for the claim-release-on-amend, amend-eligible stages, done-only supersede, and re-entry landing.

### the agent's Discretion
- Exact proto `reserved` field/RPC numbering for the retired `AuthoringService.Amend/Supersede` messages — research/planning to enumerate against the current `.proto`.
- Precise wording of the amend next-step hint (D-05) and the rewritten tool descriptions.
- Test layering details and how the MCP-only e2e simulates a done→amend→re-author sequence.
- Which skill file(s) carry the amend/supersede teaching (D-09) vs. tool-description docs.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Driving requirements
- GitHub issue **#900** (`https://github.com/specgraph/specgraph/issues/900`) — lifecycle nomenclature inversion (amend from in-flight, supersede from done); raises the execution-state (claim/slices) question.
- GitHub issue **#899** (`https://github.com/specgraph/specgraph/issues/899`) — amend re-entry lands AT target, blocking re-authoring; documents the land-one-before fix (Option A).
- `.planning/ROADMAP.md` § "Phase 7: Authoring Lifecycle Semantics" — locked goal + 4 success criteria.
- `.planning/REQUIREMENTS.md` § Authoring Lifecycle (LIFE-01, LIFE-02).

### The correct (target) lifecycle path — reuse this, do not reinvent
- `internal/storage/postgres/lifecycle.go` — `LifecycleAmendSpec` (in-flight eligibility, land-one-before via `PrecedingAuthStage`), `LifecycleSupersedeSpec` (done-only, valid replacement). The supersede `reason` (D-06) is added here.
- `internal/server/lifecycle_handler.go` — `TransitionAmend`/`TransitionSupersede` handlers + `lifecycleError` (already maps `ErrSpecNotAmendable`, `ErrSpecNotDone`, `ErrReEntryStageRequired`, etc.).
- `internal/storage/spec_domain.go` — `SpecStage`, `IsAmendEligible`, `ExcludesReEntry`, `PrecedingAuthStage`, `IsFullyTerminal`.
- `internal/storage/stage_validation.go` — `ValidateTransition` (the `from==to` no-op rule at the heart of #899), `authoringStages`, `stageIndex`.

### The broken (to retire) path
- `internal/storage/postgres/authoring.go` — `AmendSpec` (rejects `approved`, lands AT target), `SupersedeSpec` (no done requirement). Delete both. `TransitionStage` in the same file stays (funnel + approve use it).
- `internal/server/authoring_handler.go` — `AuthoringHandler.Amend` (551), `AuthoringHandler.Supersede` (593). Delete both.
- `proto/specgraph/v1/authoring.proto` — `rpc Amend` (441), `rpc Supersede` (443), `AmendRequest`/`AmendResponse`/`SupersedeRequest`/`SupersedeResponse` (~375–430). Mark `reserved` on retirement.
- `internal/storage/authoring.go` — `AuthoringBackend` interface (remove `AmendSpec`/`SupersedeSpec`), `storage.AmendResult`.

### MCP tool surface (the surface being fixed)
- `internal/mcp/tools_authoring.go` — `authorTool.def()` (schema/param docs: rename `target_stage`→`re_entry_stage`, `superseded_by`→`new_slug`), `handleAmend` (357, reroute to Lifecycle), `handleSupersede` (368, reroute to Lifecycle), `authoringStageFromString` (may become unused for amend). Note: the MCP `Client` must expose a `Lifecycle` client (confirm `t.client.Lifecycle` exists / add it).
- `cmd/specgraph/lifecycle.go` — CLI `runAmend`/`runSupersede`; add `--reason` flag to `supersedeCmd` (D-07). Reference for how the CLI wires `TransitionAmend`/`TransitionSupersede`.

### Proto + supersede reason
- `proto/specgraph/v1/lifecycle.proto` — `TransitionAmendRequest` (has `re_entry_stage`), `TransitionSupersedeRequest` (add optional `reason`, D-06).
- Regenerate with `task proto` (gen/ is committed).

### Execution state (claim release, D-08)
- `internal/storage/postgres/execution.go` — `GetActiveClaim`, `ReleaseExpiredClaims`, claim/`CLAIMED_BY` mechanics (pattern for releasing a claim inside the amend tx).
- `internal/storage/postgres/slice.go` — `ClaimSlice` and slice storage (slices left intact).
- `internal/storage/postgres/migrations/001_initial_schema.sql` — `claims` and `slices` table schemas.

### Skills (D-09)
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md`, `internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md` — MCP-first skills (Phase 6) to extend with amend/supersede/re-entry teaching. `TestContentProtoDrift`-style checks gate backticked token drift.

### Docs (light pass)
- `site/docs/concepts/lifecycle.md` — already describes correct semantics; verify still accurate after supersede `reason`.
- `site/docs/concepts/authoring.md` — funnel state diagram (#900 flagged `done-->amended`); update if it still shows the inverted flow.

### Verification (D-10)
- `e2e/api/` — Ginkgo/Gomega e2e suite (`go test -tags e2e`); home for the MCP-only amend/supersede/re-entry e2e.
- `internal/storage/postgres/authoring_test.go`, `internal/storage/postgres/lifecycle_test.go`, `internal/server/lifecycle_handler_test.go` — existing amend/supersede test patterns to extend / prune (retire `AmendSpec`/`SupersedeSpec` tests).

### Repo conventions (AGENTS.md)
- Proto removal uses `reserved`; `gen/` is committed (`task proto`). All multi-query writes use `RunInTransaction` (ADR-004). Concurrency conflicts → `ErrConcurrentModification` → `CodeAborted`. Handler errors sanitized; test assertions use error codes, not message strings. Mock backends return sentinel errors. Apache-2.0 SPDX headers + DCO sign-off required. Package doc comments required (revive).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`LifecycleAmendSpec` / `LifecycleSupersedeSpec`** (`postgres/lifecycle.go`) — the correct, tested implementations. D-01 makes the MCP tool reuse them rather than the broken `AmendSpec`/`SupersedeSpec`.
- **`SpecStage.PrecedingAuthStage` / `IsAmendEligible` / `ExcludesReEntry`** (`spec_domain.go`) — the land-one-before and eligibility logic already exists; no new stage math needed.
- **`lifecycleError`** (`lifecycle_handler.go`) — already maps every relevant sentinel (`ErrSpecNotAmendable`, `ErrSpecNotDone`, `ErrReEntryStageRequired`, `ErrInvalidReEntryStage`, `ErrConcurrentModification`) to the right connect code. The rerouted MCP tool inherits these.
- **`NextPrompts`/`promptsToProto` pattern** (`authoring_handler.go`) — precedent for D-05's amend next-step hint.
- **Claim-release precedent** (`execution.go` `ReleaseExpiredClaims`) — pattern for deleting a claim row + `CLAIMED_BY` edge inside a transaction (D-08).

### Established Patterns
- MCP `author` tool dispatches on an `action` param; handlers use `stringParam`/`errResult`/`connectErrResult`/`jsonResult`. Rerouting swaps the client call, keeps the house style.
- Storage lifecycle mutations use `RunInTransaction` + version-guarded `UPDATE ... WHERE version = $n` + `preconditionError` to distinguish concurrent-mod from precondition failures.
- Tool `Description` + param docs are agent-facing teaching surface — rewritten alongside the param renames (D-03).

### Integration Points
- MCP `Client` must expose a `Lifecycle` service client for `handleAmend`/`handleSupersede` to call (confirm it exists in `internal/mcp/*client*`; add if missing).
- The supersede `reason` (D-06) flows: MCP/CLI → `TransitionSupersedeRequest.reason` → handler → `LifecycleSupersedeSpec` → changelog entry. No storage-schema change (changelog already carries `Reason`).
- Claim release (D-08) joins the existing amend `RunInTransaction` in `LifecycleAmendSpec` — same tx as the stage update.

</code_context>

<specifics>
## Specific Ideas

- The user's mental model for re-entry matches issue #899 Option A and `lifecycle.md`: **"`re_entry_stage: shape` means I want to redo shape"** — the spec lands at `spark` so `shape` (spark→shape) succeeds. Teach it exactly that way in the tool doc and skills.
- Milestone throughline (from Phase 6): make the MCP surface **impossible for an MCP-only agent to get wrong**. The amend next-step hint (D-05) and the required, constrained `re_entry_stage` param (D-03) serve that goal.
- Preference for a **single source of truth**: the user explicitly wanted the divergent broken path retired, not just rerouted-around (D-02).

</specifics>

<deferred>
## Deferred Ideas

- **Reset/clear decompose slices on amend** — considered for D-08 but rejected for this phase; slices are the re-authored decompose output and shouldn't be discarded. If a future execution-integration phase needs a hard reset semantics, revisit there.
- Broader claim/lease reconciliation between SpecGraph amend and Gastown execution (beyond releasing the local claim) is an execution-integration concern, not this phase.

None outside these — discussion stayed within phase scope.

</deferred>

---

*Phase: 7-Authoring Lifecycle Semantics*
*Context gathered: 2026-07-14*
