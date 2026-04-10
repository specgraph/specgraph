# Concepts

SpecGraph is built on a few core concepts. Each page in this section explains
one in detail — what it is, why it matters, and how it fits into the overall
framework.

<div class="grid cards" markdown>

- :material-shield-check: **Ground Truth**

    ---

    Your project's architectural context — tech stack, constraints, and
    conventions — encoded once so engineers and agents never start cold.

    [:octicons-arrow-right-24: Ground Truth](ground-truth.md)

- :material-graph: **The Spec Graph**

    ---

    Specifications are nodes in a queryable graph with first-class dependency
    edges — not files in a folder.

    [:octicons-arrow-right-24: The Spec Graph](spec-graph.md)

- :material-gavel: **Decisions**

    ---

    Architectural decisions are first-class graph nodes with bidirectional edges
    to the specs they influence.

    [:octicons-arrow-right-24: Decisions](decisions.md)

- :material-filter: **Authoring Funnel**

    ---

    A five-stage pipeline (Spark through Approve) that transforms rough ideas
    into execution-ready, structured specifications.

    [:octicons-arrow-right-24: Authoring Funnel](authoring.md)

- :material-shield-search: **Analytical Passes & Safety**

    ---

    Red team, peripheral vision, consistency, and simplicity checks —
    posture-aware analysis that runs at each authoring stage.

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

- :material-history: **Lifecycle Transitions**

    ---

    Amendment, supersession, and abandonment — what happens when in-flight
    or completed specs need to change, with full changelog and diff support.

    [:octicons-arrow-right-24: Lifecycle Transitions](lifecycle.md)

- :material-check-decagram: **Spec Linting**

    ---

    Structural validation catches malformed specs, broken edges, and
    constitution violations before deeper analytical passes run.

    [:octicons-arrow-right-24: Spec Linting](linting.md)

- :material-file-document-outline: **Example Spec**

    ---

    A fully annotated OAuth2 refresh token spec showing every field, edge type,
    and authoring stage output in context.

    [:octicons-arrow-right-24: Example Spec](example-spec.md)

</div>
