# Ecosystem

SpecGraph is the specification layer in a broader development ecosystem. It
produces execution-ready work units; many tools can consume them. SpecGraph
is deliberately execution-agnostic — it defines **what** to build, not
**who** builds it or **how** they coordinate. It works standalone. Add
execution, sync, and tooling integrations when your workflow demands them.

---

## Execution Landscape

SpecGraph's execution bundle format — structured specs with verify
criteria, invariants, interface contracts, and explicit dependency edges —
is designed for any execution environment. The spec layer is independent of
the execution layer.

- **[OpenAI Symphony](https://github.com/openai/symphony)** — orchestrates
  Codex agents against issue trackers and task graphs.
- **[Open SWE](https://github.com/langchain-ai/open-swe)** (LangChain) —
  async multi-agent workflows with sandboxed execution.
- **[OpenAI Codex](https://openai.com/index/introducing-codex/)** —
  desktop-native multi-agent orchestration with worktree isolation.
- **[Gastown](#gastown)** — coordinates Claude Code instances via tmux and
  worktrees.
- **Direct use in Claude Code, OpenCode, or Codex CLI** — a single
  developer claiming and implementing specs without an orchestration layer.

---

## Gastown

!!! note "Status: Planned"
    Gastown is designed but not yet built. **SpecGraph functions fully
    independently without Gastown.**

Gastown is one possible execution environment for SpecGraph specs — a
multi-agent workspace manager that coordinates Claude Code instances via
tmux, git worktrees, and Beads. An AI coordinator (the Mayor) reads the
spec graph, dispatches work to ephemeral worker agents (polecats) in
isolated worktrees, and sequences merge requests in dependency order.

The target integration: SpecGraph pushes specs to a shared Beads/Dolt
store. Gastown reads that store, surfaces approved specs via `bd ready`,
and dispatches them. This pipeline requires extending the Beads adapter
(currently push-only) with pull capabilities.

---

## Beads & Dolt

!!! info "Status: Shipped (push-only)"
    The Beads adapter pushes specs to Beads as issues. Pull (Beads to
    SpecGraph) is not implemented — the adapter can poll bead status but
    does not import specs.

SpecGraph pushes specs to [Beads](https://github.com/beads-project/beads)
as issues via the sync adapter. Each spec becomes a bead with the spec's
slug, intent, and stage. The adapter can poll bead status for coordination.

Dolt provides the underlying versioned storage — every write is a commit,
branching isolates concurrent work, and remotes enable distributed sync.

---

## Sync Adapters

!!! info "Status: Shipped (push-only)"
    Beads and GitHub adapters push specs out and can poll external status.
    Neither imports data back into SpecGraph. Linear is planned.

SpecGraph pushes specs to external trackers for visibility and coordination
with teams that do not use SpecGraph directly.

- **GitHub Issues** — Spec slug, title, intent, stage, priority, and
  complexity push to GitHub as issues. The adapter polls issue state but
  does not sync changes back. Full interface contracts and constitution
  references stay in SpecGraph — GitHub gets the summary.

- **Linear** (Planned) — Push-only or bidirectional sync to Linear issues
  and projects.

- **Tool Injection** — `specgraph inject <slug>` writes a spec's interface
  contracts, constraints, and relevant constitution layers into
  `CLAUDE.md`, `.cursor/rules`, or `AGENTS.md` so any coding agent picks
  up the right context automatically.

---

## Integration Points

SpecGraph connects to the outside world through four interfaces:

- **CLI** (Shipped) — `specgraph` is the primary interface. Author specs,
  query the graph, generate execution bundles, run the linter, and manage
  the constitution from the terminal.

- **Claude Code** (Shipped) — 11 skills and hooks integrate SpecGraph into
  the IDE workflow. Author specs through conversational prompts, query the
  graph, and generate execution bundles without leaving the editor.

- **MCP Server** (Planned) — Exposes SpecGraph operations to any
  MCP-compatible client. Authoring tools, coding agents, and custom
  workflows can read specs, advance funnel stages, and query the graph
  through the Model Context Protocol.

- **ConnectRPC API** (Shipped) — JSON-over-HTTP API backed by protobuf
  schemas. Any language, any platform. gRPC-compatible for high-throughput
  use cases.

---

## Composability

SpecGraph is independently useful without any execution orchestrator or
sync adapter. The CLI and a Memgraph backend provide a complete setup —
author specs, query the graph, run the linter, generate bundles.

Every integration is independently optional. Add an execution environment
when you want automated implementation. Add GitHub sync for PM visibility.
Add tool injection for coding-agent context. No combination is degraded —
every configuration is first-class. The specification layer and the
execution layer serve different purposes and evolve at different rates.
Keeping them independent avoids coupling your design process to any
particular agent framework.
