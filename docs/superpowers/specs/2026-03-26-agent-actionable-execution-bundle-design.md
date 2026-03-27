# Agent-Actionable Execution Bundle

**Bead:** spgr-755
**Date:** 2026-03-26
**Status:** Draft

## Problem

The current execution bundle (`GenerateBundle` RPC + `renderBundleYAML`) produces a
YAML stub that is insufficient for an agent to act on. It includes only: version,
spec slug/intent/stage, decision slug/title/status, and callback endpoint. An agent
receiving this bundle has no understanding of what to build, how to verify its work,
or how to communicate status back to the server.

Meanwhile, the storage layer already fetches the full Spec (with all authoring
outputs) and full Decision objects — the handler's renderer simply throws most of
it away.

## Design Principle

The bundle is a **launchpad** — enough inline context to orient the agent and start
working, plus pointers for fresh project-wide data. Not a full data dump, not so
thin it requires a dedicated skill to interpret.

- **Inline:** spec-specific data that is frozen at approval time (authoring outputs,
  decisions, dependencies)
- **Pointer:** project-wide data that can change independently (constitution,
  coding conventions) — available via `specgraph prime <slug>`

## Decisions

### Format: Markdown with YAML Frontmatter

**Decision:** The bundle output changes from YAML to Markdown with YAML frontmatter.

**Rationale:** The primary consumer is an LLM agent. LLMs read markdown natively.
The bundle also drops directly into bead files (which are markdown) with zero
transformation. YAML frontmatter preserves machine-readable metadata (version, slug,
content hash) for tooling. Conditional section omission maps naturally to markdown
headers — no empty sections, no schema interpretation needed.

### Full Authoring Outputs Inline

**Decision:** Include complete SpecifyOutput (acceptance criteria, interfaces,
invariants, file touches) and DecomposeOutput (strategy, slices with verify/touches/
depends_on) inline in the bundle.

**Rationale:** Approved specs are frozen — there is no stale data risk. The data is
small (50-100 lines typically). Requiring a server roundtrip via GetPrime for the
agent to understand *what to build* is unnecessary friction, especially when the
bundle may be consumed offline (pasted into a session, stored as a bead file).

### Full Decisions Inline

**Decision:** Include decision body and rationale, not just slug/title/status.

**Rationale:** Decisions linked to a spec are spec-specific and few (2-5 typically).
The rationale is often the most important part for an implementer — "use Memgraph"
is less useful than "use Memgraph because we need sub-millisecond traversals."

### Constitution via Pointer Only

**Decision:** The bundle includes a pointer to `specgraph prime <slug>` for
constitution context, not the constitution inline.

**Rationale:** Constitution is project-wide (same for every spec), can change
independently of spec approval, and would be duplicated across every bundle. This
is the one piece where "go ask for fresh data" makes sense.

### Claim Instructions Always Present

**Decision:** The bundle always includes claim instructions (how to claim, report
progress, report blockers, report completion). If the spec is currently claimed,
that state is also shown.

**Rationale:** The bundle is a launchpad — it should tell the agent how to
participate in the execution protocol. The agent may not have a skill or prior
knowledge of the claim/report commands.

### Dependencies with Drift State

**Decision:** Include upstream dependencies with slug, status, and drift flag
(content hash mismatch detection).

**Rationale:** An agent needs to know whether its assumptions are still valid.
Dependency drift (upstream changed since baseline) is a concrete signal that the
agent should investigate before proceeding. The drift machinery already exists on
DEPENDS_ON edges.

## Bundle Structure

### YAML Frontmatter

```yaml
---
version: 2
slug: <spec-slug>
stage: <raw SpecStage string, e.g. "approved", "in_progress", "review", "done">
priority: <raw SpecPriority string: "p0", "p1", "p2", "p3">
content_hash: <spec's existing ContentHash field — Murmur3-128 hex, 32 chars>
generated_at: <RFC3339 timestamp>
---
```

The `content_hash` is the Spec node's existing `ContentHash` field (computed from
substantive fields by `recomputeContentHash`), not a hash of the bundle itself.

Both `stage` and `priority` emit the raw domain type string values. The frontmatter
is for machine consumption; human-readable labels belong in the markdown body.

### Section: What to Build

Source: `Spec.Intent`, `Spec.ShapeOutput` (scope), `Spec.SpecifyOutput`

Field mapping:

- `SpecifyOutput.VerifyCriteria` → rendered as "Acceptance Criteria" checkboxes
- `SpecifyOutput.Interfaces` → rendered as "Interfaces" subsection
- `SpecifyOutput.Invariants` → rendered as "Invariants" list
- `SpecifyOutput.Touches` → rendered as "File Touches" table

````markdown
## What to Build

**Intent:** <spec intent>

### Scope

- **In:** <scope_in items>
- **Out:** <scope_out items>

### Acceptance Criteria

- [ ] <category>: <description>
- [ ] <category>: <description>

### Invariants

- <invariant>
- <invariant>

### Interfaces

**<interface name>**
<interface body>

### File Touches

| Path | Purpose | Change |
|------|---------|--------|
| `<path>` | <purpose> | <change_type> |
````

Scope comes from ShapeOutput (`scope_in`, `scope_out`). All other fields from
SpecifyOutput. Each subsection is omitted when its data is empty.

**Intentionally omitted ShapeOutput fields:** `SuccessMust`/`SuccessShould`/
`SuccessWont` overlap with SpecifyOutput's `VerifyCriteria` (which are more precise
and testable). `Approaches` with tradeoffs are design rationale — the chosen
approach and risks are surfaced in "Design Context" instead.

### Section: Work Slices

Source: `Spec.DecomposeOutput`

````markdown
## Work Slices

Strategy: `<vertical_slice|layer_cake|single_unit>`

### Slice 1: <slice intent>

- **Verify:** <verify item>
- **Touches:** `<file path>`
- **Depends on:** <slice IDs or "none">
````

Entire section omitted when `DecomposeOutput` is nil — i.e., the spec has not gone
through the decompose stage at all. A `single_unit` strategy with one slice still
renders (the slice intent and verify criteria are useful context even for
single-unit work).

### Section: How to Work

Source: Claim state, DEPENDS_ON edges, CallbackConfig

````markdown
## How to Work

### Claim & Report

```text
specgraph claim <slug> --agent <your-id>
specgraph report-progress <slug> --agent <your-id> --message "..."
specgraph report-blocker <slug> --agent <your-id> --description "..."
specgraph report-completion <slug> --agent <your-id>
```

All flags shown above are required for their respective commands.

**Current claim:** unclaimed
<!-- or: **Current claim:** agent-7 (expires 2026-03-26T15:00:00Z) -->

### Dependencies

| Upstream | Status | Drifted |
|----------|--------|---------|
| <slug> | <stage> | no |
| <slug> | <stage> | **yes** — content changed since baseline |

### Constitution & Project Context

Run `specgraph prime <slug>` for constitution constraints, coding conventions,
and project context.
````

Note the hyphenated command names (`report-progress`, `report-blocker`,
`report-completion`) matching the actual CLI subcommands in
`cmd/specgraph/report.go`.

Dependency table omitted when no DEPENDS_ON edges exist. Claim section is always
present.

### Section: Decisions

Source: linked Decision nodes via DECIDED_IN edges

````markdown
## Decisions

### <decision slug>: <title>

**Status:** <display string: "accepted", "proposed", "deprecated", "superseded">
**Decision:** <body>
**Rationale:** <rationale>
````

Entire section omitted when no decisions are linked. The renderer converts
`DecisionStatus` domain values (e.g., `DECISION_STATUS_ACCEPTED`) to lowercase
display strings by stripping the `DECISION_STATUS_` prefix and lowercasing the
remainder.

### Section: Design Context

Source: `Spec.ShapeOutput`

````markdown
## Design Context

**Chosen approach:** <chosen_approach>
**Risks:**

- <risk>
- <risk>
````

Entire section omitted when ShapeOutput is nil. This is reference material (design
rationale), not primary instructions — placed last intentionally.

## Storage Layer Changes

### Domain Types

**`storage.Bundle`** — add two fields:

```go
type Bundle struct {
    Version      int32
    Spec         *Spec               // already has all authoring outputs
    Decisions    []*Decision         // already fetched
    Bootstrap    string              // retained, unused by markdown renderer
    Callbacks    *CallbackConfig     // retained for programmatic consumers, not rendered
    Claim        *Claim              // NEW: nil if unclaimed
    Dependencies []DependencyInfo    // NEW
}

// DependencyInfo captures an upstream spec's status and drift state for the bundle.
// This is a new type (not DependencyRef) because the bundle needs a pre-computed
// boolean drift flag and human-readable note, while DependencyRef carries raw
// ContentHashAtLink for programmatic drift detection. The conversion happens in
// GenerateBundle, which calls GetDependenciesWithEdgeData and computes drift.
type DependencyInfo struct {
    Slug    string
    Stage   SpecStage
    Drifted bool
    Note    string // human-readable drift explanation, empty when not drifted
}
```

### `GenerateBundle` Storage Method

Current implementation (`memgraph/execution.go`) already calls `GetSpec` (full
authoring outputs) and `fetchLinkedDecisions`. Add two operations:

1. **Fetch claim state** — `OPTIONAL MATCH (s)-[r:CLAIMED_BY]->(a)` to get the
   current claim (or nil). Single additional Cypher clause, can be added to the
   existing spec query or run as a second query.

2. **Fetch dependencies with drift** — reuse the existing
   `GetDependenciesWithEdgeData` method (`internal/storage/memgraph/graph.go:165`),
   which already uses project-scoped queries and returns `[]DependencyRef` with
   `ContentHashAtLink`. The method needs one addition: it must also return the
   upstream node's current `content_hash` so the caller can compute drift. Extend
   the Cypher RETURN clause:

   ```cypher
   MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[dep:DEPENDS_ON]->(n)
   RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label,
          COALESCE(n.stage, n.status, "") AS stage,
          COALESCE(dep.content_hash_at_link, "") AS content_hash_at_link,
          COALESCE(n.content_hash, "") AS upstream_content_hash
   ```

   Add `UpstreamContentHash string` to `DependencyRef`. `GenerateBundle` then
   computes drift: `drifted = ref.ContentHashAtLink != ref.UpstreamContentHash ||
   ref.ContentHashAtLink == ""` (empty hash = unmigrated edge, always drifted).

   The conversion from `[]DependencyRef` → `[]DependencyInfo` happens in
   `GenerateBundle`, keeping the renderer free of drift logic.

   **Drift engine unchanged:** The existing drift detection engine
   (`internal/drift/`) fetches upstream specs separately via `GetSpec` to compare
   hashes. Adding `UpstreamContentHash` to `DependencyRef` is a zero-value-safe
   addition that doesn't affect the drift engine's existing callers. The drift
   engine may optionally be updated to use this field in a future change, but that
   is out of scope here.

**`Bootstrap` and `CallbackConfig` fields:** Both are retained on the `Bundle`
struct for backward compatibility with programmatic consumers. `Bootstrap` was
always a placeholder (never populated by the storage layer). `CallbackConfig` is
still populated by the handler when the caller provides an `--endpoint` flag, but
the markdown renderer does not use it — the "How to Work" section uses CLI commands
instead of raw endpoint URLs, which is more useful for the agent consumer.

No new storage interfaces — `GenerateBundle` already returns `*Bundle`; it just
populates the new fields.

## Handler Changes

### Proto

In `proto/specgraph/v1/execution.proto`:

```protobuf
message Bundle {
  int32 version = 1;
  Spec spec = 2;
  repeated Decision decisions = 3;
  string bootstrap = 4;
  CallbackConfig callbacks = 5;
  reserved 6;
  reserved "bundle_yaml";
  string bundle_content = 7;
}
```

Field 6 (`bundle_yaml`) is reserved to avoid wire compatibility breakage. New
field 7 (`bundle_content`) holds the markdown output.

**Web frontend transition:** The generated TypeScript client
(`web/src/lib/api/gen/specgraph/v1/execution_pb.ts`) currently references
`bundleYaml`. After proto regeneration, this field disappears and `bundleContent`
appears. The web frontend does not currently render bundle content (the dashboard
focuses on spec listing and graph visualization), so this is a non-breaking change
at the UI level. If bundle rendering is added to the web UI later, it reads
`bundleContent`. During the transition, `task proto` regenerates both Go and
TypeScript, and `task check` will surface any broken references.

### Renderer

Replace `renderBundleYAML` with `renderBundleMarkdown` in
`internal/server/execution_handler.go`.

Implementation uses `text/template` (not `html/template` — no HTML escaping needed
for markdown) with a single embedded template string. Template helper functions
handle conditional section rendering.

If the template + helpers grow beyond ~150 lines, extract to
`execution_render.go` in the same package.

### Handler Method

`GenerateBundle` RPC method unchanged except:

- Calls `renderBundleMarkdown` instead of `renderBundleYAML`
- Sets `pb.BundleContent` instead of `pb.BundleYaml`

## CLI Changes

`cmd/specgraph/bundle.go`:

- Read `resp.Msg.GetBundle().GetBundleContent()` instead of `GetBundleYaml()`
- `--output` flag writes `.md` file instead of `.yaml`
- No other CLI changes needed

## Testing

### Unit Tests (execution_handler_test.go)

Test `renderBundleMarkdown` directly with constructed `*storage.Bundle` inputs:

| Case | Assertion |
|------|-----------|
| Full bundle (all sections) | All section headers present, acceptance criteria as checkboxes, decision body/rationale present |
| Minimal (spec + intent only) | No "Work Slices", no "Decisions", no "Design Context" headers |
| No decisions | No "## Decisions" header |
| No decompose output (nil) | No "## Work Slices" header |
| single_unit with one slice | "## Work Slices" header present with slice details |
| Unclaimed | "Current claim: unclaimed" |
| Claimed | Agent name and expiry in claim section |
| Dependencies with drift | "**yes**" in drift column |
| Dependencies without drift | "no" in drift column |
| No dependencies | No dependency table |

Assert structural properties (section presence, key content), not exact string
snapshots.

### Integration Tests (memgraph/execution_test.go)

Extend existing `GenerateBundle` test:

- Verify `Bundle.Claim` is populated when spec is claimed
- Create spec with DEPENDS_ON edges, advance upstream, verify `Dependencies`
  includes drift flag

### E2E Tests (e2e/api/)

Existing pipeline test already calls GenerateBundle. Update assertion:

- `BundleContent` is non-empty, starts with `---` (frontmatter)
- `BundleYaml` field no longer set (reserved)

## Migration

- Old `bundle_yaml` field is reserved, not removed — existing proto consumers
  see an empty string instead of an error
- Version field bumps from 1 to 2 — consumers can branch on this
- No database migration needed — all data already exists in the graph
- `GetPrime` RPC is unchanged — still available for constitution/project context
- Web frontend TypeScript references to `bundleYaml` will become compile errors
  after proto regeneration; these are caught by `task check` and are a no-op fix
  (the field is not currently used in the UI)
