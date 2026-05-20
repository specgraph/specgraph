# SpecGraph Routing Guide

One-screen pointer to the SpecGraph MCP server. Detail lives in `skills/`.

## Authoring

Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`,
`decompose`, `approve`) or call `author_start_stage`. Persist with the
`author` tool. See `skills/specgraph-authoring/`.

## Querying

`spec`, `graph_query`, and the `specgraph://graph/ready` resource. See
`skills/specgraph-graph-query/`.

## Analytical review

`analytical_pass` with `action: "run"`. See
`skills/specgraph-analytical-passes/`.

## Constitution

`specgraph://constitution` resource for content; `constitution` tool for
get/update.

## Setup

```bash
docker info
specgraph init
specgraph serve
```

## Never

- Don't invent dotted tool names; tools are flat with `action` parameters.
- Don't approve a spec on behalf of the user; approval requires explicit
  user sign-off.
