# Spec Linting

The linter validates specs structurally before deeper analytical passes run.
It checks that required fields are present, edges point to real nodes, and the
graph contains no cycles — fast, synchronous checks that catch malformed specs
at authoring time rather than during execution.

---

## What the Linter Checks

**Schema validation** — required fields must be present and correctly typed for
the spec's current authoring stage. A spec at the `specify` stage must have a
title, description, and acceptance criteria; a spec at `decompose` must have
slices.

**Edge consistency** — every edge must point to a spec that exists in the graph.
`depends_on`, `blocks`, and `composed_of` edges with dangling references are
flagged before they can be committed.

**Constitution compliance** — specs must respect the constraints defined in the
active constitution layers (project, domain, org). Field restrictions,
required tags, and naming conventions are all evaluated here.

**Cycle detection** — no circular dependency chains. If spec A depends on B and
B depends on A, neither can ever be completed. The linter walks the dependency
graph and reports the full cycle path.

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
