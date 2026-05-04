# SpecGraph Routing Guide

You have access to the SpecGraph MCP server for spec-driven development on
this project. This guide tells you where to go; the MCP carries the how.

## When the user wants to author or update a spec

- Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`,
  `decompose`, `approve`), or call the `author.start_stage` tool with the
  same stage and spec slug.
- Conduct the elicitation. Call the matching `author.<stage>` tool to
  persist stage output + conversation exchanges atomically in one call.

## When the user wants to query specs

- `spec.list` with filters
- `spec.get` for a single spec
- `graph_query` for dependency or impact traversal
- `specgraph://graph/ready` resource for "what can I work on"

## When the user wants to see the constitution

- `specgraph://constitution` resource for full content
- `constitution.update` tool to modify

## When the user wants analytical review

- `analytical_pass.run` for constitution-check, red-team, peripheral-vision,
  consistency, simplicity

## Never

- Don't call `author.<stage>` without `conversation_exchanges` for shape,
  specify, decompose — the server will reject the call.
- Don't approve a spec on behalf of the user; approval requires explicit
  user sign-off.

## Project setup (server not yet running)

```bash
docker info
specgraph init
specgraph serve
```
