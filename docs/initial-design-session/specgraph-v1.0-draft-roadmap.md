# SpecGraph Implementation Roadmap

**Version:** 1.0-draft
**Date:** 2025-02-25
**Status:** Planning
**Companion to:** specgraph-v1.0-draft-spec.md (main specification)

---

## Overview

Four phases, each building on the last. Phase 1 is the foundation everything else depends on. Phases 2 and 3 have independent workstreams that can overlap. Phase 4 is enterprise scaling — defer until the core is proven.

```mermaid
gantt
    title SpecGraph Implementation Phases
    dateFormat YYYY-MM-DD
    axisFormat %b %Y

    section Phase 1 — Foundation
    Spec schema + evolution fields       :p1_schema,    2025-03-01, 3w
    Constitution schema + init           :p1_const,     after p1_schema, 2w
    Beads backend integration            :p1_beads,     after p1_schema, 4w
    Postgres backend + schema            :p1_pg,        after p1_schema, 4w
    Claim protocol (both backends)       :p1_claim,     after p1_beads, 2w
    Execution bundle format              :p1_bundle,    after p1_const, 2w
    Core CLI                             :p1_cli,       after p1_claim, 3w
    Spec linter                          :p1_lint,      after p1_cli, 1w
    Backend migration command            :p1_migrate,   after p1_lint, 2w

    section Phase 2 — Authoring
    Codebase scanner                     :p2_scan,      after p1_const, 3w
    Authoring flow (spark→approve)       :p2_author,    after p2_scan, 4w
    Amendment re-entry                   :p2_amend,     after p2_author, 2w
    Agent prompt templates               :p2_prompts,   after p2_author, 3w
    Decision + red-team capture          :p2_redteam,   after p2_prompts, 2w

    section Phase 2 — CLI Agent Integration
    Constitution sync                    :p2_sync,      after p1_const, 3w
    specgraph inject/cleanup             :p2_inject,    after p1_bundle, 2w
    Claude Code skills                   :p2_skills,    after p2_author, 4w
    SessionStart hook                    :p2_hook,      after p2_skills, 1w
    Claude Code plugin packaging         :p2_plugin,    after p2_hook, 2w
    Cursor rules generation              :p2_cursor,    after p2_sync, 2w

    section Phase 3 — Multi-Agent
    Lease/heartbeat model                :p3_lease,     after p1_claim, 3w
    Command side-effects                 :p3_sideeff,   after p3_lease, 2w
    MCP server                           :p3_mcp,       after p2_inject, 4w

    section Phase 3 — Evolution
    Drift detection                      :p3_drift,     after p2_amend, 3w

    section Phase 3 — Document Export
    ADR generation                       :p3_adr,       after p2_redteam, 2w
    Mermaid + PlantUML diagrams          :p3_diagrams,  after p1_cli, 3w
    Design docs, RFC, changelog          :p3_docs,      after p3_adr, 2w
    Auto-export on lifecycle events      :p3_autoexp,   after p3_docs, 2w

    section Phase 3 — External Integration
    Gastown integration (Beads path)     :p3_gastown,   after p3_sideeff, 4w
    Issue tracker sync                   :p3_tracker,   after p3_sideeff, 3w
    Apache AGE support                   :p3_age,       after p1_pg, 3w

    section Phase 4 — Scale
    Federation                           :p4_fed,       after p3_mcp, 4w
    Multi-repo support                   :p4_multi,     after p4_fed, 3w
    Metrics + reporting                  :p4_metrics,   after p3_tracker, 3w
    Governance workflows                 :p4_gov,       after p4_fed, 3w
```

## Phase Dependencies

```mermaid
graph LR
    subgraph "Phase 1 — Foundation"
        schema[Spec Schema]
        const[Constitution]
        beads[Beads Backend]
        pg[Postgres Backend]
        claim[Claim Protocol]
        bundle[Bundle Format]
        cli[Core CLI]
        lint[Linter]
        migrate[Migration]

        schema --> beads
        schema --> pg
        schema --> const
        const --> bundle
        beads --> claim
        pg --> claim
        claim --> cli
        cli --> lint
        lint --> migrate
    end

    subgraph "Phase 2 — Authoring"
        scan[Codebase Scanner]
        author[Authoring Flow]
        amend[Amendment Re-entry]
        prompts[Prompt Templates]
        redteam[Decision + Red Team]

        const --> scan
        scan --> author
        author --> amend
        author --> prompts
        prompts --> redteam
    end

    subgraph "Phase 2 — CLI Agent"
        csync[Constitution Sync]
        inject[Inject / Cleanup]
        skills[Claude Code Skills]
        hook[SessionStart Hook]
        plugin[Plugin Packaging]
        cursor[Cursor Rules]

        const --> csync
        bundle --> inject
        author --> skills
        skills --> hook
        hook --> plugin
        csync --> cursor
    end

    subgraph "Phase 3 — Coordination"
        lease[Lease / Heartbeat]
        sideeff[Command Side-Effects]
        mcp[MCP Server]

        claim --> lease
        lease --> sideeff
        inject --> mcp
    end

    subgraph "Phase 3 — Evolution"
        drift[Drift Detection]
        amend --> drift
    end

    subgraph "Phase 3 — Export"
        adr[ADR Generation]
        diagrams[Mermaid + PlantUML]
        docs[Design Docs / RFC]
        autoexp[Auto-Export]

        redteam --> adr
        cli --> diagrams
        adr --> docs
        docs --> autoexp
    end

    subgraph "Phase 3 — Integration"
        gastown[Gastown]
        tracker[Issue Tracker Sync]
        age[Apache AGE]

        sideeff --> gastown
        sideeff --> tracker
        pg --> age
    end

    subgraph "Phase 4 — Scale"
        fed[Federation]
        multi[Multi-Repo]
        metrics[Metrics]
        gov[Governance]

        mcp --> fed
        fed --> multi
        fed --> gov
        tracker --> metrics
    end

    style schema fill:#3b82f6,color:white
    style bundle fill:#3b82f6,color:white
    style const fill:#3b82f6,color:white
    style author fill:#8b5cf6,color:white
    style skills fill:#8b5cf6,color:white
    style mcp fill:#f59e0b,color:white
    style drift fill:#f59e0b,color:white
    style adr fill:#f59e0b,color:white
    style fed fill:#6b7280,color:white
```

---

## Phase 1: Foundation

Everything downstream depends on this. No shortcuts.

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 1 | Spec schema as JSON Schema | — | Include evolution fields: `lifecycle`, `supersedes`, `amends`, `history` |
| 2 | Constitution schema + bootstrap | #1 | `specgraph init` flow |
| 3 | Beads backend integration | #1 | Custom spec type + `bd` CLI wrapper |
| 4 | Postgres backend + schema | #1 | Tables, indexes, version column |
| 5 | Claim protocol | #3, #4 | Optimistic concurrency, both backends |
| 6 | Execution bundle format | #2 | The contract between all layers |
| 7 | Core CLI | #5 | list, show, create, update, deps, next, claim, amend, supersede |
| 8 | Spec linter | #7 | Schema validation, edge consistency, constitution checks |
| 9 | Backend migration | #8 | `specgraph migrate --from=beads --to=postgres` (and reverse) |

**Highest-leverage items:** #1 (schema), #2 (constitution), #6 (bundle). These three define the contracts everything else builds on.

---

## Phase 2: Authoring, Context & CLI Agent Integration

Two parallel workstreams. Authoring builds the design experience. CLI agent integration brings it into the tools developers already use.

### Authoring

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 10 | Codebase scanner | #2 | `--scan` bootstrap, three context tiers |
| 11 | Authoring flow | #10 | spark → shape → specify → decompose → approve |
| 12 | Amendment re-entry | #11 | Done specs back into the funnel at shape/specify |
| 13 | Agent prompt templates | #11 | Per stage, posture, and analytical pass |
| 14 | Decision + red-team capture | #13 | Structured capture in spec schema |

### CLI Agent Integration

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 15 | Constitution sync | #2 | CLAUDE.md ↔ constitution ↔ .cursorrules ↔ AGENTS.md |
| 16 | `specgraph inject/cleanup` | #6 | Context injection for Claude Code, Cursor, Codex, OpenCode |
| 17 | Claude Code skills | #11 | Authoring funnel + operations as `/specgraph-*` |
| 18 | SessionStart hook | #17 | Awareness priming (the only hook) |
| 19 | Claude Code plugin | #18 | Skills + hook + MCP as single installable |
| 20 | Cursor rules generation | #15 | `specgraph init --tool=cursor` |

---

## Phase 3: Coordination, Export & Integration

Four independent workstreams. Can be prioritized based on team needs.

### Multi-Agent Coordination

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 21 | Lease/heartbeat model | #5 | Claim expiry, automatic unclaim |
| 22 | Command side-effects | #21 | complete→unblock, abandon→block, amend→flag drift |
| 23 | MCP server | #16 | Authoring agents + coding agents mid-task |

### Evolution

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 24 | Drift detection | #12 | `specgraph drift` — interface, verify, dependency |

### Document Export

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 25 | ADR generation | #14 | Spec decisions → Nygard-format ADRs |
| 26 | Mermaid + PlantUML diagrams | #7 | Deps, sequence, project graph, decomposition, critical path |
| 27 | Design docs, RFC, changelog | #25 | Full prose export from spec content |
| 28 | Auto-export on lifecycle | #27 | Trigger on approve, complete, amend, release |

### External Integration

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 29 | Gastown integration | #22 | Beads path only — specs as beads, Mayor dispatch |
| 30 | Issue tracker sync | #22 | GitHub, Linear, ADO, Jira — bidirectional or push |
| 31 | Apache AGE support | #4 | Optional graph queries on Postgres path (CTE fallback) |

---

## Phase 4: Scale & Federation

Defer until Phases 1–3 are proven in real use.

| # | Item | Depends On | Notes |
|---|------|-----------|-------|
| 32 | Federation | #23 | Remote specs, cross-team dependencies |
| 33 | Multi-repo support | #32 | Monorepo and polyrepo topologies |
| 34 | Metrics + reporting | #30 | Throughput, cycle time, spec health |
| 35 | Governance workflows | #32 | Role-based approvals, audit trail |

---

## Starting Point

If you're starting today, build these first — they unlock the most downstream value:

1. **Spec schema** (#1) — everything else is a function of the schema
2. **Constitution** (#2) — makes every subsequent spec better
3. **Execution bundle format** (#6) — the contract between authoring and execution
4. **Core CLI** (#7) — usable immediately for manual spec management
5. **Claude Code skills** (#17) — the authoring funnel inside the tool developers already use
