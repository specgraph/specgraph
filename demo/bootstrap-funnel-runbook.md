# SpecGraph Bootstrap & Authoring Funnel Demo

A step-by-step runbook for demonstrating SpecGraph's complete workflow inside Claude Code: from bootstrapping a fresh project through the full authoring funnel to querying the spec graph.

## Prerequisites

### 1. Clone and Build

```bash
git clone https://github.com/specgraph/specgraph
cd specgraph
task build
```

This generates proto code and builds the `specgraph` binary in the repo root. The plugin skills invoke `specgraph` by name, so add it to your PATH:

```bash
export PATH="$PWD:$PATH"
```

### 2. Docker

Memgraph runs in Docker. Make sure Docker is running:

```bash
docker info >/dev/null 2>&1 && echo "Docker is ready"
```

### 3. Launch Claude Code with the SpecGraph Plugin

```bash
claude --plugin-dir ./plugin/specgraph
```

Verify the plugin loaded by typing `/specgraph:specgraph` — this invokes the router skill. Skills use the `specgraph:<skill>` format.

---

## Act 1: Bootstrap

### You Say

> Initialize this project for SpecGraph and start the server

### What Happens

- Triggers `specgraph:specgraph-init` skill
- Checks Docker, runs `specgraph init` (derives slug from git remote)
- Falls back to `specgraph serve` if launchd service install fails (common in sandboxed environments)
- Verifies health, lists specs (empty), detects no constitution
- The web UI is now served on the same port as the API — open `http://localhost:8080` in a browser at any point to see the dashboard and graph
- Offers to set up constitution

---

## Act 2: Constitution Setup

### You Say

> /specgraph:specgraph-constitution

### What Happens

Claude scans the codebase (go.mod, CLAUDE.md, proto files, Taskfile, Dockerfile, ADRs) and drafts the constitution section by section:

**Tech Stack** — Claude drafts, you review. Actual output included Go, ConnectRPC, Cobra, Memgraph, testcontainers, protobuf, Murmur3-128, goreleaser, GitHub Actions.

- Your input: *(approved defaults, added svelte as planned frontend, openapi as planned webhook standard)*

**Principles** — 5 drafted from CLAUDE.md and codebase patterns:

- p-1: TDD (three tiers: unit, integration, BDD e2e)
- p-2: Specs are graph nodes, not documents
- p-3: Protobuf is the schema source of truth
- p-4: All multi-query writes use transactions (ADR-004)
- p-5: Prefer vertical slices over horizontal layers
- Your input: **"add domain types"** → Added p-6: Domain types are distinct from proto types

**Constraints** — 10 hard rules pulled from CLAUDE.md MUST rules:

- SPDX license headers, gen/ committed, RunInTransaction, error codes not strings, sentinel errors, content_hash_at_link, conventional commits, quality gates, proto reserved, shared test containers
- Your input: **"LGTM"**

**Antipatterns** — 6 from codebase history:

- Mock DBs, per-test containers (280s vs 8s), string assertions in handlers, editing gen/ directly, skipping transactions, git push/worktree in jj-colocated repo
- Your input: **"LGTM"**

**Process & References** — spec review via funnel, security review triggers, deployment via goreleaser, TDD methodology, three test tiers

- Your input: **"LGTM, good to go"**

Constitution imported (129 lines YAML, stored as graph node in Memgraph): 6 principles, 10 constraints, 6 antipatterns.

---

## Act 3: Authoring Funnel

### 3a. Spark — Capture the Idea

**You say:**

> /specgraph:specgraph-spark I want webhook notifications when specs transition between stages — so external systems like CI, Slack, or dashboards can react to spec lifecycle events

**Conversation flow:**

| Probe | Claude Asks | You Say |
|-------|-------------|---------|
| Seed | Reflects back the seed for confirmation | *(confirmed)* |
| Signal | "Why now? What happened that made this relevant?" | "yeah. We'll be integrating with slack, discord, and various agent execution systems that we might not be able to expose connectrpc/grpc to" |
| Scope sniff | "My read: medium-sized piece. The event emission point is clear (stage transitions already go through TransitionStage), but the webhook subscription/delivery system is a real subsystem." | "yes" |
| Kill test | Proposes 3 candidates: (1) all consumers speak ConnectRPC, (2) event volume too low, (3) delivery reliability outgrows simple HTTP push — pivot to message bus | "3" |

**Result:** Spec `webhook-stage-notifications` created in spark stage.

---

### 3b. Shape — Bound Scope and Explore Approaches

**You say:**

> /specgraph:specgraph-shape webhook-stage-notifications

**Conversation flow:**

| Section | Claude Proposes | You Say |
|---------|-----------------|---------|
| Scope In/Out | 9 in-scope items (stage + lifecycle events, subscription API, HMAC signing, filtering, retry, proto schema, EventBus interface), 7 out-of-scope (guaranteed delivery, DLQ, UI, payload transformation, non-HTTP, rate limiting, disk-backed queue) | *(bug: auto-advanced past approval — see spgr-0vx)* |
| Approaches | A: In-process event bus with async HTTP dispatch (recommended). B: Graph-persisted event queue. C: Handler-level fire-and-forget. Clear gains/loses for each. | "A, with room to consider in process disk backed queue" |
| Decisions | 1: webhook-event-dispatch-model (Approach A, backing-store agnostic). 2: webhook-emit-after-commit (events after transaction, not inside). Both with rationale + rejected alternatives. | "LGTM" |
| Success Criteria | Must: 10 items (event emission, CRUD API, HMAC signing, filtering, bounded retry, circuit breaker, proto schema, transactions, TDD). Should: delivery CLI, health ping, rich payloads, configurable timeout. Won't: 6 items tied to scope-out. | "LGTM" (also: "Use wiremock and testcontainers for the server side of an integration test for that verification") |
| Risks | 5 risks with likelihood/impact/mitigation table. Slow endpoints, HMAC complexity, scattered emit points, in-memory loss, WireMock CI time. | "also need circuitbreakers on #1, otherwise LGTM" |

**Result:** Shape saved with 2 decisions promoted to graph nodes, circuit breaker added as a Must.

---

### 3c. Specify — Define the Interface Contract

**You say:**

> /specgraph:specgraph-specify webhook-stage-notifications

**Conversation flow:**

| Section | Claude Proposes | You Say |
|---------|-----------------|---------|
| Interface Contract | Three surfaces: (A) Proto WebhookService (CRUD for subscriptions), (B) EventBus Go interface (Emit/Subscribe/Close), (C) HTTP delivery format (HMAC-SHA256 signing, custom headers, JSON payload with content_hash). Circuit breaker states (closed/open/half-open). | "should event type be an enum? Feels like it" → Claude checked AuthoringStage enum, proposed WebhookEventType enum (stage_transition, lifecycle_action, execution, drift_detected) with strings for stage values. → "strings are fine, keep all four" |
| Verify Criteria | Organized by domain: Event emission (4 tests), Subscription CRUD (6 tests with specific error codes), Delivery + signing (4 tests), Filtering (3 tests), Retry + circuit breaker (4 tests), EventBus interface (3 tests), E2E Ginkgo/Gomega (2 full-flow tests) | "LGTM" |
| Invariants | 5 invariants: (1) delivery never blocks mutations, (2) HMAC secret write-only, (3) best-effort ordering, (4) circuit breaker per-subscription, (5) no durability guarantees by design (tied to kill test) | "LGTM" |
| Touches | 12 new files (proto, gen, storage domain/interface/memgraph, internal/webhook/ bus/dispatcher/circuit, server handler, CLI), 6 modified files (scoper, memgraph, serve, main, config, authoring/lifecycle/execution handlers), 6 test files with tiers (unit, integration with WireMock+testcontainers, E2E) | "LGTM" |

**Note:** The SpecifyOutput proto flattens all this structured detail into strings. Filed spgr-6vl for evolving the schema to support "free-form but defined" structured containers.

**Result:** Specify saved.

---

### 3d. Decompose — Break into Slices

**You say:**

> /specgraph:specgraph-decompose webhook-stage-notifications

**Conversation flow:**

| Section | Claude Proposes | You Say |
|---------|-----------------|---------|
| Strategy | **Horizontal layer** (not vertical slice): "The webhook system is infrastructure -- there's no user-facing value until all layers connect, and each layer has a clear boundary (storage -> engine -> server wiring -> CLI). The slices are ordered bottom-up so each builds on a tested foundation." Vertical slices don't fit because you can't deliver "just stage_transition webhooks end-to-end" without the full storage + bus + dispatcher stack. | "I generally prefer vertical slices, but it makes sense here. Is there value in plumbing the interfaces all the way through ahead of time though? Size feels right" |
| Restructure | Agent picks up the hint and restructures. Three reasons for front-loading interfaces: (1) TDD is smoother -- slices 2-4 write tests against the contract, no interface churn mid-slice. (2) Parallel work becomes possible -- slices 2 and 3 can use stubs/fakes concurrently. (3) Catches design mismatches early -- if EventBus doesn't fit the handler emission pattern, you find out in slice 1 when cost of change is lowest. Revised slice 1: **interfaces-and-proto** (all contracts, all the way through: proto, domain types, WebhookBackend storage interface, EventBus interface, handler skeleton with stub, CLI skeleton returning "not implemented", WebhookConfig). No implementations -- just seams. Dependency graph changes from linear chain to diamond. | "yes" (persist) |

**Slices (revised):**

| Slice | Intent | Depends on | ~Size |
|-------|--------|------------|-------|
| interfaces-and-proto | All contracts end-to-end (proto service + messages + enum, domain types, WebhookBackend storage interface, EventBus interface, handler registration skeleton, CLI skeleton, WebhookConfig). No implementations -- just seams. Compiles, tests show "not yet implemented." | nothing | 3-4h |
| event-bus-engine | EventBus interface, in-memory channel implementation, goroutine pool dispatcher, HMAC-SHA256 signing, exponential backoff retry, per-subscription circuit breaker | interfaces-and-proto | 3-4h |
| server-integration | ConnectRPC webhook handler for subscription CRUD, emit calls in authoring/lifecycle/execution handlers, server startup wiring | interfaces-and-proto | 3-4h |
| cli-and-e2e | CLI specgraph webhook subcommand (create/get/list/update/delete), BDD e2e tests covering the full flow | event-bus-engine + server-integration | 2-3h |

**Dependency graph (diamond, not linear):**

```text
interfaces-and-proto      (no deps - start immediately)
       /        \
event-bus-engine  server-integration  (parallel - implement against interfaces)
       \        /
     cli-and-e2e                      (needs both)
```

Strategy enum: `DECOMPOSITION_STRATEGY_LAYER_CAKE` (interfaces-first variant of horizontal layer).

**Result:** Decompose saved. Slices 2 and 3 can run in parallel once slice 1 lands.

---

### 3e. Approve — Freeze for Execution

**You say:**

> continue!

*(Continued from decompose -- agent offered to proceed to approve, invoked specgraph:specgraph-approve automatically.)*

**Conversation flow:**

Agent ran all prerequisites (`specgraph show`, `specgraph deps`, `specgraph constitution show`) in a single command, then presented a 6-point conversational checklist in Drive posture:

| # | Check | Verdict | Detail |
|---|-------|---------|--------|
| 1 | Scope bounded? | PASS | 9 in-scope, 7 out-of-scope. Kill test ("pivot to message bus if delivery reliability demands outgrow simple HTTP push") is clear and falsifiable. "Out" list explicitly excludes the things most likely to creep. |
| 2 | Interface defined? | PASS | Full contract across three surfaces: proto WebhookService with 5 RPCs, EventBus Go interface with 3 methods, outbound HTTP delivery format with HMAC signing. Error conditions explicit for every operation with ConnectRPC codes. WebhookEventType enum covers all four event categories. Secret is write-only. Minor note: no max payload size, but deferred as reasonable since payloads are small and fixed-schema. |
| 3 | Verify criteria testable? | PASS | 25 verification criteria, all automatable. Coverage spans: emission (4 tests), subscription CRUD (6), delivery + signing (4), filtering (3), retry + circuit breaker (4), EventBus lifecycle (3), E2E (2). No "manually verify" language anywhere. |
| 4 | Dependencies mapped? | PASS | No external spec dependencies. Self-contained subsystem. Integration points are existing handlers (authoring, lifecycle, execution) which get bus.Emit() calls added -- modifications to this spec's scope, not dependencies on other specs. |
| 5 | Constitution compliance | PASS with one note | Checked all 6 principles + constraints. p-5 (prefer vertical slices): **"Acknowledged deviation"** -- horizontal layer strategy justified by infrastructure nature of the work, interfaces-first restructuring recovers parallelism. All others clean (TDD at every tier, graph nodes, proto source of truth, transactions, domain types distinct from proto, SPDX headers, sentinel errors). |
| 6 | Risk acknowledgment | PASS | 5 risks from Shape, all with documented mitigations. Highest-likelihood risk (slow endpoints) has three layers of mitigation (timeout + circuit breaker + bounded pool). Kill test provides escape hatch. |

**Assessment: Recommend approval. No holds.**

- Your input: **"yes"**

**Result:** Approved at 2026-03-22T00:19:53Z. Spec is frozen for execution.

**Next steps offered:**

- Generate execution bundle: `/specgraph-bundle webhook-stage-notifications`
- Claim and implement: `specgraph claim webhook-stage-notifications --agent <name>`
- Leave it in the graph, approved and ready whenever someone picks it up

---

## Act 4: Graph Exploration

### 4a. Web UI — Visual Graph Explorer

Open the SpecGraph dashboard in your browser:

```text
http://localhost:8080
```

**Dashboard (`/`)** — shows at a glance:

- Stats cards: total specs, ready count, drift count, decisions
- Authoring funnel bar: color-coded by stage (spark → shape → specify → decompose → approved → done)
- Mini graph preview: compact dependency graph, click to expand

**Graph View (`/graph`)** — click "Graph" in the nav bar:

- Interactive Dagre-layouted dependency graph
- Specs are rounded rectangles, decisions are diamonds
- Color-coded by stage (purple=spark, blue=shape, green=specify, amber=decompose, teal=approved, gray=done)
- Hover any node for a tooltip (slug, intent, stage, priority)
- Click a node to navigate to its detail page
- Use the search bar to filter — matching nodes stay highlighted, non-matching fade out
- Pan (drag background) and zoom (mouse wheel) to explore large graphs

**Detail Pages** — click any node in the graph:

- `/spec/webhook-stage-notifications` — full metadata, notes, breadcrumb nav back to graph
- `/decision/webhook-event-dispatch-model` — decision text, rationale, context

### 4b. CLI — Query the Graph

For CLI users and scripting:

```bash
specgraph show webhook-stage-notifications    # full spec with all stage outputs
specgraph deps webhook-stage-notifications    # dependency tree
specgraph list                                 # all specs
```

---

## Teardown

```bash
specgraph down --rm
```

---

## Known Issues Found During Demo

| Issue | Bead | Status |
|-------|------|--------|
| `specgraph up` fails with relative binary path in LaunchAgent | spgr-kzl (P1) | Open |
| Shape skill: background scan auto-advances past scope approval | spgr-0vx (P1) | Open |
| Shape JSON format: em dashes and schema mismatch caused 3 retries | *(fixed — added references/shape-output-format.md)* | Fixed |
| SpecifyOutput proto flattens structured detail to strings | spgr-6vl (P2) | Open |
| Skill format references need to stay in sync with proto changes | spgr-p9h (P2) | Open |
| CreateSpec allows duplicate slugs -- no uniqueness enforcement | spgr-ecm (P1) | Open |

## Tips for Recording

- **First run:** Do a practice run to warm up Docker
- **Server startup:** If recording, start the server before recording to avoid the 30-60s Docker pull
- **Posture:** Short input → Drive posture (agent leads). Longer input → Partner posture.
- **Analytical passes:** Call out when they fire — constitution_check validates against ground truth, red_team probes vulnerabilities, consistency checks for contradictions
