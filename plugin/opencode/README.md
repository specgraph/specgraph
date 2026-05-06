# SpecGraph OpenCode Plugin

A thin OpenCode integration for SpecGraph projects.

## What's here

| Path | Purpose |
|------|---------|
| `.opencode/plugins/specgraph.ts` | session.start prime + tool.use post-stage nudges |
| `.opencode/skills/` | Symlink to repo-root `skills/` (shared with Claude Code and Cursor shims) |
| `package.json` | Publishable npm shape (peer-deps `@opencode-ai/plugin`) |

## Install (local path)

Add the plugin path to your project's `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "./path/to/plugin/opencode/.opencode/plugins/specgraph.ts"
  ]
}
```

`specgraph init` writes `opencode.json` for you with the right MCP server URL
and headers. The `plugin` array above is the additional install step for the
TS plugin layer; it's separate from the MCP config.

## Install (npm — future)

Once published:

```bash
npm install -D @specgraph/opencode-plugin
```

Then reference it in `opencode.json`:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["@specgraph/opencode-plugin"]
}
```

The npm publish step lives behind a separate follow-up bead. See `bd ready`
for the current state.

## What you get

- **Session prime.** `session.start` reads `specgraph://prime` via the
  specgraph CLI and prepends it to the session, so the model starts with
  the same project digest Claude Code and Cursor get.
- **Post-stage nudges.** `tool.use` after a successful
  `mcp__specgraph__author` with a stage action (`spark`, `shape`,
  `specify`, `decompose`, `approve`) suggests running the analytical
  passes registered for that stage.
- **Shared skills.** Authoring, graph queries, analytical passes, drift,
  conventions, troubleshooting — same SKILL.md packages used by the
  Claude Code and Cursor shims.

## Refreshing the skills symlink

`task plugin:sync` from the specgraph repo root recreates the symlink for
all three harness shims. Run it after a fresh checkout.

## Compatibility note

The `tool.use` field names (`tool`, `phase`, `input.action`, `suggest`) are
best-effort against the `@opencode-ai/plugin` package contract. If a future
version of OpenCode renames any of them, update the type-narrowing block in
`specgraph.ts` accordingly. The plugin uses `execFile` (argv array, no
shell) for the CLI invocation to avoid shell-injection footguns.
