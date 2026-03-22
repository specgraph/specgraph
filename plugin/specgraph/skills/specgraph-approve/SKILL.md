---
name: specgraph-approve
description: >
  Review and approve a spec for execution. Use when ready to sign off.
  Triggered by "approve", "looks good", "ready to implement", "sign off".
---

# SpecGraph Approve

Freeze a spec for execution. This is the last gate before an agent claims and
implements — the last chance to catch issues.

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
opinion on each.

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
   criteria, risks, current stage).
3. `specgraph deps <slug>` — load the dependency graph for completeness check.
4. `specgraph constitution show` — load the constitution for compliance check.
   Summarize to user: "Your project constitution has N principles and M
   constraints. Key ones for this spec: [relevant subset]."

### Resumption

If the spec is already at or past Approve stage:

1. Load via `specgraph show <slug>`.
2. Present summary of the current state.
3. Offer to revise or re-approve.

### Persistence

After the conversational review is complete and user confirms:

1. Synthesize the review into structured output.
2. Show summary: "Here's my assessment: [summary]. Ready to approve?"
3. User confirms or requests changes.
4. Run: `specgraph approve <slug>`
5. Confirm: "Approved. Spec is now frozen for execution."

### Post-Approval Options

After approval, offer the user next steps:

- Generate execution bundle: `/specgraph-bundle <slug>`
- Claim and start implementing: `specgraph claim <slug> --agent <name>`
- Leave it for someone else to claim
