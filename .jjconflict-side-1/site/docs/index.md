# SpecGraph

**Live Spec-Driven Development Framework**

SpecGraph manages software specifications as a queryable graph, not static markdown
files. It captures your project's ground truth in a layered constitution, guides
ideas through an AI-collaborative authoring funnel, stores every spec as a node
with first-class dependency edges, and produces execution-ready work units that
agents can claim and run without human translation.

---

## The Problem

- **No live query** — you can't ask "what specs are blocked?" without grepping
  across a folder of markdown files
- **No addressability** — specs reference each other by filename, not a stable,
  structured identity
- **No execution interface** — AI agents need structured task graphs, not
  directories of prose documents
- **No ground truth** — every authoring session starts from scratch because
  project context lives in people's heads, not in a queryable store

---

## Core Concepts

<div class="grid cards" markdown>

- :material-graph: **Specs as Graph Nodes**

    ---

    Dependencies, blocks, and compositions are first-class edges in a graph
    database — not fragile filename references. Query relationships, detect
    cycles, and traverse the full dependency tree.

    [:octicons-arrow-right-24: Learn more](concepts/specs.md)

- :material-shield-check: **Constitution**

    ---

    A layered document that captures project ground truth — tech stack,
    principles, constraints, and patterns. More specific layers (Project,
    Domain) override general ones (User, Org).

    [:octicons-arrow-right-24: Learn more](concepts/constitution.md)

- :material-filter: **Authoring Funnel**

    ---

    An AI-collaborative pipeline that guides ideas from rough spark to
    execution-ready spec: Spark, Shape, Specify, Decompose, Approve. Each
    stage adds structure and validation.

    [:octicons-arrow-right-24: Learn more](concepts/authoring.md)

- :material-robot: **Agent-Native**

    ---

    Every spec is a structured, claimable work unit with clear inputs, outputs,
    and acceptance criteria. Agents can claim specs, execute them, and verify
    results without human translation.

    [:octicons-arrow-right-24: Learn more](concepts/specs.md)

</div>

---

## How It Works

SpecGraph captures your project's ground truth in a constitution, guides you
through an AI-collaborative authoring funnel (Spark &rarr; Shape &rarr; Specify
&rarr; Decompose &rarr; Approve), stores specs as nodes in a graph database, and
produces execution-ready work units. The result is a living specification graph
that agents and humans can query, traverse, and act on.

[:octicons-arrow-right-24: See the full workflow](how-it-works.md)

---

## Project Status

!!! info "Phase 2 — Authoring & CLI"

    Phase 1 (spec schema, constitution, storage, and query layer) is complete.
    Phase 2 (authoring flow, codebase scanner, CLI integration) is in progress.
    See the [roadmap](roadmap.md) for what's coming next.
