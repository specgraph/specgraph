---
name: specgraph-graph-query
summary: Query specs by relationships, status, or stage — ready work, blocked work, impact of a change.
description: Use when the user wants to find specs, see what's ready to work on, trace dependencies, or assess the impact of changing a spec. Routes to spec, graph_query, and the relevant MCP resources.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Graph Query

SpecGraph stores specs as graph nodes with first-class edges (dependencies,
blocks, compositions, decisions). This skill covers reading the graph: finding
specs, listing what's ready, walking dependencies, and impact analysis.

## When this skill applies

Trigger on questions like:

- "What specs are ready for me?"
- "What does X depend on?" / "What depends on X?"
- "Show me the graph" / "What's the impact of changing Y?"
- "List all specs" / "Find specs about Z"

## Tools and resources

| Intent | Use |
|---|---|
| List specs (with filters) | `spec` tool, `action: "list"` |
| Get a single spec | `spec` tool, `action: "get"`, with `slug` |
| Walk dependencies | `graph_query` tool, with `action: "deps"` or `"impact"` |
| Quick "what can I work on" | `specgraph://graph/ready` resource |
| Whole-graph view | `specgraph://graph` resource |
| Spec body and history | `specgraph://spec/<slug>` resource |

## Filters on `spec list`

The `spec` list action accepts:

- `stage` — filter by funnel stage (`spark`, `shape`, ..., `approved`, `done`)
- `domain` — filter by domain tag if the project uses domains
- `text_query` — substring match against intent and slug

Combine filters as needed. A typical "what's in flight" call is
`stage=specify,decompose,approved` plus the project's primary domain.

## Graph traversal

`graph_query` actions:

- `deps` — transitive dependencies of a slug (what it needs)
- `impact` — transitive dependents (what would be affected by change)
- `path` — shortest dependency path between two slugs

All traversals are bounded to 50 hops. Cycles are detected and reported.

## Resource cheat sheet

```text
specgraph://specs                   # list of all specs
specgraph://spec/{slug}             # one spec, full body
specgraph://spec/{slug}/changes     # change history
specgraph://decision/{slug}         # decision node
specgraph://constitution            # full constitution
specgraph://constitution/{layer}    # one layer (User/Org/Project/Domain)
specgraph://graph                   # graph summary
specgraph://graph/ready             # specs unblocked and ready
specgraph://findings                # all open findings
specgraph://prime                   # session-priming digest
```

## Don't

- Don't query the graph by reading raw storage paths. Use the MCP tools and
  resources; they enforce auth, edge semantics, and pagination.
- Don't infer "ready" from stage alone. A spec is ready when its dependencies
  are satisfied — `specgraph://graph/ready` does the math.

---

## Requires local CLI (source/CLI users only — MCP-only agents skip this)

The graph is fully queryable over MCP with the `spec` and `graph_query` tools
and the `specgraph://` resources above — an MCP-only agent needs nothing here.
Source/CLI users running the `specgraph` binary can list and inspect specs from
the command line, but the MCP tools and resources are the supported route for
agents and enforce the same auth, edge semantics, and pagination.
