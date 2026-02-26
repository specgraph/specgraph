# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SpecGraph is a **Live Spec-Driven Development Framework** — a system for managing software specifications as a queryable graph rather than static markdown files. It provides a constitution (project ground truth), a spec schema, an authoring process with AI collaboration, and a live storage + query layer that feeds execution engines like Gastown.

**Status:** Design phase (Phase 0). No implementation code exists yet — only specification documents, ADRs, and a roadmap.

## Repository Structure

```
specgraph/
├── docs/initial-design-session/
│   ├── specgraph-v1.0-draft-spec.md       # Main specification
│   ├── specgraph-v1.0-draft-roadmap.md    # 4-phase implementation plan
│   ├── specgraph-v1.0-draft-adr-001-storage.md   # Storage backend decision
│   ├── specgraph-v1.0-draft-adr-002-gastown.md   # Gastown integration
│   └── specgraph-v1.0-draft-adr-003-decisions.md # Decisions as first-class entities
└── LICENSE  # MIT
```

## Architecture

### Core Concepts

- **Specs are graph nodes**, not files. Dependencies, blocks, and compositions are first-class edges.
- **Progressive structure**: 3 required fields for solo devs, 30+ for enterprise. Same schema at all scales.
- **Constitution**: Layered project ground truth (User → Org → Project → Domain). More specific layers override general ones.
- **Authoring funnel**: Spark → Shape → Specify → Decompose → Approve. Produces specs clear enough for agents to execute without ambiguity.
- **Execution bundles**: The contract between authoring and execution engines.
- **Decisions are first-class nodes** (ADR-003) with bidirectional edges to specs.

### Storage Backends (ADR-001)

Two swappable backends with the spec schema as the contract:

| Backend | Multi-Agent | Graph Queries | Storage Model |
|---------|------------|---------------|---------------|
| Beads + Dolt | Native (specs as beads) | Via relations | Version-controlled |
| Postgres + AGE | Custom leasing protocol | Apache AGE (optional) | SQL rows |

### Relationship to Gastown

SpecGraph sits **upstream** of Gastown. SpecGraph does design; Gastown does execution. The authoring funnel produces specs that polecats (ephemeral worker agents) can execute without making decisions.

## Implementation Roadmap

- **Phase 1 — Foundation**: Spec schema, constitution, storage backends, claim protocol, execution bundles, core CLI, linter (~17-18 weeks)
- **Phase 2 — Authoring & CLI Integration**: Codebase scanner, authoring flow, Claude Code skills/plugin, constitution sync (~14 weeks, parallel workstreams)
- **Phase 3 — Coordination & Export**: Multi-agent leasing, MCP server, drift detection, document export, issue tracker sync, Gastown integration
- **Phase 4 — Scale**: Federation, multi-repo, metrics, governance

### Starting Point (per roadmap)

Build in this order for maximum downstream value:
1. Spec schema (JSON Schema) — everything depends on it
2. Constitution schema + `specgraph init`
3. Execution bundle format
4. Core CLI (`specgraph list|show|create|update|deps|next|claim|amend|supersede`)
5. Claude Code skills for authoring workflow
