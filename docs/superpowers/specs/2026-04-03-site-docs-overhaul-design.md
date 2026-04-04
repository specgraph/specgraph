# Site Documentation Overhaul

## Problem

The SpecGraph documentation site is stale. The Memgraph-to-Postgres migration,
new features (auth, findings, steel_thread decomposition, export/import), and
version progression to v0.4.0 are not reflected. Several pages contain factual
inaccuracies, missing features, and terminology that implies a graph database
backend that no longer exists.

## Design Decisions

**Storage terminology:** Drop "graph database" entirely from conceptual docs.
Use "queryable graph" or just "graph" to describe the model. The architecture
page explains that PostgreSQL is the backend — readers who care about
implementation details find it there. The product value is queryable
relationships, not any particular database technology.

**Current vs Planned:** Use consistent `!!! info "Planned"` admonitions wherever
a feature is documented but not yet shipped. Remove all version-pinned callouts
(no "v0.1.0", no "planned for 0.2.0"). Just state what exists and what doesn't.

**No date references:** Avoid tying documentation to specific release versions
or dates. Describe capabilities as shipped or planned.

**New content strategy:** Add sections to existing pages rather than creating
new standalone pages. Keeps nav concise and avoids orphan pages.

## Scope

### Section 1: Terminology & Accuracy Sweep (all pages)

Every page:

- "graph database" → "queryable graph" or "graph"
- Remove all Memgraph references; replace with PostgreSQL where backend is named
- Remove all Cypher references; architecture page describes SQL with recursive CTEs
- Remove version-pinned statements ("v0.1.0 is the first public release", etc.)
- Fix time estimate: use "under ten minutes" in both index.md and quickstart.md

### Section 2: Factual Corrections (per-page)

**architecture.md:**

- Rewrite Storage section: PostgreSQL is the shipped backend, pluggable interface
- Update code organization tree: add `export/`, `notify/`, `render/`; remove `memgraph/` (`auth/` already present)
- Fix Backend interface example: show composed interfaces (SpecBackend, ConstitutionBackend, GraphBackend, etc.), not monolithic
- Update system diagram: remove Memgraph mention from storage line

**quickstart.md:**

- Prerequisites: Docker (PostgreSQL container), not Memgraph
- `specgraph up` starts PostgreSQL, not Memgraph
- Use "under ten minutes" to match index.md

**how-it-works.md:**

- Pipeline diagram: remove "Cypher over Memgraph (Postgres planned)" line
- Update storage description in diagram

**authoring.md:**

- Fix post-approval change mechanisms (line 165 says specs "must be superseded"
  after approval — wrong). Corrected text: "If a spec needs to change after
  reaching done, it can be **amended** (returning to an earlier authoring stage
  for modification) or **superseded** by a new spec that replaces it.
  Amendment is for refining an existing spec; supersession is for replacing it
  with a fundamentally different approach."
- Add `steel_thread` decomposition strategy alongside vertical_slice, layer_cake,
  single_unit. Definition: "A **steel thread** cuts the thinnest possible
  vertical slice that proves the riskiest integration points first. The first
  slice (`slices[0]`) is the thread itself with no dependencies; all subsequent
  slices are reachable from it. Use when the primary risk is integration
  uncertainty rather than feature breadth." (Source: proto comment on
  `DECOMPOSITION_STRATEGY_STEEL_THREAD`)

**constitution.md:**

- Add `!!! info "Planned"` admonition for multi-layer composition and provenance trails
- Current state: single constitution per project, layer field stored but
  composition not implemented
- Fix "gRPC" in example constitution to "ConnectRPC" for consistency

**specs.md:**

- Fix informal stage names in diagram: `(draft)` and `(pending)` are not real
  stages — use actual stages (spark, shape, etc.)
- Add mention of `specgraph changes` command in the Change Tracking section
- Remove "queryable in Cypher" from ChangeLog description

**passes.md:**

- Replace version-pinned "0.1.0 Implementation Status" admonition with
  version-agnostic "Planned" callout describing what's shipped vs not

**drift.md:**

- Add `!!! info "Planned"` for interface and verify drift scopes (only deps is
  shipped)

**ecosystem.md:**

- Replace "Memgraph backend" reference with PostgreSQL
- Ecosystem integration status (ground truth for callout verification):
  - **Gastown**: Planned, not built. SpecGraph functions independently without it.
  - **Beads sync**: Shipped, push-only. Adapter pushes specs as beads issues.
  - **GitHub sync**: Shipped, push-only. Adapter pushes specs as GitHub issues.
  - **Linear sync**: Planned, not started.
  - **MCP server**: Planned, not started.
  - **Tool injection**: Shipped (`specgraph inject`).

**index.md:**

- Remove "v0.1.0 is the first public release"
- Link to changelog without version-specific language

### Section 3: Current vs Planned Admonitions

Consistent pattern:

```markdown
!!! info "Planned"
    Description of what's designed but not yet shipped, and what currently
    works instead.
```

Pages needing callouts:

- constitution.md — layer merging, provenance trails
- drift.md — interface and verify scopes
- ecosystem.md — verify existing callouts are accurate
- passes.md — replace version-pinned callout

### Section 4: Missing Content (add to existing pages)

**architecture.md — add Authentication section:**

- OIDC multi-provider auth
- Cookie-based dashboard sessions
- Auth interceptor

**passes.md — add Findings section:**

- Add a "Working with Findings" section at the end of passes.md explaining how
  `specgraph findings list <slug>` surfaces findings from analytical passes,
  with `--pass-type` filter for specific pass types

**guides — add Export/Import/Verify workflow:**

- Backup and restore via `specgraph export`, `specgraph import`, `specgraph verify`
- Add to cli-cookbook.md as a new recipe

**authoring.md — mention conversation recording:**

- `specgraph conversation record` and `specgraph conversation list`
- Brief mention in context of authoring funnel

### Section 5: Replace ASCII Diagrams with Mermaid

Convert all ASCII art diagrams to mermaid fenced blocks (```` ```mermaid ````).
May require enabling mermaid support in Zensical config (`zensical.toml`).

**New diagram — spec lifecycle state machine (authoring.md):**

Add a mermaid `stateDiagram-v2` to the authoring page showing the full spec
lifecycle. Include:

- Forward progression: spark→shape→specify→decompose→approved→in_progress→review→done
- Amendment re-entry: done→amended, then amended connects to a choice state
  that fans out to each valid re-entry stage (spark, shape, specify, decompose,
  approved, in_progress, review). Use a choice/fork node to keep the diagram
  readable rather than drawing 7 individual arrows from amended.
- Supersession: any non-terminal stage→superseded (use a note or grouped
  transition rather than drawing an arrow from every stage)
- Abandonment: any non-terminal stage→abandoned (same treatment as supersession)
- Terminal states: superseded and abandoned are `[*]` endpoints; amended is
  semi-terminal (can be superseded/abandoned but not re-amended)

Use `internal/storage/stage_validation.go` and `internal/storage/spec_domain.go`
as the source of truth for valid transitions.

**Convert existing ASCII diagrams to mermaid:**

| File | Diagram | Mermaid Type |
|------|---------|-------------|
| `decisions.md:75` | Decision state machine (proposed→accepted→deprecated/superseded) | `stateDiagram-v2` |
| `decisions.md:105` | Edge relationships (login-api→dec→session-mgmt etc.) | `graph LR` |
| `decisions.md:123` | Edge relationships (session-mgmt→dec-session-redis→api-gateway) | `graph LR` |
| `specs.md:74` | Dependency graph (auth-api→user-store→user-cache, blocks) | `graph TD` |
| `drift.md:23` | Edge with content_hash_at_link property | `graph LR` |
| `architecture.md:12` | System diagram (clients→server→sync adapters) | `graph TD` |
| `architecture.md:96` | Edge type listing (node relationships) | `graph LR` |
| `architecture.md:124` | Decision promotion — Shape stage | `graph LR` |
| `architecture.md:141` | Decision promotion — Approve stage | `graph LR` |
| `slices.md:16` | Slice→Spec COMPOSES edge | `graph LR` |
| `slices.md:23` | Spec→Spec vs Slice→Spec COMPOSES | `graph LR` |
| `slices.md:49` | Slice state machine (open→claimed→done) | `stateDiagram-v2` |
| `how-it-works.md:107` | Full SpecGraph pipeline | `graph TD` |
| `constitution.md:29` | Layer hierarchy (User→Org→Project→Domain) | `graph BT` |

**Keep as text (not diagrams):**

- `architecture.md:162` — file tree (code block, not a diagram)
- `slices.md:125` — CLI output example
- `passes.md:155` — safety net pattern list
- `example-spec.md:152` — API contract definition

### Section 6: Consistency & Polish

- **Slice slug format:** Verify what decompose actually generates (check proto
  and CLI) and use that format consistently in cli-cookbook.md examples
- **Link verification:** Check all external links:
  - Spec Kit: https://github.com/github/spec-kit
  - GSD: https://github.com/gsd-build/get-shit-done
  - Faros AI blog post
  - METR study link
  - CodeRabbit report link
  - RFC references
  - Beads project link
- **Constitution example:** "gRPC" → "ConnectRPC"

## Files Modified

All files under `site/docs/`:

| File | Changes |
|------|---------|
| `index.md` | Remove version reference, terminology |
| `problem.md` | Verify external links |
| `quickstart.md` | Memgraph→Postgres, time estimate, prerequisites |
| `how-it-works.md` | Diagram update, terminology, mermaid |
| `concepts/index.md` | Terminology if needed |
| `concepts/specs.md` | Stage names in diagram, Cypher reference, changes command, mermaid |
| `concepts/constitution.md` | Planned admonition, gRPC→ConnectRPC, mermaid |
| `concepts/authoring.md` | Amendment, steel_thread, conversation recording |
| `concepts/decisions.md` | Mermaid (state machine + edge diagrams) |
| `concepts/passes.md` | Replace version-pinned callout, findings section |
| `concepts/slices.md` | Mermaid (state machine + edge diagrams) |
| `concepts/drift.md` | Planned admonition for interface/verify, mermaid |
| `concepts/linting.md` | Review for accuracy (likely no changes) |
| `concepts/example-spec.md` | Review for accuracy (likely no changes) |
| `guides/cli-cookbook.md` | Slice slug format, add export/import recipe |
| `guides/sync.md` | Review for accuracy |
| `architecture.md` | Storage rewrite, code org tree, auth section, interface example, mermaid |
| `ecosystem.md` | Memgraph→Postgres, verify callouts |
| `cli-reference.md` | Auto-generated; review only |

## Out of Scope

- Redesigning the site theme or navigation structure
- Adding entirely new concept pages (prefer sections in existing pages)
- Rewriting content that is accurate and well-written
- Updating the changelog (auto-generated)
- CLI reference updates (auto-generated by `specgraph docs cli`)
