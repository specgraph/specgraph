# Spec Linting

The linter validates specs structurally before deeper analytical passes run.
It checks that required fields are present, edges point to real nodes, and the
graph contains no cycles — fast, synchronous checks that catch malformed specs
at authoring time rather than during execution.

---

## What the Linter Checks

**Schema validation** — required fields must be present for the spec's current
authoring stage. The linter checks that core fields like `intent` and `slug` are
populated and that stage-specific outputs exist where expected.

**Edge consistency** — every edge must point to a spec that exists in the graph.
`depends_on`, `blocks`, and `composes` edges with dangling references are
flagged before they can cause downstream failures.

**Cycle detection** — no circular dependency chains. If spec A depends on B and
B depends on A, neither can ever be completed. The linter walks the dependency
graph via DFS and flags the node where a cycle is detected.

---

## CLI Usage

Lint all specs in the graph:

```sh
specgraph lint
```

Lint a single spec by slug:

```sh
specgraph lint <slug>
```

Both commands exit non-zero if any violations are found, making them safe to
use in CI pipelines.

---

## Linting vs. Analytical Passes

| | Linting | Analytical Passes |
|---|---|---|
| Speed | Synchronous, fast | Async, deeper |
| Scope | Structural validity | Semantic analysis |
| Catches | Malformed specs, broken edges | Constitution contradictions, security risks |

Linting runs first. Analytical passes (constitution check, security review,
etc.) only run against specs that are structurally valid. This keeps the
feedback loop tight: fix structural issues locally with `specgraph lint`, then
trigger the deeper analysis once the graph is clean.
