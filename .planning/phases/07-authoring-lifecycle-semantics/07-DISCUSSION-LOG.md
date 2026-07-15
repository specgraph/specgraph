# Phase 7: Authoring Lifecycle Semantics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-14
**Phase:** 7-Authoring Lifecycle Semantics
**Areas discussed:** Consolidation strategy, MCP amend param semantics, Supersede signature, Execution-state on amend, Skills & verification

---

## Consolidation strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Route MCP tool to LifecycleService | MCP `author` amend/supersede call `TransitionAmend/TransitionSupersede`; single source of truth | ✓ |
| Fix AmendSpec/SupersedeSpec in place | Keep both RPC surfaces, rewrite storage to match lifecycle semantics | |
| Delegate storage methods to Lifecycle | Keep AuthoringService RPCs as thin wrappers over lifecycle storage | |

**User's choice:** Route MCP tool to LifecycleService.
**Notes:** User questioned why the RPCs would be removed ("doesn't the CLI need them?"). Clarified that the CLI uses `LifecycleService.TransitionAmend/TransitionSupersede` exclusively; the `AuthoringService.Amend/Supersede` RPCs are consumed only by the MCP `author` tool being rerouted. Removing them therefore doesn't touch the CLI.

### Retirement of now-unused RPCs (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Fully retire them | `reserved` in proto, delete handlers + AmendSpec/SupersedeSpec + AmendResult | ✓ |
| Leave them dead-but-present | Reroute but leave divergent code unused | |
| Keep as delegating wrappers | Preserve proto surface, delegate to lifecycle internally | |

**User's choice:** Fully retire them.
**Notes:** Wanted a single source of truth so the inverted semantics cannot silently return.

---

## MCP amend param semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Rename to re_entry_stage, required | Match TransitionAmendRequest; spark–decompose only; land-one-before | ✓ |
| Keep target_stage name, new semantics | Same behavior, less rename churn | |
| You decide the naming | Delegate exact naming to planning | |

**User's choice:** Rename to re_entry_stage, required.
**Notes:** `reason` also becomes required (TransitionAmend mandates it).

### Amend next-step guidance (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Echo the next stage command | Tool result tells the agent exactly what to run next | ✓ |
| Minimal response, rely on skills | Return landed stage + version only | |
| You decide | Delegate to planning | |

**User's choice:** Echo the next stage command.
**Notes:** Handler composes the hint from the requested re_entry_stage — no proto change needed.

---

## Supersede signature

| Option | Description | Selected |
|--------|-------------|----------|
| Match TransitionSupersede as-is (drop reason) | Smallest change; loses changelog reason | |
| Add reason to the lifecycle path | Optional reason on TransitionSupersedeRequest + changelog | ✓ |
| You decide on reason | Delegate to planning | |

**User's choice:** Add reason to the lifecycle path.

### Param naming + CLI reason (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Rename param + add CLI --reason | `superseded_by`→`new_slug`; CLI supersede gains `--reason` | ✓ |
| Rename param only | Rename, no CLI flag | |
| Keep superseded_by, add CLI --reason | Keep tool param name, add CLI flag | |

**User's choice:** Rename param + add CLI --reason. Full CLI/MCP parity.

---

## Execution-state on amend

| Option | Description | Selected |
|--------|-------------|----------|
| Release active claim on amend | Delete claim row + CLAIMED_BY edge in the amend tx; slices intact | ✓ |
| Leave execution state untouched (defer) | Rely on lease expiry / Gastown; note as deferred | |
| Release claim + reset slices | Full reset to clean authoring state | |

**User's choice:** Release active claim on amend.
**Notes:** Slices left intact (they're the decompose output being re-authored). Confirmed a `claims` table + `slices` table exist; current `LifecycleAmendSpec` leaves a stale claim, which #900 flagged.

---

## Skills & verification

| Option | Description | Selected |
|--------|-------------|----------|
| Teach amend/supersede in skills | Add teaching to MCP-first skills, incl. land-one-before | ✓ |
| Tool docs only, skip skills | Rely on tool descriptions | |
| You decide | Delegate to planning | |

**User's choice:** Teach amend/supersede in skills.

| Option | Description | Selected |
|--------|-------------|----------|
| MCP-only e2e + storage integration | Ginkgo e2e via `author` tool + storage integration tests | ✓ |
| Integration tests only | Storage/handler tests, no e2e round-trip | |
| You decide test layering | Delegate to planning | |

**User's choice:** MCP-only e2e + storage integration.

---

## the agent's Discretion

- Exact proto `reserved` field/RPC numbering for the retired AuthoringService.Amend/Supersede messages.
- Precise wording of the amend next-step hint and the rewritten tool descriptions.
- Test layering details and how the MCP-only e2e simulates done→amend→re-author.
- Which skill file(s) carry the amend/supersede teaching vs. tool-description docs.

## Deferred Ideas

- Reset/clear decompose slices on amend — rejected for this phase (slices shouldn't be discarded); revisit if a future execution-integration phase needs hard-reset semantics.
- Broader claim/lease reconciliation between SpecGraph amend and Gastown execution — an execution-integration concern beyond this phase.
