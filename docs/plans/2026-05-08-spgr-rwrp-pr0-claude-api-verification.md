# spgr-rwrp PR 0 — Claude API verification report

**Date:** 2026-05-08
**Claude version:** 2.1.133 (Claude Code)
**Spike location:** `~/Code/specgraph-pr0-spike/`
**Bead:** `spgr-9zxp`

## Summary

| Claim | Verdict |
|---|---|
| 1. Relative path acceptance in `extraKnownMarketplaces` | GREEN |
| 2. `autoUpdate: false` honoured | GREEN (with caveat — see below) |
| 3. `${CLAUDE_PLUGIN_ROOT}` resolves to plugin root | GREEN |

PR A may proceed. Two schema corrections must fold into the spec doc as v5 (see "Spec corrections" section).

## Spec corrections required

- `extraKnownMarketplaces.<name>.source.path` must point at the **marketplace root** (the directory CONTAINING `.claude-plugin/`), NOT at `.claude-plugin/` itself. PR A must use paths like `./.specgraph/agents/claude` (the dir containing `.claude-plugin/`), not `./.specgraph/agents/claude/.claude-plugin`.
- `marketplace.json` `plugins[].source` is a **directory path** with mandatory `./` prefix, NOT a file path. Use `"source": "./"` for a co-located plugin and `"source": "./sub-plugin"` for a sub-directory plugin — never `"./.claude-plugin/plugin.json"`.

## Claim 1 — Relative path acceptance

**Verdict: GREEN.**

`extraKnownMarketplaces.<name>.source.path` accepts a project-relative directory path. Claude appends `/.claude-plugin/marketplace.json` to the path itself. Test value used: `"./test-marketplace"`. Plugin loaded successfully and SessionStart hooks fired.

**Schema correction:** the path must point at the **marketplace root** (the directory CONTAINING `.claude-plugin/`), not at `.claude-plugin/` itself. The spec's design doc had this wrong; PR A must use paths like `./.specgraph/agents/claude` (the dir containing `.claude-plugin/`), not `./.specgraph/agents/claude/.claude-plugin`.

Hook output proving plugin loaded (from `.spike-out/marketplace-shape.txt`):

```text
ts: 2026-05-08T18:21:37-04:00
pwd: ~/Code/specgraph-pr0-spike
CLAUDE_PLUGIN_ROOT: /Volumes/Code/specgraph-pr0-spike/test-marketplace/
CLAUDE_PROJECT_DIR: /Volumes/Code/specgraph-pr0-spike
argv0: /Volumes/Code/specgraph-pr0-spike/test-marketplace//hooks/verify-root.sh
```

## Claim 2 — `autoUpdate: false`

**Verdict: GREEN with caveat.**

`autoUpdate: false` is a real field on the marketplace entry and is accepted without validation error. Behaviour observed:

- Registry metadata (version, lastUpdated, gitCommitSha in `~/.claude/plugins/installed_plugins.json`) stays pinned at the originally-installed version even when on-disk plugin.json is bumped 0.1.0 → 0.1.1.
- However, for `source: directory` marketplaces, Claude does NOT cache plugin files. Hooks, skills, agents, and commands are read **live** from the source directory on every session. So `autoUpdate: false` does NOT prevent file-content drift between sessions.

For SpecGraph's design, this is acceptable: we WANT every `specgraph init` refresh to be visible to the next Claude session. `autoUpdate: false` serves as documentation-of-intent that we control the version axis; the live-file-read behaviour is exactly what we want operationally.

## Claim 3 — `${CLAUDE_PLUGIN_ROOT}` resolution

**Verdict: GREEN.**

`${CLAUDE_PLUGIN_ROOT}` resolves to **the plugin's own root directory** (the dir containing the plugin's `.claude-plugin/`), regardless of marketplace location. Two test cases:

**3a: Co-located (plugin in same `.claude-plugin/` as marketplace)** — `marketplace.json` and `plugin.json` both live at `test-marketplace/.claude-plugin/`. Hook fires with:

```text
CLAUDE_PLUGIN_ROOT: /Volumes/Code/specgraph-pr0-spike/test-marketplace/
```

This is the marketplace root, which IS also the plugin root in the co-located case. ✓

**3b: Sub-plugin (plugin in a subdirectory of marketplace)** — `marketplace.json` at `test-marketplace/.claude-plugin/`, plugin at `test-marketplace/sub-plugin/.claude-plugin/`. Hook fires with:

```text
CLAUDE_PLUGIN_ROOT: /Volumes/Code/specgraph-pr0-spike/test-marketplace/sub-plugin
```

This is the plugin's own root (`sub-plugin/`), distinct from the marketplace root (`test-marketplace/`). ✓

The spec's `.specgraph/agents/claude/` layout is structurally correct — plugin root and marketplace root coincide (single-plugin co-located marketplace), and `${CLAUDE_PLUGIN_ROOT}` resolves to that path. Hook references like `${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-session-start.sh` resolve correctly.

## Bonus findings

### marketplace.json plugin source schema

Independent of the three claims but discovered during verification: the `plugins[].source` field in `marketplace.json` is a **directory path** with mandatory `./` prefix, NOT a file path. Working forms:

- Co-located plugin: `"source": "./"` (NOT `"./.claude-plugin/plugin.json"`)
- Sub-directory plugin: `"source": "./sub-plugin"` (NOT `"./sub-plugin/.claude-plugin/plugin.json"`)

`..` traversal is blocked. All sub-plugins must live INSIDE the marketplace root.

### symlink resolution

Claude resolves symlinks on the plugin path. The user's `pwd` shows `~/...` (symlink) but `CLAUDE_PLUGIN_ROOT` shows `/Volumes/Code/...` (real path). This matches the spec's choice to use `EvalSymlinks(projectRoot)` for the throttle file key — Claude does the equivalent internally.

## Methodology notes

- Claude version: 2.1.133
- Hook trigger: SessionStart, fired by `claude -p "Reply with..." --output-format text`
- All artefacts preserved at `~/Code/specgraph-pr0-spike/.spike-out/` for re-inspection
- Slash commands (`/plugin list`) are not available in `claude -p` mode — verification used SessionStart hooks + Claude's debug logs (`~/.claude/debug/<uuid>.txt`) and `~/.claude/plugins/installed_plugins.json`

## References

- Design: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
- Plan: `docs/plans/2026-05-08-spgr-rwrp-pr0-plan.md`
- Claude plugins docs: <https://code.claude.com/docs/en/plugins>, <https://code.claude.com/docs/en/discover-plugins>, <https://code.claude.com/docs/en/plugins-reference>, <https://code.claude.com/docs/en/plugin-marketplaces>
