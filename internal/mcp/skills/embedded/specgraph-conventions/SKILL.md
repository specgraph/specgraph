---
name: specgraph-conventions
summary: SpecGraph slug, edge-type, and authoring conventions the model should follow.
description: Use when you need slug, stage, edge, or approval rules — when validating user input, when naming a new spec, or when explaining why an operation is rejected.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Conventions

Quick reference for the rules that govern what goes in the graph. The server
enforces these; this skill helps you anticipate them so you don't suggest
inputs that will fail validation.

## Slugs

- Format: `kebab-case`. Lowercase letters, digits, hyphens. No leading or
  trailing hyphen, no double hyphens.
- Length: at most 64 characters.
- Stable: don't change a slug after spark. Downstream edges reference it.
- Derivation: take the spec intent, drop articles, lowercase, hyphenate.
  "Add OAuth token rotation" → `oauth-token-rotation`. The spark composer
  proposes one; the user can override.

## Stage transitions

The funnel is one-way during authoring:

```text
spark → shape → specify → decompose → approve → approved → done
```

Validation:

- Each stage requires the previous stage's output. You can't `specify`
  without a `shape`.
- Re-entering an earlier stage is allowed (e.g., back to `shape` after
  feedback). Doing so doesn't drop later content; it overlays new shape
  output.
- `approve` requires explicit user sign-off. Never approve programmatically.
- `done` is set by execution (Gastown polecats) or manually after
  out-of-band completion.

## Edge types

The proto `EdgeType` enum (exposed via the `edge` tool):

| Type | Meaning | Cardinality |
|---|---|---|
| `DEPENDS_ON` | A needs B before A can be done | n:n |
| `BLOCKS` | A blocks B — must resolve A first | n:n |
| `COMPOSED_OF` | A is built from B | n:n |
| `RELATED_TO` | Soft link, no execution semantics | n:n |
| `DECIDED_IN` | Spec → Decision (per ADR-003) | n:1 |

Internal-only edges (created by storage, not via `edge` tool):

- `HAS_CHANGE` (Spec → ChangeLog)
- `HAS_FINDING` (Spec → Finding)

## Decisions

Decisions are first-class graph nodes (ADR-003), not properties on specs.
The `decision` tool covers create/get/update; `DECIDED_IN` edges link them
to the relevant specs.

## Approval rules

- A spec can't go from `decompose` to `approved` without an explicit user
  approval action — the agent must surface the decision and wait.
- `approve` is also when accepted decisions get linked. The
  `acceptLinkedDecisions` step on the server creates `DECIDED_IN` edges
  from the spec slug (the source) to each decision slug (the target).

## Don't

- Don't fabricate edge types. The proto enum is the closed set.
- Don't promote `HAS_CHANGE` or `HAS_FINDING` to user-facing edge calls.
  Storage owns them.
- Don't use snake_case slugs. The validator will reject them.
