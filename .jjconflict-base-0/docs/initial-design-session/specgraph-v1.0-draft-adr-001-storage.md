# SpecGraph ADR-001 (v3): Architecture & Storage

## Core Principle: Composable, Not Coupled

SpecGraph has one mandatory core and three optional integrations. Every combination works:

```text
MANDATORY:
  SpecGraph core (schema, authoring, constitution, bundles, CLI/MCP)
  + ONE storage backend (Beads+Dolt OR Postgres+AGE)

OPTIONAL (independent of each other):
  + Gastown integration (multi-agent orchestration)
  + Issue tracker sync (GitHub Issues, Linear, ADO, Jira)
  + Tool injection (CLAUDE.md, .cursorrules, AGENTS.md generation)
```

Valid configurations:

| Setup | Backend | Gastown | Tracker Sync | Tool Inject |
|---|---|---|---|---|
| Solo dev, minimal | Beads | ✗ | ✗ | ✗ |
| Solo dev + Claude Code | Beads | ✗ | ✗ | ✓ |
| Solo dev + agent swarm | Beads | ✓ | ✗ | ✓ |
| Small team, enterprise PG | Postgres+AGE | ✗ | ✓ (Linear) | ✓ |
| Small team, Gastown | Beads | ✓ | ✗ | ✓ |
| Enterprise, full stack | Postgres+AGE | ✗ | ✓ (Jira) | ✓ |
| Enterprise, Gastown-native | Beads | ✓ | ✓ (GitHub) | ✓ |
| Open source project | Beads | ✗ | ✓ (GitHub) | ✓ |

Every row is a first-class citizen. No configuration is "degraded mode."

---

## The Core: What's Always There

Regardless of backend or integrations, SpecGraph always provides:

### Spec Schema

Progressive structure. Three required fields minimum, expanding to full schema:

```yaml
# Minimum viable spec (solo, just getting started)
spec: login-api
intent: "REST endpoint for OAuth2 login"
verify:
  - "Returns JWT on valid credentials"
  - "Returns 401 on invalid credentials"

# Full spec (team, complex feature)
spec: oauth-refresh-rotation
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
  - "Rotation returns new token pair"
  - "Old token rejected after rotation"
  - "Concurrent rotation: exactly one succeeds"
  - "Token family reuse → revoke entire family"
invariants:
  - "Never two valid refresh tokens for same session"
depends_on: [user-model]
decisions:
  - { decision: "Lineage chain tracking", rationale: "No schema migration needed", rejected: [...] }
```

### Authoring Funnel

Five stages (enter at any, skip any, go backward):

- **Spark** — vague idea → seed + questions
- **Shape** — scope, tradeoffs, risks, success criteria
- **Specify** — interface contract, verification criteria, invariants
- **Decompose** — break into implementable slices
- **Approve** — mark ready for execution

### Constitution

Project-wide ground truth. Layered (User → Org → Project → Domain):

```yaml
tech: { languages: [go], frameworks: [chi], infrastructure: [postgres, redis] }
principles: [backward-compat, no-shared-db, observability-first]
constraints: ["No new deps without review", "No direct DB from handlers"]
patterns:
  error_handling: "Return domain errors from service, map to HTTP in handler"
  testing: "Table-driven tests, testcontainers for integration"
antipatterns:
  - { pattern: "God service", why: "Untestable", instead: "One service per domain" }
```

### Codebase Context

Three tiers of understanding, gathered progressively:

- **Tier 0:** Languages, frameworks, directory structure, build/test commands
- **Tier 1:** Service boundaries, key interfaces, dependency graph, patterns
- **Tier 2:** File-level understanding for affected areas

### Execution Bundles

Self-contained context packages for whoever executes the spec (human, single agent, agent swarm, Gastown polecat):

```bash
specgraph bundle <slug>
```

Produces: spec + resolved deps + filtered constitution + relevant codebase context + upstream interfaces + test commands. This is the handoff protocol. It works the same whether the consumer is a Gastown polecat, a Claude Code session, a Cursor tab, or a human developer.

### Agent Collaboration

Three postures (Drive/Partner/Support), five collaboration modes, four analytical passes (Red Team, Peripheral Vision, Consistency Check, Simplicity Check). Always-on safety net for security/data-loss/contradiction issues.

### CLI + MCP Server

```bash
# Core operations (always available, any backend)
specgraph init [--backend=beads|postgres]
specgraph spark|shape|specify|decompose|approve <slug>
specgraph bundle <slug>
specgraph list|show|ready|blocked|next
specgraph deps|blocks|tree|critical-path <slug>
specgraph constitution show|edit|emit
specgraph context scan
```

---

## Storage: Two Paths, Pick One

### Path A: Beads (+Dolt)

Beads is a Dolt-backed task/issue management system. Specs are Beads issues with a custom `spec` type. Dolt provides versioning, branching, cell-level merge, sync via remotes.

**What Beads provides (SpecGraph doesn't reimplement):**

- Atomic claims (`bd claim`)
- Dependency tracking (`bd link --blocks`)
- Ready detection (`bd ready`)
- Hash-based IDs (zero merge conflicts)
- Branch-per-agent (Dolt branching)
- Sync (Dolt remotes, including Git remotes)
- Compaction (memory decay for old completed work)
- Embedded or server mode

**What SpecGraph adds on top:**

- Spec schema (structured interface, verify, invariants, decisions)
- Authoring funnel (spark → approve)
- Constitution + codebase context
- Execution bundles
- Graph analysis (critical path, transitive deps, impact — via Dolt SQL recursive CTEs)
- Analytical passes (red team, etc.)

**Graph queries via Dolt SQL (MySQL-compatible):**

```sql
-- Transitive dependencies
WITH RECURSIVE deps AS (
    SELECT target_id FROM beads_links
    WHERE source_id = ? AND link_type = 'blocks'
    UNION ALL
    SELECT l.target_id FROM beads_links l
    JOIN deps d ON l.source_id = d.target_id
    WHERE l.link_type = 'blocks'
)
SELECT i.* FROM beads_issues i JOIN deps d ON i.id = d.target_id;
```

**Progression within the Beads path:**

```text
Solo:        bd init (SQLite default) → zero infra
With agents: bd init --backend dolt  → branching, atomic claims
Team:        Dolt server mode        → multi-client, remote sync
```

### Path B: Postgres (+optional AGE)

Standard Postgres. Specs in tables. Edges in a join table or AGE graph. LISTEN/NOTIFY for events.

**What Postgres provides:**

- ACID transactions
- Atomic claims via `SELECT ... FOR UPDATE`
- LISTEN/NOTIFY for real-time events (no polling)
- Row-level locking
- JSON/JSONB columns for structured spec fields
- Full-text search
- Standard tooling (backups, monitoring, connection pooling)
- DBA-friendly (teams already know it)

**What AGE adds (optional extension, same Postgres instance):**

- Native openCypher graph queries alongside SQL
- Cleaner syntax for transitive deps, critical path, impact analysis
- Falls back to recursive CTEs if AGE not available

```cypher
-- With AGE: transitive dependencies
MATCH (s:Spec {slug: 'login-api'})-[:DEPENDS_ON*]->(dep:Spec)
RETURN dep.slug, dep.status
```

```sql
-- Without AGE: same result, recursive CTE
WITH RECURSIVE deps AS (
    SELECT to_slug FROM spec_edges WHERE from_slug = $1 AND edge_type = 'depends_on'
    UNION ALL
    SELECT e.to_slug FROM spec_edges e JOIN deps d ON e.from_slug = d.to_slug
    WHERE e.edge_type = 'depends_on'
)
SELECT s.* FROM specs s JOIN deps d ON s.slug = d.to_slug;
```

**Detection-based fallback:** On init, check if AGE extension is available. Use Cypher if yes, CTEs if no. Same query interface either way.

**Claims with leases:**

```sql
-- Atomic claim
UPDATE specs SET status = 'in-progress', owner = $agent, claimed_at = now(),
       lease_expires = now() + interval '30 minutes', version = version + 1
WHERE slug = $slug AND status = 'approved' AND owner IS NULL
  AND version = $expected_version
RETURNING *;
-- Returns 0 rows if already claimed or version mismatch
```

### Backend Interface (Abstract)

Both backends implement the same interface. SpecGraph core never talks to Beads or Postgres directly:

```python
class SpecBackend(Protocol):
    # CRUD
    def create(self, spec: Spec) -> str: ...
    def get(self, slug: str) -> Spec: ...
    def update(self, slug: str, spec: Spec, expected_version: int) -> None: ...

    # Graph
    def add_edge(self, from_slug: str, to_slug: str, edge_type: EdgeType) -> None: ...
    def deps(self, slug: str) -> list[Spec]: ...
    def transitive_deps(self, slug: str) -> list[Spec]: ...
    def critical_path(self, slug: str) -> list[Spec]: ...
    def impact(self, slug: str) -> list[Spec]: ...

    # Coordination
    def ready(self) -> list[Spec]: ...
    def claim(self, slug: str, agent_id: str) -> bool: ...
    def unclaim(self, slug: str) -> None: ...
    def heartbeat(self, slug: str) -> None: ...

    # Events
    def subscribe(self, event_types: list[str]) -> EventStream: ...

    # Query
    def list(self, filters: Filters) -> list[Spec]: ...
    def search(self, query: str) -> list[Spec]: ...

    # Context
    def execution_bundle(self, slug: str) -> Bundle: ...
```

### Migration Between Backends

```bash
specgraph migrate --from=beads --to=postgres --url="postgres://..."
specgraph migrate --from=postgres --to=beads
```

One-way door at a time. No dual-write. Migration reads from source, writes to target, verifies, flips config.

---

## Optional Integration: Gastown

See ADR-002 for full details. Summary:

When Gastown is enabled, SpecGraph specs are beads that Gastown natively reads. The Mayor coordinates, polecats execute against execution bundles, the Refinery merges. SpecGraph formulas encode the design→implement→verify→submit lifecycle as Gastown molecules.

**Requires Beads backend.** Gastown's data plane IS Beads. The Postgres path does not natively integrate with Gastown. Teams on Postgres who want multi-agent orchestration need to build or adopt a different orchestration layer.

When Gastown is NOT used:

- Execution bundles still work (they're just files/context that any agent reads)
- Claims still work (backend handles coordination)
- Specs are still valid work units — they just get executed manually or via simpler tooling
- `specgraph inject` writes context into a Claude Code session or Cursor workspace
- A single Claude Code instance can read `specgraph bundle <slug>` and execute

```bash
# Without Gastown — manual execution with Claude Code
specgraph bundle bd-k7m3p > /tmp/bundle.yaml
# Copy bundle into CLAUDE.md or feed to Claude Code session

# Or: inject directly into current workspace
specgraph inject bd-k7m3p --tool=claude-code
# Creates .specgraph/context.md with full bundle
# Claude Code picks it up from CLAUDE.md reference

# Or: MCP — Claude Code queries SpecGraph mid-task
# (MCP server exposes specgraph_constitution, specgraph_pattern, etc.)
```

---

## Optional Integration: Issue Tracker Sync

Bidirectional or push-only sync to GitHub Issues, Linear, ADO, Jira.

**What syncs:** slug/title, intent (summary), status, priority, owner, deps (as linked issues), verify items (as checklists), tags.

**What doesn't sync:** full interface contracts, invariants, decisions, constitution refs, execution bundles, codebase context.

**Configuration:**

```yaml
# .specgraph/integrations.yaml
integrations:
  - type: github
    repo: org/product
    direction: bidirectional
    conflict: specgraph-wins

  - type: linear
    team: ENG
    direction: push    # visibility only
```

**Works with either backend.** The sync layer reads from the SpecBackend interface, not from Beads or Postgres directly. It doesn't care which backend is behind it.

```bash
specgraph integration add github --repo=org/product
specgraph sync github
specgraph sync --all
```

---

## Optional Integration: Tool Context Injection

Generates coding-agent context files from the constitution:

```yaml
# .specgraph/config.yaml
toolchain:
  emit:
    - { format: claude-md, path: CLAUDE.md }
    - { format: cursorrules, path: .cursorrules }
    - { format: agents-md, path: AGENTS.md }

  sync_direction: push    # constitution → tool files (or pull, or bidirectional)
```

```bash
specgraph constitution emit          # generate all tool files from constitution
specgraph inject <slug> --tool=claude-code   # spec-specific context injection
specgraph inject <slug> --cleanup    # remove injected context after execution
```

**Works with either backend, with or without Gastown, with or without tracker sync.**

---

## Architecture Diagram (All Layers)

```text
┌─────────────────────────────────────────────────────────────┐
│  SPECGRAPH CORE (always present)                             │
│                                                              │
│  Spec Schema · Authoring Funnel · Constitution               │
│  Codebase Context · Execution Bundles · Agent Collaboration  │
│  CLI · MCP Server                                            │
│                                                              │
│  ──── Backend Interface (abstract) ────                      │
│                                                              │
│  ┌──────────────┐          ┌──────────────────┐              │
│  │ BEADS (+Dolt)│    OR    │ POSTGRES (+AGE)  │              │
│  │              │          │                  │              │
│  │ bd create    │          │ INSERT/UPDATE    │              │
│  │ bd claim     │          │ SELECT...FOR UPD │              │
│  │ bd ready     │          │ LISTEN/NOTIFY    │              │
│  │ bd link      │          │ cypher() or CTEs │              │
│  │ dolt branch  │          │                  │              │
│  └──────────────┘          └──────────────────┘              │
└──────────────┬────────────────────┬──────────────────────────┘
               │                    │
    ┌──────────┼────────────────────┼──────────┐
    │          │                    │          │
    ▼          ▼                    ▼          ▼
 ┌──────┐  ┌───────┐  ┌────────────────┐  ┌──────────┐
 │GASTOWN│  │TRACKER│  │ TOOL INJECTION │  │ MANUAL   │
 │(opt.) │  │ SYNC  │  │   (optional)   │  │EXECUTION │
 │       │  │(opt.) │  │                │  │(always)  │
 │Mayor  │  │GitHub │  │ CLAUDE.md      │  │          │
 │Polecat│  │Linear │  │ .cursorrules   │  │ Human    │
 │Convoy │  │ADO    │  │ AGENTS.md      │  │ reads    │
 │Witness│  │Jira   │  │ spec injection │  │ bundle,  │
 │etc.   │  │       │  │                │  │ executes │
 └──────┘  └───────┘  └────────────────┘  └──────────┘
 Beads     Either       Either              Either
 only      backend      backend             backend
```

---

## Configuration

```yaml
# .specgraph/config.yaml

# REQUIRED: pick one
backend: beads                      # or: postgres

backend_config:
  beads:
    dolt: true                      # false = SQLite (solo default)
    # server_mode: true             # for multi-client
    # remote: "dolthub://org/specs" # for sync
  # postgres:
  #   url: "postgres://user:pass@host:5432/specgraph"
  #   pool_size: 10
  #   age_enabled: auto             # auto-detect, or: true/false

# Constitution (always present)
constitution:
  source: .specgraph/constitution.yaml

# Agent collaboration (always present)
agent:
  posture: partner                  # drive | partner | support
  passes:
    red_team: offer                 # auto | offer | skip
    peripheral_vision: offer
    consistency_check: auto
    simplicity_check: offer

# OPTIONAL: Gastown integration (Beads backend only)
# gastown:
#   enabled: true
#   bundle_delivery: message-bead   # message-bead | formula | claude-md
#   formulas:
#     - specgraph-lifecycle
#     - specgraph-swarm

# OPTIONAL: Issue tracker sync
# integrations:
#   - type: github
#     repo: org/product
#     direction: bidirectional
#     conflict: specgraph-wins

# OPTIONAL: Tool context injection
# toolchain:
#   emit:
#     - { format: claude-md, path: CLAUDE.md }
#     - { format: cursorrules, path: .cursorrules }
#   sync_direction: push
```

---

## What Each Concern Owns

| Concern | Owner | Backend-dependent? |
|---|---|---|
| Spec schema & validation | SpecGraph core | No |
| Authoring funnel | SpecGraph core | No |
| Constitution | SpecGraph core | No |
| Codebase context | SpecGraph core | No |
| Execution bundles | SpecGraph core | No |
| Agent collaboration | SpecGraph core | No |
| Graph analysis (critical path, etc.) | SpecGraph core (queries backend) | Impl differs |
| Storage / persistence | Backend (Beads or Postgres) | Yes |
| Claims / leases | Backend | Yes |
| Edges / deps | Backend | Yes |
| Ready detection | Backend | Yes |
| Events / notifications | Backend | Yes |
| Version history | Backend | Yes |
| Multi-agent orchestration | Gastown (optional) | Beads only |
| PM visibility | Tracker sync (optional) | No |
| Coding agent context | Tool injection (optional) | No |

---

## Open Questions

1. **Beads custom type capacity.** Can a Beads spec-type issue carry the full structured schema (interface contract, invariants, decisions list) in its description? Or do we need sidecar message beads for the detailed fields?

2. **AGE availability.** Which managed Postgres providers support Apache AGE? If most don't, the CTE fallback needs to be the primary path, not the exception.

3. **Bundle format.** YAML? JSON? Markdown with YAML frontmatter? The bundle needs to be both machine-parseable (for MCP/tooling) and human-readable (for manual execution). Leaning YAML.

4. **MCP server scope.** One MCP server that works for both authoring agents and coding agents? Or two separate servers with different tool sets? Single server with role-based tool exposure seems cleanest.

5. **Constitution sync direction defaults.** Should `push` (constitution → tool files) be the default? Or `pull` (existing CLAUDE.md → constitution) for teams migrating from existing setups?
