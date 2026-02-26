# SpecGraph: Live Spec-Driven Development Framework

**Version:** 1.0-draft

---

## 1. The Problem

Existing SDD frameworks (BMAD, OpenSpec, GSD, speckit) treat specs as static documents — markdown files in a repo that agents read and humans update. This creates three gaps:

- **No live query layer.** You can't ask "what specs are blocked?" or "what changed since Tuesday?" without parsing files.
- **No addressability.** Specs reference each other by filename/path, not by stable identity.
- **No execution interface.** Agents like Gastown need a task graph, not a folder of markdown.
- **No ground truth.** Every authoring conversation starts from scratch — the agent doesn't know the project's language, architecture, or principles.
- **No codebase awareness.** The agent writes specs in a vacuum, disconnected from the code that actually exists.

Beads (Yegge's Dolt-backed task/issue management system) and Gastown (multi-agent orchestration) solve the execution and coordination problem but nobody has built the spec schema, authoring process, or project constitution that sits upstream of them — the design layer that produces work clear enough for agents to execute without making decisions.

SpecGraph is four things: a **constitution** (project ground truth), a **spec schema**, an **authoring process** with an AI collaborator, and a **live storage + query layer** that feeds execution engines — whether that's Gastown, a single Claude Code session, or a human developer.

---

## 2. Design Principles

1. **Specs are nodes in a graph, not files in a tree.** Dependencies, blocks, and compositions are first-class edges.
2. **Progressive structure.** Solo dev uses 3 fields. Enterprise adds governance layers. Same schema.
3. **Two storage paths.** Beads (+Dolt) or Postgres (+optional AGE). Pick one, migrate between them. The spec schema is the contract — the backend is swappable.
4. **Agent-native.** Every spec is a machine-readable work unit that an agentic system can claim, execute, and verify.
5. **Human-legible.** A spec reads like a good ticket, not like a BPMN diagram.
6. **Authoring is collaboration, not form-filling.** The AI agent is a thinking partner, not an interviewer.
7. **Constitution-anchored.** Every spec inherits from layered project principles, constraints, and standards — captured once, not re-negotiated per spec.
8. **Code-aware.** The agent understands the existing codebase and grounds specs in what actually exists.
9. **Composable integrations.** Gastown (multi-agent orchestration), issue tracker sync (GitHub/Linear/ADO/Jira), and tool injection (CLAUDE.md/.cursorrules) are all independently optional. Every combination works. No "degraded mode."

---

## 3. The Constitution

### 3.1 Why

Every spec exists within constraints, values, and standards that shouldn't be re-negotiated each time. "We use Go." "All APIs are backward compatible." "We deploy to Kubernetes on GCP." "No ORMs." Without a constitution, the agent asks "what language?" on spec #47. A new team member makes an architecture decision that contradicts an existing ADR. An executing agent generates code using a banned dependency.

The constitution is the ground truth that every spec inherits.

### 3.2 Layers

More specific layers override more general ones, but all layers are visible so the agent and human can see where a constraint originates.

```
┌─────────────────────────────────────────┐
│  User Layer (personal defaults)          │
│  "I use neovim. I prefer terse specs."  │
├─────────────────────────────────────────┤
│  Organization Layer                      │
│  "SOC2 compliance is mandatory"         │
│  "All services must have health checks" │
├─────────────────────────────────────────┤
│  Project Layer                           │
│  "This project uses Go 1.22 + chi"      │
│  "gRPC internal, REST external"         │
├─────────────────────────────────────────┤
│  Domain Layer (optional)                 │
│  "Auth specs require security review"    │
│  "Data pipeline specs need schema defs"  │
└─────────────────────────────────────────┘
```

A solo dev's personal project might only have User + Project. An enterprise spec in the auth domain inherits all four.

When layers conflict, the more specific layer wins and the agent surfaces the conflict: "Your user config prefers Python, but this project forbids it. The project constraint wins."

### 3.3 The Constitution Object

```yaml
# .specgraph/constitution.yaml
constitution:
  layer: project
  name: "auth-service"
  updated: 2025-02-25T10:00:00Z

  # ── Technical Standards ───────────────────────────
  tech:
    languages:
      primary: go
      allowed: [go, python]
      forbidden: [java]
      forbidden_reasons:
        java: "Team has no Java expertise"
    frameworks:
      api: "net/http + chi router"
      testing: "go test + testify"
      orm: none                         # "We use raw SQL with sqlc"
    infrastructure:
      runtime: "Kubernetes on GCP (GKE)"
      database: "PostgreSQL 15 (Cloud SQL)"
      cache: "Redis (Memorystore)"
      ci: "GitHub Actions"
      cd: "ArgoCD"
    api_standards:
      external: REST
      internal: gRPC
      versioning: "URL path (/v1/, /v2/)"
      rate_limiting: required
    data:
      migrations: "goose"
      pii_handling: "Encrypted at rest, never in logs"

  # ── Architecture Principles ───────────────────────
  principles:
    - id: backward-compat
      principle: "All API changes must be backward compatible"
      rationale: "External consumers we can't force-upgrade"
      exceptions: "Major version bumps with 6-month deprecation"
    - id: no-shared-db
      principle: "Services own their data. No shared databases."
      rationale: "2023 outage postmortem"
    - id: observability-first
      principle: "Health checks, metrics, structured logging, tracing on every service"
      rationale: "SOC2 + operational necessity"

  # ── Process Standards ─────────────────────────────
  process:
    spec_review: required
    security_review:
      when: "tags contain 'auth' or 'payments' or 'pii'"
    deployment:
      strategy: "Progressive rollout with feature flags"
      rollback: "Automated on error rate spike"
    documentation:
      api_docs: required
      runbook: "Required for new services"

  # ── Constraints ───────────────────────────────────
  constraints:
    - "No new dependencies without team review"
    - "No direct database access from API handlers"
    - "All secrets via Secret Manager, never env vars"

  # ── Anti-patterns ─────────────────────────────────
  antipatterns:
    - pattern: "Shared mutable state between services"
      why: "Caused 2023-03 cascading failure"
      instead: "Event-driven with Pub/Sub"
    - pattern: "Synchronous chains of >3 service calls"
      why: "Latency and reliability compound"
      instead: "Async where possible, saga pattern"
    - pattern: "Custom auth middleware per service"
      instead: "Use shared auth-middleware package"

  # ── References ────────────────────────────────────
  references:
    - type: adr
      path: "docs/adrs/"
    - type: runbook
      path: "docs/runbooks/"
```

**Solo constitution** can be minimal:

```yaml
constitution:
  tech:
    languages: { primary: typescript }
    infrastructure: { runtime: "Fly.io", database: "SQLite (Turso)" }
  principles:
    - principle: "Ship fast, fix later"
    - principle: "Boring technology"
  constraints:
    - "Single repo, single deploy"
```

### 3.4 How the Constitution Is Used

**During authoring:** Injected into every conversation. The agent checks constraints automatically. "You're proposing Java, but the constitution forbids it because the team has no expertise. Use Go, or update the constitution?"

**During red team:** Checked against antipatterns. "This design has a 4-service synchronous chain. The constitution flags this — the alternative is async with Pub/Sub."

**In the execution bundle:** Included so the implementing agent knows the constraints without being told per-spec.

**Updates:** Constitution is versioned. `specgraph constitution edit` opens in $EDITOR. Changes are tracked so you can see what constraints were in effect when a spec was authored.

---

## 4. Codebase Context Gathering

### 4.1 Context Tiers

The agent needs different levels of understanding at different points:

**Tier 0 — Orientation** (enough to have a conversation): Languages, frameworks, rough architecture, directory structure, how to build and test.

**Tier 1 — Navigation** (enough to author specs): Module/service boundaries, key interfaces and data models, dependency graph, existing patterns and conventions.

**Tier 2 — Deep** (enough to specify and decompose): Specific file-level understanding of affected areas, existing test patterns, deployment details, known tech debt.

Context is built progressively. Tier 0 at bootstrap. Tier 1 during shaping. Tier 2 during specification, focused on the area the spec touches.

### 4.2 Greenfield Bootstrap

```
specgraph init

Agent: Starting a new project. What are we building? (One sentence.)

You:   B2B SaaS for construction project management.

Agent: A few questions for the constitution:
       1. Primary language/stack?
       2. Deployment target?
       3. Solo or team? Size?
       4. Hard constraints? (Compliance, integrations?)

You:   Go + React. GCP with K8s. Team of 4, growing to 10. SOC2.

Agent: Here's the constitution I'd set up:
       [Drafts constitution with Go/React, GCP/K8s, SOC2 principles]

       Recommendations given SOC2:
       - Structured logging from day one
       - Audit trail on all data mutations
       - Encryption at rest for PII

       Adjust anything, or create this?
```

### 4.3 Existing Codebase Bootstrap

```
specgraph init --scan

Agent: I'll examine the codebase. One minute.

       [Scans: directory structure, go.mod/package.json, Dockerfiles,
        CI configs, README, docs/, representative source files per
        module, test files, existing specs/issues/TODOs]

Agent: Here's what I found:

       ARCHITECTURE:
       - Go monorepo, 4 services: auth, billing, projects, gateway
       - Shared pkg/ (middleware, logging, database utilities)
       - PostgreSQL via sqlc, gRPC internal, REST external
       - K8s manifests in deploy/, GitHub Actions CI, ArgoCD

       PATTERNS:
       - Repository pattern for data access
       - Chi router for HTTP
       - Table-driven tests with testify
       - Feature flags via custom pkg/flags

       CONCERNS:
       - Auth service has zero tests
       - Gateway has hardcoded internal service URLs
       - 3 dependencies with known CVEs

       Here's the constitution I'd propose based on what actually exists:
       [Drafts constitution reflecting real patterns, not aspirational]

       And the project context map:
       [Drafts context.yaml with services, interfaces, debt]

       Want me to create specs for the auth testing gap and the
       hardcoded URLs? Adjust anything in the constitution?
```

### 4.4 The Context Object

```yaml
# .specgraph/context.yaml — living document, maintained by agent
context:
  updated: 2025-02-25T14:00:00Z

  architecture:
    type: monorepo
    services:
      - name: auth
        path: services/auth/
        language: go
        api: [grpc, rest]
        database: postgres
        tests: none
        owner: @pat
        notes: "Legacy, fragile. Only Pat understands it."
      - name: billing
        path: services/billing/
        language: go
        api: [grpc]
        database: postgres
        tests: moderate
      # ...
    shared:
      - path: pkg/middleware/
        purpose: "HTTP/gRPC middleware"
      - path: pkg/database/
        purpose: "Connection, transaction helpers"

  interfaces:
    - name: "User model"
      location: services/auth/model/user.go
      summary: "type User struct { ID, Email, Provider, CreatedAt }"
    - name: "Project model"
      location: services/projects/model/project.go
      summary: "type Project struct { ID, Name, OwnerID, Status }"

  service_dependencies:
    gateway: [auth, billing, projects]
    billing: [auth]
    projects: [auth]

  build:
    command: "make build"
    test_command: "make test"
    ci: "GitHub Actions"

  tech_debt:
    - area: "Auth service"
      severity: high
      description: "No tests, single knowledge holder"
    - area: "Gateway"
      severity: medium
      description: "Hardcoded internal service URLs"
```

### 4.5 Progressive Deepening During Authoring

The agent doesn't try to understand the entire codebase at once. It deepens as needed:

**During shaping (Tier 1):** "This spec touches the auth service. Let me look at its structure." Reads directory layout, key types, existing routes, database schema for the relevant area.

**During specification (Tier 2):** "This spec will modify handler/oauth.go and add a migration." Reads the full files, understands current implementation, validates proposed interface against what exists, populates `agent.touches` accurately.

**Context refresh:** `specgraph context refresh` re-scans the codebase. The agent can also trigger targeted refreshes during authoring.

---

## 5. The Spec Object

### 5.1 Core Schema (Solo Mode — 3 Required Fields)

```yaml
spec: login-api
intent: "REST endpoint for user authentication via OAuth2"
verify:
  - "POST /auth/login returns 200 with valid Google OAuth token"
  - "Returns 401 with expired token"
  - "Rate limited to 10 req/min per IP"
```

Three fields. An agent can execute against this. The constitution provides everything else.

### 5.2 Full Schema

```yaml
# ── Identity ──────────────────────────────────────────
spec: login-api                          # immutable slug
version: 3                              # monotonic
stage: specify                          # spark|shape|specify|decompose|approve|execute|done
status: in-progress                     # draft|approved|in-progress|review|done|amended|superseded|abandoned
priority: p1                            # p0-p3
owner: @sean
team: platform

# ── Intent ────────────────────────────────────────────
intent: "REST endpoint for user authentication via OAuth2"
context: |
  Part of auth-service migration from monolith.
  Must maintain backward compat with v1 /login endpoint.

# ── Scope (from shaping) ─────────────────────────────
scope:
  in:
    - "OAuth2 refresh token flow for mobile clients"
    - "Token rotation and revocation"
  out:
    - "SAML support — separate spec, different timeline"
    - "Auth service rewrite — too big"

# ── Edges (the graph) ────────────────────────────────
depends_on: [user-model, oauth-client-config]
blocks: [dashboard-api, notification-service]
composes: [auth-service-v2]
references: [arch-decision-017, incident-2024-03-auth-outage]

# ── Contract ──────────────────────────────────────────
interface: |
  POST /auth/login
  Request:  { provider: "google"|"github", token: string }
  Response: { session_id: string, expires_at: ISO8601 }
  Errors:   401 Unauthorized, 429 Too Many Requests

invariants:
  - "Session tokens expire within 24 hours"
  - "No PII in server logs"

# ── Verification ──────────────────────────────────────
verify:
  - "POST /auth/login returns 200 with valid Google OAuth token"
  - "Returns 401 with expired token"
  - "Rate limited to 10 req/min per IP"
verify_type: integration
test_path: tests/auth/login_test.go

# ── Risks & Success (from shaping) ───────────────────
risks:
  - "Only Pat understands the current auth code"
success:
  must: ["Mobile clients can refresh without re-login"]
  should: ["Token rotation on each refresh"]

# ── Decisions (from authoring) ────────────────────────
decisions:
  - question: "Refresh token storage"
    chosen: "Postgres table"
    rationale: "Need revocation + audit. Already have Postgres."
    rejected:
      - { option: "Redis", why: "Adds ops complexity" }
      - { option: "JWT", why: "Revocation requires blocklist" }
    confidence: high

# ── Review Findings (from authoring) ──────────────────
red_team:
  - severity: critical
    finding: "Bcrypt inappropriate for token hashing"
    resolution: "SHA-256 with per-token salt"
peripheral_vision:
  - { item: "Monitoring", disposition: added_to_spec }
  - { item: "Migration path", disposition: separate_spec }

# ── Execution Hints ───────────────────────────────────
agent:
  posture: partner
  estimated_complexity: medium
  language: go
  touches:
    - services/auth/handler/oauth.go
    - services/auth/model/session.go
    - database/queries/auth.sql
  constraints:
    - "Do not modify the v1 /login endpoint"
    - "Use the existing rate limiter middleware"

# ── Decomposition ────────────────────────────────────
decomposition:
  strategy: vertical-slice
  slices:
    - id: refresh-happy-path
      intent: "Login returns refresh token, refresh endpoint issues new tokens"
      verify: ["POST /auth/login returns refresh_token", "POST /auth/refresh returns new tokens"]
      touches: [handler.go, token.go, migration.sql]
    - id: refresh-error-cases
      intent: "Expired, revoked, and invalid refresh tokens"
      depends_on: [refresh-happy-path]
      verify: ["Expired → 401", "Revoked → 401"]
    - id: refresh-security
      intent: "Rate limiting, token limits, audit logging"
      depends_on: [refresh-happy-path]
      verify: ["30 req/min per user", "Max 5 active tokens"]

# ── Governance (enterprise) ──────────────────────────
governance:
  approvals:
    - { role: tech-lead, required: true, approved_by: @pat }

# ── Metadata ──────────────────────────────────────────
created: 2025-02-20T10:00:00Z
updated: 2025-02-25T14:30:00Z
tags: [auth, api, migration]
```

### 5.3 Field Progression by Scale

| Field Group | Solo | Small Team | Enterprise |
|---|---|---|---|
| `spec`, `intent`, `verify` | ✅ required | ✅ | ✅ |
| `status`, `priority` | optional | ✅ | ✅ |
| `depends_on`, `blocks` | optional | ✅ | ✅ |
| `owner`, `team` | — | ✅ | ✅ |
| `scope`, `risks`, `success` | optional | recommended | ✅ |
| `interface`, `invariants` | optional | recommended | ✅ |
| `decisions`, `red_team` | optional | recommended | ✅ |
| `agent.*` | ✅ | ✅ | ✅ |
| `decomposition` | optional | recommended | ✅ |
| `composes`, `references` | — | optional | ✅ |
| `governance` | — | — | ✅ |

### 5.4 Spec Evolution

Specs are not write-once artifacts. They evolve — during implementation, after completion, and across the lifetime of the system they describe. The design must account for this from the start rather than treating it as an edge case.

**Expanded status model:**
```
draft → approved → in-progress → review → done
  ↑                                        │
  └────────────────────────────────────────┘  (reopen)
                                            │
                                        amended → done
                                            │
                                        superseded (terminal)
                                            │
                                        abandoned (terminal)
```

- `done` — Implementation complete, verify criteria passing. **Not terminal.** A done spec may be reopened or amended.
- `amended` — A done spec that is being revised. New version in progress. The prior version remains readable in history. Re-enters the authoring funnel at shape or specify, not spark.
- `superseded` — Replaced entirely by a different spec. Terminal. Carries a `superseded_by` link to the replacement. The old spec's decisions and history remain accessible.
- `abandoned` — Work stopped. Terminal. May carry a reason.

**Version semantics:**
Every mutation to a spec bumps its `version` counter. But not all bumps are equal:

- **Minor edits** (typo in intent, adding a tag): version bumps, no downstream impact.
- **Interface changes** (modified contract, new verify criteria, revised invariants): version bumps and triggers **impact notification** — every spec that `depends_on` this spec gets an event (`spec.interface_changed`) so they can re-validate their assumptions.
- **Amendment** (reopening a done spec): version bumps, status → `amended`, re-enters authoring funnel. The amendment carries a `reason` field documenting why the change is needed.

```yaml
# ── Evolution ────────────────────────────────────────
history:
  - version: 1
    stage: approve
    summary: "Initial design"
    date: 2025-02-20T10:00:00Z
  - version: 2
    stage: specify
    summary: "Revised interface after red-team finding on bcrypt"
    date: 2025-02-22T14:00:00Z
  - version: 3
    stage: amended
    summary: "Reopened: mobile clients need offline token refresh"
    reason: "Customer escalation — offline mode doesn't work"
    date: 2025-03-15T09:00:00Z

superseded_by: null                  # or slug of replacement spec
supersedes: null                     # or slug of spec this replaced
```

**New edge types for evolution:**
```yaml
# Existing edges (from §5.2)
depends_on: [user-model]             # must be done before this
blocks: [dashboard-api]              # this must be done before those
composes: [auth-service-v2]          # parent epic
references: [arch-decision-017]      # informational link

# Evolution edges
supersedes: legacy-login-api         # this spec replaces an older one
superseded_by: null                  # set when this spec is replaced
amends: null                         # links to the version being amended
```

**What happens to dependents when a spec changes:**

When a done spec is amended (interface change), SpecGraph:
1. Emits `spec.interface_changed` event with diff of what changed
2. For each spec that `depends_on` the amended spec: checks whether the dependent's assumptions about the upstream interface still hold
3. If a dependent spec references the old interface directly (in its own interface contract or verify criteria), emits `spec.dependency_drift` warning
4. Does not auto-block dependents — that would be too aggressive. Surfaces the drift for human or agent review.

When a spec is superseded:
1. Old spec status → `superseded`, `superseded_by` → new spec slug
2. New spec gets `supersedes` → old spec slug
3. All specs that `depends_on` the old spec get notified — they may need to update their dependency to point to the new spec
4. ADR exports for the old spec's decisions get status → `Superseded by [new spec]`

**Living specs:**
Not all specs describe one-time work. Some describe ongoing contracts: API boundaries, data models, architectural principles. These specs are never `done` in the task-completion sense — they're living documents that evolve with the system.

```yaml
spec: user-api-contract
lifecycle: living                    # living | task (default)
intent: "Public contract for the User API — consumers depend on this"
interface: |
  GET /users/{id}
  POST /users
  ...
invariants:
  - "All changes must be backward compatible"
  - "Deprecation requires 6-month notice"
```

Living specs:
- Never reach `done` status — they stay in `approved` as the current version of truth
- Version history tracks their evolution over time
- Can be amended (same flow as task specs)
- Their `interface` field IS the contract — downstream specs depend on it
- `specgraph export design user-api-contract` always generates the current version
- Changes to a living spec's interface trigger the same `spec.interface_changed` event flow as amendments to task specs

---

## 6. The Authoring Funnel

Every spec passes through a refinement funnel. Stages are not gates — you can enter at any stage, skip stages, and go backward. A solo dev can go spark → execute in one conversation. Enterprise adds review loops.

```
  spark ──▶ shape ──▶ specify ──▶ decompose ──▶ approve ──▶ execute
    │         │          │            │            │           │
    │         │          │            │            │     implementation
    │         │          │            │         sign-off  reveals issue
    │     scope/risks interface   work units      │           │
    │     decisions   contracts   task graph       │           │
    │     tradeoffs   verify                      │           │
    │                                             │           │
    └─────────────────────────────────────────────┴───────────┘
                    backward flow is normal
```

**Backward flow:** During specification you may discover the scope was wrong — back to shape. During execution an agent may discover the interface doesn't work — back to specify. This is normal, not failure. Version history captures the previous state. The agent should identify when backward flow is needed: "This interface won't work because of X. We need to reshape the scope."

### 6.1 Spark

**Input:** A vague idea, a customer problem, a bug report.
**Output:** A spark object — enough to decide whether to pursue it.

```yaml
spark: auth-rethink
seed: "Mobile auth tokens expire mid-session, users lose work."
signal:
  - "3 customer escalations this week"
  - "Acme deal blocked on SAML"
questions:
  - "Fix existing auth or rewrite?"
  - "SAML just for Acme, or general SSO?"
```

**Elicitation probes** (agent uses when user needs help):

- **Seed:** "What's the problem or idea in a sentence or two?"
- **Signal:** "Why now? What changed?"
- **Scope sniff:** "Rough size? Bug fix, feature, service, redesign?"
- **Unknowns:** "What don't you know yet?"
- **Kill test:** "What would make you abandon this?"

### 6.2 Shape

**Input:** A spark or direct entry with a clear problem.
**Output:** Shaped spec — scope bounded, approach chosen, risks and decisions captured.

Shaping is a set of analytical **moves**:

- **Bound the scope** — In, explicitly out, why. Estimate size.
- **Explore solution space** — 2-3 approaches with tradeoffs. Decide. Capture rejected alternatives.
- **Identify edges** — Dependencies, blockers, parent epics, prior decisions.
- **Surface risks** — Technical, operational, business. Specific, not abstract.
- **Define success** — Must-have, should-have, won't-have.

### 6.3 Specify

**Input:** A shaped spec or direct entry with clear requirements.
**Output:** Fully specified spec — interface contract, verification criteria, invariants.

At this stage, the agent deepens to **Tier 2 codebase context** — reading the specific files that will be affected, understanding existing patterns, validating the proposed interface against what exists.

- **Define interface contract** — Every boundary the spec touches: API endpoints, data model changes, internal interfaces. Grounded in actual existing code.
- **Write verification criteria** — Observable behaviors, not implementation details. Each maps to a test. Functional, non-functional, security. Specific: status codes, timeouts, limits.
- **State invariants** — Things that must always be true.

### 6.4 Decompose

**Input:** A specified spec.
**Output:** Agent-executable work units.

Three strategies:

- **Vertical slice** — By user-visible behavior. Best for specs with a clear API.
- **Layer cake** — By technical layer (data → logic → API → integration). Best for infrastructure.
- **Single unit** — No decomposition. Best for trivial/small specs.

**Child specs vs inline:** Inline decomposition is for one agent in one session. Child specs are for work big enough to be independently tracked and verified.

### 6.5 Approve

**Solo:** Self-approve. **Team:** Peer review of the spec (not code). **Enterprise:** Role-based sign-offs in `governance`.

### 6.6 Re-entry: Amending and Superseding

The funnel is not a one-way pipeline. Specs come back.

**Amendment** — A completed spec needs revision. Customer feedback, new requirements, implementation revealed a flaw, the system around it changed. The spec re-enters the funnel at **shape** or **specify** (never spark — the problem is known, the context exists).

```bash
specgraph amend login-api --reason "Mobile clients need offline token refresh"
# Status: done → amended
# Stage: → shape (or specify if the scope hasn't changed)
# Version: bumps
# History: records amendment reason + prior version
# Dependents: notified via spec.interface_changed if interface fields change
```

During amendment, the agent has full access to the spec's history — prior decisions, red-team findings, implementation notes. It doesn't start from scratch. The shaping conversation is anchored: "This spec was completed in v2. The interface was X. We're amending because Y. What needs to change?"

**Supersession** — A new spec replaces an old one entirely. Different approach, different scope, different everything. The old spec isn't wrong per se — it's been overtaken.

```bash
specgraph supersede legacy-login-api --with login-api-v2
# Old spec: status → superseded, superseded_by → login-api-v2
# New spec: supersedes → legacy-login-api
# Old spec's dependents: notified, may need to re-point dependencies
# Old spec's ADR exports: status → "Superseded by login-api-v2"
```

The old spec remains in the graph as history. Its decisions and rationale are still valuable context — the new spec's authoring conversation can reference them: "The previous approach used Redis for token storage. That was rejected in v1 because of ops complexity. Has anything changed?"

**When to amend vs supersede:**
- Same problem, evolved solution → amend (same slug, version bumps)
- Different problem or fundamentally different approach → supersede (new slug, old linked)
- Rule of thumb: if the `intent` is substantially the same, amend. If the `intent` has changed, supersede.

### 6.7 Drift Detection

Over time, code evolves independently of the specs that describe it. Interfaces shift, invariants get violated, verify criteria no longer pass. The spec and the system drift apart.

**What drifts:**
- **Interface drift:** The actual API or data model no longer matches what the spec's `interface` field describes. Endpoints added, fields renamed, response shapes changed — without updating the spec.
- **Invariant drift:** A stated invariant ("no PII in logs", "sessions expire in 24 hours") is no longer true in the code.
- **Verify drift:** Tests that once passed now fail, or verify criteria that were tested are no longer covered.
- **Dependency drift:** A spec depends on another spec's interface, but that upstream interface was amended. The downstream spec's assumptions may no longer hold.

**Detection methods:**

`specgraph drift` runs checks across the spec graph:

```bash
specgraph drift                          # check all done/living specs
specgraph drift login-api                # check one spec
specgraph drift --scope=interfaces       # only interface drift
```

Interface drift (for specs with structured interface contracts):
- Parse the spec's `interface` field for API endpoints, types, status codes
- Compare against actual code (route definitions, handler signatures, OpenAPI specs if available)
- Report mismatches: "Spec says POST /auth/login returns {session_id, expires_at} but handler returns {token, expires_in, refresh_token}"

Verify drift:
- Re-run verify criteria tests (using `test_path` or inferred test commands)
- Report: "2 of 5 verify criteria now failing"

Dependency drift:
- For each `depends_on` edge, check if the upstream spec's interface has changed since this spec was last updated
- Report: "login-api depends on user-model v2, but user-model is now v4 — interface changed in v3"

**Drift response:**
Drift detection surfaces problems. It doesn't auto-fix. Options:
- **Update the spec** to match reality: `specgraph amend login-api --reason "Sync with actual implementation"`
- **Fix the code** to match the spec: the spec was right, the implementation drifted
- **Acknowledge the drift** with a note: `specgraph drift acknowledge login-api --note "Interface changed intentionally, spec update pending"`

**Continuous drift monitoring:**
```yaml
drift:
  schedule: weekly                    # or on-demand only
  scope: [interfaces, verify]        # what to check
  notify: true                       # emit spec.drift_detected events
  auto_export_on_detect: true        # regenerate design docs when drift found
```

For living specs (§5.4), drift detection is especially important — they describe ongoing contracts, and drift means the contract is being violated.

---

## 7. Agent Collaboration

### 7.1 Postures

**Drive** — Agent leads. Proposes, drafts, recommends. Human edits and approves. Opening: "Here's my read on this and what I'd build."
Use when: user asks "just design this," is in unfamiliar territory, or wants speed.

**Partner** (default) — Agent asks first, then contributes. Checks what the human has in mind. Builds on their ideas. Shifts toward Drive when the human is stuck. Opening: "Do you have an approach in mind, or want me to sketch one out?"
Use when: user has domain expertise and wants a collaborator.

**Support** — Agent listens, reflects, clarifies. Opinions only when asked or when something is dangerous. Rubber duck with teeth. Opening: "Go ahead. I'm listening."
Use when: user is thinking out loud, is a domain expert, or explicitly asks.

The agent reads the room and adjusts: short vague messages → Drive; long detailed messages → Support; back-and-forth → Partner. User can override at any time.

### 7.2 Collaboration Modes

Lenses the agent shifts between fluidly during conversation:

**Generative** — Proposes solutions, drafts interfaces, suggests approaches. "The refresh endpoint should probably be separate from /login..."
In Drive: default. In Partner: after checking user's thinking. In Support: only when asked.

**Critical** — Attacks the plan, finds weaknesses, argues against the approach. "You said don't touch /login, but /login has to change — it needs to return a refresh token now."
In Drive: automatic. In Partner: offered ("Want me to argue against this?"). In Support: only critical issues.

**Expansive** — Widens the frame. Monitoring, migration, operational readiness, downstream impact. "You need refresh success rate metrics or you're flying blind."
In Drive: automatic. In Partner: offered. In Support: holds unless asked.

**Analytical** — Structures complex decisions with tradeoff maps. "Three storage options. Here's the matrix."
All postures: activates when there's a decision with multiple viable options.

**Stress Test** — Probes verification criteria for gaps. "Rate limited per user — but unauthenticated brute force isn't 'a user.' You need IP-based limiting too."
In Drive: automatic. In Partner: offered. In Support: only critical.

### 7.3 Named Analytical Passes

**Red Team** — Systematic attack. Failure modes, race conditions, scale behavior, adversarial scenarios, organizational risks. Findings are Critical/Warning/Note, captured in `red_team`.

**Peripheral Vision** — Everything the spec doesn't mention. Operational (deployment, monitoring, rollback), data (migration, retention, privacy), integration (downstream consumers, docs), human (support awareness, training). Items dispositioned as added-to-spec, separate-spec, or note-for-implementer. Captured in `peripheral_vision`.

**Consistency Check** — Spec vs itself (do verify criteria test the interface?) and vs adjacent specs (does the interface match what dependents expect?).

**Simplicity Check** — Argue for doing less. Over-specified? Over-decomposed? Over-scoped? YAGNI? Simpler approach?

**Constitution Check** — Spec against the constitution. Language allowed? Antipatterns triggered? Principles violated? Process requirements met?

In Drive posture: all passes run automatically. In Partner: offered before running. In Support: held unless asked. Exception: always-on safety net.

### 7.4 The Always-On Safety Net

Regardless of posture, the agent always flags — cannot be disabled:

- **Security:** Auth/authz issues, credential exposure, injection vectors.
- **Data loss:** Destructive operations without rollback, missing migrations.
- **Consistency:** Contradiction with a dependency or blocking spec.
- **Constitution violation:** Hard constraint breached (forbidden language, mandatory process skipped).
- **Showstopper:** Something that makes the spec unimplementable.

### 7.5 Agent Principles

1. **Read the room first.** Don't assume the user needs you to lead.
2. **Match the user's energy.** Vague → offer to lead. Detailed → listen and add. "What do you think?" → give your take.
3. **Ask before analyzing** (Partner/Support). Exception: safety net.
4. **Opinions welcome, lectures not.** "My instinct is X" beats an unsolicited wall of analysis.
5. **Earn trust incrementally.** Start lighter than you think you should.
6. **Give a way out.** "Want me to go deeper, or move on?"
7. **Every criticism comes with a fix.** Be a partner, not a gatekeeper.
8. **Preserve decisions regardless of posture.** Decision + rationale + rejected alternatives. Non-negotiable.
9. **Flag confidence levels.** "I'm confident about X. Less sure about Y — worth checking."
10. **Ground in the codebase.** Reference real files, real patterns, real constraints — not abstract best practices.

---

## 8. CLI Agent Integration

The constitution and spec context are useless if they don't reach the agent actually writing code. That agent isn't SpecGraph — it's Claude Code, Cursor, Codex, or another CLI/IDE agent. Each has its own extension model:

| Tool | Context | Hooks | Skills/Commands | Plugins |
|---|---|---|---|---|
| Claude Code | `CLAUDE.md` (hierarchical) | PreToolUse, PostToolUse, SessionStart, Stop, etc. | `.claude/skills/` (SKILL.md + supporting files) | `/plugin install` |
| Cursor | `.cursor/rules/`, `AGENTS.md` | afterFileEdit, stop (hooks.json) | Rules with `agent-requested` type | — |
| Codex | `AGENTS.md` / instructions flag | — | — | — |
| OpenCode | `AGENTS.md` + config | — | — | — |
| Aider | `.aider.conf.yml` | — | — | — |

SpecGraph meets each tool at every extension point it offers: static context files, runtime hooks, interactive skills/commands, and packaged plugins. Claude Code has the richest surface area, so we describe it in full. Other tools get equivalent support at their level of capability.

### 8.1 Constitution Sync

**Two roles for the constitution need to be distinguished.** The SpecGraph authoring agent (which helps you write specs) needs the full constitution, full codebase context, and full spec graph. The coding agent (Claude Code, Cursor, etc.) needs a filtered subset: tech stack, conventions, constraints, and antipatterns relevant to the specific spec being implemented. The execution bundle is the filter between these two scopes.

**Three sync modes:**

**Pull (recommended for existing projects):** The team's existing CLAUDE.md/.cursorrules is the source of truth. SpecGraph reads it, merges with supplemental SpecGraph-specific config (process, governance), and uses the result as the constitution. Changes to CLAUDE.md automatically update the constitution.

```yaml
constitution:
  source: CLAUDE.md
  supplemental: .specgraph/constitution-extra.yaml
  sync_direction: pull
```

**Push (recommended for new projects):** SpecGraph owns the constitution and generates tool-specific files. `specgraph constitution emit` regenerates CLAUDE.md, .cursorrules, AGENTS.md, and `.cursor/rules/` from the canonical constitution.

```yaml
constitution:
  source: .specgraph/constitution.yaml
  emit:
    - { format: claude-md, path: CLAUDE.md }
    - { format: cursorrules, path: .cursorrules }
    - { format: cursor-rules-dir, path: .cursor/rules/ }
    - { format: agents-md, path: AGENTS.md }
  sync_direction: push
```

**Bidirectional (for teams in transition):** SpecGraph reads existing tool files, merges with its own constitution, and can emit back. Conflicts are flagged.

### 8.2 Hooks

SpecGraph's job is authoring and managing specs, not supervising the coding agent's implementation work. Post-edit linting, test tracking, scope enforcement — those are the project's CI pipeline and the coding agent's own responsibility. SpecGraph shouldn't try to be a second linter or test runner bolted onto every keystroke.

The one hook that genuinely helps is **SessionStart priming**: making the agent aware that SpecGraph exists and what's on the board, without assuming the session is about spec work.

**Claude Code SessionStart hook:**
```json
{
  "hooks": {
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "specgraph hook session-start"
      }]
    }]
  }
}
```
`specgraph hook session-start` outputs `additionalContext` with a brief status summary: "SpecGraph is configured for this project. You have 1 claimed spec (oauth-refresh-flow, in-progress). 3 specs are ready for claiming. Use /specgraph-next to see them, or /specgraph-verify to check progress on your active spec." This is orientation, not a directive — the user might be here to debug something unrelated. If there's no active spec, the context is even lighter: just a note that SpecGraph is available and what skills exist.

**Cursor equivalent:** A `.cursor/rules/specgraph-awareness.md` rule with `alwaysApply: true` that includes the same orientation text, generated by `specgraph init --tool=cursor`.

That's it for hooks. Everything else SpecGraph does for the agent is better served by **skills** (interactive, invoked when needed) and **MCP** (context on demand). The agent doesn't need SpecGraph watching its every edit — it needs SpecGraph's knowledge available when it asks for it.

### 8.3 Skills: The Authoring Funnel as Slash Commands

Claude Code skills (`.claude/skills/`) are the interactive surface. Each SpecGraph authoring stage and key operation becomes a skill that Claude Code can invoke directly or auto-detect as relevant.

**Skill structure:**
```
.claude/skills/specgraph/
├── SKILL.md              # Meta-skill: SpecGraph overview, routes to sub-skills
├── spark/
│   └── SKILL.md          # /specgraph-spark — capture a vague idea
├── shape/
│   └── SKILL.md          # /specgraph-shape — bound scope, decisions, risks
├── specify/
│   └── SKILL.md          # /specgraph-specify — interface contracts, verify
├── decompose/
│   └── SKILL.md          # /specgraph-decompose — break into work units
├── approve/
│   └── SKILL.md          # /specgraph-approve — review and sign off
├── next/
│   └── SKILL.md          # /specgraph-next — what's ready to implement?
├── verify/
│   ├── SKILL.md          # /specgraph-verify — check work against spec
│   └── scripts/
│       └── verify.sh     # Runs verify criteria, reports results
├── red-team/
│   └── SKILL.md          # /specgraph-red-team — attack the design
├── export/
│   └── SKILL.md          # /specgraph-export — generate ADRs, docs, diagrams
└── constitution/
    └── SKILL.md          # /specgraph-constitution — view/edit project ground truth
```

**Example skill — `/specgraph-shape`:**
```markdown
---
name: specgraph-shape
description: >
  Shape a spec by bounding scope, exploring solution approaches, capturing
  decisions, and surfacing risks. Use when the user has a problem or idea
  that needs to be turned into a bounded, actionable spec. Also triggered
  when user says "let's design...", "scope this out", or "what's the approach for..."
---
## SpecGraph Shape

You are running the SpecGraph shaping stage. Load the project constitution
from `.specgraph/constitution.yaml` and codebase context from `.specgraph/context.yaml`.

If $ARGUMENTS contains a spec slug, load that spec and continue shaping it.
If $ARGUMENTS is a new problem description, create a new spec in shape stage.

### Shaping Moves (do all of these):

1. **Bound the scope** — What's in, what's explicitly out, and why. Estimate size.
2. **Explore solution space** — 2-3 approaches with tradeoffs. Decide. Capture rejected.
3. **Identify edges** — Dependencies, blockers, parent epics, prior decisions.
4. **Surface risks** — Technical, operational, business. Specific, not abstract.
5. **Define success** — Must-have, should-have, won't-have.

### After shaping:

Run `specgraph update <slug>` to persist the shaped spec.
Offer to continue to /specgraph-specify or /specgraph-red-team.

### Constitution constraints to check:
Read and apply all principles, constraints, and antipatterns from the constitution.
Flag any violations immediately.
```

**Example skill — `/specgraph-verify`:**
```markdown
---
name: specgraph-verify
description: >
  Verify the current implementation against the active spec's verification
  criteria. Use after implementing a spec or when user asks "does this pass?"
  or "check the spec" or "am I done?"
---
## SpecGraph Verify

Load the active spec (from `.specgraph/active-spec` or $ARGUMENTS).

For each item in `verify`:
1. Determine how to test it (use `test_path` if specified, otherwise infer)
2. Run the test or check
3. Report PASS / FAIL / UNTESTED with evidence

Summary format:
- ✅ Criterion text (PASS — evidence)
- ❌ Criterion text (FAIL — what went wrong)
- ⬜ Criterion text (UNTESTED — how to test)

If all pass: suggest `specgraph complete <slug>`.
If failures: suggest specific fixes based on the error.
```

Skills are auto-detected by Claude Code based on their `description` frontmatter — when a user says "let's design the auth flow," Claude Code can automatically load the `specgraph-shape` skill without the user typing `/specgraph-shape`. Skills can also reference each other, creating natural flow through the authoring funnel.

**For Cursor:** Skills map to `.cursor/rules/` with `agent-requested` type. SpecGraph generates equivalent rules:
```markdown
---
description: "Shape a spec — use when designing, scoping, or planning a feature"
globs: [".specgraph/**"]
alwaysApply: false
---
[same instructions as the Claude Code skill]
```

### 8.4 The SpecGraph Plugin

For Claude Code, SpecGraph packages everything — hooks, skills, MCP server — as a single installable plugin:

```bash
# Install SpecGraph plugin
claude /plugin install specgraph

# Or from a marketplace
claude /plugin marketplace add https://github.com/specgraph/specgraph
claude /plugin install specgraph
```

**What the plugin installs:**
- Hook: SessionStart awareness priming (the only hook — see §8.2)
- Skills: Full authoring funnel (`/specgraph-spark` through `/specgraph-approve`), operational skills (`/specgraph-next`, `/specgraph-verify`, `/specgraph-drift`), export skill (`/specgraph-export`), constitution management
- MCP server: `specgraph mcp serve` auto-configured for dynamic context
- CLAUDE.md additions: SpecGraph awareness section injected into project CLAUDE.md

For teams that don't use Claude Code, or prefer manual setup, all components work independently:
```bash
# Manual setup (no plugin required)
specgraph init --tool=claude-code    # generates skills + SessionStart hook + MCP config
specgraph init --tool=cursor         # generates .cursor/rules/ + awareness rule
specgraph init --tool=agents-md      # generates AGENTS.md only (any tool)
```

### 8.5 Spec-Specific Context Injection

When a coding agent executes a spec, it needs the general constitution plus the spec-specific execution context. SpecGraph injects this into the tool's native format.

```bash
specgraph inject oauth-refresh-flow --tool=claude-code
```

This generates task-scoped context (as a nested CLAUDE.md near the affected code, or equivalent for other tools) containing: the interface contract, verification criteria, constraints, decisions already made, relevant file paths, existing patterns to follow, and the filtered constitution subset.

For tools with MCP support, the coding agent can pull context dynamically instead:

```
specgraph_get_constitution    — "What are the project standards?"
specgraph_get_spec           — "What am I supposed to build?"
specgraph_get_pattern        — "How does this codebase do X?"
specgraph_check_constraint   — "Does this approach violate anything?"
specgraph_report_blocker     — "Found an issue, need to escalate"
```

Dynamic MCP context is cleaner — context on demand, no file generation, natural context window management. But static injection works everywhere, including tools without MCP support.

### 8.6 Execution Workflow

**Manual (developer with Claude Code, plugin installed):**
```bash
claude                                            # start session
# SessionStart hook primes SpecGraph awareness
claude> /specgraph-next                           # what's ready?
claude> /specgraph-verify oauth-refresh-flow      # check progress
claude> implement the refresh token endpoint      # work normally
claude> /specgraph-verify oauth-refresh-flow      # am I done?
specgraph complete oauth-refresh-flow             # report done
```

**Agent pool (generic, either backend):**
```bash
slug=$(specgraph next --format=slug)
specgraph claim $slug --agent=$AGENT_ID
specgraph inject $slug --tool=claude-code
claude-code --task="Implement spec $slug per the injected context"
specgraph complete $slug
specgraph cleanup $slug --tool=claude-code
```

For Gastown integration (Beads backend, native pipeline) and full execution model details, see section 12.

### 8.7 Handling Existing Files

```bash
# Adopt SpecGraph on a project with existing CLAUDE.md
specgraph init --from-claude-md
# Agent parses CLAUDE.md → creates constitution
# CLAUDE.md remains primary, constitution supplements it

# Adopt with existing Cursor rules
specgraph init --from-cursor-rules
# Reads .cursor/rules/ → merges into constitution

# Check for conflicts
specgraph constitution diff
# Shows mismatches between constitution and tool files

# One-time import
specgraph constitution import CLAUDE.md
specgraph constitution import .cursor/rules/
```

---

## 9. Storage & Concurrency

### 9.1 Two Paths, Pick One

SpecGraph supports two storage backends. Teams pick one. Both are fully capable — no "degraded mode."

| | **Path A: Beads (+Dolt)** | **Path B: Postgres (+optional AGE)** |
|---|---|---|
| **Best for** | Teams wanting Gastown integration, Dolt branching/versioning, Git-native sync | Teams with existing Postgres infra, DBA support, enterprise requirements |
| **Graph queries** | Dolt SQL (MySQL-compatible recursive CTEs) | Cypher via AGE extension, or Postgres recursive CTEs |
| **Concurrency** | Beads' atomic claims (`bd claim`), Dolt branch-per-agent | `SELECT...FOR UPDATE`, row-level locking, leases |
| **Events** | Dolt commit hooks | `LISTEN/NOTIFY` (real push, no polling) |
| **Sync** | Dolt remotes (including Git remotes) | DB replication, or API-level federation |
| **Gastown** | Native — specs ARE beads | Needs adapter or manual execution |
| **Version history** | Dolt commits (built-in) | Version column + events table |

Both backends implement the same abstract interface. SpecGraph core never talks to Beads or Postgres directly:

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
    
    # Query + Context
    def list(self, filters: Filters) -> list[Spec]: ...
    def search(self, query: str) -> list[Spec]: ...
    def execution_bundle(self, slug: str) -> Bundle: ...
```

### 9.2 Path A: Beads (+Dolt)

Beads is a Dolt-backed task/issue management system. SpecGraph specs are Beads issues with a custom `spec` type. Dolt provides versioning, branching, cell-level merge, and sync via remotes.

**What Beads already provides (SpecGraph doesn't reimplement):**
- Atomic claims (`bd claim`) — sets assignee + status atomically
- Dependency tracking (`bd link --blocks`) — typed edges
- Ready detection (`bd ready`) — lists issues with no open blockers
- Hash-based IDs (bd-xxxx) — zero merge conflicts in multi-agent/multi-branch
- Branch-per-agent via Dolt branching — write isolation for concurrent agents
- Sync via Dolt remotes (including Git remotes as of Dolt v1.81.10)
- Compaction — semantic "memory decay" summarizes old completed work
- Embedded or server mode — single-process or multi-client

**What SpecGraph adds on top of Beads:**
- Spec schema (structured interface, verify, invariants, decisions)
- Authoring funnel (spark → approve)
- Constitution + codebase context
- Execution bundles
- Graph analysis (critical path, transitive deps, impact — via Dolt SQL)
- Analytical passes (red team, peripheral vision, etc.)
- Tool integration (CLAUDE.md/.cursorrules generation)

**Progression within the Beads path:**
```
Solo, starting:     bd init (SQLite default) → zero infra
Solo with agents:   bd init --backend dolt   → branching, atomic claims
Team:               Dolt server mode         → multi-client, remote sync
Federation:         Dolt push/pull           → cross-team dep resolution
```

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

### 9.3 Path B: Postgres (+optional AGE)

Standard Postgres. Specs in tables with JSONB columns for structured fields. Optional Apache AGE extension for native graph queries.

**Postgres schema:**
```sql
CREATE TABLE specs (
    slug TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    stage TEXT NOT NULL DEFAULT 'spark',
    intent TEXT,
    interface JSONB,
    verify JSONB,
    invariants JSONB,
    decisions JSONB,
    complexity TEXT,
    priority TEXT,
    owner TEXT,
    tags TEXT[],
    constitution_ref TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE spec_edges (
    from_slug TEXT REFERENCES specs(slug),
    to_slug TEXT REFERENCES specs(slug),
    edge_type TEXT NOT NULL,
    PRIMARY KEY (from_slug, to_slug, edge_type)
);

CREATE TABLE claims (
    spec_slug TEXT PRIMARY KEY REFERENCES specs(slug),
    agent_id TEXT NOT NULL,
    claimed_at TIMESTAMPTZ DEFAULT now(),
    lease_expires TIMESTAMPTZ NOT NULL,
    last_heartbeat TIMESTAMPTZ DEFAULT now()
);
```

**Apache AGE (optional, same Postgres instance):** If the AGE extension is available, graph queries use native openCypher syntax. If not, falls back to recursive CTEs. Detection is automatic on init.

```cypher
-- With AGE: transitive deps
MATCH (s:Spec {slug: 'login-api'})-[:DEPENDS_ON*]->(dep:Spec)
RETURN dep.slug, dep.status

-- With AGE: impact analysis
MATCH (s:Spec {slug: 'user-model'})<-[:DEPENDS_ON*]-(d:Spec)
WHERE d.status IN ['approved', 'in-progress']
RETURN d.slug, d.owner
```

AGE is additive — same Postgres instance, transactions span both relational and graph, all existing Postgres tooling applies. If AGE isn't available (managed providers, restricted environments), CTEs provide identical results with less readable syntax.

### 9.4 The Claim Protocol

The most critical concurrency operation — an agent claims a spec for execution — must be atomic. Both backends implement this:

```
1. Agent: specgraph claim <slug> --agent=agent-1
2. Backend atomically:
   a. Verify: status="approved" AND deps_met=true AND owner IS NULL
   b. Set: status="in-progress", owner="agent-1", claimed_at=now()
   c. If any check fails: return error (already claimed or deps not met)
3. Compare-and-swap. No two agents can claim the same spec.
```

**On the Beads path:** `bd claim` handles this natively.

**On the Postgres path:** `SELECT...FOR UPDATE` + conditional update in a transaction.

**Leases:** Claimed specs get a lease (default 30 min). Heartbeats renew. Expired leases (agent crash, network partition) auto-unclaim, making the spec claimable again.

**Optimistic concurrency:** Every spec has a monotonic version. Updates include the expected version. Mismatch → re-read, merge, retry.

### 9.5 Multi-Agent Coordination

```yaml
agents:
  pool:
    - id: agent-1
      capabilities: [go, postgres, grpc]
      max_concurrent: 3
    - id: agent-2
      capabilities: [go, react, testing]
      max_concurrent: 1
  assignment:
    strategy: capability-match       # capability-match|round-robin|priority
  lease:
    timeout: 30m
    heartbeat_interval: 5m
    auto_unclaim_on_expire: true
```

```bash
specgraph next --agent=agent-1          # match capabilities
specgraph claim <slug> --agent=agent-1  # atomic claim with lease
specgraph heartbeat <slug>              # renew lease
specgraph unclaim <slug>                # release back to approved
specgraph reassign <slug> --to=agent-2  # transfer ownership
```

### 9.6 Federation

Cross-team dependencies with independent spec stores:

```yaml
spec: dashboard-integration
depends_on:
  - user-model                          # local
  - team-b://user-api-v3               # remote (federated)
```

**Beads path:** Dolt push/pull. `specgraph remote add team-b <url>`. Read-only copies for querying.

**Postgres path:** Lightweight API federation. Remote specs cached with TTL.

```bash
specgraph remote add team-b https://specgraph.internal/team-b
specgraph show team-b://user-api-v3
specgraph bundle dashboard-integration   # resolves remote deps
```

### 9.7 Migration Between Backends

```bash
specgraph migrate --from=beads --to=postgres --url="postgres://..."
specgraph migrate --from=postgres --to=beads
```

One-way door at a time. No dual-write. Migration reads all specs and edges from source, writes to target, verifies counts and integrity, flips config. Source remains as archive.

---

## 10. Command Side-Effects (Keeping the Graph Consistent)

When a spec changes, other specs may be affected. SpecGraph handles this inline as part of the command that made the change — not as a separate event bus or reactive system.

### 10.1 What Happens When

**`specgraph complete <slug>`:**
- Mark spec as done
- Check all specs that `depends_on` this one: if all their deps are now met, they become claimable (visible in `specgraph next`)
- If auto_export configured: update changelog

**`specgraph status <slug> abandoned`:**
- Mark spec as abandoned
- Check all specs that `depends_on` this one: mark them as blocked (with reason: "dependency abandoned")

**`specgraph amend <slug>`:**
- Mark spec as amended, re-enter authoring funnel
- If interface fields changed: check all specs that `depends_on` this one, flag potential dependency drift in their metadata
- If auto_export configured: regenerate ADRs and design docs

**`specgraph supersede <slug> --with <new>`:**
- Old spec → superseded, `superseded_by` → new slug
- New spec gets `supersedes` → old slug
- Dependents of old spec get a warning: "dependency superseded, may need to re-point to new-slug"
- Old spec's exported ADRs get status → "Superseded by new-slug"

**`specgraph update <slug>` (when interface fields change):**
- Version bumps
- Check dependents for assumption drift (their interface expectations vs the updated contract)
- Flag mismatches in dependent spec metadata

All of this happens synchronously as part of the command. No background process, no event bus, no daemon. The command writes to the store and does its bookkeeping in the same transaction.

### 10.2 External Notification (Optional, for Scaling)

For the solo dev and small team cases, command side-effects are sufficient. There's nothing to notify — you're the one running the commands, and you can see the output.

At scale, two cases benefit from real notification:

**Gastown (Beads path only):** The Mayor needs to know when specs become claimable so it can dispatch polecats. On the Beads path, Dolt commit hooks naturally surface this — Gastown already has its own reactive model built on Beads' event system. SpecGraph doesn't need to build a parallel one.

**Multi-agent coordination (Postgres path):** If multiple agents are polling `specgraph next`, Postgres `LISTEN/NOTIFY` can push "new spec claimable" events instead of requiring polling. This is a Postgres feature, not a SpecGraph event system — SpecGraph just fires a `NOTIFY` at the end of relevant commands.

```sql
-- At the end of specgraph complete, if any spec was unblocked:
NOTIFY specgraph_unblocked, 'oauth-refresh-flow';
```

Both of these are narrow, backend-specific concerns. They don't require a general-purpose event model. They're implemented as optional post-command hooks in the storage backend, not as a first-class abstraction.

---

## 11. The Agent Execution Bundle

`specgraph bundle <slug>` produces a self-contained bundle for Gastown or any execution engine. This is the handoff protocol.

```yaml
task:
  spec: oauth-refresh-flow
  intent: "Add OAuth2 refresh token endpoint for mobile clients"
  interface: |
    POST /auth/refresh
    Request:  { refresh_token: string }
    Response: { access_token, refresh_token, expires_in }
    Errors:   401, 429
  verification_criteria:
    - "POST /auth/refresh returns new token pair"
    - "Returns 401 with expired refresh token"
    - "Rate limited to 30 req/min per user"
  constraints:
    - "Do not modify the v1 /login endpoint"
  decomposition:
    - "Add refresh_tokens migration"
    - "Implement token model and storage"
    - "Add refresh handler"
    - "Modify login to issue refresh token"
    - "Write integration tests"
  files_to_examine:
    - services/auth/handler/oauth.go
    - services/auth/model/session.go

constitution:
  language: go
  framework: "net/http + chi router"
  testing: "go test + testify, table-driven"
  database: "PostgreSQL via sqlc"
  principles:
    - "All API changes must be backward compatible"
    - "No direct database access from handlers"
  antipatterns:
    - pattern: "Custom auth middleware per service"
      instead: "Use shared auth-middleware package"

code_context:
  existing_patterns:
    handler: "func (h *Handler) Name(w, r) { validate → service → respond }"
    test: "Table-driven with testify"
    migration: "goose: -- +goose Up / -- +goose Down"
  relevant_interfaces:
    - file: services/auth/model/session.go
      summary: "Session struct, CreateSession, GetSession"
    - file: middleware/ratelimit.go
      summary: "RateLimit(n, window) middleware"

resolved_dependencies:
  - spec: user-model
    interface: "type User struct { ID, Email, Provider, CreatedAt }"

decisions:
  - question: "Token storage"
    chosen: "Postgres table"
    rationale: "Need revocation + audit"

invariants:
  - "Session tokens expire within 24 hours"
  - "No PII in server logs"
```

---

## 12. Execution & Integration

### 12.1 Execution Bundles Work Everywhere

The execution bundle (`specgraph bundle <slug>`) is the universal handoff — it works regardless of who or what executes the spec:

**Manual (developer with Claude Code):**
```bash
specgraph next                                    # what's ready?
specgraph inject oauth-refresh-flow --tool=claude-code  # inject context
claude "implement the refresh token endpoint"     # work normally
specgraph complete oauth-refresh-flow             # report done
```

**Single agent (Claude Code, Cursor, Codex):**
```bash
slug=$(specgraph next --format=slug)
specgraph claim $slug --agent=$AGENT_ID
specgraph inject $slug --tool=claude-code
claude-code --task="Implement spec $slug per the injected context"
specgraph complete $slug
specgraph cleanup $slug --tool=claude-code
```

**Gastown (multi-agent orchestration, Beads path only):**
On the Beads path, specs ARE beads. No adapter needed. The pipeline is native:

```
1. Spec authored via SpecGraph → stored as Beads issue (bd create --type spec)
2. Spec approved, deps met → bd ready surfaces it
3. Mayor creates convoy → gt convoy create "Sprint" bd-k7m3p
4. gt sling bd-k7m3p myproject → polecat spawns
5. Polecat reads hook → sees spec bead → reads execution bundle
6. Polecat implements per bundle → gt done → MR to Refinery
7. Refinery merges → spec marked complete → downstream specs unblocked
```

Gastown's key insight applies directly: "Polecats thrive on well-defined, fully-spec'd beads — they shouldn't have to make decisions." SpecGraph's authoring funnel exists to produce specs clear enough for a polecat to execute without judgment.

See ADR-002 for full Gastown integration details including formula definitions, Mayor context, crew vs polecat roles, and failure handling.

### 12.2 Optional Integrations (Independent of Each Other)

**Gastown** (requires Beads backend):
Multi-agent orchestration. Specs are beads, Gastown dispatches polecats, Refinery merges, Witness monitors. Native integration on the Beads path. Not available on the Postgres path — teams on Postgres manage agents via the claim protocol and manual/scripted execution.

**Issue tracker sync** (either backend):
Bidirectional or push-only sync to GitHub Issues, Linear, ADO, Jira. Syncs status, priority, owner, deps, verify items. Source of truth stays in SpecGraph. Issue trackers provide PM visibility, notifications, and discussion.

```yaml
integrations:
  - type: github
    repo: org/product
    direction: bidirectional
    conflict: specgraph-wins
  - type: linear
    team: ENG
    direction: push              # visibility only
```

**Tool injection** (either backend):
Generates coding-agent context files from the constitution. `specgraph constitution emit` produces CLAUDE.md, .cursorrules, AGENTS.md. `specgraph inject <slug>` produces spec-specific context for the active task.

**MCP server** (either backend):
Exposes SpecGraph operations for both authoring agents and coding agents mid-task. See section 14 for tool list.

**Document export** (either backend):
Generates human-readable design artifacts from specs and the spec graph. These are **output-only sync targets** — generated from SpecGraph, not synced back. The rationalized design work captured during authoring (decisions, interfaces, constraints, red-team findings) is too valuable to stay locked in the spec store. Exported artifacts serve as durable reference material for humans, future agents, onboarding, audits, and any context where SpecGraph isn't installed.

Export formats:

**ADRs (Architecture Decision Records):**
Each spec's `decisions` array maps directly to Nygard-format ADRs. One ADR per significant decision. Includes status, context (from spec's `intent` + `context`), decision, rationale, rejected alternatives (with reasons), and consequences (from red-team findings). ADRs are numbered and linked to their source spec.

```bash
specgraph export adr login-api --output=docs/adrs/
# Produces:
#   docs/adrs/0017-refresh-token-storage.md
#   docs/adrs/0018-token-hashing-algorithm.md
```

```markdown
# ADR-0017: Refresh Token Storage

**Status:** Accepted
**Date:** 2025-02-25
**Spec:** login-api (v3)
**Deciders:** @sean, @pat

## Context
Part of auth-service migration from monolith. Must maintain backward
compat with v1 /login endpoint. Need revocation + audit trail.

## Decision
Store refresh tokens in a Postgres table with per-token salt.

## Rationale
Already have Postgres. Need revocation (can't do with stateless JWT).
Audit trail comes free with table + triggers.

## Rejected Alternatives
- **Redis:** Adds ops complexity for a persistence problem
- **JWT with blocklist:** Revocation requires blocklist anyway, might as well use DB

## Consequences
- Migration required (new refresh_tokens table)
- Need token cleanup job for expired tokens
- Red team finding: bcrypt inappropriate for token hashing → using SHA-256

## References
- Spec: login-api
- Red team: RT-001 (bcrypt finding)
- Constitution principle: backward-compat
```

**Design documents (Markdown):**
Full prose design doc generated from the spec, suitable for wiki, Notion, Confluence, or repo docs. Includes overview (intent + context), scope (in/out), architecture (interface contracts), data model, verification plan, risks, decisions, and open questions. Progressive — solo specs produce a 1-page summary; fully-shaped enterprise specs produce a detailed design doc.

```bash
specgraph export design login-api --output=docs/designs/
# Produces: docs/designs/login-api-design.md
```

**Mermaid diagrams:**
Generated from the spec graph, individual spec structure, or execution flow. Multiple diagram types:

```bash
# Dependency graph for a spec and its transitive deps
specgraph export mermaid login-api --type=deps
# Produces flowchart: login-api → user-model → ... with status coloring

# Sequence diagram from interface contract
specgraph export mermaid login-api --type=sequence
# Produces sequence diagram: Client → Gateway → Auth Service → DB

# Full project graph (all specs, colored by status)
specgraph export mermaid --type=project-graph
# Produces: approved=green, in-progress=yellow, blocked=red, draft=gray

# Decomposition tree
specgraph export mermaid auth-service-v2 --type=decomposition
# Produces tree: parent → children with status badges

# Critical path
specgraph export mermaid release-2.0 --type=critical-path
# Produces: highlighted path through dependency graph
```

Example output (dependency graph):
```mermaid
graph LR
    login-api:::approved --> user-model:::done
    login-api:::approved --> oauth-client-config:::done
    dashboard-api:::blocked --> login-api:::approved
    notification-service:::draft --> login-api:::approved

    classDef done fill:#22c55e,color:white
    classDef approved fill:#3b82f6,color:white
    classDef blocked fill:#ef4444,color:white
    classDef draft fill:#9ca3af,color:white
```

**PlantUML diagrams:**
Same diagram types as Mermaid, in PlantUML syntax for teams using PlantUML toolchains.

```bash
specgraph export puml login-api --type=sequence --output=docs/diagrams/
```

**Changelogs and release notes:**
Aggregate completed specs within a time range or tagged release into a changelog.

```bash
specgraph export changelog --since=2025-02-01 --format=keep-a-changelog
specgraph export release-notes --tag=v2.1 --audience=engineering
specgraph export release-notes --tag=v2.1 --audience=customer
# Engineering version includes technical detail
# Customer version includes user-facing impact only (from intent + success criteria)
```

**RFC / proposal format:**
For specs still in shape/specify stage, generate an RFC-style document for broader review outside SpecGraph. Includes problem statement, proposed solution, alternatives considered, risks, and open questions.

```bash
specgraph export rfc auth-service-v2 --output=docs/rfcs/
```

**Export configuration:**

```yaml
export:
  output_dir: docs/specgraph/
  formats:
    adr:
      enabled: true
      path: docs/adrs/
      numbering: auto                # auto-increment from existing ADRs
      template: nygard               # nygard | madr | custom
    design:
      enabled: true
      path: docs/designs/
      include_mermaid: true          # embed diagrams in design docs
    mermaid:
      enabled: true
      path: docs/diagrams/
      theme: default
    changelog:
      enabled: true
      path: CHANGELOG.md
      format: keep-a-changelog
  auto_export:
    on_approve: [adr, design, mermaid]      # generate on spec approval
    on_complete: [changelog]                 # update changelog on completion
    on_release: [release-notes]             # generate on tag
```

**Continuous vs one-shot:** Export can be one-shot (`specgraph export ...`) or continuous via event hooks. When `auto_export.on_approve` is set, approving a spec automatically regenerates its ADRs and design doc. This keeps the docs directory always in sync without manual effort.

**Batch export for existing specs:**
```bash
specgraph export all --formats=adr,design,mermaid --output=docs/specgraph/
# Exports everything. Good for initial setup or periodic refresh.
```

### 12.3 System Flow

```
  ┌─────────────────────────────────────────────────────────┐
  │  GROUND TRUTH                                            │
  │  Constitution ◄──► CLAUDE.md / .cursorrules / AGENTS.md  │
  │  Codebase Context (architecture, patterns, interfaces)   │
  └────────────────────────┬────────────────────────────────┘
                           │ informs
  ┌────────────────────────▼────────────────────────────────┐
  │  AUTHORING LAYER                                         │
  │  Postures: Drive │ Partner │ Support                     │
  │  Modes: Generative · Critical · Expansive · Analytical   │
  │  Passes: Red Team · Peripheral Vision · Simplicity ·     │
  │          Consistency · Constitution Check                 │
  │                                                         │
  │  spark ──▶ shape ──▶ specify ──▶ decompose ──▶ approve  │
  │              ◀──────────◀──────────◀  (backward flow)    │
  └────────────────────────┬────────────────────────────────┘
                           │ produces
  ┌────────────────────────▼────────────────────────────────┐
  │  STORAGE LAYER                                           │
  │  Beads (+Dolt) ────── OR ────── Postgres (+AGE)          │
  │                                                         │
  │  Events: created · unblocked · claimed · completed       │
  │  Query: list · show · context · deps · next · blocked    │
  └──┬───────────┬────────────┬────────────┬───────────────┘
     │           │            │            │
     ▼           ▼            ▼            ▼
┌────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│GASTOWN │ │ TRACKER  │ │  TOOL    │ │ DOCUMENT │ │ MANUAL   │
│(opt.)  │ │  SYNC   │ │INJECTION │ │  EXPORT  │ │EXECUTION │
│        │ │ (opt.)  │ │  (opt.)  │ │  (opt.)  │ │(always)  │
│Mayor   │ │GitHub   │ │CLAUDE.md │ │ADRs      │ │Human or  │
│Polecat │ │Linear   │ │.cursor/  │ │Design doc│ │single    │
│Convoy  │ │ADO/Jira │ │AGENTS.md │ │Mermaid   │ │agent     │
│        │ │         │ │Hooks     │ │PlantUML  │ │          │
│        │ │         │ │Skills    │ │Changelog │ │          │
└────────┘ └─────────┘ └──────────┘ └──────────┘ └──────────┘
 Beads      Either      Either       Either       Either
 only       backend     backend      backend      backend
```

---

## 13. Scaling Model

### Solo Dev
- Backend: Beads (SQLite) or Postgres — either works
- 3-field specs, minimal constitution, self-approve, skip stages
- `specgraph next | agent-execute`
- 2-5 minutes per spec

### Solo Dev + Agent Swarm
- Backend: Beads (+Dolt) recommended — native Gastown integration
- Or Postgres — manage agents via claim protocol + scripts
- Add priority, capability matching, lease/heartbeat
- 5-15 minutes per spec, parallel execution

### Small Team (3-8)
- Backend: Beads (+Dolt server mode) or Postgres (+AGE)
- Add owner, status, priority, edges
- Issue tracker sync for PM visibility
- CI: `specgraph lint`, `specgraph orphans`
- Multi-agent pools with capability matching
- 10-30 minutes per spec

### Enterprise (50+)
- Backend: Either path at scale
  - Beads: Dolt federation (push/pull between team instances)
  - Postgres: DB replication or API federation
- Full schema, governance, multi-layer constitution
- Role-based approvals, cross-team dependency negotiation
- Metrics: cycle time, blocked-time, completion velocity
- Hours to days per significant spec

---

## 14. CLI & MCP Reference

### CLI

```bash
# Bootstrap
specgraph init [--backend=beads|postgres] [--scan] [--from-claude-md] [--from-cursor-rules]
specgraph init --tool=claude-code            # generate skills + SessionStart hook + MCP config
specgraph init --tool=cursor                 # generate .cursor/rules/ + awareness rule
specgraph init --tool=agents-md              # generate AGENTS.md only
specgraph constitution edit
specgraph constitution show
specgraph constitution emit [--all] [--format=claude-md|cursorrules|cursor-rules-dir|agents-md]
specgraph constitution diff                  # diff vs tool files
specgraph constitution import CLAUDE.md
specgraph constitution import .cursor/rules/
specgraph context refresh [service-name]

# Authoring
specgraph spark "description"
specgraph shape <slug>
specgraph specify <slug>
specgraph decompose <slug>
specgraph approve <slug>

# Lifecycle
specgraph new <slug> --intent "..." --verify "..."
specgraph edit <slug>
specgraph status <slug> <new-status>
specgraph assign <slug> <owner>
specgraph amend <slug> --reason "..."            # reopen completed spec
specgraph supersede <slug> --with <new-slug>     # replace spec entirely
specgraph drift [<slug>] [--scope=interfaces|verify|deps]  # detect drift
specgraph drift acknowledge <slug> --note "..."  # acknowledge known drift

# Query
specgraph list [--status=X] [--owner=X] [--tag=X]
specgraph show <slug>
specgraph bundle <slug>                      # generate execution bundle
specgraph next [--owner=X]
specgraph blocked

# Graph
specgraph deps <slug>
specgraph blocks <slug>
specgraph tree <slug>
specgraph critical-path <slug>
specgraph cycle-check

# History
specgraph log <slug>
specgraph diff <slug> [--v1=N --v2=M]
specgraph changed [--since=DATE]

# Quality
specgraph lint
specgraph orphans
specgraph coverage
specgraph stats

# Toolchain integration
specgraph inject <slug> --tool=claude-code   # inject spec context
specgraph cleanup <slug> --tool=claude-code  # remove task context
specgraph execute <slug> --tool=claude-code  # inject + invoke

# Hook (called by CLI agent hook config, not directly by users)
specgraph hook session-start                 # SessionStart: prime SpecGraph awareness

# Multi-agent
specgraph claim <slug> --agent=agent-1       # atomic claim with lease
specgraph heartbeat <slug>                   # renew lease
specgraph unclaim <slug>                     # release back to approved
specgraph reassign <slug> --to=agent-2       # transfer ownership

# Storage
specgraph migrate --from=beads --to=postgres --url=...
specgraph remote add <name> <url>            # federation
specgraph remote list

# Document Export (optional)
specgraph export adr <slug> [--output=docs/adrs/]
specgraph export design <slug> [--output=docs/designs/]
specgraph export mermaid <slug> --type=deps|sequence|decomposition|critical-path
specgraph export mermaid --type=project-graph
specgraph export puml <slug> --type=deps|sequence|decomposition
specgraph export changelog [--since=DATE] [--tag=TAG] [--format=keep-a-changelog]
specgraph export release-notes --tag=TAG [--audience=engineering|customer]
specgraph export rfc <slug> [--output=docs/rfcs/]
specgraph export all --formats=adr,design,mermaid [--output=docs/specgraph/]

# Live
specgraph watch
specgraph mcp serve                          # MCP server for coding agents

# Issue Tracker Integration (optional)
specgraph integration add github --repo=org/product
specgraph integration add linear --team=ENG
specgraph integration sync github              # push+pull per config
specgraph integration sync --all
specgraph integration status
```

### MCP Server

```
# Spec management
specgraph_list       — Filter specs by status, owner, team, tags
specgraph_show       — Full spec details
specgraph_bundle     — Agent execution bundle
specgraph_create     — Create spec
specgraph_update     — Update spec fields
specgraph_deps       — Dependency graph
specgraph_next       — Next claimable spec (with capability matching)
specgraph_claim      — Atomic claim with lease
specgraph_heartbeat  — Renew claim lease
specgraph_complete   — Report spec completion
specgraph_spark      — Start interactive authoring
specgraph_red_team   — Run red team pass

# Constitution & context (for coding agents mid-task)
specgraph_constitution — Read constitution (full or filtered)
specgraph_codebase     — Query codebase context
specgraph_get_pattern  — "How does this codebase do X?"
specgraph_check_constraint — "Does this approach violate anything?"
specgraph_report_blocker   — "Found an issue, need to escalate"

# Export (for authoring agents or automation)
specgraph_export_adr      — Generate ADR from spec decisions
specgraph_export_diagram  — Generate Mermaid/PlantUML from spec graph
specgraph_export_design   — Generate design doc from spec
```

---

## 15. Configuration

```yaml
# .specgraph/config.yaml

# Storage backend — pick one
backend: beads                           # beads|postgres
backend_config:
  beads:
    dolt: true                           # false = SQLite (solo default)
    # server_mode: true                  # for multi-client
    # remote: "dolthub://org/specs"      # for sync
  # postgres:
  #   url: postgres://user:pass@host:5432/specgraph
  #   pool_size: 10
  #   age_enabled: auto                  # auto-detect, true, or false

# Constitution sync
constitution:
  source: .specgraph/constitution.yaml   # or CLAUDE.md for pull mode
  sync_direction: push                   # push|pull|bidirectional
  emit:
    - { format: claude-md, path: CLAUDE.md }
    - { format: cursorrules, path: .cursorrules }
    - { format: agents-md, path: AGENTS.md }
  on_conflict: warn

# Coding tool integration
toolchain:
  default: claude-code                   # claude-code|cursor|codex|opencode|aider
  inject_on_claim: true
  cleanup_on_complete: true
  mcp_enabled: true
  session_priming: true                  # SessionStart hook: prime awareness
  skills:
    install: true                        # install .claude/skills/specgraph/ on init
    auto_detect: true                    # allow Claude to auto-invoke skills

# Agent collaboration (authoring)
agent:
  posture: partner                       # drive|partner|support
  posture_rules:
    - when: "complexity == trivial"
      posture: support
    - when: "tags contain 'security'"
      posture: partner
  passes:
    red_team: offer                      # auto|offer|off
    peripheral_vision: offer
    consistency_check: auto
    simplicity_check: offer
    constitution_check: auto

# Multi-agent coordination (execution)
agents:
  pool:
    - { id: agent-1, capabilities: [go, postgres, grpc], max_concurrent: 3 }
    - { id: agent-2, capabilities: [go, react, testing], max_concurrent: 1 }
  assignment:
    strategy: capability-match
  lease:
    timeout: 30m
    heartbeat_interval: 5m
    auto_unclaim_on_expire: true

# Federation (enterprise)
federation:
  remotes:
    - { name: team-b, url: "https://specgraph.internal/team-b" }
  sync_interval: 5m

# Codebase context
context:
  auto_refresh: true
  tier2_on_specify: true

# OPTIONAL: Gastown integration (Beads backend only)
# gastown:
#   enabled: true
#   bundle_delivery: message-bead       # message-bead | formula | claude-md
#   formulas: [specgraph-lifecycle, specgraph-swarm]

# OPTIONAL: Issue tracker sync (either backend)
# integrations:
#   - type: github
#     repo: org/product
#     direction: bidirectional
#     conflict: specgraph-wins
#   - type: linear
#     team: ENG
#     direction: push

# OPTIONAL: Document export (either backend)
# export:
#   output_dir: docs/specgraph/
#   formats:
#     adr:
#       enabled: true
#       path: docs/adrs/
#       numbering: auto
#       template: nygard              # nygard | madr | custom
#     design:
#       enabled: true
#       path: docs/designs/
#       include_mermaid: true
#     mermaid:
#       enabled: true
#       path: docs/diagrams/
#     changelog:
#       enabled: true
#       path: CHANGELOG.md
#       format: keep-a-changelog
#   auto_export:
#     on_approve: [adr, design, mermaid]
#     on_complete: [changelog]
#     on_release: [release-notes]
```

---

## 16. Implementation Roadmap

See **specgraph-v1.0-draft-roadmap.md** for the full implementation plan with Gantt chart, dependency graph, and per-item detail.

**Summary:** Four phases. Phase 1 (Foundation) establishes the spec schema, both storage backends, claim protocol, execution bundle format, and core CLI. Phase 2 (Authoring & CLI Agent Integration) builds the authoring funnel and brings it into Claude Code as skills and into Cursor as rules. Phase 3 (Coordination, Export & Integration) adds multi-agent support, drift detection, document export, Gastown, and issue tracker sync. Phase 4 (Scale) adds federation, multi-repo, metrics, and governance.

**Highest-leverage starting points:** spec schema, constitution, execution bundle format, core CLI, and Claude Code skills.
