# Roadmap

## Philosophy

SpecGraph delivers in vertical slices. Each slice is end-to-end and shippable — you can stop after any slice and have a working system. This is not a traditional roadmap where Phase 1 is "useless without Phase 2." Every slice adds real, standalone value. If you adopt SpecGraph today and never upgrade, what you have still works.

## Phase 1 — Foundation

The core system: spec schema, storage, authoring, and execution primitives.

| Slice | Delivers | Status |
|-------|----------|--------|
| 1 | Core proto schema, ConnectRPC server, Memgraph backend, CLI basics | Done |
| 2 | Constitution creation/storage/validation, codebase scanner Tier 0, emitters | Done |
| 3 | Full authoring funnel (Spark → Approve), AI postures, analytical passes, safety net, CLI commands, Tier 1-2 codebase scanner | In Progress |
| 4 | Execution bundles, prime endpoint, agent callbacks, claim leasing | Planned |
| 5 | Spec lifecycle (amend/supersede/abandon), drift detection, spec linter | Planned |
| 6 | Sync adapters (Beads, GitHub Issues, Linear), tool injection (CLAUDE.md, .cursorrules) | Planned |
| 7 | Claude Code plugin (skills, hooks, MCP integration) | Planned |

After Phase 1, you have a complete spec-driven development system: author specs with AI assistance, store them in a graph, query dependencies, claim work, and sync to your issue tracker.

## Phase 2 — Authoring & CLI Integration

Polishes the authoring experience. Codebase scanner improvements for deeper code understanding, authoring flow refinements based on Phase 1 feedback, Claude Code skills and plugin for IDE integration, constitution sync with existing project configurations. The goal is to make the spec authoring loop feel natural — fast enough that developers reach for it before writing code, not after.

## Phase 3 — Coordination & Export

Multi-agent coordination. Multi-agent claim leasing with lease renewal and conflict resolution, MCP server for any MCP-compatible client, drift detection between specs and implementation, structured exports (ADRs, reports), Gastown integration for multi-agent orchestration. This phase turns SpecGraph from a single-developer tool into a team-scale coordination layer.

## Phase 4 — Scale

Enterprise features. Federation across multiple SpecGraph instances, multi-repo support, metrics and analytics dashboards, governance workflows with approval chains. Phase 4 addresses organizations where specs span teams, repositories, and deployment boundaries.

## Design Philosophy

Each phase builds on the previous one, but each slice within a phase is independently valuable. SpecGraph is designed for incremental adoption — start with the spec schema and constitution, add authoring when ready, layer on sync and coordination as your team grows. You never have to buy the whole vision to get value from what exists today.
