# SpecGraph ADR-003: Decisions as First-Class Graph Entities

## Status

Proposed

## Context

Decisions are currently embedded as a JSONB array inside each spec:

```yaml
decisions:
  - question: "Refresh token storage"
    chosen: "Postgres table"
    rationale: "Need revocation + audit. Already have Postgres."
    rejected:
      - { option: "Redis", why: "Adds ops complexity" }
      - { option: "JWT", why: "Revocation requires blocklist" }
    confidence: high
```

This creates three problems:

1. **No cross-referencing.** The session-management spec, audit-logging spec, and mobile-client spec all care about the "refresh token storage" decision made in the login-api spec. Currently they have no way to reference it — they either duplicate the decision, hand-wave at it, or ignore it entirely.

2. **Scaling under churn.** A complex spec in an active codebase can accumulate dozens of decisions. Some get amended, some get superseded by later decisions in other specs. Embedded arrays don't surface this — you have to read through every spec to find out what's current.

3. **ADR export is a transform, not a projection.** Because decisions are buried inside specs, ADR generation requires extracting, numbering, cross-referencing, and tracking supersession across specs. If decisions were already nodes with edges, export would be a straightforward traversal.

## Decision

**Decisions become first-class nodes in the spec graph** with their own identity, lifecycle, and bidirectional edges to specs.

### Decision Identity

Decision identity follows the same backend-native pattern as specs:

**Beads path:** The decision IS a bead. Its identity is the bead ID — content-addressed, conflict-handled by Beads/Dolt. No additional scheme needed.

**Postgres path:** Short hash from the title — `d-` prefix + first 8 hex chars of `sha256(normalize(title))`. Collisions (same title = same decision) are intentional — they surface duplicates for merging rather than letting them proliferate silently.

Both paths give you the same property: two specs that independently decide "refresh token storage" converge on the same identity. The CLI resolves by title so humans never type IDs:

```bash
specgraph decision show "refresh token storage"
specgraph decision reference mobile-client "refresh token storage"
```

### Decision Schema

```yaml
# ── Identity ────────────────────────────────────────
id: d-a7f3b2c1                              # Postgres: short hash; Beads: bead ID
title: "Refresh token storage mechanism"    # human-readable, source of identity
question: "Where and how to persist refresh tokens?"

# ── Resolution ──────────────────────────────────────
chosen: "Postgres table with SHA-256 hashed tokens"
rationale: "Need revocation and audit trail. Postgres already in stack."
rejected:
  - option: "Redis"
    why: "Adds operational complexity for a non-performance-critical path"
  - option: "Stateless JWT"
    why: "Revocation requires a blocklist, negating the stateless benefit"
confidence: high                            # high | medium | low

# ── Context ─────────────────────────────────────────
tags: [auth, storage, security]             # searchable
scope: project                              # project | team | org
                                            # how broadly this decision applies

# ── Provenance ──────────────────────────────────────
origin_spec: login-api                      # spec where this decision was made
origin_stage: specify                       # which authoring stage
date: 2025-02-20

# ── Lifecycle ───────────────────────────────────────
status: accepted                            # proposed | accepted | deprecated | superseded
supersedes: null                            # id of decision this replaces
superseded_by: null                         # set when replaced
deprecated_reason: null                     # why deprecated (if applicable)

# ── History ─────────────────────────────────────────
history:
  - version: 1
    summary: "Initial decision during login-api specify"
    date: 2025-02-20
  - version: 2
    summary: "Added per-token salt after red-team finding"
    date: 2025-02-22
```

### Edge Types

Decisions connect to specs through two directed edge types:

```
spec ──decided_in──▶ decision       # "this spec made this decision"
spec ──references──▶ decision       # "this spec depends on this decision"
```

The reverse traversals are implicit:

```
decision.origin_spec                # which spec authored it
decision.referencing_specs          # which specs depend on it
```

**Examples:**

```
login-api       ──decided_in──▶  d-a7f3b2c1
session-mgmt    ──references──▶  d-a7f3b2c1
audit-logging   ──references──▶  d-a7f3b2c1
mobile-client   ──references──▶  d-a7f3b2c1
```

Now when the refresh token storage decision changes, every spec that references it can be notified — same mechanism as `spec.interface_changed` but for decisions.

### Lifecycle

Decisions have a simpler lifecycle than specs:

```
proposed → accepted → (deprecated | superseded)
```

- **proposed** — Decision captured during authoring but not yet finalized (e.g., still in shape/specify stage).
- **accepted** — Decision finalized. Specs can reference it.
- **deprecated** — Decision no longer applies (context changed, requirement removed). Carries a reason. Not replaced by another decision — just retired.
- **superseded** — Replaced by a different decision. Carries `superseded_by` link. Referencing specs get notified.

Decisions move to `accepted` when their origin spec is approved. If the origin spec is abandoned, its proposed decisions are also abandoned (or orphaned for manual review).

### Decision Supersession

When a new decision supersedes an old one:

```bash
specgraph decision supersede "refresh token storage" \
  --with "refresh token storage v2"
```

1. Old decision: status → `superseded`, `superseded_by` → new id
2. New decision: `supersedes` → old id
3. All specs with `references` edges to the old decision get flagged: "Referenced decision has been superseded. Update reference or acknowledge."
4. ADR exports for the old decision get status → "Superseded by [new decision]"

This is the same pattern as spec supersession — because it is the same graph operation.

### Cross-Spec Referencing

During authoring, when the agent encounters a decision that already exists in the graph:

```
Agent: "For token storage, I see there's an existing decision 
        ('Refresh token storage mechanism') that chose Postgres. 
        Should this spec reference that decision, or do we need 
        a different approach here?"

User: "Reference the existing one."

Agent: specgraph decision reference mobile-client "refresh token storage"
```

During drift detection, referenced decisions are checked:
- Is the referenced decision still `accepted`? (Not deprecated/superseded)
- Has the decision been amended since this spec last acknowledged it?
- Does the spec's implementation still align with the decision?

### What Stays in the Spec

The spec's `decisions` array becomes a **reference list**, not the decision content itself:

```yaml
# Before (embedded):
decisions:
  - question: "Refresh token storage"
    chosen: "Postgres table"
    rationale: "Need revocation + audit"
    rejected:
      - { option: "Redis", why: "Adds ops complexity" }
    confidence: high

# After (references):
decisions:
  decided:
    - d-a7f3b2c1     # decisions made IN this spec
    - d-c3d9e5f2
  references:
    - d-e1a4b7c8     # decisions made elsewhere, used here
    - d-f6b2a9d3
```

The spec still knows which decisions it cares about. The decision content lives in the decision node. No duplication.

### Storage

**Beads path:** Decisions are a new bead type. Edges stored as Dolt relations. Natural fit — decisions are content-addressed artifacts with version history.

**Postgres path:** New `decisions` table alongside `specs`. Edge tables for `decided_in` and `references_decision`. Same pattern as spec dependency edges.

```sql
CREATE TABLE decisions (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    question    TEXT NOT NULL,
    chosen      TEXT NOT NULL,
    rationale   TEXT,
    rejected    JSONB,
    confidence  TEXT CHECK (confidence IN ('high','medium','low')),
    tags        TEXT[],
    scope       TEXT DEFAULT 'project',
    origin_spec TEXT REFERENCES specs(slug),
    origin_stage TEXT,
    status      TEXT DEFAULT 'proposed'
                CHECK (status IN ('proposed','accepted','deprecated','superseded')),
    supersedes      TEXT REFERENCES decisions(id),
    superseded_by   TEXT REFERENCES decisions(id),
    deprecated_reason TEXT,
    history     JSONB,
    version     INTEGER DEFAULT 1,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE spec_decisions (
    spec_slug     TEXT REFERENCES specs(slug),
    decision_id   TEXT REFERENCES decisions(id),
    edge_type     TEXT CHECK (edge_type IN ('decided_in', 'references')),
    PRIMARY KEY (spec_slug, decision_id, edge_type)
);
```

### CLI

```bash
# Creating (usually happens during authoring, but can be manual)
specgraph decision create --question "..." --chosen "..." --origin login-api

# Referencing from another spec
specgraph decision reference mobile-client "refresh token storage"

# Listing
specgraph decision list                          # all decisions
specgraph decision list --spec login-api         # decisions for a spec
specgraph decision list --tag auth               # by tag
specgraph decision list --status accepted        # by status
specgraph decision show "refresh token storage"

# Lifecycle
specgraph decision accept "refresh token storage"
specgraph decision deprecate "refresh token storage" --reason "..."
specgraph decision supersede "refresh token storage" --with "refresh token storage v2"

# Impact analysis
specgraph decision impact "refresh token storage"
# → Referenced by: session-mgmt, audit-logging, mobile-client
# → Origin spec: login-api (status: done)
```

### ADR Export

Becomes trivial. Each accepted decision IS an ADR:

```bash
specgraph export adr                             # all accepted decisions → ADRs
specgraph export adr --spec login-api            # decisions from one spec
specgraph export adr "refresh token storage"     # single decision
```

No extraction logic needed. The decision schema already contains title, context (from origin spec's intent), chosen option, rationale, rejected alternatives, and supersession chain. Format into Nygard template and done.

### Authoring Flow Impact

The authoring funnel changes slightly:

**Shape stage:** Agent identifies decisions to be made. Creates decision nodes with status `proposed`.

**Specify stage:** Agent resolves proposed decisions — fills in chosen option, rationale, rejected alternatives. Decisions stay `proposed` until spec approved.

**Approve stage:** When spec is approved, its `proposed` decisions move to `accepted`.

**Amendment:** When a spec is amended, its decisions may need revision. Amended decisions either get updated in place (minor change, version bump) or superseded (fundamentally different choice).

The skills (`/specgraph-shape`, `/specgraph-specify`) are updated to work with decision nodes rather than inline arrays. The agent creates, resolves, and links decisions as part of the normal flow.

### Constitution Relationship

Constitution entries are NOT decisions in this model. The constitution captures standing constraints, values, and standards ("We use Go", "All APIs backward compatible"). Decisions are specific choices made in the context of a spec ("Use Postgres for token storage"). 

However, a decision can *reference* a constitution entry as justification: "Chose Postgres because the constitution says 'minimize infrastructure dependencies' and we already run Postgres." This is an informational link, not an edge type — captured in the decision's `rationale` field.

If a constitution entry was *itself* the result of a decision (which it often is), that decision should exist as a decision node with `scope: project` or `scope: org`, and the constitution entry can reference it. This gives you a provenance chain: why does the constitution say "no ORMs"? Because of the "ORM prohibition" decision, made in spec architecture-principles, which rejected ORMs for reasons X, Y, Z.

## Consequences

**Positive:**
- Decisions become referenceable across specs — no duplication
- Impact analysis: "which specs are affected if we change this decision?"
- ADR export becomes a projection, not a transformation
- Supersession chain for decisions is explicit and traversable
- Drift detection can check decision currency (is this decision still accepted?)
- Decision discovery: agents can search existing decisions before proposing new ones

**Negative:**
- Additional entity type in the graph (more schema, more storage, more CLI surface)
- Authoring flow slightly more complex (create decision nodes, not just fill in arrays)

**Neutral:**
- Red-team findings stay embedded in specs. They're observations about a specific spec, not reusable decisions. A finding might *lead* to a decision, but the finding itself is spec-local.
- The `references` field on specs (informational links like `arch-decision-017`) is superseded by the decision graph for decision-related references. Plain-text references to external content (incidents, RFCs, etc.) remain as-is.


