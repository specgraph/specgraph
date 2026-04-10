# SpecGraph

**One ground truth for every decision, dependency, and engineer.**

SpecGraph is Spec-Driven Development at enterprise scale. Your specs live
in a queryable graph. It enforces architectural constraints and traces
decisions back to the specs they shaped, so every engineer (human or AI)
gets the full picture before writing code.

[:octicons-arrow-right-24: How It Works — understand the full SDD picture](how-it-works.md){ .md-button }
[:octicons-arrow-right-24: Quick Start — author your first spec in under ten minutes](quickstart.md)

!!! tip "When to Use SpecGraph"
    SpecGraph targets teams where multiple engineers and AI agents share a
    codebase — shared architectural context, live dependency tracking, and
    governance at the spec layer. Solo developers with a handful of specs
    can start with simpler tools.

---

## Why

AI coding teams generate code fast. What they can't do is verify it's the
right code, and that's a spec problem. Static specs in files can't
coordinate parallel workers or enforce architectural constraints. Try
asking your markdown folder "what's the critical path?" Spec-Driven
Development solves this. SpecGraph is SDD at enterprise scale.

[:octicons-arrow-right-24: The full problem statement](problem.md)

---

## Core Concepts

<div class="grid cards" markdown>

- :material-shield-check: **Ground Truth**

    ---

    No engineer starts cold. Your tech stack, constraints, and architecture,
    encoded once and queryable by anyone before they write anything.

    [:octicons-arrow-right-24: Learn more](concepts/ground-truth.md)

- :material-graph: **The Spec Graph**

    ---

    Query your architecture. Specs are live graph nodes with typed edges.
    Find what's blocked, trace the critical path, detect drift. One
    command each.

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
    passes, and drift detection catch problems before code review. Or
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
