---
name: specgraph-specify
description: >
  Define interface contracts, verification criteria, invariants, and touches.
  Use when the spec is shaped and ready for precise technical definition.
  Triggered by "define the interface", "acceptance criteria", "specify",
  "write the contract", or "make it testable".
---

# SpecGraph Specify

Make the spec precise enough to implement and test. Define contracts, not code.
After Specify, the spec is testable — every success criterion from Shape has a
concrete verification assertion, every interface has defined inputs, outputs, and
error conditions.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

### Posture behavior during Specify

- **Drive:** Agent drafts everything from Shape output — interface contract,
  verify criteria, invariants, touches — and presents the complete Specify
  output for review. Runs analytical passes automatically.
- **Partner:** Agent drafts each section, pauses for input and discussion
  before moving on to the next.
- **Support:** Agent asks the user to describe contracts in their own words,
  then refines, fills gaps, and challenges.

---

## Domain: The Specify Conversation

Specify turns the shaped proposal into precise, testable contracts. The agent
DRAFTS all technical detail based on Shape output. The user confirms, tweaks, or
redirects — they never author contracts from scratch.

The key principle: after Specify, a spec is testable. Every success criterion has
a verification assertion. Every interface has defined inputs, outputs, and error
conditions.

### Elicitation Sequence

Work through all four in order. Each is a conversation, not a single prompt.

#### 1. Interface Contract

Agent DRAFTS based on Shape output — API endpoints, function signatures, inputs,
outputs, status codes, error conditions. Presents to user:

"Based on what we shaped, the interface would look like: [draft]. Does that match
what you're thinking?"

- Define inputs with types and constraints.
- Define outputs with success and error shapes.
- Define error conditions explicitly — every failure mode the caller can
  encounter.
- Define status codes or error types for each failure mode.
- If the Shape chose an approach with integration points, define the contract for
  each boundary.

#### 2. Verify Criteria

Agent PROPOSES test assertions for each success criterion from Shape. Presents to
user:

"For each must-have, here's how I'd test it: [assertions]. Anything to add?"

- Map each `success_must` and `success_should` from Shape to one or more
  concrete test assertions.
- Each assertion should be automatable — no "manually verify" language.
- Include both happy-path and error-path assertions.

#### 3. Invariants

Agent PROPOSES system-level guarantees that must hold forever. Presents to user:

"An invariant holds forever. A verify criterion is a test for this spec. Here's
what I'd propose: [invariants]."

- Invariants are properties of the system, not tests for this spec.
- Example invariant: "Auth tokens never exceed 4096 bytes."
- Example verify criterion (not an invariant): "The new endpoint returns 200 for
  valid input."
- Distinguish clearly. If the user proposes an invariant that is really a verify
  criterion, push back.

#### 4. Touches

Agent PROPOSES files and packages that will change, based on codebase analysis.
Presents to user:

"Based on the interface, I'd expect these files to change: [list]."

- Grep the codebase for related code — packages, interfaces, test files.
- Group by: new files, modified files, test files.
- Flag files that are also touched by other in-progress specs (collision risk).

### Quality Heuristics

Apply these throughout the conversation — they are red flags that require
pushback:

| Signal | Problem | Pushback |
|--------|---------|----------|
| Verify criteria that restate the contract | No interesting test coverage | "That restates the contract. The verify criterion should test the interesting case — what about concurrent requests? Expired tokens?" |
| Missing error conditions | Incomplete interface contract | "Happy path is defined. What about: invalid input, auth failure, conflict, timeout?" |
| Invariants that are really verify criteria | Confused scope | "Is this 'must hold forever' or 'must pass this test'?" |
| No touches identified | Disconnected from codebase | "Every spec changes something. What files does this touch?" |
| Overlapping touches with other specs | Collision risk | "Spec `[other-slug]` also modifies `[file]` — your invariants should be compatible with theirs." |

### Background Research

At the start of the specify conversation, dispatch background research:

1. **Dependency scan:** `specgraph deps <slug>` — check dependencies for
   invariant consistency across related specs.
2. **Codebase scan:** Grep for files, packages, and interfaces that the spec
   will touch. Note existing patterns and test coverage.
3. **Graph scan:** Look for specs with overlapping touches or shared invariants.

Surface findings when relevant — don't block the conversation waiting for
results.

---

## Execution

### Prerequisites

Run these before starting the specify conversation:

```bash
# 1. Verify server is reachable
specgraph health

# 2. Load constitution — summarize relevant principles/constraints to user
specgraph constitution show

# 3. Load current spec state (especially Shape output)
specgraph show <slug>

# 4. Check dependencies for invariant consistency
specgraph deps <slug>
```

After loading the constitution, summarize to the user: "Your project constitution
has N principles and M constraints. Key ones for this spec: [relevant subset]."

If the spec is already at or past Specify stage, present a summary of existing
specify data and offer to revise or continue to the next stage.

### Specify Conversation

Walk through the elicitation sequence above. The conversation structure depends
on the detected posture, but the four moves (interface contract, verify criteria,
invariants, touches) are always completed.

During the conversation, run background research as described in the Domain
section. Surface findings when relevant — don't wait until the end.

### Persistence

When the specify conversation is complete:

1. **Synthesize** the conversation into a SpecifyOutput JSON structure:

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
       {"path": "internal/storage/memgraph/lease_queries.go", "purpose": "Cypher queries for leases", "changeType": "new"},
       {"path": "internal/server/claim_handler_test.go", "purpose": "handler tests", "changeType": "new"}
     ]
   }
   ```

2. **Show the user** a human-readable summary: "Here's what I'm going to save to
   the graph: [summary]. Look right?"

3. **Wait for confirmation.** User confirms or requests changes. Iterate until
   they approve.

4. **Write and persist:**

   ```bash
   # Write synthesized output to temp file
   cat > /tmp/specify-<slug>.json << 'SPECIFY_EOF'
   { ... }
   SPECIFY_EOF

   # Persist to the graph
   specgraph specify <slug> --json-file /tmp/specify-<slug>.json
   ```

5. **Record the conversation:** See `references/conversation-recording.md` for the exchange format.

   ```bash
   cat > /tmp/conv-<slug>.json << 'CONV_EOF'
   { "exchanges": [ ... accumulated probe/response exchanges ... ] }
   CONV_EOF
   specgraph conversation record <slug> --stage specify --json-file /tmp/conv-<slug>.json
   rm /tmp/conv-<slug>.json
   ```

### Analytical Passes

After persisting specify output, run analytical passes per the shared protocol.

> **Read `references/analytical-passes.md`** for the full dispatch-collate-present
> protocol, including posture-aware behavior and severity-based gating.

Passes for Specify stage:

- **Drive:** `red-team` + `consistency` + `constitution-check` (auto-run all).
- **Partner:** `constitution-check` (auto-run). Offer `red-team` + `consistency`.
- **Support:** `constitution-check` (auto-run). Offer `red-team` + `consistency`.

Dispatch all auto-run passes as parallel background subagents. Wait for
completion, then present findings per the protocol before offering to
continue to Decompose.

6. **Confirm:** "Saved. Spec is now at Decompose stage."

### Stage Transition

After persisting, offer to continue:

"Specify is saved. Want to continue to Decompose? I can propose how to break this
into slices."

- Continue to **Decompose** → `/specgraph-decompose <slug>`
- Or stop here — the spec is saved at Specify stage and can be resumed later.
