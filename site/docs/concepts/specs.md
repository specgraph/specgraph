# Specs & the Graph

## What is a Spec?

A spec is a **work unit** in the SpecGraph graph. Every spec has a stable,
content-addressable identity (e.g. `spec-k7m3p`), a human-readable slug
(e.g. `oauth-refresh-rotation`), and structured content that progresses through
the [authoring funnel](authoring.md). Specs are the fundamental building
block — everything else in SpecGraph exists to create, connect, validate, or
execute them.

---

## Progressive Structure

SpecGraph uses a single schema that scales from solo projects to enterprise
teams. You start with the fields you need and add structure as your project
demands it.

**Minimal spec** — a solo developer getting started:

```yaml
spec: login-api
intent: "REST endpoint for OAuth2 login"
verify:
  - "Returns JWT on valid credentials"
  - "Returns 401 on invalid credentials"
```

Three fields. No ceremony. This is enough for SpecGraph to track the work,
build dependency edges, and feed the spec to an executor.

**Full spec** — a team that needs priority, interface contracts, and invariants:

```yaml
spec: oauth-refresh-rotation
intent: "Implement automatic refresh token rotation per RFC 6749"
stage: approved
priority: p1
complexity: medium
interface:
  input: { endpoint: "POST /auth/token", body: { grant_type, refresh_token } }
  output: { success: { access_token, refresh_token, expires_in }, errors: [...] }
verify:
  - "Rotation returns new token pair"
  - "Old token rejected after rotation"
  - "Concurrent rotation: exactly one succeeds"
invariants:
  - "Never two valid refresh tokens for same session"
depends_on: [user-model]
```

Same schema, more fields. Enterprise teams can layer on governance metadata and
approval chains without changing the underlying data model. The schema grows
with the team, not against it.

---

## The Graph

Specs connect to each other via **first-class edges** stored in a graph
database. These are not fragile filename references or hand-maintained lists —
they are queryable, traversable relationships:

| Edge type | Meaning |
|---|---|
| `depends_on` | This spec requires another to be done first |
| `blocks` | This spec prevents another from starting |
| `composes` | This spec was decomposed from a parent spec |
| `references` | Links to decisions, ADRs, incidents, or other artifacts |

```text
┌──────────────────┐
│  auth-api        │
│  (approved)      │
└────────┬─────────┘
         │ depends_on
         ▼
┌──────────────────┐     blocks      ┌──────────────────┐
│  user-store      │ ──────────────▶ │  migration       │
│  (in-progress)   │                 │  (pending)       │
└────────┬─────────┘                 └──────────────────┘
         │ composes
         ▼
┌──────────────────┐
│  user-cache      │
│  (draft)         │
└──────────────────┘
```

Edges carry semantics. A `depends_on` edge tells the scheduler not to release a
spec until its dependency is complete. A `blocks` edge surfaces bottlenecks. A
`composes` edge traces how a large spec was broken into deliverable slices. A
`references` edge connects specs to the [decisions](decisions.md) that informed
them.

---

## Identity

Every spec has a **content-addressable identity** with the format
`spec-{short-hash}`. The hash is derived from the spec's content, which gives
you three properties:

1. **Merge-conflict-free** — two developers can create specs independently and
   merge without ID collisions. There are no sequential counters to fight over.
2. **Change detection** — if the content changes, the hash changes. You always
   know whether a spec has been modified since you last saw it.
3. **Distributed-safe** — no central authority assigns IDs. Teams across repos,
   time zones, or organizations produce globally unique identifiers by default.

The human-readable slug (`oauth-refresh-rotation`) exists for convenience in
conversation, CLI output, and documentation. The stable `spec-{hash}` is what
the graph stores and what edges reference.

---

## Core Schema

The full spec schema is organized into five categories:

| Category | Fields |
|---|---|
| **Identity** | `id`, `slug`, `version`, `created_at`, `updated_at` |
| **Intent** | `intent`, `stage` (spark / shape / specify / decompose / approved / in_progress / done), `priority` (p0-p3), `complexity` |
| **Edges** | `depends_on`, `blocks`, `composes`, `references` |
| **Authoring Outputs** | `spark_output`, `shape_output`, `specify_output`, `decompose_output` |
| **Verification** | `verify` (acceptance criteria), `invariants` (conditions that must hold before and after execution) |

Not every field is required. The minimal spec uses only `slug` (as the `spec`
key), `intent`, and `verify`. Additional fields appear as the spec moves through
the authoring funnel and as team needs grow.

---

## Why a Graph?

Traditional spec management stores specifications as files in a directory.
Relationships between specs are implicit — buried in prose references, filename
conventions, or external tracking tools. Answering questions like "what's the
critical path?" or "what does this spec impact?" requires writing bespoke grep
scripts or manually tracing links across documents.

In SpecGraph, those questions are first-class graph operations. "What's
blocked?" is a single-edge traversal. "What's the critical path?" is a
longest-path query weighted by complexity. "What does this spec impact?" is a
downstream walk from a node through its `blocks` and `depends_on` edges. The
graph makes structural queries cheap and reliable — you query the shape of
your project the same way you query its data.
