---
name: specgraph-drift
summary: Detect, acknowledge, or fix drift between specs and their upstream dependencies.
description: Use when the user wants to detect or acknowledge drift on a done spec — when a dependency's content has changed since baseline, when interface or verification claims are stale, or when running `specgraph drift` from the CLI.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Drift Detection

Drift is the gap between what a spec claims and what's true now. Once a spec
is `done`, the world changes around it. SpecGraph baselines edges at
done-transition time so drift can be detected later.

## When this skill applies

- The user runs `specgraph drift` from the CLI or asks about a stale dep.
- A done spec needs re-baselining after an upstream change.
- Findings show drift and the user wants to acknowledge or fix.

## How drift works

When a spec transitions to `done`, every `DEPENDS_ON` edge captures the
upstream's `ContentHash` in a property called `content_hash_at_link`. Drift
detection compares that snapshot to the upstream's current `ContentHash`. A
mismatch is drift; an empty edge hash (unmigrated edges) always reports.

## Drift scopes

| Scope | What it checks | Status |
|---|---|---|
| `dependency` | DEPENDS_ON edge baselines vs current upstream hash | Implemented |
| `interface` | Public contract changes downstream consumers haven't acknowledged | Designed, not yet implemented |
| `verify` | Automated checks that spec claims still hold | Designed, not yet implemented |

The CLI accepts `--scope dependency` (and will accept `interface` / `verify`
when those land). Default is dependency.

## Acknowledging drift

`specgraph drift acknowledge <slug>` re-baselines the spec's edges. Use
`--all` to baseline every DEPENDS_ON edge regardless of current hash. This
is the correct fix when the upstream change is intentional and the spec
text doesn't need revision.

When the upstream change does require spec revision, instead transition the
spec back through `specify` or `approve` — the done-transition path will
re-baseline naturally.

## MCP surface

- `drift` tool, `action: "detect"` — runs detection for a slug or whole project
- `drift` tool, `action: "acknowledge"` — baselines edges
- Findings of type `drift` surface in `specgraph://findings`

## Don't

- Don't ack drift without reading the upstream change. Acknowledgment means
  "I've considered this; the spec is still correct as written." If the spec
  needs to change, route through the funnel instead.
- Don't expect drift to surface on never-done specs. Edges aren't baselined
  until the first done transition.
