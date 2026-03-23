# Markdown CLI Output

**Date:** 2026-03-22
**Closes:** spgr-f3s

## Problem

CLI read commands output either plain key-value text (human) or protojson
(scripts). Neither format works well for AI agents, which need structured
but readable content they can reason over. The current text format also
omits authoring outputs (spark/shape/specify/decompose data), edges, and
decisions from `specgraph show`.

## Approach

Replace the text output format with Markdown as the default for all read
commands. JSON stays via `--json` flag for scripts. A new `internal/render/`
package centralizes Markdown rendering to keep it DRY across commands.

Primary consumer: AI agents reading specs from the graph. Human readability
is a secondary benefit.

## Prerequisite: Authoring Output Read Path

**Critical dependency:** `GetSpec` currently returns only metadata fields --
no authoring outputs (spark/shape/specify/decompose). The storage layer
stores these as JSON properties on the Memgraph node, and there are
`Store*Output` write methods, but no read-back RPCs.

Before `render.Spec` can show authoring sections, we need either:

1. **Extend `GetSpec` to include authoring outputs** in the response proto
   (add optional output fields to the `Spec` message), or
2. **Add a `GetSpecDetail` RPC** that returns the spec with all outputs.

This spec assumes option 1 (extend `GetSpec`) as a prerequisite task.
If authoring output fields are not yet available in the proto response,
`render.Spec` renders only the metadata table and notes.

## Package: `internal/render/`

New package with one function per entity type. Functions accept proto types
(commands already have proto responses) and return Markdown strings.

```go
package render

func Spec(s *specv1.Spec) string
func SpecList(specs []*specv1.Spec) string
func EdgeList(slug string, edges []*specv1.Edge) string
func Decision(d *specv1.Decision) string
func DecisionList(ds []*specv1.Decision) string
func Constitution(c *specv1.Constitution) string
func DriftReport(reports []*specv1.DriftReport) string
func Findings(fs []*specv1.AnalyticalFinding) string
func NodeRefList(title string, refs []*specv1.NodeRef) string
```

### Shared Helpers

```go
func metadataTable(pairs [][2]string) string     // | Field | Value | table
func itemTable(headers []string, rows [][]string) string  // generic markdown table
func section(level int, title, body string) string         // ## Title\n\nbody
```

## Markdown Format

### `specgraph show <slug>` (Spec)

```markdown
# login-api

> Implement OAuth2 login flow with refresh token rotation

| Field | Value |
|-------|-------|
| Stage | specify |
| Priority | p1 |
| Complexity | medium |
| Version | 3 |
| Lifecycle | task |
```

Authoring output sections (Spark, Shape, Specify, Decompose) appear only
when the proto response includes output data. See Prerequisite section.

### `specgraph list` (SpecList)

Intentionally drops the ID column (agents don't need ULIDs; Intent is more
useful). Adds Intent column.

```markdown
| Slug | Stage | Priority | Intent |
|------|-------|----------|--------|
| login-api | specify | p1 | Implement OAuth2 login flow |
| webhook-notifications | shape | p2 | Event notification system |
```

### `specgraph edge list <slug>` (EdgeList)

Direction is derived from whether `from_id` matches the queried slug.
`content_hash_at_link` is not available in the current `Edge` proto -- the
Content Hash column is omitted until the proto is extended.

```markdown
## Edges for login-api

| Type | Direction | Target |
|------|-----------|--------|
| DEPENDS_ON | outgoing | token-storage |
| BLOCKS | incoming | api-gateway |
```

### `specgraph decision show <slug>` (Decision)

The `Spec` field is not available in the `Decision` proto (the link is via
DECIDED_IN edges, not embedded). Omitted from the metadata table.

```markdown
# refresh-token-rotation

> Use rotating tokens with family tracking

| Field | Value |
|-------|-------|
| Status | accepted |

**Decision:** Use rotating refresh tokens with family revocation on reuse.

**Rationale:** Security audit requires token rotation. Sliding sessions
don't meet compliance requirements.
```

### `specgraph deps/ready/critical-path/impact` (NodeRefList)

Graph traversal commands (`deps`, `ready`, `critical-path`, `impact`) all
use `printNodeRefs` today. Replace with `render.NodeRefList`:

```markdown
## Dependencies

| Slug | Stage |
|------|-------|
| token-storage | approved |
| crypto-utils | done |
```

### Other Commands

- **`constitution show`**: Sections for principles, constraints, tech stack,
  references. Similar to the existing emitter output but rendered to stdout.
- **`drift`**: Grouped by spec slug, showing upstream hash vs edge hash per
  drift item. Accepts `[]*specv1.DriftReport` (not flattened DriftItems).

### Out of Scope

- **`progress`**: Operational/streaming output with timestamps. Stays as
  custom text format.
- **`constitution emit --format`**: The `--format claude-md|cursorrules|agents-md`
  flag serves a different purpose (file generation, not display). Not touched.

## CLI Flag Changes

- `specgraph show`: Drop `--format text|json`, add `--json` boolean flag.
  This is the only command that currently has `--format`.
- All other read commands: Add `--json` boolean flag. Most had no format
  flag before -- they just get a new `--json` option.

Affected commands and their render functions:

| Command | Render function |
|---------|-----------------|
| `specgraph show <slug>` | `render.Spec` |
| `specgraph list` | `render.SpecList` |
| `specgraph edge list <slug>` | `render.EdgeList` |
| `specgraph decision show <slug>` | `render.Decision` |
| `specgraph decision list` | `render.DecisionList` |
| `specgraph constitution show` | `render.Constitution` |
| `specgraph drift <slug>` | `render.DriftReport` |
| `specgraph deps <slug>` | `render.NodeRefList` |
| `specgraph ready` | `render.NodeRefList` |
| `specgraph critical-path <slug>` | `render.NodeRefList` |
| `specgraph impact <slug>` | `render.NodeRefList` |

Write commands (`create`, `update`, `spark`, `shape`, `specify`, `decompose`,
`approve`, `claim`, `report`) keep their current one-line confirmation output
unchanged.

### Findings Command

`specgraph findings list <slug>` does not exist yet. This spec includes
creating it as a new command file.

## Affected Files

| File | Change |
|------|--------|
| `internal/render/markdown.go` | New -- shared helpers |
| `internal/render/spec.go` | New -- Spec + SpecList |
| `internal/render/edge.go` | New -- EdgeList |
| `internal/render/decision.go` | New -- Decision + DecisionList |
| `internal/render/constitution.go` | New -- Constitution |
| `internal/render/drift.go` | New -- DriftReport |
| `internal/render/findings.go` | New -- Findings |
| `internal/render/noderef.go` | New -- NodeRefList |
| `internal/render/*_test.go` | New -- unit tests per file |
| `cmd/specgraph/spec.go` | Replace text rendering, --format -> --json |
| `cmd/specgraph/edge.go` | Replace text rendering, add --json flag |
| `cmd/specgraph/decision.go` | Replace text rendering, add --json flag |
| `cmd/specgraph/constitution.go` | Replace text rendering, add --json flag |
| `cmd/specgraph/constitution_test.go` | Update test assertions if needed |
| `cmd/specgraph/graph.go` | Replace printNodeRefs with render.NodeRefList, add --json |
| `cmd/specgraph/lifecycle.go` | Replace drift text rendering with render.DriftReport, add --json (drift acknowledge is a write command -- unchanged) |
| `cmd/specgraph/findings.go` | New -- findings list command |
| `cmd/specgraph/table.go` | May be removable if all commands use render package |
| `e2e/cli/*.go` | Update assertions that check text output format |

## Testing

Each render function gets a unit test with a proto message fixture verifying
the Markdown output contains expected sections, tables, and content. Tests
use `strings.Contains` for key sections rather than exact string matching
(fragile).

CLI e2e tests that assert on text output need updating to expect Markdown
format.
