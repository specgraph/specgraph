# Concepts

SpecGraph is built on a few core concepts. Each page in this section explains
one in detail — what it is, why it matters, and how it fits into the overall
framework.

<div class="grid cards" markdown>

- :material-graph: **Specs & the Graph**

    ---

    Specifications are nodes in a queryable graph with first-class dependency
    edges — not files in a folder.

    [:octicons-arrow-right-24: Specs & the Graph](specs.md)

- :material-shield-check: **Constitution**

    ---

    A layered document that captures project ground truth — stack, constraints,
    and conventions — so agents and humans never start cold.

    [:octicons-arrow-right-24: Constitution](constitution.md)

- :material-filter: **Authoring Funnel**

    ---

    A five-stage pipeline (Spark through Approve) that transforms rough ideas
    into execution-ready, structured specifications.

    [:octicons-arrow-right-24: Authoring Funnel](authoring.md)

- :material-gavel: **Decisions**

    ---

    Architectural decisions are first-class graph nodes with bidirectional edges
    to the specs they influence.

    [:octicons-arrow-right-24: Decisions](decisions.md)

- :material-shield-search: **Analytical Passes & Safety**

    ---

    Automated analysis passes that detect cycles, unreachable specs, missing
    verify criteria, and other structural problems before execution begins.

    [:octicons-arrow-right-24: Passes & Safety](passes.md)

- :material-puzzle: **Slices & Execution Units**

    ---

    Decompose creates independently claimable slice nodes in the graph — each
    with its own lifecycle from open through claimed to completed.

    [:octicons-arrow-right-24: Slices](slices.md)

- :material-swap-horizontal: **Drift Detection**

    ---

    Per-edge content hashing detects when upstream specs change after a
    dependency was baselined — keeping downstream assumptions honest.

    [:octicons-arrow-right-24: Drift Detection](drift.md)

- :material-check-decagram: **Spec Linting**

    ---

    Structural validation catches malformed specs, broken edges, and
    constitution violations before deeper analytical passes run.

    [:octicons-arrow-right-24: Spec Linting](linting.md)

</div>
