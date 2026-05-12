# Cursor plugin smoke test

Manual end-to-end procedure for verifying `.cursor/rules/specgraph.mdc` and
`.cursor/rules/specgraph-post-stage.mdc` against a running Cursor session.
Captures the contract that has no automated test coverage today.

## Prereqs

- Cursor installed (any reasonably recent version with `.cursor/rules/` support)
- `specgraph` binary on `PATH` (`task build && ln -sf $(pwd)/specgraph ~/.local/bin/`)
- `specgraph` server running (`specgraph serve &`) and reachable
- `SPECGRAPH_API_KEY` set in environment
- A fresh project directory (NOT this repo, to avoid dogfood collisions)

## Setup

In a fresh project dir:

````bash
specgraph init --slug smoke-test --server-url http://localhost:9090
````

This writes (among others):

- `.cursor/mcp.json`
- `.cursor/rules/specgraph-bootstrap.mdc`
- `.cursor/rules/specgraph.mdc`
- `.cursor/rules/specgraph-post-stage.mdc`

Verify all three Cursor rules landed:

````bash
ls -la .cursor/rules/
````

Each file should be present. Inspect one:

````bash
head -10 .cursor/rules/specgraph.mdc
````

Expected layout:

````text
---
description: SpecGraph routing — use when the user mentions specs, ...
alwaysApply: false
---

<!-- specgraph:init v=2 sha256=<hex> -->
# SpecGraph Routing

You have access to the SpecGraph MCP server. ...
````

Sentinel line sits between the closing `---` and the `# SpecGraph Routing` heading.
Cursor's mdc parser should treat the comment as inert.

## Verification

1. **Open the project in Cursor.** Open Cursor's Rules panel (Settings → Rules, or
   the editor's rules sidebar).
2. **Confirm both rules appear** with their descriptions:
   - `specgraph.mdc`: "SpecGraph routing — use when the user mentions specs, ..."
   - `specgraph-post-stage.mdc`: "After a SpecGraph stage transition, run the
     analytical passes ..."
3. **Trigger `specgraph.mdc`** by prompting Cursor's agent with a question about
   specs, e.g. "What's the next step in the shape stage?". Cursor should apply the
   rule (visible in the rule-fired indicator if your version of Cursor shows one).
   The agent should respond with awareness of MCP prompts
   `spark`/`shape`/`specify`/`decompose`/`approve`.
4. **Trigger `specgraph-post-stage.mdc`** by completing a stage transition via the
   `author` MCP tool. The agent should follow up by calling `analytical_pass` for
   each registered pass type.
5. **Inspect the rule preview** in Cursor's Rules panel for both files. The HTML
   comment with the sentinel SHOULD NOT render as visible content — if it appears in
   the rendered preview, that's a regression; file a bead.

## Idempotency

Run `specgraph init` again:

````bash
specgraph init
````

Verify no file changed:

````bash
git -C . status   # or jj status — if the test dir is a repo
````

Both rule files should show as untouched (no diff). If a hash byte changed, the
canonical content drifted from the embedded source; investigate.

## Cleanup

````bash
rm -rf .cursor/ .specgraph/ opencode.json
````
