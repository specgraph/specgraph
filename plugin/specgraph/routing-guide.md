# SpecGraph Routing Guide

You have access to the SpecGraph MCP server for spec-driven development on
this project. This guide tells you where to go; the MCP carries the how.

## When the user wants to author or update a spec

- Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`,
  `decompose`, `approve`), or call the `author_start_stage` tool with the
  same `stage`, spec `slug`, and optional `posture`.
- Conduct the elicitation. Persist stage output with the `author` tool by
  setting `action` to `spark`, `shape`, `specify`, `decompose`, or `approve`.

## When the user wants to query specs

- `spec` with `action: "list"` and optional filters
- `spec` with `action: "get"` for a single spec
- `graph_query` for dependency or impact traversal
- `specgraph://graph/ready` resource for "what can I work on"

## When the user wants to see the constitution

- `specgraph://constitution` resource for full content
- `constitution` with `action: "get"` for JSON round-tripping
- `constitution` with `action: "update"` to modify

## When the user wants analytical review

- `analytical_pass` with `action: "run"` for constitution-check, red-team,
  peripheral-vision, consistency, simplicity

## Never

- Don't invent dotted tool names; the MCP tools are flat names with `action`
  parameters.
- Don't approve a spec on behalf of the user; approval requires explicit
  user sign-off.

## Project setup (server not yet running)

```bash
docker info
specgraph init
specgraph serve
```
