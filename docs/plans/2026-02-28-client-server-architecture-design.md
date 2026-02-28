# SpecGraph v2 Design: Client/Server Architecture

**Date:** 2026-02-28
**Status:** Approved (brainstorming)
**Supersedes:** Initial design session documents (v1.0-draft)

---

## Summary

Redesign SpecGraph from an embedded library with dual storage backends into a **client/server system** with a ConnectRPC API boundary. Key changes:

- **ConnectRPC API** between all clients (CLI, skills, MCP proxy, future UI) and the server
- **Memgraph** as the default graph storage backend (pluggable, Postgres+AGE as alternative)
- **Beads demoted** from core storage backend to sync adapter (like issue trackers)
- **Agent callbacks** — executing agents call back to the server directly for status updates and mid-task queries
- **Progressive context** — bundles are lean (spec + bootstrap + prime URL), agents pull detailed context from the server

---

## 1. System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  CLIENTS                                                     │
│                                                              │
│  specgraph CLI ─────┐                                        │
│  Claude Code skills ─┤── ConnectRPC (JSON/HTTP) ──┐          │
│  MCP server proxy ──┤                             │          │
│  Tauri+Svelte (future)┘                           │          │
│                                                    │          │
│                                              ┌─────▼────────┐│
│  SPECGRAPH SERVER                            │              ││
│                                              │  ConnectRPC  ││
│  ┌────────────────────────────────────────┐  │  Service     ││
│  │ Core Domain                            │  │  Layer       ││
│  │                                        │  │              ││
│  │  Spec Schema · Constitution            │  └──────┬───────┘│
│  │  Authoring Funnel · Execution Bundles  │         │        │
│  │  Decisions · Agent Collaboration       │         │        │
│  │  Graph Analysis                        │         │        │
│  └────────────────────┬───────────────────┘         │        │
│                       │                             │        │
│              ┌────────▼─────────┐                   │        │
│              │ Storage Backend  │◄──────────────────┘        │
│              │ (pluggable)      │                            │
│              │                  │                            │
│              │ ┌──────────────┐ │   ┌──────────────┐         │
│              │ │  Memgraph    │ │   │ Postgres+AGE │         │
│              │ │  (default)   │ │   │ (alternative)│         │
│              │ └──────────────┘ │   └──────────────┘         │
│              └──────────────────┘                            │
│                                                              │
│  SYNC ADAPTERS (outbound)                                    │
│  ┌──────┐  ┌────────┐  ┌────────┐  ┌──────────────┐         │
│  │Beads │  │GitHub   │  │Linear  │  │Tool Injection│         │
│  │(→GT) │  │Issues   │  │        │  │(CLAUDE.md,   │         │
│  │      │  │         │  │        │  │ .cursorrules)│         │
│  └──────┘  └────────┘  └────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### Key Architectural Decisions

- **Single server process** owns all domain logic and storage
- **ConnectRPC** chosen over gRPC for browser compatibility (JSON over HTTP, works with `fetch()`) while maintaining gRPC wire compatibility and protobuf type safety
- **No embedded/library mode** — always client/server
- **Beads is a sync adapter** alongside issue trackers, not a core backend

---

## 2. Storage & Graph Model

### Backend Options

Both backends speak Cypher. AGE is **required** when using Postgres (no CTE fallback).

| Backend | Query Language | Use Case |
|---------|---------------|----------|
| Memgraph (default) | Native Cypher | Solo, teams, general use |
| Postgres + AGE (required) | Cypher via AGE | Teams with existing Postgres infrastructure |

### Identity Scheme

Content-addressable hashing (merge-conflict-free IDs), type-prefixed:

| Entity | Prefix | Example |
|--------|--------|---------|
| Spec | `spec-` | `spec-k7m3p` |
| Decision | `dec-` | `dec-a7f3b2c1` |

### Node Types

```cypher
(:Spec {
  id: "spec-k7m3p",
  slug: "oauth-refresh-rotation",
  intent: "...",
  stage: "approved",        // spark | shape | specify | decompose | approved | in_progress | done
  priority: "p1",
  complexity: "medium",
  interface: { ... },       // JSON
  verify: [ ... ],
  invariants: [ ... ],
  constitution_ref: "v3",
  version: 1,
  created_at: datetime(),
  updated_at: datetime()
})

(:Decision {
  id: "dec-a7f3b2c1",
  title: "Refresh token storage mechanism",
  question: "...",
  chosen: "...",
  rationale: "...",
  rejected: [ ... ],
  confidence: "high",       // high | medium | low
  scope: "project",         // project | team | org
  status: "accepted",       // proposed | accepted | deprecated | superseded
  version: 1
})

(:Constitution {
  version: "v3",
  tech: { ... },
  principles: [ ... ],
  constraints: [ ... ],
  patterns: { ... },
  antipatterns: [ ... ]
})
```

### Edge Types

```cypher
// Spec relationships
(a:Spec)-[:DEPENDS_ON]->(b:Spec)
(parent:Spec)-[:COMPOSES]->(child:Spec)
(a:Spec)-[:REFERENCES]->(b:Spec)

// Decision relationships
(s:Spec)-[:DECIDED_IN]->(d:Decision)
(s:Spec)-[:REFERENCES_DECISION]->(d:Decision)
(d1:Decision)-[:SUPERSEDES]->(d2:Decision)

// Constitution
(s:Spec)-[:AUTHORED_AGAINST]->(c:Constitution)

// Claims
(s:Spec)-[:CLAIMED_BY {since: datetime(), lease_expires: datetime()}]->(a:Agent)
```

---

## 3. ConnectRPC API Surface

### Services

```protobuf
service SpecService {
  rpc CreateSpec(CreateSpecRequest) returns (Spec);
  rpc GetSpec(GetSpecRequest) returns (Spec);
  rpc UpdateSpec(UpdateSpecRequest) returns (Spec);
  rpc ListSpecs(ListSpecsRequest) returns (ListSpecsResponse);
  rpc GetDependencies(GetDepsRequest) returns (GetDepsResponse);
  rpc GetTransitiveDeps(GetDepsRequest) returns (GetDepsResponse);
  rpc GetCriticalPath(GetCriticalPathRequest) returns (CriticalPathResponse);
  rpc GetImpact(GetImpactRequest) returns (GetImpactResponse);
  rpc GetReady(GetReadyRequest) returns (ListSpecsResponse);
  rpc AddEdge(AddEdgeRequest) returns (AddEdgeResponse);
  rpc RemoveEdge(RemoveEdgeRequest) returns (RemoveEdgeResponse);
  rpc ClaimSpec(ClaimRequest) returns (ClaimResponse);
  rpc UnclaimSpec(UnclaimRequest) returns (UnclaimResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}

service AuthoringService {
  rpc Spark(SparkRequest) returns (Spec);
  rpc Shape(ShapeRequest) returns (Spec);
  rpc Specify(SpecifyRequest) returns (Spec);
  rpc Decompose(DecomposeRequest) returns (DecomposeResponse);
  rpc Approve(ApproveRequest) returns (Spec);
  rpc Amend(AmendRequest) returns (Spec);
  rpc Supersede(SupersedeRequest) returns (Spec);
}

service DecisionService {
  rpc CreateDecision(CreateDecisionRequest) returns (Decision);
  rpc GetDecision(GetDecisionRequest) returns (Decision);
  rpc ListDecisions(ListDecisionsRequest) returns (ListDecisionsResponse);
  rpc ReferenceDecision(ReferenceRequest) returns (ReferenceResponse);
  rpc SupersedeDecision(SupersedeDecisionRequest) returns (Decision);
  rpc GetDecisionImpact(GetDecisionImpactRequest) returns (GetImpactResponse);
}

service ConstitutionService {
  rpc GetConstitution(GetConstitutionRequest) returns (Constitution);
  rpc UpdateConstitution(UpdateConstitutionRequest) returns (Constitution);
  rpc GetPattern(GetPatternRequest) returns (Pattern);
  rpc CheckViolation(CheckViolationRequest) returns (ViolationResponse);
  rpc EmitToolFiles(EmitRequest) returns (EmitResponse);
}

service ExecutionService {
  rpc GenerateBundle(BundleRequest) returns (Bundle);
  rpc GetPrime(PrimeRequest) returns (PrimeResponse);
  rpc ReportProgress(ProgressRequest) returns (ProgressResponse);
  rpc ReportBlocker(BlockerRequest) returns (BlockerResponse);
  rpc ReportCompletion(CompletionRequest) returns (CompletionResponse);
}

service SyncService {
  rpc SyncBeads(SyncBeadsRequest) returns (SyncResponse);
  rpc SyncTracker(SyncTrackerRequest) returns (SyncResponse);
  rpc GetSyncStatus(SyncStatusRequest) returns (SyncStatusResponse);
}

service ServerService {
  rpc Health(HealthRequest) returns (HealthResponse);
}
```

### CLI Mapping

```
specgraph serve              → starts the server (not an RPC)
specgraph spark <slug>       → AuthoringService/Spark
specgraph shape <slug>       → AuthoringService/Shape
specgraph ready              → SpecService/GetReady
specgraph bundle <slug>      → ExecutionService/GenerateBundle
specgraph deps <slug>        → SpecService/GetDependencies
specgraph critical-path      → SpecService/GetCriticalPath
specgraph decision list      → DecisionService/ListDecisions
specgraph constitution show  → ConstitutionService/GetConstitution
specgraph sync beads         → SyncService/SyncBeads
```

---

## 4. Deployment & Configuration

### Three Deployment Modes

```
Solo/local:     specgraph serve (docker mode)  → server + storage in containers
Contributor:    specgraph serve (external mode) → local server process, existing DB
Team member:    CLI only (remote mode)          → points to shared server
```

### Configuration

```yaml
# .specgraph/config.yaml

# Mode A: Run the server locally
server:
  mode: docker          # docker | external
  host: "0.0.0.0"
  port: 9090

# Mode B: Connect to shared server (no serve needed)
# server:
#   remote: "https://specgraph.internal.company.com:9090"

storage:
  backend: memgraph     # memgraph | postgres
  docker:
    compose_file: .specgraph/docker-compose.yaml
  memgraph:
    bolt_uri: "bolt://localhost:7687"
  # postgres:
  #   url: "postgres://..."   (AGE extension required)

constitution:
  source: .specgraph/constitution.yaml

agent:
  posture: partner
  passes:
    red_team: offer
    peripheral_vision: offer
    consistency_check: auto
    simplicity_check: offer

sync:
  beads:
    enabled: false
    trigger: on_approve
  trackers: []
  toolchain:
    emit: []
```

### Docker Mode

`specgraph serve` manages a docker-compose stack with both the SpecGraph server and storage:

```yaml
# .specgraph/docker-compose.yaml (auto-generated)
services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "9090:9090"
    volumes:
      - ./constitution.yaml:/etc/specgraph/constitution.yaml
    depends_on:
      memgraph:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=memgraph
      - SPECGRAPH_STORAGE_BOLT_URI=bolt://memgraph:7687

  memgraph:
    image: memgraph/memgraph:latest
    ports:
      - "7687:7687"
    volumes:
      - specgraph-data:/var/lib/memgraph
    healthcheck:
      test: ["CMD", "mgconsole", "--execute", "RETURN 1;"]
      interval: 5s
      retries: 5

volumes:
  specgraph-data:
```

### Init Flow

```bash
specgraph init
# 1. Run your own server or connect to existing? [local / remote]
# If remote: server URL → validate → done
# If local:
#   2. Storage backend? [memgraph / postgres]
#   3. Mode? [docker / external]
#   4. If external: connection details
#   5. Generates config + compose (if docker)
```

---

## 5. Execution Bundles & Agent Callbacks

### Bundle Format

Bundles are lean: the spec, bootstrap instructions, and callback URLs. Agents pull detailed context from the prime endpoint.

```yaml
# Generated by: specgraph bundle spec-k7m3p

# ── Bootstrap ──────────────────────────────────────────────
bootstrap: |
  You are executing a SpecGraph specification. This bundle contains a fully
  designed work unit — the intent, interface contract, verification criteria,
  and key decisions have already been made. Your job is to implement it.

  ## Before you start
  Call the prime endpoint below to get project context, coding conventions,
  relevant files, and available operations:

    POST {{callbacks.endpoint}}{{callbacks.prime}}

  The prime response will tell you:
  - Project tech stack and conventions
  - Which files to modify and patterns to follow
  - How to report progress, blockers, and completion
  - Resolved decisions (don't re-decide these)
  - Upstream interfaces you can depend on

  ## Workflow
  1. Read the spec below (intent, interface, verify, invariants)
  2. Call prime to get project context and callback operations
  3. Implement per the spec and conventions
  4. Report progress as you go
  5. Verify against all criteria in the spec
  6. Report completion when all verify criteria pass

  ## Rules
  - Do NOT make design decisions — they're already made (see decisions)
  - Do NOT deviate from the interface contract
  - If something is unclear or blocked, report a blocker — don't guess
  - Follow the project conventions from the prime response

# ── Spec ───────────────────────────────────────────────────
spec:
  id: "spec-k7m3p"
  slug: "oauth-refresh-rotation"
  intent: |
    Implement automatic refresh token rotation per RFC 6749.
  stage: approved
  priority: p1
  complexity: medium
  interface:
    input: { endpoint: "POST /auth/token", body: { grant_type, refresh_token } }
    output: { success: { access_token, refresh_token, expires_in }, errors: [...] }
    side_effects: ["Previous refresh token invalidated"]
  verify:
    - "Refresh token rotation returns new token pair"
    - "Old refresh token rejected after rotation"
    - "Concurrent rotation: exactly one succeeds"
    - "Token family reuse → revoke entire family"
  invariants:
    - "Token store never contains two valid refresh tokens for same session"
  decisions:
    decided: [dec-a7f3b2c1]
    references: [dec-e1a4b7c8]

# ── Callbacks ──────────────────────────────────────────────
callbacks:
  endpoint: "https://specgraph.internal.company.com:9090"
  prime: "/prime/spec-k7m3p"
```

### Prime Endpoint

Returns tailored orientation for the executing agent:

- What SpecGraph is and how to call it (JSON POST examples)
- Constitution excerpt filtered for this spec's domain
- Codebase context (relevant files, patterns to follow, test commands)
- Resolved decisions with rationale
- Upstream spec interfaces the agent can depend on

### Injection

Bundles can be delivered as files for tool-specific injection:

```
.specgraph/bundles/spec-k7m3p/
  PROMPT.md          # bootstrap instructions
  bundle.yaml        # spec + callbacks
```

```bash
specgraph inject spec-k7m3p --tool=claude-code
# Writes PROMPT.md into workspace context
```

### Callback Flow

```
1. Bundle generated → synced to Beads (if enabled) or injected into workspace
2. Agent reads bundle, sees prime URL
3. Agent calls prime → gets orientation + context
4. Agent executes, calling back to report progress/blockers
5. Agent reports completion
6. Server updates spec status, notifies sync adapters
```

---

## 6. Sync Adapters

All sync adapters implement a common interface:

```go
type SyncAdapter interface {
    Sync(ctx context.Context, specs []Spec, decisions []Decision) (*SyncResult, error)
    Poll(ctx context.Context) ([]ExternalChange, error)
    Status(ctx context.Context) (*AdapterStatus, error)
}
```

### Beads Adapter

- **Direction:** Push-only. Approved specs → Beads issues. Status flows back via agent callbacks to the ConnectRPC API, not through Beads.
- **What syncs:** Specs as issues (type: `spec`), bundle bootstrap + callback URLs as message thread, dependency edges as Beads links
- **ID mapping:** `spec-k7m3p` ↔ `bd-xxxxxx`, stored as a property on the spec node in the graph

### Issue Tracker Adapters

- GitHub Issues, Linear, ADO, Jira via the same SyncAdapter interface
- Syncs: slug/title, intent, status, priority, deps (as linked issues), verify (as checklists)
- Does not sync: interface contracts, invariants, decisions, constitution, codebase context
- Direction: configurable per tracker (push, pull, bidirectional)

### Tool Injection Adapter

- Generates CLAUDE.md, .cursorrules, AGENTS.md from the constitution
- Handles spec-specific injection into workspaces

---

## 7. Unchanged from Original Design

These survive as-is:

- **Spec schema** — progressive structure (3 required → full schema), same fields
- **Constitution** — layered ground truth (User → Org → Project → Domain)
- **Authoring funnel** — Spark → Shape → Specify → Decompose → Approve
- **Decisions as first-class nodes** (ADR-003) — `dec-` prefixed, bidirectional edges
- **Agent collaboration** — three postures, five modes, four analytical passes, safety net

---

## 8. Revised Roadmap

### Phase 1 — Foundation

| # | Item | Notes |
|---|------|-------|
| 1 | Protobuf schema (`.proto` files) | API contract — all message types and service definitions |
| 2 | Spec schema as JSON Schema | Validation, progressive structure |
| 3 | Constitution schema | Layered ground truth |
| 4 | Server scaffold | Go + ConnectRPC, config loading, health endpoint |
| 5 | Memgraph storage backend | Graph model, Cypher queries, CRUD |
| 6 | Postgres+AGE storage backend | Same interface, AGE required |
| 7 | SpecService + DecisionService | Core CRUD, graph queries, ready detection, claims |
| 8 | CLI client | Thin client over ConnectRPC |
| 9 | Docker compose + `specgraph serve` | All three deployment modes |
| 10 | Execution bundle + bootstrap + prime | Bundle generation, prime endpoint |
| 11 | `specgraph init` flow | Interactive setup |

### Phase 2 — Authoring & Integration

| # | Item | Notes |
|---|------|-------|
| 12 | AuthoringService | Spark → Shape → Specify → Decompose → Approve |
| 13 | Codebase scanner | Three context tiers, feeds prime endpoint |
| 14 | ConstitutionService | Query patterns, check violations, emit tool files |
| 15 | Agent collaboration (postures + passes) | Drive/Partner/Support, Red Team, etc. |
| 16 | Beads sync adapter | Push-only, approved specs → beads |
| 17 | Claude Code skills + plugin | Authoring funnel as `/specgraph-*` skills |
| 18 | Tool injection adapter | CLAUDE.md, .cursorrules generation |

### Phase 3 — Coordination & Export

| # | Item | Notes |
|---|------|-------|
| 19 | ExecutionService callbacks | Progress, blockers, completion from agents |
| 20 | Lease/heartbeat model | Claim expiry, automatic unclaim |
| 21 | Issue tracker adapters | GitHub, Linear — via SyncAdapter interface |
| 22 | Drift detection | Interface drift, decision currency, dep staleness |
| 23 | ADR/document export | Decisions → ADRs, spec graph → Mermaid diagrams |

### Phase 4 — Scale & UI

| # | Item | Notes |
|---|------|-------|
| 24 | Tauri 2 + Svelte UI | Browser UI against ConnectRPC API |
| 25 | Federation | Remote specs, cross-team dependencies |
| 26 | Multi-repo support | Monorepo and polyrepo topologies |
| 27 | Metrics + reporting | Throughput, cycle time, spec health |

---

## 9. Open Questions

1. **Tauri+Svelte UI repo placement.** Monorepo with the Go server? Separate repo? Needs decision before Phase 4.

2. **Memgraph BSL licensing.** BSL 1.1 allows internal business use. Cannot embed/redistribute Memgraph in SpecGraph distribution. Docker compose referencing the official image is fine. Converts to Apache 2.0 in February 2030.

3. **Bundle format.** YAML chosen for human readability + machine parseability. Confirm this is the right call vs JSON or markdown with YAML frontmatter.

4. **MCP server role.** With ConnectRPC serving all clients, the MCP server becomes a thin proxy translating MCP tool calls → ConnectRPC RPCs. Confirm this is sufficient vs a dedicated MCP implementation.

5. **Beads sync granularity.** Push on approve is the default trigger. Should we also support pushing on other stage transitions (e.g., for visibility into the authoring pipeline)?

6. **Authentication/authorization.** The shared server mode implies multi-user access. Auth model (API keys, OAuth, mTLS) needs design before team deployments.

---

## 10. Relationship to Original Design Documents

This design supersedes the architectural and storage decisions in:
- `specgraph-v1.0-draft-spec.md` — architecture sections
- `specgraph-v1.0-draft-adr-001-storage.md` — entirely (new storage model)
- `specgraph-v1.0-draft-adr-002-gastown.md` — integration model (Beads sync adapter replaces native coupling)

The following survive with minor updates:
- `specgraph-v1.0-draft-spec.md` — spec schema, constitution, authoring funnel, agent collaboration
- `specgraph-v1.0-draft-adr-003-decisions.md` — decisions model unchanged
- `specgraph-v1.0-draft-roadmap.md` — superseded by revised roadmap above
