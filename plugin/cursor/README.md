# SpecGraph Cursor Shim

A thin Cursor integration for SpecGraph projects.

## What's here

| Path | Purpose |
|------|---------|
| `.cursor/rules/specgraph.mdc` | One-screen routing rule. Written by `specgraph init`; do not edit by hand. |
| `.cursor/rules/specgraph-post-stage.mdc` | Post-stage analytical-pass guidance. Written by `specgraph init`; do not edit by hand. |
| `.cursor/rules/specgraph-bootstrap.mdc` | Project pointer block. Written by `specgraph init` (markdown-block strategy — user owns the rest of the file). |
| `.cursor/skills/` | Symlink to repo-root `skills/` (shared with Claude Code and OpenCode shims) |

## Install (per project)

Run `specgraph init` in the project root. This writes the three `.cursor/rules/*.mdc`
files above and configures `.cursor/mcp.json` with the right URL and headers.
No manual file copying required.

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
