# SpecGraph Claude Code Plugin

Thin Claude Code client for SpecGraph. Rich authoring workflow guidance —
spark, shape, specify, decompose, approve — is delivered by the SpecGraph
MCP server, not by skills bundled in this plugin.

## Install

**Inside the specgraph repo:**

```bash
claude --plugin-dir ./plugin/specgraph
```

**For other projects:** Install via Claude Code plugin marketplace (coming soon).

## Layout

| Path | Purpose |
|------|---------|
| `.claude-plugin/plugin.json` | Plugin manifest |
| `hooks/session-start.sh` | Reads `specgraph://prime` from the MCP server at session start |
| `routing-guide.md` | Tells Claude which MCP prompts/tools/resources to use for each user intent |

The MCP server (`specgraph serve`) exposes:

- Prompts for each authoring stage (`spark`, `shape`, `specify`, `decompose`, `approve`)
- Tools: `author.start_stage`, `author.<stage>`, `spec.list`, `spec.get`,
  `graph_query`, `constitution.update`, `analytical_pass.run`
- Resources: `specgraph://prime`, `specgraph://constitution`,
  `specgraph://graph/ready`

See `routing-guide.md` for routing rules.
