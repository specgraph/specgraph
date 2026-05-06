---
name: specgraph-analytical-passes
description: Use when the user wants analytical review of a spec — constitution compliance, peripheral vision, red-team, consistency, or simplicity checks. Also use immediately after a stage transition to surface auto-registered passes.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Analytical Passes

Analytical passes are first-class review steps. Each pass type has a prompt
template and produces findings (graph nodes attached to the spec). The
`passRegistry` defines which passes auto-run at each stage; the rest are
opt-in.

## When this skill applies

- The user asks "review this spec" or "check this against the constitution."
- A stage transition just succeeded — the registered passes for that stage
  are the right next step.
- A finding came up and the user wants to re-run a pass after edits.

## Pass types

| Pass | Question it answers |
|---|---|
| `constitution-check` | Does this spec respect project ground truth? |
| `peripheral-vision` | What dependencies and downstream effects are we missing? |
| `red-team` | What's the strongest case against this design? |
| `consistency` | Does this contradict other specs or its own claims? |
| `simplicity` | Is this the smallest thing that solves the problem? |

## How to invoke

The `analytical_pass` tool with `action: "run"` and `pass_type` is the
universal entrypoint. Two prompt-level shortcuts:

- `constitution_check` MCP prompt — wraps `analytical_pass` for
  constitution-check with a one-arg `spec_slug` input.
- `dependency_review` MCP prompt — wraps `analytical_pass` for
  peripheral-vision.

For the other three (`red-team`, `consistency`, `simplicity`), call
`analytical_pass` directly.

## Auto-run schedule (passRegistry)

| Stage | Auto-passes |
|---|---|
| `spark` | constitution-check |
| `shape` | constitution-check, peripheral-vision |
| `specify` | constitution-check, red-team, consistency |
| `decompose` | constitution-check, simplicity |
| `approve` | constitution-check |

After a stage `author` call succeeds, call `passes_for_stage` if you want the
exact list, then run each.

## Storing findings

Pass execution returns a prompt template. Run the pass, then call
`StoreFindings` (via the `findings` tool, `action: "store"`) with the results.
Findings become graph nodes via `HAS_FINDING` edges and surface in
`specgraph://findings`.

## Template overrides

Project-local overrides go in `.specgraph/templates/<pass_type>.md`. The
handler checks the override directory first, then falls back to the embedded
default. Override paths are configured via the server's
`templateOverrideDir`.

## Don't

- Don't treat findings as blockers without context. A `simplicity` warning
  on a deliberately-rich spec is a discussion, not a stop.
- Don't run all five passes on every transition. The registry is intentional;
  running off-stage passes wastes tokens and dilutes signal.
