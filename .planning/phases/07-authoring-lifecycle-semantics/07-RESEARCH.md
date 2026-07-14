# Phase 7: Authoring Lifecycle Semantics - Research

**Researched:** 2026-07-14
**Domain:** Go / ConnectRPC service consolidation, protobuf removal, pgx transactional storage, MCP tool surface
**Confidence:** HIGH (every claim grounded in verbatim source at cited file:line)

## Summary

This is a **consolidation refactor**, not a greenfield build. Two divergent amend/supersede implementations exist; the correct one (`LifecycleService` → `LifecycleAmendSpec`/`LifecycleSupersedeSpec`) already ships and is e2e-tested. Phase 7 reroutes the MCP `author` tool to that correct path and deletes the broken twin (`AuthoringService.Amend/Supersede` → `AmendSpec`/`SupersedeSpec`). All design decisions (D-01…D-10) are LOCKED in `07-CONTEXT.md`; this research confirms exact mechanics, signatures, call sites, and the full blast radius so the planner can sequence removals without breaking `go build ./...` or `task check`.

**Every open question from CONTEXT is resolved by source inspection.** The biggest de-risking finding: `client.Lifecycle` **already exists** on the MCP `Client` struct (`internal/mcp/client.go:25` + wired at `:43`) — the reroute needs **zero new client wiring**, just a call-site swap. The supersede `reason` (D-06) requires a genuine chain of signature changes (proto field → `LifecycleSupersedeSpec` param → `LifecycleBackend` interface → 3 fake backends → handler), which this document enumerates exhaustively.

**Primary recommendation:** Execute in dependency order — (1) proto edits + `task proto`, (2) reroute MCP handlers + rename params, (3) thread supersede `reason`, (4) add claim-release to `LifecycleAmendSpec`, (5) delete the broken path + all its fakes/tests, (6) skills + MCP-only e2e. Deleting the broken path LAST avoids compile breaks while the reroute is in flight.

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Route MCP `author` tool's `handleAmend`/`handleSupersede` (`internal/mcp/tools_authoring.go`) to `client.Lifecycle.TransitionAmend` / `client.Lifecycle.TransitionSupersede`.
- **D-02:** Fully retire the broken path: mark retired proto elements appropriately, run `task proto`, delete `AuthoringHandler.Amend/Supersede`, `AmendSpec`/`SupersedeSpec` storage methods, and `storage.AmendResult`. CLI untouched.
- **D-03:** Rename tool param `target_stage` → `re_entry_stage`; required; valid `spark|shape|specify|decompose`; reject `approved`/`done`/terminal. Rewrite tool `Description` + param doc to teach the land-one-before model.
- **D-04:** `reason` is required on the MCP amend action.
- **D-05:** Amend tool result echoes the next stage command (compose from `re_entry_stage`; no proto change — `TransitionAmendResponse` already returns `Spec`).
- **D-06:** Add optional `reason` to `TransitionSupersedeRequest`; thread through `LifecycleSupersedeSpec` into the supersede changelog entry. Proto change + `task proto`.
- **D-07:** Rename MCP supersede param `superseded_by` → `new_slug`; add optional `--reason` flag to CLI `supersede`. Done-only precondition + valid replacement already enforced by `LifecycleSupersedeSpec`.
- **D-08:** Amending an in_progress/review spec releases its active claim (delete claim row + `CLAIMED_BY` edge) inside the same amend transaction. Slices left intact. Confirm exact claim/edge mechanics.
- **D-09:** Add amend/supersede/re-entry teaching to `specgraph-authoring` and/or `specgraph-troubleshooting` skills, including the land-one-before model.
- **D-10:** Verification gate = MCP-only e2e (Phase 6 style, `e2e/api/`) + storage integration tests (claim-release-on-amend, amend-eligible stages, done-only supersede, re-entry landing).

### the agent's Discretion

- Exact proto removal mechanics for the retired `AuthoringService.Amend/Supersede` messages (enumerated below).
- Precise wording of the amend next-step hint (D-05) and rewritten tool descriptions.
- Test layering details and how the MCP-only e2e simulates done→amend→re-author.
- Which skill file(s) carry the amend/supersede teaching (D-09) vs. tool-description docs.

### Deferred Ideas (OUT OF SCOPE)

- Reset/clear decompose slices on amend — rejected; slices are the re-authored decompose output.
- Broader claim/lease reconciliation between SpecGraph amend and Gastown execution.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LIFE-01 (#900) | `amend` works while in-flight (`>= approved && < done`); `supersede` only on `done`. | `LifecycleAmendSpec` already enforces `stage IN ('approved','in_progress','review')` (`lifecycle.go:64-65`) via `IsAmendEligible` (`spec_domain.go:79-86`); `LifecycleSupersedeSpec` already enforces `oldCheck.Stage != done → ErrSpecNotDone` (`lifecycle.go:123-125`). Rerouting the MCP tool (D-01) inherits both. Claim-release (D-08) closes the stale-lease gap. |
| LIFE-02 (#899) | After `amend --re-entry <stage>`, that stage runs immediately without an `invalid stage transition` no-op. | `LifecycleAmendSpec` lands the spec at `targetStage.PrecedingAuthStage()` (`lifecycle.go:52`, helper `spec_domain.go:92-98`), so the authoring command (preceding→target) is a forward transition accepted by `ValidateTransition` (`stage_validation.go:63`). The broken `AmendSpec` lands AT target — the #899 no-op — and is being deleted. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| amend/supersede eligibility + stage math | Storage (`postgres/lifecycle.go`) | Domain (`spec_domain.go`) | Precondition checks and land-one-before math already live here; single source of truth. |
| amend/supersede request validation + error mapping | API handler (`server/lifecycle_handler.go`) | — | `TransitionAmend/Supersede` validate slug/reason/re_entry_stage and map sentinels via `lifecycleError`. |
| MCP agent-facing surface (params, next-step hint, teaching) | MCP tool (`mcp/tools_authoring.go`) | Skills (`skills/embedded/`) | The `author` tool is the MCP-only agent's only surface; it must be impossible to misuse. |
| CLI surface | CLI (`cmd/specgraph/lifecycle.go`) | — | Already correct; only add `--reason` to `supersede`. |
| claim/lease state | Storage (`postgres/execution.go`, `claims` table) | — | Claims are execution state; releasing them belongs in the same storage tx as the amend. |

## Standard Stack

No new external packages. Phase reuses the existing stack:

| Component | Version | Purpose | Source |
|-----------|---------|---------|--------|
| Go | per `go.mod` | Implementation language | repo |
| ConnectRPC (`connectrpc.com/connect`) | in use | RPC handlers + generated clients | `internal/server`, `gen/` |
| pgx v5 (`github.com/jackc/pgx/v5`) | in use | Postgres driver, transactions | `internal/storage/postgres` |
| buf + `task proto` | in use | Protobuf → Go codegen (gen/ committed) | Taskfile |
| Ginkgo/Gomega (`github.com/onsi/ginkgo/v2`) | in use | e2e suite (`go test -tags e2e`) | `e2e/api` |
| `github.com/mark3labs/mcp-go` | in use | MCP client used by the MCP-only e2e | `e2e/api/mcp_only_authoring_test.go:15-17` |

## Package Legitimacy Audit

Not applicable — this phase installs **no external packages**. It edits proto, Go source, and Markdown skills only.

## Architecture Patterns

### Data flow — MCP amend (after Phase 7)

```
MCP agent
  → author tool  action=amend  slug, reason(required), re_entry_stage(required)   [tools_authoring.go handle → handleAmend]
      → t.client.Lifecycle.TransitionAmend(TransitionAmendRequest{Slug, Reason, ReEntryStage})   [D-01 reroute]
          → LifecycleHandler.TransitionAmend  [lifecycle_handler.go:42]  validate slug/reason/re_entry_stage
              → store.LifecycleAmendSpec(ctx, slug, reason, reEntryStage)  [postgres/lifecycle.go:38]
                  RunInTransaction:
                    - land stage = re_entry.PrecedingAuthStage()   (fixes #899)
                    - [D-08 NEW] release active claim + CLAIMED_BY edge if one exists
                    - recompute content hash, write changelog (Reason=reason)
          ← TransitionAmendResponse{Spec}
      ← compose next-step hint from re_entry_stage (D-05), jsonResult
```

### Data flow — MCP supersede (after Phase 7)

```
author tool  action=supersede  slug, new_slug(required), reason(optional)   [handleSupersede]
  → t.client.Lifecycle.TransitionSupersede(TransitionSupersedeRequest{Slug, NewSlug, Reason})   [D-01/D-06/D-07]
      → LifecycleHandler.TransitionSupersede  [lifecycle_handler.go:79]
          → store.LifecycleSupersedeSpec(ctx, oldSlug, newSlug, reason)   [SIGNATURE CHANGE: +reason]
              done-only guard (ErrSpecNotDone), valid replacement, SUPERSEDES edge, changelog(Reason)
```

### Confirmed: `client.Lifecycle` already exists (resolves CONTEXT's key open question)

`internal/mcp/client.go` — the MCP `Client` struct **already** has the field and wiring. No new plumbing:

```go
// internal/mcp/client.go:25
Lifecycle      specgraphv1connect.LifecycleServiceClient
// internal/mcp/client.go:43
Lifecycle:      specgraphv1connect.NewLifecycleServiceClient(httpClient, baseURL),
```

`[VERIFIED: internal/mcp/client.go:14-46]` The reroute in `handleAmend`/`handleSupersede` is a call-site swap only.

### Anti-Patterns to Avoid

- **Do NOT reserve field numbers for whole-message removals.** See "Common Pitfalls → Proto removal semantics" — the AGENTS.md `reserved` gotcha applies to *fields within a retained message*, not to deleting entire messages or RPC methods. Misapplying it will confuse the diff.
- **Do NOT touch the CLI amend/supersede request path beyond adding `--reason`.** `cmd/specgraph/lifecycle.go` already talks only to `LifecycleService` (`runAmend:50`, `runSupersede:79`).
- **Do NOT delete `TransitionStage` in `postgres/authoring.go`.** Only `AmendSpec`/`SupersedeSpec` are retired; `TransitionStage` (`authoring.go:27`) powers the whole funnel + approve.
- **Do NOT clear the `slices` table on amend** (deferred idea; slices are the re-authored decompose output).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Amend eligibility / land-one-before math | New stage logic | `SpecStage.IsAmendEligible` (`spec_domain.go:79`), `PrecedingAuthStage` (`:92`), `ExcludesReEntry` (`:29`) | Already correct + unit-tested. |
| Error → connect code mapping | New switch | `LifecycleHandler.lifecycleError` (`lifecycle_handler.go:275`) — already maps `ErrSpecNotAmendable`, `ErrSpecNotDone`, `ErrReEntryStageRequired`, `ErrInvalidReEntryStage`, `ErrConcurrentModification`, etc. | Rerouted MCP tool inherits all mappings. |
| Claim + edge deletion inside a tx | New SQL | Copy the exact two-statement pattern from `RecordCompletion` (`execution.go:162-177`) | Deletes `claims` row then `CLAIMED_BY` edge; proven. |
| Next-step hint | New format | Mirror `promptsToProto`/`NextPrompts` intent (`authoring_handler.go:119,210,329,426`); compose a plain string in the MCP handler | D-05 needs no proto change. |

**Key insight:** Nearly everything this phase "adds" already exists on the lifecycle path. The work is *deletion + rerouting + one storage insert (claim release) + one proto field (reason)*, not new logic.

## Runtime State Inventory

This is a refactor/removal phase — inventory of what survives file edits.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `claims` table (`migrations/001_initial_schema.sql:139-146`), PK `(project_slug, spec_slug)`; `CLAIMED_BY` edges in `edges`. A spec amended from in_progress/review may hold an active claim row + edge. | D-08: delete claim row + edge in the amend tx **conditionally** (only if a claim exists — `GetActiveClaim` returns `nil` for unclaimed specs, `execution.go:337-338`). `slices` table left intact. |
| Live service config | None — no external service embeds amend/supersede semantics. | None. |
| OS-registered state | None. | None — verified by scope (Go/proto/MD only). |
| Secrets/env vars | None. | None. |
| Build artifacts | `gen/specgraph/v1/authoring.pb.go` + `authoring.connect.go` are **committed generated code**; they carry `Amend*/Supersede*` message types + `AuthoringService.Amend/Supersede` client/handler methods (`gen/.../authoring.pb.go:1985-2226`, `specgraphv1connect/authoring.connect.go`). | After proto edits, `task proto` regenerates and **commits** the reduced `gen/` (AGENTS.md: gen/ is committed; `task proto:check` verifies staleness). Editing `gen/` directly is blocked by a Claude Code PreToolUse hook — edit `.proto` sources. |

**Nothing found in Live service config / OS state / Secrets categories — verified by phase scope (internal code refactor).**

## Blast Radius — every non-gen, non-test caller of retired symbols

### `AuthoringService.Amend` / `Supersede` (proto RPCs)
- **Proto:** `proto/specgraph/v1/authoring.proto:440-443` (rpc declarations); messages `AmendRequest` (`:375-384`), `AmendResponse` (`:387-394`), `SupersedeRequest` (`:397-404`), `SupersedeResponse` (`:407-412`).
- **Handler impls (delete):** `AuthoringHandler.Amend` (`server/authoring_handler.go:551`), `AuthoringHandler.Supersede` (`:593`). These are the only impls of the connect handler methods.
- **MCP callers (reroute, then dead):** `handleAmend` calls `t.client.Authoring.Amend` (`tools_authoring.go:357`); `handleSupersede` calls `t.client.Authoring.Supersede` (`:377`). After D-01 these call `t.client.Lifecycle.*` instead.
- **Generated (auto after `task proto`):** `AuthoringServiceClient`/`AuthoringServiceHandler` interfaces lose the methods (`gen/.../specgraphv1connect/authoring.connect.go`).

### `AmendSpec` / `SupersedeSpec` (storage methods) + `storage.AmendResult`
- **Impl (delete):** `Store.AmendSpec` (`postgres/authoring.go:363`), `Store.SupersedeSpec` (`postgres/authoring.go:297`).
- **Interface (remove methods):** `AuthoringSpecLifecycle` (`storage/authoring.go:158-161`) declares both; it is composed into `AuthoringBackend` (`:166-171`). Removing the two methods from `AuthoringSpecLifecycle` is the interface change. `storage.AmendResult` struct (`storage/authoring.go:136-141`) is deleted.
- **Only production caller:** the two `AuthoringHandler` methods above (`authoring_handler.go:575`, `:616`) — both being deleted anyway.
- **`ValidateAmendTransition` (storage):** `storage.ValidateAmendTransition` (`stage_validation.go:78`) has exactly ONE production caller — `AmendSpec` (`authoring.go:374`). After deletion it becomes dead code. It is exported so `unused`/`revive` will NOT flag it, but it is genuinely orphaned. **Recommendation:** delete `ValidateAmendTransition` + `TestValidateAmendTransition` (`stage_validation_test.go:86`) as cleanup, or leave with a note. (Note: a separate `authoring.ValidateAmendTransition` in `internal/authoring/stages.go:135` is a different function, already only test-referenced; NOT in scope.)
- **`authoringStageFromString` (MCP):** `tools_authoring.go:111`; sole production caller is `handleAmend:352`. After D-03 (amend uses `SpecStage` strings passed straight through as `re_entry_stage`), it becomes unused in production (only `tools_authoring_test.go:952-960` reference it). **Delete it + `TestAuthoringStageFromString`** to avoid an unused-function lint on an unexported symbol.

### Test doubles that MUST be updated in the same change (compile-blocking)
| File:line | Symbol | Change |
|-----------|--------|--------|
| `internal/server/authoring_handler_test.go:32` | `fakeAuthoringBackend.amendResult` field | remove |
| `internal/server/authoring_handler_test.go:67,71` | `fakeAuthoringBackend.SupersedeSpec/AmendSpec` | remove methods |
| `internal/server/authoring_handler_test.go:191,195` | `authoringTestBackend.SupersedeSpec/AmendSpec` | remove methods |
| `internal/server/authoring_handler_test.go:719,1120,1135,1330,1389,1630` | `TestAmend*`/`AmendResult` fixtures | prune amend/supersede handler tests |
| `internal/server/test_scoper_test.go:192,196` | `stubBackend.SupersedeSpec/AmendSpec` | remove methods |
| `internal/mcp/testhelpers_test.go:316-317,378-389` | `mockAuthoringService.amend/supersede` fns + `Amend/Supersede` methods | remove (connect client interface loses these after `task proto`) |
| `internal/mcp/tools_authoring_test.go:136-176` | amend/supersede tool tests hitting `mockAuthoringService` | rewrite to assert on a `mockLifecycleService` (or extend existing lifecycle mock) |
| `internal/storage/postgres/authoring_test.go:423-560` | `TestSupersedeSpec_Authoring`, `TestSupersedeSpec_NotFound`, `TestAmendSpec*` | delete (methods gone) |

### Supersede `reason` chain (D-06) — signature ripple to update together
| File:line | Current | New |
|-----------|---------|-----|
| `proto/specgraph/v1/lifecycle.proto:114-117` | `TransitionSupersedeRequest{slug=1, new_slug=2}` | add `string reason = 3;` |
| `internal/storage/lifecycle.go:114` | `LifecycleSupersedeSpec(ctx, oldSlug, newSlug string) (*Spec, *Spec, error)` | `(ctx, oldSlug, newSlug, reason string)` |
| `internal/storage/postgres/lifecycle.go:113` | impl signature | add `reason`; use it in the old-spec changelog `Reason` (currently `fmt.Sprintf("Superseded by %s", newSlug)`, `:217`). Decide: prefer user `reason` when non-empty, else keep default. |
| `internal/server/lifecycle_handler.go:95` | `store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug)` | pass `msg.Reason` |
| `internal/server/lifecycle_handler_test.go:48` | `fakeLifecycleBackend.LifecycleSupersedeSpec(ctx, oldSlug, newSlug string)` | add `reason` |
| `internal/server/test_scoper_test.go:246` | `stubBackend.LifecycleSupersedeSpec(...string, string)` | add param |
| `internal/server/error_sanitize_test.go:196` | `errorBackend.LifecycleSupersedeSpec(...)` | add param |
| `cmd/specgraph/lifecycle.go:79-82` | `runSupersede` builds `TransitionSupersedeRequest{Slug, NewSlug}` | add `Reason: supersedeReason`; register `--reason` flag in `init()` (`:314`) |

`ChangeLogEntry.Reason` already exists (`internal/storage/changelog.go:20`) — **no schema migration.** `[VERIFIED: changelog.go:12-23]`

## Claim-release mechanics (D-08) — exact pattern to copy

`RecordCompletion` (`internal/storage/postgres/execution.go:162-177`) shows the canonical two-statement delete, already inside a `RunInTransaction`:

```go
// execution.go:162 — delete claim row
`DELETE FROM claims WHERE project_slug = $1 AND spec_slug = $2 AND agent = $3`
// execution.go:171 — delete CLAIMED_BY edge
`DELETE FROM edges WHERE project_slug = $1 AND from_slug = $2 AND to_slug = $3 AND edge_type = 'CLAIMED_BY'`
```

- **Edge direction:** `CLAIMED_BY` is `from_slug = spec_slug`, `to_slug = agent` (`execution.go:172-173`, and `ReleaseExpiredClaims:310-311`). `[VERIFIED]`
- **Get the agent conditionally:** `LifecycleAmendSpec` does not know the agent. Use `GetActiveClaim(txCtx, slug)` (`execution.go:326`) which returns `*storage.Claim` with `.Agent` or `nil` if unclaimed (`:337-338`). **Only delete if the claim is non-nil** — unclaimed amend-eligible specs (`approved`) hold no claim, so an unconditional delete is a harmless no-op on the row but the guidance is: fetch → if `claim != nil` delete row+edge by `claim.Agent`. This avoids relying on `spec_slug`-only deletes and matches the agent-scoped pattern.
- **Placement:** inside the existing `RunInTransaction` in `LifecycleAmendSpec` (`lifecycle.go:55-107`), after the stage `UPDATE` succeeds and before/after `recomputeContentHash`. Same tx = the lease cannot linger. `slices` table untouched.
- **`claims` PK** is `(project_slug, spec_slug)` (`migrations/001_initial_schema.sql:145`) — at most one claim per spec, so no fan-out.

## Proto removal semantics (correction / flag for planner)

CONTEXT D-02 and AGENTS.md say "mark reserved for both field/RPC number and name." **This is imprecise for Phase 7's removals and must not be applied literally:**

- `reserved` in protobuf reserves **field numbers within a retained message** or **enum values within an enum**. It does **not** apply to (a) deleting an entire message type, or (b) removing an RPC method from a service. Proto3 has no `reserved` for service methods or top-level message names.
- **Correct actions:**
  - Delete `rpc Amend(...)` and `rpc Supersede(...)` from `service AuthoringService` (`authoring.proto:440-443`). No reservation.
  - Delete the four message definitions `AmendRequest/AmendResponse/SupersedeRequest/SupersedeResponse` (`:375-412`). No reservation (these are whole-type removals, not field removals).
  - For the `TransitionSupersedeRequest.reason` **addition**, field number `3` is free (existing fields are `slug=1`, `new_slug=2`) — no reservation needed.
- The AGENTS.md `reserved` gotcha remains correct **when you remove a field from a message you keep** — not the case here.

After edits: run `task proto` (incremental; fingerprints `.proto`), then `go build ./...`, then commit `gen/`.

## Common Pitfalls

### Pitfall 1: Deleting the broken path before the reroute compiles
**What goes wrong:** removing `AmendSpec`/`SupersedeSpec` or the proto RPCs while `handleAmend`/`handleSupersede` still reference `t.client.Authoring.Amend` → build break.
**How to avoid:** reroute FIRST (D-01), regenerate proto, then delete impls + fakes. Sequence: proto add-reason → reroute → delete.

### Pitfall 2: Forgetting a fake backend
**What goes wrong:** `AuthoringBackend`/`LifecycleBackend` are satisfied by multiple test doubles (`fakeAuthoringBackend`, `authoringTestBackend`, `stubBackend`, `errorBackend`, `fakeLifecycleBackend`, plus MCP `mockAuthoringService`). Removing an interface method or changing `LifecycleSupersedeSpec`'s signature breaks every unfixed double via the compile-time assertions (`var _ storage.AuthoringBackend = (*Store)(nil)` at `authoring.go:20`; `var _ storage.LifecycleBackend = (*Store)(nil)` at `lifecycle.go:19`).
**How to avoid:** use the Blast Radius tables above as a checklist; `go build ./... && go vet ./...` after each interface edit.

### Pitfall 3: Handler error assertions use codes, not messages
**What goes wrong:** rewriting amend/supersede tests to assert on error strings.
**How to avoid:** assert `connect.CodeOf(err)` (AGENTS.md: handler errors are sanitized). The lifecycle path already returns `CodeFailedPrecondition` for `ErrSpecNotDone`/`ErrSpecNotAmendable`, `CodeInvalidArgument` for `re_entry_stage`, `CodeAborted` for concurrent-mod (`lifecycle_handler.go:283-322`). e2e already asserts codes (`lifecycle_pipeline_test.go:206,216`).

### Pitfall 4: MCP mock returns non-sentinel errors
**What goes wrong:** rerouted tool tests need a lifecycle mock; if it returns `fmt.Errorf` where handler code does `errors.Is`, mappings break.
**How to avoid:** AGENTS.md — mock backends return sentinel errors (`storage.ErrSpecNotDone`, etc.).

### Pitfall 5: `re_entry_stage` validation duplicated wrong
**What goes wrong:** the MCP tool re-validating stage names and diverging from the handler.
**How to avoid:** the handler `TransitionAmend` already rejects empty/invalid/excluded re-entry stages (`lifecycle_handler.go:54-65` using `SpecStage.IsValid`/`ExcludesReEntry`). The MCP tool should pass `re_entry_stage` through as a string and surface the handler's `connectErrResult`; keep only a friendly required-field check.

## Code Examples

### Rerouted `handleAmend` (illustrative shape)
```go
// internal/mcp/tools_authoring.go — replaces current :344-366
func (t *authorTool) handleAmend(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" { return errResult("slug is required for amend"), nil }
	reEntry := stringParam(params, "re_entry_stage")
	if reEntry == "" { return errResult("re_entry_stage is required (spark|shape|specify|decompose)"), nil }
	reason := stringParam(params, "reason")
	if reason == "" { return errResult("reason is required for amend"), nil } // D-04
	resp, err := t.client.Lifecycle.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug: slug, Reason: reason, ReEntryStage: reEntry,
	}))
	if err != nil { return connectErrResult(err) }
	// D-05: compose next-step hint from reEntry (spec landed one stage before).
	// e.g. "Amended; re-entering shape — call `author action=shape` next."
	return jsonResult(resp.Msg), nil // + hint text
}
```
`[Source: current handler internal/mcp/tools_authoring.go:344-366; lifecycle client internal/mcp/client.go:25]`

### How a spec reaches `done` in tests (for e2e/integration setup)
CLI/ConnectRPC path (`e2e/api/helpers_test.go:88-201`): `advanceStage(ctx, slug, "done")` walks shape→specify→decompose→approve, then **claims** (`ClaimSpec`, `:180`) and **completes** (`ReportCompletion`, `:193`) → stage `done`. Or `claimAndComplete(ctx, slug)` (`:206`) from an already-approved spec.

**MCP-only path (for D-10 e2e):** the `author` tool only reaches `approved`. To reach `done` through MCP alone, use the `claim` tool (`tools_execution.go:48`, action `claim`) then the `report` tool (`tools_execution.go:336`, action `completion` at `:361`). This makes a fully MCP-driven done→amend→re-author→supersede sequence feasible without any ConnectRPC client.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MCP `author` amend/supersede → `AuthoringService` (rejects `approved`, lands AT target, no done-guard) | Route to `LifecycleService` (in-flight amend, done-only supersede, land-one-before) | PRs #889/#892 fixed the lifecycle path; Phase 7 points MCP at it | Single source of truth; #899 no-op + #900 inversion fixed on the MCP surface. |
| Two `ValidateAmendTransition` + `AmendSpec` stage math | `IsAmendEligible`/`PrecedingAuthStage`/`ExcludesReEntry` on `SpecStage` | Lifecycle refactor | `AmendSpec`-only `ValidateAmendTransition` (storage) becomes dead. |

**Deprecated/outdated after this phase:**
- `AuthoringService.Amend/Supersede` RPCs + messages (deleted).
- `Store.AmendSpec`/`Store.SupersedeSpec`, `storage.AmendResult`, `AuthoringSpecLifecycle` (interface emptied → likely removed entirely; confirm no other methods).
- `storage.ValidateAmendTransition`, `authoringStageFromString` (orphaned).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Removing both methods from `AuthoringSpecLifecycle` empties the interface, so `AuthoringBackend` should drop the `AuthoringSpecLifecycle` embed entirely (`storage/authoring.go:166-171`). | Blast Radius | Low — planner verifies the interface has no other methods; grep shows only `SupersedeSpec`/`AmendSpec` in it (`:159-160`). |
| A2 | For supersede `reason` threading, preferred behavior is "use user `reason` in the old-spec changelog when non-empty, else keep the existing `Superseded by <new>` default." CONTEXT D-06 says "into the supersede changelog entry" but not which field/precedence. | Supersede reason chain | Low — cosmetic changelog wording; confirm with planner/discuss if audit format matters. |
| A3 | No existing content-drift test gates the **skills** files. `TestContentProtoDrift` (`internal/authoring/drift_test.go:15`) only checks `internal/authoring/content/stage-*.md` against stage-output proto messages — NOT skills. CONTEXT D-09's "TestContentProtoDrift-style check that could catch backticked-token drift" for skills would be **new** if desired. | Skills & verification | Medium — if planner assumes an existing skills drift-gate, none exists; treat as optional new test, not a reuse. |
| A4 | The MCP-only e2e can reach `done` via the `claim` + `report` MCP tools (no ConnectRPC). | Code Examples / Validation | Low — tools confirmed at `tools_execution.go:48,336,361`; exact arg names to verify when writing the test. |

## Open Questions

1. **Should `AuthoringSpecLifecycle` be deleted or kept empty?**
   - What we know: it contains only `SupersedeSpec` + `AmendSpec` (`storage/authoring.go:158-161`).
   - What's unclear: whether to remove the named interface and its embed in `AuthoringBackend`, or keep a stub.
   - Recommendation: delete the interface and its embed; `revive` prefers no empty interfaces.

2. **Exact placement of claim-release relative to `recomputeContentHash`/changelog in `LifecycleAmendSpec`.**
   - What we know: must be same tx (`lifecycle.go:55-107`).
   - Recommendation: after the stage `UPDATE` guard passes (`:71`), before `recomputeContentHash` (`:83`) — ordering is not semantically load-bearing; keep it adjacent to the stage mutation for readability.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | postgres integration tests (testcontainers) + `task pr-prep` e2e | assumed per repo workflow | `pgvector/pgvector:pg18` | Unit tests (`task check`) run without Docker; integration/e2e need it. |
| buf / `task proto` | proto regen | yes (repo toolchain) | — | none — required to regen `gen/`. |

**Missing dependencies with no fallback:** none identified for planning; Docker is required to run the D-10 integration + e2e gates (matches existing lifecycle tests).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing`; postgres integration via testcontainers (`//go:build integration` implied by `internal/storage/postgres`); e2e via Ginkgo/Gomega (`//go:build e2e`) |
| Config file | Taskfile.dev (`task test`, `task test:integration`, `task test:e2e`, `task check`, `task pr-prep`) |
| Quick run command | `task test` (skips integration + e2e) then `task check` |
| Full suite command | `task pr-prep` (check → integration → e2e; requires Docker) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LIFE-01 | amend from approved/in_progress/review; reject done | integration | `go test -tags integration ./internal/storage/postgres/ -run TestLifecycle` | ✅ extend `lifecycle_test.go:19-183` (add claim-release cases) |
| LIFE-01 | supersede only on done; `reason` recorded | integration | same | ✅ extend `lifecycle_test.go:184-320` (add reason assertion) |
| LIFE-01 | claim released on amend of claimed spec | integration | `go test -tags integration ./internal/storage/postgres/ -run TestLifecycleAmend_ReleasesClaim` | ❌ Wave 0 — new test |
| LIFE-02 | re-entry lands one-before; next stage runs | integration + e2e | `lifecycle_test.go` + `e2e/api/lifecycle_pipeline_test.go:57-91` | ✅ pipeline already asserts land-at-spark for re-entry shape |
| LIFE-01/02 | full MCP-only done→amend→re-author→supersede | e2e | `go test -tags e2e ./e2e/api/ -run "MCP"` | ❌ Wave 0 — new `mcp_only_lifecycle_test.go` modeled on `mcp_only_authoring_test.go` |
| D-01/D-03/D-07 | rerouted MCP tool calls Lifecycle; param renames | unit | `go test ./internal/mcp/ -run TestAuthorTool` | 🔧 rewrite `tools_authoring_test.go:136-176` against a lifecycle mock |
| D-02 | handler/storage retirements compile + old tests pruned | unit | `go build ./... && task check` | 🔧 prune per Blast Radius |

### Sampling Rate
- **Per task commit:** `go build ./... && task test` (fast, no Docker).
- **Per wave merge:** `task check` (fmt/license/lint/build/unit).
- **Phase gate:** `task pr-prep` green (integration + e2e) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/storage/postgres/lifecycle_test.go` — new `TestLifecycleAmend_ReleasesClaim` (claim + `CLAIMED_BY` edge gone; unclaimed amend is a no-op) — covers D-08/LIFE-01.
- [ ] `internal/storage/postgres/lifecycle_test.go` — extend supersede tests to assert `reason` lands in the changelog — covers D-06.
- [ ] `e2e/api/mcp_only_lifecycle_test.go` — MCP-only done→amend→re-author→supersede, asserting stage landings + rejections (amend-on-done, supersede-on-non-done) — covers D-10.
- [ ] Rewrite `internal/mcp/tools_authoring_test.go` amend/supersede cases against a lifecycle mock; delete `authoring_handler_test.go` + `authoring_test.go` amend/supersede cases; update fakes (Blast Radius).
- [ ] Framework install: none — all frameworks already present.

## Security Domain

Internal refactor of already-validated paths; no new auth/crypto/session surface.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | yes (project scoping) | `scopeStore` enforces `X-Specgraph-Project` on all write paths (`lifecycle_handler.go:43`); rerouted MCP tool inherits it. |
| V5 Input Validation | yes | Handler validates slug/reason/re_entry_stage (`lifecycle_handler.go:48-65`); `SpecStage.IsValid`/`ExcludesReEntry`. MCP exchanges parsed in isolation to prevent JSON injection (existing pattern). |
| V6 Cryptography | no | Content hash is Murmur3 (integrity, not security); unchanged. |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stale claim/lease after amend (a spec returns to authoring but stays "claimed" → agent could keep executing) | Elevation / Tampering | D-08 claim + `CLAIMED_BY` release in the amend tx (this phase's core security-adjacent fix, #900). |
| Error message leakage | Information Disclosure | Handlers sanitize; assertions use codes (AGENTS.md; `lifecycle_handler.go:275`). |
| SQL injection | Tampering | Parameterized pgx queries only; `storeJSONColumn` allowlists columns (`authoring.go:424-447`). |

## Sources

### Primary (HIGH confidence — verbatim source this session)
- `internal/mcp/client.go:14-46` — `Lifecycle` client already wired.
- `internal/mcp/tools_authoring.go:130-386` — `author` tool def, `handleAmend`, `handleSupersede`, `authoringStageFromString`.
- `internal/storage/postgres/lifecycle.go:32-243` — `LifecycleAmendSpec`, `LifecycleSupersedeSpec`.
- `internal/storage/postgres/authoring.go:27-414` — `TransitionStage` (keep), `SupersedeSpec`/`AmendSpec` (delete).
- `internal/server/lifecycle_handler.go:42-325` — `TransitionAmend/Supersede` + `lifecycleError`.
- `internal/server/authoring_handler.go:551-623` — `Amend`/`Supersede` handlers (delete) + `NextPrompts`/`promptsToProto` pattern.
- `internal/storage/authoring.go:136-171` — `AmendResult`, `AuthoringSpecLifecycle`, `AuthoringBackend`.
- `internal/storage/lifecycle.go:105-124` — `LifecycleBackend` interface.
- `internal/storage/spec_domain.go:27-111`, `stage_validation.go:46-93` — stage helpers + `ValidateTransition`/`ValidateAmendTransition`.
- `internal/storage/postgres/execution.go:162-350` — claim + `CLAIMED_BY` delete pattern, `GetActiveClaim`.
- `internal/storage/changelog.go:12-23` — `ChangeLogEntry.Reason` (no migration).
- `proto/specgraph/v1/authoring.proto:375-450`, `lifecycle.proto:100-189` — messages, RPCs, `TransitionSupersedeRequest`.
- `cmd/specgraph/lifecycle.go:33-332` — CLI amend/supersede + flag registration.
- `internal/storage/postgres/migrations/001_initial_schema.sql:139-146` — `claims` table.
- `e2e/api/mcp_only_authoring_test.go` (Phase 6 template), `lifecycle_pipeline_test.go`, `helpers_test.go:88-229` (done setup).
- `internal/authoring/drift_test.go:15-60` — `TestContentProtoDrift` scope (authoring content, not skills).
- Blast-radius greps across `*.go` for `AmendSpec|SupersedeSpec|AmendResult|.Amend(|.Supersede(|authoringStageFromString`.

### Secondary (MEDIUM)
- `07-CONTEXT.md` (locked decisions), AGENTS.md (repo conventions).

## Metadata

**Confidence breakdown:**
- Consolidation blast radius: HIGH — enumerated every non-gen caller + every fake via grep and read.
- Claim-release mechanics: HIGH — exact SQL + edge direction copied from `RecordCompletion`/`ReleaseExpiredClaims`.
- Supersede reason chain: HIGH — full signature ripple traced; `ChangeLogEntry.Reason` confirmed.
- Proto removal semantics: HIGH — corrected the "reserved" phrasing against protobuf rules.
- Skills drift-gate: MEDIUM — no existing skills drift test; flagged as new-if-wanted (A3).

**Research date:** 2026-07-14
**Valid until:** ~2026-08-13 (30 days; stable internal code, low churn risk).
