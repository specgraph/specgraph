# SpecGraph Cursor Shim

A thin Cursor integration for SpecGraph projects.

## What's here

| Path | Purpose |
|------|---------|
| `.cursor/rules/specgraph.md` | One-screen routing rule pointing to MCP prompts/tools/resources |
| `.cursor/rules/post-stage.md` | Post-stage analytical-pass guidance |
| `.cursor/skills/` | Symlink to repo-root `skills/` (shared with Claude Code and OpenCode shims) |

## Install (per project)

Two options.

**Option 1 — copy in.** Copy the `.cursor/rules/*.md` files into your
project's `.cursor/rules/`. Resolve the `skills/` symlink and copy the
directory if you want skills available locally.

**Option 2 — Cursor remote rules.** Cursor supports importing rules from a
GitHub repository (Settings → Rules → Add). Point it at this repo's
`plugin/cursor/.cursor/rules/` path.

The MCP server itself is configured by `specgraph init`, which writes
`.cursor/mcp.json` with the right URL and headers. You don't need to edit
it by hand.

## What you get

- Routing guidance covering authoring, query, constitution, analytical
  passes, and drift.
- Post-stage automation: after a stage transition the agent runs the
  registered analytical passes for that stage.
- The shared SpecGraph skills (`specgraph-authoring`, `specgraph-graph-query`,
  `specgraph-analytical-passes`, `specgraph-drift`, `specgraph-conventions`,
  `specgraph-troubleshooting`).

## Refreshing the skills symlink

`task plugin:sync` from the specgraph repo root recreates the symlink for
all three harness shims. Run it after a fresh checkout.
