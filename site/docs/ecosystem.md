# Ecosystem

SpecGraph is the design layer in a broader spec-driven development ecosystem. It
produces execution-ready specifications; other tools consume them. SpecGraph
works standalone — no other tool is required — but becomes more powerful when
integrated with execution and coordination systems.

---

## Gastown

!!! note "Status: Planned"
    Gastown is designed but not yet built. The integration described here
    is the target architecture. **SpecGraph functions fully independently
    without Gastown** — author specs, query the graph, run the linter, and
    generate execution bundles using just the CLI and a Memgraph backend.

Gastown is a multi-agent workspace manager that coordinates Claude Code instances
(and other agents) via tmux, git worktrees, and Beads. It sits downstream of
SpecGraph in the design-to-execution pipeline: SpecGraph decides **what** to
build and **how** to build it; Gastown decides **who** builds it and **when**.

Gastown organizes work through five key components. The **Mayor** is an AI
coordinator with full workspace context — it reads the spec graph, understands
priorities and dependencies, and dispatches work. **Polecats** are ephemeral
worker agents that spawn in isolated git worktrees, execute a single spec,
submit a merge request, and disappear. **Crew** members are persistent agents
for ongoing collaboration — code review, architecture guidance, or long-running
tasks that span multiple specs. The **Refinery** is a merge queue processor
that sequences and lands polecat MRs in dependency order. The **Witness**
monitors agent health, detects stalls, and reports progress.

SpecGraph's value proposition for Gastown is directness: the authoring funnel
produces specs clear enough that polecats can execute without making design
decisions. There is no adapter or bridge between the two systems. SpecGraph
writes specs as beads into the same Beads/Dolt store that Gastown reads. The
flow is linear: SpecGraph authors a spec → the spec reaches `approved` status →
`bd ready` surfaces it as available work → the Mayor dispatches it via
`gt sling` → the polecat reads the execution bundle (interface contracts,
constitution refs, verify items) → implements → submits an MR. This pipeline
requires the Beads storage backend. The Postgres path does not natively connect
to Gastown — teams using Postgres would need a sync adapter or manual execution
handoff.

---

## Beads & Dolt

!!! info "Status: Shipped"
    The Beads adapter pushes specs to Beads as issues. Pull (Beads to SpecGraph)
    is not yet implemented — the adapter can poll bead status but does not
    import or create specs from Beads.

Beads is a git-backed issue tracker with Dolt for versioned relational storage.
SpecGraph can push specs to Beads as issues via the `bd` CLI. The adapter
creates a bead issue with the spec's slug and intent, and can poll the bead's
status for coordination.

Dolt provides versioning (every write is a commit), branching (branch-per-agent
isolates concurrent work), cell-level merge (agents can modify different specs
without conflicts), and sync via remotes (distributed teams share a single
source of truth). When Gastown is built, the shared Beads/Dolt store becomes the
data plane connecting design and execution.

---

## Sync Adapters

!!! info "Status: Shipped (push-only)"
    Beads and GitHub adapters are implemented. Both push specs out and can poll
    external status, but do not import data back into SpecGraph. Linear is planned.

SpecGraph can push specs to external trackers for visibility and coordination
with teams that do not use SpecGraph directly.

- **GitHub Issues** — Push-only sync. Spec slug, title, intent, stage,
  priority, and complexity are pushed to GitHub as issues via the `gh` CLI.
  The adapter can poll issue state but does not sync changes from GitHub back
  into SpecGraph. Full interface contracts and constitution references stay in
  SpecGraph — GitHub gets the summary, SpecGraph keeps the detail.

- **Linear** (Planned) — Push-only or bidirectional sync. The same fields as
  GitHub would sync to Linear issues and projects.

- **Tool Injection** — Emit the constitution and per-spec context into
  coding-agent context files: `CLAUDE.md`, `.cursor/rules`, `AGENTS.md`. The
  command `specgraph inject <slug>` writes a spec's interface contracts,
  constraints, and relevant constitution layers into the current working
  directory so that any coding agent picks them up automatically.

---

## Integration Points

SpecGraph connects to the outside world through four interfaces. Each serves a
different audience and use case; all share the same server and data.

- **CLI** (Shipped) — `specgraph` is the primary interface for local use. Author specs,
  query the graph, generate execution bundles, run the linter, and manage the
  constitution from the terminal.

- **Claude Code** (Shipped) — 10 skills and hooks integrate SpecGraph into the IDE workflow.
  Author specs through conversational prompts (`/spark`, `/shape`, `/specify`,
  `/decompose`, `/approve`), query the graph (`/list`, `/show`, `/deps`,
  `/ready`), and generate execution bundles (`/bundle`) — all without
  leaving the editor.

- **MCP Server** (Planned — Phase 3) — Exposes SpecGraph operations to any MCP-compatible client.
  Authoring tools, coding agents, and custom workflows can read specs, advance
  funnel stages, and query the graph through the Model Context Protocol.

- **ConnectRPC API** (Shipped) — A JSON-over-HTTP API backed by protobuf schemas. Any
  language and any platform can integrate: create specs, query dependencies,
  read execution bundles, and manage constitutions. gRPC-compatible for
  high-throughput use cases.

---

## Composability

Every integration is independently optional. SpecGraph with just a CLI and
Memgraph is a valid, fully functional setup — author specs, query the graph, run
the linter, generate bundles. Add Gastown for multi-agent execution. Add GitHub
sync for PM visibility. Add tool injection for coding-agent context. Add the MCP
server for cross-tool interoperability. No combination is "degraded mode" — every
configuration is first-class, and you adopt integrations only when your workflow
demands them.
