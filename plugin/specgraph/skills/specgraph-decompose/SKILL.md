---
name: specgraph-decompose
description: >
  Break a spec into independently deliverable, testable slices. Use when ready
  to split work into implementation chunks. Triggered by "break this down",
  "decompose", "split into slices", "work units", "break apart".
---

# SpecGraph Decompose

Break the spec into independently deliverable, testable slices. Each slice
should be small enough to implement, test, and review in isolation.

**Key principle:** You PROPOSE the decomposition. The user confirms, adjusts,
or redirects.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

### Posture behavior during Decompose

- **Drive:** Agent proposes strategy + all slices with deps, presents complete
  plan for approval.
- **Partner:** Agent proposes strategy, discusses, then proposes slices one at
  a time.
- **Support:** Agent asks user how they'd break it down, then refines and
  challenges.

---

## Domain

### Elicitation Sequence

#### 1. Strategy

RECOMMEND a decomposition strategy based on the spec's shape:

| Strategy | When to recommend | Description |
|----------|-------------------|-------------|
| **Vertical slice** | User-facing features | Each slice delivers end-to-end value |
| **Horizontal layer** | Infrastructure work | Split by architecture tier (storage -> API -> UI) |
| **Steel thread** | Unproven interfaces, maximizing future parallelism | First slice proves riskiest integration points end-to-end; remaining slices broaden from it |
| **Single unit** | Small, self-contained work | Deliver the spec as-is without decomposition |

Explain why you recommend one and ask the user to confirm.

#### 2. Slices

PROPOSE 2-5 slices. Each slice has:

| Field | Description |
|-------|-------------|
| **Id** | kebab-case identifier |
| **Intent** | What this slice delivers |
| **Verify** | How you know it's done |
| **Touches** | Files/packages this slice creates or modifies |
| **Depends on** | Which other slices must complete first |

#### 3. Dependency Ordering

Present the dependency graph in plain language: "Slice A has no dependencies, so
it can start immediately. Slices B and C both depend on A and can run in
parallel."

#### Steel Thread Guidance

When steel thread strategy is selected, guide the author through these steps:

1. **Identify risk:** Which integration points are least understood? Where could
   assumptions break? The thread slice must exercise these.
2. **Minimal cut:** Design the thread slice as the thinnest possible path through
   all layers. Its purpose is proving interfaces work, not delivering features.
3. **Verify contracts:** Thread slice `verify` criteria should focus on interface
   contracts -- "request round-trips through storage and back" not "all CRUD
   operations work."
4. **Fan-out:** Remaining slices broaden from the thread. Each adds depth or
   features using the now-proven interfaces. These can parallelize freely.

**Example:**

| Slice | Intent | Depends on |
|-------|--------|------------|
| `prove-roundtrip` | Proto-storage-handler-CLI round-trip for create | (none -- this is the thread) |
| `broaden-crud` | Add update and delete using proven interfaces | `prove-roundtrip` |
| `broaden-query` | Add list and filter operations | `prove-roundtrip` |

Note: `broaden-crud` and `broaden-query` can execute in parallel because they
both depend only on the thread, not on each other.

#### 4. Size Check

Evaluate each slice against the 1-4 hour target. If a slice feels bigger, push:
"Slice D feels bigger than 4 hours — should we split it?"

### Quality Heuristics

Apply these throughout the conversation. Push back when they fire.

| Signal | Push |
|--------|------|
| More than 5 slices for a medium spec | "7 slices for a medium spec is a lot of coordination overhead. Can we merge [A] and [B]?" |
| Slice with no verify criteria | "How do you know this slice is done?" |
| All slices chain linearly | "Everything chains through slice-1. Is there anything that could start independently?" |
| Slice that's just "write tests" | "Tests should be part of each slice, not a separate slice." |
| Steel thread slice has feature-level verify criteria | "The thread slice should prove interfaces, not deliver features. Can we narrow the verify criteria to contract validation?" |

---

## Execution

### Prerequisites

```bash
specgraph health
specgraph constitution show
specgraph show <slug>
```

Load and summarize the constitution, then load the spec — especially Specify
output for contract details.

### Resumption

If the spec is already at or past Decompose:

1. Load via `specgraph show <slug>`.
2. Present the existing decomposition summary.
3. Offer to revise or continue to Approve.

### Persistence

1. Synthesize DecomposeOutput JSON containing:
   - `strategy` — the chosen decomposition strategy
   - `slices` — array of objects with `id`, `intent`, `verify`, `touches`, `dependsOn`
2. Show the user: "Here's the decomposition I'm going to save: [summary]. Look right?"
3. User confirms or tweaks.
4. Write temp file, call CLI:

```bash
specgraph decompose <slug> --json-file <tmpfile>
```

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   {
     "exchanges": [
       {"role": "probe", "content": "...", "stage": "decompose", "sequence": 1},
       {"role": "response", "content": "...", "stage": "decompose", "sequence": 1},
       ...
     ]
   }
   CONV_EOF
   specgraph conversation record <slug> --stage decompose --json-file "$CONV_TMP"
   ```

### Analytical Passes

After persisting decompose output, run analytical passes per the shared protocol.

> **Read `references/analytical-passes.md`** for the full dispatch-collate-present
> protocol, including posture-aware behavior and severity-based gating.

Passes for Decompose stage:

- **Drive:** `simplicity` + `constitution-check` (auto-run both).
- **Partner:** `constitution-check` (auto-run). Offer `simplicity`.
- **Support:** `constitution-check` (auto-run). Offer `simplicity`.

Dispatch all auto-run passes as parallel background subagents. Wait for
completion, then present findings per the protocol before offering to
continue to Approve.

6. Confirm: "Saved. Spec is now at Decompose."

### Transition

Offer to continue: "Decompose is saved. Want to continue to Approve? I'll run
through a review checklist."

User controls whether to proceed or stop.
