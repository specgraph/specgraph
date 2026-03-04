# How It Works

SpecGraph rests on five pillars: a **constitution** that captures your project's
ground truth, a **codebase scanner** that grounds specs in what actually exists,
a **spec schema** that represents work as graph nodes with first-class edges,
an **AI-collaborative authoring funnel** that guides ideas from rough spark to
execution-ready specification, and a **storage + query layer** that makes every
spec live and queryable. This page walks through each pillar at a high level and
shows how they fit together.

---

## The Constitution

Every SpecGraph project begins with a constitution — a layered document that
records the decisions, constraints, and conventions that define how the project
works. The constitution has four layers, from most general to most specific:

**User &rarr; Org &rarr; Project &rarr; Domain**

The **User** layer captures personal preferences (editor, language defaults).
The **Org** layer records organization-wide standards (security policies, CI
requirements). The **Project** layer pins the tech stack, repo structure, and
architectural principles. The **Domain** layer captures bounded-context details
— naming conventions, invariants, and patterns that apply to a specific part of
the codebase.

More specific layers override more general ones. If your org says "use REST" but
your project constitution says "use ConnectRPC," the project layer wins. This
means agents never start cold: before writing a single line of code, they can
query the constitution to understand what technology to use, what patterns to
follow, and what constraints to respect.

[:octicons-arrow-right-24: Deep dive into the constitution](concepts/constitution.md)

---

## Codebase Context

The constitution tells SpecGraph what your project *values*. The codebase
scanner tells it what your project *is*. During authoring, SpecGraph
progressively gathers context about the actual codebase so that specs are
grounded in reality — not written in a vacuum.

The scanner operates at three tiers, each deeper than the last:

| Tier | Name | When | What It Gathers |
|------|------|------|-----------------|
| **0** | Orientation | On init | Languages, frameworks, directory structure, build and test commands — enough to have a conversation about the project. |
| **1** | Navigation | During Shape | Module and service boundaries, key interfaces, dependency graph, existing patterns and conventions — enough to author well-scoped specs. |
| **2** | Deep | During Specify | File-level understanding of the areas a spec touches: existing handlers, data models, test helpers, deployment details — enough to write interface contracts that match what actually exists. |

Each tier builds on the previous one, and context is gathered only when needed.
Tier 0 runs once during project initialization. Tier 1 activates during the
Shape stage, when the spec needs to understand how the codebase is organized.
Tier 2 activates during Specify, focused narrowly on the files and modules the
spec will actually touch.

This progressive approach keeps the scanner fast and focused. A spec about a new
API endpoint doesn't need to understand the entire codebase — it needs to know
the router pattern, the existing middleware stack, the test conventions, and the
data models it will interact with. Tier 2 gathers exactly that, so the spec's
interface contract reflects the code that already exists rather than inventing
new patterns.

---

## Specs as a Graph

In SpecGraph, every specification is a **node** in a graph database. Relationships
between specs are **first-class edges**, not fragile filename references or
hand-maintained lists:

- **`depends_on`** — this spec requires another spec to be complete first
- **`blocks`** — this spec prevents another from starting
- **`composes`** — this spec is a parent that breaks down into child specs

Because relationships are edges in a graph, you can query them: "show me every
spec blocked by this one," "find all leaf specs with no dependencies," or
"detect cycles in the dependency tree."

```text
┌──────────────┐
│  auth-api    │
│  (approved)  │
└──────┬───────┘
       │ depends_on
       ▼
┌──────────────┐     blocks      ┌──────────────┐
│  user-store  │ ──────────────▶ │  migration   │
│  (in-progress)│                │  (pending)   │
└──────┬───────┘                 └──────────────┘
       │ composes
       ▼
┌──────────────┐
│  user-cache  │
│  (draft)     │
└──────────────┘
```

Specs also support **progressive structure**. A solo developer can start with
just three fields — title, status, and a description — while a large team can
use the full schema with interface contracts, verify criteria, and invariants.
Every spec has a **content-addressable identity**: its ID is derived from its
content, so you can detect when a spec has changed and track its history without
relying on mutable names or paths.

[:octicons-arrow-right-24: Deep dive into the spec schema](concepts/specs.md)

---

## The Authoring Funnel

Ideas don't arrive execution-ready. The authoring funnel is a five-stage
pipeline that adds structure and validation at each step:

| Stage | Purpose |
|---|---|
| **Spark** | Capture the raw idea — a sentence, a bug report, a feature request. No structure required. |
| **Shape** | Scope the work. Identify tradeoffs, surface risks, and decide what's in and out. |
| **Specify** | Define the interface contract, verify criteria, and invariants. The spec becomes machine-readable. |
| **Decompose** | Break large specs into smaller, independently-deliverable slices connected by `composes` edges. |
| **Approve** | Freeze the spec for execution. After approval, the spec is immutable and claimable. |

Each stage can be driven by a human, an AI agent, or both. SpecGraph defines
three **AI postures** that control how much initiative the agent takes:

- **Drive** — the agent leads; the human reviews and approves.
- **Partner** — human and agent collaborate interactively (the default).
- **Support** — the human leads; the agent answers questions and fills gaps.

The posture can change at any stage. You might let the agent drive during Spark
to brainstorm, switch to Partner for Specify to nail down interfaces together,
and take Support during Approve to keep the human in full control.

[:octicons-arrow-right-24: Deep dive into the authoring funnel](concepts/authoring.md)

---

## Execution-Ready Output

When a spec reaches the **Approved** stage, it becomes a claimable work unit.
Each approved spec carries everything an executor — human or agent — needs to
act without further clarification: **verify criteria** that define "done,"
**invariants** that must hold before and after execution, and **interface
contracts** that specify inputs and outputs. Dependencies are explicit graph
edges, so the executor knows exactly what must be complete before starting.

Agents (or humans) **claim** an approved spec, locking it to prevent duplicate
work. They execute against the verify criteria and report completion. If the
spec's invariants are violated or the verify criteria aren't met, the claim
fails and the spec returns to the pool. This creates a tight feedback loop:
specs are precise enough to execute mechanically, and the graph structure
ensures work proceeds in the right order.

---

## Putting It Together

The full pipeline flows from ground truth through structured authoring to
execution:

```text
                         SpecGraph Pipeline
  ═══════════════════════════════════════════════════════

  ┌─────────────────┐     ┌─────────────────┐
  │   Constitution   │     │ Codebase Scanner │
  │  U → O → P → D  │     │ Tier 0 → 1 → 2  │
  │  (the rules)     │     │ (the reality)    │
  └────────┬────────┘     └────────┬────────┘
           │ informs                │ grounds
           └──────────┬─────────────┘
                      ▼
  ┌─────────────────────────────────────────────────┐
  │              Authoring Funnel                    │
  │                                                  │
  │  Spark → Shape → Specify → Decompose → Approve  │
  │                                                  │
  │  AI postures: Drive | Partner | Support          │
  └────────────────────────┬────────────────────────┘
                           │ produces
                           ▼
  ┌─────────────────────────────────────────────────┐
  │              Spec Graph (Storage)                │
  │                                                  │
  │   Nodes: specs, decisions, constitution layers   │
  │   Edges: depends_on, blocks, composes            │
  │   Query: Cypher over Memgraph / Postgres+AGE     │
  └────────────────────────┬────────────────────────┘
                           │ serves
                           ▼
  ┌─────────────────────────────────────────────────┐
  │              Execution                           │
  │                                                  │
  │   Claim → Execute → Verify → Complete            │
  │   Agents or humans consume approved specs        │
  └─────────────────────────────────────────────────┘
```

The constitution and codebase scanner feed context into every authoring session
— the rules and the reality. The funnel produces structured specs that land in
the graph. The graph serves approved specs to executors. Each layer builds on
the one before it, and every artifact is queryable at every stage.
