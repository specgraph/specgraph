# SpecGraph Routing Guide

One-screen pointer to the SpecGraph MCP server. Detail lives in the skills
served via MCP.

## Loading a SpecGraph skill

When the conversation calls for SpecGraph guidance, fetch the skill via MCP
rather than relying on local files:

- `specgraph_skills_list` — see what's available.
- `specgraph_skills_search` — keyword/regex lookup
  (`{"query": "drift"}` or `{"query": "\\bdrift\\b", "regex": true}`).
- `specgraph_skills_get` — fetch a specific skill by name
  (`{"name": "specgraph-authoring"}`).
- `specgraph://skills/<name>` — the same payload as `specgraph_skills_get`
  via the resource URI.

## Authoring

Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`,
`decompose`, `approve`) or call `author_start_stage`. Persist with the
`author` tool. Fetch `specgraph_skills_get` with `specgraph-authoring` for
the full workflow guidance.

## Querying

`spec`, `graph_query`, and the `specgraph://graph/ready` resource. Fetch
`specgraph_skills_get` with `specgraph-graph-query` for detail.

## Analytical review

`analytical_pass` with `action: "run"`. Fetch `specgraph_skills_get` with
`specgraph-analytical-passes` for detail.

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
