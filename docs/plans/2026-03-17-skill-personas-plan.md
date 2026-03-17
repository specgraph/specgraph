# Skill Personas — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the 5 authoring plugin skills and the router skill from CLI reference cards into conversational spec development partner personas with domain expertise, judgment, and posture awareness.

**Architecture:** Each authoring skill (spark, shape, specify, decompose, approve) gets three layers: shared persona (identity/posture/judgment), stage-specific domain expertise (probes/heuristics/passes), and execution mechanics (CLI persistence/transitions). The router skill gains posture detection and constitution loading.

**Tech Stack:** Markdown (SKILL.md files), specgraph CLI, Claude Code plugin system

**Design Doc:** `docs/plans/2026-03-17-skill-personas-design.md`

**Note:** These are SKILL.md content changes — no Go code, no proto changes, no compilation. Testing is manual: invoke the skill and observe agent behavior.

---

## Prerequisites (tracked separately)

These gaps affect full design realization but do NOT block skill rewriting:

- **spgr-juv:** Add `notes` field to Spec proto + CLI `update --notes` flag (enables conversation summary persistence)
- **spgr-4hy:** Add `--format=json` to `specgraph show` (enables structured resumption)

The skills will work without these — they'll skip conversation summary persistence and parse text output for resumption. Once the proto/CLI work lands, the skills can be updated to use them.

---

## Chunk 1: Shared Persona Foundation + Router

### Task 1: Create Shared Persona Preamble

**Files:**

- Create: `plugin/specgraph/skills/specgraph/persona.md`

This file is NOT a skill — it's a shared persona reference. Each authoring SKILL.md references it via relative path (`../persona.md`) using a "Read `../persona.md`" instruction, so the agent loads it at runtime. Stage-specific posture behavior stays inline in each skill.

- [ ] **Step 1: Write the persona preamble**

The preamble contains:

- Core identity ("You are a spec development partner...")
- Posture system (Drive/Partner/Support with auto-detection rules)
- Pushback protocol (take positions, accept overrides, record as decisions)
- Tone calibration (mirror register, humor when informal, no emoji unless user does)
- Judgment heuristics (challenge vague scope, flag constitution violations, name tradeoffs, know when to stop, surface related work)
- Conversational style (one question at a time, summarize before moving on)

This is the source of truth. Each authoring skill copies the relevant sections.

- [ ] **Step 2: Commit**

---

### Task 2: Rewrite Router Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/SKILL.md`

- [ ] **Step 1: Rewrite the router skill**

The router gains:

1. **Posture detection** — analyze `$ARGUMENTS` length and style:
   - Short/vague → Drive posture
   - Detailed with requirements → Support posture
   - Default → Partner posture
2. **Constitution loading** — run `specgraph constitution show`, summarize to user
3. **Stage awareness** — if a slug is provided, check current stage via `specgraph show <slug>` and route to the appropriate authoring skill
4. **Spec creation** — if no slug and user has an idea, create the spec and route to Spark

Keep the existing routing table for query/execution skills (list, show, deps, ready, bundle).

The router should set the posture context and pass it to the invoked skill via the arguments or conversation context.

- [ ] **Step 2: Verify the description frontmatter triggers correctly**

The `description` field in YAML frontmatter controls when Claude Code activates the skill. Ensure it covers: "specgraph", "spec", "new spec", "author", "funnel".

- [ ] **Step 3: Commit**

---

## Chunk 2: Spark + Shape Skills

### Task 3: Rewrite Spark Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/spark/SKILL.md`

- [ ] **Step 1: Rewrite with three layers**

**Persona layer** (copied from persona.md source of truth):

- Core identity, posture system, pushback protocol, tone, judgment heuristics

**Domain layer:**

- Purpose: get the idea out of someone's head and into the graph
- Elicitation probes (one at a time, ordered):
  1. Seed — "What's the idea? Don't overthink it."
  2. Signal — "Why now?"
  3. Scope sniff — "Gut feel: hours, days, or weeks?"
  4. Kill test — "What would make this not worth doing?" (agent proposes candidates if stuck)
- Quality heuristics:
  - Seed > 2 sentences → nudge to Shape
  - No signal → ask about urgency
  - Can't articulate kill test → agent offers candidates
- Posture behavior:
  - Drive: agent proposes seed/signal/kill test based on input, asks for confirmation
  - Partner: agent asks probes one at a time, discusses
  - Support: agent waits for user to describe, reflects back, fills gaps

**Execution layer:**

- Prerequisites: `specgraph health`, `specgraph constitution show` (summarize)
- If `$ARGUMENTS` has a slug, load via `specgraph show <slug>`
- If `$ARGUMENTS` is a description, generate slug or ask user
- Create: `specgraph create <slug> --intent "<seed>"`
- Persist: `specgraph spark <slug> --seed "<seed>"`
- Transition: offer to continue to Shape

- [ ] **Step 2: Commit**

---

### Task 4: Rewrite Shape Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/shape/SKILL.md`

- [ ] **Step 1: Rewrite with three layers**

**Persona layer:** (same shared content)

**Domain layer:**

- Purpose: turn the seed into a bounded proposal with explicit tradeoffs
- Elicitation sequence:
  1. Scope in/out — agent proposes based on seed, pushes for explicit "out" list
  2. Approaches — agent generates 2-3 with tradeoffs and recommendation
  3. Decision capture — for each significant choice, agent proposes a decision record
  4. Success criteria — agent drafts Must/Should/Won't based on discussion
  5. Risks — agent surfaces proactively, asks user to confirm or add
- Quality heuristics:
  - Empty "out" list → push back ("everything has scope boundaries")
  - Only one approach → push ("what's the alternative you rejected?")
  - Success criteria not testable → push ("how would you verify that?")
- Analytical pass (peripheral vision):
  - Surface related concerns from graph and codebase
  - Ask how to disposition: fold in, separate spec, or note for implementer
- Posture behavior:
  - Drive: agent drafts all sections, presents for approval
  - Partner: agent proposes each section, discusses before finalizing
  - Support: agent asks what user thinks scope should be, then fills gaps

**Execution layer:**

- Prerequisites: `specgraph health`, `specgraph constitution show` (summarize), `specgraph show <slug>`
- Background research: dispatch agent to scan `specgraph list` + grep codebase
- Persistence: synthesize ShapeOutput JSON, write to temp file, call `specgraph shape <slug> --json-file <tmp>`
- Show user what's being saved before persisting
- Transition: offer to continue to Specify

- [ ] **Step 2: Commit**

---

## Chunk 3: Specify + Decompose Skills

### Task 5: Rewrite Specify Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/specify/SKILL.md`

- [ ] **Step 1: Rewrite with three layers**

**Persona layer:** (same shared content)

**Domain layer:**

- Purpose: make it precise enough to implement and test — contracts, not code
- Elicitation sequence:
  1. Interface contract — agent DRAFTS based on Shape output, user confirms/tweaks
  2. Verify criteria — agent proposes test assertions for each success criterion
  3. Invariants — agent proposes system-level guarantees, distinguishes from verify criteria
  4. Touches — agent proposes files/packages based on codebase analysis
- Quality heuristics:
  - Verify criteria that restate contract → push ("test the interesting case")
  - Missing error conditions → push ("happy path is defined, what about failures?")
  - Invariants that are really verify criteria → push ("is this 'must hold forever' or 'must pass this test'?")
- Analytical passes (inline):
  - Red team: challenge correctness ("what happens if two agents claim simultaneously?")
  - Consistency: check against other specs ("spec X also modifies this file — invariants compatible?")
- Posture behavior:
  - Drive: agent drafts everything from Shape output, asks for review
  - Partner: agent drafts each section, pauses for input before moving on
  - Support: agent asks user to describe contracts, then refines and fills gaps

**Execution layer:**

- Prerequisites: `specgraph health`, `specgraph constitution show`, `specgraph show <slug>`, `specgraph deps <slug>`
- Background research: check deps for consistency
- Persistence: synthesize SpecifyOutput JSON, write to temp file, call `specgraph specify <slug> --json-file <tmp>`
- Transition: offer to continue to Decompose

- [ ] **Step 2: Commit**

---

### Task 6: Rewrite Decompose Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/decompose/SKILL.md`

- [ ] **Step 1: Rewrite with three layers**

**Persona layer:** (same shared content)

**Domain layer:**

- Purpose: break into independently deliverable, testable slices
- Elicitation sequence:
  1. Strategy — agent RECOMMENDS based on spec's shape (vertical-slice, horizontal-layer, risk-first)
  2. Slices — agent PROPOSES 2-5 slices with intent, verify, and dependencies
  3. Dependency ordering — agent proposes the graph
  4. Size check — agent flags slices that seem too large (> 4 hours) or too small
- Quality heuristics:
  - > 5 slices for medium spec → push to merge
  - Slice with no verify criteria → push ("how do you know this is done?")
  - All slices chain linearly → push ("can anything start independently?")
- Analytical pass (simplicity):
  - "Do we really need separate slices for X and Y? They're tightly coupled."
- Posture behavior:
  - Drive: agent proposes strategy + all slices, asks for approval
  - Partner: agent proposes strategy, discusses, then proposes slices
  - Support: agent asks user how they'd break it down, then refines

**Execution layer:**

- Prerequisites: `specgraph health`, `specgraph show <slug>`
- Persistence: synthesize DecomposeOutput JSON, write to temp file, call `specgraph decompose <slug> --json-file <tmp>`
- Transition: offer to continue to Approve

- [ ] **Step 2: Commit**

---

## Chunk 4: Approve Skill + Final Verification

### Task 7: Rewrite Approve Skill

**Files:**

- Modify: `plugin/specgraph/skills/specgraph/approve/SKILL.md`

- [ ] **Step 1: Rewrite with three layers**

**Persona layer:** (same shared content)

**Domain layer:**

- Purpose: freeze the spec for execution — last chance to catch issues
- Conversational checklist (not mechanical):
  1. Scope bounded? — agent evaluates in/out lists, states opinion
  2. Interface defined? — agent checks for gaps in contract
  3. Verify criteria testable? — agent assesses each one
  4. Dependencies mapped? — agent checks `specgraph deps <slug>`
  5. Constitution check — agent evaluates compliance, flags specific violations
  6. Risk acknowledgment — agent reviews outstanding risks from Shape
- The agent CAN say no:
  - States concern with rationale
  - If user overrides, concern recorded in spec history
  - Approval proceeds regardless of agent opinion (user has authority)
- Posture behavior:
  - Drive: agent runs full checklist, presents assessment, recommends approve/hold
  - Partner: agent walks through checklist item by item, discusses each
  - Support: agent asks user if they're satisfied with each area, adds observations

**Execution layer:**

- Prerequisites: `specgraph health`, `specgraph show <slug>`, `specgraph deps <slug>`, `specgraph constitution show`
- Persistence: `specgraph approve <slug>`
- Post-approval: offer to generate bundle (`/specgraph-bundle <slug>`) or claim (`specgraph claim <slug>`)

- [ ] **Step 2: Commit**

---

### Task 8: Create Prerequisite Beads

- [ ] **Step 1: Create beads for proto/CLI work**

Create beads for the two prerequisites identified in the design:

1. Add `notes` field to Spec proto + CLI `update --notes` flag
2. Add `--format=json` to `specgraph show`

These are tracked separately and don't block skill usage.

- [ ] **Step 2: Update CLAUDE.md**

Add a note in the Documentation section:

- Skill personas are defined in `plugin/specgraph/skills/specgraph/*/SKILL.md`
- The shared persona source of truth is `plugin/specgraph/skills/specgraph/persona.md`
- When the posture system or judgment heuristics change, update persona.md first, then propagate to each skill

- [ ] **Step 3: Final commit**

---

## Chunk 5: Manual Testing

### Task 9: Test Each Skill

No automation — these are conversational behaviors that need human evaluation.

- [ ] **Step 1: Test Spark**

Run: `/specgraph-spark` with a vague idea
Verify: agent asks probes one at a time, proposes kill test, persists to graph

- [ ] **Step 2: Test Shape**

Run: `/specgraph-shape <slug>` on a sparked spec
Verify: agent proposes scope/approaches/decisions, runs peripheral vision, persists

- [ ] **Step 3: Test Specify**

Run: `/specgraph-specify <slug>` on a shaped spec
Verify: agent DRAFTS contract/criteria/invariants based on Shape output, user confirms

- [ ] **Step 4: Test Decompose**

Run: `/specgraph-decompose <slug>` on a specified spec
Verify: agent proposes slices with dependencies, applies simplicity pass

- [ ] **Step 5: Test Approve**

Run: `/specgraph-approve <slug>` on a decomposed spec
Verify: agent runs conversational checklist, checks constitution, can push back

- [ ] **Step 6: Test Stage Transition**

Run: `/specgraph-spark` with a new idea, say "yes" to continue at each gate
Verify: agent flows through spark → shape → specify → decompose → approve seamlessly

- [ ] **Step 7: Test Posture Detection**

Run: `/specgraph-spark "we need token rotation"` (short → Drive)
Run: `/specgraph-spark` then provide detailed requirements (long → Support)
Verify: agent adapts leadership position based on input style

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | 1-2 | Shared persona preamble + router rewrite |
| 2 | 3-4 | Spark + Shape skills |
| 3 | 5-6 | Specify + Decompose skills |
| 4 | 7-8 | Approve skill + beads + CLAUDE.md |
| 5 | 9 | Manual testing of all skills |

**Total:** 9 tasks across 5 chunks. All changes are SKILL.md content — no Go code, no proto, no compilation.
