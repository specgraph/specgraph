# Decisions

## What are Decisions?

A decision is a **first-class node** in the spec graph. When you make a choice
during authoring — "use Postgres for token storage", "authenticate via OAuth2,
not API keys" — that choice becomes a graph node with a stable identity, a
lifecycle, and bidirectional edges to every spec it touches.

Decisions are not embedded arrays inside spec YAML. They are not separate ADR
files living in a `docs/adr/` folder. They are queryable nodes in the same graph
as your specs, connected by typed edges that let you answer "what breaks if we
change this?" with a single traversal.

---

## Why First-Class?

Embedding decisions inside specs (or managing them as standalone ADR files)
creates three problems that get worse as projects scale:

**No cross-referencing.** The `session-management` spec, `audit-logging` spec,
and `mobile-client` spec all care about the "refresh token storage" decision. If
that decision lives inside `login-api`'s YAML, the other three specs have no
way to reference it. You end up duplicating rationale, and duplicated rationale
drifts.

**Scaling under churn.** Complex specs accumulate dozens of decisions over their
lifetime — some current, some superseded, some deprecated when the context
changed. An embedded array doesn't surface what's current. You have to read every
entry and mentally reconstruct the history.

**Export requires extraction.** If you want to generate ADR documents (for
compliance, onboarding, or audit), embedded decisions must be extracted and
cross-referenced manually. With graph nodes, ADR export is a straightforward
traversal: walk the `decided_in` and `informs` edges, render each node.

Making decisions first-class solves all three: one authoritative node, referenced
from anywhere, with lifecycle state that tells you whether it's still active.

---

## Decision Schema

A decision node carries the question, the answer, and the alternatives that were
rejected:

```yaml
id: dec-a7f3b2c1
title: "Refresh token storage mechanism"
question: "Where and how to persist refresh tokens?"
chosen: "Postgres table with SHA-256 hashed tokens"
rationale: "Need revocation and audit trail. Postgres already in stack."
rejected:
  - option: "Redis"
    why: "Adds operational complexity for a non-performance-critical path"
  - option: "Stateless JWT"
    why: "Revocation requires a blocklist, negating the stateless benefit"
confidence: high
status: accepted
tags: [auth, storage, security]
origin_spec: login-api
```

The `rejected` list is not decoration. Six months from now, when someone asks
"why didn't we use Redis?", the answer is in the graph — not in a Slack thread
that has since been garbage-collected.

---

## Lifecycle

Decisions move through a simple state machine:

```text
proposed ──▶ accepted ──▶ deprecated
                    │
                    └──▶ superseded ──▶ (links to replacement)
```

| Status | Meaning |
|---|---|
| **proposed** | Captured during authoring, not yet finalized. The origin spec is still moving through the funnel. |
| **accepted** | Finalized when the origin spec is approved. This is the steady state for most decisions. |
| **deprecated** | No longer applies because the context changed. Carries a `reason` explaining why. |
| **superseded** | Replaced by a different decision. Carries a link to the replacement node. |

When a decision is superseded, every spec that references the old decision gets
flagged. You don't discover stale assumptions during implementation — you
discover them the moment the replacement is recorded.

---

## Edges

A decision connects to specs via two edge types:

| Edge | Direction | Meaning |
|---|---|---|
| `decided_in` | spec &rarr; decision | "This spec made this decision" (origin) |
| `informs` | decision &rarr; spec | "This decision informs this spec" (consumer) |

One decision, four specs, two relationship types:

```text
login-api       ──decided_in──▶  dec-a7f3b2c1
dec-a7f3b2c1    ──informs──▶     session-mgmt
dec-a7f3b2c1    ──informs──▶     audit-logging
dec-a7f3b2c1    ──informs──▶     mobile-client
```

Query: *"What specs are affected if we change the refresh token storage
decision?"* Walk the `decided_in` and `informs` edges from
`dec-a7f3b2c1` — instant answer: four specs, one origin, three consumers.

---

## Example: Superseding a Decision

Your team originally decided to store session data in Redis. Three specs
reference this decision:

```text
session-mgmt       ──decided_in──▶  dec-session-redis
dec-session-redis   ──informs──▶    api-gateway
dec-session-redis   ──informs──▶    rate-limiter
```

Six months later, operational costs push you to move session storage into
Postgres. You create a new decision:

```yaml
id: dec-session-pg
title: "Session storage migration to Postgres"
question: "Where to store session data going forward?"
chosen: "Postgres with connection pooling via PgBouncer"
rationale: "Eliminate Redis operational overhead. Session reads are not latency-sensitive."
rejected:
  - option: "Keep Redis"
    why: "Operational cost of maintaining a separate data store for non-critical reads"
confidence: high
status: accepted
tags: [sessions, storage, infrastructure]
origin_spec: session-mgmt
```

The old decision transitions to `superseded` with a link to `dec-session-pg`.
At that moment, SpecGraph flags `api-gateway` and `rate-limiter` as referencing
a superseded decision. No one has to remember which specs cared about Redis
session storage — the graph already knows.
