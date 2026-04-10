# SpecGraph

**One ground truth. Every decision, every dependency, every engineer.**

SpecGraph is Spec-Driven Development at enterprise scale — a live, queryable
spec graph where architectural constraints are enforced, decisions are
traceable, and every member of your team starts with the full picture.
Human or AI, no one builds cold.

[:octicons-arrow-right-24: How It Works — understand the full SDD picture](how-it-works.md){ .md-button }
[:octicons-arrow-right-24: Quick Start — author your first spec in under ten minutes](quickstart.md)

!!! tip "When to Use SpecGraph"
    SpecGraph is designed for enterprise teams where multiple engineers and
    AI agents need shared architectural context, live dependency tracking,
    and governance at the spec layer. For solo developers or small projects,
    simpler tools are a better fit.

---

## Why

AI coding teams produce code fast. The bottleneck has moved upstream — to
specification, governance, and verification. Static specs in files can't
coordinate parallel workers, can't enforce architectural constraints, and
can't answer "what's the critical path?" Spec-Driven Development solves
this. SpecGraph is SDD built for enterprise scale.

[:octicons-arrow-right-24: The full problem statement](problem.md)

---

## Core Concepts

<div class="grid cards" markdown>

- :material-shield-check: **Ground Truth**

    ---

    No engineer starts cold. Your tech stack, constraints, and architectural
    decisions — encoded once, inherited by every engineer and agent. Query
    before you build.

    [:octicons-arrow-right-24: Learn more](concepts/ground-truth.md)

- :material-graph: **The Spec Graph**

    ---

    Query your architecture. Specs are live graph nodes with typed edges.
    Find what's blocked, trace the critical path, detect drift — one
    command, not a grep script.

    [:octicons-arrow-right-24: Learn more](concepts/spec-graph.md)

- :material-filter: **Authoring Funnel**

    ---

    From rough idea to execution-ready spec. A five-stage AI-collaborative
    pipeline — Spark, Shape, Specify, Decompose, Approve. Human or agent,
    the funnel adds just enough structure at each step.

    [:octicons-arrow-right-24: Learn more](concepts/authoring.md)

- :material-gavel: **Architectural Governance**

    ---

    Violations surface at the spec layer. Constitution checks, red-team
    passes, and drift detection catch problems before code review — or
    production.

    [:octicons-arrow-right-24: How it works](how-it-works.md)

</div>

---

## Project Status

Core authoring, graph queries, ground truth, drift detection, and sync
adapters are shipped. CLI and Claude Code plugin available now.

See the [changelog](changelog.md) for the latest release.
[Author your first spec](quickstart.md) in under ten minutes, or read
the [architecture overview](architecture.md) to understand the system design.
