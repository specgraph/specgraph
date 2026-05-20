# SpecGraph Claude Code Plugin

Thin Claude Code client for SpecGraph. Rich authoring workflow guidance —
spark, shape, specify, decompose, approve — is delivered by the SpecGraph
MCP server. Routing-and-when-to-call lives in skills shared across all
three supported harnesses (Claude Code, Cursor, OpenCode).

## Install

**Inside the specgraph repo:**

```bash
claude --plugin-dir ./plugin/specgraph
```

**For other projects:** Install via `specgraph init`, which writes the
canonical files into `.specgraph/agents/claude/` and manages
`.claude/settings.json` (adds the plugin to `extraKnownMarketplaces` and
`enabledPlugins`).

## Layout

This directory is an **authoring-convenience view**. Most entries (except
`README.md`) are symlinks pointing at the canonical location under
`internal/config/managedfiles/embedded/claude/`. Editing via the symlinks
edits the canonical copy directly — changes flow into the binary on the
next `task build`.

| Path | Symlink target | Purpose |
|------|----------------|---------|
| `.claude-plugin/` | `../../internal/config/managedfiles/embedded/claude/.claude-plugin` | Plugin manifest directory (`plugin.json`, `marketplace.json`) |
| `hooks/` | `../../internal/config/managedfiles/embedded/claude/hooks` | Hook scripts directory |
| `hooks/specgraph-session-start.sh` | (via `hooks/`) | Reads `specgraph://prime` from the MCP server at session start |
| `hooks/specgraph-post-stage.sh` | (via `hooks/`) | PostToolUse on `mcp__specgraph__author` — surfaces analytical passes after stage transitions |
| `routing-guide.md` | `../../internal/config/managedfiles/embedded/claude/routing-guide.md` | One-screen pointer |
| `README.md` | (this file) | Not a symlink; not written to end-user projects |

**Notes:**

- `plugin.json` inlines the hooks array directly — there is no separate `hooks.json`.
- Hook scripts carry a `specgraph-` prefix to avoid colliding with hooks a user has already added to `.claude/hooks/`.

Skills are not copied into end-user projects. They live in the CLI binary
(`internal/mcp/skills/embedded/`) and are fetched at runtime via MCP:

- `specgraph_skills_list` — catalog all available skills.
- `specgraph_skills_get` — fetch a specific skill by name (e.g., `specgraph-authoring`).
- `specgraph_skills_search` — keyword or regex lookup across skill content.
- `specgraph://skills/<name>` — resource URI for the same payload as `specgraph_skills_get`.

The MCP server (`specgraph serve`) also exposes:

- Prompts for each authoring stage (`spark`, `shape`, `specify`, `decompose`, `approve`)
- Tools: `author_start_stage`, `author`, `spec`, `graph_query`,
  `constitution`, `analytical_pass`, and others (see `Settings → Tools & MCPs`
  for the full list once connected)
- Resources: `specgraph://prime`, `specgraph://constitution`,
  `specgraph://graph/ready`

See `routing-guide.md` for the one-screen pointer.

## Post-stage automation contract

All three supported harnesses (Claude Code, Cursor, OpenCode) implement the
same user-visible behavior after a successful authoring stage transition:
the analytical passes registered for that stage in the server's
`passRegistry` are surfaced. The mechanism differs per harness; the
contract is the same.

| Harness | Mechanism | Source |
|---|---|---|
| Claude Code | `PostToolUse` hook on `mcp__specgraph__author` returning a `block` decision with the run-passes prompt | `plugin/specgraph/hooks/specgraph-post-stage.sh` |
| Cursor | A `.cursor/rules/specgraph-post-stage.mdc` rule that the model consults after stage edits | `plugin/cursor/.cursor/rules/specgraph-post-stage.mdc` |
| OpenCode | A `tool.execute.after` hook records the stage; the next `experimental.chat.system.transform` appends the run-passes prompt to the system prompt | `plugin/opencode/.opencode/plugins/specgraph.ts` |

Per-stage pass list (the `passes_for_stage` MCP tool returns the same data
at runtime):

| Stage | Auto-passes |
|---|---|
| `spark` | constitution-check |
| `shape` | constitution-check, peripheral-vision |
| `specify` | constitution-check, red-team, consistency |
| `decompose` | constitution-check, simplicity |
| `approve` | constitution-check |

The server is the source of truth. If you change `passRegistry` in
`internal/authoring/passes.go`, the per-harness automation picks the new
behavior up automatically — there's nothing per-harness to recompile.
