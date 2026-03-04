# The Problem

**Why specs-as-files fails**

---

## The Status Quo

Most teams manage specifications the same way: markdown files in a `specs/` folder,
Confluence pages, Jira epics, or Google Docs shared across Slack threads. A product
manager writes up requirements, an engineer adds technical details, and the document
gets reviewed in a pull request or a meeting. Then it sits there. The spec was useful
for the two weeks it took to align the team, but by the time implementation starts,
it's already drifting from reality.

The problem isn't that teams don't write specs — it's that the format can't keep up.
A markdown file doesn't know what other specs depend on it. A Jira ticket can link to
other tickets, but those links carry no semantics: is this a hard dependency, a
soft reference, or just "related"? A wiki page can be updated, but nothing forces
consistency between the page and the codebase it describes.

When AI agents enter the picture, these gaps become acute. An agent that receives
"implement the feature in spec-42.md" faces a wall of prose with no machine-readable
structure, no way to extract acceptance criteria programmatically, and no connection
to the project's architectural constraints. The spec format that was "good enough" for
human teams actively blocks agent-assisted development.

---

## Six Gaps

### No Live Query

You can't ask your spec repository a question. "What specs are currently blocked?"
"Which specs changed since the last release?" "Show me everything downstream of the
auth service." These are natural questions for anyone planning a sprint or assessing
risk, but answering them requires writing a custom script that parses filenames,
greps for keywords, and hopes the results are accurate.

**Example:** A team lead preparing for a release wants to know the critical path —
which specs must be completed before the release can ship, and which of those are
blocked. With specs-as-files, they open each document, manually trace the
`depends_on` references, build a mental graph, and hope they didn't miss a
transitive dependency three levels deep. With a live query layer, it's a single
traversal.

### No Addressability

Specs reference each other by file path or ticket number. This creates a brittle
web of string-based pointers that breaks whenever someone reorganizes a directory,
renames a file, or migrates from one tool to another. There's no stable identity
behind the reference — just a path that might or might not resolve.

**Example:** Your auth spec lives at `specs/auth/login-spec.md` and twelve other
specs reference it with `depends_on: ../auth/login-spec.md`. The team decides to
reorganize the `auth/` directory into `identity/authn/` and `identity/authz/`.
Every reference breaks silently. Nothing warns you. The specs still render fine in
your markdown viewer — the dangling references just quietly become lies.

### No Execution Interface

AI agents and automation systems need structured task graphs with clear inputs,
outputs, dependencies, and completion criteria. Prose documents — even well-written
ones — don't provide that. An agent can read a spec, but it can't reliably extract
what to build, what to build it on top of, what "done" looks like, or where to
report status.

**Example:** An orchestration agent receives the instruction "implement the feature
described in spec-42.md." It parses the markdown and finds requirements written in
natural language, a section labeled "Dependencies" that lists filenames it can't
resolve, and acceptance criteria phrased as "the user should be able to..." The agent
has to guess at structure, infer dependency ordering, and improvise a completion
check. Every ambiguity becomes a decision the agent makes silently, without the
context to make it well.

### No Ground Truth

Every authoring session starts cold. When a developer or an AI agent sits down to
write a new spec, they don't know the project's tech stack, architectural principles,
naming conventions, or constraints — unless someone explicitly tells them. That
context lives in people's heads, in scattered ADRs, or in tribal knowledge that never
got written down.

**Example:** An AI agent is asked to draft a spec for a new microservice. The project
is exclusively Go, uses ConnectRPC for service communication, and stores data in
PostgreSQL. But nothing in the spec environment encodes these constraints. The agent
suggests a Java service with REST endpoints and a MongoDB store — a technically valid
design that violates every architectural decision the team has made. The review
catches it, but the rework costs a full sprint cycle.

### No Codebase Awareness

Specs are written in a vacuum, disconnected from the actual codebase they describe.
The author doesn't know — and has no way to discover — what patterns the code already
uses, what interfaces exist, or what conventions the team follows. The spec describes
what should be built without any grounding in what already exists.

**Example:** A spec describes a new API endpoint for user profiles. It proposes a
handler structure, middleware chain, and test approach. But the codebase already has
an established router pattern, a standard middleware stack, and test helpers that
handle setup and teardown. The spec author (human or AI) didn't know any of this, so
the spec describes an implementation that's structurally incompatible with the
existing code. The implementing engineer has to mentally diff the spec against the
codebase and reconcile the two — work that should have been done at authoring time.

### No Governance

Architectural decisions exist — often as ADRs, style guides, or team agreements —
but nothing enforces them at the spec level. A decision like "no shared databases
between services" or "all public APIs must have rate limiting" lives in a document
somewhere, but the spec authoring process has no mechanism to check new specs against
these constraints. Violations are caught in code review if you're lucky, or in
production if you're not.

**Example:** ADR-007 states that services must not share databases — each service
owns its data and exposes it through APIs. A new spec for a reporting feature
casually includes a cross-service database join for performance reasons. The spec
gets approved because the reviewer didn't remember ADR-007 (or didn't know it
existed). The implementation ships, creates a hidden coupling between two services,
and the team discovers the violation six months later during a migration that was
supposed to be isolated.

---

## The Cost

These gaps compound. A spec written without codebase awareness gets approved without
governance checks, handed to an agent that has no ground truth, which produces an
implementation that conflicts with existing patterns. The review catches some issues,
the author rewrites, another review, another rewrite. What should have been a
single-pass authoring flow becomes a multi-week rework cycle. Multiply this across
every spec in a growing project and you get teams that stop writing specs at all —
not because specs aren't valuable, but because the cost of maintaining bad ones
exceeds the cost of having none.

The agent-assisted case makes it worse. When agents make decisions without context,
they make them confidently and at scale. An agent that doesn't know your architecture
will produce internally consistent work that's externally wrong. The output looks
correct until a human with project context reviews it and realizes the agent built
the right thing on the wrong foundation. Rework at that point isn't a quick fix —
it's a redesign.

---

## The Opportunity

When specifications live in a graph instead of a folder, every gap closes. Live
queries replace manual tracing: "what's on the critical path?" is a graph traversal,
not a grep. Stable identities replace file paths: renaming or reorganizing specs
doesn't break references because the identity is independent of location. Structured
nodes replace prose: each spec carries machine-readable fields for dependencies,
acceptance criteria, and completion status that agents can consume directly.

A layered constitution anchors every authoring session in project ground truth — the
tech stack, architectural principles, and constraints that should shape every spec.
Codebase scanning grounds specs in what actually exists, not what the author imagines.
Governance becomes live enforcement: a spec that violates an architectural decision
is flagged before it's approved, not after it's implemented. The result is specs
that are correct by construction — authored with full context, validated against
constraints, and structured for both human review and agent execution.
