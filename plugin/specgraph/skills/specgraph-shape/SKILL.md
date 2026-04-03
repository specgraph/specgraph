---
name: specgraph-shape
description: >
  Shape a spec by bounding scope, exploring approaches, capturing decisions, and
  surfacing risks. Use when the user wants to design or plan. Triggered by
  "let's design...", "scope this out", "what's the approach for...", or "shape".
---

# SpecGraph Shape

Turn the seed into a bounded proposal with explicit tradeoffs. This is where most
design work happens — scope gets bounded, approaches get weighed, decisions get
recorded, and risks get surfaced.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

### Posture behavior during Shape

- **Drive:** Agent drafts all sections (scope, approaches, decisions, success
  criteria, risks), presents each for approval. Runs analytical passes
  automatically and weaves findings into proposals.
- **Partner:** Agent proposes each section, discusses with user before
  finalizing. Surfaces graph/codebase findings as discussion points.
- **Support:** Agent asks user what they think scope should be, then fills gaps
  and challenges where needed. Offers to draft sections when user seems stuck.

---

## Domain: The Shaping Conversation

Shape is the most design-intensive stage of the authoring funnel. The agent walks
through five elicitation moves — scope, approaches, decisions, success criteria,
and risks — as a structured conversation, not a form-filling exercise.

### Elicitation Sequence

Work through all five in order. Each is a conversation, not a single prompt.

#### 1. Scope In / Out

Agent PROPOSES scope based on the seed from Spark. Pushes for an explicit "out"
list — this is where scope creep dies.

- Load the seed from Spark stage (title, signal, scope sniff, unknowns).
- Draft an "in scope" list and an "out of scope" list.
- Present both lists to the user for discussion.
- Push hard on the "out" list. An empty "out" list is an unbounded spec.

#### 2. Approaches

Agent GENERATES 2-3 approaches with tradeoffs and its recommendation. The user
does not have to come up with approaches — that is the agent's job.

- For each approach: name, description, tradeoffs (what you gain, what you lose).
- State which approach the agent recommends and why.
- Discuss with the user until one is chosen (or a hybrid emerges).
- If the user only considers one approach, push: "What's the alternative you
  rejected? Even if it's obviously worse, naming it sharpens the rationale."

#### 3. Decision Capture

For each significant choice made during scoping and approach selection, propose a
decision record.

- Each decision has: slug, title, decision (what was chosen), rationale (why).
- Decisions are first-class graph nodes (ADR-003) with bidirectional edges to the
  spec.
- Capture both the chosen option and what was explicitly rejected.
- If the user overrode the agent's recommendation, record rationale as "author
  override" with the user's reasoning.

#### 4. Success Criteria

Agent DRAFTS Must/Should/Won't criteria based on the discussion so far. User
confirms or adjusts.

- **Must:** Non-negotiable. The spec fails without these.
- **Should:** Expected. Absence needs justification.
- **Won't:** Explicitly excluded from this spec (may become future specs).
- Every criterion should be testable. If it isn't, push: "How would you verify
  that in a test?"

#### 5. Risks

Agent surfaces risks proactively based on the approaches and scope. Asks user to
confirm or add.

- Technical risks (performance, complexity, integration).
- Operational risks (deployment, migration, rollback).
- Business risks (timeline, dependency on external teams).
- Capture each risk as a concise string suitable for `risks: string[]` in the persisted output.

### Quality Heuristics

Apply these throughout the conversation — they are red flags that require
pushback:

| Signal | Problem | Pushback |
|--------|---------|----------|
| Empty "out" list | Unbounded scope | "Everything has scope boundaries. What are you consciously not doing?" |
| Only one approach considered | No tradeoff analysis | "What's the alternative you rejected? Even if it's obviously worse, naming it sharpens the rationale." |
| Untestable success criteria | Ambiguous acceptance | "How would you verify that in a test?" |
| Scope sniff jumped from Spark estimate | Scope creep during shaping | "In Spark this was estimated at [X]. It's now looking like [Y]. Should we split, or has the understanding genuinely changed?" |
| No risks identified | Blind optimism | "Every design has risks. What's the thing most likely to bite you during implementation?" |
| Constitution violation | Conflicting with project ground truth | "Your constitution says '[principle].' This seems to conflict — how do you want to reconcile?" |

### Analytical Pass (Peripheral Vision)

Woven into the conversation, not a separate step. While scoping and discussing
approaches, proactively surface related concerns from the graph and codebase.

- Check `specgraph list` for related or overlapping specs.
- Grep the codebase for files and packages that the spec will likely touch.
- Surface findings naturally: "I checked the graph — there's already a spec for X
  that touches the same files."
- When a related concern surfaces, ask how to disposition it:
  - **Fold in:** Add it to this spec's scope.
  - **Track separately:** Create a new spec or link to an existing one.
  - **Note for implementer:** Add as context, not scope.

Example: "This touches the auth module — I see `token-encryption-at-rest` is also
in progress there. Collision risk?"

### Background Research

At the start of the shaping conversation, dispatch background research:

1. **Graph scan:** `specgraph list` — look for specs with
   overlapping scope, shared dependencies, or touching the same domain area.
2. **Codebase scan:** Grep for files, packages, and modules that the spec's scope
   is likely to touch. Note existing patterns, interfaces, and test coverage.
3. **Constitution check:** Identify principles and constraints relevant to the
   scope being discussed.

Surface findings naturally when they arrive — don't block the conversation
waiting for results. Example: "I checked the graph — there's already a spec for X
that touches the same files. Want to factor that in?"

**Important:** Background agent completions are NOT user input. When a background
agent returns, fold its results into the current discussion but do NOT treat the
notification as user approval of the current step.

---

## Execution

### Prerequisites

Run these before starting the shaping conversation:

```bash
# 1. Verify server is reachable
specgraph health

# 2. Load constitution — summarize relevant principles/constraints to user
specgraph constitution show

# 3. Load current spec state
specgraph show <slug>
```

After loading the constitution, summarize to the user: "Your project constitution
has N principles and M constraints. Key ones for this spec: [relevant subset]."

If the spec is already at or past Shape stage, present a summary of existing shape
data and offer to revise or continue to the next stage.

### Shaping Conversation

Walk through the elicitation sequence above. The conversation structure depends on
the detected posture, but the five moves (scope, approaches, decisions, success
criteria, risks) are always completed.

During the conversation, run background research as described in the Domain
section. Surface findings when relevant — don't wait until the end.

#### Step Gating (CRITICAL)

Each elicitation step MUST be explicitly approved by the user before advancing to
the next. Background agent notifications (e.g., codebase scan results) are NOT
user approval — they are supplementary context to weave into the current step.

- After presenting scope in/out: WAIT for user to confirm, adjust, or discuss.
- After presenting approaches: WAIT for user to choose or discuss.
- After presenting decisions: WAIT for user to confirm or revise.
- After presenting success criteria: WAIT for user to confirm or adjust.
- After presenting risks: WAIT for user to confirm or add.

Use `AskUserQuestion` when presenting each step for approval. If a background
agent returns results while waiting, incorporate the findings into the current
step's presentation but do NOT advance to the next step.

### Persistence

When the shaping conversation is complete:

1. **Synthesize** the conversation into a ShapeOutput JSON structure:

   ```json
   {
     "scopeIn": ["item 1", "item 2"],
     "scopeOut": ["item 1", "item 2"],
     "approaches": [
       {
         "name": "approach-a",
         "description": "...",
         "tradeoffs": ["Pro or con as a string"]
       }
     ],
     "chosenApproach": "approach-a",
     "risks": ["Risk description as string"],
     "successMust": ["criterion 1"],
     "successShould": ["criterion 2"],
     "successWont": ["criterion 3"],
     "decisions": [
       {
         "slug": "decision-slug",
         "title": "Decision Title",
         "decision": "What was chosen",
         "rationale": "Why"
       }
     ]
   }
   ```

2. **Show the user** a human-readable summary: "Here's what I'm going to save to
   the graph: [summary]. Look right?"

3. **Wait for confirmation.** User confirms or requests changes. Iterate until
   they approve.

4. **Write and persist:** Read `references/shape-output-format.md` for the
   exact JSON schema. Use only ASCII characters — no em dashes or Unicode.

   ```bash
   # Write synthesized output to temp file (see references/shape-output-format.md for schema)
   cat > /tmp/shape-<slug>.json << 'SHAPE_EOF'
   { ... }
   SHAPE_EOF

   # Persist to the graph
   specgraph shape <slug> --json-file /tmp/shape-<slug>.json
   ```

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   CONV_TMP="$(mktemp /tmp/conv-XXXXXX.json)"
   trap 'rm -f "$CONV_TMP"' EXIT
   cat > "$CONV_TMP" << 'CONV_EOF'
   {
     "exchanges": [
       {"role": "probe", "content": "...", "stage": "shape", "sequence": 1},
       {"role": "response", "content": "...", "stage": "shape", "sequence": 1},
       ...
     ]
   }
   CONV_EOF
   specgraph conversation record <slug> --stage shape --json-file "$CONV_TMP"
   ```

6. **Confirm:** "Saved. Spec is now at Specify stage."

### Stage Transition

After persisting, offer to continue:

"Shape is saved. Want to continue to Specify? I can draft the interface contract
based on what we just shaped."

- Continue to **Specify** → `/specgraph-specify <slug>`
- Or stop here — the spec is saved at Shape stage and can be resumed later.
