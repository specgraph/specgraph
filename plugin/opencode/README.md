# SpecGraph OpenCode Plugin

A thin OpenCode integration for SpecGraph projects.

## What's here

| Path | Purpose |
|------|---------|
| `.opencode/plugins/specgraph.ts` | System-prompt prime + post-stage nudges |
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

- **Session prime.** The `experimental.chat.system.transform` hook reads
  `specgraph://prime` via the specgraph CLI and appends it to the system
  prompt on every turn, so the model has the same project digest Claude Code
  and Cursor get. The prime output is cached for the lifetime of the plugin
  instance — one CLI call per session, not per turn.
- **Post-stage nudges.** `tool.execute.after` watches for successful
  `mcp__specgraph__author` calls with a stage action (`spark`, `shape`,
  `specify`, `decompose`, `approve`) and queues a one-shot system-prompt
  addendum on the next turn that asks the model to run the registered
  analytical passes.
- **Shared skills.** Authoring, graph queries, analytical passes, drift,
  conventions, troubleshooting — same SKILL.md packages used by the
  Claude Code and Cursor shims.

## Refreshing the skills symlink

`task plugin:sync` from the specgraph repo root recreates the symlink for
all three harness shims. Run it after a fresh checkout.

## Implementation notes

The plugin matches the `Hooks` interface shipped with `@opencode-ai/plugin`
≥ 1.3 (verified against `index.d.ts`):

- `Plugin` is `(input: PluginInput) => Promise<Hooks>` — an async function
  returning the hooks object, not a plain object literal.
- `experimental.chat.system.transform` is the hook that exposes `output.system`
  for prepending to the system prompt. The `event` hook does not have an
  output channel and so cannot surface the prime.
- `tool.execute.after` receives `{ tool, sessionID, callID, args }` as `input`.
  The post-stage nudge is queued in a closure variable consumed by the next
  `system.transform` because `tool.execute.after`'s `output` parameter is
  scoped to the tool's own response, not the next message.

The plugin uses `execFile` (argv array, no shell) for the CLI invocation to
avoid shell-injection footguns if the URI list ever grows.

The OpenCode runtime integration has not yet been validated end-to-end inside
a running OpenCode session — the structure matches the type definitions, but
behavior verification is tracked as a follow-up bead.
