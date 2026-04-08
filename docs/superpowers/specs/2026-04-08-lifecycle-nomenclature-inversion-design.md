# Lifecycle Nomenclature Inversion — Design Spec

**Date:** 2026-04-08
**Status:** Approved
**Bead:** spgr-rth
**Goal:** Fix the inverted semantics of amend and supersede, remove the `amended` parking stage, and align the lifecycle model with natural language meaning.

## Context

The current lifecycle model has amend only working from `done` (post-implementation) and supersede working from any non-terminal stage. This is backwards:

- **Amend** should mean "the spec needs to change while work is in flight — send it back to authoring." You amend a bill before it's passed, not after.
- **Supersede** should mean "this completed work is obsolete — create a replacement." Supersession replaces finished work.

Additionally, the `amended` semi-terminal stage exists as a parking state when no re-entry stage is specified during amend. This adds unnecessary complexity. The skill/plugin layer should guide users to the right re-entry stage, with `spark` as the default.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Amend eligible from `{approved, in_progress, review}` | These are in-flight stages where spec corrections belong |
| Amend excluded from `done` | Clean separation: amend = in-flight, supersede = post-completion |
| Supersede restricted to `done` only | "This finished work needs a fundamentally different replacement" |
| Abandon unchanged (any non-fully-terminal) | No semantic issue with current behavior |
| Remove `amended` stage entirely | Force callers to always specify a re-entry stage; skill layer picks `spark` as default |
| `re_entry_stage` required on amend | No more defaulting to the removed `amended` stage |
| Slices preserved during amend | Matches existing idempotency design in `StoreDecomposeOutput`; amend changes the spec, not necessarily the decomposition |
| Uniform amend behavior regardless of source stage | Changelog already records source stage in the checkpoint delta; no special casing needed |
| Supersede bootstrapping out of scope | The `SUPERSEDES` edge gives skills enough to pull old spec content into context for the new spec |

## Eligibility Matrix

| Operation | Current (broken) | New (correct) |
|-----------|-----------------|---------------|
| **Amend** | `{done}` | `{approved, in_progress, review}` |
| **Supersede** | Any non-terminal | `{done}` |
| **Abandon** | Any non-fully-terminal | Unchanged |
| **`amended` stage** | Exists (semi-terminal) | Removed |

## Changes

### 1. Storage Domain (`internal/storage/spec_domain.go`)

- Remove `SpecStageAmended` constant
- Remove `amended` from `ExcludesReEntry()` (the function still excludes `done`, `superseded`, `abandoned`)
- Remove `amended` from any semi-terminal helpers
- Add `IsAmendEligible() bool` method on `SpecStage` returning true for `{approved, in_progress, review}`

### 2. Stage Validation (`internal/storage/stage_validation.go`)

- `ValidateAmendTransition`: update to validate transitions from execution stages (`approved`, `in_progress`, `review`) to authoring stages (`spark`, `shape`, `specify`, `decompose`). The concept changes from "backward within the funnel" to "execution → authoring."

### 3. Storage Backend (`internal/storage/postgres/lifecycle.go`)

**`LifecycleAmendSpec`:**
- If `reEntryStage` is empty, return `ErrReEntryStageRequired` (new sentinel)
- Change SQL WHERE from `stage = 'done'` to `stage IN ('approved', 'in_progress', 'review')`
- Update changelog summary from `"Amended from done"` to `"Amended from <source_stage>"`
- `ExcludesReEntry()` check on target stays (excludes `done`, `superseded`, `abandoned`)

**`LifecycleSupersedeSpec`:**
- Change eligibility from `if terminalStages[oldCheck.Stage]` to `if oldCheck.Stage != storage.SpecStageDone`
- Error becomes `ErrSpecNotDone` (semantically correct now)

### 4. Lifecycle Interface (`internal/storage/lifecycle.go`)

- Update `LifecycleAmendSpec` doc: eligible from `{approved, in_progress, review}`, `re_entry_stage` required
- Update `LifecycleSupersedeSpec` doc: eligible from `{done}` only

### 5. Error Sentinels

- `ErrSpecNotDone` — keep for supersede ("spec must be done")
- `ErrSpecNotAmendable` — new sentinel for amend ("spec must be in an amend-eligible stage")
- `ErrReEntryStageRequired` — new sentinel for missing re-entry stage

### 6. Handler (`internal/server/lifecycle_handler.go`)

- `TransitionAmend`: validate `re_entry_stage` non-empty at handler level → `CodeInvalidArgument`
- Error mapping: `ErrSpecNotAmendable` → `CodeFailedPrecondition`, `ErrReEntryStageRequired` → `CodeInvalidArgument`
- `TransitionSupersede`: `ErrSpecNotDone` → `CodeFailedPrecondition` (stays)

**Cross-referencing error messages:**
- Amend on `done`: "spec is not amendable (stage=done); use supersede for completed specs"
- Supersede on non-done: "spec is not done (stage=in_progress); use amend for in-flight specs"

### 7. Proto (`proto/specgraph/v1/lifecycle.proto`)

- Update `TransitionAmendRequest.re_entry_stage` comment to document it as required
- No structural changes

### 8. CLI (`cmd/specgraph/`)

- `amend` subcommand: require re-entry stage argument. Error with guidance if missing: "re-entry stage required — one of: spark, shape, specify, decompose"
- Update help text: "amend an in-flight spec" not "amend a completed spec"
- `supersede` subcommand: update help text: "supersede a completed spec"

### 9. Plugin/Skill (`plugin/specgraph/skills/specgraph/SKILL.md`)

Extend the stage-routing table (Step 3A) to cover execution and lifecycle stages:

| Current Stage | Suggest | Action |
|---|---|---|
| `in_progress` | "Work is underway. Need to amend the spec?" | Offer `specgraph lifecycle amend <slug> --re-entry-stage <stage>` |
| `review` | "In review. Need to amend?" | Same |
| `done` | "This spec is complete. Supersede it or start something new?" | Offer `specgraph lifecycle supersede` |

### 10. Site Documentation

**Major rewrites:**
- `site/docs/concepts/lifecycle.md` — decision tree, eligibility, state diagram, "when to use" examples
- `site/docs/concepts/authoring.md` — state machine diagram (remove `done-->amended` path, add `approved/in_progress/review --> [re-entry stage]`)

**Scan and update:**
- `site/docs/concepts/specs.md`
- `site/docs/concepts/slices.md`
- `site/docs/cli-reference.md`
- `site/docs/guides/cli-cookbook.md`
- `site/docs/quickstart.md`
- `site/docs/architecture.md`
- `site/docs/concepts/index.md`
- `site/docs/concepts/decisions.md`
- `site/docs/guides/index.md`
- `site/docs/concepts/example-spec.md`

Remove all references to the `amended` stage across all docs.

### 11. Tests

**`internal/storage/stage_validation_test.go`:**
- Valid amend sources: `{approved, in_progress, review}`
- Invalid amend from `done` → error
- Remove `amended` stage references

**`internal/storage/postgres/lifecycle_test.go`:**
- Amend tests: create specs at `approved`/`in_progress`/`review` before amending
- Amend errors: from `done` → `ErrSpecNotAmendable`, empty re-entry → `ErrReEntryStageRequired`
- Remove "amended spec can be superseded/abandoned" tests
- Remove default-to-amended tests
- Supersede: from `done` → success, from non-done → `ErrSpecNotDone`

**`e2e/api/lifecycle_test.go`:**
- Amendment flow: create → advance to `in_progress` → amend to `shape` → verify changelog
- Supersession flow: create → advance to `done` → supersede → verify edge + terminal
- Error paths: amend-on-done, supersede-on-non-done, amend-without-re-entry

### 12. Postgres Migration

Add a goose migration that moves any specs currently in the `amended` stage to `spark` (safe default re-entry point). This is a safety net — in practice there may be no specs in this state, but the migration ensures consistency after the stage constant is removed.

## Follow-Up Work (Out of Scope)

- **Supersede bootstrapping** — Allow the new spec to be pre-populated from the old spec's content. The `SUPERSEDES` edge provides the lineage; the skill/agent layer should use it to bring old spec context into the authoring session.
