---
name: specgraph-authoring
summary: Author a SpecGraph spec through the spark → shape → specify → decompose → approve funnel.
description: Use when the user wants to author or update a SpecGraph spec — sparking an idea, shaping a problem statement, specifying detail, decomposing into work, or walking a spec to approval. Routes to the right MCP prompt for the current authoring stage.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Authoring

The SpecGraph MCP server delivers full authoring guidance through prompts:
persona, orchestration heuristics, and stage-specific instructions. This
skill is the router plus the write contract. It tells you which MCP prompt to
invoke, and — critically — the exact payload the `author` tool accepts so you
never hand it a shape it can't parse.

Everything here runs over MCP. No local CLI is required (source/CLI users have
an equivalent path in the gated appendix; MCP-only agents skip it).

## When this skill applies

Trigger on any of:

- The user wants to start a new spec ("let's spec out X")
- The user is mid-funnel and wants to advance ("ready to specify this")
- The user asks for help with an existing spec by slug
- A stage transition just succeeded and the next stage is in scope

## The five-stage funnel

| Stage | Purpose | Output |
|---|---|---|
| `spark` | Take a topic, draft an idea | initial intent + posture |
| `shape` | Refine into a clear problem statement | problem, success criteria |
| `specify` | Add full design detail | constraints, interfaces, risks |
| `decompose` | Break into actionable work | slices, dependencies, edges |
| `approve` | Capture decisions, finalize | decisions + edges + sign-off |

## How to invoke

Start from `specgraph://prime` (the session digest) and the companion
`specgraph_skills_get` / `specgraph_skills_search` tools if you need to reload
guidance. Each stage has an MCP prompt of the same name. Two equivalent paths:

1. Invoke the MCP prompt directly: `spark`, `shape`, `specify`, `decompose`,
   or `approve`. Prompts assemble persona + orchestration + stage content +
   current state in one call.
2. Call the `author_start_stage` tool with `stage` and (after spark) `slug`.
   This is the documented entry path when you want the composed prompt body
   returned as a tool result for inspection rather than dispatched as a
   message. Do not drop it — it is the re-entry hook for mid-conversation
   stage work.

After the conversation produces a stage output, persist it with the `author`
tool, setting `action` to the stage name (`spark`, `shape`, `specify`,
`decompose`, or `approve`).

## The write payload (this is the #1002 fix)

Two arguments carry the work into the `author` tool:

- `output` — the stage result as **friendly YAML** using **snake_case** field
  names (below). This is what you show the user AND what you send — no
  translation step.
- `exchanges` — the conversation log as a **JSON array** (below). Required for
  shape/specify/decompose, optional for spark, not needed for approve.

### `output` per stage (friendly YAML, snake_case)

Spark — distill the idea. `exchanges` are OPTIONAL here:

```yaml
seed: "one-line idea or problem statement"
signal: "why this matters now"
questions:
  - "clarifying question to sharpen scope"
scope_sniff: small        # tiny | small | medium | large | epic
kill_test: "condition that would make this not worth pursuing"
```

Shape — bound the scope and choose an approach:

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

Specify — make it precise and testable:

```yaml
interfaces:
  - name: "WebhookService proto"
    body: "the contract content (proto, method signatures, etc.)"
verify_criteria:
  - category: "emission"
    description: "a testable acceptance criterion"
invariants:
  - "condition that must never be violated"
touches:
  - path: "internal/foo/bar.go"
    purpose: "what changes and why"
    change_type: "new"       # new | modify | delete
```

Decompose — break into deliverable slices:

```yaml
strategy: vertical_slice     # vertical_slice | layer_cake | single_unit | steel_thread
slices:
  - id: "slice-1"
    intent: "what this slice accomplishes"
    verify:
      - "condition that must hold for this slice to be done"
    touches:
      - "internal/foo/"
    depends_on:
      - "slice-0"
```

> Use these snake_case keys verbatim. Do NOT camelCase them (`scopeIn`,
> `chosenApproach`, `verifyCriteria` will be rejected) — the snake_case
> field-name typo is the exact class that produced #1002.

### `exchanges` (JSON array — required for shape/specify/decompose)

`exchanges` stays a **JSON array** for this milestone (it is NOT converted to
friendly YAML). Pass the accumulated probe/response history alongside the
`output` on the SAME `author` call — they commit atomically. Fields:

- `role` — `"probe"` (agent asks) or `"response"` (user answers)
- `content` — the substantive text
- `stage` — the authoring stage (`shape`, `specify`, `decompose`)
- `sequence` — strictly increasing integer ≥ 1; same number pairs a probe
  with its response

Minimal valid example for a shape call:

```json
[
  { "role": "probe",    "content": "What is explicitly out of scope?", "stage": "shape", "sequence": 1 },
  { "role": "response", "content": "Anything touching billing.",       "stage": "shape", "sequence": 2 }
]
```

**Required for shape / specify / decompose** — the server enforces at least
one exchange for these stages; omitting them fails the call. Set `stage` to
match the call.

**Spark `exchanges` are OPTIONAL** — the server validates spark exchanges only
when they are present (a seed-only spark from a single idea, with no
back-and-forth, is valid without any `exchanges` array). Do not fabricate a
spark conversation just to satisfy a non-existent requirement.

**Approve** needs only the `slug` (and explicit user sign-off). It does not
require an `output` or `exchanges` on a clean acceptance.

## Amending and superseding a spec

Two lifecycle actions on the `author` tool move a spec *backward* or retire it.
They are distinct, and each has a strict precondition:

| Action | When it's allowed | What it does | Required params |
|---|---|---|---|
| `amend` | while the spec is **in flight** (`approved`, `in_progress`, or `review`) | returns the spec to authoring so you can re-author a stage | `slug`, `re_entry_stage`, `reason` |
| `supersede` | **only** on a `done` spec | retires it and points to a replacement (draws a `SUPERSEDES` edge) | `slug`, `new_slug` (`reason` optional) |

Amending a `done` spec, or superseding a spec that is not `done`, is rejected
server-side — the two are not interchangeable.

### The land-one-before model (this is the #899 fix)

`re_entry_stage` is **the stage you want to redo**, one of
`spark | shape | specify | decompose`. On amend the spec does NOT land *at*
that stage — it lands **one stage before** it, so re-running the stage is a
valid forward transition instead of a same-stage no-op.

Worked example — the canonical happy path:

> `re_entry_stage: shape` means "redo shape". The spec lands at `spark`, and
> `author action=shape` then succeeds (`spark → shape` is a valid transition).

```yaml
# 1. amend an in-flight spec, asking to redo shape
author:
  action: amend
  slug: my-spec
  re_entry_stage: shape      # "I want to redo shape"
  reason: "scope changed after review"
# → spec lands at `spark`; the tool result names the next step:
#   "Next step: run author action=shape …"

# 2. re-author the stage — now a valid transition, not a no-op
author:
  action: shape
  slug: my-spec
  output: { ... }
  exchanges: [ ... ]
```

Always present `shape` (or `specify` / `decompose`) as the re-entry example.

> **Caveat — don't lead with `re_entry_stage: spark`.** The stage before
> `spark` is `spark` itself, so the spec lands at `spark`. Re-running
> `author action=spark` on an existing spec returns `ALREADY_EXISTS` — there is
> no spark re-author path. Never present it as a next step.

### Superseding a done spec

Once a spec is `done` you cannot amend it — retire it with a replacement:

```yaml
author:
  action: supersede
  slug: old-auth          # must be `done`
  new_slug: new-auth      # the replacement spec (non-terminal)
  reason: "rebuilt on the new lifecycle model"   # optional
```

## Posture

Three postures govern how much the agent leads:

- **Drive** — short, vague user input. Agent proposes, drafts, runs auto-passes.
- **Partner** — default. Back-and-forth.
- **Support** — long, detailed user input. Agent listens, reflects, drafts on
  request.

The composer detects posture from first-turn signals; the user can override
with "switch to drive mode" at any point.

## Conventions

- Slugs are `kebab-case`, derived from intent. The spark stage proposes one;
  the user can override.
- Stage transitions are validated server-side. You can't `specify` without a
  shape, can't `decompose` without a specify, etc.
- `approve` is the only stage that requires explicit user sign-off. Never
  approve on the user's behalf.

## Don't

- Don't camelCase the `output` keys. The parser accepts snake_case.
- Don't invent dotted MCP tool names. SpecGraph tools are flat (e.g., `author`,
  not `author.spark`) and use an `action` parameter.
- Don't skip stages. The validation will reject you, and the elicitation is
  designed to catch rushed thinking.
- Don't recreate the composer prompts in your own message. Invoke the MCP
  prompt and let the server produce the body.

## Reference

Companion skills:

- `specgraph-graph-query` — for "what specs exist," "what's ready," "what
  depends on this."
- `specgraph-analytical-passes` — for review passes that run alongside
  authoring.
- `specgraph-conventions` — for slug, edge type, and stage transition rules.

---

## Requires local CLI (source/CLI users only — MCP-only agents skip this)

The authoring funnel is fully driven over MCP with the `author` tool and the
stage prompts above — an MCP-only agent needs nothing here. Source/CLI users
running the `specgraph` binary directly can inspect authoring state and specs
through the local command surface; the MCP path is the supported route for
agents and is the source of truth for the write payload.
