# Bootstrap & Authoring Funnel Demo — Design Spec

## Goal

A reproducible runbook that walks through SpecGraph's complete bootstrap → constitution → authoring funnel → graph queries flow inside Claude Code, using the SpecGraph plugin skills. Suitable for team onboarding and eventual VHS recording.

## Audience

Internal team and early adopters. Can show internals (graph structure, analytical passes, decision promotion).

## Deliverable

A markdown runbook (`demo/bootstrap-funnel-runbook.md`) documenting the complete flow step by step. The runbook is a guide for a human running the demo inside Claude Code — not an automated script.

## Prerequisites

The runbook documents these setup steps:

1. **Install SpecGraph binary** — `go install github.com/specgraph/specgraph/cmd/specgraph@latest` or `brew install specgraph/tap/specgraph`
2. **Clone the repo** — `git clone https://github.com/specgraph/specgraph && cd specgraph`
3. **Start Docker** — Memgraph runs in Docker via testcontainers
4. **Verify plugin** — The `.claude/plugins/specgraph` symlink is committed; confirm skills are available by typing `/specgraph-list` in Claude Code

## Demo Flow

### Act 1: Bootstrap (inside Claude Code)

The user asks Claude to initialize the project:

> "Initialize this project for SpecGraph and start the server"

Claude runs:

- `specgraph init` — derives slug from git remote (`specgraph/specgraph`), auto-starts server
- `specgraph list` — confirms empty project, server healthy

**What to look for:** `.specgraph.yaml` created, server running, empty spec list.

### Act 2: Constitution Setup (inside Claude Code)

The user asks Claude to create a constitution:

> "Create a project constitution for SpecGraph based on the codebase"

Claude:

- Reads the codebase to understand tech stack, principles, patterns
- Crafts a constitution YAML covering: Go 1.26, ConnectRPC, Memgraph, protobuf, key principles (YAGNI, composition-over-inheritance, specs-as-graphs), constraints, antipatterns
- Runs `specgraph constitution import constitution.yaml`
- Runs `specgraph constitution show` to verify

**What to look for:** Constitution loaded with project-specific tech stack and principles. These will be checked by the `constitution_check` analytical pass during authoring.

### Act 3: Authoring Funnel (interactive Claude Code session)

#### 3a. Spark

User types: `/specgraph-spark`

Then provides the idea: "I want webhook notifications when specs transition between stages — so external systems like CI, Slack, or dashboards can react to spec lifecycle events."

Claude runs the Spark skill:

- Elicits seed, signal, scope sniff, kill test through conversation
- Submits via `specgraph spark webhook-stage-notifications --seed "..."`
- Constitution check fires automatically

**What to look for:** Spec created in Spark stage. Safety flags if any. Claude's conversational elicitation style.

#### 3b. Shape

User types: `/specgraph-shape webhook-stage-notifications`

Claude runs the Shape skill in Drive posture:

- Bounds scope (in: stage transition webhooks, retry on failure; out: streaming/SSE, UI dashboard)
- Explores 2-3 approaches (HTTP POST webhooks vs. message queue vs. polling)
- Captures a decision about delivery semantics (at-least-once with idempotency keys)
- Surfaces risks (webhook endpoint down, retry storms, secret management for signing)
- Submits via `specgraph shape webhook-stage-notifications --json-file shape-output.json`

**What to look for:** Decision promoted to a first-class graph node (visible via `specgraph show`). Peripheral vision pass fires (if in Drive posture). Scope explicitly bounded.

#### 3c. Specify

User types: `/specgraph-specify webhook-stage-notifications`

Claude runs the Specify skill:

- Defines interface contract (HTTP POST to registered URLs, JSON event payload with spec slug, from/to stage, timestamp, HMAC signature)
- Sets verify criteria (delivery confirmed within 30s, retries up to 3x with exponential backoff)
- Documents invariants (events are ordered per-spec, idempotency key prevents duplicate processing)
- Lists touched files (`internal/webhook/`, `internal/server/`, `proto/specgraph/v1/webhook.proto`)
- Submits via `specgraph specify webhook-stage-notifications --json-file specify-output.json`

**What to look for:** Red team and consistency check passes fire. Interface contract is concrete and testable.

#### 3d. Decompose

User types: `/specgraph-decompose webhook-stage-notifications`

Claude runs the Decompose skill:

- Chooses vertical slice strategy
- Defines 3 slices:
  1. **Event emitter** — fire-and-forget event when stage transitions (internal, no external I/O)
  2. **Webhook dispatcher** — HTTP POST to registered URLs with signing and timeout
  3. **Retry queue** — persistent retry with exponential backoff and dead-letter
- Each slice has verify criteria and dependency declarations
- Submits via `specgraph decompose webhook-stage-notifications --json-file decompose-output.json`

**What to look for:** Simplicity check fires. Slices have explicit dependencies. Child spec slugs created.

#### 3e. Approve

User types: `/specgraph-approve webhook-stage-notifications`

Claude runs the Approve skill:

- Reviews the complete spec across all stages
- Submits via `specgraph approve webhook-stage-notifications`

**What to look for:** Spec transitions to `approved` stage. Approval timestamp recorded.

### Act 4: Graph Exploration (inside Claude Code)

User asks Claude to explore the graph:

> "Show me the spec and its graph relationships"

Claude runs:

- `specgraph show webhook-stage-notifications` — full spec with all stage outputs
- `specgraph deps webhook-stage-notifications` — dependency tree
- `specgraph list` — all specs in the project

**What to look for:** The spec has all 5 stages populated. Decisions appear as linked nodes. Child specs from decomposition visible. The graph is queryable, not just a static document.

## Example Spec Content

The demo uses "webhook-stage-notifications" — a feature for SpecGraph itself that sends HTTP webhooks when specs transition between authoring stages. This is self-referential (SpecGraph authoring a spec about its own features) and realistic enough to demonstrate all funnel stages meaningfully.

## Notes

- The runbook documents expected Claude outputs and suggested user responses at each interactive prompt, but the conversation is organic — Claude's skills adapt to the user's input.
- The funnel skills run analytical passes automatically in Drive posture. The demo should call out when passes fire and what findings look like.
- Constitution setup is done conversationally (Claude reads the codebase and crafts the YAML) rather than importing a pre-made fixture. This is more impressive for onboarding.
- The runbook should note that the first run may take 30-60 seconds as Docker pulls the Memgraph image.
- Include a teardown section: `specgraph uninstall` to unregister the service and stop the container (data preserved), or `task dev:reset` for a full wipe-and-rebuild. Do not reference `specgraph down --rm` — it was retired in 2026-04-22-cli-lifecycle-split-design.md.
