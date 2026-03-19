# Authoring Funnel

The authoring funnel is an AI-collaborative process that transforms vague ideas
into execution-ready specifications. It has five stages — **Spark, Shape,
Specify, Decompose, Approve** — each with a clear purpose, defined outputs, and
entry/exit criteria. You can enter at any stage, skip stages that don't apply,
or go backward when new information surfaces.

The funnel is not a gate-keeping ceremony. It is a progressive-refinement tool
that adds just enough structure at each step to make the next step possible.

---

## The Five Stages

### Spark

**Purpose:** Capture the raw idea before it evaporates. No structure required —
just enough to start a conversation.

**Key outputs:**

| Field | Description |
|-------|-------------|
| `seed` | The idea itself, in the author's own words. |
| `signal` | Why now — what triggered this idea (a bug report, a security audit, a customer request). |
| `scope_sniff` | Initial size estimate: `tiny`, `small`, `medium`, `large`, or `epic`. |
| `kill_test` | What would make this idea not worth pursuing. If the kill test is true, stop here. |

**Example:**

> *"We need OAuth2 refresh token rotation."*
>
> - **seed:** OAuth2 refresh token rotation
> - **signal:** Security audit finding
> - **scope_sniff:** medium
> - **kill_test:** If we drop OAuth2 entirely

At this point, SpecGraph also generates clarifying **questions** to sharpen the
idea before it moves to Shape. The author can answer them, ignore them, or jump
straight to Shape if the idea is already well-understood.

---

### Shape

**Purpose:** Bound the scope and explore solutions. Shape turns a seed into a
scoped proposal with explicit tradeoffs.

**Key outputs:**

| Field | Description |
|-------|-------------|
| `scope_in` | What is included in this spec. |
| `scope_out` | What is explicitly excluded — prevents scope creep by making boundaries visible. |
| `approaches` | 2-3 candidate implementation strategies with tradeoffs. |
| `chosen_approach` | Which approach was selected and why. |
| `risks` | Known risks that could affect delivery or outcomes. |
| `success_must` / `success_should` / `success_wont` | MoSCoW-style success criteria. |
| `decisions` | Design decisions made during shaping, tracked as ADR candidates. |

**Example:**

> For OAuth2 refresh token rotation:
>
> - **scope_in:** Rotate refresh token on every use, invalidate old token after
>   grace period
> - **scope_out:** Token introspection endpoint, PKCE changes
> - **approaches:** (1) single-use tokens, (2) lineage chain tracking,
>   (3) sliding-window expiry
> - **chosen_approach:** Lineage chain tracking — handles race conditions from
>   concurrent requests without breaking mobile clients
> - **risks:** Mobile clients may replay old tokens during network retries

Shape also runs a **peripheral vision** pass that surfaces related concerns
noticed during scoping (e.g., "the token storage table needs an index on
lineage_id"). Each peripheral item gets a disposition: fold it into this spec,
track it as a separate spec, or note it for the implementer.

---

### Specify

**Purpose:** Define the precise interface contract and invariants. After Specify,
the spec is fully structured and testable.

**Key outputs:**

| Field | Description |
|-------|-------------|
| `interface_contract` | The API shape, data model, or behaviour contract — inputs, outputs, side effects. |
| `verify_criteria` | Testable conditions that define "done" (e.g., "old token rejected within grace period"). |
| `invariants` | Conditions that must always hold, before and after execution (e.g., "never two valid refresh tokens for the same session"). |
| `touches` | Files, packages, or components in the codebase that will be modified. |

**Example:**

> - **interface_contract:** `POST /oauth/token` with `grant_type=refresh_token`
>   returns new access + refresh token pair; old refresh token enters grace
>   period
> - **verify_criteria:** Old token rejected after grace period; new token valid
>   immediately; concurrent rotation requests produce consistent lineage
> - **invariants:** At most one active refresh token per session outside the
>   grace window
> - **touches:** `internal/auth/token.go`, `internal/auth/store.go`,
>   `migrations/007_token_lineage.sql`

Specify also runs a **red team** pass — an adversarial analysis that challenges
the spec's correctness and safety — and a **consistency** check against other
specs in the graph to detect overlapping scope or conflicting invariants.

---

### Decompose

**Purpose:** Break large specs into independently deliverable slices. Each slice
becomes a child spec connected to the parent by `composes` edges.

**Key outputs:**

| Field | Description |
|-------|-------------|
| `strategy` | How the spec is being sliced: `vertical_slice`, `layer_cake`, or `single_unit`. |
| `slices` | Ordered list of child specs, each with its own intent, verify criteria, touches, and internal dependencies. |

A **vertical slice** delivers end-to-end value in each slice (e.g., "rotation
for password grant" then "rotation for authorization code grant"). A **layer
cake** splits by architectural layer (e.g., storage first, then API, then
client). **Single unit** means the spec is small enough to deliver as-is.

**Example:**

> Using `vertical_slice` strategy for OAuth2 token rotation:
>
> 1. **Slice 1 — Lineage tracking storage:** Add `lineage_id` and
>    `parent_token_id` columns. *Verify:* migration runs, old tokens still work.
> 2. **Slice 2 — Rotation on refresh:** Implement rotation in the token
>    endpoint. *Verify:* new token issued, old token grace period active.
> 3. **Slice 3 — Grace period enforcement:** Background job invalidates tokens
>    past grace period. *Verify:* old token rejected after window expires.

Decompose runs a **simplicity** pass that looks for opportunities to reduce
complexity — unnecessary slices, slices that could be merged, or overly fine
granularity.

---

### Approve

**Purpose:** Freeze the spec for execution. After approval, the spec is
immutable and becomes a claimable work unit.

**Approval checks:**

- All verify criteria are defined and testable.
- All dependencies are mapped as graph edges.
- The constitution has been checked — no violations of project constraints.
- Red team findings (if any critical ones exist) have been resolved.

Once approved, the spec enters the execution pipeline. Agents or humans can
**claim** the spec, execute against its verify criteria, and report completion.
If a spec needs to change after approval, it must be **superseded** by a new
spec rather than edited in place — preserving the design history.

---

## AI Postures

The authoring funnel supports three collaboration modes that determine how the
AI participates at each stage. The posture is independent of the stage — you can
use any posture at any stage.

### Drive

The agent proposes, drafts, and recommends. All analytical passes (red team,
peripheral vision, consistency, simplicity) run automatically. The human reviews
and approves.

Best for experienced users who want speed, or for well-understood work where the
agent has enough context to lead.

### Partner (default)

The agent asks first, then contributes. Analytical passes are offered before
running — the human decides which ones to invoke. Decisions are made together
through back-and-forth conversation.

Best for collaborative authoring where both human and AI bring valuable
perspective.

### Support

The agent listens, reflects, and clarifies. Analytical passes are held unless
explicitly requested. The human drives the authoring process; the agent fills
gaps and answers questions.

Best for users who want to lead the design process and use the AI as a
sounding board.

### Auto-detection

Posture auto-detects from conversation style:

- **Short, vague messages** (e.g., "we need token rotation") suggest the user
  wants the agent to drive.
- **Long, detailed messages** with specific requirements suggest the user wants
  to lead (Support).
- **Back-and-forth exchanges** with questions and refinements suggest
  collaborative work (Partner).

Users can also set the posture explicitly at any point during authoring, and the
posture can change between stages.

---

## Why a Funnel?

Progressive refinement prevents premature detail. You don't write interface
contracts for a vague idea, and you don't estimate scope before you've explored
approaches. Each stage has clear entry and exit criteria, so you always know
where you are and what "done" looks like for the current step. This means the
funnel naturally handles the most common failure mode in spec writing: jumping
straight from a rough idea to implementation details, skipping the scoping and
tradeoff analysis that prevents rework later.

The funnel also captures *why* decisions were made, not just *what* was decided.
Every stage records its outputs as structured data — the approaches considered
during Shape, the kill test from Spark, the red team findings from Specify. This
creates a traceable design history that survives beyond the original author's
memory. When someone asks "why did we use lineage chain tracking instead of
single-use tokens?" six months later, the answer is in the Shape output, not
buried in a Slack thread.
