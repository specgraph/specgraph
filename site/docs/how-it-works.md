# How It Works

SpecGraph rests on four pillars: a **constitution** that captures project
ground truth, a **graph-native spec schema** that makes relationships
queryable, an **AI-collaborative authoring funnel** that guides ideas from
spark to execution-ready spec, and a **storage + query layer** that keeps
every artifact live. This page walks through each pillar and shows how they
fit together.

---

## The Constitution

Every SpecGraph project begins with a constitution — a layered document
that records the decisions, constraints, and conventions that define how
the project works. The constitution has four layers, from most general to
most specific:

**User &rarr; Org &rarr; Project &rarr; Domain**

The **User** layer captures personal preferences (editor, language
defaults). The **Org** layer records organization-wide standards (security
policies, CI requirements). The **Project** layer pins the tech stack, repo
structure, and architectural principles. The **Domain** layer captures
bounded-context details — naming conventions, invariants, and patterns that
apply to a specific part of the codebase.

More specific layers override more general ones. If the org constitution
says "use REST" but the project constitution says "use ConnectRPC," the
project layer wins. Agents never start cold: before writing a single line
of code, they query the constitution to understand what technology to use,
what patterns to follow, and what constraints to respect.

[:octicons-arrow-right-24: Deep dive into the constitution](concepts/constitution.md)

---

## Specs as a Graph

Every specification is a **node** in a graph database. Relationships
between specs are **first-class edges**, not filename references or
hand-maintained lists:

- **`depends_on`** — this spec requires another spec to be complete first
- **`blocks`** — this spec prevents another from starting
- **`composes`** — this spec is a parent that breaks down into child specs

Because relationships are graph edges, you can query them directly — "show
me every spec blocked by this one" is a single traversal. Every spec has a
**stable identity** (ULID-based) and a **content hash** (Murmur3-128
fingerprint) that changes when content changes, enabling drift detection
without field-by-field comparison.

[:octicons-arrow-right-24: See the full graph model](concepts/specs.md)

---

## The Authoring Funnel

Ideas do not arrive execution-ready. The authoring funnel is a five-stage
pipeline that adds structure and validation at each step:

| Stage | Purpose |
|---|---|
| **Spark** | Capture the raw idea — a sentence, a bug report, a feature request. No structure required. |
| **Shape** | Scope the work. Identify tradeoffs, surface risks, decide what is in and what is out. |
| **Specify** | Define the interface contract, verify criteria, and invariants. The spec becomes structured and claimable. |
| **Decompose** | Break large specs into smaller, independently deliverable slices connected by `composes` edges. |
| **Approve** | Freeze the spec for execution. After approval, the spec is immutable and claimable. |

Each stage can be driven by a human, an AI agent, or both. SpecGraph
defines three **AI postures** that control how much initiative the agent
takes:

- **Drive** — the agent leads; the human reviews and approves.
- **Partner** — human and agent collaborate interactively (the default).
- **Support** — the human leads; the agent answers questions and fills gaps.

The posture can change at any stage. Let the agent drive during Spark to
brainstorm, switch to Partner for Specify to nail down interfaces together,
and take Support during Approve to keep the human in full control.

[:octicons-arrow-right-24: Deep dive into the authoring funnel](concepts/authoring.md)

---

## Execution-Ready Output

When a spec reaches the **Approved** stage, it becomes a claimable work
unit. Each approved spec carries everything an executor — human or agent —
needs to act without further clarification: **verify criteria** that define
"done," **invariants** that must hold before and after execution, and
**interface contracts** that specify inputs and outputs. Dependencies are
explicit graph edges, so the executor knows exactly what must be complete
before starting.

Agents (or humans) **claim** an approved spec, locking it to prevent
duplicate work. They execute against the verify criteria and report
completion. If the invariants are violated or the criteria are not met, the
claim fails and the spec returns to the pool. The graph structure ensures
work proceeds in dependency order.

---

## Putting It Together

```text
                         SpecGraph Pipeline
  ═══════════════════════════════════════════════════════

  ┌─────────────────┐
  │   Constitution   │  Ground truth: stack, constraints,
  │  U → O → P → D  │  principles, patterns
  └────────┬────────┘
           │ informs
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
  │   Query: Cypher over Memgraph (Postgres planned) │
  └────────────────────────┬────────────────────────┘
                           │ serves
                           ▼
  ┌─────────────────────────────────────────────────┐
  │              Execution                           │
  │                                                  │
  │   Claim → In Progress → Review → Done            │
  │   Agents or humans consume approved specs        │
  │                                                  │
  │   Terminal: Amended | Superseded | Abandoned      │
  └─────────────────────────────────────────────────┘
```
