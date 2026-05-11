# OpenCode plugin smoke test

Manual end-to-end procedure for verifying `.specgraph/agents/opencode/specgraph.ts`
against a running OpenCode session. Captures the contract that has no
automated test coverage today (bead `spgr-f0di`).

## Prereqs

- OpenCode CLI installed (`opencode --version` ≥ 1.14)
- `specgraph` binary on `PATH` (`task build && ln -sf $(pwd)/specgraph ~/.local/bin/`)
- specgraph server running (`specgraph serve &`) and reachable
- `SPECGRAPH_API_KEY` set in environment
- `opencode.json` in the project root with both an `mcp.specgraph` entry
  and the plugin path in `plugin`. Run `specgraph init` to write both:

  ```bash
  specgraph init
  ```

  This writes `.specgraph/agents/opencode/specgraph.ts` from the embedded
  source and adds `./.specgraph/agents/opencode/specgraph.ts` to
  `opencode.json`'s `plugin` array via union-merge. Verify both landed:

  ```bash
  ls -la .specgraph/agents/opencode/specgraph.ts
  grep -A 3 '"plugin"' opencode.json
  ```

  The resulting `opencode.json` should contain:

  ```json
  {
    "$schema": "https://opencode.ai/config.json",
    "plugin": ["./.specgraph/agents/opencode/specgraph.ts"],
    "mcp": {
      "specgraph": {
        "type": "remote",
        "enabled": true,
        "url": "http://127.0.0.1:7890/mcp/",
        "headers": {
          "Authorization": "Bearer {env:SPECGRAPH_API_KEY}",
          "X-Specgraph-Project": "specgraph"
        }
      }
    }
  }
  ```

  Both the MCP fields and the `plugin` array entry are managed by
  `specgraph init` (idempotent).

## Observation strategy

OpenCode does not expose the assembled system prompt in its logs. To observe
the plugin's behaviour directly, temporarily add `console.error` calls inside
the two hooks (`experimental.chat.system.transform`, `tool.execute.after`) —
they surface in OpenCode's stderr stream when running with
`--print-logs --log-level INFO` or `DEBUG`. **Strip the instrumentation
before committing.**

Suggested marker prefix: `[specgraph-smoke]`. Grep for it after each run.

## Test cases

### 1. Plugin loads without import errors

```bash
opencode run --print-logs --log-level DEBUG -- "Reply: PONG-1" 2>&1 \
  | grep "loading plugin" | grep specgraph
```

**Expected:** one `service=plugin path=...specgraph.ts loading plugin` line.

### 2. MCP transport connects

```bash
opencode run --print-logs --log-level DEBUG -- "Reply: PONG-1" 2>&1 \
  | grep "service=mcp key=specgraph"
```

**Expected:** `transport=StreamableHTTP connected` and a non-zero `toolCount`.

### 3. Prime injected into system prompt every turn

With instrumentation in `chat.system.transform`:

```ts
console.error(`[specgraph-smoke] system.transform fired; prime.len=${prime.length}`);
```

Run any prompt. The hook should fire at least once per LLM call (so often
twice per `opencode run`: title-gen + main agent).

**Expected:** `prime.len` non-zero on every fire; first chars `# SpecGraph
Session Prime`.

### 4. Stage nudge queues on author tool, consumes one-shot on next turn

Drive the author tool from a prompt:

```bash
opencode run --dangerously-skip-permissions --print-logs --log-level INFO -- \
  'Use the specgraph_author tool with arguments {"action":"spark","intent":"smoke","problem":"verify nudge","context":"smoke"}. Then reply: SPARK-DONE.'
```

With instrumentation around the queue/consume points, expect this exact
sequence per stage transition:

```text
[specgraph-smoke] tool.execute.after fired tool=specgraph_author
[specgraph-smoke] queued nudge for stage=spark
[specgraph-smoke] system.transform fired; prime.len=N pendingNudge=true
[specgraph-smoke] pushed nudge first40="Run the analytical passes registered for"
[specgraph-smoke] system.transform fired; ... pendingNudge=false   ← one-shot consumption
```

### 5. Non-author tools are ignored

The same run typically also fires `tool.execute.after` for `read`, `grep`,
`specgraph_spec`, `specgraph_author_start_stage`, etc. None should result in
`queued nudge` lines — the suffix check (`endsWith("_author")`) and the
action whitelist exclude all of them.

### 6. Soft-fail when CLI is missing or server is unreachable

```bash
# Stop the server (or temporarily unset the symlink for the CLI)
pkill -f "specgraph serve"
opencode run --print-logs --log-level INFO -- "Reply: PONG-OFFLINE"
```

**Expected:** session completes; `prime.len` is non-zero (the soft-fail
message length); `first40` starts with `"# specgraph prime unavailable"`.
The cache holds the failure message, so a server that comes online
mid-session is **not** retried — this is intentional.

## Known limitations (not fixed here)

- **Multi-stage transitions in a single LLM turn.** `pendingStageNudge` is a
  single slot; if `tool.execute.after` fires twice for two different stages
  before any `system.transform` runs, the second overwrites the first. In
  practice the model issues stage transitions over multiple turns, so the
  queue depth has been "good enough" — but if we ever want strict ordering,
  switch to an array.
- **Long-running session retry.** `loadPrime` caches both success and
  failure for the lifetime of the plugin instance. A server that comes online
  mid-session keeps showing the failure message. Acceptable for short
  sessions; revisit if real users report stale-failure complaints.

## Cleanup

After each run, abandon any specs created during testing:

```bash
specgraph abandon <slug> --reason="smoke test cleanup"
```

If you instrumented the plugin with `console.error` statements, revert
those before committing.

`.specgraph/agents/` is gitignored, so the written plugin file does not
need to be removed from version control — it lives only in the local
working tree.
