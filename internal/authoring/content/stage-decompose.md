# Stage: Decompose

Break the spec into independently deliverable, testable slices. Each slice
should be small enough to implement, test, and review in isolation.

The agent PROPOSES the decomposition. The user confirms, adjusts, or redirects.

---

## What Decompose Produces

A Decompose record contains a chosen strategy and a slice list. Most strategies
produce 2-5 slices. The **Single unit** strategy records 1 slice (the full spec
delivered as-is without decomposition). Each slice is an independently
executable unit of work with a clear verify condition and an explicit dependency
on any prior slices it requires.

---

## Elicitation Sequence

### 1. Strategy Selection

Recommend a decomposition strategy based on the spec's shape. Explain the
recommendation and ask the user to confirm.

| Strategy | When to recommend | Description |
|----------|-------------------|-------------|
| **Vertical slice** | User-facing features | Each slice delivers end-to-end value |
| **Horizontal layer** | Infrastructure work | Split by architecture tier (storage -> API -> UI) |
| **Steel thread** | Unproven interfaces, maximizing future parallelism | First slice proves riskiest integration points end-to-end; remaining slices broaden from it |
| **Single unit** | Small, self-contained work | Deliver the spec as-is without decomposition |

### 2. Slices

Propose 2-5 slices. Each slice has:

| Field | Description |
|-------|-------------|
| **id** | kebab-case identifier |
| **intent** | What this slice delivers |
| **verify** | How you know it is done |
| **touches** | Files/packages this slice creates or modifies |
| **dependsOn** | Which other slices must complete first |

### 3. Dependency Ordering

Present the dependency graph in plain language: "Slice A has no dependencies, so
it can start immediately. Slices B and C both depend on A and can run in
parallel."

### 4. Size Check

Evaluate each slice against the 1-4 hour target. If a slice feels bigger, push:
"Slice D feels bigger than 4 hours -- should we split it?"

---

## Steel Thread Specifics

When steel thread strategy is selected, guide the author through these steps:

1. **Identify risk:** Which integration points are least understood? Where could
   assumptions break? The thread slice must exercise these.
2. **Minimal cut:** Design the thread slice as the thinnest possible path through
   all layers. Its purpose is proving interfaces work, not delivering features.
3. **Verify contracts:** Thread slice verify criteria should focus on interface
   contracts -- "request round-trips through storage and back" not "all CRUD
   operations work."
4. **Fan-out:** Remaining slices broaden from the thread. Each adds depth or
   features using the now-proven interfaces. These can parallelize freely.

Example:

| Slice | Intent | Depends on |
|-------|--------|------------|
| `prove-roundtrip` | Proto-storage-handler-CLI round-trip for create | (none -- this is the thread) |
| `broaden-crud` | Add update and delete using proven interfaces | `prove-roundtrip` |
| `broaden-query` | Add list and filter operations | `prove-roundtrip` |

Note: `broaden-crud` and `broaden-query` can execute in parallel because they
both depend only on the thread, not on each other.

---

## Quality Signals

| Signal | Push |
|--------|------|
| More than 5 slices for a medium spec | "7 slices for a medium spec is a lot of coordination overhead. Can we merge [A] and [B]?" |
| Slice with no verify criteria | "How do you know this slice is done?" |
| All slices chain linearly | "Everything chains through slice-1. Is there anything that could start independently?" |
| Slice that is just "write tests" | "Tests should be part of each slice, not a separate slice." |
| Steel thread slice has feature-level verify criteria | "The thread slice should prove interfaces, not deliver features. Can we narrow the verify criteria to contract validation?" |

---

## Persistence Contract

When the decompose conversation is complete, synthesize the conversation into a
`DecomposeOutput` structure containing:

- `strategy` -- the chosen decomposition strategy
- `slices` -- array of objects with `id`, `intent`, `verify`, `touches`,
  `dependsOn`

Show the user: "Here's the decomposition I'm going to save: [summary]. Look
right?" Wait for confirmation before persisting.

Persist the Decompose output with the accumulated conversation exchanges —
they commit atomically with the stage output. Exchanges are REQUIRED for this
stage: include the full probe/response history from the decompose
conversation. Conversation recording is part of this step, not an optional
follow-up.

After persisting, confirm: "Decompose is saved. Want to continue to Approve?
I'll run through a review checklist."

---

## Next Stage

Approve -- the final quality gate before a spec is frozen for execution.
