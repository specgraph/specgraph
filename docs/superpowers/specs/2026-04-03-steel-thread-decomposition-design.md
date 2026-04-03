# Steel Thread Decomposition Strategy

**Bead:** spgr-47v
**Date:** 2026-04-03
**Status:** Draft

## Problem

SpecGraph's decompose stage supports three strategies: vertical slice, layer
cake, and single unit. None explicitly models the common pattern of cutting a
thin vertical slice through the riskiest integration points first to prove out
interfaces, then broadening with parallel work.

This pattern -- steel thread -- is distinct from generic vertical slice because:

- The first slice exists to prove interfaces, not deliver user-visible value.
- All subsequent slices depend on the thread and can parallelize freely.
- The topology is structurally constrained (single root, full reachability).

## Design

### Proto: New Enum Value

Add `DECOMPOSITION_STRATEGY_STEEL_THREAD = 4` to the existing
`DecompositionStrategy` enum in `proto/specgraph/v1/authoring.proto`:

```protobuf
enum DecompositionStrategy {
  DECOMPOSITION_STRATEGY_UNSPECIFIED = 0;
  DECOMPOSITION_STRATEGY_VERTICAL_SLICE = 1;
  DECOMPOSITION_STRATEGY_LAYER_CAKE = 2;
  DECOMPOSITION_STRATEGY_SINGLE_UNIT = 3;
  // Thin vertical cut proving riskiest integration points first.
  // slices[0] is the thread (no dependsOn); all others reachable from it.
  DECOMPOSITION_STRATEGY_STEEL_THREAD = 4;
}
```

No new messages or fields. `DecomposeOutput` and `DecompositionSlice` remain
unchanged. The thread slice is identified by position: `slices[0]`.

### Server Validation

Add a `validateSteelThread(slices)` helper in the authoring handler, called
when `strategy == DECOMPOSITION_STRATEGY_STEEL_THREAD`:

1. **Single root check:** `slices[0].dependsOn` must be empty. If not, return
   `INVALID_ARGUMENT` with `"steel thread strategy requires slices[0] to have
   no dependencies (it is the thread)"`.

2. **Reachability check:** Walk each non-root slice's `dependsOn` chain
   transitively. Every slice must reach `slices[0].id` (the thread slice's
   string identifier). If any slice is disconnected, return
   `INVALID_ARGUMENT` with `"slice %q is not reachable from thread slice %q"`.

This is a graph traversal on a small in-memory slice list (typically fewer than
10 slices). The reachability check catches most disconnected-graph issues but
does not guarantee acyclicity -- a cycle that still has a path to the root would
pass reachability. The existing no-cycles validation covers that case separately.

Other strategies (`VERTICAL_SLICE`, `LAYER_CAKE`, `SINGLE_UNIT`) are
unaffected.

### CLI

No code changes. The CLI passes the strategy string from JSON input to proto
via proto3 JSON unmarshaling, which handles the new enum value automatically
after proto regeneration.

### Reference Docs

Update `plugin/specgraph/skills/specgraph-decompose/references/decompose-output-format.md`:

Add to Strategy Values:

```text
- `DECOMPOSITION_STRATEGY_STEEL_THREAD` -- First slice (slices[0]) is a thin
  vertical cut proving riskiest interfaces; all other slices depend on it and
  can parallelize
```

Add to Field Notes:

```text
- When using `STEEL_THREAD`, `slices[0]` is the thread slice and must have no
  `dependsOn`. All other slices must transitively depend on it.
```

### Decompose Skill

Update `plugin/specgraph/skills/specgraph-decompose/SKILL.md`:

**Strategy selection guidance:** Add steel thread as a decomposition option:

- Choose steel thread when the spec spans multiple layers/services and the
  interfaces between them are unproven.
- Choose steel thread when maximizing parallelism for subsequent work is a
  priority.
- Contrast with vertical slice: vertical slice delivers independent user value
  per slice; steel thread's first slice delivers confidence in interfaces, not
  necessarily user-visible value.

**Thread slice authoring guidance:** When steel thread is selected, guide the
author to:

1. Identify the riskiest or least-understood integration points.
2. Design the thread slice as the thinnest possible cut exercising those
   integration points end-to-end.
3. Focus `verify` criteria on interface contracts working, not feature
   completeness.
4. Frame remaining slices as "broadening" -- adding depth/features now that
   interfaces are proven.

**Include a concrete example** showing a thread slice that proves a round-trip
(e.g., proto -> storage -> handler -> CLI for a single operation) with
broadening slices adding remaining operations in parallel.

### E2E Test Fixture

Add `e2e/cli/testdata/decompose-output-steel-thread.json`:

```json
{
  "strategy": "DECOMPOSITION_STRATEGY_STEEL_THREAD",
  "slices": [
    {
      "id": "prove-roundtrip",
      "intent": "Prove proto-storage-handler-CLI round-trip for create operation",
      "verify": ["create request returns valid response", "stored entity retrievable via get"]
    },
    {
      "id": "broaden-crud",
      "intent": "Add update and delete operations using proven interfaces",
      "verify": ["update modifies stored entity", "delete removes entity"],
      "dependsOn": ["prove-roundtrip"]
    },
    {
      "id": "broaden-query",
      "intent": "Add list and filter operations",
      "verify": ["list returns all entities", "filter narrows results"],
      "dependsOn": ["prove-roundtrip"]
    }
  ]
}
```

### Testing

**Unit tests** (server validation, added to existing test files):

- Valid steel thread: thread at `[0]` with no deps, 2+ broadening slices all
  depending on it -- passes.
- Valid with chained broadening: B depends on thread, C depends on B -- passes
  (transitive reachability to root).
- Invalid: `slices[0]` has `dependsOn` -- `INVALID_ARGUMENT`.
- Invalid: broadening slice has no path to `slices[0]` -- `INVALID_ARGUMENT`.
- Regression: non-steel-thread strategies unaffected by new validation.

**E2E tests** (added to existing pipeline/authoring test files):

- Full pipeline: spark -> shape -> specify -> decompose (steel thread) ->
  approve. Verify slice graph nodes created with correct DEPENDS_ON edges.
- Negative: submit steel thread with disconnected slice, verify rejection.

## Files Changed

| File | Change |
|------|--------|
| `proto/specgraph/v1/authoring.proto` | Add `STEEL_THREAD = 4` enum value |
| `gen/specgraph/v1/authoring.pb.go` | Regenerated (via `task proto`) |
| `internal/server/authoring_handler.go` | Add `validateSteelThread()` helper |
| `internal/server/authoring_handler_test.go` | Unit tests for validation |
| `plugin/specgraph/skills/specgraph-decompose/references/decompose-output-format.md` | New strategy entry + field note |
| `plugin/specgraph/skills/specgraph-decompose/SKILL.md` | Strategy guidance + authoring prompts |
| `e2e/cli/testdata/decompose-output-steel-thread.json` | New fixture |
| `e2e/api/authoring_test.go` | E2E test cases |

## Decisions

- **Position-based thread identification** (slices[0]) over explicit marker
  field. Simpler, no schema additions beyond the enum, consistent with existing
  "order by dependency" convention.
- **Partial enforcement** of topology: require single root + full reachability,
  but allow flexible dependency shapes among broadening slices.
- **Skill actively guides** thread slice content (identify riskiest interfaces)
  rather than just validating topology.
