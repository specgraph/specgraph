# Stage: Specify

Make the spec precise enough to implement and test. Define contracts, not code.
After Specify, the spec is testable -- every success criterion from Shape has a
concrete verification assertion, every interface has defined inputs, outputs, and
error conditions.

The agent DRAFTS all technical detail based on Shape output. The user confirms,
tweaks, or redirects -- they never author contracts from scratch.

---

## What Specify Produces

A Specify record answers four questions:

1. **Interface contract** -- What are the precise inputs, outputs, and error
   conditions for each boundary this spec defines?
2. **Verify criteria** -- For each must/should success criterion from Shape, what
   is the concrete, automatable test assertion?
3. **Invariants** -- What system-level guarantees must hold forever, independent
   of this spec's verification criteria?
4. **Touches** -- What files and packages will change as a result of implementing
   this spec?

---

## Elicitation Sequence

Work through all four in order. Draft one section at a time -- do not dump a
full template at the start. Each section is a conversation, not a form.

### 1. Interface Contract

The agent DRAFTS based on Shape output -- API endpoints, function signatures,
inputs, outputs, status codes, error conditions.

Present to the user: "Based on what we shaped, the interface would look like:
[draft]. Does that match what you're thinking?"

- Define inputs with types and constraints.
- Define outputs with success and error shapes.
- Define error conditions explicitly -- every failure mode the caller can
  encounter.
- Define status codes or error types for each failure mode.
- If the Shape chose an approach with integration points, define the contract for
  each boundary.

### 2. Verify Criteria

The agent PROPOSES test assertions for each success criterion from Shape.

Present to the user: "For each must-have, here's how I'd test it: [assertions].
Anything to add?"

- Map each `successMust` and `successShould` from Shape to one or more concrete
  test assertions.
- Each assertion should be automatable -- no "manually verify" language.
- Include both happy-path and error-path assertions.

### 3. Invariants

The agent PROPOSES system-level guarantees that must hold forever.

Present to the user: "An invariant holds forever. A verify criterion is a test
for this spec. Here's what I'd propose: [invariants]."

- Invariants are properties of the system, not tests for this spec.
- Example invariant: "Auth tokens never exceed 4096 bytes."
- Example verify criterion (not an invariant): "The new endpoint returns 200 for
  valid input."
- If the user proposes something as an invariant that is really a verify
  criterion, push back and clarify the distinction.

### 4. Touches

The agent PROPOSES files and packages that will change, based on codebase
analysis.

Present to the user: "Based on the interface, I'd expect these files to change:
[list]."

- Group by: new files, modified files, test files.
- Flag files that are also touched by other in-progress specs (collision risk).

---

## Quality Signals

| Signal | Problem | Pushback |
|--------|---------|----------|
| Verify criteria that restate the contract | No interesting test coverage | "That restates the contract. The verify criterion should test the interesting case -- what about concurrent requests? Expired tokens?" |
| Missing error conditions | Incomplete interface contract | "Happy path is defined. What about: invalid input, auth failure, conflict, timeout?" |
| Invariants that are really verify criteria | Confused scope | "Is this 'must hold forever' or 'must pass this test'?" |
| No touches identified | Disconnected from codebase | "Every spec changes something. What files does this touch?" |
| Overlapping touches with other specs | Collision risk | "Spec `[other-slug]` also modifies `[file]` -- your invariants should be compatible with theirs." |

---

## Background Research

At the start of the specify conversation, dispatch background research:

1. **Dependency scan** -- check spec dependencies for invariant consistency
   across related specs.
2. **Codebase scan** -- grep for files, packages, and interfaces that the spec
   will touch. Note existing patterns and test coverage.
3. **Graph scan** -- look for specs with overlapping touches or shared invariants.

Surface findings when relevant -- don't block the conversation waiting for
results.

---

## Persistence Contract

When the specify conversation is complete, synthesize the conversation into a
`SpecifyOutput` structure:

```json
{
  "interfaces": [
    {
      "name": "ClaimService proto",
      "body": "POST /api/v1/specs/{slug}/claim\n  Input: { agent_id: string, ttl_seconds: int }\n  Output: { lease_id: string, expires_at: timestamp }\n  Errors:\n    404 - Spec not found\n    409 - Already claimed\n    422 - Invalid TTL"
    }
  ],
  "verifyCriteria": [
    {"category": "happy-path", "description": "POST /claim with valid agent_id returns 200 and a lease_id"},
    {"category": "conflict", "description": "POST /claim on already-claimed spec returns 409"},
    {"category": "expiry", "description": "Lease expires after TTL seconds and spec becomes claimable again"}
  ],
  "invariants": [
    "A spec may have at most one active lease at any time",
    "Lease expiry is monotonically increasing (no backdating)"
  ],
  "touches": [
    {"path": "internal/server/claim_handler.go", "purpose": "new claim handler", "changeType": "new"},
    {"path": "internal/storage/lease.go", "purpose": "lease domain types", "changeType": "new"},
    {"path": "internal/server/claim_handler_test.go", "purpose": "handler tests", "changeType": "new"}
  ]
}
```

Show the user a human-readable summary and wait for their confirmation before
persisting.

Persist the Specify output with the accumulated conversation exchanges — they
commit atomically with the stage output. Exchanges are REQUIRED for this
stage: include the full probe/response history from the specify conversation.
Conversation recording is part of this step, not an optional follow-up.

After persisting, confirm: "Specify is saved. Want to continue to Decompose? I
can propose how to break this into slices."

---

## Next Stage

Decompose -- breaks the spec into independently deliverable, testable slices
with explicit dependency ordering.
