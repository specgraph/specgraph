# Stage: Approve

Freeze a spec for execution. This is the final quality gate -- the last chance
to catch issues before an agent claims and implements.

**Human review gate.** The agent MUST NOT self-approve. Approval is a human
judgment call. The agent presents evidence, flags concerns, and runs the
checklist -- but the human makes the approve/reject decision. If no human is
present to review, the agent MUST NOT proceed with approval.

---

## Self-Approval Prohibition

The agent that authored or contributed to a spec MUST NOT approve it. If the
current conversation authored this spec (spark through decompose), the agent
MUST say: "I helped author this spec, so I can't approve it. Please have a
human reviewer sign off."

The agent MUST NOT offer to approve on behalf of the user (e.g., "Want me to
approve this?"). The only acceptable prompt after all checkpoints are reviewed
is: "All checkpoints reviewed. When you're ready, confirm approval and I'll
record it."

---

## Conversational Checklist

Each item is a considered assessment, not a checkbox. The agent states a clear
opinion on each. Present each item individually and wait for the human's
explicit response before proceeding to the next. Do not batch items or assume
agreement.

1. **Scope bounded?** -- Evaluate the in/out lists from Shape. State an opinion.
   - Good: "The scope looks solid -- in/out are explicit and the boundaries make
     sense."
   - Concern: "The 'out' list is thin -- I'd expect more exclusions for a spec
     this size."

2. **Interface defined?** -- Check the contract from Specify for gaps.
   - Good: "The contract covers all CRUD operations with clear error semantics."
   - Concern: "The contract covers create and read, but I don't see error
     handling for duplicate slugs."

3. **Verify criteria testable?** -- Assess each criterion individually.
   - Good: "All 6 criteria map to clear test assertions."
   - Concern: "Criterion 3 ('performs well') isn't testable -- what's the
     latency threshold?"

4. **Dependencies mapped?** -- Evaluate the dependency graph for completeness.
   - Good: "Dependencies: [list]. These look complete."
   - Concern: "This spec touches auth middleware but doesn't depend on
     `auth-refactor` -- should it?"

5. **Constitution compliance** -- Check each principle and constraint.
   - Good: "Checked against your constitution -- no violations."
   - Concern: "Against your constitution: 'no external dependencies without
     review' -- this spec adds Redis. Has that been reviewed?"

6. **Risk acknowledgment** -- Review outstanding risks from Shape.
   - Good: "Risks from Shape: [list]. All have mitigations documented."
   - Concern: "Two risks have no mitigation strategy. Are these accepted as-is?"

---

## The Agent Can Say No

The agent never blocks, but it expresses strong opinions and can recommend
holding off on approval.

When recommending a hold, the agent explains exactly what needs to change:

> "I'd hold off on approving this. The verify criteria for concurrent rotation
> are vague -- 'both succeed' doesn't specify what 'succeed' means when the
> tokens are in the same lineage. Can we tighten that before approving?"

If the user overrides: the concern is recorded in spec history, and approval
proceeds. The agent accepts gracefully and records the override as a decision
with rationale "author override."

---

## Persistence Contract

### Accept path

After ALL checklist items have been individually reviewed and the human has
confirmed each one, present the final approval summary and ask: "All checkpoints
reviewed. When you're ready, confirm approval and I'll record it."

When the human confirms, persist the approval with the accept disposition.
Exchanges capturing the approval rationale and the checklist discussion are
REQUIRED on the accept path — they commit atomically with the accept
disposition and are load-bearing for audit. Do NOT omit exchanges on a clean
acceptance. Record provenance: who reviewed, that the review was
agent-facilitated, and any overrides noted.

### Reject path

If the human declines or requests changes, persist the approval with the
reject disposition. Exchanges capturing the rejection reason and the checklist
discussion are REQUIRED on the reject path — they commit atomically with the
rejection and are load-bearing for audit. Do NOT omit exchanges on rejection.

After recording a rejection: note the hold reason, suggest which stage to
revisit, and do NOT re-offer approval.

---

## Next Stage

A spec that reaches Approve and is accepted is frozen for execution. It may be
claimed and implemented by an execution agent. A rejected spec returns to the
appropriate earlier stage for revision.
