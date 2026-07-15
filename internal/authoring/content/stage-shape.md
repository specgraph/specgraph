# Stage: Shape

Turn the seed into a bounded proposal with explicit tradeoffs. This is where
most design work happens — scope gets bounded, approaches get weighed, decisions
get recorded, and risks get surfaced.

Shape is the most design-intensive stage of the authoring funnel. The agent
walks through five elicitation moves as a structured conversation, not a
form-filling exercise.

---

## What Shape Produces

A Shape record answers: what is in scope, what is explicitly out, which approach
was chosen and why, what decisions were made, what success looks like, and what
risks are acknowledged.

---

## Elicitation Sequence

Work through all five in order. Each is a conversation, not a single prompt.

### 1. Scope In / Out

The agent PROPOSES scope based on the seed from Spark. Pushes for an explicit
"out" list — this is where scope creep dies.

- Load the seed from Spark (title, signal, scope sniff, unknowns).
- Draft an "in scope" list and an "out of scope" list.
- Present both lists for discussion.
- Push hard on the "out" list. An empty "out" list is an unbounded spec.

### 2. Approaches

The agent GENERATES 2-3 approaches with tradeoffs and its recommendation. The
user does not have to come up with approaches — that is the agent's job.

- For each approach: name, description, tradeoffs (what you gain, what you
  lose).
- State which approach the agent recommends and why.
- Discuss until one is chosen (or a hybrid emerges).
- If the user only considers one approach, push: "What's the alternative you
  rejected? Even if it's obviously worse, naming it sharpens the rationale."

### 3. Decision Capture

For each significant choice made during scoping and approach selection, propose
a decision record.

- Each decision has: `slug`, `title`, `decision` (what was chosen), `rationale`
  (why).
- Decisions are first-class graph nodes (ADR-003) with bidirectional edges to
  the spec. They are Decision nodes, not inline string annotations.
- Capture both the chosen option and what was explicitly rejected.
- If the user overrode the agent's recommendation, record rationale as "author
  override" with the user's reasoning.

### 4. Success Criteria

The agent DRAFTS Must / Should / Won't criteria based on the discussion so far.
User confirms or adjusts.

- **Must:** Non-negotiable. The spec fails without these.
- **Should:** Expected. Absence needs justification.
- **Won't:** Explicitly excluded from this spec (may become future specs).
- Every criterion should be testable. If it isn't, push: "How would you verify
  that in a test?"

### 5. Risks

The agent surfaces risks proactively based on the approaches and scope. Asks
the user to confirm or add.

- Technical risks (performance, complexity, integration).
- Operational risks (deployment, migration, rollback).
- Business risks (timeline, dependency on external teams).
- Capture each risk as a concise string.

---

## Step Gating (Critical)

Each elicitation step MUST be explicitly approved by the user before advancing
to the next. Background research results (graph scans, codebase findings) are
NOT user approval — they are supplementary context to weave into the current
step.

- After presenting scope in/out: wait for user to confirm, adjust, or discuss.
- After presenting approaches: wait for user to choose or discuss.
- After presenting decisions: wait for user to confirm or revise.
- After presenting success criteria: wait for user to confirm or adjust.
- After presenting risks: wait for user to confirm or add.

Do not advance to the next step on the basis of a background result arriving.

---

## Quality Signals

| Signal | Problem | Pushback |
|--------|---------|----------|
| Empty "out" list | Unbounded scope | "Everything has scope boundaries. What are you consciously not doing?" |
| Only one approach considered | No tradeoff analysis | "What's the alternative you rejected? Even if it's obviously worse, naming it sharpens the rationale." |
| Untestable success criteria | Ambiguous acceptance | "How would you verify that in a test?" |
| Scope sniff jumped from Spark estimate | Scope creep during shaping | "In Spark this was estimated at [X]. It's now looking like [Y]. Should we split, or has the understanding genuinely changed?" |
| No risks identified | Blind optimism | "Every design has risks. What's the thing most likely to bite you during implementation?" |
| Constitution violation | Conflicting with project ground truth | "Your constitution says '[principle].' This seems to conflict -- how do you want to reconcile?" |

---

## Background Research

At the start of the shaping conversation, dispatch background research in
parallel with the conversation:

1. **Graph scan** -- look for specs with overlapping scope, shared dependencies,
   or touching the same domain area.
2. **Codebase scan** -- grep for files, packages, and modules the spec's scope
   is likely to touch. Note existing patterns, interfaces, and test coverage.
3. **Constitution check** -- identify principles and constraints relevant to the
   scope being discussed.

Surface findings naturally when they arrive -- don't block the conversation
waiting for results. Example: "I checked the graph -- there's already a spec for
X that touches the same files. Want to factor that in?"

Background agent completions are NOT user input. When a background result
arrives, fold it into the current discussion but do NOT treat it as user
approval of the current step.

---

## Persistence Contract

When the shaping conversation is complete, persist the Shape output with the
`author` tool (`action: shape`). The `output` argument is **friendly
snake_case YAML** — the same shape you show the user, no translation step. Use
these keys verbatim; do NOT camelCase them (`scopeIn`, `chosenApproach`,
`successMust` are rejected by the parser):

```yaml
scope_in:
  - "capability explicitly included"
scope_out:
  - "capability explicitly excluded"
approaches:
  - name: "approach-a"
    description: "how it would work"
    tradeoffs:
      - "what you gain / what you lose"
chosen_approach: "approach-a"
risks:
  - "risk that could affect delivery"
success_must:
  - "non-negotiable outcome"
success_should:
  - "expected but not strictly required outcome"
success_wont:
  - "explicitly excluded outcome"
decisions:
  - slug: "decision-slug"
    title: "Decision Title"
    decision: "what was chosen"
    rationale: "why"
```

Show the user a human-readable summary and wait for their confirmation before
persisting.

Pass the accumulated conversation `exchanges` alongside the `output` on the
same `author` call — they commit atomically with the stage output. `exchanges`
is a **JSON array** and is REQUIRED for this stage (the server enforces at
least one exchange for shape). Include the full probe/response history from the
shaping conversation; conversation recording is part of this step, not an
optional follow-up:

```json
[
  { "role": "probe",    "content": "What is explicitly out of scope?", "stage": "shape", "sequence": 1 },
  { "role": "response", "content": "Anything touching billing.",       "stage": "shape", "sequence": 2 }
]
```

After persisting, confirm: "Shape is saved. Want to continue to Specify? I can
draft the interface contract based on what we just shaped."

---

## Next Stage

Specify -- makes the spec precise and testable: interface contracts, verification
criteria, invariants, and file touches.
