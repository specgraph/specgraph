# SpecGraph Vertical Slice Roadmap

> **For Claude:** This document describes the slice roadmap. Each slice will have its own implementation plan created via the `writing-plans` skill before work begins.

**Goal:** Complete the SpecGraph implementation through a series of vertical slices, each delivering a usable end-to-end capability. The primary path is: Constitution → Authoring (with full AI collaboration) → Execution Bundles → Lifecycle → Sync → Claude Code Plugin.

**Approach:** Feature-First slices. Each slice delivers something you can use. Dependencies flow naturally: constitution feeds authoring, authoring feeds bundles. You can stop after any slice and have a working system.

**Backend:** Memgraph only. Postgres+AGE deferred to a future effort.

**Design Docs:** `docs/plans/2026-02-28-client-server-architecture-design.md` (v2 architecture), `docs/initial-design-session/specgraph-v1.0-draft-spec.md` (full spec)

---

## Slice 1: Vertical Slice (Complete)

**Status:** Done (commits `50b504c`, `9fd18e5`)

**Delivered:**

- Protobuf schema — 5 services (Spec, Decision, Claim, Graph, Server)
- ConnectRPC server with all handlers
- Memgraph storage backend with integration tests (testcontainers)
- CLI client — `create`, `update`, `list`, `show`, `decision`, `claim`, `edge`, `deps`, `ready`, `critical-path`, `impact`, `health`
- Config (docker/external/remote modes), Docker Compose management
- `specgraph init` (local modes)
- Dockerfile, E2E smoke test
- Code quality (lefthook, golangci-lint)

---

## Slice 2: Constitution

**Goal:** Create, store, query, and validate constitutions. `specgraph init --scan` generates one from an existing codebase.

**What you can do after this slice:**

- `specgraph init` creates a constitution (interactive or from codebase scan)
- `specgraph constitution show` displays the active constitution
- `specgraph constitution update` modifies constitution fields
- `specgraph constitution check <slug>` validates a spec against constraints
- `specgraph constitution emit` generates CLAUDE.md/.cursorrules/AGENTS.md
- Constitution stored as a versioned graph node in Memgraph

### Components

**Proto — `constitution.proto`**

```text
ConstitutionService:
  GetConstitution(GetConstitutionRequest) → Constitution
  UpdateConstitution(UpdateConstitutionRequest) → Constitution
  GetPattern(GetPatternRequest) → Pattern
  CheckViolation(CheckViolationRequest) → CheckViolationResponse
  EmitToolFiles(EmitRequest) → EmitResponse
```

Constitution message mirrors the YAML schema from the spec (Section 3.3):

- `layer` (user/org/project/domain)
- `name`, `updated`
- `tech` (languages, frameworks, infrastructure, api_standards, data)
- `principles` (repeated Principle — id, principle, rationale, exceptions)
- `process` (spec_review, security_review, deployment, documentation)
- `constraints` (repeated string)
- `antipatterns` (repeated Antipattern — pattern, why, instead)
- `references` (repeated Reference — type, path)

**Storage — `ConstitutionBackend` interface**

```go
type ConstitutionBackend interface {
    GetConstitution(ctx context.Context) (*Constitution, error)
    UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
    GetPattern(ctx context.Context, area string) ([]Pattern, error)
    CheckViolation(ctx context.Context, specSlug string) ([]Violation, error)
}
```

Memgraph model:

- `(:Constitution {layer, name, version, ...})` node with JSON-encoded tech/principles/constraints fields
- `(:Spec)-[:GOVERNED_BY]->(:Constitution)` edge (implicit — all specs governed by active constitution)
- Constitution is versioned: updates bump version, old versions queryable via history

**Handler — ConstitutionService**

Standard ConnectRPC handler. `CheckViolation` queries the spec and constitution, applies rules:

- Language in forbidden list → violation
- Antipattern match → warning
- Missing required process (e.g., security review for auth-tagged spec) → violation

**CLI**

- `constitution show` — formatted display of active constitution
- `constitution update --tech.languages.primary=go --add-constraint="..."` — field-level updates
- `constitution check <slug>` — runs violation check, outputs findings
- `constitution emit --format=claude-md --output=CLAUDE.md` — generates tool file
- Enhanced `init` — constitution creation step added to the interactive flow

**Constitution Bootstrap**

`specgraph init` creates a constitution interactively or via agent survey — no custom AST scanning. The authoring agent reads project signals (manifests, CI configs, directory structure, existing convention files) and proposes a draft constitution the user can review and approve.

**Emitters**

Constitution → tool-specific file formatters:

- `claude-md` — generates CLAUDE.md sections for tech stack, constraints, conventions
- `cursorrules` — generates .cursorrules with equivalent content
- `agents-md` — generates AGENTS.md

Sync direction configurable: push (SpecGraph owns truth), pull (existing files are truth), bidirectional.

### Dependencies

Slice 1 (done). No new external Go deps expected.

### Estimated Tasks: 8-10

---

## Slice 3: Authoring Funnel

**Goal:** Author specs through Spark→Shape→Specify→Decompose→Approve with full AI collaboration — postures, analytical passes, and the always-on safety net.

**What you can do after this slice:**

- `specgraph spark <slug>` — capture idea with elicitation probes
- `specgraph shape <slug>` — bound scope, explore solutions, surface risks
- `specgraph specify <slug>` — define interface, verify criteria, invariants (grounded in code)
- `specgraph decompose <slug>` — break into work units
- `specgraph approve <slug>` — mark ready for execution
- Agent postures (Drive/Partner/Support) adapt to user's communication style
- Analytical passes (Red Team, Peripheral Vision, Consistency, Simplicity, Constitution Check) produce structured findings stored on the spec
- Always-on safety net flags security, data loss, consistency, and constitution violations

### Components

**Proto — `authoring.proto`**

```text
AuthoringService:
  Spark(SparkRequest) → SparkResponse
  Shape(ShapeRequest) → ShapeResponse
  Specify(SpecifyRequest) → SpecifyResponse
  Decompose(DecomposeRequest) → DecomposeResponse
  Approve(ApproveRequest) → ApproveResponse
  Amend(AmendRequest) → AmendResponse
  Supersede(SupersedeRequest) → SupersedeResponse
```

Stage-specific output messages:

- `SparkOutput` — seed, signal, questions, scope_sniff, kill_test
- `ShapeOutput` — scope (in/out), approaches (with tradeoffs), chosen_approach, risks, success (must/should/wont), decisions
- `SpecifyOutput` — interface_contract, verify_criteria, invariants, touches (files affected)
- `DecomposeOutput` — strategy (vertical_slice/layer_cake/single_unit), slices (repeated — id, intent, verify, touches, depends_on)

Analytical pass messages:

- `RedTeamFinding` — severity (critical/warning/note), finding, resolution
- `PeripheralVisionItem` — item, disposition (added_to_spec/separate_spec/note_for_implementer)
- `ConsistencyIssue` — type, description, affected_specs
- `SimplicityFinding` — area, suggestion
- `ConstitutionViolation` — constraint, violation, severity

**Storage — `AuthoringBackend` interface**

```go
type AuthoringBackend interface {
    // Stage transitions with validation
    TransitionStage(ctx context.Context, slug string, from, to string) error

    // Store analytical pass results
    StoreRedTeam(ctx context.Context, slug string, findings []*RedTeamFinding) error
    StorePeripheralVision(ctx context.Context, slug string, items []*PeripheralVisionItem) error

    // Decomposition — creates child specs with edges
    Decompose(ctx context.Context, parentSlug string, slices []*DecompositionSlice) ([]*Spec, error)

    // Store decisions made during authoring
    StoreDecision(ctx context.Context, slug string, decision *AuthoringDecision) error
}
```

Memgraph model:

- Stage stored on `(:Spec)` node, transitions validated (spark→shape→specify→decompose→approve, backward flow allowed)
- Analytical pass results stored as JSON properties on the spec node (or as connected sub-nodes for complex data)
- Decomposition creates child `(:Spec)` nodes with `[:COMPOSES]` edges back to parent
- Decisions stored as `(:Decision)` nodes linked to spec via `[:DECIDED_IN]` edge

**Handler — AuthoringService**

Each RPC:

1. Validates preconditions (correct stage, spec exists, required fields present)
2. Transitions stage
3. Stores stage-specific output (analytical pass results, decisions, scope, etc.)
4. Returns structured response for the CLI to use in driving the conversation

**CLI — Interactive Authoring Commands**

Each command is an interactive session:

- `spark <slug>` — prompts for seed, runs elicitation probes, creates spec at spark stage
- `shape <slug>` — reads current spec, runs shaping moves, stores scope/risks/decisions
- `specify <slug>` — deepens to Tier 2 context, prompts for interface/verify/invariants
- `decompose <slug>` — proposes decomposition strategy, creates child specs
- `approve <slug>` — self-approve (solo) or governance check (team)

**Prompt Templates**

Server-side templates per stage, used by the CLI to structure the AI conversation:

- **Spark probes:** Seed, Signal, Scope Sniff, Unknowns, Kill Test
- **Shape moves:** Bound Scope, Explore Solution Space, Identify Edges, Surface Risks, Define Success
- **Specify prompts:** Define Interface Contract, Write Verification Criteria, State Invariants
- **Decompose strategies:** Vertical Slice, Layer Cake, Single Unit — with heuristics for which to recommend

**Postures**

Stored as configuration (per-spec or session-level):

- **Drive** — agent proposes, drafts, recommends. All analytical passes run automatically.
- **Partner** (default) — agent asks first, then contributes. Passes offered before running.
- **Support** — agent listens, reflects, clarifies. Passes held unless asked.

Posture detection heuristic: short vague messages → Drive; long detailed messages → Support; back-and-forth → Partner. User override at any time.

**Analytical Passes**

Each pass is a structured analysis that produces typed output:

| Pass | Trigger | Output |
|------|---------|--------|
| Red Team | After specify (auto in Drive, offered in Partner) | `RedTeamFinding[]` |
| Peripheral Vision | After shape (auto in Drive, offered in Partner) | `PeripheralVisionItem[]` |
| Consistency Check | After specify — spec vs itself, vs adjacent specs | `ConsistencyIssue[]` |
| Simplicity Check | After decompose — YAGNI, over-specification | `SimplicityFinding[]` |
| Constitution Check | All stages — spec vs constitution constraints | `ConstitutionViolation[]` |

**Safety Net**

Always-on, cannot be disabled. Fires at every stage regardless of posture:

- Security: auth/authz, credential exposure, injection
- Data loss: destructive without rollback, missing migrations
- Consistency: contradiction with dependency or blocking spec
- Constitution violation: hard constraint breached
- Showstopper: makes spec unimplementable

**Agent-Driven Codebase Context**

During authoring, the agent reads relevant code to ground specs in reality — no custom scanner code required:

- **Tier 1 (Shape):** Agent reads service/module boundaries, key interfaces and data models, dependency graph, existing patterns and conventions
- **Tier 2 (Specify):** Agent reads file-level details of affected areas, existing test patterns, deployment details, known tech debt

### Dependencies

Slice 2 (Constitution) — constitution check pass, constitution creation.

### Estimated Tasks: 12-15

---

## Slice 4: Execution Bundles & Prime

**Goal:** Generate execution bundles for approved specs. Agents call the prime endpoint for context and report progress/blockers/completion back.

**What you can do after this slice:**

- `specgraph bundle <slug>` — generates lean YAML bundle
- Agents call `POST /prime/<spec-id>` for orientation (constitution, context, conventions, operations)
- Agents report progress, blockers, completion via callbacks
- Spec status updates automatically from agent reports
- Expired claims auto-released by background sweeper

### Components

**Proto — `execution.proto`**

```text
ExecutionService:
  GenerateBundle(BundleRequest) → Bundle
  GetPrime(PrimeRequest) → PrimeResponse
  ReportProgress(ProgressRequest) → ProgressResponse
  ReportBlocker(BlockerRequest) → BlockerResponse
  ReportCompletion(CompletionRequest) → CompletionResponse
```

Bundle message: spec snapshot, bootstrap text, callback config (endpoint, prime URL).
PrimeResponse: constitution summary, project context (constitution + agent-gathered), resolved decisions, coding conventions, callback operation docs.

**Storage — `ExecutionBackend` interface**

```go
type ExecutionBackend interface {
    // Bundle generation — joins spec + decisions + constitution + edges
    GenerateBundle(ctx context.Context, slug string) (*Bundle, error)

    // Agent callback event storage
    RecordProgress(ctx context.Context, slug, agent, message string) error
    RecordBlocker(ctx context.Context, slug, agent, description string) error
    RecordCompletion(ctx context.Context, slug, agent string) error
}
```

**Handler — ExecutionService**

- `GenerateBundle` — assembles bundle from spec + decisions + constitution, returns lean YAML
- `GetPrime` — composes orientation response from constitution + codebase context + decisions
- Callbacks validate agent holds claim, then record event and transition spec status

**CLI**

- `bundle <slug>` — outputs bundle YAML to stdout or `--output` file
- `progress <slug>` — shows execution progress (events, blockers, current status)

**Prime Endpoint**

HTTP endpoint (also RPC) composing:

- Constitution summary (tech stack, constraints, conventions)
- Project context (constitution + agent-gathered — architecture, service boundaries, patterns)
- Resolved decisions for this spec
- Coding conventions (from constitution + codebase scan)
- Callback operation documentation (how to report progress/blockers/completion)

Progressive: agents can request deeper context for specific areas.

**Bundle Format**

Lean YAML — the contract between authoring and execution:

```yaml
bundle:
  version: 1
  spec: { ... }           # Full spec snapshot
  decisions: [ ... ]      # Resolved decisions
  bootstrap: |            # Human-readable instructions
    Call the prime endpoint to get project context...
  callbacks:
    endpoint: "https://..."
    prime: "/prime/spec-k7m3p"
    progress: "/progress/spec-k7m3p"
    blocker: "/blocker/spec-k7m3p"
    completion: "/completion/spec-k7m3p"
```

**Lease Sweeper**

Background goroutine in `serve`:

- Periodically queries for expired `CLAIMED_BY` relationships
- Releases expired claims (deletes relationship, logs event)
- Configurable interval (default: 60s check, 15m default lease)

### Dependencies

Slice 3 (Authoring) — bundles generated from approved specs. Constitution from Slice 2 feeds prime.

### Estimated Tasks: 8-10

---

## Slice 5: Spec Lifecycle

**Goal:** Amend, supersede, and abandon specs. Drift detection. Spec linter with JSON Schema validation.

**What you can do after this slice:**

- `specgraph amend <slug> --reason "..."` — reopen done spec, re-enter funnel
- `specgraph supersede <slug> --with <new-slug>` — replace spec, link old→new
- `specgraph abandon <slug> --reason "..."` — terminal state
- `specgraph drift` — check all done/living specs for divergence
- `specgraph drift <slug>` — check single spec
- `specgraph drift acknowledge <slug> --note "..."` — mark drift as intentional
- `specgraph lint` / `specgraph lint <slug>` — validate spec structure and consistency
- Living specs (`lifecycle: living`) supported

### Components

**Proto extensions**

Extend `spec.proto`:

- Add `lifecycle` field (task/living), `superseded_by`, `supersedes`, `history` repeated field
- Drift messages: `DriftReport`, `DriftItem` (type, severity, description, spec_slug)
- Lint messages: `LintResult`, `LintViolation` (rule, severity, message, location)

Amend/Supersede/Abandon RPCs in AuthoringService (from Slice 3 proto, implemented here).

**Storage**

- Amend: validates spec is done, transitions to amended, bumps version, records reason in history, notifies dependents
- Supersede: creates `[:SUPERSEDES]` edge, updates both specs, notifies dependents
- Abandon: terminal transition, records reason
- Drift queries: compare spec interface fields against upstream dep versions

**Drift Engine**

Three drift types:

- **Interface drift:** Compare spec's `interface` field against stored data (code-level analysis deferred)
- **Verify drift:** Check if `test_path` tests still exist/pass (via `go test` or configured test command)
- **Dependency drift:** Check if upstream spec versions changed since last update

Drift report with severity levels and actionable suggestions.

**Spec JSON Schema**

`spec.schema.json` — formal JSON Schema for the full spec object:

- Progressive validation (3-field minimum for solo, full for enterprise)
- Used by linter for structural validation

**Linter**

Rules:

- Schema validation (required fields, types, enum values)
- Edge consistency (no dangling `depends_on` references)
- Constitution compliance (language allowed, antipattern check)
- Verify criteria format (actionable, testable)
- Cycle detection in dependency graph

**Event Notifications**

`spec.interface_changed`, `spec.dependency_drift` — initially implemented as flags/properties on dependent spec nodes. Full event bus deferred.

### Dependencies

Slices 2-3 (Constitution for lint checks, Authoring for amend re-entering funnel). Can be built partially in parallel with Slice 4.

### Estimated Tasks: 8-10

---

## Slice 6: Sync & Integration

**Goal:** Specs flow outward to Beads, GitHub Issues, and tool-specific workspace files.

**What you can do after this slice:**

- `specgraph sync beads` — push approved specs as beads issues
- `specgraph sync github` — push specs as GitHub Issues
- `specgraph sync status` — show sync state
- `specgraph inject <slug> --tool=claude-code` — write bundle + context into workspace

### Components

**Proto — `sync.proto`**

```text
SyncService:
  SyncBeads(SyncBeadsRequest) → SyncResponse
  SyncGitHub(SyncGitHubRequest) → SyncResponse
  GetSyncStatus(SyncStatusRequest) → SyncStatusResponse
  Inject(InjectRequest) → InjectResponse
```

**SyncAdapter Interface**

```go
type SyncAdapter interface {
    Push(ctx context.Context, spec *Spec) (externalID string, err error)
    Pull(ctx context.Context, externalID string) (status string, err error)
    MapID(ctx context.Context, specID, externalID string) error
    GetSyncState(ctx context.Context) (*SyncState, error)
}
```

**Beads Adapter**

- Creates beads issues from approved specs via `bd` CLI
- Maps `spec-k7m3p` ↔ `beads-xxxxxx`
- Threads bundle content as comment
- Syncs status changes bidirectionally

**GitHub Adapter**

- Creates GitHub Issues via `gh` CLI or API
- Structured body (spec fields formatted as markdown)
- Labels for stage/priority, milestone for epics
- Status sync via poll (webhook support deferred)

**Tool Injection**

- `inject <slug> --tool=claude-code` — writes bundle + constitution subset to workspace
- Formats for: CLAUDE.md section, .cursorrules, AGENTS.md
- Scoped to the specific spec being implemented

**Sync State Storage**

ID mappings and last-sync timestamps stored in Memgraph:

- `(:Spec)-[:SYNCED_TO {adapter, external_id, last_sync}]->(:ExternalRef)` pattern

### Dependencies

Slices 2-4. Beads adapter can start early since Beads is in the project.

### Estimated Tasks: 8-10

---

## Slice 7: Claude Code Plugin

**Goal:** Run the full SpecGraph workflow from Claude Code via skills and hooks.

**What you can do after this slice:**

- `/specgraph-spark`, `/specgraph-shape`, `/specgraph-specify`, `/specgraph-decompose`, `/specgraph-approve`
- `/specgraph-list`, `/specgraph-show`, `/specgraph-deps`, `/specgraph-ready`, `/specgraph-bundle`
- SessionStart hook injects context for active claimed specs
- PostToolUse hook optionally reports progress
- Installable: `claude plugin install specgraph`

### Components

**Plugin Manifest**

`plugin.json` — metadata, skill declarations, hook registrations.

**Authoring Skills**

One SKILL.md per funnel stage. Each skill:

1. Invokes the corresponding CLI command
2. Structures the AI conversation using prompt templates from Slice 3
3. Stores results back via CLI

**Query Skills**

Thin wrappers: `specgraph-list.md`, `specgraph-show.md`, `specgraph-deps.md`, `specgraph-ready.md`.

**SessionStart Hook**

Checks for active claims, injects constitution summary + active spec context.

**Progress Hooks**

Optional PostToolUse hook — reports file writes as progress to SpecGraph server.

### Dependencies

Slices 2-4 (authoring + execution must work via CLI). Slice 6 nice-to-have but not required.

### Estimated Tasks: 6-8

---

## Dependency Graph

```text
Slice 1 (done)
    │
    ▼
Slice 2: Constitution
    │
    ▼
Slice 3: Authoring Funnel
    │
    ├──────────────┐
    ▼              ▼
Slice 4:       Slice 5:
Execution      Lifecycle
    │              │
    ├──────────────┤
    ▼              ▼
Slice 6: Sync & Integration
    │
    ▼
Slice 7: Claude Code Plugin
```

Slices 4 and 5 can be built partially in parallel. Slice 7 needs 2-4 but not 5-6.

---

## What's Deferred

| Item | Reason |
|------|--------|
| Postgres+AGE backend | Memgraph working, no demand yet |
| Tauri 2 + Svelte UI | Phase 4 |
| Federation / Multi-repo | Phase 4 |
| Metrics + reporting | Phase 4 |
| Full event bus | Flags/properties sufficient initially |
| Webhook-based sync | Poll sufficient initially |
| Code-level drift analysis | Interface drift against stored data first |
