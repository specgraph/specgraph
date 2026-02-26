# SpecGraph ADR-002: Gastown Integration

> **This is an optional integration.** SpecGraph works fully without Gastown — specs, authoring, bundles, constitution, and all graph analysis function on either backend with or without multi-agent orchestration. Gastown integration requires the **Beads (+Dolt) backend**; the Postgres+AGE path does not natively connect to Gastown. See ADR-001 (v3) for the full composability model.

## Context

Gas Town is a multi-agent workspace manager that coordinates Claude Code instances (and other agents like Codex, Cursor, Augment, AMP) via tmux, git worktrees, and Beads. It already has:

**Hierarchy:**
- **Town** (`~/gt/`) — workspace root, houses all rigs
- **Mayor** — AI coordinator (Claude Code instance with full workspace context). You talk to the Mayor; the Mayor orchestrates everything.
- **Rig** — project container wrapping a git repo. Has its own polecats, witness, refinery, crew.
- **Polecats** — ephemeral worker agents. Spawn, do a task in an isolated git worktree, submit MR, disappear. Swarms of these do bulk work.
- **Crew** — persistent, named agents for ongoing collaboration. Maintain context across sessions.
- **Witness** — rig-level monitor. Watches polecats, detects stuck agents, triggers recovery.
- **Refinery** — merge queue processor. Lands polecat MRs on main.
- **Deacon** — daemon watchdog. Patrols, health checks, ensures the whole system keeps running.

**Work flow:**
```
You → Mayor → creates beads + convoy
                    │
              gt sling <bead> <rig>
                    │
              Polecat spawns (fresh worktree)
              Work lands on HOOK
              GUPP: "If hook has work, RUN IT"
                    │
              Polecat executes autonomously
                    │
              gt done → MR to Refinery
              Refinery merges to main
              Convoy tracks completion
```

**Key Gastown insight:** "Polecats thrive on well-defined, fully-spec'ed beads epics — they shouldn't have to make decisions."

This is **exactly** SpecGraph's value proposition. The authoring funnel exists to produce specs that are clear enough for a polecat to execute without ambiguity.

**Beads is the data plane.** Every piece of work in Gastown is a bead. Convoys wrap beads. Hooks pin beads. Formulas create beads with dependency chains. Wisps are ephemeral beads for transient orchestration. Dolt branches isolate polecat writes. This is not optional infrastructure — it IS Gastown.

## The Integration: SpecGraph Produces What Gastown Consumes

SpecGraph sits upstream of Gastown. It does the design work; Gastown does the execution:

```
DESIGN TIME (SpecGraph)                    EXECUTION TIME (Gastown)
                                           
  Human + AI authoring agent               Mayor + Polecats + Crew
          │                                        │
    ┌─────▼──────┐                          ┌──────▼──────┐
    │   Spark    │  vague idea              │  gt sling   │
    │   Shape    │  scope, tradeoffs        │  (dispatch) │
    │   Specify  │  interface, verify       └──────┬──────┘
    │  Decompose │  vertical slices                │
    │   Approve  │  ready for execution     ┌──────▼──────┐
    └─────┬──────┘                          │   Polecat   │
          │                                 │ reads hook  │
    ┌─────▼──────┐                          │ sees spec   │
    │   Beads    │ ◄────────────────────► │ has bundle  │
    │  (Dolt)    │  same database           │ executes    │
    └────────────┘                          └──────┬──────┘
                                                   │
                                            ┌──────▼──────┐
                                            │  gt done    │
                                            │  MR → Ref.  │
                                            └─────────────┘
```

There is no adapter. There is no bridge. SpecGraph writes specs as beads into the same Beads/Dolt store that Gastown reads. The Mayor sees them. `bd ready` surfaces them. `gt sling` dispatches them.

## How Specs Become Beads

### Custom Issue Type

SpecGraph registers a `spec` type in Beads:

```yaml
# .beads/config.yaml
types:
  custom:
    - name: spec
      description: "SpecGraph specification — design-complete work unit"
```

A spec-type bead carries the full spec schema in its description/body as structured YAML:

```bash
bd create --type spec --title "OAuth2 refresh token rotation" -p 1 \
  --description "$(cat spec-body.yaml)"
```

Where the body contains:

```yaml
intent: |
  Implement automatic refresh token rotation per RFC 6749.
  When a refresh token is used, issue a new one and invalidate the old.

stage: approved

interface:
  input:
    endpoint: "POST /auth/token"
    body: { grant_type: "refresh_token", refresh_token: "string" }
  output:
    success: { access_token, refresh_token, expires_in }
    errors: [invalid_grant, server_error]
  side_effects:
    - "Previous refresh token invalidated"
    - "New token pair stored in token_store"

verify:
  - "Refresh token rotation returns new token pair"
  - "Old refresh token rejected after rotation"
  - "Concurrent rotation attempts: exactly one succeeds"
  - "Token family detection: if reuse detected, revoke entire family"

invariants:
  - "Token store never contains two valid refresh tokens for same session"
  - "Access token lifetime unchanged by rotation"

complexity: medium

decisions:
  - decision: "Token family tracking via lineage chain, not session table"
    rationale: "Avoids schema migration, works with existing token_store"
    rejected: ["Session-based tracking (requires new table)", "Stateless (can't detect reuse)"]

constitution_ref: "v3"   # which version of the constitution this was authored against
```

### Dependency Edges via Beads Links

```bash
# This spec depends on user-model being done first
bd link <oauth-refresh-id> --blocks <user-model-id>

# This spec is part of the auth-service-v2 epic
bd link <auth-v2-epic-id> --parent <oauth-refresh-id>

# This spec references (but doesn't block on) the rate-limiter
bd link <oauth-refresh-id> --relates-to <rate-limiter-id>
```

Beads' existing link types map directly:
| SpecGraph Edge | Beads Link |
|---|---|
| `depends_on` | `--blocks` (target blocks source) |
| `composes` | `--parent` (parent-child) |
| `references` | `--relates-to` |

### The Ready → Sling Pipeline

Once a spec is approved and its dependencies are met:

```bash
# bd ready already does this — lists beads with no open blockers
bd ready --type spec

# Output:
# bd-k7m3p  P1  OAuth2 refresh token rotation    [spec] [approved]
# bd-w2x9q  P2  Dashboard error handling          [spec] [approved]

# Mayor (or human) creates a convoy and slings
gt convoy create "Auth Sprint" bd-k7m3p bd-w2x9q --notify
gt sling bd-k7m3p myproject
gt sling bd-w2x9q myproject
```

Each sling spawns a polecat. The polecat wakes up, checks its hook, sees the spec bead.

## The Execution Bundle: What the Polecat Actually Reads

This is SpecGraph's key contribution. A raw bead has a title and description. A spec bead has structured intent, interface, and verification criteria. But a **polecat needs more** — it needs the full execution context to work autonomously without making design decisions.

### What `specgraph bundle` Produces

When a spec is about to be slung, SpecGraph generates an execution bundle. This is attached to the bead (as a linked message bead or structured comment) before slinging:

```yaml
# Execution bundle for bd-k7m3p
spec: bd-k7m3p
title: "OAuth2 refresh token rotation"
generated_at: "2025-02-25T10:00:00Z"

# === FROM THE SPEC ===
intent: |
  Implement automatic refresh token rotation per RFC 6749...

interface:
  input: { ... }
  output: { ... }
  side_effects: [ ... ]

verify:
  - description: "Refresh token rotation returns new token pair"
    hint: "Test via POST /auth/token with valid refresh token"
  - description: "Old refresh token rejected after rotation"
    hint: "Use the old token immediately after rotation — expect 401"
  - description: "Concurrent rotation attempts: exactly one succeeds"
    hint: "Race two goroutines with same refresh token"
  - description: "Token family detection"
    hint: "Reuse an old token, verify entire family revoked"

invariants:
  - "Token store never contains two valid refresh tokens for same session"

decisions:
  - "Token family tracking via lineage chain, not session table"

# === FROM THE CONSTITUTION (filtered for this spec) ===
constitution:
  tech:
    language: go
    framework: chi
    db: postgres
  conventions:
    - "All handlers in internal/handler/"
    - "Business logic in internal/service/"
    - "Database access via internal/store/ — never from handlers"
    - "Errors wrapped with fmt.Errorf and %w"
  constraints:
    - "No new dependencies without review"
    - "All endpoints must have OpenAPI annotations"
  patterns:
    error_handling: "Return domain errors from service, map to HTTP in handler"
    testing: "Table-driven tests, testcontainers for integration"

# === FROM CODEBASE CONTEXT (scanned) ===
codebase:
  relevant_files:
    - path: "internal/handler/auth.go"
      role: "Existing auth handlers — add refresh endpoint here"
    - path: "internal/service/token.go"
      role: "Token service — add rotation logic here"
    - path: "internal/store/token_store.go"
      role: "Token persistence — add invalidation + family tracking"
    - path: "internal/model/token.go"
      role: "Token model — may need lineage_parent field"
  
  existing_patterns:
    - name: "Handler structure"
      example_file: "internal/handler/login.go"
      note: "Follow this pattern for the new refresh endpoint"
    - name: "Service error handling"
      example_file: "internal/service/session.go"
      note: "Wraps store errors, returns domain errors"
  
  test_commands:
    unit: "go test ./internal/service/..."
    integration: "go test -tags=integration ./internal/store/..."
    lint: "golangci-lint run"

# === RESOLVED UPSTREAM INTERFACES ===
upstream:
  - spec: "bd-m1n2o3 (user-model)"
    status: done
    interface:
      "User struct in internal/model/user.go"
      "UserStore.GetByID(ctx, id) → (*User, error)"
```

This bundle is everything a polecat needs to execute without asking questions. It knows:
- What to build (interface contract)
- How to verify it worked (verification criteria with hints)
- What decisions were already made (don't re-decide)
- How the codebase is structured (where to put things)
- What patterns to follow (copy these, not invent new ones)
- What upstream interfaces it can depend on (already done)
- How to run tests

### Bundle Delivery to the Polecat

Three options, in order of preference:

**Option 1: Linked message bead (recommended)**

```bash
# SpecGraph creates the bundle as a message bead linked to the spec
bd create --type message --thread bd-k7m3p \
  --title "Execution Bundle" \
  --description "$(specgraph bundle bd-k7m3p --format yaml)"
```

The polecat, upon reading its hooked bead, follows the thread to find the execution bundle. This is how Gastown already handles supplementary context — message beads threaded to work beads.

**Option 2: Formula with bundle step**

SpecGraph registers a formula that includes bundle generation as a step:

```toml
# .beads/formulas/specgraph-execute.formula.toml
formula = "specgraph-execute"
description = "Execute a SpecGraph spec with full context"

[[steps]]
id = "bundle"
description = "Generate execution bundle from spec + constitution + codebase"

[[steps]]
id = "implement"
needs = ["bundle"]
description = "Implement the spec per the execution bundle"

[[steps]]
id = "verify"
needs = ["implement"]
description = "Run all verification criteria from the spec"

[[steps]]
id = "submit"
needs = ["verify"]
description = "Submit MR with spec reference in description"
```

This wraps the SpecGraph execution flow as a Gastown molecule. Each step is a bead. The molecule survives crashes — if a polecat dies mid-implement, a new one picks up from the verify step.

**Option 3: CLAUDE.md injection**

For simpler setups, `specgraph inject` writes the bundle into the polecat's worktree as CLAUDE.md context:

```bash
# Before or during sling, inject context into the worktree
specgraph inject bd-k7m3p --worktree ~/gt/myproject/polecats/Furiosa/
```

This writes `.specgraph/context.md` into the worktree, which gets picked up by Claude Code's context loading.

## Where the Mayor Fits

The Mayor is not just a dispatcher. It's the AI coordinator with full workspace context. SpecGraph's authoring funnel can run **inside a Mayor session**:

```
You → Mayor: "We need OAuth2 refresh token rotation"

Mayor (using SpecGraph):
  1. specgraph spark → captures the idea
  2. specgraph shape → asks you about scope, identifies edges
  3. specgraph specify → drafts interface contract, verification criteria
  4. specgraph decompose → if needed, breaks into sub-specs
  5. Validates against constitution
  6. Runs red team pass
  7. specgraph approve → marks ready

Mayor (using Gastown):
  8. bd ready → confirms spec is ready (deps met)
  9. specgraph bundle → generates execution bundle
  10. gt convoy create → wraps the spec(s) for tracking
  11. gt sling → dispatches to polecat(s)
  12. Monitors via convoy status, witness, feed
```

The Mayor is the natural home for SpecGraph's authoring agent. It already has:
- Full workspace context (knows all rigs, all agents, all in-progress work)
- Beads access (can create, link, query specs)
- Coordination authority (can sling, convoy, nudge)
- Visibility (knows what's blocked, what's done, what's stuck)

### SpecGraph as Mayor Context

The Mayor's CLAUDE.md would include SpecGraph context:

```markdown
## SpecGraph

You have access to the specgraph CLI for design-driven development.
Use it when the user describes features, asks for architecture, or
wants to plan work before executing.

### Available commands:
- `specgraph spark <slug>` — capture a vague idea
- `specgraph shape <slug>` — scope and tradeoff analysis  
- `specgraph specify <slug>` — interface contract + verification
- `specgraph decompose <slug>` — break into implementable slices
- `specgraph approve <slug>` — mark ready for execution
- `specgraph bundle <slug>` — generate execution bundle for polecat
- `specgraph constitution show` — view project standards
- `specgraph ready` — list specs ready for slinging

### Workflow:
When the user wants to build something:
1. Use specgraph to design it (spark → approve)
2. Generate bundle
3. Sling to polecats via gt sling

When a polecat reports a blocker:
1. Check if the blocker is a spec issue (bad interface, missing invariant)
2. If yes, update the spec and re-bundle
3. Re-sling to a fresh polecat
```

## The Crew vs Polecat Distinction

Gastown has two kinds of workers that interact with specs differently:

**Polecats** (ephemeral, swarm workers):
- Get fully-spec'd work. Don't make decisions.
- Read the execution bundle. Follow it. Submit MR.
- Perfect for: implementing a single, well-defined spec.
- SpecGraph ensures the spec IS well-defined enough for a polecat.

**Crew** (persistent, thoughtful workers):
- Handle ambiguous, design-heavy work. CAN make decisions.
- Perfect for: SpecGraph authoring itself (shaping, specifying).
- A crew member could run the full authoring funnel on a vague idea.
- Also good for: reviewing polecat output against the spec's verify criteria.

This maps cleanly:

```
AUTHORING (Crew or Mayor):
  "Design the OAuth refresh token rotation"
  → Crew member runs specgraph shape, specify
  → Produces a well-spec'd bead

EXECUTION (Polecats):
  "Implement bd-k7m3p per the execution bundle"
  → Polecat follows the bundle
  → Submits MR

REVIEW (Crew or Witness):
  "Verify bd-k7m3p meets its spec"
  → Crew member checks MR against verify criteria
  → Approves or sends back with feedback
```

## Formulas: Encoding the SpecGraph Workflow

Gastown formulas are TOML-defined multi-step workflows. SpecGraph can ship formulas for its own patterns:

### Full lifecycle formula

```toml
# .beads/formulas/specgraph-lifecycle.formula.toml
formula = "specgraph-lifecycle"
description = "Full spec lifecycle: design → implement → verify → ship"

[[steps]]
id = "design"
description = "Run specgraph authoring funnel (spark → approve)"
# Assigned to Crew (needs judgment)

[[steps]]
id = "bundle"
needs = ["design"]
description = "Generate execution bundle"

[[steps]]
id = "implement"
needs = ["bundle"]
description = "Implement per execution bundle"
# Assigned to Polecat (fully spec'd, no decisions)

[[steps]]
id = "verify"  
needs = ["implement"]
description = "Verify implementation against spec criteria"
# Can be automated (run tests) or Crew (review)

[[steps]]
id = "submit"
needs = ["verify"]
description = "Submit MR and close spec"
```

### Swarm implementation formula (for decomposed epics)

```toml
# .beads/formulas/specgraph-swarm.formula.toml  
formula = "specgraph-swarm"
description = "Parallel implementation of decomposed spec"

# One design step for the epic
[[steps]]
id = "design-epic"
description = "Design and decompose the epic into specs"

# Gate: all sub-specs must be approved
[[steps]]
id = "approve-all"
needs = ["design-epic"]
description = "All sub-specs approved and bundled"

# Parallel implementation — each sub-spec slung to its own polecat
# (Gastown handles the fanout via convoy)
[[steps]]
id = "implement-all"
needs = ["approve-all"]
description = "Sling all sub-specs to polecats"

# Gate: all MRs merged
[[steps]]
id = "integrate"
needs = ["implement-all"]
description = "All sub-spec MRs merged, integration test"

[[steps]]
id = "close-epic"
needs = ["integrate"]
description = "Mark epic spec as done"
```

## Handling Failures: When Polecats Get Stuck

Polecats fail. They get confused, hit ambiguity, or discover the spec was wrong. SpecGraph needs to handle the feedback loop:

### Polecat reports a blocker

```bash
# Polecat discovers the spec is wrong/ambiguous
bd create --type message --thread bd-k7m3p \
  --title "BLOCKER: Token store interface doesn't support family queries" \
  --description "The spec says to track token families via lineage chain, 
  but token_store.go has no method for querying by lineage. Need either 
  a new store method or a different approach."
```

The Witness (or Mayor) sees this. The flow:

1. Witness detects stuck polecat
2. Mayor (or Crew member) reviews the blocker
3. If spec issue: `specgraph reshape bd-k7m3p` → update interface/decisions → re-bundle
4. If implementation issue: nudge polecat with clarification via `gt mail`
5. If fundamental problem: `specgraph block bd-k7m3p --reason "..."` → convoy paused

### Spec-level retry

```bash
# Polecat gave up or produced bad work
gt sling bd-k7m3p myproject --force  # new polecat, same spec

# But first, update the bundle if the spec was the problem
specgraph bundle bd-k7m3p --refresh   # regenerate with updated context
```

## MCP Integration (Optional Enhancement)

For richer agent ↔ SpecGraph interaction, an MCP server can expose SpecGraph operations. This is useful when the polecat (or crew member) needs to query project context mid-task:

```
MCP Tools (for coding agents mid-task):
  specgraph_constitution  → "What are project standards?"
  specgraph_pattern       → "How does this codebase do error handling?"
  specgraph_check         → "Does this approach violate any constraints?"
  specgraph_blocker       → "Report a blocker on this spec"
  specgraph_verify_hint   → "How should I test this verify criterion?"

MCP Tools (for Mayor / authoring agents):
  specgraph_spark         → Create a spark
  specgraph_shape         → Run shaping
  specgraph_specify       → Draft interface + verify
  specgraph_approve       → Mark ready
  specgraph_bundle        → Generate execution bundle
  specgraph_ready         → List ready specs
  specgraph_deps          → Show dependency graph
  specgraph_critical_path → What's the longest unresolved chain?
```

This is optional because polecats can just read the execution bundle (which has everything pre-resolved). MCP is for when they need to go deeper or hit unexpected situations.

## What SpecGraph Does NOT Touch in Gastown

SpecGraph is upstream of execution. It does not:

- Manage polecats, crew, witnesses, refineries (that's Gastown)
- Handle git worktree creation or isolation (that's Gastown)
- Process merge queues or MRs (that's the Refinery)
- Monitor agent health or detect stuck workers (that's the Witness/Deacon)
- Route mail or messages between agents (that's Gastown mail)
- Handle handoffs or session management (that's Gastown)
- Create or manage convoys (that's the Mayor, using Gastown)

SpecGraph's boundary is: **design the work, produce the bundle, write it as a bead. Gastown takes it from there.**

## Revised Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     SPECGRAPH                            │
│                                                          │
│  Constitution ─┐                                         │
│  Codebase ctx ─┤─→ Authoring Funnel ──→ Spec Beads      │
│  Human intent ─┘   (spark→approve)      (bd create)     │
│                         │                    │           │
│                         │              Bundle Generator  │
│                         │              (spec + constit.  │
│                         │               + codebase ctx)  │
│                         │                    │           │
│                         ▼                    ▼           │
│              ┌─────────────────────────────────┐         │
│              │    BEADS (+Dolt)                 │         │
│              │    Spec beads, links, bundles    │         │
│              │    (shared data plane)           │         │
│              └──────────────┬──────────────────┘         │
└─────────────────────────────┼────────────────────────────┘
                              │
┌─────────────────────────────┼────────────────────────────┐
│                     GASTOWN │                             │
│                             │                             │
│  Mayor ◄────────────────────┘                             │
│    │      reads specs, creates convoys                    │
│    │                                                      │
│    ├──→ gt sling ──→ Polecat (reads hook → reads bundle)  │
│    │                    │                                  │
│    │                    ├──→ executes per bundle            │
│    │                    ├──→ gt done (MR to Refinery)       │
│    │                    └──→ reports blockers               │
│    │                                                      │
│    ├──→ Witness (monitors polecats)                        │
│    ├──→ Refinery (merges MRs to main)                      │
│    └──→ Deacon (system health)                             │
│                                                           │
│  Convoy tracking, dashboards, mail, formulas, molecules   │
└───────────────────────────────────────────────────────────┘
```

## What This Means for the Postgres Path

Teams on the Postgres path (no Beads/Dolt) lose native Gastown integration. They need:

1. **A Gastown adapter** that reads Postgres specs and writes them as beads for Gastown to consume. This is a sync process, not a native integration.
2. **Or** they don't use Gastown. They use a simpler execution model — `specgraph inject` writes context into worktrees, and they manage agents manually or with their own tooling.

This is an honest tradeoff. The Beads path gets Gastown for free. The Postgres path gets enterprise Postgres infrastructure but pays with a Gastown adapter or manual execution.

```
BEADS PATH:  SpecGraph → Beads → Gastown (native, no adapter)
POSTGRES PATH:  SpecGraph → Postgres → ??? → Gastown (adapter needed)
                                      └──→ Manual execution (no Gastown)
```

This is a real decision teams make. SpecGraph should be clear about it.
