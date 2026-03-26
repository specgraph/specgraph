---
name: specgraph-approve
description: >
  Review and approve a spec for execution. Use when ready to sign off.
  Triggered by "approve", "looks good", "ready to implement", "sign off".
---

# SpecGraph Approve

> **Human review gate.** This skill facilitates human review of specs before
> approval. The agent MUST NOT self-approve — approval is a human judgment call.
> The agent presents evidence, flags concerns, and runs checklists, but the human
> makes the approve/hold decision. If no human is present to review, the skill
> MUST NOT proceed with approval.

Freeze a spec for execution. This is the last gate before an agent claims and
implements — the last chance to catch issues.

---

## Guardrails

1. **No self-approval.** The agent that authored or contributed to a spec MUST
   NOT approve it. Approval must be performed by a human with review authority.
   If the current conversation authored this spec (spark through decompose),
   the agent MUST say: "I helped author this spec, so I can't approve it.
   Please have a human reviewer sign off on this approval."
2. **Present the full spec.** The agent MUST show the complete spec content
   (scope, contract, verify criteria, risks, dependencies) — not a summary.
   The reviewer needs to see exactly what they're approving.
3. **Explicit sign-off per checkpoint.** The agent MUST present each checklist
   item and wait for explicit human confirmation before moving to the next.
   Responses like "yes", "looks good", or "approved" are acceptable — but the
   agent MUST NOT assume approval or skip checkpoints.
4. **No approval offers.** The agent MUST NOT offer to approve on behalf of the
   user (e.g., "Want me to approve this?", "I'll go ahead and approve it").
   The agent facilitates; the human decides. The only acceptable prompt is:
   "All checkpoints reviewed. When you're ready, confirm approval and I'll
   record it."
5. **Record review provenance.** When approval is recorded, the agent MUST note
   who reviewed (human) and that the review was agent-facilitated.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

### Posture behavior during Approve

- **Drive:** Agent runs full checklist, presents a summary assessment with
  recommendation (approve/hold), waits for confirmation.
- **Partner:** Agent walks through checklist item by item, discusses each
  finding before moving on.
- **Support:** Agent asks user if they're satisfied with each area, adds
  observations and flags only significant concerns.

---

## Domain

### Purpose

Freeze the spec for execution. This is the final quality gate — the agent runs a
conversational review, not a mechanical checklist. It evaluates, states opinions,
and can say "I don't think this is ready."

### Conversational Checklist

Each item is a considered assessment, not a checkbox. The agent states a clear
opinion on each. **The agent MUST present each item individually and wait for
the human's explicit response before proceeding to the next item.** Do not
batch items or assume agreement.

1. **Scope bounded?** — Evaluate the in/out lists from Shape. State an opinion.
   - Good: "The scope looks solid — in/out are explicit and the boundaries make sense."
   - Concern: "The 'out' list is thin — I'd expect more exclusions for a spec this size."

2. **Interface defined?** — Check the contract from Specify for gaps.
   - Good: "The contract covers all CRUD operations with clear error semantics."
   - Concern: "The contract covers create and read, but I don't see error handling for duplicate slugs."

3. **Verify criteria testable?** — Assess each criterion individually.
   - Good: "All 6 criteria map to clear test assertions."
   - Concern: "Criterion 3 ('performs well') isn't testable — what's the latency threshold?"

4. **Dependencies mapped?** — Run `specgraph deps <slug>` and evaluate completeness.
   - Good: "Dependencies: [list]. These look complete."
   - Concern: "This spec touches auth middleware but doesn't depend on `auth-refactor` — should it?"

5. **Constitution compliance** — Load constitution and check each principle/constraint.
   - Good: "Checked against your constitution — no violations."
   - Concern: "Against your constitution: 'no external dependencies without review' — this spec adds Redis. Has that been reviewed?"

6. **Risk acknowledgment** — Review outstanding risks from Shape.
   - Good: "Risks from Shape: [list]. All have mitigations documented."
   - Concern: "Two risks have no mitigation strategy. Are these accepted as-is?"

### The Agent Can Say No

The agent never blocks, but it expresses strong opinions and can recommend
holding off on approval.

When recommending a hold, the agent explains exactly what needs to change:

> "I'd hold off on approving this. The verify criteria for concurrent rotation
> are vague — 'both succeed' doesn't specify what 'succeed' means when the
> tokens are in the same lineage. Can we tighten that before approving?"

If the user overrides: the concern is recorded in spec history, and approval
proceeds. The agent accepts gracefully and records the override as a decision
with rationale "author override."

---

## Execution

### Prerequisites

Run these before starting the review:

```bash
specgraph health
specgraph show <slug>
specgraph deps <slug>
specgraph constitution show
```

1. `specgraph health` — confirm the server is reachable.
2. `specgraph show <slug>` — load the full spec state (scope, contract, verify
   criteria, risks, current stage). **Present the full output to the reviewer —
   do not summarize.** The reviewer must see exactly what they're approving.
3. `specgraph deps <slug>` — load the dependency graph for completeness check.
   **Show the full dependency tree.**
4. `specgraph constitution show` — load the constitution for compliance check.
   Summarize to user: "Your project constitution has N principles and M
   constraints. Key ones for this spec: [relevant subset]."

### Resumption

If the spec is already at or past Approve stage:

1. Load via `specgraph show <slug>`.
2. Present summary of the current state.
3. Offer to revise or re-approve.

### Persistence

After ALL checklist items have been individually reviewed and the human has
confirmed each one:

1. Synthesize the review into structured output.
2. Present the final approval summary with all checkpoint results.
3. Ask: "All checkpoints reviewed. When you're ready, confirm approval and
   I'll record it." Do NOT offer to approve — wait for the human to state
   their decision.
4. If the human confirms approval, run: `specgraph approve <slug>`
5. Record provenance: who reviewed, agent-facilitated, any overrides noted.
6. Confirm: "Approved. Spec is now frozen for execution."

**If the human declines or wants changes:**

1. Record the conversation (see `references/conversation-recording.md`). Only
   record on hold/decline — approvals are self-evident.

   ```bash
   cat > /tmp/conv-<slug>.json << 'CONV_EOF'
   { "exchanges": [ ... review discussion + rejection rationale ... ] }
   CONV_EOF
   specgraph conversation record <slug> --stage approve --json-file /tmp/conv-<slug>.json
   rm /tmp/conv-<slug>.json
   ```

2. Record the hold reason, suggest which stage to revisit, and do NOT re-offer
   approval.

### Post-Approval Options

After approval, offer the user next steps:

- Generate execution bundle: `/specgraph-bundle <slug>`
- Claim and start implementing: `specgraph claim <slug> --agent <name>`
- Leave it for someone else to claim
