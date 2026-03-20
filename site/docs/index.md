# SpecGraph

**Spec-Driven Development infrastructure — specifications as a queryable graph**

SpecGraph manages software specifications as nodes in a graph database, not
files in a folder. Dependencies, blocks, and compositions are first-class
edges. Architectural constraints live in a layered constitution that agents
query before writing a single line of code. An AI-collaborative authoring
funnel guides ideas from rough spark to execution-ready spec. The result is
a specification graph that humans and agents can query, traverse, and act on.

SpecGraph is a reference implementation of
[Spec-Driven Development](https://seanbrandt.dev/blog/intro-to-sdd) (SDD)
infrastructure — the practice of treating specifications as the primary
engineering artifact and code as generated output.

[:octicons-arrow-right-24: Quick Start — author your first spec in 10 minutes](quickstart.md)

!!! tip "When to Use SpecGraph"
    SpecGraph is designed for teams where file-based specs break down:
    cross-spec queries, dependency tracking, multi-agent coordination, and
    layered governance at scale. For solo developers or small projects,
    simpler tools — markdown files,
    [Spec Kit](https://github.com/github/spec-kit), lightweight execution
    frameworks — are a better fit.

---

## Why

AI agents produce code fast. The bottleneck has moved upstream — to
specification, review, and verification. File-based specs cannot keep up:
no stable identity, no queryable dependencies, no governance enforcement,
no structured execution interface.

[:octicons-arrow-right-24: The full problem statement](problem.md)

---

## Core Concepts

<div class="grid cards" markdown>

- :material-graph: **Specs as Graph Nodes**

    ---

    Dependencies, blocks, and compositions are typed edges in a graph
    database — not filename references. Query relationships, detect cycles,
    and traverse the full dependency tree.

    [:octicons-arrow-right-24: Learn more](concepts/specs.md)

- :material-shield-check: **Constitution**

    ---

    A layered document that captures project ground truth — tech stack,
    principles, constraints, and patterns. More specific layers (Project,
    Domain) override general ones (Org, User).

    [:octicons-arrow-right-24: Learn more](concepts/constitution.md)

- :material-filter: **Authoring Funnel**

    ---

    An AI-collaborative pipeline that guides ideas from rough spark to
    execution-ready spec: Spark, Shape, Specify, Decompose, Approve. Each
    stage adds structure and validation.

    [:octicons-arrow-right-24: Learn more](concepts/authoring.md)

- :material-robot: **Execution-Ready Output**

    ---

    Every approved spec is a structured, claimable work unit with clear
    inputs, outputs, and acceptance criteria. Agents claim specs, execute
    them, and verify results — no human translation required.

    [:octicons-arrow-right-24: How it works](how-it-works.md)

</div>

---

## Project Status

v0.1.0 <!-- x-release-please-version -->

SpecGraph v0.1.0 is the first public release.
[Author your first spec](quickstart.md) in under 10 minutes, or read
the [architecture overview](architecture.md) to understand the system
design.
