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
skill is a router. It tells you which MCP prompt to invoke and when. It does
not duplicate the prompt bodies; the server is the source of truth.

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

Each stage has an MCP prompt of the same name. Two equivalent paths:

1. Invoke the MCP prompt directly: `spark`, `shape`, `specify`, `decompose`,
   or `approve`. Prompts assemble persona + orchestration + stage content +
   current state in one call.
2. Call the `author_start_stage` tool with `stage` and (after spark) `slug`.
   Useful when you want the prompt body returned as a tool result for
   inspection rather than dispatched as a message.

After the conversation produces a stage output, persist it with the `author`
tool, setting `action` to the stage name (`spark`, `shape`, `specify`,
`decompose`, or `approve`).

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
